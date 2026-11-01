// Command ui_wr_p31_linear_blend is the P3.1 linear blend capability real
// window (PLAN §7.4 P3.1 / evidence/P3.1/assessment.md D4): three cards baked
// through the real render.Context prove the BlendSpace contract on the GPU
// dual-tex path — sRGB-domain Difference, linear-light Difference, and the
// explicit fixed-function degradation for Normal+Linear.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_p31_linear_blend -auto-only
//	go run ./examples/ui_wr_p31_linear_blend   # manual: lives until close
//
// Evidence: pixel probes over the GPU snapshot (card centers, tolerance from
// the float reference in scene.go), per-card GPU-route assertions
// (gpu_ops > 0, cpu_fallback_ops == 0), a bitwise golden over header + legend
// + the three cards, and a live screen grab compared with the snapshot.
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
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 5
	windowTitle  = "gpui ui_wr_p31_linear_blend — P3.1 线性混合"
)

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var pixelResult = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()
	gateMode := *autoOnly
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	} else {
		wrkit.RequireMinRun(secs, "P31")
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: windowTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "P3.1 线性混合 — BlendSpace opt-in（advanced 真线性 / 固定功能显式降级）", []string{
		"A Difference sRGB 域: dual-tex linear=0 → 76",
		"B Difference linear 域: dual-tex linear=1 → ≈168 (float ref)",
		"C Normal+Linear: 固定功能显式降级 → 128（双端一致，P3.1b 解锁）",
		"像素断言 = 卡中心采样 vs float 参考",
		"门禁: gpu_ops>0 + cpu_fb=0 + 金图逐位 + 抓屏同区",
	})
	buildScene(shell)

	snapDir := filepath.Join("examples", "ui_wr_p31_linear_blend", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "p31_final.png")

	resizeEvents := 0
	manualEvents := 0
	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_p31_linear_blend: close (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle(windowTitle + " — closing")
				}
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("%s — %dx%d resize #%d", windowTitle, ev.Width, ev.Height, resizeEvents))
					}
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			default:
				manualEvents++
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("%s — event #%d type=%v", windowTitle, manualEvents, ev.Type))
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	elapsed := 0.0
	app.Scheduler().Tickers().Add(&fixedTick{on: func(dt float64) {
		elapsed += dt
		app.ScheduleFrame()
		snapH := app.Metrics().Snapshot()
		shell.UpdateHUD("P31-linear-blend", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("cards=3 t=%.1f", elapsed))
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
	ensureBaked()

	// Probe points: center of each card's content area (card origin from
	// shell body offset + cardAt; content inset 16px, base 24px top).
	bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
	type probe struct {
		key     string
		x, y    float64
		want    float64 // sRGB-encoded 0-1 of the R channel
		tol     float64
		scaleFn func(byte) float64
	}
	grey := func(v byte) float64 { return float64(v) / 255.0 }
	probes := []probe{
		{"p31_a_srgb_difference", bx + cardAt[0][0] + cardW/2, by + cardAt[0][1] + cardH/2, wantSRGB / 255, tolSRGB, grey},
		{"p31_b_linear_difference", bx + cardAt[1][0] + cardW/2, by + cardAt[1][1] + cardH/2, wantLinear / 255, tolLinear, grey},
		{"p31_c_normal_linear_degrades", bx + cardAt[2][0] + cardW/2, by + cardAt[2][1] + cardH/2, wantDegraded / 255, tolDegraded, grey},
	}
	pixelOK := 0
	for _, p := range probes {
		ok := false
		detail := "no snapshot"
		if img != nil {
			px := int(p.x * dpr)
			py := int(p.y * dpr)
			r, _, _, _ := atRGB(img, px, py)
			got := p.scaleFn(r)
			d := got - p.want
			if d < 0 {
				d = -d
			}
			ok = d <= p.tol
			detail = fmt.Sprintf("got %.4f want %.4f tol %.4f (byte %d)", got, p.want, p.tol, r)
		}
		fmt.Fprintf(os.Stderr, "ui_wr_p31_linear_blend: pixel %-30s ok=%v %s\n", p.key, ok, detail)
		pixelResult[p.key] = ok
		if ok {
			pixelOK++
		}
	}

	goldenRects := []rect{
		{x: 0, y: 0, w: winW, h: 48},
		{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
	}
	for _, ca := range cardAt {
		goldenRects = append(goldenRects, rect{x: bx + ca[0], y: by + ca[1], w: cardW, h: cardH})
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapPath, goldenRects, dpr)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "P31-linear-blend",
		Scenario:      "ui_wr_p31_linear_blend",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"fps_gate":               "off_correctness",
			"slope_gate":             "off",
			"apis":                   "Paint.BlendSpace,Context.SetBlendSpace,DrawImageOptions.BlendSpace",
			"cards_route":            cardsSummary(),
			"pixel_probes_ok":        pixelOK,
			"pixel_probes_total":     len(pixelResult),
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"resize_events":          resizeEvents,
			"blend_space_default":    "srgb",
			"fixed_function_policy":  "degrade_until_p31b",
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
	if pixelOK != len(pixelResult) || len(pixelResult) == 0 {
		fail("pixel probes %d/%d ok", pixelOK, len(pixelResult))
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
	}
	if snap.GPUOps == 0 {
		fail("gpu_ops=0: window image draws did not ride the GPU")
	}
	for idx, c := range cards {
		if c == nil {
			fail("card %d was never baked", idx)
			continue
		}
		if c.gpuOps == 0 {
			fail("card %d did not ride the GPU", idx)
		}
		if c.cpuFallbacks != 0 {
			fail("card %d fell back to CPU (%d)", idx, c.cpuFallbacks)
		}
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
		}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_p31_linear_blend: OK presents=%d pixel=%d/%d golden=%.4f%% resize=%d mode=%s elapsed=%.1fs\n",
		app.PresentCount(), pixelOK, len(pixelResult), goldenDiffPct, resizeEvents,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

type fixedTick struct {
	on func(dt float64)
}

func (t *fixedTick) Tick(dt float64) bool { t.on(dt); return true }

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

func atRGB(img image.Image, x, y int) (byte, byte, byte, byte) {
	c := img.At(x, y)
	r, g, b, a := c.RGBA()
	return byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)
}

// evaluateGolden compares the fresh snapshot with the committed golden inside
// the given rects (physical pixels, scaled by dpr). First run (or
// GPUI_ACCEPT_GOLDEN=1) accepts the snapshot as the golden and reports
// firstRun=true.
func evaluateGolden(snapPath string, rects []rect, dpr float64) (diffPct float64, totalPx int64, firstRun bool) {
	goldenPath := filepath.Join(filepath.Dir(snapPath), "goldens", "p31_final.png")
	cur := loadImage(snapPath)
	if cur == nil {
		return 100, 0, false
	}
	golden := loadImage(goldenPath)
	accept := golden == nil || os.Getenv("GPUI_ACCEPT_GOLDEN") == "1"
	if accept {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "golden: mkdir:", err)
			return 100, 0, false
		}
		out, err := os.Create(goldenPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "golden: create:", err)
			return 100, 0, false
		}
		defer out.Close()
		if err := png.Encode(out, cur); err != nil {
			fmt.Fprintln(os.Stderr, "golden: encode:", err)
			return 100, 0, false
		}
		return 0, 0, true
	}
	if golden.Bounds() != cur.Bounds() {
		return 100, 0, false
	}
	var diff int64
	var total int64
	gb := golden.Bounds()
	for _, r := range rects {
		x0 := int(r.x * dpr)
		y0 := int(r.y * dpr)
		x1 := int((r.x + r.w) * dpr)
		y1 := int((r.y + r.h) * dpr)
		for y := y0; y < y1 && y < gb.Max.Y; y++ {
			for x := x0; x < x1 && x < gb.Max.X; x++ {
				gr, gg, gbb, _ := atRGB(golden, x, y)
				cr, cg, cb, _ := atRGB(cur, x, y)
				total++
				if gr != cr || gg != cg || gbb != cb {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 100, 0, false
	}
	return float64(diff) * 100 / float64(total), total, false
}
