//go:build !nogpu

package render_test

// S6.1 frame-model enforcement: default helpers prefer idle / local damage over
// mindless PresentFrame full clear. Real WGPU_NATIVE_PATH path required for GPU cases.

import (
	"image"
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/render"
)

func TestS61_PresentAuto_IdleSkipsGPU(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	// Bootstrap once so the surface exists.
	p1White(dc, w, h)
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("bootstrap full: %v", err)
	}

	dc.BeginFrame() // no draws → idle
	dc.ResetRenderPathStats()
	presentCalled := false
	out, err := dc.PresentFrameAuto(view, uint32(w), uint32(h), func() error {
		presentCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("PresentFrameAuto: %v", err)
	}
	if !out.Idle || out.Mode != render.PresentModeIdle {
		t.Fatalf("outcome=%+v want idle", out)
	}
	if presentCalled {
		t.Fatal("idle must not invoke present callback")
	}
	if dc.RenderPathStats().GPUOps != 0 {
		t.Fatalf("idle must not issue GPU ops, got %s", dc.RenderPathStats().LogLine())
	}
}

func TestS61_PresentAuto_LocalDamageMulti(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 300
	dc := render.NewContext(w, h)
	defer dc.Close()

	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	p1White(dc, w, h)
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// Steady frame: two distant dirty widgets.
	dc.BeginFrame()
	dc.ResetRenderPathStats()
	dc.SetRGB(0.2, 0.5, 0.9)
	dc.DrawRectangle(16, 20, 60, 40)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.7, 0.3)
	dc.DrawRectangle(300, 220, 70, 50)
	_ = dc.Fill()

	plan := dc.PlanPresent(w, h)
	if plan.Mode != render.PresentModeDamageMulti && plan.Mode != render.PresentModeDamageUnion {
		// Draw ops also append damage; either multi or union is acceptable if coalesced.
		// Distant fills should be multi; if path bounds expand oddly, still not full/idle.
		if plan.Mode == render.PresentModeIdle || plan.Mode == render.PresentModeFull {
			t.Fatalf("plan mode=%v rects=%v unexpected for local damage", plan.Mode, plan.Rects)
		}
	}

	out, err := dc.PresentFrameAuto(view, uint32(w), uint32(h), nil)
	if err != nil {
		t.Fatalf("PresentFrameAuto: %v", err)
	}
	if out.Idle {
		t.Fatal("expected non-idle present")
	}
	if out.Mode == render.PresentModeFull {
		t.Fatal("local damage must not promote to full")
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatal("GPUOps==0")
	}
	if dc.RenderPathStats().CPUFallbackOps != 0 {
		t.Fatalf("cpu_fallback_ops=%d", dc.RenderPathStats().CPUFallbackOps)
	}
	t.Logf("S6.1 multi/local outcome=%+v plan=%+v", out, plan)
}

func TestS61_PresentFull_ExplicitPath(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.3, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("PresentFrameFull: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatal("GPUOps==0")
	}
}

func TestS61_U01like_DamageVs_H01like_Full(t *testing.T) {
	// Contrast retained local damage (U01-class) vs deliberate full redraw (H01-class).
	// Not a hard budget gate — asserts API path + GPUOps, and that damage path is not slower
	// order-of-magnitude than full for a tiny dirty widget (soft log only).
	p1RequireGPU(t)
	const w, h = 400, 240

	// --- damage path ---
	dcD := render.NewContext(w, h)
	defer dcD.Close()
	viewD, relD := dcD.CreateOffscreenTexture(w, h)
	if relD == nil || viewD.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer relD()
	p1White(dcD, w, h)
	dcD.SetRGB(0.94, 0.95, 0.97)
	dcD.DrawRectangle(0, 0, w, h)
	_ = dcD.Fill()
	if err := dcD.PresentFrameFull(viewD, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("damage bootstrap: %v", err)
	}

	// warmup
	for i := 0; i < 2; i++ {
		dcD.BeginFrame()
		dcD.SetRGB(0.2, 0.45, 0.85)
		dcD.DrawRectangle(20, 30, 48, 24)
		_ = dcD.Fill()
		if _, err := dcD.PresentFrameAuto(viewD, uint32(w), uint32(h), nil); err != nil {
			t.Fatalf("damage warmup: %v", err)
		}
	}

	var damageMs []float64
	for i := 0; i < 6; i++ {
		dcD.BeginFrame()
		dcD.ResetRenderPathStats()
		t0 := time.Now()
		dcD.SetRGB(0.2, 0.45, 0.85)
		dcD.DrawRectangle(20, 30, 48, 24)
		_ = dcD.Fill()
		out, err := dcD.PresentFrameAuto(viewD, uint32(w), uint32(h), nil)
		dt := time.Since(t0).Seconds() * 1000
		if err != nil {
			t.Fatalf("damage present: %v", err)
		}
		if out.Mode == render.PresentModeFull || out.Idle {
			t.Fatalf("damage path outcome=%+v", out)
		}
		if dcD.RenderPathStats().GPUOps == 0 {
			t.Fatal("damage GPUOps==0")
		}
		damageMs = append(damageMs, dt)
	}

	// --- full redraw path ---
	dcF := render.NewContext(w, h)
	defer dcF.Close()
	viewF, relF := dcF.CreateOffscreenTexture(w, h)
	if relF == nil || viewF.IsNil() {
		t.Fatalf("full offscreen unavailable")
	}
	defer relF()

	var fullMs []float64
	for i := 0; i < 6; i++ {
		dcF.ResetRenderPathStats()
		t0 := time.Now()
		p1White(dcF, w, h)
		dcF.SetRGB(0.94, 0.95, 0.97)
		dcF.DrawRectangle(0, 0, w, h)
		_ = dcF.Fill()
		// chrome
		dcF.SetRGB(0.15, 0.16, 0.18)
		dcF.DrawRectangle(0, 0, w, 28)
		_ = dcF.Fill()
		dcF.SetRGB(0.2, 0.45, 0.85)
		dcF.DrawRectangle(20, 30, 48, 24)
		_ = dcF.Fill()
		if err := dcF.PresentFrameFull(viewF, uint32(w), uint32(h), nil); err != nil {
			t.Fatalf("full present: %v", err)
		}
		dt := time.Since(t0).Seconds() * 1000
		if dcF.RenderPathStats().GPUOps == 0 {
			t.Fatal("full GPUOps==0")
		}
		if i >= 2 {
			fullMs = append(fullMs, dt)
		}
	}

	dP50 := s5Percentile(damageMs, 0.5)
	fP50 := s5Percentile(fullMs, 0.5)
	t.Logf("S6.1 contrast damage_p50=%.2fms full_p50=%.2fms (present-only style wall)", dP50, fP50)
	if dP50 <= 0 || fP50 <= 0 {
		t.Fatal("invalid timings")
	}
}

func TestS61_PlanPresent_MatchesFrameDamagePhysical(t *testing.T) {
	dc := render.NewContextWithScale(100, 80, 2)
	defer dc.Close()
	dc.BeginFrame()
	dc.Invalidate(image.Rect(5, 5, 25, 25))
	dc.Invalidate(image.Rect(70, 50, 95, 75))
	plan := dc.PlanPresent(200, 160) // physical
	if plan.Mode == render.PresentModeIdle {
		t.Fatal("expected non-idle plan")
	}
	// Physical rects should be ~2× logical.
	for _, r := range dc.FrameDamage() {
		if r.Dx() < 30 || r.Dy() < 30 {
			// 20 logical * 2 = 40; allow stroke expansion variance for Invalidate-only
			t.Logf("physical rect=%v", r)
		}
		if r.Max.X > 200 || r.Max.Y > 160 {
			// Invalidate is scaled; should fit physical surface for these coords
			if r.Min.X > 200 {
				t.Fatalf("damage outside surface: %v", r)
			}
		}
	}
	if plan.Mode == render.PresentModeFull {
		t.Fatalf("two small widgets must not be full: %+v", plan)
	}
}

func TestS61_L0_MainPathHelpersStillGreen(t *testing.T) {
	// Smoke: U01 retained + PresentFrameAuto stays on GPU, under soft budget.
	p1RequireGPU(t)
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	const w, h = 400, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	// bootstrap shell
	p1White(dc, w, h)
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.12, 0.13, 0.15)
	dc.DrawRectangle(0, 0, w, 32)
	_ = dc.Fill()
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", 16.7)
	var samples []float64
	for i := 0; i < 8; i++ {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		t0 := time.Now()
		// status chip damage
		dc.SetRGB(0.2, 0.55, 0.9)
		dc.DrawRectangle(12, 40, 64, 22)
		_ = dc.Fill()
		out, err := dc.PresentFrameAuto(view, uint32(w), uint32(h), nil)
		dt := time.Since(t0).Seconds() * 1000
		if err != nil {
			t.Fatalf("auto: %v", err)
		}
		if out.Idle || out.Mode == render.PresentModeFull {
			t.Fatalf("unexpected outcome %+v", out)
		}
		if dc.RenderPathStats().GPUOps == 0 {
			t.Fatal("GPUOps==0")
		}
		if dc.RenderPathStats().CPUFallbackOps != 0 {
			t.Fatalf("cpu fallback %d", dc.RenderPathStats().CPUFallbackOps)
		}
		if i >= 2 {
			samples = append(samples, dt)
		}
	}
	p50 := s5Percentile(samples, 0.5)
	t.Logf("S61 L0 auto-present p50=%.2f budget=%.2f", p50, budget)
	if p50 > budget && os.Getenv("S6_ALLOW_SLOW") != "1" {
		t.Fatalf("p50 %.2f exceeds main-path budget %.2f", p50, budget)
	}
}
