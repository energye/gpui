package rendering_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

func circleRecord(r *scene.PictureRecorder, ox, oy float64) {
	r.FillCircle(ox+40, oy+30, 15, 1, 0, 0, 1)
}

func paintPictureBoxWithCache(t *testing.T, cache *rendering.BoundaryCache, n rendering.RenderObject, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	pc.BoundaryCache, pc.UseBoundaryCache = cache, true
	cache.BeginFrame()
	n.Paint(pc)
	return dc.Image()
}

// TestPictureBox_SkipWhenClean drives the new boundary type: first paint
// records (rerecord=1), second clean paint replays (skip=1, no re-record),
// pixels identical.
func TestPictureBox_SkipWhenClean(t *testing.T) {
	cache := rendering.NewBoundaryCache()
	box := rendering.NewRenderPictureBox(80, 60, circleRecord)
	box.SetRepaintBoundary(true)
	box.Layout(rendering.Tight(80, 60))

	img1 := paintPictureBoxWithCache(t, cache, box, 80, 60)
	rr, skip, _, _ := cache.FrameCounts()
	if rr != 1 || skip != 0 {
		t.Fatalf("first paint rerecord=%d skip=%d want 1/0", rr, skip)
	}
	if !cache.HasValid(box) {
		t.Fatal("boundary must have a valid entry after first paint")
	}

	img2 := paintPictureBoxWithCache(t, cache, box, 80, 60)
	rr, skip, _, _ = cache.FrameCounts()
	if rr != 0 || skip != 1 {
		t.Fatalf("clean repaint rerecord=%d skip=%d want 0/1", rr, skip)
	}
	for _, pt := range [][2]int{{40, 30}, {5, 5}} {
		a1, _, _ := sampleBoxRGB(img1.At(pt[0], pt[1]))
		a2, _, _ := sampleBoxRGB(img2.At(pt[0], pt[1]))
		if a1 != a2 {
			t.Fatalf("replay pixel (%d,%d) differs: %04x vs %04x", pt[0], pt[1], a1, a2)
		}
	}
}

// TestPictureBox_RerecordOnDirty: MarkNeedsPaint forces a re-record, not a skip.
func TestPictureBox_RerecordOnDirty(t *testing.T) {
	cache := rendering.NewBoundaryCache()
	box := rendering.NewRenderPictureBox(80, 60, circleRecord)
	box.SetRepaintBoundary(true)
	box.Layout(rendering.Tight(80, 60))
	paintPictureBoxWithCache(t, cache, box, 80, 60)
	paintPictureBoxWithCache(t, cache, box, 80, 60)

	box.MarkNeedsPaint()
	paintPictureBoxWithCache(t, cache, box, 80, 60)
	rr, skip, _, _ := cache.FrameCounts()
	if rr != 1 || skip != 0 {
		t.Fatalf("dirty repaint rerecord=%d skip=%d want 1/0", rr, skip)
	}
}

// TestPictureBox_RecordLiveEquivalence: box paint equals the same facade
// draws issued directly — replay can never diverge from live paint.
func TestPictureBox_RecordLiveEquivalence(t *testing.T) {
	cache := rendering.NewBoundaryCache()
	box := rendering.NewRenderPictureBox(80, 60, circleRecord)
	box.SetRepaintBoundary(true)
	box.Layout(rendering.Tight(80, 60))
	imgBox := paintPictureBoxWithCache(t, cache, box, 80, 60)

	dc := render.NewContext(80, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	rendering.FillCircle(pc, 40, 30, 15, 1, 0, 0, 1)
	imgDirect := dc.Image()

	for _, pt := range [][2]int{{40, 30}, {55, 30}, {40, 45}, {5, 5}} {
		b1, _, _ := sampleBoxRGB(imgBox.At(pt[0], pt[1]))
		b2, _, _ := sampleBoxRGB(imgDirect.At(pt[0], pt[1]))
		if b1 != b2 {
			t.Fatalf("pixel (%d,%d): box=%04x direct=%04x", pt[0], pt[1], b1, b2)
		}
	}
}

// TestPictureBox_NilRecordNeverCaches: nil Record paints nothing, stores
// nothing, never panics.
func TestPictureBox_NilRecordNeverCaches(t *testing.T) {
	cache := rendering.NewBoundaryCache()
	box := rendering.NewRenderPictureBox(80, 60, nil)
	box.SetRepaintBoundary(true)
	box.Layout(rendering.Tight(80, 60))
	img := paintPictureBoxWithCache(t, cache, box, 80, 60)
	if cache.HasValid(box) {
		t.Fatal("nil-Record boundary must never hold an entry")
	}
	rr, skip, _, _ := cache.FrameCounts()
	if rr != 0 || skip != 0 {
		t.Fatalf("nil paint rerecord=%d skip=%d want 0/0", rr, skip)
	}
	pr, pg, pb := sampleBoxRGB(img.At(40, 30))
	if pr < 0xC000 || pg < 0xC000 || pb < 0xC000 {
		t.Fatalf("center #%04x%04x%04x want untouched white", pr, pg, pb)
	}
}

// TestPictureBox_NonBoundaryDisablesParentCache: a non-boundary picture box
// inside an AbsoluteBox boundary disables that parent's caching (live paint
// still correct) instead of risking a stale bake.
func TestPictureBox_NonBoundaryDisablesParentCache(t *testing.T) {
	cache := rendering.NewBoundaryCache()
	root := rendering.NewAbsoluteBox(80, 60)
	root.SetRepaintBoundary(true)
	child := rendering.NewRenderPictureBox(80, 60, circleRecord)
	root.Place(child, 0, 0)
	root.Layout(rendering.Tight(80, 60))

	img := paintPictureBoxWithCache(t, cache, root, 80, 60)
	if cache.HasValid(root) {
		t.Fatal("parent baking a non-boundary picture box must not cache")
	}
	cr, cg, cb := sampleBoxRGB(img.At(40, 30))
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("center #%04x%04x%04x want live-painted red", cr, cg, cb)
	}
}

// TestPictureBox_LayerRecordsOps: the packet path records picture-box content
// UI-side (no raster-thread RasterExtra defer) with a stable cache key.
func TestPictureBox_LayerRecordsOps(t *testing.T) {
	box := rendering.NewRenderPictureBox(60, 40, func(r *scene.PictureRecorder, ox, oy float64) {
		r.FillCircle(ox+30, oy+20, 12, 0, 0, 1, 1)
	})
	box.MarkNeedsPaint()
	pkt := rendering.BuildLayerTree(box).BuildPacket(1, 1, 100, 80)

	found := false
	scene.Walk(pkt.Root, func(l scene.Layer) {
		pl, ok := l.(*scene.PictureLayer)
		if !ok || pl.CacheKey == 0 {
			return
		}
		for _, op := range pl.Picture.Ops {
			if op.Kind == scene.OpFillCircle {
				found = true
			}
		}
		if pl.RasterExtra != nil {
			t.Error("picture-box layer must record UI-side, no RasterExtra defer")
		}
	})
	if !found {
		t.Fatal("no keyed picture layer carries the recorded circle op")
	}
}

func sampleBoxRGB(c color.Color) (uint32, uint32, uint32) {
	r, g, b, _ := c.RGBA()
	return r, g, b
}
