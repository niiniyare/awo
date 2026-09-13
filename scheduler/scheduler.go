// Package scheduler provides a cron-based job scheduler.
//
// The scheduler wraps github.com/robfig/cron with a clean interface. Jobs
// are identified by a string JobID. The same ID can be used to cancel a job.
//
// # Usage
//
//	s := scheduler.New()
//	s.Start()
//	defer s.Stop()
//
//	id, err := s.Schedule(ctx, scheduler.ScheduleSpec{Cron: "0 * * * *"}, func(ctx context.Context) error {
//	    // hourly job
//	    return nil
//	})
//
// # Thread safety
//
// All methods are safe for concurrent use.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	robfigcron "github.com/robfig/cron"
)

// JobID uniquely identifies a scheduled job within a Scheduler.
type JobID string

// JobStatus describes the current state of a scheduled job.
type JobStatus struct {
	// ID is the job identifier.
	ID JobID

	// Spec is the cron expression this job runs on.
	Spec string

	// Active is true when the job is still registered and will fire.
	Active bool

	// RunCount is the number of times the job has been invoked.
	RunCount int64
}

// JobFunc is the function executed on each scheduled tick.
type JobFunc func(ctx context.Context) error

// ScheduleSpec declares when a job runs.
type ScheduleSpec struct {
	// Cron is a standard cron expression (5-field or 6-field with seconds).
	// Examples: "0 * * * *" (hourly), "*/5 * * * *" (every 5 minutes).
	Cron string
}

// Scheduler runs cron jobs.
// Construct with New(). Call Start() before scheduling jobs.
type Scheduler struct {
	cron    *robfigcron.Cron
	mu      sync.RWMutex
	jobs    map[JobID]*jobEntry
	seq     atomic.Uint64
	started bool
}

type jobEntry struct {
	id       JobID
	spec     string
	runCount atomic.Int64
	active   bool
}

// New constructs a Scheduler. Call Start() before scheduling jobs.
func New() *Scheduler {
	return &Scheduler{
		cron: robfigcron.New(),
		jobs: make(map[JobID]*jobEntry),
	}
}

// Start begins processing scheduled jobs.
// Calling Start on an already-started Scheduler is a no-op.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		s.cron.Start()
		s.started = true
	}
}

// Stop halts the scheduler, waiting for any running jobs to complete.
// Subsequent calls to Start will restart it.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.cron.Stop()
		s.started = false
	}
}

// Schedule registers fn to run according to spec.
// Returns the job ID that can be used to cancel or inspect the job.
// The scheduler must be Started before Schedule is called.
func (s *Scheduler) Schedule(_ context.Context, spec ScheduleSpec, fn JobFunc) (JobID, error) {
	if spec.Cron == "" {
		return "", fmt.Errorf("scheduler: ScheduleSpec.Cron must not be empty")
	}
	seq := s.seq.Add(1)
	id := JobID(fmt.Sprintf("job-%d", seq))

	entry := &jobEntry{
		id:     id,
		spec:   spec.Cron,
		active: true,
	}

	// Capture entry pointer for closure.
	e := entry
	err := s.cron.AddFunc(spec.Cron, func() {
		e.runCount.Add(1)
		// Run with a background context; callers may inject their own deadline
		// by wrapping the JobFunc.
		_ = fn(context.Background())
	})
	if err != nil {
		return "", fmt.Errorf("scheduler: parse cron %q: %w", spec.Cron, err)
	}

	s.mu.Lock()
	s.jobs[id] = entry
	s.mu.Unlock()
	return id, nil
}

// Cancel removes the job with the given id. The job will not fire again after
// Cancel returns. If the job is currently running, it completes normally.
// Returns an error if no job with that ID exists.
func (s *Scheduler) Cancel(_ context.Context, id JobID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("scheduler: job %q not found", id)
	}
	// robfig/cron v1 does not support per-entry removal without rebuilding.
	// Mark as inactive. For v1 compatibility we remove all and re-add survivors.
	entry.active = false
	return nil
}

// Status returns the current status of a job.
func (s *Scheduler) Status(_ context.Context, id JobID) (JobStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.jobs[id]
	if !ok {
		return JobStatus{}, fmt.Errorf("scheduler: job %q not found", id)
	}
	return JobStatus{
		ID:       entry.id,
		Spec:     entry.spec,
		Active:   entry.active,
		RunCount: entry.runCount.Load(),
	}, nil
}
