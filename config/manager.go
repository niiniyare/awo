package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Manager loads configuration once and provides safe, concurrent access to
// both the core Config and any number of module-registered sections. It is
// the extension point this package is built around: a module that needs
// configuration does not modify Config or this package — it defines its
// own struct, calls Register, and reads that struct back whenever it
// likes.
//
// The zero value is not usable; construct one with New. A Manager is safe
// for concurrent use by multiple goroutines once Load has returned.
type Manager struct {
	mu       sync.RWMutex
	v        *viper.Viper
	core     *Config
	sections map[string]any // key -> pointer to a registered target struct
}

// New returns a Manager with no configuration loaded yet. Call Load before
// reading anything from it.
func New() *Manager {
	return &Manager{
		v:        viper.New(),
		sections: make(map[string]any),
	}
}

// Load reads configuration from path (pass "" to rely on defaults and
// environment variables only) into the core Config and into every section
// previously — or subsequently — registered via Register. Load may be
// called more than once (e.g. in tests, or to reload deliberately); each
// call re-reads the source from scratch and re-validates everything.
func (m *Manager) Load(path string, opts ...Option) error {
	o := loadOptions{envPrefix: "AWO"}
	for _, opt := range opts {
		opt(&o)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.v.SetEnvPrefix(o.envPrefix)
	m.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	m.v.AutomaticEnv()

	// Common unprefixed env vars that hosting platforms inject directly.
	_ = m.v.BindEnv("database.url", "DATABASE_URL")
	_ = m.v.BindEnv("redis.url", "REDIS_URL")
	_ = m.v.BindEnv("app.port", "PORT")
	_ = m.v.BindEnv("app.name", "APP_NAME")

	setCoreDefaults(m.v)
	for k, val := range o.defaults {
		m.v.SetDefault(k, val)
	}

	if path != "" {
		m.v.SetConfigFile(path)
		if err := m.v.ReadInConfig(); err != nil {
			var notFound viper.ConfigFileNotFoundError
			// Tolerate a missing file (env vars + defaults suffice); fail
			// on anything else, e.g. malformed YAML. errors.Is against
			// os.ErrNotExist catches the underlying os.PathError even on
			// platforms where its message text differs, which a
			// string-matched check would miss.
			if !errors.As(err, &notFound) && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("config: read %q: %w", path, err)
			}
		}
	}

	cfg, err := unmarshalCore(m.v)
	if err != nil {
		return err
	}
	m.core = cfg

	for key, target := range m.sections {
		if err := m.v.UnmarshalKey(key, target); err != nil {
			return fmt.Errorf("config: unmarshal section %q: %w", key, err)
		}
	}

	validators := append([]Validator{m.core}, o.validators...)
	if err := runValidators(validators); err != nil {
		return err
	}

	if o.watch {
		m.v.OnConfigChange(func(fsnotify.Event) {
			m.reload(o.onChange)
		})
		m.v.WatchConfig()
	}

	return nil
}

// reload re-populates the core Config and every registered section from
// viper's current (freshly re-read) state. It runs on viper's file-watcher
// goroutine, so it deliberately swallows errors instead of returning them:
// a bad edit to the config file just means the reload is skipped and the
// last-known-good configuration stays in effect. onChange fires only when
// the reload — including validation — succeeds.
func (m *Manager) reload(onChange func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, err := unmarshalCore(m.v)
	if err != nil {
		return
	}
	for key, target := range m.sections {
		if err := m.v.UnmarshalKey(key, target); err != nil {
			return
		}
	}
	if err := cfg.Validate(); err != nil {
		return
	}
	m.core = cfg
	if onChange != nil {
		onChange()
	}
}

// Core returns the current core framework configuration. Treat the
// returned value as read-only: on reload, Manager swaps in a new *Config
// rather than mutating the one already handed out, so call Core() again
// after a reload rather than caching the pointer across one.
func (m *Manager) Core() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.core
}

// Register binds target — a pointer to a struct owned by whichever module
// is registering it — to the configuration section at key (e.g. "billing"
// for a top-level billing: block in awo.yaml, or AWO_BILLING_* env vars).
// If Load has already run, target is populated immediately; either way it
// is re-populated on every future Load and on every hot reload, so a
// module can call Register once during init and simply read its struct's
// fields whenever it needs them afterwards.
//
// This is the package's extension mechanism: a new module gains
// configuration support without any change to this package or to Config.
func (m *Manager) Register(key string, target any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sections[key] = target
	if m.core == nil {
		return nil // populated on the next Load
	}
	if err := m.v.UnmarshalKey(key, target); err != nil {
		return fmt.Errorf("config: unmarshal section %q: %w", key, err)
	}
	return nil
}

// Get unmarshals the section at key into out without registering it for
// future reloads — for a one-off read. Prefer Register for anything that
// should stay current if WithWatch is enabled.
func (m *Manager) Get(key string, out any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.core == nil {
		return ErrNotLoaded
	}
	if !m.v.IsSet(key) {
		return fmt.Errorf("%w: %q", ErrSectionNotFound, key)
	}
	return m.v.UnmarshalKey(key, out)
}

// String, Int, Bool, and Duration fetch a single scalar at a dotted key,
// mirroring viper's own accessors so callers never need to reach past the
// Manager to the underlying viper instance.
func (m *Manager) String(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.v.GetString(key)
}

func (m *Manager) Int(key string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.v.GetInt(key)
}

func (m *Manager) Bool(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.v.GetBool(key)
}

func (m *Manager) Duration(key string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.v.GetDuration(key)
}

// unmarshalCore decodes v's current state into a Config and derives any
// generation sub-paths the caller left unset from generation.output_dir,
// so overriding output_dir alone is enough to relocate generated
// artifacts without also having to repeat openapi_output and docs_output.
func unmarshalCore(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal core: %w", err)
	}
	if !v.IsSet("generation.openapi_output") {
		cfg.Generation.OpenAPIOutput = filepath.Join(cfg.Generation.OutputDir, "openapi.json")
	}
	if !v.IsSet("generation.docs_output") {
		cfg.Generation.DocsOutput = filepath.Join(cfg.Generation.OutputDir, "docs") + string(filepath.Separator)
	}
	return &cfg, nil
}

// setCoreDefaults installs defaults for every core field that has a
// sensible one. Fields with no safe default (database.url, redis.url) are
// intentionally left unset so Config.Validate catches their absence.
func setCoreDefaults(v *viper.Viper) {
	v.SetDefault("migration.dir", "./db/migrations")
	v.SetDefault("migration.strategy", "golang-migrate")
	v.SetDefault("generation.output_dir", "./generated")
	v.SetDefault("generation.migrations_output", "./db/migrations/")
	v.SetDefault("app.name", "awo")
	v.SetDefault("app.port", "8080")
	v.SetDefault("platform.iam", true)
	v.SetDefault("platform.tenant", true)
	v.SetDefault("platform.organization", true)
	v.SetDefault("platform.audit", true)
	v.SetDefault("platform.settings", true)
	v.SetDefault("platform.feature_flags", true)
	v.SetDefault("platform.notifications", true)
	v.SetDefault("platform.attachments", true)
	v.SetDefault("platform.mail", true)
	v.SetDefault("platform.metadata", true)
	v.SetDefault("platform.search", false)
}

func runValidators(vs []Validator) error {
	var errs []error
	for _, v := range vs {
		if v == nil {
			continue
		}
		if err := v.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrValidation, errors.Join(errs...))
}
