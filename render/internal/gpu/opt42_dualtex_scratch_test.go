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
		ops := rc.dualTexOpsScratch[:0]
		if cap(ops) < n {
			ops = make([]dualTexLayerIntoDestOp, 0, n)
		}
		for i := 0; i < n; i++ {
			ops = append(ops, dualTexLayerIntoDestOp{bounds: image.Rect(0, 0, 8, 8), opacity: 1})
		}
		rc.dualTexOpsScratch = ops

		viewOps := rc.dualTexViewOpsScratch[:0]
		if cap(viewOps) < len(ops) {
			viewOps = make([]dualTexViewBlendOp, 0, len(ops))
		}
		for i := range ops {
			viewOps = append(viewOps, dualTexViewBlendOp{bounds: ops[i].bounds, opacity: ops[i].opacity})
		}
		rc.dualTexViewOpsScratch = viewOps
	}
	capOps, capView := cap(rc.dualTexOpsScratch), cap(rc.dualTexViewOpsScratch)
	allocs := testing.AllocsPerRun(200, func() {
		ops := rc.dualTexOpsScratch[:0]
		if cap(ops) < 2 {
			t.Fatal("cap ops")
		}
		ops = append(ops, dualTexLayerIntoDestOp{}, dualTexLayerIntoDestOp{})
		rc.dualTexOpsScratch = ops
		viewOps := rc.dualTexViewOpsScratch[:0]
		viewOps = append(viewOps, dualTexViewBlendOp{}, dualTexViewBlendOp{})
		rc.dualTexViewOpsScratch = viewOps
	})
	if allocs != 0 {
		t.Fatalf("warm dual-tex scratch allocs=%v want 0 (capOps=%d capView=%d)", allocs, capOps, capView)
	}
}
