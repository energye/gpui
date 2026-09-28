//go:build linux && !nogpu

package gles

import (
	"errors"
	"testing"
)

// Bind failure must be stoppable: TryLock reports LockMakeCurrentErr
// instead of handing back a context with nothing current (the old Lock
// only logged, so Configure fell through into 0-handle allocs and the
// cold-start crash surfaced as a bare "glGenRenderbuffers returned 0").
func TestTryLockNilContextStops(t *testing.T) {
	c := &AdapterContext{}
	if _, err := c.TryLock(); !errors.Is(err, LockMakeCurrentErr) && err == nil {
		t.Fatalf("TryLock(nil ctx) = nil error, want a stop error")
	}
}
