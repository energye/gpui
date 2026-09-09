package scene_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/textinput"
)

// A transparent re-record must never leave stale pixels behind. The caret bar
// re-records every blink; its OFF frames replay a single zero-alpha FillRect,
// which queues no GPU work — the reused texture slot would otherwise keep the
// previous ON pixels while the entry still reports recorded (caret stuck on).
// The cache drops fully-transparent re-records instead, so the composite
// falls back to vector replay (which correctly draws nothing).
func TestCaretTransparentRerecordClearsStale(t *testing.T) {
	ed := textinput.New()
	box := textinput.NewMultiLineInputBox(ed, 880, 90, 16)
	root := rendering.NewAbsoluteBox(1200, 800)
	root.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	root.Place(box, 296, 312)
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
	// The stale-slot bug only exists on the GPU texture path: without an
	// offscreen device every frame vector-replays (trivially correct), so a
	// pass here would prove nothing — skip honestly instead of fake-green.
	if v, rel := dc.CreateOffscreenTexture(8, 8); v.IsNil() || rel == nil {
		t.Skipf("no GPU offscreen device: retained texture path inactive")
	} else {
		rel()
	}
	tex := scene.NewPictureTextureCache(dc, 256)

	carets := func(img image.Image) []int {
		var out []int
		for x := 296; x < 1176; x++ {
			n := 0
			for y := 312; y < 402; y += 2 {
				r, g, _, _ := img.At(x, y).RGBA()
				if r > 0x9600 && g > 0x9600 {
					n++
				}
			}
			if n >= 6 {
				out = append(out, x)
			}
		}
		return out
	}
	frame := func(id uint64) (img image.Image, bar uint64, st scene.TexturedStats) {
		dpr := dc.DeviceScale()
		if dpr <= 0 {
			dpr = 1
		}
		pkt := rendering.BuildFramePacketWithSaveLayer(root, id, dpr, 1200, 800, nil, nil)
		// The caret bar is the only thin-tall FillRect layer (1.5x24).
		scene.Walk(pkt.Root, func(l scene.Layer) {
			pl, ok := l.(*scene.PictureLayer)
			if !ok {
				return
			}
			for i := range pl.Picture.Ops {
				op := &pl.Picture.Ops[i]
				if op.Kind == scene.OpFillRect && op.W <= 4 && op.H <= 26 {
					bar = pl.CacheKey
				}
			}
		})
		keys, n := scene.CollectCacheableKeys(pkt)
		tex.EnsureCapacity(n + n/4 + 16)
		tex.SetLiveKeys(keys)
		st = scene.CompositeFramePacketTextured(pkt, dc, tex)
		owner.ConsumeNeedsPaint()
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("frame %d flush: %v", id, err)
		}
		return dc.Image(), bar, st
	}

	box.Sync()
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	dc.BeginFrame()
	img1, _, _ := frame(1)
	if got := carets(img1); len(got) == 0 {
		t.Fatalf("f1 fresh: expected caret on, got none")
	}
	// Type so both ring slots get opaque-burned, then blink OFF: before the
	// fix this exact sequence left stale caret pixels on the reused slot.
	ed.AddText("q")
	box.TickCaret(0.6)
	box.TickCaret(0.6)
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	dc.BeginFrame()
	img2, bar, _ := frame(2)
	if got := carets(img2); len(got) == 0 {
		t.Fatalf("f2 typed on: expected caret on, got none")
	}
	if bar == 0 {
		t.Fatalf("f2: caret bar entry not found among live keys")
	}
	if !tex.Has(bar) {
		t.Fatalf("f2: caret bar has no live texture (test needs the GPU path)")
	}
	box.TickCaret(0.6)
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	dc.BeginFrame()
	img3, _, st3 := frame(3)
	if got := carets(img3); len(got) > 0 {
		t.Fatalf("f3 typed off: stale caret pixels remain: %v", got)
	}
	if tex.Has(bar) {
		t.Fatalf("f3: transparent re-record kept a live bar texture")
	}
	// The cleared region must be reported as damage, otherwise a retained
	// damage present (LoadOpLoad) keeps the old caret pixels on screen.
	if len(st3.DamageRects) == 0 {
		t.Fatalf("f3: no damage reported for the cleared caret region")
	}
	// Extra blink cycles: OFF frames must always be clean, ON frames lit.
	for i := 0; i < 4; i++ {
		box.TickCaret(0.6)
		owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
		dc.BeginFrame()
		img, _, _ := frame(uint64(4 + i)) //nolint:gosec // small loop index
		if box.IsCaretOn() {
			if got := carets(img); len(got) == 0 {
				t.Fatalf("blink %d on: expected caret, got none", i)
			}
			continue
		}
		if got := carets(img); len(got) > 0 {
			t.Fatalf("blink %d off: stale caret pixels remain: %v", i, got)
		}
	}
}
