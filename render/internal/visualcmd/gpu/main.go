package main

import (
	"flag"
	"fmt"
	"log"
	"math"

	"github.com/energye/gpui/render"
)

const (
	canvasW = 800
	canvasH = 600
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

	dc := render.NewContext(canvasW, canvasH)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA2(0.95, 0.95, 0.98, 1))

	colors := [][3]float64{
		{0.9, 0.3, 0.3},
		{0.3, 0.9, 0.3},
		{0.3, 0.3, 0.9},
		{0.9, 0.9, 0.3},
		{0.9, 0.3, 0.9},
	}
	for i, c := range colors {
		x := 100 + float64(i)*150
		dc.SetRGB(c[0], c[1], c[2])
		dc.DrawCircle(x, 100, 50)
		_ = dc.Fill()
	}

	dc.SetRGBA(0.2, 0.4, 0.9, 0.8)
	dc.DrawRoundedRectangle(50, 200, 200, 120, 20)
	_ = dc.Fill()

	dc.SetRGBA(0.0, 0.9, 0.9, 1.0)
	dc.SetLineWidth(3.0)
	dc.DrawCircle(400, 260, 60)
	_ = dc.Stroke()

	dc.SetRGBA(1.0, 0.6, 0.1, 0.9)
	dc.MoveTo(600, 180)
	dc.LineTo(700, 320)
	dc.LineTo(500, 320)
	dc.ClosePath()
	_ = dc.Fill()

	dc.SetRGBA(0.2, 0.8, 0.4, 0.85)
	drawRegularPolygon(dc, 150, 430, 60, 5)
	_ = dc.Fill()

	dc.SetRGBA(0.8, 0.2, 0.6, 0.85)
	drawRegularPolygon(dc, 350, 430, 55, 6)
	_ = dc.Fill()

	dc.SetRGBA(1, 0.7, 0.1, 1.0)
	drawStar(dc, 550, 430, 65, 30, 5)
	_ = dc.Fill()

	dc.SetRGBA(0.4, 0.2, 0.8, 0.7)
	dc.MoveTo(650, 380)
	dc.CubicTo(750, 350, 750, 500, 650, 480)
	dc.CubicTo(550, 460, 550, 360, 650, 380)
	dc.ClosePath()
	_ = dc.Fill()

	_ = dc.FlushGPU()

	fmt.Println(dc.RenderPathStats().LogLine())

	if err := dc.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}

func drawRegularPolygon(dc *render.Context, cx, cy, radius float64, sides int) {
	for i := 0; i < sides; i++ {
		angle := float64(i)*2*math.Pi/float64(sides) - math.Pi/2
		x := cx + radius*math.Cos(angle)
		y := cy + radius*math.Sin(angle)
		if i == 0 {
			dc.MoveTo(x, y)
		} else {
			dc.LineTo(x, y)
		}
	}
	dc.ClosePath()
}

func drawStar(dc *render.Context, cx, cy, outerR, innerR float64, points int) {
	for i := 0; i < points*2; i++ {
		angle := float64(i)*math.Pi/float64(points) - math.Pi/2
		r := outerR
		if i%2 == 1 {
			r = innerR
		}
		x := cx + r*math.Cos(angle)
		y := cy + r*math.Sin(angle)
		if i == 0 {
			dc.MoveTo(x, y)
		} else {
			dc.LineTo(x, y)
		}
	}
	dc.ClosePath()
}
