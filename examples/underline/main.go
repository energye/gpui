// GPU Test: Underline - 下划线文本渲染验证
//
// 预期效果:
// =========
// 1. 普通下划线: "Underlined text example" 28px, 黑色文字+黑色线
// 2. 蓝色下划线: "Blue underlined text" 28px, 蓝色文字+蓝色线
// 3. 粗绿色下划线: "Thick green underline" 20px, 绿色文字+绿色粗线(3px)
// 4. 双红色下划线: "Double red underline" 20px, 红色文字+红色双线
//
// 验证标准:
// - 下划线位置在文字底部
// - 线宽正确
// - 不同颜色显示正确
package main

import (
	"github.com/energye/gpui/examples"
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/gpui/internal/gg/text"
	"github.com/energye/lcl/api/libname"
	"github.com/energye/lcl/tool/exec"
	"log"
	"path/filepath"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Underline", Width: 800, Height: 600})

	win.OnInit(func(ctrl *gpui.TGPUControl) {
		src, err := text.NewFontSource(examples.Font)
		if err != nil {
			log.Fatalf("Font load error: %v", err)
		}
		face28 := src.Face(28)
		face20 := src.Face(20)

		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetFont(face28)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Underlined text example", 100, 100)
			ctx.SetLineWidth(1)
			ctx.MoveTo(100, 110)
			ctx.LineTo(450, 110)
			ctx.Stroke()
			ctx.SetRGBA(0, 0, 0.8, 1)
			ctx.DrawString("Blue underlined text", 100, 200)
			ctx.SetRGBA(0, 0, 0.8, 1)
			ctx.MoveTo(100, 210)
			ctx.LineTo(400, 210)
			ctx.Stroke()
			ctx.SetFont(face20)
			ctx.SetRGBA(0, 0.5, 0, 1)
			ctx.DrawString("Thick green underline", 100, 300)
			ctx.SetRGBA(0, 0.5, 0, 1)
			ctx.SetLineWidth(3)
			ctx.MoveTo(100, 310)
			ctx.LineTo(380, 310)
			ctx.Stroke()
			ctx.SetRGBA(0.8, 0, 0, 1)
			ctx.DrawString("Double red underline", 100, 400)
			ctx.SetRGBA(0.8, 0, 0, 1)
			ctx.SetLineWidth(1)
			ctx.MoveTo(100, 410)
			ctx.LineTo(380, 410)
			ctx.Stroke()
			ctx.MoveTo(100, 416)
			ctx.LineTo(380, 416)
			ctx.Stroke()
			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/underline/gpu_underline.png"))
		})
	})
	app.AddWindow(win)
	app.Run()
}
