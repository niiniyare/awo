// Command migrate runs database migrations for the AWO framework.
//
// Two modes are supported:
//
//	-mode=embedded (default)
//	    Uses all migrations registered by framework module init() functions.
//	    Import side effects from the blank imports below drive registration.
//	    This is the standard production mode — no separate SQL directory needed.
//
//	-mode=file
//	    Reads SQL files from -dir (default: ./db/migration).
//	    Preserved for local development and emergency manual overrides.
//
// Usage:
//
//	migrate [-mode=embedded|file] [-dir ./db/migration] -db $DATABASE_URL up
//	migrate [-mode=embedded|file] [-dir ./db/migration] -db $DATABASE_URL down [N]
//	migrate [-mode=embedded|file] [-dir ./db/migration] -db $DATABASE_URL version
//	migrate [-mode=embedded|file] [-dir ./db/migration] -db $DATABASE_URL force <version>
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"awo.so/awo/migration"

	// Blank imports register all framework module migrations.
	// Add new framework modules here as they are created.
	_ "awo.so/awo/migration/bootstrap"
	_ "awo.so/awo/platform/audit/migrations"
	_ "awo.so/awo/platform/iam/migrations"
	_ "awo.so/awo/platform/tenant/migrations"
)

func main() {
	mode := flag.String("mode", "embedded", "migration source: embedded | file")
	dir := flag.String("dir", "./db/migration", "SQL files directory (file mode only)")
	db := flag.String("db", os.Getenv("DATABASE_URL"), "database connection URL")
	flag.Parse()

	if *db == "" {
		log.Fatal("migrate: -db or DATABASE_URL is required")
	}

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("migrate: command required: up | down [N] | version | force <version>")
	}

	m, err := newMigrate(*mode, *db, *dir)
	if err != nil {
		log.Fatalf("migrate: init: %v", err)
	}
	defer func() { src, db := m.Close(); _ = src; _ = db }()

	cmd := args[0]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("migrate: up complete")

	case "down":
		n := 1
		if len(args) > 1 {
			v, err := strconv.Atoi(args[1])
			if err != nil {
				log.Fatalf("migrate: invalid N for down: %v", err)
			}
			n = v
		}
		if err := m.Steps(-n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Printf("migrate: down %d complete\n", n)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		fmt.Printf("version: %d, dirty: %v\n", version, dirty)

	case "force":
		if len(args) < 2 {
			log.Fatal("migrate force: version required")
		}
		v, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("migrate force: invalid version: %v", err)
		}
		if err := m.Force(v); err != nil {
			log.Fatalf("migrate force: %v", err)
		}
		fmt.Printf("migrate: forced to version %d\n", v)

	default:
		log.Fatalf("migrate: unknown command %q", cmd)
	}
}

func newMigrate(mode, dbURL, dir string) (*migrate.Migrate, error) {
	switch mode {
	case "embedded":
		vfs := migration.BuildFS()
		d, err := iofs.New(vfs, ".")
		if err != nil {
			return nil, fmt.Errorf("build iofs source: %w", err)
		}
		return migrate.NewWithSourceInstance("iofs", d, dbURL)
	case "file":
		return migrate.New("file://"+dir, dbURL)
	default:
		return nil, fmt.Errorf("unknown mode %q; use embedded or file", mode)
	}
}
