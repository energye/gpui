// Command ui_wr_m5_hdr is the M5 HDR full-chain capability real window.
//
//	GPU card: real render.Context draws linear colors > 1 into an RGBA16Float
//	          offscreen target, tonemapped (exposure → Reinhard → sRGB) to 8bit.
//	CPU card: the same scene through hdr.FloatTarget + Encode (the reference).
//	Rows:     exposure 0.5 / 1 / 4, left GPU vs right CPU.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=6 go run ./examples/ui_wr_m5_hdr -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_m5_hdr -auto-only  # seed once
//
// Evidence: pixel probes compare GPU vs CPU at rect interiors (max diff <= 2) and
// require the HDR red to survive (> 200), a bitwise golden over header/legend/cards,
// and GPU route assertions (gpu_ops > 0, cpu_fallback_ops == 0).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/hdr"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 6
	windowTitle  = "gpui ui_wr_m5_hdr — M5 HDR 全链（16F → tonemap → 8bit）"

	cardW, cardH = 340, 150
)

var clearBG = [3]float64{0.06, 0.07, 0.09}

var exposures = []float64{0.5, 1, 4}

// Scene: three pixel-aligned rects with HDR colors (linear, >1 allowed).
var (
	sceneBG    = [4]float64{0.01, 0.012, 0.02, 1}
	sceneRects = []struct {
		x, y, w, h float64
		r, g, b, a float64
	}{
		{12, 16, 100, 118, 2.0, 0.5, 0.25, 1},
		{120, 16, 100, 118, 0.25, 1.6, 3.0, 1},
		{228, 16, 100, 118, 0.1, 0.5, 4.0, 1},
	}
	// Probe points inside each rect (well away from AA edges).
	sceneProbes = [][2]float64{{62, 75}, {170, 75}, {278, 75}}
)

type hdrCard struct {
	ctx     *render.Context
	view    render.TextureView
	release func()
	ref     render.TextureRef
}

var (
	gpuCards [3]*hdrCard
	cpuCards [3]*render.ImageBuf
	baked    bool
	gpuOps   int
	cpuFb    int
)

func drawScene(dc *render.Context) {
	dc.SetRGBA(sceneBG[0], sceneBG[1], sceneBG[2], sceneBG[3])
	dc.DrawRectangle(0, 0, cardW, cardH)
	_ = dc.Fill()
	for _, r := range sceneRects {
		dc.SetRGBA(r.r, r.g, r.b, r.a)
		dc.DrawRectangle(r.x, r.y, r.w, r.h)
		_ = dc.Fill()
	}
}

// cpuCard renders the same scene through the CPU HDR target + Encode.
func cpuCard(exposure float64) *render.ImageBuf {
	t, err := hdr.NewFloatTarget(cardW, cardH)
	if err != nil {
		return nil
	}
	for y := 0; y < cardH; y++ {
		for x := 0; x < cardW; x++ {
			c := [4]float32{float32(sceneBG[0]), float32(sceneBG[1]), float32(sceneBG[2]), 1}
			for _, r := range sceneRects {
				if float64(x) >= r.x && float64(x) < r.x+r.w && float64(y) >= r.y && float64(y) < r.y+r.h {
					c = [4]float32{float32(r.r), float32(r.g), float32(r.b), 1}
				}
			}
			if !t.Set(x, y, c) {
				return nil
			}
		}
	}
	img, err := t.Encode(exposure)
	if err != nil {
		return nil
	}
	return render.ImageBufFromImage(img)
}

func ensureBaked() {
	if baked {
		return
	}
	baked = true
	for i, exp := range exposures {
		dc := render.NewContext(cardW, cardH)
		drawScene(dc)
		stats := dc.RenderPathStats()
		gpuOps += stats.GPUOps
		cpuFb += stats.CPUFallbackOps
		view, rel, err := dc.CreateOffscreenTextureFormat(cardW, cardH, types.TextureFormatRGBA16Float)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_m5_hdr: create 16F target (exp %.2f): %v\n", exp, err)
			_ = dc.Close()
			continue
		}
		if err := dc.FlushGPUWithView(view, cardW, cardH); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_m5_hdr: flush 16F (exp %.2f): %v\n", exp, err)
			rel()
			_ = dc.Close()
			continue
		}
		toned, relTone, err := dc.TonemapView(view, cardW, cardH, exp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_m5_hdr: tonemap (exp %.2f): %v\n", exp, err)
			rel()
			_ = dc.Close()
			continue
		}
		rel()
		card := &hdrCard{ctx: dc, view: toned, release: relTone}
		card.ref = render.RegisterTextureRef(func() render.TextureView { return card.view })
		gpuCards[i] = card
		cpuCards[i] = cpuCard(exp)
	}
}

func buildScene(shell *wrkit.ShellChrome) {
	for row, exp := range exposures {
		y := 24 + float64(row)*170
		leftX := 12.0
		rightX := 12 + cardW + 16.0

		shell.Body.LabelAt(fmt.Sprintf("GPU  exp %.2f   (RGBA16Float → tonemap)", exp), 11, leftX, y-14, 0.86, 0.90, 0.96)
		shell.Body.LabelAt(fmt.Sprintf("CPU  exp %.2f   (hdr.FloatTarget + Encode)", exp), 11, rightX, y-14, 0.86, 0.90, 0.96)

		gpuBox := rendering.NewRenderBox()
		gpuBox.FixedWidth, gpuBox.FixedHeight = cardW, cardH
		r := row
		gpuBox.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ensureBaked()
			c := gpuCards[r]
			if c == nil || !c.ref.Valid() {
				return
			}
			pc.DC.DrawTextureRef(c.ref, pc.OriginX, pc.OriginY, cardW, cardH)
		}
		shell.Body.Place(gpuBox, leftX, y)

		cpuBox := rendering.NewRenderBox()
		cpuBox.FixedWidth, cpuBox.FixedHeight = cardW, cardH
		cpuBox.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ensureBaked()
			img := cpuCards[r]
			if img == nil {
				return
			}
			pc.DC.DrawImage(img, pc.OriginX, pc.OriginY)
		}
		shell.Body.Place(cpuBox, rightX, y)
	}
}

type fixedTick struct{ on func(dt float64) }

func (t *fixedTick) Tick(dt float64) bool { t.on(dt); return true }

func loadImage(p string) image.Image {
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

func atRGB(img image.Image, x, y int) (byte, byte, byte) {
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return 0, 0, 0
	}
	r, g, b, _ := img.At(x, y).RGBA()
	return byte(r >> 8), byte(g >> 8), byte(b >> 8)
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()
	gateMode := *autoOnly
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	} else {
		wrkit.RequireMinRun(secs, "M5")
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: windowTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "M5 HDR 全链 — RGBA16Float 离屏面 → tonemap → 8bit 呈现", []string{
		"左：真实 render.Context 画线性 HDR 色（2.0 / 1.6 / 4.0）进 16F 面，再 tonemap",
		"右：同一场景走 hdr.FloatTarget + Encode（CPU 参考）",
		"三行曝光 0.5 / 1 / 4；像素断言 = 矩形内部 GPU vs CPU（max ≤2）",
		"门禁: gpu_ops>0 + cpu_fb=0 + 金图逐位",
	})
	buildScene(shell)

	snapDir := filepath.Join("examples", "ui_wr_m5_hdr", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "m5_hdr_final.png")

	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	elapsed := 0.0
	app.Scheduler().Tickers().Add(&fixedTick{on: func(dt float64) {
		elapsed += dt
		app.ScheduleFrame()
		snapH := app.Metrics().Snapshot()
		shell.UpdateHUD("M5-HDR", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("cards=6 t=%.1f", elapsed))
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
	app.Close()
	win.Close()
	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	img := loadImage(snapPath)
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y

	pixelOK := 0
	pixelTotal := 0
	for row := 0; row < len(exposures); row++ {
		y := by + 24 + float64(row)*170
		for i, p := range sceneProbes {
			left := bx + 12 + p[0]
			right := bx + 12 + cardW + 16 + p[0]
			py := y + p[1]
			ok := false
			detail := "no snapshot"
			if img != nil {
				lr, lg, lb := atRGB(img, int(left*dpr), int(py*dpr))
				rr, rg, rb := atRGB(img, int(right*dpr), int(py*dpr))
				d := maxAbs(int(lr)-int(rr), maxAbs(int(lg)-int(rg), int(lb)-int(rb)))
				ok = d <= 2
				detail = fmt.Sprintf("gpu=%d/%d/%d cpu=%d/%d/%d maxdiff=%d", lr, lg, lb, rr, rg, rb, d)
				// HDR red (2.0) must beat the 1.0 clamp (about 188).
				if row != 0 && i == 0 && lr <= 200 {
					ok = false
					detail += " (HDR red clamped?)"
				}
			}
			pixelTotal++
			if ok {
				pixelOK++
			}
			fmt.Fprintf(os.Stderr, "ui_wr_m5_hdr: pixel exp=%.2f rect=%d ok=%v %s\n", exposures[row], i, ok, detail)
		}
	}

	goldenRects := []wrkit.GoldenRect{
		{X: 0, Y: 0, W: winW, H: 48},
		{X: shell.Legend.X, Y: shell.Legend.Y, W: shell.Legend.W, H: shell.Legend.H},
	}
	for row := 0; row < len(exposures); row++ {
		y := by + 24 + float64(row)*170
		goldenRects = append(goldenRects,
			wrkit.GoldenRect{X: bx + 12, Y: y, W: cardW, H: cardH},
			wrkit.GoldenRect{X: bx + 12 + cardW + 16, Y: y, W: cardW, H: cardH},
		)
	}
	baseName := "m5_hdr_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		baseName = "m5_hdr_final_go.png"
	}
	basePath := filepath.Join(snapDir, "goldens", baseName)
	diffPct, totalPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, goldenRects)

	offscreenFallbacks := uint64(0)
	for _, c := range gpuCards {
		if c != nil {
			offscreenFallbacks += c.ctx.OffscreenFormatFallbacks()
		}
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "M5-HDR",
		Scenario:      "ui_wr_m5_hdr",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"fps_gate":                "off_correctness",
			"slope_gate":              "off",
			"apis":                    "Context.CreateOffscreenTextureFormat,Context.TonemapView,Context.FlushGPUWithView,Canvas.DrawTextureRef",
			"exposures":               exposures,
			"hdr_gpu_ops":             gpuOps,
			"hdr_cpu_fallback_ops":    cpuFb,
			"offscreen_fmt_fallbacks": offscreenFallbacks,
			"pixel_probes_ok":         pixelOK,
			"pixel_probes_total":      pixelTotal,
			"pixel_golden_diff_pct":   diffPct,
			"pixel_golden_total_px":   totalPx,
			"pixel_golden_first_run":  seeded,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if pixelOK != pixelTotal || pixelTotal == 0 {
		fail("pixel probes %d/%d ok", pixelOK, pixelTotal)
	}
	if goldenErr != nil {
		fail("golden: %v", goldenErr)
	} else if seeded {
		fail("golden seeded from this run — not a pass; review + commit the baseline, then re-run")
	} else if diffPct != 0 {
		fail("pixel_golden_diff_pct=%.4f want 0 over %d px", diffPct, totalPx)
	}
	if gpuOps == 0 {
		fail("gpu_ops=0: HDR draws did not ride the GPU")
	}
	if cpuFb != 0 {
		fail("cpu_fallback_ops=%d: HDR path fell back to CPU", cpuFb)
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
		}); err != nil {
			fail("%v", err)
		}
	}

	for _, c := range gpuCards {
		if c != nil {
			c.release()
			_ = c.ctx.Close()
		}
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_m5_hdr: OK presents=%d pixel=%d/%d golden=%.4f%% gpu_ops=%d elapsed=%.1fs\n",
		app.PresentCount(), pixelOK, pixelTotal, diffPct, gpuOps, elapsedSec)
}

func maxAbs(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if a > b {
		return a
	}
	return b
}
