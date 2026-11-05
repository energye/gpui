//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import (
	"os"
	"testing"

	"github.com/energye/gpui/render/text"
)

// P3.3 capability pins for rich text: builder span semantics, per-run
// rendering with paint restore, Justify space distribution (double-write),
// and MaxLines/ellipsis truncation.

func paragraphTestFace(t *testing.T) text.Face {
	t.Helper()
	data, err := os.ReadFile("text/testdata/goregular.ttf")
	if err != nil {
		t.Skipf("test font missing: %v", err)
	}
	source, err := text.NewFontSource(data)
	if err != nil {
		t.Skipf("test font missing: %v", err)
	}
	t.Cleanup(func() { _ = source.Close() })
	return source.Face(16.0)
}

// TestParagraphBuilderSpanSemantics pins Push/Pop inheritance and the run→
// span annotation.
func TestParagraphBuilderSpanSemantics(t *testing.T) {
	face := paragraphTestFace(t)
	b := NewParagraphBuilder(face, TextStyle{})
	b.AddText("red ")
	b.PushStyle(TextStyle{Color: RGBA{R: 1, A: 1}})
	b.AddText("green ")
	b.PushStyle(TextStyle{Color: RGBA{G: 1, A: 1}})
	b.AddText("blue ")
	b.PopStyle()
	b.AddText("green again")
	p := b.Build(text.LayoutOptions{MaxWidth: 400})
	if p.StyleCount() != 4 {
		t.Fatalf("StyleCount=%d, want 4 spans", p.StyleCount())
	}
	// Span 0 uses the base style: colorless (inherits the ambient paint).
	if got, _ := p.StyleForRun(0); got.Color.A != 0 {
		t.Fatalf("span 0 color = %+v, want unspecified (base)", got.Color)
	}
	// 收紧断言（审查 P2-14：旧状 `G!=1 && R!=1` 逻辑与、且 c=14 落 span2，
	// PopStyle 恢复路径零断言）。本用例文本命名与颜色故意交叉，按颜色真值断：
	// span0 base 无色；span1 红域 R=1；span2 绿域 G=1；span3 PopStyle 出绿域
	// 回到外层红域。
	if got, _ := p.StyleForRun(5); got.Color.R != 1 || got.Color.G != 0 || got.Color.B != 0 {
		t.Fatalf("span 1 color = %+v, want pure red (R=1, G=B=0)", got.Color)
	}
	if got, _ := p.StyleForRun(11); got.Color.G != 1 || got.Color.R != 0 || got.Color.B != 0 {
		t.Fatalf("span 2 color = %+v, want pure green (G=1, R=B=0)", got.Color)
	}
	// PopStyle restored the enclosing RED scope for the 4th span (c=17 ∈ [15,26)).
	if got, _ := p.StyleForRun(17); got.Color.R != 1 || got.Color.G != 0 || got.Color.B != 0 {
		t.Fatalf("span 3 color = %+v, want restored red (R=1, G=B=0)", got.Color)
	}
	// Runs carry the span index.
	runs := 0
	for _, line := range p.Layout.Lines {
		for _, r := range line.Runs {
			if r.Style < 0 || r.Style >= 4 {
				t.Fatalf("run.Style=%d out of span range", r.Style)
			}
			runs++
		}
	}
	if runs == 0 {
		t.Fatal("no runs laid out")
	}
}

// TestJustifyDistributesSpaces pins the space distribution: interior lines
// stretch to MaxWidth, the last line does not, and both glyph copies stay in
// lockstep.
func TestJustifyDistributesSpaces(t *testing.T) {
	face := paragraphTestFace(t)
	// A line of several short words that cannot fill a wide container.
	// Justify needs a wrapped multi-line paragraph: a single-line paragraph
	// is always the last line and must not stretch.
	lay := text.LayoutText("aa bb cc dd ee ff gg hh ii jj kk ll mm nn oo", face, text.LayoutOptions{
		MaxWidth: 160, Alignment: text.AlignJustify,
	})
	if len(lay.Lines) < 2 {
		t.Fatalf("lines=%d, want >=2 (wrapped)", len(lay.Lines))
	}
	for i := 0; i < len(lay.Lines)-1; i++ {
		line := &lay.Lines[i]
		if fdiff(line.Width, 160) > 1 {
			t.Fatalf("interior line %d width=%.2f, want 160±1", i, line.Width)
		}
		// Double-write lockstep: run glyphs must match line glyphs exactly.
		for ri := range line.Runs {
			if len(line.Runs[ri].Glyphs) != len(line.Glyphs) {
				continue
			}
			for gi := range line.Glyphs {
				if fdiff(line.Runs[ri].Glyphs[gi].X, line.Glyphs[gi].X) > 0.01 ||
					fdiff(line.Runs[ri].Glyphs[gi].XAdvance, line.Glyphs[gi].XAdvance) > 0.01 {
					t.Fatalf("line %d glyph %d diverges: line=(%.2f,%.2f) run=(%.2f,%.2f)",
						i, gi, line.Glyphs[gi].X, line.Glyphs[gi].XAdvance,
						line.Runs[ri].Glyphs[gi].X, line.Runs[ri].Glyphs[gi].XAdvance)
				}
			}
		}
	}
	last := lay.Lines[len(lay.Lines)-1]
	if fdiff(last.Width, 160) < 1 {
		t.Fatal("last line must not be stretched to MaxWidth")
	}
}

func fdiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestMaxLinesEllipsis pins truncation: line budget, tail width ≤ MaxWidth,
// and the last glyph being the ellipsis marker.
func TestMaxLinesEllipsis(t *testing.T) {
	face := paragraphTestFace(t)
	lay := text.LayoutText("word one two three four five six seven eight nine ten", face, text.LayoutOptions{
		MaxWidth: 180, MaxLines: 2, Overflow: text.OverflowEllipsis,
	})
	if len(lay.Lines) != 2 {
		t.Fatalf("lines=%d, want 2", len(lay.Lines))
	}
	last := &lay.Lines[1]
	if last.Width > 180+0.01 {
		t.Fatalf("tail width %.2f exceeds MaxWidth", last.Width)
	}
	// 实断言（交付后审查 P1-5/14：旧状这里是 `_ = lastGlyph` 空转）：
	// 末位 glyph 必须是省略号标记，GID 对齐直测基准，cluster 单调且在段内。
	ell := text.ShapeUncached("…", face)
	if len(ell) == 0 {
		t.Skip("base face has no U+2026")
	}
	lastGlyph := last.Glyphs[len(last.Glyphs)-1]
	if lastGlyph.GID != ell[0].GID {
		t.Fatalf("tail glyph GID=%d, want ellipsis GID=%d（省略号缺席或错位）", lastGlyph.GID, ell[0].GID)
	}
	if prev := last.Glyphs[len(last.Glyphs)-2]; lastGlyph.Cluster <= prev.Cluster {
		t.Fatalf("ellipsis cluster=%d must follow the kept prefix cluster=%d", lastGlyph.Cluster, prev.Cluster)
	}
	if rc := len([]rune("word one two three four five six seven eight nine ten")); lastGlyph.Cluster >= rc {
		t.Fatalf("ellipsis cluster=%d out of paragraph rune range %d", lastGlyph.Cluster, rc)
	}
	if lay.Height <= 0 {
		t.Fatal("layout height must stay positive after truncation")
	}
}

// TestDrawParagraphStyledRestoresPaint pins that the styled draw never leaks
// run colors into the ambient paint.
func TestDrawParagraphStyledRestoresPaint(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	face := paragraphTestFace(t)
	b := NewParagraphBuilder(face, TextStyle{Color: RGBA{R: 1, A: 1}})
	b.PushStyle(TextStyle{Color: RGBA{G: 1, B: 1, A: 1}})
	b.AddText("cyan")
	p := b.Build(text.LayoutOptions{MaxWidth: 200})

	dc := NewContext(64, 64)
	t.Cleanup(func() { _ = dc.Close() })
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawParagraphStyled(p, 4, 40)
	if dc.paint.solidColor.R != 1 || dc.paint.solidColor.G != 0 {
		t.Fatalf("ambient paint leaked: %+v, want red preserved", dc.paint.solidColor)
	}
	// Cyan run actually landed.
	pm := dc.PixmapForTest()
	found := false
	for y := 0; y < 64 && !found; y++ {
		for x := 0; x < 64; x++ {
			r, g, bl, _ := pixelAt(pm, x, y)
			if g > 150 && bl > 150 && r < 100 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("styled cyan run did not render any pixels")
	}
}

func pixelAt(pm *Pixmap, x, y int) (uint8, uint8, uint8, uint8) {
	data := pm.Data()
	i := (y*pm.Width() + x) * 4
	return data[i], data[i+1], data[i+2], data[i+3]
}

// TestPaintMaskFilterSoftensEdges pins the P3.4 paint-level filter path on
// the CPU tier: a sharp rectangle with MaskFilterRadius gets a feathered
// edge. The background is TRANSPARENT so the alpha channel encodes coverage
// — an opaque background would make the "edge is feathered" assertion
// vacuous (alpha is 255 everywhere regardless of the blur).
func TestPaintMaskFilterSoftensEdges(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := NewContext(64, 64)
	t.Cleanup(func() { _ = dc.Close() })
	dc.ClearWithColor(Transparent)
	dc.SetRGB(1, 1, 1)
	dc.paint.MaskFilterRadius = 3
	dc.DrawRectangle(16, 16, 32, 32)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	pm := dc.PixmapForTest()
	alphaAt2 := func(x, y int) uint8 { return pm.Data()[(y*pm.Width()+x)*4+3] }
	// Inside the shape: still opaque.
	if a := alphaAt2(32, 32); a < 200 {
		t.Fatalf("interior alpha=%d, want opaque", a)
	}
	// At the old boundary: feathered — neither fully covered nor empty (a
	// sharp rectangle would be 255 here, an unfiltered draw has no ramp).
	if a := alphaAt2(16, 32); a == 0 || a > 245 {
		t.Fatalf("boundary alpha=%d, want a feathered intermediate value", a)
	}
	// Just outside the old boundary: the blur halo must extend outward.
	if a := alphaAt2(14, 32); a == 0 {
		t.Fatal("no outward halo at x=14 — MaskFilter blur did not apply")
	}
	// Far outside the blur reach: fully transparent (nothing drawn).
	if a := alphaAt2(4, 32); a != 0 {
		t.Fatalf("far-field alpha=%d, want untouched transparent background", a)
	}
}

// TestApplyMergeAndDisplacement pins the P3.4 composition entry points at the
// Context level (CPU mode).
func TestApplyMergeAndDisplacement(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	// Register a minimal offset closure in-process (render/filters would
	// create an import cycle from this internal test). The registration is
	// global — save the current ops and restore them so later tests (and
	// -count>=2 reruns of this package) keep the real registry.
	saved := snapshotFilterOps()
	t.Cleanup(func() { RegisterFilterOps(saved) })
	RegisterFilterOps(FilterOpImpls{
		Invert: func(src, dst *Pixmap) {
			d := dst.Data()
			for i2 := 0; i2+3 < len(d); i2 += 4 {
				a := d[i2+3]
				d[i2], d[i2+1], d[i2+2] = a-d[i2], a-d[i2+1], a-d[i2+2]
			}
		},
		Erode:          func(src, dst *Pixmap, radius int) {},
		Dilate:         func(src, dst *Pixmap, radius int) {},
		Offset:         func(src, dst *Pixmap, dx, dy int) {},
		AlphaThreshold: func(src, dst *Pixmap, threshold uint8) {},
		Tile:           func(src, dst *Pixmap, srcRect [4]float32) {},
	})
	dc := NewContext(32, 32)
	t.Cleanup(func() { _ = dc.Close() })
	dc.ClearWithColor(Black)
	dc.SetRGB(0, 1, 0)
	dc.DrawRectangle(4, 4, 8, 8)
	_ = dc.Fill()
	// Merge: layer 1 shifts a copy of the content right by 12; layer 2
	// inverts the base (green → magenta). Both land over the base.
	dc.ApplyMerge(
		[]ImageFilterNode{{Kind: ImageFilterOffset, IntOffsetX: 12}},
		[]ImageFilterNode{{Kind: ImageFilterInvert}},
	)
	pm := dc.PixmapForTest()
	_, sgG, _, _ := pixelAt(pm, 20, 8)    // green block moved to x=16..24
	invR, _, invB, _ := pixelAt(pm, 4, 4) // inverted base: magenta
	if sgG <= 150 {
		t.Fatalf("shifted layer at (20,8) g=%d, want green present", sgG)
	}
	if invR <= 150 || invB <= 150 {
		t.Fatalf("inverted layer at (4,4)=(%d,%d), want magenta present", invR, invB)
	}
}
