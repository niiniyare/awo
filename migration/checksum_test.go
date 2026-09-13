package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyChecksums_AllMatch(t *testing.T) {
	steps := []Step{
		{Version: 1, Direction: DirectionUp, Checksum: "aaa"},
		{Version: 2, Direction: DirectionUp, Checksum: "bbb"},
	}
	applied := map[uint64]string{1: "aaa", 2: "bbb"}
	require.NoError(t, VerifyChecksums(steps, applied))
}

func TestVerifyChecksums_Mismatch(t *testing.T) {
	steps := []Step{
		{Version: 1, Description: "create_users", Direction: DirectionUp, Checksum: "new"},
	}
	applied := map[uint64]string{1: "old"}
	assert.Error(t, VerifyChecksums(steps, applied))
}

func TestVerifyChecksums_SkipsUnapplied(t *testing.T) {
	steps := []Step{
		{Version: 1, Direction: DirectionUp, Checksum: "aaa"},
		{Version: 2, Direction: DirectionUp, Checksum: "bbb"},
	}
	applied := map[uint64]string{1: "aaa"}
	require.NoError(t, VerifyChecksums(steps, applied), "unapplied step must not be verified")
}

func TestVerifyChecksums_SkipsDownSteps(t *testing.T) {
	steps := []Step{
		{Version: 1, Direction: DirectionDown, Checksum: "totally-different"},
	}
	applied := map[uint64]string{1: "original"}
	require.NoError(t, VerifyChecksums(steps, applied), "down steps should be skipped entirely")
}

func TestVerifyChecksums_Empty(t *testing.T) {
	require.NoError(t, VerifyChecksums(nil, nil))
}
