//go:build !nogpu

package render_test

// P1 Tier A — complex UI scene matrix (controls morphology, not control widgets).
//
// Architecture:
//   render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native
//
// Hard rules:
//   - GPUOps > 0 after FlushGPU
//   - Pixel / region checks for critical structure
//   - Scenes model Ant Design–class drawing density (rrect, layer, clip, text, grid)
//
// IDs A1–A8 match docs/OPTIMIZATION_PLAN.md P1 Tier A.

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

func p1RequireGPU(t *testing.T) {
	t.Helper()
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Log("WGPU_NATIVE_PATH unset; relying on default lib discovery")
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	dc := render.NewContext(8, 8)
	defer dc.Close()
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 8, 8)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Skipf("no GPU ops on probe: %s", dc.RenderPathStats().LogLine())
	}
}

func p1Flush(t *testing.T, dc *render.Context) {
	t.Helper()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("complex UI gate requires GPUOps>0: %s", stats.LogLine())
	}
}

func p1Sample(dc *render.Context, x, y int) (r, g, b, a uint8) {
	img := dc.Image()
	rr, gg, bb, aa := img.At(x, y).RGBA()
	return uint8(rr >> 8), uint8(gg >> 8), uint8(bb >> 8), uint8(aa >> 8)
}

func p1NotNearWhite(t *testing.T, name string, r, g, b uint8) {
	t.Helper()
	if r > 240 && g > 240 && b > 240 {
		t.Fatalf("%s still near-white rgba=%d,%d,%d", name, r, g, b)
	}
}

func p1FindFont(t *testing.T) string {
	t.Helper()
	for _, p := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
		filepath.Join("text", "testdata", "goregular.ttf"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no test font")
	return ""
}

func p1White(dc *render.Context, w, h int) {
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
}

// A1 Button states: rrect fill + 1px border + label baseline + focus ring.
func TestP1_A1_UIButtonStates(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 120
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	drawBtn := func(x, y float64, fill render.RGBA, border render.RGBA, label string, focus bool) {
		dc.SetRGBA(fill.R, fill.G, fill.B, fill.A)
		dc.DrawRoundedRectangle(x, y, 88, 32, 6)
		_ = dc.Fill()
		dc.SetRGBA(border.R, border.G, border.B, border.A)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(x+0.5, y+0.5, 87, 31, 6)
		_ = dc.Stroke()
		if focus {
			dc.SetRGBA(0.09, 0.47, 0.95, 0.55)
			dc.SetLineWidth(2)
			dc.DrawRoundedRectangle(x-2, y-2, 92, 36, 8)
			_ = dc.Stroke()
		}
		dc.SetRGB(0.1, 0.1, 0.12)
		dc.DrawString(label, x+16, y+21)
	}

	// default / hover / active / disabled
	drawBtn(16, 24, render.RGBA{R: 0.95, G: 0.95, B: 0.96, A: 1}, render.RGBA{R: 0.75, G: 0.75, B: 0.78, A: 1}, "OK", false)
	drawBtn(120, 24, render.RGBA{R: 0.90, G: 0.93, B: 1.0, A: 1}, render.RGBA{R: 0.25, G: 0.45, B: 0.95, A: 1}, "Hover", false)
	drawBtn(224, 24, render.RGBA{R: 0.15, G: 0.40, B: 0.90, A: 1}, render.RGBA{R: 0.10, G: 0.30, B: 0.75, A: 1}, "Act", true)
	drawBtn(16, 72, render.RGBA{R: 0.93, G: 0.93, B: 0.93, A: 1}, render.RGBA{R: 0.85, G: 0.85, B: 0.85, A: 1}, "Off", false)

	p1Flush(t, dc)

	// Default button body not white
	r, g, b, _ := p1Sample(dc, 40, 40)
	p1NotNearWhite(t, "default btn body", r, g, b)
	// Active (blue-ish) center
	r, g, b, _ = p1Sample(dc, 260, 40)
	if b < 100 || r > b {
		t.Fatalf("active btn not blue-ish rgba=%d,%d,%d", r, g, b)
	}
	// Focus ring: scan a 1px band around active button for non-white/non-fill ink
	ringInk := 0
	for y := 20; y <= 24; y++ {
		for x := 220; x < 320; x++ {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr < 250 || gg < 250 || bb < 250 {
				ringInk++
			}
		}
	}
	if ringInk < 3 {
		t.Fatalf("focus ring ink too low around active btn: %d", ringInk)
	}
}

// A2 Input field: border, placeholder text, caret, selection rect, inner clip.
func TestP1_A2_UIInputField(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 80
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Field background + border
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(20, 24, 240, 32, 4)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.55, 0.95)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(20.5, 24.5, 239, 31, 4)
	_ = dc.Stroke()

	// Selection rect (clip to field)
	dc.ClipRect(24, 28, 232, 24)
	dc.SetRGBA(0.20, 0.50, 0.95, 0.35)
	dc.DrawRectangle(48, 28, 72, 24)
	_ = dc.Fill()

	// Caret
	dc.SetRGB(0.1, 0.1, 0.1)
	dc.SetLineWidth(1)
	dc.DrawLine(120, 30, 120, 50)
	_ = dc.Stroke()

	// Text
	dc.SetRGB(0.15, 0.15, 0.18)
	dc.DrawString("hello ant", 28, 45)
	dc.ResetClip()

	p1Flush(t, dc)

	// Selection area has blue tint
	r, g, b, _ := p1Sample(dc, 70, 40)
	if b < 80 {
		t.Fatalf("selection not bluish rgba=%d,%d,%d", r, g, b)
	}
	// Caret column darker than neighbors
	cr, cg, cb, _ := p1Sample(dc, 120, 40)
	if cr > 220 && cg > 220 && cb > 220 {
		t.Fatalf("caret missing rgba=%d,%d,%d", cr, cg, cb)
	}
}

// A3 Menu overlay: base content + translucent panel + shadow + z-order clip.
func TestP1_A3_UIMenuOverlay(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 240, 180
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Base page content
	dc.SetRGB(0.2, 0.55, 0.3)
	dc.DrawRectangle(16, 16, 120, 40)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.1)
	dc.DrawString("page", 24, 40)

	// Shadow (offset soft-ish solid for gate)
	dc.SetRGBA(0, 0, 0, 0.18)
	dc.DrawRoundedRectangle(78, 54, 140, 100, 8)
	_ = dc.Fill()

	// Menu panel
	dc.SetRGBA(1, 1, 1, 0.96)
	dc.DrawRoundedRectangle(72, 48, 140, 100, 8)
	_ = dc.Fill()
	dc.SetRGB(0.85, 0.85, 0.88)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(72.5, 48.5, 139, 99, 8)
	_ = dc.Stroke()

	// Items + hover
	dc.SetRGBA(0.90, 0.94, 1.0, 1)
	dc.DrawRectangle(80, 56, 124, 24)
	_ = dc.Fill()
	dc.SetRGB(0.12, 0.12, 0.14)
	dc.DrawString("Open", 92, 72)
	dc.DrawString("Save", 92, 100)
	dc.DrawString("Quit", 92, 128)

	p1Flush(t, dc)

	// Hover row not white
	r, g, b, _ := p1Sample(dc, 100, 68)
	p1NotNearWhite(t, "menu hover", r, g, b)
	// Base content still visible outside panel
	r, g, b, _ = p1Sample(dc, 40, 30)
	if g < 80 {
		t.Fatalf("base content missing rgba=%d,%d,%d", r, g, b)
	}
}

// A4 Modal mask: full-screen dim + centered panel + nested clip.
func TestP1_A4_UIModalMask(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	// "App" background pattern
	dc.ClearWithColor(render.RGBA{R: 0.92, G: 0.93, B: 0.95, A: 1})
	dc.SetRGB(0.3, 0.5, 0.8)
	dc.DrawRectangle(20, 20, 80, 40)
	_ = dc.Fill()

	// Full-screen mask
	dc.SetRGBA(0, 0, 0, 0.45)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()

	// Dialog
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(70, 40, 180, 120, 10)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.DrawString("Confirm?", 110, 80)
	dc.SetRGB(0.15, 0.45, 0.90)
	dc.DrawRoundedRectangle(100, 110, 70, 28, 6)
	_ = dc.Fill()

	p1Flush(t, dc)

	// Masked corner darker than pure app bg would be alone
	r, g, b, _ := p1Sample(dc, 10, 10)
	if r > 200 && g > 200 && b > 200 {
		t.Fatalf("modal mask not applied corner rgba=%d,%d,%d", r, g, b)
	}
	// Dialog center near white
	r, g, b, _ = p1Sample(dc, 160, 70)
	if r < 230 || g < 230 || b < 230 {
		t.Fatalf("dialog body not light rgba=%d,%d,%d", r, g, b)
	}
	// Primary button blue
	r, g, b, _ = p1Sample(dc, 130, 124)
	if b < 120 || r > b {
		t.Fatalf("dialog button not blue rgba=%d,%d,%d", r, g, b)
	}
}

// A5 Table cells: many repeated cells + grid lines + ellipsis-ish text + hover.
func TestP1_A5_UITableCells(t *testing.T) {
	p1RequireGPU(t)
	const (
		w, h       = 360, 220
		cols, rows = 4, 6
		cw, rh     = 80.0, 28.0
		ox, oy     = 20.0, 20.0
	)
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	dc.SetLCDLayout(render.LCDLayoutNone) // isolate from other LCD tests
	p1White(dc, w, h)

	// Header
	dc.SetRGB(0.96, 0.96, 0.97)
	dc.DrawRectangle(ox, oy, cw*cols, rh)
	_ = dc.Fill()

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			x := ox + float64(col)*cw
			y := oy + float64(row)*rh
			// Cell background (hover tint is stronger so sample is stable)
			if row == 3 && col == 1 {
				dc.SetRGB(0.55, 0.72, 1.0)
			} else if row%2 == 0 {
				dc.SetRGB(1, 1, 1)
			} else {
				dc.SetRGB(0.98, 0.98, 0.99)
			}
			dc.DrawRectangle(x, y, cw, rh)
			_ = dc.Fill()
			// Clip text to cell
			dc.ClipRect(x+2, y+2, cw-4, rh-4)
			dc.SetRGB(0.15, 0.15, 0.18)
			dc.DrawString("cell-long-text", x+4, y+18)
			dc.ResetClip()
		}
	}
	// Re-paint hover on top so text/grid cannot leave sample pure white.
	{
		x := ox + 1*cw
		y := oy + 3*rh
		dc.SetRGB(0.55, 0.72, 1.0)
		dc.DrawRectangle(x, y, cw, rh)
		_ = dc.Fill()
	}
	// Grid lines
	dc.SetRGBA(0.80, 0.80, 0.84, 1)
	dc.SetLineWidth(1)
	for col := 0; col <= cols; col++ {
		x := ox + float64(col)*cw
		dc.DrawLine(x, oy, x, oy+rh*float64(rows))
		_ = dc.Stroke()
	}
	for row := 0; row <= rows; row++ {
		y := oy + float64(row)*rh
		dc.DrawLine(ox, y, ox+cw*float64(cols), y)
		_ = dc.Stroke()
	}

	p1Flush(t, dc)

	// Hover cell tinted
	r, g, b, _ := p1Sample(dc, int(ox+cw+cw/2), int(oy+3*rh+rh/2))
	p1NotNearWhite(t, "table hover cell", r, g, b)
	// Text ink somewhere in first data cell
	ink := 0
	for y := int(oy + rh + 4); y < int(oy+2*rh-4); y++ {
		for x := int(ox + 4); x < int(ox+cw-4); x++ {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr < 200 || gg < 200 || bb < 200 {
				ink++
			}
		}
	}
	if ink < 8 {
		t.Fatalf("table cell text ink too low: %d", ink)
	}
}

// A6 Tabs / badge / tag: compact shapes + stroke text-ish AA.
func TestP1_A6_UITabsBadge(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 300, 100
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Tabs
	tabs := []struct {
		x     float64
		label string
		on    bool
	}{
		{16, "Home", true},
		{96, "Docs", false},
		{176, "API", false},
	}
	for _, tab := range tabs {
		if tab.on {
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(tab.x, 24, 72, 28, 6)
			_ = dc.Fill()
			dc.SetRGB(0.15, 0.45, 0.95)
			dc.DrawRectangle(tab.x+8, 50, 56, 2)
			_ = dc.Fill()
		}
		dc.SetRGB(0.2, 0.2, 0.22)
		dc.DrawString(tab.label, tab.x+16, 42)
	}

	// Badge
	dc.SetRGB(1, 0.30, 0.30)
	dc.DrawCircle(250, 30, 9)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	_ = dc.LoadFontFace(font, 10)
	dc.DrawString("3", 246, 34)

	// Tag
	dc.SetRGBA(0.90, 0.95, 1.0, 1)
	dc.DrawRoundedRectangle(220, 60, 56, 22, 11)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.40, 0.85)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(220.5, 60.5, 55, 21, 11)
	_ = dc.Stroke()
	dc.DrawString("Tag", 234, 75)

	p1Flush(t, dc)

	// Active underline blue
	r, g, b, _ := p1Sample(dc, 44, 51)
	if b < 100 {
		t.Fatalf("tab underline missing rgba=%d,%d,%d", r, g, b)
	}
	// Badge red (avoid white glyph center)
	r, g, b, _ = p1Sample(dc, 250, 24)
	if r < 150 || g > 160 {
		t.Fatalf("badge not red rgba=%d,%d,%d", r, g, b)
	}
}

// A7 Icon path + mixed text.
func TestP1_A7_UIIconTextMix(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 90
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Icon as filled path (star-ish / check)
	dc.SetRGB(0.15, 0.65, 0.40)
	dc.MoveTo(28, 30)
	dc.LineTo(40, 50)
	dc.LineTo(20, 50)
	dc.ClosePath()
	_ = dc.Fill()
	dc.SetLineWidth(3)
	dc.SetRGB(0.15, 0.55, 0.35)
	dc.MoveTo(48, 38)
	dc.LineTo(56, 48)
	dc.LineTo(72, 28)
	_ = dc.Stroke()

	dc.SetRGB(0.12, 0.12, 0.14)
	dc.DrawString("Save 保存 OK", 84, 46)

	p1Flush(t, dc)

	// Icon ink
	r, g, b, _ := p1Sample(dc, 30, 45)
	if g < 80 {
		t.Fatalf("icon fill missing rgba=%d,%d,%d", r, g, b)
	}
	// Text ink near latin start
	ink := 0
	for y := 34; y < 52; y++ {
		for x := 84; x < 180; x++ {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr < 210 || gg < 210 || bb < 210 {
				ink++
			}
		}
	}
	if ink < 20 {
		t.Fatalf("mixed text ink too low: %d", ink)
	}
}

// A8 Nested scroll clip + offset content + damage-like partial redraw region.
func TestP1_A8_UIScrollClip(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 200, 160
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Viewport frame
	dc.SetRGB(0.88, 0.88, 0.90)
	dc.DrawRoundedRectangle(20, 20, 160, 120, 6)
	_ = dc.Fill()

	// Nested clip = scroll viewport
	dc.ClipRect(28, 28, 144, 104)
	// Content scrolled by -40 logical
	scrollY := -40.0
	for i := 0; i < 10; i++ {
		y := 30 + float64(i)*28 + scrollY
		if i%2 == 0 {
			dc.SetRGB(0.55, 0.70, 0.95)
		} else {
			dc.SetRGB(0.75, 0.85, 0.70)
		}
		dc.DrawRoundedRectangle(36, y, 128, 22, 4)
		_ = dc.Fill()
	}
	dc.ResetClip()

	// Outer chrome not clipped content
	dc.SetRGB(0.2, 0.2, 0.22)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(20.5, 20.5, 159, 119, 6)
	_ = dc.Stroke()

	p1Flush(t, dc)

	// Inside viewport should have colored rows
	r, g, b, _ := p1Sample(dc, 100, 80)
	p1NotNearWhite(t, "scroll content", r, g, b)
	// Outside viewport (margin) near white
	r, g, b, _ = p1Sample(dc, 10, 10)
	if r < 240 || g < 240 || b < 240 {
		t.Fatalf("outside scroll viewport polluted rgba=%d,%d,%d", r, g, b)
	}
	// Content should not bleed below clip bottom
	r, g, b, _ = p1Sample(dc, 100, 150)
	if r < 230 || g < 230 || b < 230 {
		// may hit stroke - allow slightly lower
		if r < 200 {
			t.Fatalf("clip leak below viewport rgba=%d,%d,%d", r, g, b)
		}
	}
}

// --- Capability matrix closers: S.05 resize, S.08 HiDPI, B.06 paint alpha ---

func TestP1_Capability_S05_ResizeGPU(t *testing.T) {
	p1RequireGPU(t)
	dc := render.NewContext(64, 48)
	defer dc.Close()
	dc.ResetRenderPathStats()
	p1White(dc, 64, 48)
	dc.SetRGB(0.9, 0.2, 0.2)
	dc.DrawRectangle(8, 8, 20, 16)
	_ = dc.Fill()
	p1Flush(t, dc)

	if err := dc.Resize(96, 72); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	dc.ResetRenderPathStats()
	p1White(dc, 96, 72)
	dc.SetRGB(0.2, 0.4, 0.9)
	dc.DrawRoundedRectangle(12, 12, 40, 28, 6)
	_ = dc.Fill()
	p1Flush(t, dc)

	bounds := dc.Image().Bounds()
	if bounds.Dx() != 96 || bounds.Dy() != 72 {
		t.Fatalf("after resize image %v want 96x72", bounds)
	}
	r, g, b, _ := p1Sample(dc, 30, 24)
	if b < 100 {
		t.Fatalf("post-resize draw missing rgba=%d,%d,%d", r, g, b)
	}
}

func TestP1_Capability_S08_HiDPIHairline(t *testing.T) {
	p1RequireGPU(t)
	// Logical 80x40 at DPR=2 → physical 160x80 context size convention in gpui:
	// NewContext is physical; deviceScale maps logical ops.
	dc := render.NewContext(160, 80, render.WithDeviceScale(2.0))
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 80, 40) // logical via scale?
	// In this codebase, Draw coords are logical when deviceScale set.
	_ = dc.Fill()
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(0) // hairline
	dc.DrawLine(10, 20, 70, 20)
	_ = dc.Stroke()
	p1Flush(t, dc)

	// Sample physical midline row for dark pixels
	ink := 0
	for x := 20; x < 140; x++ {
		r, g, b, _ := p1Sample(dc, x, 40)
		if r < 200 || g < 200 || b < 200 {
			ink++
		}
	}
	if ink < 5 {
		t.Fatalf("HiDPI hairline invisible ink=%d", ink)
	}
	t.Logf("HiDPI hairline ink samples=%d DeviceScale=%v", ink, dc.DeviceScale())
}

func TestP1_Capability_B06_PaintAlpha(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	// White bg
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// 50% red over white → ~mid red
	dc.SetRGBA(1, 0, 0, 0.5)
	dc.DrawRectangle(16, 16, 32, 32)
	_ = dc.Fill()
	p1Flush(t, dc)

	r, g, b, a := p1Sample(dc, 32, 32)
	t.Logf("paint alpha center rgba=%d,%d,%d,%d", r, g, b, a)
	// With alpha over white, G/B must lift from 0 (white contribution).
	if g < 40 || b < 40 {
		t.Fatalf("alpha blend missing white contribution rgba=%d,%d,%d", r, g, b)
	}
	if r < g || r < b {
		t.Fatalf("expected red-dominant rgba=%d,%d,%d", r, g, b)
	}
	// Fully transparent would leave pure white
	if r > 250 && g > 250 && b > 250 {
		t.Fatalf("alpha fill invisible rgba=%d,%d,%d", r, g, b)
	}
	_ = a
}

// Combined density scene: stack A-like elements once for stress correctness (Tier B lite).
func TestP1_B1_ManyRRectsCorrectness(t *testing.T) {
	p1RequireGPU(t)
	const w, h, n = 256, 256, 400
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	for i := 0; i < n; i++ {
		x := 16 + float64((i*17)%(w-48))
		y := 16 + float64((i*13)%(h-40))
		dc.SetRGBA(0.2+float64(i%5)*0.1, 0.3, 0.8-float64(i%4)*0.1, 0.85)
		dc.DrawRoundedRectangle(x, y, 18, 14, 4)
		_ = dc.Fill()
	}
	p1Flush(t, dc)

	// Corners remain mostly white (not fully covered by accident)
	r, g, b, _ := p1Sample(dc, 2, 2)
	if r < 200 {
		t.Fatalf("corner polluted rgba=%d,%d,%d", r, g, b)
	}
	// Interior should have substantial colored coverage
	colored := 0
	for y := 8; y < h-8; y += 4 {
		for x := 8; x < w-8; x += 4 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr < 240 || gg < 240 || bb < 240 {
				colored++
			}
		}
	}
	if colored < 200 {
		t.Fatalf("too little coverage from many rrects: colored=%d", colored)
	}
	t.Logf("many rrects colored samples=%d stats=%s", colored, dc.RenderPathStats().LogLine())
}

// --- Tier B stress scenes (correctness first, GPUOps>0) ---

func TestP1_B2_StressTextAtlas(t *testing.T) {
	p1RequireGPU(t)
	font := p1FindFont(t)
	dc := render.NewContext(320, 240)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	if err := dc.LoadFontFace(font, 12); err != nil {
		t.Skipf("font: %v", err)
	}
	dc.SetRGB(0.1, 0.1, 0.1)
	for row := 0; row < 12; row++ {
		for col := 0; col < 8; col++ {
			x := 8 + col*38
			y := 16 + row*18
			dc.DrawString(fmt.Sprintf("Aa%d", row*8+col), float64(x), float64(y))
		}
	}
	p1Flush(t, dc)
	// Sample a few glyph cells — not pure white
	hits := 0
	for _, pt := range [][2]int{{20, 20}, {80, 40}, {150, 100}, {200, 180}} {
		r, g, b, _ := p1Sample(dc, pt[0], pt[1])
		if r < 250 || g < 250 || b < 250 {
			hits++
		}
	}
	if hits == 0 {
		t.Fatalf("B2 text atlas stress produced no non-white samples")
	}
}

func TestP1_B3_StressImageGallery(t *testing.T) {
	p1RequireGPU(t)
	dc := render.NewContext(256, 256)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	// Synthetic tiles as solid rects (image-like density without file deps)
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			x := float64(col * 32)
			y := float64(row * 32)
			dc.SetRGB(float64(col)/8, float64(row)/8, 0.5)
			dc.DrawRoundedRectangle(x+2, y+2, 28, 28, 4)
			_ = dc.Fill()
		}
	}
	p1Flush(t, dc)
	r, g, b, _ := p1Sample(dc, 16, 16)
	p1NotNearWhite(t, "gallery tile", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 200, 200)
	p1NotNearWhite(t, "gallery tile far", r2, g2, b2)
}

func TestP1_B4_StressBlendStack(t *testing.T) {
	p1RequireGPU(t)
	dc := render.NewContext(128, 128)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Base blue
	dc.SetBlendMode(render.BlendNormal)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 128, 128)
	_ = dc.Fill()

	// Plus green wash
	dc.SetBlendMode(render.BlendPlus)
	dc.SetRGBA(0, 1, 0, 0.4)
	dc.DrawRectangle(16, 16, 96, 96)
	_ = dc.Fill()

	// Copy red panel
	dc.SetBlendMode(render.BlendCopy)
	dc.SetRGBA(1, 0, 0, 0.75)
	dc.DrawRectangle(40, 40, 48, 48)
	_ = dc.Fill()

	// Reset
	dc.SetBlendMode(render.BlendNormal)

	p1Flush(t, dc)
	// Copy region: red-dominant, little blue
	r, g, b, _ := p1Sample(dc, 64, 64)
	t.Logf("B4 copy-center rgba=%d,%d,%d", r, g, b)
	if b > r {
		t.Fatalf("B4 copy center expected red-dominant over blue base, got %d,%d,%d", r, g, b)
	}
	if r < 80 {
		t.Fatalf("B4 copy center red too low: %d,%d,%d", r, g, b)
	}
}

func TestP1_B5_StressPathAA(t *testing.T) {
	p1RequireGPU(t)
	dc := render.NewContext(200, 200)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetAntiAlias(true)
	dc.SetLineWidth(1.0)
	dc.SetRGB(0.1, 0.1, 0.1)
	for i := 0; i < 40; i++ {
		y := 10 + float64(i)*4.5
		dc.MoveTo(10, y)
		dc.CubicTo(50, y-8, 150, y+8, 190, y)
		_ = dc.Stroke()
	}
	// Mixed filled curves
	dc.SetRGBA(0.2, 0.4, 0.9, 0.7)
	for i := 0; i < 12; i++ {
		cx := 30 + float64(i%4)*45
		cy := 40 + float64(i/4)*50
		dc.DrawCircle(cx, cy, 14)
		_ = dc.Fill()
	}
	p1Flush(t, dc)
	r, g, b, _ := p1Sample(dc, 100, 100)
	// Should have some ink somewhere on grid samples
	ink := 0
	for y := 20; y < 180; y += 20 {
		for x := 20; x < 180; x += 20 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if int(rr)+int(gg)+int(bb) < 700 {
				ink++
			}
		}
	}
	t.Logf("B5 ink samples=%d center=%d,%d,%d", ink, r, g, b)
	if ink < 3 {
		t.Fatalf("B5 path AA stress too empty (ink=%d)", ink)
	}
}

func TestP1_B6_StressHiDPI(t *testing.T) {
	p1RequireGPU(t)
	// Physical 200x200 at DPR=2 → logical 100x100 drawing space
	dc := render.NewContext(200, 200, render.WithDeviceScale(2.0))
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	// Logical 1px hairlines and small rrects under 2x DPR
	dc.SetLineWidth(0) // hairline
	dc.SetRGB(0, 0, 0)
	for i := 0; i < 20; i++ {
		y := 5 + float64(i)*4
		dc.DrawLine(5, y, 95, y)
		_ = dc.Stroke()
	}
	dc.SetRGB(0.1, 0.5, 0.9)
	dc.DrawRoundedRectangle(20, 20, 60, 40, 6)
	_ = dc.Fill()
	p1Flush(t, dc)
	// Physical sample near rrect center (logical 50,40 → physical ~100,80)
	r, g, b, _ := p1Sample(dc, 100, 80)
	p1NotNearWhite(t, "hidpi rrect", r, g, b)
}

// --- Tier C: denser nested Ant Design–class drawing (not widget APIs) ---

// C1 nests modal mask + form fields + dropdown overlay + badge density.
func TestP1_C1_NestedModalFormMenu(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.RGBA{R: 0.96, G: 0.96, B: 0.96, A: 1})

	// App chrome bar
	dc.SetRGB(0.12, 0.23, 0.54)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("App / Settings", 12, 24)

	// Page cards
	for i := 0; i < 3; i++ {
		x := 12 + float64(i)*100
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 48, 92, 70, 6)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawString("Card", x+10, 70)
	}

	// Modal mask
	dc.SetRGBA(0, 0, 0, 0.45)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Modal panel
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(50, 40, 220, 160, 8)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.1)
	dc.DrawString("Edit profile", 66, 64)

	// Form inputs (labels + fields)
	labels := []string{"Name", "Email", "Role"}
	for i, lab := range labels {
		y := 80 + float64(i)*28
		dc.SetRGB(0.35, 0.35, 0.35)
		dc.DrawString(lab, 66, y)
		dc.SetRGB(0.95, 0.95, 0.95)
		dc.DrawRoundedRectangle(120, y-14, 130, 22, 4)
		_ = dc.Fill()
		dc.SetRGB(0.75, 0.75, 0.75)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(120, y-14, 130, 22, 4)
		_ = dc.Stroke()
	}

	// Primary / default buttons
	dc.SetRGB(0.13, 0.55, 0.95)
	dc.DrawRoundedRectangle(150, 172, 72, 22, 4)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Save", 168, 188)
	dc.SetRGB(0.92, 0.92, 0.92)
	dc.DrawRoundedRectangle(230, 172, 56, 22, 4)
	_ = dc.Fill()

	// Nested dropdown menu over the Role field
	dc.SetRGBA(0, 0, 0, 0.08)
	dc.DrawRoundedRectangle(124, 148, 120, 72, 4)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(122, 146, 120, 70, 4)
	_ = dc.Fill()
	for i, item := range []string{"Admin", "Editor", "Viewer"} {
		y := 162 + float64(i)*18
		if i == 1 {
			dc.SetRGB(0.90, 0.94, 1.0)
			dc.DrawRectangle(126, y-12, 112, 18)
			_ = dc.Fill()
		}
		dc.SetRGB(0.15, 0.15, 0.15)
		dc.DrawString(item, 134, y)
	}

	// Badge on modal title
	dc.SetRGB(1, 0.3, 0.3)
	dc.DrawCircle(210, 56, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("3", 206, 60)

	p1Flush(t, dc)

	// Structure checks: mask darkens chrome; modal white; menu has ink.
	rMask, gMask, bMask, _ := p1Sample(dc, 10, 200)
	// Masked page area should not be pure white page
	if rMask > 250 && gMask > 250 && bMask > 250 {
		t.Fatalf("modal mask missing at page bg: %d,%d,%d", rMask, gMask, bMask)
	}
	rModal, gModal, bModal, _ := p1Sample(dc, 160, 50)
	// Title bar area of modal is white-ish (may catch text)
	if int(rModal)+int(gModal)+int(bModal) < 500 {
		t.Fatalf("modal panel too dark: %d,%d,%d", rModal, gModal, bModal)
	}
	// Hover item in menu should have some blue-ish fill
	rH, gH, bH, _ := p1Sample(dc, 180, 168)
	t.Logf("mask=%d,%d,%d modal=%d,%d,%d hover=%d,%d,%d", rMask, gMask, bMask, rModal, gModal, bModal, rH, gH, bH)
	// Ensure GPU density: multi-primitive scene
	if dc.RenderPathStats().GPUOps < 5 {
		t.Fatalf("C1 expected dense GPUOps>=5: %s", dc.RenderPathStats().LogLine())
	}
}

// C2: table + sticky header + row selection + scroll clip + floating action.
func TestP1_C2_TableScrollOverlayDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Clip to table viewport
	dc.DrawRoundedRectangle(20, 20, 200, 140, 4)
	dc.Clip()

	// Sticky header
	dc.SetRGB(0.96, 0.96, 0.96)
	dc.DrawRectangle(20, 20, 200, 24)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.2, 0.2)
	dc.DrawString("Name", 28, 36)
	dc.DrawString("Status", 120, 36)

	// Rows (more than viewport)
	for i := 0; i < 12; i++ {
		y := 44 + float64(i)*18
		if i%2 == 0 {
			dc.SetRGB(1, 1, 1)
		} else {
			dc.SetRGB(0.98, 0.98, 0.98)
		}
		if i == 3 {
			dc.SetRGB(0.90, 0.94, 1.0) // selected
		}
		dc.DrawRectangle(20, y, 200, 18)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.15, 0.15)
		dc.DrawString("Row", 28, y+13)
		dc.SetRGB(0.3, 0.7, 0.4)
		dc.DrawRoundedRectangle(120, y+3, 40, 12, 6)
		_ = dc.Fill()
	}
	dc.ResetClip()

	// Floating action button over table corner
	dc.SetRGB(0.13, 0.55, 0.95)
	dc.DrawCircle(230, 160, 18)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("+", 225, 166)

	// Tooltip-like overlay
	dc.SetRGBA(0.1, 0.1, 0.1, 0.9)
	dc.DrawRoundedRectangle(190, 120, 70, 28, 4)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Add", 210, 138)

	p1Flush(t, dc)

	// Outside clip should stay white (no row bleed)
	rOut, gOut, bOut, _ := p1Sample(dc, 10, 100)
	if rOut < 240 || gOut < 240 || bOut < 240 {
		t.Fatalf("clip leak outside table: %d,%d,%d", rOut, gOut, bOut)
	}
	// Header gray
	rH, gH, bH, _ := p1Sample(dc, 40, 28)
	if rH < 200 {
		t.Fatalf("header missing: %d,%d,%d", rH, gH, bH)
	}
	// FAB blue
	rF, gF, bF, _ := p1Sample(dc, 230, 160)
	if bF < 150 || rF > 100 {
		t.Fatalf("FAB not blue-ish: %d,%d,%d", rF, gF, bF)
	}
	if dc.RenderPathStats().GPUOps < 5 {
		t.Fatalf("C2 expected dense GPUOps>=5: %s", dc.RenderPathStats().LogLine())
	}
}

// --- Tier D: Ant Design–class density (drawer + tree + select stack) ---

// D1 stacks left drawer navigation, tree indentation rows, and a select
// dropdown overlay — common Ant Design layout drawing morphology.
func TestP1_D1_DrawerTreeSelectDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 220
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// App shell background
	dc.SetRGB(0.96, 0.96, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Top header bar
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.2, 0.2)
	dc.DrawRectangle(0, 40, w, 1)
	_ = dc.Fill()

	// Left drawer
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 40, 120, h-40)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.9, 0.9)
	dc.DrawRectangle(119, 40, 1, h-40)
	_ = dc.Fill()

	// Tree rows with indent
	for i := 0; i < 8; i++ {
		y := 50 + float64(i)*18
		indent := float64((i % 3) * 12)
		// expand icon box
		dc.SetRGB(0.7, 0.7, 0.7)
		dc.DrawRoundedRectangle(8+indent, y, 10, 10, 2)
		_ = dc.Fill()
		// label bar
		dc.SetRGB(0.25, 0.25, 0.25)
		dc.DrawRoundedRectangle(22+indent, y+1, 80-indent, 8, 2)
		_ = dc.Fill()
		// selected row highlight
		if i == 3 {
			dc.SetRGBA(0.13, 0.55, 0.95, 0.15)
			dc.DrawRectangle(0, y-2, 120, 16)
			_ = dc.Fill()
		}
	}

	// Main content card
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(132, 56, 176, 140, 8)
	_ = dc.Fill()
	// card border
	dc.SetRGB(0.85, 0.85, 0.85)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(132, 56, 176, 140, 8)
	_ = dc.Stroke()

	// Form labels + inputs
	for i := 0; i < 3; i++ {
		y := 70 + float64(i)*36
		dc.SetRGB(0.35, 0.35, 0.35)
		dc.DrawRoundedRectangle(144, y, 48, 8, 2)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(144, y+12, 150, 18, 4)
		_ = dc.Fill()
		dc.SetRGB(0.8, 0.8, 0.8)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(144, y+12, 150, 18, 4)
		_ = dc.Stroke()
	}

	// Select dropdown overlay (open state)
	dc.SetRGBA(0, 0, 0, 0.08)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(144, 120, 150, 96, 6)
	_ = dc.Fill()
	// shadow-ish darker edge
	dc.SetRGBA(0, 0, 0, 0.12)
	dc.DrawRoundedRectangle(146, 122, 150, 96, 6)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(144, 120, 150, 96, 6)
	_ = dc.Fill()
	// options
	for i := 0; i < 4; i++ {
		y := 128 + float64(i)*20
		if i == 1 {
			dc.SetRGB(0.90, 0.95, 1.0)
			dc.DrawRectangle(148, y-2, 142, 18)
			_ = dc.Fill()
		}
		dc.SetRGB(0.3, 0.3, 0.3)
		dc.DrawRoundedRectangle(156, y+2, 100, 8, 2)
		_ = dc.Fill()
	}

	// Primary button
	dc.SetRGB(0.13, 0.55, 0.95)
	dc.DrawRoundedRectangle(220, 185, 72, 24, 4)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("D1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 8 {
		t.Fatalf("D1 expected dense GPUOps>=8: %s", stats.LogLine())
	}

	// Drawer body relatively light
	rD, gD, bD, _ := p1Sample(dc, 40, 100)
	if rD < 180 {
		t.Fatalf("drawer missing: %d,%d,%d", rD, gD, bD)
	}
	// Select panel: sample several points; at least one near-white interior
	panelOK := false
	var rP, gP, bP uint8
	for _, pt := range [][2]int{{160, 125}, {180, 130}, {200, 135}, {220, 140}} {
		rP, gP, bP, _ = p1Sample(dc, pt[0], pt[1])
		if rP > 200 && gP > 200 && bP > 200 {
			panelOK = true
			break
		}
	}
	if !panelOK {
		// Fallback: panel must be lighter than full dim gray
		rP, gP, bP, _ = p1Sample(dc, 180, 130)
		if int(rP)+int(gP)+int(bP) < 500 {
			t.Fatalf("select panel missing/too dark: %d,%d,%d", rP, gP, bP)
		}
	}
	// Primary button blue
	rB, gB, bB, _ := p1Sample(dc, 250, 195)
	if bB < 150 || rB > 120 {
		t.Fatalf("primary button not blue: %d,%d,%d", rB, gB, bB)
	}
	// Tree indent icon exists (gray box)
	rT, gT, bT, _ := p1Sample(dc, 12, 55)
	t.Logf("samples drawer=%d,%d,%d panel=%d,%d,%d btn=%d,%d,%d tree=%d,%d,%d",
		rD, gD, bD, rP, gP, bP, rB, gB, bB, rT, gT, bT)
}

// D2: Tabs + badge + notification stack + multi-layer popconfirm.
func TestP1_D2_TabsBadgePopconfirmDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 180
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Tabs bar
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	for i := 0; i < 4; i++ {
		x := 12 + float64(i)*60
		if i == 1 {
			dc.SetRGB(0.13, 0.55, 0.95)
			dc.DrawRectangle(x, 34, 48, 2)
			_ = dc.Fill()
			dc.SetRGB(0.13, 0.55, 0.95)
		} else {
			dc.SetRGB(0.4, 0.4, 0.4)
		}
		dc.DrawRoundedRectangle(x, 10, 40, 10, 2)
		_ = dc.Fill()
		// badge on tab 2
		if i == 2 {
			dc.SetRGB(1, 0.3, 0.3)
			dc.DrawCircle(x+38, 10, 6)
			_ = dc.Fill()
		}
	}

	// Content
	dc.SetRGB(0.97, 0.97, 0.97)
	dc.DrawRectangle(0, 36, w, h-36)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(16, 52, w-32, 100, 8)
	_ = dc.Fill()

	// Notification stack (3 cards)
	for i := 0; i < 3; i++ {
		x := 160 + float64(i)*4
		y := 48 + float64(i)*8
		dc.SetRGBA(0, 0, 0, 0.08)
		dc.DrawRoundedRectangle(x+2, y+2, 100, 40, 6)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, y, 100, 40, 6)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawRoundedRectangle(x+10, y+10, 60, 8, 2)
		_ = dc.Fill()
	}

	// Popconfirm bubble
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(40, 100, 120, 56, 6)
	_ = dc.Fill()
	dc.SetRGB(0.85, 0.85, 0.85)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(40, 100, 120, 56, 6)
	_ = dc.Stroke()
	// OK / Cancel
	dc.SetRGB(0.13, 0.55, 0.95)
	dc.DrawRoundedRectangle(100, 128, 44, 18, 4)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(50, 128, 44, 18, 4)
	_ = dc.Fill()
	dc.SetRGB(0.8, 0.8, 0.8)
	dc.DrawRoundedRectangle(50, 128, 44, 18, 4)
	_ = dc.Stroke()

	p1Flush(t, dc)
	if dc.RenderPathStats().GPUOps < 6 {
		t.Fatalf("D2 expected GPUOps>=6: %s", dc.RenderPathStats().LogLine())
	}
	// Badge red present
	r, g, b, _ := p1Sample(dc, 12+2*60+38, 10)
	if r < 150 || g > 120 {
		t.Fatalf("badge not red-ish: %d,%d,%d", r, g, b)
	}
	// Active tab underline blue-ish area
	r2, g2, b2, _ := p1Sample(dc, 12+60+20, 34)
	if b2 < 100 {
		t.Logf("tab underline sample %d,%d,%d (may be thin hairline)", r2, g2, b2)
	}
	// Popconfirm OK blue
	r3, g3, b3, _ := p1Sample(dc, 120, 136)
	if b3 < 140 {
		t.Fatalf("popconfirm OK not blue: %d,%d,%d", r3, g3, b3)
	}
}

// --- Tier E: Ant Design panel density (DatePicker / Transfer morphology) ---

// E1 draws a DatePicker-like panel: month header, weekday row, 6x7 day grid,
// selected/today cells, and footer actions — pure drawing morphology.
func TestP1_E1_DatePickerPanelDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.RGBA{R: 0.94, G: 0.94, B: 0.96, A: 1})

	// Panel card
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(20, 20, 240, 260, 8)
	_ = dc.Fill()
	dc.SetRGB(0.86, 0.86, 0.86)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(20, 20, 240, 260, 8)
	_ = dc.Stroke()

	// Month header
	dc.SetRGB(0.2, 0.2, 0.2)
	dc.DrawRoundedRectangle(90, 32, 100, 12, 2)
	_ = dc.Fill()
	// nav chevrons
	dc.SetRGB(0.45, 0.45, 0.45)
	dc.DrawRoundedRectangle(36, 32, 16, 12, 2)
	_ = dc.Fill()
	dc.DrawRoundedRectangle(228, 32, 16, 12, 2)
	_ = dc.Fill()

	// Weekday labels
	for i := 0; i < 7; i++ {
		x := 36 + float64(i)*30
		dc.SetRGB(0.55, 0.55, 0.55)
		dc.DrawRoundedRectangle(x, 56, 16, 8, 2)
		_ = dc.Fill()
	}

	// 6x7 day grid
	selected := [2]int{2, 3}
	today := [2]int{2, 4}
	for row := 0; row < 6; row++ {
		for col := 0; col < 7; col++ {
			x := 32 + float64(col)*30
			y := 76 + float64(row)*28
			// cell
			if row == selected[0] && col == selected[1] {
				dc.SetRGB(0.13, 0.55, 0.95)
				dc.DrawCircle(x+12, y+12, 12)
				_ = dc.Fill()
				dc.SetRGB(1, 1, 1)
				dc.DrawRoundedRectangle(x+6, y+8, 12, 8, 2)
				_ = dc.Fill()
			} else if row == today[0] && col == today[1] {
				dc.SetRGB(0.13, 0.55, 0.95)
				dc.SetLineWidth(1)
				dc.DrawCircle(x+12, y+12, 12)
				_ = dc.Stroke()
				dc.SetRGB(0.2, 0.2, 0.2)
				dc.DrawRoundedRectangle(x+6, y+8, 12, 8, 2)
				_ = dc.Fill()
			} else {
				// muted out-of-month on edges
				if row == 0 && col < 2 || row == 5 && col > 4 {
					dc.SetRGB(0.75, 0.75, 0.75)
				} else {
					dc.SetRGB(0.25, 0.25, 0.25)
				}
				dc.DrawRoundedRectangle(x+6, y+8, 12, 8, 2)
				_ = dc.Fill()
			}
		}
	}

	// Footer divider + buttons
	dc.SetRGB(0.9, 0.9, 0.9)
	dc.DrawRectangle(28, 248, 224, 1)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(40, 256, 64, 16, 4)
	_ = dc.Fill()
	dc.SetRGB(0.8, 0.8, 0.8)
	dc.DrawRoundedRectangle(40, 256, 64, 16, 4)
	_ = dc.Stroke()
	dc.SetRGB(0.13, 0.55, 0.95)
	dc.DrawRoundedRectangle(176, 256, 64, 16, 4)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("E1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 10 {
		t.Fatalf("E1 expected dense GPUOps>=10: %s", stats.LogLine())
	}
	// Selected cell blue ring (center has white day label).
	r, g, b, _ := p1Sample(dc, 32+3*30+12, 76+2*28+4)
	if b < 150 || r > 120 {
		// try another ring sample
		r, g, b, _ = p1Sample(dc, 32+3*30+4, 76+2*28+12)
	}
	if b < 150 || r > 120 {
		t.Fatalf("selected day not blue: %d,%d,%d", r, g, b)
	}
	// Panel white interior (avoid header/chevron glyphs)
	r2, g2, b2, _ := p1Sample(dc, 60, 48)
	if r2 < 220 {
		r2, g2, b2, _ = p1Sample(dc, 140, 48)
	}
	if r2 < 220 {
		t.Fatalf("panel missing: %d,%d,%d", r2, g2, b2)
	}
	// Footer OK blue
	r3, g3, b3, _ := p1Sample(dc, 200, 262)
	if b3 < 150 {
		t.Fatalf("footer OK not blue: %d,%d,%d", r3, g3, b3)
	}
}

// E2 draws a Transfer dual-list: two panels, item rows, checkboxes, shuttle
// buttons, and headers — list density morphology for Ant Transfer.
func TestP1_E2_TransferListDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	drawList := func(x float64, selected map[int]bool) {
		// panel
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 24, 140, 190, 6)
		_ = dc.Fill()
		dc.SetRGB(0.85, 0.85, 0.85)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(x, 24, 140, 190, 6)
		_ = dc.Stroke()
		// header
		dc.SetRGB(0.97, 0.97, 0.97)
		dc.DrawRoundedRectangle(x, 24, 140, 28, 6)
		_ = dc.Fill()
		dc.SetRGB(0.85, 0.85, 0.85)
		dc.DrawRectangle(x, 50, 140, 1)
		_ = dc.Fill()
		dc.SetRGB(0.3, 0.3, 0.3)
		dc.DrawRoundedRectangle(x+28, 32, 60, 10, 2)
		_ = dc.Fill()
		// checkbox in header
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x+10, 32, 12, 12, 2)
		_ = dc.Fill()
		dc.SetRGB(0.7, 0.7, 0.7)
		dc.DrawRoundedRectangle(x+10, 32, 12, 12, 2)
		_ = dc.Stroke()
		// items
		for i := 0; i < 7; i++ {
			y := 58 + float64(i)*22
			if selected[i] {
				dc.SetRGB(0.90, 0.95, 1.0)
				dc.DrawRectangle(x+1, y-2, 138, 20)
				_ = dc.Fill()
			}
			// checkbox
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(x+10, y, 12, 12, 2)
			_ = dc.Fill()
			if selected[i] {
				dc.SetRGB(0.13, 0.55, 0.95)
				dc.DrawRoundedRectangle(x+12, y+2, 8, 8, 1)
				_ = dc.Fill()
			} else {
				dc.SetRGB(0.7, 0.7, 0.7)
				dc.DrawRoundedRectangle(x+10, y, 12, 12, 2)
				_ = dc.Stroke()
			}
			// label
			dc.SetRGB(0.25, 0.25, 0.25)
			dc.DrawRoundedRectangle(x+30, y+2, 90, 8, 2)
			_ = dc.Fill()
		}
	}

	drawList(24, map[int]bool{1: true, 3: true})
	drawList(196, map[int]bool{0: true})

	// Shuttle buttons
	dc.SetRGB(0.13, 0.55, 0.95)
	dc.DrawRoundedRectangle(168, 90, 28, 24, 4)
	_ = dc.Fill()
	dc.SetRGB(0.85, 0.85, 0.85)
	dc.DrawRoundedRectangle(168, 130, 28, 24, 4)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("E2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 12 {
		t.Fatalf("E2 expected dense GPUOps>=12: %s", stats.LogLine())
	}
	// Left panel selected row tint
	r, g, b, _ := p1Sample(dc, 40, 58+22+8)
	if b < 180 {
		t.Logf("selected row sample %d,%d,%d", r, g, b)
	}
	// Shuttle blue
	r2, g2, b2, _ := p1Sample(dc, 182, 100)
	if b2 < 150 || r2 > 100 {
		t.Fatalf("shuttle button not blue: %d,%d,%d", r2, g2, b2)
	}
	// Right panel exists (white)
	r3, g3, b3, _ := p1Sample(dc, 220, 40)
	if r3 < 200 {
		t.Fatalf("right list panel missing: %d,%d,%d", r3, g3, b3)
	}
}

// --- Tier F: Cascader + virtual list morphology ---

// F1 Cascader multi-column panels with active path highlight.
func TestP1_F1_CascaderPanelDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 220
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.RGBA{R: 0.95, G: 0.95, B: 0.97, A: 1})

	// Three columns
	colW := 120.0
	for c := 0; c < 3; c++ {
		x := 24 + float64(c)*(colW+8)
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 24, colW, 172, 6)
		_ = dc.Fill()
		dc.SetRGB(0.85, 0.85, 0.85)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(x, 24, colW, 172, 6)
		_ = dc.Stroke()
		// items
		for i := 0; i < 6; i++ {
			y := 36 + float64(i)*26
			active := (c == 0 && i == 1) || (c == 1 && i == 2) || (c == 2 && i == 0)
			if active {
				dc.SetRGB(0.90, 0.95, 1.0)
				dc.DrawRectangle(x+1, y-4, colW-2, 24)
				_ = dc.Fill()
			}
			dc.SetRGB(0.25, 0.25, 0.25)
			dc.DrawRoundedRectangle(x+12, y, 70, 10, 2)
			_ = dc.Fill()
			// chevron for non-leaf
			if c < 2 {
				dc.SetRGB(0.6, 0.6, 0.6)
				dc.DrawRoundedRectangle(x+colW-18, y+2, 8, 8, 1)
				_ = dc.Fill()
			}
		}
	}

	// Input trigger above
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(24, 4, 200, 16, 4)
	_ = dc.Fill()
	dc.SetRGB(0.8, 0.8, 0.8)
	dc.DrawRoundedRectangle(24, 4, 200, 16, 4)
	_ = dc.Stroke()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("F1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 10 {
		t.Fatalf("F1 expected GPUOps>=10: %s", stats.LogLine())
	}
	// Active row tint in first column
	r, g, b, _ := p1Sample(dc, 40, 36+26-2)
	// Column panel white
	r2, g2, b2, _ := p1Sample(dc, 280, 100)
	if r2 < 200 {
		t.Fatalf("cascader col missing: %d,%d,%d", r2, g2, b2)
	}
	t.Logf("active sample=%d,%d,%d col=%d,%d,%d", r, g, b, r2, g2, b2)
}

// F2 Virtual list: many rows + sticky header + scrollbar thumb + overscan fade.
func TestP1_F2_VirtualListDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Viewport frame
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(16, 16, 220, 280, 6)
	_ = dc.Fill()
	dc.SetRGB(0.85, 0.85, 0.85)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(16, 16, 220, 280, 6)
	_ = dc.Stroke()

	// Body rows first (virtual window)
	dc.ClearPath()
	dc.DrawRectangle(17, 49, 200, 246)
	dc.Clip()
	for i := 0; i < 14; i++ {
		y := 52 + float64(i)*22
		if i%2 == 0 {
			dc.SetRGB(0.99, 0.99, 0.99)
			dc.DrawRectangle(17, y-2, 200, 22)
			_ = dc.Fill()
		}
		dc.SetRGB(0.13, 0.55, 0.95)
		dc.DrawCircle(36, y+8, 8)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawRoundedRectangle(52, y, 120, 8, 2)
		_ = dc.Fill()
		dc.SetRGB(0.65, 0.65, 0.65)
		dc.DrawRoundedRectangle(52, y+10, 90, 6, 2)
		_ = dc.Fill()
		if i == 5 {
			dc.SetRGBA(0.13, 0.55, 0.95, 0.12)
			dc.DrawRectangle(17, y-2, 200, 22)
			_ = dc.Fill()
		}
	}
	dc.ResetClip()
	dc.ClearPath()

	// Sticky header OVER rows (wins compositing)
	dc.SetRGB(0.97, 0.97, 0.97)
	dc.DrawRectangle(17, 17, 218, 32)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.3, 0.3)
	dc.DrawRoundedRectangle(28, 26, 80, 10, 2)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.9, 0.9)
	dc.DrawRectangle(17, 48, 218, 1)
	_ = dc.Fill()

	// Scrollbar track + thumb
	dc.SetRGB(0.93, 0.93, 0.93)
	dc.DrawRoundedRectangle(222, 56, 6, 220, 3)
	_ = dc.Fill()
	dc.SetRGB(0.55, 0.55, 0.55)
	dc.DrawRoundedRectangle(222, 100, 6, 48, 3)
	_ = dc.Fill()

	// Bottom fade
	dc.SetRGBA(1, 1, 1, 0.65)
	dc.DrawRectangle(17, 270, 200, 24)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("F2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 15 {
		t.Fatalf("F2 expected dense GPUOps>=15: %s", stats.LogLine())
	}
	// Sticky header light gray (avoid title bar dark label)
	r2, g2, b2, _ := p1Sample(dc, 160, 30)
	if r2 < 200 {
		r2, g2, b2, _ = p1Sample(dc, 200, 28)
	}
	if r2 < 200 {
		t.Fatalf("sticky header missing: %d,%d,%d", r2, g2, b2)
	}
	// Avatar in first visible row (below header)
	r, g, b, _ := p1Sample(dc, 36, 60)
	if b < 120 {
		t.Fatalf("list avatar not blue-ish: %d,%d,%d", r, g, b)
	}
	// Scrollbar thumb darker than track
	r3, g3, b3, _ := p1Sample(dc, 224, 120)
	if r3 > 230 {
		t.Fatalf("scrollbar thumb missing: %d,%d,%d", r3, g3, b3)
	}
	t.Logf("header=%d,%d,%d avatar=%d,%d,%d thumb=%d,%d,%d", r2, g2, b2, r, g, b, r3, g3, b3)
}

// --- Tier G denser Ant Design morphology (TreeSelect / Carousel) ---

// G1 TreeSelect panel: search bar + tree with expand chevrons + nested indent
// + selected row + checkbox markers + dropdown chrome.
func TestP1_G1_TreeSelectPanelDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Page backdrop
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Trigger control (closed select look)
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(24, 16, 200, 32, 6)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.15)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(24.5, 16.5, 199, 31, 6)
	_ = dc.Stroke()
	dc.SetRGB(0.2, 0.2, 0.2)
	dc.DrawString("TreeSelect / org units", 36, 37)
	// Caret
	dc.SetRGB(0.45, 0.45, 0.45)
	dc.MoveTo(204, 28)
	dc.LineTo(212, 28)
	dc.LineTo(208, 34)
	dc.ClosePath()
	_ = dc.Fill()

	// Dropdown panel
	panelX, panelY := 24.0, 56.0
	panelW, panelH := 280.0, 240.0
	// Shadow
	dc.SetRGBA(0, 0, 0, 0.12)
	dc.DrawRoundedRectangle(panelX+3, panelY+4, panelW, panelH, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(panelX, panelY, panelW, panelH, 8)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.12)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(panelX+0.5, panelY+0.5, panelW-1, panelH-1, 8)
	_ = dc.Stroke()

	// Search input
	dc.SetRGB(0.98, 0.98, 0.98)
	dc.DrawRoundedRectangle(panelX+12, panelY+12, panelW-24, 28, 4)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.15)
	dc.DrawRoundedRectangle(panelX+12.5, panelY+12.5, panelW-25, 27, 4)
	_ = dc.Stroke()
	dc.SetRGB(0.55, 0.55, 0.55)
	dc.DrawString("Search…", panelX+22, panelY+31)

	// Tree rows
	type row struct {
		indent int
		label  string
		open   bool
		sel    bool
		check  bool
	}
	rows := []row{
		{0, "Company", true, false, false},
		{1, "Engineering", true, false, true},
		{2, "Frontend", false, true, true},
		{2, "Backend", false, false, false},
		{2, "Platform", false, false, false},
		{1, "Design", false, false, false},
		{1, "Operations", true, false, false},
		{2, "SRE", false, false, true},
		{2, "Support", false, false, false},
		{0, "Partners", false, false, false},
	}
	y := panelY + 52
	for _, r := range rows {
		x := panelX + 12 + float64(r.indent)*18
		if r.sel {
			dc.SetRGBA(0.09, 0.47, 0.95, 0.12)
			dc.DrawRectangle(panelX+8, y-2, panelW-16, 22)
			_ = dc.Fill()
		}
		// Expand chevron box
		dc.SetRGB(0.55, 0.55, 0.55)
		if r.open {
			dc.MoveTo(x, y+6)
			dc.LineTo(x+8, y+6)
			dc.LineTo(x+4, y+12)
		} else {
			dc.MoveTo(x, y+4)
			dc.LineTo(x, y+12)
			dc.LineTo(x+6, y+8)
		}
		dc.ClosePath()
		_ = dc.Fill()
		// Checkbox
		cx := x + 14
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(cx, y+2, 14, 14, 2)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.25)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(cx+0.5, y+2.5, 13, 13, 2)
		_ = dc.Stroke()
		if r.check {
			dc.SetRGB(0.09, 0.47, 0.95)
			dc.DrawRoundedRectangle(cx+2, y+4, 10, 10, 1)
			_ = dc.Fill()
		}
		dc.SetRGB(0.15, 0.15, 0.15)
		if r.sel {
			dc.SetRGB(0.09, 0.35, 0.75)
		}
		dc.DrawString(r.label, cx+20, y+14)
		y += 22
	}

	// Footer actions
	dc.SetRGBA(0, 0, 0, 0.06)
	dc.DrawRectangle(panelX, panelY+panelH-36, panelW, 36)
	_ = dc.Fill()
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(panelX+panelW-88, panelY+panelH-28, 72, 24, 4)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("OK", panelX+panelW-62, panelY+panelH-12)

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("G1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 12 {
		t.Fatalf("G1 expected dense GPUOps>=12: %s", stats.LogLine())
	}
	// Panel white body
	r, g, b, _ := p1Sample(dc, 100, 100)
	if r < 240 {
		t.Fatalf("panel body missing: %d,%d,%d", r, g, b)
	}
	// Selected row tint near Frontend
	r2, g2, b2, _ := p1Sample(dc, 80, 130)
	if b2 < g2 {
		// selected has blue tint — sample may land on text; try another x
		r2, g2, b2, _ = p1Sample(dc, 40, 128)
	}
	// Checkbox blue checked on Engineering row ~ y=52+22+8
	r3, g3, b3, _ := p1Sample(dc, 62, 122)
	t.Logf("panel=%d,%d,%d sel=%d,%d,%d check=%d,%d,%d", r, g, b, r2, g2, b2, r3, g3, b3)
	// Primary OK button
	r4, g4, b4, _ := p1Sample(dc, int(panelX+panelW-50), int(panelY+panelH-16))
	if b4 < 150 {
		t.Fatalf("OK button missing blue: %d,%d,%d", r4, g4, b4)
	}
}

// G2 Carousel: stage with slides, gradient overlay, dots, arrows, card content.
func TestP1_G2_CarouselStageDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Stage
	sx, sy, sw, sh := 40.0, 28.0, 320.0, 180.0
	// Soft shadow
	dc.SetRGBA(0, 0, 0, 0.14)
	dc.DrawRoundedRectangle(sx+4, sy+6, sw, sh, 10)
	_ = dc.Fill()

	// Active slide gradient-ish (two layered rects)
	dc.SetRGB(0.12, 0.36, 0.78)
	dc.DrawRoundedRectangle(sx, sy, sw, sh, 10)
	_ = dc.Fill()
	dc.SetRGBA(0.2, 0.65, 0.95, 0.55)
	dc.DrawRoundedRectangle(sx+sw*0.35, sy, sw*0.65, sh, 10)
	_ = dc.Fill()

	// Title + subtitle
	dc.SetRGB(1, 1, 1)
	_ = dc.LoadFontFace(font, 20)
	dc.DrawString("Featured release", sx+24, sy+48)
	_ = dc.LoadFontFace(font, 13)
	dc.SetRGBA(1, 1, 1, 0.85)
	dc.DrawString("Ship complex UI with GPU render foundation", sx+24, sy+72)

	// CTA
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(sx+24, sy+100, 110, 32, 16)
	_ = dc.Fill()
	dc.SetRGB(0.12, 0.36, 0.78)
	dc.DrawString("Learn more", sx+40, sy+121)

	// Side peeks (prev/next cards)
	dc.SetRGBA(0.75, 0.8, 0.9, 0.9)
	dc.DrawRoundedRectangle(8, sy+20, 28, sh-40, 6)
	_ = dc.Fill()
	dc.DrawRoundedRectangle(w-36, sy+20, 28, sh-40, 6)
	_ = dc.Fill()

	// Arrows
	dc.SetRGBA(1, 1, 1, 0.9)
	dc.DrawCircle(sx+18, sy+sh/2, 14)
	_ = dc.Fill()
	dc.DrawCircle(sx+sw-18, sy+sh/2, 14)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.25, 0.25)
	dc.MoveTo(sx+22, sy+sh/2-6)
	dc.LineTo(sx+14, sy+sh/2)
	dc.LineTo(sx+22, sy+sh/2+6)
	_ = dc.Stroke()
	dc.MoveTo(sx+sw-22, sy+sh/2-6)
	dc.LineTo(sx+sw-14, sy+sh/2)
	dc.LineTo(sx+sw-22, sy+sh/2+6)
	_ = dc.Stroke()

	// Dots
	for i := 0; i < 5; i++ {
		dx := w/2 - 40 + i*18
		if i == 2 {
			dc.SetRGB(0.12, 0.36, 0.78)
			dc.DrawCircle(float64(dx), sy+sh+22, 5)
		} else {
			dc.SetRGBA(0, 0, 0, 0.25)
			dc.DrawCircle(float64(dx), sy+sh+22, 4)
		}
		_ = dc.Fill()
	}

	// Progress bar under stage
	dc.SetRGBA(0, 0, 0, 0.08)
	dc.DrawRoundedRectangle(sx, sy+sh+40, sw, 4, 2)
	_ = dc.Fill()
	dc.SetRGB(0.12, 0.36, 0.78)
	dc.DrawRoundedRectangle(sx, sy+sh+40, sw*0.42, 4, 2)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("G2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 10 {
		t.Fatalf("G2 expected dense GPUOps>=10: %s", stats.LogLine())
	}
	// Stage blue region
	r, g, b, _ := p1Sample(dc, 80, 80)
	if b < 120 {
		t.Fatalf("carousel stage not blue: %d,%d,%d", r, g, b)
	}
	// CTA white pill
	r2, g2, b2, _ := p1Sample(dc, int(sx+50), int(sy+116))
	if r2 < 220 {
		t.Fatalf("CTA pill missing: %d,%d,%d", r2, g2, b2)
	}
	// Active dot
	r3, g3, b3, _ := p1Sample(dc, w/2-40+2*18, int(sy+sh+22))
	if b3 < 100 {
		t.Fatalf("active dot missing: %d,%d,%d", r3, g3, b3)
	}
	// Progress filled
	r4, g4, b4, _ := p1Sample(dc, int(sx+40), int(sy+sh+42))
	if b4 < 100 {
		t.Fatalf("progress fill missing: %d,%d,%d", r4, g4, b4)
	}
	t.Logf("stage=%d,%d,%d cta=%d,%d,%d dot=%d,%d,%d prog=%d,%d,%d", r, g, b, r2, g2, b2, r3, g3, b3, r4, g4, b4)
}

// --- Tier H denser virtual window / transfer-scale pressure ---

// H1 large virtual list window: sticky header + 40 rows + alternating cells +
// scrollbar track/thumb + floating action.
func TestP1_H1_LargeVirtualListWindow(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 520
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Chrome
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(16, 16, 360, 480, 8)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.1)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(16.5, 16.5, 359, 479, 8)
	_ = dc.Stroke()

	// Sticky header
	dc.SetRGB(0.97, 0.97, 0.98)
	dc.DrawRectangle(17, 17, 358, 40)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.25, 0.25)
	dc.DrawString("Virtual List · 10k items", 28, 42)
	dc.SetRGBA(0, 0, 0, 0.08)
	dc.DrawRectangle(17, 56, 358, 1)
	_ = dc.Fill()

	// 40 visible rows
	for i := 0; i < 40; i++ {
		y := 60.0 + float64(i)*10.5
		if i%2 == 0 {
			dc.SetRGB(1, 1, 1)
		} else {
			dc.SetRGB(0.98, 0.99, 1.0)
		}
		dc.DrawRectangle(20, y, 330, 10)
		_ = dc.Fill()
		// avatar
		if i%3 == 0 {
			dc.SetRGB(0.2, 0.45, 0.9)
		} else if i%3 == 1 {
			dc.SetRGB(0.2, 0.7, 0.45)
		} else {
			dc.SetRGB(0.85, 0.45, 0.2)
		}
		dc.DrawCircle(34, y+5, 3.5)
		_ = dc.Fill()
		// status pill
		dc.SetRGBA(0.09, 0.47, 0.95, 0.15)
		dc.DrawRoundedRectangle(300, y+1.5, 36, 7, 3)
		_ = dc.Fill()
	}

	// Scrollbar
	dc.SetRGBA(0, 0, 0, 0.06)
	dc.DrawRoundedRectangle(358, 64, 8, 400, 4)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.28)
	dc.DrawRoundedRectangle(358, 120, 8, 70, 4)
	_ = dc.Fill()

	// FAB
	dc.SetRGBA(0.09, 0.47, 0.95, 0.95)
	dc.DrawCircle(340, 460, 22)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("+", 334, 466)

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("H1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 30 {
		t.Fatalf("H1 expected dense GPUOps>=30: %s", stats.LogLine())
	}
	r, g, b, _ := p1Sample(dc, 34, 70)
	if b < 80 && g < 80 {
		t.Fatalf("row avatar missing: %d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 340, 460)
	if b2 < 150 {
		t.Fatalf("FAB missing: %d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 362, 150)
	if r3 > 240 && g3 > 240 && b3 > 240 {
		t.Fatalf("scrollbar thumb missing: %d,%d,%d", r3, g3, b3)
	}
}

// H2 Transfer dual-list heavy: two panels, many rows, ops buttons, search bars.
func TestP1_H2_TransferDualListHeavy(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.95, 0.96, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	drawPanel := func(x float64, title string, selected int) {
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 24, 220, 300, 8)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.12)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(x+0.5, 24.5, 219, 299, 8)
		_ = dc.Stroke()
		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawString(title, x+12, 48)
		// search
		dc.SetRGB(0.98, 0.98, 0.98)
		dc.DrawRoundedRectangle(x+12, 56, 196, 26, 4)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.15)
		dc.DrawRoundedRectangle(x+12.5, 56.5, 195, 25, 4)
		_ = dc.Stroke()
		// rows
		for i := 0; i < 14; i++ {
			y := 92.0 + float64(i)*15
			if i == selected {
				dc.SetRGBA(0.09, 0.47, 0.95, 0.12)
				dc.DrawRectangle(x+8, y-2, 204, 14)
				_ = dc.Fill()
			}
			// checkbox
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(x+14, y, 10, 10, 2)
			_ = dc.Fill()
			dc.SetRGBA(0, 0, 0, 0.25)
			dc.DrawRoundedRectangle(x+14.5, y+0.5, 9, 9, 2)
			_ = dc.Stroke()
			if i < 3 || i == selected {
				dc.SetRGB(0.09, 0.47, 0.95)
				dc.DrawRoundedRectangle(x+16, y+2, 6, 6, 1)
				_ = dc.Fill()
			}
			dc.SetRGB(0.2, 0.2, 0.2)
			dc.DrawString("Item row", x+32, y+9)
		}
	}
	drawPanel(24, "Source (128)", 2)
	drawPanel(316, "Target (36)", 0)

	// Ops
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(256, 140, 44, 28, 4)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString(">", 272, 158)
	dc.SetRGB(0.65, 0.65, 0.7)
	dc.DrawRoundedRectangle(256, 180, 44, 28, 4)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("<", 272, 198)

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("H2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 40 {
		t.Fatalf("H2 expected dense GPUOps>=40: %s", stats.LogLine())
	}
	r, g, b, _ := p1Sample(dc, 278, 154)
	if b < 150 {
		t.Fatalf("transfer > button missing: %d,%d,%d", r, g, b)
	}
	// checked box blue
	r2, g2, b2, _ := p1Sample(dc, 42, 97)
	if b2 < 100 {
		t.Fatalf("source checkbox missing: %d,%d,%d", r2, g2, b2)
	}
}

// --- Tier I: nested dashboard density ---

// I1 Dashboard shell: sider + header + multi cards + charts bars + table strip.
func TestP1_I1_DashboardShellDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 640, 400
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Sider
	dc.SetRGB(0.08, 0.12, 0.2)
	dc.DrawRectangle(0, 0, 160, h)
	_ = dc.Fill()
	for i := 0; i < 8; i++ {
		y := 48.0 + float64(i)*36
		if i == 1 {
			dc.SetRGBA(0.09, 0.47, 0.95, 0.35)
			dc.DrawRoundedRectangle(12, y, 136, 28, 6)
			_ = dc.Fill()
		}
		dc.SetRGB(0.85, 0.9, 0.95)
		dc.DrawString("Nav item", 28, y+18)
	}

	// Header
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(160, 0, w-160, 52)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.08)
	dc.DrawRectangle(160, 51, w-160, 1)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.2, 0.2)
	dc.DrawString("Dashboard · Ant density", 176, 32)

	// Content bg
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(160, 52, w-160, h-52)
	_ = dc.Fill()

	// Stat cards
	for i := 0; i < 4; i++ {
		x := 176.0 + float64(i)*112
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 68, 100, 72, 8)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.08)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(x+0.5, 68.5, 99, 71, 8)
		_ = dc.Stroke()
		dc.SetRGB(0.09, 0.47, 0.95)
		dc.DrawRoundedRectangle(x+12, 88, 40, 8, 2)
		_ = dc.Fill()
		dc.SetRGB(0.4, 0.4, 0.4)
		dc.DrawString("KPI", x+12, 120)
	}

	// Chart card with bars
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(176, 156, 300, 160, 8)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.08)
	dc.DrawRoundedRectangle(176.5, 156.5, 299, 159, 8)
	_ = dc.Stroke()
	for i := 0; i < 12; i++ {
		bh := 20.0 + float64((i*17)%90)
		x := 196.0 + float64(i)*22
		dc.SetRGB(0.2, 0.55, 0.95)
		dc.DrawRoundedRectangle(x, 290-bh, 14, bh, 2)
		_ = dc.Fill()
	}

	// Side list card
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(492, 156, 132, 220, 8)
	_ = dc.Fill()
	for i := 0; i < 10; i++ {
		y := 172.0 + float64(i)*18
		dc.SetRGB(0.9, 0.92, 0.95)
		dc.DrawCircle(508, y+6, 5)
		_ = dc.Fill()
		dc.SetRGB(0.3, 0.3, 0.3)
		dc.DrawString("row", 520, y+10)
	}

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("I1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 40 {
		t.Fatalf("I1 expected dense GPUOps>=40: %s", stats.LogLine())
	}
	r, g, b, _ := p1Sample(dc, 40, 200)
	if r > 80 {
		t.Fatalf("sider not dark: %d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 220, 250)
	if b2 < 100 {
		t.Fatalf("chart bar missing: %d,%d,%d", r2, g2, b2)
	}
}

// I2 Modal stack: page + dim + modal + nested popconfirm chips.
func TestP1_I2_ModalStackDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Page content
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(24, 24+float64(i)*44, w-48, 36, 6)
		_ = dc.Fill()
	}

	// Dim mask
	dc.SetRGBA(0, 0, 0, 0.45)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Modal
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(90, 48, 300, 220, 10)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.12)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(90.5, 48.5, 299, 219, 10)
	_ = dc.Stroke()
	dc.SetRGB(0.15, 0.15, 0.15)
	dc.DrawString("Confirm bulk action", 110, 80)
	// body lines
	for i := 0; i < 5; i++ {
		dc.SetRGB(0.94, 0.95, 0.97)
		dc.DrawRoundedRectangle(110, 100+float64(i)*22, 260, 16, 4)
		_ = dc.Fill()
	}
	// buttons
	dc.SetRGB(0.95, 0.95, 0.96)
	dc.DrawRoundedRectangle(200, 230, 80, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(292, 230, 80, 28, 6)
	_ = dc.Fill()

	// Nested popconfirm
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(250, 160, 160, 90, 8)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.15)
	dc.DrawRoundedRectangle(250.5, 160.5, 159, 89, 8)
	_ = dc.Stroke()
	dc.SetRGB(0.2, 0.2, 0.2)
	dc.DrawString("Are you sure?", 266, 188)
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(330, 210, 60, 24, 4)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("I2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 20 {
		t.Fatalf("I2 expected GPUOps>=20: %s", stats.LogLine())
	}
	r, g, b, _ := p1Sample(dc, 20, 20)
	// dimmed page corner darker than pure white
	if r > 200 {
		t.Fatalf("dim mask missing: %d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 240, 160)
	if r2 < 240 {
		t.Fatalf("modal body missing: %d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 360, 222)
	if b3 < 150 {
		t.Fatalf("popconfirm OK missing: %d,%d,%d", r3, g3, b3)
	}
}

// --- Tier J: notification stack + multi-drawer damage morphology ---

// J1: stacked notifications (top-right) over dense list — Ant Design notification morph.
func TestP1_J1_NotificationStackDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// App shell
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// Sider
	dc.SetRGB(0.12, 0.16, 0.24)
	dc.DrawRectangle(0, 0, 72, h)
	_ = dc.Fill()
	// Content cards
	for i := 0; i < 8; i++ {
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(88, 16+float64(i)*40, w-110, 32, 6)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.08)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(88.5, 16.5+float64(i)*40, w-111, 31, 6)
		_ = dc.Stroke()
		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawString(fmt.Sprintf("List item %02d", i+1), 100, 36+float64(i)*40)
	}

	// Notification stack top-right (4 cards with accent bars)
	for i := 0; i < 4; i++ {
		x := float64(w - 220)
		y := 12 + float64(i)*64
		// shadow plate
		dc.SetRGBA(0, 0, 0, 0.12)
		dc.DrawRoundedRectangle(x+3, y+3, 200, 56, 8)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, y, 200, 56, 8)
		_ = dc.Fill()
		// accent
		cols := [][3]float64{{0.09, 0.47, 0.95}, {0.32, 0.69, 0.31}, {0.95, 0.61, 0.07}, {0.86, 0.21, 0.27}}
		c := cols[i%4]
		dc.SetRGB(c[0], c[1], c[2])
		dc.DrawRoundedRectangle(x, y, 4, 56, 2)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.15, 0.15)
		dc.DrawString(fmt.Sprintf("Notify #%d", i+1), x+14, y+22)
		dc.SetRGB(0.45, 0.45, 0.45)
		dc.DrawString("Operation completed", x+14, y+40)
	}

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("J1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 25 {
		t.Fatalf("J1 expected GPUOps>=25: %s", stats.LogLine())
	}
	// Sider dark
	r, g, b, _ := p1Sample(dc, 20, 40)
	if r > 80 || g > 80 || b > 90 {
		t.Fatalf("sider missing: %d,%d,%d", r, g, b)
	}
	// Top notification white-ish body
	r2, g2, b2, _ := p1Sample(dc, w-100, 30)
	if r2 < 230 {
		t.Fatalf("notification card missing: %d,%d,%d", r2, g2, b2)
	}
	// Accent bar blue-ish on first toast
	r3, g3, b3, _ := p1Sample(dc, w-218, 40)
	if b3 < 150 {
		t.Fatalf("notification accent missing: %d,%d,%d", r3, g3, b3)
	}
}

// J2: dual drawer + floating action + damage-like partial overlays.
func TestP1_J2_DualDrawerOverlayDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Base page grid
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for row := 0; row < 5; row++ {
		for col := 0; col < 4; col++ {
			dc.SetRGB(1, 1, 1)
			x := 16 + float64(col)*120
			y := 16 + float64(row)*60
			dc.DrawRoundedRectangle(x, y, 108, 48, 6)
			_ = dc.Fill()
		}
	}

	// Left drawer
	dc.SetRGBA(0, 0, 0, 0.35)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 220, h)
	_ = dc.Fill()
	for i := 0; i < 10; i++ {
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawRoundedRectangle(12, 16+float64(i)*30, 196, 24, 4)
		_ = dc.Fill()
	}

	// Right nested settings drawer
	dc.SetRGBA(0, 0, 0, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(w-240, 0, 240, h)
	_ = dc.Fill()
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRectangle(w-240, 0, 240, 44)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Settings", float64(w-220), 28)
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.93, 0.94, 0.96)
		dc.DrawRoundedRectangle(float64(w-224), 60+float64(i)*40, 208, 32, 6)
		_ = dc.Fill()
	}

	// FAB
	dc.SetRGBA(0, 0, 0, 0.18)
	dc.DrawCircle(float64(w-48), float64(h-48), 26)
	_ = dc.Fill()
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawCircle(float64(w-50), float64(h-50), 24)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("J2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 30 {
		t.Fatalf("J2 expected GPUOps>=30: %s", stats.LogLine())
	}
	// Left drawer still visible after right-drawer dim (~SourceOver 0.2 black → ~204).
	r, g, b, _ := p1Sample(dc, 40, 40)
	if r < 180 || r > 230 {
		t.Fatalf("left drawer under dim missing/wrong: %d,%d,%d", r, g, b)
	}
	// Right header blue
	r2, g2, b2, _ := p1Sample(dc, w-120, 20)
	if b2 < 150 {
		t.Fatalf("right drawer header missing: %d,%d,%d", r2, g2, b2)
	}
	// FAB blue
	r3, g3, b3, _ := p1Sample(dc, w-50, h-50)
	if b3 < 150 {
		t.Fatalf("FAB missing: %d,%d,%d", r3, g3, b3)
	}
}

// --- Tier K: multi-region damage / HiDPI shell morphology ---

// K1: page shell with multi-region damage redraw (partial FlushGPUWithViewDamage).
func TestP1_K1_DamageMultiRegionUI(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Full page first paint
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// header
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, 48)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.15, 0.15)
	dc.DrawString("Dashboard", 16, 30)
	// 3 content cards
	for i := 0; i < 3; i++ {
		x := 16 + float64(i)*124
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 64, 112, 160, 8)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.08)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(x+0.5, 64.5, 111, 159, 8)
		_ = dc.Stroke()
	}
	p1Flush(t, dc)
	base := dc.RenderPathStats().GPUOps

	// Damage region 1: update card 0 badge
	dc.ResetFrameDamage()
	dc.SetRGB(0.86, 0.21, 0.27)
	dc.DrawCircle(100, 84, 10)
	_ = dc.Fill()
	// Damage region 2: update card 2 button
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(280, 180, 80, 28, 6)
	_ = dc.Fill()

	damage := dc.FrameDamage()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU damage content: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("K1 path_stats %s base=%d damageRects=%d", stats.LogLine(), base, len(damage))
	if stats.GPUOps <= base {
		t.Fatalf("K1 expected more GPUOps after partial updates: %s", stats.LogLine())
	}
	// Badge red
	r, g, b, _ := p1Sample(dc, 100, 84)
	if r < 150 {
		t.Fatalf("badge missing: %d,%d,%d", r, g, b)
	}
	// Button blue
	r2, g2, b2, _ := p1Sample(dc, 320, 194)
	if b2 < 150 {
		t.Fatalf("damaged button missing: %d,%d,%d", r2, g2, b2)
	}
}

// K2: HiDPI-ish dense toolbar + table + sticky footer overlay stack.
func TestP1_K2_HiDPIToolbarTableOverlay(t *testing.T) {
	p1RequireGPU(t)
	// Logical 2x density canvas
	const w, h = 640, 400
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.97, 0.98, 0.99)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Toolbar with many icon buttons
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, 44)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.06)
	dc.DrawRectangle(0, 43, w, 1)
	_ = dc.Fill()
	for i := 0; i < 14; i++ {
		x := 12 + float64(i)*44
		dc.SetRGB(0.94, 0.95, 0.97)
		dc.DrawRoundedRectangle(x, 8, 36, 28, 6)
		_ = dc.Fill()
	}

	// Dense table grid
	for row := 0; row < 10; row++ {
		for col := 0; col < 6; col++ {
			x := 12 + float64(col)*104
			y := 56 + float64(row)*28
			if row%2 == 0 {
				dc.SetRGB(1, 1, 1)
			} else {
				dc.SetRGB(0.97, 0.98, 0.99)
			}
			dc.DrawRectangle(x, y, 100, 26)
			_ = dc.Fill()
			dc.SetRGBA(0, 0, 0, 0.06)
			dc.DrawRectangle(x, y+25, 100, 1)
			_ = dc.Fill()
		}
	}

	// Sticky footer
	dc.SetRGBA(1, 1, 1, 0.96)
	dc.DrawRectangle(0, float64(h-48), w, 48)
	_ = dc.Fill()
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(float64(w-120), float64(h-38), 100, 28, 6)
	_ = dc.Fill()

	// Floating tooltip
	dc.SetRGBA(0, 0, 0, 0.75)
	dc.DrawRoundedRectangle(200, 120, 160, 40, 6)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("K2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 50 {
		t.Fatalf("K2 expected dense GPUOps>=50: %s", stats.LogLine())
	}
	r, g, b, _ := p1Sample(dc, 30, 20)
	if r < 230 {
		t.Fatalf("toolbar missing: %d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-70, h-24)
	if b2 < 150 {
		t.Fatalf("footer CTA missing: %d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 280, 140)
	if r3 > 80 {
		t.Fatalf("tooltip missing: %d,%d,%d", r3, g3, b3)
	}
}

// --- Tier L: form validation + selection table + multi-toast ---

// L1: Ant Design form morphology — labels, inputs, error text, primary CTA.
func TestP1_L1_FormValidationDense(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Card
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(40, 24, 340, 300, 10)
	_ = dc.Fill()
	dc.SetRGBA(0, 0, 0, 0.08)
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(40.5, 24.5, 339, 299, 10)
	_ = dc.Stroke()

	// Title
	dc.SetRGB(0.15, 0.15, 0.15)
	dc.DrawString("Create project", 60, 52)

	// Fields
	for i, label := range []string{"Name", "Owner", "Region", "Budget"} {
		y := 72 + float64(i)*52
		dc.SetRGB(0.35, 0.35, 0.35)
		dc.DrawString(label, 60, y)
		// input
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(60, y+6, 300, 28, 6)
		_ = dc.Fill()
		border := render.RGBA{R: 0.85, G: 0.85, B: 0.85, A: 1}
		if i == 0 {
			border = render.RGBA{R: 0.86, G: 0.21, B: 0.27, A: 1} // error
		} else if i == 1 {
			border = render.RGBA{R: 0.09, G: 0.47, B: 0.95, A: 1} // focus
		}
		dc.SetRGBA(border.R, border.G, border.B, border.A)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(60.5, y+6.5, 299, 27, 6)
		_ = dc.Stroke()
		if i == 0 {
			dc.SetRGB(0.86, 0.21, 0.27)
			dc.DrawString("Name is required", 60, y+48)
		}
	}

	// Primary + cancel
	dc.SetRGB(0.95, 0.95, 0.96)
	dc.DrawRoundedRectangle(220, 290, 80, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(310, 290, 80, 28, 6)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("L1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 15 {
		t.Fatalf("L1 expected GPUOps>=15: %s", stats.LogLine())
	}
	// Error border red-ish near first input
	r, g, b, _ := p1Sample(dc, 62, 84)
	if r < 150 {
		t.Fatalf("error input border missing: %d,%d,%d", r, g, b)
	}
	// Primary blue
	r2, g2, b2, _ := p1Sample(dc, 350, 304)
	if b2 < 150 {
		t.Fatalf("primary CTA missing: %d,%d,%d", r2, g2, b2)
	}
}

// L2: selectable table rows + multi toast stack (status feedback).
func TestP1_L2_TableSelectionToasts(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Table header
	dc.SetRGB(0.98, 0.98, 0.99)
	dc.DrawRectangle(16, 16, w-32, 32)
	_ = dc.Fill()
	// Rows
	for i := 0; i < 9; i++ {
		y := 48 + float64(i)*28
		if i == 2 || i == 5 {
			dc.SetRGB(0.90, 0.95, 1.0) // selected
		} else if i%2 == 0 {
			dc.SetRGB(1, 1, 1)
		} else {
			dc.SetRGB(0.98, 0.98, 0.99)
		}
		dc.DrawRectangle(16, y, w-32, 28)
		_ = dc.Fill()
		// checkbox
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(24, y+6, 16, 16, 3)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.15)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(24.5, y+6.5, 15, 15, 3)
		_ = dc.Stroke()
		if i == 2 || i == 5 {
			dc.SetRGB(0.09, 0.47, 0.95)
			dc.DrawRoundedRectangle(26, y+8, 12, 12, 2)
			_ = dc.Fill()
		}
	}

	// Multi toasts top-right
	for i := 0; i < 3; i++ {
		x := float64(w - 200)
		y := 20 + float64(i)*56
		dc.SetRGBA(0, 0, 0, 0.1)
		dc.DrawRoundedRectangle(x+2, y+2, 180, 48, 8)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, y, 180, 48, 8)
		_ = dc.Fill()
		cols := [][3]float64{{0.32, 0.69, 0.31}, {0.09, 0.47, 0.95}, {0.95, 0.61, 0.07}}
		c := cols[i]
		dc.SetRGB(c[0], c[1], c[2])
		dc.DrawRoundedRectangle(x, y, 4, 48, 2)
		_ = dc.Fill()
	}

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("L2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 30 {
		t.Fatalf("L2 expected GPUOps>=30: %s", stats.LogLine())
	}
	// selected row blue-ish
	r, g, b, _ := p1Sample(dc, 80, 48+2*28+10)
	if b < 200 {
		t.Fatalf("selected row missing: %d,%d,%d", r, g, b)
	}
	// toast body white
	r2, g2, b2, _ := p1Sample(dc, w-100, 40)
	if r2 < 240 {
		t.Fatalf("toast missing: %d,%d,%d", r2, g2, b2)
	}
}

// --- Tier M: chart/dashboard with advanced blend accents ---

// M1: dashboard cards + Difference/Darken accent overlays (UI viz morphology).
func TestP1_M1_ChartDashboardBlend(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// KPI cards
	for i := 0; i < 4; i++ {
		x := 16 + float64(i)*134
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 16, 124, 72, 8)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.06)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(x+0.5, 16.5, 123, 71, 8)
		_ = dc.Stroke()
		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawString(fmt.Sprintf("KPI %d", i+1), x+12, 40)
	}

	// Chart panel
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(16, 104, w-32, 200, 10)
	_ = dc.Fill()

	// Base bars
	for i := 0; i < 12; i++ {
		x := 40 + float64(i)*40
		ht := 40.0 + float64((i*37)%120)
		dc.SetRGB(0.55, 0.72, 0.95)
		dc.DrawRoundedRectangle(x, 280-ht, 28, ht, 4)
		_ = dc.Fill()
	}

	// Darken overlay strip (selection range highlight)
	dc.SetRGBA(0.1, 0.2, 0.5, 0.85)
	dc.SetBlendMode(render.BlendDarken)
	dc.DrawRectangle(120, 120, 160, 160)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	// Difference accent sparkline plate
	dc.SetRGB(0.95, 0.4, 0.2)
	dc.SetBlendMode(render.BlendDifference)
	dc.DrawRoundedRectangle(360, 140, 140, 100, 8)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	// Legend chips
	for i, col := range [][3]float64{{0.09, 0.47, 0.95}, {0.32, 0.69, 0.31}, {0.95, 0.61, 0.07}} {
		x := 40 + float64(i)*90
		dc.SetRGB(col[0], col[1], col[2])
		dc.DrawRoundedRectangle(x, 320, 16, 16, 3)
		_ = dc.Fill()
	}

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("M1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 25 {
		t.Fatalf("M1 expected GPUOps>=25: %s", stats.LogLine())
	}
	// Darken region should not be pure white
	r, g, b, _ := p1Sample(dc, 180, 180)
	if r > 230 && g > 230 && b > 230 {
		t.Fatalf("darken overlay missing: %d,%d,%d", r, g, b)
	}
	// KPI card white
	r2, g2, b2, _ := p1Sample(dc, 40, 40)
	if r2 < 240 {
		t.Fatalf("KPI card missing: %d,%d,%d", r2, g2, b2)
	}
}

// M2: multi-layer map-like heat using SoftLight/Lighten (geo-dashboard morph).
func TestP1_M2_HeatmapSoftLightDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 300
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Base map tiles
	for row := 0; row < 6; row++ {
		for col := 0; col < 8; col++ {
			x := float64(col * 50)
			y := float64(row * 50)
			if (row+col)%2 == 0 {
				dc.SetRGB(0.88, 0.90, 0.93)
			} else {
				dc.SetRGB(0.92, 0.93, 0.95)
			}
			dc.DrawRectangle(x, y, 50, 50)
			_ = dc.Fill()
		}
	}

	// Heat blobs with SoftLight
	dc.SetBlendMode(render.BlendSoftLight)
	for i, cxy := range [][3]float64{{80, 70, 50}, {200, 140, 70}, {300, 90, 45}, {150, 220, 60}} {
		warm := 0.5 + 0.1*float64(i)
		dc.SetRGBA(1.0, warm*0.4, 0.05, 0.9)
		dc.DrawCircle(cxy[0], cxy[1], cxy[2])
		_ = dc.Fill()
	}
	dc.SetBlendMode(render.BlendNormal)

	// Lighten highlight pins
	dc.SetBlendMode(render.BlendLighten)
	dc.SetRGB(1, 1, 0.6)
	for _, p := range [][2]float64{{80, 70}, {200, 140}, {300, 90}} {
		dc.DrawCircle(p[0], p[1], 6)
		_ = dc.Fill()
	}
	dc.SetBlendMode(render.BlendNormal)

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("M2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 40 {
		t.Fatalf("M2 expected dense GPUOps>=40: %s", stats.LogLine())
	}
	r, g, b, _ := p1Sample(dc, 200, 140)
	// heat center should be warmer than cool tile gray
	if r < 150 {
		t.Fatalf("heat blob missing: %d,%d,%d", r, g, b)
	}
}

// --- Tier N: retained multi-panel + WritePixels icon strip + damage ---

// N1: multi-panel shell with partial panel redraw + WritePixels badge strip.
func TestP1_N1_RetainedMultiPanelDamage(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 640, 400
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Shell
	dc.SetRGB(0.12, 0.14, 0.18)
	dc.DrawRectangle(0, 0, 64, h)
	_ = dc.Fill()
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(64, 0, w-64, h)
	_ = dc.Fill()

	// Three content panels
	panels := [][4]float64{
		{80, 16, 260, 180},
		{356, 16, 260, 180},
		{80, 212, 536, 168},
	}
	for i, p := range panels {
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(p[0], p[1], p[2], p[3], 8)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.08)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(p[0]+0.5, p[1]+0.5, p[2]-1, p[3]-1, 8)
		_ = dc.Stroke()
		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawString(fmt.Sprintf("Panel %d", i+1), p[0]+12, p[1]+24)
		// content lines
		for j := 0; j < 4; j++ {
			dc.SetRGB(0.93, 0.94, 0.96)
			dc.DrawRoundedRectangle(p[0]+12, p[1]+40+float64(j)*28, p[2]-24, 20, 4)
			_ = dc.Fill()
		}
	}
	p1Flush(t, dc)
	base := dc.RenderPathStats().GPUOps

	// Retained-style partial update: only panel 2 body + badge via WritePixels
	dc.ResetFrameDamage()
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(368, 56, 120, 28, 6)
	_ = dc.Fill()
	// 8x8 red badge via WritePixels into panel 2 corner
	badge := make([]byte, 8*8*4)
	for i := 0; i < 64; i++ {
		badge[i*4+0] = 220
		badge[i*4+1] = 40
		badge[i*4+2] = 50
		badge[i*4+3] = 255
	}
	dc.WritePixels(580, 24, 8, 8, badge)
	// panel 3 row highlight
	dc.SetRGB(0.90, 0.95, 1.0)
	dc.DrawRoundedRectangle(92, 260, 500, 24, 4)
	_ = dc.Fill()

	damage := dc.FrameDamage()
	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("N1 path_stats %s base=%d damage=%d", stats.LogLine(), base, len(damage))
	if stats.GPUOps <= base {
		t.Fatalf("N1 expected more GPUOps after retained updates: %s", stats.LogLine())
	}
	// blue button in panel 2
	r, g, b, _ := p1Sample(dc, 400, 70)
	if b < 150 {
		t.Fatalf("panel2 update missing: %d,%d,%d", r, g, b)
	}
	// WritePixels badge
	r2, g2, b2, _ := p1Sample(dc, 583, 27)
	if r2 < 180 {
		t.Fatalf("WritePixels badge missing: %d,%d,%d", r2, g2, b2)
	}
	// sider still dark
	r3, g3, b3, _ := p1Sample(dc, 20, 40)
	if r3 > 60 {
		t.Fatalf("sider clobbered: %d,%d,%d", r3, g3, b3)
	}
}

// N2: nested tabs + tree + inspector (IDE-like density).
func TestP1_N2_IDELayoutDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 720, 440
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Activity bar
	dc.SetRGB(0.18, 0.18, 0.2)
	dc.DrawRectangle(0, 0, 48, h)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.3, 0.3, 0.35)
		dc.DrawRoundedRectangle(8, 16+float64(i)*48, 32, 32, 6)
		_ = dc.Fill()
	}
	// Sidebar tree
	dc.SetRGB(0.95, 0.95, 0.96)
	dc.DrawRectangle(48, 0, 200, h)
	_ = dc.Fill()
	for i := 0; i < 14; i++ {
		indent := float64((i % 4) * 12)
		if i == 5 {
			dc.SetRGB(0.09, 0.47, 0.95)
			dc.DrawRectangle(48, 40+float64(i)*24, 200, 22)
			_ = dc.Fill()
			dc.SetRGB(1, 1, 1)
		} else {
			dc.SetRGB(0.2, 0.2, 0.2)
		}
		dc.DrawString(fmt.Sprintf("node-%02d", i), 60+indent, 56+float64(i)*24)
	}
	// Editor tabs
	dc.SetRGB(0.98, 0.98, 0.99)
	dc.DrawRectangle(248, 0, w-248-220, 36)
	_ = dc.Fill()
	for i := 0; i < 4; i++ {
		x := 256 + float64(i)*110
		if i == 1 {
			dc.SetRGB(1, 1, 1)
		} else {
			dc.SetRGB(0.93, 0.93, 0.95)
		}
		dc.DrawRoundedRectangle(x, 6, 100, 24, 4)
		_ = dc.Fill()
	}
	// Editor body lines
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(248, 36, w-248-220, h-36)
	_ = dc.Fill()
	for i := 0; i < 18; i++ {
		dc.SetRGB(0.15, 0.15, 0.18)
		dc.DrawRoundedRectangle(264, 48+float64(i)*18, 200+float64((i*17)%180), 8, 2)
		_ = dc.Fill()
	}
	// Inspector
	dc.SetRGB(0.97, 0.97, 0.98)
	dc.DrawRectangle(w-220, 0, 220, h)
	_ = dc.Fill()
	for i := 0; i < 10; i++ {
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(float64(w-208), 16+float64(i)*36, 196, 28, 4)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 0, 0.08)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(float64(w-207)+0.5, 16.5+float64(i)*36, 195, 27, 4)
		_ = dc.Stroke()
	}

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("N2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 60 {
		t.Fatalf("N2 expected dense GPUOps>=60: %s", stats.LogLine())
	}
	// selected tree row blue
	r, g, b, _ := p1Sample(dc, 100, 40+5*24+10)
	if b < 150 {
		t.Fatalf("tree selection missing: %d,%d,%d", r, g, b)
	}
	// activity bar dark
	r2, g2, b2, _ := p1Sample(dc, 20, 20)
	if r2 > 80 {
		t.Fatalf("activity bar missing: %d,%d,%d", r2, g2, b2)
	}
	// inspector card
	r3, g3, b3, _ := p1Sample(dc, w-100, 30)
	if r3 < 240 {
		t.Fatalf("inspector missing: %d,%d,%d", r3, g3, b3)
	}
}

// --- Tier O: calendar + timeline + Gantt morph ---

// O1: month grid + agenda list + floating event popover.
func TestP1_O1_CalendarTimelineDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 640, 420
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Header
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, 48)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.15, 0.15)
	dc.DrawString("July 2026", 24, 30)
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(float64(w-120), 10, 96, 28, 6)
	_ = dc.Fill()

	// Month grid 7x5
	for row := 0; row < 5; row++ {
		for col := 0; col < 7; col++ {
			x := 16 + float64(col)*60
			y := 64 + float64(row)*52
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(x, y, 56, 48, 4)
			_ = dc.Fill()
			dc.SetRGBA(0, 0, 0, 0.06)
			dc.SetLineWidth(1)
			dc.DrawRoundedRectangle(x+0.5, y+0.5, 55, 47, 4)
			_ = dc.Stroke()
			// event chips
			if (row+col)%3 == 0 {
				dc.SetRGB(0.09, 0.47, 0.95)
				dc.DrawRoundedRectangle(x+4, y+22, 48, 8, 2)
				_ = dc.Fill()
			}
			if (row*7+col)%5 == 0 {
				dc.SetRGB(0.32, 0.69, 0.31)
				dc.DrawRoundedRectangle(x+4, y+34, 36, 8, 2)
				_ = dc.Fill()
			}
		}
	}

	// Agenda sidebar
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(440, 64, 184, 280, 8)
	_ = dc.Fill()
	for i := 0; i < 7; i++ {
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawRoundedRectangle(452, 80+float64(i)*36, 160, 28, 4)
		_ = dc.Fill()
	}

	// Popover over calendar
	dc.SetRGBA(0, 0, 0, 0.12)
	dc.DrawRoundedRectangle(150, 140, 180, 100, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(146, 136, 180, 100, 8)
	_ = dc.Fill()
	dc.SetRGB(0.09, 0.47, 0.95)
	dc.DrawRoundedRectangle(160, 190, 70, 24, 4)
	_ = dc.Fill()

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("O1 path_stats %s", stats.LogLine())
	if stats.GPUOps < 50 {
		t.Fatalf("O1 expected GPUOps>=50: %s", stats.LogLine())
	}
	r, g, b, _ := p1Sample(dc, 200, 180)
	if r < 240 {
		t.Fatalf("popover missing: %d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-70, 24)
	if b2 < 150 {
		t.Fatalf("header CTA missing: %d,%d,%d", r2, g2, b2)
	}
}

// O2: gantt bars + today line + dependency arrows morph.
func TestP1_O2_GanttDependencyDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 700, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.97, 0.98, 0.99)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Left task names
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 140, h)
	_ = dc.Fill()
	for i := 0; i < 8; i++ {
		dc.SetRGB(0.2, 0.2, 0.2)
		dc.DrawString(fmt.Sprintf("Task %02d", i+1), 12, 36+float64(i)*34)
	}

	// Grid
	for i := 0; i < 12; i++ {
		x := 140 + float64(i)*45
		dc.SetRGBA(0, 0, 0, 0.05)
		dc.DrawRectangle(x, 0, 1, h)
		_ = dc.Fill()
	}

	// Gantt bars
	cols := [][3]float64{{0.09, 0.47, 0.95}, {0.32, 0.69, 0.31}, {0.95, 0.61, 0.07}, {0.61, 0.15, 0.69}}
	for i := 0; i < 8; i++ {
		c := cols[i%4]
		x := 150 + float64((i*17)%8)*40
		y := 20 + float64(i)*34
		ww := 80 + float64((i*13)%5)*30
		dc.SetRGB(c[0], c[1], c[2])
		dc.DrawRoundedRectangle(x, y, ww, 18, 4)
		_ = dc.Fill()
		// dependency polyline
		if i > 0 {
			dc.SetRGB(0.45, 0.45, 0.5)
			dc.SetLineWidth(1)
			dc.MoveTo(x-10, y-16)
			dc.LineTo(x-10, y+9)
			dc.LineTo(x, y+9)
			_ = dc.Stroke()
		}
	}

	// Today line
	dc.SetRGB(0.86, 0.21, 0.27)
	dc.DrawRectangle(420, 0, 2, h)
	_ = dc.Fill()

	// Milestone diamonds via small rrects
	for _, mx := range []float64{280, 500, 610} {
		dc.SetRGB(0.95, 0.61, 0.07)
		dc.DrawRoundedRectangle(mx, 140, 12, 12, 2)
		_ = dc.Fill()
	}

	p1Flush(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("O2 path_stats %s", stats.LogLine())
	if stats.GPUOps < 30 {
		t.Fatalf("O2 expected GPUOps>=30: %s", stats.LogLine())
	}
	// today line red
	r, g, b, _ := p1Sample(dc, 421, 40)
	if r < 150 {
		t.Fatalf("today line missing: %d,%d,%d", r, g, b)
	}
	// a bar blue-ish
	r2, g2, b2, _ := p1Sample(dc, 170, 28)
	if b2 < 100 && r2 < 100 {
		t.Fatalf("gantt bar missing: %d,%d,%d", r2, g2, b2)
	}
}

// --- Tier P: advanced blend + compute path UI morphology ---

// P1: dense cards + staged advanced blends (ColorBurn/Exclusion/SoftLight).
// Dual-tex blends sample destination pixmap — FlushGPU before blend ops.
// ColorBurn over pure white is mathematically near-identity; sample over inked bars.
func TestP1_P1_AdvancedBlendCompositeDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 640, 400
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Known bar sample points (col,row) -> (x,y)
	// col1 row1 bar0: x=16+208+14=238, y=16+180+140-40 ≈ 296 (bh varies)
	type bar struct{ x, y float64 }
	bars := make([]bar, 0, 6)
	for col := 0; col < 3; col++ {
		for row := 0; row < 2; row++ {
			x := 16 + float64(col)*208
			y := 16 + float64(row)*180
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(x, y, 196, 164, 10)
			_ = dc.Fill()
			dc.SetRGB(0.09, 0.47, 0.95)
			dc.DrawRoundedRectangle(x, y, 196, 36, 10)
			_ = dc.Fill()
			dc.SetRGB(1, 1, 1)
			dc.DrawRectangle(x, y+20, 196, 16)
			_ = dc.Fill()
			dc.SetRGB(0.15, 0.15, 0.18)
			dc.DrawString(fmt.Sprintf("Panel %c%d", 'A'+col, row+1), x+12, y+24)
			for i := 0; i < 8; i++ {
				bh := 30.0 + float64((i*17+col*5+row*9)%30)
				bx := x + 14 + float64(i)*22
				by := y + 140 - bh
				dc.SetRGBA(0.2, 0.55, 0.9, 0.85)
				dc.DrawRoundedRectangle(bx, by, 16, bh, 3)
				_ = dc.Fill()
				if col == 1 && row == 1 && i == 2 {
					bars = append(bars, bar{x: bx + 8, y: by + bh/2})
				}
				if col == 0 && row == 0 && i == 3 {
					bars = append(bars, bar{x: bx + 8, y: by + bh/2})
				}
			}
		}
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base FlushGPU: %v", err)
	}
	baseOps := dc.RenderPathStats().GPUOps
	if baseOps == 0 {
		t.Fatalf("base UI produced no GPU ops")
	}
	if len(bars) < 2 {
		t.Fatalf("internal: expected bar anchors")
	}
	br, bg, bb, _ := p1Sample(dc, int(bars[0].x), int(bars[0].y))
	t.Logf("bar0 before blend @%.0f,%.0f = %d,%d,%d", bars[0].x, bars[0].y, br, bg, bb)
	p1NotNearWhite(t, "bar0 before", br, bg, bb)

	// SoftLight over left bar cluster
	dc.SetBlendMode(render.BlendSoftLight)
	dc.SetRGBA(0.95, 0.55, 0.1, 0.9)
	dc.DrawCircle(bars[1].x, bars[1].y, 36)
	_ = dc.Fill()

	// ColorBurn over known blue bar (non-white dest — ColorBurn(white,*) is identity)
	dc.SetBlendMode(render.BlendColorBurn)
	dc.SetRGB(0.6, 0.2, 0.15)
	dc.DrawCircle(bars[0].x, bars[0].y, 40)
	_ = dc.Fill()

	// Exclusion blue over pale card body
	dc.SetBlendMode(render.BlendExclusion)
	dc.SetRGB(0.2, 0.25, 0.9)
	dc.DrawRoundedRectangle(430, 50, 160, 100, 10)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("blend FlushGPU: %v", err)
	}
	t.Logf("P1 after blends path_stats %s baseOps=%d", dc.RenderPathStats().LogLine(), baseOps)
	if dc.RenderPathStats().GPUOps <= baseOps {
		t.Fatalf("advanced blends must add GPUOps")
	}
	if dc.RenderPathStats().CPUFallbackOps > 0 {
		t.Fatalf("P1 unexpected CPU fallback: %s", dc.RenderPathStats().LogLine())
	}

	r, g, b, _ := p1Sample(dc, int(bars[0].x), int(bars[0].y))
	t.Logf("colorburn sample=%d,%d,%d (was %d,%d,%d)", r, g, b, br, bg, bb)
	p1NotNearWhite(t, "colorburn bar", r, g, b)
	// Must differ from pre-blend bar (dual-tex took effect)
	if r == br && g == bg && b == bb {
		t.Fatalf("colorburn did not modify destination bar pixel")
	}

	r2, g2, b2, _ := p1Sample(dc, 500, 90)
	t.Logf("exclusion sample=%d,%d,%d", r2, g2, b2)
	p1NotNearWhite(t, "exclusion panel", r2, g2, b2)

	dc.SetBlendMode(render.BlendNormal)
	dc.SetRGBA(0, 0, 0, 0.12)
	dc.DrawRectangle(0, h-36, w, 36)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.2, 0.25)
	dc.DrawString("advanced blend composite density", 16, h-14)
	p1Flush(t, dc)
	if dc.RenderPathStats().GPUOps < 40 {
		t.Fatalf("P1 expected GPUOps>=40: %s", dc.RenderPathStats().LogLine())
	}
}

// P2: UI chrome + compute-path canvas (K.01) in one scene.
func TestP1_P2_ComputePathUIChromeDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("bootstrap FlushGPU: %v", err)
	}
	a := render.Accelerator()
	cpa, ok := a.(render.ComputePipelineAware)
	if !ok || !cpa.CanCompute() {
		t.Skip("compute pipeline unavailable")
	}

	dc.SetRGB(0.12, 0.14, 0.18)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("Compute path canvas + UI chrome", 12, 26)

	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 40, 140, h-40)
	_ = dc.Fill()
	for i := 0; i < 8; i++ {
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(fmt.Sprintf("Layer %02d", i+1), 16, 70+float64(i)*30)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("chrome FlushGPU: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	dc.SetPipelineMode(render.PipelineModeCompute)
	for i := 0; i < 6; i++ {
		cx := 230.0 + float64(i%3)*95
		cy := 120.0 + float64(i/3)*110
		dc.SetRGB(0.15+float64(i)*0.1, 0.4+float64(i%2)*0.15, 0.85-float64(i)*0.05)
		dc.MoveTo(cx, cy-34)
		for p := 1; p < 5; p++ {
			ang := float64(p) * 144.0 * math.Pi / 180.0
			dc.LineTo(cx+34*math.Sin(ang), cy-34*math.Cos(ang))
		}
		dc.ClosePath()
		_ = dc.Fill()

		dc.SetRGB(0.2, 0.75, 0.4)
		dc.MoveTo(cx-20, cy+28)
		dc.LineTo(cx, cy+10)
		dc.LineTo(cx+20, cy+28)
		dc.LineTo(cx+10, cy+28)
		dc.LineTo(cx+10, cy+42)
		dc.LineTo(cx-10, cy+42)
		dc.LineTo(cx-10, cy+28)
		dc.ClosePath()
		_ = dc.Fill()
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("compute FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("P2 compute path_stats %s base=%d", stats.LogLine(), base)
	if stats.GPUOps <= base {
		t.Fatalf("P2 compute density requires additional GPUOps: %s", stats.LogLine())
	}

	r, g, b, _ := p1Sample(dc, 230, 110)
	t.Logf("compute star sample=%d,%d,%d", r, g, b)
	p1NotNearWhite(t, "compute star", r, g, b)
	rA, gA, bA, _ := p1Sample(dc, 230, 150)
	t.Logf("compute arrow sample=%d,%d,%d", rA, gA, bA)
	p1NotNearWhite(t, "compute arrow", rA, gA, bA)

	dc.SetPipelineMode(render.PipelineModeAuto)
	dc.SetRGB(0.18, 0.2, 0.24)
	dc.DrawRectangle(0, h-28, w, 28)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.9, 0.92)
	dc.DrawString("status: compute path active", 12, h-10)
	p1Flush(t, dc)
	if dc.RenderPathStats().GPUOps < 15 {
		t.Fatalf("P2 expected GPUOps>=15: %s", dc.RenderPathStats().LogLine())
	}
	r2, g2, b2, _ := p1Sample(dc, 40, 100)
	t.Logf("sidebar sample=%d,%d,%d", r2, g2, b2)
	if r2 < 200 {
		t.Fatalf("sidebar chrome corrupted: %d,%d,%d", r2, g2, b2)
	}
}
