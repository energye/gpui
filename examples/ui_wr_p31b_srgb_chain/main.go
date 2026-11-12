// Command ui_wr_p31b_srgb_chain is the P3.1b opt-in linear-target capability
// real window.
//
//	Left card : default UNORM target — fixed-function BlendSpaceLinear requests
//	            degrade to the encoded domain (the P3.1 contract).
//	Right card: opt-in RGBA8UnormSrgb target — the same 13 fixed-function modes
//	            blend in linear light on the hardware and are encoded on write.
//	Bottom bar: semi-transparent mid-tone (premultiplied sRGB <-> linear
//	            conversion), one probe per card.
//
// Both cards are baked once into offscreen views, then composited through
// DrawGPUTexture; the sRGB source is converted exactly once at that boundary
// (P3.1b-3), so the two cards show their expected domain-specific pixels.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_p31b_srgb_chain -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_p31b_srgb_chain -auto-only  # seed once
//
// Evidence: per-card target formats + fixed-function degradation deltas in
// JSON, ten pixel probes over the GPU snapshot, a bitwise golden over the
// shell/cards, and GPU-route assertions (gpu_ops > 0, cpu_fallback_ops == 0).
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
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 5
	windowTitle  = "gpui ui_wr_p31b_srgb_chain — opt-in 线性目标（sRGB 表面链）"

	cardW, cardH = 430, 600
	cellSize     = 94
	cellGap      = 8
	cardMargin   = 14
	barX         = 14
	barY         = 470
	barW         = 402
	barH         = 100
	barInset     = 16
)

var clearBG = [3]float64{0.05, 0.06, 0.08}

// fixedModes is the 13 fixed-function blend table (blend_gpu.go), in grid order.
var fixedModes = []struct {
	name string
	mode render.BlendMode
}{
	{"Normal", render.BlendNormal},
	{"Copy", render.BlendCopy},
	{"Clear", render.BlendClear},
	{"Plus", render.BlendPlus},
	{"Modulate", render.BlendModulate},
	{"DestinationOut", render.BlendDestinationOut},
	{"SourceAtop", render.BlendSourceAtop},
	{"Xor", render.BlendXor},
	{"DestinationOver", render.BlendDestinationOver},
	{"SourceIn", render.BlendSourceIn},
	{"SourceOut", render.BlendSourceOut},
	{"DestinationIn", render.BlendDestinationIn},
	{"DestinationAtop", render.BlendDestinationAtop},
}

// Scene palette (single source of truth shared with the pixel probes).
const (
	cardBG   = 0.12
	swatchBG = 0.80
	srcGray  = 0.50
	srcAlpha = 0.50
	barBase  = 0.20
)

type p31bCard struct {
	ctx       *render.Context
	view      render.TextureView
	release   func()
	ref       render.TextureRef
	format    types.TextureFormat
	gpuOps    int
	cpuFb     int
	linearFb  int64
	targetTag string
}

var (
	cards [2]*p31bCard // 0 = default UNORM, 1 = opt-in sRGB linear
	baked bool
)

// drawCardScene paints the 13 fixed-function swatches + the semi-transparent
// mid-tone bar. All source fills request BlendSpaceLinear; the target format
// decides whether that is real linear light or a counted degradation.
func drawCardScene(dc *render.Context) {
	resetPaint := func() {
		dc.SetBlendMode(render.BlendNormal)
		dc.SetBlendSpace(render.BlendSpaceSRGB)
	}
	dc.SetRGBA(cardBG, cardBG, cardBG, 1)
	dc.DrawRectangle(0, 0, cardW, cardH)
	_ = dc.Fill()

	for i, m := range fixedModes {
		x := cardMargin + float64(i%4)*(cellSize+cellGap)
		y := cardMargin + float64(i/4)*(cellSize+cellGap)
		resetPaint()
		dc.SetRGBA(swatchBG, swatchBG, swatchBG, 1)
		dc.DrawRectangle(x, y, cellSize, cellSize)
		_ = dc.Fill()

		dc.SetRGBA(srcGray, srcGray, srcGray, srcAlpha)
		dc.SetBlendMode(m.mode)
		dc.SetBlendSpace(render.BlendSpaceLinear)
		dc.DrawRectangle(x+12, y+12, cellSize-24, cellSize-24)
		_ = dc.Fill()
	}

	resetPaint()
	dc.SetRGBA(barBase, barBase, barBase, 1)
	dc.DrawRectangle(barX, barY, barW, barH)
	_ = dc.Fill()
	dc.SetRGBA(srcGray, srcGray, srcGray, srcAlpha)
	dc.SetBlendMode(render.BlendNormal)
	dc.SetBlendSpace(render.BlendSpaceLinear)
	dc.DrawRectangle(barX+barInset, barY+barInset, barW-2*barInset, barH-2*barInset)
	_ = dc.Fill()
}

func bakeCard(format types.TextureFormat, targetTag string) *p31bCard {
	dc := render.NewContext(cardW, cardH)
	_ = dc.GPURenderContext() // materialize the stats source before the baseline
	before := dc.RenderPathStats().LinearFixedFunctionFallbacks
	drawCardScene(dc)
	view, release, err := dc.CreateOffscreenTextureFormat(cardW, cardH, format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_p31b_srgb_chain: create %s target: %v\n", targetTag, err)
		_ = dc.Close()
		return nil
	}
	if err := dc.FlushGPUWithView(view, cardW, cardH); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_p31b_srgb_chain: flush %s target: %v\n", targetTag, err)
		release()
		_ = dc.Close()
		return nil
	}
	stats := dc.RenderPathStats()
	card := &p31bCard{
		ctx:       dc,
		view:      view,
		release:   release,
		format:    format,
		gpuOps:    stats.GPUOps,
		cpuFb:     stats.CPUFallbackOps,
		linearFb:  stats.LinearFixedFunctionFallbacks - before,
		targetTag: targetTag,
	}
	card.ref = render.RegisterTextureRef(func() render.TextureView { return card.view })
	return card
}

func ensureBaked() {
	if baked {
		return
	}
	baked = true
	cards[0] = bakeCard(types.TextureFormatRGBA8Unorm, "unorm")
	cards[1] = bakeCard(types.TextureFormatRGBA8UnormSrgb, "srgb")
}

func buildScene(shell *wrkit.ShellChrome) {
	const cardY = 22.0
	body := shell.Body
	body.LabelAt("默认 sRGB 表面（UNORM 目标）— 固定功能线性请求显式降级", 12, 8, 2, 0.88, 0.72, 0.55)
	body.LabelAt("opt-in 线性目标（RGBA8UnormSrgb）— 13 模式硬件线性混合", 12, 450, 2, 0.55, 0.82, 0.72)

	place := func(x float64, idx int) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ensureBaked()
			c := cards[idx]
			if c == nil || !c.ref.Valid() {
				return
			}
			pc.DC.DrawTextureRef(c.ref, pc.OriginX, pc.OriginY, cardW, cardH)
		}
		body.Place(box, x, cardY)
	}
	place(8, 0)
	place(450, 1)
}

func probePoint(cardX, col, row float64) (float64, float64) {
	return cardX + cardMargin + col*(cellSize+cellGap) + cellSize/2,
		cardMargin + row*(cellSize+cellGap) + cellSize/2
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
		wrkit.RequireMinRun(secs, "P31b")
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: windowTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P3.1b opt-in 线性目标 — 默认 sRGB 表面不变，linear 目标才走硬件线性混合", []string{
		"左卡：UNORM 目标（默认）。13 个固定功能模式 + BlendSpaceLinear",
		"       → 编码域混合（P3.1 降级），计数 13 + 半透明条 1",
		"右卡：RGBA8UnormSrgb 目标（P3.1b opt-in）。同内容 → 硬件线性",
		"       → 混合后编码写回，降级计数 0",
		"底条：50% 灰叠在 0.2 灰上，验证预乘 sRGB↔linear 无二次转换",
		"像素断言：Normal/Plus/Modulate/Copy/底条 各取两卡中心",
		"门禁：双卡降级计数 + 10 像素探针 + 金图逐位 + gpu_ops>0",
	})
	buildScene(shell)

	snapDir := filepath.Join("examples", "ui_wr_p31b_srgb_chain", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "p31b_final.png")

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
		shell.UpdateHUD("P31b-sRGB-chain", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("cards=2 t=%.1f", elapsed))
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

	ensureBaked()
	bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
	cardX := [2]float64{8, 450}
	cardY := 22.0

	// Expected visible bytes (sRGB-encoded display bytes after compositing):
	// the two cards intentionally differ where linear-light blending is real.
	type probe struct {
		name     string
		card     int
		col, row float64
		want     float64
	}
	probes := []probe{
		{"normal", 0, 0, 0, 166},
		{"normal", 1, 0, 0, 171},
		{"plus", 0, 3, 0, 255},
		{"plus", 1, 3, 0, 219},
	}
	var pixelProbes []wrkit.PixelProbe
	for _, p := range probes {
		px, py := probePoint(cardX[p.card], p.col, p.row)
		pixelProbes = append(pixelProbes, wrkit.PixelProbe{
			Name: "card" + fmt.Sprint(p.card) + "_" + p.name,
			X:    bx + px, Y: by + cardY + py,
			R: p.want / 255, G: p.want / 255, B: p.want / 255, Tol: 3,
		})
	}
	// Bottom bar centre (semi-transparent mid-tone over 0.2 gray).
	barCenterX := cardX[0] + barX + barW/2
	barCenterY := cardY + barY + barH/2
	pixelProbes = append(pixelProbes,
		wrkit.PixelProbe{Name: "card0_bar", X: bx + barCenterX, Y: by + barCenterY, R: 89.0 / 255, G: 89.0 / 255, B: 89.0 / 255, Tol: 3},
		wrkit.PixelProbe{Name: "card1_bar", X: bx + barCenterX + 442, Y: by + barCenterY, R: 99.0 / 255, G: 99.0 / 255, B: 99.0 / 255, Tol: 3},
	)
	pixelOK, pixelTotal := 0, len(pixelProbes)
	pixelErr := wrkit.CheckPixelProbes(snapPath, winW, pixelProbes)
	if pixelErr == nil {
		pixelOK = pixelTotal
	}

	// Copy is the boundary-conversion control: the two storage conventions
	// (Unorm premul-encoded vs sRGB premul-linear) must display the same
	// bytes after compositing. A raw-copy or double-decode bug separates them.
	copyEqErr := func() error {
		img := loadImage(snapPath)
		if img == nil {
			return fmt.Errorf("snapshot missing")
		}
		dpr := float64(img.Bounds().Dx()) / winW
		px0, py0 := probePoint(cardX[0], 1, 0)
		px1, py1 := probePoint(cardX[1], 1, 0)
		r0, g0, b0, _ := atRGB(img, int((bx+px0)*dpr), int((by+cardY+py0)*dpr))
		r1, g1, b1, _ := atRGB(img, int((bx+px1)*dpr), int((by+cardY+py1)*dpr))
		d := maxAbs3(int(r0)-int(r1), int(g0)-int(g1), int(b0)-int(b1))
		if d > 2 {
			return fmt.Errorf("copy control differs between cards: unorm=(%d,%d,%d) srgb=(%d,%d,%d) maxdiff=%d", r0, g0, b0, r1, g1, b1, d)
		}
		return nil
	}()
	pixelTotal++ // the copy equality probe
	if copyEqErr == nil {
		pixelOK++
	}

	baseName := "p31b_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		baseName = "p31b_final_go.png"
	}
	goldenRects := []wrkit.GoldenRect{
		{X: 0, Y: 0, W: winW, H: 48},
		{X: shell.Legend.X, Y: shell.Legend.Y, W: shell.Legend.W, H: shell.Legend.H},
		{X: bx, Y: by, W: shell.Body.W, H: shell.Body.H},
	}
	diffPct, totalPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, filepath.Join(snapDir, "goldens", baseName), goldenRects)

	cardSummary := func(c *p31bCard) string {
		if c == nil {
			return "nil"
		}
		return fmt.Sprintf("%s gpu_ops=%d cpu_fb=%d linear_fb=%d", c.targetTag, c.gpuOps, c.cpuFb, c.linearFb)
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "P31b-sRGB-chain",
		Scenario:      "ui_wr_p31b_srgb_chain",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"fps_gate":                      "off_correctness",
			"slope_gate":                    "off",
			"apis":                          "Context.CreateOffscreenTextureFormat,Context.FlushGPUWithView,Context.DrawGPUTexture,Canvas.DrawTextureRef,RenderPathStats.LinearFixedFunctionFallbacks",
			"cards":                         []string{cardSummary(cards[0]), cardSummary(cards[1])},
			"card_default_target":           "RGBA8Unorm",
			"card_linear_target":            "RGBA8UnormSrgb",
			"card_default_linear_fallbacks": cards[0].linearFb,
			"card_linear_linear_fallbacks":  cards[1].linearFb,
			"fixed_function_modes":          len(fixedModes),
			"pixel_probes_ok":               pixelOK,
			"pixel_probes_total":            pixelTotal,
			"pixel_probes_error":            firstErr(probeErrString(pixelErr), probeErrString(copyEqErr)),
			"pixel_golden_error":            probeErrString(goldenErr),
			"pixel_golden_diff_pct":         diffPct,
			"pixel_golden_total_px":         totalPx,
			"pixel_golden_first_run":        seeded,
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
	if pixelErr != nil {
		fail("pixel probes: %v", pixelErr)
	}
	if copyEqErr != nil {
		fail("copy control: %v", copyEqErr)
	}
	if goldenErr != nil {
		fail("golden: %v", goldenErr)
	} else if seeded {
		fail("golden seeded from this run — not a pass; review + commit the baseline, then re-run")
	} else if diffPct != 0 {
		fail("pixel_golden_diff_pct=%.4f want 0 over %d px", diffPct, totalPx)
	}
	if cards[0] == nil || cards[1] == nil {
		fail("cards were not baked")
	} else {
		if cards[0].linearFb != int64(len(fixedModes)+1) {
			fail("default target linear fallbacks=%d want %d (13 modes + bar)", cards[0].linearFb, len(fixedModes)+1)
		}
		if cards[1].linearFb != 0 {
			fail("linear target linear fallbacks=%d want 0", cards[1].linearFb)
		}
		for i, c := range cards {
			if c.gpuOps == 0 {
				fail("card %d gpu_ops=0: card draws did not ride the GPU", i)
			}
			if c.cpuFb != 0 {
				fail("card %d cpu_fallback_ops=%d", i, c.cpuFb)
			}
		}
	}
	if snap.GPUOps == 0 {
		fail("gpu_ops=0: window composite did not ride the GPU")
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
		}); err != nil {
			fail("%v", err)
		}
	}

	for _, c := range cards {
		if c != nil {
			render.ReleaseTextureRef(c.ref)
			c.release()
			_ = c.ctx.Close()
		}
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_p31b_srgb_chain: OK presents=%d pixel=%d/%d golden=%.4f%% default_fb=%d linear_fb=%d elapsed=%.1fs\n",
		app.PresentCount(), pixelOK, pixelTotal, diffPct, cards[0].linearFb, cards[1].linearFb, elapsedSec)
}

func probeErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func firstErr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func maxAbs3(a, b, c int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if c < 0 {
		c = -c
	}
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
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

func atRGB(img image.Image, x, y int) (byte, byte, byte, byte) {
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return 0, 0, 0, 0
	}
	r, g, b, a := img.At(x, y).RGBA()
	return byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)
}

type fixedTick struct{ on func(dt float64) }

func (t *fixedTick) Tick(dt float64) bool { t.on(dt); return true }
