// GPU Test: Anti-aliasing - 抗锯齿渲染验证
//
// 预期效果:
// =========
//
//  1. 小圆角矩形 (按钮尺寸)
//     位置: (100,100) 大小: 80x32  圆角半径: 4
//     填充色: rgba(0.2, 0.5, 1.0, 1.0) 蓝色
//     边框色: rgba(0.1, 0.3, 0.8, 1.0) 深蓝 线宽1px
//     预期: 边缘平滑无锯齿, 四角弧度均匀
//
//  2. 中圆角矩形 (输入框尺寸)
//     位置: (200,100) 大小: 200x40  圆角半径: 8
//     填充色: rgba(0.9, 0.95, 1.0, 1.0) 浅蓝
//     边框色: rgba(0, 0, 0, 0.3) 半透明黑色 线宽2px
//     预期: 边缘平滑, 圆角过渡自然
//
//  3. 大圆角矩形 (卡片尺寸)
//     位置: (100,200) 大小: 300x150  圆角半径: 16
//     填充色: rgba(0.95, 0.95, 0.95, 1.0) 浅灰
//     边框色: rgba(0, 0, 0, 0.2) 线宽2px
//     预期: 大圆角平滑, 无阶梯感
//
//  4. 胶囊形 (Tag/Pill)
//     位置: (500,100) 大小: 120x32  圆角半径: 16
//     填充色: rgba(0.2, 0.8, 0.4, 1.0) 绿色
//     预期: 两端半圆, 连接处平滑
//
//  5. 圆形
//     位置: (600,300) 半径: 60
//     填充色: rgba(1.0, 0.5, 0, 1.0) 橙色
//     边框色: rgba(0.8, 0.4, 0, 1.0) 深橙 线宽2px
//     预期: 圆形完美无椭圆变形, 边缘平滑
//
//  6. 小圆形 (头像尺寸)
//     位置: (750,150) 半径: 20
//     填充色: rgba(1.0, 0, 0, 1.0) 红色
//     预期: 小尺寸圆形边缘仍保持平滑
//
//  7. 斜线测试 - 8条不同角度线段
//     起点: (400,450) 长度: 150px
//     角度: 0° 22.5° 45° 67.5° 90° 112.5° 135° 157.5°
//     线宽: 1px  颜色: 黑色
//     预期: 各角度线段边缘平滑, 无像素化阶梯
//
// 8. 文字渲染测试
//
//   - 18px字体: "Hello World 你好世界" 黑色
//
//   - 24px字体: "Anti-aliasing Text Test" 蓝色
//
//   - 14px字体: "Small text for testing" 灰色
//     预期: 文字边缘平滑, 中英文混排正确
//
//     9. 混合形状
//     圆角矩形: (500,400) 120x80 r=8 红色半透明
//     圆形: (560,480) r=30 绿色半透明
//     文字: "Mixed" 白色 14px 在圆角矩形上
//     预期: 形状叠加正确, 透明度混合正确
//
// 验证标准:
// - 所有形状边缘平滑无锯齿
// - 无像素化阶梯
// - 文字清晰可读
// - 透明度混合正确
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
	"github.com/energye/lcl/lcl"
)

func main() {
	libname.LibName = "/home/yanghy/app/workspace/gen/gout/libenergy-x86_64-linux.so"
	libname.UseWS = "gtk3"
	lcl.Init()
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "AA Test", Width: 800, Height: 600})

	win.OnInit(func(ctrl *gpui.TGPUControl) {
		// Load font
		ctx := gg.NewContext(800, 600)
		if err := ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 14); err != nil {
			panic(err)
		}

		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})

			// 1. 小圆角矩形
			ctx.SetRGBA(0.2, 0.5, 1, 1)
			ctx.DrawRoundedRectangle(100, 100, 80, 32, 4)
			ctx.Fill()
			ctx.SetRGBA(0.1, 0.3, 0.8, 1)
			ctx.SetLineWidth(1)
			ctx.DrawRoundedRectangle(100, 100, 80, 32, 4)
			ctx.Stroke()

			// 2. 中圆角矩形
			ctx.SetRGBA(0.9, 0.95, 1, 1)
			ctx.DrawRoundedRectangle(200, 100, 200, 40, 8)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 0.3)
			ctx.SetLineWidth(2)
			ctx.DrawRoundedRectangle(200, 100, 200, 40, 8)
			ctx.Stroke()

			// 3. 大圆角矩形
			ctx.SetRGBA(0.95, 0.95, 0.95, 1)
			ctx.DrawRoundedRectangle(100, 200, 300, 150, 16)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 0.2)
			ctx.SetLineWidth(2)
			ctx.DrawRoundedRectangle(100, 200, 300, 150, 16)
			ctx.Stroke()

			// 4. 胶囊形
			ctx.SetRGBA(0.2, 0.8, 0.4, 1)
			ctx.DrawRoundedRectangle(500, 100, 120, 32, 16)
			ctx.Fill()

			// 5. 圆形
			ctx.SetRGBA(1, 0.5, 0, 1)
			ctx.DrawCircle(600, 300, 60)
			ctx.Fill()
			ctx.SetRGBA(0.8, 0.4, 0, 1)
			ctx.SetLineWidth(2)
			ctx.DrawCircle(600, 300, 60)
			ctx.Stroke()

			// 6. 小圆形
			ctx.SetRGBA(1, 0, 0, 1)
			ctx.DrawCircle(750, 150, 20)
			ctx.Fill()

			// 7. 斜线
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.SetLineWidth(1)
			for i := 0; i < 8; i++ {
				angle := float64(i) * 3.14159 / 8
				ctx.MoveTo(400, 450)
				ctx.LineTo(400+150*cos(angle), 450+150*sin(angle))
				ctx.Stroke()
			}

			// 8. 文字渲染
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 18)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Hello World 你好世界", 100, 400)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 24)
			ctx.SetRGBA(0.2, 0.4, 0.8, 1)
			ctx.DrawString("Anti-aliasing Text Test", 100, 430)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 14)
			ctx.SetRGBA(0.5, 0.5, 0.5, 1)
			ctx.DrawString("Small text for testing", 100, 450)

			// 9. 混合形状
			ctx.SetRGBA(0.8, 0.2, 0.2, 0.7)
			ctx.DrawRoundedRectangle(500, 400, 120, 80, 8)
			ctx.Fill()
			ctx.SetRGBA(0.2, 0.8, 0.2, 0.7)
			ctx.DrawCircle(560, 480, 30)
			ctx.Fill()
			ctx.SetRGBA(1, 1, 1, 1)
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 14)
			ctx.DrawString("Mixed", 530, 435)

			ctx.SavePNG("gpu_anti_aliasing.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}
func cos(x float64) float64 { return 1 - x*x/2 + x*x*x*x/24 }
func sin(x float64) float64 { return x - x*x*x/6 + x*x*x*x*x/120 }
