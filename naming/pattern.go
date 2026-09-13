// Package naming implements the NamingSeries service: atomic, tenant-aware,
// organisation-aware sequence generation for entity identifiers.
//
// Pattern syntax:
//
//	INV-{YYYY}-{MM}-{SEQ:6}       → INV-2026-07-000001
//	PAY-{YYYYMM}-{000001}         → PAY-202607-000001
//	JE-{FY}-{SEQ:5}               → JE-FY2026-00001
//	{ORG}-PO-{YYYY}-{SEQ:4}       → ACME-PO-2026-0001
//
// Supported tokens:
//
//	{YYYY}        four-digit year
//	{YY}          two-digit year
//	{MM}          two-digit month (01-12)
//	{DD}          two-digit day (01-31)
//	{YYYYMM}      year + month with no separator
//	{FY}          fiscal year label: "FY{YYYY}" or "FY{YYYY}/{YY+1}" depending on config
//	{ORG}         organisation code from context (empty string when not set)
//	{SEQ:N}       zero-padded sequence of width N (e.g. {SEQ:6} → 000001)
//	{000...0}     same as SEQ; width = number of zeros (e.g. {000001} = SEQ:6)
package naming

import (
	"fmt"
	"strings"
	"time"
)

// Token kinds emitted by the pattern lexer.
type tokenKind int

const (
	tokenLiteral tokenKind = iota
	tokenYYYY
	tokenYY
	tokenMM
	tokenDD
	tokenYYYYMM
	tokenFY
	tokenORG
	tokenSEQ
)

// token is a single parsed element of a naming-series pattern.
type token struct {
	kind     tokenKind
	literal  string // tokenLiteral: the literal text
	seqWidth int    // tokenSEQ: zero-padding width
}

// ParsePattern parses a naming-series pattern string into tokens.
// Returns an error when the pattern contains unrecognised or malformed tokens.
func ParsePattern(pattern string) ([]token, error) {
	var tokens []token
	remaining := pattern

	for len(remaining) > 0 {
		open := strings.Index(remaining, "{")
		if open < 0 {
			tokens = append(tokens, token{kind: tokenLiteral, literal: remaining})
			break
		}
		if open > 0 {
			tokens = append(tokens, token{kind: tokenLiteral, literal: remaining[:open]})
		}
		remaining = remaining[open:]

		close := strings.Index(remaining, "}")
		if close < 0 {
			return nil, fmt.Errorf("naming: unclosed '{' in pattern %q", pattern)
		}

		placeholder := remaining[1:close] // contents between { }
		remaining = remaining[close+1:]

		t, err := parsePlaceholder(placeholder)
		if err != nil {
			return nil, fmt.Errorf("naming: pattern %q: %w", pattern, err)
		}
		tokens = append(tokens, t)
	}

	return tokens, nil
}

func parsePlaceholder(s string) (token, error) {
	switch {
	case s == "YYYY":
		return token{kind: tokenYYYY}, nil
	case s == "YY":
		return token{kind: tokenYY}, nil
	case s == "MM":
		return token{kind: tokenMM}, nil
	case s == "DD":
		return token{kind: tokenDD}, nil
	case s == "YYYYMM":
		return token{kind: tokenYYYYMM}, nil
	case s == "FY":
		return token{kind: tokenFY}, nil
	case s == "ORG":
		return token{kind: tokenORG}, nil
	case strings.HasPrefix(s, "SEQ:"):
		width, err := parseWidth(s[4:])
		if err != nil {
			return token{}, fmt.Errorf("invalid SEQ width %q: %w", s[4:], err)
		}
		return token{kind: tokenSEQ, seqWidth: width}, nil
	case isAllZeros(s):
		return token{kind: tokenSEQ, seqWidth: len(s)}, nil
	default:
		return token{}, fmt.Errorf("unknown placeholder {%s}", s)
	}
}

func parseWidth(s string) (int, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty width")
	}
	w := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-numeric width %q", s)
		}
		w = w*10 + int(c-'0')
	}
	if w < 1 || w > 20 {
		return 0, fmt.Errorf("width %d out of range [1,20]", w)
	}
	return w, nil
}

func isAllZeros(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c != '0' {
			return false
		}
	}
	return true
}

// PatternVars carries the variable values substituted into pattern tokens.
type PatternVars struct {
	// Now is the timestamp used for date tokens. Defaults to UTC now when zero.
	Now time.Time

	// OrgCode is the organisation code injected into {ORG} tokens.
	// Empty string when the context has no organisation.
	OrgCode string

	// Sequence is the allocated sequence number injected into {SEQ:N} tokens.
	// Must be set before calling Render when the pattern contains a SEQ token.
	Sequence int64

	// FiscalYearLabel overrides the computed FY label when set.
	FiscalYearLabel string
}

// Render substitutes vars into the parsed tokens and returns the final string.
func Render(tokens []token, vars PatternVars) string {
	now := vars.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var sb strings.Builder
	for _, t := range tokens {
		switch t.kind {
		case tokenLiteral:
			sb.WriteString(t.literal)
		case tokenYYYY:
			fmt.Fprintf(&sb, "%04d", now.Year())
		case tokenYY:
			fmt.Fprintf(&sb, "%02d", now.Year()%100)
		case tokenMM:
			fmt.Fprintf(&sb, "%02d", int(now.Month()))
		case tokenDD:
			fmt.Fprintf(&sb, "%02d", now.Day())
		case tokenYYYYMM:
			fmt.Fprintf(&sb, "%04d%02d", now.Year(), int(now.Month()))
		case tokenFY:
			if vars.FiscalYearLabel != "" {
				sb.WriteString(vars.FiscalYearLabel)
			} else {
				fmt.Fprintf(&sb, "FY%04d", now.Year())
			}
		case tokenORG:
			sb.WriteString(vars.OrgCode)
		case tokenSEQ:
			w := t.seqWidth
			if w < 1 {
				w = 6
			}
			fmt.Fprintf(&sb, "%0*d", w, vars.Sequence)
		}
	}
	return sb.String()
}

// CounterKey derives the Redis counter key for a given pattern and scope.
// The key encodes all dimensions that define a unique sequence:
// tenant, the fixed (non-SEQ) parts of the rendered pattern, and the period.
//
// Example: naming:counter:tenant-uuid:INV-2026-07: → incremented each invocation.
func CounterKey(tenantID, orgCode, patternKey string, now time.Time) string {
	// patternKey is the rendered prefix (all tokens except SEQ rendered with seq=0)
	return fmt.Sprintf("naming:counter:%s:%s:%s:",
		tenantID,
		patternKey,
		fmt.Sprintf("%04d%02d", now.Year(), int(now.Month())),
	)
}

// HasSEQ reports whether the parsed token list contains a sequence token.
func HasSEQ(tokens []token) bool {
	for _, t := range tokens {
		if t.kind == tokenSEQ {
			return true
		}
	}
	return false
}

// PreviewPattern renders the pattern with a placeholder sequence (e.g. 1) and
// the given time. Useful for showing the user what a series looks like.
func PreviewPattern(pattern string, now time.Time, orgCode string) (string, error) {
	tokens, err := ParsePattern(pattern)
	if err != nil {
		return "", err
	}
	return Render(tokens, PatternVars{
		Now:      now,
		OrgCode:  orgCode,
		Sequence: 1,
	}), nil
}
