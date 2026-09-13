package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DefaultActivityOptions is the default retry/schedule configuration for Awo
// activities. Apply to a workflow context with workflow.WithActivityOptions
// before calling ExecuteActivity.
var DefaultActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 30 * time.Second,
	RetryPolicy: &temporal.RetryPolicy{
		MaximumAttempts:    3,
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
	},
}

// ExecuteActivity is a typed wrapper around workflow.ExecuteActivity.
// T is the return type of the activity function. It applies
// DefaultActivityOptions if no ActivityOptions are already set on ctx.
//
// Usage inside a workflow:
//
//	result, err := workflow.ExecuteActivity[MyOutput](ctx, activities.MyActivity, input)
func ExecuteActivity[T any](ctx workflow.Context, fn any, args ...any) (T, error) {
	// Apply default options if the caller has not already set them.
	if opts := workflow.GetActivityOptions(ctx); opts.StartToCloseTimeout == 0 {
		ctx = workflow.WithActivityOptions(ctx, DefaultActivityOptions)
	}

	var zero T
	future := workflow.ExecuteActivity(ctx, fn, args...)
	if err := future.Get(ctx, &zero); err != nil {
		return zero, err
	}
	return zero, nil
}
