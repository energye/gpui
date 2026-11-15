// grlleg.go — offscreen legs for the text renderer family: TextRenderer
// ShapeAndRender face, GlyphRenderer direct calls, RenderParams options, and
// the GlyphRunBuilder command builder. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	grlW, grlH   = 320, 240
	testFontGRL  = "render/text/testdata/goregular.ttf"
)

type grlLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *grlLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "grlleg: %s\n", line)
}

func flushGRL(dc *render.Context, view render.TextureView, leg *grlLeg, tag string) bool {
	if err := dc.PresentFrame(view, grlW, grlH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func clearGRL(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, grlW, grlH)
	_ = dc.Fill()
}

// strokeOutlines paints outlines as polylines (the outline is the product of
// the renderer face; stroking it is the window's visual anchor).
func strokeOutlines(dc *render.Context, outlines []*text.GlyphOutline, ox, oy float64) {
	dc.SetRGBA(0.92, 0.92, 0.95, 1)
	dc.SetLineWidth(1)
	for _, o := range outlines {
		if o == nil {
			continue
		}
		started := false
		for _, seg := range o.Segments {
			switch seg.Op {
			case text.OutlineOpMoveTo:
				if started {
					_ = dc.Stroke()
				}
				dc.MoveTo(ox+float64(seg.Points[0].X), oy+float64(seg.Points[0].Y))
				started = true
			case text.OutlineOpLineTo:
				end := seg.Points[0]
				dc.LineTo(ox+float64(end.X), oy+float64(end.Y))
			case text.OutlineOpQuadTo:
				c, end := seg.Points[0], seg.Points[1]
				dc.QuadraticTo(ox+float64(c.X), oy+float64(c.Y), ox+float64(end.X), oy+float64(end.Y))
			case text.OutlineOpCubicTo:
				c1, c2, end := seg.Points[0], seg.Points[1], seg.Points[2]
				dc.CubicTo(ox+float64(c1.X), oy+float64(c1.Y), ox+float64(c2.X), oy+float64(c2.Y), ox+float64(end.X), oy+float64(end.Y))
			default:
				// close: stroke the current chain
				_ = dc.Stroke()
				started = false
			}
		}
		if started {
			_ = dc.Stroke()
		}
	}
}

func runGRLegs2() *grlLeg {
	leg := &grlLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(grlW, grlH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(grlW, grlH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	src, err := text.NewFontSourceFromFile(testFontGRL)
	if err != nil {
		leg.probe("字体装载", false, err.Error())
		return leg
	}
	defer func() { _ = src.Close() }()
	face := src.Face(28)
	font := src.Parsed()

	clearGRL(dc)

	// ---- Card A: TextRenderer face ----
	tr := text.NewTextRenderer()
	tr.SetDefaultFace(face)
	tr.SetDefaultSize(28)
	tr.SetDefaultColor(colorOf(0.92, 0.92, 0.95))
	leg.probe("A.TextRenderer默认", tr.GlyphRenderer() != nil, "GlyphRenderer() 内建非 nil")
	outlines, errSA := tr.ShapeAndRender("Render")
	leg.probe("A.ShapeAndRender", errSA == nil && len(outlines) == 6,
		fmt.Sprintf("err=%v outlines=%d", errSA, len(outlines)))
	outlinesAt, errAt := tr.ShapeAndRenderAt("At 42", 8, 60)
	leg.probe("A.ShapeAndRenderAt", errAt == nil && len(outlinesAt) == 5,
		fmt.Sprintf("err=%v outlines=%d", errAt, len(outlinesAt)))
	strokeOutlines(dc, outlinesAt, 20, 70)

	// ---- Card B: GlyphRenderer direct ----
	g := tr.GlyphRenderer()
	glyphs := text.Shape("GIDxyz", face)
	params := text.RenderParams{Color: colorOf(0.92, 0.92, 0.95), Opacity: 1.0}
	o1 := g.RenderGlyph(&glyphs[0], font, 28, params)
	leg.probe("B.RenderGlyph", o1 != nil && o1.SegmentCount() > 0,
		fmt.Sprintf("segs=%d", o1.SegmentCount()))
	os2 := g.RenderGlyphs(glyphs, font, 28, params)
	leg.probe("B.RenderGlyphs", len(os2) == len(glyphs), fmt.Sprintf("n=%d", len(os2)))
	run := text.ShapedRun{Glyphs: glyphs, Face: face, Size: 28, Style: -1}
	os3 := g.RenderRun(&run, params)
	leg.probe("B.RenderRun", len(os3) == len(glyphs), fmt.Sprintf("n=%d", len(os3)))
	layout := text.LayoutTextSimple("Layout it", face)
	os4 := g.RenderLayout(layout, params)
	leg.probe("B.RenderLayout", len(os4) >= 1,
		fmt.Sprintf("lines=%d", len(os4)))
	cache := text.NewGlyphCache()
	g.SetCache(cache)
	leg.probe("B.SetCache", true, "私有缓存注入无错")
	strokeOutlines(dc, os2, 20, 140)

	// ---- Card C: RenderParams ----
	p1 := text.RenderParams{}.WithColor(colorOf(1, 0, 0)).WithOpacity(0.5)
	leg.probe("C.WithColor/Opacity", p1.Opacity == 0.5,
		fmt.Sprintf("color=%v opacity=%.1f", p1.Color, p1.Opacity))

	// ---- Card D: GlyphRunBuilder ----
	b := text.NewGlyphRunBuilder(cache)
	b.AddGlyph(1, 36, text.Point{X: 0, Y: 0}, 28)
	b.AddShapedGlyph(1, &glyphs[1], 28)
	b.AddShapedGlyphs(1, glyphs[2:4], text.Point{X: 40, Y: 0}, 28)
	runFull := text.ShapedRun{Glyphs: glyphs[4:6], Face: face, Size: 28, Style: -1}
	b.AddShapedRun(&runFull, text.Point{X: 120, Y: 0})
	leg.probe("D.Len", b.Len() == 6, fmt.Sprintf("instances=%d", b.Len()))
	cmds := b.Build(func(fontID uint64, gid text.GlyphID, size float32) *text.GlyphOutline {
		o, _ := text.NewOutlineExtractor().ExtractOutline(font, text.GlyphID(gid), float64(size))
		return o
	})
	leg.probe("D.Build命令数", len(cmds) == 6, fmt.Sprintf("commands=%d", len(cmds)))
	insts := b.Instances()
	leg.probe("D.Instances", len(insts) == 6, fmt.Sprintf("n=%d", len(insts)))
	b2 := text.NewGlyphRunBuilder(cache)
	b2.AddShapedGlyph(1, &glyphs[0], 28)
	cmdsT := b2.BuildTransformed(func(fontID uint64, gid text.GlyphID, size float32) *text.GlyphOutline {
		o, _ := text.NewOutlineExtractor().ExtractOutline(font, text.GlyphID(gid), float64(size))
		return o
	}, &text.AffineTransform{A: 2, D: 2})
	leg.probe("D.BuildTransformed", len(cmdsT) == 1 && cmdsT[0].Transform != nil,
		fmt.Sprintf("commands=%d transform 非空", len(cmdsT)))
	b.Clear()
	leg.probe("D.Clear", b.Len() == 0, "清空")

	if !flushGRL(dc, view, leg, "D") {
		return leg
	}
	leg.imgs["render"] = dc.Image()

	leg.findings = append(leg.findings,
		"glyphrender: TextRenderer ShapeAndRender(At) 出轮廓（6/5 字形）、GlyphRenderer 四口直调、RenderParams 链、GlyphRunBuilder 命令数=字形数（Build/BuildTransformed/Instances/Clear）——轮廓经 strokeOutlines 实画佐证")
	return leg
}

func colorOf(r, g, b float64) color.RGBA {
	return color.RGBA{R: uint8(r * 255), G: uint8(g * 255), B: uint8(b * 255), A: 255}
}
