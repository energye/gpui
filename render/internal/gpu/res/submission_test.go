package res

import "testing"

func TestSubmission_Lifecycle(t *testing.T) {
	reg := NewRegistry()
	sub := NewSubmission(reg)

	f := &fakeNative{tag: 1}
	ref := reg.Register(f)
	reg.Retire(ref) // mark for destruction

	sub.Begin()
	sub.Track(ref)   // submitted
	reg.Release(ref) // refs==0, inflight==1 → retain

	if f.released != 0 {
		t.Fatalf("released before SubmitDone: %d", f.released)
	}
	if n := len(sub.InFlight()); n != 1 {
		t.Fatalf("expected 1 in-flight job, got %d", n)
	}

	sub.SubmitDone()
	if f.released != 1 {
		t.Fatalf("expected release after SubmitDone, got %d", f.released)
	}
	if n := len(sub.InFlight()); n != 0 {
		t.Fatalf("expected no in-flight jobs after SubmitDone, got %d", n)
	}
}

func TestSubmission_InvalidateOnDeviceLoss(t *testing.T) {
	reg := NewRegistry()
	sub := NewSubmission(reg)

	f := &fakeNative{tag: 1}
	ref := reg.Register(f)
	reg.Retire(ref)
	sub.Track(ref)
	reg.Release(ref)

	// Device lost: the fence will never arrive. Invalidate force-clears the
	// in-flight state WITHOUT calling native Release (abandon flow owns that).
	sub.Invalidate()
	if f.released != 0 {
		t.Fatalf("Invalidate must not release native (abandon owns it): %d", f.released)
	}
	if n := len(sub.InFlight()); n != 0 {
		t.Fatalf("expected no in-flight jobs after Invalidate, got %d", n)
	}
	// Registry teardown is separate (Registry.InvalidateAll), so the entry
	// stays until then — no double-free paths here.
	if reg.Count() != 1 {
		t.Fatalf("registry must retain entries until Registry.InvalidateAll, got %d", reg.Count())
	}
}

func TestSubmission_TrackAfterInvalidateShortCircuits(t *testing.T) {
	reg := NewRegistry()
	sub := NewSubmission(reg)
	f := &fakeNative{tag: 2}

	ref := reg.Register(f)
	sub.Invalidate()
	sub.Track(ref)                      // device lost: tracking must be harmless
	sub.SubmitDone()                    // no jobs → no-op
	reg.Release(ref)                    // not retired → nothing released
	if f.released != 0 {
		t.Fatalf("unexpected release: %d", f.released)
	}
}