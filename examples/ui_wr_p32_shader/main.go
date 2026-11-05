// Command ui_wr_p32_shader is the P3.2 user-shader capability real window
// (PLAN §5.1 / evidence/P3.2): cards baked through the real render.Context
// prove Paint.Shader on the GPU — a uniform-driven gradient, and a bound
// image sampled by the effect.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_p32_shader -auto-only
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
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

const (
	winW, winH   = 1200, 800
	closeSeconds = 5
	cardW        = 340.0
	cardH        = 240.0
	windowTitle  = "gpui ui_wr_p32_shader — P3.2 自定义 shader"
)

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

// gradientWGSL paints a vertical two-stop gradient driven by the uniform
// block: color = mix(c0, c1, uv.y), premultiplied.
const gradientWGSL = `
struct Uniforms { c0: vec4<f32>, c1: vec4<f32>, }
var<private> uDebug: bool = false;
@group(0) @binding(0) var<uniform> u: Uniforms;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let c = mix(u.c0.rgb, u.c1.rgb, uv.y);
    let dbg = select(c, vec3<f32>(1.0, 0.0, 0.0), uDebug);
    return vec4<f32>(dbg, 1.0);
}
`

// tintImageWGSL samples the bound image and tints it toward blue by uv.x.
const tintImageWGSL = `
struct Uniforms { pad: vec4<f32>, }
@group(0) @binding(0) var<uniform> u: Uniforms;
@group(0) @binding(1) var src_tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let s = textureSample(src_tex, samp, uv);
    return vec4<f32>(s.rgb * 0.5 + vec3<f32>(0.0, 0.0, 0.5) * s.a * uv.x, s.a);
}
`

type p32Card struct {
	img          *render.ImageBuf
	gpuOps       int
	cpuFallbacks int
}

var (
	bakeOnce sync.Once
	cards    [2]*p32Card
)

func bakeCard(w, h int, draw func(*render.Context)) *p32Card {
	dc := render.NewContext(w, h)
	draw(dc)
	stats := dc.RenderPathStats()
	img := dc.Image()
	_ = dc.Close()
	return &p32Card{img: render.ImageBufFromImage(img), gpuOps: stats.GPUOps, cpuFallbacks: stats.CPUFallbackOps}
}

// checkerImage builds an 8x8 black/white checker as the bound image.
func checkerImage() *render.ImageBuf {
	img, _ := render.NewImageBuf(8, 8, render.FormatRGBA8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			v := uint8(255)
			if (x/2+y/2)%2 == 0 {
				v = 0
			}
			_ = img.SetRGBA(x, y, v, v, v, 255)
		}
	}
	return img
}

func ensureBaked() {
	bakeOnce.Do(func() {
		cards[0] = bakeCard(int(cardW), int(cardH), func(dc *render.Context) {
			effect, err := render.NewShaderEffect(gradientWGSL)
			if err != nil {
				fmt.Fprintln(os.Stderr, "bake A: compile:", err)
				return
			}
			defer effect.Close()
			_ = effect.SetFloatUniforms([]float32{0.1, 0.2, 0.8, 1, 0.9, 0.3, 0.1, 1})
			dc.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.13, A: 1})
			dc.SetShader(effect)
			dc.DrawRectangle(0, 0, cardW, cardH)
			_ = dc.Fill()
		})
		cards[1] = bakeCard(int(cardW), int(cardH), func(dc *render.Context) {
			effect, err := render.NewShaderEffect(tintImageWGSL)
			if err != nil {
				fmt.Fprintln(os.Stderr, "bake B: compile:", err)
				return
			}
			defer effect.Close()
			effect.SetImage(checkerImage())
			dc.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.13, A: 1})
			dc.SetShader(effect)
			dc.DrawRectangle(0, 0, cardW, cardH)
			_ = dc.Fill()
		})
	})
}

var cardAt = [][2]float64{{8, 60}, {372, 60}}

func buildScene(shell *wrkit.ShellChrome) {
	drawBaked := func(idx int) func(*rendering.PaintContext, rendering.Size) {
		return func(pc *rendering.PaintContext, _ rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ensureBaked()
			c := cards[idx]
			if c == nil || c.img == nil {
				return
			}
			pc.DC.DrawImageEx(c.img, render.DrawImageOptions{
				X: pc.OriginX, Y: pc.OriginY, DstWidth: cardW, DstHeight: cardH,
				Interpolation: render.InterpNearest,
			})
		}
	}
	for i, title := range []string{
		"A uniform 渐变（mix(c0,c1,uv.y)）",
		"B 纹理输入（checker×tint(uv.x)）",
	} {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = drawBaked(i)
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.85, 0.9, 0.95), cardAt[i][0]+12, cardAt[i][1]+8)
	}
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode")
	flag.Parse()
	gateMode := *autoOnly
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	} else {
		wrkit.RequireMinRun(secs, "P32")
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: windowTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P3.2 自定义 shader — Paint.Shader GPU 执行（WGSL+uniform+纹理）", []string{
		"A uniform 渐变: mix(c0,c1,uv.y) 顶/中/底三点断言",
		"B 纹理输入: 绑定 checker 图 + uv.x 蓝移 tint",
		"门禁: gpu_ops>0 + cpu_fallback=0 + 金图逐位",
	})
	buildScene(shell)

	snapDir := filepath.Join("examples", "ui_wr_p32_shader", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p32_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.Scheduler().Tickers().Add(&fixedTick{on: func(dt float64) { app.ScheduleFrame() }})
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

	// Pixel probes on card A: the gradient is deterministic — top=c0, mid=mix,
	// bottom=c1 (premul straight=1 alpha, so bytes = 255*value).
	c0 := [3]byte{26, 51, 204}
	c1 := [3]byte{229, 76, 26}
	type probe struct {
		key  string
		x, y float64
		want [3]byte
		tol  float64
	}
	// Card absolute position: body offset + card placement (P0.2 rule 2 —
	// probe geometry is resolved from the layout chain, never hardcoded).
	bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
	cardAX, cardAY := bx+cardAt[0][0], by+cardAt[0][1]
	probes := []probe{
		{"p32_a_top", cardAX + 170, cardAY + 6, c0, 12.0 / 255},
		{"p32_a_mid", cardAX + 170, cardAY + 120, [3]byte{(c0[0] + c1[0]) / 2, (c0[1] + c1[1]) / 2, (c0[2] + c1[2]) / 2}, 14.0 / 255},
		{"p32_a_bottom", cardAX + 170, cardAY + 234, c1, 12.0 / 255},
	}
	pixelOK := 0
	pixelTotal := 0
	for _, p := range probes {
		ok := false
		detail := "no snapshot"
		if img != nil {
			r, g, b, _ := at(img, int(p.x*dpr), int(p.y*dpr))
			dr := absd(int(r), int(p.want[0]))
			dg := absd(int(g), int(p.want[1]))
			db := absd(int(b), int(p.want[2]))
			m := max(max(dr, dg), db)
			ok = float64(m)/255.0 <= p.tol
			detail = fmt.Sprintf("got (%d,%d,%d) want %v maxΔ=%d", r, g, b, p.want, m)
		}
		fmt.Fprintf(os.Stderr, "ui_wr_p32_shader: pixel %-22s ok=%v %s\n", p.key, ok, detail)
		if ok {
			pixelOK++
		}
		pixelTotal++
	}

	goldenRects := []rect{{x: 0, y: 0, w: winW, h: 48}, {x: bx + cardAt[0][0], y: by + cardAt[0][1], w: cardW, h: cardH}, {x: bx + cardAt[1][0], y: by + cardAt[1][1], w: cardW, h: cardH}}
	goldenDiffPct, goldenTotalPx, goldenSeeded, goldenMissing := evaluateGolden(snapPath, goldenRects, dpr)

	route := ""
	for i, c := range cards {
		if c != nil {
			route += fmt.Sprintf("card%d:gpu_ops=%d,cpu_fb=%d;", i, c.gpuOps, c.cpuFallbacks)
		}
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P32-shader", Scenario: "ui_wr_p32_shader", Backend: win.Backend().String(),
		Snap: snap, PresentCount: app.PresentCount(), ElapsedSec: elapsedSec,
		SurfaceAreaPx: winW * winH, Warmup: true,
		Extra: map[string]any{
			"fps_gate": "off_correctness", "slope_gate": "off",
			"apis":            "NewShaderEffect,Paint.Shader,SetUniformBytes,SetFloatUniforms,SetImage",
			"route":           route,
			"pixel_probes_ok": pixelOK, "pixel_probes_total": pixelTotal,
			"pixel_golden_diff_pct": goldenDiffPct, "pixel_golden_total_px": goldenTotalPx,
			"pixel_golden_first_run": goldenSeeded,
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
	if goldenMissing {
		fail("golden baseline missing (goldens/p32_final.png) — re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit")
	} else if goldenSeeded {
		fail("golden seeded from this run — not a pass; review + commit the baseline, then re-run")
	} else if goldenDiffPct != 0 {
		fail("golden diff %.4f%% want 0 over %d px", goldenDiffPct, goldenTotalPx)
	}
	for i, c := range cards {
		if c == nil || c.gpuOps == 0 {
			fail("card %d did not ride the GPU", i)
		}
		if c != nil && c.cpuFallbacks != 0 {
			fail("card %d fell back to CPU", i)
		}
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_p32_shader: OK presents=%d pixel=%d/%d golden=%.4f%% gate=%v elapsed=%.1fs\n",
		app.PresentCount(), pixelOK, pixelTotal, goldenDiffPct, gateMode, elapsedSec)
}

type fixedTick struct {
	on func(dt float64)
}

func (t *fixedTick) Tick(dt float64) bool { t.on(dt); return true }

func absd(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

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

func at(img image.Image, x, y int) (byte, byte, byte, byte) {
	r, g, b, a := img.At(x, y).RGBA()
	return byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)
}

// evaluateGolden 按 wrkit.GoldenGate 的 fail-closed 口径比对基线：
// 缺基线且没给 GPUI_ACCEPT_GOLDEN=1 ⇒ missing=true（判红，不自证绿）；
// 显式播种 ⇒ seeded=true（这一跑不算 PASS，须 review + 提交基线后复跑）。
func evaluateGolden(snapPath string, rects []rect, dpr float64) (diffPct float64, totalPx int64, seeded bool, missing bool) {
	goldenPath := filepath.Join(filepath.Dir(snapPath), "goldens", "p32_final.png")
	cur := loadImage(snapPath)
	if cur == nil {
		return 100, 0, false, true
	}
	golden := loadImage(goldenPath)
	if golden == nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			return 100, 0, false, true
		}
		_ = os.MkdirAll(filepath.Dir(goldenPath), 0o755)
		out, err := os.Create(goldenPath)
		if err != nil {
			return 100, 0, false, true
		}
		defer out.Close()
		if err := png.Encode(out, cur); err != nil {
			return 100, 0, false, true
		}
		return 0, 0, true, false
	}
	if golden.Bounds() != cur.Bounds() {
		return 100, 0, false, true
	}
	var diff, total int64
	for _, r := range rects {
		for y := int(r.y * dpr); y < int((r.y+r.h)*dpr); y++ {
			for x := int(r.x * dpr); x < int((r.x+r.w)*dpr); x++ {
				a1, b1, c1, _ := at(golden, x, y)
				a2, b2, c2, _ := at(cur, x, y)
				total++
				if a1 != a2 || b1 != b2 || c1 != c2 {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 100, 0, false, true
	}
	return float64(diff) * 100 / float64(total), total, false, false
}
