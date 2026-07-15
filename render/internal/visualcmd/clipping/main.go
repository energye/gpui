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
	canvasH = 900
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

	dc.ClearWithColor(render.White)
	example1CircularClip(dc)
	example2RectClip(dc)
	example3ClipPreserve(dc)
	example4NestedClips(dc)
	example5ComplexClip(dc)
	example6RoundRectClip(dc)
	example7ResetClip(dc)

	if err := dc.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}

func example1CircularClip(dc *render.Context) {
	dc.Push()
	dc.DrawCircle(150, 150, 80)
	dc.Clip()
	for i := 0; i < 200; i++ {
		hue := float64(i) / 200.0
		dc.SetRGB(hue, 0.7, 0.9)
		dc.DrawRectangle(50+float64(i), 50, 2, 200)
		dc.Fill()
	}
	dc.Pop()
}

func example2RectClip(dc *render.Context) {
	dc.Push()
	dc.ClipRect(300, 50, 160, 160)
	for y := 0; y < 200; y += 10 {
		for x := 0; x < 200; x += 10 {
			dc.SetRGB(float64(x)/200.0, float64(y)/200.0, 0.5)
			dc.DrawRectangle(float64(250+x), float64(y), 10, 10)
			dc.Fill()
		}
	}
	dc.Pop()

	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(2)
	dc.DrawRectangle(300, 50, 160, 160)
	dc.Stroke()
}

func example3ClipPreserve(dc *render.Context) {
	dc.Push()
	drawStar(dc, 150, 450, 70, 5)
	dc.ClipPreserve()
	for i := 0; i < 200; i++ {
		dc.SetRGBA(0.8, 0.3, float64(i)/200.0, 1)
		dc.DrawRectangle(50, 350+float64(i), 200, 2)
		dc.Fill()
	}
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(3)
	dc.Stroke()
	dc.Pop()
}

func example4NestedClips(dc *render.Context) {
	dc.Push()
	dc.ClipRect(300, 350, 160, 160)
	for y := 0; y < 200; y += 5 {
		dc.SetRGBA(0.2, 0.6, 0.9, 0.8)
		dc.DrawRectangle(250, 300+float64(y), 250, 3)
		dc.Fill()
	}
	dc.Push()
	dc.DrawCircle(380, 430, 60)
	dc.Clip()
	for i := 0; i < 50; i++ {
		dc.SetRGBA(0.9, 0.3, 0.2, 0.7)
		dc.DrawCircle(380, 430, float64(60-i))
		dc.Stroke()
	}
	dc.Pop()
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(2)
	dc.DrawRectangle(300, 350, 160, 160)
	dc.Stroke()
	dc.Pop()
}

func example5ComplexClip(dc *render.Context) {
	dc.Push()
	drawComplexClipPath(dc)
	dc.Clip()
	dc.SetLineWidth(2)
	for i := -100; i < 300; i += 8 {
		dc.SetRGB(0.2, 0.7, 0.4)
		dc.DrawLine(float64(550+i), 50, float64(650+i), 200)
		dc.Stroke()
	}
	dc.Pop()

	drawComplexClipPath(dc)
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(3)
	dc.Stroke()
}

func example6RoundRectClip(dc *render.Context) {
	dc.Push()
	dc.ClipRoundRect(50, 600, 200, 140, 25)
	colors := [][3]float64{
		{0.9, 0.2, 0.3},
		{0.2, 0.7, 0.9},
		{0.9, 0.8, 0.2},
		{0.3, 0.9, 0.4},
		{0.7, 0.3, 0.9},
	}
	for i, c := range colors {
		dc.SetRGB(c[0], c[1], c[2])
		dc.DrawCircle(80+float64(i)*35, 650+float64(i)*12, 45)
		dc.Fill()
	}
	dc.Pop()

	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(50, 600, 200, 140, 25)
	dc.Stroke()
}

func example7ResetClip(dc *render.Context) {
	dc.Push()
	dc.ClipRect(300, 600, 160, 160)
	dc.SetRGB(0.9, 0.7, 0.3)
	dc.DrawRectangle(250, 550, 250, 250)
	dc.Fill()
	dc.ResetClip()
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(2)
	dc.DrawRectangle(300, 600, 160, 160)
	dc.Stroke()
	dc.Pop()
}

func drawComplexClipPath(dc *render.Context) {
	dc.MoveTo(600, 120)
	dc.CubicTo(650, 50, 700, 50, 750, 120)
	dc.CubicTo(700, 180, 650, 180, 600, 120)
	dc.ClosePath()
}

func drawStar(dc *render.Context, cx, cy, r float64, points int) {
	angle := 2 * math.Pi / float64(points)
	halfAngle := angle / 2
	for i := 0; i < points*2; i++ {
		a := float64(i) * halfAngle
		radius := r
		if i%2 == 1 {
			radius = r * 0.4
		}
		x := cx + radius*math.Cos(a-math.Pi/2)
		y := cy + radius*math.Sin(a-math.Pi/2)
		if i == 0 {
			dc.MoveTo(x, y)
		} else {
			dc.LineTo(x, y)
		}
	}
	dc.ClosePath()
}
