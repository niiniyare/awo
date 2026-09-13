// Package sqlbuild translates awo/filter predicates into parameterized
// PostgreSQL WHERE clauses. It is used exclusively by the pgx driver
// implementation and must not be imported by framework core packages.
//
// Design: the builder accumulates $N positional parameters compatible with
// pgx's simple query protocol. All values are passed as arguments, never
// interpolated into SQL strings (prevents SQL injection).
package sqlbuild

import (
	"fmt"
	"strings"

	"awo.so/awo/filter"
)

// Result holds the generated WHERE clause fragment and its bound values.
// The clause does not include the "WHERE" keyword — callers prepend it.
type Result struct {
	// Clause is the SQL fragment, e.g. "tenant_id = $1 AND status = $2".
	// Empty string means no predicates (match all rows).
	Clause string
	// Args are the positional parameter values corresponding to $1, $2, ...
	Args []any
}

// Builder constructs a parameterized WHERE clause from a filter tree.
// The offset parameter allows callers to start parameter numbering after
// existing query parameters (e.g. offset=2 produces $3, $4, ...).
type Builder struct {
	args   []any
	offset int
}

// New creates a Builder. offset is the number of parameters already bound
// before the WHERE clause (typically 0).
func New(offset int) *Builder {
	return &Builder{offset: offset}
}

// Build translates f into a WHERE clause fragment.
// f may be nil, in which case Result.Clause is empty.
func Build(f *filter.Filter, paramOffset int) (Result, error) {
	if f == nil {
		return Result{}, nil
	}
	b := New(paramOffset)
	clause, err := b.expr(f)
	if err != nil {
		return Result{}, err
	}
	return Result{Clause: clause, Args: b.args}, nil
}

// nextParam returns the next positional parameter token and advances the counter.
func (b *Builder) nextParam(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", b.offset+len(b.args))
}

// expr recursively produces the SQL expression for the given filter node.
func (b *Builder) expr(f *filter.Filter) (string, error) {
	if f == nil {
		return "TRUE", nil
	}
	switch f.Kind {
	case filter.KindEq:
		if f.Value == nil {
			return fmt.Sprintf("%s IS NULL", quoteIdent(f.Field)), nil
		}
		return fmt.Sprintf("%s = %s", quoteIdent(f.Field), b.nextParam(f.Value)), nil

	case filter.KindNeq:
		if f.Value == nil {
			return fmt.Sprintf("%s IS NOT NULL", quoteIdent(f.Field)), nil
		}
		return fmt.Sprintf("%s != %s", quoteIdent(f.Field), b.nextParam(f.Value)), nil

	case filter.KindGt:
		return fmt.Sprintf("%s > %s", quoteIdent(f.Field), b.nextParam(f.Value)), nil

	case filter.KindGte:
		return fmt.Sprintf("%s >= %s", quoteIdent(f.Field), b.nextParam(f.Value)), nil

	case filter.KindLt:
		return fmt.Sprintf("%s < %s", quoteIdent(f.Field), b.nextParam(f.Value)), nil

	case filter.KindLte:
		return fmt.Sprintf("%s <= %s", quoteIdent(f.Field), b.nextParam(f.Value)), nil

	case filter.KindBetween:
		lo := b.nextParam(f.Lo)
		hi := b.nextParam(f.Hi)
		return fmt.Sprintf("%s BETWEEN %s AND %s", quoteIdent(f.Field), lo, hi), nil

	case filter.KindIn:
		if len(f.In) == 0 {
			return "FALSE", nil
		}
		params := make([]string, len(f.In))
		for i, v := range f.In {
			params[i] = b.nextParam(v)
		}
		return fmt.Sprintf("%s = ANY(ARRAY[%s])", quoteIdent(f.Field), strings.Join(params, ",")), nil

	case filter.KindNotIn:
		if len(f.In) == 0 {
			return "TRUE", nil
		}
		params := make([]string, len(f.In))
		for i, v := range f.In {
			params[i] = b.nextParam(v)
		}
		return fmt.Sprintf("%s != ALL(ARRAY[%s])", quoteIdent(f.Field), strings.Join(params, ",")), nil

	case filter.KindIsNull:
		return fmt.Sprintf("%s IS NULL", quoteIdent(f.Field)), nil

	case filter.KindIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", quoteIdent(f.Field)), nil

	case filter.KindContains:
		// Case-insensitive substring (trigram index friendly).
		return fmt.Sprintf("%s ILIKE %s", quoteIdent(f.Field), b.nextParam("%"+escapeLike(fmt.Sprintf("%v", f.Value))+"%")), nil

	case filter.KindStartsWith:
		return fmt.Sprintf("%s ILIKE %s", quoteIdent(f.Field), b.nextParam(escapeLike(fmt.Sprintf("%v", f.Value))+"%")), nil

	case filter.KindEndsWith:
		return fmt.Sprintf("%s ILIKE %s", quoteIdent(f.Field), b.nextParam("%"+escapeLike(fmt.Sprintf("%v", f.Value)))), nil

	case filter.KindAnd:
		return b.combineLogical("AND", f.Sub)

	case filter.KindOr:
		return b.combineLogical("OR", f.Sub)

	case filter.KindNot:
		if len(f.Sub) != 1 {
			return "", fmt.Errorf("sqlbuild: NOT filter must have exactly one child, got %d", len(f.Sub))
		}
		inner, err := b.expr(f.Sub[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("NOT (%s)", inner), nil

	case filter.KindCustomEq:
		// Custom fields stored in jsonb column: data->>'field' = $N
		return fmt.Sprintf("data->>'%s' = %s", escapeSingleQuote(f.Field), b.nextParam(fmt.Sprintf("%v", f.Value))), nil

	case filter.KindCustomGt:
		return fmt.Sprintf("(data->>'%s')::numeric > %s", escapeSingleQuote(f.Field), b.nextParam(f.Value)), nil

	case filter.KindCustomLt:
		return fmt.Sprintf("(data->>'%s')::numeric < %s", escapeSingleQuote(f.Field), b.nextParam(f.Value)), nil

	case filter.KindCustomIn:
		if len(f.In) == 0 {
			return "FALSE", nil
		}
		params := make([]string, len(f.In))
		for i, v := range f.In {
			params[i] = b.nextParam(fmt.Sprintf("%v", v))
		}
		return fmt.Sprintf("data->>'%s' = ANY(ARRAY[%s])", escapeSingleQuote(f.Field), strings.Join(params, ",")), nil

	case filter.KindCustomNull:
		if v, _ := f.Value.(bool); v {
			return fmt.Sprintf("data->>'%s' IS NULL", escapeSingleQuote(f.Field)), nil
		}
		return fmt.Sprintf("data->>'%s' IS NOT NULL", escapeSingleQuote(f.Field)), nil

	default:
		return "", &TranslationError{
			Operator: string(f.Kind),
			Field:    f.Field,
			Value:    f.Value,
			Reason:   "unsupported filter kind",
		}
	}
}

func (b *Builder) combineLogical(op string, subs []*filter.Filter) (string, error) {
	if len(subs) == 0 {
		if op == "AND" {
			return "TRUE", nil
		}
		return "FALSE", nil
	}
	parts := make([]string, 0, len(subs))
	for _, sub := range subs {
		s, err := b.expr(sub)
		if err != nil {
			return "", err
		}
		parts = append(parts, "("+s+")")
	}
	return strings.Join(parts, " "+op+" "), nil
}

// quoteIdent wraps a PostgreSQL identifier in double-quotes.
// This prevents SQL injection via field names and handles reserved words.
// Only simple single-segment identifiers (column names) are handled here.
func quoteIdent(name string) string {
	// Escape any double-quotes inside the name by doubling them.
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`
}

// escapeLike escapes LIKE pattern metacharacters (%, _).
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// escapeSingleQuote escapes single-quotes for JSONB field name embedding.
func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
