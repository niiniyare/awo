// Package filterparse translates HTTP query parameters into filter.Filter trees.
//
// Query parameter format (applied as AND across all params):
//
//	?filter[status][eq]=active
//	?filter[amount][gte]=100
//	?filter[name][contains]=acme
//	?filter[due_date][between]=2026-01-01,2026-12-31
//	?filter[id][in]=uuid1,uuid2,uuid3
//
// Operators: eq, neq, gt, gte, lt, lte, between, in, not_in,
//
//	is_null, is_not_null, contains, starts_with, ends_with
//
// All filters are AND-combined. Use nested JSON body for complex OR/NOT trees.
// URL filter params are intentionally simple — complex queries belong in POST
// request bodies with explicit filter DSL.
package filterparse

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"awo.so/awo/filter"
)

// FromQuery extracts filter parameters from a Fiber request context.
// Returns nil if no filter params are present.
// Returns an error if a param has an unrecognised format.
func FromQuery(c *fiber.Ctx) (*filter.Filter, error) {
	args := c.Context().QueryArgs()
	var predicates []*filter.Filter

	args.VisitAll(func(key, val []byte) {
		k := string(key)
		v := string(val)
		// Expect format: filter[field][op]
		if !strings.HasPrefix(k, "filter[") {
			return
		}
		// Parse filter[field][op]
		rest := k[len("filter["):]
		brackClose := strings.Index(rest, "]")
		if brackClose < 0 {
			return
		}
		field := rest[:brackClose]
		rest = rest[brackClose+1:]
		if len(rest) < 3 || rest[0] != '[' {
			return
		}
		op := strings.TrimSuffix(rest[1:], "]")
		f := buildFilter(field, op, v)
		if f != nil {
			predicates = append(predicates, f)
		}
	})

	if len(predicates) == 0 {
		return nil, nil
	}
	return filter.And(predicates...), nil
}

// ParseQueryString parses a raw URL query string (without the leading "?")
// and returns the combined AND filter. Returns nil if no filter params are
// found. This is the testable core extracted from [FromQuery].
//
//	f, err := filterparse.ParseQueryString("filter[status][eq]=active&filter[amount][gte]=100")
func ParseQueryString(qs string) (*filter.Filter, error) {
	var predicates []*filter.Filter
	for _, part := range strings.Split(qs, "&") {
		if part == "" {
			continue
		}
		eqIdx := strings.IndexByte(part, '=')
		if eqIdx < 0 {
			continue
		}
		k := part[:eqIdx]
		v := part[eqIdx+1:]
		if !strings.HasPrefix(k, "filter[") {
			continue
		}
		rest := k[len("filter["):]
		brackClose := strings.Index(rest, "]")
		if brackClose < 0 {
			continue
		}
		field := rest[:brackClose]
		rest = rest[brackClose+1:]
		if len(rest) < 3 || rest[0] != '[' {
			continue
		}
		op := strings.TrimSuffix(rest[1:], "]")
		f := buildFilter(field, op, v)
		if f != nil {
			predicates = append(predicates, f)
		}
	}
	if len(predicates) == 0 {
		return nil, nil
	}
	return filter.And(predicates...), nil
}

func buildFilter(field, op, value string) *filter.Filter {
	switch op {
	case "eq":
		return filter.Eq(field, value)
	case "neq":
		return filter.Neq(field, value)
	case "gt":
		return filter.Gt(field, value)
	case "gte":
		return filter.Gte(field, value)
	case "lt":
		return filter.Lt(field, value)
	case "lte":
		return filter.Lte(field, value)
	case "contains":
		return filter.Contains(field, value)
	case "starts_with":
		return filter.StartsWith(field, value)
	case "ends_with":
		return filter.EndsWith(field, value)
	case "is_null":
		return filter.IsNull(field)
	case "is_not_null":
		return filter.IsNotNull(field)
	case "in":
		parts := strings.Split(value, ",")
		vals := make([]any, len(parts))
		for i, p := range parts {
			vals[i] = strings.TrimSpace(p)
		}
		return filter.In(field, vals...)
	case "not_in":
		parts := strings.Split(value, ",")
		vals := make([]any, len(parts))
		for i, p := range parts {
			vals[i] = strings.TrimSpace(p)
		}
		return filter.NotIn(field, vals...)
	case "between":
		parts := strings.SplitN(value, ",", 2)
		if len(parts) != 2 {
			return nil
		}
		return filter.Between(field, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	default:
		_ = fmt.Sprintf("filterparse: unknown op %q for field %q", op, field)
		return nil
	}
}
