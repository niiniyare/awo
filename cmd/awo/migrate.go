package main

// runMigrate implements "awo migrate up|down|version|status" by driving the
// same framework migration.Runner (migration/engine.go) that the standalone
// cmd/migrate binary uses — not a re-implementation, and not a shell-out to
// that binary (the previous behavior: this command used to just print
// "go run ./cmd/migrate <sub>" and do nothing itself). This closes the
// biggest gap in the CLI developer journey: "apply migrations to a
// development database" previously required leaving the awo CLI entirely.
//
// The blank imports below register every framework/platform module's
// migrations (plus the examples/expense showcase module's). They MUST be
// kept in sync with cmd/migrate/main.go's own blank-import list — the two
// binaries are expected to apply the identical migration set. A module
// missing from one but not the other would silently diverge on which
// tables exist depending on which binary a developer happened to run.

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"awo.so/awo/migration"

	// Keep in sync with cmd/migrate/main.go's blank-import list.
	_ "awo.so/awo/events/outbox/migrations"
	_ "awo.so/awo/examples/expense/migrations"
	_ "awo.so/awo/migration/bootstrap"
	_ "awo.so/awo/platform/audit/migrations"
	_ "awo.so/awo/platform/iam/migrations"
	_ "awo.so/awo/platform/tenant/migrations"
)

func runMigrate(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	rest := args
	if len(args) > 0 {
		rest = args[1:]
	}

	dbURL := os.Getenv("DATABASE_URL")
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--db" && i+1 < len(rest) {
			dbURL = rest[i+1]
			i++
		}
	}
	if dbURL == "" {
		return fmt.Errorf("migrate: --db or DATABASE_URL is required")
	}

	d, err := iofs.New(migration.FS(), ".")
	if err != nil {
		return fmt.Errorf("migrate: build migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", d, dbURL)
	if err != nil {
		return fmt.Errorf("migrate: connect: %w", err)
	}
	runner := migration.NewRunner(m)
	defer runner.Close()

	switch sub {
	case "up":
		if err := runner.Up(); err != nil {
			return err
		}
		fmt.Println("migrate: up complete")
		return nil

	case "down":
		if err := runner.Steps(-1); err != nil {
			return err
		}
		fmt.Println("migrate: down 1 complete")
		return nil

	case "version":
		v, dirty, err := runner.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("version: none applied yet")
				return nil
			}
			return err
		}
		fmt.Printf("version: %d, dirty: %v\n", v, dirty)
		return nil

	case "status":
		v, dirty, err := runner.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("status: no migrations applied yet")
				return nil
			}
			return err
		}
		fmt.Printf("status: current version %d, dirty: %v\n", v, dirty)
		if dirty {
			fmt.Println("warning: database is in a dirty state — a previous migration failed partway through. Use 'migrate down' or fix manually before 'migrate up'.")
		}
		return nil

	default:
		return fmt.Errorf("unknown migrate sub-command %q (use: up, down, version, status)", sub)
	}
}
