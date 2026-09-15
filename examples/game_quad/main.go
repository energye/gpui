// Command game_quad is the R1 (1.4) independent real window: trapezoid alignment.
//
// Quad corners are user-space TL,TR,BR,BL. GPU splits TL-TR-BL + TR-BR-BL.
// CPU uses the same split for pixel parity (R1). Degenerate quads no-op,
// Ex variant returns sentinel errors instead of drawing wrong.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/game_quad
//
// Window: 1200x800. RUN_SECONDS>=5 auto gate, unset runs resident for manual view.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R1")
	}
	wrkit.EnsureUIFace()

	// F evidence first: offscreen CPU/GPU parity before the window session
	// starts (post-close offscreen GPU draws lose depth and paint white).
	parity := quadParityOffscreen()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui game_quad — R1 梯形贴图对齐", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R1 梯形贴图 — 左Nearest右Bilinear, 白边=非包络证明", []string{
		"TL红 TR绿 BL蓝 BR黄 (8x8四象限)",
		"梯形上窄下宽, 白边=CPU非包络",
		"左Nearest精确, 右Bilinear平滑",
		"白边消失=退化成方块=FAIL",
		"JSON看parity_changed_pct<=1",
	})

	src := makeQuadSrc()

	// Two quad views side by side inside body.
	quadL := rendering.NewRenderBox()
	quadL.FixedWidth, quadL.FixedHeight = 340, 220
	quadL.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintQuadCard(pc, src, render.InterpNearest, "Nearest 精确")
	}
	shell.Body.Place(quadL, 20, 20)

	quadR := rendering.NewRenderBox()
	quadR.FixedWidth, quadR.FixedHeight = 340, 220
	quadR.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintQuadCard(pc, src, render.InterpBilinear, "Bilinear 平滑")
	}
	shell.Body.Place(quadR, 380, 20)

	note := wrkit.Label("上窄下宽梯形, 外圈白边必须留住", 12, 0.75, 0.82, 0.9)
	shell.Body.Place(note, 20, 260)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "game_quad: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		quadL.MarkNeedsPaint()
		quadR.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0
		shell.UpdateHUD("R1", "Steady", app, gateOK,
			fmt.Sprintf("presents=%d", app.PresentCount()), "trap-not-box")
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R1",
		Scenario:      "game_quad",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         parity,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if v, ok := numExtra(parity, "parity_changed_pct"); !ok || v > 1.0 {
		fmt.Fprintf(os.Stderr, "FAIL: parity_changed_pct=%v want <=1 (CPU/GPU diverge)\n", parity["parity_changed_pct"])
		os.Exit(1)
	}
	if v, ok := numExtra(parity, "outside_white"); !ok || v != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: outside_white=%v want 1 (bbox fallback?)\n", parity["outside_white"])
		os.Exit(1)
	}
	if v, ok := numExtra(parity, "golden_changed_pct"); !ok || v > 1.0 {
		fmt.Fprintf(os.Stderr, "FAIL: golden_changed_pct=%v want <=1\n", parity["golden_changed_pct"])
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "game_quad: OK presents=%d parity=%v outside=1 golden=%v elapsed=%.1fs\n",
		app.PresentCount(), parity["parity_changed_pct"], parity["golden_changed_pct"], elapsed)
}

func numExtra(m map[string]any, k string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[k]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func makeQuadSrc() *render.ImageBuf {
	img, _ := render.NewImageBuf(8, 8, render.FormatRGBA8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			var r, g, b uint8
			switch {
			case x < 4 && y < 4:
				r, g, b = 255, 0, 0
			case x >= 4 && y < 4:
				r, g, b = 0, 255, 0
			case x < 4 && y >= 4:
				r, g, b = 0, 0, 255
			default:
				r, g, b = 255, 255, 0
			}
			_ = img.SetRGBA(x, y, r, g, b, 255)
		}
	}
	return img
}

func paintQuadCard(pc *rendering.PaintContext, src *render.ImageBuf, interp render.InterpolationMode, tag string) {
	if pc == nil || pc.DC == nil {
		return
	}
	// Card white background so outside-trapezoid white edge is visible.
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(1, 1, 1)
	pc.DC.DrawRectangle(ax, ay, 340, 220)
	_ = pc.DC.Fill()
	// Trapezoid inside card: top narrow, bottom wide.
	corners := [4]render.Point{
		{X: ax + 120, Y: ay + 20},
		{X: ax + 220, Y: ay + 20},
		{X: ax + 300, Y: ay + 180},
		{X: ax + 40, Y: ay + 180},
	}
	_ = pc.DC.DrawImageQuadEx(src, corners, render.QuadDrawOptions{
		Interpolation: interp,
		Opacity:       1,
		BlendMode:     render.BlendNormal,
	})
	if face := wrkit.FaceAt(12); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(0.1, 0.12, 0.15, 1)
	pc.DC.DrawString(tag, ax+12, ay+208)
}

// quadParityOffscreen renders CPU vs GPU trapezoid offscreen in this process.
// Runs BEFORE the window opens (caller orders it first): after the window
// closes the shared GPU session is torn down and fresh offscreen GPU draws
// lose depth (1x1 fallback) and paint white, which is a session artifact,
// not an R1 parity failure.
func quadParityOffscreen() map[string]any {
	out := map[string]any{}
	srcCPU := makeQuadSrc()
	srcGPU := makeQuadSrc()
	corners := [4]render.Point{{X: 40, Y: 10}, {X: 60, Y: 10}, {X: 80, Y: 50}, {X: 20, Y: 50}}
	// CPU.
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	dcCPU := render.NewContext(100, 60)
	dcCPU.ClearWithColor(render.White)
	_ = dcCPU.DrawImageQuadEx(srcCPU, corners, render.QuadDrawOptions{Interpolation: render.InterpBilinear})
	cpuImg := dcCPU.Image()
	cpuPix := image.NewRGBA(cpuImg.Bounds())
	copy(cpuPix.Pix, cpuImg.(*image.RGBA).Pix)
	dcCPU.Close()
	// GPU.
	_ = os.Unsetenv("GOGPU_RENDER_MODE")
	dcGPU := render.NewContext(100, 60)
	dcGPU.ClearWithColor(render.White)
	_ = dcGPU.DrawImageQuadEx(srcGPU, corners, render.QuadDrawOptions{Interpolation: render.InterpBilinear})
	_ = dcGPU.FlushGPU()
	gpuOps := dcGPU.RenderPathStats().GPUOps
	gpuImg := dcGPU.Image()
	gpuPix := image.NewRGBA(gpuImg.Bounds())
	copy(gpuPix.Pix, gpuImg.(*image.RGBA).Pix)
	dcGPU.Close()
	changed, total, mean := diffImages(cpuPix, gpuPix)
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(changed) / float64(total)
	}
	out["parity_changed_pct"] = pct
	out["parity_mean_abs"] = mean
	out["parity_gpu_ops"] = gpuOps
	// Outside bbox corner must stay white (not bbox fallback).
	r, g, b, _ := cpuImg.At(22, 12).RGBA()
	out["outside_white"] = 0
	if uint8(r>>8) == 255 && uint8(g>>8) == 255 && uint8(b>>8) == 255 {
		out["outside_white"] = 1
	}
	// Golden vs frozen window golden.
	out["golden_changed_pct"] = 100.0
	if f, err := os.Open("examples/game_quad/testdata/quad_window_golden.png"); err == nil {
		if want, err := png.Decode(f); err == nil {
			c2, t2, m2 := diffImages(cpuImg, want)
			if t2 > 0 {
				out["golden_changed_pct"] = 100 * float64(c2) / float64(t2)
			}
			out["golden_mean_abs"] = m2
		}
		_ = f.Close()
	}
	return out
}

func diffImages(a, b image.Image) (changed, total int, mean float64) {
	ab, bb := a.Bounds(), b.Bounds()
	if !ab.Eq(bb) {
		return 1, 1, 255
	}
	var sum float64
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			ar, ag, ab2, _ := a.At(x, y).RGBA()
			br, bg, bb2, _ := b.At(x, y).RGBA()
			ds := [3]int{int(ar>>8) - int(br>>8), int(ag>>8) - int(bg>>8), int(ab2>>8) - int(bb2>>8)}
			hit := false
			for _, d := range ds {
				if d < 0 {
					d = -d
				}
				sum += float64(d)
				if d > 2 {
					hit = true
				}
			}
			if hit {
				changed++
			}
			total++
		}
	}
	if total > 0 {
		mean = sum / float64(total*3)
	}
	return changed, total, mean
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
