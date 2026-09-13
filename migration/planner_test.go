package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func upStep(version uint64, desc string) Step {
	return Step{Version: version, Description: desc, Direction: DirectionUp, SQL: "SELECT 1;", Checksum: "abc"}
}

func downStep(version uint64, desc string) Step {
	return Step{Version: version, Description: desc, Direction: DirectionDown, SQL: "SELECT 2;", Checksum: "def"}
}

var testSteps = []Step{
	upStep(1, "create_users"),
	downStep(1, "create_users"),
	upStep(2, "add_email"),
	downStep(2, "add_email"),
	upStep(3, "add_index"),
	downStep(3, "add_index"),
}

func TestBuildPlan_UpFromZero(t *testing.T) {
	plan, err := BuildPlan(testSteps, 0, DirectionUp, 0)
	require.NoError(t, err)
	assert.Len(t, plan.Steps, 3)
	assert.Equal(t, uint64(0), plan.From)
	assert.Equal(t, uint64(3), plan.To)
}

func TestBuildPlan_UpPartial(t *testing.T) {
	plan, err := BuildPlan(testSteps, 1, DirectionUp, 0)
	require.NoError(t, err)
	assert.Len(t, plan.Steps, 2, "only versions 2 and 3 should be selected")
}

func TestBuildPlan_UpToTarget(t *testing.T) {
	plan, err := BuildPlan(testSteps, 0, DirectionUp, 2)
	require.NoError(t, err)
	assert.Len(t, plan.Steps, 2, "only versions 1 and 2 should be selected")
	assert.Equal(t, uint64(2), plan.To)
}

func TestBuildPlan_UpAlreadyCurrent(t *testing.T) {
	plan, err := BuildPlan(testSteps, 3, DirectionUp, 0)
	require.NoError(t, err)
	assert.Empty(t, plan.Steps, "already at latest — no steps needed")
}

func TestBuildPlan_DownAll(t *testing.T) {
	plan, err := BuildPlan(testSteps, 3, DirectionDown, 0)
	require.NoError(t, err)
	assert.Len(t, plan.Steps, 3)
	// Steps must be in descending order: 3, 2, 1.
	for i, s := range plan.Steps {
		assert.Equal(t, uint64(3-i), s.Version, "step[%d] should be version %d", i, 3-i)
	}
}

func TestBuildPlan_DownToTarget(t *testing.T) {
	plan, err := BuildPlan(testSteps, 3, DirectionDown, 1)
	require.NoError(t, err)
	assert.Len(t, plan.Steps, 2, "should roll back versions 3 and 2 only")
}

func TestBuildPlan_DownTargetAboveCurrent_Error(t *testing.T) {
	_, err := BuildPlan(testSteps, 2, DirectionDown, 5)
	require.Error(t, err, "target > currentVersion for down direction should error")
}
