//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// BenchmarkSceneCreation benchmarks scene creation and encoding.
func BenchmarkSceneCreation(b *testing.B) {
	benchmarks := []struct {
		name  string
		setup func(s *scene.Scene)
	}{
		{"single_rect", func(s *scene.Scene) {
			rect := scene.NewRectShape(10, 10, 80, 80)
			s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.Red), rect)
		}},
		{"10_rects", func(s *scene.Scene) {
			for i := 0; i < 10; i++ {
				rect := scene.NewRectShape(float32(i*10), float32(i*10), 50, 50)
				s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.Red), rect)
			}
		}},
		{"100_rects", func(s *scene.Scene) {
			for i := 0; i < 100; i++ {
				x := float32((i % 10) * 50)
				y := float32((i / 10) * 50)
				rect := scene.NewRectShape(x, y, 40, 40)
				s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.Red), rect)
			}
		}},
		{"with_layers", func(s *scene.Scene) {
			rect := scene.NewRectShape(0, 0, 100, 100)
			s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.White), rect)

			s.PushLayer(scene.BlendMultiply, 0.8, nil)
			circle := scene.NewCircleShape(50, 50, 30)
			s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.Red), circle)
			s.PopLayer()
		}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				s := scene.NewScene()
				bm.setup(s)
				_ = s.Encoding()
			}
		})
	}
}
