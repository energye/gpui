//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestOffscreenPass_SuspendsAndRestoresMainStream pins the suspend-and-restore
// contract of BeginOffscreenPass/EndOffscreenPass (C8 pass-ownership rework):
// the main pass's pending queues, clip timeline, batch seals, frame tracking
// and deferred advanced layers are moved out wholesale at Begin and restored
// verbatim at End — a sub-pass can never merge into or consume main commands.
func TestOffscreenPass_SuspendsAndRestoresMainStream(t *testing.T) {
	rc := &GPURenderContext{hasPendingTarget: true}

	// Simulate a main surface frame in flight: queued draws across tiers,
	// an active clip timeline with two scissor segments, sealed text batches,
	// LoadOpLoad tracking on a present view, and a deferred advanced layer.
	rc.pendingShapes = []SDFRenderShape{{ColorA: 1}}
	rc.pendingConvexCommands = []ConvexDrawCommand{{
		PackedVerts: rc.appendTestMesh([]render.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}}...),
	}}
	rc.glyphMaskQuadStore = append(rc.glyphMaskQuadStore,
		GlyphMaskQuad{X0: 1, Y0: 1, X1: 9, Y1: 9},
		GlyphMaskQuad{X0: 2, Y0: 2, X1: 8, Y1: 8},
	)
	rc.pendingGlyphMaskBatches = []GlyphMaskBatch{{Quads: rc.glyphMaskQuadStore[:2]}}
	rc.scissorSegments = []scissorSegment{{sdfCount: 0, glyphCount: 1}, {sdfCount: 1}}
	rect := [4]uint32{10, 20, 30, 40}
	rc.clipRect = &rect
	rc.textBatchSealed = true
	rc.frameRendered = true // main surface already painted → LoadOpLoad semantics
	mainPending := rc.PendingCount()

	restore := rc.BeginOffscreenPass(render.GPURenderTarget{})
	if !rc.hasPendingTarget {
		t.Fatalf("sub-pass must start bound to its own target")
	}
	if got := rc.PendingCount(); got != 0 {
		t.Fatalf("sub-pass sees %d pending commands, want 0 (main stream suspended)", got)
	}
	if rc.clipRect != nil || len(rc.scissorSegments) != 0 {
		t.Fatalf("sub-pass inherited the main clip timeline")
	}
	if rc.frameRendered {
		t.Fatalf("sub-pass must get fresh-target LoadOpClear tracking")
	}

	// The sub-pass queues its own work (as recordLocalWith would).
	rc.pendingShapes = append(rc.pendingShapes, SDFRenderShape{ColorA: 0.5})
	rc.recordScissorSegment(nil)

	restore()

	if got := rc.PendingCount(); got != mainPending {
		t.Fatalf("after restore PendingCount=%d want %d", got, mainPending)
	}
	if !rc.hasPendingTarget || len(rc.pendingShapes) != 1 || rc.pendingShapes[0].ColorA != 1 {
		t.Fatalf("main shapes not restored verbatim: %+v has=%v", rc.pendingShapes, rc.hasPendingTarget)
	}
	if len(rc.pendingConvexCommands) != 1 {
		t.Fatalf("main convex lost")
	}
	if len(rc.pendingGlyphMaskBatches) != 1 || len(rc.pendingGlyphMaskBatches[0].Quads) != 2 {
		t.Fatalf("main glyph batches not restored: %d batches", len(rc.pendingGlyphMaskBatches))
	}
	if len(rc.glyphMaskQuadStore) != 2 || rc.glyphMaskQuadStore[0] != (GlyphMaskQuad{X0: 1, Y0: 1, X1: 9, Y1: 9}) {
		t.Fatalf("glyph quad store not restored home: %+v", rc.glyphMaskQuadStore)
	}
	if len(rc.scissorSegments) != 2 || rc.scissorSegments[1].sdfCount != 1 {
		t.Fatalf("main scissor timeline not restored: %+v", rc.scissorSegments)
	}
	if rc.clipRect == nil || *rc.clipRect != rect {
		t.Fatalf("main clipRect not restored")
	}
	if !rc.textBatchSealed {
		t.Fatalf("main text seal flag not restored")
	}
	if !rc.frameRendered {
		t.Fatalf("main frame tracking (LoadOpLoad) not restored")
	}
}

// TestOffscreenPass_SubPassCannotConsumeMain pins the C8 root cause fix:
// queueing during an active sub-pass must never touch main-pass state, even
// across target switches inside the sub-pass (the old shared-stream bug let
// prepareTarget stash main blits together with sub-pass ops).
func TestOffscreenPass_SubPassCannotConsumeMain(t *testing.T) {
	rc := &GPURenderContext{hasPendingTarget: true}
	rc.pendingGPUTextureCommands = []GPUTextureDrawCommand{{DstX: 7}}

	restore := rc.BeginOffscreenPass(render.GPURenderTarget{})

	// Sub-pass lifecycle mirrors recordLocalWith: queue → flush-to-view clears
	// queues → more queueing for a second layer. Main stays untouched behind
	// the suspension barrier throughout.
	rc.pendingImageCommands = append(rc.pendingImageCommands, ImageDrawCommand{DstX: 1})
	rc.pendingImageCommands = rc.pendingImageCommands[:0]
	rc.pendingTextBatches = append(rc.pendingTextBatches, TextBatch{})

	restore()

	if len(rc.pendingGPUTextureCommands) != 1 || rc.pendingGPUTextureCommands[0].DstX != 7 {
		t.Fatalf("main GPU-texture blit consumed by sub-pass: %+v", rc.pendingGPUTextureCommands)
	}
	if len(rc.pendingImageCommands) != 0 && !(len(rc.pendingImageCommands) == 0) {
		t.Fatal("unreachable")
	}
	if len(rc.pendingImageCommands) != 0 {
		t.Fatalf("sub-pass image command leaked into main stream")
	}
	if len(rc.pendingTextBatches) != 0 {
		t.Fatalf("sub-pass text batch leaked into main stream")
	}
}

// appendTestMesh packs points as a fake mesh payload so PackedVerts slice
// identity can survive the suspend/restore round trip.
func (rc *GPURenderContext) appendTestMesh(pts ...render.Point) []byte {
	buf := make([]byte, len(pts)*convexMeshVertexStride)
	for i := range pts {
		off := i * convexMeshVertexStride
		b0 := byte(pts[i].X)
		b1 := byte(pts[i].Y)
		buf[off], buf[off+4] = b0, b1
	}
	return buf
}
