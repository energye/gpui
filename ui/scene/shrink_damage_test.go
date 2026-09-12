package scene_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/textinput"
)

// testFace resolves the system sans through the same lookup the engine uses
// (no hardcoded font paths): without a shaped face the text texture
// degrades to full-box bounds and per-glyph damage assertions are
// meaningless.
func testFace(t *testing.T) text.Face {
	t.Helper()
	face, _, err := rendering.LoadFaceByFamily("sans-serif", 16)
	if err != nil || face == nil {
		t.Skipf("need system sans font: %v", err)
	}
	return face
}

// textInkDamage returns the text-row damage rects of a frame: the text band
// (not full-box repaints, not the 2px caret bar). Font-independent: the box
// sits at (296,114) so ink lives in y 116..150 and spans ≥5px.
func textInkDamage(rs []image.Rectangle) []image.Rectangle {
	var out []image.Rectangle
	for _, r := range rs {
		if r.Min.Y >= 116 && r.Max.Y <= 150 && r.Dx() >= 5 && r.Dx() <= 200 {
			out = append(out, r)
		}
	}
	return out
}

// coversInk reports whether rects cover want with 1px slack (AA rounding).
func coversInk(rects []image.Rectangle, want image.Rectangle) bool {
	want.Min.X++
	want.Min.Y++
	want.Max.X--
	want.Max.Y--
	for _, d := range rects {
		if d.Min.X <= want.Min.X && d.Min.Y <= want.Min.Y && d.Max.X >= want.Max.X && d.Max.Y >= want.Max.Y {
			return true
		}
	}
	return false
}

// A shrink re-record must damage the cleared tail. Typing "hi" then
// backspace leaves "h": damage that covers only the new (smaller) bounds
// never repaints the removed "i" pixels, so a retained damage present
// (LoadOpLoad) keeps showing "hi" — the IME backspace stall in A/B boxes.
// Damage for a re-recorded layer is previous ∪ current bounds (Flutter
// damage semantics); the transparent-clear path already keeps old-region
// damage via the viewless shell, shrink must do the same.
func TestShrinkRerecordDamagesClearedTail(t *testing.T) {
	ed := textinput.New()
	box := textinput.NewBox(ed, 880, 40, 16)
	box.SetFace(testFace(t))
	root := rendering.NewAbsoluteBox(1200, 800)
	root.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	root.Place(box, 296, 114)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, true)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatalf("focus request failed")
	}
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	dc.BeginFrame()
	// Same honesty gate as the caret test: without an offscreen device
	// every frame vector-replays, so a pass would prove nothing.
	if v, rel := dc.CreateOffscreenTexture(8, 8); v.IsNil() || rel == nil {
		t.Skipf("no GPU offscreen device: retained texture path inactive")
	} else {
		rel()
	}
	tex := scene.NewPictureTextureCache(dc, 256)

	frame := func(id uint64) scene.TexturedStats {
		dpr := dc.DeviceScale()
		if dpr <= 0 {
			dpr = 1
		}
		pkt := rendering.BuildFramePacketWithSaveLayer(root, id, dpr, 1200, 800, nil, nil)
		keys, n := scene.CollectCacheableKeys(pkt)
		tex.EnsureCapacity(n + n/4 + 16)
		tex.SetLiveKeys(keys)
		st := scene.CompositeFramePacketTextured(pkt, dc, tex)
		owner.ConsumeNeedsPaint()
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("frame %d flush: %v", id, err)
		}
		return st
	}

	ed.AddText("hi")
	box.Sync()
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	dc.BeginFrame()
	st1 := frame(1)
	ink1 := textInkDamage(st1.DamageRects)
	if len(ink1) == 0 {
		t.Fatalf("f1 grow: no text damage %v", st1.DamageRects)
	}
	grown := ink1[0]
	for _, r := range ink1[1:] {
		grown = grown.Union(r)
	}

	ed.Backspace()
	box.Sync()
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	dc.BeginFrame()
	st2 := frame(2)
	// The cleared "i" tail sits inside the grown ink: shrink damage
	// (previous ∪ current) must still cover it or LoadOpLoad keeps it.
	if !coversInk(st2.DamageRects, grown) {
		t.Fatalf("f2 shrink damage %v does not cover grown ink %v: cleared tail not repainted", st2.DamageRects, grown)
	}
}

// Clearing the box to empty must still damage the old region. The emptied
// text layer keeps a viewless shell (no texture to blit, no vector ops to
// replay); if the composite skips damage for op-less layers, a retained
// damage present (LoadOpLoad) keeps the stale glyphs until an incidental
// full repaint (A-box IME second backspace: "h"→"" kept showing "h").
func TestClearToEmptyDamagesOldRegion(t *testing.T) {
	ed := textinput.New()
	box := textinput.NewBox(ed, 880, 40, 16)
	box.SetFace(testFace(t))
	root := rendering.NewAbsoluteBox(1200, 800)
	root.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	root.Place(box, 296, 114)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, true)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatalf("focus request failed")
	}
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	dc.BeginFrame()
	if v, rel := dc.CreateOffscreenTexture(8, 8); v.IsNil() || rel == nil {
		t.Skipf("no GPU offscreen device: retained texture path inactive")
	} else {
		rel()
	}
	tex := scene.NewPictureTextureCache(dc, 256)

	frame := func(id uint64) scene.TexturedStats {
		dpr := dc.DeviceScale()
		if dpr <= 0 {
			dpr = 1
		}
		pkt := rendering.BuildFramePacketWithSaveLayer(root, id, dpr, 1200, 800, nil, nil)
		keys, n := scene.CollectCacheableKeys(pkt)
		tex.EnsureCapacity(n + n/4 + 16)
		tex.SetLiveKeys(keys)
		st := scene.CompositeFramePacketTextured(pkt, dc, tex)
		owner.ConsumeNeedsPaint()
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("frame %d flush: %v", id, err)
		}
		return st
	}

	ed.AddText("h")
	box.Sync()
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	dc.BeginFrame()
	st1 := frame(1)
	ink1 := textInkDamage(st1.DamageRects)
	if len(ink1) == 0 {
		t.Fatalf("f1 grow: no text damage %v", st1.DamageRects)
	}
	grown := ink1[0]
	for _, r := range ink1[1:] {
		grown = grown.Union(r)
	}

	// Backspace once: single "h" deleted to empty (production BS2 shape).
	ed.Backspace()
	box.Sync()
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	dc.BeginFrame()
	st2 := frame(2)
	if !coversInk(st2.DamageRects, grown) {
		t.Fatalf("f2 clear damage %v does not cover old ink %v: stale glyph would survive LoadOpLoad", st2.DamageRects, grown)
	}
}
