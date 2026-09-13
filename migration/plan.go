// Package migration provides types and helpers for planning and verifying
// golang-migrate compatible SQL migration files.
package migration

import "fmt"

// Direction indicates whether a migration step runs the up or down script.
type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
)

// Step describes a single migration script (one direction of one version).
type Step struct {
	// Version is the 14-digit Unix timestamp prefix from the filename.
	Version uint64

	// Description is the human-readable slug from the filename,
	// e.g. "create_finance_invoice".
	Description string

	// Direction is up or down.
	Direction Direction

	// SQL is the full content of the migration script.
	SQL string

	// Checksum is the SHA-256 hex digest of SQL. Used to detect tampering.
	Checksum string
}

// Plan is an ordered sequence of Steps that moves the database from one
// version to another.
type Plan struct {
	// Steps are the migration steps to execute, in execution order.
	Steps []Step

	// From is the current database version (0 = fresh, unversioned database).
	From uint64

	// To is the target version after all Steps are applied.
	To uint64
}

// String returns a human-readable summary of the plan.
func (p Plan) String() string {
	return fmt.Sprintf("migration plan: %d → %d (%d steps)", p.From, p.To, len(p.Steps))
}
