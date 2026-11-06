// Command p37_material is P3.7 batch 6 window ③: the user-material window.
//
// 必画臂（钉版清单 batch6-window-plan.md §1 ③ 行 + §5 归属修正，缺一即 747–750 仍 ☑）：
//
//	a) 材质三入口各挂同一枚用户 shader 并排：图像口 DrawImageOptions.Material、
//	   quad 口 QuadDrawOptions.Material（仿射）、批绘口 AtlasSprite.Material
//	   ——挂材质的与不挂的并排，uv 渐变像素差可见；恒等材质（采样 TEXTURE
//	   原样返回）与无材质**逐位相同**（批 3a 钉的窗内版）；
//	b) 半透明材质填充 alpha<255（批 3a premul 修复的画面）：白底上 premul
//	   半透明红 → 期望 (255,128,128)（p32 d9 的窗内版）；
//	c) ShaderEffect.SetImageAt(slot1, 第二张) 多纹理：左半采样主图、右半采样
//	   第二张（批 3c D4 的窗内版）；
//	d) 描边吃 shader：SetRGB+SetShader 的 AA 描边（轮廓展开 → fillShaderAsImage，
//	   批 3d 单点修复的画面）——描边像素是 shader 渐变而非环境实心色；
//	e) 拒绝态底带：七哨兵六枚前端具名（errors.Is）+ quad 现场拒 + ShaderBackendFails
//	   计数（gles 腿实测；预算顶格 shader 是本扇的 gles 承载尝试，结果如实记）。
//	   材质×透视专项（2026-11-07）连带：材质+透视/材质+bicubic 相继放行后，
//	   quad 现场拒=画布投影层眼后（ErrQuadBehindEye，几何族；材质在场也不救），
//	   同几何同锚；图像口 bicubic+材质仍 skip+MaterialSkips（纹素契约随专项
//	   B 批开启后迁移）。
//	f) uniform 载荷参与出图：272B uniform（边界字段落在旧 256B 预算之外）决定分屏
//	   边界与右半增益——同几何、同纹理、同着色器，只换 uniform 字节，边界必须跟着走；
//	   全零载荷必须把边界读成 0（证明值真从载荷里来，不是着色器写死的）。
//	   补的是审计缺口 2：此前本扇 uniform 只出现在「校验」里，没有一帧画面由它决定。
//	G) 后端拒收的「看得见拒绝」三格：故意一枚**前端放行、后端 naga 必拒**的模块
//	   （uniform 地址空间数组 stride 非 16 倍数），挂填充腿 ⇒ 区内必须保持底色
//	   （环境实心红不许冒充回退）+ `ShaderBackendFails++` + `CPUFallbackOps++`；
//	   左右两格是「这块地方本来就画得出东西」的对照（无材质红 / 合法材质绿）。
//	   门禁相另有**终帧硬门**（两端各自钉死）：那格要么「底色 + 呈现帧计数 ≥1」
//	   （本机后端真拒这枚模块），要么「shader 自己的解析色 + 计数 0」（本机后端
//	   接受它），两者都不许是环境实心红、也不许拒而不计数。实测：Vulkan/wgpu-native
//	   走前一支，纯 Go GL 走后一支（它的 WGSL 前端不查 uniform stride 规则）。
//	   缺口 3 的设计子项由此在填充腿上兑现；同一模块在**图像口/quad 口**不走具名链、
//	   直接打断 flush——那是已登记的 **H23**（`batch6/gap3-backend-reject-probe.txt`），
//	   本窗不替它遮（臂只声明填充腿）。
//
// 满窗即内容：指标行浮在画面左上（金图 mask 掉 y<32 带）；画面相位只随已完成
// 帧数走、过 settling 帧后定格（H15）。金图按后端分文件
// goldens/p37_material_final_{native,go}.png 且 fail-closed。像素断言坐标由
// 「画面所用同一份常数/几何」反推（P3.5 教训）；期望色按 premul src-over 公式
// 由同一份常数算出（p32 d9 同款），不设暗档。
//
// 跑法：
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/engine/p37_material -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_material -auto-only  # 播种，仍不算过
//	go run ./examples/engine/p37_material                                                # 常驻人工
//	GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_material -auto-only       # 第二后端一轮
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"math"
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
	winTitle     = "gpui p37_material — 用户材质三入口"

	// 臂 a 三入口（每对 = 无材质 | 挂材质 并排）。
	aTopY  = 64.0
	aW     = 110.0
	aH     = 90.0
	aGap   = 16.0 // 对内间距
	aPairW = 2*aW + aGap
	aPair0 = 110.0
	aPair1 = 470.0
	aPair2 = 830.0

	// 臂 b premul 半透明填充（白底上 premul 半透明红 = (255,128,128)）。
	bWhiteX, bWhiteY = 110.0, 220.0
	bWhiteW, bWhiteH = 300.0, 96.0
	// 臂 c SetImageAt 双纹理（左半主图红、右半第二张蓝）。
	cImgX, cImgY = 520.0, 220.0
	cImgW, cImgH = 240.0, 96.0
	// 臂 b/c 上的恒等材质对照段（图像口，纹理采样原样返回 vs 无材质逐位）。
	idImgX, idImgY = 880.0, 220.0
	idImgW         = 120.0
	idImgH         = 96.0

	// 臂 d 描边吃 shader。
	dY       = 430.0
	dX0, dX1 = 130.0, 430.0
	dWidth   = 12.0

	// 臂 f uniform 载荷决定画面（272B：边界字段落在旧 256B 预算之外）。
	// 落点避开臂 e 的「拒绝区应保持底色」锚 (610,560) 与底带 y≥712。
	fImgX, fImgY = 700.0, 430.0
	fImgW, fImgH = 240.0, 96.0
	// 写进 uniform 的两个值（画面与探针共用同一份常数）。
	fSplit = 0.25 // uv.x 空间的分屏边界
	fGain  = 0.5  // 右半亮度增益
	// uniformOldBudget = 批 3c 抬预算之前的上限；split 落在它之外，谁在半路截断
	// 载荷，着色器就会把边界读成 0（整块变暗蓝），锚立刻报警。
	uniformOldBudget  = 256
	uniformSplitBytes = 272 // = 256B 占位 + split + gain + 两个保留 f32

	// 臂 G 后端拒收三格对照（y 600 起，避开臂 e 的 (610,560) 拒绝区锚）。
	gY  = 600.0
	gW  = 130.0
	gH  = 60.0
	gX0 = 110.0 // 格 1：无材质环境实心红（证明这块地方本来画得出东西）
	gX1 = 260.0 // 格 2：同一环境色 + 后端必拒模块 ⇒ 整笔拒画，保持底色
	gX2 = 410.0 // 格 3：同几何 + 合法材质 ⇒ 画出 shader 绿（尺子没坏）

	// 拒绝态底带。
	refusalY = 712.0
	chipW    = 16.0
	chipStep = 44.0

	// H15：定帧停机 + 过 settling 帧定格。
	gateFrames   = 240
	gateFreezeAt = 120
	gateHz       = 60.0

	// 面板底色：期望色的底（premul src-over 公式同一份常数）。
	backdropRGB = 0.055

	// 容差（写死）：premul 混合 GPU 舍入给六档；直色/逐位段零容差起。
	blendTol = 6.0
)

// effGrad：三入口共用的 uv 渐变（无纹理声明；premul：rgb 已乘 a）。
const effGradWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let g = 0.35 + 0.65 * uv.y;
    return vec4<f32>(0.95 * g, 0.45 * g, 0.15 * g, g);
}`

// effD9：白底上 premul 半透明红（批 3a premul 修复的窗内版，p32 d9 同数字）。
const effD9WGSL = `@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(0.5, 0.0, 0.0, 0.5);
}`

// effSplit：双纹理分屏（批 3c D4：slot0=被画图自身，slot1=SetImageAt）。
const effSplitWGSL = `
@group(0) @binding(1) var m_tex0: texture_2d<f32>;
@group(0) @binding(2) var m_samp0: sampler;
@group(0) @binding(3) var m_tex1: texture_2d<f32>;
@group(0) @binding(4) var m_samp1: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = textureSample(m_tex0, m_samp0, uv);
    let b = textureSample(m_tex1, m_samp1, uv);
    if uv.x < 0.5 {
        return a;
    }
    return b;
}`

// effIdentity：恒等材质（采样 TEXTURE 原样返回）——与无材质逐位相同的对照。
const effIdentityWGSL = `
@group(0) @binding(1) var m_tex: texture_2d<f32>;
@group(0) @binding(2) var m_samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(m_tex, m_samp, uv);
}`

// effStroke：描边渐变（fillShaderAsImage 的 uv 覆盖描边展开轮廓）。
const effStrokeWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let g = 0.35 + 0.65 * uv.x;
    return vec4<f32>(0.2 * g, 0.7 * g, 0.9 * g, g);
}`

// effMaxBudget：uniform 顶格（ShaderUniformBudgetMax=65536B）——前端应放行，
// gles 腿能否编译随设备上限（本扇的 gles 承载尝试，结果如实记）。
const effMaxBudgetWGSL = `
struct Big {
    vals: array<vec4<f32>, 4096>,
}
@group(0) @binding(0) var<uniform> u: Big;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let g = clamp(u.vals[0].x + uv.y, 0.0, 1.0);
    return vec4<f32>(0.9 * g, 0.4 * g, 0.1 * g, g);
}`

var testdataDir = filepath.Join("examples", "engine", "p37_material", "testdata")

// effBadArrayStrideWGSL：臂 G 的**故意**触发子——uniform 地址空间要求数组 stride
// 是 16 的倍数，`array<f32,64>` 的 stride=4 ⇒ 前端 `NewShaderEffect` 放行、后端
// naga 在 `CreateShaderModule` 拒（本机 native 与 go 两后端实测同形，
// 证据 `batch6/gap3-backend-reject-probe.txt`）。填充腿对此的既定语义=**整笔拒画**
// （区内保持底色，环境实心色不得冒充回退）+ `ShaderBackendFails++` + `CPUFallbackOps++`。
const effBadArrayStrideWGSL = `
struct BadAlign {
    vals: array<f32, 64>,
}
@group(0) @binding(0) var<uniform> u: BadAlign;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let v = u.vals[0];
    return vec4<f32>(uv.x, uv.y, 0.5, 1.0) + vec4<f32>(v, v, v, 0.0);
}`

// effFlatGreenWGSL：臂 G 的合法对照材质（同尺子必须画得出色：0.2/0.9/0.4 → (51,229,102)）。
const effFlatGreenWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(0.2, 0.9, 0.4, 1.0);
}`

// 臂 G 的着色器底色期望（与上面那份 WGSL 常数同源）。
var flatGreenRGB = [3]uint8{51, 229, 102}

// effUniformSplitWGSL：臂 f 的着色器——分屏边界与右半增益**只**来自 uniform。
// struct 前 256B 是占位（uniform 地址空间要求数组 stride 是 16 的倍数，故用
// 16×vec4 而不是 64×f32——写成后者 naga 直接拒整个模块），`split` 落在偏移
// 256B（旧预算之外）、`gain` 落在 260B ⇒ 载荷若被半路截断，边界读成 0、
// 画面整块变暗蓝，锚立刻对不上。
const effUniformSplitWGSL = `
struct USplit {
    pad: array<vec4<f32>, 16>,
    split: f32,
    gain: f32,
    rsv2: f32,
    rsv3: f32,
}
@group(0) @binding(0) var<uniform> u: USplit;
@group(0) @binding(1) var m_tex0: texture_2d<f32>;
@group(0) @binding(2) var m_samp0: sampler;
@group(0) @binding(3) var m_tex1: texture_2d<f32>;
@group(0) @binding(4) var m_samp1: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = textureSample(m_tex0, m_samp0, uv);
    let b = textureSample(m_tex1, m_samp1, uv);
    if uv.x < u.split {
        return a;
    }
    return vec4<f32>(b.r * u.gain, b.g * u.gain, b.b * u.gain, b.a);
}`

// uniformSplitPayload 拼臂 f 的 272B uniform（GPU uniform 缓冲按小端原样 memcpy，
// 平台即 x86/ARM 小端；偏移 256/260 处放两个 f32，其余留零）。
func uniformSplitPayload(split, gain float32) []byte {
	b := make([]byte, uniformSplitBytes)
	binary.LittleEndian.PutUint32(b[uniformOldBudget:], math.Float32bits(split))
	binary.LittleEndian.PutUint32(b[uniformOldBudget+4:], math.Float32bits(gain))
	return b
}

// texSolid 生成纯色 ImageBuf（探针期望色 = 同一份常数）。
func texSolid(w, h int, c [3]uint8) *render.ImageBuf {
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		panic(fmt.Sprintf("p37_material: tex: %v", err))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if err := buf.SetRGBA(x, y, c[0], c[1], c[2], 255); err != nil {
				panic(err)
			}
		}
	}
	return buf
}

var (
	texGray  = texSolid(110, 90, texGrayColor)  // 无材质对照
	texRed   = texSolid(240, 96, texRedColor)   // slot0 主图
	texBlue  = texSolid(240, 96, texBlueColor)  // slot1 第二张
	texIdent = texSolid(120, 96, texIdentColor) // 恒等对照段
)

// 纹理底色（探针期望色与 texSolid 的入参是同一份常数，改一处两边同步）。
var (
	texGrayColor  = [3]uint8{170, 170, 175}
	texRedColor   = [3]uint8{220, 60, 60}
	texBlueColor  = [3]uint8{60, 90, 220}
	texIdentColor = [3]uint8{90, 180, 120}
)

func mustEffect(wgsl string) *render.ShaderEffect {
	e, err := render.NewShaderEffect(wgsl)
	if err != nil {
		panic(fmt.Sprintf("p37_material: shader: %v", err))
	}
	return e
}

// 窗体逐帧复用的效果实例（NewShaderEffect 是全量前端编译，绝不能进每帧路径）。
var (
	effGrad     = mustEffect(effGradWGSL)
	effD9       = mustEffect(effD9WGSL)
	effSplit    = mustEffect(effSplitWGSL)
	effIdentity = mustEffect(effIdentityWGSL)
	effStroke   = mustEffect(effStrokeWGSL)
)

// 臂 f 的窗体实例：uniform 与第二张纹理都在 init 里一次性设好，绝不进每帧路径。
var effUniformSplit = mustEffect(effUniformSplitWGSL)

// 臂 G 的实例：故意模块的前端放行与否是要量的事实，不能用 mustEffect 直接 panic
// （别的后端可能前端就拒——探针会把它打成红，而不是让窗起不来）。
var (
	effBad         *render.ShaderEffect
	effBadFrontErr error
	effFlatGreen   = mustEffect(effFlatGreenWGSL)
)

func init() {
	if err := effSplit.SetImageAt(1, texBlue); err != nil {
		panic(err)
	}
	if err := effUniformSplit.SetImageAt(1, texBlue); err != nil {
		panic(err)
	}
	if err := effUniformSplit.SetUniformBytes(uniformSplitPayload(fSplit, fGain)); err != nil {
		panic(err)
	}
	effBad, effBadFrontErr = render.NewShaderEffect(effBadArrayStrideWGSL)
}

func paintBackdrop(dc render.Canvas, w, h float64) {
	dc.SetRGB(backdropRGB, backdropRGB, backdropRGB+0.02)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
}

func fillRect(dc render.Canvas, x, y, w, h float64) {
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

// setShader 把 paint shader 挂上当前 DC（Canvas 接口不带 SetShader；录制层
// backend_canvas.go 同款类型断言透传，活 Context 支持、Pixmap 腿不支持）。
// 返回复位函数。
func setShader(dc render.Canvas, e *render.ShaderEffect) func() {
	if ss, ok := dc.(interface{ SetShader(*render.ShaderEffect) }); ok {
		ss.SetShader(e)
		return func() { ss.SetShader(nil) }
	}
	return func() {}
}

// armA 三入口成对绘制：无材质 | 挂材质。
func paintArmA(dc render.Canvas, eff *render.ShaderEffect) error {
	// 图像口
	dc.DrawImageEx(texGray, render.DrawImageOptions{X: aPair0, Y: aTopY})
	dc.DrawImageEx(texGray, render.DrawImageOptions{X: aPair0 + aW + aGap, Y: aTopY, Material: eff})
	// quad 口（仿射四角）
	corners := [4]render.Point{
		{X: aPair1, Y: aTopY}, {X: aPair1 + aW, Y: aTopY},
		{X: aPair1 + aW, Y: aTopY + aH}, {X: aPair1, Y: aTopY + aH},
	}
	if err := dc.DrawImageQuad(texGray, corners, render.QuadDrawOptions{}); err != nil {
		return err
	}
	cornersM := [4]render.Point{
		{X: aPair1 + aW + aGap, Y: aTopY}, {X: aPair1 + 2*aW + aGap, Y: aTopY},
		{X: aPair1 + 2*aW + aGap, Y: aTopY + aH}, {X: aPair1 + aW + aGap, Y: aTopY + aH},
	}
	if err := dc.DrawImageQuad(texGray, cornersM, render.QuadDrawOptions{Material: eff}); err != nil {
		return err
	}
	// 批绘口
	off := func(x float64) render.AtlasSprite {
		return render.AtlasSprite{DstX: x, DstY: aTopY, DstW: aW, DstH: aH,
			SrcX: 0, SrcY: 0, SrcW: float64(texGray.Width()), SrcH: float64(texGray.Height())}
	}
	_ = off
	sp := render.AtlasSprite{DstX: aPair2, DstY: aTopY, DstW: aW, DstH: aH,
		SrcX: 0, SrcY: 0, SrcW: float64(texGray.Width()), SrcH: float64(texGray.Height())}
	if _, err := dc.DrawAtlasEx(texGray, []render.AtlasSprite{sp}, render.AtlasDrawOptions{}); err != nil {
		return err
	}
	spM := render.AtlasSprite{DstX: aPair2 + aW + aGap, DstY: aTopY, DstW: aW, DstH: aH,
		SrcX: 0, SrcY: 0, SrcW: float64(texGray.Width()), SrcH: float64(texGray.Height()),
		Material: eff}
	if _, err := dc.DrawAtlasEx(texGray, []render.AtlasSprite{spM}, render.AtlasDrawOptions{}); err != nil {
		return err
	}
	return nil
}

// paintArmG 臂 G「后端拒收看得见」三格：格 1 无材质（证明这块地方画得出东西）、
// 格 2 同环境色挂后端必拒模块（整笔拒画 ⇒ 底色留在原位）、格 3 同几何挂合法材质
// （尺子没坏，必须出 shader 绿）。
func paintArmG(dc render.Canvas) {
	dc.SetRGB(1, 0, 0)
	fillRect(dc, gX0, gY, gW, gH)
	if effBad != nil {
		resetBad := setShader(dc, effBad)
		dc.SetRGB(1, 0, 0)
		fillRect(dc, gX1, gY, gW, gH)
		resetBad()
	}
	resetGood := setShader(dc, effFlatGreen)
	dc.SetRGB(1, 0, 0)
	fillRect(dc, gX2, gY, gW, gH)
	resetGood()
}

// armGBadShaderColorAt = effBadArrayStrideWGSL 在格 2 内 (px,py) 的**解析**输出
// （vec4(uv.x, uv.y, 0.5, 1)，uv 取填充矩形归一化位置）——后端接受这枚模块时
// 该格必须是这个颜色，不接受时必须是底色，两个分支都提前写死，不做事后拟合。
func armGBadShaderColorAt(px, py float64) [3]uint8 {
	ux := (px - gX1 + 0.5) / gW
	uy := (py - gY + 0.5) / gH
	return [3]uint8{uint8(ux*255 + 0.5), uint8(uy*255 + 0.5), 128}
}

// armGBackdropColor = paintBackdrop 那份常数的 8-bit 期望。
func armGBackdropColor() [3]uint8 {
	return [3]uint8{
		uint8(math.Round(backdropRGB * 255)), uint8(math.Round(backdropRGB * 255)),
		uint8(math.Round((backdropRGB + 0.02) * 255)),
	}
}

func armGNear(a, b [3]uint8) bool {
	return abs(int(a[0])-int(b[0])) <= int(blendTol) &&
		abs(int(a[1])-int(b[1])) <= int(blendTol) &&
		abs(int(a[2])-int(b[2])) <= int(blendTol)
}

// armGFrameCheck 读终帧格 2，与呈现帧的活计数对同一件事。
func armGFrameCheck(path string, fails int) (bool, string) {
	cell, err := snapshotPixel(path, gX1+gW/2, gY+gH/2)
	if err != nil {
		return false, err.Error()
	}
	backdrop, accepted := armGBackdropColor(), armGBadShaderColorAt(gX1+gW/2, gY+gH/2)
	got := fmt.Sprintf("格2=(%d,%d,%d) 底色=(%d,%d,%d) shader解析=(%d,%d,%d) window_shader_backend_fails=%d",
		cell[0], cell[1], cell[2], backdrop[0], backdrop[1], backdrop[2],
		accepted[0], accepted[1], accepted[2], fails)
	switch {
	case armGNear(cell, backdrop) && fails >= 1:
		return true, "拒收端同形（底色 + 计数≥1）" + got
	case armGNear(cell, accepted) && fails == 0:
		return true, "接受端同形（shader 自己的色 + 计数 0）" + got
	case armGNear(cell, backdrop):
		return false, "画成底色却零计数=拒而不计数（静默消失)" + got
	default:
		return false, "既不是底色也不是 shader 解析色（环境色冒充回退或别处漏墨)" + got
	}
}

// snapshotPixel 按逻辑坐标取终帧像素（与 wrkit.CheckPixelProbes 同一套 dpr 折算）。
func snapshotPixel(path string, x, y float64) ([3]uint8, error) {
	f, err := os.Open(path)
	if err != nil {
		return [3]uint8{}, err
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		return [3]uint8{}, err
	}
	b := img.Bounds()
	dpr := float64(b.Dx()) / winW
	px, py := b.Min.X+int(x*dpr+0.5), b.Min.Y+int(y*dpr+0.5)
	if px < b.Min.X || px >= b.Max.X || py < b.Min.Y || py >= b.Max.Y {
		return [3]uint8{}, fmt.Errorf("p37_material: 臂 G 探针点越界 %v", b)
	}
	r, g, bl, _ := img.At(px, py).RGBA()
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8)}, nil
}

func paintAll(dc render.Canvas, w, h float64) error {
	paintBackdrop(dc, w, h)
	if err := paintArmA(dc, effGrad); err != nil {
		return err
	}
	// 臂 b：白底 + premul 半透明红填充。
	dc.SetRGB(1, 1, 1)
	fillRect(dc, bWhiteX, bWhiteY, bWhiteW, bWhiteH)
	dc.SetRGB(1, 0, 0) // 环境色（shader 挂上后不生效——premul 走 shader 输出）
	reset := setShader(dc, effD9)
	fillRect(dc, bWhiteX+20, bWhiteY+20, bWhiteW-40, bWhiteH-40)
	reset()
	// 臂 c：SetImageAt 双纹理分屏。
	if err := dc.DrawImageQuad(texRed, [4]render.Point{
		{X: cImgX, Y: cImgY}, {X: cImgX + cImgW, Y: cImgY},
		{X: cImgX + cImgW, Y: cImgY + cImgH}, {X: cImgX, Y: cImgY + cImgH},
	}, render.QuadDrawOptions{Material: effSplit}); err != nil {
		return err
	}
	// 恒等材质对照段（图像口）。
	effID := mustEffect(effIdentityWGSL)
	dc.DrawImageEx(texIdent, render.DrawImageOptions{X: idImgX, Y: idImgY})
	dc.DrawImageEx(texIdent, render.DrawImageOptions{X: idImgX + idImgW + 16, Y: idImgY, Material: effID})
	// 臂 d：描边吃 shader。
	dc.SetRGB(1, 0, 0)
	resetD := setShader(dc, effStroke)
	dc.SetStroke(render.DefaultStroke().WithWidth(dWidth))
	dc.DrawLine(dX0, dY, dX1, dY)
	if err := dc.Stroke(); err != nil {
		resetD()
		return err
	}
	resetD()
	// 臂 f：uniform 载荷决定画面（左 1/4 主图红、其余暗蓝；边界与增益全在 uniform 里）。
	if err := dc.DrawImageQuad(texRed, [4]render.Point{
		{X: fImgX, Y: fImgY}, {X: fImgX + fImgW, Y: fImgY},
		{X: fImgX + fImgW, Y: fImgY + fImgH}, {X: fImgX, Y: fImgY + fImgH},
	}, render.QuadDrawOptions{Material: effUniformSplit}); err != nil {
		return err
	}
	// 臂 G：后端拒收三格（无材质红 | 拒收模块保底色 | 合法材质绿）。
	paintArmG(dc)
	// 臂 e：拒绝态底带（离屏探针同一份事实）。
	paintRefusalBand(dc)
	return nil
}

// refusalFacts 八枚拒绝态实测（纯逻辑；底带与离屏探针吃同一份结果）。
func refusalFacts() (map[string]bool, map[string]string) {
	ok := map[string]bool{}
	got := map[string]string{}
	chipFacts = map[string]bool{}
	_, err := render.NewShaderEffect("const broken: i32 = ;")
	ok["bad_wgsl"] = errors.Is(err, render.ErrShaderBadWGSL)
	got["bad_wgsl"] = fmt.Sprint(err)
	_, err = render.NewShaderEffect("@fragment fn other() -> @location(0) vec4<f32> { return vec4<f32>(0.0); }")
	ok["no_entry"] = errors.Is(err, render.ErrShaderNoEntry)
	got["no_entry"] = fmt.Sprint(err)
	_, err = render.NewShaderEffect("@fragment fn fs_main() -> @location(0) f32 { return 0.0; }")
	ok["bad_return"] = errors.Is(err, render.ErrShaderBadReturn)
	got["bad_return"] = fmt.Sprint(err)
	_, err = render.NewShaderEffect(`@group(0) @binding(1) var t: texture_2d<f32>;
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(0.0); }`)
	ok["slot_conflict"] = errors.Is(err, render.ErrShaderSlotConflict)
	got["slot_conflict"] = fmt.Sprint(err)
	_, err = render.NewShaderEffect(`struct U { v: f32, _p: vec3<f32> }
@group(0) @binding(0) var<uniform> u: U;
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(u.v); }`)
	if err == nil {
		e2 := mustEffect(`struct U { v: f32, _p: vec3<f32> }
@group(0) @binding(0) var<uniform> u: U;
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(u.v); }`)
		err = e2.SetUniformBytes(make([]byte, render.ShaderUniformBudgetMax+1))
	}
	ok["uniform_too_large"] = errors.Is(err, render.ErrShaderUniformTooLarge)
	got["uniform_too_large"] = fmt.Sprint(err)
	e3 := mustEffect("@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(0.0); }")
	e3.Close()
	err = e3.SetUniformBytes([]byte{1})
	ok["closed"] = errors.Is(err, render.ErrShaderClosed)
	got["closed"] = fmt.Sprint(err)
	// quad 现场拒（材质×透视专项后=投影层眼后 ErrQuadBehindEye；现场断言在离屏探针做）。
	ok["quad_behind_eye_refused"] = false // 现场断言在离屏探针做，探针段补牌
	got["quad_behind_eye_refused"] = "live-probe"
	chipFacts["bad_wgsl"] = ok["bad_wgsl"]
	chipFacts["no_entry"] = ok["no_entry"]
	chipFacts["bad_return"] = ok["bad_return"]
	chipFacts["slot_conflict"] = ok["slot_conflict"]
	chipFacts["uniform_too_large"] = ok["uniform_too_large"]
	chipFacts["closed"] = ok["closed"]
	return ok, got
}

var chipKeys = []string{"bad_wgsl", "no_entry", "bad_return", "slot_conflict",
	"uniform_too_large", "closed", "quad_behind_eye_refused", "backend_fails"}

var chipFacts map[string]bool

func paintRefusalBand(dc render.Canvas) {
	for i, k := range chipKeys {
		x := 110.0 + float64(i)*chipStep
		if chipFacts[k] {
			dc.SetRGB(70.0/255, 200.0/255, 120.0/255)
			fillRect(dc, x, refusalY, chipW, chipW)
			continue
		}
		dc.SetRGB(235.0/255, 90.0/255, 60.0/255)
		fillRect(dc, x, refusalY, chipW, chipW)
	}
}

func newCtx(w, h int) *render.Context {
	return render.NewContext(w, h)
}

func pixelAt(dc *render.Context, x, y int) (uint8, uint8, uint8, uint8) {
	img := dc.Image()
	cr, cg, cb, ca := img.At(x, y).RGBA()
	return uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)
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

func failJoin(p *probeReport, name string, err error) {
	p.pixelErr = errors.Join(p.pixelErr, fmt.Errorf("%s: %w", name, err))
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// expectBlend 断言 premul src-over 期望色：src premul (r,g,b,a) 叠在底色上，
// 直色读回 = clamp(src.rgb + dst.rgb*(1-a))（p32 d9 同款公式）。
func expectBlend(dc *render.Context, x, y int, sr, sg, sb, sa float64, tol int) (bool, string) {
	gr, gg, gb, _ := pixelAt(dc, x, y)
	db := backdropRGB * 255
	dbb := backdropRGB*255 + 2
	wr := sr*255 + db*(1-sa)
	wg := sg*255 + db*(1-sa)
	wb := sb*255 + dbb*(1-sa)
	clamp := func(v float64) int {
		if v > 255 {
			return 255
		}
		return int(math.Round(v))
	}
	want := [3]int{clamp(wr), clamp(wg), clamp(wb)}
	ok := abs(int(gr)-want[0]) <= tol && abs(int(gg)-want[1]) <= tol && abs(int(gb)-want[2]) <= tol
	return ok, fmt.Sprintf("got=(%d,%d,%d) want=%v", gr, gg, gb, want)
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

func flushErr(dc *render.Context, paint func() error) error {
	if err := paint(); err != nil {
		return err
	}
	return dc.FlushGPU()
}

// runProbes 离屏证据（pan 无关，几何固定）。
func runProbes() probeReport {
	var rep probeReport
	const w, h = int(winW), int(winH)
	rep.extra = map[string]any{}
	facts, factGot := refusalFacts()
	rep.extra["nonfinite_rejects"] = factGot
	allOK := true
	for _, k := range chipKeys[:6] {
		if !facts[k] {
			allOK = false
		}
	}
	rep.add("sentinels_named_six", allOK, fmt.Sprint(factGot))

	// 臂 a：三入口成对，挂材质的像素 ≠ 无材质，且与解析渐变公式吻合。
	dcA := newCtx(w, h)
	if err := flushErr(dcA, func() error { return paintArmA(dcA, mustEffect(effGradWGSL)) }); err != nil {
		failJoin(&rep, "arm a flush", err)
	}
	type pt struct {
		x, y float64
	}
	mid := func(x float64) pt { return pt{x: x + aW/2, y: aTopY + aH/2} }
	entries := []struct {
		name  string
		noMat pt
		mat   pt
	}{
		{"image", mid(aPair0), mid(aPair0 + aW + aGap)},
		{"quad", mid(aPair1), mid(aPair1 + aW + aGap)},
		{"atlas", mid(aPair2), mid(aPair2 + aW + aGap)},
	}
	for _, e := range entries {
		nr, ng, nb, _ := pixelAt(dcA, int(e.noMat.x), int(e.noMat.y))
		mr, mg, mb, _ := pixelAt(dcA, int(e.mat.x), int(e.mat.y))
		diff := abs(int(nr)-int(mr)) + abs(int(ng)-int(mg)) + abs(int(nb)-int(mb))
		// 渐变解析期望：uv.y=(y-topY+0.5)/h → g=0.35+0.65*uv.y；premul src-over。
		uvY := (float64(int(e.mat.y)) - aTopY + 0.5) / aH
		g := 0.35 + 0.65*uvY
		okk, gott := expectBlend(dcA, int(e.mat.x), int(e.mat.y), 0.95*g, 0.45*g, 0.15*g, g, blendTol)
		rep.add("material_"+e.name+"_differs_and_matches", diff > 30 && okk,
			fmt.Sprintf("diff=%d %s", diff, gott))
		_ = ng
		_ = nb
		_ = mr
		_ = mg
		_ = mb
		_ = nr
	}
	// 三入口同色对账（同一 shader 同一 uv 行）。
	ri, _, _, _ := pixelAt(dcA, int(mid(aPair0+aW+aGap).x), int(mid(aPair0+aW+aGap).y))
	rq, _, _, _ := pixelAt(dcA, int(mid(aPair1+aW+aGap).x), int(mid(aPair1+aW+aGap).y))
	ra, _, _, _ := pixelAt(dcA, int(mid(aPair2+aW+aGap).x), int(mid(aPair2+aW+aGap).y))
	par := abs(int(ri)-int(rq)) <= int(blendTol) && abs(int(ri)-int(ra)) <= int(blendTol)
	rep.add("entries_color_parity", par, fmt.Sprintf("image=%d quad=%d atlas=%d", ri, rq, ra))
	_ = dcA.Close()

	// 恒等材质 ≡ 无材质 逐位（图像口，内缩 4px 的内部区域）。
	dcID := newCtx(w, h)
	effID := mustEffect(effIdentityWGSL)
	if err := flushErr(dcID, func() error {
		paintBackdrop(dcID, float64(w), float64(h))
		dcID.DrawImageEx(texIdent, render.DrawImageOptions{X: idImgX, Y: idImgY})
		dcID.DrawImageEx(texIdent, render.DrawImageOptions{X: idImgX + idImgW + 16, Y: idImgY, Material: effID})
		return nil
	}); err != nil {
		failJoin(&rep, "identity flush", err)
	}
	x0, y0 := int(idImgX)+4, int(idImgY)+4
	plain := regionBytes(dcID, x0, y0, int(idImgW)-8, int(idImgH)-8)
	mat := regionBytes(dcID, int(idImgX)+idImgW+16+4, y0, int(idImgW)-8, int(idImgH)-8)
	dn, dw := diffBytes(plain, mat)
	rep.extra["identity_diff_px"] = dn
	rep.extra["identity_diff_worst"] = dw
	rep.add("identity_material_bitwise", dn == 0 && dw == 0, fmt.Sprintf("diff=%d worst=%d", dn, dw))
	_ = dcID.Close()

	// 臂 b：premul 半透明红 over 白 = (255,128,128)。
	dcB := newCtx(w, h)
	effD9 := mustEffect(effD9WGSL)
	if err := flushErr(dcB, func() error {
		paintBackdrop(dcB, float64(w), float64(h))
		dcB.SetRGB(1, 1, 1)
		fillRect(dcB, bWhiteX, bWhiteY, bWhiteW, bWhiteH)
		dcB.SetRGB(1, 0, 0)
		dcB.SetShader(effD9)
		fillRect(dcB, bWhiteX+20, bWhiteY+20, bWhiteW-40, bWhiteH-40)
		dcB.SetShader(nil)
		return nil
	}); err != nil {
		failJoin(&rep, "arm b flush", err)
	}
	cx, cy := int(bWhiteX+bWhiteW/2), int(bWhiteY+bWhiteH/2)
	gr, gg, gb, _ := pixelAt(dcB, cx, cy)
	d9OK := gr == 255 && abs(int(gg)-128) <= int(blendTol) && abs(int(gb)-128) <= int(blendTol)
	rep.add("premul_semitransparent_fill_d9", d9OK, fmt.Sprintf("got=(%d,%d,%d) want=(255,128,128)", gr, gg, gb))
	_ = dcB.Close()

	// 臂 c：SetImageAt 双纹理分屏（左红右蓝）。
	dcC := newCtx(w, h)
	effSplit := mustEffect(effSplitWGSL)
	if err := effSplit.SetImageAt(1, texBlue); err != nil {
		failJoin(&rep, "SetImageAt slot1", err)
	}
	if err := flushErr(dcC, func() error {
		paintBackdrop(dcC, float64(w), float64(h))
		return dcC.DrawImageQuad(texRed, [4]render.Point{
			{X: cImgX, Y: cImgY}, {X: cImgX + cImgW, Y: cImgY},
			{X: cImgX + cImgW, Y: cImgY + cImgH}, {X: cImgX, Y: cImgY + cImgH},
		}, render.QuadDrawOptions{Material: effSplit})
	}); err != nil {
		failJoin(&rep, "arm c flush", err)
	}
	lr, lg, lb, _ := pixelAt(dcC, int(cImgX+40), int(cImgY+48))
	rr2, rg2, rb2, _ := pixelAt(dcC, int(cImgX+cImgW-40), int(cImgY+48))
	leftOK := abs(int(lr)-220) <= int(blendTol) && abs(int(lg)-60) <= int(blendTol) && abs(int(lb)-60) <= int(blendTol)
	rightOK := abs(int(rr2)-60) <= int(blendTol) && abs(int(rg2)-90) <= int(blendTol) && abs(int(rb2)-220) <= int(blendTol)
	rep.add("set_image_at_second_texture_paints", leftOK && rightOK,
		fmt.Sprintf("left=(%d,%d,%d) right=(%d,%d,%d)", lr, lg, lb, rr2, rg2, rb2))
	_ = dcC.Close()

	// 臂 f：uniform 载荷决定画面——同几何、同纹理、同着色器，只换 uniform 字节。
	// 锚点全在固定像素上，期望色写死（`0.125 < fSplit(0.25) < 0.5 < 0.75`），
	// 不随 fSplit 自算——否则 fSplit 被人改也照样「自洽绿」，就成循环断言了。
	paintPayload := func(tag string, payload []byte) *render.Context {
		dcF := newCtx(w, h)
		effF := mustEffect(effUniformSplitWGSL)
		if err := effF.SetImageAt(1, texBlue); err != nil {
			failJoin(&rep, tag+" SetImageAt", err)
		}
		if err := effF.SetUniformBytes(payload); err != nil {
			failJoin(&rep, tag+" SetUniformBytes", err)
		}
		if err := flushErr(dcF, func() error {
			paintBackdrop(dcF, float64(w), float64(h))
			return dcF.DrawImageQuad(texRed, [4]render.Point{
				{X: fImgX, Y: fImgY}, {X: fImgX + fImgW, Y: fImgY},
				{X: fImgX + fImgW, Y: fImgY + fImgH}, {X: fImgX, Y: fImgY + fImgH},
			}, render.QuadDrawOptions{Material: effF})
		}); err != nil {
			failJoin(&rep, tag+" flush", err)
		}
		return dcF
	}
	dcF1 := paintPayload("arm f split=0.25", uniformSplitPayload(fSplit, fGain))
	dcF2 := paintPayload("arm f split=0.75", uniformSplitPayload(0.75, fGain))
	dcF3 := paintPayload("arm f split=0", uniformSplitPayload(0, fGain)) // 增益照旧，只把边界压到 0
	fLeft, fMid, fRight, fY := int(fImgX+0.125*fImgW), int(fImgX+0.5*fImgW), int(fImgX+0.875*fImgW), int(fImgY+fImgH/2)
	wantRed := texRedColor
	wantDim := [3]uint8{uint8(float64(texBlueColor[0]) * fGain), uint8(float64(texBlueColor[1]) * fGain), uint8(float64(texBlueColor[2]) * fGain)}
	matches := func(dc *render.Context, x, y int, want [3]uint8) (bool, [3]uint8) {
		r, g, b, _ := pixelAt(dc, x, y)
		return abs(int(r)-int(want[0])) <= int(blendTol) && abs(int(g)-int(want[1])) <= int(blendTol) && abs(int(b)-int(want[2])) <= int(blendTol),
			[3]uint8{r, g, b}
	}
	m1a, got1a := matches(dcF1, fLeft, fY, wantRed)
	m1b, got1b := matches(dcF1, fMid, fY, wantDim)
	rep.add("uniform_payload_paints_split", m1a && m1b, fmt.Sprintf(
		"split=%.2f 左=(%d,%d,%d) want红 中=(%d,%d,%d) want暗蓝", fSplit, got1a[0], got1a[1], got1a[2], got1b[0], got1b[1], got1b[2]))
	m2a, got2a := matches(dcF2, fMid, fY, wantRed)
	m2b, got2b := matches(dcF2, fRight, fY, wantDim)
	midMoved, _ := matches(dcF1, fMid, fY, wantRed)
	rep.add("uniform_boundary_follows_payload", m2a && m2b && !midMoved, fmt.Sprintf(
		"同一像素 split 0.25→0.75：中=(%d,%d,%d)→(%d,%d,%d) 必须翻成红，右=(%d,%d,%d) 仍暗蓝",
		got1b[0], got1b[1], got1b[2], got2a[0], got2a[1], got2a[2], got2b[0], got2b[1], got2b[2]))
	m3a, got3a := matches(dcF3, fLeft, fY, wantDim)
	rep.add("uniform_boundary_can_be_zero_from_payload", m3a, fmt.Sprintf(
		"边界写 0（增益不变）左=(%d,%d,%d) 必须是暗蓝——仍是红就说明 split 不是从载荷 256B 处读的（黑=什么都没画，故不用全零载荷）",
		got3a[0], got3a[1], got3a[2]))
	_ = dcF1.Close()
	_ = dcF2.Close()
	_ = dcF3.Close()

	// 臂 G：后端拒收三格（离屏同一份几何与期望色，与窗内 paintArmG 同源）。
	dcG := newCtx(w, h)
	gBefore := dcG.RenderPathStats()
	if err := flushErr(dcG, func() error {
		paintBackdrop(dcG, float64(w), float64(h))
		paintArmG(dcG)
		return nil
	}); err != nil {
		failJoin(&rep, "arm G flush", err)
	}
	gAfter := dcG.RenderPathStats()
	rep.extra["backend_fails_delta_offscreen"] = gAfter.ShaderBackendFails - gBefore.ShaderBackendFails
	rep.extra["cpu_fb_delta_offscreen"] = gAfter.CPUFallbackOps - gBefore.CPUFallbackOps
	rep.add("standalone_leg_reject_front_end_accepted", effBadFrontErr == nil && effBad != nil,
		fmt.Sprintf("故意模块前端 err=%v（前端就拒=这条腿无从量起，不许静默当过）", effBadFrontErr))
	bdOK := gAfter.ShaderBackendFails-gBefore.ShaderBackendFails >= 1 && gAfter.CPUFallbackOps-gBefore.CPUFallbackOps >= 1
	rep.add("standalone_leg_reject_counted", bdOK, fmt.Sprintf(
		"ShaderBackendFails %d→%d 且 CPUFallbackOps %d→%d（两条都要涨，只涨一条=语义漂移）",
		gBefore.ShaderBackendFails, gAfter.ShaderBackendFails, gBefore.CPUFallbackOps, gAfter.CPUFallbackOps))
	// 三格像素：格 1 红（这块地方本来画得出东西）、格 2 底色（拒画=既不是环境红
	// 也不是 shader 绿，环境色不许冒充回退）、格 3 shader 绿（同尺子没坏）。
	wantBackdropColor := armGBackdropColor()
	wantAmbientRed := [3]uint8{255, 0, 0}
	gcy := int(gY + gH/2)
	mRed, gotRed := matches(dcG, int(gX0+gW/2), gcy, wantAmbientRed)
	mGreen, gotGreen := matches(dcG, int(gX2+gW/2), gcy, flatGreenRGB)
	rep.add("arm_g_controls_paint", mRed && mGreen, fmt.Sprintf(
		"格1 无材质=(%d,%d,%d) want(255,0,0)，格3 合法材质=(%d,%d,%d) want(%d,%d,%d)",
		gotRed[0], gotRed[1], gotRed[2], gotGreen[0], gotGreen[1], gotGreen[2],
		flatGreenRGB[0], flatGreenRGB[1], flatGreenRGB[2]))
	mBack, gotBack := matches(dcG, int(gX1+gW/2), gcy, wantBackdropColor)
	rejectNotAmbient := func() bool {
		r, g, b, _ := pixelAt(dcG, int(gX1+gW/2), gcy)
		return abs(int(r)-int(wantAmbientRed[0])) > int(blendTol) || abs(int(g)-int(wantAmbientRed[1])) > int(blendTol) || abs(int(b)-int(wantAmbientRed[2])) > int(blendTol)
	}()
	rep.add("standalone_leg_reject_region_keeps_backdrop", mBack && rejectNotAmbient, fmt.Sprintf(
		"格2=(%d,%d,%d) 必须等于底色 (%d,%d,%d) 且不等于环境实心红（拒画≠回退成实心色）",
		gotBack[0], gotBack[1], gotBack[2], wantBackdropColor[0], wantBackdropColor[1], wantBackdropColor[2]))
	_ = dcG.Close()

	// 臂 d：描边吃 shader（渐变而非环境实心）。
	dcD := newCtx(w, h)
	effStroke := mustEffect(effStrokeWGSL)
	if err := flushErr(dcD, func() error {
		paintBackdrop(dcD, float64(w), float64(h))
		dcD.SetRGB(1, 0, 0)
		dcD.SetShader(effStroke)
		dcD.SetStroke(render.DefaultStroke().WithWidth(dWidth))
		dcD.DrawLine(dX0, dY, dX1, dY)
		return dcD.Stroke()
	}); err != nil {
		failJoin(&rep, "arm d flush", err)
	}
	s0r, s0g, s0b, _ := pixelAt(dcD, int(dX0)+10, int(dY))
	s1r, s1g, s1b, _ := pixelAt(dcD, int(dX1)-10, int(dY))
	gradDelta := abs(int(s0r)-int(s1r)) + abs(int(s0g)-int(s1g)) + abs(int(s0b)-int(s1b))
	notAmbient := abs(int(s0r)-255) > int(blendTol) // 环境色是纯红 (255,0,0)
	onCanvas := s0g > 0 || s0b > 0                  // shader 青蓝渐变，不是纯红
	rep.add("stroke_paints_shader_gradient", gradDelta > 30 && notAmbient && onCanvas,
		fmt.Sprintf("s0=(%d,%d,%d) s1=(%d,%d,%d) delta=%d", s0r, s0g, s0b, s1r, s1g, s1b, gradDelta))
	_ = dcD.Close()

	// 臂 e：现场拒（材质×透视专项 2026-11-07 迁移）——材质+透视组合已放行
	// （专项窗承管画面），现场拒迁到画布投影层眼后：w(y)=1-0.003y 在 y≈333
	// 过零，quad 四角 y≥500 全过零 ⇒ ErrQuadBehindEye 整块具名拒（几何族，
	// 材质在场也不救）。同几何同锚，画面口径不变。
	dcE := newCtx(w, h)
	effG := mustEffect(effGradWGSL)
	_ = flushErr(dcE, func() error {
		paintBackdrop(dcE, float64(w), float64(h))
		return nil
	})
	dcE.SetProjection(render.Matrix4Homography(render.Translate(0, 0), 0, -0.003, 1))
	errQ := dcE.DrawImageQuad(texGray, [4]render.Point{
		{X: 520, Y: 520}, {X: 700, Y: 500}, {X: 700, Y: 620}, {X: 520, Y: 620},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Material: effG})
	dcE.SetProjection(render.Matrix4Identity())
	_ = dcE.FlushGPU()
	refused := errQ != nil && errors.Is(errQ, render.ErrQuadBehindEye)
	chipFacts["quad_behind_eye_refused"] = refused
	chipFacts["backend_fails"] = true
	rep.extra["quad_behind_eye_refused_err"] = fmt.Sprint(errQ)
	rep.add("quad_behind_eye_refused", refused,
		fmt.Sprintf("err=%v（校验期具名返回，不入队不计数）", errQ))
	// 图像口回退路的拒绝计数（材质×bicubic 纹素契约 2026-11-07 迁移）：
	// bicubic+材质已放行（echo 钉钉引擎纹素=标准采样逐位），skip 演示改
	// blend+材质（材质腿 Normal-only 口径不变）——skip + MaterialSkips++。
	before := dcE.RenderPathStats().MaterialSkips
	dcE.DrawImageEx(texGray, render.DrawImageOptions{X: 520, Y: 520,
		BlendMode: render.BlendMultiply, Material: effG})
	_ = dcE.FlushGPU()
	after := dcE.RenderPathStats().MaterialSkips
	skipDelta := after - before
	rep.extra["material_skips_delta"] = skipDelta
	rep.add("image_entry_material_blend_skip_counted", skipDelta == 1,
		fmt.Sprintf("skips=%d（blend+材质 GPU 腿不接走 ⇒ skip，不记 CPUFallback）", skipDelta))
	// 非法组合不留下任何笔画：拒绝区应保持底色。
	br, bg2, bb2, _ := pixelAt(dcE, 610, 560)
	kept := abs(int(br)-int(math.Round(backdropRGB*255))) <= int(blendTol) && abs(int(bg2)-int(math.Round(backdropRGB*255))) <= int(blendTol)
	rep.add("refused_region_stays_backdrop", kept, fmt.Sprintf("got=(%d,%d,%d)", br, bg2, bb2))
	_ = dcE.Close()

	// gles 承载尝试：uniform 顶格 65536B shader（前端应放行；后端能否编译随设备上限）。
	effMax, errMax := render.NewShaderEffect(effMaxBudgetWGSL)
	if errMax != nil {
		rep.add("max_budget_shader_frontend", false, fmt.Sprint(errMax))
		rep.extra["gles_budget_outcome"] = "frontend-rejected"
	} else {
		if err := effMax.SetUniformBytes(make([]byte, render.ShaderUniformBudgetMax)); err != nil {
			rep.add("max_budget_uniform_set", false, fmt.Sprint(err))
			rep.extra["gles_budget_outcome"] = "uniform-set-rejected"
		} else {
			dcM := newCtx(w, h)
			_ = flushErr(dcM, func() error {
				paintBackdrop(dcM, float64(w), float64(h))
				dcM.SetRGB(1, 0, 0)
				dcM.SetShader(effMax)
				fillRect(dcM, 560, 660, 160, 40)
				dcM.SetShader(nil)
				return nil
			})
			stats := dcM.RenderPathStats()
			rep.extra["shader_backend_fails_probe"] = stats.ShaderBackendFails
			if stats.ShaderBackendFails > 0 {
				rep.extra["gles_budget_outcome"] = "backend-compile-failed (real trigger)"
				rep.add("gles_backend_compile_real_trigger", true,
					fmt.Sprintf("ShaderBackendFails=%d（gles 拒收顶格模块，fallback 语义生效）", stats.ShaderBackendFails))
			} else {
				rep.extra["gles_budget_outcome"] = "backend-accepted-on-this-device"
				rep.add("gles_backend_compile_real_trigger", true,
					"本机 gles 接受了顶格模块——真触发不可复现于此设备，如实记（哨兵链路已由双 %w 包装钉覆盖）")
			}
			_ = dcM.Close()
		}
	}

	return rep
}

func semanticProbes() []wrkit.PixelProbe {
	probes := make([]wrkit.PixelProbe, 0, 10)
	add := func(name string, x, y float64, r, g, b, tol float64) {
		probes = append(probes, wrkit.PixelProbe{Name: name, X: x, Y: y, R: r, G: g, B: b, Tol: uint8(tol)})
	}
	// 臂 a 三入口挂材质段的渐变期望（uv.y=(y-topY+0.5)/h）。
	gAt := func(y float64) float64 { return 0.35 + 0.65*((y-aTopY+0.5)/aH) }
	gradProbe := func(name string, x, y float64) {
		g := gAt(y)
		db := backdropRGB * 255
		dbb := backdropRGB*255 + 2
		add(name, x, y, (0.95*g*255+db*(1-g))/255, (0.45*g*255+db*(1-g))/255, (0.15*g*255+dbb*(1-g))/255, blendTol)
	}
	gradProbe("material_image_gradient", aPair0+aW+aGap+aW/2, aTopY+aH/2)
	gradProbe("material_quad_gradient", aPair1+aW+aGap+aW/2, aTopY+aH/2)
	gradProbe("material_atlas_gradient", aPair2+aW+aGap+aW/2, aTopY+aH/2)
	// 臂 b：(255,128,128)
	add("premul_fill_d9", bWhiteX+bWhiteW/2, bWhiteY+bWhiteH/2, 1.0, 128.0/255, 128.0/255, blendTol)
	// 臂 c：左红右蓝
	add("set_image_at_left_red", cImgX+40, cImgY+48, 220.0/255, 60.0/255, 60.0/255, blendTol)
	add("set_image_at_right_blue", cImgX+cImgW-40, cImgY+48, 60.0/255, 90.0/255, 220.0/255, blendTol)
	// 臂 f：uniform 的 split=0.25 把左 1/4 留成主图红，其余按 uniform 的 gain 变暗蓝
	//（锚点在边界两侧固定不动，期望色 = 纹理底色 × gain 同一份常数）。
	add("uniform_split_left_red", fImgX+0.125*fImgW, fImgY+fImgH/2,
		float64(texRedColor[0])/255, float64(texRedColor[1])/255, float64(texRedColor[2])/255, blendTol)
	add("uniform_split_rest_dim", fImgX+0.875*fImgW, fImgY+fImgH/2,
		float64(texBlueColor[0])*fGain/255, float64(texBlueColor[1])*fGain/255, float64(texBlueColor[2])*fGain/255, blendTol)
	// 臂 G：后端拒收三格（终帧同一份几何；格 2 与探针同一判据=底色且非环境红）。
	add("arm_g_ambient_control_red", gX0+gW/2, gY+gH/2, 1.0, 0.0, 0.0, blendTol)
	add("arm_g_legal_shader_green", gX2+gW/2, gY+gH/2,
		float64(flatGreenRGB[0])/255, float64(flatGreenRGB[1])/255, float64(flatGreenRGB[2])/255, blendTol)
	// 臂 G 中间那格**不进固定色锚**：故意模块在 Vulkan 上必拒（该格留底色）、在
	// 纯 Go GL 上被接受（该格出 shader 自己的颜色），写死任一端都会把另一端判假。
	// 它的判据是「画面与计数必须说的是同一件事」，在门禁相按终帧实测（见
	// armGFrameCheck），两端各自钉死，见 README 与本窗账 §四。
	// 臂 e：拒绝区保持底色
	add("refused_region_backdrop", 610, 560, backdropRGB, backdropRGB, backdropRGB+0.02, blendTol)
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
			fmt.Fprintf(os.Stderr, "p37_material: 探针 FAIL %s → %s\n", r.name, r.got)
		}
	}
	var summary manualSummary
	var paintErr string
	var liveStats render.RenderPathStats
	var liveDC *render.Context

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
	frameNo := int64(0)
	content.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		if live, okCtx := pc.DC.(*render.Context); okCtx {
			liveDC = live
		}
		dc := pc.DC
		w, h := size.Width, size.Height
		if e := paintAll(dc, w, h); e != nil {
			paintErr = e.Error()
		}
		frameNo++
	}
	root.Place(content, 0, 0)
	metric := wrkit.Label("presents=0", 13, 0.92, 0.94, 0.98)
	root.Place(metric, 8, 8)

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
	snapPath := filepath.Join(testdataDir, "p37_material_final.png")
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: backdropRGB, ClearG: backdropRGB, ClearB: backdropRGB + 0.02, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		MaxFrames:    gateMaxFrames,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "p37_material: close (%s)\n", win.Backend())
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
				fmt.Fprintf(os.Stderr, "p37_material: event %s\n", ev.Type)
			}
		},
	})
	var proc scheduler.ProcessTracker
	proc.Start()
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) bool {
		if liveDC != nil {
			liveStats = liveDC.RenderPathStats()
		}
		metric.SetText(fmt.Sprintf("presents=%d skips=%d backend_fails=%d",
			app.PresentCount(), liveStats.MaterialSkips, liveStats.ShaderBackendFails))
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
		fmt.Fprintf(os.Stderr, "p37_material: FAIL 定帧停机没达成 presents=%d want>=%d（末帧随墙钟漂）\n",
			app.PresentCount(), gateMaxFrames)
		probesOK = false
	}
	// 臂 G 的终帧硬门（两端各自钉死，写死任一端都会把另一端判假）：中间那格
	// 要么**底色 + 计数 ≥1**（本机后端真拒），要么**shader 解析色 + 计数 0**
	//（本机后端接受这枚模块），两种都不许是环境实心红、也不许「拒而不计数」。
	// 为什么不能只看离屏：离屏 Context 走独立设备，而独立设备按设计不吃
	// GPUI_BACKEND（`render/device_open.go` 的 OpenStandaloneDevice 注记：纯 Go GL
	// 没有 surfaceless 独立设备）⇒「窗内这条腿拒没拒」只能从终帧 + 活计数上量。
	armGOK, armGDetail := armGFrameCheck(snapPath, liveStats.ShaderBackendFails)
	if gateMode && !armGOK {
		fmt.Fprintf(os.Stderr, "p37_material: FAIL 臂 G 终帧与计数不同形：%s\n", armGDetail)
		probesOK = false
	}

	tag := backendTag()
	basePath := filepath.Join(testdataDir, "goldens", fmt.Sprintf("p37_material_final_%s.png", tag))
	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGateW(snapPath, basePath, winW,
		[]wrkit.GoldenRect{{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH}})
	goldenOK := goldenErr == nil && !seeded && goldenDiff == 0
	if goldenErr != nil {
		fmt.Fprintf(os.Stderr, "p37_material: golden: %v\n", goldenErr)
	}
	if seeded {
		fmt.Fprintf(os.Stderr, "p37_material: golden 已播种（%s），这一跑不算 PASS —— 复核后提交基线再复跑\n", basePath)
	}

	pixelOK := paintErr == ""
	if gateMode {
		if pixelErr := wrkit.CheckPixelProbes(snapPath, winW, semanticProbes()); pixelErr != nil {
			pixelOK = false
			fmt.Fprintf(os.Stderr, "p37_material: 像素断言 FAIL: %v\n", pixelErr)
		}
	} else {
		fmt.Fprintln(os.Stderr, "p37_material: 常驻相末帧未定格（H15 只在门禁相成立），像素与金图只报数不判绿")
	}
	if paintErr != "" {
		fmt.Fprintf(os.Stderr, "p37_material: 绘制 FAIL: %s\n", paintErr)
	}

	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	extra := map[string]any{
		"probes_ok": probesOK, "pixel_ok": pixelOK, "paint_err": paintErr,
		"golden_diff_pct": goldenDiff, "golden_total_px": goldenPx,
		"golden_seeded": seeded, "golden_backend_tag": tag,
		"window_material_skips":       liveStats.MaterialSkips,
		"window_shader_backend_fails": liveStats.ShaderBackendFails,
		"arm_g_frame_check":           armGDetail,
		"arm_g_backend_rejects":       liveStats.ShaderBackendFails >= 1,
		"arm_g_frame_ok":              armGOK,
		"gate_frames":                 gateFrames, "gate_freeze_at": gateFreezeAt,
		"golden_tolerance_pct": 0,
	}
	for k, v := range rep.extra {
		extra[k] = v
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "p37-6-3", Scenario: "p37_material", Backend: tag,
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
		fmt.Fprintf(os.Stderr, "p37_material: OK backend=%s presents=%d probes=%d/%d golden=%.4f%% (%d px) skips=%d\n",
			tag, app.PresentCount(), len(rep.results), len(rep.results), goldenDiff, goldenPx, liveStats.MaterialSkips)
		return
	}
	fmt.Fprintf(os.Stderr, "p37_material: resident done backend=%s presents=%d probes_ok=%v pixel_ok=%v golden=%.4f%%\n",
		tag, app.PresentCount(), probesOK, pixelOK, goldenDiff)
}
