package config

// defaultManager backs the package-level convenience functions below.
// Reach for New() instead when a program needs more than one independently
// configured Manager (tests commonly do); the shared default exists purely
// for the common case of a single process with one configuration source.
var defaultManager = New()

// Load loads configuration into the package-level default Manager and
// returns its core Config. This keeps the original config.Load(path)
// signature working unchanged for existing callers — everything else in
// this package is additive.
func Load(path string, opts ...Option) (*Config, error) {
	if err := defaultManager.Load(path, opts...); err != nil {
		return nil, err
	}
	return defaultManager.Core(), nil
}

// Register registers a module configuration section on the default
// Manager. See (*Manager).Register.
func Register(key string, target any) error {
	return defaultManager.Register(key, target)
}

// Get fetches a section from the default Manager. See (*Manager).Get.
func Get(key string, out any) error {
	return defaultManager.Get(key, out)
}
