//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render/internal/gpu/res"
)

// Two window sessions share one device-level registry (Skia GrContext: one
// cache, N surfaces). Closing one window must release exactly its entries and
// leave the other window's bookkeeping intact.
func TestSessionRegistry_SharedDeviceOwnerScope(t *testing.T) {
	reg := res.NewRegistry()
	a := NewGPURenderSessionWithRegistry(nil, nil, 4, reg, nil)
	b := NewGPURenderSessionWithRegistry(nil, nil, 4, reg, nil)
	if a.Res().ID() == b.Res().ID() {
		t.Fatal("sessions must get distinct owner ids")
	}

	na := &countingNative{}
	nb := &countingNative{}
	a.Res().Register(na)
	b.Res().Register(nb)

	a.Destroy()
	if na.n != 1 {
		t.Fatalf("window A teardown must release its native once, got %d", na.n)
	}
	if nb.n != 0 {
		t.Fatalf("window A teardown must not touch window B's native, got %d", nb.n)
	}
	if got := reg.Count(); got != 1 {
		t.Fatalf("registry must keep window B's entry, count=%d", got)
	}

	b.Destroy()
	if nb.n != 1 {
		t.Fatalf("window B teardown must release its native once, got %d", nb.n)
	}
	if got := reg.Count(); got != 0 {
		t.Fatalf("registry must be empty after both windows close, count=%d", got)
	}
}

// Device loss drops one session's bookkeeping without releasing natives (the
// abandon flow owns native teardown) and leaves other sessions untouched.
func TestSessionRegistry_InvalidateForDeviceLossScoped(t *testing.T) {
	reg := res.NewRegistry()
	a := NewGPURenderSessionWithRegistry(nil, nil, 4, reg, nil)
	b := NewGPURenderSessionWithRegistry(nil, nil, 4, reg, nil)

	na := &countingNative{}
	nb := &countingNative{}
	a.Res().Register(na)
	b.Res().Register(nb)

	a.InvalidateForDeviceLoss()
	if na.n != 0 || nb.n != 0 {
		t.Fatalf("device-loss invalidation must not release natives: a=%d b=%d", na.n, nb.n)
	}
	if got := reg.Count(); got != 1 {
		t.Fatalf("only the lost session's entries drop, count=%d", got)
	}

	a.Destroy()
	b.Destroy()
	if nb.n != 1 {
		t.Fatalf("surviving session must still release on teardown, got %d", nb.n)
	}
}
