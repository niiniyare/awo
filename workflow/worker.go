package workflow

import (
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// WorkerConfig holds all parameters needed to create a Temporal worker pool.
type WorkerConfig struct {
	// TemporalHost is the Temporal frontend address, e.g. "temporal:7233".
	TemporalHost string

	// Namespace is the Temporal namespace, e.g. "awo-production".
	Namespace string

	// TaskQueues lists the task queues this worker will poll.
	// A separate worker.Worker is started for each task queue.
	TaskQueues []string

	// Workflows is the list of workflow functions to register on every worker.
	Workflows []any

	// Activities is the list of activity functions (or activity struct pointers)
	// to register on every worker.
	Activities []any
}

// Worker manages a pool of Temporal workers, one per task queue.
type Worker struct {
	client  client.Client
	workers []worker.Worker
}

// NewWorker creates the Temporal client and builds worker.Worker instances for
// each task queue declared in cfg. Call Start to begin polling.
func NewWorker(cfg WorkerConfig) (*Worker, error) {
	if len(cfg.TaskQueues) == 0 {
		return nil, fmt.Errorf("workflow.NewWorker: at least one TaskQueue is required")
	}

	c, err := client.Dial(client.Options{
		HostPort:  cfg.TemporalHost,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("workflow.NewWorker: dial Temporal at %s: %w", cfg.TemporalHost, err)
	}

	w := &Worker{client: c}

	for _, queue := range cfg.TaskQueues {
		wkr := worker.New(c, queue, worker.Options{})
		for _, wf := range cfg.Workflows {
			wkr.RegisterWorkflow(wf)
		}
		for _, act := range cfg.Activities {
			wkr.RegisterActivity(act)
		}
		w.workers = append(w.workers, wkr)
	}

	return w, nil
}

// Start begins polling all task queues. It is non-blocking — each worker
// runs in its own goroutine managed by the Temporal SDK.
func (w *Worker) Start() error {
	for _, wkr := range w.workers {
		if err := wkr.Start(); err != nil {
			return fmt.Errorf("workflow.Worker.Start: %w", err)
		}
	}
	return nil
}

// Stop gracefully shuts down all workers and closes the Temporal client.
func (w *Worker) Stop() {
	for _, wkr := range w.workers {
		wkr.Stop()
	}
	w.client.Close()
}

// Client returns the underlying Temporal client. Use it to start workflows,
// send signals, or query workflow state from outside a workflow context.
func (w *Worker) Client() client.Client {
	return w.client
}
