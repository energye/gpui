// GPU Test: Freetype Text - 文字渲染验证
//
// 预期效果:
// =========
// 1. 字号12px: "The quick brown fox..." 黑色
// 2. 字号16px: "The quick brown fox..." 黑色
// 3. 字号20px: "ABCDEFGHIJKLMNOPQRSTUVWXYZ" 黑色
// 4. 字号28px: "abcdefghijklmnopqrstuvwxyz" 黑色
// 5. 字号36px: "0123456789" 黑色
// 6. 中文文本: "中文文本渲染测试 - 你好世界" 蓝色 20px
// 7. 混合文本: "Mixed Text English + 中文" 红色 20px
// 8. 彩色文字: 5种不同颜色的"Colored Text Example" 16px
// 9. 位置标注: 每个文字样式的描述标签
//
// 验证标准:
// - 文字边缘平滑, 无锯齿
// - 中英文混排正确
// - 不同字号比例正确
// - 文字颜色正确
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Text Test", Width: 800, Height: 600})

	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R:1,G:1,B:1,A:1})
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 12)
			ctx.SetRGBA(0,0,0,1); ctx.DrawString("Font Size 12 - The quick brown fox jumps over the lazy dog", 100, 80)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 16)
			ctx.DrawString("Font Size 16 - The quick brown fox jumps over the lazy dog", 100, 120)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 20)
			ctx.DrawString("Font Size 20 - ABCDEFGHIJKLMNOPQRSTUVWXYZ", 100, 160)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 28)
			ctx.DrawString("Font Size 28 - abcdefghijklmnopqrstuvwxyz", 100, 210)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 36)
			ctx.DrawString("Font Size 36 - 0123456789", 100, 270)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 20)
			ctx.SetRGBA(0,0,0.8,1); ctx.DrawString("中文文本渲染测试 - 你好世界", 100, 320)
			ctx.SetRGBA(0.8,0,0,1); ctx.DrawString("混合文本 Mixed Text English + 中文", 100, 360)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 16)
			colors := []gg.RGBA{{R:1,G:0,B:0,A:1},{R:0,G:1,B:0,A:1},{R:0,G:0,B:1,A:1},{R:1,G:1,B:0,A:1},{R:0.8,G:0,B:0.8,A:1}}
			for i, c := range colors {
				ctx.SetRGBA(c.R,c.G,c.B,c.A); ctx.DrawString("Colored Text Example", 100+float64(i)*130, 420)
			}
			ctx.SavePNG("gpu_freetype_text.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}