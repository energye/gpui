package main

import (
	"flag"
	"fmt"
	"log"

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

	renderer := scene.NewRenderer(canvasW, canvasH,
		scene.WithWorkers(0),
		scene.WithCacheSize(32),
	)
	if renderer == nil {
		log.Fatal("failed to create scene renderer")
	}
	defer renderer.Close()

	target := render.NewPixmap(canvasW, canvasH)
	target.Clear(render.RGBA{R: 1, G: 1, B: 1, A: 1})

	sceneGraph := buildScene()
	if err := renderer.Render(target, sceneGraph); err != nil {
		log.Fatalf("render: %v", err)
	}

	stats := renderer.Stats()
	fmt.Printf("tiles_total=%d tiles_rendered=%d\n", stats.TilesTotal, stats.TilesRendered)

	if err := target.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}

func buildScene() *scene.Scene {
	builder := scene.NewSceneBuilder()

	builder.FillRect(0, 0, 512, 512,
		scene.SolidBrush(render.RGBA{R: 0.95, G: 0.95, B: 1.0, A: 1}))

	colors := []render.RGBA{
		{R: 0.9, G: 0.3, B: 0.3, A: 1},
		{R: 0.3, G: 0.9, B: 0.3, A: 1},
		{R: 0.3, G: 0.3, B: 0.9, A: 1},
		{R: 0.9, G: 0.9, B: 0.3, A: 1},
	}
	for i := 0; i < 4; i++ {
		x := float32(50 + i*110)
		builder.FillRect(x, 50, 80, 80, scene.SolidBrush(colors[i]))
	}

	builder.WithTransform(scene.TranslateAffine(256, 256), func(b *scene.SceneBuilder) {
		b.FillCircle(0, 0, 100, scene.SolidBrush(render.RGBA{R: 0.8, G: 0.4, B: 0.8, A: 0.7}))
		b.StrokeCircle(0, 0, 100, scene.SolidBrush(render.RGBA{R: 0.4, G: 0.2, B: 0.4, A: 1}), 3)
	})

	builder.Layer(scene.BlendMultiply, 0.8, nil, func(lb *scene.SceneBuilder) {
		lb.FillRect(200, 180, 150, 150,
			scene.SolidBrush(render.RGBA{R: 1, G: 0.8, B: 0.2, A: 1}))
	})

	builder.Layer(scene.BlendScreen, 0.6, nil, func(lb *scene.SceneBuilder) {
		lb.FillCircle(280, 280, 80,
			scene.SolidBrush(render.RGBA{R: 0.2, G: 0.6, B: 1, A: 1}))
	})

	builder.StrokeRect(30, 400, 100, 80,
		scene.SolidBrush(render.RGBA{R: 0.2, G: 0.2, B: 0.2, A: 1}), 2)

	builder.DrawLine(150, 440, 350, 440,
		scene.SolidBrush(render.RGBA{R: 0.5, G: 0.5, B: 0.5, A: 1}), 4)

	builder.Fill(
		scene.NewRoundedRectShape(380, 400, 100, 80, 15),
		scene.SolidBrush(render.RGBA{R: 0.3, G: 0.7, B: 0.5, A: 1}))

	return builder.Build()
}
