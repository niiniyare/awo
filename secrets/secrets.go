// Package secrets provides the SecretProvider abstraction for retrieving
// application secrets at runtime.
//
// Framework code never hard-codes secrets. All sensitive values — database
// passwords, JWT signing keys, API keys — are retrieved through a
// SecretProvider that callers supply at startup.
//
// # Providers
//
//   - EnvProvider  — reads from environment variables (default, suitable for containers)
//   - StaticProvider — holds a fixed map (testing only)
//
// # Usage
//
//	p := secrets.NewEnvProvider(secrets.WithPrefix("AWO_"))
//	jwtKey, err := p.Get(ctx, "JWT_SECRET")
package secrets

import (
	"context"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
)

// SecretProvider retrieves secret values by key.
// Implementations must be safe for concurrent use.
type SecretProvider interface {
	// Get returns the secret value for key.
	// Returns ErrNotFound if the secret is not configured.
	Get(ctx context.Context, key string) (string, error)
}

// ErrNotFound is returned when a secret key is not configured.
type ErrNotFound struct {
	Key string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("secrets: key %q not found", e.Key)
}

// IsNotFound reports whether err is an ErrNotFound.
func IsNotFound(err error) bool {
	_, ok := err.(*ErrNotFound)
	return ok
}

// --- EnvProvider ---

// EnvOption configures an EnvProvider.
type EnvOption func(*envConfig)

type envConfig struct {
	prefix    string
	uppercase bool
}

// WithPrefix adds a prefix to every key before looking up the environment
// variable. E.g. prefix "AWO_" makes Get("JWT_SECRET") read AWO_JWT_SECRET.
func WithPrefix(prefix string) EnvOption {
	return func(c *envConfig) { c.prefix = prefix }
}

// WithUppercase converts keys to uppercase before lookup (default true).
func WithUppercase(u bool) EnvOption {
	return func(c *envConfig) { c.uppercase = u }
}

// EnvProvider reads secrets from environment variables.
// This is the default provider for containerised deployments where secrets
// are injected via env vars from a secret manager (Vault, AWS SM, etc.).
type EnvProvider struct {
	cfg envConfig
}

// NewEnvProvider creates an EnvProvider with the given options.
func NewEnvProvider(opts ...EnvOption) *EnvProvider {
	cfg := envConfig{uppercase: true}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &EnvProvider{cfg: cfg}
}

// Get reads the environment variable named "{prefix}{key}" (uppercased if configured).
func (p *EnvProvider) Get(_ context.Context, key string) (string, error) {
	envKey := p.cfg.prefix + key
	if p.cfg.uppercase {
		envKey = strings.ToUpper(envKey)
	}
	val, ok := os.LookupEnv(envKey)
	if !ok {
		return "", &ErrNotFound{Key: key}
	}
	return val, nil
}

// --- StaticProvider ---

// StaticProvider holds a fixed map of secrets. Use in tests only.
// Never use in production — static providers hard-code secrets in memory.
type StaticProvider struct {
	mu      sync.RWMutex
	secrets map[string]string
}

// NewStaticProvider creates a StaticProvider with the given key-value pairs.
func NewStaticProvider(m map[string]string) *StaticProvider {
	s := make(map[string]string, len(m))
	maps.Copy(s, m)
	return &StaticProvider{secrets: s}
}

// Get retrieves the value for key from the static map.
func (p *StaticProvider) Get(_ context.Context, key string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.secrets[key]
	if !ok {
		return "", &ErrNotFound{Key: key}
	}
	return v, nil
}

// Set adds or replaces a secret in the static map.
// Use in tests to rotate secrets mid-test.
func (p *StaticProvider) Set(key, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.secrets[key] = value
}

// --- MustGet helper ---

// MustGet retrieves key from p and panics if the key is not found.
// Use at startup to enforce required secrets early.
func MustGet(ctx context.Context, p SecretProvider, key string) string {
	v, err := p.Get(ctx, key)
	if err != nil {
		panic(fmt.Sprintf("secrets.MustGet: required secret %q not configured: %v", key, err))
	}
	return v
}

// RequireAll retrieves all keys from p and returns them as a map.
// Returns an error listing all missing keys.
func RequireAll(ctx context.Context, p SecretProvider, keys ...string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	var missing []string
	for _, k := range keys {
		v, err := p.Get(ctx, k)
		if err != nil {
			missing = append(missing, k)
			continue
		}
		result[k] = v
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("secrets.RequireAll: missing required secrets: %s", strings.Join(missing, ", "))
	}
	return result, nil
}
