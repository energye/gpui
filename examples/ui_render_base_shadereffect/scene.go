// scene.go paints the shadereffect showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_shadereffect — ShaderEffect 内省面 · WGSL/Gen/Uniforms/Binding/Composed"

var (
	faceTitle text.Face
	faceLeg   text.Face
	faceCard  text.Face
	faceSmall text.Face
)

func ensureFaces() {
	faceTitle = wrkit.FaceAt(16)
	faceLeg = wrkit.FaceAt(12)
	faceCard = wrkit.FaceAt(13)
	faceSmall = wrkit.FaceAt(11)
}

// probe anchors
var (
	colShaderPaint = [3]float64{0.85, 0.45, 0.20} // shader 渐变画面主题色
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A 构造+绑定面: ShaderBinding 三字段",
		"B Uniforms/Gen: 状态变更自增",
		"C 组合源: Composed Fill/Quad",
		"D Context.SetShader 实画 + 读回",
		"probe: 内省读回+腿读回",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

func thumbImg(dc *render.Context, imgs map[string]image.Image, x, y, ix float64, name, label string) {
	if img := imgs[name]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 90})
	}
	wrkit.DirectDrawText(dc, label, x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
}

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A NewShaderEffect · ShaderInterface 派生绑定面", faceCard)
	thumbImg(dc, imgs, x, y, 0, "plain", "uv-only")
	thumbImg(dc, imgs, x, y, 128, "tinted", "tint 声明")
	wrkit.DirectDrawText(dc, "UsesImage/TexturePairs/UniformBytes 与模块声明一致", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B SetUniformBytes/SetFloatUniforms · Gen 自增 · Uniforms 读回", faceCard)
	thumbImg(dc, imgs, x, y, 0, "gen", "gen 阶梯")
	wrkit.DirectDrawText(dc, "每次状态变更 gen+1 · Uniforms()==SetBytes · SetFloat 与 Bytes 等价", x+16, y+146, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C ComposedFillWGSL/ComposedQuadWGSL(For) 组合源", faceCard)
	thumbImg(dc, imgs, x, y, 0, "composed", "组合源非空")
	wrkit.DirectDrawText(dc, "组合含用户源 · 含 fs_main · For(bicubic) 变体存在性", x+16, y+146, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D Context.SetShader 实画 · Shader() 读回 · ImageAt/SetImageAt", faceCard)
	if img := imgs["shaderpaint"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "SetShader 后 Shader()==effect · 清除后 nil", x+16, y+248, faceSmall, wrkit.DColTextDim)
	thumbImg(dc, imgs, x, y, 296-16, "imgat", "ImageAt")
	wrkit.DirectDrawText(dc, "slot 读写一致 · ErrShader* 登记", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *seLeg) {
	imgs := map[string]image.Image{}
	if leg != nil {
		imgs = leg.imgs
	}
	paintChrome()(dc)
	paintCardA(dc, imgs)
	paintCardB(dc, imgs)
	paintCardC(dc, imgs)
	paintCardD(dc, imgs)
}
