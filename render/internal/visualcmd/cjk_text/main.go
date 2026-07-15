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
	canvasH = 600
)

func main() {
	out := flag.String("out", "", "PNG output path")
	flag.Parse()
	if *out == "" {
		log.Fatal("-out is required")
	}

	fontPath := findCJKFont()
	if fontPath == "" {
		log.Fatal("no CJK font found")
	}

	source, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		log.Fatalf("load font: %v", err)
	}
	defer func() { _ = source.Close() }()
	defer render.CloseAccelerator()

	if accel := render.Accelerator(); accel != nil {
		fmt.Printf("accelerator=%s direct=%v font=%s path=%s\n",
			accel.Name(), render.AcceleratorCanRenderDirect(), source.Name(), fontPath)
	} else {
		fmt.Printf("accelerator=<nil> direct=false font=%s path=%s\n", source.Name(), fontPath)
	}

	dc := render.NewContext(canvasW, canvasH)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	face14 := source.Face(14)
	face16 := source.Face(16)

	dc.SetFont(face14)
	dc.SetRGB(0.2, 0.2, 0.8)
	dc.DrawString("CJK Body Text", 20, 30)

	dc.SetRGB(0, 0, 0)
	y := 60.0
	for _, size := range []float64{12, 14, 16, 18, 20, 24} {
		face := source.Face(size)
		dc.SetFont(face)
		label := fmt.Sprintf("%gpx: 中文测试 日本語テスト 한국어 — The quick brown fox", size)
		dc.DrawString(label, 20, y)
		y += size + 10
	}

	dc.SetFont(face14)
	dc.SetRGB(0.2, 0.2, 0.8)
	dc.DrawString("CJK Display Text", 20, y+10)
	dc.SetRGB(0, 0, 0)
	y += 45
	for _, size := range []float64{36, 48} {
		face := source.Face(size)
		dc.SetFont(face)
		dc.DrawString("中文大标题", 20, y)
		y += size + 12
	}

	dc.SetFont(face14)
	dc.SetRGB(0.2, 0.2, 0.8)
	dc.DrawString("Mixed Script", 500, 30)
	dc.SetRGB(0, 0, 0)
	dc.SetFont(face16)
	dc.DrawString("Hello 世界!", 500, 60)
	dc.DrawString("Go言語 is 素晴らしい", 500, 90)
	dc.DrawString("1234 가나다라", 500, 120)

	fmt.Println(dc.RenderPathStats().LogLine())

	if err := dc.SavePNG(*out); err != nil {
		log.Fatalf("save png: %v", err)
	}
}

func findCJKFont() string {
	candidates := []string{
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/arphic/uming.ttc",
		"C:/Windows/Fonts/msyh.ttc",
		"C:/Windows/Fonts/simsun.ttc",
		"C:/Windows/Fonts/malgun.ttf",
		"/System/Library/Fonts/PingFang.ttc",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
