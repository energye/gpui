// GPU Test: Text Cursor - 文本光标渲染验证
//
// 预期效果:
// =========
// 1. 末尾光标: "Text with cursor|" 28px, 黑色文字, 竖线在文字末尾
// 2. 中间光标: "Cursor position indicator" 28px, 黑色文字, 竖线在文字中间
// 3. 多位置光标: 3个不同颜色的竖线(红/蓝/绿) 在"Multiple cursor positions:"下方
// 4. 编辑光标: "Edit this text" 28px, 黑色文字, 竖线在文字中间位置
//
// 验证标准:
// - 光标位置正确
// - 光标高度与文字匹配
// - 光标颜色正确
// - 文字内容清晰可读
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
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Text Cursor", Width: 800, Height: 600})

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
			ctx.DrawString("Text with cursor|", 100, 100)
			ctx.SetLineWidth(2)
			ctx.MoveTo(370, 88)
			ctx.LineTo(370, 108)
			ctx.Stroke()
			ctx.DrawString("Cursor position indicator", 100, 200)
			ctx.SetRGBA(0, 0, 0, 0.5)
			ctx.MoveTo(420, 188)
			ctx.LineTo(420, 208)
			ctx.Stroke()
			ctx.SetFont(face20)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Multiple cursor positions:", 100, 300)
			ctx.SetRGBA(1, 0, 0, 1)
			ctx.SetLineWidth(2)
			ctx.MoveTo(100, 310)
			ctx.LineTo(100, 328)
			ctx.Stroke()
			ctx.SetRGBA(0, 0, 1, 1)
			ctx.MoveTo(200, 310)
			ctx.LineTo(200, 328)
			ctx.Stroke()
			ctx.SetRGBA(0, 0.5, 0, 1)
			ctx.MoveTo(300, 310)
			ctx.LineTo(300, 328)
			ctx.Stroke()
			ctx.SetFont(face28)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Edit this text", 100, 400)
			ctx.SetRGBA(0, 0, 0, 0.7)
			ctx.SetLineWidth(2)
			ctx.MoveTo(250, 388)
			ctx.LineTo(250, 408)
			ctx.Stroke()
			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/text_cursor/gpu_text_cursor.png"))
		})
	})
	app.AddWindow(win)
	app.Run()
}
