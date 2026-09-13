package audit

import (
	"context"
	"testing"
)

func TestWithRequestContext_RoundTrip(t *testing.T) {
	t.Parallel()

	rc := RequestContext{
		RequestID: "req-abc",
		IPAddress: "203.0.113.1",
		SessionID: "deadbeef",
	}
	ctx := WithRequestContext(context.Background(), rc)

	got, ok := RequestContextFromContext(ctx)
	if !ok {
		t.Fatal("RequestContextFromContext: expected ok=true")
	}
	if got.RequestID != rc.RequestID {
		t.Errorf("RequestID: got %q, want %q", got.RequestID, rc.RequestID)
	}
	if got.IPAddress != rc.IPAddress {
		t.Errorf("IPAddress: got %q, want %q", got.IPAddress, rc.IPAddress)
	}
	if got.SessionID != rc.SessionID {
		t.Errorf("SessionID: got %q, want %q", got.SessionID, rc.SessionID)
	}
}

func TestRequestContextFromContext_Missing(t *testing.T) {
	t.Parallel()

	_, ok := RequestContextFromContext(context.Background())
	if ok {
		t.Error("RequestContextFromContext on plain context: expected ok=false")
	}
}
