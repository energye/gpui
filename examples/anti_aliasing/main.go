// GPU Test: Anti-aliasing - 抗锯齿渲染验证
package main

import (
	"github.com/energye/gpui/examples"
	"github.com/energye/gpui/render"
	"github.com/energye/lcl/tool/exec"
	"path/filepath"
)

func main() {
	const W, H = 800, 600
	ctx := render.NewContext(W, H)
	ctx.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})

	face := examples.Face

	ctx.SetFont(face(14))
	ctx.SetRGBA(0.2, 0.5, 1, 1)
	ctx.DrawRoundedRectangle(100, 100, 80, 32, 4)
	ctx.Fill()
	ctx.SetRGBA(0.1, 0.3, 0.8, 1)
	ctx.SetLineWidth(1)
	ctx.DrawRoundedRectangle(100, 100, 80, 32, 4)
	ctx.Stroke()
	ctx.SetRGBA(0.9, 0.95, 1, 1)
	ctx.DrawRoundedRectangle(200, 100, 200, 40, 8)
	ctx.Fill()
	ctx.SetRGBA(0, 0, 0, 0.3)
	ctx.SetLineWidth(2)
	ctx.DrawRoundedRectangle(200, 100, 200, 40, 8)
	ctx.Stroke()
	ctx.SetRGBA(0.95, 0.95, 0.95, 1)
	ctx.DrawRoundedRectangle(100, 200, 300, 150, 16)
	ctx.Fill()
	ctx.SetRGBA(0, 0, 0, 0.2)
	ctx.SetLineWidth(2)
	ctx.DrawRoundedRectangle(100, 200, 300, 150, 16)
	ctx.Stroke()
	ctx.SetRGBA(0.2, 0.8, 0.4, 1)
	ctx.DrawRoundedRectangle(500, 100, 120, 32, 16)
	ctx.Fill()
	ctx.SetRGBA(1, 0.5, 0, 1)
	ctx.DrawCircle(600, 300, 60)
	ctx.Fill()
	ctx.SetRGBA(0.8, 0.4, 0, 1)
	ctx.SetLineWidth(2)
	ctx.DrawCircle(600, 300, 60)
	ctx.Stroke()
	ctx.SetRGBA(1, 0, 0, 1)
	ctx.DrawCircle(750, 150, 20)
	ctx.Fill()
	ctx.SetRGBA(0, 0, 0, 1)
	ctx.SetLineWidth(1)
	for i := 0; i < 8; i++ {
		a := float64(i) * 3.14159 / 8
		ctx.MoveTo(400, 450)
		ctx.LineTo(400+150*cos(a), 450+150*sin(a))
		ctx.Stroke()
	}
	ctx.SetFont(face(18))
	ctx.SetRGBA(0, 0, 0, 1)
	ctx.DrawString("Hello World 你好世界", 100, 400)
	ctx.SetFont(face(24))
	ctx.SetRGBA(0.2, 0.4, 0.8, 1)
	ctx.DrawString("Anti-aliasing Text Test", 100, 430)
	ctx.SetFont(face(14))
	ctx.SetRGBA(0.5, 0.5, 0.5, 1)
	ctx.DrawString("Small text for testing", 100, 450)
	ctx.SetRGBA(0.8, 0.2, 0.2, 0.7)
	ctx.DrawRoundedRectangle(500, 400, 120, 80, 8)
	ctx.Fill()
	ctx.SetRGBA(0.2, 0.8, 0.2, 0.7)
	ctx.DrawCircle(560, 480, 30)
	ctx.Fill()
	ctx.SetFont(face(14))
	ctx.SetRGBA(1, 1, 1, 1)
	ctx.DrawString("Mixed", 530, 435)
	ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/anti_aliasing/gpu_anti_aliasing.png"))
}

func cos(x float64) float64 { return 1 - x*x/2 + x*x*x*x/24 }

func sin(x float64) float64 { return x - x*x*x/6 + x*x*x*x*x/120 }
