package gpu

import (
	"image"
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// Empty-clip dispatch (Skia isClipEmpty): draws recorded under an empty clip
// must join an explicitly empty scissor group that dispatch skips — never the
// unclipped (nil-rect, full-target) group. Regression test for the F-box text
// overdraw on window shrink: a box scrolled/clipped fully outside the pixmap
// produced an empty clipStack.Bounds, no segment was recorded, and the blit
// painted with a stale full scissor on full-surface presents.
func TestEmptyClipSegment_BuildsSkippableGroup(t *testing.T) {
	rc := &GPURenderContext{shared: NewGPUShared()}
	target := makeTestTarget(1200, 500)
	view := gpucontext.NewTextureView(unsafe.Pointer(new(int)))

	// Explicit empty clip, as Context.setGPUClipRect records for empty bounds.
	rc.SetClipRect(240, 540, 0, 0)
	rc.QueueGPUTextureDraw(target, view, 240, 127, 858, 498, 1.0, 1200, 500)
	rc.ClearClipRect()
	// Contrast: an unclipped draw joins the full-target group.
	rc.QueueGPUTextureDraw(target, view, 0, 0, 1200, 50, 1.0, 1200, 500)

	groups := rc.buildScissorGroups()
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2 (empty + full)", len(groups))
	}
	g0 := groups[0]
	if g0.Rect == nil {
		t.Fatalf("clipped draw joined the unclipped group (segment lost)")
	}
	if g0.Rect[2] != 0 || g0.Rect[3] != 0 {
		t.Fatalf("clipped group rect = %v, want zero area", *g0.Rect)
	}
	if len(g0.GPUTextureCommands) != 1 {
		t.Fatalf("clipped group draws = %d, want 1", len(g0.GPUTextureCommands))
	}
	g1 := groups[1]
	if g1.Rect != nil {
		t.Fatalf("unclipped draw joined rect group %v, want full (nil)", *g1.Rect)
	}
	if len(g1.GPUTextureCommands) != 1 {
		t.Fatalf("full group draws = %d, want 1", len(g1.GPUTextureCommands))
	}
}

// The dispatch decision itself must not touch the render pass for skips:
// nil encoder is safe exactly when the group is skipped.
func TestApplyGroupScissor_EmptyRectSkips(t *testing.T) {
	s := &GPURenderSession{}
	empty := &[4]uint32{240, 540, 0, 0}
	if s.applyGroupScissor(nil, empty, 1200, 800) {
		t.Fatalf("applyGroupScissor(empty) = true, want false (skip)")
	}
	if s.applyGroupScissorWithDamageRects(nil, empty, 1200, 800, nil) {
		t.Fatalf("damageRects(empty group, no damage) = true, want false")
	}
	dmg := []image.Rectangle{image.Rect(0, 728, 1200, 800)}
	if s.applyGroupScissorWithDamageRects(nil, empty, 1200, 800, dmg) {
		t.Fatalf("damageRects(empty group, HUD damage) = true, want false")
	}
	// NOTE: non-empty groups need a real encoder (SetScissorRect touches GPU);
	// the draw path is covered by the true-window shrink verification.
}
