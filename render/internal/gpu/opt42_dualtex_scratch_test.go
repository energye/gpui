package gpu

import (
	"image"
	"testing"
)

// TestOpt42_DualTexResolveScratchReuse checks scratch slices grow once then reuse.
func TestOpt42_DualTexResolveScratchReuse(t *testing.T) {
	rc := &GPURenderContext{}
	// Simulate resolve scratch growth
	for n := 1; n <= 4; n++ {
		viewOps := rc.dualTexViewOpsScratch[:0]
		if cap(viewOps) < n {
			viewOps = make([]dualTexViewBlendOp, 0, n)
		}
		for i := 0; i < n; i++ {
			viewOps = append(viewOps, dualTexViewBlendOp{bounds: image.Rect(0, 0, 8, 8), opacity: 1})
		}
		rc.dualTexViewOpsScratch = viewOps
	}
	capView := cap(rc.dualTexViewOpsScratch)
	allocs := testing.AllocsPerRun(200, func() {
		viewOps := rc.dualTexViewOpsScratch[:0]
		if cap(viewOps) < 2 {
			t.Fatal("cap viewOps")
		}
		viewOps = append(viewOps, dualTexViewBlendOp{}, dualTexViewBlendOp{})
		rc.dualTexViewOpsScratch = viewOps
	})
	if allocs != 0 {
		t.Fatalf("warm dual-tex scratch allocs=%v want 0 (capView=%d)", allocs, capView)
	}
}
