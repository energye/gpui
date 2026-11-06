// Command p37_scene is P3.7 batch 6 window ⑥: the scene-flow close window.
//
// 收口对象（batch6-window-plan.md §5 归属修正后的第 ⑥ 扇）：
//   - 745 行（批 3e-1 scene 4×4 投影口）：Scene.SetProjection/PushProjection/
//     PopProjection 经 scene 流（录制 → Encoding → Decoder →
//     GPUSceneRenderer.RenderScene 回放）出图；
//   - 746 行（批 3e-2 quad+权重通道）：Scene.DrawImageQuad 梯形+权重出图；
//   - 751 行（批 3e-3 item 级材质编码）：Scene.SetMaterial 粘滞出图 +
//     两腿表态可见（GPU 腿装上真画、CPU 瓦片腿整笔拒画并计数——顺带把
//     批 3 收尾登记的「脏区腿材质表态」一并核掉）。
//
// engine/、ui/ 对 render/scene 零引用是批 3e-2 钉下的结构约束，本窗因此放
// examples/ 顶层 ui_wr_ 族（design `windows-3456-design.md` 一/五）。
//
// 链路真实性：JSON 带 Encoding.Size() 字节数、逐场景 tag 流计数
// （TagProjection/TagImageQuad/TagMaterial 载荷宽度全走查，走查式承接
// materialPayloadFromStream 钉的纪律——未登记 tag 直接判失败），不是直绘冒充。
//
// 三钉的窗内像素版：回放 ≡ 手写 Context 调用**逐位 diff=0**（投影/quad/材质
// 三路各自），每条都带反向鉴别（装与不装必须成片不同）与 painted>0 防空转。
//
// 满窗即内容：指标行浮左上（金图 mask 掉 y<32 带）；场景静态、定帧停机（H15）。
// 金图 goldens/p37_scene_final_{native,go}.png fail-closed。
//
// H16 两变异（窗体侧一行，跑完还原）：① 摘 SetProjection 发射（投影回仿射 ⇒
// A1 面板与 A2 恒等、鉴别探针红 + 金图报警）；② 摘 SetMaterial 发射（材质臂
// 回无材质 ⇒ C 面板渐变块回刷色 + 金图报警）。
//
// 跑法：
//
//	export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib DISPLAY=:0
//	RUN_SECONDS=8 go run ./examples/ui_wr_p37_scene -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/ui_wr_p37_scene -auto-only  # 播种，仍不算过
//	GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/ui_wr_p37_scene -auto-only       # 第二后端
//	go run ./examples/ui_wr_p37_scene -manual-seconds 6                              # 常驻人工
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/engine/renderconv"
	"github.com/energye/gpui/engine/tex"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200.0, 800.0
	metricStripH = 32.0
	winTitle     = "gpui p37_scene — scene 流收口（投影·quad·材质）"

	gateFrames   = 240
	gateFreezeAt = 120

	backdropRGB = 0.055
	cardRGB     = 0.10
)

// 面板几何（满窗即内容，七格；E 为缺口 4 档 A 的 tex 档位面板）。
const (
	pA1x, pA1y, pW1, pH1 = 12.0, 40.0, 372.0, 272.0  // 投影层（真投影 homography）
	pA2y                 = 40.0                      // 恒等对照
	pA3x, pA3y, pW3, pH3 = 804.0, 40.0, 384.0, 272.0 // Push/Pop 栈三态
	pBx, pBy, pW2, pH2   = 12.0, 328.0, 372.0, 256.0 // quad+权重腿
	pCx, pCy             = 408.0, 328.0              // 材质粘滞+清除
	pDx, pDy             = 804.0, 328.0              // 拒绝态+CPU 腿表态芯片
	pEx, pEy, pWE, pHE   = 12.0, 600.0, 372.0, 184.0 // tex 档位（单图路 DrawImageEx）
)

var (
	// projM：全 dyadic 元素的真投影 homography（float32 编码逐位精确往返，
	// 3e-1 钉同源纪律）；w 行取 1/256 让画面收成可见梯形而不退化。
	projM = render.Matrix4FromElements([16]float64{
		1, 0, 0.0625, 0,
		0, 1, 0.125, 0,
		0, 0, 1, 0,
		0, 0.00390625, 0, 1,
	})

	colRed   = render.RGBA{R: 0.86, G: 0.20, B: 0.22, A: 1}
	colBlue  = render.RGBA{R: 0.20, G: 0.42, B: 0.92, A: 1}
	colGreen = render.RGBA{R: 0.20, G: 0.78, B: 0.35, A: 1}
)

// 渐变材质（③ 窗同款常数：顶 (0.35) → 底 (1.0) 的琥珀 ramp）。
const gradWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let g = 0.35 + 0.65 * uv.y;
    return vec4<f32>(0.95 * g, 0.45 * g, 0.15 * g, g);
}`

func mustEffect(wgsl string) *render.ShaderEffect {
	e, err := render.NewShaderEffect(wgsl)
	if err != nil {
		panic(fmt.Sprintf("p37_scene: shader: %v", err))
	}
	return e
}

var gradEffect = mustEffect(gradWGSL)

// ---- 场景与图像构造 ----

func newCheckerImage(w, h int, cell int, a, b [3]uint8) *scene.Image {
	img := scene.NewImage(w, h)
	img.Data = make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := a
			if ((x/cell)+(y/cell))%2 == 1 {
				c = b
			}
			off := (y*w + x) * 4
			img.Data[off], img.Data[off+1], img.Data[off+2], img.Data[off+3] = c[0], c[1], c[2], 255
		}
	}
	return img
}

func rectFill(s *scene.Scene, r render.RGBA, x, y, w, h float64) {
	s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(r),
		scene.NewRectShape(float32(x), float32(y), float32(w), float32(h)))
}

// sceneProj：恒等蓝条打底 → SetProjection(projM) → 红块（投影变形）。
func sceneProj(w, h float64) *scene.Scene {
	s := scene.NewScene()
	rectFill(s, colBlue, 8, 8, w-16, 24)
	s.SetProjection(projM)
	rectFill(s, colRed, 20, 60, 160, 100)
	return s
}

// sceneFlat：同景不装投影（恒等对照）。
func sceneFlat(w, h float64) *scene.Scene {
	s := scene.NewScene()
	rectFill(s, colBlue, 8, 8, w-16, 24)
	rectFill(s, colRed, 20, 60, 160, 100)
	return s
}

// sceneStack：平 → Push(projM) 内块 → Pop → 平尾块；Push/Pop 各发一枚 tag。
func sceneStack(w, h float64) *scene.Scene {
	s := scene.NewScene()
	rectFill(s, colGreen, 8, 8, w-16, 40)
	s.PushProjection(projM)
	rectFill(s, colRed, 20, 80, 160, 100)
	s.PopProjection()
	rectFill(s, colBlue, 8, h-50, w-16, 40)
	return s
}

// quadCorners/weights：ProjectorMatrix4 出口「算好再喂」（2.5D 正路，不装
// scene 投影层——scene.go:170-175 明文两路不叠用）。
func quadGeometry(w, h float64) ([4]render.Point, [4]float64) {
	m, ok := render.ProjectorMatrix4(520, render.Point{X: w / 2, Y: h / 2}, render.Point{})
	if !ok {
		panic("p37_scene: ProjectorMatrix4 failed")
	}
	world := [4]render.Point{
		{X: 30, Y: 30}, {X: 30 + 180, Y: 30},
		{X: 30 + 180, Y: 30 + 120}, {X: 30, Y: 30 + 120},
	}
	depths := [4]float64{40, 240, 240, 40}
	corners, weights, ok := m.ProjectQuad(world, depths)
	if !ok {
		panic("p37_scene: ProjectQuad failed")
	}
	return corners, weights
}

// sceneQuad：透视梯形（权重非 1）+ 恒等对照平行四边形同图。
func sceneQuad(w, h float64, img *scene.Image, corners [4]render.Point, weights [4]float64) *scene.Scene {
	s := scene.NewScene()
	s.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Perspective: true, Weights: weights, Interpolation: render.InterpNearest, Opacity: 1,
	})
	s.DrawImageQuad(img, [4]render.Point{
		{X: 240, Y: 40}, {X: 330, Y: 40}, {X: 330, Y: 130}, {X: 240, Y: 130},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1})
	return s
}

// sceneMaterial：SetMaterial(渐变) → 填充块吃材质 → DrawImage 也吃 →
// SetMaterial(nil) 清除 → 后续平色块。粘滞+清除全部走 tag 流。
func sceneMaterial(w, h float64, img *scene.Image) *scene.Scene {
	s := scene.NewScene()
	s.SetMaterial(gradEffect)
	rectFill(s, colRed, 20, 20, 120, 120)
	s.DrawImage(img, scene.TranslateAffine(160, 20))
	s.SetMaterial(nil)
	rectFill(s, colGreen, 20, 160, 120, 60)
	return s
}

// sceneRefusal：透视 quad + 粘滞材质 —— quad 腿校验期具名拒（③ 口径：
// 不入队、不计数、该笔不画）。
// sceneRefusal：拒绝态要「看得见拒」——上下两枚无材质对照 quad 必画，
// 中段透视 quad 挂粘滞材质被校验期具名拒（不入队、不画、画面保持卡底），
// 清除材质后再一枚对照必画（拒不污染后续）。纯一格空是截图不出拒绝，
// 2026-11-06 用户肉眼指出后补的结构。
func sceneRefusal(img *scene.Image, corners [4]render.Point, weights [4]float64) *scene.Scene {
	s := scene.NewScene()
	s.DrawImageQuad(img, [4]render.Point{
		{X: 8, Y: 150}, {X: 88, Y: 150}, {X: 88, Y: 195}, {X: 8, Y: 195},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1})
	s.SetMaterial(gradEffect)
	s.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Perspective: true, Weights: weights, Interpolation: render.InterpNearest, Opacity: 1,
	})
	s.SetMaterial(nil)
	s.DrawImageQuad(img, [4]render.Point{
		{X: 110, Y: 150}, {X: 192, Y: 150}, {X: 192, Y: 195}, {X: 110, Y: 195},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1})
	return s
}

var (
	checkerImg = newCheckerImage(128, 128, 32, [3]uint8{224, 64, 64}, [3]uint8{64, 124, 224})
	quadCorn   [4]render.Point
	quadW      [4]float64

	scProj    *scene.Scene
	scFlat    *scene.Scene
	scStack   *scene.Scene
	scQuadS   *scene.Scene
	scMatS    *scene.Scene
	scRefusal *scene.Scene
)

// texPanelNoise：E 面板源图——128×128 LCG 确定性噪声（配方承接
// p37_mip_exit，种子 12345）。高频噪声让 2x 整数缩小的三档 mip 也必分叉
// （规则棋盘配整数倍缩小会采样点恒落同格、三档数学等图——④ 窗首版教训）。
var texPanelNoise = func() *render.ImageBuf {
	buf, err := render.NewImageBuf(128, 128, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	data := buf.Data()
	x := uint32(12345)
	for i := 0; i < 128*128; i++ {
		x = x*1664525 + 1013904223
		g := uint8(x >> 24)
		off := i * 4
		data[off], data[off+1], data[off+2], data[off+3] = g, g, g, 255
	}
	buf.NotifyPixelsChanged()
	return buf
}()

// texPanelFilters：E 面板三格的真源档位（与 ④ 臂 f 同口径）：e0 零值 Mip
// 透传（未选→MipmapNone 历史单层路）、e1 历史默认（bilinear+mip nearest）、
// e2 trilinear；aniso 全 ≤1（渲染路无承载字段）。
var texPanelFilters = []tex.Filter{
	{Near: tex.KindNearest, Far: tex.KindNearest},
	tex.DefaultFilter(),
	{Near: tex.KindLinear, Far: tex.KindLinear, Mip: tex.MipLinear, MaxAniso: tex.DefaultAniso},
}

// E 面板三格几何（面板内坐标）：128→64 整数 2x 缩小，LOD=1.0——None 取 0 层、
// Nearest 取 1 层、Linear 按 0.5/0.5 跨层混合，三格必互异。
const (
	texCellW   = 64.0
	texCellX0  = 24.0
	texCellGap = 112.0
	texCellY   = 72.0
)

// 窗内六场景与离屏探针同一份 200×200 几何：语义锚的落点直接从「画面所用
// 同一份回放输出」反推（redMaskBBox），不从公式反推结论（P3.5 教训）。
func buildScenes() {
	quadCorn, quadW = quadGeometry(200, 200)
	scProj = sceneProj(200, 200)
	scFlat = sceneFlat(200, 200)
	scStack = sceneStack(200, 200)
	scQuadS = sceneQuad(200, 200, checkerImg, quadCorn, quadW)
	scMatS = sceneMaterial(200, 200, checkerImg)
	scRefusal = sceneRefusal(checkerImg, quadCorn, quadW)
}

// ---- 走查（载荷宽度全消费，纪律承接 materialPayloadFromStream 钉） ----

type streamCounts struct {
	projection, quad, material, image, fill int
	projModes                               []render.ProjectionMode
	projMatrices                            []render.Matrix4
	quadCorners                             [4]render.Point
	quadWeights                             [4]float64
	quadPerspective                         bool
	materialIdx                             []uint32
	materialCleared                         []bool
	unregistered                            []string
}

func walkStream(enc *scene.Encoding) streamCounts {
	var c streamCounts
	dec := scene.NewDecoder(enc)
	for dec.Next() {
		switch dec.Tag() {
		case scene.TagProjection:
			mode, m := dec.Projection()
			c.projection++
			c.projModes = append(c.projModes, mode)
			c.projMatrices = append(c.projMatrices, m)
		case scene.TagImageQuad:
			_, corners, weights, persp, _, _ := dec.ImageQuad()
			c.quad++
			if c.quad == 1 {
				c.quadCorners, c.quadWeights, c.quadPerspective = corners, weights, persp
			}
		case scene.TagMaterial:
			idx, cleared := dec.Material()
			c.material++
			c.materialIdx = append(c.materialIdx, idx)
			c.materialCleared = append(c.materialCleared, cleared)
		case scene.TagImage:
			dec.Image()
			c.image++
		case scene.TagFill:
			dec.Fill()
			c.fill++
		case scene.TagStroke:
			dec.Stroke()
		case scene.TagFillRoundRect:
			dec.FillRoundRect()
		case scene.TagPushLayer:
			dec.PushLayer()
		case scene.TagSetAntiAlias:
			dec.AntiAlias()
		case scene.TagTransform:
			dec.Transform()
		case scene.TagText:
			dec.Text()
		case scene.TagBeginPath, scene.TagMoveTo, scene.TagLineTo, scene.TagQuadTo, scene.TagCubicTo,
			scene.TagClosePath, scene.TagEndPath, scene.TagPopLayer, scene.TagBeginClip, scene.TagEndClip, scene.TagBrush:
			// 不占 drawData 槽（几何在 pathData、变换在 transforms、刷子在 brushes 独立流）。
		default:
			c.unregistered = append(c.unregistered, fmt.Sprint(dec.Tag()))
		}
	}
	return c
}

// ---- 离屏回放（探针底座） ----

func replayInto(size int, build func(dc *render.Context)) *render.ImageBuf {
	dc := render.NewContext(size, size)
	dc.SetRGB(backdropRGB, backdropRGB, backdropRGB+0.02)
	dc.DrawRectangle(0, 0, float64(size), float64(size))
	_ = dc.Fill()
	build(dc)
	_ = dc.FlushGPU()
	src := dc.Image()
	out, err := render.NewImageBuf(size, size, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			if e := out.SetRGBA(x, y, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)); e != nil {
				panic(e)
			}
		}
	}
	_ = dc.Close()
	return out
}

func replayScene(size int, s *scene.Scene) *render.ImageBuf {
	return replayInto(size, func(dc *render.Context) {
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			panic(err)
		}
	})
}

// drawTexTrio 按 route 画 E 面板三格（缺口 4 档 A）：route="tex" 走真源
// tex.Filter→renderconv.FilterToRender 的引擎路；route="enum" 直写等价枚举对
// （鉴别臂）。两路同几何同源图，逐位 diff=0 才算携带忠实。E 面板走单图路
// DrawImageEx——scene 流不编码 mip（quad 口只带 interp，批 3e-2 口径），
// mip 轴证据只能在单图路上出，口径如实。
func drawTexTrio(dc render.Canvas, route string) {
	enumPairs := [3]struct {
		f render.InterpolationMode
		m render.MipmapMode
	}{{render.InterpNearest, render.MipmapNone}, {render.InterpBilinear, render.MipmapNearest}, {render.InterpBilinear, render.MipmapLinear}}
	for i, tf := range texPanelFilters {
		interp, mip := enumPairs[i].f, enumPairs[i].m
		if route == "tex" {
			var err error
			interp, mip, err = renderconv.FilterToRender(tf)
			if err != nil {
				panic(err)
			}
		}
		dc.DrawImageEx(texPanelNoise, render.DrawImageOptions{
			X: texCellX0 + float64(i)*texCellGap, Y: texCellY,
			DstWidth: texCellW, DstHeight: texCellW,
			Interpolation: interp, Mipmap: mip, Opacity: 1,
		})
	}
}

func diffBuf(a, b *render.ImageBuf) (n int, worst int) {
	w := a.Width()
	for y := 0; y < a.Height(); y++ {
		for x := 0; x < w; x++ {
			ar, ag, ab, _ := a.GetRGBA(x, y)
			br, bg, bb, _ := b.GetRGBA(x, y)
			for _, d := range [3]int{int(ar) - int(br), int(ag) - int(bg), int(ab) - int(bb)} {
				if d != 0 {
					n++
					if absInt(d) > worst {
						worst = absInt(d)
					}
					break
				}
			}
		}
	}
	return n, worst
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func mat4Equal(a, b render.Matrix4) bool {
	ea, eb := a.Elements(), b.Elements()
	for i := range ea {
		if ea[i] != eb[i] {
			return false
		}
	}
	return true
}

// redMaskBBox 扫 colRed 特征像素 (R>150, 40<G<90, 30<B<90) 的包络与计数——
// A1 语义锚的落点从画面所用同一份回放输出反推（不从公式反推结论）。
func redMaskBBox(buf *render.ImageBuf) (x0, y0, x1, y1, n int) {
	x0, y0, x1, y1 = 1<<30, 1<<30, -1, -1
	for y := 0; y < buf.Height(); y++ {
		for x := 0; x < buf.Width(); x++ {
			r, g, b, _ := buf.GetRGBA(x, y)
			if r > 150 && g > 40 && g < 90 && b > 30 && b < 90 {
				n++
				if x < x0 {
					x0 = x
				}
				if x > x1 {
					x1 = x
				}
				if y < y0 {
					y0 = y
				}
				if y > y1 {
					y1 = y
				}
			}
		}
	}
	if n == 0 {
		return 0, 0, 0, 0, 0
	}
	return x0, y0, x1, y1, n
}

var projRedCenter [2]int

// dcSkipProbe 回放一遍场景，返回 Context 侧 MaterialSkips 增量。
func dcSkipProbe(s *scene.Scene, size int) int {
	dc := render.NewContext(size, size)
	defer dc.Close()
	dc.ResetRenderPathStats()
	if e := scene.NewGPUSceneRenderer(dc).RenderScene(s); e != nil {
		panic(e)
	}
	_ = dc.FlushGPU()
	return dc.RenderPathStats().MaterialSkips
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
	const probeSize = 200

	// 探针 1：链路真实发生 —— 逐场景 Encoding 走查（tag 载荷全消费）。
	sProjP := sceneProj(probeSize, probeSize)
	sStackP := sceneStack(probeSize, probeSize)
	cQuadP := newCheckerImage(128, 128, 32, [3]uint8{224, 64, 64}, [3]uint8{64, 124, 224})
	cornersP, weightsP := quadGeometry(probeSize, probeSize)
	sQuadP := sceneQuad(probeSize, probeSize, cQuadP, cornersP, weightsP)
	sMatP := sceneMaterial(probeSize, probeSize, cQuadP)

	wProj := walkStream(sProjP.Encoding())
	wStack := walkStream(sStackP.Encoding())
	wQuad := walkStream(sQuadP.Encoding())
	wMat := walkStream(sMatP.Encoding())
	countsOK := len(wProj.unregistered) == 0 && len(wStack.unregistered) == 0 &&
		len(wQuad.unregistered) == 0 && len(wMat.unregistered) == 0 &&
		wProj.projection == 1 && wStack.projection == 2 &&
		wQuad.quad == 2 && wMat.material == 2 && wMat.image == 1 && wMat.fill == 2
	rep.add("stream_walk_counts", countsOK, fmt.Sprintf(
		"proj=%d stack=%d quad=%d mat=%d img=%d fill=%d unreg=%d%d%d%d",
		wProj.projection, wStack.projection, wQuad.quad, wMat.material, wMat.image, wMat.fill,
		len(wProj.unregistered), len(wStack.unregistered), len(wQuad.unregistered), len(wMat.unregistered)))
	rep.extra["enc_bytes"] = map[string]int{
		"proj": sProjP.Encoding().Size(), "stack": sStackP.Encoding().Size(),
		"quad": sQuadP.Encoding().Size(), "material": sMatP.Encoding().Size(),
	}
	rep.extra["enc_tags"] = map[string]int{
		"proj": len(sProjP.Encoding().Tags()), "stack": len(sStackP.Encoding().Tags()),
		"quad": len(sQuadP.Encoding().Tags()), "material": len(sMatP.Encoding().Tags()),
	}

	// 探针 2：投影 payload 回环——mode 与 16 元素逐值等（空流走安全访问，
	// 变异态下报红不 panic）。
	modeOK := wProj.projection >= 1 && wProj.projModes[0] == render.ProjectionSet
	elemOK := wProj.projection >= 1 && mat4Equal(wProj.projMatrices[0], projM)
	projRT := wProj.projection == 1 && modeOK && elemOK
	rep.add("projection_payload_roundtrip", projRT, fmt.Sprintf("proj=%d mode_set=%v 元素逐值等=%v",
		wProj.projection, modeOK, elemOK))

	// 探针 3：quad payload 回环——encoding 存 float32，期望=录制值的 float32 折算。
	quadRT := wQuad.quad == 2 && wQuad.quadPerspective
	for i := 0; i < 4 && quadRT; i++ {
		expC := render.Point{X: float64(float32(cornersP[i].X)), Y: float64(float32(cornersP[i].Y))}
		expW := float64(float32(weightsP[i]))
		if wQuad.quadCorners[i] != expC || wQuad.quadWeights[i] != expW {
			quadRT = false
		}
	}
	rep.add("quad_payload_roundtrip", quadRT, fmt.Sprintf("persp=%v w[0]=%.4f w[1]=%.4f", wQuad.quadPerspective, wQuad.quadWeights[0], wQuad.quadWeights[1]))

	// 探针 4：材质 payload 回环——[注册索引, 0xFFFFFFFF 清除]，注册表指向同一枚效果。
	clearIdx := uint32(0xFFFFFFFF)
	matRT := wMat.material == 2 && len(wMat.materialIdx) == 2 &&
		wMat.materialIdx[0] == 0 && !wMat.materialCleared[0] &&
		wMat.materialIdx[1] == clearIdx && wMat.materialCleared[1] &&
		len(sMatP.Materials()) == 1 && sMatP.Materials()[0] == gradEffect
	rep.add("material_payload_roundtrip_with_clear", matRT, fmt.Sprintf("idx=%v cleared=%v registry=%d", wMat.materialIdx, wMat.materialCleared, len(sMatP.Materials())))

	// 探针 5：回放 ≡ 手写（投影路）逐位 + 反向鉴别 + 非空转。
	// 手写臂顺序与场景一致：蓝条在无投影段，红块在 SetProjection 之后。
	replayProj := replayInto(probeSize, func(dc *render.Context) {
		_ = scene.NewGPUSceneRenderer(dc).RenderScene(sProjP)
	})
	handProj := replayInto(probeSize, func(dc *render.Context) {
		dc.SetRGB(colBlue.R, colBlue.G, colBlue.B)
		dc.DrawRectangle(8, 8, probeSize-16, 24)
		_ = dc.Fill()
		dc.SetProjection(projM)
		dc.SetRGB(colRed.R, colRed.G, colRed.B)
		dc.DrawRectangle(20, 60, 160, 100)
		_ = dc.Fill()
	})
	flatProbe := replayInto(probeSize, func(dc *render.Context) {
		_ = scene.NewGPUSceneRenderer(dc).RenderScene(sceneFlat(probeSize, probeSize))
	})
	n5a, _ := diffBuf(replayProj, handProj)
	n5b, _ := diffBuf(replayProj, flatProbe)
	_, _, _, _, nRed := redMaskBBox(replayProj)
	// A1 语义锚：与窗体 paintAll 同形（Translate(12,40)+回放）离屏出一张
	// 全窗尺寸的画，红 mask bbox 中心即呈现帧里红块的落点——投影在设备域
	// 生效（1d 钉的链序口径），直接拿无 translate 的探针 bbox 会偏。
	winShape := render.NewContext(int(winW), int(winH))
	winShape.SetRGB(backdropRGB, backdropRGB, backdropRGB+0.02)
	winShape.DrawRectangle(0, 0, winW, winH)
	_ = winShape.Fill()
	winShape.Push()
	winShape.Translate(pA1x, pA1y)
	winShape.SetRGB(cardRGB, cardRGB, cardRGB+0.04)
	winShape.DrawRectangle(0, 0, pW1, pH1)
	_ = winShape.Fill()
	if e := scene.NewGPUSceneRenderer(winShape).RenderScene(scProj); e != nil {
		panic(e)
	}
	winShape.Pop()
	_ = winShape.FlushGPU()
	wsBuf, wsErr := render.NewImageBuf(int(winW), int(winH), render.FormatRGBA8)
	if wsErr != nil {
		panic(wsErr)
	}
	wsImg := winShape.Image()
	for y := 0; y < int(winH); y++ {
		for x := 0; x < int(winW); x++ {
			r, g, b, a := wsImg.At(x, y).RGBA()
			if e := wsBuf.SetRGBA(x, y, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)); e != nil {
				panic(e)
			}
		}
	}
	wx0, wy0, wx1, wy1, wN := redMaskBBox(wsBuf)
	projRedCenter = [2]int{(wx0 + wx1) / 2, (wy0 + wy1) / 2}
	rep.extra["a1_red_window_bbox"] = []int{wx0, wy0, wx1, wy1, wN}
	wsBuf.Dispose()
	winShape.Close()
	rep.extra["a1_red_probe_bbox_count"] = nRed
	rep.add("replay_projection_equals_handwritten", n5a == 0 && nRed > 400 && wN > 400,
		fmt.Sprintf("逐位 diff=%d；vs 不装投影 %d 像素（鉴别）；探针红像素=%d 窗形=%d bbox=(%d,%d)-(%d,%d)", n5a, n5b, nRed, wN, wx0, wy0, wx1, wy1))
	rep.add("projection_layer_not_shell", n5b >= 100, fmt.Sprintf("装投影 vs 不装 = %d 像素差（≥100 才算口没成空壳）", n5b))

	// 探针 6：Push/Pop 栈回放 ≡ 手写 push/pop 序逐位。
	replayStack := replayInto(probeSize, func(dc *render.Context) {
		_ = scene.NewGPUSceneRenderer(dc).RenderScene(sStackP)
	})
	handStack := replayInto(probeSize, func(dc *render.Context) {
		dc.SetRGB(colGreen.R, colGreen.G, colGreen.B)
		dc.DrawRectangle(8, 8, probeSize-16, 40)
		_ = dc.Fill()
		dc.PushProjection(projM)
		dc.SetRGB(colRed.R, colRed.G, colRed.B)
		dc.DrawRectangle(20, 80, 160, 100)
		_ = dc.Fill()
		dc.PopProjection()
		dc.SetRGB(colBlue.R, colBlue.G, colBlue.B)
		dc.DrawRectangle(8, probeSize-50, probeSize-16, 40)
		_ = dc.Fill()
	})
	n6, _ := diffBuf(replayStack, handStack)
	rep.add("replay_stack_equals_handwritten", n6 == 0 && wStack.projection == 2, fmt.Sprintf("逐位 diff=%d，push+pop 共 %d 枚 tag", n6, wStack.projection))

	// 探针 7：quad+权重回放 ≡ 手写 DrawImageQuad 逐位 + 透视开关鉴别。
	camImg := imageBufFromScene(checkerImg)
	replayQ := replayInto(probeSize, func(dc *render.Context) {
		_ = scene.NewGPUSceneRenderer(dc).RenderScene(sQuadP)
	})
	handQ := replayInto(probeSize, func(dc *render.Context) {
		dc.DrawImageQuad(camImg, cornersP, render.QuadDrawOptions{
			Perspective: true, Weights: weightsP, Interpolation: render.InterpNearest, Opacity: 1,
		})
		dc.DrawImageQuad(camImg, [4]render.Point{
			{X: 240, Y: 40}, {X: 330, Y: 40}, {X: 330, Y: 130}, {X: 240, Y: 130},
		}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1})
	})
	flatQ := replayInto(probeSize, func(dc *render.Context) {
		dc.DrawImageQuad(camImg, cornersP, render.QuadDrawOptions{
			Interpolation: render.InterpNearest, Opacity: 1,
		})
		dc.DrawImageQuad(camImg, [4]render.Point{
			{X: 240, Y: 40}, {X: 330, Y: 40}, {X: 330, Y: 130}, {X: 240, Y: 130},
		}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1})
	})
	n7a, _ := diffBuf(replayQ, handQ)
	n7b, _ := diffBuf(replayQ, flatQ)
	rep.add("replay_quad_weights_equals_handwritten", n7a == 0, fmt.Sprintf("逐位 diff=%d；vs 透视关 %d 像素（鉴别）", n7a, n7b))
	rep.add("quad_perspective_not_shell", n7b >= 100, fmt.Sprintf("梯形权重真透视 vs 关掉 = %d 像素差", n7b))
	camImg.Dispose()

	// 探针 8：材质粘滞回放 ≡ 手写「装 shader→画→摘→再画」逐位 + 无材质鉴别。
	replayM := replayInto(probeSize, func(dc *render.Context) {
		_ = scene.NewGPUSceneRenderer(dc).RenderScene(sMatP)
	})
	handM := replayInto(probeSize, func(dc *render.Context) {
		dc.SetShader(gradEffect)
		dc.SetRGB(colRed.R, colRed.G, colRed.B)
		dc.DrawRectangle(20, 20, 120, 120)
		_ = dc.Fill()
		dc.SetShader(nil)
		// 粘滞账：场景里 SetMaterial(nil) 在 DrawImage 之前没发生——图像臂带的
		// 是 item 级材质（opts.Material），手写腿必须同样交 Material。
		imgBuf := imageBufFromScene(checkerImg)
		dc.DrawImageEx(imgBuf, render.DrawImageOptions{
			X: 160, Y: 20, DstWidth: 128, DstHeight: 128,
			Opacity: 1, Material: gradEffect,
		})
		imgBuf.Dispose()
		dc.SetRGB(colGreen.R, colGreen.G, colGreen.B)
		dc.DrawRectangle(20, 160, 120, 60)
		_ = dc.Fill()
	})
	noMat := replayInto(probeSize, func(dc *render.Context) {
		s := scene.NewScene()
		s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colRed), scene.NewRectShape(20, 20, 120, 120))
		s.DrawImage(checkerImg, scene.TranslateAffine(160, 20))
		s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colGreen), scene.NewRectShape(20, 160, 120, 60))
		_ = scene.NewGPUSceneRenderer(dc).RenderScene(s)
	})
	n8a, _ := diffBuf(replayM, handM)
	n8b, _ := diffBuf(replayM, noMat)
	rep.add("replay_material_equals_handwritten", n8a == 0, fmt.Sprintf("逐位 diff=%d；vs 无材质 %d 像素（粘滞可见性）", n8a, n8b))
	rep.add("material_sticky_visible_not_shell", n8b >= 100, fmt.Sprintf("材质块 vs 无材质 = %d 像素差", n8b))

	// 探针 9：透视 quad + 材质 —— ③ 口径复现：校验期具名拒、不入队不计数，
	// 该笔不画（落点=同底色 control），RenderPathStats.MaterialSkips 不动。
	refused := sceneRefusal(checkerImg, cornersP, weightsP)
	onlyMat := scene.NewScene()
	onlyMat.SetMaterial(gradEffect)
	var rr9 *scene.GPUSceneRenderer
	refImg := replayInto(probeSize, func(dc *render.Context) {
		dc.ResetRenderPathStats()
		rr9 = scene.NewGPUSceneRenderer(dc)
		if e := rr9.RenderScene(refused); e != nil {
			panic(e)
		}
	})
	skipAfter := dcSkipProbe(refused, probeSize)
	ctrlImg := replayInto(probeSize, func(dc *render.Context) {
		_ = scene.NewGPUSceneRenderer(dc).RenderScene(onlyMat)
	})
	xp, yp := int(cornersP[0].X)+12, int(cornersP[0].Y)+12
	r1, g1, b1, _ := refImg.GetRGBA(xp, yp)
	r0, g0, b0, _ := ctrlImg.GetRGBA(xp, yp)
	refusedOK := skipAfter == 0 && r1 == r0 && g1 == g0 && b1 == b0 &&
		absInt(int(r1)-int(r0))+absInt(int(g1)-int(g0))+absInt(int(b1)-int(b0)) == 0
	rep.add("quad_perspective_material_refused", refusedOK,
		fmt.Sprintf("MaterialSkips=%d 落点(%d,%d)=(%d,%d,%d) control=(%d,%d,%d)（不画且逐位同底）",
			skipAfter, xp, yp, r1, g1, b1, r0, g0, b0))
	refImg.Dispose()
	ctrlImg.Dispose()

	// 探针 15（H22，2026-11-07）：回放腿拒绝账——同一幕拒景回放后，拒绝必须
	// 可查（计数=1、错误具名 ErrQuadMaterialUnsupported），不再 `_ =` 吞掉；
	// 画面语义不变（拒笔照旧不画，探针 9 的像素断言继续管）。
	h22OK := rr9 != nil && rr9.Refusals() == 1 && errors.Is(rr9.LastRefusal(), render.ErrQuadMaterialUnsupported)
	rep.add("h22_replay_quad_refusal_accounted", h22OK,
		fmt.Sprintf("refusals=%d last=%v（回放腿拒绝账具名可查）", rr9.Refusals(), rr9.LastRefusal()))

	// 探针 10：CPU 脏区腿两态——材质 fill 整笔拒画并计数（批 3 收尾登记的
	// 「脏区腿材质表态」在此核掉）；投影下的 image 走的是另一条拒画计数
	// （ProjectionDegrades）——两态各配各的门，材质粘滞时先被材质门拦下，
	// 所以投影那条必须在「材质已清除」的段里测。
	sBoth := scene.NewScene()
	sBoth.SetMaterial(gradEffect)
	sBoth.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colRed), scene.NewRectShape(10, 10, 60, 60))
	sBoth.SetMaterial(nil)
	sBoth.SetProjection(projM)
	sBoth.DrawImage(checkerImg, scene.TranslateAffine(90, 10))
	rr2 := scene.NewRenderer(probeSize, probeSize, scene.WithWorkers(1))
	pm := render.NewPixmap(probeSize, probeSize)
	rr2.MarkAllDirty()
	if e := rr2.RenderDirty(pm, sBoth, nil); e != nil {
		rep.add("cpu_leg_stances", false, fmt.Sprintf("RenderDirty: %v", e))
	} else {
		skips, degr := rr2.MaterialSkips(), rr2.ProjectionDegrades()
		// 被拒的笔不裸画：材质 fill 落点应为瓦片腿底色（不出现材质/刷色），
		// 取 (40,40) 一点核对不是 colRed。
		c := pm.GetPixel(40, 40)
		notPainted := c.R < colRed.R-0.05
		rep.add("cpu_leg_stances", skips >= 1 && degr >= 1 && notPainted,
			fmt.Sprintf("material_skips=%d projection_degrades=%d 材质fill落点R=%.3f(不裸画)", skips, degr, c.R))
	}
	rr2.Close()

	// 探针 11：零发射纪律——不调三动词的场景 tag 流与 Hash 不被投影/材质污染；
	// SetProjection(Matrix4Identity()) 与零值同「按恒等关闭」不发 tag。
	sA := scene.NewScene()
	sA.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colBlue), scene.NewRectShape(4, 4, 20, 20))
	sB := scene.NewScene()
	sB.SetProjection(render.Matrix4{})
	sB.SetProjection(render.Matrix4Identity())
	sB.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colBlue), scene.NewRectShape(4, 4, 20, 20))
	wB := walkStream(sB.Encoding())
	rep.add("identity_and_zero_projection_emit_nothing",
		wB.projection == 0 && sA.Encoding().Hash() == sB.Encoding().Hash(),
		fmt.Sprintf("proj_tags=%d hash等=%v", wB.projection, sA.Encoding().Hash() == sB.Encoding().Hash()))

	// 探针 12：Hash 吞三态——投影/材质进入场景身份（LayerCache 键），
	// 与不装必须不同。
	plainRef := scene.NewScene()
	plainRef.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colBlue), scene.NewRectShape(4, 4, 20, 20))
	hashOK := sProjP.Encoding().Hash() != sceneFlat(probeSize, probeSize).Encoding().Hash() &&
		sMatP.Encoding().Hash() != plainRef.Encoding().Hash()
	rep.add("hash_swallows_projection_and_material", hashOK, "投影/材质进入 Encoding.Hash（与不装必不同）")

	// 探针 13：tex 档位面板（缺口 4 档 A）——单图路 DrawImageEx 携带三件套：
	// tex 路 ≡ 枚举路逐位（转换忠实携带）、三档两两互异（mip 真进画面）。
	// 128→64 整数 2x：None 取 0 层、Nearest 取 1 层、Linear 0.5/0.5 跨层混合。
	texTrio := replayInto(probeSize, func(dc *render.Context) { drawTexTrio(dc, "tex") })
	enumTrio := replayInto(probeSize, func(dc *render.Context) { drawTexTrio(dc, "enum") })
	nEq, _ := diffBuf(texTrio, enumTrio)
	rep.add("tex_road_bitwise_matches_enum_road", nEq == 0,
		fmt.Sprintf("tex路↔枚举路逐位 diff=%d（同几何同源图，转换忠实）", nEq))
	texRegion := func(buf *render.ImageBuf, i int) []byte {
		out := make([]byte, 0, int(texCellW)*int(texCellW)*3)
		x0 := int(texCellX0 + float64(i)*texCellGap)
		for y := int(texCellY); y < int(texCellY+texCellW); y++ {
			for x := x0; x < x0+int(texCellW); x++ {
				r, g, b, _ := buf.GetRGBA(x, y)
				out = append(out, r, g, b)
			}
		}
		return out
	}
	byteDiff := func(a, b []byte) int {
		n := 0
		for i := range a {
			if a[i] != b[i] {
				n++
			}
		}
		return n
	}
	d01 := byteDiff(texRegion(texTrio, 0), texRegion(texTrio, 1))
	d02 := byteDiff(texRegion(texTrio, 0), texRegion(texTrio, 2))
	d12 := byteDiff(texRegion(texTrio, 1), texRegion(texTrio, 2))
	rep.extra["tex_mip_pair_diffs"] = map[string]int{"none_vs_nearest": d01, "none_vs_linear": d02, "nearest_vs_linear": d12}
	rep.add("tex_road_mip_three_levels_differ", d01 > 200 && d02 > 200 && d12 > 200,
		fmt.Sprintf("none↔nearest %d none↔linear %d nearest↔linear %d 字节", d01, d02, d12))
	texTrio.Dispose()
	enumTrio.Dispose()

	// 探针 14：FilterToRender 拒绝三态（混合轴/未选轴/aniso>1 具名拒绝；
	// 不许借采样折叠的屏蔽放行——nearest 组合的 aniso>1 同样拒）。
	mixedF := tex.Filter{Near: tex.KindNearest, Far: tex.KindLinear, Mip: tex.MipNearest, MaxAniso: tex.DefaultAniso}
	unsetF := tex.Filter{Near: tex.KindLinear, Far: tex.KindLinear, Mip: tex.MipNearest, MaxAniso: tex.DefaultAniso}
	unsetF.Near = tex.KindUnknown
	_, _, errMixed := renderconv.FilterToRender(mixedF)
	_, _, errUnset := renderconv.FilterToRender(unsetF)
	_, _, errAniso := renderconv.FilterToRender(tex.RealisticFilter())
	refuseOK := errors.Is(errMixed, renderconv.ErrFilterMixedAxes) &&
		errors.Is(errUnset, renderconv.ErrFilterUnknownKind) &&
		errors.Is(errAniso, renderconv.ErrFilterAnisoUnsupported)
	rep.add("filtertorender_refuses_uncarryable", refuseOK,
		fmt.Sprintf("mixed=%v unset=%v aniso4=%v", errMixed, errUnset, errAniso))

	return rep
}

func imageBufFromScene(img *scene.Image) *render.ImageBuf {
	buf, err := render.NewImageBuf(img.Width, img.Height, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	copy(buf.Data(), img.Data)
	buf.NotifyPixelsChanged()
	return buf
}

// ---- 窗口 ----

func paintAll(pc *rendering.PaintContext, size rendering.Size) {
	dc := pc.DC
	w, h := size.Width, size.Height
	dc.SetRGB(backdropRGB, backdropRGB, backdropRGB+0.02)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	c := dc
	type panel struct {
		x, y, w, hh float64
		title       string
		s           *scene.Scene
	}
	panels := []panel{
		{pA1x, pA1y, pW1, pH1, "A1 投影层", scProj},
		{408, pA2y, pW1, pH1, "A2 恒等对照", scFlat},
		{pA3x, pA3y, pW3, pH3, "A3 Push/Pop 栈", scStack},
		{pBx, pBy, pW2, pH2, "B quad+权重", scQuadS},
		{pCx, pCy, pW2, pH2, "C 材质粘滞+清除", scMatS},
		{pDx, pDy, pW3, pH2, "D 对照-拒-对照", scRefusal},
	}
	for _, p := range panels {
		c.Push()
		c.Translate(p.x, p.y)
		c.SetRGB(cardRGB, cardRGB, cardRGB+0.04)
		c.DrawRectangle(0, 0, p.w, p.hh)
		_ = c.Fill()
		// H21 修复后场景回放是可录制动词：录制腿进显示表、回放腿在
		// CanvasBackend 执行；live 腿直画。两条路呈现同一画面。
		if err := scene.RenderSceneTo(c, p.s); err != nil {
			fmt.Fprintf(os.Stderr, "p37_scene: RenderSceneTo %s: %v\n", p.title, err)
		}
		c.Pop()
	}

	// E tex 档位面板（缺口 4 档 A，2026-11-06）：scene 流不编码 mip（quad 口
	// 只带 interp，批 3e-2 明确），mip 轴证据在单图路 DrawImageEx——真源
	// tex.Filter 经 renderconv.FilterToRender 折出 (Interpolation, Mipmap)
	// 窗上直画；跨路逐位等价与三档互异由探针 13/14 离屏核对。
	c.Push()
	c.Translate(pEx, pEy)
	c.SetRGB(cardRGB, cardRGB, cardRGB+0.04)
	c.DrawRectangle(0, 0, pWE, pHE)
	_ = c.Fill()
	drawTexTrio(c, "tex")
	c.Pop()
}

func semanticProbes() []wrkit.PixelProbe {
	probes := make([]wrkit.PixelProbe, 0, 4)
	// A2 恒等面板红块中心：GPU 场景回放必须出现在呈现帧里（防空帧假绿）。
	probes = append(probes, wrkit.PixelProbe{Name: "flat_panel_red",
		X: 408 + 100, Y: pA2y + 110,
		R: colRed.R, G: colRed.G, B: colRed.B, Tol: 3})
	// A1 投影面板：红块中心 = 与窗体同形（Translate+回放）离屏输出的红 mask
	// bbox 中心，已是窗坐标（落点从画面同源反推，不从公式反推结论）。
	probes = append(probes, wrkit.PixelProbe{Name: "projected_panel_red",
		X: float64(projRedCenter[0]), Y: float64(projRedCenter[1]),
		R: colRed.R, G: colRed.G, B: colRed.B, Tol: 4})
	// C 面板材质渐变块中点：uv.y=(80+0.5−20)/120=0.504，g=0.35+0.65·uv.y。
	// 引擎把 shader 输出按 premul 直出（③ d9 实测 (255,128,128) 钉同一口径）：
	// out = srcRGB + card·(1−a)，src=(0.95g,0.45g,0.15g)、a=g。
	uvy := (80.0 + 0.5 - 20) / 120.0
	g := 0.35 + 0.65*uvy
	probes = append(probes, wrkit.PixelProbe{Name: "material_gradient_mid",
		X: pCx + 80, Y: pCy + 80,
		R: 0.95*g + cardRGB*(1-g), G: 0.45*g + cardRGB*(1-g), B: 0.15*g + (cardRGB+0.04)*(1-g), Tol: 5})
	// C 面板清除后平色块：SetMaterial(nil) 之后的 fill 回到不透明刷色。
	probes = append(probes, wrkit.PixelProbe{Name: "material_cleared_plain",
		X: pCx + 80, Y: pCy + 190,
		R: colGreen.R, G: colGreen.G, B: colGreen.B, Tol: 3})
	// D 面板对照①（无材质 affine quad，必画）：u=(40-8)/80=0.4、v=(168-150)/45=0.4
	// → 纹素 (51,51)，(51/32+51/32)%2=0 → 红 (224,64,64)。
	probes = append(probes, wrkit.PixelProbe{Name: "refusal_contrast_left_drawn",
		X: pDx + 40, Y: pDy + 168,
		R: 224.0 / 255, G: 64.0 / 255, B: 64.0 / 255, Tol: 3})
	// D 面板对照②（清除材质后必画）：u=(150.5-110)/82≈0.494、v=(180.5-150)/45≈0.678
	// → 纹素 (63,86)，(1+2)%2=1 → 蓝 (64,124,224)。
	probes = append(probes, wrkit.PixelProbe{Name: "refusal_contrast_right_drawn",
		X: pDx + 150, Y: pDy + 180,
		R: 64.0 / 255, G: 124.0 / 255, B: 224.0 / 255, Tol: 3})
	// D 面板中段：透视 quad+粘滞材质=校验期具名拒不画 ⇒ 该点保持卡片底
	// (cardRGB,cardRGB,cardRGB+0.04)——拒绝态可见（对照两角必画）。
	probes = append(probes, wrkit.PixelProbe{Name: "refusal_region_stays_card",
		X: pDx + 70, Y: pDy + 60,
		R: cardRGB, G: cardRGB, B: cardRGB + 0.04, Tol: 6})
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
	_ = os.MkdirAll(filepath.Join("examples", "ui_wr_p37_scene", "testdata"), 0o755)

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly && !secsSet {
		secs = 8
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	}
	gateMode := *autoOnly
	wrkit.EnsureUIFace()
	buildScenes()

	t0 := time.Now()
	rep := runProbes()
	probesOK := true
	for _, r := range rep.results {
		if !r.ok {
			probesOK = false
			fmt.Fprintf(os.Stderr, "p37_scene: 探针 FAIL %s → %s\n", r.name, r.got)
		}
	}
	var summary manualSummary

	testdataDir := filepath.Join("examples", "ui_wr_p37_scene", "testdata")
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
		paintAll(pc, size)
	}
	root.Place(content, 0, 0)
	metric := wrkit.Label("presents=0", 13, 0.92, 0.94, 0.98)
	root.Place(metric, 8, 8)
	titles := []string{"A1 scene 投影层（homography）", "A2 恒等对照", "A3 Push/Pop 栈",
		"B quad+权重（算好再喂）", "C 材质粘滞+清除", "D 对照(必画)·拒(不画)·对照(必画)",
		"E tex 档位（单图路 · 缺口4档A）"}
	tx := []float64{pA1x, 408, pA3x, pBx, pCx, pDx, pEx}
	for i := range titles {
		row := i / 3
		y := 26.0
		if row == 1 {
			y = pBy - 14
		}
		if row == 2 {
			y = pEy - 14
		}
		root.Place(wrkit.Label(titles[i], 11, 0.90, 0.92, 0.96), tx[i]+4, y)
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
	snapPath := filepath.Join(testdataDir, "p37_scene_final.png")
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: backdropRGB, ClearG: backdropRGB, ClearB: backdropRGB + 0.02, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		MaxFrames:    gateMaxFrames,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "p37_scene: close (%s)\n", win.Backend())
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
		metric.SetText(fmt.Sprintf("presents=%d scenes=6 tex=1 live_skips=%d", app.PresentCount(), rendering.LiveOnlySkips()))
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
		fmt.Fprintf(os.Stderr, "p37_scene: FAIL 定帧停机没达成 presents=%d want>=%d\n",
			app.PresentCount(), gateMaxFrames)
		probesOK = false
	}

	tag := backendTag()
	basePath := filepath.Join(testdataDir, "goldens", fmt.Sprintf("p37_scene_final_%s.png", tag))
	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGateW(snapPath, basePath, winW,
		[]wrkit.GoldenRect{{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH}})
	goldenOK := goldenErr == nil && !seeded && goldenDiff == 0
	if goldenErr != nil {
		fmt.Fprintf(os.Stderr, "p37_scene: golden: %v\n", goldenErr)
	}
	if seeded {
		fmt.Fprintf(os.Stderr, "p37_scene: golden 已播种（%s），这一跑不算 PASS —— 复核后提交基线再复跑\n", basePath)
	}

	pixelOK := true
	if gateMode {
		if pixelErr := wrkit.CheckPixelProbes(snapPath, winW, semanticProbes()); pixelErr != nil {
			pixelOK = false
			fmt.Fprintf(os.Stderr, "p37_scene: 像素断言 FAIL: %v\n", pixelErr)
		}
	} else {
		fmt.Fprintln(os.Stderr, "p37_scene: 常驻相末帧未定格（H15 只在门禁相成立），像素与金图只报数不判绿")
	}
	liveSkips := rendering.LiveOnlySkips()

	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	extra := map[string]any{
		"probes_ok": probesOK, "pixel_ok": pixelOK,
		"golden_diff_pct": goldenDiff, "golden_total_px": goldenPx,
		"golden_seeded": seeded, "golden_backend_tag": tag,
		"gate_frames": gateFrames, "gate_freeze_at": gateFreezeAt,
		"golden_tolerance_pct": 0,
		"live_only_skips":      liveSkips,
	}
	for k, v := range rep.extra {
		extra[k] = v
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "p37-6-6", Scenario: "p37_scene", Backend: tag,
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
		if liveSkips != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: live_only_skips=%d（FullPaint 下每帧应 live 走查，非 0=scene 回放被跳过）\n", liveSkips)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "p37_scene: OK backend=%s presents=%d probes=%d/%d golden=%.4f%% (%d px) skips=%d\n",
			tag, app.PresentCount(), len(rep.results), len(rep.results), goldenDiff, goldenPx, liveSkips)
		return
	}
	fmt.Fprintf(os.Stderr, "p37_scene: resident done backend=%s presents=%d probes_ok=%v pixel_ok=%v golden=%.4f%% skips=%d\n",
		tag, app.PresentCount(), probesOK, pixelOK, goldenDiff, liveSkips)
}
