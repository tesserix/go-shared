package temporal

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Runtime is a thin, reusable wrapper around a Temporal client. Any tesserix
// service uses it to start workflows and run workers with almost no boilerplate.
// Producers hold one Runtime for the process lifetime; workers usually just call
// the package-level RunWorkers.
type Runtime struct{ c client.Client }

// NewRuntime dials Temporal from environment configuration (see Config).
func NewRuntime() (*Runtime, error) {
	c, err := NewClient()
	if err != nil {
		return nil, err
	}
	return &Runtime{c: c}, nil
}

// Close releases the underlying client. Safe on a nil Runtime.
func (r *Runtime) Close() {
	if r != nil && r.c != nil {
		r.c.Close()
	}
}

// Client exposes the raw SDK client for advanced use (schedules, queries, …).
func (r *Runtime) Client() client.Client { return r.c }

// Start launches a workflow with an idempotent workflow ID. If a workflow with
// the same ID is already running, ExecuteWorkflow returns the existing run with
// no error (the conflict policy defaults to "use existing" via
// WorkflowExecutionErrorWhenAlreadyStarted=false), so producer-side retries are
// a safe no-op. id should follow "<product>:<domain>:<entityID>" — use
// WorkflowID(...) to build it.
func (r *Runtime) Start(ctx context.Context, taskQueue, id string, wf any, args ...any) (client.WorkflowRun, error) {
	return r.c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        id,
		TaskQueue: taskQueue,
	}, wf, args...)
}

// Signal delivers a signal to a running workflow (e.g. a webhook → workflow).
func (r *Runtime) Signal(ctx context.Context, workflowID, signalName string, arg any) error {
	return r.c.SignalWorkflow(ctx, workflowID, "", signalName, arg)
}

// RunWorkers starts the given workers, blocks until SIGINT/SIGTERM, then stops
// them gracefully.
func (r *Runtime) RunWorkers(specs ...WorkerSpec) error {
	running := make([]worker.Worker, 0, len(specs))
	for _, s := range specs {
		w := NewWorker(r.c, s.queue)
		for _, wf := range s.wfs {
			w.RegisterWorkflow(wf)
		}
		for _, a := range s.acts {
			w.RegisterActivity(a)
		}
		if err := w.Start(); err != nil {
			return err
		}
		running = append(running, w)
	}
	defer func() {
		for _, w := range running {
			w.Stop()
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	return nil
}

// RunWorkers is the one-call entrypoint for a worker binary: it dials Temporal,
// runs the given workers, and blocks until shutdown. A service's entire worker
// main can be:
//
//	temporal.RunWorkers(
//	    temporal.Queue(temporal.TaskQueue("homechef", "notifications")).
//	        Workflows(workflows.NotificationWorkflow).
//	        Activities(workflows.SendNotificationActivity),
//	)
func RunWorkers(specs ...WorkerSpec) error {
	rt, err := NewRuntime()
	if err != nil {
		return err
	}
	defer rt.Close()
	return rt.RunWorkers(specs...)
}
