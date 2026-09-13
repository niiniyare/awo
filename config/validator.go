package config

// Validator is implemented by any configuration — the core Config or a
// module-registered section — that needs to check itself after loading.
// Supply one for a registered section with WithValidator; Config already
// implements it via Config.Validate.
type Validator interface {
	Validate() error
}
