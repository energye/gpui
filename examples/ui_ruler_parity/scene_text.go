package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// buildTextScene mirrors tools/ruler_flutter text: 48 header, 12px body
// padding, 560px text column, M5 corpus (multiscript/emoji/richtext/DPR),
// 72 HUD, static after the first frame. Flutter's text engine and gpui's
// shaper/rasterizer are different, so glyph outlines are explicitly out of
// scope; the layout/flow, corpus, colors and sizes are aligned.
func buildTextScene() (*sceneSpec, error) {
	root := newRoot(winListW, winListH, colBg)
	addHeader(root, "gpui parity — dense text (M5 semantic)",
		"P3 parity ruler · RULER_SCENE=text")

	content := rendering.NewAbsoluteBox(1176, 1000)
	vp := rendering.NewRenderViewport(content)
	vp.FixedWidth = 1176
	vp.FixedHeight = 656
	root.Place(vp, 12, 60)

	multi := loadText("multiscript.txt")
	emoji := loadText("emoji.txt")
	richLines := strings.Split(strings.TrimSpace(loadText("richtext.txt")), "\n")

	y := 0.0
	addLabel := func(s string) {
		l := labelAt(content, s, 11, 0, y, colLabel)
		y += textHeight(l) + 4
	}
	addBody := func(s string, maxW float64) *rendering.RenderText {
		t := labelWrapped(content, s, 14, maxW, 0, y, colInk)
		t.LineSpacing = 1.2
		y += textHeight(t) + 6
		return t
	}

	addLabel("A 多脚本 (multiscript.txt)")
	addBody(multi, 560)

	addLabel("B emoji + 组合字符 (emoji.txt)")
	// Build the emoji MultiFace *before* measuring: the color-emoji face has
	// different advances/line metrics, and a post-measure face swap would
	// leave the following zone overlapping this wrapped block.
	tb := labelWrapped(content, emoji, 14, 560, 0, y, colInk)
	tb.LineSpacing = 1.2
	if ef := emojiFace(14); ef != nil {
		if mf, err := text.NewMultiFace(ef, wrkit.FaceAt(14)); err == nil {
			tb.SetFace(mf)
		}
	}
	y += textHeight(tb) + 6

	addLabel("C 富文本 run (20/14/8/12pt)")
	// The Flutter side puts each rich line in its own TextSpan with a trailing
	// newline. Keep the same per-line runs as explicit line nodes here: this
	// avoids a mixed-size paragraph height that changes when the color-emoji
	// face is installed, while preserving the exact visible lines/colors.
	sizes := []float64{20, 14, 8, 12}
	colors := [][3]float64{
		{0xF2 / 255.0, 0xE6 / 255.0, 0x80 / 255.0},
		{0xD9 / 255.0, 0xE0 / 255.0, 0xEB / 255.0},
		{0x99 / 255.0, 0xCC / 255.0, 0xF2 / 255.0},
		{0xBF / 255.0, 0xF2 / 255.0, 0xBF / 255.0},
	}
	for i, ln := range richLines {
		sz := sizes[i%len(sizes)]
		c := colors[i%len(colors)]
		td := labelAt(content, ln, sz, 0, y, rgbColor{R: c[0], G: c[1], B: c[2]})
		td.LineSpacing = 1.2
		y += sz * 1.2
	}
	y += 6

	addLabel("D 多DPR同串 (12/15/24/36pt)")
	for i, sz := range []float64{12, 15, 24, 36} {
		td := wrkit.Label(fmt.Sprintf("%gx: Hello 世界 مرحبا กข", []float64{1, 1.25, 2, 3}[i]), sz, colInk.R, colInk.G, colInk.B)
		td.LineSpacing = 1.1
		content.Place(td, 0, y)
		y += textHeight(td)
	}

	addHUD(root, "M5 / Flutter",
		"text: static after first frame — no ticker", "")
	if y > 656 {
		fmt.Fprintf(os.Stderr, "ui_ruler_parity: text content %.1f > 656 (first screen clipped)\n", y)
	}

	return &sceneSpec{
		name:   sceneText,
		ref:    "tools/ruler_flutter RULER_SCENE=text",
		winW:   winListW,
		winH:   winListH,
		title:  "gpui ruler parity — Flutter text (1200x800)",
		root:   root,
		clearR: colBg.R, clearG: colBg.G, clearB: colBg.B,
		// nil tick: the Flutter text arm has no ticker after first frame.
		goldenRects: []wrkit.GoldenRect{
			{X: 0, Y: 0, W: winListW, H: 48},
			{X: 0, Y: 48, W: winListW, H: 680},
		},
		probes: []wrkit.PixelProbe{
			{Name: "text_header", X: 600, Y: 20, R: colHeader.R, G: colHeader.G, B: colHeader.B, Tol: 4},
			{Name: "text_body_bg", X: 1100, Y: 700, R: colBg.R, G: colBg.G, B: colBg.B, Tol: 4},
			{Name: "text_hud", X: 600, Y: 760, R: colHud.R, G: colHud.G, B: colHud.B, Tol: 4},
		},
		extraCheck: func(snapshot string) error {
			img, err := loadSnapshot(snapshot)
			if err != nil {
				return fmt.Errorf("decode text snapshot: %w", err)
			}
			n := inkCount(img, 12, 60, 560, 620, colBg, 6)
			if n < 500 {
				return fmt.Errorf("text body ink too small: %d px (want >=500)", n)
			}
			return nil
		},
	}, nil
}
