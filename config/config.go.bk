// Package config loads and validates awo.yaml configuration.
// Configuration is read from the file specified by --config (default: awo.yaml)
// with environment variable overrides. Environment variables use the prefix AWO_
// and replace dots with underscores (e.g. AWO_DATABASE_URL overrides database.url).
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all framework configuration loaded from awo.yaml.
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
	// Strategy is the migration runner (currently only "golang-migrate").
	Strategy string `mapstructure:"strategy"`
}

// GenerationConfig holds code/schema generation output paths.
type GenerationConfig struct {
	// OutputDir is the base output directory.
	OutputDir string `mapstructure:"output_dir"`
	// OpenAPIOutput is the path for the generated OpenAPI spec.
	OpenAPIOutput string `mapstructure:"openapi_output"`
	// DocsOutput is the directory for generated Markdown docs.
	DocsOutput string `mapstructure:"docs_output"`
	// MigrationsOutput is the directory for generated SQL migration files.
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

// Load reads configuration from the given file path. Environment variables
// with the AWO_ prefix override file values. Returns defaults if the file
// does not exist (allows the CLI to work without a config file).
func Load(path string) (*Config, error) {
	v := viper.New()

	// Environment variable overrides.
	v.SetEnvPrefix("AWO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind common env vars that don't have the AWO_ prefix.
	_ = v.BindEnv("database.url", "DATABASE_URL")
	_ = v.BindEnv("redis.url", "REDIS_URL")
	_ = v.BindEnv("app.port", "PORT")
	_ = v.BindEnv("app.name", "APP_NAME")

	// Defaults.
	v.SetDefault("migration.dir", "./db/migrations")
	v.SetDefault("migration.strategy", "golang-migrate")
	v.SetDefault("generation.output_dir", "./generated")
	v.SetDefault("generation.openapi_output", "./generated/openapi.json")
	v.SetDefault("generation.docs_output", "./generated/docs/")
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

	// File config (optional).
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			// Tolerate missing file — env vars and defaults suffice.
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				// Only fail for malformed files, not missing files.
				if !strings.Contains(err.Error(), "no such file") &&
					!strings.Contains(err.Error(), "not found") {
					return nil, fmt.Errorf("config: read %q: %w", path, err)
				}
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	return &cfg, nil
}

// Validate returns an error when required fields are absent.
// Only fields required for server start are checked here.
func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("config: database.url is required (set DATABASE_URL or database.url in awo.yaml)")
	}
	if c.Redis.URL == "" {
		return fmt.Errorf("config: redis.url is required (set REDIS_URL or redis.url in awo.yaml)")
	}
	return nil
}
