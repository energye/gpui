//go:build !nogpu

package gpu

import (
	"errors"
	"sync"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// direct_submit_test.go pins the P2.4d device-level submission gate: a pooled
// texture that a shared pool submitted on its own command buffer must stay
// un-lendable and un-destroyable until that submission's completion token
// reports done — the submits that used to be naked Queue.Submit calls.

// manualToken is a hal.CompletionToken the test flips by hand.
type manualToken struct {
	mu   sync.Mutex
	done bool
}

func (m *manualToken) Done() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.done
}

func (m *manualToken) Wait() error {
	return nil
}

func (m *manualToken) finish() {
	m.mu.Lock()
	m.done = true
	m.mu.Unlock()
}

// holdQueue wraps the noop queue so the test controls the completion token and
// the submit outcome while Submit stays real (submission index still moves).
type holdQueue struct {
	hal.Queue
	token       hal.CompletionToken
	tokenErr    error
	submitErr   error
	submissions int
}

func (q *holdQueue) Submit(cbs ...hal.CommandBuffer) (uint64, error) {
	q.submissions++
	if q.submitErr != nil {
		return 0, q.submitErr
	}
	return q.Queue.Submit(cbs...)
}

func (q *holdQueue) OnSubmittedWorkDone() (hal.CompletionToken, error) {
	return q.token, q.tokenErr
}

func newHoldQueue(device hal.Device, tok hal.CompletionToken) *holdQueue {
	return &holdQueue{Queue: device.Queue(), token: tok}
}

func snapNative(t *testing.T, s *GPUShared, ref res.Ref) *dualTexSnapNative {
	t.Helper()
	owner := dualTexOf(s).resOwner
	native := owner.Registry().NativeOf(ref)
	n, ok := native.(*dualTexSnapNative)
	if !ok || n == nil {
		t.Fatalf("dual-tex snap native is %T, want *dualTexSnapNative", native)
	}
	return n
}

// TestDirectSubmit_HoldBlocksRepoolUntilTokenDone is the core race pin: with the
// hold outstanding, returning the snap must not put it back in the bucket (a
// second window would get a texture the GPU is still writing); resolving the
// token must re-pool it so the next request reuses it.
func TestDirectSubmit_HoldBlocksRepoolUntilTokenDone(t *testing.T) {
	device, s := newPooledShared(t)
	t.Cleanup(s.Close)

	d := s.directSubmitLocked()
	gate := d.gate
	tok := &manualToken{}
	q := newHoldQueue(device, tok)

	snap := poolSnap(t, s, device, 256, 256)
	domain := dualTexDomain(t, s)
	if _, err := submitGated(d, q, []res.Ref{snap.ref}); err != nil {
		t.Fatalf("gated submit: %v", err)
	}
	if gate.Pending() != 1 {
		t.Fatalf("gate pending=%d want 1 (hold must be open before the token resolves)", gate.Pending())
	}

	recycleSnap(t, s, snap)
	if st := domain.Stats(); st.Idle != 0 || st.Entries != 1 {
		t.Fatalf("after recycle with the hold open: entries=%d idle=%d want entries=1 idle=0", st.Entries, st.Idle)
	}

	// While the hold is open the same key must miss and take a second native.
	second := poolSnap(t, s, device, 256, 256)
	st := domain.Stats()
	if st.Entries != 2 || st.Idle != 0 {
		t.Fatalf("in-flight snap was re-lent: entries=%d idle=%d want entries=2 idle=0", st.Entries, st.Idle)
	}
	if first, other := snapNative(t, s, snap.ref), snapNative(t, s, second.ref); first.tex == other.tex {
		t.Fatal("the second acquire returned the same native the GPU is still writing")
	}
	if st.Misses < 2 {
		t.Fatalf("misses=%d want >=2 (the held snap must not count as a hit)", st.Misses)
	}

	tok.finish()
	d.pump()
	if gate.Pending() != 0 {
		t.Fatalf("gate pending=%d want 0 after the token reported done", gate.Pending())
	}
	if got := domain.Stats().Idle; got != 1 {
		t.Fatalf("idle after pump=%d want 1 (the resolved snap must return to its bucket)", got)
	}

	// The resolved snap must be the one the next request gets back (the second
	// one is still checked out).
	reused := poolSnap(t, s, device, 256, 256)
	if a, b := snapNative(t, s, reused.ref), snapNative(t, s, snap.ref); a.tex != b.tex {
		t.Fatal("reuse after completion did not take the resolved snap")
	}
	recycleSnap(t, s, reused)
	recycleSnap(t, s, second)
}

// TestDirectSubmit_SubmitErrorClosesHold: nothing reached the GPU, so the hold
// must not stay open forever waiting on a fence that will never exist.
func TestDirectSubmit_SubmitErrorClosesHold(t *testing.T) {
	device, s := newPooledShared(t)
	t.Cleanup(s.Close)

	d := s.directSubmitLocked()
	q := newHoldQueue(device, &manualToken{})
	q.submitErr = errors.New("injected submit failure")

	snap := poolSnap(t, s, device, 128, 128)
	if _, err := submitGated(d, q, []res.Ref{snap.ref}); err == nil {
		t.Fatal("injected submit error must surface to the caller")
	}
	if got := d.gate.Pending(); got != 0 {
		t.Fatalf("gate pending=%d want 0 (failed submit must resolve its own hold)", got)
	}
	recycleSnap(t, s, snap)
	if got := dualTexDomain(t, s).Stats().Idle; got != 1 {
		t.Fatalf("idle=%d want 1 (a snap whose submit failed must pool again at once)", got)
	}
}

// TestDirectSubmit_MissingTokenWaitsForDrain: a backend that gives no fence
// keeps the hold open until a real drain (WaitIdle proved the GPU finished).
// Guessing here would reopen the race.
func TestDirectSubmit_MissingTokenWaitsForDrain(t *testing.T) {
	device, s := newPooledShared(t)
	t.Cleanup(s.Close)

	d := s.directSubmitLocked()
	q := newHoldQueue(device, nil) // no token, no error
	q.tokenErr = nil

	snap := poolSnap(t, s, device, 64, 64)
	if _, err := submitGated(d, q, []res.Ref{snap.ref}); err != nil {
		t.Fatalf("gated submit: %v", err)
	}
	if got := d.gate.Pending(); got != 1 {
		t.Fatalf("gate pending=%d want 1 (unfenced submit stays held)", got)
	}
	recycleSnap(t, s, snap)
	if got := dualTexDomain(t, s).Stats().Idle; got != 0 {
		t.Fatalf("idle=%d want 0: an unfenced submission must not re-pool on a pump", got)
	}
	d.pump()
	if got := dualTexDomain(t, s).Stats().Idle; got != 0 {
		t.Fatalf("idle=%d want 0 after pump: only a drain may close a token-less hold", got)
	}

	d.drain()
	if got := d.gate.Pending(); got != 0 {
		t.Fatalf("gate pending=%d want 0 after drain", got)
	}
	if got := dualTexDomain(t, s).Stats().Idle; got != 1 {
		t.Fatalf("idle=%d want 1 after drain (the GPU is proven idle)", got)
	}
}

// TestDirectSubmit_InvalidateNeverResurrectsNative is the device-loss shape:
// dropping holds must not decrement in-flight and must not lend the lost
// device's native back out.
func TestDirectSubmit_InvalidateNeverResurrectsNative(t *testing.T) {
	device, s := newPooledShared(t)
	t.Cleanup(s.Close)

	d := s.directSubmitLocked()
	q := newHoldQueue(device, &manualToken{})

	snap := poolSnap(t, s, device, 32, 32)
	if _, err := submitGated(d, q, []res.Ref{snap.ref}); err != nil {
		t.Fatalf("gated submit: %v", err)
	}
	if got := d.gate.Pending(); got != 1 {
		t.Fatalf("gate pending=%d want 1 before the loss path", got)
	}
	recycleSnap(t, s, snap)
	domain := dualTexDomain(t, s)
	if got := domain.Stats().Idle; got != 0 {
		t.Fatalf("idle=%d want 0 while the submission is still held", got)
	}

	d.invalidate()
	if got := d.gate.Pending(); got != 0 {
		t.Fatalf("gate pending=%d want 0 after invalidate", got)
	}
	if got := domain.Stats().Idle; got != 0 {
		t.Fatalf("idle=%d want 0: a lost device's native must never re-enter the pool", got)
	}
	lost := snapNative(t, s, snap.ref)
	fresh := poolSnap(t, s, device, 32, 32)
	if got := snapNative(t, s, fresh.ref); got.tex == lost.tex {
		t.Fatal("the invalidated native was handed out again")
	}
	// Double-resolving the dropped handle must stay a no-op.
	d.drain()
	if got := d.gate.Pending(); got != 0 {
		t.Fatalf("gate pending=%d want 0 after drain following invalidate", got)
	}
	recycleSnap(t, s, fresh)
}

// TestSubmitGated_WithoutGateIsPlainSubmit: bare cache literals and
// registry-less paths must keep the old direct submit behaviour.
func TestSubmitGated_WithoutGateIsPlainSubmit(t *testing.T) {
	device, s := newPooledShared(t)
	t.Cleanup(s.Close)

	q := newHoldQueue(device, &manualToken{})
	idx, err := submitGated(nil, q, nil)
	if err != nil {
		t.Fatalf("submitGated without a gate: %v", err)
	}
	if idx == 0 || q.submissions != 1 {
		t.Fatalf("submission index=%d, submits=%d want a real plain submit", idx, q.submissions)
	}
}
