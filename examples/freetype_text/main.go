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
	"github.com/energye/lcl/tool/exec"
	"log"
	"path/filepath"
)

func main() {
	const W, H = 800, 600
	ctx := render.NewContext(W, H)
	ctx.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})

	src, err := text.NewFontSource(examples.Font)
	if err != nil {
		log.Fatalf("Font load error: %v", err)
	}
	defer src.Close()

	ctx.SetFont(src.Face(14))
	ctx.SetRGBA(0, 0, 0, 1)
	ctx.DrawString("Font Size 14 - The quick brown fox jumps over the lazy dog", 100, 80)

	ctx.SetFont(src.Face(18))
	ctx.DrawString("Font Size 18 - The quick brown fox jumps over the lazy dog", 100, 120)

	ctx.SetFont(src.Face(24))
	ctx.DrawString("Font Size 24 - ABCDEFGHIJKLMNOPQRSTUVWXYZ", 100, 160)

	ctx.SetFont(src.Face(28))
	ctx.DrawString("Font Size 28 - abcdefghijklmnopqrstuvwxyz", 100, 210)

	ctx.SetFont(src.Face(36))
	ctx.DrawString("Font Size 36 - 0123456789", 100, 270)

	ctx.SetFont(src.Face(20))
	ctx.SetRGBA(0, 0, 0.8, 1)
	ctx.DrawString("中文文本渲染测试 - 你好世界", 100, 320)
	ctx.SetRGBA(0.8, 0, 0, 1)
	ctx.DrawString("日本語のテキストレンダリング - こんにちは", 100, 360)
	ctx.SetRGBA(0, 0.5, 0, 1)
	ctx.DrawString("한국어 텍스트 렌더링 - 안녕하세요", 100, 400)
	ctx.SetRGBA(0.5, 0, 0.5, 1)
	ctx.DrawString("Mixed Text English + 中文 + 日本語 + 한국어", 100, 440)

	ctx.SetFont(src.Face(16))
	colors := []render.RGBA{
		{R: 1, G: 0, B: 0, A: 1}, {R: 0, G: 1, B: 0, A: 1},
		{R: 0, G: 0, B: 1, A: 1}, {R: 1, G: 1, B: 0, A: 1},
		{R: 0.8, G: 0, B: 0.8, A: 1},
	}
	for i, c := range colors {
		ctx.SetRGBA(c.R, c.G, c.B, c.A)
		ctx.DrawString("Colored Text Example", 100+float64(i)*130, 500)
	}
	ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/freetype_text/gpu_freetype_text.png"))
}
