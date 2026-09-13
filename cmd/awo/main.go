// Command awo is the Awo framework developer CLI.
//
// Usage:
//
//	awo <command> [flags]
//
// Commands:
//
//	new module <name>      Scaffold a new platform/business module
//	new entity <mod> <name> Scaffold a new entity definition
//	new workflow <name>    Scaffold a Temporal workflow stub
//	schema inspect         Print schema endpoint hint
//	schema fingerprint     Print fingerprint endpoint hint
//	migrate up             Apply pending migrations (delegates to cmd/migrate)
//	migrate down           Roll back last migration
//	migrate version        Print current migration version
//	migrate status         List applied/pending migrations
//	module list            List registered modules
//	docgen                 Generate Markdown API reference
//	doctor                 Check development environment prerequisites
//	version                Print build information
//	validate <file>        Validate an entity definition YAML/JSON file
package main

import (
	"fmt"
	"os"

	"awo.so/awo/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	// Strip global flags from args before command dispatch.
	gf, args := parseGlobalFlags(args)
	_ = gf // available to sub-commands via parseGlobalFlags in cmds_schema.go

	var err error
	switch command {
	case "serve":
		err = runServe(args)
	case "new":
		err = runNew(args)
	case "schema":
		err = runSchemaV2(args)
	case "entity":
		err = runEntityV2(args)
	case "generate":
		err = runGenerateV2(args)
	case "migrate":
		err = runMigrate(args)
	case "module":
		err = runModule(args)
	case "docgen":
		err = runDocgen(args)
	case "doctor":
		err = runDoctor()
	case "version", "--version", "-v":
		printVersion()
	case "validate":
		err = runValidate(args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "awo: unknown command %q\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "awo %s: %v\n", command, err)
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Println(version.Banner("awo"))
}

func printUsage() {
	fmt.Print(`awo — Awo framework developer CLI

Usage:
  awo <command> [flags]

Server:
  serve [--port PORT] [--db URL] [--redis URL] [--log-level info] [--open]
                              Start the Awo HTTP server

Scaffolding:
  new module <name>           Scaffold a new module (definition, hooks, service, migration)
  new entity <module> <name>  Scaffold a new entity definition file
  new workflow <name>         Scaffold a Temporal workflow + activities stub

Schema:
  schema compile              Compile all registered entities and report counts
  schema validate             Validate the compiled schema (exits 1 on error)
  schema graph                Print entity dependency graph in topological order
  schema inspect              Print a human-readable summary of the compiled schema
  schema fingerprint          Print the SHA-256 fingerprint of the compiled schema

Migrations:
  migrate up                  Apply all pending up migrations
  migrate down                Roll back the last applied migration
  migrate version             Print current database migration version
  migrate status              List applied/pending migrations

Modules:
  module list                 List registered modules in dependency order

Documentation:
  docgen [--out <dir>]        Generate Markdown API reference (stdout or directory)

Diagnostics:
  doctor                      Check development environment prerequisites
  validate <file>             Validate an entity definition file (YAML/JSON)

Build:
  version                     Print build version, git commit, and runtime info

Flags:
  --dir <path>                Migration directory (default: db/migration)
  --db  <url>                 PostgreSQL database URL (default: $DATABASE_URL)
  --out <path>                Output file for docgen (default: stdout)

Environment:
  DATABASE_URL                PostgreSQL connection string
  REDIS_URL                   Redis connection string
  TEMPORAL_HOST               Temporal server address
  AWO_MIGRATION_DIR           Override default migration directory

`)
}

func runNew(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: awo new <module|entity|workflow> [args...]")
	}
	switch args[0] {
	case "module":
		return runNewModule(args[1:])
	case "entity":
		return runNewEntity(args[1:])
	case "workflow":
		return runNewWorkflow(args[1:])
	default:
		return fmt.Errorf("unknown new sub-command %q (use: module, entity, workflow)", args[0])
	}
}

func runNewModule(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: awo new module <name>")
	}
	name := args[0]
	files := scaffoldModule(name)
	for path, content := range files {
		if err := writeScaffold(path, content); err != nil {
			return err
		}
	}
	fmt.Printf("scaffolded module %q:\n", name)
	for path := range files {
		fmt.Printf("  %s\n", path)
	}
	return nil
}

func runNewEntity(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: awo new entity <module> <name>")
	}
	module, name := args[0], args[1]
	files := scaffoldEntity(module, name)
	for path, content := range files {
		if err := writeScaffold(path, content); err != nil {
			return err
		}
	}
	fmt.Printf("scaffolded entity %s_%s:\n", module, name)
	for path := range files {
		fmt.Printf("  %s\n", path)
	}
	return nil
}

func runNewWorkflow(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: awo new workflow <name>")
	}
	name := args[0]
	files := scaffoldWorkflow(name)
	for path, content := range files {
		if err := writeScaffold(path, content); err != nil {
			return err
		}
	}
	fmt.Printf("scaffolded workflow %q:\n", name)
	for path := range files {
		fmt.Printf("  %s\n", path)
	}
	return nil
}


func runMigrate(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "up", "down", "version", "status":
		fmt.Printf("migrate %s: run the dedicated migration binary:\n", sub)
		fmt.Printf("  go run ./cmd/migrate %s\n", sub)
		return nil
	default:
		return fmt.Errorf("unknown migrate sub-command %q (use: up, down, version, status)", sub)
	}
}

func runModule(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		fmt.Println("module list: expose GET /api/v1/modules in development and run:")
		fmt.Println("  curl http://localhost:3000/api/v1/modules | jq .")
		return nil
	default:
		return fmt.Errorf("unknown module sub-command %q (use: list)", sub)
	}
}

func runDocgen(args []string) error {
	return runDocgenCmd(args)
}

func runValidate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: awo validate <file>")
	}
	// Validation of entity YAML/JSON definition files is performed by the
	// awo/compiler package at startup. For standalone file validation, callers
	// should write a small Go program that calls registry.BuildFrom + compiler.Validate.
	fmt.Printf("validate %s: static file validation not yet implemented.\n", args[0])
	fmt.Println("  Use 'go run ./cmd/server' to exercise full registry + compiler validation.")
	return nil
}
