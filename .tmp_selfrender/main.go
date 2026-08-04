package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/energye/gpui/render/text"
)

// selfrender: 用引擎自研路径(text.Draw + RasterizeHinted + 整数放置)渲染顶栏同布局,
// 与 bar_ft_light.png 做并排对照。
func main() {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	for i := range img.Pix {
		img.Pix[i] = 20
	}
	for y := 6; y < 70; y++ {
		for x := 10; x < 950; x++ {
			img.SetRGBA(x, y, color.RGBA{41, 46, 66, 255})
		}
	}
	for yy := 24; yy < 50; yy++ {
		for xx := 340; xx < 426; xx++ {
			img.SetRGBA(xx, yy, color.RGBA{56, 133, 224, 255})
		}
		for xx := 424; xx < 510; xx++ {
			img.SetRGBA(xx, yy, color.RGBA{158, 107, 51, 255})
		}
	}

	cjkSrc, err := text.NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	if err != nil {
		fmt.Println("FAIL cjk:", err)
		os.Exit(1)
	}
	latSrc, err := text.NewFontSourceFromFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	if err != nil {
		fmt.Println("FAIL latin:", err)
		os.Exit(1)
	}
	mode := text.HintingVertical
	latMode := text.HintingFull
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "none":
			mode = text.HintingNone
			latMode = text.HintingNone
		case "full":
			mode = text.HintingFull
			latMode = text.HintingFull
		case "noto":
			mode = text.HintingVertical // CJK Vertical, Latin Full — 但拉丁也用 Noto 字形
		}
	}
	useNotoLatin := len(os.Args) > 3
	draw := func(s string, size float64, x, by float64, c color.RGBA, cjk bool) {
		src := latSrc
		if cjk || useNotoLatin {
			src = cjkSrc
		}
		hm := mode
		if !cjk && !useNotoLatin {
			hm = latMode
		}
		face := src.Face(size, text.WithHinting(hm))
		text.Draw(img, s, face, x, by, c)
	}
	white := color.RGBA{242, 245, 252, 255}
	btnWhite := color.RGBA{255, 255, 255, 255}
	szWhite := color.RGBA{235, 242, 255, 255}

	draw("SHELL — 标题/按钮/相位 壳层", 16, 24, 32, white, true)
	draw("按钮-1", 11, 340, 35, btnWhite, true)
	draw("按钮-2", 11, 424, 35, btnWhite, true)
	for i, s := range []float64{8, 10, 12, 14, 16} {
		xs := []float64{540, 592, 650, 714, 784}
		draw(fmt.Sprintf("%dpx 样张", int(s)), s, xs[i], 50+s, szWhite, true)
	}
	f, _ := os.Create(os.Args[1])
	defer f.Close()
	png.Encode(f, img)
	fmt.Println("OK", os.Args[1])
}
