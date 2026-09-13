package config

import "errors"

var (
	// ErrNotLoaded is returned by Manager.Get (and the package-level Get)
	// when called before Load has completed successfully.
	ErrNotLoaded = errors.New("config: not loaded")

	// ErrSectionNotFound is returned by Manager.Get when the requested key
	// has no data anywhere in the loaded configuration — no matching block
	// in the config file, no matching env vars, and no default registered
	// for it.
	ErrSectionNotFound = errors.New("config: section not found")

	// ErrValidation wraps the combined errors from Config.Validate and any
	// Validators supplied via WithValidator. Use errors.Is(err,
	// ErrValidation) to detect it, and errors.Unwrap / errors.Join
	// semantics to inspect the individual failures.
	ErrValidation = errors.New("config: validation failed")
)
