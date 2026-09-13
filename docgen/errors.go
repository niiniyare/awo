package docgen

import "fmt"

// Error codes for this package, namespaced under "docgen." per the Awo
// AwoError convention (dot-namespaced code, RFC7807-style HTTP problem output).
const (
	// CodeWriteFailed wraps any underlying io.Writer failure encountered
	// while rendering. The original error is preserved as the cause.
	CodeWriteFailed = "docgen.write_failed"
)

// wrapWriteErr wraps an io.Writer error using Awo's standard error type so
// failures surface consistently whether docgen is invoked from a build-time
// CLI or an on-demand HTTP handler.
func wrapWriteErr(cause error) error {
	if cause == nil {
		return nil
	}
	return fmt.Errorf("docgen.write_failed: failed to write documentation output: %w", cause)
}
