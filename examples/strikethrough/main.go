// GPU Test: Strikethrough - 删除线文本渲染验证
//
// 预期效果:
// =========
// 1. 普通删除线: "This text has strikethrough" 28px, 黑色文字+黑色线
// 2. 红色删除线: "Red strikethrough example" 28px, 红色文字+红色线
// 3. 双删除线: "Double strikethrough line" 20px, 黑色双线
// 4. 粗删除线: "Thick green strikethrough" 28px, 绿色文字+绿色粗线(4px)
//
// 验证标准:
// - 删除线位置在文字中间
// - 线宽正确
// - 文字可读性不受影响
package main

import (
	"github.com/energye/gpui/examples"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui"
	"github.com/energye/lcl/api/libname"
	"github.com/energye/lcl/tool/exec"
	"log"
	"path/filepath"
)

func main() {
	libname.UseWS = "gtk3"
	app := ui.NewApplication()
	win := ui.NewWindow(ui.WindowConfig{Title: "Strikethrough", Width: 800, Height: 600})

	win.OnInit(func(ctrl *ui.TGPUControl) {
		src, err := text.NewFontSource(examples.Font)
		if err != nil {
			log.Fatalf("Font load error: %v", err)
		}
		face28 := src.Face(28)
		face20 := src.Face(20)

		ctrl.SetOnRender(func(ctx *render.Context) {
			ctx.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetFont(face28)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("This text has strikethrough", 100, 100)
			ctx.SetLineWidth(2)
			ctx.MoveTo(100, 108)
			ctx.LineTo(480, 108)
			ctx.Stroke()
			ctx.SetRGBA(0.8, 0, 0, 1)
			ctx.DrawString("Red strikethrough example", 100, 200)
			ctx.SetRGBA(1, 0, 0, 1)
			ctx.MoveTo(100, 208)
			ctx.LineTo(430, 208)
			ctx.Stroke()
			ctx.SetFont(face20)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Double strikethrough line", 100, 300)
			ctx.SetLineWidth(1)
			ctx.MoveTo(100, 306)
			ctx.LineTo(400, 306)
			ctx.Stroke()
			ctx.MoveTo(100, 312)
			ctx.LineTo(400, 312)
			ctx.Stroke()
			ctx.SetFont(face28)
			ctx.SetRGBA(0, 0.5, 0, 1)
			ctx.DrawString("Thick green strikethrough", 100, 400)
			ctx.SetRGBA(0, 0.5, 0, 1)
			ctx.SetLineWidth(4)
			ctx.MoveTo(100, 408)
			ctx.LineTo(460, 408)
			ctx.Stroke()
			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/strikethrough/gpu_strikethrough.png"))
		})
	})
	app.AddWindow(win)
	app.Run()
}
