package naming

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/cache"
)

// NamingSeriesService allocates unique, human-readable identifiers for entity
// records. Each call to Allocate:
//
//  1. Parses and renders the pattern prefix (all tokens except SEQ).
//  2. Atomically increments the Redis counter keyed by (tenant, prefix, period).
//  3. Renders the full string with the allocated sequence number.
//
// The service is safe for concurrent use. All counters are tenant-isolated.
//
// Rollback safety: Redis INCR is not transactional with PostgreSQL. When a
// database transaction rolls back, the sequence number is consumed but not
// returned. This produces gaps (e.g. INV-2026-000003 may not exist). Gaps are
// intentional and expected in document numbering systems; they are not bugs.
type NamingSeriesService struct {
	counter cache.Counter

	// mu protects the in-process token cache.
	mu     sync.RWMutex
	tokens map[string][]token // pattern → parsed tokens
}

// NewNamingSeriesService creates a NamingSeriesService backed by counter.
// counter must implement atomic Redis INCR semantics (see cache.Counter).
func NewNamingSeriesService(counter cache.Counter) *NamingSeriesService {
	return &NamingSeriesService{
		counter: counter,
		tokens:  make(map[string][]token),
	}
}

// AllocateInput carries the inputs for a single identifier allocation.
type AllocateInput struct {
	// TenantID is the owning tenant.
	TenantID uuid.UUID

	// Pattern is the naming-series pattern declared on the FieldDef.
	// Example: "INV-{YYYY}-{SEQ:6}"
	Pattern string

	// OrgCode is the organisation code for {ORG} tokens.
	// Empty when the record has no organisation context.
	OrgCode string

	// FiscalYearLabel overrides the {FY} token when set.
	FiscalYearLabel string

	// Now is the reference time for date tokens.
	// Defaults to UTC now when zero.
	Now time.Time
}

// Allocate atomically allocates the next identifier for the given input.
// Returns the rendered identifier string and the sequence number allocated.
func (s *NamingSeriesService) Allocate(ctx context.Context, in AllocateInput) (identifier string, seq int64, err error) {
	tokens, err := s.parsePattern(in.Pattern)
	if err != nil {
		return "", 0, fmt.Errorf("naming.Allocate: parse %q: %w", in.Pattern, err)
	}

	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	vars := PatternVars{
		Now:             now,
		OrgCode:         in.OrgCode,
		FiscalYearLabel: in.FiscalYearLabel,
		Sequence:        0, // placeholder; filled after counter increment
	}

	// Build the counter key from the pattern with SEQ rendered as empty,
	// so the key encodes only the stable prefix dimensions.
	prefixKey := renderWithoutSEQ(tokens, vars)
	counterKey := fmt.Sprintf("naming:counter:%s:%s:%04d%02d",
		in.TenantID.String(),
		prefixKey,
		now.Year(),
		int(now.Month()),
	)

	seq, err = s.counter.Increment(ctx, counterKey, 1)
	if err != nil {
		return "", 0, fmt.Errorf("naming.Allocate: increment counter: %w", err)
	}

	vars.Sequence = seq
	identifier = Render(tokens, vars)
	return identifier, seq, nil
}

// Preview returns a non-atomically-allocated sample identifier showing what
// the pattern will produce with seq=1 and the given time.
// Useful for UI display; never use the result as an actual identifier.
func (s *NamingSeriesService) Preview(pattern, orgCode string, now time.Time) (string, error) {
	return PreviewPattern(pattern, now, orgCode)
}

// Validate checks whether pattern is syntactically valid.
func (s *NamingSeriesService) Validate(pattern string) error {
	_, err := ParsePattern(pattern)
	return err
}

// parsePattern returns cached tokens or parses and caches the pattern.
func (s *NamingSeriesService) parsePattern(pattern string) ([]token, error) {
	s.mu.RLock()
	if t, ok := s.tokens[pattern]; ok {
		s.mu.RUnlock()
		return t, nil
	}
	s.mu.RUnlock()

	t, err := ParsePattern(pattern)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.tokens[pattern] = t
	s.mu.Unlock()

	return t, nil
}

// renderWithoutSEQ renders the pattern substituting SEQ tokens with an empty
// string to produce the stable counter key prefix.
func renderWithoutSEQ(tokens []token, vars PatternVars) string {
	noSeqVars := vars
	noSeqVars.Sequence = 0
	// Override SEQ tokens to produce empty during prefix rendering by using a
	// copy of tokens with SEQ replaced by literal empty string.
	stripped := make([]token, len(tokens))
	for i, t := range tokens {
		if t.kind == tokenSEQ {
			stripped[i] = token{kind: tokenLiteral, literal: ""}
		} else {
			stripped[i] = t
		}
	}
	return Render(stripped, noSeqVars)
}

// ── Pipeline integration ──────────────────────────────────────────────────────

// NamingFieldContext is passed by the pipeline to AllocateForRecord.
type NamingFieldContext struct {
	// FieldName is the entity field that holds the naming series value.
	FieldName string

	// Pattern is the pattern declared on the FieldDef.
	Pattern string

	// TenantID is the tenant owning the record.
	TenantID uuid.UUID

	// OrgCode is extracted from the record's organisation context.
	OrgCode string

	// FiscalYearLabel is extracted from the record's accounting period context.
	FiscalYearLabel string
}

// AllocateForRecord allocates a naming-series value for a single field on a
// record being created. Called by the pipeline's RunBeforeCreate stage for
// every FieldTypeNamingSeries field that has no value already set (allowing
// manual override when the actor has the required permission).
func (s *NamingSeriesService) AllocateForRecord(
	ctx context.Context,
	nfc NamingFieldContext,
) (string, error) {
	id, _, err := s.Allocate(ctx, AllocateInput{
		TenantID:        nfc.TenantID,
		Pattern:         nfc.Pattern,
		OrgCode:         nfc.OrgCode,
		FiscalYearLabel: nfc.FiscalYearLabel,
		Now:             time.Now().UTC(),
	})
	if err != nil {
		return "", fmt.Errorf("naming: allocate %q for field %q: %w",
			nfc.Pattern, nfc.FieldName, err)
	}
	return id, nil
}
