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
	p1White(dc, w, h)

	// Header
	dc.SetRGB(0.96, 0.96, 0.97)
	dc.DrawRectangle(ox, oy, cw*cols, rh)
	_ = dc.Fill()

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			x := ox + float64(col)*cw
			y := oy + float64(row)*rh
			if row == 3 && col == 1 {
				dc.SetRGBA(0.90, 0.94, 1.0, 1)
				dc.DrawRectangle(x, y, cw, rh)
				_ = dc.Fill()
			}
			// Clip text to cell
			dc.ClipRect(x+2, y+2, cw-4, rh-4)
			dc.SetRGB(0.15, 0.15, 0.18)
			dc.DrawString("cell-long-text", x+4, y+18)
			dc.ResetClip()
		}
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
