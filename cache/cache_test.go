package cache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "awo.so/awo/cache"
)

var ctx = context.Background()

// --- NoopCache ---

func TestNoopCache_Get_ReturnsMiss(t *testing.T) {
	var c NoopCache
	err := c.Get(ctx, "key", new(string))
	if !errors.Is(err, ErrMiss) {
		t.Fatalf("expected ErrMiss, got %v", err)
	}
}

func TestNoopCache_Set_NoError(t *testing.T) {
	var c NoopCache
	if err := c.Set(ctx, "key", "value", time.Minute); err != nil {
		t.Fatalf("Set should not error: %v", err)
	}
}

func TestNoopCache_Delete_NoError(t *testing.T) {
	var c NoopCache
	if err := c.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete should not error: %v", err)
	}
}

func TestNoopCache_DeletePrefix_NoError(t *testing.T) {
	var c NoopCache
	if err := c.DeletePrefix(ctx, "prefix:"); err != nil {
		t.Fatalf("DeletePrefix should not error: %v", err)
	}
}

func TestNoopCache_Exists_ReturnsFalse(t *testing.T) {
	var c NoopCache
	ok, err := c.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists should not error: %v", err)
	}
	if ok {
		t.Fatal("NoopCache.Exists should always return false")
	}
}

func TestNoopCache_ImplementsInterface(t *testing.T) {
	var _ Cache = NoopCache{}
}

// --- NoopCounter ---

func TestNoopCounter_Increment_ReturnsZero(t *testing.T) {
	var c NoopCounter
	v, err := c.Increment(ctx, "rate:key", 1)
	if err != nil {
		t.Fatalf("Increment should not error: %v", err)
	}
	if v != 0 {
		t.Errorf("NoopCounter.Increment should return 0, got %d", v)
	}
}

func TestNoopCounter_IncrementWithReset_ReturnsZero(t *testing.T) {
	var c NoopCounter
	v, err := c.IncrementWithReset(ctx, "rate:key", 1, time.Minute)
	if err != nil {
		t.Fatalf("IncrementWithReset should not error: %v", err)
	}
	if v != 0 {
		t.Errorf("NoopCounter.IncrementWithReset should return 0, got %d", v)
	}
}

func TestNoopCounter_ImplementsInterface(t *testing.T) {
	var _ Counter = NoopCounter{}
}

// --- IsMiss helper ---

func TestIsMiss_True_OnErrMiss(t *testing.T) {
	if !IsMiss(ErrMiss) {
		t.Fatal("IsMiss should return true for ErrMiss")
	}
}

func TestIsMiss_False_OnOtherError(t *testing.T) {
	if IsMiss(errors.New("other error")) {
		t.Fatal("IsMiss should return false for non-ErrMiss errors")
	}
}

func TestIsMiss_False_OnNil(t *testing.T) {
	if IsMiss(nil) {
		t.Fatal("IsMiss should return false for nil")
	}
}
