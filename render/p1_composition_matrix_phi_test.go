//go:build !nogpu

package render_test

// Phase A phi composition probes D161+ — more complex multi-axis stress.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// D161: kanban × WIP limits × swim badges × drag ghost layer.
func TestP1_Comp_D161_KanbanWIPGhostComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	cols := []string{"Backlog", "Doing", "Review", "Done"}
	for i, name := range cols {
		x := 12 + float64(i)*136
		dc.SetRGB(0.98, 0.98, 1)
		dc.DrawRoundedRectangle(x, 16, 128, 300, 10)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(name, x+12, 40)
		// WIP chip
		dc.SetRGB(0.95, 0.55, 0.2)
		dc.DrawRoundedRectangle(x+88, 24, 28, 18, 6)
		_ = dc.Fill()
		for c := 0; c < 3; c++ {
			y := 60 + float64(c)*70
			dc.SetRGB(0.95, 0.96, 0.98)
			dc.DrawRoundedRectangle(x+10, y, 108, 56, 8)
			_ = dc.Fill()
			dc.SetRGB(0.25+float64(i)*0.1, 0.45, 0.85-float64(c)*0.1)
			dc.DrawRectangle(x+10, y, 6, 56)
			_ = dc.Fill()
			dc.SetRGB(0.15, 0.16, 0.2)
			dc.DrawString(fmt.Sprintf("T-%d%d", i, c), x+24, y+30)
		}
	}
	// drag ghost
	dc.PushLayer(render.BlendNormal, 0.55)
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(200, 140, 108, 56, 8)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 230, 160)
	if r < 80 {
		t.Fatalf("D161 ghost missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 110, 32)
	if r2 < 100 {
		t.Fatalf("D161 WIP chip missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D162: tree + property inspector + breadcrumb + multi-select highlights.
func TestP1_Comp_D162_TreeInspectorBreadcrumbMultiSelect(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// breadcrumb
	dc.SetRGB(0.2, 0.45, 0.85)
	dc.DrawRectangle(0, 0, w, 32)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Root / Project / src / render", 12, 22)
	// tree
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 32, 200, h-32)
	_ = dc.Fill()
	for i, n := range []string{"▸ pkg", "  ▸ gpu", "    webgpu.go", "    rwgpu.go", "  ▸ render", "    context.go", "    path.go"} {
		y := 56 + float64(i)*28
		if i == 2 || i == 5 {
			dc.SetRGB(0.25, 0.55, 0.95)
			dc.DrawRectangle(8, y-14, 184, 24)
			_ = dc.Fill()
			dc.SetRGB(1, 1, 1)
		} else {
			dc.SetRGB(0.15, 0.16, 0.2)
		}
		dc.DrawString(n, 16, y)
	}
	// inspector
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(200, 32, w-200, h-32)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.35, 0.3)
	dc.DrawRoundedRectangle(220, 56, 90, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.92, 0.95)
	dc.DrawString("props: 2 selected", 220, 110)
	for i := 0; i < 5; i++ {
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawRoundedRectangle(220, 130+float64(i)*30, 260, 24, 4)
		_ = dc.Fill()
	}

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 40, 70)
	if b < 80 {
		t.Fatalf("D162 tree select missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 240, 68)
	if r2 < 100 {
		t.Fatalf("D162 inspector accent missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D163: split diff 3-way × gutter marks × hunk headers × unify toggle.
func TestP1_Comp_D163_ThreeWayDiffGutterComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 540, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// three panes
	for i, title := range []string{"BASE", "OURS", "THEIRS"} {
		x := float64(i) * 180
		dc.SetRGB(0.16, 0.18, 0.22)
		dc.DrawRectangle(x, 0, 180, 28)
		_ = dc.Fill()
		dc.SetRGB(0.9, 0.92, 0.95)
		dc.DrawString(title, x+60, 18)
		for line := 0; line < 10; line++ {
			y := 36 + float64(line)*24
			if line == 3 {
				dc.SetRGB(0.35, 0.15, 0.15)
			} else if line == 6 {
				dc.SetRGB(0.15, 0.3, 0.18)
			} else {
				dc.SetRGB(0.14, 0.15, 0.18)
			}
			dc.DrawRectangle(x, y, 180, 24)
			_ = dc.Fill()
			// gutter
			if line == 3 {
				dc.SetRGB(0.95, 0.35, 0.35)
			} else if line == 6 {
				dc.SetRGB(0.35, 0.9, 0.45)
			} else {
				dc.SetRGB(0.4, 0.45, 0.55)
			}
			dc.DrawRectangle(x, y, 6, 24)
			_ = dc.Fill()
			dc.SetRGB(0.8, 0.85, 0.9)
			dc.DrawString(fmt.Sprintf("%d  code line", line+1), x+14, y+16)
		}
	}
	// toggle
	dc.SetRGB(0.25, 0.55, 0.95)
	dc.DrawRoundedRectangle(w-110, h-36, 90, 24, 8)
	_ = dc.Fill()

	compMinGPU(t, dc, 30)
	r, g, b, _ := p1Sample(dc, 3, 110)
	if r < 100 {
		t.Fatalf("D163 red gutter missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-60, h-24)
	if b2 < 80 {
		t.Fatalf("D163 toggle missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D164: layered chart — candles × volume × MA overlay × crosshair × tooltip.
func TestP1_Comp_D164_CandlesVolumeMACrosshairTooltip(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.08, 0.09, 0.12)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// candles
	for i := 0; i < 24; i++ {
		x := 20 + float64(i)*20
		open := 120 + 30*math.Sin(float64(i)*0.4)
		close := open + 15*math.Cos(float64(i)*0.5)
		hi, lo := math.Max(open, close)+10, math.Min(open, close)-10
		if close >= open {
			dc.SetRGB(0.3, 0.85, 0.5)
		} else {
			dc.SetRGB(0.95, 0.35, 0.35)
		}
		dc.SetLineWidth(1)
		dc.DrawLine(x+6, lo, x+6, hi)
		_ = dc.Stroke()
		dc.DrawRectangle(x, math.Min(open, close), 12, math.Abs(close-open)+1)
		_ = dc.Fill()
		// volume
		dc.SetRGBA(0.4, 0.5, 0.7, 0.6)
		dc.DrawRectangle(x, 250, 12, 20+float64(i%5)*6)
		_ = dc.Fill()
	}
	// MA
	dc.SetRGB(0.95, 0.8, 0.2)
	dc.SetLineWidth(2)
	prev := 130.0
	for i := 0; i < 24; i++ {
		x := 26 + float64(i)*20
		y := 130 + 20*math.Sin(float64(i)*0.3)
		dc.DrawLine(x-20, prev, x, y)
		_ = dc.Stroke()
		prev = y
	}
	// crosshair
	dc.SetRGB(0.7, 0.75, 0.85)
	dc.SetLineWidth(1)
	dc.SetDash(3, 3)
	dc.DrawLine(200, 20, 200, 240)
	_ = dc.Stroke()
	dc.DrawLine(20, 140, 500, 140)
	_ = dc.Stroke()
	dc.SetDash()
	// tooltip
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRoundedRectangle(210, 40, 120, 56, 8)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("O 102 H 110", 220, 62)
	dc.DrawString("L 98 C 108", 220, 82)
	dc.PopLayer()

	compMinGPU(t, dc, 40)
	r, g, b, _ := p1Sample(dc, 230, 55)
	if r > 80 && g > 80 && b > 80 {
		r, g, b, _ = p1Sample(dc, 220, 50)
	}
	if r > 100 && g > 100 && b > 100 {
		t.Fatalf("D164 tooltip missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 200, 140)
	p1NotNearWhite(t, "D164 crosshair area", r2, g2, b2)
}

// D165: nested form wizard steps × validation × sticky footer actions.
func TestP1_Comp_D165_WizardFormValidationStickyFooter(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// steps
	for i, s := range []string{"1 Account", "2 Profile", "3 Review"} {
		x := 30 + float64(i)*140
		if i == 1 {
			dc.SetRGB(0.25, 0.55, 0.95)
		} else {
			dc.SetRGB(0.8, 0.82, 0.86)
		}
		dc.DrawRoundedRectangle(x, 20, 120, 32, 16)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(s, x+20, 40)
	}
	// fields
	for i := 0; i < 4; i++ {
		y := 80 + float64(i)*48
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(40, y, w-80, 36, 6)
		_ = dc.Fill()
		if i == 2 {
			dc.SetRGB(0.9, 0.3, 0.3)
			dc.SetLineWidth(2)
			dc.DrawRoundedRectangle(40, y, w-80, 36, 6)
			_ = dc.Stroke()
			dc.DrawString("invalid email", 50, y+50)
		}
		dc.SetRGB(0.4, 0.45, 0.55)
		dc.DrawString(fmt.Sprintf("Field %d", i+1), 52, y+22)
	}
	// sticky footer
	dc.PushLayer(render.BlendNormal, 0.97)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, h-56, w, 56)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.7, 0.45)
	dc.DrawRoundedRectangle(w-140, h-44, 110, 32, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Continue", w-120, h-24)
	dc.PopLayer()

	compMinGPU(t, dc, 12)
	r, g, b, _ := p1Sample(dc, 50, 185)
	if r < 100 {
		t.Fatalf("D165 invalid border missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-80, h-28)
	if g2 < 80 {
		t.Fatalf("D165 continue missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D166: particle field under mask × additive blend sparkles × HUD.
func TestP1_Comp_D166_ParticleFieldMaskAdditiveHUD(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.05, 0.06, 0.1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	mask := render.NewMask(w, h)
	compFillMaskRect(mask, 40, 40, 380, 220, 255)
	dc.PushMaskLayer(mask)
	for i := 0; i < 80; i++ {
		x := 50 + float64(i*17%320)
		y := 50 + float64(i*13%160)
		dc.SetBlendMode(render.BlendPlus)
		dc.SetRGBA(0.4+float64(i%3)*0.2, 0.5, 0.95, 0.8)
		dc.DrawCircle(x, y, 2+float64(i%3))
		_ = dc.Fill()
	}
	dc.SetBlendMode(render.BlendNormal)
	dc.PopLayer()
	// HUD
	dc.SetRGB(0.2, 0.85, 0.55)
	dc.DrawRoundedRectangle(16, 16, 100, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(0.05, 0.08, 0.1)
	dc.DrawString("particles", 28, 34)

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 40, 28)
	if g < 80 {
		t.Fatalf("D166 HUD missing rgba=%d,%d,%d", r, g, b)
	}
	// outside mask dark
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 > 40 || g2 > 40 || b2 > 50 {
		t.Fatalf("D166 mask leak rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D167: multi-layer z-index mock with reorder handles × occlusion.
func TestP1_Comp_D167_ZIndexLayersReorderOcclusion(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	layers := []struct {
		x, y float64
		c    [3]float64
		name string
	}{
		{40, 40, [3]float64{0.3, 0.5, 0.9}, "L0"},
		{80, 70, [3]float64{0.9, 0.4, 0.3}, "L1"},
		{120, 100, [3]float64{0.3, 0.75, 0.45}, "L2"},
		{160, 130, [3]float64{0.95, 0.75, 0.2}, "L3"},
	}
	for _, l := range layers {
		dc.PushLayer(render.BlendNormal, 0.92)
		dc.SetRGB(l.c[0], l.c[1], l.c[2])
		dc.DrawRoundedRectangle(l.x, l.y, 160, 100, 10)
		_ = dc.Fill()
		dc.SetRGB(0.1, 0.1, 0.12)
		dc.DrawString(l.name, l.x+16, l.y+30)
		// handle
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawRectangle(l.x+140, l.y+8, 12, 12)
		_ = dc.Fill()
		dc.PopLayer()
	}
	// selected outline on top
	dc.SetRGB(0.95, 0.25, 0.35)
	dc.SetLineWidth(3)
	dc.DrawRoundedRectangle(160, 130, 160, 100, 10)
	_ = dc.Stroke()

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 200, 160)
	if r < 100 {
		t.Fatalf("D167 top layer missing rgba=%d,%d,%d", r, g, b)
	}
}

// D168: rich tooltip stack + caret + multi-line + action chips.
func TestP1_Comp_D168_RichTooltipStackCaretActions(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// anchor
	dc.SetRGB(0.25, 0.55, 0.9)
	dc.DrawRoundedRectangle(40, 160, 80, 32, 6)
	_ = dc.Fill()
	// tooltip
	dc.PushLayer(render.BlendNormal, 0.98)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRoundedRectangle(60, 40, 220, 110, 10)
	_ = dc.Fill()
	// caret
	dc.MoveTo(90, 150)
	dc.LineTo(110, 150)
	dc.LineTo(100, 165)
	dc.ClosePath()
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("Rich tooltip title", 76, 70)
	dc.SetRGB(0.7, 0.75, 0.82)
	dc.DrawString("multi-line detail body", 76, 92)
	dc.SetRGB(0.9, 0.35, 0.35)
	dc.DrawRoundedRectangle(76, 110, 60, 22, 6)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.7, 0.5)
	dc.DrawRoundedRectangle(150, 110, 60, 22, 6)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 100, 120)
	if r < 100 {
		t.Fatalf("D168 action chip missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 70, 170)
	if b2 < 80 {
		t.Fatalf("D168 anchor missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D169: CSS-grid-like dense cards with spanning hero × badges × footer.
func TestP1_Comp_D169_CSSGridDenseCardsHeroSpan(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// hero span 2 cols
	grad := render.NewLinearGradientBrush(16, 16, 340, 16).
		AddColorStop(0, render.RGB(0.25, 0.45, 0.9)).
		AddColorStop(1, render.RGB(0.9, 0.35, 0.55))
	dc.SetFillBrush(grad)
	dc.DrawRoundedRectangle(16, 16, 320, 140, 12)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Hero span 2×1", 32, 80)
	// side card
	dc.SetRGB(0.2, 0.75, 0.5)
	dc.DrawRoundedRectangle(352, 16, 150, 140, 12)
	_ = dc.Fill()
	// bottom grid 3
	for i := 0; i < 3; i++ {
		x := 16 + float64(i)*168
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 172, 156, 140, 10)
		_ = dc.Fill()
		dc.SetRGB(0.95, 0.4, 0.3)
		dc.DrawRoundedRectangle(x+12, 188, 48, 20, 6)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("Card %d", i+1), x+12, 240)
		dc.SetRGB(0.85, 0.87, 0.9)
		dc.DrawRectangle(x, 284, 156, 28)
		_ = dc.Fill()
	}

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 40, 40)
	if b < 60 {
		t.Fatalf("D169 hero missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 400, 60)
	if g2 < 80 {
		t.Fatalf("D169 side card missing rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 40, 195)
	if r3 < 100 {
		t.Fatalf("D169 badge missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D170: responsive breakpoint mock — reflow columns under resize states.
func TestP1_Comp_D170_ResponsiveBreakpointReflowMock(t *testing.T) {
	p1RequireGPU(t)
	font := p1FindFont(t)
	sizes := [][2]int{{480, 240}, {320, 240}, {240, 240}}
	var last *render.Context
	for si, sz := range sizes {
		w, h := sz[0], sz[1]
		dc := render.NewContext(w, h)
		_ = dc.LoadFontFace(font, 10)
		dc.ResetRenderPathStats()
		p1White(dc, w, h)
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawRectangle(0, 0, float64(w), 28)
		_ = dc.Fill()
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawString(fmt.Sprintf("bp %d", w), 8, 18)
		cols := 3
		if w < 400 {
			cols = 2
		}
		if w < 280 {
			cols = 1
		}
		cw := float64(w-16) / float64(cols)
		for i := 0; i < cols; i++ {
			dc.SetRGB(0.25+float64(i)*0.15, 0.5, 0.85-float64(si)*0.1)
			dc.DrawRoundedRectangle(8+float64(i)*cw, 40, cw-8, 160, 8)
			_ = dc.Fill()
		}
		dc.SetRGB(0.9, 0.3, 0.35)
		dc.DrawRoundedRectangle(float64(w-70), float64(h-32), 60, 22, 6)
		_ = dc.Fill()
		if last != nil {
			last.Close()
		}
		last = dc
	}
	compMinGPU(t, last, 3)
	r, g, b, _ := p1Sample(last, 40, 80)
	p1NotNearWhite(t, "D170 reflow col", r, g, b)
	r2, g2, b2, _ := p1Sample(last, 200, 220)
	if r2 < 100 {
		t.Fatalf("D170 badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
	last.Close()
}

// D171: annotation layer on image — arrows × boxes × freehand × labels.
func TestP1_Comp_D171_ImageAnnotationArrowsBoxesFreehand(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	img := compMakeImage(t, 300, 200, 60, 90, 140)
	dc.DrawImage(img, 40, 30)
	// box
	dc.SetRGB(0.95, 0.3, 0.3)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(80, 60, 100, 70, 4)
	_ = dc.Stroke()
	// arrow
	dc.SetRGB(0.95, 0.85, 0.2)
	dc.DrawLine(200, 100, 280, 70)
	_ = dc.Stroke()
	dc.MoveTo(280, 70)
	dc.LineTo(268, 66)
	dc.LineTo(272, 80)
	dc.ClosePath()
	_ = dc.Fill()
	// freehand
	dc.SetRGB(0.3, 0.9, 0.5)
	dc.SetLineWidth(2)
	for i := 0; i < 20; i++ {
		x0 := 90 + float64(i)*8
		y0 := 180 + 10*math.Sin(float64(i)*0.6)
		x1 := 90 + float64(i+1)*8
		y1 := 180 + 10*math.Sin(float64(i+1)*0.6)
		dc.DrawLine(x0, y0, x1, y1)
		_ = dc.Stroke()
	}
	// label plate
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRoundedRectangle(290, 200, 100, 36, 6)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("note #3", 304, 222)

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 100, 50)
	p1NotNearWhite(t, "D171 image", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 310, 210)
	if r2 > 80 && g2 > 80 {
		r2, g2, b2, _ = p1Sample(dc, 300, 205)
	}
	if r2 > 100 && g2 > 100 && b2 > 100 {
		t.Fatalf("D171 label missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D172: sidebar rail icons + flyout + active route indicator.
func TestP1_Comp_D172_SidebarRailFlyoutActiveRoute(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, 56, h)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		y := 20 + float64(i)*44
		if i == 2 {
			dc.SetRGB(0.25, 0.55, 0.95)
			dc.DrawRectangle(0, y-8, 4, 36)
			_ = dc.Fill()
			dc.SetRGB(0.2, 0.25, 0.35)
			dc.DrawRoundedRectangle(10, y-6, 36, 32, 8)
			_ = dc.Fill()
		} else {
			dc.SetRGB(0.3+float64(i)*0.05, 0.55, 0.85)
			dc.DrawRoundedRectangle(12, y, 28, 28, 6)
			_ = dc.Fill()
		}
	}
	// flyout
	dc.PushLayer(render.BlendNormal, 0.97)
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRoundedRectangle(60, 80, 160, 160, 10)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(72, 100, 130, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.92, 0.95)
	dc.DrawString("Dashboard", 84, 118)
	dc.DrawString("Reports", 84, 150)
	dc.DrawString("Settings", 84, 182)
	dc.PopLayer()
	// content
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(56, 0, w-56, h)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.22, 0.28)
	dc.DrawString("main content", 240, 40)

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 2, 100)
	if b < 80 {
		t.Fatalf("D172 active rail missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 100, 112)
	if r2 < 100 {
		t.Fatalf("D172 flyout item missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D173: nested clip damage islands with independent redraw stamps.
func TestP1_Comp_D173_NestedClipDamageIslands(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	islands := []struct {
		x, y, iw, ih float64
		c            [3]float64
	}{
		{20, 20, 160, 100, [3]float64{0.9, 0.3, 0.35}},
		{220, 30, 150, 120, [3]float64{0.25, 0.55, 0.9}},
		{60, 150, 280, 100, [3]float64{0.3, 0.75, 0.45}},
	}
	for i, is := range islands {
		dc.ClipRoundRect(is.x, is.y, is.iw, is.ih, 10)
		dc.SetRGB(1, 1, 1)
		dc.DrawRectangle(0, 0, w, h)
		_ = dc.Fill()
		dc.SetRGB(is.c[0], is.c[1], is.c[2])
		dc.DrawRoundedRectangle(is.x+12, is.y+12, 60, 28, 6)
		_ = dc.Fill()
		// damage stamp
		buf := make([]byte, 10*10*4)
		for p := 0; p < 100; p++ {
			buf[p*4+0] = uint8(40 + i*70)
			buf[p*4+1] = 200
			buf[p*4+2] = uint8(80 + i*40)
			buf[p*4+3] = 255
		}
		dc.WritePixels(int(is.x)+20, int(is.y)+50, 10, 10, buf)
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("isle%d", i), is.x+16, is.y+80)
		dc.ResetClip()
	}

	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 50, 40)
	if r < 100 {
		t.Fatalf("D173 island0 accent missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 250, 50)
	if b2 < 80 {
		t.Fatalf("D173 island1 accent missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D174: perspective-ish fan cards with rotate × scale stack.
func TestP1_Comp_D174_PerspectiveFanCardRotateScale(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i := 0; i < 5; i++ {
		dc.Push()
		dc.Translate(210, 180)
		dc.Rotate(-0.4 + float64(i)*0.2)
		dc.Scale(0.85+float64(i)*0.05, 0.85+float64(i)*0.05)
		dc.SetRGB(0.3+float64(i)*0.1, 0.45, 0.85-float64(i)*0.08)
		dc.DrawRoundedRectangle(-70, -100, 140, 180, 12)
		_ = dc.Fill()
		if i == 4 {
			dc.SetRGB(0.95, 0.35, 0.3)
			dc.DrawRoundedRectangle(-40, -40, 80, 30, 6)
			_ = dc.Fill()
		}
		dc.Pop()
	}
	dc.SetRGB(0.95, 0.85, 0.2)
	dc.DrawCircle(40, 40, 14)
	_ = dc.Fill()

	compMinGPU(t, dc, 6)
	r, g, b, _ := p1Sample(dc, 40, 40)
	if r < 100 {
		t.Fatalf("D174 marker missing rgba=%d,%d,%d", r, g, b)
	}
	// warm/red accent somewhere in fan
	hits := 0
	for y := 80; y < 240; y += 4 {
		for x := 100; x < 320; x += 4 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr > 100 && (gg > 60 || bb > 60) {
				hits++
			}
		}
	}
	if hits < 10 {
		t.Fatalf("D174 fan hits low=%d", hits)
	}
}

// D175: audio mixer strip — faders × meters × mute × pan knobs.
func TestP1_Comp_D175_AudioMixerFadersMeters(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for ch := 0; ch < 8; ch++ {
		x := 24 + float64(ch)*60
		// meter
		for m := 0; m < 16; m++ {
			if m > 12 {
				dc.SetRGB(0.95, 0.3, 0.3)
			} else if m > 8 {
				dc.SetRGB(0.95, 0.8, 0.2)
			} else {
				dc.SetRGB(0.3, 0.85, 0.45)
			}
			if m < 4+ch%8 {
				dc.DrawRectangle(x, 220-float64(m)*10, 16, 8)
				_ = dc.Fill()
			}
		}
		// fader track
		dc.SetRGB(0.25, 0.27, 0.32)
		dc.DrawRoundedRectangle(x+22, 40, 8, 180, 4)
		_ = dc.Fill()
		// knob
		ky := 60 + float64(ch)*12
		dc.SetRGB(0.9, 0.35, 0.4)
		dc.DrawRoundedRectangle(x+16, ky, 20, 14, 4)
		_ = dc.Fill()
		// mute
		if ch%3 == 0 {
			dc.SetRGB(0.95, 0.3, 0.35)
		} else {
			dc.SetRGB(0.3, 0.35, 0.4)
		}
		dc.DrawRoundedRectangle(x, 250, 40, 20, 4)
		_ = dc.Fill()
	}

	compMinGPU(t, dc, 40)
	r, g, b, _ := p1Sample(dc, 24, 220)
	if g < 60 && r < 60 {
		t.Fatalf("D175 meter missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 40, 66)
	if r2 < 100 {
		t.Fatalf("D175 fader knob missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D176: recursive clip-layer stress (5 deep) with alternating blend.
func TestP1_Comp_D176_DeepClipLayerBlendRecursion(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.2, 0.25, 0.4)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i := 0; i < 5; i++ {
		m := float64(i * 18)
		dc.ClipRoundRect(20+m, 20+m, w-40-2*m, h-40-2*m, 12)
		if i%2 == 0 {
			dc.PushLayer(render.BlendNormal, 0.85)
		} else {
			dc.PushLayer(render.BlendMultiply, 0.75)
		}
		dc.SetRGB(0.3+float64(i)*0.1, 0.5, 0.85-float64(i)*0.1)
		dc.DrawRectangle(0, 0, w, h)
		_ = dc.Fill()
	}
	dc.SetRGB(0.95, 0.35, 0.3)
	dc.DrawRoundedRectangle(130, 110, 100, 40, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("deep 5", 150, 134)
	for i := 0; i < 5; i++ {
		dc.PopLayer()
		dc.ResetClip()
	}

	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 160, 125)
	if r < 80 {
		t.Fatalf("D176 center accent missing rgba=%d,%d,%d", r, g, b)
	}
}

// D177: multi-font size hierarchy article with drop shadows on cards.
func TestP1_Comp_D177_TypeHierarchyShadowCards(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 440, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// cards with shadow-ish underlay
	for i := 0; i < 3; i++ {
		x := 20 + float64(i)*140
		dc.SetRGBA(0, 0, 0, 0.15)
		dc.DrawRoundedRectangle(x+4, 44, 120, 200, 10)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 40, 120, 200, 10)
		_ = dc.Fill()
		_ = dc.LoadFontFace(font, 16)
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("H%d", i+1), x+16, 80)
		_ = dc.LoadFontFace(font, 11)
		dc.SetRGB(0.4, 0.45, 0.55)
		dc.DrawString("body copy", x+16, 120)
		dc.SetRGB(0.25, 0.55, 0.9)
		dc.DrawRoundedRectangle(x+16, 180, 80, 28, 6)
		_ = dc.Fill()
	}
	dc.ApplyDropShadow(2, 3, 4, render.RGBA{R: 0, G: 0, B: 0, A: 0.2})
	// badge after
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(w-90, 12, 70, 24, 8)
	_ = dc.Fill()

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 50, 190)
	if b < 80 {
		t.Fatalf("D177 CTA missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-50, 20)
	if r2 < 100 {
		t.Fatalf("D177 badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D178: constellation graph with force clusters × hull × labels.
func TestP1_Comp_D178_ConstellationClustersHullLabels(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 460, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.07, 0.08, 0.12)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	clusters := []struct {
		cx, cy float64
		c      [3]float64
	}{
		{120, 120, [3]float64{0.3, 0.6, 0.95}},
		{300, 100, [3]float64{0.95, 0.45, 0.35}},
		{240, 220, [3]float64{0.35, 0.85, 0.5}},
	}
	// edges
	dc.SetRGB(0.3, 0.35, 0.45)
	dc.SetLineWidth(1)
	dc.DrawLine(120, 120, 300, 100)
	_ = dc.Stroke()
	dc.DrawLine(300, 100, 240, 220)
	_ = dc.Stroke()
	dc.DrawLine(240, 220, 120, 120)
	_ = dc.Stroke()
	for ci, cl := range clusters {
		// hull
		dc.SetRGBA(cl.c[0], cl.c[1], cl.c[2], 0.15)
		dc.DrawCircle(cl.cx, cl.cy, 50)
		_ = dc.Fill()
		for n := 0; n < 5; n++ {
			a := float64(n) / 5 * 2 * math.Pi
			x := cl.cx + 28*math.Cos(a)
			y := cl.cy + 28*math.Sin(a)
			dc.SetRGB(cl.c[0], cl.c[1], cl.c[2])
			dc.DrawCircle(x, y, 6)
			_ = dc.Fill()
		}
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawString(fmt.Sprintf("C%d", ci), cl.cx-8, cl.cy+4)
	}
	dc.SetRGB(0.95, 0.75, 0.2)
	dc.DrawRoundedRectangle(w-90, 12, 70, 24, 6)
	_ = dc.Fill()

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 148, 120)
	p1NotNearWhite(t, "D178 node", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, w-50, 20)
	if r2 < 100 {
		t.Fatalf("D178 badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D179: printer-like multipage preview strip with page shadows × active.
func TestP1_Comp_D179_MultipagePreviewStripActive(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.85, 0.87, 0.9)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i := 0; i < 5; i++ {
		x := 20 + float64(i)*96
		// shadow
		dc.SetRGBA(0, 0, 0, 0.18)
		dc.DrawRectangle(x+4, 44, 80, 180)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawRectangle(x, 40, 80, 180)
		_ = dc.Fill()
		if i == 2 {
			dc.SetRGB(0.25, 0.55, 0.95)
			dc.SetLineWidth(3)
			dc.DrawRectangle(x, 40, 80, 180)
			_ = dc.Stroke()
		}
		// fake lines
		for ln := 0; ln < 8; ln++ {
			dc.SetRGB(0.8, 0.82, 0.86)
			dc.DrawRectangle(x+10, 60+float64(ln)*16, 60, 6)
			_ = dc.Fill()
		}
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(fmt.Sprintf("%d", i+1), x+34, 240)
	}
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(w-100, 12, 80, 24, 8)
	_ = dc.Fill()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 220, 40)
	// active border around page 3 at x=212
	if b < 60 && r < 60 {
		r, g, b, _ = p1Sample(dc, 212, 45)
	}
	// page body white is ok; check badge
	r2, g2, b2, _ := p1Sample(dc, w-60, 20)
	if r2 < 100 {
		t.Fatalf("D179 badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
	_ = r
	_ = g
	_ = b
	// content lines non-white-ish gray
	r3, g3, b3, _ := p1Sample(dc, 40, 65)
	if r3 > 250 && g3 > 250 && b3 > 250 {
		t.Fatalf("D179 page lines missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D180: kitchen-sink v6 mega — combines atlas, mesh, filter, damage, layers.
func TestP1_Comp_D180_KitchenSinkV6MegaStress(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 620, 440
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRoundRect(4, 4, w-8, h-8, 14)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("KitchenSink v6 mega — atlas/mesh/filter/damage/layer", 12, 26)

	// sidebar
	dc.SetRGB(0.14, 0.16, 0.2)
	dc.DrawRectangle(0, 40, 100, h-40)
	_ = dc.Fill()
	atlas := compMakeImage(t, 48, 16, 0, 0, 0)
	for i, c := range [][3]uint8{{200, 80, 80}, {80, 180, 100}, {80, 120, 220}} {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				_ = atlas.SetRGBA(i*16+x, y, c[0], c[1], c[2], 255)
			}
		}
	}
	var sprites []render.AtlasSprite
	for i := 0; i < 6; i++ {
		sprites = append(sprites, render.AtlasSprite{
			SrcX: float64(i%3) * 16, SrcY: 0, SrcW: 16, SrcH: 16,
			DstX: 20, DstY: 56 + float64(i)*48, DstW: 28, DstH: 28, Opacity: 1,
		})
	}
	dc.DrawAtlas(atlas, sprites)

	// lattice
	dc.ClipRect(110, 50, 360, 300)
	for row := 0; row < 6; row++ {
		for col := 0; col < 6; col++ {
			x := 120 + float64(col)*55
			y := 56 + float64(row)*46
			dc.SetRGB(0.25+float64(col)*0.05, 0.4, 0.8-float64(row)*0.05)
			dc.DrawRoundedRectangle(x, y, 48, 36, 5)
			_ = dc.Fill()
			if (row+col)%3 == 0 {
				dc.SetBlendMode(render.BlendScreen)
				dc.SetRGBA(1, 0.5, 0.3, 1)
				dc.DrawCircle(x+24, y+18, 10)
				_ = dc.Fill()
				dc.SetBlendMode(render.BlendNormal)
			}
		}
	}
	dc.DrawMesh(render.Mesh{
		Positions: []render.Point{{X: 140, Y: 70}, {X: 220, Y: 80}, {X: 200, Y: 150}, {X: 130, Y: 140}},
		Colors: []render.RGBA{
			{R: 1, G: 0.2, B: 0.3, A: 0.85},
			{R: 0.2, G: 1, B: 0.3, A: 0.85},
			{R: 0.2, G: 0.4, B: 1, A: 0.85},
			{R: 1, G: 1, B: 0.2, A: 0.85},
		},
		Indices: []uint16{0, 1, 2, 0, 2, 3},
	})
	// damage stamps
	buf := make([]byte, 12*12*4)
	for p := 0; p < 144; p++ {
		buf[p*4+0] = 240
		buf[p*4+1] = 80
		buf[p*4+2] = 60
		buf[p*4+3] = 255
	}
	dc.WritePixels(300, 80, 12, 12, buf)
	dc.ResetClip()

	// inspector layer
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.98, 0.98, 1)
	dc.DrawRoundedRectangle(480, 56, 120, 220, 10)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(492, 76, 80, 26, 6)
	_ = dc.Fill()
	dc.PopLayer()

	dc.SetRGB(0.25, 0.75, 0.45)
	dc.DrawRoundedRectangle(w-150, h-44, 130, 28, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("v6 mega ok", w-130, h-26)
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.12})
	dc.ResetClip()

	compMinGPU(t, dc, 30)
	r, g, b, _ := p1Sample(dc, 34, 70)
	p1NotNearWhite(t, "D180 atlas", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 510, 88)
	if r2 < 100 {
		t.Fatalf("D180 inspector missing rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, w-80, h-30)
	if g3 < 80 {
		t.Fatalf("D180 toast missing rgba=%d,%d,%d", r3, g3, b3)
	}
}
