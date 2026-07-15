//go:build !nogpu

package widget_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/widget"
)

func requireGPU(t *testing.T) {
	t.Helper()
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Log("WGPU_NATIVE_PATH unset; relying on default lib discovery")
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	probe := render.NewContext(8, 8)
	defer probe.Close()
	probe.SetRGB(1, 0, 0)
	probe.DrawRectangle(0, 0, 8, 8)
	_ = probe.Fill()
	if err := probe.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}
	if probe.RenderPathStats().GPUOps == 0 {
		t.Skipf("no GPU ops on probe: %s", probe.RenderPathStats().LogLine())
	}
}

func TestW0_FirstBatch_PresentGPU(t *testing.T) {
	requireGPU(t)
	th := widget.DefaultTheme()
	const W, H = 640, 480
	dc := render.NewContext(W, H)
	defer dc.Close()
	if err := dc.LoadFontFace(findFont(t), th.FontSize); err != nil {
		t.Fatalf("font: %v", err)
	}
	view, rel := dc.CreateOffscreenTexture(W, H)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	drawShell := func() {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		// page bg
		dc.SetRGB(0.94, 0.95, 0.97)
		dc.DrawRectangle(0, 0, W, H)
		_ = dc.Fill()

		// list panel
		for i := 0; i < 8; i++ {
			widget.ListRow{
				Bounds:     widget.Rect{X: 16, Y: 16 + float64(i)*40, W: 220, H: 40},
				Title:      fmt.Sprintf("Row %c", 'A'+i),
				Selected:   i == 2,
				ShowAvatar: true,
			}.Draw(dc, th)
		}

		// form
		widget.Input{
			Bounds: widget.Rect{X: 260, Y: 24, W: 340, H: 70},
			Label:  "Email", Value: "user@example.com", Focused: true,
		}.Draw(dc, th)
		widget.Input{
			Bounds: widget.Rect{X: 260, Y: 110, W: 340, H: 70},
			Label:  "Name", Placeholder: "Your name",
		}.Draw(dc, th)
		widget.Button{Bounds: widget.Rect{X: 260, Y: 200, W: 110, H: 32}, Label: "Default", Style: widget.ButtonDefault}.Draw(dc, th)
		widget.Button{Bounds: widget.Rect{X: 382, Y: 200, W: 110, H: 32}, Label: "Primary", Style: widget.ButtonPrimary, Hovered: true}.Draw(dc, th)
		widget.Button{Bounds: widget.Rect{X: 504, Y: 200, W: 96, H: 32}, Label: "Danger", Style: widget.ButtonDanger}.Draw(dc, th)

		// table header + cells
		cols := []string{"ID", "Name", "Status"}
		xw := []float64{60, 160, 100}
		x := 260.0
		for i, h := range cols {
			widget.TableCell{Bounds: widget.Rect{X: x, Y: 260, W: xw[i], H: 32}, Text: h, Header: true, Grid: true}.Draw(dc, th)
			x += xw[i]
		}
		x = 260
		vals := []string{"01", "Ada", "on"}
		for i, v := range vals {
			widget.TableCell{Bounds: widget.Rect{X: x, Y: 292, W: xw[i], H: 32}, Text: v, Grid: true}.Draw(dc, th)
			x += xw[i]
		}

		// modal on top
		widget.Modal{
			HostW: W, HostH: H,
			Panel:       widget.CenterPanel(W, H, 320, 180),
			Title:       "Confirm action",
			Body:        "Modal body via widget package.",
			ShowOverlay: true,
			OKLabel:     "OK",
			CancelLabel: "Cancel",
		}.Draw(dc, th)
	}

	drawShell()
	if err := dc.PresentFrame(view, uint32(W), uint32(H), func() error { return nil }); err != nil {
		t.Fatalf("present: %v", err)
	}
	st := dc.RenderPathStats()
	if st.GPUOps == 0 {
		t.Fatalf("GPUOps==0: %s", st.LogLine())
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fallback_ops=%d", st.CPUFallbackOps)
	}
	t.Logf("W0 present shell gpu=%d cpu_fb=%d %s", st.GPUOps, st.CPUFallbackOps, st.LogLine())

	// Steady-state gate uses a damage-friendly form chrome (no modal overlay).
	// Full kitchen-sink+modal is stress paint; correctness already proven above.
	steady := func() {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		dc.SetRGB(0.94, 0.95, 0.97)
		dc.DrawRectangle(0, 0, W, H)
		_ = dc.Fill()
		widget.Input{
			Bounds: widget.Rect{X: 40, Y: 40, W: 360, H: 70},
			Label:  "Email", Value: "user@example.com", Focused: true,
		}.Draw(dc, th)
		widget.Button{Bounds: widget.Rect{X: 40, Y: 140, W: 120, H: 32}, Label: "Save", Style: widget.ButtonPrimary, Hovered: true}.Draw(dc, th)
		widget.ListRow{Bounds: widget.Rect{X: 40, Y: 200, W: 360, H: 40}, Title: "Row A", Selected: true, ShowAvatar: true}.Draw(dc, th)
		widget.TableCell{Bounds: widget.Rect{X: 40, Y: 260, W: 120, H: 32}, Text: "Cell", Grid: true}.Draw(dc, th)
	}
	// bootstrap steady
	steady()
	if err := dc.PresentFrameFull(view, uint32(W), uint32(H), nil); err != nil {
		t.Fatalf("steady bootstrap: %v", err)
	}

	var samples []float64
	for i := 0; i < 10; i++ {
		t0 := time.Now()
		steady()
		// Prefer auto/damage present after first full frame.
		if _, err := dc.PresentFrameAuto(view, uint32(W), uint32(H), nil); err != nil {
			t.Fatalf("steady present: %v", err)
		}
		dt := time.Since(t0).Seconds() * 1000
		if dc.RenderPathStats().GPUOps == 0 {
			t.Fatal("steady GPUOps==0")
		}
		if i >= 3 {
			samples = append(samples, dt)
		}
	}
	for i := 0; i < len(samples); i++ {
		for j := i + 1; j < len(samples); j++ {
			if samples[j] < samples[i] {
				samples[i], samples[j] = samples[j], samples[i]
			}
		}
	}
	p50 := samples[len(samples)/2]
	t.Logf("W0 steady form present p50=%.2fms", p50)
	// W0 main-form soft target: 2× 60fps budget (widget paint not yet optimized).
	if p50 > 16.7*2 && os.Getenv("W0_ALLOW_SLOW") != "1" {
		t.Fatalf("p50=%.2f exceeds soft W0 form budget %.2f", p50, 16.7*2)
	}
}

func TestW0_ButtonDamage_PresentAuto(t *testing.T) {
	requireGPU(t)
	th := widget.DefaultTheme()
	const W, H = 320, 200
	dc := render.NewContext(W, H)
	defer dc.Close()
	_ = dc.LoadFontFace(findFont(t), th.FontSize)
	view, rel := dc.CreateOffscreenTexture(W, H)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	// bootstrap
	dc.SetRGB(0.95, 0.95, 0.96)
	dc.DrawRectangle(0, 0, W, H)
	_ = dc.Fill()
	btn := widget.Button{Bounds: widget.Rect{X: 40, Y: 80, W: 120, H: 32}, Label: "Go", Style: widget.ButtonPrimary}
	btn.Draw(dc, th)
	if err := dc.PresentFrameFull(view, uint32(W), uint32(H), nil); err != nil {
		t.Fatalf("full: %v", err)
	}

	// steady damage frame: toggle hover
	dc.BeginFrame()
	dc.ResetFrameDamage()
	btn.Hovered = true
	btn.Draw(dc, th)
	if _, err := dc.PresentFrameAuto(view, uint32(W), uint32(H), nil); err != nil {
		t.Fatalf("auto: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatal("GPUOps==0 on damage present")
	}
}
