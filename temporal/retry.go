package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DefaultRetryPolicy is the platform's standard activity retry policy:
// exponential backoff, retry until the activity's timeout (no attempt cap).
func DefaultRetryPolicy() *temporal.RetryPolicy {
	return &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
	}
}

// Activities applies platform defaults (timeout + retry) and returns a context
// ready for ExecuteActivity. Use inside a workflow:
//
//	ctx = temporal.Activities(ctx, 30*time.Second)
//	workflow.ExecuteActivity(ctx, MyActivity, in).Get(ctx, nil)
func Activities(ctx workflow.Context, startToClose time.Duration) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: startToClose,
		RetryPolicy:         DefaultRetryPolicy(),
	})
}
