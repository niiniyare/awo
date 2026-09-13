package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// ConfigSuite exercises Config.Validate in isolation, with no Manager or
// file I/O involved.
type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) validConfig() Config {
	return Config{
		Database:  DatabaseConfig{URL: "postgres://localhost/awo"},
		Redis:     RedisConfig{URL: "redis://localhost:6379"},
		Migration: MigrationConfig{Dir: "./db/migrations", Strategy: "golang-migrate"},
	}
}

func (s *ConfigSuite) TestValidate_Valid() {
	cfg := s.validConfig()
	s.Require().NoError(cfg.Validate())
}

func (s *ConfigSuite) TestValidate_MissingDatabaseURL() {
	cfg := s.validConfig()
	cfg.Database.URL = ""

	err := cfg.Validate()
	s.Require().Error(err)
	s.ErrorContains(err, "database.url is required")
}

func (s *ConfigSuite) TestValidate_MissingRedisURL() {
	cfg := s.validConfig()
	cfg.Redis.URL = ""

	err := cfg.Validate()
	s.Require().Error(err)
	s.ErrorContains(err, "redis.url is required")
}

func (s *ConfigSuite) TestValidate_UnsupportedMigrationStrategy() {
	cfg := s.validConfig()
	cfg.Migration.Strategy = "flyway"

	err := cfg.Validate()
	s.Require().Error(err)
	s.ErrorContains(err, `migration.strategy "flyway" is not supported`)
}

func (s *ConfigSuite) TestValidate_JoinsAllFailures() {
	cfg := Config{Migration: MigrationConfig{Strategy: "flyway"}}

	err := cfg.Validate()
	s.Require().Error(err)
	s.ErrorContains(err, "database.url is required")
	s.ErrorContains(err, "redis.url is required")
	s.ErrorContains(err, `migration.strategy "flyway" is not supported`)
}

// ManagerSuite exercises Manager.Load against real temp files and env vars.
// SetupTest/TearDownTest give every test a clean working directory and a
// scrubbed environment, since env var bleed between tests is the single
// most common cause of flaky config tests.
type ManagerSuite struct {
	suite.Suite

	dir     string
	origEnv map[string]string
	envKeys []string
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}

func (s *ManagerSuite) SetupTest() {
	s.dir = s.T().TempDir()
	s.envKeys = []string{
		"AWO_DATABASE_URL", "AWO_REDIS_URL", "AWO_APP_NAME", "AWO_APP_PORT",
		"AWO_MIGRATION_STRATEGY", "AWO_PLATFORM_SEARCH",
		"DATABASE_URL", "REDIS_URL", "PORT", "APP_NAME",
	}
	s.origEnv = make(map[string]string, len(s.envKeys))
	for _, k := range s.envKeys {
		if v, ok := os.LookupEnv(k); ok {
			s.origEnv[k] = v
		}
		s.Require().NoError(os.Unsetenv(k))
	}
}

func (s *ManagerSuite) TearDownTest() {
	for _, k := range s.envKeys {
		s.Require().NoError(os.Unsetenv(k))
	}
	for k, v := range s.origEnv {
		s.Require().NoError(os.Setenv(k, v))
	}
}

func (s *ManagerSuite) writeConfigFile(name, contents string) string {
	path := filepath.Join(s.dir, name)
	s.Require().NoError(os.WriteFile(path, []byte(contents), 0o644))
	return path
}

func (s *ManagerSuite) TestLoad_MissingFileTolerated() {
	m := New()
	s.T().Setenv("AWO_DATABASE_URL", "postgres://localhost/awo")
	s.T().Setenv("AWO_REDIS_URL", "redis://localhost:6379")

	err := m.Load(filepath.Join(s.dir, "does-not-exist.yaml"))
	s.Require().NoError(err)

	core := m.Core()
	s.Require().NotNil(core)
	s.Equal("postgres://localhost/awo", core.Database.URL)
}

func (s *ManagerSuite) TestLoad_MalformedFileFails() {
	path := s.writeConfigFile("awo.yaml", "database:\n  url: [unterminated\n")

	m := New()
	err := m.Load(path)
	s.Require().Error(err)
	s.ErrorContains(err, "config: read")
}

func (s *ManagerSuite) TestLoad_ValidationFailureSurfaced() {
	// No database/redis URL anywhere -> Validate must fail and Load must
	// propagate it wrapped in ErrValidation.
	m := New()
	err := m.Load("")
	s.Require().Error(err)
	s.ErrorIs(err, ErrValidation)
}

func (s *ManagerSuite) TestLoad_DefaultsApplied() {
	m := New()
	s.T().Setenv("AWO_DATABASE_URL", "postgres://localhost/awo")
	s.T().Setenv("AWO_REDIS_URL", "redis://localhost:6379")

	s.Require().NoError(m.Load(""))

	core := m.Core()
	s.Equal("golang-migrate", core.Migration.Strategy)
	s.Equal("./db/migrations", core.Migration.Dir)
	s.Equal("awo", core.App.Name)
	s.Equal("8080", core.App.Port)
	s.True(core.Platform.IAM)
	s.False(core.Platform.Search)
}

func (s *ManagerSuite) TestLoad_FileValuesOverrideDefaults() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
app:
  name: from-file
  port: "9090"
platform:
  search: true
`)

	m := New()
	s.Require().NoError(m.Load(path))

	core := m.Core()
	s.Equal("postgres://file/awo", core.Database.URL)
	s.Equal("from-file", core.App.Name)
	s.Equal("9090", core.App.Port)
	s.True(core.Platform.Search)
}

func (s *ManagerSuite) TestLoad_EnvOverridesFile() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
app:
  name: from-file
`)
	s.T().Setenv("AWO_APP_NAME", "from-env")

	m := New()
	s.Require().NoError(m.Load(path))

	s.Equal("from-env", m.Core().App.Name)
}

func (s *ManagerSuite) TestLoad_UnprefixedCommonEnvVarsHonored() {
	s.T().Setenv("DATABASE_URL", "postgres://common/awo")
	s.T().Setenv("REDIS_URL", "redis://common:6379")
	s.T().Setenv("PORT", "3000")

	m := New()
	s.Require().NoError(m.Load(""))

	core := m.Core()
	s.Equal("postgres://common/awo", core.Database.URL)
	s.Equal("redis://common:6379", core.Redis.URL)
	s.Equal("3000", core.App.Port)
}

func (s *ManagerSuite) TestLoad_GenerationPathsDeriveFromOutputDir() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
generation:
  output_dir: ./build
`)

	m := New()
	s.Require().NoError(m.Load(path))

	core := m.Core()
	s.Equal(filepath.Join("build", "openapi.json"), filepath.Clean(core.Generation.OpenAPIOutput))
	s.Contains(core.Generation.DocsOutput, filepath.Join("build", "docs"))
}

func (s *ManagerSuite) TestLoad_ExplicitGenerationPathsNotOverridden() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
generation:
  output_dir: ./build
  openapi_output: ./custom/spec.json
`)

	m := New()
	s.Require().NoError(m.Load(path))

	s.Equal("./custom/spec.json", m.Core().Generation.OpenAPIOutput)
}

func (s *ManagerSuite) TestLoad_WithDefaultsOption() {
	m := New()
	s.T().Setenv("AWO_DATABASE_URL", "postgres://localhost/awo")
	s.T().Setenv("AWO_REDIS_URL", "redis://localhost:6379")

	err := m.Load("", WithDefaults(map[string]any{
		"billing.currency": "USD",
	}))
	s.Require().NoError(err)
	s.Equal("USD", m.String("billing.currency"))
}

func (s *ManagerSuite) TestLoad_WithValidatorRuns() {
	m := New()
	s.T().Setenv("AWO_DATABASE_URL", "postgres://localhost/awo")
	s.T().Setenv("AWO_REDIS_URL", "redis://localhost:6379")

	called := false
	failing := validatorFunc(func() error {
		called = true
		return assertionError("module validator failed")
	})

	err := m.Load("", WithValidator(failing))
	s.Require().Error(err)
	s.True(called)
	s.ErrorIs(err, ErrValidation)
}

func (s *ManagerSuite) TestLoad_WithEnvPrefixOverride() {
	s.T().Setenv("MYAPP_DATABASE_URL", "postgres://prefixed/awo")
	s.T().Setenv("MYAPP_REDIS_URL", "redis://prefixed:6379")

	m := New()
	err := m.Load("", WithEnvPrefix("MYAPP"))
	s.Require().NoError(err)
	s.Equal("postgres://prefixed/awo", m.Core().Database.URL)
}

// --- Register / Get -------------------------------------------------------

type billingConfig struct {
	Currency  string `mapstructure:"currency"`
	TrialDays int    `mapstructure:"trial_days"`
}

func (s *ManagerSuite) TestRegister_BeforeLoad_PopulatedByLoad() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
billing:
  currency: EUR
  trial_days: 14
`)

	m := New()
	var billing billingConfig
	s.Require().NoError(m.Register("billing", &billing))
	s.Require().NoError(m.Load(path))

	s.Equal("EUR", billing.Currency)
	s.Equal(14, billing.TrialDays)
}

func (s *ManagerSuite) TestRegister_AfterLoad_PopulatedImmediately() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
billing:
  currency: GBP
  trial_days: 7
`)

	m := New()
	s.Require().NoError(m.Load(path))

	var billing billingConfig
	s.Require().NoError(m.Register("billing", &billing))
	s.Equal("GBP", billing.Currency)
	s.Equal(7, billing.TrialDays)
}

func (s *ManagerSuite) TestGet_SectionNotFound() {
	m := New()
	s.T().Setenv("AWO_DATABASE_URL", "postgres://localhost/awo")
	s.T().Setenv("AWO_REDIS_URL", "redis://localhost:6379")
	s.Require().NoError(m.Load(""))

	var out billingConfig
	err := m.Get("nonexistent", &out)
	s.Require().Error(err)
	s.ErrorIs(err, ErrSectionNotFound)
}

func (s *ManagerSuite) TestGet_BeforeLoad_ErrNotLoaded() {
	m := New()
	var out billingConfig
	err := m.Get("billing", &out)
	s.Require().Error(err)
	s.ErrorIs(err, ErrNotLoaded)
}

func (s *ManagerSuite) TestGet_ReturnsCurrentSection() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
billing:
  currency: KES
  trial_days: 30
`)

	m := New()
	s.Require().NoError(m.Load(path))

	var billing billingConfig
	s.Require().NoError(m.Get("billing", &billing))
	s.Equal("KES", billing.Currency)
	s.Equal(30, billing.TrialDays)
}

// --- Scalar accessors -------------------------------------------------------

func (s *ManagerSuite) TestScalarAccessors() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
app:
  name: scalar-test
  port: "9999"
platform:
  search: true
timeout: 5s
`)

	m := New()
	s.Require().NoError(m.Load(path))

	s.Equal("scalar-test", m.String("app.name"))
	s.Equal(9999, m.Int("app.port"))
	s.True(m.Bool("platform.search"))
	s.Equal(5*time.Second, m.Duration("timeout"))
}

// --- Hot reload -------------------------------------------------------------

func (s *ManagerSuite) TestWatch_ReloadsOnFileChange() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
app:
  name: original
`)

	m := New()
	reloaded := make(chan struct{}, 1)
	err := m.Load(path, WithWatch(func() {
		select {
		case reloaded <- struct{}{}:
		default:
		}
	}))
	s.Require().NoError(err)
	s.Equal("original", m.Core().App.Name)

	updated := "database:\n  url: postgres://file/awo\nredis:\n  url: redis://file:6379\napp:\n  name: updated\n"
	s.Require().NoError(os.WriteFile(path, []byte(updated), 0o644))

	select {
	case <-reloaded:
	case <-time.After(5 * time.Second):
		s.FailNow("timed out waiting for config reload")
	}

	s.Equal("updated", m.Core().App.Name)
}

func (s *ManagerSuite) TestWatch_BadReloadKeepsLastKnownGood() {
	path := s.writeConfigFile("awo.yaml", `
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
app:
  name: good
`)

	m := New()
	reloaded := make(chan struct{}, 1)
	err := m.Load(path, WithWatch(func() {
		select {
		case reloaded <- struct{}{}:
		default:
		}
	}))
	s.Require().NoError(err)

	// Remove the required redis.url -> Validate must reject the reload.
	broken := "database:\n  url: postgres://file/awo\napp:\n  name: bad\n"
	s.Require().NoError(os.WriteFile(path, []byte(broken), 0o644))

	select {
	case <-reloaded:
		s.FailNow("onChange must not fire for a reload that fails validation")
	case <-time.After(1 * time.Second):
		// expected: no reload event
	}

	s.Equal("good", m.Core().App.Name)
}

// --- Package-level default Manager wrappers ---------------------------------

type DefaultManagerSuite struct {
	suite.Suite
	dir string
}

func TestDefaultManagerSuite(t *testing.T) {
	suite.Run(t, new(DefaultManagerSuite))
}

func (s *DefaultManagerSuite) SetupTest() {
	s.dir = s.T().TempDir()
	defaultManager = New() // isolate from any other test's package-level state
}

func (s *DefaultManagerSuite) TestLoad_RegisterGet_RoundTrip() {
	path := filepath.Join(s.dir, "awo.yaml")
	s.Require().NoError(os.WriteFile(path, []byte(`
database:
  url: postgres://file/awo
redis:
  url: redis://file:6379
billing:
  currency: USD
  trial_days: 21
`), 0o644))

	var billing billingConfig
	s.Require().NoError(Register("billing", &billing))

	cfg, err := Load(path)
	s.Require().NoError(err)
	s.Require().NotNil(cfg)
	s.Equal("postgres://file/awo", cfg.Database.URL)
	s.Equal("USD", billing.Currency)

	var again billingConfig
	s.Require().NoError(Get("billing", &again))
	s.Equal(21, again.TrialDays)
}

// --- test helpers -------------------------------------------------------

type validatorFunc func() error

func (f validatorFunc) Validate() error { return f() }

type assertionError string

func (e assertionError) Error() string { return string(e) }
