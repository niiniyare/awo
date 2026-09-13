package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/docgen"
	"awo.so/awo/generator"
	"awo.so/awo/generator/openapi"
	"awo.so/awo/registry"

	// Platform entity init() registration side effects.
	_ "awo.so/awo/platform/attachment"
	_ "awo.so/awo/platform/audit"
	_ "awo.so/awo/platform/flags"
	_ "awo.so/awo/platform/iam"
	_ "awo.so/awo/platform/mail"
	_ "awo.so/awo/platform/metadata"
	_ "awo.so/awo/platform/notification"
	_ "awo.so/awo/platform/organization"
	_ "awo.so/awo/platform/registry"
	_ "awo.so/awo/platform/settings"
	_ "awo.so/awo/platform/tenant"

	// Finance module entity registration.
	_ "awo.so/modules/finance"
)

// globalFlags are parsed from os.Args before the sub-command is dispatched.
type globalFlags struct {
	JSON   bool
	DryRun bool
	Quiet  bool
}

// parseGlobalFlags scans args for --json, --dry-run, --quiet.
// It returns the remaining args (with the flags stripped).
func parseGlobalFlags(args []string) (globalFlags, []string) {
	var gf globalFlags
	var rest []string
	for _, a := range args {
		switch a {
		case "--json":
			gf.JSON = true
		case "--dry-run":
			gf.DryRun = true
		case "--quiet", "-q":
			gf.Quiet = true
		default:
			rest = append(rest, a)
		}
	}
	return gf, rest
}

// compileRegisteredSchema builds and compiles the global entity registry.
// All platform entity init() calls have already run via package imports above.
func compileRegisteredSchema() (*compiler.CompiledSchema, error) {
	reg := registry.Build()
	audit.Seal()
	return compiler.Compile(reg)
}

// printJSONValue encodes v as indented JSON to stdout.
func printJSONValue(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// ── schema subcommands ────────────────────────────────────────────────────────

// runSchemaV2 dispatches schema subcommands with global flag support.
// Replaces the stub runSchema in main.go when called from the updated switch.
func runSchemaV2(args []string) error {
	gf, rest := parseGlobalFlags(args)
	sub := ""
	if len(rest) > 0 {
		sub = rest[0]
	}
	switch sub {
	case "compile":
		return schemaCompile(gf)
	case "validate":
		return schemaValidate(gf)
	case "graph":
		return schemaGraph(gf)
	case "inspect":
		return schemaInspect(gf)
	case "fingerprint":
		return schemaFingerprint(gf)
	default:
		return fmt.Errorf("unknown schema sub-command %q (use: compile, validate, graph, inspect, fingerprint)", sub)
	}
}

func schemaCompile(gf globalFlags) error {
	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}
	if gf.JSON {
		return printJSONValue(map[string]any{
			"entities": len(schema.Entities),
			"routes":   len(schema.Routes),
			"grants":   len(schema.CapabilityGrants),
			"warnings": len(schema.Diagnostics),
		})
	}
	if !gf.Quiet {
		fmt.Printf("compiled %d entities, %d routes, %d capability grants\n",
			len(schema.Entities), len(schema.Routes), len(schema.CapabilityGrants))
		for _, d := range schema.Diagnostics {
			fmt.Printf("  [%s] %s\n", d.Severity, d.Message)
		}
	}
	return nil
}

func schemaValidate(gf globalFlags) error {
	_, err := compileRegisteredSchema()
	if err != nil {
		if gf.JSON {
			_ = printJSONValue(map[string]any{"valid": false, "error": err.Error()})
		} else {
			fmt.Fprintln(os.Stderr, "schema invalid:", err)
		}
		os.Exit(1)
	}
	if gf.JSON {
		return printJSONValue(map[string]any{"valid": true})
	}
	if !gf.Quiet {
		fmt.Println("schema valid")
	}
	return nil
}

func schemaGraph(gf globalFlags) error {
	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}
	if schema.Graph == nil {
		return fmt.Errorf("dependency graph not available")
	}
	order := schema.Graph.TopologicalOrder
	// Reconstruct edge list using the public Deps() method.
	type graphEdge struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	var edges []graphEdge
	for _, name := range order {
		for _, dep := range schema.Graph.Deps(name) {
			edges = append(edges, graphEdge{From: name, To: dep})
		}
	}
	if gf.JSON {
		return printJSONValue(map[string]any{
			"topological_order": order,
			"edges":             edges,
		})
	}
	if !gf.Quiet {
		fmt.Println("Topological order (safe migration order):")
		for i, name := range order {
			fmt.Printf("  %d. %s\n", i+1, name)
		}
		if len(edges) > 0 {
			fmt.Printf("\nDependencies (%d edges):\n", len(edges))
			for _, e := range edges {
				fmt.Printf("  %s → %s\n", e.From, e.To)
			}
		}
	}
	return nil
}

func schemaInspect(gf globalFlags) error {
	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}
	if gf.JSON {
		items := make([]map[string]any, 0, len(schema.Entities))
		for _, es := range schema.Entities {
			items = append(items, map[string]any{
				"name":         es.QualifiedName,
				"module":       es.Module,
				"field_count":  len(es.Fields),
				"action_count": len(es.Actions),
			})
		}
		graphSummary := "not available"
		if schema.Graph != nil {
			graphSummary = fmt.Sprintf("%d entities, %d edges", len(schema.Graph.TopologicalOrder), len(schema.Entities))
		}
		return printJSONValue(map[string]any{
			"entity_count": len(schema.Entities),
			"route_count":  len(schema.Routes),
			"entities":     items,
			"graph":        graphSummary,
		})
	}
	if !gf.Quiet {
		fmt.Printf("Schema summary: %d entities, %d routes\n\n", len(schema.Entities), len(schema.Routes))
		fmt.Printf("%-40s %-8s %-8s\n", "ENTITY", "FIELDS", "ACTIONS")
		fmt.Println(strings.Repeat("-", 60))
		for _, es := range schema.Entities {
			fmt.Printf("%-40s %-8d %-8d\n", es.QualifiedName, len(es.Fields), len(es.Actions))
		}
		if schema.Graph != nil {
			fmt.Printf("\nDependency graph: %d entities in topological order\n", len(schema.Graph.TopologicalOrder))
		} else {
			fmt.Println("\nDependency graph: not available")
		}
	}
	return nil
}

func schemaFingerprint(gf globalFlags) error {
	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}
	fp := compiler.Fingerprint(schema)
	if gf.JSON {
		return printJSONValue(map[string]any{"fingerprint": fp})
	}
	if !gf.Quiet {
		fmt.Println(fp)
	}
	return nil
}

// ── entity subcommands ────────────────────────────────────────────────────────

// runEntityV2 dispatches entity subcommands.
func runEntityV2(args []string) error {
	gf, rest := parseGlobalFlags(args)
	sub := ""
	if len(rest) > 0 {
		sub = rest[0]
	}
	switch sub {
	case "list":
		return entityList(gf)
	case "inspect":
		name := ""
		if len(rest) > 1 {
			name = rest[1]
		}
		return entityInspect(gf, name)
	default:
		return fmt.Errorf("unknown entity sub-command %q (use: list, inspect <name>)", sub)
	}
}

func entityList(gf globalFlags) error {
	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}
	if gf.JSON {
		items := make([]map[string]any, 0, len(schema.Entities))
		for _, es := range schema.Entities {
			items = append(items, map[string]any{
				"name":        es.QualifiedName,
				"module":      es.Module,
				"label":       es.Label,
				"is_system":   es.IsSystem,
				"field_count": len(es.Fields),
				"table":       es.TableName,
			})
		}
		return printJSONValue(items)
	}
	if !gf.Quiet {
		fmt.Printf("%-40s %-12s %-6s %s\n", "NAME", "MODULE", "TYPE", "LABEL")
		fmt.Println(strings.Repeat("-", 80))
		for _, es := range schema.Entities {
			kind := "custom"
			if es.IsSystem {
				kind = "system"
			}
			fmt.Printf("%-40s %-12s %-6s %s\n", es.QualifiedName, es.Module, kind, es.Label)
		}
	}
	return nil
}

func entityInspect(gf globalFlags, name string) error {
	if name == "" {
		return fmt.Errorf("usage: awo entity inspect <entity-name>")
	}
	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}
	es, ok := schema.ByName[name]
	if !ok {
		return fmt.Errorf("entity %q not found", name)
	}
	if gf.JSON {
		fields := make([]map[string]any, 0, len(es.Fields))
		for _, f := range es.Fields {
			fields = append(fields, map[string]any{
				"name":       f.Name,
				"type":       string(f.Type),
				"required":   f.Required,
				"unique":     f.Unique,
				"immutable":  f.Immutable,
				"sensitive":  f.Sensitive,
				"searchable": f.Searchable,
			})
		}
		return printJSONValue(map[string]any{
			"name":         es.QualifiedName,
			"module":       es.Module,
			"label":        es.Label,
			"is_system":    es.IsSystem,
			"table":        es.TableName,
			"allow_audit":  es.AllowAudit,
			"scope":        string(es.Scope),
			"fields":       fields,
			"route_prefix": es.RoutePrefix,
		})
	}
	if !gf.Quiet {
		fmt.Printf("Entity:      %s\n", es.QualifiedName)
		fmt.Printf("Module:      %s\n", es.Module)
		fmt.Printf("Label:       %s\n", es.Label)
		fmt.Printf("Table:       %s\n", es.TableName)
		fmt.Printf("Type:        %s\n", map[bool]string{true: "system", false: "custom"}[es.IsSystem])
		fmt.Printf("Scope:       %s\n", es.Scope)
		fmt.Printf("AllowAudit:  %v\n", es.AllowAudit)
		fmt.Printf("Routes:      %s\n", es.RoutePrefix)
		fmt.Printf("\nFields (%d):\n", len(es.Fields))
		fmt.Printf("  %-30s %-14s %s\n", "NAME", "TYPE", "FLAGS")
		fmt.Println("  " + strings.Repeat("-", 60))
		for _, f := range es.Fields {
			fmt.Printf("  %-30s %-14s %s\n", f.Name, string(f.Type), fmtFieldFlags(f))
		}
		if len(es.Actions) > 0 {
			fmt.Printf("\nActions (%d):\n", len(es.Actions))
			for _, a := range es.Actions {
				fmt.Printf("  %s %s/%s\n", a.Method, es.RoutePrefix, a.Name)
			}
		}
	}
	return nil
}

// fmtFieldFlags returns a compact flag string for a field.
func fmtFieldFlags(f def.FieldDef) string {
	var flags []string
	if f.Required {
		flags = append(flags, "required")
	}
	if f.Unique {
		flags = append(flags, "unique")
	}
	if f.Immutable {
		flags = append(flags, "immutable")
	}
	if f.Sensitive {
		flags = append(flags, "sensitive")
	}
	if f.Searchable {
		flags = append(flags, "searchable")
	}
	return strings.Join(flags, " ")
}

// ── generate subcommands ──────────────────────────────────────────────────────

// runGenerateV2 dispatches generate subcommands.
func runGenerateV2(args []string) error {
	gf, rest := parseGlobalFlags(args)
	sub := ""
	if len(rest) > 0 {
		sub = rest[0]
	}
	switch sub {
	case "migrations":
		return generateMigrations(gf, rest[1:])
	case "docs":
		return generateDocs(gf, rest[1:])
	case "openapi":
		return generateOpenAPI(gf, rest[1:])
	default:
		return fmt.Errorf("unknown generate sub-command %q (use: migrations, docs, openapi)", sub)
	}
}

func generateMigrations(gf globalFlags, args []string) error {
	// Parse --output and --start-seq flags.
	outDir := "./db/migrations"
	startSeq := 1
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output":
			if i+1 < len(args) {
				outDir = args[i+1]
				i++
			}
		case "--start-seq":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &startSeq)
				i++
			}
		}
	}

	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}

	plan, err := generator.Generate(schema, generator.Options{
		MigrationDir: outDir,
		StartSeq:     startSeq,
	})
	if err != nil {
		return fmt.Errorf("generate migrations: %w", err)
	}

	if gf.JSON {
		items := make([]map[string]any, 0, len(plan.Files))
		for _, f := range plan.Files {
			items = append(items, map[string]any{
				"file": f.Name + ".sql",
				"path": outDir + "/" + f.Name + ".sql",
			})
		}
		return printJSONValue(map[string]any{
			"dry_run": gf.DryRun,
			"output":  outDir,
			"files":   items,
		})
	}

	if gf.DryRun {
		fmt.Printf("dry-run: would write %d migration file(s) to %s\n", len(plan.Files), outDir)
		for _, f := range plan.Files {
			fmt.Printf("  %s.sql\n", f.Name)
		}
		return nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %q: %w", outDir, err)
	}
	for _, f := range plan.Files {
		path := outDir + "/" + f.Name + ".sql"
		if err := os.WriteFile(path, []byte(f.SQL), 0o644); err != nil {
			return fmt.Errorf("write %q: %w", path, err)
		}
		if !gf.Quiet {
			fmt.Printf("  wrote %s\n", path)
		}
	}
	if !gf.Quiet {
		fmt.Printf("generated %d migration file(s) in %s\n", len(plan.Files), outDir)
	}
	return nil
}

func generateDocs(gf globalFlags, args []string) error {
	// Parse --output flag.
	outDir := "./docs/entities"
	for i := 0; i < len(args); i++ {
		if args[i] == "--output" && i+1 < len(args) {
			outDir = args[i+1]
			i++
		}
	}

	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}

	plan, err := docgen.Generate(schema)
	if err != nil {
		return fmt.Errorf("generate docs: %w", err)
	}

	if gf.JSON {
		items := make([]map[string]any, 0, len(plan.Files))
		for _, f := range plan.Files {
			items = append(items, map[string]any{
				"file": f.Name + ".md",
				"path": outDir + "/" + f.Name + ".md",
			})
		}
		return printJSONValue(map[string]any{
			"dry_run": gf.DryRun,
			"output":  outDir,
			"files":   items,
		})
	}

	if gf.DryRun {
		fmt.Printf("dry-run: would write %d doc file(s) to %s\n", len(plan.Files), outDir)
		for _, f := range plan.Files {
			fmt.Printf("  %s.md\n", f.Name)
		}
		return nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %q: %w", outDir, err)
	}
	for _, f := range plan.Files {
		path := outDir + "/" + f.Name + ".md"
		if err := os.WriteFile(path, []byte(f.Content), 0o644); err != nil {
			return fmt.Errorf("write %q: %w", path, err)
		}
		if !gf.Quiet {
			fmt.Printf("  wrote %s\n", path)
		}
	}
	if !gf.Quiet {
		fmt.Printf("generated %d doc file(s) in %s\n", len(plan.Files), outDir)
	}
	return nil
}

func generateOpenAPI(gf globalFlags, args []string) error {
	outFile := "./openapi.json"
	title := "AwoERP API"
	version := "0.1.0"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output":
			if i+1 < len(args) {
				outFile = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				title = args[i+1]
				i++
			}
		case "--api-version":
			if i+1 < len(args) {
				version = args[i+1]
				i++
			}
		}
	}

	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}

	spec, err := openapi.Generate(schema, openapi.Options{
		Title:   title,
		Version: version,
	})
	if err != nil {
		return fmt.Errorf("generate openapi: %w", err)
	}

	if gf.JSON || gf.DryRun {
		return printJSONValue(spec)
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal openapi spec: %w", err)
	}
	if err := os.WriteFile(outFile, data, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", outFile, err)
	}
	if !gf.Quiet {
		fmt.Printf("wrote OpenAPI spec (%d paths) to %s\n", len(spec.Paths), outFile)
	}
	return nil
}

// ── docgen command ────────────────────────────────────────────────────────────

// runDocgenCmd generates Markdown API reference docs and writes them to stdout
// or to a directory when --out <dir> is provided.
func runDocgenCmd(args []string) error {
	outPath := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--out" && i+1 < len(args) {
			outPath = args[i+1]
			i++
		}
	}

	schema, err := compileRegisteredSchema()
	if err != nil {
		return err
	}

	plan, err := docgen.Generate(schema)
	if err != nil {
		return fmt.Errorf("docgen: %w", err)
	}

	if outPath == "" {
		// Write all files concatenated to stdout.
		for _, f := range plan.Files {
			fmt.Print(f.Content)
		}
		return nil
	}

	if err := os.MkdirAll(outPath, 0o755); err != nil {
		return fmt.Errorf("create output dir %q: %w", outPath, err)
	}
	for _, f := range plan.Files {
		path := outPath + "/" + f.Name + ".md"
		if err := os.WriteFile(path, []byte(f.Content), 0o644); err != nil {
			return fmt.Errorf("write %q: %w", path, err)
		}
		fmt.Printf("  wrote %s\n", path)
	}
	fmt.Printf("docgen: wrote %d files to %s\n", len(plan.Files), outPath)
	return nil
}
