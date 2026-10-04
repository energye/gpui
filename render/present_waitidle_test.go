package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestPresentTarget_WaitIdleNilSafe: nil targets report idle (no device to
// wait on). Close paths call WaitIdle unconditionally before destroying
// view-bound caches; a nil target (never opened) must not fail the close.
func TestPresentTarget_WaitIdleNilSafe(t *testing.T) {
	var target *render.PresentTarget
	if err := target.WaitIdle(); err != nil {
		t.Fatalf("nil WaitIdle() = %v, want nil", err)
	}
}
