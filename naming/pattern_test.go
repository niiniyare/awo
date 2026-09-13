package naming

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixed reference time: 2026-07-15 (year=2026, month=7, day=15)
var refTime = time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

// --- ParsePattern + Render ---

func TestRender_LiteralOnly(t *testing.T) {
	tokens, err := ParsePattern("INVOICE")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime})
	assert.Equal(t, "INVOICE", got)
}

func TestRender_YYYY(t *testing.T) {
	tokens, err := ParsePattern("INV-{YYYY}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime})
	assert.Equal(t, "INV-2026", got)
}

func TestRender_YY(t *testing.T) {
	tokens, err := ParsePattern("{YY}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime})
	assert.Equal(t, "26", got)
}

func TestRender_MM(t *testing.T) {
	tokens, err := ParsePattern("{MM}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime})
	assert.Equal(t, "07", got)
}

func TestRender_DD(t *testing.T) {
	tokens, err := ParsePattern("{DD}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime})
	assert.Equal(t, "15", got)
}

func TestRender_YYYYMM(t *testing.T) {
	tokens, err := ParsePattern("{YYYYMM}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime})
	assert.Equal(t, "202607", got)
}

func TestRender_FY_Default(t *testing.T) {
	tokens, err := ParsePattern("{FY}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime})
	assert.Equal(t, "FY2026", got)
}

func TestRender_FY_Override(t *testing.T) {
	tokens, err := ParsePattern("{FY}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime, FiscalYearLabel: "FY2026/27"})
	assert.Equal(t, "FY2026/27", got)
}

func TestRender_ORG(t *testing.T) {
	tokens, err := ParsePattern("{ORG}-PO-{YYYY}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime, OrgCode: "ACME"})
	assert.Equal(t, "ACME-PO-2026", got)
}

func TestRender_ORG_Empty(t *testing.T) {
	tokens, err := ParsePattern("{ORG}-ITEM")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime, OrgCode: ""})
	assert.Equal(t, "-ITEM", got)
}

func TestRender_SEQ_Named(t *testing.T) {
	tokens, err := ParsePattern("INV-{YYYY}-{SEQ:6}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime, Sequence: 42})
	assert.Equal(t, "INV-2026-000042", got)
}

func TestRender_SEQ_Zeros(t *testing.T) {
	// {000000} = all-zero placeholder = SEQ:6
	tokens, err := ParsePattern("PAY-{000000}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime, Sequence: 1})
	assert.Equal(t, "PAY-000001", got)
}

func TestRender_SEQ_Width5(t *testing.T) {
	tokens, err := ParsePattern("JE-{SEQ:5}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime, Sequence: 999})
	assert.Equal(t, "JE-00999", got)
}

func TestRender_FullPattern(t *testing.T) {
	// Finance module canonical pattern: INV-{YYYY}-{SEQ:6}
	tokens, err := ParsePattern("INV-{YYYY}-{SEQ:6}")
	require.NoError(t, err)
	got := Render(tokens, PatternVars{Now: refTime, Sequence: 1})
	assert.Equal(t, "INV-2026-000001", got)
}

// --- ParsePattern error cases ---

func TestParsePattern_UnknownPlaceholder(t *testing.T) {
	_, err := ParsePattern("{BOGUS}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BOGUS")
}

func TestParsePattern_UnclosedBrace(t *testing.T) {
	_, err := ParsePattern("INV-{YYYY")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed")
}

func TestParsePattern_SEQ_InvalidWidth(t *testing.T) {
	_, err := ParsePattern("{SEQ:abc}")
	require.Error(t, err)
}

func TestParsePattern_SEQ_ZeroWidth(t *testing.T) {
	// {SEQ:0} is out of range [1,20]
	_, err := ParsePattern("{SEQ:0}")
	require.Error(t, err)
}

func TestParsePattern_SEQ_TooLarge(t *testing.T) {
	_, err := ParsePattern("{SEQ:21}")
	require.Error(t, err)
}

// --- HasSEQ ---

func TestHasSEQ_True(t *testing.T) {
	tokens, err := ParsePattern("INV-{YYYY}-{SEQ:6}")
	require.NoError(t, err)
	assert.True(t, HasSEQ(tokens))
}

func TestHasSEQ_False(t *testing.T) {
	tokens, err := ParsePattern("INV-{YYYY}")
	require.NoError(t, err)
	assert.False(t, HasSEQ(tokens))
}

// --- PreviewPattern ---

func TestPreviewPattern_OK(t *testing.T) {
	preview, err := PreviewPattern("INV-{YYYY}-{SEQ:6}", refTime, "")
	require.NoError(t, err)
	assert.Equal(t, "INV-2026-000001", preview)
}

func TestPreviewPattern_Error(t *testing.T) {
	_, err := PreviewPattern("{INVALID}", refTime, "")
	require.Error(t, err)
}

// --- CounterKey ---

func TestCounterKey_Format(t *testing.T) {
	key := CounterKey("tenant-abc", "", "INV-2026-", refTime)
	assert.Contains(t, key, "naming:counter:")
	assert.Contains(t, key, "tenant-abc")
	assert.Contains(t, key, "202607")
}

func TestCounterKey_Deterministic(t *testing.T) {
	k1 := CounterKey("t1", "", "INV-", refTime)
	k2 := CounterKey("t1", "", "INV-", refTime)
	assert.Equal(t, k1, k2)
}

func TestCounterKey_DiffersByTenant(t *testing.T) {
	k1 := CounterKey("tenant-a", "", "INV-", refTime)
	k2 := CounterKey("tenant-b", "", "INV-", refTime)
	assert.NotEqual(t, k1, k2)
}

func TestCounterKey_DiffersByMonth(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	k1 := CounterKey("t", "", "INV-", t1)
	k2 := CounterKey("t", "", "INV-", t2)
	assert.NotEqual(t, k1, k2)
}
