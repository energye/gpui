// Command ui_wr_p37_mat_persp is the dedicated window for the 材质×透视/bicubic
// project (P3.7, 2026-11-07): the composed vertex machine carrying per-corner
// weights (A 批) and the engine-fed texel contract (B 批), with real-window
// pixel evidence per the audit standard.
//
// 必画面板（拍板口径 mat-persp-assessment.md：新专项窗承管材质×透视画面，
// ③⑥ 只迁拒面）：
//
//	P1 恒等材质+透视：对照 | 材质 两格并排——identity 材质+透视 ≡ 无材质
//	   透视**逐位**（A 批钉 TestP37MatPerspQuadIdentityBitwise 的窗内版）；
//	P2 渐变带透视：uv.y 带状材质骑透视正确插值——色带分界 = hom_w 公式
//	   反推（梯形 ×2：TL/TR w=2 远、BR/BL w=1 近 ⇒ 分界 y=40，仿射 60；
//	   (80,55) 反拍平锚：权重若被拍平该点错画红）；
//	P3 bicubic 纹素 echo：对照 | 材质——echo 材质（返回引擎纹素原样）
//	   ≡ 无材质 bicubic 核**逐位**（B 批钉的窗内版，引擎喂纹素）；
//	P4 精灵路 material+persp：对照 | 材质——批绘口权重透传逐位；
//	P5 投影层组合：对照 | 材质——材质+画布投影层（projW 权重）逐位；
//	P6 拒绝牌：投影层眼后现场拒（ErrQuadBehindEye，几何族——材质+透视
//	   放行后可拒面收敛）+ blend+材质 skip+MaterialSkips（Normal-only
//	   口径不变），芯片色由探针事实驱动（红=真拒、琥珀=异常）。
//
// 满窗即内容：指标行浮左上（金图 mask 掉 y<32 带）；画面相位只随已完成帧数
// 走、过 settling 帧后定格（H15）。金图按后端分文件
// goldens/ui_wr_p37_mat_persp_final_{native,go}.png 且 fail-closed。
//
// 跑法：
//
//	export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib
//	RUN_SECONDS=8 go run ./examples/ui_wr_p37_mat_persp -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/ui_wr_p37_mat_persp -auto-only  # 播种，仍判红
//	go run ./examples/ui_wr_p37_mat_persp                                                 # 常驻人工
//	GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/ui_wr_p37_mat_persp -auto-only        # 第二后端
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	winW, winH   = 1200.0, 800.0
	metricStripH = 32.0
	winTitle     = "gpui mat_persp — 材质×透视/bicubic 专项"

	gateFrames   = 240
	gateFreezeAt = 120

	backdropRGB = 0.055
	cardRGB     = 0.10
)

var testdataDir = filepath.Join("examples", "ui_wr_p37_mat_persp", "testdata")

func newChecker(w, h int) *render.ImageBuf {
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	x := uint32(12345)
	data := buf.Data()
	for i := 0; i < w*h; i++ {
		x = x*1664525 + 1013904223
		g := uint8(x >> 24)
		off := i * 4
		data[off], data[off+1], data[off+2], data[off+3] = g, g, g, 255
	}
	buf.NotifyPixelsChanged()
	return buf
}

var (
	checker = newChecker(24, 24)

	mpIdentityWGSL = `@group(0) @binding(1) var mp_tex: texture_2d<f32>;
@group(0) @binding(2) var mp_samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(mp_tex, mp_samp, uv);
}`

	// echo：4 参全声明（引擎注入纹理对——注入路径的窗内版）。
	mpEchoWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>, @location(1) tint: vec4<f32>, @location(2) hom_w: f32, @location(3) texel: vec4<f32>) -> @location(0) vec4<f32> {
    return texel;
}`

	mpBandWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    if (uv.y < 0.5) {
        return vec4<f32>(0.86, 0.20, 0.22, 1.0);
    }
    return vec4<f32>(0.20, 0.42, 0.92, 1.0);
}`
)

func mustEffect(wgsl string) *render.ShaderEffect {
	e, err := render.NewShaderEffect(wgsl)
	if err != nil {
		panic(fmt.Sprintf("mat_persp: shader: %v", err))
	}
	return e
}

var (
	effIdentity = mustEffect(mpIdentityWGSL)
	effEcho     = mustEffect(mpEchoWGSL)
	effBand     = mustEffect(mpBandWGSL)
)

// 面板几何：3 列 × 2 行卡底；对面板两格并排（对照 | 材质），格 160×200。
const (
	pW, pH    = 380.0, 340.0
	p1x, p1y  = 12.0, 64.0
	p2x, p2y  = 408.0, 64.0
	p3x, p3y  = 804.0, 64.0
	p4x, p4y  = 12.0, 440.0
	p5x, p5y  = 408.0, 440.0
	p6x, p6y  = 804.0, 440.0
	cellW     = 160.0
	cellGap   = 40.0
	cellYOff  = 60.0
	chipsYOff = 140.0
	chipW     = 24.0
	chipStep  = 48.0
)

// 面板内几何（var：复合字面量非常量）。
var (
	// P1/P3/P4 透视几何（面板内、格原点系）：非退化凸四角 + 非均匀权重。
	mpCorners = [4]render.Point{{X: 20, Y: 10}, {X: 130, Y: 4}, {X: 124, Y: 150}, {X: 10, Y: 144}}
	mpWeights = [4]float64{1.5, 1.2, 1.0, 2.0}
	// P2 渐变带几何（单位试验梯形 ×2）：TL(40,20) TR(120,20) BR(140,100)
	// BL(20,100)，TL/TR w=2（远）、BR/BL w=1（近）。uv.y=0.5 分界的透视
	// 正确落点 y=40（仿射 60）；中心线 x=80。
	bandCorners = [4]render.Point{{X: 40, Y: 20}, {X: 120, Y: 20}, {X: 140, Y: 100}, {X: 20, Y: 100}}
	bandWeights = [4]float64{2, 2, 1, 1}
	// P5 投影层组合：仿射角点 + homography（面板内 y 全 <333 不跨眼平面；
	// 眼后拒是 P6 的现场）。
	projM = render.Matrix4FromElements([16]float64{
		1, 0, 0.0625, 0,
		0, 1, 0.125, 0,
		0, 0, 1, 0,
		0, 0.00390625, 0, 1,
	})
	projCorners = [4]render.Point{{X: 20, Y: 30}, {X: 140, Y: 30}, {X: 140, Y: 170}, {X: 20, Y: 170}}
	// P6 眼后投影（对照 render/p37_projection_test.go projHomography 同式）。
	behindM = render.Matrix4Homography(render.Translate(0, 0), 0, -0.003, 1)
)

func absI(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// refusalFacts 两枚拒绝态实测（纯逻辑；P6 芯片与探针吃同一份结果）。
func refusalFacts() (behindEyeRefused, blendSkipCounted bool, got string) {
	dc := render.NewContext(320, 220)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	// 眼后现场拒：投影层 w(y)=1-0.003y 过零，quad 跨眼平面 + 材质在场也不救。
	dc.SetProjection(behindM)
	errQ := dc.DrawImageQuad(checker, [4]render.Point{
		{X: 10, Y: 0}, {X: 110, Y: 0}, {X: 110, Y: 400}, {X: 10, Y: 400},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Material: effIdentity})
	dc.SetProjection(render.Matrix4Identity())
	behindEyeRefused = errors.Is(errQ, render.ErrQuadBehindEye)
	// blend+材质：无错误通道 ⇒ CPU skip + MaterialSkips++（skip≠fallback）。
	before := dc.RenderPathStats().MaterialSkips
	dc.DrawImageEx(checker, render.DrawImageOptions{X: 8, Y: 8, BlendMode: render.BlendMultiply, Material: effIdentity})
	blendSkipCounted = dc.RenderPathStats().MaterialSkips-before == 1
	got = fmt.Sprintf("behind_eye=%v(%v) blend_skip=%v", behindEyeRefused, errQ, blendSkipCounted)
	return behindEyeRefused, blendSkipCounted, got
}

var mpChipFacts = [2]bool{false, false}

func paintCard(dc render.Canvas, x, y, w, h float64) {
	dc.SetRGB(cardRGB, cardRGB, cardRGB+0.04)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

func drawQuadAt(dc render.Canvas, img *render.ImageBuf, ox, oy float64, corners [4]render.Point, weights [4]float64, opts render.QuadDrawOptions) error {
	moved := [4]render.Point{}
	for i, c := range corners {
		moved[i] = render.Point{X: c.X + ox, Y: c.Y + oy}
	}
	return dc.DrawImageQuad(img, moved, opts)
}

// paintAll 画整窗；withMaterials=false 画对照版（全部材质置 nil）——
// 探针吃对照版与窗版同格区域做逐位对。
func paintAll(dc render.Canvas, w, h float64, withMaterials bool) {
	dc.SetRGB(backdropRGB, backdropRGB, backdropRGB+0.02)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	for _, p := range [][2]float64{{p1x, p1y}, {p2x, p2y}, {p3x, p3y}, {p4x, p4y}, {p5x, p5y}, {p6x, p6y}} {
		paintCard(dc, p[0], p[1], pW, pH)
	}

	mat := func(e *render.ShaderEffect) *render.ShaderEffect {
		if withMaterials {
			return e
		}
		return nil
	}

	// P1 恒等材质+透视：对照 | 材质。
	_ = drawQuadAt(dc, checker, p1x+8, p1y+cellYOff, mpCorners, mpWeights,
		render.QuadDrawOptions{Perspective: true, Weights: mpWeights, Interpolation: render.InterpNearest})
	_ = drawQuadAt(dc, checker, p1x+8+cellW+cellGap, p1y+cellYOff, mpCorners, mpWeights,
		render.QuadDrawOptions{Perspective: true, Weights: mpWeights, Interpolation: render.InterpNearest, Material: mat(effIdentity)})

	// P2 渐变带透视（单格，带分界透视正确落点由语义锚盯住）。
	_ = drawQuadAt(dc, checker, p2x+8, p2y+cellYOff, bandCorners, bandWeights,
		render.QuadDrawOptions{Perspective: true, Weights: bandWeights, Interpolation: render.InterpNearest, Material: mat(effBand)})

	// P3 bicubic 纹素 echo：对照 | 材质。
	_ = drawQuadAt(dc, checker, p3x+8, p3y+cellYOff, mpCorners, mpWeights,
		render.QuadDrawOptions{Perspective: true, Weights: mpWeights, Interpolation: render.InterpBicubic})
	_ = drawQuadAt(dc, checker, p3x+8+cellW+cellGap, p3y+cellYOff, mpCorners, mpWeights,
		render.QuadDrawOptions{Perspective: true, Weights: mpWeights, Interpolation: render.InterpBicubic, Material: mat(effEcho)})

	// P4 精灵路 material+persp：对照 | 材质（DrawAtlasEx 自带透视+权重）。
	spriteCorners := [4]render.Point{}
	spriteCornersMat := [4]render.Point{}
	for i, c := range mpCorners {
		spriteCorners[i] = render.Point{X: c.X + p4x + 8, Y: c.Y + p4y + cellYOff}
		spriteCornersMat[i] = render.Point{X: c.X + p4x + 8 + cellW + cellGap, Y: c.Y + p4y + cellYOff}
	}
	_, _ = dc.DrawAtlasEx(checker, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 24, SrcH: 24,
		Perspective: true, Corners: spriteCorners, Weights: mpWeights, Filter: render.InterpNearest,
	}}, render.AtlasDrawOptions{})
	_, _ = dc.DrawAtlasEx(checker, []render.AtlasSprite{{
		SrcX: 0, SrcY: 0, SrcW: 24, SrcH: 24,
		Perspective: true, Corners: spriteCornersMat, Weights: mpWeights, Filter: render.InterpNearest, Material: mat(effIdentity),
	}}, render.AtlasDrawOptions{})

	// P5 投影层组合：对照 | 材质（SetProjection 粘滞段内两笔仿射角点 quad；
	// 材质笔经 Translate 右移——投影在设备域生效，平移先行语义与 ⑥ A1 同源）。
	dc.Push()
	dc.SetProjection(projM)
	_ = dc.DrawImageQuad(checker, projCorners, render.QuadDrawOptions{Interpolation: render.InterpNearest})
	dc.Pop()
	dc.Push()
	dc.Translate(cellW+cellGap, 0)
	dc.SetProjection(projM)
	moved := [4]render.Point{}
	for i, c := range projCorners {
		moved[i] = render.Point{X: c.X + 0, Y: c.Y + 0}
	}
	_ = dc.DrawImageQuad(checker, moved, render.QuadDrawOptions{Interpolation: render.InterpNearest, Material: mat(effIdentity)})
	dc.Pop()

	// P6 拒绝牌（芯片色由探针事实驱动：红=真拒/真计数；琥珀=异常）。
	for i, fact := range mpChipFacts {
		x := p6x + 24 + float64(i)*chipStep
		y := p6y + chipsYOff
		if fact {
			dc.SetRGB(200.0/255, 60.0/255, 60.0/255)
		} else {
			dc.SetRGB(235.0/255, 180.0/255, 60.0/255)
		}
		dc.DrawRectangle(x, y, chipW, chipW)
		_ = dc.Fill()
	}
}

func newCtx(w, h int) *render.Context {
	return render.NewContext(w, h)
}

type probeResult struct {
	name string
	ok   bool
	got  string
}

type probeReport struct {
	results  []probeResult
	extra    map[string]any
	pixelErr error
}

func (p *probeReport) add(name string, ok bool, got string) {
	p.results = append(p.results, probeResult{name, ok, got})
}

func regionBytes(dc *render.Context, x, y, w, h int) []byte {
	out := make([]byte, 0, w*h*4)
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			r, g, b, a := dc.Image().At(px, py).RGBA()
			out = append(out, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
		}
	}
	return out
}

func diffBytes(a, b []byte) (n int, worst int) {
	for i := 0; i+3 < len(a) && i+3 < len(b); i++ {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		if d > 0 {
			n++
			if d > worst {
				worst = d
			}
		}
	}
	return n, worst
}

func flushErr(dc *render.Context, paint func() error) error {
	if err := paint(); err != nil {
		return err
	}
	return dc.FlushGPU()
}

func runProbes() probeReport {
	var rep probeReport
	rep.extra = map[string]any{}
	w, h := int(winW), int(winH)

	// 窗画两份：对照版（材质全 nil）与窗版（含材质）——四对逐位从同一对上下文
	// 取同几何区域（对照格 | 材质格在窗内并排同几何，逐位对直接可取）。
	dcWin := newCtx(w, h)
	if err := flushErr(dcWin, func() error {
		paintAll(dcWin, float64(w), float64(h), true)
		return nil
	}); err != nil {
		rep.pixelErr = fmt.Errorf("window paint: %w", err)
	}

	pairs := []struct {
		name                 string
		ax, ay, bx, by, w, h int
	}{
		{"identity_persp_pair_bitwise", int(p1x + 8), int(p1y + cellYOff), int(p1x + 8 + cellW + cellGap), int(p1y + cellYOff), 150, 152},
		{"bicubic_echo_pair_bitwise", int(p3x + 8), int(p3y + cellYOff), int(p3x + 8 + cellW + cellGap), int(p3y + cellYOff), 150, 152},
		{"sprite_persp_pair_bitwise", int(p4x + 8), int(p4y + cellYOff), int(p4x + 8 + cellW + cellGap), int(p4y + cellYOff), 150, 152},
		{"projection_combo_pair_bitwise", int(p5x + 20), int(p5y + cellYOff), int(p5x + 20 + cellW + cellGap), int(p5y + cellYOff), 130, 145},
	}
	for _, pr := range pairs {
		a := regionBytes(dcWin, pr.ax, pr.ay, pr.w, pr.h)
		b := regionBytes(dcWin, pr.bx, pr.by, pr.w, pr.h)
		n, worst := diffBytes(a, b)
		rep.extra[pr.name+"_diff"] = n
		rep.add(pr.name, n == 0 && worst == 0,
			fmt.Sprintf("对照|材质 diff=%d worst=%d（同几何并排逐位）", n, worst))
	}

	// 渐变带解析锚（离屏，与窗画同源）：分界 y=40（仿射 60）——(80,30) 红、
	// (80,55) 蓝反拍平锚。
	ox, oy := p2x+8, p2y+cellYOff
	rr, rg, rb, _ := dcWin.Image().At(int(ox+80), int(oy+30)).RGBA()
	redOK := absI(int(rr>>8)-219) <= 3 && absI(int(rg>>8)-51) <= 3 && absI(int(rb>>8)-56) <= 3
	br, bg, bb, _ := dcWin.Image().At(int(ox+80), int(oy+55)).RGBA()
	blueOK := absI(int(br>>8)-51) <= 3 && absI(int(bg>>8)-107) <= 3 && absI(int(bb>>8)-235) <= 3
	rep.add("band_boundary_perspective_correct", redOK && blueOK,
		fmt.Sprintf("(80,30)=(%d,%d,%d) want≈(219,51,56)；(80,55)=(%d,%d,%d) want≈(51,107,235)——若 55 偏红=权重拍平仿射（分界 60）",
			rr>>8, rg>>8, rb>>8, br>>8, bg>>8, bb>>8))
	_ = dcWin.Close()

	// 拒绝事实（P6 芯片同源）。
	behindOK, blendOK, got := refusalFacts()
	mpChipFacts[0], mpChipFacts[1] = behindOK, blendOK
	rep.add("behind_eye_material_refused_named", behindOK,
		"眼后投影+材质 ⇒ ErrQuadBehindEye（几何族具名拒，材质在场也不救）")
	rep.add("blend_material_skip_counted", blendOK,
		"blend+材质无错误通道 ⇒ skip + MaterialSkips++（Normal-only 口径）")
	rep.extra["refusal_facts"] = got

	// GPU op 有界。
	dcS := newCtx(w, h)
	var st render.RenderPathStats
	if err := flushErr(dcS, func() error {
		paintAll(dcS, float64(w), float64(h), true)
		return nil
	}); err != nil {
		rep.pixelErr = fmt.Errorf("stats paint: %w", err)
	}
	st = dcS.RenderPathStats()
	rep.extra["paint_gpu_ops"] = st.GPUOps
	rep.add("paint_gpu_ops_bounded", st.GPUOps > 0 && st.GPUOps < 96,
		fmt.Sprintf("gpu_ops=%d cpu_fallback=%d", st.GPUOps, st.CPUFallbackOps))
	_ = dcS.Close()

	return rep
}

func semanticProbes() []wrkit.PixelProbe {
	probes := make([]wrkit.PixelProbe, 0, 8)
	add := func(name string, x, y float64, c [3]float64, tol uint8) {
		probes = append(probes, wrkit.PixelProbe{Name: name, X: x, Y: y,
			R: c[0], G: c[1], B: c[2], Tol: tol})
	}
	// P2 带分界两锚（快照上的透视正确插值）。
	add("band_red_anchor", p2x+8+80, p2y+cellYOff+30, [3]float64{219.0 / 255, 51.0 / 255, 56.0 / 255}, 3)
	add("band_blue_anchor", p2x+8+80, p2y+cellYOff+55, [3]float64{51.0 / 255, 107.0 / 255, 235.0 / 255}, 3)
	// P6 芯片色（拒绝可见，③ 窗「空格不出拒绝」教训同源）。
	add("refusal_chip_behind_eye", p6x+24+chipW/2, p6y+chipsYOff+chipW/2, [3]float64{200.0 / 255, 60.0 / 255, 60.0 / 255}, 3)
	add("refusal_chip_blend_skip", p6x+24+chipStep+chipW/2, p6y+chipsYOff+chipW/2, [3]float64{200.0 / 255, 60.0 / 255, 60.0 / 255}, 3)
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
			fmt.Fprintf(os.Stderr, "mat_persp: 探针 FAIL %s → %s\n", r.name, r.got)
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
		paintAll(pc.DC, size.Width, size.Height, true)
		_ = pc
	}
	root.Place(content, 0, 0)
	metric := wrkit.Label("presents=0", 13, 0.92, 0.94, 0.98)
	root.Place(metric, 8, 8)
	titles := [][2]any{
		{"P1 恒等材质+透视（对照|材质 逐位）", [2]float64{p1x + 4, 40}},
		{"P2 渐变带·透视正确插值（分界 y≈40）", [2]float64{p2x + 4, 40}},
		{"P3 bicubic 纹素 echo（对照|材质 逐位）", [2]float64{p3x + 4, 40}},
		{"P4 精灵路 material+persp（对照|材质）", [2]float64{p4x + 4, 416}},
		{"P5 投影层组合（对照|材质）", [2]float64{p5x + 4, 416}},
		{"P6 拒绝牌：眼后现场拒 · blend+材质 skip", [2]float64{p6x + 4, 416}},
	}
	for _, ti := range titles {
		pos := ti[1].([2]float64)
		root.Place(wrkit.Label(ti[0].(string), 11, 0.90, 0.92, 0.96), pos[0], pos[1])
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
	snapPath := filepath.Join(testdataDir, "ui_wr_p37_mat_persp_final.png")
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: backdropRGB, ClearG: backdropRGB, ClearB: backdropRGB + 0.02, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		MaxFrames:    gateMaxFrames,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "mat_persp: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
				}
			}
		},
	})
	var proc scheduler.ProcessTracker
	proc.Start()
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) bool {
		metric.SetText(fmt.Sprintf("presents=%d pairs=4", app.PresentCount()))
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
		fmt.Fprintf(os.Stderr, "mat_persp: FAIL 定帧停机没达成 presents=%d want>=%d\n",
			app.PresentCount(), gateMaxFrames)
		probesOK = false
	}

	tag := backendTag()
	basePath := filepath.Join(testdataDir, "goldens", fmt.Sprintf("ui_wr_p37_mat_persp_final_%s.png", tag))
	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGateW(snapPath, basePath, winW,
		[]wrkit.GoldenRect{{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH}})
	goldenOK := goldenErr == nil && !seeded && goldenDiff == 0
	if goldenErr != nil {
		fmt.Fprintf(os.Stderr, "mat_persp: golden: %v\n", goldenErr)
	}
	if seeded {
		fmt.Fprintf(os.Stderr, "mat_persp: golden 已播种（%s），这一跑不算 PASS —— 复核后提交基线再复跑\n", basePath)
	}

	pixelOK := paintErr == ""
	if gateMode {
		if pixelErr := wrkit.CheckPixelProbes(snapPath, winW, semanticProbes()); pixelErr != nil {
			pixelOK = false
			fmt.Fprintf(os.Stderr, "mat_persp: 像素断言 FAIL: %v\n", pixelErr)
		}
	} else {
		fmt.Fprintln(os.Stderr, "mat_persp: 常驻相末帧未定格（H15 只在门禁相成立），像素与金图只报数不判绿")
	}
	if paintErr != "" {
		fmt.Fprintf(os.Stderr, "mat_persp: 绘制 FAIL: %s\n", paintErr)
	}

	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	extra := map[string]any{
		"probes_ok": probesOK, "pixel_ok": pixelOK, "paint_err": paintErr,
		"golden_diff_pct": goldenDiff, "golden_total_px": goldenPx,
		"golden_seeded": seeded, "golden_backend_tag": tag,
		"gate_frames": gateFrames, "gate_freeze_at": gateFreezeAt,
		"golden_tolerance_pct": 0,
	}
	for k, v := range rep.extra {
		extra[k] = v
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "p37-mat-persp", Scenario: "ui_wr_p37_mat_persp", Backend: tag,
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
		fmt.Fprintf(os.Stderr, "mat_persp: OK backend=%s presents=%d probes=%d/%d golden=%.4f%% (%d px)\n",
			tag, app.PresentCount(), len(rep.results), len(rep.results), goldenDiff, goldenPx)
		return
	}
	fmt.Fprintf(os.Stderr, "mat_persp: resident done backend=%s presents=%d probes_ok=%v pixel_ok=%v golden=%.4f%%\n",
		tag, app.PresentCount(), probesOK, pixelOK, goldenDiff)
}
