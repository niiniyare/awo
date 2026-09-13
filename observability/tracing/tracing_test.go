package tracing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/observability/tracing"
)

func TestNewProvider_Noop(t *testing.T) {
	ctx := context.Background()
	p, err := tracing.NewProvider(ctx, tracing.Config{
		ServiceName: "test",
		Exporter:    tracing.ExporterNoop,
	})
	require.NoError(t, err)
	assert.NotNil(t, p)
	require.NoError(t, p.Shutdown(ctx))
}

func TestNewProvider_Stdout(t *testing.T) {
	ctx := context.Background()
	p, err := tracing.NewProvider(ctx, tracing.Config{
		ServiceName: "test",
		Exporter:    tracing.ExporterStdout,
	})
	require.NoError(t, err)
	require.NoError(t, p.Shutdown(ctx))
}

func TestStart_CreatesSpan(t *testing.T) {
	ctx := context.Background()
	p, err := tracing.NewProvider(ctx, tracing.Config{
		ServiceName: "test",
		Exporter:    tracing.ExporterNoop,
	})
	require.NoError(t, err)
	defer p.Shutdown(ctx)

	ctx2, span := tracing.Start(ctx, "test.operation")
	assert.NotNil(t, span)
	assert.NotNil(t, ctx2)
	span.End()
}

func TestRecordError_Nil(t *testing.T) {
	ctx := context.Background()
	p, _ := tracing.NewProvider(ctx, tracing.Config{Exporter: tracing.ExporterNoop})
	defer p.Shutdown(ctx)

	_, span := tracing.Start(ctx, "op")
	tracing.RecordError(span, nil) // must not panic
	span.End()
}

func TestEnd_WithError(t *testing.T) {
	ctx := context.Background()
	p, _ := tracing.NewProvider(ctx, tracing.Config{Exporter: tracing.ExporterNoop})
	defer p.Shutdown(ctx)

	var err error
	_, span := tracing.Start(ctx, "op")
	err = assert.AnError
	tracing.End(span, &err) // records error, ends span
}

func TestAttributes(t *testing.T) {
	assert.Equal(t, "awo.entity", string(tracing.EntityName("invoice").Key))
	assert.Equal(t, "awo.tenant_id", string(tracing.TenantID("t1").Key))
	assert.Equal(t, "awo.user_id", string(tracing.UserID("u1").Key))
	assert.Equal(t, "awo.workflow_id", string(tracing.WorkflowID("w1").Key))
	assert.Equal(t, "awo.operation", string(tracing.Operation("create").Key))
}

func TestNewProvider_DefaultSampleRate(t *testing.T) {
	ctx := context.Background()
	p, err := tracing.NewProvider(ctx, tracing.Config{
		ServiceName: "test",
		// SampleRate not set — should default to 1.0
	})
	require.NoError(t, err)
	require.NoError(t, p.Shutdown(ctx))
}
