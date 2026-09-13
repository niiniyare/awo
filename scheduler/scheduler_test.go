package scheduler_test

import (
	"context"
	"testing"
	"time"

	. "awo.so/awo/scheduler"
)

var ctx = context.Background()

func TestScheduler_Schedule_ReturnsjobID(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	id, err := s.Schedule(ctx, ScheduleSpec{Cron: "* * * * *"}, func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty job ID")
	}
}

func TestScheduler_Schedule_EmptyCron_Error(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	_, err := s.Schedule(ctx, ScheduleSpec{Cron: ""}, func(_ context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for empty cron spec")
	}
}

func TestScheduler_Schedule_InvalidCron_Error(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	_, err := s.Schedule(ctx, ScheduleSpec{Cron: "not-a-cron"}, func(_ context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestScheduler_Status_AfterSchedule(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	id, err := s.Schedule(ctx, ScheduleSpec{Cron: "* * * * *"}, func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	status, err := s.Status(ctx, id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.ID != id {
		t.Errorf("expected ID %q, got %q", id, status.ID)
	}
	if !status.Active {
		t.Error("expected job to be active")
	}
	if status.Spec != "* * * * *" {
		t.Errorf("expected spec '* * * * *', got %q", status.Spec)
	}
}

func TestScheduler_Status_UnknownJob_Error(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	_, err := s.Status(ctx, "nonexistent-job")
	if err == nil {
		t.Fatal("expected error for unknown job ID")
	}
}

func TestScheduler_Cancel_MarksInactive(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	id, err := s.Schedule(ctx, ScheduleSpec{Cron: "* * * * *"}, func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	if err := s.Cancel(ctx, id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	status, err := s.Status(ctx, id)
	if err != nil {
		t.Fatalf("Status after Cancel: %v", err)
	}
	if status.Active {
		t.Error("expected job to be inactive after Cancel")
	}
}

func TestScheduler_Cancel_UnknownJob_Error(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	if err := s.Cancel(ctx, "no-such-job"); err == nil {
		t.Fatal("expected error cancelling unknown job")
	}
}

func TestScheduler_Start_IdempotentDoubleStart(t *testing.T) {
	s := New()
	s.Start()
	s.Start() // must not panic
	s.Stop()
}

func TestScheduler_MultipleJobs_IndependentIDs(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	id1, _ := s.Schedule(ctx, ScheduleSpec{Cron: "* * * * *"}, func(_ context.Context) error { return nil })
	id2, _ := s.Schedule(ctx, ScheduleSpec{Cron: "* * * * *"}, func(_ context.Context) error { return nil })

	if id1 == id2 {
		t.Errorf("each scheduled job must have a unique ID, got duplicate: %q", id1)
	}
}

func TestScheduler_RunCount_IncreasesOnFire(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	fired := make(chan struct{}, 1)
	// "@every 1s" fires every second — short enough for a unit test.
	id, err := s.Schedule(ctx, ScheduleSpec{Cron: "@every 1s"}, func(_ context.Context) error {
		select {
		case fired <- struct{}{}:
		default:
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	select {
	case <-fired:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not fire within 3 seconds")
	}

	status, err := s.Status(ctx, id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.RunCount < 1 {
		t.Errorf("expected RunCount >= 1 after job fired, got %d", status.RunCount)
	}
}
