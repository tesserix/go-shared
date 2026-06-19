package temporal

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// WorkerSpec declaratively describes a task queue and the workflows/activities
// it serves. Build it fluently:
//
//	temporal.Queue(temporal.TaskQueue("homechef", "orders")).
//	    Workflows(workflows.OrderWorkflow).
//	    Activities(act.CapturePayment, act.DispatchDelivery)
type WorkerSpec struct {
	queue string
	wfs   []any
	acts  []any
}

// Queue begins a WorkerSpec for the given task queue.
func Queue(taskQueue string) WorkerSpec { return WorkerSpec{queue: taskQueue} }

// Workflows registers one or more workflow functions on the queue.
func (s WorkerSpec) Workflows(wf ...any) WorkerSpec { s.wfs = append(s.wfs, wf...); return s }

// Activities registers one or more activity functions / activity structs.
func (s WorkerSpec) Activities(a ...any) WorkerSpec { s.acts = append(s.acts, a...); return s }

// NewWorker constructs a worker bound to a task queue with platform defaults.
// Most callers should use RunWorkers instead of touching this directly.
func NewWorker(c client.Client, taskQueue string) worker.Worker {
	return worker.New(c, taskQueue, worker.Options{
		// Conservative defaults; tune per task queue during hardening.
		MaxConcurrentActivityExecutionSize:     100,
		MaxConcurrentWorkflowTaskExecutionSize: 100,
	})
}
