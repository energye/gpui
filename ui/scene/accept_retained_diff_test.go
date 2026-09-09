package scene_test

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/textinput"
)

func acceptBoxB(t *testing.T) (*rendering.AbsoluteBox, *rendering.PipelineOwner, *textinput.ViewportInputBox) {
	t.Helper()
	wrkit.EnsureUIFace()
	face := wrkit.FaceAt(16)
	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "ui_text_edit_accept", "testdata", "b_5000.txt"))
	if err != nil {
		t.Fatalf("read b_5000: %v", err)
	}
	ed := textinput.New()
	box := textinput.NewViewportInputBox(ed, 880, 40, 16)
	if face != nil {
		box.SetFace(face)
	}
	ed.SetTextSimple(string(raw))
	root := rendering.NewAbsoluteBox(1200, 800)
	root.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	root.Place(box, 296, 180)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, true)
	return root, owner, box
}

func compositeFrame(t *testing.T, dc *render.Context, tex *scene.PictureTextureCache, owner *rendering.PipelineOwner, root rendering.RenderObject, frameID uint64) image.Image {
	t.Helper()
	dpr := dc.DeviceScale()
	if dpr <= 0 {
		dpr = 1
	}
	pkt := rendering.BuildFramePacketWithSaveLayer(root, frameID, dpr, 1200, 800, nil, nil)
	keys, n := scene.CollectCacheableKeys(pkt)
	tex.EnsureCapacity(n + n/4 + 16)
	tex.SetLiveKeys(keys)
	scene.CompositeFramePacketTextured(pkt, dc, tex)
	owner.ConsumeNeedsPaint()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("frame %d flush: %v", frameID, err)
	}
	return dc.Image()
}

func vectorFrame(t *testing.T, owner *rendering.PipelineOwner, root rendering.RenderObject) image.Image {
	t.Helper()
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	dc.BeginFrame()
	pc := rendering.NewPaintContext(dc, dc.DeviceScale())
	owner.FlushPaint(pc, true)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("vector flush: %v", err)
	}
	return dc.Image()
}

// pixDiff counts pixels differing by more than tol (8-bit levels) inside r
// and returns the differing bounding box.
func pixDiff(a, b image.Image, r image.Rectangle, tol uint32) (int, image.Rectangle) {
	n := 0
	var bb image.Rectangle
	first := true
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb2, _ := b.At(x, y).RGBA()
			var d uint32
			for _, dd := range []uint32{sub(ar, br), sub(ag, bg), sub(ab, bb2)} {
				if dd > d {
					d = dd
				}
			}
			if d > tol*257 {
				n++
				if first {
					bb = image.Rect(x, y, x+1, y+1)
					first = false
				} else {
					bb = bb.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
	}
	return n, bb
}

func sub(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// Static B tail: the retained steady composite must cover the box exactly
// like the full vector repaint (no holes, no shifted text — the "static
// blur/overflow that resize heals" class). Pixel-equality cannot hold:
// records bake DrawString substrings into textures while the vector path
// submits shaped glyphs, so edge AA differs. Assert structure instead: flat
// background/border pixels match exactly; ink rows/cols match position.
func TestRetainedMatchesVector_BTail(t *testing.T) {
	root, owner, _ := acceptBoxB(t)
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	dc.BeginFrame()
	tex := scene.NewPictureTextureCache(dc, 256)
	t1 := compositeFrame(t, dc, tex, owner, root, 1)
	if tex.Len() == 0 {
		t.Skipf("no GPU texture entries (no GPU?)")
	}
	t2 := compositeFrame(t, dc, tex, owner, root, 2)
	vec := vectorFrame(t, owner, root)
	box := image.Rect(296, 180, 1176, 220)
	assertStructEqual(t, t1, vec, box)
	assertStructEqual(t, t2, vec, box)
}

// assertStructEqual checks coverage-correctness of a retained composite
// against the vector repaint inside r: pixels both sides agree are
// background must match exactly (holes fail here); every ink pixel on
// either side must have ink within 2px on the other (shifts/spills fail).
// The 2px window absorbs baked-texture resampling vs direct-glyph AA.
func assertStructEqual(t *testing.T, got, want image.Image, r image.Rectangle) {
	t.Helper()
	const tol = 2 * 257
	isInk := func(im image.Image, x, y int) bool {
		rr, _, bb, _ := im.At(x, y).RGBA()
		return int(bb)-int(rr) > 60*257
	}
	at := func(m map[[2]int]bool, x, y int) bool { return m[[2]int{x, y}] }
	inkW := map[[2]int]bool{}
	inkG := map[[2]int]bool{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if isInk(want, x, y) {
				inkW[[2]int{x, y}] = true
			}
			if isInk(got, x, y) {
				inkG[[2]int{x, y}] = true
			}
		}
	}
	near := func(m map[[2]int]bool, x, y int) bool {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				if at(m, x+dx, y+dy) {
					return true
				}
			}
		}
		return false
	}
	// Text zone (either side's ink ±2px): AA halo distribution differs
	// between baked-texture resampling and direct glyph coverage, so exact
	// equality cannot hold there. Outside the zone pixels must match.
	// Inside: scattered sub-threshold pixels get a small budget, but a dense
	// 8px cell of misses/spills (a dropped or shifted run) still fails.
	bgDiff := 0
	var bgBB image.Rectangle
	firstBg := true
	missCover, spill := 0, 0
	missCell := map[[2]int]int{}
	spillCell := map[[2]int]int{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			w, g := at(inkW, x, y), at(inkG, x, y)
			if near(inkW, x, y) || near(inkG, x, y) {
				if w && !near(inkG, x, y) {
					missCover++
					missCell[[2]int{x / 8, y / 8}]++
				}
				if g && !near(inkW, x, y) {
					spill++
					spillCell[[2]int{x / 8, y / 8}]++
				}
				continue
			}
			ar, ag, ab, _ := got.At(x, y).RGBA()
			br, bg, bb, _ := want.At(x, y).RGBA()
			d := sub(ar, br)
			if dd := sub(ag, bg); dd > d {
				d = dd
			}
			if dd := sub(ab, bb); dd > d {
				d = dd
			}
			if d > tol {
				bgDiff++
				if firstBg {
					bgBB = image.Rect(x, y, x+1, y+1)
					firstBg = false
				} else {
					bgBB = bgBB.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
	}
	if bgDiff > 0 {
		t.Fatalf("background/border differ: %d px bbox=%v", bgDiff, bgBB)
	}
	for c, n := range missCell {
		if n > 32 {
			t.Fatalf("ink missing in cell %v: %d px (total %d)", c, n, missCover)
		}
	}
	for c, n := range spillCell {
		if n > 32 {
			t.Fatalf("ink spills in cell %v: %d px (total %d)", c, n, spill)
		}
	}
	if missCover > 600 {
		t.Fatalf("ink missing beyond 2px: %d px", missCover)
	}
	if spill > 600 {
		t.Fatalf("ink spills beyond 2px: %d px", spill)
	}
}

// Focused B caret toggle: exactly the caret pixels may change between the
// two composites; anything else is a retained stale/misplaced blit (the
// "blink flashes other regions" / "other boxes never blink" class).
func TestRetainedCaretToggle_BOnly(t *testing.T) {
	root, owner, box := acceptBoxB(t)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatalf("B focus request failed")
	}
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	dc.BeginFrame()
	tex := scene.NewPictureTextureCache(dc, 256)
	on := compositeFrame(t, dc, tex, owner, root, 1)
	if tex.Len() == 0 {
		t.Skipf("no GPU texture entries (no GPU?)")
	}
	if !box.IsCaretOn() {
		t.Fatalf("focused B caret should start visible")
	}
	box.TickCaret(0.6)
	if box.IsCaretOn() {
		t.Fatalf("B caret should toggle off after 600ms")
	}
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	off := compositeFrame(t, dc, tex, owner, root, 2)
	boxR := image.Rect(296, 180, 1176, 220)
	n, bb := pixDiff(on, off, boxR, 2)
	if n == 0 {
		t.Fatalf("caret toggle changed nothing on screen")
	}
	allow := image.Rect(1150, 180, 1176, 220)
	if !bb.In(allow) {
		t.Fatalf("caret toggle moved %d px outside caret area bbox=%v", n, bb)
	}
	rest := image.Rect(0, 0, 1200, 800)
	nAll, bbAll := pixDiff(on, off, rest, 2)
	if nAll != n || bbAll != bb {
		t.Fatalf("caret toggle leaked outside B box: all=%d bbox=%v box=%d bbox=%v", nAll, bbAll, n, bb)
	}
}
