//go:build !nogpu

package render_test

// Phase A tau composition probes D141+ — more multi-axis stress, not widgets.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// D141: command palette over dense workspace — scrim × list × shortcuts × selection.
func TestP1_Comp_D141_CommandPaletteOverWorkspace(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// dense workspace
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i := 0; i < 8; i++ {
		dc.SetRGB(0.18, 0.2, 0.25)
		dc.DrawRoundedRectangle(12, 12+float64(i)*40, w-24, 34, 6)
		_ = dc.Fill()
		dc.SetRGB(0.7, 0.75, 0.82)
		dc.DrawString(fmt.Sprintf("workspace row content %02d", i), 24, 34+float64(i)*40)
	}
	// scrim
	dc.SetRGBA(0.02, 0.03, 0.05, 0.55)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// palette
	dc.PushLayer(render.BlendNormal, 0.98)
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRoundedRectangle(80, 48, 360, 260, 12)
	_ = dc.Fill()
	dc.SetRGB(0.22, 0.24, 0.3)
	dc.DrawRoundedRectangle(96, 64, 328, 36, 8)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.92, 0.95)
	dc.DrawString("> open file, run test, toggle...", 108, 86)
	for i, cmd := range []string{"Open File", "Run Comp Suite", "Toggle GPU Path", "Show Path Stats", "Reload Fonts"} {
		y := 116 + float64(i)*34
		if i == 1 {
			dc.SetRGB(0.25, 0.45, 0.85)
			dc.DrawRoundedRectangle(96, y-8, 328, 30, 6)
			_ = dc.Fill()
		}
		dc.SetRGB(0.92, 0.94, 0.97)
		dc.DrawString(cmd, 112, y+10)
		dc.SetRGB(0.55, 0.6, 0.7)
		dc.DrawString(fmt.Sprintf("Ctrl+%d", i+1), 360, y+10)
	}
	dc.PopLayer()

	compMinGPU(t, dc, 15)
	// selection row i=1 at y≈150
	r, g, b, _ := p1Sample(dc, 200, 152)
	if b < 80 {
		t.Fatalf("D141 selection missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 20, 20)
	if r2 > 90 && g2 > 90 && b2 > 90 {
		t.Fatalf("D141 workspace/scrim too bright rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D142: nested popover cascade — menu × submenu × tip × outside dismiss dim.
func TestP1_Comp_D142_NestedPopoverCascade(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// menubar
	dc.SetRGB(0.2, 0.45, 0.85)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("File  Edit  View  Help", 16, 24)
	// L1 menu
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(40, 40, 140, 180, 8)
	_ = dc.Fill()
	dc.SetRGB(0.85, 0.87, 0.9)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(40, 40, 140, 180, 8)
	_ = dc.Stroke()
	for i, s := range []string{"New", "Open", "Share ▸", "Export", "Quit"} {
		y := 60 + float64(i)*30
		if i == 2 {
			dc.SetRGB(0.25, 0.5, 0.9)
			dc.DrawRectangle(44, y-12, 132, 26)
			_ = dc.Fill()
			dc.SetRGB(1, 1, 1)
		} else {
			dc.SetRGB(0.15, 0.16, 0.2)
		}
		dc.DrawString(s, 56, y)
	}
	// L2 submenu
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(176, 100, 150, 120, 8)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRectangle(180, 128, 142, 26)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Copy link", 192, 146)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Email", 192, 176)
	dc.DrawString("Embed", 192, 206)
	// L3 tip
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRoundedRectangle(330, 120, 120, 40, 6)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("permalink", 342, 144)
	dc.PopLayer()

	compMinGPU(t, dc, 12)
	r, g, b, _ := p1Sample(dc, 200, 140)
	if r < 100 {
		t.Fatalf("D142 L2 accent missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 360, 135)
	if r2 > 80 && g2 > 80 && b2 > 80 {
		// dark tip panel
		r2, g2, b2, _ = p1Sample(dc, 340, 130)
	}
	if r2 > 100 && g2 > 100 && b2 > 100 {
		t.Fatalf("D142 tip panel missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D143: dual-viewport mock — two compose regions × independent clips × shared chrome.
func TestP1_Comp_D143_DualViewportMockComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, 0, w, 32)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("dual viewport mock", 12, 22)

	// left viewport
	dc.ClipRect(8, 40, 260, 240)
	dc.SetRGB(0.2, 0.35, 0.7)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.75, 0.2)
	dc.DrawRoundedRectangle(30, 70, 160, 80, 10)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.DrawString("LEFT VP", 70, 115)
	dc.ResetClip()

	// right viewport
	dc.ClipRect(292, 40, 260, 240)
	dc.SetRGB(0.2, 0.55, 0.4)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.35, 0.4)
	dc.DrawCircle(420, 140, 50)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("RIGHT VP", 390, 145)
	dc.ResetClip()

	// gutter
	dc.SetRGB(0.85, 0.3, 0.35)
	dc.DrawRectangle(272, 40, 16, 240)
	_ = dc.Fill()

	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 100, 100)
	if r < 100 {
		t.Fatalf("D143 left accent missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 420, 140)
	if r2 < 100 {
		t.Fatalf("D143 right accent missing rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 280, 100)
	if r3 < 100 {
		t.Fatalf("D143 gutter missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D144: sankey flow density — bands × nodes × labels × clip legend.
func TestP1_Comp_D144_SankeyFlowDensityComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// source nodes
	src := []struct {
		y, hh float64
		c     [3]float64
	}{
		{40, 70, [3]float64{0.25, 0.5, 0.9}},
		{130, 50, [3]float64{0.9, 0.4, 0.3}},
		{200, 80, [3]float64{0.3, 0.75, 0.45}},
	}
	for i, s := range src {
		dc.SetRGB(s.c[0], s.c[1], s.c[2])
		dc.DrawRoundedRectangle(30, s.y, 24, s.hh, 4)
		_ = dc.Fill()
		// flow band as thick bezier-ish polyline fill via rects
		for t0 := 0; t0 < 20; t0++ {
			tt := float64(t0) / 19
			x := 54 + tt*300
			y := s.y + s.hh*0.3 + math.Sin(tt*math.Pi)*30 + float64(i)*8
			dc.SetRGBA(s.c[0], s.c[1], s.c[2], 0.45)
			dc.DrawRectangle(x, y, 16, 10+s.hh*0.25)
			_ = dc.Fill()
		}
	}
	// sink nodes
	for i := 0; i < 4; i++ {
		dc.SetRGB(0.2+float64(i)*0.1, 0.25, 0.55)
		dc.DrawRoundedRectangle(420, 40+float64(i)*60, 28, 40, 4)
		_ = dc.Fill()
	}
	// legend clip
	dc.ClipRect(20, 280, 200, 30)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("sankey flow density", 24, 300)
	dc.ResetClip()
	dc.SetRGB(0.95, 0.55, 0.15)
	dc.DrawRoundedRectangle(400, 280, 90, 24, 6)
	_ = dc.Fill()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 42, 70)
	if b < 80 {
		t.Fatalf("D144 source node missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 430, 290)
	if r2 < 100 {
		t.Fatalf("D144 legend badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D145: map composition — tiles × route × markers × legend × compass.
func TestP1_Comp_D145_MapTilesRouteMarkersComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// tiles
	for row := 0; row < 6; row++ {
		for col := 0; col < 8; col++ {
			x, y := float64(col)*60, float64(row)*50
			if (row+col)%2 == 0 {
				dc.SetRGB(0.82, 0.9, 0.82)
			} else {
				dc.SetRGB(0.78, 0.86, 0.78)
			}
			dc.DrawRectangle(x, y, 60, 50)
			_ = dc.Fill()
			// water tile
			if row == 2 && col >= 3 && col <= 5 {
				dc.SetRGB(0.55, 0.75, 0.92)
				dc.DrawRectangle(x, y, 60, 50)
				_ = dc.Fill()
			}
		}
	}
	// route
	dc.SetRGB(0.2, 0.45, 0.95)
	dc.SetLineWidth(4)
	pts := [][2]float64{{40, 280}, {120, 200}, {220, 180}, {320, 120}, {400, 80}}
	for i := 0; i+1 < len(pts); i++ {
		dc.DrawLine(pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1])
		_ = dc.Stroke()
	}
	// markers
	for i, p := range pts {
		if i == len(pts)-1 {
			dc.SetRGB(0.9, 0.25, 0.3)
		} else {
			dc.SetRGB(0.15, 0.2, 0.35)
		}
		dc.DrawCircle(p[0], p[1], 6)
		_ = dc.Fill()
	}
	// legend + compass
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(12, 12, 110, 70, 8)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.45, 0.95)
	dc.DrawRectangle(24, 28, 20, 4)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("route", 50, 34)
	dc.SetRGB(0.9, 0.25, 0.3)
	dc.DrawCircle(34, 56, 5)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("end", 50, 60)
	dc.PopLayer()
	dc.SetRGB(0.95, 0.6, 0.15)
	dc.DrawCircle(w-36, 36, 18)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.DrawString("N", w-42, 40)

	compMinGPU(t, dc, 30)
	r, g, b, _ := p1Sample(dc, 400, 80)
	if r < 100 {
		t.Fatalf("D145 end marker missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-36, 36)
	if r2 < 100 {
		t.Fatalf("D145 compass missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D146: trading terminal — order book × spark chart × trades tape × badge.
func TestP1_Comp_D146_TradingTerminalComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.08, 0.09, 0.12)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// order book
	dc.ClipRect(8, 8, 160, 280)
	for i := 0; i < 12; i++ {
		y := 12 + float64(i)*22
		if i < 6 {
			dc.SetRGB(0.15, 0.35, 0.22)
		} else {
			dc.SetRGB(0.35, 0.15, 0.18)
		}
		dc.DrawRectangle(12, y, 40+float64(i%5)*18, 18)
		_ = dc.Fill()
		dc.SetRGB(0.85, 0.9, 0.95)
		dc.DrawString(fmt.Sprintf("%.2f", 100-float64(i)*0.3), 100, y+12)
	}
	dc.ResetClip()
	// chart
	dc.ClipRect(180, 8, 240, 200)
	dc.SetRGB(0.12, 0.14, 0.18)
	dc.DrawRectangle(180, 8, 240, 200)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.85, 0.55)
	dc.SetLineWidth(2)
	prevY := 150.0
	for i := 0; i < 40; i++ {
		x := 190 + float64(i)*5.5
		y := 100 + 40*math.Sin(float64(i)*0.35) + float64(i%7)
		dc.DrawLine(x, prevY, x+5.5, y)
		_ = dc.Stroke()
		prevY = y
	}
	dc.ResetClip()
	// trades tape
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(430, 8, 120, 280)
	_ = dc.Fill()
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			dc.SetRGB(0.3, 0.85, 0.5)
		} else {
			dc.SetRGB(0.95, 0.4, 0.4)
		}
		dc.DrawString(fmt.Sprintf("%d  %.2f", 10-i, 99.5+float64(i)*0.1), 440, 30+float64(i)*24)
	}
	// badge
	dc.SetRGB(0.95, 0.75, 0.2)
	dc.DrawRoundedRectangle(20, 310, 100, 28, 8)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.DrawString("BTC-PERP", 32, 328)

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 50, 320)
	if r < 100 {
		t.Fatalf("D146 badge missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 30, 40)
	p1NotNearWhite(t, "D146 book", r2, g2, b2)
}

// D147: coverage heatmap overlay on code surface.
func TestP1_Comp_D147_CoverageHeatmapCodeOverlay(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// gutters + lines
	for i := 0; i < 12; i++ {
		y := 20 + float64(i)*22
		// coverage bar
		if i%4 == 0 {
			dc.SetRGB(0.9, 0.3, 0.3)
		} else if i%3 == 0 {
			dc.SetRGB(0.95, 0.75, 0.2)
		} else {
			dc.SetRGB(0.25, 0.75, 0.4)
		}
		dc.DrawRectangle(0, y-10, 8, 20)
		_ = dc.Fill()
		dc.SetRGB(0.75, 0.78, 0.85)
		dc.DrawString(fmt.Sprintf("%2d  fn cover line sample content", i+1), 20, y)
	}
	// heatmap overlay strip
	dc.PushLayer(render.BlendNormal, 0.25)
	dc.SetRGB(0.95, 0.2, 0.25)
	dc.DrawRectangle(0, 84, w, 44)
	_ = dc.Fill()
	dc.PopLayer()
	// summary chip
	dc.SetRGB(0.25, 0.75, 0.45)
	dc.DrawRoundedRectangle(w-120, 12, 100, 26, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("92% cov", w-100, 30)

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 4, 20)
	if g < 60 && r < 60 {
		t.Fatalf("D147 gutter missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-70, 24)
	if g2 < 80 {
		t.Fatalf("D147 cov chip missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D148: recursive card stack with offset depth + selected top.
func TestP1_Comp_D148_RecursiveCardStackDepth(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		off := float64(i) * 14
		dc.PushLayer(render.BlendNormal, 0.85+float64(i)*0.02)
		dc.SetRGB(0.98-float64(i)*0.03, 0.98, 1)
		dc.DrawRoundedRectangle(60+off, 40+off, 220, 140, 12)
		_ = dc.Fill()
		if i == 5 {
			dc.SetRGB(0.25, 0.5, 0.95)
			dc.SetLineWidth(3)
			dc.DrawRoundedRectangle(60+off, 40+off, 220, 140, 12)
			_ = dc.Stroke()
			dc.SetRGB(0.9, 0.3, 0.35)
			dc.DrawRoundedRectangle(80+off, 70+off, 80, 28, 6)
			_ = dc.Fill()
		}
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(fmt.Sprintf("card %d", i), 80+off, 120+off)
		dc.PopLayer()
	}

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 150, 140)
	if r < 80 {
		t.Fatalf("D148 top accent missing rgba=%d,%d,%d", r, g, b)
	}
}

// D149: before/after filter compare strip with divider.
func TestP1_Comp_D149_BeforeAfterFilterCompare(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// left colorful before
	dc.ClipRect(0, 0, w/2, h)
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(30, 40, 140, 120, 12)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.55, 0.9)
	dc.DrawCircle(160, 120, 40)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("BEFORE", 50, 200)
	dc.ResetClip()
	// right after-ish grayscale panel
	dc.ClipRect(w/2, 0, w/2, h)
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(30, 40, 140, 120, 12)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.55, 0.9)
	dc.DrawCircle(160, 120, 40)
	_ = dc.Fill()
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterGrayscale})
	// re-draw labels after filter on right only won't work globally — badge after full
	dc.ResetClip()
	// divider
	dc.SetRGB(0.95, 0.85, 0.2)
	dc.DrawRectangle(float64(w/2-2), 0, 4, h)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("AFTER", w/2+40, 200)
	dc.SetRGB(0.95, 0.4, 0.2)
	dc.DrawRoundedRectangle(w-90, 12, 70, 24, 8)
	_ = dc.Fill()

	compMinGPU(t, dc, 6)
	r, g, b, _ := p1Sample(dc, w/2, 100)
	if r < 100 && g < 100 {
		t.Fatalf("D149 divider missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-50, 20)
	if r2 < 100 {
		t.Fatalf("D149 badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D150: dense atlas toolbar icons × overflow menu × badge.
func TestP1_Comp_D150_DenseAtlasToolbarOverflow(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 160
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRoundedRectangle(8, 40, w-16, 56, 10)
	_ = dc.Fill()

	atlas := compMakeImage(t, 64, 16, 0, 0, 0)
	// paint colored cells into atlas
	cols := [][3]uint8{{220, 70, 70}, {70, 180, 90}, {70, 120, 220}, {220, 180, 50}}
	for i, c := range cols {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				_ = atlas.SetRGBA(i*16+x, y, c[0], c[1], c[2], 255)
			}
		}
	}
	sprites := make([]render.AtlasSprite, 0, 12)
	for i := 0; i < 12; i++ {
		src := float64(i % 4)
		sprites = append(sprites, render.AtlasSprite{
			SrcX: src * 16, SrcY: 0, SrcW: 16, SrcH: 16,
			DstX: 20 + float64(i)*34, DstY: 52, DstW: 28, DstH: 28,
			Opacity: 1,
		})
	}
	dc.DrawAtlas(atlas, sprites)
	// overflow
	dc.SetRGB(0.95, 0.35, 0.4)
	dc.DrawRoundedRectangle(w-70, 52, 40, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("•••", w-58, 70)
	// notif badge
	dc.SetRGB(0.95, 0.25, 0.3)
	dc.DrawCircle(w-28, 44, 8)
	_ = dc.Fill()

	compMinGPU(t, dc, 5)
	r, g, b, _ := p1Sample(dc, 34, 66)
	p1NotNearWhite(t, "D150 atlas icon", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, w-50, 66)
	if r2 < 100 {
		t.Fatalf("D150 overflow missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D151: mesh terrain fan + contour labels + clip legend.
func TestP1_Comp_D151_MeshTerrainContourLabels(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.14, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// fan mesh
	pos := []render.Point{{X: 200, Y: 150}}
	cols := []render.RGBA{{R: 0.2, G: 0.5, B: 0.9, A: 1}}
	for i := 0; i <= 12; i++ {
		a := float64(i) / 12 * 2 * math.Pi
		pos = append(pos, render.Point{X: 200 + 110*math.Cos(a), Y: 150 + 90*math.Sin(a)})
		cols = append(cols, render.RGBA{
			R: 0.2 + 0.6*float64(i%3)/3,
			G: 0.4 + 0.4*float64(i%4)/4,
			B: 0.3 + 0.5*float64(i%5)/5,
			A: 1,
		})
	}
	dc.DrawVertices(pos, cols, render.VertexModeTriangleFan)
	// contours
	dc.SetRGB(0.95, 0.95, 0.9)
	dc.SetLineWidth(1)
	dc.SetDash(3, 3)
	dc.DrawCircle(200, 150, 50)
	_ = dc.Stroke()
	dc.DrawCircle(200, 150, 80)
	_ = dc.Stroke()
	dc.SetDash()
	dc.SetRGB(0.95, 0.8, 0.3)
	dc.DrawString("120m", 250, 100)
	dc.DrawString("80m", 270, 150)
	// legend
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(16, 16, 70, 24, 6)
	_ = dc.Fill()

	compMinGPU(t, dc, 4)
	r, g, b, _ := p1Sample(dc, 220, 150)
	p1NotNearWhite(t, "D151 mesh", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 40, 24)
	if r2 < 100 {
		t.Fatalf("D151 legend missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D152: PresentFrameDamage multi-rect after partial panel updates.
func TestP1_Comp_D152_PresentFrameDamageMultiRect(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// three panels
	for i := 0; i < 3; i++ {
		dc.SetRGB(0.9, 0.91, 0.93)
		dc.DrawRoundedRectangle(16+float64(i)*112, 24, 100, 160, 10)
		_ = dc.Fill()
	}
	// damage updates
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(28, 48, 70, 40, 6)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.55, 0.9)
	dc.DrawRoundedRectangle(140, 100, 70, 40, 6)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.75, 0.45)
	dc.DrawRoundedRectangle(250, 140, 70, 40, 6)
	_ = dc.Fill()

	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()
	rects := []image.Rectangle{
		image.Rect(28, 48, 98, 88),
		image.Rect(140, 100, 210, 140),
		image.Rect(250, 140, 320, 180),
	}
	called := false
	if err := dc.PresentFrameDamageRects(view, uint32(w), uint32(h), rects, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("PresentFrameDamageRects: %v", err)
	}
	if !called {
		t.Fatal("present not called")
	}
	p1Flush(t, dc)
	r, g, b, _ := p1Sample(dc, 50, 60)
	if r < 100 {
		t.Fatalf("D152 red damage missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 170, 120)
	if b2 < 80 {
		t.Fatalf("D152 blue damage missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D153: nested backdrop translucent panels over busy scene.
func TestP1_Comp_D153_NestedBackdropTranslucentPanels(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// busy background
	for i := 0; i < 10; i++ {
		dc.SetRGB(0.3+float64(i)*0.05, 0.4, 0.75-float64(i)*0.04)
		dc.DrawRoundedRectangle(20+float64(i)*18, 20+float64(i%3)*30, 90, 50, 8)
		_ = dc.Fill()
	}
	dc.PushBackdropLayer(render.BlendNormal, 0.85)
	dc.SetRGBA(0.98, 0.98, 1, 0.9)
	dc.DrawRoundedRectangle(80, 60, 200, 120, 12)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("backdrop panel A", 100, 120)
	dc.PopLayer()
	dc.PushBackdropLayer(render.BlendNormal, 0.75)
	dc.SetRGBA(0.2, 0.25, 0.35, 0.9)
	dc.DrawRoundedRectangle(180, 140, 180, 100, 12)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("backdrop panel B", 200, 190)
	dc.PopLayer()
	// accent
	dc.SetRGB(0.95, 0.35, 0.3)
	dc.DrawCircle(360, 60, 16)
	_ = dc.Fill()

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 360, 60)
	if r < 100 {
		t.Fatalf("D153 accent missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 160, 100)
	p1NotNearWhite(t, "D153 panel", r2, g2, b2)
}

// D154: path art dash/join gallery under nested clip + labels.
func TestP1_Comp_D154_PathDashJoinGalleryClip(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRoundRect(16, 16, w-32, h-32, 14)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	styles := []struct {
		dash []float64
		col  [3]float64
		y    float64
	}{
		{[]float64{12, 6}, [3]float64{0.3, 0.7, 0.95}, 60},
		{[]float64{4, 4}, [3]float64{0.95, 0.5, 0.3}, 110},
		{[]float64{16, 4, 4, 4}, [3]float64{0.4, 0.9, 0.5}, 160},
		{[]float64{2, 6}, [3]float64{0.95, 0.85, 0.3}, 210},
	}
	for _, st := range styles {
		dc.SetRGB(st.col[0], st.col[1], st.col[2])
		dc.SetLineWidth(4)
		if len(st.dash) > 0 {
			dc.SetDash(st.dash...)
		}
		dc.DrawLine(40, st.y, w-60, st.y)
		_ = dc.Stroke()
		dc.SetDash()
		dc.DrawString(fmt.Sprintf("dash %v", st.dash), 50, st.y-12)
	}
	dc.ResetClip()
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(w-90, 20, 70, 24, 6)
	_ = dc.Fill()

	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, w-50, 30)
	if r < 100 {
		t.Fatalf("D154 badge missing rgba=%d,%d,%d", r, g, b)
	}
	// stroke samples should find non-dark on dash lines
	hits := 0
	for _, y := range []int{60, 110, 160, 210} {
		for x := 50; x < 350; x += 4 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr > 60 || gg > 60 || bb > 60 {
				hits++
				break
			}
		}
	}
	if hits < 3 {
		t.Fatalf("D154 dash strokes low hits=%d", hits)
	}
}

// D155: image quad warp under path clip + stroke text plate.
func TestP1_Comp_D155_ImageQuadWarpClipStrokeText(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.12, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.MoveTo(40, 40)
	dc.LineTo(360, 50)
	dc.LineTo(340, 220)
	dc.LineTo(50, 210)
	dc.ClosePath()
	dc.Clip()

	img := compMakeImage(t, 64, 64, 40, 120, 220)
	dc.DrawImageQuad(img, [4]render.Point{
		{X: 60, Y: 50}, {X: 300, Y: 70}, {X: 280, Y: 200}, {X: 80, Y: 190},
	})
	dc.SetRGB(0.95, 0.4, 0.3)
	dc.DrawCircle(200, 120, 30)
	_ = dc.Fill()
	dc.ResetClip()

	dc.SetRGB(0.95, 0.96, 0.98)
	dc.StrokeString("quad×clip", 40, 250)
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(250, 230, 110, 30, 6)
	_ = dc.Fill()

	compMinGPU(t, dc, 4)
	r, g, b, _ := p1Sample(dc, 200, 120)
	if r < 80 {
		t.Fatalf("D155 accent circle missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 280, 240)
	if r2 < 100 {
		t.Fatalf("D155 plate missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D156: radial + sweep gradient badges under layer opacity.
func TestP1_Comp_D156_RadialSweepGradientLayerStack(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	rad := render.NewRadialGradientBrush(120, 120, 0, 70).
		AddColorStop(0, render.RGB(1, 0.9, 0.3)).
		AddColorStop(1, render.RGB(0.9, 0.2, 0.3))
	dc.SetFillBrush(rad)
	dc.DrawCircle(120, 120, 70)
	_ = dc.Fill()

	sw := render.NewSweepGradientBrush(280, 130, 0).
		AddColorStop(0, render.RGB(0.2, 0.5, 0.95)).
		AddColorStop(0.5, render.RGB(0.3, 0.9, 0.5)).
		AddColorStop(1, render.RGB(0.2, 0.5, 0.95))
	dc.PushLayer(render.BlendNormal, 0.9)
	dc.SetFillBrush(sw)
	dc.DrawCircle(280, 130, 60)
	_ = dc.Fill()
	dc.PopLayer()

	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("radial × sweep", 130, 230)

	compMinGPU(t, dc, 4)
	r, g, b, _ := p1Sample(dc, 120, 120)
	p1NotNearWhite(t, "D156 radial", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 280, 130)
	p1NotNearWhite(t, "D156 sweep", r2, g2, b2)
}

// D157: window tiling manager mock — many panes × splitters × active.
func TestP1_Comp_D157_WindowTilingManagerMock(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	panes := []struct {
		x, y, pw, ph float64
		active       bool
		title        string
		col          [3]float64
	}{
		{4, 4, 250, 160, true, "editor", [3]float64{0.16, 0.18, 0.22}},
		{258, 4, 258, 100, false, "preview", [3]float64{0.15, 0.17, 0.2}},
		{258, 108, 258, 56, false, "term", [3]float64{0.12, 0.14, 0.16}},
		{4, 168, 160, 168, false, "tree", [3]float64{0.15, 0.16, 0.2}},
		{168, 168, 348, 168, false, "diff", [3]float64{0.14, 0.16, 0.19}},
	}
	for _, p := range panes {
		dc.SetRGB(p.col[0], p.col[1], p.col[2])
		dc.DrawRectangle(p.x, p.y, p.pw, p.ph)
		_ = dc.Fill()
		if p.active {
			dc.SetRGB(0.25, 0.55, 0.95)
			dc.SetLineWidth(2)
			dc.DrawRectangle(p.x+1, p.y+1, p.pw-2, p.ph-2)
			_ = dc.Stroke()
		}
		dc.SetRGB(0.85, 0.88, 0.92)
		dc.DrawString(p.title, p.x+10, p.y+18)
		// fake content
		dc.SetRGB(0.3+p.col[0], 0.5, 0.75)
		dc.DrawRoundedRectangle(p.x+12, p.y+36, math.Min(80, p.pw-24), 24, 4)
		_ = dc.Fill()
	}
	// splitters
	dc.SetRGB(0.9, 0.4, 0.3)
	dc.DrawRectangle(254, 4, 4, 160)
	_ = dc.Fill()

	compMinGPU(t, dc, 12)
	r, g, b, _ := p1Sample(dc, 256, 40)
	if r < 100 {
		t.Fatalf("D157 splitter missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 30, 50)
	p1NotNearWhite(t, "D157 editor content", r2, g2, b2)
}

// D158: circular progress rings dashboard tiles.
func TestP1_Comp_D158_CircularProgressDashboard(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	for i := 0; i < 4; i++ {
		cx := 70 + float64(i)*100
		cy := 120.0
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(cx-50, 40, 96, 180, 12)
		_ = dc.Fill()
		// track
		dc.SetRGB(0.9, 0.91, 0.93)
		dc.SetLineWidth(10)
		dc.DrawCircle(cx, cy, 34)
		_ = dc.Stroke()
		// progress arc approx with thick short segments
		prog := 0.35 + float64(i)*0.15
		dc.SetRGB(0.25+float64(i)*0.1, 0.55, 0.9-float64(i)*0.1)
		dc.SetLineWidth(10)
		steps := int(prog * 24)
		for s := 0; s < steps; s++ {
			a0 := -math.Pi/2 + float64(s)/24*2*math.Pi
			a1 := -math.Pi/2 + float64(s+1)/24*2*math.Pi
			dc.DrawLine(cx+34*math.Cos(a0), cy+34*math.Sin(a0), cx+34*math.Cos(a1), cy+34*math.Sin(a1))
			_ = dc.Stroke()
		}
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("%d%%", int(prog*100)), cx-14, cy+4)
		dc.SetRGB(0.9, 0.35, 0.3)
		dc.DrawRoundedRectangle(cx-30, 190, 60, 18, 4)
		_ = dc.Fill()
	}

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 70, 200)
	if r < 100 {
		t.Fatalf("D158 tile badge missing rgba=%d,%d,%d", r, g, b)
	}
	// progress arc starts at top of ring (cx=70, cy=120, r=34) → sample near top
	hits := 0
	for _, pt := range [][2]int{{70, 86}, {170, 86}, {270, 86}, {370, 86}, {100, 100}, {90, 95}} {
		rr, gg, bb, _ := p1Sample(dc, pt[0], pt[1])
		if rr < 240 || gg < 240 || bb < 240 {
			if rr > 40 || gg > 40 || bb > 40 {
				hits++
			}
		}
	}
	if hits < 2 {
		r2, g2, b2, _ := p1Sample(dc, 70, 86)
		t.Fatalf("D158 ring progress low hits=%d sample rgba=%d,%d,%d", hits, r2, g2, b2)
	}
}

// D159: multi-context parallel composition snapshot merge mock.
func TestP1_Comp_D159_MultiContextSnapshotMerge(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 260
	main := render.NewContext(w, h)
	defer main.Close()
	font := p1FindFont(t)
	_ = main.LoadFontFace(font, 12)

	main.ResetRenderPathStats()
	p1White(main, w, h)
	main.SetRGB(0.94, 0.95, 0.97)
	main.DrawRectangle(0, 0, w, h)
	_ = main.Fill()

	// side contexts
	for i := 0; i < 3; i++ {
		c := render.NewContext(100, 80)
		c.SetRGB(0.2+float64(i)*0.2, 0.4, 0.8-float64(i)*0.15)
		c.DrawRoundedRectangle(0, 0, 100, 80, 8)
		_ = c.Fill()
		c.SetRGB(1, 1, 1)
		_ = c.LoadFontFace(font, 11)
		c.DrawString(fmt.Sprintf("ctx%d", i), 20, 45)
		if err := c.FlushGPU(); err != nil {
			c.Close()
			t.Fatalf("side flush: %v", err)
		}
		img := c.Image()
		// blit via ImageBuf
		buf, err := render.NewImageBuf(100, 80, render.FormatRGBA8)
		if err != nil {
			c.Close()
			t.Fatalf("imgbuf: %v", err)
		}
		for y := 0; y < 80; y++ {
			for x := 0; x < 100; x++ {
				rr, gg, bb, aa := img.At(x, y).RGBA()
				_ = buf.SetRGBA(x, y, uint8(rr>>8), uint8(gg>>8), uint8(bb>>8), uint8(aa>>8))
			}
		}
		main.DrawImage(buf, 30+float64(i)*120, 40)
		c.Close()
	}
	main.SetRGB(0.9, 0.3, 0.35)
	main.DrawRoundedRectangle(140, 160, 120, 36, 8)
	_ = main.Fill()
	main.SetRGB(1, 1, 1)
	main.DrawString("merged", 170, 182)

	compMinGPU(t, main, 4)
	r, g, b, _ := p1Sample(main, 60, 70)
	p1NotNearWhite(t, "D159 ctx0", r, g, b)
	r2, g2, b2, _ := p1Sample(main, 180, 175)
	if r2 < 100 {
		t.Fatalf("D159 merge badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D160: kitchen-sink v5 — ultimate multi-axis stress (clip/layer/blend/text/image/mesh/filter).
func TestP1_Comp_D160_KitchenSinkV5UltimateStress(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 600, 420
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRoundRect(6, 6, w-12, h-12, 16)
	// header
	dc.SetRGB(0.1, 0.12, 0.16)
	dc.DrawRectangle(0, 0, w, 44)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("KitchenSink v5 — ultimate composition stress", 16, 28)

	// nav
	dc.SetRGB(0.14, 0.16, 0.2)
	dc.DrawRectangle(0, 44, 110, h-44)
	_ = dc.Fill()
	for i := 0; i < 9; i++ {
		if i == 3 {
			dc.SetRGB(0.25, 0.5, 0.9)
		} else {
			dc.SetRGB(0.18, 0.2, 0.25)
		}
		dc.DrawRoundedRectangle(8, 56+float64(i)*34, 94, 28, 6)
		_ = dc.Fill()
	}

	// content lattice with blends
	dc.ClipRect(120, 54, 340, 280)
	for row := 0; row < 7; row++ {
		for col := 0; col < 6; col++ {
			x := 130 + float64(col)*52
			y := 60 + float64(row)*38
			dc.SetBlendMode(render.BlendNormal)
			dc.SetRGB(0.25+float64(col)*0.05, 0.35+float64(row)*0.04, 0.78)
			dc.DrawRoundedRectangle(x, y, 46, 32, 5)
			_ = dc.Fill()
			if (row+col)%4 == 0 {
				dc.SetBlendMode(render.BlendMultiply)
				dc.SetRGBA(1, 0.6, 0.3, 1)
				dc.DrawCircle(x+23, y+16, 10)
				_ = dc.Fill()
				dc.SetBlendMode(render.BlendNormal)
			}
		}
	}
	// mesh overlay
	dc.DrawMesh(render.Mesh{
		Positions: []render.Point{{X: 150, Y: 80}, {X: 230, Y: 70}, {X: 220, Y: 140}, {X: 140, Y: 130}},
		Colors: []render.RGBA{
			{R: 1, G: 0.3, B: 0.3, A: 0.8},
			{R: 0.3, G: 1, B: 0.3, A: 0.8},
			{R: 0.3, G: 0.4, B: 1, A: 0.8},
			{R: 1, G: 1, B: 0.3, A: 0.8},
		},
		Indices: []uint16{0, 1, 2, 0, 2, 3},
	})
	dc.ResetClip()

	// image strip
	img := compMakeImage(t, 32, 32, 200, 80, 40)
	for i := 0; i < 5; i++ {
		dc.DrawImage(img, 130+float64(i)*40, 350)
	}

	// inspector
	dc.PushLayer(render.BlendNormal, 0.96)
	dc.SetRGB(0.98, 0.98, 1)
	dc.DrawRoundedRectangle(470, 60, 110, 200, 10)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(482, 80, 70, 24, 6)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("inspect", 488, 130)
	dc.PopLayer()

	// toast + filter whisper
	dc.SetRGB(0.25, 0.75, 0.45)
	dc.DrawRoundedRectangle(w-160, h-48, 140, 30, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("v5 compose ok", w-140, h-28)
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.15})
	dc.ResetClip()

	compMinGPU(t, dc, 40)
	r, g, b, _ := p1Sample(dc, 150, 80)
	p1NotNearWhite(t, "D160 content", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 500, 90)
	if r2 < 100 {
		t.Fatalf("D160 inspector accent missing rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, w-90, h-35)
	if g3 < 80 {
		t.Fatalf("D160 toast missing rgba=%d,%d,%d", r3, g3, b3)
	}
}
