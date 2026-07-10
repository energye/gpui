// GPU Test: Arrows - 箭头渲染验证
//
// 预期效果:
// =========
// 1. 红色箭头: 从(100,100)到(200,100), 线宽2px, 指向右方
// 2. 绿色箭头: 从(300,100)到(200,100), 线宽3px, 指向左方
// 3. 蓝色箭头: 从(400,200)到(400,100), 线宽2px, 指向上方
// 4. 黄色箭头: 从(500,100)到(500,200), 线宽3px, 指向下方
// 5. 紫色双向箭头: 从(100,300)到(250,300)再返回, 线宽2px
// 6. 青色45°箭头: 从(400,400)到(550,250), 线宽2px
// 7. 多色箭头组合: 4条平行线, 线宽1-4px递增
// 8. 文字标注: 每个箭头下方有文字说明
//
// 验证标准:
// - 箭头尖端清晰锐利, 无断裂
// - 线段平滑无锯齿
// - 箭头方向正确
// - 文字标注清晰
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
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Arrow Test", Width: 800, Height: 600})

	win.OnInit(func(ctrl *gpui.TGPUControl) {
		// Font loading with fallback paths
		src, err := text.NewFontSource(examples.Font)
		if err != nil {
			log.Fatalf("Font load error: %v", err)
		}
		face14 := src.Face(14)

		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetFont(face14)

			drawArrow(ctx, 100, 100, 200, 100, gg.RGBA{R: 1, G: 0, B: 0, A: 1}, 2)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Right", 100, 80)

			drawArrow(ctx, 300, 100, 200, 100, gg.RGBA{R: 0, G: 1, B: 0, A: 1}, 3)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Left", 200, 80)

			drawArrow(ctx, 400, 200, 400, 100, gg.RGBA{R: 0, G: 0, B: 1, A: 1}, 2)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Up", 380, 80)

			drawArrow(ctx, 500, 100, 500, 200, gg.RGBA{R: 1, G: 1, B: 0, A: 1}, 3)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Down", 480, 220)

			drawArrow(ctx, 100, 300, 250, 300, gg.RGBA{R: 0.5, G: 0, B: 0.5, A: 1}, 2)
			drawArrow(ctx, 250, 300, 100, 300, gg.RGBA{R: 0.5, G: 0, B: 0.5, A: 1}, 2)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Double", 120, 280)

			drawArrow(ctx, 400, 400, 550, 250, gg.RGBA{R: 0, G: 0.5, B: 0.5, A: 1}, 2)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("45° Arrow", 420, 240)

			colors := []gg.RGBA{{R: 1, G: 0, B: 0, A: 1}, {R: 0, G: 1, B: 0, A: 1}, {R: 0, G: 0, B: 1, A: 1}, {R: 1, G: 1, B: 0, A: 1}}
			for i, c := range colors {
				drawArrow(ctx, 600, 100+float64(i*50), 750, 100+float64(i*50), c, float64(i)+1)
			}
			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/arrow/gpu_arrow.png"))
		})
	})
	app.AddWindow(win)
	app.Run()
}

func drawArrow(ctx *gg.Context, x1, y1, x2, y2 float64, col gg.RGBA, w float64) {
	dx, dy := x2-x1, y2-y1
	l := sqrt(dx*dx + dy*dy)
	if l < 1 {
		return
	}
	ux, uy := dx/l, dy/l
	ctx.SetRGBA(col.R, col.G, col.B, col.A)
	ctx.SetLineWidth(w)
	ctx.MoveTo(x1, y1)
	ctx.LineTo(x2, y2)
	ctx.Stroke()
	aL := 15.0
	aA := 0.5
	ctx.MoveTo(x2, y2)
	ctx.LineTo(x2-aL*(ux*cos(aA)-uy*sin(aA)), y2-aL*(uy*cos(aA)+ux*sin(aA)))
	ctx.MoveTo(x2, y2)
	ctx.LineTo(x2-aL*(ux*cos(aA)+uy*sin(aA)), y2-aL*(uy*cos(aA)-ux*sin(aA)))
	ctx.Stroke()
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	s := x / 2
	for i := 0; i < 10; i++ {
		s = (s + x/s) / 2
	}
	return s
}

func cos(x float64) float64 { return 1 - x*x/2 + x*x*x*x/24 }

func sin(x float64) float64 { return x - x*x*x/6 + x*x*x*x*x/120 }
