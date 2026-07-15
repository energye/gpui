//go:build !nogpu

package render_test

// Phase A sigma composition probes D121–D140 — deeper multi-axis stress, not widgets.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// D121: modal stack — scrim × stacked dialogs × focus ring × action bar.
func TestP1_Comp_D121_ModalStackComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// page content
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.92-float64(i)*0.04, 0.93, 0.95)
		dc.DrawRoundedRectangle(20, 20+float64(i)*42, w-40, 36, 6)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(fmt.Sprintf("page row %d", i), 32, 42+float64(i)*42)
	}
	// scrim (alpha fill — not pure white page)
	dc.SetRGBA(0.05, 0.06, 0.08, 0.55)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// back dialog
	dc.PushLayer(render.BlendNormal, 0.98)
	dc.SetRGB(0.98, 0.98, 1)
	dc.DrawRoundedRectangle(70, 40, 300, 200, 12)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Settings", 90, 68)
	dc.PopLayer()
	// front dialog + focus ring
	dc.SetRGB(0.25, 0.55, 0.95)
	dc.SetLineWidth(3)
	dc.DrawRoundedRectangle(118, 88, 244, 164, 12)
	_ = dc.Stroke()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(120, 90, 240, 160, 10)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.25, 0.3)
	dc.DrawRoundedRectangle(140, 200, 90, 32, 6)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Confirm", 155, 220)
	dc.SetRGB(0.12, 0.14, 0.18)
	dc.DrawString("Overwrite file?", 140, 130)

	compMinGPU(t, dc, 12)
	r, g, b, _ := p1Sample(dc, 185, 216)
	if r < 120 {
		t.Fatalf("D121 confirm btn missing rgba=%d,%d,%d", r, g, b)
	}
	// scrim darkened page row band (was light gray cards)
	r2, g2, b2, _ := p1Sample(dc, 40, 40)
	if r2 > 230 && g2 > 230 && b2 > 230 {
		t.Fatalf("D121 scrim not applied rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D122: multi-column article — columns × pull-quote × drop-cap × caption.
func TestP1_Comp_D122_MultiColumnArticleComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.97, 0.97, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// drop cap
	_ = dc.LoadFontFace(font, 42)
	dc.SetRGB(0.85, 0.2, 0.25)
	dc.DrawString("A", 24, 70)
	_ = dc.LoadFontFace(font, 11)
	// three columns
	cols := []float64{70, 210, 350}
	for ci, cx := range cols {
		dc.ClipRect(cx, 24, 130, 220)
		for i := 0; i < 10; i++ {
			dc.SetRGB(0.15, 0.16, 0.18)
			dc.DrawString(fmt.Sprintf("col%d line %d text body", ci, i), cx+4, 40+float64(i)*20)
		}
		dc.ResetClip()
	}
	// pull quote
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.2, 0.45, 0.85)
	dc.DrawRectangle(24, 260, 6, 60)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.28, 0.35)
	dc.DrawString("Pull quote — composition not widgets.", 40, 290)
	dc.PopLayer()
	// caption bar
	dc.SetRGB(0.9, 0.55, 0.15)
	dc.DrawRoundedRectangle(360, 270, 140, 40, 6)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Fig. caption", 375, 294)

	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 30, 55)
	if r < 100 {
		t.Fatalf("D122 drop-cap missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 400, 285)
	if r2 < 120 {
		t.Fatalf("D122 caption missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D123: CAD blueprint — grid × dashed dims × annotations × selection.
func TestP1_Comp_D123_CADBlueprintComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.08, 0.12, 0.18)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// grid
	dc.SetRGB(0.12, 0.2, 0.28)
	dc.SetLineWidth(1)
	for x := 0; x < w; x += 20 {
		dc.DrawLine(float64(x), 0, float64(x), h)
		_ = dc.Stroke()
	}
	for y := 0; y < h; y += 20 {
		dc.DrawLine(0, float64(y), w, float64(y))
		_ = dc.Stroke()
	}
	// shape
	dc.SetRGB(0.3, 0.85, 0.95)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(80, 60, 220, 140, 4)
	_ = dc.Stroke()
	dc.DrawCircle(300, 160, 40)
	_ = dc.Stroke()
	// dashed dimensions
	dc.SetDash(6, 4)
	dc.SetRGB(0.95, 0.75, 0.2)
	dc.DrawLine(80, 220, 300, 220)
	_ = dc.Stroke()
	dc.SetDash()
	dc.DrawString("220.0 mm", 150, 236)
	// selection handles
	for _, p := range [][2]float64{{80, 60}, {300, 60}, {300, 200}, {80, 200}} {
		dc.SetRGB(1, 0.4, 0.2)
		dc.DrawRectangle(p[0]-4, p[1]-4, 8, 8)
		_ = dc.Fill()
	}
	// annotation bubble
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.95, 0.3, 0.35)
	dc.DrawRoundedRectangle(340, 40, 120, 48, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("NOTE #12", 355, 68)
	dc.PopLayer()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 360, 60)
	if r < 100 {
		t.Fatalf("D123 note missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 80, 60)
	if r2 < 150 {
		t.Fatalf("D123 handle missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D124: video call mosaic — tiles × speaking border × nameplates × controls.
func TestP1_Comp_D124_VideoCallMosaicComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	names := []string{"Ada", "Bob", "Cara", "Dan", "Eve", "Fay"}
	for i, name := range names {
		col, row := i%3, i/3
		x, y := 16+float64(col)*168, 16+float64(row)*140
		// tile bg image-like solid
		dc.SetRGB(0.2+float64(i)*0.05, 0.25, 0.4-float64(i)*0.03)
		dc.DrawRoundedRectangle(x, y, 156, 128, 10)
		_ = dc.Fill()
		if i == 1 {
			dc.SetRGB(0.2, 0.9, 0.45)
			dc.SetLineWidth(4)
			dc.DrawRoundedRectangle(x+2, y+2, 152, 124, 8)
			_ = dc.Stroke()
			// solid speaking chip (stroke AA may miss edge samples)
			dc.DrawRoundedRectangle(x+12, y+12, 36, 12, 4)
			_ = dc.Fill()
		}
		// nameplate
		dc.PushLayer(render.BlendNormal, 0.85)
		dc.SetRGB(0.05, 0.06, 0.08)
		dc.DrawRoundedRectangle(x+8, y+96, 80, 22, 6)
		_ = dc.Fill()
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawString(name, x+16, y+111)
		dc.PopLayer()
	}
	// control bar
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRoundedRectangle(140, 310, 240, 40, 20)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.25, 0.3)
	dc.DrawCircle(260, 330, 12)
	_ = dc.Fill()

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 260, 330)
	if r < 120 {
		t.Fatalf("D124 hangup missing rgba=%d,%d,%d", r, g, b)
	}
	// speaking chip inside Bob tile (col1,row0 -> x=184)
	r2, g2, b2, _ := p1Sample(dc, 200, 34)
	if g2 < 100 {
		t.Fatalf("D124 speaking chip missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D125: spreadsheet — formula bar × frozen panes × multi-range selection.
func TestP1_Comp_D125_SpreadsheetFrozenMultiRange(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// formula bar
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, 32)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.55, 0.9)
	dc.DrawRoundedRectangle(8, 6, 28, 20, 4)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("=SUM(A1:C3)+OFFSET(D4,1,0)", 44, 20)
	// grid
	dc.ClipRect(0, 32, w, h-32)
	for row := 0; row < 12; row++ {
		for col := 0; col < 8; col++ {
			x, y := float64(col)*64, 32+float64(row)*24
			if row == 0 || col == 0 {
				dc.SetRGB(0.88, 0.9, 0.93)
			} else if (row+col)%2 == 0 {
				dc.SetRGB(1, 1, 1)
			} else {
				dc.SetRGB(0.97, 0.98, 0.99)
			}
			dc.DrawRectangle(x, y, 64, 24)
			_ = dc.Fill()
			dc.SetRGB(0.25, 0.28, 0.32)
			dc.DrawString(fmt.Sprintf("%c%d", 'A'+col, row+1), x+8, y+16)
		}
	}
	// multi-range selection accents
	dc.SetRGBA(0.25, 0.55, 0.95, 0.35)
	dc.DrawRectangle(64, 56, 192, 72)
	_ = dc.Fill()
	dc.SetRGBA(0.95, 0.55, 0.2, 0.35)
	dc.DrawRectangle(256, 128, 128, 48)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)
	// active cell border
	dc.SetRGB(0.15, 0.45, 0.9)
	dc.SetLineWidth(2)
	dc.DrawRectangle(64, 56, 64, 24)
	_ = dc.Stroke()
	dc.ResetClip()

	compMinGPU(t, dc, 30)
	r, g, b, _ := p1Sample(dc, 20, 16)
	if b < 100 {
		t.Fatalf("D125 fx badge missing rgba=%d,%d,%d", r, g, b)
	}
	// selection tint over white cells should be bluish
	r2, g2, b2, _ := p1Sample(dc, 120, 80)
	if b2 < g2 {
		t.Fatalf("D125 blue selection missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D126: timeline scrubber — tracks × keyframes × playhead × waveform.
func TestP1_Comp_D126_TimelineScrubberComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// ruler
	dc.SetRGB(0.18, 0.2, 0.24)
	dc.DrawRectangle(0, 0, w, 28)
	_ = dc.Fill()
	for i := 0; i < 12; i++ {
		x := 20 + float64(i)*40
		dc.SetRGB(0.7, 0.75, 0.8)
		dc.DrawLine(x, 18, x, 28)
		_ = dc.Stroke()
		dc.DrawString(fmt.Sprintf("%ds", i), x-6, 14)
	}
	// tracks
	for tIdx := 0; tIdx < 4; tIdx++ {
		y := 40 + float64(tIdx)*50
		dc.SetRGB(0.16, 0.18, 0.22)
		dc.DrawRectangle(0, y, w, 44)
		_ = dc.Fill()
		// clip
		dc.SetRGB(0.25+float64(tIdx)*0.1, 0.45, 0.85-float64(tIdx)*0.1)
		dc.DrawRoundedRectangle(40+float64(tIdx)*30, y+8, 180+float64(tIdx)*20, 28, 6)
		_ = dc.Fill()
		// keyframes
		for k := 0; k < 5; k++ {
			dc.SetRGB(0.95, 0.85, 0.2)
			dc.DrawCircle(60+float64(tIdx)*30+float64(k)*35, y+22, 3)
			_ = dc.Fill()
		}
	}
	// waveform track decoration
	dc.SetRGB(0.3, 0.9, 0.55)
	for i := 0; i < 60; i++ {
		bh := 4 + 12*math.Abs(math.Sin(float64(i)*0.35))
		dc.DrawRectangle(50+float64(i)*6, 230-bh, 3, bh*2)
		_ = dc.Fill()
	}
	// playhead
	dc.SetRGB(0.95, 0.3, 0.35)
	dc.SetLineWidth(2)
	dc.DrawLine(200, 0, 200, h)
	_ = dc.Stroke()

	compMinGPU(t, dc, 40)
	r, g, b, _ := p1Sample(dc, 200, 100)
	if r < 100 {
		t.Fatalf("D126 playhead missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 80, 230)
	if g2 < 80 {
		t.Fatalf("D126 waveform missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D127: nested scrollports — sticky header × sticky col × floating selection.
func TestP1_Comp_D127_NestedScrollStickySelection(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// outer viewport content (scrolled)
	dc.ClipRect(0, 0, w, h)
	for i := 0; i < 14; i++ {
		dc.SetRGB(0.95-float64(i%3)*0.02, 0.96, 0.97)
		dc.DrawRectangle(80, float64(i)*40, w-80, 40)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(fmt.Sprintf("row content body %02d", i), 100, 24+float64(i)*40)
	}
	// sticky column
	dc.SetRGB(0.88, 0.9, 0.94)
	dc.DrawRectangle(0, 0, 80, h)
	_ = dc.Fill()
	for i := 0; i < 8; i++ {
		dc.SetRGB(0.15, 0.18, 0.22)
		dc.DrawString(fmt.Sprintf("L%02d", i), 16, 56+float64(i)*32)
	}
	// sticky header
	dc.PushLayer(render.BlendNormal, 0.96)
	dc.SetRGB(0.2, 0.45, 0.85)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Sticky Header × Nested Scroll", 16, 24)
	dc.PopLayer()
	// floating selection
	dc.SetRGB(0.95, 0.4, 0.2)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(120, 120, 200, 48, 6)
	_ = dc.Stroke()
	dc.SetRGBA(0.95, 0.4, 0.2, 0.15)
	dc.DrawRoundedRectangle(120, 120, 200, 48, 6)
	_ = dc.Fill()
	dc.ResetClip()

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 40, 18)
	if b < 100 {
		t.Fatalf("D127 sticky header missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 120, 120)
	if r2 < 100 {
		t.Fatalf("D127 selection missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D128: color picker panel — hue strip × SV plane × swatches × alpha.
func TestP1_Comp_D128_ColorPickerPanelComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.16, 0.17, 0.2)
	dc.DrawRoundedRectangle(12, 12, w-24, h-24, 12)
	_ = dc.Fill()
	// SV plane approximation
	for y := 0; y < 160; y++ {
		for x := 0; x < 160; x += 2 {
			s := float64(x) / 160
			v := 1 - float64(y)/160
			dc.SetRGB(v, v*(1-s*0.5), v*(1-s))
			dc.DrawRectangle(32+float64(x), 32+float64(y), 2, 1)
			_ = dc.Fill()
		}
	}
	// hue strip
	for i := 0; i < 160; i++ {
		t0 := float64(i) / 160
		dc.SetRGB(0.5+0.5*math.Sin(t0*6.28), 0.5+0.5*math.Sin(t0*6.28+2), 0.5+0.5*math.Sin(t0*6.28+4))
		dc.DrawRectangle(210, 32+float64(i), 18, 1)
		_ = dc.Fill()
	}
	// alpha bar
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(32, 210, 160, 14, 4)
	_ = dc.Fill()
	dc.PushLayer(render.BlendNormal, 0.4)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(32, 210, 60, 14)
	_ = dc.Fill()
	dc.PopLayer()
	// swatches
	cols := [][3]float64{{0.9, 0.2, 0.25}, {0.2, 0.7, 0.4}, {0.2, 0.45, 0.9}, {0.95, 0.75, 0.2}}
	for i, c := range cols {
		dc.SetRGB(c[0], c[1], c[2])
		dc.DrawRoundedRectangle(250, 40+float64(i)*40, 70, 28, 6)
		_ = dc.Fill()
	}
	dc.SetRGB(0.9, 0.92, 0.95)
	dc.DrawString("HEX #E64550", 32, 250)

	compMinGPU(t, dc, 40)
	r, g, b, _ := p1Sample(dc, 270, 50)
	if r < 100 {
		t.Fatalf("D128 red swatch missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 40, 216)
	if r2 < 100 {
		t.Fatalf("D128 alpha bar missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D129: isometric board — depth layers × cards × connectors.
func TestP1_Comp_D129_IsometricBoardComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	drawIso := func(x, y, s float64, r, g, b float64) {
		dc.SetRGB(r, g, b)
		dc.MoveTo(x, y)
		dc.LineTo(x+s, y-s*0.5)
		dc.LineTo(x, y-s)
		dc.LineTo(x-s, y-s*0.5)
		dc.ClosePath()
		_ = dc.Fill()
		dc.SetRGB(r*0.8, g*0.8, b*0.8)
		dc.MoveTo(x, y)
		dc.LineTo(x+s, y-s*0.5)
		dc.LineTo(x+s, y-s*0.5+s*0.4)
		dc.LineTo(x, y+s*0.4)
		dc.ClosePath()
		_ = dc.Fill()
	}
	drawIso(160, 200, 50, 0.3, 0.55, 0.9)
	drawIso(240, 180, 50, 0.9, 0.4, 0.3)
	drawIso(200, 240, 50, 0.3, 0.75, 0.45)
	// connectors
	dc.SetRGB(0.2, 0.25, 0.35)
	dc.SetLineWidth(2)
	dc.SetDash(4, 3)
	dc.DrawLine(160, 150, 240, 130)
	_ = dc.Stroke()
	dc.SetDash()
	// floating label
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRoundedRectangle(300, 40, 140, 48, 8)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("iso depth stack", 312, 68)
	dc.PopLayer()

	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 240, 150)
	p1NotNearWhite(t, "D129 iso tile", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 330, 60)
	if r2 > 80 && g2 > 80 && b2 > 80 && r2 < 50 {
		t.Fatalf("D129 label missing rgba=%d,%d,%d", r2, g2, b2)
	}
	// dark label body
	if r2 > 60 || g2 > 60 || b2 > 70 {
		// allow light text pixel; try darker sample
		r2, g2, b2, _ = p1Sample(dc, 310, 50)
	}
	if r2 > 100 && g2 > 100 && b2 > 100 {
		t.Fatalf("D129 label panel missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D130: multi-doc IDE — tabs × split editors × minimap × problems.
func TestP1_Comp_D130_MultiDocIDEComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// activity bar
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, 40, h)
	_ = dc.Fill()
	for i := 0; i < 5; i++ {
		dc.SetRGB(0.35+float64(i)*0.05, 0.55, 0.9)
		dc.DrawRoundedRectangle(8, 16+float64(i)*48, 24, 24, 4)
		_ = dc.Fill()
	}
	// tabs
	for i, name := range []string{"main.go", "render.go", "gpu.go"} {
		x := 48 + float64(i)*110
		if i == 0 {
			dc.SetRGB(0.18, 0.2, 0.26)
		} else {
			dc.SetRGB(0.14, 0.15, 0.18)
		}
		dc.DrawRoundedRectangle(x, 8, 100, 28, 6)
		_ = dc.Fill()
		dc.SetRGB(0.85, 0.88, 0.92)
		dc.DrawString(name, x+12, 26)
	}
	// split editors
	dc.ClipRect(48, 44, 360, 240)
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(48, 44, 180, 240)
	_ = dc.Fill()
	dc.SetRGB(0.16, 0.17, 0.2)
	dc.DrawRectangle(228, 44, 180, 240)
	_ = dc.Fill()
	dc.SetRGB(0.4, 0.85, 0.55)
	for i := 0; i < 8; i++ {
		dc.DrawString(fmt.Sprintf("%d  code line alpha", i+1), 56, 64+float64(i)*22)
		dc.DrawString(fmt.Sprintf("%d  split beta", i+1), 236, 64+float64(i)*22)
	}
	dc.ResetClip()
	// minimap
	dc.SetRGB(0.18, 0.2, 0.24)
	dc.DrawRectangle(420, 44, 60, 240)
	_ = dc.Fill()
	for i := 0; i < 30; i++ {
		dc.SetRGB(0.3, 0.55+float64(i%5)*0.05, 0.85)
		dc.DrawRectangle(428, 52+float64(i)*7, 20+float64(i%7)*3, 4)
		_ = dc.Fill()
	}
	// problems panel
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(48, 290, w-56, 60)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.4, 0.3)
	dc.DrawCircle(64, 320, 6)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.92, 0.95)
	dc.DrawString("2 errors · 4 warnings", 80, 324)

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 20, 28)
	if b < 80 {
		t.Fatalf("D130 activity icon missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 64, 320)
	if r2 < 100 {
		t.Fatalf("D130 problems accent missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D131: deep transform chain — rotate×scale×translate under nested clip + text.
func TestP1_Comp_D131_DeepTransformChainClipText(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRoundRect(40, 40, 280, 220, 16)
	dc.SetRGB(0.2, 0.35, 0.7)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// known-position warm plate under transform stack
	dc.Push()
	dc.Translate(180, 150)
	dc.Rotate(0.35)
	dc.Scale(1.05, 0.95)
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.95, 0.75, 0.15)
	dc.DrawRoundedRectangle(-70, -45, 140, 90, 10)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.12, 0.16)
	dc.DrawString("xform", -30, 5)
	dc.PopLayer()
	dc.SetRGB(0.95, 0.25, 0.3)
	dc.DrawCircle(50, -30, 14)
	_ = dc.Fill()
	dc.Pop()
	// non-rotated accent for structural check
	dc.SetRGB(0.95, 0.45, 0.15)
	dc.DrawRoundedRectangle(250, 200, 50, 30, 6)
	_ = dc.Fill()
	dc.ResetClip()

	compMinGPU(t, dc, 6)
	// blue clip fill should be visible
	r, g, b, _ := p1Sample(dc, 50, 50)
	if b < 80 {
		t.Fatalf("D131 clip fill missing rgba=%d,%d,%d", r, g, b)
	}
	// warm/red structural accents somewhere inside clip
	warm := 0
	for y := 60; y < 240; y += 3 {
		for x := 60; x < 320; x += 3 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr > 130 && (gg > 60 || bb < 100) && bb < 150 {
				warm++
			}
		}
	}
	if warm < 5 {
		t.Fatalf("D131 transformed warm accents low hits=%d", warm)
	}
	r2, g2, b2, _ := p1Sample(dc, 270, 210)
	if r2 < 100 {
		t.Fatalf("D131 static accent missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D132: advanced blend cascade strip over textured base.
func TestP1_Comp_D132_AdvancedBlendCascadeStrip(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 220
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// base photo-like gradient blocks
	for i := 0; i < 10; i++ {
		dc.SetRGB(0.3+float64(i)*0.05, 0.4, 0.7-float64(i)*0.04)
		dc.DrawRectangle(float64(i)*52, 0, 52, h)
		_ = dc.Fill()
	}
	modes := []struct {
		m    render.BlendMode
		name string
	}{
		{render.BlendMultiply, "Mul"},
		{render.BlendScreen, "Scr"},
		{render.BlendOverlay, "Ovl"},
		{render.BlendPlus, "Plus"},
		{render.BlendDarken, "Drk"},
		{render.BlendLighten, "Ltn"},
		{render.BlendDifference, "Dif"},
		{render.BlendExclusion, "Exc"},
	}
	for i, bm := range modes {
		x := 20 + float64(i)*60
		dc.SetBlendMode(bm.m)
		dc.SetRGBA(0.95, 0.45, 0.2, 1)
		dc.DrawRoundedRectangle(x, 40, 50, 100, 8)
		_ = dc.Fill()
		dc.SetBlendMode(render.BlendNormal)
		dc.SetRGB(0.05, 0.05, 0.08)
		dc.DrawString(bm.name, x+8, 170)
	}
	// badge
	dc.SetRGB(0.95, 0.2, 0.3)
	dc.DrawRoundedRectangle(w-90, 12, 70, 24, 8)
	_ = dc.Fill()

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, w-50, 20)
	if r < 100 {
		t.Fatalf("D132 badge missing rgba=%d,%d,%d", r, g, b)
	}
	// at least some non-base colors in blend row
	hits := 0
	for i := 0; i < 8; i++ {
		x := 40 + i*60
		rr, gg, bb, _ := p1Sample(dc, x, 90)
		if rr > 40 || gg > 40 || bb > 40 {
			hits++
		}
	}
	if hits < 6 {
		t.Fatalf("D132 blend cascade low hits=%d", hits)
	}
}

// D133: filter graph chain — blur × grayscale × color matrix over scene.
func TestP1_Comp_D133_FilterGraphChainComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// colorful scene
	dc.SetRGB(0.9, 0.25, 0.3)
	dc.DrawRoundedRectangle(30, 30, 120, 90, 12)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.55, 0.9)
	dc.DrawRoundedRectangle(120, 70, 140, 100, 12)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.8, 0.45)
	dc.DrawCircle(260, 80, 40)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.12, 0.16)
	dc.DrawString("pre-filter", 40, 200)

	// mild chain so structure remains
	dc.ApplyImageFilterGraph(
		render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.4},
		render.ImageFilterNode{Kind: render.ImageFilterColorMatrix, Matrix: [20]float32{
			1.1, 0, 0, 0, 0,
			0, 1.0, 0, 0, 0,
			0, 0, 0.9, 0, 0,
			0, 0, 0, 1, 0,
		}},
	)
	// post badge after filter graph
	dc.SetRGB(0.98, 0.8, 0.1)
	dc.DrawRoundedRectangle(240, 200, 100, 36, 6)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.DrawString("after", 260, 222)

	compMinGPU(t, dc, 4)
	// scan badge region (filter may slightly shift edges)
	badge := 0
	for y := 200; y < 236; y += 2 {
		for x := 240; x < 340; x += 2 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr > 140 && gg > 90 && bb < 120 {
				badge++
			}
		}
	}
	if badge < 5 {
		r, g, b, _ := p1Sample(dc, 280, 218)
		t.Fatalf("D133 post badge low hits=%d sample rgba=%d,%d,%d", badge, r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 90, 70)
	p1NotNearWhite(t, "D133 filtered body", r2, g2, b2)
}

// D134: masked gradient plate × particles × label.
func TestP1_Comp_D134_MaskedGradientParticlesLabel(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.08, 0.09, 0.12)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// mask
	mask := render.NewMask(w, h)
	compFillMaskRect(mask, 40, 40, 360, 200, 255)
	dc.PushMaskLayer(mask)
	grad := render.NewLinearGradientBrush(40, 40, 360, 200).
		AddColorStop(0, render.RGB(0.2, 0.4, 0.95)).
		AddColorStop(0.5, render.RGB(0.9, 0.3, 0.6)).
		AddColorStop(1, render.RGB(0.95, 0.75, 0.2))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// particles
	for i := 0; i < 40; i++ {
		dc.SetRGBA(1, 1, 1, 0.35+float64(i%5)*0.1)
		dc.DrawCircle(60+float64(i*8%280), 60+float64((i*13)%120), 2+float64(i%3))
		_ = dc.Fill()
	}
	dc.PopLayer()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("masked gradient × particles", 70, 240)

	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 100, 100)
	p1NotNearWhite(t, "D134 gradient body", r, g, b)
	// outside mask should stay dark bg
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 > 50 || g2 > 50 || b2 > 50 {
		t.Fatalf("D134 mask leak rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D135: infinite canvas frames — multi-frame × connectors × selection.
func TestP1_Comp_D135_InfiniteCanvasFramesComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// dotted canvas grid light
	dc.SetRGB(0.8, 0.82, 0.86)
	for y := 0; y < h; y += 16 {
		for x := 0; x < w; x += 16 {
			dc.DrawRectangle(float64(x), float64(y), 1, 1)
			_ = dc.Fill()
		}
	}
	frames := []struct {
		x, y, fw, fh float64
		title        string
		col          [3]float64
	}{
		{40, 40, 180, 120, "Frame A", [3]float64{0.95, 0.96, 0.98}},
		{260, 60, 200, 140, "Frame B", [3]float64{0.98, 0.97, 0.94}},
		{100, 200, 220, 100, "Frame C", [3]float64{0.94, 0.97, 0.98}},
	}
	for i, f := range frames {
		dc.SetRGB(f.col[0], f.col[1], f.col[2])
		dc.DrawRoundedRectangle(f.x, f.y, f.fw, f.fh, 8)
		_ = dc.Fill()
		if i == 1 {
			dc.SetRGB(0.25, 0.55, 0.95)
			dc.SetLineWidth(2)
			dc.DrawRoundedRectangle(f.x, f.y, f.fw, f.fh, 8)
			_ = dc.Stroke()
		} else {
			dc.SetRGB(0.7, 0.72, 0.78)
			dc.SetLineWidth(1)
			dc.DrawRoundedRectangle(f.x, f.y, f.fw, f.fh, 8)
			_ = dc.Stroke()
		}
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(f.title, f.x+12, f.y+22)
		// fake content blocks
		dc.SetRGB(0.3+float64(i)*0.15, 0.5, 0.85-float64(i)*0.1)
		dc.DrawRoundedRectangle(f.x+16, f.y+40, 80, 40, 6)
		_ = dc.Fill()
	}
	// connectors
	dc.SetRGB(0.35, 0.4, 0.5)
	dc.SetLineWidth(2)
	dc.DrawLine(220, 100, 260, 120)
	_ = dc.Stroke()
	dc.DrawLine(200, 160, 160, 200)
	_ = dc.Stroke()
	// selection handles on B
	for _, p := range [][2]float64{{260, 60}, {460, 60}, {460, 200}, {260, 200}} {
		dc.SetRGB(0.25, 0.55, 0.95)
		dc.DrawRectangle(p[0]-4, p[1]-4, 8, 8)
		_ = dc.Fill()
	}

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 280, 90)
	p1NotNearWhite(t, "D135 frame B content", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 260, 60)
	if b2 < 100 {
		t.Fatalf("D135 handle missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D136: HiDPI switch mid composition — dpr change × redraw × badge.
func TestP1_Comp_D136_HiDPISwitchMidComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 240, 180
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.2, 0.45, 0.85)
	dc.DrawRoundedRectangle(20, 20, 120, 80, 10)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.12, 0.16)
	dc.DrawString("dpr1", 40, 65)

	// switch to higher DPR if supported via Scale / transform proxy
	dc.Push()
	dc.Scale(1.5, 1.5)
	dc.SetRGB(0.9, 0.35, 0.3)
	dc.DrawRoundedRectangle(90, 50, 70, 40, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("dpr+", 100, 75)
	dc.Pop()

	dc.SetRGB(0.25, 0.75, 0.45)
	dc.DrawCircle(50, 140, 18)
	_ = dc.Fill()

	compMinGPU(t, dc, 5)
	r, g, b, _ := p1Sample(dc, 40, 40)
	if b < 80 {
		t.Fatalf("D136 base card missing rgba=%d,%d,%d", r, g, b)
	}
	// scaled red card lands around 135,75
	r2, g2, b2, _ := p1Sample(dc, 150, 100)
	if r2 < 80 {
		// scan
		found := false
		for y := 70; y < 140; y += 2 {
			for x := 120; x < 220; x += 2 {
				rr, gg, bb, _ := p1Sample(dc, x, y)
				if rr > 140 && gg < 120 {
					found = true
					break
				}
				_ = bb
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatalf("D136 scaled card missing rgba=%d,%d,%d", r2, g2, b2)
		}
	}
}

// D137: multi-pass damage — WritePixels stamps + partial region redraw + overlay.
func TestP1_Comp_D137_MultiPassDamageStampComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// base panels
	for i := 0; i < 3; i++ {
		dc.SetRGB(0.92, 0.93, 0.95)
		dc.DrawRoundedRectangle(16+float64(i)*112, 24, 100, 160, 10)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(fmt.Sprintf("P%d", i+1), 40+float64(i)*112, 50)
	}
	// damage stamps via WritePixels
	for i := 0; i < 3; i++ {
		buf := make([]byte, 12*12*4)
		for p := 0; p < 12*12; p++ {
			buf[p*4+0] = uint8(40 + i*80)
			buf[p*4+1] = uint8(180 - i*40)
			buf[p*4+2] = uint8(90 + i*50)
			buf[p*4+3] = 255
		}
		dc.WritePixels(40+i*112, 80, 12, 12, buf)
	}
	// overlay badge after damage
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.9, 0.25, 0.35)
	dc.DrawRoundedRectangle(w-90, 16, 70, 24, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("DMG", w-75, 32)
	dc.PopLayer()

	compMinGPU(t, dc, 6)
	r, g, b, _ := p1Sample(dc, 46, 86)
	p1NotNearWhite(t, "D137 stamp0", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, w-50, 24)
	if r2 < 100 {
		t.Fatalf("D137 badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D138: mixed text modes under layer opacity + path clip.
func TestP1_Comp_D138_MixedTextModesLayerClip(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 16)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.18, 0.28)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// path clip diamond-ish
	dc.MoveTo(w/2, 20)
	dc.LineTo(w-30, h/2)
	dc.LineTo(w/2, h-20)
	dc.LineTo(30, h/2)
	dc.ClosePath()
	dc.Clip()

	dc.PushLayer(render.BlendNormal, 0.92)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	modes := []struct {
		m render.TextMode
		s string
		y float64
		c [3]float64
	}{
		{render.TextModeAuto, "Auto text mode", 70, [3]float64{0.1, 0.12, 0.16}},
		{render.TextModeGlyphMask, "GlyphMask text", 105, [3]float64{0.85, 0.25, 0.3}},
		{render.TextModeVector, "Vector text", 140, [3]float64{0.2, 0.45, 0.85}},
		{render.TextModeBitmap, "Bitmap text", 175, [3]float64{0.2, 0.65, 0.4}},
	}
	for _, tm := range modes {
		dc.SetTextMode(tm.m)
		dc.SetRGB(tm.c[0], tm.c[1], tm.c[2])
		dc.DrawString(tm.s, 90, tm.y)
	}
	dc.SetTextMode(render.TextModeAuto)
	dc.PopLayer()
	dc.ResetClip()

	// outside clip marker
	dc.SetRGB(0.95, 0.6, 0.15)
	dc.DrawCircle(24, 24, 10)
	_ = dc.Fill()

	compMinGPU(t, dc, 6)
	r, g, b, _ := p1Sample(dc, 24, 24)
	if r < 100 {
		t.Fatalf("D138 outside marker missing rgba=%d,%d,%d", r, g, b)
	}
	// inside clip should be light plate
	r2, g2, b2, _ := p1Sample(dc, w/2, h/2)
	if r2 < 150 {
		t.Fatalf("D138 clipped plate missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D139: pattern fill × dash stroke × image clip × text plate.
func TestP1_Comp_D139_PatternDashImageClipText(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// pattern-like hatch via loops (portable)
	dc.ClipRoundRect(30, 30, 240, 180, 16)
	for i := 0; i < 40; i++ {
		dc.SetRGB(0.85, 0.88, 0.95)
		dc.DrawRectangle(0, float64(i)*8, w, 4)
		_ = dc.Fill()
		dc.SetRGB(0.75, 0.8, 0.9)
		dc.DrawRectangle(0, float64(i)*8+4, w, 4)
		_ = dc.Fill()
	}
	img := compMakeImage(t, 48, 48, 220, 80, 40)
	dc.DrawImage(img, 50, 50)
	dc.DrawImage(img, 120, 90)
	dc.SetRGB(0.25, 0.45, 0.85)
	dc.SetLineWidth(3)
	dc.SetDash(8, 5)
	dc.DrawRoundedRectangle(40, 40, 200, 140, 12)
	_ = dc.Stroke()
	dc.SetDash()
	dc.ResetClip()

	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRoundedRectangle(200, 200, 170, 48, 8)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("pattern×dash×img", 214, 228)
	dc.PopLayer()

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 60, 60)
	p1NotNearWhite(t, "D139 image", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 220, 220)
	if r2 > 80 || g2 > 80 {
		// dark plate expected
		if r2 > 100 && g2 > 100 && b2 > 100 {
			t.Fatalf("D139 text plate missing rgba=%d,%d,%d", r2, g2, b2)
		}
	}
}

// D140: kitchen-sink v4 — multi-axis stress lattice + overlays + filters accent.
func TestP1_Comp_D140_KitchenSinkV4Stress(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 400
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRoundRect(8, 8, w-16, h-16, 14)
	// chrome
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("KitchenSink v4 — arbitrary composition stress", 16, 26)

	// sidebar
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRectangle(0, 40, 120, h-40)
	_ = dc.Fill()
	for i := 0; i < 8; i++ {
		if i == 2 {
			dc.SetRGB(0.25, 0.45, 0.85)
		} else {
			dc.SetRGB(0.2, 0.22, 0.28)
		}
		dc.DrawRoundedRectangle(10, 56+float64(i)*36, 100, 28, 6)
		_ = dc.Fill()
		dc.SetRGB(0.9, 0.92, 0.95)
		dc.DrawString(fmt.Sprintf("nav-%d", i), 24, 74+float64(i)*36)
	}

	// content lattice
	dc.ClipRect(130, 50, w-150, h-70)
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			x := 140 + float64(col)*50
			y := 56 + float64(row)*38
			dc.SetBlendMode(render.BlendNormal)
			dc.SetRGB(0.25+float64(col)*0.05, 0.35+float64(row)*0.04, 0.75)
			dc.DrawRoundedRectangle(x, y, 42, 30, 5)
			_ = dc.Fill()
			if (row+col)%3 == 0 {
				dc.SetBlendMode(render.BlendMultiply)
				dc.SetRGBA(1, 0.7, 0.4, 1)
				dc.DrawCircle(x+21, y+15, 10)
				_ = dc.Fill()
				dc.SetBlendMode(render.BlendNormal)
			}
			if col%4 == 0 {
				dc.PushLayer(render.BlendNormal, 0.55)
				dc.SetRGB(1, 1, 1)
				dc.DrawRectangle(x+4, y+4, 12, 12)
				_ = dc.Fill()
				dc.PopLayer()
			}
		}
	}
	// floating inspector
	dc.PushLayer(render.BlendNormal, 0.96)
	dc.SetRGB(0.98, 0.98, 1)
	dc.DrawRoundedRectangle(340, 70, 180, 160, 10)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(356, 90, 60, 24, 6)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("inspector", 360, 140)
	dc.DrawString("axes: clip/layer/blend", 360, 164)
	dc.PopLayer()
	dc.ResetClip()
	dc.ResetClip()

	// toast
	dc.SetRGB(0.2, 0.7, 0.45)
	dc.DrawRoundedRectangle(w-170, h-50, 140, 30, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("compose ok", w-150, h-30)

	// light filter accent on whole (tiny)
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.2})

	compMinGPU(t, dc, 50)
	r, g, b, _ := p1Sample(dc, 160, 70)
	if b < 60 {
		t.Fatalf("D140 lattice cell missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 370, 100)
	if r2 < 100 {
		t.Fatalf("D140 inspector accent missing rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, w-100, h-35)
	if g3 < 80 {
		t.Fatalf("D140 toast missing rgba=%d,%d,%d", r3, g3, b3)
	}
}
