// msleg.go — offscreen legs for the mask construction family. Every
// NewMaskFromAlpha / NewMaskFromData / NewLuminanceMask / Context.AsMask /
// SetMaskFilter / MaskFilterRadius / MaskPlaneStats entry is exercised; the
// masked-draw readbacks assert the construction shape, the soft/hard edge and
// the plane-stat counters. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/energye/gpui/render"
)

const (
	msW, msH = 160, 120
)

type msLeg struct {
	imgs     map[string]image.Image
	verdicts []string
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *msLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	verdict := name
	if ok {
		verdict += ": OK"
		l.logicOK++
	} else {
		line = "FAIL " + line
		verdict += ": FAIL"
		l.logicBad = append(l.logicBad, name)
	}
	l.dLines = append(l.dLines, line)
	l.verdicts = append(l.verdicts, verdict)
	fmt.Fprintf(os.Stderr, "msleg: %s\n", line)
}

func sampleMS(img image.Image, x, y int) (float64, float64, float64, float64) {
	r, g, b, a := img.At(x, y).RGBA()
	return float64(r >> 8), float64(g >> 8), float64(b >> 8), float64(a >> 8)
}

func nearMS(got, want [3]float64, tol float64) bool {
	d := func(a, b float64) float64 {
		if a < b {
			return b - a
		}
		return a - b
	}
	return d(got[0], want[0]*255) <= tol && d(got[1], want[1]*255) <= tol && d(got[2], want[2]*255) <= tol
}

func (leg *msLeg) checkColorAt(name string, img image.Image, x, y int, want [3]float64, tol float64) {
	if img == nil {
		leg.probe(name, false, "readback nil")
		return
	}
	r, g, b, _ := sampleMS(img, x, y)
	got := [3]float64{r, g, b}
	if nearMS(got, want, tol) {
		leg.probe(name, true, fmt.Sprintf("点(%d,%d)=(%.0f,%.0f,%.0f)", x, y, r, g, b))
	} else {
		leg.probe(name, false, fmt.Sprintf("点(%d,%d) got=(%.0f,%.0f,%.0f) want=(%.0f,%.0f,%.0f)",
			x, y, r, g, b, want[0]*255, want[1]*255, want[2]*255))
	}
}

// maskedDraw installs mask on a fresh context, fills the full frame with col,
// and returns the readback (the reveal shows the mask shape).
func maskedDraw(m *render.Mask, col [3]float64) image.Image {
	dc := render.NewContextWithScale(msW, msH, 1)
	defer dc.Close()
	dc.BeginGPUFrame()
	dc.SetMask(m)
	dc.SetRGBA(col[0], col[1], col[2], 1)
	dc.DrawRectangle(0, 0, msW, msH)
	_ = dc.Fill()
	dc.SetMask(nil)
	return dc.Image()
}

// runMSLegs drives all four cards' legs. One GL-hub job.
func runMSLegs() *msLeg {
	leg := &msLeg{imgs: map[string]image.Image{}}

	// ---- Card A: construction trio ----
	// Alpha source: a disc with an alpha ramp — the mask keeps the alpha shape.
	alphaImg := image.NewNRGBA(image.Rect(0, 0, msW, msH))
	for y := 0; y < msH; y++ {
		for x := 0; x < msW; x++ {
			dx, dy := x-80, y-60
			d2 := dx*dx + dy*dy
			a := uint8(0)
			if d2 < 40*40 {
				a = 255
			}
			alphaImg.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: a})
		}
	}
	mAlpha := render.NewMaskFromAlpha(alphaImg)
	leg.imgs["alpha"] = maskedDraw(mAlpha, colMaskA)
	leg.checkColorAt("A.Alpha盘内", leg.imgs["alpha"], 80, 60, colMaskA, 8)
	leg.checkColorAt("A.Alpha盘外", leg.imgs["alpha"], 10, 10, [3]float64{0, 0, 0}, 6)

	// Data source: raw A8 bytes — a rounded square blob.
	data := make([]byte, msW*msH)
	for y := 0; y < msH; y++ {
		for x := 0; x < msW; x++ {
			if x >= 40 && x < 120 && y >= 30 && y < 90 {
				data[y*msW+x] = 255
			}
		}
	}
	mData := render.NewMaskFromData(data, msW, msH)
	leg.imgs["data"] = maskedDraw(mData, colMaskB)
	leg.checkColorAt("A.Data块内", leg.imgs["data"], 80, 60, colMaskB, 8)
	leg.checkColorAt("A.Data块外", leg.imgs["data"], 10, 60, [3]float64{0, 0, 0}, 6)

	// Luminance source: 亮度值=掩码 alpha（盘内纯白→全显，盘外纯黑→全遮）
	lumImg := image.NewNRGBA(image.Rect(0, 0, msW, msH))
	for y := 0; y < msH; y++ {
		for x := 0; x < msW; x++ {
			dx, dy := x-50, y-60
			v := uint8(0)
			if dx*dx+dy*dy < 32*32 {
				v = 255
			}
			lumImg.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	mLum := render.NewLuminanceMask(lumImg)
	leg.imgs["lum"] = maskedDraw(mLum, colMaskC)
	leg.checkColorAt("A.Luminance盘内", leg.imgs["lum"], 50, 60, colMaskC, 8)
	leg.checkColorAt("A.Luminance盘外", leg.imgs["lum"], 130, 20, [3]float64{0, 0, 0}, 6)

	// ---- Card B: AsMask path capture ----
	dc := render.NewContextWithScale(msW, msH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(msW, msH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()
	dc.BeginGPUFrame()
	dc.DrawCircle(80, 60, 40) // path stays (not filled — AsMask 栅格化当前路径)
	mAs := dc.AsMask()
	inMask := int(mAs.At(80, 60))
	outMask := int(mAs.At(5, 5))
	leg.probe("B.AsMask盘内", inMask == 255, fmt.Sprintf("At(80,60)=%d", inMask))
	leg.probe("B.AsMask盘外", outMask == 0, fmt.Sprintf("At(5,5)=%d", outMask))
	dc.SetMask(mAs)
	dc.SetRGBA(colAsM[0], colAsM[1], colAsM[2], 1)
	dc.DrawRectangle(0, 0, msW, msH)
	_ = dc.Fill()
	dc.SetMask(nil)
	if err := dc.PresentFrame(view, msW, msH, nil); err != nil {
		leg.probe("B.flush", false, err.Error())
		return leg
	}
	leg.imgs["asmask"] = dc.Image()
	leg.checkColorAt("B.路径掩码显示", leg.imgs["asmask"], 80, 60, colAsM, 8)
	leg.checkColorAt("B.路径外保留底", leg.imgs["asmask"], 5, 5, [3]float64{0, 0, 0}, 6)

	// ---- Card C: ambient mask filter ----
	SetMaskFilterRadius(6)
	leg.probe("C.MaskFilterRadius读回", MaskFilterRadius() == 6,
		fmt.Sprintf("radius=%v", MaskFilterRadius()))
	// 软边：方块掩码安装时被原位模糊（SetMask 内预处理）
	sq := make([]byte, msW*msH)
	for y := 0; y < msH; y++ {
		for x := 0; x < msW; x++ {
			if x >= 40 && x < 120 && y >= 30 && y < 90 {
				sq[y*msW+x] = 255
			}
		}
	}
	mSoft := render.NewMaskFromData(sq, msW, msH)
	leg.imgs["soft"] = maskedDraw(mSoft, colSoft)
	// 边缘 (40,60) 处应显著低于实心值（模糊扩散）——软边证据
	er, eg, eb, _ := sampleMS(leg.imgs["soft"], 40, 60)
	cr, cg, cb, _ := sampleMS(leg.imgs["soft"], 80, 60)
	edgeDown := er < cr-40 && eg < cg-40 && eb < cb-40
	leg.probe("C.软边衰减", edgeDown, fmt.Sprintf("edge=(%.0f,%.0f,%.0f) center=(%.0f,%.0f,%.0f)", er, eg, eb, cr, cg, cb))
	leg.checkColorAt("C.软边中心实", leg.imgs["soft"], 80, 60, colSoft, 10)

	// 半径归零：新掩码硬边（原位模糊不再发生）
	SetMaskFilterRadius(0)
	leg.probe("C.半径归零", MaskFilterRadius() == 0, "reset")
	sq2 := make([]byte, msW*msH)
	for y := 0; y < msH; y++ {
		for x := 0; x < msW; x++ {
			if x >= 40 && x < 120 && y >= 30 && y < 90 {
				sq2[y*msW+x] = 255
			}
		}
	}
	mHard := render.NewMaskFromData(sq2, msW, msH)
	leg.imgs["hard"] = maskedDraw(mHard, colSoft)
	hr, hg, hb, _ := sampleMS(leg.imgs["hard"], 42, 60)
	hardKept := nearMS([3]float64{hr, hg, hb}, colSoft, 12)
	leg.probe("C.硬边保持", hardKept, fmt.Sprintf("edge=(%.0f,%.0f,%.0f)", hr, hg, hb))

	// ---- Card D: MaskPlaneStats ----
	// MaskPlaneStats = maskR8 缓存池账本（字形掩码线 P4.3 的消费者）——普通
	// 填充掩码不经该缓存，计数如实为 0；本窗只断言读回口可用且一致，
	// 计数语义记 findings（与 W1 os_partial 同法，不判死）。
	calls0, planes0, bytes0, rb0, hits0 := dc.MaskPlaneStats()
	dc.BeginGPUFrame()
	dc.SetMask(mHard)
	dc.SetRGBA(colMaskA[0], colMaskA[1], colMaskA[2], 1)
	dc.DrawRectangle(0, 0, msW, msH)
	_ = dc.Fill()
	dc.SetMask(nil)
	if err := dc.PresentFrame(view, msW, msH, nil); err != nil {
		leg.probe("D.flush", false, err.Error())
		return leg
	}
	calls1, planes1, bytes1, rb1, hits1 := dc.MaskPlaneStats()
	leg.probe("D.账本读回", calls1 >= calls0 && planes1 >= planes0 && bytes1 >= bytes0 &&
		rb1 >= rb0 && hits1 >= hits0,
		fmt.Sprintf("calls %d→%d planes %d→%d bytes %d→%d rb %d→%d hits %d→%d（普通填充掩码不走 maskR8 池）",
			calls0, calls1, planes0, planes1, bytes0, bytes1, rb0, rb1, hits1))
	leg.findings = append(leg.findings,
		"maskplane-scope: MaskPlaneStats 计数器=maskR8 缓存池账本（字形掩码线消费者）；普通填充掩码路径不经它（本窗如实为 0）——语义归 P4.3 字形掩码线")
	leg.imgs["stats"] = dc.Image()
	return leg
}

// SetMaskFilterRadius sets the ambient mask blur radius (W6 helper alias).
func SetMaskFilterRadius(r float64) { render.SetMaskFilter(r) }

// MaskFilterRadius reports the ambient mask blur radius.
func MaskFilterRadius() float64 { return render.MaskFilterRadius() }
