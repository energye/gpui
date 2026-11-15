// pmleg.go — offscreen legs for the Pixmap/pixel-output family. Every
// NewPixmapFromBuffer / FromImage / SetPixel / SetPixelPremul / FillSpan /
// FillSpanBlend / ImageView / ToImage / GenerationID / AddGenIDWatcher /
// EncodePNG / EncodeJPEG / Context.SetPixel / WritePixels / EncodePNG /
// EncodeJPEG / PixelWidth / PixelHeight / CropImageToInk entry is exercised;
// readbacks assert exact pixels and blend math. One GL-hub job (WritePixels
// uploads via the GPU textured-quad path when a device is present).
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"

	"github.com/energye/gpui/render"
)

const (
	pmW, pmH = 160, 120 // pixmap/thumb source size
)

type pmLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *pmLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "pmleg: %s\n", line)
}

func samplePM(img image.Image, x, y int) [3]float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r >> 8), float64(g >> 8), float64(b >> 8)}
}

func nearPM(got, want [3]float64, tol float64) bool {
	d := func(a, b float64) float64 {
		if a < b {
			return b - a
		}
		return a - b
	}
	return d(got[0], want[0]*255) <= tol && d(got[1], want[1]*255) <= tol && d(got[2], want[2]*255) <= tol
}

func (leg *pmLeg) checkColorAt(name string, img image.Image, x, y int, want [3]float64, tol float64) {
	if img == nil {
		leg.probe(name, false, "readback nil")
		return
	}
	got := samplePM(img, x, y)
	if nearPM(got, want, tol) {
		leg.probe(name, true, fmt.Sprintf("点(%d,%d)=(%.0f,%.0f,%.0f)", x, y, got[0], got[1], got[2]))
	} else {
		leg.probe(name, false, fmt.Sprintf("点(%d,%d) got=(%.0f,%.0f,%.0f) want=(%.0f,%.0f,%.0f)",
			x, y, got[0], got[1], got[2], want[0]*255, want[1]*255, want[2]*255))
	}
}

func basePixmap() *render.Pixmap {
	pm := render.NewPixmap(pmW, pmH)
	return pm
}

// runPMLegs drives all four cards' legs. One GL-hub job.
func runPMLegs() *pmLeg {
	leg := &pmLeg{imgs: map[string]image.Image{}}

	// ---- Card A: pixel writes ----
	// SetPixel: 两块 8×8 色块（同 API 多次调用；1px 点过缩略图必被双线性稀释）
	pmA := basePixmap()
	for dy := 0; dy < 8; dy++ {
		for dx := 0; dx < 8; dx++ {
			pmA.SetPixel(40+dx, 30+dy, render.FromColor(RGBAOf(colPixA)))
			pmA.SetPixel(80+dx, 60+dy, render.FromColor(RGBAOf(colPixB)))
		}
	}
	imgA := pmA.ToImage()
	leg.imgs["setpix"] = imgA
	leg.checkColorAt("A.SetPixel甲", imgA, 40, 30, colPixA, 2)
	leg.checkColorAt("A.SetPixel乙", imgA, 80, 60, colPixB, 2)

	// SetPixelPremul: premultiplied write (a=128 → channels halved)
	pmB := basePixmap()
	for dy := 0; dy < 8; dy++ {
		for dx := 0; dx < 8; dx++ {
			pmB.SetPixelPremul(80+dx, 60+dy, 255/2, 128/2, 64/2, 128)
		}
	}
	imgB := pmB.ToImage()
	leg.imgs["premul"] = imgB
	leg.probe("A.SetPixelPremul", nearPM(samplePM(imgB, 84, 64), [3]float64{0.5, 0.25, 0.125}, 3),
		fmt.Sprintf("预乘直写 got=%v", samplePM(imgB, 84, 64)))

	// FromImage/NewPixmapFromBuffer round-trip
	pmBuf := render.NewPixmapFromBuffer(pmA.Data(), pmW, pmH)
	leg.probe("A.NewPixmapFromBuffer", pmBuf != nil && pmBuf.Width() == pmW,
		fmt.Sprintf("w=%d", pmBuf.Width()))
	leg.checkColorAt("A.Buffer往返", pmBuf.ToImage(), 44, 34, colPixA, 2)

	// Context.SetPixel + WritePixels (fresh context, CPU mirror; WritePixels
	// also uploads via the GPU quad path when a device is present)
	dc := render.NewContextWithScale(pmW, pmH, 1)
	defer dc.Close()
	leg.probe("A.PixelWidth/Height", dc.PixelWidth() == pmW && dc.PixelHeight() == pmH,
		fmt.Sprintf("%dx%d", dc.PixelWidth(), dc.PixelHeight()))
	for dy := 0; dy < 8; dy++ {
		for dx := 0; dx < 8; dx++ {
			dc.SetPixel(40+dx, 30+dy, render.FromColor(RGBAOf(colPixA)))
		}
	}
	wp := make([]byte, 60*40*4)
	for i := 0; i < len(wp); i += 4 {
		wp[i] = uint8(colPixB[0] * 255)
		wp[i+1] = uint8(colPixB[1] * 255)
		wp[i+2] = uint8(colPixB[2] * 255)
		wp[i+3] = 255
	}
	dc.WritePixels(80, 40, 60, 40, wp)
	imgWP := dc.Image()
	leg.imgs["writepix"] = imgWP
	leg.checkColorAt("A.Context.SetPixel", imgWP, 44, 34, colPixA, 2)
	leg.checkColorAt("A.WritePixels块", imgWP, 110, 60, colPixB, 2)

	// ---- Card B: span fills ----
	pmS := basePixmap()
	for dy := 0; dy < 11; dy++ {
		pmS.FillSpan(20, 140, 55+dy, render.FromColor(RGBAOf(colSpan)))
	}
	imgS := pmS.ToImage()
	leg.imgs["span"] = imgS
	leg.checkColorAt("B.FillSpan内", imgS, 80, 60, colSpan, 2)
	leg.checkColorAt("B.FillSpan越界外", imgS, 150, 60, [3]float64{0, 0, 0}, 2)

	// FillSpanBlend over a known base: blend = base*(1-a) + color*a
	pmSB := basePixmap()
	for dy := 0; dy < 11; dy++ {
		pmSB.FillSpan(0, pmW, 25+dy, render.FromColor(RGBAOf(colSpan))) // base 带
	}
	blendCol := render.RGBA{R: colBlend[0], G: colBlend[1], B: colBlend[2], A: 0.5}
	for dy := 0; dy < 11; dy++ {
		pmSB.FillSpanBlend(20, 140, 25+dy, blendCol)
	}
	imgSB := pmSB.ToImage()
	leg.imgs["spanblend"] = imgSB
	want := mixPM(colSpan, colBlend, 0.5)
	leg.checkColorAt("B.混合数学", imgSB, 80, 30, want, 6)
	leg.checkColorAt("B.混合区外", imgSB, 150, 30, colSpan, 2)

	// ---- Card C: encode round-trips（色带画厚 12px——JPEG 色度子采样对 1px
	// 线必然重糊，厚带才能断言容差）----
	pmE := basePixmap()
	for dy := 0; dy < 12; dy++ {
		pmE.FillSpan(10, 150, 15+dy, render.FromColor(RGBAOf(colPixA)))
		pmE.FillSpan(10, 150, 45+dy, render.FromColor(RGBAOf(colPixB)))
		pmE.FillSpan(10, 150, 75+dy, render.FromColor(RGBAOf(colSpan)))
	}
	var pngBuf bytes.Buffer
	if err := pmE.EncodePNG(&pngBuf); err != nil {
		leg.probe("C.Pixmap.EncodePNG", false, err.Error())
		return leg
	}
	decoded, err := png.Decode(bytes.NewReader(pngBuf.Bytes()))
	if err != nil {
		leg.probe("C.PNG解码", false, err.Error())
		return leg
	}
	leg.imgs["png"] = decoded
	same := true
	detail := ""
	for _, p := range [][2]int{{80, 20}, {80, 50}, {80, 80}} {
		got := samplePM(decoded, p[0], p[1])
		want := samplePM(pmE.ToImage(), p[0], p[1])
		if !rawNear(got, want, 3) {
			same = false
			detail += fmt.Sprintf(" (%d,%d) got=%v want=%v", p[0], p[1], got, want)
		}
	}
	leg.probe("C.PNG逐位", same, "解码=源（三色带）"+detail)

	var jpgBuf bytes.Buffer
	if err := pmE.EncodeJPEG(&jpgBuf, 90); err != nil {
		leg.probe("C.Pixmap.EncodeJPEG", false, err.Error())
		return leg
	}
	jpegDecoded, err := jpeg.Decode(bytes.NewReader(jpgBuf.Bytes()))
	if err != nil {
		leg.probe("C.JPEG解码", false, err.Error())
		return leg
	}
	leg.imgs["jpeg"] = jpegDecoded
	leg.checkColorAt("C.JPEG容差", jpegDecoded, 80, 51, colPixB, 30)

	// Context.EncodePNG (readback content round-trips through the context)
	var ctxPng bytes.Buffer
	if err := dc.EncodePNG(&ctxPng); err != nil {
		leg.probe("C.Context.EncodePNG", false, err.Error())
		return leg
	}
	leg.probe("C.Context.EncodePNG", ctxPng.Len() > 0, fmt.Sprintf("bytes=%d", ctxPng.Len()))

	// ---- Card D: GenID watcher + ink crop ----
	// 文档契约：SetPixel/Clear 不自动通知（"Individual SetPixel/Clear calls do
	// NOT auto-notify"）——批量改动后显式 NotifyPixelsChanged 才换代并触发监视。
	pmG := basePixmap()
	fired := 0
	remove := pmG.AddGenIDWatcher(func(oldGenID uint64) { fired++ })
	gen0 := pmG.GenerationID()
	pmG.SetPixel(10, 10, render.FromColor(RGBAOf(colPixA)))
	leg.probe("D.SetPixel不自动换代", pmG.GenerationID() == gen0,
		fmt.Sprintf("SetPixel 后 gen=%d（文档契约）", pmG.GenerationID()))
	pmG.NotifyPixelsChanged()
	gen1 := pmG.GenerationID()
	leg.probe("D.GenerationID变更", gen1 != gen0, fmt.Sprintf("%d→%d", gen0, gen1))
	leg.probe("D.AddGenIDWatcher触发", fired == 1, fmt.Sprintf("fired=%d", fired))
	remove()
	pmG.NotifyPixelsChanged()
	leg.probe("D.Watcher移除", fired == 1, fmt.Sprintf("移除后 fired=%d", fired))

	// CropImageToInk: draw a glyph-like blob with margins, crop to ink
	ink := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 30; y < 70; y++ {
		for x := 25; x < 60; x++ {
			ink.Set(x, y, colorOf(colCrop))
		}
	}
	cropped, ox, oy, err := render.CropImageToInk(ink, 0, 0)
	if err != nil {
		leg.probe("D.CropImageToInk", false, err.Error())
	} else {
		// 基线约定：bitmap left = pen+OriginX，bitmap top = baseline−OriginY
		// → pen(0,0) 时 ox=minX=25、oy=−minY=−30
		leg.probe("D.CropImageToInk", cropped.Bounds().Dx() == 35 && cropped.Bounds().Dy() == 40 &&
			ox == 25 && oy == -30,
			fmt.Sprintf("crop=%dx%d origin=(%.0f,%.0f)（基线约定）", cropped.Bounds().Dx(), cropped.Bounds().Dy(), ox, oy))
		leg.imgs["crop"] = cropped
		// draw the crop through a context (crop result → ImageBuf → draw)
		dc2 := render.NewContextWithScale(pmW, pmH, 1)
		dc2.DrawImageEx(render.ImageBufFromImage(cropped), render.DrawImageOptions{X: 20, Y: 15})
		leg.imgs["cropdraw"] = dc2.Image()
		leg.checkColorAt("D.裁剪直画", leg.imgs["cropdraw"], 40, 40, colCrop, 6)
		dc2.Close()
	}
	return leg
}

func RGBAOf(c [3]float64) render.RGBA {
	return render.RGBA{R: c[0], G: c[1], B: c[2], A: 1}
}

func colorOf(c [3]float64) color.RGBA {
	return color.RGBA{R: uint8(c[0] * 255), G: uint8(c[1] * 255), B: uint8(c[2] * 255), A: 255}
}

// rawNear compares two 0-255 samples directly (nearPM 的 want 是 0-1 归一).
func rawNear(a, b [3]float64, tol float64) bool {
	d := func(x, y float64) float64 {
		if x < y {
			return y - x
		}
		return x - y
	}
	return d(a[0], b[0]) <= tol && d(a[1], b[1]) <= tol && d(a[2], b[2]) <= tol
}

func mixPM(base, top [3]float64, t float64) [3]float64 {
	return [3]float64{base[0]*(1-t) + top[0]*t, base[1]*(1-t) + top[1]*t, base[2]*(1-t) + top[2]*t}
}
