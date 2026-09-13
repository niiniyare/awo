package config

// Option customizes Load. Options are applied in the order given, so a
// later option can override an earlier one.
type Option func(*loadOptions)

type loadOptions struct {
	envPrefix  string
	defaults   map[string]any
	validators []Validator
	watch      bool
	onChange   func()
}

// WithEnvPrefix overrides the default "AWO" environment variable prefix.
func WithEnvPrefix(prefix string) Option {
	return func(o *loadOptions) { o.envPrefix = prefix }
}

// WithDefault registers a single default value for a dotted key, e.g.
// "billing.currency". For several at once, use WithDefaults.
func WithDefault(key string, value any) Option {
	return func(o *loadOptions) {
		if o.defaults == nil {
			o.defaults = make(map[string]any)
		}
		o.defaults[key] = value
	}
}

// WithDefaults registers several default values at once.
func WithDefaults(defaults map[string]any) Option {
	return func(o *loadOptions) {
		if o.defaults == nil {
			o.defaults = make(map[string]any, len(defaults))
		}
		for k, v := range defaults {
			o.defaults[k] = v
		}
	}
}

// WithValidator registers an additional Validator to run after Load,
// alongside the core Config's own Validate. Modules that register their
// own configuration sections (see Manager.Register) supply their own
// Validators this way instead of the framework hardcoding module-specific
// checks into Config.Validate.
func WithValidator(v Validator) Option {
	return func(o *loadOptions) { o.validators = append(o.validators, v) }
}

// WithWatch enables hot-reloading: the Manager watches the config file for
// changes and re-unmarshals the core Config and every registered section
// on write. onChange, if non-nil, is invoked after each successful reload
// so callers can react (log it, re-derive dependent state, etc). onChange
// runs on viper's internal file-watcher goroutine, not the goroutine that
// called Load, so it must be safe for concurrent use on its own.
//
// A reload that fails to unmarshal or fails Config.Validate is discarded
// silently — the last-known-good configuration stays in effect, and
// onChange is not called.
func WithWatch(onChange func()) Option {
	return func(o *loadOptions) {
		o.watch = true
		o.onChange = onChange
	}
}
