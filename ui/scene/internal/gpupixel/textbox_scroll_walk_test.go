package gpupixel

// Scrolled text bands must show the rows at the CURRENT scroll, never a
// stale recording: pasting long text and walking through all content
// (head -> quarters -> middle -> end) must paint live rows at every stop
// on both single-line viewport and multi-line wrap boxes (B/F middle-blank:
// the tail band kept blitting at a middle scroll).
//
// Stale-entry guard lives one layer down (TestRecordLocal_RefusalEvictsStaleEntry);
// this file pins the end-to-end pixel behavior through the retained composite.

import (
	"fmt"
	"image"
	"os"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/textinput"
)

func scrollFace(t *testing.T) text.Face {
	t.Helper()
	b, err := os.ReadFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	if err != nil {
		t.Skipf("DejaVu unavailable: %v", err)
	}
	src, err := text.NewFontSource(b)
	if err != nil {
		t.Skipf("font source: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	face := src.Face(16)
	if face == nil {
		t.Skip("face nil")
	}
	return face
}

func inkAt(img image.Image, x, y int) bool {
	r, g, _, _ := img.At(x, y).RGBA()
	return int(g>>8) > 120 && int(r>>8) < 150
}

// viewInk counts ink in left/mid/right thirds plus per-row coverage of the
// box view. Every third and (for full multi-line boxes) every row band must
// carry ink: a zero third/row is a blank gap in viewed content.
func viewInk(t *testing.T, tag string, img image.Image, ox, oy, ow, oh, rows int) (thirds [3]int, blankRows int) {
	t.Helper()
	for y := oy; y < oy+oh; y++ {
		for x := ox; x < ox+ow; x++ {
			if inkAt(img, x, y) {
				thirds[min(x-ox, ow-1)*3/ow]++
			}
		}
	}
	if rows > 0 {
		for i := 0; i < rows; i++ {
			y0, y1 := oy+i*oh/rows, oy+(i+1)*oh/rows
			found := false
			for y := y0; y < y1 && !found; y++ {
				for x := ox; x < ox+ow; x++ {
					if inkAt(img, x, y) {
						found = true
						break
					}
				}
			}
			if !found {
				blankRows++
			}
		}
	}
	t.Logf("%s: thirds=%v blankRows=%d/%d", tag, thirds, blankRows, rows)
	return thirds, blankRows
}

// walkScroll presents one persistent texture cache across scroll stops, like
// a real window: each stop composites 3 frames, and the final frame must
// show that stop's rows (ink in every third/row) AND differ from a
// different stop's frame (scroll reuse must not freeze the picture).
func walkScroll(t *testing.T, root rendering.RenderObject, W, H int, stops []struct {
	tag  string
	move func()
	ox   int
	oy   int
	ow   int
	oh   int
	rows int
},
) {
	t.Helper()
	dc := render.NewContext(W, H)
	defer dc.Close()
	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	scene.ResetLayerIDGen()
	tex := scene.NewPictureTextureCache(dc, 64)
	frame := 0
	present := func() image.Image {
		frame++
		pkt := rendering.BuildFramePacketWithSaveLayer(root, uint64(frame), 1, float64(W), float64(H), nil, nil)
		keys, n := scene.CollectCacheableKeys(pkt)
		tex.EnsureCapacity(n + n/4 + 16)
		tex.SetLiveKeys(keys)
		scene.CompositeFramePacketTextured(pkt, dc, tex)
		if err := dc.FlushGPUWithView(view, uint32(W), uint32(H)); err != nil {
			t.Fatalf("frame %d flush: %v", frame, err)
		}
		out := render.NewContext(W, H)
		defer out.Close()
		out.ClearWithColor(render.Black)
		out.DrawGPUTexture(view, 0, 0, W, H)
		if err := out.FlushGPU(); err != nil {
			t.Fatalf("composite: %v", err)
		}
		return out.Image()
	}
	for i := 0; i < 3; i++ {
		present()
	}
	var prevImg image.Image
	for _, st := range stops {
		st.move()
		var img image.Image
		for i := 0; i < 3; i++ {
			img = present()
		}
		thirds, blank := viewInk(t, st.tag, img, st.ox, st.oy, st.ow, st.oh, st.rows)
		for i, n := range thirds {
			if n == 0 {
				t.Fatalf("%s: third %d has zero ink (blank gap in viewed content)", st.tag, i)
			}
		}
		if st.rows > 0 && blank > 1 {
			t.Fatalf("%s: %d blank rows in view", st.tag, blank)
		}
		if prevImg != nil {
			same := true
			for y := st.oy; y < st.oy+st.oh && same; y += 2 {
				for x := st.ox; x < st.ox+st.ow && same; x += 2 {
					r1, g1, b1, _ := prevImg.At(x, y).RGBA()
					r2, g2, b2, _ := img.At(x, y).RGBA()
					if r1 != r2 || g1 != g2 || b1 != b2 {
						same = false
					}
				}
			}
			if same {
				t.Fatalf("%s: frame identical to previous stop (scroll did not move the picture)", st.tag)
			}
		}
		prevImg = img
	}
}

func TestScrollWalk_WrapBoxShowsLiveRows(t *testing.T) {
	requireGPU(t)
	face := scrollFace(t)
	ed := textinput.New()
	box := textinput.NewMultiLineInputBox(ed, 300, 120, 16)
	box.SetFace(face)
	box.SetWrap(true)
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "line %03d abcdefghijklmnop\n", i)
	}
	full := sb.String()
	n := len([]rune(full))
	sel := func(base int) func() {
		return func() {
			ed.SetSelection(textinput.TextRange{Base: base, Extent: base})
			box.Sync()
		}
	}
	stops := []struct {
		tag  string
		move func()
		ox   int
		oy   int
		ow   int
		oh   int
		rows int
	}{
		{"paste-end", func() { ed.SetTextSimple(full); box.Sync() }, 0, 0, 300, 120, 5},
		{"head", sel(0), 0, 0, 300, 120, 5},
		{"q1", sel(n / 4), 0, 0, 300, 120, 5},
		{"mid", sel(n / 2), 0, 0, 300, 120, 5},
		{"q3", sel(3 * n / 4), 0, 0, 300, 120, 5},
		{"end", sel(n), 0, 0, 300, 120, 5},
	}
	walkScroll(t, rendering.RenderObject(box), 320, 140, stops)
}

func TestScrollWalk_SingleBoxShowsLiveSlice(t *testing.T) {
	requireGPU(t)
	face := scrollFace(t)
	ed := textinput.New()
	box := textinput.NewViewportInputBox(ed, 300, 40, 16)
	box.SetFace(face)
	full := strings.Repeat("ab", 2000)
	n := len([]rune(full))
	sel := func(base int) func() {
		return func() {
			ed.SetSelection(textinput.TextRange{Base: base, Extent: base})
			box.Sync()
		}
	}
	stops := []struct {
		tag  string
		move func()
		ox   int
		oy   int
		ow   int
		oh   int
		rows int
	}{
		{"paste-end", func() { ed.SetTextSimple(full); box.Sync() }, 0, 0, 300, 40, 0},
		{"head", sel(0), 0, 0, 300, 40, 0},
		{"mid", sel(n / 2), 0, 0, 300, 40, 0},
		{"end", sel(n), 0, 0, 300, 40, 0},
	}
	walkScroll(t, rendering.RenderObject(box), 320, 60, stops)
}

// A window resize (outer FixedWidth change + MarkNeedsLayout, no edit) must
// widen the painted box: border, background and text window all follow in
// the presented frames. Regression for the accept window staying 880-wide
// on paint while layout already reported 1080.
func TestResize_WidensPaintedBox(t *testing.T) {
	requireGPU(t)
	face := scrollFace(t)
	ed := textinput.New()
	box := textinput.NewViewportInputBox(ed, 300, 40, 16)
	box.SetFace(face)
	box.SetTextColor(0.9, 0.9, 0.9, 1)
	ed.SetTextSimple(strings.Repeat("ab", 500))
	box.Sync()

	const W, H = 560, 60
	// Window shape: the box is a placed child of a fixed panel (absolute
	// offsets keep their own size; a bare root would stretch to constraints).
	shell := rendering.NewAbsoluteBox(W, H)
	shell.Place(rendering.RenderObject(box), 0, 0)
	// Window pipeline order: owner layout, owner paint, packet build.
	pipe := rendering.NewPipelineOwner(rendering.RenderObject(shell))
	pipe.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	pipe.FlushPaint(&rendering.PaintContext{}, true)
	dc := render.NewContext(W, H)
	defer dc.Close()
	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()
	scene.ResetLayerIDGen()
	tex := scene.NewPictureTextureCache(dc, 64)
	frame := 0
	present := func() image.Image {
		frame++
		pkt := rendering.BuildFramePacketWithSaveLayer(rendering.RenderObject(box), uint64(frame), 1, float64(W), float64(H), nil, nil)
		keys, n := scene.CollectCacheableKeys(pkt)
		tex.EnsureCapacity(n + n/4 + 16)
		tex.SetLiveKeys(keys)
		scene.CompositeFramePacketTextured(pkt, dc, tex)
		if err := dc.FlushGPUWithView(view, uint32(W), uint32(H)); err != nil {
			t.Fatalf("frame %d flush: %v", frame, err)
		}
		out := render.NewContext(W, H)
		defer out.Close()
		out.ClearWithColor(render.Black)
		out.DrawGPUTexture(view, 0, 0, W, H)
		if err := out.FlushGPU(); err != nil {
			t.Fatalf("composite: %v", err)
		}
		return out.Image()
	}
	borderSpan := func(img image.Image) (lo, hi int) {
		lo, hi = -1, -1
		for y := 0; y < 4; y++ {
			for x := 0; x < W; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				rr, gg, bb := int(r>>8), int(g>>8), int(b>>8)
				if rr >= 80 && rr < 115 && gg >= 95 && gg < 120 && bb >= 115 && bb < 140 {
					if lo < 0 {
						lo = x
					}
					hi = x
				}
			}
		}
		return lo, hi
	}
	for i := 0; i < 3; i++ {
		present()
	}
	img := present()
	lo, hi := borderSpan(img)
	t.Logf("before resize: border x %d..%d", lo, hi)
	if hi-lo < 290 {
		t.Fatalf("before resize border span %d..%d want ~300 wide", lo, hi)
	}
	// Resize with no edit, exactly like the example's setBoxSize, then the
	// window's per-frame passes in window order (layout, packet build,
	// composite; paint-flag clearing happens after composite, never before
	// the build that must observe it).
	box.FixedWidth = 540
	box.MarkNeedsLayout()
	pipe.FlushLayout(rendering.Size{Width: W, Height: H}, false)
	t.Logf("after FlushLayout: fixed=%.0f size=%.0fx%.0f",
		box.FixedWidth, box.Size().Width, box.Size().Height)
	for i := 0; i < 3; i++ {
		img = present()
	}
	lo, hi = borderSpan(img)
	t.Logf("after resize: border x %d..%d", lo, hi)
	if hi-lo < 530 {
		t.Fatalf("REPRODUCED: after resize border span %d..%d want ~540 wide", lo, hi)
	}
}
