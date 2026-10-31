//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render/internal/gpu/res"
)

// drainProbeToken counts observations and discards separately.
type drainProbeToken struct {
	observed  int
	discarded int
}

func (t *drainProbeToken) Done() bool { t.observed++; return true }

func (t *drainProbeToken) Wait() error { return nil }

func (t *drainProbeToken) Discard() { t.discarded++ }

// j2Native is a minimal res.Native for hold bookkeeping.
type j2Native struct{ released int }

func (n *j2Native) Release() { n.released++ }

// TestDirectSubmit_DrainObservesTokensAndInvalidateDiscards pins J2 (P2.4e):
// drain (GPU proven idle) observes each token once so the backend releases its
// completion record; invalidate (device loss) discards the record without
// observing — neither path leaks an unobserved record in the backend map.
func TestDirectSubmit_DrainObservesTokensAndInvalidateDiscards(t *testing.T) {
	reg := res.NewRegistry()
	d := newDirectSubmit(reg)
	if d == nil || d.gate == nil {
		t.Fatal("directSubmit with gate required")
	}
	ref := reg.DeviceOwner().Register(&j2Native{})

	tok := &drainProbeToken{}
	hold := d.gate.Hold(ref)
	if hold == nil {
		t.Fatal("hold required")
	}
	d.mu.Lock()
	d.live = append(d.live, directCompletion{token: tok, hold: hold})
	d.mu.Unlock()

	d.drain()
	if tok.observed != 1 {
		t.Fatalf("drain must observe the token exactly once, got %d", tok.observed)
	}
	if tok.discarded != 0 {
		t.Fatal("drain must not discard")
	}
	if d.gate.Pending() != 0 {
		t.Fatalf("drain must settle all holds, pending=%d", d.gate.Pending())
	}

	tok2 := &drainProbeToken{}
	hold2 := d.gate.Hold(ref)
	if hold2 == nil {
		t.Fatal("second hold required")
	}
	d.mu.Lock()
	d.live = append(d.live, directCompletion{token: tok2, hold: hold2})
	d.mu.Unlock()

	d.invalidate()
	if tok2.discarded != 1 {
		t.Fatalf("invalidate must discard the record exactly once, got %d", tok2.discarded)
	}
	if tok2.observed != 0 {
		t.Fatal("invalidate must not observe a never-completing token")
	}
	if d.gate.Pending() != 0 {
		t.Fatalf("invalidate must settle all holds, pending=%d", d.gate.Pending())
	}
}
