package main

import (
	"flag"
	"fmt"
	"image"
	"log"

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
	dc.ClearWithColor(render.RGB(0.95, 0.95, 0.95))

	testImg, err := createTestImage(100, 100)
	if err != nil {
		log.Fatalf("create test image: %v", err)
	}
	drawExamples(dc, testImg)

	if err := dc.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}

func createTestImage(width, height int) (*render.ImageBuf, error) {
	img, err := render.NewImageBuf(width, height, render.FormatRGBA8)
	if err != nil {
		return nil, fmt.Errorf("create image buffer: %w", err)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8(x * 255 / width)
			g := uint8(y * 255 / height)
			b := uint8((x + y) * 128 / (width + height))
			_ = img.SetRGBA(x, y, r, g, b, 255)
		}
	}
	return img, nil
}

func drawExamples(dc *render.Context, img *render.ImageBuf) {
	dc.SetRGB(0.2, 0.2, 0.2)
	drawText(dc, 400, 30)

	drawText(dc, 100, 80)
	dc.DrawImage(img, 50, 100)

	drawText(dc, 100, 240)
	dc.DrawImageEx(img, render.DrawImageOptions{
		X:             50,
		Y:             260,
		DstWidth:      200,
		DstHeight:     200,
		Interpolation: render.InterpBilinear,
		Opacity:       1.0,
		BlendMode:     render.BlendNormal,
	})

	drawText(dc, 350, 80)
	dc.DrawImageEx(img, render.DrawImageOptions{
		X:             300,
		Y:             100,
		Interpolation: render.InterpBilinear,
		Opacity:       0.5,
		BlendMode:     render.BlendNormal,
	})

	drawText(dc, 550, 80)
	dc.DrawImageEx(img, render.DrawImageOptions{
		X:             500,
		Y:             100,
		DstWidth:      150,
		DstHeight:     150,
		Interpolation: render.InterpNearest,
		Opacity:       1.0,
		BlendMode:     render.BlendNormal,
	})

	drawText(dc, 350, 240)
	srcRect := image.Rect(0, 0, 50, 50)
	dc.DrawImageEx(img, render.DrawImageOptions{
		X:             300,
		Y:             260,
		DstWidth:      100,
		DstHeight:     100,
		SrcRect:       &srcRect,
		Interpolation: render.InterpBilinear,
		Opacity:       1.0,
		BlendMode:     render.BlendNormal,
	})

	drawText(dc, 550, 240)
	dc.SetRGB(0.8, 0.6, 0.4)
	dc.DrawRectangle(500, 260, 100, 100)
	dc.Fill()
	dc.DrawImageEx(img, render.DrawImageOptions{
		X:             500,
		Y:             260,
		Interpolation: render.InterpBilinear,
		Opacity:       1.0,
		BlendMode:     render.BlendMultiply,
	})

	drawText(dc, 100, 490)
	dc.Push()
	dc.Translate(150, 540)
	dc.Rotate(0.3)
	dc.Scale(0.8, 0.8)
	dc.DrawImage(img, -50, -50)
	dc.Pop()

	drawText(dc, 350, 490)
	pattern := dc.CreateImagePattern(img, 0, 0, 50, 50)
	dc.SetFillPattern(pattern)
	dc.DrawCircle(450, 540, 60)
	dc.Fill()
}

func drawText(dc *render.Context, x, y float64) {
	dc.SetRGB(0.2, 0.2, 0.2)
	dc.DrawCircle(x-10, y, 3)
	dc.Fill()
}
