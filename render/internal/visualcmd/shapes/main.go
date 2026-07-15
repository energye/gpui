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

	dc.ClearWithColor(render.White)
	drawShapes(dc)

	if err := dc.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}

func drawShapes(dc *render.Context) {
	dc.SetRGB(0.8, 0.2, 0.2)
	dc.DrawRectangle(50, 50, 150, 100)
	dc.Fill()

	dc.SetRGB(0.2, 0.8, 0.2)
	dc.DrawRoundedRectangle(250, 50, 150, 100, 20)
	dc.Fill()

	dc.SetRGB(0.2, 0.2, 0.8)
	dc.DrawCircle(500, 100, 60)
	dc.Fill()

	dc.SetRGB(0.8, 0.8, 0.2)
	dc.DrawEllipse(650, 100, 80, 50)
	dc.Fill()

	dc.SetRGB(1, 0.5, 0)
	dc.DrawRegularPolygon(5, 100, 300, 50, -math.Pi/2)
	dc.Fill()

	dc.SetRGB(0.5, 0, 1)
	dc.DrawRegularPolygon(6, 250, 300, 50, 0)
	dc.Fill()

	dc.SetRGB(0, 0.8, 0.8)
	dc.DrawRegularPolygon(8, 400, 300, 50, 0)
	dc.Fill()

	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(3)
	dc.DrawLine(50, 450, 750, 450)
	dc.Stroke()

	dc.SetRGB(0.8, 0, 0.8)
	dc.SetLineWidth(5)
	dc.DrawArc(650, 300, 60, 0, math.Pi*1.5)
	dc.Stroke()

	dc.Push()
	dc.Translate(400, 500)
	dc.Rotate(math.Pi / 4)
	dc.SetRGB(0.2, 0.6, 0.8)
	dc.DrawRectangle(-40, -40, 80, 80)
	dc.Fill()
	dc.Pop()
}
