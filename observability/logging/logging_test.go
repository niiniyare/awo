package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/observability/logging"
)

func TestNew_WritesJSON(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, slog.LevelInfo)
	log.Info("created", "entity", "invoice")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "created", entry["msg"])
	assert.Equal(t, "invoice", entry["entity"])
}

func TestWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, slog.LevelInfo).WithRequestID("req-123")
	log.Info("ok")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "req-123", entry["request_id"])
}

func TestWithTenantID(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, slog.LevelInfo).WithTenantID("tenant-abc")
	log.Info("ok")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "tenant-abc", entry["tenant_id"])
}

func TestWithContext_FromContext(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, slog.LevelInfo).WithUserID("user-42")
	ctx := log.WithContext(context.Background())

	got := logging.FromContext(ctx)
	got.Info("retrieved")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "user-42", entry["user_id"])
}

func TestFromContext_NoLogger_ReturnsNop(t *testing.T) {
	ctx := context.Background()
	log := logging.FromContext(ctx)
	// Nop logger must not panic.
	log.Info("nop", "k", "v")
}

func TestCorrelationChain(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, slog.LevelInfo).
		WithRequestID("r1").
		WithTenantID("t1").
		WithUserID("u1").
		WithWorkflowID("w1").
		WithTraceID("tr1")
	log.Info("full correlation")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "r1", entry["request_id"])
	assert.Equal(t, "t1", entry["tenant_id"])
	assert.Equal(t, "u1", entry["user_id"])
	assert.Equal(t, "w1", entry["workflow_id"])
	assert.Equal(t, "tr1", entry["trace_id"])
}

func TestWithStr(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, slog.LevelInfo).WithStr("module", "finance")
	log.Info("ok")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "finance", entry["module"])
}

func TestDebugLevelFiltered(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, slog.LevelInfo) // only INFO+
	log.Debug("should not appear")
	assert.Empty(t, buf.String())
}
