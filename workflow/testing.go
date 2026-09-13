package workflow

import (
	"testing"

	"go.temporal.io/sdk/testsuite"
)

// WorkflowTestEnv wraps Temporal's test environment with convenience methods.
// Use it in unit tests to execute workflows without a running Temporal server.
type WorkflowTestEnv struct {
	*testsuite.TestWorkflowEnvironment
	suite *testsuite.WorkflowTestSuite
}

// NewTestEnv creates a WorkflowTestEnv for t. The test environment is
// automatically completed when the workflow function under test returns.
func NewTestEnv(t testing.TB) *WorkflowTestEnv {
	t.Helper()
	s := &testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	return &WorkflowTestEnv{
		TestWorkflowEnvironment: env,
		suite:                   s,
	}
}

// MockActivityResult registers a successful mock result for the given activity
// function. Use this when the activity returns (T, error) and you want to mock
// a successful execution.
//
// Example:
//
//	env.MockActivityResult(activities.SendEmail, SendEmailResult{MessageID: "abc"})
func (e *WorkflowTestEnv) MockActivityResult(fn any, result any) {
	e.OnActivity(fn).Return(result, nil)
}

// MockActivityError registers a mock failure for the given activity function.
// The activity will appear to return the given error when executed.
func (e *WorkflowTestEnv) MockActivityError(fn any, err error) {
	e.OnActivity(fn).Return(nil, err)
}
