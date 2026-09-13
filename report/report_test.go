package report_test

import (
	"strings"
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/filter"
	"awo.so/awo/registry"

	. "awo.so/awo/report"
)

// buildSchema compiles the given defs into a CompiledSchema for testing.
func buildSchema(t *testing.T, defs []def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	return schema
}

// invoiceDef is a minimal entity for report generation tests.
func invoiceDef() def.EntityDefinition {
	return &def.SystemDefinition{
		Name:        "report_invoice",
		Module:      "report",
		Label:       "Invoice",
		LabelPlural: "Invoices",
		Fields: []def.FieldDef{
			{Name: "invoice_number", Type: def.FieldTypeData, Required: true},
			{Name: "total_amount", Type: def.FieldTypeCurrency},
			{Name: "status", Type: def.FieldTypeSelect, Options: []string{"open", "paid", "cancelled"}},
			{Name: "secret_key", Type: def.FieldTypeData, Sensitive: true},
		},
		Permissions: def.PermissionSet{
			Read: []string{"report.invoice.read"},
		},
	}
}

// --- GenerateSQL happy-path tests ---

func TestGenerateSQL_UnknownEntity_Error(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	_, err := GenerateSQL(ReportDefinition{Name: "x", Entity: "nonexistent"}, schema)
	if err == nil {
		t.Fatal("expected error for unknown entity")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention entity name, got: %v", err)
	}
}

func TestGenerateSQL_AllFields_SelectStar(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	q, err := GenerateSQL(ReportDefinition{
		Name:   "all_invoices",
		Entity: "report_invoice",
	}, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil query")
	}
	if !strings.Contains(q.SQL, "SELECT") {
		t.Errorf("SQL must contain SELECT, got: %s", q.SQL)
	}
	if !strings.Contains(q.SQL, "FROM") {
		t.Errorf("SQL must contain FROM, got: %s", q.SQL)
	}
	// Sensitive fields must be excluded.
	if strings.Contains(q.SQL, "secret_key") {
		t.Errorf("sensitive field 'secret_key' must not appear in report SQL")
	}
}

func TestGenerateSQL_ExplicitFields(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	q, err := GenerateSQL(ReportDefinition{
		Name:   "invoice_summary",
		Entity: "report_invoice",
		Fields: []ReportField{
			{Name: "invoice_number"},
			{Name: "total_amount"},
		},
	}, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(q.SQL, `"invoice_number"`) {
		t.Errorf("expected invoice_number in SQL, got: %s", q.SQL)
	}
	if !strings.Contains(q.SQL, `"total_amount"`) {
		t.Errorf("expected total_amount in SQL, got: %s", q.SQL)
	}
	if len(q.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(q.Columns))
	}
}

func TestGenerateSQL_UnknownField_Error(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	_, err := GenerateSQL(ReportDefinition{
		Name:   "bad",
		Entity: "report_invoice",
		Fields: []ReportField{{Name: "nonexistent_field"}},
	}, schema)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(err.Error(), "nonexistent_field") {
		t.Errorf("error should mention field name, got: %v", err)
	}
}

func TestGenerateSQL_RawExpression(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	q, err := GenerateSQL(ReportDefinition{
		Name:   "expr_report",
		Entity: "report_invoice",
		Fields: []ReportField{
			{Expr: "COUNT(*)", Alias: "row_count"},
		},
	}, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(q.SQL, "COUNT(*)") {
		t.Errorf("expected COUNT(*) in SQL, got: %s", q.SQL)
	}
	if q.Columns[0] != "row_count" {
		t.Errorf("expected column alias 'row_count', got %q", q.Columns[0])
	}
}

func TestGenerateSQL_WithFilter(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	f := filter.Eq("status", "open")
	q, err := GenerateSQL(ReportDefinition{
		Name:    "open_invoices",
		Entity:  "report_invoice",
		Filters: f,
	}, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(q.SQL, "WHERE") {
		t.Errorf("expected WHERE clause, got: %s", q.SQL)
	}
	if len(q.Args) != 1 {
		t.Errorf("expected 1 filter arg, got %d", len(q.Args))
	}
	if q.Args[0] != "open" {
		t.Errorf("expected arg 'open', got %v", q.Args[0])
	}
}

func TestGenerateSQL_WithOrderBy(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	q, err := GenerateSQL(ReportDefinition{
		Name:   "sorted",
		Entity: "report_invoice",
		OrderBy: []ReportOrder{
			{Field: "total_amount", Desc: true},
		},
	}, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(q.SQL, "ORDER BY") {
		t.Errorf("expected ORDER BY, got: %s", q.SQL)
	}
	if !strings.Contains(q.SQL, "DESC") {
		t.Errorf("expected DESC, got: %s", q.SQL)
	}
}

func TestGenerateSQL_WithLimitOffset(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	q, err := GenerateSQL(ReportDefinition{
		Name:   "paged",
		Entity: "report_invoice",
		Limit:  10,
		Offset: 20,
	}, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(q.SQL, "LIMIT") {
		t.Errorf("expected LIMIT, got: %s", q.SQL)
	}
	if !strings.Contains(q.SQL, "OFFSET") {
		t.Errorf("expected OFFSET, got: %s", q.SQL)
	}
	// Args should contain 10 and 20.
	found10, found20 := false, false
	for _, a := range q.Args {
		if a == 10 {
			found10 = true
		}
		if a == 20 {
			found20 = true
		}
	}
	if !found10 || !found20 {
		t.Errorf("expected LIMIT=10 and OFFSET=20 in args, got: %v", q.Args)
	}
}

func TestGenerateSQL_WithGroupByAndAggregate(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	q, err := GenerateSQL(ReportDefinition{
		Name:    "by_status",
		Entity:  "report_invoice",
		Fields:  []ReportField{{Name: "status"}},
		GroupBy: []string{"status"},
		Aggregates: []ReportAggregate{
			{Func: AggregateCOUNT, Field: "*", Alias: "cnt"},
			{Func: AggregateAVG, Field: "total_amount", Alias: "avg_amount"},
		},
	}, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(q.SQL, "GROUP BY") {
		t.Errorf("expected GROUP BY, got: %s", q.SQL)
	}
	if !strings.Contains(q.SQL, "COUNT(*)") {
		t.Errorf("expected COUNT(*), got: %s", q.SQL)
	}
	if !strings.Contains(q.SQL, "AVG(") {
		t.Errorf("expected AVG aggregate, got: %s", q.SQL)
	}
}

func TestGenerateSQL_AggregateNoAlias_Error(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	_, err := GenerateSQL(ReportDefinition{
		Name:   "bad_agg",
		Entity: "report_invoice",
		Aggregates: []ReportAggregate{
			{Func: AggregateSUM, Field: "total_amount", Alias: ""},
		},
	}, schema)
	if err == nil {
		t.Fatal("expected error for aggregate without alias")
	}
}

func TestGenerateSQL_ParametersArePositional(t *testing.T) {
	schema := buildSchema(t, []def.EntityDefinition{invoiceDef()})
	f := filter.And(
		filter.Eq("status", "open"),
		filter.Gt("total_amount", 100),
	)
	q, err := GenerateSQL(ReportDefinition{
		Name:    "multi_filter",
		Entity:  "report_invoice",
		Filters: f,
	}, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Positional params: $1, $2
	if !strings.Contains(q.SQL, "$1") {
		t.Errorf("expected $1 in SQL, got: %s", q.SQL)
	}
	if !strings.Contains(q.SQL, "$2") {
		t.Errorf("expected $2 in SQL, got: %s", q.SQL)
	}
	// No injection risk: no literal values in SQL.
	if strings.Contains(q.SQL, "open") {
		t.Errorf("literal value 'open' must not appear in parameterized SQL")
	}
}
