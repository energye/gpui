// Command p37_bloom is P3.7 batch 6 window ⑤: the bloom (泛光) close window.
//
// 必画臂（钉版清单 batch6-window-plan.md §1 ⑤ 行 + windows-3456-design.md 四）：
//
//	a) 亮部场景 × `Context.ApplyBloom(BloomParams)` 扫参——恒等/基准/阈值组/
//	   半径组/强度组五面板并排，泛光晕像素断言（暗区被点亮、亮区向外扩散）；
//	b) 恒等参数（Intensity=0）≡ 无泛光**逐位**（diff=0）；
//	c) LDR 三处声明探针（显式进 JSON + 底带可见）：① 本窗四 pass 走 RGBA8
//	   LDR 近似（真 float 全链入池 M5）；② `render/hdr` GPU 侧未接——
//	   `FloatTarget.TextureView()` 恒 nil（探针实测）；③ `engine/fx` CPU 泛光
//	   出口 clamp01=LDR（探针实测超亮输入出被钳值）。
//
// 口径如实（两件事写在前头）：
//  1. Canvas/Recorder 录制面没有 ApplyBloom 口（批 5 余账登记「要接另拍」），
//     窗内生产每帧 DC 是 Recorder——所以五面板在**离屏 *render.Context** 上
//     画场景+真调 ApplyBloom（GPU 四 pass 在引擎里被真实调用），结果以
//     **1:1 整数位 DrawImage** 贴回窗（scale=1 ⇒ 采样点落纹素中心，任何
//     采样核恒等，贴回逐位不改画面）。泛光画面进金图、恒等面板当语义像素
//     的逐位锚；这不是绕洞，能力面缺口本身已登记。
//  2. fps/p95 **只报数不判绿**（§3 硬要求第 5 条：性能门重定标未做）。
//
// 满窗即内容：指标行浮左上（金图 mask 掉 y<32 带）；画面静态无墙钟相位，
// 门禁相定帧 240 停机（H15）。金图按后端分文件
// goldens/p37_bloom_final_{native,go}.png 且 fail-closed（缺基线红、播种不算过）。
//
// H16 两变异（窗参一行，跑完还原）：① 基准强度 0.6→0（泛光消失，面板回恒等
// 帧 ⇒ 探针+金图双红）；② 阈值 0.6→0.05（全画面泛光 ⇒ 阈值组关系翻转+金图红）。
//
// 跑法：
//
//	export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib DISPLAY=:0
//	RUN_SECONDS=8 go run ./examples/engine/p37_bloom -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_bloom -auto-only  # 播种，仍不算过
//	GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_bloom -auto-only       # 第二后端
//	go run ./examples/engine/p37_bloom -manual-seconds 6                              # 常驻人工
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/fx"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/hdr"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200.0, 800.0
	metricStripH = 32.0
	winTitle     = "gpui p37_bloom — 泛光 RGBA8 四 pass"

	sceneW, sceneH = 384, 256

	// 面板参数（H16 变异位：refIntensity / refThreshold 各一行）。
	refThreshold  = 0.60
	refSigma      = 8.0
	refIntensity  = 0.60
	threshSweep   = 0.15 // 阈值组：同基准，阈值压低 ⇒ 中灰块也被点亮
	sigmaSweep    = 16.0 // 半径组：同基准，σ 抬到域顶 16 ⇒ 远环被点亮（12 时实测仅 +3.9，8-bit 尾太薄，如实记后抬域内最大）
	intensitySwp  = 1.6  // 强度组：同基准，增益抬高 ⇒ 近环更亮
	maxSigmaProbe = 17.0 // Validate 越界探针用（> MaxBloomSigma=16）

	// H15：定帧停机。
	gateFrames   = 240
	gateFreezeAt = 120

	backdropRGB = 0.055
)

var (
	// 五面板网格：行1 恒等/基准/阈值组，行2 半径组/强度组/声明带。
	panelX = [3]float64{12, 408, 804}
	panelY = [2]float64{36, 304}

	// 场景几何（scene 局部坐标；语义像素与 ROI 亮度都从这里反推）。
	// 亮块提取值（软膝 ramp=(L−T)/(1−T)）与高斯半平面尾 Φ(d/σ√2) 先把账算好：
	//   基准 σ8 ⇒ 亮黄块缘外 8px 处 ≈ +26/255（可判），缘外 20px ≈ +1（近零）；
	//   半径组 σ16 ⇒ 20px 处实测 ≈ 基准+4..+9 ⇒ 与基准差 ≥5；强度组 ×2.67 ⇒ 8px 处 +26→+69。
	coreRect = [4]float64{48, 96, 64, 64}   // 亮黄块（250,240,120）L≈0.86，T0.6 提取 ramp≈0.65
	whiteRct = [4]float64{256, 48, 48, 48}  // 近白块（245,245,250）L≈0.96，提取≈满
	midRect  = [4]float64{288, 176, 64, 48} // 中灰块（100,105,115）L≈0.41：T0.6 下 ramp=0，
	//                                          T0.15 下 ramp≈0.30 ⇒ 只有阈值组把它点亮
	bgRGB = [3]uint8{15, 15, 22}

	// ROI 点（scene 局部）。
	corePt   = [2]int{80, 128}  // 亮黄块中心
	midPt    = [2]int{320, 200} // 中灰块中心
	haloNear = [2]int{120, 128} // 亮黄块右缘外 8px（3.5σ@σ8 可判位）
	haloFar  = [2]int{132, 128} // 右缘外 20px（基准≈零、σ16 可判位）
	bgPt     = [2]int{40, 232}  // 远角背景（任何 σ 下都收不到光）
)

var testdataDir = filepath.Join("examples", "engine", "p37_bloom", "testdata")

// ---- 场景与面板 ----

func paintScene(dc render.Canvas) {
	dc.SetRGB(float64(bgRGB[0])/255, float64(bgRGB[1])/255, float64(bgRGB[2])/255)
	dc.DrawRectangle(0, 0, sceneW, sceneH)
	_ = dc.Fill()
	dc.SetRGB(250.0/255, 240.0/255, 120.0/255)
	dc.DrawRectangle(coreRect[0], coreRect[1], coreRect[2], coreRect[3])
	_ = dc.Fill()
	dc.SetRGB(245.0/255, 245.0/255, 250.0/255)
	dc.DrawRectangle(whiteRct[0], whiteRct[1], whiteRct[2], whiteRct[3])
	_ = dc.Fill()
	dc.SetRGB(100.0/255, 105.0/255, 115.0/255)
	dc.DrawRectangle(midRect[0], midRect[1], midRect[2], midRect[3])
	_ = dc.Fill()
}

// makePanel 在离屏 *render.Context 上画同一场景，按 apply 决定是否真调
// Context.ApplyBloom（GPU 四 pass 生产腿），FlushGPU 后深拷贝出面板位图。
func makePanel(p render.BloomParams, apply bool) (*render.ImageBuf, render.RenderPathStats) {
	dc := render.NewContext(sceneW, sceneH)
	paintScene(dc)
	if apply {
		dc.ApplyBloom(p)
	}
	_ = dc.FlushGPU()
	src := dc.Image()
	out, err := render.NewImageBuf(sceneW, sceneH, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	for y := 0; y < sceneH; y++ {
		for x := 0; x < sceneW; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			if err := out.SetRGBA(x, y, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)); err != nil {
				panic(err)
			}
		}
	}
	st := dc.RenderPathStats()
	_ = dc.Close()
	return out, st
}

func panelParams() []struct {
	p     render.BloomParams
	apply bool
} {
	return []struct {
		p     render.BloomParams
		apply bool
	}{
		{render.BloomParams{}, false}, // 0 恒等：不调用 ApplyBloom（与 Intensity=0 逐位，由探针1咬）
		{render.BloomParams{Threshold: refThreshold, Sigma: refSigma, Intensity: refIntensity}, true},
		{render.BloomParams{Threshold: threshSweep, Sigma: refSigma, Intensity: refIntensity}, true},
		{render.BloomParams{Threshold: refThreshold, Sigma: sigmaSweep, Intensity: refIntensity}, true},
		{render.BloomParams{Threshold: refThreshold, Sigma: refSigma, Intensity: intensitySwp}, true},
	}
}

var panels [5]*render.ImageBuf

// ---- 像素工具 ----

func lumAt(img *render.ImageBuf, x, y int) float64 {
	// 3×3 均值，全部 ROI 点在平坦块内，均值=单值。
	var sum float64
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			r, g, b, _ := img.GetRGBA(x+dx, y+dy)
			sum += 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
		}
	}
	return sum / 9.0
}

func regionBytes(img *render.ImageBuf, inset int) []byte {
	out := make([]byte, 0, (sceneW-2*inset)*(sceneH-2*inset)*4)
	for y := inset; y < sceneH-inset; y++ {
		for x := inset; x < sceneW-inset; x++ {
			r, g, b, a := img.GetRGBA(x, y)
			out = append(out, r, g, b, a)
		}
	}
	return out
}

func diffBytes(a, b []byte) (n int, worst int) {
	for i := 0; i+3 < len(a) && i+3 < len(b); i++ {
		d := abs(int(a[i]) - int(b[i]))
		if d > 0 {
			n++
			if d > worst {
				worst = d
			}
		}
	}
	return n, worst
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ---- 探针 ----

type probeResult struct {
	name string
	ok   bool
	got  string
}

type probeReport struct {
	results []probeResult
	extra   map[string]any
}

func (p *probeReport) add(name string, ok bool, got string) {
	p.results = append(p.results, probeResult{name, ok, got})
}

func runProbes() probeReport {
	var rep probeReport
	rep.extra = map[string]any{}
	params := panelParams()
	var stats [5]render.RenderPathStats
	for i, pp := range params {
		panels[i], stats[i] = makePanel(pp.p, pp.apply)
	}

	// 1) 恒等参数 ≡ 无泛光逐位：真调 ApplyBloom{Intensity:0}（门口短路语义
	//    filter_ops_cpu.go:532）后的画面与完全不调用逐位比。
	ctxCalled := render.NewContext(sceneW, sceneH)
	paintScene(ctxCalled)
	ctxCalled.ApplyBloom(render.BloomParams{Threshold: refThreshold, Sigma: refSigma, Intensity: 0})
	_ = ctxCalled.FlushGPU()
	srcImg := ctxCalled.Image()
	calledBuf, cerr := render.NewImageBuf(sceneW, sceneH, render.FormatRGBA8)
	if cerr != nil {
		panic(cerr)
	}
	for y := 0; y < sceneH; y++ {
		for x := 0; x < sceneW; x++ {
			r, g, b, a := srcImg.At(x, y).RGBA()
			if e := calledBuf.SetRGBA(x, y, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)); e != nil {
				panic(e)
			}
		}
	}
	d0, w0 := diffBytes(regionBytes(panels[0], 4), regionBytes(calledBuf, 4))
	rep.add("bloom_intensity_zero_bitwise_vs_no_call", d0 == 0 && w0 == 0,
		fmt.Sprintf("ApplyBloom{I=0} 调用后面板 ≡ 无泛光逐位 diff=%d worst=%d", d0, w0))
	_ = ctxCalled.Close()

	// 2) 参数域具名拒绝。
	badCases := []struct {
		name string
		p    render.BloomParams
	}{
		{"threshold>1", render.BloomParams{Threshold: 1.5, Sigma: 4, Intensity: 0.6}},
		{"sigma>max", render.BloomParams{Threshold: 0.6, Sigma: maxSigmaProbe, Intensity: 0.6}},
		{"intensity>max", render.BloomParams{Threshold: 0.6, Sigma: 4, Intensity: 9}},
		{"threshold NaN", render.BloomParams{Threshold: nan(), Sigma: 4, Intensity: 0.6}},
	}
	allRefused := true
	refusedGot := ""
	for _, c := range badCases {
		if e := c.p.Validate(); e == nil {
			allRefused = false
			refusedGot += c.name + " 被放行;"
		}
	}
	if e := (render.BloomParams{}).Validate(); e != nil {
		allRefused = false
		refusedGot += "零值被拒:" + e.Error()
	}
	rep.add("bloom_params_named_rejects", allRefused, refusedGot+"四枚越界具名拒+零值合法")

	// 3) IsIdentity 语义。
	idSem := (render.BloomParams{Threshold: 0.6, Sigma: 4, Intensity: 0}).IsIdentity() &&
		!(render.BloomParams{Threshold: 0.6, Sigma: 4, Intensity: 0.001}).IsIdentity()
	rep.add("bloom_is_identity_semantics", idSem, "I=0 恒等真、I=0.001 非恒等")

	// 4) 基准面板光晕成立（缘外 8px 被点亮，高于恒等面板同点；尾值账见 var 段）。
	haloRef := lumAt(panels[1], haloNear[0], haloNear[1])
	haloId := lumAt(panels[0], haloNear[0], haloNear[1])
	bgId := lumAt(panels[0], bgPt[0], bgPt[1])
	rep.add("reference_halo_engages", haloRef > haloId+8 && haloRef > bgId+8,
		fmt.Sprintf("基准近环 %.2f / 恒等近环 %.2f / 恒等背景 %.2f（门限 +8）", haloRef, haloId, bgId))

	// 5) 阈值组：中灰块被点亮（T=0.15 让 L≈0.41 过阈，基准 T=0.6 下 ramp=0）。
	midThresh := lumAt(panels[2], midPt[0], midPt[1])
	midRef := lumAt(panels[1], midPt[0], midPt[1])
	rep.add("threshold_sweep_lights_mid_region", midThresh > midRef+8,
		fmt.Sprintf("阈值组中灰块 %.2f > 基准 %.2f +8", midThresh, midRef))

	// 6) 半径组：远环被点亮（σ16 尾达 20px；基准 σ8 在该点≈零）。
	farSig := lumAt(panels[3], haloFar[0], haloFar[1])
	farRef := lumAt(panels[1], haloFar[0], haloFar[1])
	rep.add("sigma_sweep_widens_halo", farSig > farRef+5,
		fmt.Sprintf("半径组远环 %.2f > 基准 %.2f +5", farSig, farRef))

	// 7) 强度组：近环更亮（I=1.6 对 0.6 增益 ×2.67）。
	nearInt := lumAt(panels[4], haloNear[0], haloNear[1])
	rep.add("intensity_sweep_brightens_halo", nearInt > haloRef+8,
		fmt.Sprintf("强度组近环 %.2f > 基准 %.2f +8", nearInt, haloRef))

	// 8) 五面板两两互异（含恒等 vs 基准）。
	pairOK := true
	pairGot := ""
	diffs := map[string]int{}
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			n, _ := diffBytes(regionBytes(panels[i], 4), regionBytes(panels[j], 4))
			diffs[fmt.Sprintf("p%d_vs_p%d", i, j)] = n
			if n < 200 {
				pairOK = false
				pairGot += fmt.Sprintf("p%d↔p%d=%d;", i, j, n)
			}
		}
	}
	rep.extra["panel_pair_diffs"] = diffs
	rep.add("five_panels_pairwise_differ", pairOK, pairGot+"10 对全 ≥200 字节")

	// 9) LDR 声明②：render/hdr 的 GPU 面未接——TextureView 恒 nil（实测）。
	ft, ferr := hdr.NewFloatTarget(64, 64)
	viewNil := ferr == nil && ft != nil && ft.TextureView() == nil
	rep.add("hdr_float_texture_view_nil", viewNil, fmt.Sprintf("NewFloatTarget err=%v TextureView()==nil:%v", ferr, viewNil))

	// 10) LDR 声明③：engine/fx CPU 泛光出口 clamp01=LDR（实测超亮输入被钳）。
	fxSrc := &fx.Image{W: 2, H: 2, Pix: []core.Color{
		{R: 2, G: 2, B: 2, A: 1}, {R: 2, G: 2, B: 2, A: 1},
		{R: 2, G: 2, B: 2, A: 1}, {R: 2, G: 2, B: 2, A: 1},
	}}
	fb, fbxErr := fx.NewBloom(0.5, 1.0, 1.0)
	clamped := true
	if fbxErr == nil {
		fout, e := fb.Apply(fxSrc)
		if e != nil || fout == nil {
			clamped = false
		} else {
			for _, c := range fout.Pix {
				if c.R > 1 || c.G > 1 || c.B > 1 {
					clamped = false
				}
			}
		}
	} else {
		clamped = false
	}
	rep.add("fx_cpu_bloom_clamps_to_ldr", clamped, fmt.Sprintf("NewBloom err=%v 出口逐通道 ≤1", fbxErr))

	// 11) 泛光真的改了画面（基准 vs 恒等，非空转绿）。
	nBase, _ := diffBytes(regionBytes(panels[0], 4), regionBytes(panels[1], 4))
	rep.add("bloom_changes_picture_vs_identity", nBase > 1000, fmt.Sprintf("基准 vs 恒等 %d 字节差", nBase))

	// 报数不判绿：泛光腿身份（GPU/CPU）与 pass 归属放 JSON，§3 第 5 条。
	rep.extra["bloom_leg_stats_report_only"] = map[string]any{
		"ref_gpu_ops":   stats[1].GPUOps,
		"ref_cpu_fb":    stats[1].CPUFallbackOps,
		"ref_last_fb":   stats[1].LastCPUFallbackReason,
		"sigma_gpu_ops": stats[3].GPUOps,
	}
	rep.extra["ldr_declarations"] = map[string]string{
		"window_gpu_pass": "本窗四 pass 走 RGBA8 LDR 近似（真 float 全链=在册 M5）",
		"hdr_gpu_unwired": "render/hdr FloatTarget.TextureView() 恒 nil（探针 9 实测）",
		"fx_cpu_leg_ldr":  "engine/fx CPU 泛光出口 clamp01=LDR（探针 10 实测）",
	}
	return rep
}

func nan() float64 { return math.NaN() }

// ---- 窗口 ----

func paintAll(dc render.Canvas, w, h float64) error {
	dc.SetRGB(backdropRGB, backdropRGB, backdropRGB+0.02)
	dc.DrawRectangle(0, 0, w, h)
	if err := dc.Fill(); err != nil {
		return err
	}
	// 1:1 整数位贴回：scale=1 ⇒ 采样点落纹素中心，任何采样核恒等。
	dc.DrawImage(panels[0], panelX[0], panelY[0])
	dc.DrawImage(panels[1], panelX[1], panelY[0])
	dc.DrawImage(panels[2], panelX[2], panelY[0])
	dc.DrawImage(panels[3], panelX[0], panelY[1])
	dc.DrawImage(panels[4], panelX[1], panelY[1])
	return nil
}

func semanticProbes() []wrkit.PixelProbe {
	// 恒等面板（无泛光）四色块逐位锚——同时验证 1:1 贴回链路不改画面。
	probes := make([]wrkit.PixelProbe, 0, 4)
	add := func(name string, sx, sy int, c [3]uint8) {
		x := panelX[0] + float64(sx)
		y := panelY[0] + float64(sy)
		probes = append(probes, wrkit.PixelProbe{Name: name, X: x, Y: y,
			R: float64(c[0]) / 255, G: float64(c[1]) / 255, B: float64(c[2]) / 255, Tol: 2})
	}
	add("identity_panel_core", corePt[0], corePt[1], [3]uint8{250, 240, 120})
	add("identity_panel_white", 280, 72, [3]uint8{245, 245, 250})
	add("identity_panel_mid", midPt[0], midPt[1], [3]uint8{100, 105, 115})
	add("identity_panel_bg", bgPt[0], bgPt[1], bgRGB)
	return probes
}

func backendTag() string {
	b, err := render.ResolveBackendFor(render.BackendNative)
	if err != nil {
		return "native"
	}
	return b.String()
}

type tick struct{ on func(dt float64) bool }

func (t *tick) Tick(dt float64) bool { return t.on(dt) }

type manualSummary struct {
	Pointer, Key, Resize int
}

func main() {
	out := flag.String("out", "", "extra JSON path (optional)")
	autoOnly := flag.Bool("auto-only", false, "gate mode: fixed-frame run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Int("manual-seconds", 0, "resident phase timeout in seconds (0 = until close)")
	flag.Parse()
	_ = os.MkdirAll(testdataDir, 0o755)

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly && !secsSet {
		secs = 8
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	}
	gateMode := *autoOnly
	wrkit.EnsureUIFace()

	t0 := time.Now()
	rep := runProbes()
	probesOK := true
	for _, r := range rep.results {
		if !r.ok {
			probesOK = false
			fmt.Fprintf(os.Stderr, "p37_bloom: 探针 FAIL %s → %s\n", r.name, r.got)
		}
	}
	var summary manualSummary
	var paintErr string

	win, err := platform.Open(platform.Options{Width: int(winW), Height: int(winH), Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: backdropRGB, G: backdropRGB, B: backdropRGB + 0.02, A: 1}

	content := rendering.NewRenderBox()
	content.FixedWidth, content.FixedHeight = winW, winH
	content.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		if e := paintAll(pc.DC, size.Width, size.Height); e != nil {
			paintErr = e.Error()
		}
	}
	root.Place(content, 0, 0)

	metric := wrkit.Label("presents=0 panels=5", 13, 0.92, 0.94, 0.98)
	root.Place(metric, 8, 8)
	// 面板标题（恒等面板在行1 左，声明带占据行2 右）。
	titles := []string{"① 恒等（无泛光）", "② 基准 T0.60 σ8 I0.60", "③ 阈值组 T0.15", "④ 半径组 σ16", "⑤ 强度组 I1.60"}
	tx := [5]float64{panelX[0], panelX[1], panelX[2], panelX[0], panelX[1]}
	ty := [5]float64{panelY[0], panelY[0], panelY[0], panelY[1], panelY[1]}
	for i, s := range titles {
		root.Place(wrkit.Label(s, 11, 0.90, 0.92, 0.96), tx[i]+4, ty[i]+4)
	}
	// LDR 三处声明底带（显式可见，JSON 同步带 ldr_declarations）。
	decls := []string{
		"LDR 声明①：本窗四 pass 走 RGBA8 LDR 近似（真 float 全链 = 在册 M5，不冒充 HDR）",
		"LDR 声明②：render/hdr 的 GPU 面未接——FloatTarget.TextureView() 恒 nil（窗内探针实测）",
		"LDR 声明③：engine/fx CPU 泛光出口 clamp01 = LDR（窗内探针实测超亮输入被钳）",
		"fps/p95 只报数不判绿（批 6 硬要求 §3-5：性能门重定标未做，不冒充绿）",
	}
	for i, s := range decls {
		root.Place(wrkit.Label(s, 11, 0.85, 0.88, 0.94), 12, 580+float64(i)*22)
	}

	setTitle := func() {
		ctl := win.Controls()
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — ptr=%d key=%d rs=%d t=%.0fs", winTitle,
			summary.Pointer, summary.Key, summary.Resize, time.Since(t0).Seconds()))
	}

	gateMaxFrames := int64(0)
	if gateMode {
		gateMaxFrames = gateFrames
	}
	snapPath := filepath.Join(testdataDir, "p37_bloom_final.png")
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: backdropRGB, ClearG: backdropRGB, ClearB: backdropRGB + 0.02, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		MaxFrames:    gateMaxFrames,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "p37_bloom: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				setTitle()
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					content.FixedWidth, content.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsPaint()
				}
			case platform.EventPointer:
				summary.Pointer++
				if summary.Pointer%24 == 1 {
					setTitle()
				}
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					setTitle()
				}
			default:
			}
		},
	})
	var proc scheduler.ProcessTracker
	proc.Start()
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) bool {
		metric.SetText(fmt.Sprintf("presents=%d panels=5", app.PresentCount()))
		metric.MarkNeedsPaint()
		setTitle()
		content.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		return true
	}})
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: app.Run: %v\n", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	if gateMaxFrames > 0 && app.PresentCount() < gateMaxFrames {
		fmt.Fprintf(os.Stderr, "p37_bloom: FAIL 定帧停机没达成 presents=%d want>=%d（末帧随墙钟漂）\n",
			app.PresentCount(), gateMaxFrames)
		probesOK = false
	}

	tag := backendTag()
	basePath := filepath.Join(testdataDir, "goldens", fmt.Sprintf("p37_bloom_final_%s.png", tag))
	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGateW(snapPath, basePath, winW,
		[]wrkit.GoldenRect{{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH}})
	goldenOK := goldenErr == nil && !seeded && goldenDiff == 0
	if goldenErr != nil {
		fmt.Fprintf(os.Stderr, "p37_bloom: golden: %v\n", goldenErr)
	}
	if seeded {
		fmt.Fprintf(os.Stderr, "p37_bloom: golden 已播种（%s），这一跑不算 PASS —— 复核后提交基线再复跑\n", basePath)
	}

	pixelOK := paintErr == ""
	if gateMode {
		if pixelErr := wrkit.CheckPixelProbes(snapPath, winW, semanticProbes()); pixelErr != nil {
			pixelOK = false
			fmt.Fprintf(os.Stderr, "p37_bloom: 像素断言 FAIL: %v\n", pixelErr)
		}
	} else {
		fmt.Fprintln(os.Stderr, "p37_bloom: 常驻相末帧未定格（H15 只在门禁相成立），像素与金图只报数不判绿")
	}
	if paintErr != "" {
		fmt.Fprintf(os.Stderr, "p37_bloom: 绘制 FAIL: %s\n", paintErr)
	}

	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	extra := map[string]any{
		"probes_ok": probesOK, "pixel_ok": pixelOK, "paint_err": paintErr,
		"golden_diff_pct": goldenDiff, "golden_total_px": goldenPx,
		"golden_seeded": seeded, "golden_backend_tag": tag,
		"gate_frames": gateFrames, "gate_freeze_at": gateFreezeAt,
		"golden_tolerance_pct": 0,
		"perf_note":            "fps/p95 只报数不判绿（性能门重定标未做）",
	}
	for k, v := range rep.extra {
		extra[k] = v
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "p37-6-5", Scenario: "p37_bloom", Backend: tag,
		Snap: snap, PresentCount: app.PresentCount(), ElapsedSec: time.Since(t0).Seconds(),
		SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
	})
	raw, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(raw))
	if *out != "" {
		_ = os.WriteFile(*out, raw, 0o644)
	}

	if *autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if !probesOK {
			fmt.Fprintln(os.Stderr, "FAIL: 逻辑探针未全绿")
			os.Exit(1)
		}
		if !pixelOK {
			fmt.Fprintln(os.Stderr, "FAIL: 语义像素断言未过")
			os.Exit(1)
		}
		if !goldenOK {
			fmt.Fprintf(os.Stderr, "FAIL: golden diff=%.4f%% over %d px seeded=%v err=%v\n", goldenDiff, goldenPx, seeded, goldenErr)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "p37_bloom: OK backend=%s presents=%d probes=%d/%d golden=%.4f%% (%d px)\n",
			tag, app.PresentCount(), len(rep.results), len(rep.results), goldenDiff, goldenPx)
		return
	}
	fmt.Fprintf(os.Stderr, "p37_bloom: resident done backend=%s presents=%d probes_ok=%v pixel_ok=%v golden=%.4f%%\n",
		tag, app.PresentCount(), probesOK, pixelOK, goldenDiff)
}
