package main

import (
	"fmt"
	"image"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

var (
	v1Base  = [3]float64{0.30, 0.34, 0.55}
	varBase = [3]float64{0.62, 0.28, 0.38}
	qTR     = [3]float64{0.75, 0.25, 0.45}
	rectL   = image.Rect(24, 40, 128, 120)
)

func fill(dc *render.Context, r image.Rectangle, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawRectangle(float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()))
	_ = dc.Fill()
}

func px(img image.Image, x, y int) string {
	r, g, b, _ := img.At(x, y).RGBA()
	return fmt.Sprintf("(%d,%d,%d)", r>>8, g>>8, b>>8)
}

func sample(img image.Image, tag string) {
	fmt.Printf("%s: Lc=%s corner=%s br=%s\n", tag, px(img, 76, 80), px(img, 5, 5), px(img, 240, 150))
}

func main() {
	dc := render.NewContextWithScale(320, 200, 1)
	defer dc.Close()

	viewA, relA := dc.CreateOffscreenTexture(320, 200)
	defer relA()
	viewB, relB := dc.CreateOffscreenTexture(320, 200)
	defer relB()
	viewC, relC := dc.CreateOffscreenTexture(160, 100)
	defer relC()

	// 源纹理 C = 纯 qTR（160×100 小纹理）
	fill(dc, image.Rect(0, 0, 160, 100), qTR)
	dc.PresentFrame(viewC, 160, 100, nil)

	// A = v1
	fill(dc, image.Rect(0, 0, 320, 200), v1Base)
	dc.PresentFrame(viewA, 320, 200, nil)

	blit := func() {
		dc.DrawGPUTexture(viewC, 160, 100, 160, 100) // 整 viewC blit 到右下
	}
	blit()
	dc.PresentFrameDamage(viewA, 320, 200, rectL, nil)
	sample(dc.Image(), "A-dmg#1 ")
	blit()
	dc.PresentFrameDamage(viewA, 320, 200, rectL, nil)
	sample(dc.Image(), "A-dmg#2 ")
	fill(dc, image.Rect(0, 0, 320, 200), varBase)
	dc.PresentFrame(viewB, 320, 200, nil)
	sample(dc.Image(), "B-full  ")
	blit()
	dc.PresentFrameDamage(viewA, 320, 200, rectL, nil)
	sample(dc.Image(), "A-dmg#3 ")
	fmt.Println("判读: v1(76,87,140)=保留 黑=清屏 qTR(191,64,115)=blit落点 varBase(158,71,97)=B 内容泄漏")
}
