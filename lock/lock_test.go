package lock_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"awo.so/awo/lock"
)

// TestErrLockNotAcquired_IsSentinel verifies that the exported sentinel is not
// nil and has a meaningful error message.
func TestErrLockNotAcquired_IsSentinel(t *testing.T) {
	assert.NotNil(t, lock.ErrLockNotAcquired)
	assert.NotEmpty(t, lock.ErrLockNotAcquired.Error())
}

// TestIsNotAcquired_True verifies that IsNotAcquired returns true for the
// sentinel error (direct reference).
func TestIsNotAcquired_True(t *testing.T) {
	assert.True(t, lock.IsNotAcquired(lock.ErrLockNotAcquired))
}

// TestIsNotAcquired_Wrapped verifies that IsNotAcquired returns true when the
// sentinel is wrapped in another error (errors.Is semantics).
func TestIsNotAcquired_Wrapped(t *testing.T) {
	wrapped := errors.Join(errors.New("outer"), lock.ErrLockNotAcquired)
	assert.True(t, lock.IsNotAcquired(wrapped))
}

// TestIsNotAcquired_False verifies that IsNotAcquired returns false for
// unrelated errors.
func TestIsNotAcquired_False(t *testing.T) {
	assert.False(t, lock.IsNotAcquired(errors.New("some other error")))
}

// TestIsNotAcquired_Nil verifies that IsNotAcquired returns false for nil.
func TestIsNotAcquired_Nil(t *testing.T) {
	assert.False(t, lock.IsNotAcquired(nil))
}
