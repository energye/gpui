package main

import (
	"flag"
	"fmt"
	"log"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

const (
	canvasW = 512
	canvasH = 512
)

func main() {
	out := flag.String("out", "", "PNG output path")
	flag.Parse()
	if *out == "" {
		log.Fatal("-out is required")
	}
	defer render.CloseAccelerator()

	if accel := render.Accelerator(); accel != nil {
		fmt.Printf("accelerator=%s direct=%v\n", accel.Name(), render.AcceleratorCanRenderDirect())
	} else {
		fmt.Println("accelerator=<nil> direct=false")
	}

	renderer := scene.NewRenderer(canvasW, canvasH)
	if renderer == nil {
		log.Fatal("failed to create scene renderer")
	}
	defer renderer.Close()

	target := render.NewPixmap(canvasW, canvasH)
	target.Clear(render.RGBA{R: 0.08, G: 0.08, B: 0.12, A: 1})

	s := buildScene(canvasW, canvasH)
	if err := renderer.Render(target, s); err != nil {
		log.Fatalf("render: %v", err)
	}

	stats := renderer.Stats()
	if stats.TilesRendered == 0 {
		fmt.Println("path=gpu tiles_rendered=0")
	} else {
		fmt.Printf("path=cpu tiles_rendered=%d\n", stats.TilesRendered)
	}

	fmt.Println("gpu_ops=0 cpu_fallback_ops=0 note=scene_renderer")
	if err := target.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}

func buildScene(w, h int) *scene.Scene {
	b := scene.NewSceneBuilder()
	cx, cy := float32(w)/2, float32(h)/2

	b.FillRect(0, 0, float32(w), float32(h),
		scene.SolidBrush(render.RGBA{R: 0.1, G: 0.1, B: 0.18, A: 1}))

	n := 12
	radius := float32(140)
	for i := 0; i < n; i++ {
		angle := float64(i) * 2 * math.Pi / float64(n)
		x := cx + radius*float32(math.Cos(angle))
		y := cy + radius*float32(math.Sin(angle))
		r := 12 + float32(i)*3
		color := hueToRGB(float64(i) / float64(n))
		b.FillCircle(x, y, r, scene.SolidBrush(color))
	}

	b.FillCircle(cx, cy, 60, scene.SolidBrush(render.RGBA{R: 0.2, G: 0.3, B: 0.8, A: 0.8}))
	b.StrokeCircle(cx, cy, 60, scene.SolidBrush(render.RGBA{R: 0.4, G: 0.5, B: 1, A: 1}), 2)
	b.StrokeCircle(cx, cy, radius, scene.SolidBrush(render.RGBA{R: 0.3, G: 0.3, B: 0.4, A: 0.5}), 1)

	b.FillRect(20, 20, 80, 80, scene.SolidBrush(render.RGBA{R: 0.9, G: 0.3, B: 0.3, A: 0.7}))
	b.Fill(scene.NewRoundedRectShape(float32(w)-100, 20, 80, 80, 12),
		scene.SolidBrush(render.RGBA{R: 0.3, G: 0.9, B: 0.3, A: 0.7}))
	b.Fill(scene.NewRoundedRectShape(20, float32(h)-100, 80, 80, 12),
		scene.SolidBrush(render.RGBA{R: 0.3, G: 0.3, B: 0.9, A: 0.7}))
	b.FillRect(float32(w)-100, float32(h)-100, 80, 80,
		scene.SolidBrush(render.RGBA{R: 0.9, G: 0.9, B: 0.3, A: 0.7}))

	b.Layer(scene.BlendScreen, 0.6, nil, func(lb *scene.SceneBuilder) {
		lb.FillCircle(cx-80, cy+80, 50,
			scene.SolidBrush(render.RGBA{R: 1, G: 0.4, B: 0.2, A: 1}))
	})

	return b.Build()
}

func hueToRGB(h float64) render.RGBA {
	h = h - math.Floor(h)
	s, v := 1.0, 1.0
	i := int(h * 6)
	f := h*6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, q
	case 5:
		r, g, b = v, p, q
	}
	return render.RGBA{R: r, G: g, B: b, A: 1}
}
