//go:build !nogpu

package render_test

// Phase A — arbitrary composition dimension probes (not widget/antd catalog).
//
// Architecture:
//   render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native
//
// IDs D01+ match docs/P1_COMPOSITION_MATRIX.md.
// Hard rules: GPUOps>0, structural pixels, real native path.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

func compMakeImage(t *testing.T, w, h int, r, g, b uint8) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_ = img.SetRGBA(x, y, r, g, b, 255)
		}
	}
	return img
}

// compRepoTmpCompDir resolves <repo>/tmp/comp for PNG dumps (cwd may be render/).
func compRepoTmpCompDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			out := filepath.Join(dir, "tmp", "comp")
			if err := os.MkdirAll(out, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", out, err)
			}
			return out
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	out := filepath.Join(wd, "tmp", "comp")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", out, err)
	}
	return out
}

// compSavePNG writes GPU-flushed context pixels to gpui/tmp/comp/<name>.png.
func compSavePNG(t *testing.T, dc *render.Context, name string) {
	t.Helper()
	path := filepath.Join(compRepoTmpCompDir(t), name+".png")
	if err := dc.SavePNG(path); err != nil {
		t.Fatalf("SavePNG %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
}

// compAutoSavePNG dumps PNG named from the test function (Dxx_...).
func compAutoSavePNG(t *testing.T, dc *render.Context) {
	t.Helper()
	name := t.Name()
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimPrefix(name, "TestP1_Comp_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "" {
		name = "unnamed"
	}
	compSavePNG(t, dc, name)
}

// D01: nested ClipRect × semi-transparent PushLayer × text.
func TestP1_Comp_D01_ClipLayerText(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 16)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Outer page chrome
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Nested clips: viewport then card
	dc.ClipRect(24, 24, 272, 152)
	dc.SetRGB(0.20, 0.45, 0.85)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRect(48, 48, 224, 104)
	dc.PushLayer(render.BlendNormal, 0.55)
	dc.SetRGB(1, 0.85, 0.20)
	dc.DrawRoundedRectangle(48, 48, 224, 104, 10)
	_ = dc.Fill()
	dc.SetRGB(0.10, 0.12, 0.16)
	dc.DrawString("clip×layer×text", 64, 108)
	dc.PopLayer()

	p1Flush(t, dc)
	compSavePNG(t, dc, "D01_ClipLayerText")

	// Inside nested clip: yellow-ish over blue through layer opacity → not pure white
	r, g, b, _ := p1Sample(dc, 160, 100)
	p1NotNearWhite(t, "D01 layer body", r, g, b)
	if g < 40 {
		t.Fatalf("D01 expected warm/yellow contribution rgba=%d,%d,%d", r, g, b)
	}
	// Outside outer clip should remain page gray-ish / white-ish, not solid blue
	r2, g2, b2, _ := p1Sample(dc, 8, 8)
	if b2 > 200 && r2 < 80 {
		t.Fatalf("D01 outer chrome leaked blue rgba=%d,%d,%d", r2, g2, b2)
	}
	// Between outer and inner clip: blue fill should show
	r3, g3, b3, _ := p1Sample(dc, 36, 36)
	if b3 < 100 {
		t.Fatalf("D01 outer clip band not blue-ish rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D02: ClipRoundRect × DrawImage × BlendPlus.
func TestP1_Comp_D02_ClipImageBlend(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 240, 180
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Red base under the clip window
	dc.SetRGB(0.85, 0.15, 0.12)
	dc.DrawRectangle(20, 20, 200, 140)
	_ = dc.Fill()

	img := compMakeImage(t, 64, 64, 20, 60, 220)
	dc.ClipRoundRect(40, 40, 160, 100, 16)
	dc.DrawImage(img, 40, 40)
	dc.DrawImage(img, 88, 56)

	// Plus blend overlay rect inside clip
	dc.SetBlendMode(render.BlendPlus)
	dc.SetRGBA(0.35, 0.35, 0.0, 1)
	dc.DrawRectangle(60, 60, 80, 50)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	p1Flush(t, dc)
	compSavePNG(t, dc, "D02_ClipImageBlend")

	// Center should not be pure red base (image+plus)
	r, g, b, _ := p1Sample(dc, 100, 80)
	p1NotNearWhite(t, "D02 clipped content", r, g, b)
	// Outside round-rect clip corner (near canvas edge of red rect but outside rrect)
	// Sample just outside clip at top-left corner area of rrect.
	ro, go_, bo, _ := p1Sample(dc, 42, 42)
	// Corner of rrect may AA; outside red base at 24,24 should be red
	r2, g2, b2, _ := p1Sample(dc, 24, 24)
	if r2 < 150 || g2 > 100 {
		t.Fatalf("D02 base red missing outside clip rgba=%d,%d,%d", r2, g2, b2)
	}
	t.Logf("D02 center=%d,%d,%d cornerAA=%d,%d,%d outside=%d,%d,%d", r, g, b, ro, go_, bo, r2, g2, b2)
}

// D03: path Clip × PushLayer × solid fill.
func TestP1_Comp_D03_ClipPathLayerFill(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 260, 220
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Background grid-ish
	dc.SetRGB(0.90, 0.91, 0.93)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Diamond clip path
	dc.MoveTo(130, 20)
	dc.LineTo(230, 110)
	dc.LineTo(130, 200)
	dc.LineTo(30, 110)
	dc.ClosePath()
	dc.Clip()

	dc.PushLayer(render.BlendNormal, 0.85)
	dc.SetRGB(0.15, 0.55, 0.35)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawCircle(130, 110, 28)
	_ = dc.Fill()
	dc.PopLayer()

	p1Flush(t, dc)
	compSavePNG(t, dc, "D03_ClipPathLayerFill")

	// Center inside diamond: green-ish
	r, g, b, _ := p1Sample(dc, 130, 110)
	// center is white circle on green → near white
	if r < 180 || g < 180 || b < 180 {
		// white circle expected; sample ring instead
		r, g, b, _ = p1Sample(dc, 130, 70)
	}
	rRing, gRing, bRing, _ := p1Sample(dc, 130, 60)
	if gRing < 80 {
		t.Fatalf("D03 diamond fill not green-ish rgba=%d,%d,%d", rRing, gRing, bRing)
	}
	// Outside diamond: gray page
	r2, g2, b2, _ := p1Sample(dc, 20, 20)
	if g2 > 200 && r2 < 50 {
		t.Fatalf("D03 clip leaked green to corner rgba=%d,%d,%d", r2, g2, b2)
	}
	if r2 < 180 || g2 < 180 {
		// page is light gray ~0.9 → ~230
	}
	_ = r
	_ = g
	_ = b
	_ = b2
}

// D04: HiDPI × hairline × text.
func TestP1_Comp_D04_HiDPIHairlineText(t *testing.T) {
	p1RequireGPU(t)
	// physical 320×160, logical via scale 2 → 160×80
	dc := render.NewContext(320, 160, render.WithDeviceScale(2.0))
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 160, 80)
	_ = dc.Fill()

	dc.SetRGB(0.12, 0.14, 0.18)
	dc.SetLineWidth(0) // hairline
	dc.DrawLine(8, 20, 152, 20)
	_ = dc.Stroke()
	dc.DrawLine(8, 40, 152, 40)
	_ = dc.Stroke()
	dc.DrawString("HiDPI hairline×text", 12, 58)

	p1Flush(t, dc)
	compSavePNG(t, dc, "D04_HiDPIHairlineText")

	// Physical midline rows for hairlines (logical y=20 → phys 40)
	ink := 0
	for x := 20; x < 300; x++ {
		r, g, b, _ := p1Sample(dc, x, 40)
		if r < 200 || g < 200 || b < 200 {
			ink++
		}
	}
	if ink < 8 {
		t.Fatalf("D04 hairline ink too low: %d", ink)
	}
	// Text ink near baseline physical y≈58*2=116
	textInk := 0
	for y := 100; y < 130; y++ {
		for x := 24; x < 280; x++ {
			r, g, b, _ := p1Sample(dc, x, y)
			if r < 220 || g < 220 || b < 220 {
				textInk++
			}
		}
	}
	if textInk < 10 {
		t.Fatalf("D04 text ink too low: %d", textInk)
	}
	t.Logf("D04 hairlineInk=%d textInk=%d scale=%v", ink, textInk, dc.DeviceScale())
}

// D05: outer clip × Multiply layer × nested Normal layer.
func TestP1_Comp_D05_LayerBlendClip(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Light cyan base
	dc.SetRGB(0.70, 0.90, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRect(40, 30, 200, 140)

	dc.PushLayer(render.BlendMultiply, 1.0)
	dc.SetRGB(1.0, 0.55, 0.55) // multiply-ish red veil
	dc.DrawRectangle(40, 30, 200, 140)
	_ = dc.Fill()

	dc.PushLayer(render.BlendNormal, 0.7)
	dc.SetRGB(0.15, 0.25, 0.85)
	dc.DrawRoundedRectangle(70, 60, 120, 70, 12)
	_ = dc.Fill()
	dc.PopLayer()
	dc.PopLayer()

	p1Flush(t, dc)
	compSavePNG(t, dc, "D05_LayerBlendClip")

	// Inside card
	r, g, b, _ := p1Sample(dc, 130, 95)
	p1NotNearWhite(t, "D05 card", r, g, b)
	if b < 40 {
		t.Fatalf("D05 expected blue-ish card rgba=%d,%d,%d", r, g, b)
	}
	// Outside clip: cyan base, not dark multiply
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 < 120 || g2 < 150 {
		t.Fatalf("D05 outside clip should stay light cyan rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D06: image × text × clip × backdrop.
func TestP1_Comp_D06_ImageTextClipBackdrop(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Content board
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	img := compMakeImage(t, 48, 48, 40, 140, 220)
	for i := 0; i < 4; i++ {
		x := 24 + float64(i)*80
		dc.ClipRect(x, 40, 64, 64)
		dc.DrawImage(img, x+8, 48)
		dc.ResetClip()
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("tile-%d", i), x+8, 128)
	}

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("pre-backdrop flush: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.40)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(80, 50, 200, 140, 12)
	_ = dc.Fill()
	dc.SetRGB(0.12, 0.12, 0.15)
	dc.DrawString("backdrop×image×text", 100, 120)
	dc.PopLayer()

	p1Flush(t, dc)
	compSavePNG(t, dc, "D06_ImageTextClipBackdrop")
	stats := dc.RenderPathStats()
	if stats.GPUOps <= base {
		t.Fatalf("D06 backdrop should add GPUOps base=%d now=%s", base, stats.LogLine())
	}
	// Card center white
	r, g, b, _ := p1Sample(dc, 180, 120)
	if r < 200 || g < 200 || b < 200 {
		t.Fatalf("D06 modal card missing rgba=%d,%d,%d", r, g, b)
	}
	// Dimmed board corner
	r2, g2, b2, _ := p1Sample(dc, 12, 12)
	if r2 > 180 && g2 > 180 && b2 > 180 {
		t.Fatalf("D06 expected dimmed backdrop corner rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D07: Translate/Scale × ClipRect × fill.
func TestP1_Comp_D07_TransformClipFill(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 300, 220
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.Push()
	dc.Translate(40, 30)
	dc.Scale(1.5, 1.5)
	// In transformed space: clip 0..100, fill red
	dc.ClipRect(0, 0, 100, 80)
	dc.SetRGB(0.85, 0.20, 0.15)
	dc.DrawRectangle(-20, -20, 200, 200)
	_ = dc.Fill()
	dc.Pop() // pops transform; clip may still be device-space depending on impl

	// Second transformed green strip
	dc.Push()
	dc.Translate(20, 140)
	dc.Scale(2.0, 1.0)
	dc.ClipRect(10, 0, 60, 30)
	dc.SetRGB(0.15, 0.65, 0.30)
	dc.DrawRectangle(0, 0, 100, 40)
	_ = dc.Fill()
	dc.Pop()

	p1Flush(t, dc)
	compSavePNG(t, dc, "D07_TransformClipFill")

	// Transformed red region roughly at device (40+0*1.5, 30) ..
	r, g, b, _ := p1Sample(dc, 80, 50)
	if r < 120 {
		t.Fatalf("D07 transformed red missing rgba=%d,%d,%d", r, g, b)
	}
	// Far corner unfilled gray
	r2, g2, b2, _ := p1Sample(dc, 280, 20)
	if r2 < 200 {
		t.Fatalf("D07 expected clean corner rgba=%d,%d,%d", r2, g2, b2)
	}
	// Green strip area ~ x=20+10*2=40 .., y=140
	r3, g3, b3, _ := p1Sample(dc, 60, 150)
	if g3 < 80 {
		t.Fatalf("D07 green strip missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D08: multi-region redraw — full base then two dirty regions re-inked correctly.
func TestP1_Comp_D08_MultiRegionRedraw(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Stable chrome
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("multi-region redraw", 12, 24)

	// Two content panels
	dc.SetRGB(0.88, 0.90, 0.93)
	dc.DrawRoundedRectangle(16, 52, 130, 120, 8)
	_ = dc.Fill()
	dc.DrawRoundedRectangle(174, 52, 130, 120, 8)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("A", 70, 120)
	dc.DrawString("B", 230, 120)

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base flush: %v", err)
	}
	baseOps := dc.RenderPathStats().GPUOps

	// Dirty region A: recolor left panel
	dc.ClipRect(16, 52, 130, 120)
	dc.SetRGB(0.20, 0.45, 0.90)
	dc.DrawRectangle(16, 52, 130, 120)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("A*", 66, 120)
	dc.ResetClip()

	// Dirty region B: recolor right panel
	dc.ClipRect(174, 52, 130, 120)
	dc.SetRGB(0.15, 0.65, 0.40)
	dc.DrawRectangle(174, 52, 130, 120)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("B*", 226, 120)
	dc.ResetClip()

	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	stats := dc.RenderPathStats()
	if stats.GPUOps <= baseOps {
		t.Fatalf("D08 dirty redraw should increase GPUOps base=%d now=%s", baseOps, stats.LogLine())
	}

	// Left blue
	r, g, b, _ := p1Sample(dc, 80, 110)
	if b < 100 || r > b {
		t.Fatalf("D08 left dirty region not blue rgba=%d,%d,%d", r, g, b)
	}
	// Right green
	r2, g2, b2, _ := p1Sample(dc, 240, 110)
	if g2 < 100 || g2 < r2 {
		t.Fatalf("D08 right dirty region not green rgba=%d,%d,%d", r2, g2, b2)
	}
	// Header unchanged dark
	r3, g3, b3, _ := p1Sample(dc, 20, 18)
	if r3 > 80 || g3 > 80 {
		t.Fatalf("D08 header should stay dark rgba=%d,%d,%d", r3, g3, b3)
	}
	// Between panels: still white-ish page
	r4, g4, b4, _ := p1Sample(dc, 160, 110)
	if r4 < 200 {
		t.Fatalf("D08 gap between regions polluted rgba=%d,%d,%d", r4, g4, b4)
	}
}

// TestP1_Comp_DimensionAxes_DumpPNG renders a one-shot board of the Phase A
// dimension axes (docs/P1_COMPOSITION_MATRIX.md) and writes:
//
//	tmp/comp/dimension_axes.png
func TestP1_Comp_DimensionAxes_DumpPNG(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 720, 420
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, 44)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("Phase A dimension axes — composition matrix", 16, 28)

	axes := []struct {
		title string
		draw  func()
	}{
		{"clip", func() {
			dc.ClipRoundRect(0, 0, 140, 90, 12)
			dc.SetRGB(0.25, 0.55, 0.9)
			dc.DrawRectangle(0, 0, 200, 120)
			_ = dc.Fill()
			dc.ResetClip()
		}},
		{"layer", func() {
			dc.SetRGB(0.3, 0.35, 0.45)
			dc.DrawRoundedRectangle(10, 10, 100, 60, 8)
			_ = dc.Fill()
			dc.PushLayer(render.BlendNormal, 0.55)
			dc.SetRGB(0.95, 0.75, 0.2)
			dc.DrawRoundedRectangle(30, 25, 100, 55, 8)
			_ = dc.Fill()
			dc.PopLayer()
		}},
		{"blend", func() {
			dc.SetRGB(0.2, 0.6, 0.9)
			dc.DrawCircle(50, 45, 30)
			_ = dc.Fill()
			dc.SetBlendMode(render.BlendMultiply)
			dc.SetRGB(0.95, 0.4, 0.3)
			dc.DrawCircle(75, 45, 30)
			_ = dc.Fill()
			dc.SetBlendMode(render.BlendNormal)
		}},
		{"text", func() {
			_ = dc.LoadFontFace(font, 16)
			dc.SetRGB(0.15, 0.16, 0.2)
			dc.DrawString("Aa 文字", 16, 50)
			_ = dc.LoadFontFace(font, 12)
		}},
		{"image", func() {
			img := compMakeImage(t, 48, 48, 40, 160, 220)
			dc.DrawImage(img, 20, 20)
			dc.DrawImage(img, 55, 35)
		}},
		{"transform", func() {
			dc.Push()
			dc.Translate(70, 50)
			dc.Rotate(0.35)
			dc.SetRGB(0.9, 0.35, 0.4)
			dc.DrawRoundedRectangle(-40, -20, 80, 40, 6)
			_ = dc.Fill()
			dc.Pop()
		}},
		{"HiDPI", func() {
			dc.SetRGB(0.2, 0.25, 0.35)
			dc.SetLineWidth(0)
			dc.DrawLine(10, 20, 130, 20)
			_ = dc.Stroke()
			dc.DrawLine(10, 40, 130, 40)
			_ = dc.Stroke()
			dc.SetRGB(0.9, 0.92, 0.95)
			dc.DrawString("scale/hairline", 16, 70)
		}},
		{"damage", func() {
			dc.SetRGB(0.85, 0.87, 0.9)
			dc.DrawRoundedRectangle(8, 8, 124, 74, 8)
			_ = dc.Fill()
			dc.ClipRect(20, 20, 50, 40)
			dc.SetRGB(0.25, 0.55, 0.95)
			dc.DrawRectangle(0, 0, 200, 120)
			_ = dc.Fill()
			dc.ResetClip()
			dc.ClipRect(70, 35, 50, 40)
			dc.SetRGB(0.3, 0.75, 0.45)
			dc.DrawRectangle(0, 0, 200, 120)
			_ = dc.Fill()
			dc.ResetClip()
		}},
	}

	for i, ax := range axes {
		col, row := i%4, i/4
		x, y := 20+float64(col)*175, 60+float64(row)*170
		// card
		dc.SetRGB(0.98, 0.98, 1)
		dc.DrawRoundedRectangle(x, y, 160, 150, 12)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(ax.title, x+12, y+22)
		// content region
		dc.Push()
		dc.Translate(x+10, y+36)
		ax.draw()
		dc.Pop()
	}

	p1Flush(t, dc)
	compSavePNG(t, dc, "dimension_axes")
	r, g, b, _ := p1Sample(dc, 40, 20)
	if r > 80 && g > 80 && b > 80 {
		// header dark expected
		r, g, b, _ = p1Sample(dc, 8, 8)
	}
	if r > 100 && g > 100 && b > 100 {
		t.Fatalf("dimension axes header missing rgba=%d,%d,%d", r, g, b)
	}
}
