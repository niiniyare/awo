package audit

import "fmt"

// errorf constructs a formatted error for use within the audit package.
// Using a package-local helper keeps error messages consistent and avoids
// importing a third-party errors package in the audit foundation layer.
func errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
