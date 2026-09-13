// Package logging provides the framework-wide structured logger.
//
// The logger is built on log/slog and adds standard correlation fields that
// propagate through every request, workflow, and migration operation:
//
//   - request_id  — HTTP request UUID
//   - tenant_id   — authenticated tenant UUID
//   - user_id     — authenticated user UUID
//   - workflow_id — Temporal workflow ID (when applicable)
//   - trace_id    — OpenTelemetry trace ID (when applicable)
//
// # Usage
//
// Retrieve the logger from context in any layer:
//
//	log := logging.FromContext(ctx)
//	log.Info("invoice created", "entity", "invoice", "id", id)
//
// The logger is safe for concurrent use. Each With* call returns a new
// logger instance — the parent logger is not modified.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// Logger wraps slog.Logger and adds framework-specific correlation helpers.
type Logger struct {
	sl *slog.Logger
}

// New creates a Logger that writes JSON to w at the given level.
// In production, w is os.Stderr.
func New(w io.Writer, level slog.Level) *Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return &Logger{sl: slog.New(h)}
}

// NewDev creates a Logger with human-readable text output to stderr.
// Use in development and local testing only.
func NewDev() *Logger {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	return &Logger{sl: slog.New(h)}
}

// Slog returns the underlying *slog.Logger for interoperability with
// libraries that require it directly.
func (l *Logger) Slog() *slog.Logger { return l.sl }

// WithRequestID returns a new Logger with request_id set.
func (l *Logger) WithRequestID(id string) *Logger {
	return &Logger{sl: l.sl.With("request_id", id)}
}

// WithTenantID returns a new Logger with tenant_id set.
func (l *Logger) WithTenantID(id string) *Logger {
	return &Logger{sl: l.sl.With("tenant_id", id)}
}

// WithUserID returns a new Logger with user_id set.
func (l *Logger) WithUserID(id string) *Logger {
	return &Logger{sl: l.sl.With("user_id", id)}
}

// WithWorkflowID returns a new Logger with workflow_id set.
func (l *Logger) WithWorkflowID(id string) *Logger {
	return &Logger{sl: l.sl.With("workflow_id", id)}
}

// WithTraceID returns a new Logger with trace_id set.
func (l *Logger) WithTraceID(id string) *Logger {
	return &Logger{sl: l.sl.With("trace_id", id)}
}

// WithStr returns a new Logger with an additional string field.
func (l *Logger) WithStr(key, value string) *Logger {
	return &Logger{sl: l.sl.With(key, value)}
}

// WithErr returns a new Logger with an error field.
func (l *Logger) WithErr(err error) *Logger {
	return &Logger{sl: l.sl.With("error", err)}
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) { l.sl.Debug(msg, args...) }

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) { l.sl.Info(msg, args...) }

// Warn logs at warn level.
func (l *Logger) Warn(msg string, args ...any) { l.sl.Warn(msg, args...) }

// Error logs at error level.
func (l *Logger) Error(msg string, args ...any) { l.sl.Error(msg, args...) }

// WithContext stores the logger in ctx and returns the new context.
// Retrieve it in any package with [FromContext].
func (l *Logger) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the Logger stored by [Logger.WithContext].
// If no logger is stored, returns a no-op logger that discards all output.
// Callers never need to nil-check the returned logger.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(contextKey{}).(*Logger); ok && l != nil {
		return l
	}
	return nop()
}

// nop returns a logger that discards all output.
func nop() *Logger {
	return &Logger{sl: slog.New(noopHandler{})}
}

// noopHandler discards all log records.
type noopHandler struct{}

func (noopHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (noopHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (h noopHandler) WithAttrs(_ []slog.Attr) slog.Handler        { return h }
func (h noopHandler) WithGroup(_ string) slog.Handler             { return h }

// Global is a package-level logger used by framework internals before a
// context is established (e.g. startup, migration). Replace in main():
//
//	logging.Global = logging.New(os.Stderr, slog.LevelInfo)
var Global = NewDev()
