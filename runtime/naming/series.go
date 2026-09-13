// Package naming implements the NamingSeries field type resolver.
//
// Format: "INV-{YYYY}-{SEQ:5}" where:
//   - {YYYY}  → 4-digit year
//   - {MM}    → 2-digit month
//   - {DD}    → 2-digit day
//   - {SEQ:N} → zero-padded atomic sequence, N digits wide
//
// Sequences are per-tenant per-entity per-field per-period (year by default).
// The period resets automatically based on the format string.
// Counters are stored in Redis (atomic INCR) with the database as backup.
//
// TenantOverridable: when true, tenants can replace the prefix portion of the
// format string via the Settings module without a code change.
package naming

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/cache"
)

// tokenRe matches format tokens like {YYYY}, {MM}, {DD}, {SEQ:5}.
var tokenRe = regexp.MustCompile(`\{([^}]+)\}`)

// Generator resolves NamingSeries format strings to sequential values.
type Generator struct {
	counter cache.Counter
}

// New creates a Generator backed by the given counter.
func New(counter cache.Counter) *Generator {
	return &Generator{counter: counter}
}

// Next generates the next value in the series for the given entity, field,
// and tenant. The format string is resolved using the current time and an
// atomic sequence counter.
//
// Example: format="INV-{YYYY}-{SEQ:5}", tenant=abc → "INV-2026-00001"
func (g *Generator) Next(ctx context.Context, format string, entityName, fieldName string, tenantID uuid.UUID) (string, error) {
	now := time.Now().UTC()

	// Extract the period key (year by default; year+month if {MM} present).
	period := periodKey(format, now)
	counterKey := fmt.Sprintf("seq:%s:%s:%s:%s", tenantID, entityName, fieldName, period)

	// Resolve all tokens.
	var resolveErr error
	result := tokenRe.ReplaceAllStringFunc(format, func(token string) string {
		if resolveErr != nil {
			return token
		}
		inner := token[1 : len(token)-1] // strip { }
		switch {
		case inner == "YYYY":
			return fmt.Sprintf("%04d", now.Year())
		case inner == "YY":
			return fmt.Sprintf("%02d", now.Year()%100)
		case inner == "MM":
			return fmt.Sprintf("%02d", int(now.Month()))
		case inner == "DD":
			return fmt.Sprintf("%02d", now.Day())
		case strings.HasPrefix(inner, "SEQ:"):
			widthStr := strings.TrimPrefix(inner, "SEQ:")
			var width int
			fmt.Sscanf(widthStr, "%d", &width)
			if width < 1 {
				width = 5
			}
			n, err := g.counter.Increment(ctx, counterKey, 1)
			if err != nil {
				resolveErr = fmt.Errorf("naming.Next: increment counter %q: %w", counterKey, err)
				return token
			}
			return fmt.Sprintf("%0*d", width, n)
		default:
			return token // unknown token — leave as-is
		}
	})

	if resolveErr != nil {
		return "", resolveErr
	}
	return result, nil
}

// periodKey returns the cache key segment representing the reset period.
// If format contains {MM}, period resets monthly; otherwise annually.
func periodKey(format string, t time.Time) string {
	if strings.Contains(format, "{MM}") {
		return fmt.Sprintf("%d-%02d", t.Year(), t.Month())
	}
	return fmt.Sprintf("%d", t.Year())
}
