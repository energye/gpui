package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	canvasW = 900
	canvasH = 700

	cols = 3
	rows = 3

	cellW = 280.0
	cellH = 210.0

	gridLeft = 25.0
	gridTop  = 70.0

	textOffX = 30.0
	textOffY = 100.0
)

type cell struct {
	label     string
	transform func(dc *render.Context)
}

func main() {
	out := flag.String("out", "", "PNG output path")
	flag.Parse()
	if *out == "" {
		log.Fatal("-out is required")
	}

	fontPath := findSystemFont()
	if fontPath == "" {
		log.Fatal("no system font found")
	}

	source, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		log.Fatalf("load font: %v", err)
	}
	defer func() { _ = source.Close() }()
	defer render.CloseAccelerator()

	if accel := render.Accelerator(); accel != nil {
		fmt.Printf("accelerator=%s direct=%v\n", accel.Name(), render.AcceleratorCanRenderDirect())
	} else {
		fmt.Println("accelerator=<nil> direct=false")
	}

	dc := render.NewContext(canvasW, canvasH)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	faceTitle := source.Face(28)
	faceLabel := source.Face(12)
	faceText := source.Face(24)

	dc.SetFont(faceTitle)
	dc.SetRGB(0.1, 0.1, 0.1)
	dc.DrawString("Text Transform Pipeline (diagnostic)", gridLeft, 45)

	cells := [rows][cols]cell{
		{
			{"Identity", func(_ *render.Context) {}},
			{"Translate(100, 50)", func(dc *render.Context) { dc.Translate(100, 50) }},
			{"Scale(2, 2)", func(dc *render.Context) { dc.Scale(2, 2) }},
		},
		{
			{"Scale(0.7, 0.7)", func(dc *render.Context) { dc.Scale(0.7, 0.7) }},
			{"Scale(3, 1) non-uniform", func(dc *render.Context) { dc.Scale(3, 1) }},
			{"Rotate(pi/6 = 30deg)", func(dc *render.Context) { dc.Rotate(math.Pi / 6) }},
		},
		{
			{"Rotate(pi/4 = 45deg)", func(dc *render.Context) { dc.Rotate(math.Pi / 4) }},
			{"Shear (faux italic)", func(dc *render.Context) { dc.Shear(-0.3, 0) }},
			{"Scale(2)+Rotate(pi/8)", func(dc *render.Context) {
				dc.Scale(2, 2)
				dc.Rotate(math.Pi / 8)
			}},
		},
	}

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			c := cells[row][col]
			cx := gridLeft + float64(col)*cellW + float64(col)*10
			cy := gridTop + float64(row)*cellH + float64(row)*5

			dc.SetRGB(0.95, 0.95, 0.95)
			dc.DrawRectangle(cx, cy, cellW, cellH)
			dc.Fill()

			dc.SetRGB(0.8, 0.8, 0.8)
			dc.SetLineWidth(1)
			dc.DrawRectangle(cx, cy, cellW, cellH)
			dc.Stroke()

			dc.SetFont(faceLabel)
			dc.SetRGB(0.45, 0.45, 0.45)
			dc.DrawString(c.label, cx+8, cy+16)

			dc.Push()
			dc.ClipRect(cx, cy, cellW, cellH)
			dc.Translate(cx+textOffX, cy+textOffY)
			c.transform(dc)

			dc.SetRGB(0.9, 0.2, 0.2)
			dc.SetLineWidth(0.5)
			dc.DrawLine(-6, 0, 6, 0)
			dc.Stroke()
			dc.DrawLine(0, -6, 0, 6)
			dc.Stroke()

			dc.SetFont(faceText)
			dc.SetRGB(0.12, 0.12, 0.12)
			dc.DrawString("Hello gg!", 0, 0)

			dc.Pop()
		}
	}

	fmt.Println(dc.RenderPathStats().LogLine())

	if err := dc.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}

func findSystemFont() string {
	candidates := []string{
		"C:\\Windows\\Fonts\\arial.ttf",
		"C:\\Windows\\Fonts\\calibri.ttf",
		"C:\\Windows\\Fonts\\segoeui.ttf",
		"/Library/Fonts/Arial.ttf",
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/System/Library/Fonts/Supplemental/Courier New.ttf",
		"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
		"/System/Library/Fonts/Monaco.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
