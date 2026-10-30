//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package res

import "testing"

// The three former bare deferred-release queues (session textures/buffers,
// image-cache entries, cache pending) now share one registry state machine:
// Register → Track → Release → Retire, released only when SubmitDone clears
// the in-flight count. These tests pin that exact order (unlike
// TestReleaseSequenceTiming, which pins Retire-before-Release).

func TestSessionRetireSequence_GatedBySubmitDone(t *testing.T) {
	reg := NewRegistry()
	sub := NewSubmission(reg)
	f := &fakeNative{tag: 1}

	ref := reg.Register(f) // refs=1 (usage ref owned by the caller)
	sub.Track(ref)         // inflight=1 (current submission references it)
	reg.Release(ref)       // refs=0, inflight=1 → still alive
	reg.Retire(ref)        // retire requested while in flight
	if f.released != 0 {
		t.Fatalf("released before SubmitDone: %d", f.released)
	}
	if n := reg.PendingReleaseCount(); n != 1 {
		t.Fatalf("PendingReleaseCount=%d want 1", n)
	}
	if reg.Count() != 1 {
		t.Fatalf("registry must retain the slot until the submission completes")
	}

	sub.SubmitDone()
	if f.released != 1 {
		t.Fatalf("released=%d want exactly 1 after SubmitDone", f.released)
	}
	if reg.Count() != 0 || reg.PendingReleaseCount() != 0 {
		t.Fatalf("registry not drained after SubmitDone: count=%d pending=%d",
			reg.Count(), reg.PendingReleaseCount())
	}

	// Duplicate completion must not release twice.
	sub.SubmitDone()
	if f.released != 1 {
		t.Fatalf("duplicate SubmitDone released again: %d", f.released)
	}
}

func TestIdleCallback_FiresOnlyWhenRefsAndInflightClear(t *testing.T) {
	reg := NewRegistry()
	sub := NewSubmission(reg)
	f := &fakeNative{tag: 1}
	ref := reg.Register(f)
	idle := 0
	reg.SetIdleCallback(reg.ID(ref), func() { idle++ })

	sub.Track(ref)
	reg.Release(ref) // refs=0 but inflight=1 → not idle yet
	if idle != 0 {
		t.Fatalf("idle callback fired with a submission in flight: %d", idle)
	}
	sub.SubmitDone()
	if idle != 1 {
		t.Fatalf("idle callback after SubmitDone=%d want 1", idle)
	}
	// The native stays alive: idle re-pools, it does not destroy.
	if f.released != 0 {
		t.Fatalf("idle callback must not release the native: %d", f.released)
	}
}

func TestIdleCallback_SkippedForRetiredEntry(t *testing.T) {
	reg := NewRegistry()
	f := &fakeNative{tag: 1}
	ref := reg.Register(f)
	idle := 0
	reg.SetIdleCallback(reg.ID(ref), func() { idle++ })

	reg.Retire(ref)
	reg.Release(ref) // refs=0 && retire → released, never re-pooled
	if idle != 0 {
		t.Fatalf("retired entry must not fire the idle callback: %d", idle)
	}
	if f.released != 1 {
		t.Fatalf("retired idle entry must release exactly once, got %d", f.released)
	}
}

// Device loss: Invalidate clears in-flight bookkeeping without releasing the
// natives (the abandon flow owns native teardown), and InvalidateAll then
// drops the slots.
func TestSubmissionInvalidate_NoNativeRelease(t *testing.T) {
	reg := NewRegistry()
	sub := NewSubmission(reg)
	f := &fakeNative{tag: 1}
	ref := reg.Register(f)
	sub.Track(ref)
	reg.Release(ref)
	reg.Retire(ref)

	sub.Invalidate()
	if f.released != 0 {
		t.Fatalf("Invalidate must not release natives: %d", f.released)
	}
	reg.InvalidateAll()
	if reg.Count() != 0 {
		t.Fatalf("registry not cleared after InvalidateAll")
	}
	if f.released != 0 {
		t.Fatalf("InvalidateAll must not release natives: %d", f.released)
	}
}
