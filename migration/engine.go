package migration

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
)

// Runner executes framework migrations against a PostgreSQL database.
// It wraps golang-migrate and uses the registered module sources as the
// migration source, eliminating manual SQL file management.
//
// Callers must supply a SourceInstance created from [BuildFS] via the iofs
// driver — see [awo/cmd/migrate] for the wiring example.
type Runner struct {
	m *migrate.Migrate
}

// NewRunner creates a Runner from a pre-initialised golang-migrate instance.
// Use [NewRunnerFromFS] for the common embedded-source case.
func NewRunner(m *migrate.Migrate) *Runner {
	return &Runner{m: m}
}

// Up applies all pending migrations in dependency order.
// Returns nil if no migrations are pending (ErrNoChange is suppressed).
func (r *Runner) Up() error {
	if err := r.m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	v, _, _ := r.m.Version()
	slog.Info("migrations applied", "version", v)
	return nil
}

// Steps applies n migrations (positive = up, negative = down).
func (r *Runner) Steps(n int) error {
	if err := r.m.Steps(n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate steps(%d): %w", n, err)
	}
	return nil
}

// Version returns the current applied migration version and dirty state.
func (r *Runner) Version() (uint, bool, error) {
	return r.m.Version()
}

// Force sets the migration version without running any SQL.
// Use to recover from a dirty state after manual intervention.
func (r *Runner) Force(version int) error {
	return r.m.Force(version)
}

// Close releases database and source connections held by the runner.
func (r *Runner) Close() {
	src, db := r.m.Close()
	if src != nil {
		slog.Debug("migration source close error", "err", src)
	}
	if db != nil {
		slog.Debug("migration db close error", "err", db)
	}
}

// FS returns the merged virtual fs.FS built from all registered module sources.
// Callers (e.g. cmd/migrate) pass this to the iofs source driver.
func FS() fs.FS {
	return BuildFS()
}
