package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	canvasW = 800
	canvasH = 400
)

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
		fmt.Printf("accelerator=%s direct=%v font=%s\n", accel.Name(), render.AcceleratorCanRenderDirect(), source.Name())
	} else {
		fmt.Printf("accelerator=<nil> direct=false font=%s\n", source.Name())
	}

	dc := render.NewContext(canvasW, canvasH)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	face48 := source.Face(48)
	face24 := source.Face(24)
	face16 := source.Face(16)

	dc.SetFont(face48)
	dc.SetRGB(0.1, 0.1, 0.1)
	dc.DrawString("Hello, GoGPU!", 50, 80)

	dc.SetFont(face24)
	dc.SetRGB(0.3, 0.3, 0.3)
	dc.DrawString("Text rendering with TrueType fonts", 50, 130)

	dc.SetFont(face16)
	dc.SetRGB(0.5, 0.5, 0.5)
	dc.DrawString("Left aligned (default)", 50, 180)

	dc.SetRGB(0.2, 0.4, 0.8)
	dc.DrawStringAnchored("Center aligned", 400, 220, 0.5, 0.5)

	dc.SetRGB(0.8, 0.2, 0.2)
	dc.DrawStringAnchored("Right aligned", 750, 260, 1.0, 0.5)

	dc.SetFont(face24)
	testText := "Measured text"
	w, h := dc.MeasureString(testText)
	dc.SetRGB(0.9, 0.9, 0.9)
	dc.DrawRectangle(50, 290, w, h)
	dc.Fill()
	dc.SetRGB(0.1, 0.5, 0.1)
	dc.DrawString(testText, 50, 290+h*0.8)

	dc.SetFont(face16)
	dc.SetRGB(0.4, 0.4, 0.4)
	dc.DrawString("Font: "+source.Name(), 50, 370)

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
