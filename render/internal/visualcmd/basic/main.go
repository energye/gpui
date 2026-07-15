package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/energye/gpui/render"
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

	dc := render.NewContext(canvasW, canvasH)
	defer dc.Close()

	dc.ClearWithColor(render.White)

	dc.SetRGB(1, 0, 0)
	dc.DrawCircle(256, 256, 100)
	dc.Fill()

	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(100, 100, 150, 100)
	dc.Fill()

	dc.SetRGB(0, 1, 0)
	dc.SetLineWidth(5)
	dc.DrawCircle(400, 150, 50)
	dc.Stroke()

	fmt.Println(dc.RenderPathStats().LogLine())

	if err := dc.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}
