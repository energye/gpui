//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// 深审 L3-A1 钉：Path1D/Path2D 盖章原型带 conic 时，GPU 描边腿必须在展开前
// 降换 conic（CPU 两腿已有防线）。旧状 GPU 腿效果输出直进 stroke expander →
// panic("conic verb reached Expand")——公开 API 组合（NewPath1DPathEffect +
// Path.ConicTo 原型）即崩，且全仓该组合零测试。

func TestP35StrokeConicPrototypeEffectNoPanic(t *testing.T) {
	requireFilterOpsGPU(t)
	conicProto := func(size float64) *render.Path {
		p := render.NewPath()
		p.MoveTo(-size, size*0.6)
		p.ConicTo(0, -size, size, size*0.6, 0.707)
		p.Close()
		return p
	}
	effects := []render.PathEffect{
		render.NewPath1DPathEffect(conicProto(5), render.Path1DTranslate, 12, 0),
		render.NewPath1DPathEffect(conicProto(5), render.Path1DRotate, 14, 0),
		render.NewPath2DPathEffect(conicProto(4), 14, 12),
	}
	for i, eff := range effects {
		dc := render.NewContext(96, 96)
		dc.ClearWithColor(render.White)
		dc.SetRGB(0, 0, 1)
		st := render.Stroke{
			Width: 1.5, Cap: render.LineCapButt, Join: render.LineJoinMiter, MiterLimit: 4,
		}
		dc.SetStroke(st.WithPathEffect(eff))
		dc.DrawRectangle(8, 8, 80, 64)
		if err := dc.Stroke(); err != nil {
			dc.Close()
			t.Fatalf("effect %d stroke: %v", i, err)
		}
		if err := dc.FlushGPU(); err != nil {
			dc.Close()
			t.Fatalf("effect %d flush: %v", i, err)
		}
		ink := 0
		img := dc.Image()
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, _, bl, _ := img.At(x, y).RGBA()
				if bl>>8 > 128 && r>>8 < 128 {
					ink++
				}
			}
		}
		dc.Close()
		if ink < 50 {
			t.Fatalf("effect %d painted %d ink px, want >=50（conic 盖章被丢/未画出）", i, ink)
		}
	}
}
