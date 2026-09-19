// Package report defines the ReportDefinition DSL and generates parameterized
// SELECT SQL from it.
//
// A ReportDefinition describes which entity to query, which fields to project,
// optional edge joins, filter predicates, grouping, and ordering. The SQL
// generator translates this into a single parameterized PostgreSQL query.
//
// # Usage
//
//	def := report.ReportDefinition{
//	    Name:   "open_invoices",
//	    Entity: "finance_invoice",
//	    Fields: []report.ReportField{
//	        {Name: "invoice_number"},
//	        {Name: "total_amount"},
//	        {Expr: "SUM(total_amount)", Alias: "sum_amount"},
//	    },
//	    Filters: filter.Eq("status", "open"),
//	    GroupBy: []string{"invoice_number"},
//	    OrderBy: []report.ReportOrder{
//	        {Field: "total_amount", Desc: true},
//	    },
//	    Limit: 100,
//	}
//	sql, args, err := report.GenerateSQL(def, schema)
package report

import (
	"fmt"
	"strings"

	"awo.so/awo/compiler"
	"awo.so/awo/filter"
)

// ReportField is one projected column in a report.
// Either Name (entity field) or Expr (raw SQL expression) must be set.
type ReportField struct {
	// Name is the entity field name (e.g. "invoice_number", "total_amount").
	// When set, the generator emits the column reference with the table alias.
	Name string

	// Expr is a raw SQL expression (e.g. "SUM(total_amount)", "COALESCE(name,'—')").
	// When Name is empty and Expr is non-empty, Expr is used verbatim — with
	// no validation and no allowlist check, unlike every other field on
	// ReportDefinition. This is intentional: it is the escape hatch for
	// computed columns the Name-based path can't express. It is only safe
	// because a ReportDefinition is Go-code-constructed by whoever defines
	// the report, the same trust level as writing the SQL by hand — it MUST
	// NEVER be populated from end-user or API-request input.
	// Expressions must use column names without table qualifiers.
	Expr string

	// Alias is the AS alias for this column. Required when Expr is set.
	// Optional when Name is set (defaults to the field name).
	Alias string
}

// ReportJoin declares an edge-based JOIN for a report.
// Only edges declared in the entity's EdgeDef can be joined.
type ReportJoin struct {
	// EdgeName is the name of the declared edge to join.
	EdgeName string

	// Fields are the fields to project from the joined entity.
	Fields []ReportField

	// JoinType is the SQL join type: "INNER", "LEFT", "RIGHT". Default: "LEFT".
	JoinType string
}

// ReportOrder defines an ORDER BY clause entry.
type ReportOrder struct {
	// Field is the entity field name or an alias from ReportField.Alias.
	Field string

	// Desc requests descending order (DESC). Default: ascending (ASC).
	Desc bool
}

// AggregateFunc is the SQL aggregate function to apply.
type AggregateFunc string

const (
	AggregateSUM   AggregateFunc = "SUM"
	AggregateAVG   AggregateFunc = "AVG"
	AggregateCOUNT AggregateFunc = "COUNT"
	AggregateMAX   AggregateFunc = "MAX"
	AggregateMIN   AggregateFunc = "MIN"
)

// ReportAggregate declares an aggregate column in addition to the field list.
type ReportAggregate struct {
	// Func is the aggregate function (SUM, AVG, COUNT, MAX, MIN).
	Func AggregateFunc

	// Field is the field to aggregate. Use "*" for COUNT(*).
	Field string

	// Alias is the output column name. Required.
	Alias string
}

// ReportDefinition describes a report query against a single source entity.
type ReportDefinition struct {
	// Name is the stable report identifier. Used in logging and cache keys.
	Name string

	// Entity is the qualified source entity name (e.g. "finance_invoice").
	Entity string

	// Fields are the projected columns. If empty, all entity fields are selected.
	Fields []ReportField

	// Joins declares edge-based JOINs.
	Joins []ReportJoin

	// Filters is the WHERE predicate. Nil means no filter.
	Filters *filter.Filter

	// GroupBy lists field names or aliases for the GROUP BY clause.
	GroupBy []string

	// Having is a filter applied after GROUP BY (HAVING clause). Nil means no HAVING.
	Having *filter.Filter

	// Aggregates are additional aggregate columns projected alongside Fields.
	Aggregates []ReportAggregate

	// OrderBy defines the ORDER BY clause.
	OrderBy []ReportOrder

	// Limit caps the number of result rows. 0 means no limit.
	Limit int

	// Offset skips the first N result rows. 0 means no offset.
	Offset int
}

// GeneratedQuery is the output of GenerateSQL.
type GeneratedQuery struct {
	// SQL is the parameterized PostgreSQL SELECT statement.
	SQL string

	// Args are the positional parameter values ($1, $2, ...) for the SQL statement.
	Args []any

	// Columns is the ordered list of output column names (aliases or field names).
	Columns []string
}

// GenerateSQL translates a ReportDefinition into a parameterized SELECT query.
// schema is used to validate field names, resolve join targets, and build
// column references. Returns an error if the definition references unknown
// entities or fields.
func GenerateSQL(def ReportDefinition, schema *compiler.CompiledSchema) (*GeneratedQuery, error) {
	es, ok := schema.ByName[def.Entity]
	if !ok {
		return nil, fmt.Errorf("report %q: entity %q not found in schema", def.Name, def.Entity)
	}

	const tableAlias = "t"
	var args []any
	argN := 0

	// ── SELECT clause ─────────────────────────────────────────────────────────
	var selects []string
	var columns []string

	if len(def.Fields) == 0 {
		// Default: all entity fields.
		for _, f := range es.Fields {
			if f.Sensitive {
				continue // exclude sensitive fields from reports
			}
			col := tableAlias + "." + quote(f.Name)
			selects = append(selects, col)
			columns = append(columns, f.Name)
		}
	} else {
		for _, rf := range def.Fields {
			if rf.Expr != "" {
				alias := rf.Alias
				if alias == "" {
					alias = "col_" + fmt.Sprint(len(columns))
				}
				selects = append(selects, rf.Expr+" AS "+quote(alias))
				columns = append(columns, alias)
			} else {
				if _, ok := es.FieldsByName[rf.Name]; !ok {
					return nil, fmt.Errorf("report %q: field %q not found on entity %q", def.Name, rf.Name, def.Entity)
				}
				alias := rf.Alias
				if alias == "" {
					alias = rf.Name
				}
				col := tableAlias + "." + quote(rf.Name)
				if alias != rf.Name {
					col += " AS " + quote(alias)
				}
				selects = append(selects, col)
				columns = append(columns, alias)
			}
		}
	}

	// Aggregates.
	for _, agg := range def.Aggregates {
		if agg.Alias == "" {
			return nil, fmt.Errorf("report %q: aggregate on field %q requires an alias", def.Name, agg.Field)
		}
		if !isValidAggregateFunc(agg.Func) {
			// agg.Func is interpolated as a bare SQL function-name token, not
			// an identifier — it cannot be quote()'d the way a column name
			// can (quoting would turn it into a column reference instead of
			// a function call), so this exact-match check against the known
			// set is the only thing standing between it and arbitrary SQL.
			return nil, fmt.Errorf("report %q: unsupported aggregate function %q", def.Name, agg.Func)
		}
		fieldRef := agg.Field
		if fieldRef != "*" {
			if _, ok := es.FieldsByName[fieldRef]; !ok {
				return nil, fmt.Errorf("report %q: aggregate field %q not found on entity %q", def.Name, fieldRef, def.Entity)
			}
			fieldRef = tableAlias + "." + quote(fieldRef)
		}
		expr := string(agg.Func) + "(" + fieldRef + ") AS " + quote(agg.Alias)
		selects = append(selects, expr)
		columns = append(columns, agg.Alias)
	}

	// ── FROM clause ───────────────────────────────────────────────────────────
	fromClause := quote(es.TableName) + " AS " + tableAlias

	// ── JOIN clauses ──────────────────────────────────────────────────────────
	var joins []string
	for i, j := range def.Joins {
		edge, ok := es.EdgesByName[j.EdgeName]
		if !ok {
			return nil, fmt.Errorf("report %q: edge %q not found on entity %q", def.Name, j.EdgeName, def.Entity)
		}
		targetES, ok := schema.ByName[edge.Target]
		if !ok {
			return nil, fmt.Errorf("report %q: edge target %q not found in schema", def.Name, edge.Target)
		}
		joinAlias := fmt.Sprintf("j%d", i)
		jType := j.JoinType
		if jType == "" {
			jType = "LEFT"
		}
		fk := edge.ForeignKey
		if fk == "" {
			fk = es.QualifiedName + "_id"
		}
		joinLine := fmt.Sprintf("%s JOIN %s AS %s ON %s.%s = %s.id",
			jType, quote(targetES.TableName), joinAlias,
			joinAlias, quote(fk), tableAlias)
		joins = append(joins, joinLine)

		// Project join fields.
		for _, jf := range j.Fields {
			if jf.Expr != "" {
				alias := jf.Alias
				if alias == "" {
					alias = "j_col_" + fmt.Sprint(len(columns))
				}
				selects = append(selects, jf.Expr+" AS "+quote(alias))
				columns = append(columns, alias)
			} else {
				alias := jf.Alias
				if alias == "" {
					alias = jf.Name
				}
				col := joinAlias + "." + quote(jf.Name)
				if alias != jf.Name {
					col += " AS " + quote(alias)
				}
				selects = append(selects, col)
				columns = append(columns, alias)
			}
		}
	}

	// ── WHERE clause ──────────────────────────────────────────────────────────
	var whereClause string
	if def.Filters != nil {
		w, wArgs, err := buildFilterSQL(def.Filters, tableAlias, argN)
		if err != nil {
			return nil, fmt.Errorf("report %q: filter: %w", def.Name, err)
		}
		whereClause = w
		args = append(args, wArgs...)
		argN += len(wArgs)
	}

	// knownOutputName reports whether name is either a real entity field or
	// one of the aliases already projected in the SELECT clause above —
	// GroupBy and OrderBy may reference either (per their doc comments).
	// quote() already prevents identifier-quote breakout (it strips embedded
	// quote characters before wrapping), so this check is an authorization
	// gate, not the injection defense — but a caller should not be able to
	// GROUP BY / ORDER BY a column that isn't actually part of this report
	// just because quote() would render some syntactically valid SQL for it.
	knownColumns := make(map[string]bool, len(es.FieldsByName)+len(columns))
	for name := range es.FieldsByName {
		knownColumns[name] = true
	}
	for _, c := range columns {
		knownColumns[c] = true
	}
	knownOutputName := func(name string) bool { return knownColumns[name] }

	// ── GROUP BY ──────────────────────────────────────────────────────────────
	var groupClause string
	if len(def.GroupBy) > 0 {
		quoted := make([]string, len(def.GroupBy))
		for i, g := range def.GroupBy {
			if !knownOutputName(g) {
				return nil, fmt.Errorf("report %q: group by field %q not found on entity %q or its output columns", def.Name, g, def.Entity)
			}
			quoted[i] = quote(g)
		}
		groupClause = strings.Join(quoted, ", ")
	}

	// ── HAVING ────────────────────────────────────────────────────────────────
	var havingClause string
	if def.Having != nil {
		h, hArgs, err := buildFilterSQL(def.Having, tableAlias, argN)
		if err != nil {
			return nil, fmt.Errorf("report %q: having: %w", def.Name, err)
		}
		havingClause = h
		args = append(args, hArgs...)
		argN += len(hArgs)
	}

	// ── ORDER BY ──────────────────────────────────────────────────────────────
	var orderClauses []string
	for _, o := range def.OrderBy {
		if !knownOutputName(o.Field) {
			return nil, fmt.Errorf("report %q: order by field %q not found on entity %q or its output columns", def.Name, o.Field, def.Entity)
		}
		dir := "ASC"
		if o.Desc {
			dir = "DESC"
		}
		orderClauses = append(orderClauses, quote(o.Field)+" "+dir)
	}

	// ── Assemble ──────────────────────────────────────────────────────────────
	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(selects, ", "))
	sb.WriteString("\nFROM ")
	sb.WriteString(fromClause)
	for _, j := range joins {
		sb.WriteString("\n")
		sb.WriteString(j)
	}
	if whereClause != "" {
		sb.WriteString("\nWHERE ")
		sb.WriteString(whereClause)
	}
	if groupClause != "" {
		sb.WriteString("\nGROUP BY ")
		sb.WriteString(groupClause)
	}
	if havingClause != "" {
		sb.WriteString("\nHAVING ")
		sb.WriteString(havingClause)
	}
	if len(orderClauses) > 0 {
		sb.WriteString("\nORDER BY ")
		sb.WriteString(strings.Join(orderClauses, ", "))
	}
	if def.Limit > 0 {
		argN++
		args = append(args, def.Limit)
		sb.WriteString(fmt.Sprintf("\nLIMIT $%d", argN))
	}
	if def.Offset > 0 {
		argN++
		args = append(args, def.Offset)
		sb.WriteString(fmt.Sprintf("\nOFFSET $%d", argN))
	}

	return &GeneratedQuery{
		SQL:     sb.String(),
		Args:    args,
		Columns: columns,
	}, nil
}

// quote wraps an identifier in double-quotes for PostgreSQL.
// Only identifiers go through this — never user values (which must be parameters).
func quote(id string) string {
	// Reject identifiers with double-quotes to prevent injection via identifier names.
	// Entity/field names are compiler-validated and should never contain quotes.
	return `"` + strings.ReplaceAll(id, `"`, ``) + `"`
}

// isValidAggregateFunc reports whether fn is one of the SQL aggregate
// functions GenerateSQL knows how to emit.
func isValidAggregateFunc(fn AggregateFunc) bool {
	switch fn {
	case AggregateSUM, AggregateAVG, AggregateCOUNT, AggregateMAX, AggregateMIN:
		return true
	default:
		return false
	}
}

// buildFilterSQL translates a *filter.Filter into SQL and positional args,
// starting arg numbering from startN+1.
// Returns ("", nil, nil) for a nil filter.
func buildFilterSQL(f *filter.Filter, tableAlias string, startN int) (string, []any, error) {
	if f == nil {
		return "", nil, nil
	}
	b := &filterBuilder{alias: tableAlias, n: startN}
	sql, err := b.build(f)
	return sql, b.args, err
}

type filterBuilder struct {
	alias string
	n     int
	args  []any
}

func (b *filterBuilder) param(v any) string {
	b.n++
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", b.n)
}

func (b *filterBuilder) colRef(field string) string {
	return b.alias + "." + quote(field)
}

func (b *filterBuilder) build(f *filter.Filter) (string, error) {
	switch f.Kind {
	case filter.KindAnd:
		parts := make([]string, 0, len(f.Sub))
		for _, c := range f.Sub {
			s, err := b.build(c)
			if err != nil {
				return "", err
			}
			parts = append(parts, "("+s+")")
		}
		return strings.Join(parts, " AND "), nil
	case filter.KindOr:
		parts := make([]string, 0, len(f.Sub))
		for _, c := range f.Sub {
			s, err := b.build(c)
			if err != nil {
				return "", err
			}
			parts = append(parts, "("+s+")")
		}
		return strings.Join(parts, " OR "), nil
	case filter.KindNot:
		if len(f.Sub) != 1 {
			return "", fmt.Errorf("NOT filter must have exactly one child")
		}
		s, err := b.build(f.Sub[0])
		if err != nil {
			return "", err
		}
		return "NOT (" + s + ")", nil
	case filter.KindEq:
		return b.colRef(f.Field) + " = " + b.param(f.Value), nil
	case filter.KindNeq:
		return b.colRef(f.Field) + " != " + b.param(f.Value), nil
	case filter.KindGt:
		return b.colRef(f.Field) + " > " + b.param(f.Value), nil
	case filter.KindGte:
		return b.colRef(f.Field) + " >= " + b.param(f.Value), nil
	case filter.KindLt:
		return b.colRef(f.Field) + " < " + b.param(f.Value), nil
	case filter.KindLte:
		return b.colRef(f.Field) + " <= " + b.param(f.Value), nil
	case filter.KindIsNull:
		return b.colRef(f.Field) + " IS NULL", nil
	case filter.KindIsNotNull:
		return b.colRef(f.Field) + " IS NOT NULL", nil
	case filter.KindContains:
		return b.colRef(f.Field) + " ILIKE " + b.param("%"+fmt.Sprint(f.Value)+"%"), nil
	case filter.KindStartsWith:
		return b.colRef(f.Field) + " ILIKE " + b.param(fmt.Sprint(f.Value)+"%"), nil
	case filter.KindEndsWith:
		return b.colRef(f.Field) + " ILIKE " + b.param("%"+fmt.Sprint(f.Value)), nil
	case filter.KindIn:
		placeholders := make([]string, len(f.In))
		for i, v := range f.In {
			placeholders[i] = b.param(v)
		}
		return b.colRef(f.Field) + " IN (" + strings.Join(placeholders, ", ") + ")", nil
	case filter.KindNotIn:
		placeholders := make([]string, len(f.In))
		for i, v := range f.In {
			placeholders[i] = b.param(v)
		}
		return b.colRef(f.Field) + " NOT IN (" + strings.Join(placeholders, ", ") + ")", nil
	case filter.KindBetween:
		return b.colRef(f.Field) + " BETWEEN " + b.param(f.Lo) + " AND " + b.param(f.Hi), nil
	default:
		return "", fmt.Errorf("unsupported filter kind %q in report SQL generator", f.Kind)
	}
}
