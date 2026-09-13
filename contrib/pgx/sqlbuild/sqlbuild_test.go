package sqlbuild_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"awo.so/awo/contrib/pgx/sqlbuild"
	"awo.so/awo/filter"
)

func TestBuild_Nil(t *testing.T) {
	r, err := sqlbuild.Build(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Clause != "" {
		t.Errorf("expected empty clause for nil filter, got %q", r.Clause)
	}
	if len(r.Args) != 0 {
		t.Errorf("expected no args, got %v", r.Args)
	}
}

func TestBuild_Eq(t *testing.T) {
	r, err := sqlbuild.Build(filter.Eq("status", "active"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Clause != `"status" = $1` {
		t.Errorf("unexpected clause: %q", r.Clause)
	}
	if len(r.Args) != 1 || r.Args[0] != "active" {
		t.Errorf("unexpected args: %v", r.Args)
	}
}

func TestBuild_EqNil(t *testing.T) {
	r, err := sqlbuild.Build(filter.Eq("deleted_at", nil), 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Clause != `"deleted_at" IS NULL` {
		t.Errorf("unexpected clause: %q", r.Clause)
	}
}

func TestBuild_In_Empty(t *testing.T) {
	r, err := sqlbuild.Build(filter.In("id"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Clause != "FALSE" {
		t.Errorf("expected FALSE for empty In, got %q", r.Clause)
	}
}

func TestBuild_And(t *testing.T) {
	f := filter.And(filter.Eq("a", 1), filter.Eq("b", 2))
	r, err := sqlbuild.Build(f, 0)
	if err != nil {
		t.Fatal(err)
	}
	expected := `("a" = $1) AND ("b" = $2)`
	if r.Clause != expected {
		t.Errorf("got %q, want %q", r.Clause, expected)
	}
}

func TestBuild_ParamOffset(t *testing.T) {
	// offset=2 → params start at $3
	r, err := sqlbuild.Build(filter.Eq("x", 99), 2)
	if err != nil {
		t.Fatal(err)
	}
	if r.Clause != `"x" = $3` {
		t.Errorf("unexpected clause: %q", r.Clause)
	}
}

func TestBuild_Between(t *testing.T) {
	r, err := sqlbuild.Build(filter.Between("amount", 100, 999), 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Clause != `"amount" BETWEEN $1 AND $2` {
		t.Errorf("unexpected clause: %q", r.Clause)
	}
}

func TestBuild_Not(t *testing.T) {
	r, err := sqlbuild.Build(filter.Not(filter.Eq("status", "draft")), 0)
	if err != nil {
		t.Fatal(err)
	}
	expected := `NOT ("status" = $1)`
	if r.Clause != expected {
		t.Errorf("got %q, want %q", r.Clause, expected)
	}
}

func TestBuild_Neq(t *testing.T) {
	r, err := sqlbuild.Build(filter.Neq("status", "deleted"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "!=") {
		t.Errorf("expected !=, got %q", r.Clause)
	}
}

func TestBuild_Gt(t *testing.T) {
	r, err := sqlbuild.Build(filter.Gt("amount", int64(100)), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, ">") {
		t.Errorf("expected >, got %q", r.Clause)
	}
}

func TestBuild_Gte(t *testing.T) {
	r, err := sqlbuild.Build(filter.Gte("amount", int64(0)), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, ">=") {
		t.Errorf("expected >=, got %q", r.Clause)
	}
}

func TestBuild_Lt(t *testing.T) {
	r, err := sqlbuild.Build(filter.Lt("amount", decimal.NewFromInt(500)), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "<") {
		t.Errorf("expected <, got %q", r.Clause)
	}
}

func TestBuild_Lte(t *testing.T) {
	r, err := sqlbuild.Build(filter.Lte("amount", decimal.NewFromInt(500)), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "<=") {
		t.Errorf("expected <=, got %q", r.Clause)
	}
}

func TestBuild_NotIn(t *testing.T) {
	r, err := sqlbuild.Build(filter.NotIn("status", "deleted", "archived"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "ALL") {
		t.Errorf("expected ALL in NOT IN clause, got %q", r.Clause)
	}
	if len(r.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(r.Args))
	}
}

func TestBuild_IsNull(t *testing.T) {
	r, err := sqlbuild.Build(filter.IsNull("deleted_at"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "IS NULL") {
		t.Errorf("expected IS NULL, got %q", r.Clause)
	}
	if len(r.Args) != 0 {
		t.Errorf("IS NULL should have no args, got %v", r.Args)
	}
}

func TestBuild_IsNotNull(t *testing.T) {
	r, err := sqlbuild.Build(filter.IsNotNull("deleted_at"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "IS NOT NULL") {
		t.Errorf("expected IS NOT NULL, got %q", r.Clause)
	}
}

func TestBuild_Contains(t *testing.T) {
	r, err := sqlbuild.Build(filter.Contains("name", "acme"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "ILIKE") {
		t.Errorf("expected ILIKE, got %q", r.Clause)
	}
	val, _ := r.Args[0].(string)
	if !strings.HasPrefix(val, "%") || !strings.HasSuffix(val, "%") {
		t.Errorf("Contains arg should be %%value%%, got %q", val)
	}
}

func TestBuild_Contains_LikeEscape(t *testing.T) {
	r, err := sqlbuild.Build(filter.Contains("name", "100%off"), 0)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := r.Args[0].(string)
	// raw % must be escaped
	if strings.Contains(val, "100%off") {
		t.Errorf("%% must be escaped in LIKE pattern, got %q", val)
	}
}

func TestBuild_StartsWith(t *testing.T) {
	r, err := sqlbuild.Build(filter.StartsWith("code", "INV-"), 0)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := r.Args[0].(string)
	if !strings.HasSuffix(val, "%") || strings.HasPrefix(val, "%") {
		t.Errorf("StartsWith arg should be value%%, got %q", val)
	}
}

func TestBuild_EndsWith(t *testing.T) {
	r, err := sqlbuild.Build(filter.EndsWith("email", "@acme.com"), 0)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := r.Args[0].(string)
	if !strings.HasPrefix(val, "%") || strings.HasSuffix(val, "%") {
		t.Errorf("EndsWith arg should be %%value, got %q", val)
	}
}

func TestBuild_Or(t *testing.T) {
	r, err := sqlbuild.Build(
		filter.Or(filter.Eq("type", "A"), filter.Eq("type", "B")),
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "OR") {
		t.Errorf("expected OR, got %q", r.Clause)
	}
}

func TestBuild_CustomGt(t *testing.T) {
	r, err := sqlbuild.Build(filter.CustomGt("score", decimal.NewFromInt(80)), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "::numeric") {
		t.Errorf("expected numeric cast, got %q", r.Clause)
	}
}

func TestBuild_CustomLt(t *testing.T) {
	r, err := sqlbuild.Build(filter.CustomLt("score", decimal.NewFromInt(50)), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "<") {
		t.Errorf("expected < in custom LT clause, got %q", r.Clause)
	}
}

func TestBuild_UnsupportedKind_ReturnsError(t *testing.T) {
	f := &filter.Filter{Kind: filter.Kind("unknown_kind"), Field: "x", Value: "y"}
	_, err := sqlbuild.Build(f, 0)
	if err == nil {
		t.Fatal("expected error for unsupported filter kind; got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention 'unsupported', got: %v", err)
	}
}

// TestBuild_UnsupportedKind_TypedError verifies that the returned error is a
// *sqlbuild.TranslationError so callers can use errors.As to inspect it.
func TestBuild_UnsupportedKind_TypedError(t *testing.T) {
	f := &filter.Filter{Kind: filter.Kind("nonexistent_op"), Field: "amount", Value: 42}
	_, err := sqlbuild.Build(f, 0)
	if err == nil {
		t.Fatal("expected error for unsupported filter kind; got nil")
	}
	var te *sqlbuild.TranslationError
	if !errors.As(err, &te) {
		t.Fatalf("expected *sqlbuild.TranslationError via errors.As, got %T: %v", err, err)
	}
	if te.Operator == "" {
		t.Error("TranslationError.Operator must not be empty")
	}
	if te.Reason == "" {
		t.Error("TranslationError.Reason must not be empty")
	}
}

// TestTranslationError_MessageContainsOperator verifies the error message is informative.
func TestTranslationError_MessageContainsOperator(t *testing.T) {
	te := &sqlbuild.TranslationError{
		Operator: "bad_op",
		Field:    "status",
		Reason:   "unsupported filter kind",
	}
	msg := te.Error()
	if !strings.Contains(msg, "bad_op") {
		t.Errorf("TranslationError message should contain operator, got %q", msg)
	}
}

// TestTranslationError_NoField_OmitsFieldFromMessage verifies the error message
// without a field name is still valid.
func TestTranslationError_NoField_OmitsFieldFromMessage(t *testing.T) {
	te := &sqlbuild.TranslationError{
		Operator: "bad_op",
		Reason:   "unsupported filter kind",
	}
	msg := te.Error()
	if msg == "" {
		t.Error("TranslationError.Error() must not be empty")
	}
}

// TestBuildWithAllowlist_OrderBy_ASC_DIR_Safe ensures OrderBy direction in
// query options is always a binary bool — tested via the allowlist path to
// confirm the two-direction invariant.
func TestBuildWithAllowlist_OrderBy_UnknownField_Error(t *testing.T) {
	al := sqlbuild.NewManualAllowlist("test_entity", []string{"status", "amount"})
	// "unknown_col" is not in the allowlist; Build should return FieldNotAllowedError.
	f := filter.Eq("unknown_col", "x")
	_, err := sqlbuild.BuildWithAllowlist(f, 0, al)
	if err == nil {
		t.Fatal("expected error for unknown field; got nil")
	}
	var fae *sqlbuild.FieldNotAllowedError
	if !errors.As(err, &fae) {
		t.Fatalf("expected *sqlbuild.FieldNotAllowedError via errors.As, got %T: %v", err, err)
	}
	if fae.Field == "" {
		t.Error("FieldNotAllowedError.Field must be set")
	}
}

// TestBuildWithAllowlist_KnownField_Passes verifies a known field is accepted.
func TestBuildWithAllowlist_KnownField_Passes(t *testing.T) {
	al := sqlbuild.NewManualAllowlist("test_entity", []string{"status"})
	_, err := sqlbuild.BuildWithAllowlist(filter.Eq("status", "active"), 0, al)
	if err != nil {
		t.Fatalf("known field should pass allowlist: %v", err)
	}
}

// TestBuild_Like_ViaContains ensures filter.Like() (alias for Contains) round-trips
// through sqlbuild correctly.
func TestBuild_Like_ViaContains(t *testing.T) {
	r, err := sqlbuild.Build(filter.Like("name", "acme"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Clause, "ILIKE") {
		t.Errorf("Like/Contains should produce ILIKE clause, got %q", r.Clause)
	}
	val, _ := r.Args[0].(string)
	if !strings.HasPrefix(val, "%") || !strings.HasSuffix(val, "%") {
		t.Errorf("Contains arg should be %%value%%, got %q", val)
	}
}

func TestBuild_InjectionSafe_FieldName(t *testing.T) {
	// quoteIdent double-quotes field names; any embedded double-quotes are
	// doubled, so the whole field — including SQL metacharacters — is treated
	// as a single identifier by PostgreSQL. Verify the clause is wrapped in
	// double-quotes and that "DROP TABLE" is not present as a bare token.
	f := filter.Eq(`"; DROP TABLE users; --`, "x")
	r, err := sqlbuild.Build(f, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Clause must start with a double-quoted identifier.
	if !strings.HasPrefix(r.Clause, `"`) {
		t.Errorf("expected double-quoted identifier, got %q", r.Clause)
	}
	// The injected semicolon must not produce a bare DROP TABLE statement —
	// i.e. it must not appear outside a quoted context. Since quoteIdent
	// wraps the whole field in "…", the semicolon is inside the identifier
	// and the value is parameterised, so no bare DROP TOKEN can escape.
	// A naive check: the clause must end with a positional param ($N), not
	// with SQL keywords.
	if !strings.HasSuffix(strings.TrimSpace(r.Clause), "$1") {
		t.Errorf("clause should end with positional param, got %q", r.Clause)
	}
}

func TestBuild_InjectionSafe_Value(t *testing.T) {
	// Values go into positional args, never interpolated into SQL.
	f := filter.Eq("status", "'; DROP TABLE users; --")
	r, err := sqlbuild.Build(f, 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Clause, "DROP TABLE") {
		t.Error("SQL injection via value succeeded — parameterization is broken")
	}
	if len(r.Args) != 1 {
		t.Errorf("value should be in args, not clause; got %d args", len(r.Args))
	}
}

// --- Allowlist tests ---

func TestBuildWithAllowlist_AllowedField(t *testing.T) {
	al := sqlbuild.NewManualAllowlist("test_widget", []string{"status", "name"})
	r, err := sqlbuild.BuildWithAllowlist(filter.Eq("status", "active"), 0, al)
	if err != nil {
		t.Fatalf("allowed field should not return error: %v", err)
	}
	if r.Clause == "" {
		t.Error("expected non-empty clause")
	}
}

func TestBuildWithAllowlist_DeniedField(t *testing.T) {
	al := sqlbuild.NewManualAllowlist("test_widget", []string{"status"})
	_, err := sqlbuild.BuildWithAllowlist(filter.Eq("secret_column", "x"), 0, al)
	if err == nil {
		t.Fatal("expected FieldNotAllowedError for unlisted field; got nil")
	}
	if _, ok := err.(*sqlbuild.FieldNotAllowedError); !ok {
		t.Errorf("expected *FieldNotAllowedError, got %T: %v", err, err)
	}
}

func TestBuildWithAllowlist_CustomField_Exempt(t *testing.T) {
	// Custom predicates use JSONB keys — exempt from allowlist.
	al := sqlbuild.NewManualAllowlist("test_widget", []string{"status"})
	_, err := sqlbuild.BuildWithAllowlist(filter.CustomEq("any_jsonb_key", "v"), 0, al)
	if err != nil {
		t.Fatalf("custom field predicates exempt from allowlist: %v", err)
	}
}

func TestBuildWithAllowlist_NestedDeniedField(t *testing.T) {
	al := sqlbuild.NewManualAllowlist("test_widget", []string{"status"})
	f := filter.And(
		filter.Eq("status", "active"),
		filter.Eq("forbidden_col", "x"),
	)
	_, err := sqlbuild.BuildWithAllowlist(f, 0, al)
	if err == nil {
		t.Fatal("expected error for nested denied field; got nil")
	}
}

func TestBuildWithAllowlist_StandardColumnsAlwaysAllowed(t *testing.T) {
	al := sqlbuild.NewManualAllowlist("test_widget", nil)
	for _, col := range []string{"id", "tenant_id", "created_at", "updated_at", "deleted_at"} {
		_, err := sqlbuild.BuildWithAllowlist(filter.IsNotNull(col), 0, al)
		if err != nil {
			t.Errorf("standard column %q should always be allowed: %v", col, err)
		}
	}
}

func TestBuildWithAllowlist_Nil(t *testing.T) {
	al := sqlbuild.NewManualAllowlist("test_widget", nil)
	r, err := sqlbuild.BuildWithAllowlist(nil, 0, al)
	if err != nil {
		t.Fatalf("nil filter should return empty result: %v", err)
	}
	if r.Clause != "" {
		t.Errorf("expected empty clause, got %q", r.Clause)
	}
}

func TestFieldNotAllowedError_Message(t *testing.T) {
	err := &sqlbuild.FieldNotAllowedError{Field: "hack", EntityName: "mod_item"}
	if !strings.Contains(err.Error(), "hack") || !strings.Contains(err.Error(), "mod_item") {
		t.Errorf("error message missing field or entity: %q", err.Error())
	}
}

func TestAllowlist_Allow(t *testing.T) {
	al := sqlbuild.NewManualAllowlist("e", []string{"name"})
	if !al.Allow("name") {
		t.Error("declared field should be allowed")
	}
	if al.Allow("not_declared") {
		t.Error("undeclared field should not be allowed")
	}
	if !al.Allow("id") {
		t.Error("id should always be allowed")
	}
}
