//go:build !nogpu

package gpu

import "testing"

// An idle present must not strand queued draw work: DiscardPending drops the
// queues, releases deferred advanced-layer holds, and clears the pending
// target so the next frame starts clean.
func TestDiscardPendingDropsQueuedDraws(t *testing.T) {
	released := 0
	rc := &GPURenderContext{}
	rc.pendingGPUTextureCommands = append(rc.pendingGPUTextureCommands, GPUTextureDrawCommand{})
	rc.pendingAdvancedLayers = append(rc.pendingAdvancedLayers, pendingAdvancedLayer{
		release: func() { released++ },
	})
	rc.layerReleaseHold = append(rc.layerReleaseHold, func() { released++ })
	rc.hasPendingTarget = true

	if rc.PendingCount() == 0 {
		t.Fatal("PendingCount=0 with queued draws")
	}
	rc.DiscardPending()

	if got := len(rc.pendingGPUTextureCommands); got != 0 {
		t.Fatalf("pendingGPUTextureCommands=%d want 0", got)
	}
	if got := len(rc.pendingAdvancedLayers); got != 0 {
		t.Fatalf("pendingAdvancedLayers=%d want 0", got)
	}
	if got := len(rc.layerReleaseHold); got != 0 {
		t.Fatalf("layerReleaseHold=%d want 0", got)
	}
	if rc.hasPendingTarget {
		t.Fatal("hasPendingTarget=true want false")
	}
	if released != 2 {
		t.Fatalf("released=%d want 2 (advanced layer + deferred hold)", released)
	}
	if got := rc.PendingCount(); got != 0 {
		t.Fatalf("PendingCount=%d after discard want 0", got)
	}
}
