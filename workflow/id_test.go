package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildID_Format(t *testing.T) {
	id := BuildID("tenant-1", "finance_invoice", "rec-99", "on_submit")
	assert.Equal(t, "tenant-1.finance_invoice.rec-99.on_submit", id)
}

func TestParseID_RoundTrip(t *testing.T) {
	tenant, entity, record, event := "t1", "mod_item", "r2", "on_create"
	id := BuildID(tenant, entity, record, event)

	gotTenant, gotEntity, gotRecord, gotEvent, err := ParseID(id)
	require.NoError(t, err)
	assert.Equal(t, tenant, gotTenant)
	assert.Equal(t, entity, gotEntity)
	assert.Equal(t, record, gotRecord)
	assert.Equal(t, event, gotEvent)
}

func TestParseID_TooFewSegments(t *testing.T) {
	_, _, _, _, err := ParseID("only.three.parts")
	assert.Error(t, err)
}

func TestParseID_EventWithDot(t *testing.T) {
	// SplitN(4) keeps the event segment intact even if it contains dots.
	id := BuildID("t1", "mod_item", "r1", "on.complex.event")
	_, _, _, gotEvent, err := ParseID(id)
	require.NoError(t, err)
	assert.Equal(t, "on.complex.event", gotEvent)
}
