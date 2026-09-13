// Package config loads and validates awo.yaml configuration.
//
// The framework's own settings live on Config (database, redis, platform
// toggles, etc.) and are always available through Manager.Core (or the
// package-level Load). Anything module-specific does not belong on
// Config — it is registered under its own namespaced key via
// Manager.Register (or the package-level Register, which uses a shared
// default Manager) and read back with Manager.Get. This keeps the core
// framework config closed while still letting every module in an
// application add and fetch its own configuration through one consistent
// API, without ever touching this package again.
//
// Configuration is read from the file passed to Load (conventionally
// awo.yaml) with environment variable overrides. Environment variables use
// the prefix AWO_ and replace dots with underscores, e.g. AWO_DATABASE_URL
// overrides database.url. A handful of common unprefixed variables
// (DATABASE_URL, REDIS_URL, PORT, APP_NAME) are also honored, since those
// are the names most hosting platforms inject automatically.
//
// # Adding a new configuration section
//
//	type BillingConfig struct {
//		Currency string `mapstructure:"currency"`
//		TrialDays int   `mapstructure:"trial_days"`
//	}
//
//	var billing BillingConfig
//	if err := config.Register("billing", &billing); err != nil {
//		// handle error
//	}
//	// billing.Currency and billing.TrialDays are populated once Load runs,
//	// and stay current across any hot reload (see WithWatch).
//
// A billing: block in awo.yaml, or AWO_BILLING_CURRENCY /
// AWO_BILLING_TRIAL_DAYS env vars, now feed straight into that struct —
// nothing in this package changed to support it.
package config

import (
	"errors"
	"fmt"
)

// Config holds all core framework configuration.
type Config struct {
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Migration  MigrationConfig  `mapstructure:"migration"`
	Generation GenerationConfig `mapstructure:"generation"`
	Platform   PlatformConfig   `mapstructure:"platform"`
	App        AppConfig        `mapstructure:"app"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL string `mapstructure:"url"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL string `mapstructure:"url"`
}

// MigrationConfig holds migration runner settings.
type MigrationConfig struct {
	// Dir is the directory containing migration SQL files.
	Dir string `mapstructure:"dir"`
	// Strategy is the migration runner. Currently only "golang-migrate" is
	// supported; Config.Validate rejects anything else.
	Strategy string `mapstructure:"strategy"`
}

// GenerationConfig holds code/schema generation output paths. OpenAPIOutput
// and DocsOutput are derived from OutputDir when left unset in both the
// config file and the environment — see unmarshalCore in manager.go — so
// overriding output_dir alone is enough to relocate generated artifacts.
type GenerationConfig struct {
	OutputDir        string `mapstructure:"output_dir"`
	OpenAPIOutput    string `mapstructure:"openapi_output"`
	DocsOutput       string `mapstructure:"docs_output"`
	MigrationsOutput string `mapstructure:"migrations_output"`
}

// PlatformConfig controls which platform modules are enabled.
type PlatformConfig struct {
	IAM           bool `mapstructure:"iam"`
	Tenant        bool `mapstructure:"tenant"`
	Organization  bool `mapstructure:"organization"`
	Audit         bool `mapstructure:"audit"`
	Settings      bool `mapstructure:"settings"`
	FeatureFlags  bool `mapstructure:"feature_flags"`
	Notifications bool `mapstructure:"notifications"`
	Attachments   bool `mapstructure:"attachments"`
	Mail          bool `mapstructure:"mail"`
	Metadata      bool `mapstructure:"metadata"`
	Search        bool `mapstructure:"search"`
}

// AppConfig holds application identity settings.
type AppConfig struct {
	Name string `mapstructure:"name"`
	Port string `mapstructure:"port"`
}

// Validate returns an error when required core fields are absent or
// inconsistent. Manager.Load calls this automatically; call it directly
// only when a Config is constructed by some other means, e.g. in tests.
func (c *Config) Validate() error {
	var errs []error
	if c.Database.URL == "" {
		errs = append(errs, errors.New("database.url is required (set DATABASE_URL or database.url in awo.yaml)"))
	}
	if c.Redis.URL == "" {
		errs = append(errs, errors.New("redis.url is required (set REDIS_URL or redis.url in awo.yaml)"))
	}
	if c.Migration.Strategy != "golang-migrate" {
		errs = append(errs, fmt.Errorf("migration.strategy %q is not supported (want %q)", c.Migration.Strategy, "golang-migrate"))
	}
	return errors.Join(errs...)
}
