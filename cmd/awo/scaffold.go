package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// scaffoldModule returns a map of file path → content for a new module.
// Generated files follow the standard module layout:
//
//	internal/<module>/definition.go
//	internal/<module>/hooks.go
//	internal/<module>/policy.go
//	internal/<module>/service.go
//	internal/<module>/module.go
func scaffoldModule(name string) map[string]string {
	pkg := sanitizeIdent(name)
	pascal := toPascal(name)
	dir := filepath.Join("internal", name)

	return map[string]string{
		filepath.Join(dir, "definition.go"): moduleDefinition(pkg, pascal, name),
		filepath.Join(dir, "hooks.go"):      moduleHooks(pkg, pascal),
		filepath.Join(dir, "policy.go"):     modulePolicy(pkg, pascal),
		filepath.Join(dir, "service.go"):    moduleService(pkg, pascal),
		filepath.Join(dir, "module.go"):     moduleInit(pkg, pascal),
	}
}

// scaffoldEntity returns a map of file path → content for a new entity definition
// inside an existing module.
func scaffoldEntity(module, name string) map[string]string {
	pkg := sanitizeIdent(module)
	entityName := module + "_" + name
	pascal := toPascal(name)
	dir := filepath.Join("internal", module)

	fname := fmt.Sprintf("entity_%s.go", sanitizeIdent(name))
	return map[string]string{
		filepath.Join(dir, fname): entityDefinition(pkg, pascal, entityName, module),
	}
}

// scaffoldWorkflow returns a map of file path → content for a new Temporal
// workflow stub.
func scaffoldWorkflow(name string) map[string]string {
	pkg := "workflows"
	pascal := toPascal(name)
	dir := "workflows"

	fname := fmt.Sprintf("%s_workflow.go", sanitizeIdent(name))
	testName := fmt.Sprintf("%s_workflow_test.go", sanitizeIdent(name))
	return map[string]string{
		filepath.Join(dir, fname):    workflowStub(pkg, pascal),
		filepath.Join(dir, testName): workflowTest(pkg, pascal),
	}
}

// writeScaffold writes content to path, creating intermediate directories.
// Returns an error if the file already exists (to prevent accidental overwrites).
func writeScaffold(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s (remove it first or choose a different name)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// --- template helpers --------------------------------------------------------

func moduleDefinition(pkg, pascal, module string) string {
	return fmt.Sprintf(`package %s

import "awo.so/awo/sdk"

// %sDef is the primary entity definition for the %s module.
// Add fields, edges, hooks, permissions, and workflow triggers here.
var %sDef = sdk.System(
	"%s_record",   // entity name: {module}_{noun}
	"%s",          // module
	"%s Record",   // label
	"%s Records",  // label plural
).
	Permissions(sdk.TenantScoped()).
	Build()
`, pkg, pascal, module, pascal, module, module, pascal, pascal)
}

func moduleHooks(pkg, pascal string) string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"fmt"

	"awo.so/awo/def"
)

// %sValidator enforces business rules before a record is persisted.
type %sValidator struct{}

// BeforeCreate validates the record before creation.
func (v *%sValidator) BeforeCreate(ctx context.Context, rec *def.EntityRecord) error {
	// TODO: add validation logic
	_ = rec
	return nil
}

// BeforeUpdate validates the record before update.
func (v *%sValidator) BeforeUpdate(ctx context.Context, rec *def.EntityRecord) error {
	// TODO: add validation logic
	_ = rec
	return nil
}

// Ensure interface compliance at compile time.
var _ def.BeforeCreateHook = (*%sValidator)(nil)
var _ def.BeforeUpdateHook = (*%sValidator)(nil)

// ErrInvalid is returned when validation fails.
var ErrInvalid = fmt.Errorf("%s: validation failed")
`, pkg, pascal, pascal, pascal, pascal, pascal, pascal, pkg)
}

func modulePolicy(pkg, pascal string) string {
	return fmt.Sprintf(`package %s

import (
	"context"

	"awo.so/awo/def"
)

// OwnerPolicy restricts row visibility to the record's creator.
// Replace or extend this with domain-specific predicate logic.
func OwnerPolicy(ctx context.Context) def.Filter {
	// TODO: derive actor from ctx and return a filter predicate.
	// Example using the filter package:
	//   actor := session.ActorFromContext(ctx)
	//   return filter.Eq("created_by", actor.UserID)
	_ = ctx
	return nil // nil = no additional row filter (all rows visible)
}

// Ensure the function signature matches def.PolicyFunc.
var _ def.PolicyFunc = OwnerPolicy

// %sPolicy is the default policy for %s entities.
// Wire it to the entity definition via sdk.WithPolicy(sdk.TenantScoped(), %sPolicy).
var %sPolicy def.PolicyFunc = OwnerPolicy
`, pkg, pascal, pkg, pascal, pascal)
}

func moduleService(pkg, pascal string) string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/driver"
)

// Service provides domain operations for the %s module.
type Service struct {
	repo driver.EntityRepository[*def.EntityRecord]
}

// NewService creates a Service backed by the given repository.
func NewService(repo driver.EntityRepository[*def.EntityRecord]) *Service {
	return &Service{repo: repo}
}

// Get returns a single record by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*def.EntityRecord, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s.Get: %%w", err)
	}
	return rec, nil
}

// TODO: add domain-specific service methods here.
`, pkg, pkg, pkg)
}

func moduleInit(pkg, pascal string) string {
	return fmt.Sprintf(`package %s

import "awo.so/awo/def"

func init() {
	def.Register(&%sDef)
}
`, pkg, pascal)
}

func entityDefinition(pkg, pascal, entityName, module string) string {
	return fmt.Sprintf(`package %s

import "awo.so/awo/sdk"

// %sDef defines the %s entity.
var %sDef = sdk.System(
	"%s",
	"%s",
	"%s",
	"%ss",
).
	Field(sdk.Data("name").Required().Searchable().Build()).
	Permissions(sdk.TenantScoped()).
	Build()

func init() {
	// TODO: register this entity in the module's init() or module.go.
	// def.Register(&%sDef)
}
`, pkg, pascal, entityName, pascal, entityName, module, pascal, pascal, pascal)
}

func workflowStub(pkg, pascal string) string {
	return fmt.Sprintf(`package %s

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// %sInput is the input payload for %sWorkflow.
type %sInput struct {
	TenantID string
	RecordID string
}

// %sResult is the result payload for %sWorkflow.
type %sResult struct {
	Message string
}

// %sWorkflow is a Temporal workflow stub.
//
// Rules:
//   - No time.Now() → use workflow.Now(ctx)
//   - No time.Sleep() → use workflow.Sleep(ctx, d)
//   - No direct I/O → call activities
//   - No raw goroutines → use workflow.Go(ctx, fn)
func %sWorkflow(ctx workflow.Context, input %sInput) (*%sResult, error) {
	// Configure activity options.
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// TODO: execute activities.
	// var result SomeActivityResult
	// if err := workflow.ExecuteActivity(ctx, activities.DoSomethingActivity, input).Get(ctx, &result); err != nil {
	// 	return nil, err
	// }

	return &%sResult{Message: "completed"}, nil
}
`, pkg, pascal, pascal, pascal, pascal, pascal, pascal, pascal, pascal, pascal, pascal, pascal)
}

func workflowTest(pkg, pascal string) string {
	return fmt.Sprintf(`package %s

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func Test%sWorkflow(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(%sWorkflow, %sInput{
		TenantID: "tenant-123",
		RecordID: "record-456",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result %sResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.NotEmpty(t, result.Message)
}
`, pkg, pascal, pascal, pascal, pascal)
}

// --- string helpers ----------------------------------------------------------

// sanitizeIdent converts a name to a valid Go package identifier (lowercase,
// underscores for non-alphanumeric chars).
func sanitizeIdent(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// toPascal converts snake_case or kebab-case to PascalCase.
func toPascal(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		b.WriteRune(unicode.ToUpper(runes[0]))
		b.WriteString(string(runes[1:]))
	}
	return b.String()
}
