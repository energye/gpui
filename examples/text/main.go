// GPU Test: Freetype Text - 文字渲染验证
//
// 预期效果:
// =========
// 1. 字号14px: "The quick brown fox..." 黑色
// 2. 字号18px: "The quick brown fox jumps..." 黑色
// 3. 字号24px: "ABCDEFGHIJKLMNOPQRSTUVWXYZ" 黑色
// 4. 字号28px: "abcdefghijklmnopqrstuvwxyz" 黑色
// 5. 字号36px: "0123456789" 黑色
// 6. 中文: "中文文本渲染测试 - 你好世界" 蓝色 20px
// 7. 日文: "日本語のテキストレンダリング - こんにちは" 红色 20px
// 8. 韩文: "한국어 텍스트 렌더링 - 안녕하세요" 绿色 20px
// 9. 混合: "Mixed Text English + 中文 + 日本語 + 한국어" 紫色 20px
// 10. 彩色文字: 5种不同颜色的"Colored Text" 16px
//
// 验证标准:
// - 文字边缘平滑, 无锯齿
// - 中/日/韩/英文混排正确
// - 不同字号比例正确
// - 文字颜色正确
package main

import (
	"github.com/energye/gpui/examples"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui"
	"github.com/energye/lcl/api/libname"
	"github.com/energye/lcl/tool/exec"
	"path/filepath"
)

func main() {
	libname.UseWS = "gtk3"

	sources := examples.LoadTextSources()
	face := func(size float64) text.Face {
		return examples.MakeFallbackFace(sources, size)
	}

	app := ui.NewApplication()

	win := ui.NewWindow(ui.WindowConfig{
		Title:  "GPUI - Text Rendering",
		Width:  800,
		Height: 600,
	})
	win.OnInit(func(ctrl *ui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *render.Context) {
			drawTextDemo(ctx, face)
			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/text/gpu_text.png"))
		})
	})

	app.AddWindow(win)
	app.Run()
}

func drawTextDemo(ctx *render.Context, face func(float64) text.Face) {
	ctx.ClearWithColor(render.White)
	//ctx.SetTextMode(render.TextModeBitmap)

	ctx.SetFont(face(12))
	ctx.SetRGBA(0, 0, 0, 1)
	ctx.DrawString("Font Size 12 - The quick brown fox jumps over the lazy dog", 100, 40)

	ctx.SetFont(face(14))
	ctx.SetRGBA(0, 0, 0, 1)
	ctx.DrawString("Font Size 14 - The quick brown fox jumps over the lazy dog", 100, 80)

	ctx.SetFont(face(18))
	ctx.DrawString("Font Size 18 - The quick brown fox jumps over the lazy dog", 100, 120)

	ctx.SetFont(face(24))
	ctx.DrawString("Font Size 24 - ABCDEFGHIJKLMNOPQRSTUVWXYZ", 100, 160)

	ctx.SetFont(face(28))
	ctx.DrawString("Font Size 28 - abcdefghijklmnopqrstuvwxyz", 100, 210)

	ctx.SetFont(face(36))
	ctx.DrawString("Font Size 36 - 0123456789", 100, 270)

	ctx.SetFont(face(20))
	ctx.SetRGBA(0, 0, 0.8, 1)
	ctx.DrawString("中文文本渲染测试 - 你好世界", 100, 320)
	ctx.SetRGBA(0.8, 0, 0, 1)
	ctx.DrawString("日本語のテキストレンダリング - こんにちは", 100, 360)
	ctx.SetRGBA(0, 0.5, 0, 1)
	ctx.DrawString("한국어 텍스트 렌더링 - 안녕하세요", 100, 400)
	ctx.SetRGBA(0.5, 0, 0.5, 1)
	ctx.DrawString("Mixed Text English + 中文 + 日本語 + 한국어", 100, 440)

	ctx.SetFont(face(16))
	colors := []render.RGBA{
		{R: 1, G: 0, B: 0, A: 1}, {R: 0, G: 1, B: 0, A: 1},
		{R: 0, G: 0, B: 1, A: 1}, {R: 1, G: 1, B: 0, A: 1},
		{R: 0.8, G: 0, B: 0.8, A: 1},
	}
	for i, c := range colors {
		ctx.SetRGBA(c.R, c.G, c.B, c.A)
		ctx.DrawString("Colored Text Example", 100+float64(i)*130, 500)
	}
}
