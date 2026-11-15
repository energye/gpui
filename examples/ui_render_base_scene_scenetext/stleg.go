// stleg.go — offscreen legs for the scene text family: scene TextRenderer
// (config / RenderGlyph / RenderGlyphs / RenderText / ToCompositePath),
// TextShape, TextRendererPool, and the encoding text value types. One GL-hub
// job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
	"github.com/energye/gpui/render/text"
)

const (
	stW, stH   = 320, 240
	testFontST = "render/text/testdata/goregular.ttf"
)

type stLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *stLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "stleg: %s\n", line)
}

func flushST(dc *render.Context, view render.TextureView, leg *stLeg, tag string) bool {
	if err := dc.PresentFrame(view, stW, stH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func runSTLegs() *stLeg {
	leg := &stLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(stW, stH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(stW, stH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	src, err := text.NewFontSourceFromFile(testFontST)
	if err != nil {
		leg.probe("字体装载", false, err.Error())
		return leg
	}
	defer func() { _ = src.Close() }()
	face := src.Face(28)

	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, stW, stH)
	_ = dc.Fill()

	// ---- Card A: scene TextRenderer ----
	cfg := scene.DefaultTextRendererConfig()
	leg.probe("A.DefaultConfig", !cfg.FlipY && cfg.SubpixelPositioning && !cfg.HintingEnabled,
		fmt.Sprintf("FlipY=%v subpixel=%v hinting=%v", cfg.FlipY, cfg.SubpixelPositioning, cfg.HintingEnabled))
	tr := scene.NewTextRendererWithConfig(cfg)
	leg.probe("A.Config读回", tr.Config() == cfg, "Config()==设置值")
	glyphs := text.Shape("Scene", face)
	rgs, errRG := tr.RenderGlyphs(glyphs, face)
	leg.probe("A.RenderGlyphs", errRG == nil && len(rgs) == len(glyphs),
		fmt.Sprintf("err=%v rendered=%d", errRG, len(rgs)))
	rg1, errR1 := tr.RenderGlyph(glyphs[0], face)
	leg.probe("A.RenderGlyph", errR1 == nil && rg1 != nil && rg1.Path != nil && !rg1.Path.IsEmpty(),
		fmt.Sprintf("err=%v path verbs=%d", errR1, func() int {
			if rg1 != nil && rg1.Path != nil {
				return rg1.Path.VerbCount()
			}
			return 0
		}()))
	composite := tr.ToCompositePath(rgs)
	leg.probe("A.ToCompositePath", composite != nil && composite.VerbCount() > 0,
		fmt.Sprintf("composite verbs=%d", composite.VerbCount()))
	rt, errRT := tr.RenderText("Rendered!", face)
	leg.probe("A.RenderText", errRT == nil && len(rt) == 9,
		fmt.Sprintf("err=%v glyphs=%d", errRT, len(rt)))

	// ---- Card B: TextShape ----
	ts, errTS := scene.NewTextShape("TextShape 12", face, 30, 120)
	leg.probe("B.NewTextShape", errTS == nil && ts != nil && ts.ToPath() != nil,
		fmt.Sprintf("err=%v verbs=%d", errTS, func() int {
			if ts != nil && ts.ToPath() != nil {
				return ts.ToPath().VerbCount()
			}
			return 0
		}()))
	tb := ts.Bounds()
	leg.probe("B.TextShapeBounds", tb.MaxX > tb.MinX,
		fmt.Sprintf("bounds=[%.0f,%.0f %.0f,%.0f]", tb.MinX, tb.MinY, tb.MaxX, tb.MaxY))

	// ---- Card C: TextRendererPool ----
	pool := scene.NewTextRendererPool()
	tr2 := pool.Get()
	pool.Put(tr2)
	leg.probe("C.TextRendererPool", tr2 != nil, "Get/Put")

	// ---- Card D: encoding text value types + scene paint ----
	leg.probe("D.TextFlags", scene.TextFlagHinting == 1 && scene.TextFlagCJK == 2,
		"Hinting=1<<0 CJK=1<<1")
	runData := scene.GlyphRunData{FontSourceID: 7, FontSize: 28, GlyphCount: 3, OriginX: 10, OriginY: 20}
	leg.probe("D.GlyphRunData", runData.FontSize == 28 && runData.GlyphCount == 3,
		fmt.Sprintf("size=%.0f count=%d", runData.FontSize, runData.GlyphCount))
	entry := scene.GlyphEntry{GlyphID: 36, X: 1, Y: 2, Cluster: 3}
	leg.probe("D.GlyphEntry", entry.GlyphID == 36 && entry.Cluster == 3, "gid/cluster 读回")

	// scene paint: TextShape filled via Scene
	sc := scene.NewScene()
	sc.Fill(scene.FillNonZero, scene.IdentityAffine(),
		scene.SolidBrush(render.RGBA2(0.92, 0.92, 0.95, 1)), ts)
	pm := render.NewPixmap(stW, stH)
	rend := scene.NewRenderer(stW, stH, scene.WithWorkers(1))
	defer rend.Close()
	if err := rend.Render(pm, sc); err == nil {
		ink := 0
		for y := 80; y < 140; y++ {
			for x := 20; x < 300; x++ {
				r, _, _, _ := pm.At(x, y).RGBA()
				if r > 120 {
					ink++
				}
			}
		}
		leg.probe("D.SceneFill(TextShape)实画", ink > 50,
			fmt.Sprintf("带内墨迹=%d px", ink))
	}
	dc.DrawImageEx(render.ImageBufFromImage(pm), render.DrawImageOptions{X: 0, Y: 0, DstWidth: stW, DstHeight: stH})
	dc.SetRGBA(1, 1, 1, 1)
	if !flushST(dc, view, leg, "D") {
		return leg
	}
	leg.imgs["text"] = dc.Image()

	leg.findings = append(leg.findings,
		"scenetext: scene TextRenderer（Config 读回/RenderGlyph/Glyphs/Text/ToCompositePath）、TextShape（轮廓合成+Bounds）、TextRendererPool、TextFlags/GlyphRunData/GlyphEntry 编码值类型、Scene.Fill(TextShape) CPU 实画墨迹")
	return leg
}
