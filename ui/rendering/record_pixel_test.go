//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering_test

import (
	"fmt"
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// 节点录制覆盖的像素断言（ENGINE_NODE_RECORD_COVERAGE_PLAN §6 步 2）：
// 每个节点类型都必须证明「录制回放」与「活画」逐像素一致——不是只看计数器。
// 目录统一 120×90、根是白底 RepaintBoundary；被测节点作为根的非边界子节点，
// 走父边界的录制路径（这正是生产里边界包含子节点的形态）。
const (
	rpW = 120
	rpH = 90
)

func rpScene(node rendering.RenderObject) *rendering.AbsoluteBox {
	root := rendering.NewAbsoluteBox(rpW, rpH)
	root.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	root.SetRepaintBoundary(true)
	if node != nil {
		root.Place(node, 20, 20)
	}
	_ = root.Layout(rendering.Tight(rpW, rpH))
	return root
}

// rpPaint renders one frame; cache nil means the pure live path.
func rpPaint(t *testing.T, root rendering.RenderObject, cache *rendering.BoundaryCache) image.Image {
	t.Helper()
	dc := render.NewContext(rpW, rpH)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	if cache != nil {
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
	}
	root.Paint(pc)
	return dc.Image()
}

// rpPixelDiff counts mismatches with an EXPLICIT tolerance, never a mean:
// a channel may differ by at most 3/255, the rasterizer's anti-aliasing /
// curve-reconstruction error when a stroke path is recorded and replayed. Any
// larger delta is a real content difference and fails. The count is exact, so
// a few near-tie AA pixels cannot dilute a wrong region.
func rpPixelDiff(want, got image.Image) (bad int, near int, first string) {
	if want.Bounds() != got.Bounds() {
		return -1, 0, fmt.Sprintf("bounds %v vs %v", want.Bounds(), got.Bounds())
	}
	// 3/255 covers the recorder's arc→cubic reconstruction of rounded-rect
	// strokes (live uses analytic corners); measured worst here is 3/255,
	// while solid fills stay exact (0 delta).
	const aaTie = 0x303
	b := want.Bounds()
	worstD := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r1, g1, b1, a1 := want.At(x, y).RGBA()
			r2, g2, b2, a2 := got.At(x, y).RGBA()
			d := maxDelta(r1, r2)
			d = maxInt(d, maxDelta(g1, g2))
			d = maxInt(d, maxDelta(b1, b2))
			d = maxInt(d, maxDelta(a1, a2))
			if d == 0 {
				continue
			}
			if d <= aaTie {
				near++
				continue
			}
			bad++
			if d > worstD {
				worstD = d
				first = fmt.Sprintf("(%d,%d) live=#%04x%04x%04x%04x replay=#%04x%04x%04x%04x delta=%d",
					x, y, r1, g1, b1, a1, r2, g2, b2, a2, d)
			}
		}
	}
	return bad, near, first
}

func maxDelta(a, b uint32) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func rpDiff(t *testing.T, name string, live, replay image.Image) {
	t.Helper()
	if bad, near, worst := rpPixelDiff(live, replay); bad != 0 {
		t.Fatalf("%s: replay differs from live at %d px beyond the 3/255 AA tie (%d px at the tie), worst %s",
			name, bad, near, worst)
	}
}

// rpAssertCacheable proves the full contract for a cacheable node:
// live == first record == clean replay, and the replay really skipped.
func rpAssertCacheable(t *testing.T, name string, root rendering.RenderObject) {
	t.Helper()
	cache := rendering.NewBoundaryCache()
	live := rpPaint(t, root, nil)
	first := rpPaint(t, root, cache)
	rpDiff(t, name+" record-vs-live", live, first)
	if !cache.HasValid(root) {
		t.Fatalf("%s: boundary must hold a recording after the first paint", name)
	}
	if st := cache.Stats(); st.FrameRerecord != 1 || st.FrameSkip != 0 {
		t.Fatalf("%s: first paint rerecord=%d skip=%d want 1/0", name, st.FrameRerecord, st.FrameSkip)
	}
	replay := rpPaint(t, root, cache)
	if st := cache.Stats(); st.FrameRerecord != 0 || st.FrameSkip != 1 {
		t.Fatalf("%s: clean paint rerecord=%d skip=%d want 0/1", name, st.FrameRerecord, st.FrameSkip)
	}
	rpDiff(t, name+" replay-vs-live", live, replay)
}

// rpAssertRefused proves a boundary containing a non-recordable node falls back
// to live paint (honest degradation, never a stale bake).
func rpAssertRefused(t *testing.T, name string, root rendering.RenderObject) {
	t.Helper()
	cache := rendering.NewBoundaryCache()
	live := rpPaint(t, root, nil)
	first := rpPaint(t, root, cache)
	second := rpPaint(t, root, cache)
	if cache.HasValid(root) {
		t.Fatalf("%s: boundary must refuse caching", name)
	}
	if st := cache.Stats(); st.FrameRerecord != 0 {
		t.Fatalf("%s: refused boundary rerecord=%d want 0", name, st.FrameRerecord)
	}
	rpDiff(t, name+" first-live", live, first)
	rpDiff(t, name+" second-live", live, second)
}

func TestRecordCoverage_ContainerAndLeafTypes(t *testing.T) {
	face, _, _ := rendering.LoadFaceByFamily("sans-serif", 18)

	img, err := render.NewImageBuf(24, 24, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer img.Dispose()
	img.Fill(200, 40, 40, 255)
	img.NotifyPixelsChanged()

	mkText := func() *rendering.RenderText {
		txt := rendering.NewRenderText("录制 Replay")
		txt.FontSize = 18
		txt.MaxWidth = 90
		if face != nil {
			txt.SetFace(face)
		}
		return txt
	}

	mkImage := func() *rendering.RenderImage {
		im := rendering.NewRenderImage(48, 36)
		im.SetImageShared(img)
		return im
	}

	mkAlign := func() *rendering.RenderAlignBox {
		a := rendering.NewRenderAlignBox(rendering.NewRenderColorBox(20, 10, 0.15, 0.65, 0.35, 1), 0.75, 0.25)
		return a
	}

	mkClip := func() *rendering.RenderClipRRect {
		c := rendering.NewRenderClipRRect(rendering.NewRenderColorBox(60, 40, 0.85, 0.2, 0.2, 1))
		c.FixedWidth, c.FixedHeight = 40, 30
		c.Radius = 10
		return c
	}

	mkOpacity := func() *rendering.RenderOpacity {
		red := rendering.NewRenderColorBox(40, 30, 0.9, 0.1, 0.1, 1)
		blue := rendering.NewRenderColorBox(40, 30, 0.1, 0.2, 0.9, 1)
		return rendering.NewRenderOpacity(0.5, red, blue)
	}

	mkColorFilter := func() *rendering.RenderColorFilter {
		return rendering.NewRenderGrayscale(rendering.NewRenderColorBox(40, 30, 0.9, 0.15, 0.15, 1))
	}

	mkImageFilter := func() *rendering.RenderImageFilter {
		return rendering.NewRenderImageFilter(4, rendering.NewRenderColorBox(40, 30, 0.85, 0.2, 0.2, 1))
	}

	mkTransform := func() *rendering.RenderTransform {
		tr := rendering.NewRenderTransform(rendering.NewRenderColorBox(40, 30, 0.2, 0.4, 0.85, 1))
		tr.FixedWidth, tr.FixedHeight = 60, 40
		tr.SetRotation(0.45)
		tr.SetScale(0.9, 0.8)
		return tr
	}

	mkBoxPaint := func() *rendering.RenderBox {
		b := rendering.NewRenderBox()
		b.FixedWidth, b.FixedHeight = 50, 30
		b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			rendering.FillRect(pc, 5, 5, size.Width-10, size.Height-10, 0.9, 0.35, 0.1, 1)
			rendering.StrokeRoundRect(pc, 5, 5, size.Width-10, size.Height-10, 4, 1.5, 0.1, 0.15, 0.5, 1)
		}
		return b
	}

	cases := []struct {
		name string
		node rendering.RenderObject
	}{
		{"ColorBox", rendering.NewRenderColorBox(40, 30, 0.1, 0.45, 0.9, 1)},
		{"Text", mkText()},
		{"Image", mkImage()},
		{"PictureBox", rendering.NewRenderPictureBox(50, 40, func(dc render.Canvas, ox, oy float64) {
			dc.SetRGBA(0.1, 0.55, 0.2, 1)
			dc.DrawCircle(ox+25, oy+20, 12)
			_ = dc.Fill()
		})},
		{"BoxOnPaint", mkBoxPaint()},
		{"AlignBox", mkAlign()},
		{"ClipRRect", mkClip()},
		{"Opacity", mkOpacity()},
		{"ColorFilter", mkColorFilter()},
		{"ImageFilter", mkImageFilter()},
		{"Transform", mkTransform()},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "Text" && face == nil {
				t.Skip("no system face for text recording coverage")
			}
			rpAssertCacheable(t, c.name, rpScene(c.node))
		})
	}
}

// TestRecordCoverage_VolatileSpinnerStaysLive: Spinner declares Volatile, so it
// must never own a boundary recording; changing its phase between frames must
// change pixels (proof the second frame was live paint, not a replayed bake).
func TestRecordCoverage_VolatileSpinnerStaysLive(t *testing.T) {
	sp := rendering.NewRenderSpinner(28)
	root := rpScene(sp)
	cache := rendering.NewBoundaryCache()

	first := rpPaint(t, root, cache)
	if cache.HasValid(sp) {
		t.Fatal("volatile spinner must never hold a recording")
	}
	// Off-grid phase: quarter/half turns are rotation-symmetric for this
	// four-dot spinner and would look identical by design.
	sp.SetPhase(0.1)
	second := rpPaint(t, root, cache)
	if cache.HasValid(sp) {
		t.Fatal("volatile spinner must never hold a recording (second frame)")
	}
	if bad, _, _ := rpPixelDiff(first, second); bad <= 0 {
		t.Fatal("spinner phase change produced identical pixels — second frame was not live paint")
	}
	live := rpPaint(t, root, nil)
	rpDiff(t, "Spinner live-vs-cached-walk", live, second)
}

// TestRecordCoverage_VideoAndViewportRefuseParentBake: volatile video makes its
// enclosing boundary refuse caching; the structural viewport is never its own
// cache unit (its content layer is), and a non-boundary viewport cannot be
// baked by a parent either.
func TestRecordCoverage_VideoAndViewportRefuseParentBake(t *testing.T) {
	t.Run("Video", func(t *testing.T) {
		v := rendering.NewRenderVideo(40, 30, nil)
		if !v.Volatile() {
			t.Fatal("RenderVideo must declare Volatile")
		}
		rpAssertRefused(t, "Video", rpScene(v))
	})

	t.Run("ViewportStructural", func(t *testing.T) {
		mk := func() *rendering.RenderViewport {
			content := rendering.NewAbsoluteBox(80, 200)
			content.Place(rendering.NewRenderColorBox(80, 60, 0.2, 0.6, 0.9, 1), 0, 0)
			content.Place(rendering.NewRenderColorBox(80, 60, 0.9, 0.5, 0.2, 1), 0, 100)
			vp := rendering.NewRenderViewport(content)
			vp.FixedWidth, vp.FixedHeight = 60, 40
			return vp
		}
		// Default: the viewport is its own paint boundary but never a cache
		// unit; the scene stays pixel-correct through the cached paint walk.
		vp := mk()
		root := rpScene(vp)
		cache := rendering.NewBoundaryCache()
		live := rpPaint(t, root, nil)
		first := rpPaint(t, root, cache)
		second := rpPaint(t, root, cache)
		if cache.HasValid(vp) {
			t.Fatal("structural viewport must never hold a recording")
		}
		rpDiff(t, "Viewport first-live", live, first)
		rpDiff(t, "Viewport second-live", live, second)

		// Even when forced to be a plain (non-boundary) child, the viewport
		// cannot be baked: the parent boundary must refuse caching.
		vp2 := mk()
		vp2.SetRepaintBoundary(false)
		root2 := rpScene(vp2)
		rpAssertRefused(t, "ViewportNonBoundary", root2)
	})
}
