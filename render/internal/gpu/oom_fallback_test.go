//go:build !nogpu

package gpu

import (
	"errors"
	"strings"
	"testing"
)

// TestTextureSet_FailFastLatch exercises the terminal-OOM invalid bit without
// a GPU: a latched size fails fast, a different size re-probes once.
func TestTextureSet_FailFastLatch(t *testing.T) {
	var ts textureSet
	if err := ts.failedFast(1200, 800); err != nil {
		t.Fatalf("unset latch must pass, got %v", err)
	}
	oom := errors.New("CreateTexture failed: out of memory")
	ret := ts.failTerminal(1200, 800, "full size and 1x1 fallback", "test_depth", oom)
	if ret == nil {
		t.Fatal("failTerminal must return an error")
	}
	for _, want := range []string{"1200", "800", "test_depth", "GPUI_POWER"} {
		if !strings.Contains(ret.Error(), want) {
			t.Fatalf("terminal error must name level/size/action, missing %q in %q", want, ret.Error())
		}
	}
	got := ts.failedFast(1200, 800)
	if got == nil {
		t.Fatal("same size must fail fast with the latched error")
	}
	if got.Error() != ret.Error() {
		t.Fatalf("latched error must be identical, got %q want %q", got.Error(), ret.Error())
	}
	if err := ts.failedFast(640, 480); err != nil {
		t.Fatalf("different size must re-probe once, got %v", err)
	}
	if ts.failedErr != nil {
		t.Fatal("re-probe must clear the latch")
	}
	ts.clearFailed()
	if err := ts.failedFast(1200, 800); err != nil {
		t.Fatalf("clearFailed must drop the latch, got %v", err)
	}
}

// TestCreateTextureRetryOOM_SmallHeapInjection is the texture-level OOM
// injection gate (multiwindow 1.3 验收3). There is no small-heap device
// fixture in this repo (no fake webgpu.Device injection point), so a live
// OOM cannot be forced deterministically here — the downgrade path is
// instead proven by the dual-window pressure真窗 (R6+m5 occupy the 1GB
// dGPU, accept downgrades with gpu_fallbacks=1) plus the latch unit test
// above. Explicit skip, never silent green.
func TestCreateTextureRetryOOM_SmallHeapInjection(t *testing.T) {
	t.Skipf("no small-heap webgpu.Device fixture: live texture-OOM injection needs a capped-heap device which this repo does not provide; coverage via pressure真窗 (gpu_fallbacks=1) + TestTextureSet_FailFastLatch")
}
