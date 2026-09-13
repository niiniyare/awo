package compiler

import (
	"fmt"
	"strings"
)

// Severity of a Diagnostic.
type Severity int

const (
	SeverityInfo    Severity = iota
	SeverityWarning Severity = iota
	SeverityError   Severity = iota
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Diagnostic is a validation message produced during compilation.
type Diagnostic struct {
	Severity   Severity
	EntityName string
	FieldName  string // empty if entity-level
	Message    string
}

func (d Diagnostic) String() string {
	if d.FieldName != "" {
		return fmt.Sprintf("[%s] %s.%s: %s", d.Severity, d.EntityName, d.FieldName, d.Message)
	}
	return fmt.Sprintf("[%s] %s: %s", d.Severity, d.EntityName, d.Message)
}

func (d Diagnostic) IsError() bool { return d.Severity == SeverityError }

// Diagnostics is a slice of Diagnostic with helpers.
type Diagnostics []Diagnostic

func (ds Diagnostics) HasErrors() bool {
	for _, d := range ds {
		if d.IsError() {
			return true
		}
	}
	return false
}

func (ds Diagnostics) Errors() []Diagnostic {
	var out []Diagnostic
	for _, d := range ds {
		if d.Severity == SeverityError {
			out = append(out, d)
		}
	}
	return out
}

func (ds Diagnostics) Warnings() []Diagnostic {
	var out []Diagnostic
	for _, d := range ds {
		if d.Severity == SeverityWarning {
			out = append(out, d)
		}
	}
	return out
}

func (ds Diagnostics) String() string {
	if len(ds) == 0 {
		return "no diagnostics"
	}
	var sb strings.Builder
	for _, d := range ds {
		sb.WriteString(d.String())
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

// AsError returns a non-nil error if any error-severity diagnostic exists.
func (ds Diagnostics) AsError() error {
	if !ds.HasErrors() {
		return nil
	}
	var msgs []string
	for _, d := range ds.Errors() {
		msgs = append(msgs, d.String())
	}
	return fmt.Errorf("schema compilation errors:\n%s", strings.Join(msgs, "\n"))
}
