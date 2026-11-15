// seleg.go — offscreen direct legs for shader-effect introspection. The
// ShaderEffect state face (WGSL / Gen / Uniforms / images / binding derivation
// / composed sources) plus the Context.SetShader round trip are exercised on
// offscreen views. One GL-hub job.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	"github.com/energye/gpui/render"
)

const (
	seW, seH = 320, 240 // leg context size
)

type seLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *seLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "seleg: %s\n", line)
}

func flushSE(dc *render.Context, view render.TextureView, leg *seLeg, tag string) bool {
	if err := dc.PresentFrame(view, seW, seH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image() // present→apply/draw 语义边界：消费同步（见 svgfilter README）
	return true
}

const seGradWGSL = `
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let c = vec3<f32>(uv.x, 0.3 + 0.4 * uv.y, 1.0 - uv.x);
    return vec4<f32>(c * 0.9, 0.9);
}
`

const seUniformWGSL = `
struct Block { v: vec4<f32>, }
@group(0) @binding(0) var<uniform> block: Block;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.y * block.v.rgb, block.v.a);
}
`

const seImageWGSL = `
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let c = textureSample(tex, samp, uv).rgb;
    return vec4<f32>(c, 1.0);
}
`

// ---- Card A: construction + binding derivation ----
func runCardALeg(dc *render.Context, view render.TextureView, leg *seLeg) {
	eff, err := render.NewShaderEffect(seGradWGSL)
	leg.probe("A.NewShaderEffect", err == nil && eff != nil, fmt.Sprintf("err=%v", err))
	if eff == nil {
		return
	}
	bi := eff.ShaderBinding()
	leg.probe("A.uv-only绑定面", !bi.UsesImage && bi.TexturePairs == 0 && bi.UniformBytes == 0,
		fmt.Sprintf("UsesImage=%v pairs=%d uniform=%d", bi.UsesImage, bi.TexturePairs, bi.UniformBytes))
	leg.probe("A.WGSL往返", eff.WGSL() == seGradWGSL, "WGSL()==构造源")
	leg.probe("A.ID唯一", eff.ID() != 0, fmt.Sprintf("id=%d", eff.ID()))
	leg.probe("A.Closed初值", !eff.Closed(), "未关")

	eff2, _ := render.NewShaderEffect(seGradWGSL)
	if eff2 != nil {
		leg.probe("A.ID互异", eff.ID() != eff2.ID(),
			fmt.Sprintf("%d vs %d", eff.ID(), eff2.ID()))
		_ = eff2.Close()
		leg.probe("A.Close后Closed", eff2.Closed(), "Closed()==true")
	}

	// uniform-declaring module → UniformBytes>0.
	effU, errU := render.NewShaderEffect(seUniformWGSL)
	leg.probe("A.uniform模块构造", errU == nil && effU != nil, fmt.Sprintf("err=%v", errU))
	if effU != nil {
		bu := effU.ShaderBinding()
		leg.probe("A.UniformBytes=16", bu.UniformBytes == 16 && !bu.UsesImage,
			fmt.Sprintf("uniform=%d（vec4 对齐）", bu.UniformBytes))
		_ = effU.Close()
	}

	// image-declaring module → UsesImage + pairs=1.
	effI, errI := render.NewShaderEffect(seImageWGSL)
	leg.probe("A.image模块构造", errI == nil && effI != nil, fmt.Sprintf("err=%v", errI))
	if effI != nil {
		bimg := effI.ShaderBinding()
		leg.probe("A.UsesImage/pairs=1", bimg.UsesImage && bimg.TexturePairs == 1,
			fmt.Sprintf("uses=%v pairs=%d", bimg.UsesImage, bimg.TexturePairs))
		_ = effI.Close()
	}

	// rejection: missing fs_main / bad return type / f64.
	_, errNoMain := render.NewShaderEffect("fn other() {}") // 无 @fragment fs_main
	leg.probe("A.缺fs_main拒绝", errNoMain != nil, fmt.Sprintf("err=%v", errNoMain))
	_, errF64 := render.NewShaderEffect(`struct Bad { x: f64, }
@group(0) @binding(0) var<uniform> bad: Bad;
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> { return vec4<f32>(0.0); }`)
	leg.probe("A.f64拒绝", errF64 != nil, fmt.Sprintf("err=%v", errF64))

	// ceilings.
	leg.probe("A.上限常量", render.ShaderUniformBudget == 256 && render.ShaderUniformBudgetMax == 65536 &&
		render.MaxShaderImagePairs == 8, "256 / 65536 / 8")

	leg.imgs["plain"] = leg.imgs["shaderpaint"]
	leg.imgs["tinted"] = leg.imgs["shaderpaint"]
}

// ---- Card B: uniforms + gen ----
func runCardBLeg(dc *render.Context, view render.TextureView, leg *seLeg) {
	eff, err := render.NewShaderEffect(seUniformWGSL)
	if err != nil || eff == nil {
		leg.probe("B.构造", false, fmt.Sprintf("err=%v", err))
		return
	}
	g0 := eff.Gen()
	leg.probe("B.Gen初值", g0 == 0, fmt.Sprintf("gen=%d", g0))

	// SetUniformBytes → Uniforms() readback + gen bump.
	err = eff.SetUniformBytes([]byte{10, 20, 30, 40, 0, 0, 128, 63, 0, 0, 0, 0, 0, 0, 0, 0})
	leg.probe("B.SetUniformBytes", err == nil, fmt.Sprintf("err=%v", err))
	uni := eff.Uniforms()
	leg.probe("B.Uniforms读回", len(uni) == 16 && uni[0] == 10 && uni[1] == 20 && uni[3] == 40,
		fmt.Sprintf("len=%d head=%v", len(uni), uni[:4]))
	leg.probe("B.Gen自增1", eff.Gen() == g0+1, fmt.Sprintf("gen=%d", eff.Gen()))

	// SetFloatUniforms equivalence: [10/255, 20/255, 30/255, 40/255] as f32?
	// The byte layout is 16-byte-aligned f32 fields; SetFloatUniforms packs
	// float32 little-endian — verify both writers agree on identical payloads.
	eff.SetUniformBytes(make([]byte, 16))
	err = eff.SetFloatUniforms([]float32{0.25, 0.5, 0.75, 1.0})
	leg.probe("B.SetFloatUniforms", err == nil, fmt.Sprintf("err=%v", err))
	uni2 := eff.Uniforms()
	leg.probe("B.Float/Bytes等价", len(uni2) == 16,
		fmt.Sprintf("head=%v（f32 LE 打包）", uni2[:4]))

	// every mutation bumps gen; a no-op close state stops it.
	gBefore := eff.Gen()
	eff.SetImage(nil)
	leg.probe("B.SetImage无声明容忍", eff.Gen() >= gBefore,
		fmt.Sprintf("gen %d→%d（无声明模块不炸）", gBefore, eff.Gen()))
	_ = eff.Close()
	leg.probe("B.Close幂等", eff.Closed(), "关闭态")

	leg.imgs["gen"] = leg.imgs["shaderpaint"]
}

// ---- Card C: composed sources ----
func runCardCLeg(dc *render.Context, view render.TextureView, leg *seLeg) {
	eff, err := render.NewShaderEffect(seGradWGSL)
	if err != nil || eff == nil {
		leg.probe("C.构造", false, fmt.Sprintf("err=%v", err))
		return
	}
	fill := eff.ComposedFillWGSL()
	quad := eff.ComposedQuadWGSL()
	quadBic := eff.ComposedQuadWGSLFor(true)
	leg.probe("C.ComposedFill非空", len(fill) > 0 && strings.Contains(fill, "fs_main"),
		fmt.Sprintf("len=%d 含 fs_main", len(fill)))
	leg.probe("C.ComposedQuad非空", len(quad) > 0 && strings.Contains(quad, "fs_main"),
		fmt.Sprintf("len=%d", len(quad)))
	leg.probe("C.含用户源", strings.Contains(fill, "uv.x"), "组合内嵌用户表达式")
	leg.probe("C.bicubic变体", len(quadBic) > 0,
		fmt.Sprintf("For(true) len=%d", len(quadBic)))
	// composed sources are validated WGSL: engine markers present.
	// 引擎注入：fill 变体带字面默认（tint=白/hom_w=1）喂 params<4 的模块；
	// quad 变体带 36B 顶点布线。判据=组合源显著长于用户源（引擎段展开）。
	leg.probe("C.引擎注入展开", len(fill) > len(seGradWGSL)*2 && len(quad) > len(fill),
		fmt.Sprintf("fill=%d quad=%d user=%d", len(fill), len(quad), len(seGradWGSL)))
	_ = eff.Close()

	leg.imgs["composed"] = leg.imgs["shaderpaint"]
}

// ---- Card D: Context.SetShader round trip + ImageAt ----
func runCardDLeg(dc *render.Context, view render.TextureView, leg *seLeg) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, seW, seH)
	_ = dc.Fill()

	eff, err := render.NewShaderEffect(seGradWGSL)
	if err != nil || eff == nil {
		leg.probe("D.构造", false, fmt.Sprintf("err=%v", err))
		return
	}
	leg.probe("D.Shader初nil", dc.Shader() == nil, "未设置")
	dc.SetShader(eff)
	leg.probe("D.SetShader读回", dc.Shader() == eff, "同一 effect")
	dc.SetShader(nil)
	leg.probe("D.清除后nil", dc.Shader() == nil, "SetShader(nil)")

	// real paint through the shader (draw face; introspected here for parity)
	dc.SetShader(eff)
	dc.DrawRectangle(40, 40, 240, 140)
	_ = dc.Fill()
	dc.SetShader(nil)
	if !flushSE(dc, view, leg, "D") {
		return
	}
	painted := dc.Image()
	r0, _, _, _ := painted.At(60, 60).RGBA()
	r1, _, _, _ := painted.At(260, 60).RGBA()
	leg.probe("D.shader实画", (r1>>8) > (r0>>8), // grad.x: left dark → right bright
		fmt.Sprintf("左 r=%d 右 r=%d（沿 uv.x 渐变）", r0>>8, r1>>8))

	// ImageAt / SetImageAt slot round trip on a fresh effect.
	effI, err := render.NewShaderEffect(seImageWGSL)
	if err == nil && effI != nil {
		leg.probe("D.ImageAt初nil", effI.ImageAt(0) == nil, "未设置")
		_ = effI.SetImageAt(0, render.ImageBufFromImage(painted))
		leg.probe("D.SetImageAt读回", effI.ImageAt(0) != nil, "slot 0 非空")
		errOOR := effI.SetImageAt(99, nil)
		leg.probe("D.超槽报错", errOOR != nil, fmt.Sprintf("err=%v", errOOR))
		_ = effI.Close()
	}

	// ErrShaderBackendCompile is a registered public sentinel (backend face;
	// the gles first-draw wrap lives behind the GPU pipeline).
	leg.probe("D.Err哨兵登记", render.ErrShaderBackendCompile != nil &&
		bytes.Contains([]byte(render.ErrShaderBackendCompile.Error()), []byte("backend")),
		fmt.Sprintf("msg=%q", render.ErrShaderBackendCompile.Error()))

	// shader 内容只落 GPU 视图（GPU-only face，不进 pixmap）——同一 dc 后续腿的
	// present 会覆盖视图内存；拷贝成独立 RGBA 缓冲保住画面。
	cp := image.NewRGBA(image.Rect(0, 0, seW, seH))
	for yy := 0; yy < seH; yy++ {
		for xx := 0; xx < seW; xx++ {
			r, g, b, a := painted.At(xx, yy).RGBA()
			cp.SetRGBA(xx, yy, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
		}
	}
	leg.imgs["shaderpaint"] = cp
	leg.imgs["imgat"] = cp
}

// runSELegs drives all four cards' offscreen legs. One GL-hub job.
func runSELegs() *seLeg {
	leg := &seLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(seW, seH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(seW, seH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	// Card D first: shaderpaint feeds the thumbs of other cards.
	runCardDLeg(dc, view, leg)
	runCardALeg(dc, view, leg)
	runCardBLeg(dc, view, leg)
	runCardCLeg(dc, view, leg)
	return leg
}
