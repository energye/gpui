// Package main — Text axis multi-script smoke (ENGINE_UI_RENDER_BASE §8 / §24).
package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

type textScene struct {
	Root *rendering.AbsoluteBox

	ColorPulse *rendering.RenderText
	DCPanel    *rendering.RenderBox

	face  text.Face
	phase float64
}

type langSample struct {
	tag, sample string
	r, g, b     float64
}

func label(s string, size float64, r, g, b float64, face text.Face) *rendering.RenderText {
	t := rendering.NewRenderText(s)
	t.FontSize = size
	t.R, t.G, t.B, t.A = r, g, b, 1
	if face != nil {
		t.SetFace(face)
	}
	return t
}

func panelBox(w, h float64, paint func(pc *rendering.PaintContext, size rendering.Size)) *rendering.RenderBox {
	b := rendering.NewRenderBox()
	b.FixedWidth, b.FixedHeight = w, h
	b.SetRepaintBoundary(true)
	b.OnPaint = paint
	return b
}

func fillRect(pc *rendering.PaintContext, x, y, w, h, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawRectangle(pc.OriginX+x, pc.OriginY+y, w, h)
	_ = pc.DC.Fill()
}

func strokeRect(pc *rendering.PaintContext, x, y, w, h, lw, r, g, b, a float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	if lw <= 0 {
		lw = 1
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lw)
	pc.DC.DrawRectangle(pc.OriginX+x, pc.OriginY+y, w, h)
	_ = pc.DC.Stroke()
}

func placeLangCard(root *rendering.AbsoluteBox, face text.Face, x, y, w, h float64, tag, sample string, cr, cg, cb float64) *rendering.RenderText {
	root.Place(label(tag, 11, 0.6, 0.85, 1, face), x, y)
	card := panelBox(w, h, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.16, 0.20, 1)
		strokeRect(pc, 1, 1, sz.Width-2, sz.Height-2, 1, 0.3, 0.35, 0.4, 1)
	})
	root.Place(card, x, y+16)
	rt := rendering.NewRenderText(sample)
	rt.FontSize = 16
	rt.ApproxCharW = 0.75
	rt.R, rt.G, rt.B, rt.A = cr, cg, cb, 1
	rt.SetRepaintBoundary(true)
	if face != nil {
		rt.SetFace(face)
	}
	root.Place(rt, x+12, y+40)
	return rt
}

func buildTextScene(winW, winH float64, face text.Face, fontPath string) *textScene {
	s := &textScene{face: face}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.09, G: 0.10, B: 0.12, A: 1}
	s.Root = root

	root.Place(label("Text axis — multi-language sample board", 13, 0.75, 0.78, 0.85, face), 12, 6)
	sub := "MultiFace chain · layout≈1 on color pulse"
	if fontPath != "" {
		sub += " | " + fontPath
	}
	root.Place(label(sub, 10, 0.5, 0.55, 0.6, face), 12, 26)

	// Compact language grid (3 columns).
	samples := []langSample{
		{"EN English", "The quick brown fox — 123", 0.95, 0.95, 0.98},
		{"ZH 简体中文", "你好世界 · 渲染矩阵", 0.95, 0.85, 0.45},
		{"ZH-TW 繁體", "繁體中文測試 · 字型回退", 0.9, 0.8, 0.55},
		{"JA 日本語", "こんにちは · 日本語テキスト", 0.55, 0.9, 0.75},
		{"KO 한국어", "안녕하세요 · 한글 텍스트", 0.7, 0.75, 1.0},
		{"RU Русский", "Привет мир · кириллица", 0.85, 0.7, 0.95},
		{"EL Ελληνικά", "Γειά σου κόσμε · ελληνικά", 0.7, 0.9, 0.95},
		{"AR العربية", "مرحبا بالعالم · نص عربي", 0.95, 0.75, 0.55},
		{"HE עברית", "שלום עולם · עברית", 0.8, 0.85, 0.6},
		{"TH ไทย", "สวัสดีชาวโลก · ภาษาไทย", 0.6, 0.95, 0.7},
		{"HI हिन्दी", "नमस्ते दुनिया · देवनागरी", 0.95, 0.7, 0.8},
		{"MIX mixed", "Hi 你好 こんにちは 안녕 Привет ก", 0.9, 0.9, 0.85},
	}

	const (
		cols = 3
		cw   = 250.0
		ch   = 72.0
		gx   = 12.0
		gy   = 10.0
		left = 12.0
		top0 = 48.0
	)
	for i, samp := range samples {
		col := i % cols
		row := i / cols
		x := left + float64(col)*(cw+gx)
		y := top0 + float64(row)*(ch+gy+14)
		placeLangCard(root, face, x, y, cw, ch, samp.tag, samp.sample, samp.r, samp.g, samp.b)
	}

	// Bottom band: ellipsis demos + color pulse + DC multi-script
	nRows := (len(samples) + cols - 1) / cols
	bandY := top0 + float64(nRows)*(ch+gy+14) + 8
	root.Place(label("FT-MAXLINES / FT-OVERFLOW ellipsis · FT-PARAGRAPH multi-run", 11, 0.95, 0.75, 0.4, face), 12, bandY)

	// Minimal ParagraphBuilder: two colored runs (not single-style string).
	para := rendering.NewParagraphBuilder()
	para.SetDefaultStyle(face, 13, 0.95, 0.55, 0.35, 1, 0.55)
	para.AddText("Paragraph ")
	para.SetDefaultStyle(face, 13, 0.45, 0.85, 1.0, 1, 0.55)
	para.AddText("multi-run")
	para.SetDefaultStyle(face, 13, 0.85, 0.9, 0.75, 1, 0.55)
	para.AddText(" · 色/字号分 span")
	paraRT := para.Build()
	paraRT.SetRepaintBoundary(true)
	root.Place(paraRT, 400, bandY+18)

	ellip1 := rendering.NewRenderText("Single-line ellipsis: The quick brown fox jumps over the lazy dog — 超长单行省略号演示文本")
	ellip1.FontSize = 13
	ellip1.ApproxCharW = 0.55
	ellip1.SetMaxWidth(360)
	ellip1.SetMaxLines(1)
	ellip1.SetOverflow(rendering.TextOverflowEllipsis)
	ellip1.SetRepaintBoundary(true)
	if face != nil {
		ellip1.SetFace(face)
	}
	root.Place(ellip1, 12, bandY+18)

	ellip2 := rendering.NewRenderText("Two-line maxLines=2 ellipsis wraps then cuts: English 中文 日本語 mixed script sample for list subtitles and table cells that must not grow unbounded in height when content is long.")
	ellip2.FontSize = 12
	ellip2.ApproxCharW = 0.55
	ellip2.LineSpacing = 1.25
	ellip2.SetMaxWidth(520)
	ellip2.SetMaxLines(2)
	ellip2.SetOverflow(rendering.TextOverflowEllipsis)
	ellip2.SetRepaintBoundary(true)
	if face != nil {
		ellip2.SetFace(face)
	}
	root.Place(ellip2, 12, bandY+42)

	pulseY := bandY + 78
	root.Place(label("Color pulse (paint-only) · pc.DC multi-script line", 11, 0.95, 0.75, 0.4, face), 12, pulseY)

	s.ColorPulse = rendering.NewRenderText("Color · 颜色 · 色 · 색 · Цвет · لون")
	s.ColorPulse.FontSize = 15
	s.ColorPulse.ApproxCharW = 0.7
	s.ColorPulse.SetRepaintBoundary(true)
	if face != nil {
		s.ColorPulse.SetFace(face)
	}

	s.DCPanel = panelBox(winW-24, 100, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.12, 0.14, 0.18, 1)
		strokeRect(pc, 1, 1, sz.Width-2, sz.Height-2, 1, 0.35, 0.4, 0.45, 1)
		if pc.DC == nil {
			return
		}
		if face != nil {
			pc.DC.SetFont(face)
		}
		pc.DC.SetRGBA(0.4+0.5*s.phase, 0.85, 1-0.3*s.phase, 1)
		pc.DC.DrawString("DC: Hello 你好 こんにちは 안녕 Привет שלום สวัสดี नमस्ते", pc.OriginX+12, pc.OriginY+32)
		pc.DC.SetRGBA(0.88, 0.9, 0.85, 1)
		pc.DC.DrawStringWrapped(
			"Wrapped: English 中文 日本語 한국어 Русский العربية עברית ไทย हिन्दी — MultiFace picks a face per glyph.",
			pc.OriginX+12, pc.OriginY+48, 0, 0, sz.Width-28, 1.3, render.AlignLeft,
		)
	})
	root.Place(s.DCPanel, 12, pulseY+18)
	root.Place(s.ColorPulse, 24, pulseY+18+8)

	noteY := pulseY + 130
	if noteY > winH-20 {
		noteY = winH - 20
	}
	root.Place(label(
		"Missing script? install fonts (noto-cjk, fonts-tlwg, lohit-devanagari) or GPUI_UI_FONT_* | RTL may look LTR without full BiDi UI",
		10, 0.5, 0.52, 0.55, face,
	), 12, noteY)

	return s
}

func (s *textScene) onTick(dt float64, schedule func()) {
	if s == nil {
		return
	}
	s.phase += dt * 0.6
	if s.phase > 1 {
		s.phase -= 1
	}
	if s.ColorPulse != nil {
		s.ColorPulse.SetColor(0.5+0.45*s.phase, 0.7, 1-0.4*s.phase, 1)
	}
	if s.DCPanel != nil {
		s.DCPanel.MarkNeedsPaint()
	}
	if schedule != nil {
		schedule()
	}
}
