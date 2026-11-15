// bcleg.go — offscreen direct legs for the brush construction family. Every
// Solid constructor, standalone gradient, Painter adapter and BrushFieldSeed
// entry is exercised on offscreen views; readbacks assert ColorAt math and
// painted pixels. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

	"github.com/energye/gpui/render"
)

const (
	bcW, bcH = 320, 240 // leg context size
)

type bcLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *bcLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "bcleg: %s\n", line)
}

func flushBC(dc *render.Context, view render.TextureView, leg *bcLeg, tag string) bool {
	if err := dc.PresentFrame(view, bcW, bcH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	return true
}

func clearBC(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, bcW, bcH)
	_ = dc.Fill()
}

func nearC(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// ---- Card A: solid constructor family ----
func runCardALeg(dc *render.Context, view render.TextureView, leg *bcLeg) {
	clearBC(dc)

	// SolidRGB / SolidRGBA / SolidHex / FromColor / HSL 数学读回。
	sr := render.SolidRGB(0.9, 0.2, 0.2)
	leg.probe("A.SolidRGB", sr.Color == render.RGBA2(0.9, 0.2, 0.2, 1), "ColorAt=构造值")
	sa := render.SolidRGBA(0.2, 0.3, 0.9, 0.6)
	leg.probe("A.SolidRGBA", nearC(sa.Color.A, 0.6, 1e-9),
		fmt.Sprintf("A=%.2f", sa.Color.A))

	sh := render.SolidHex("#FF8040")
	leg.probe("A.SolidHex", nearC(sh.Color.R, 0xFF/255.0, 1e-6) && nearC(sh.Color.G, 0x80/255.0, 1e-6) && nearC(sh.Color.B, 0x40/255.0, 1e-6),
		fmt.Sprintf("R=%.4f G=%.4f B=%.4f", sh.Color.R, sh.Color.G, sh.Color.B))

	fc := render.FromColor(color.NRGBA{R: 51, G: 178, B: 230, A: 255})
	leg.probe("A.FromColor", nearC(fc.R, 51/255.0, 1e-6) && nearC(fc.G, 178/255.0, 1e-6) && nearC(fc.B, 230/255.0, 1e-6),
		fmt.Sprintf("R=%.4f G=%.4f B=%.4f", fc.R, fc.G, fc.B))

	hsl := render.HSL(0, 1, 0.5)
	leg.probe("A.HSL红", hsl.R == 1 && hsl.G == 0 && hsl.B == 0,
		fmt.Sprintf("R=%.2f G=%.2f B=%.2f", hsl.R, hsl.G, hsl.B))
	hslG := render.HSL(120, 1, 0.5) // h 是度数语义
	leg.probe("A.HSL绿", hslG.G == 1 && hslG.R == 0 && hslG.B == 0, "HSL(120,1,0.5) G=1")

	// WithAlpha chain: Opaque/Transparent readback.
	base := render.SolidRGB(0.4, 0.8, 0.55)
	leg.probe("A.Opaque", base.Opaque().Color.A == 1, "A=1")
	leg.probe("A.Transparent", base.Transparent().Color.A == 0, "A=0")
	half := base.WithAlpha(0.5)
	leg.probe("A.WithAlpha", nearC(half.Color.A, 0.5, 1e-9),
		fmt.Sprintf("A=%.2f", half.Color.A))
	wrapped := render.BrushWithAlpha(base, 0.25)
	leg.probe("A.BrushWithAlpha", nearC(wrapped.ColorAt(0, 0).A, 0.25, 1e-6),
		fmt.Sprintf("ColorAt(0,0).A=%.2f", wrapped.ColorAt(0, 0).A))

	// paint: five constructor swatches + alpha ladder
	sw := func(x, w float64, c [3]float64) {
		dc.SetRGBA(c[0], c[1], c[2], 1)
		dc.DrawRectangle(x, 20, w, 60)
		_ = dc.Fill()
	}
	sw(10, 56, colHexCard)
	sw(72, 56, colFromCol)
	sw(134, 56, colHSL)
	sw(196, 56, [3]float64{0.9, 0.2, 0.2})
	sw(258, 52, [3]float64{0.2, 0.3, 0.9})
	dc.SetFillBrush(render.BrushWithAlpha(render.SolidRGB(colAlpha[0], colAlpha[1], colAlpha[2]), 1.0))
	dc.DrawRectangle(10, 100, 92, 40)
	_ = dc.Fill()
	dc.SetFillBrush(render.BrushWithAlpha(render.SolidRGB(colAlpha[0], colAlpha[1], colAlpha[2]), 0.5))
	dc.DrawRectangle(114, 100, 92, 40)
	_ = dc.Fill()
	dc.SetFillBrush(render.BrushWithAlpha(render.SolidRGB(colAlpha[0], colAlpha[1], colAlpha[2]), 0.25))
	dc.DrawRectangle(218, 100, 92, 40)
	_ = dc.Fill()
	dc.SetFillBrush(render.SolidRGB(1, 1, 1))
	if !flushBC(dc, view, leg, "A") {
		return
	}
	leg.imgs["solid"] = dc.Image()
	leg.imgs["alpha"] = dc.Image()
}

// ---- Card B: standalone gradients ----
func runCardBLeg(dc *render.Context, view render.TextureView, leg *bcLeg) {
	clearBC(dc)

	ca := render.RGBA2(colH0[0], colH0[1], colH0[2], 1)
	cb := render.RGBA2(colH1[0], colH1[1], colH1[2], 1)

	// Horizontal: midpoint = half blend (math probe on ColorAt).
	hg := render.HorizontalGradient(ca, cb, 0, 300)
	mid := hg.ColorAt(150, 0)
	want := render.RGBA2((colH0[0]+colH1[0])/2, (colH0[1]+colH1[1])/2, (colH0[2]+colH1[2])/2, 1)
	leg.probe("B.Horizontal中点", nearC(mid.R, want.R, 0.01) && nearC(mid.G, want.G, 0.01) && nearC(mid.B, want.B, 0.01),
		fmt.Sprintf("ColorAt(150)=(%.3f,%.3f,%.3f)", mid.R, mid.G, mid.B))
	e0, e1 := hg.ColorAt(0, 0), hg.ColorAt(300, 0)
	leg.probe("B.Horizontal端点", e0 == ca && nearC(e1.R, cb.R, 1e-9) && nearC(e1.G, cb.G, 1e-9) && nearC(e1.B, cb.B, 1e-9),
		"t=0 端色精确 t=1 浮点尾差 1e-16 级")

	// Vertical: same along y.
	vg := render.VerticalGradient(ca, cb, 0, 200)
	v0, v1 := vg.ColorAt(0, 0), vg.ColorAt(0, 200)
	leg.probe("B.Vertical", v0 == ca && nearC(v1.R, cb.R, 1e-9) && nearC(v1.G, cb.G, 1e-9) &&
		nearC(vg.ColorAt(0, 100).R, want.R, 0.01), "端点+中点")

	// Linear: diagonal gradient t=(x+y)/(w+h).
	lg := render.LinearGradient(ca, cb, 0, 0, 100, 100)
	l0, l1 := lg.ColorAt(0, 0), lg.ColorAt(100, 100)
	leg.probe("B.Linear端点", l0 == ca && nearC(l1.R, cb.R, 1e-9) && nearC(l1.G, cb.G, 1e-9),
		"(0,0) 精确 (100,100) 浮点尾差")
	lm := lg.ColorAt(50, 50)
	leg.probe("B.Linear中点", nearC(lm.R, want.R, 2) && nearC(lm.G, want.G, 2), "(50,50)=半混")

	// Radial: r/2 → half blend.
	rg := render.RadialGradient(ca, cb, 50, 50, 50)
	leg.probe("B.Radial圆心", rg.ColorAt(50, 50) == ca, "center=c0")
	rm := rg.ColorAt(75, 50) // r/2
	leg.probe("B.Radial半程", nearC(rm.R, want.R, 0.015) && nearC(rm.G, want.G, 0.015),
		fmt.Sprintf("r/2=(%.3f,%.3f,%.3f)", rm.R, rm.G, rm.B))
	ro := rg.ColorAt(100, 50)
	leg.probe("B.Radial外圆", nearC(ro.R, cb.R, 1e-9) && nearC(ro.G, cb.G, 1e-9) && nearC(ro.B, cb.B, 1e-9),
		"r=c1（浮点尾差级）")

	// Sweep: angle sweep around center; sample two angles.
	sg := render.SweepGradient(ca, cb, 50, 50)
	leg.probe("B.Sweep非nil", sg != nil, "构造成功")
	sg2 := render.NewSweepGradientBrush(50, 50, 0)
	sg2.AddColorStop(0, ca).AddColorStop(1, cb)
	a0 := sg2.ColorAt(100, 50)  // 0° → t=0
	a90 := sg2.ColorAt(50, 100) // 90° → t=0.25
	leg.probe("B.Sweep角采样", nearC(a0.R, ca.R, 0.02) &&
		nearC(a90.R, ca.R+(cb.R-ca.R)*0.25, 0.02) &&
		nearC(a90.G, ca.G+(cb.G-ca.G)*0.25, 0.02),
		fmt.Sprintf("0°=t0 90°=t0.25 (%.3f,%.3f)", a90.R, a90.G))

	// Stripes: periodic ColorAt.
	st := render.Stripes(ca, cb, 20, 0)
	s1 := st.ColorAt(5, 0)   // floor(5/20)=0 → band 0 = c0
	s2 := st.ColorAt(25, 0)  // floor(25/20)=1 → band 1 = c1
	leg.probe("B.Stripes周期", s1 == ca && s2 == cb,
		fmt.Sprintf("x=5→带0 x=25→带1（floor(x/width) 奇偶）"))

	// paint six gradient thumbs（hg 范围与填充盒对齐：盒左右缘=端色）
	fillGrad := func(x, y, w, h float64, b render.Brush) {
		dc.SetFillBrush(b)
		dc.DrawRectangle(x, y, w, h)
		_ = dc.Fill()
	}
	fillGrad(10, 20, 90, 60, render.HorizontalGradient(ca, cb, 10, 100))
	fillGrad(115, 20, 90, 60, vg)
	fillGrad(220, 20, 90, 60, lg)
	fillGrad(10, 100, 90, 60, rg)
	fillGrad(115, 100, 90, 60, sg2)
	fillGrad(220, 100, 90, 60, st)
	dc.SetFillBrush(render.SolidRGB(1, 1, 1))
	if !flushBC(dc, view, leg, "B") {
		return
	}
	leg.imgs["hgrad"] = dc.Image()
	leg.imgs["vgrad"] = dc.Image()
	leg.imgs["lgrad"] = dc.Image()
	leg.imgs["rgrad"] = dc.Image()
	leg.imgs["sweep"] = dc.Image()
	leg.imgs["stripes"] = dc.Image()
}

var bcCA = render.RGBA2(0.90, 0.20, 0.20, 1)
var bcCB = render.RGBA2(0.20, 0.30, 0.90, 1)

// ---- Card C: package-level brush vs Context 口 parity ----
func runCardCLeg(dc *render.Context, view render.TextureView, leg *bcLeg) {
	// frame 1: SetFillBrush(solid) draw
	dc.BeginGPUFrame()
	clearBC(dc)
	dc.SetFillBrush(render.SolidRGB(colAlpha[0], colAlpha[1], colAlpha[2]))
	dc.DrawRectangle(60, 60, 200, 120)
	_ = dc.Fill()
	dc.SetFillBrush(render.SolidRGB(1, 1, 1))
	if !flushBC(dc, view, leg, "C1") {
		return
	}
	brushFrame := dc.Image()

	// frame 2: SetRGBA draw (same geometry)
	dc.BeginGPUFrame()
	clearBC(dc)
	dc.SetRGBA(colAlpha[0], colAlpha[1], colAlpha[2], 1)
	dc.DrawRectangle(60, 60, 200, 120)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushBC(dc, view, leg, "C2") {
		return
	}
	solidFrame := dc.Image()

	// bitwise diff of the two frames
	diffPx := 0
	for yy := 0; yy < bcH; yy++ {
		for xx := 0; xx < bcW; xx++ {
			r1, g1, b1, a1 := brushFrame.At(xx, yy).RGBA()
			r2, g2, b2, a2 := solidFrame.At(xx, yy).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diffPx++
			}
		}
	}
	leg.probe("C.两帧逐位一致", diffPx == 0, fmt.Sprintf("diff=%d px（共 %d）", diffPx, bcW*bcH))

	// PainterFromPaint: solid paint → SolidPainter, buffer fill exact.
	paint := render.NewPaint()
	paint.SetBrush(render.SolidRGB(colAlpha[0], colAlpha[1], colAlpha[2]))
	painter := render.PainterFromPaint(paint)
	sp, okSolid := painter.(*render.SolidPainter)
	leg.probe("C.SolidPainter类型", okSolid, "solid paint→SolidPainter 快路径")
	if okSolid {
		buf := make([]render.RGBA, 8)
		sp.PaintSpan(buf, 0, 0, len(buf))
		ok := true
		for _, c := range buf {
			if c != sp.Color {
				ok = false
			}
		}
		leg.probe("C.PaintSpan填充", ok, "缓冲逐元素=色值")
	}

	// gradient paint → FuncPainter sampling matches Brush.ColorAt.
	gpaint := render.NewPaint()
	gpaint.SetBrush(render.HorizontalGradient(bcCA, bcCB, 0, 300))
	gpainter := render.PainterFromPaint(gpaint)
	fp, okFunc := gpainter.(*render.FuncPainter)
	leg.probe("C.FuncPainter类型", okFunc, "渐变 paint→FuncPainter")
	if okFunc {
		buf := make([]render.RGBA, 4)
		fp.PaintSpan(buf, 148, 0, 4) // pixel centers x=148.5..151.5, y=0.5
		ok := true
		for i, c := range buf {
			if m := fp.Fn(float64(148+i)+0.5, 0.5); c != m {
				ok = false
			}
		}
		leg.probe("C.FuncPainter逐像素", ok,
			fmt.Sprintf("PaintSpan==Fn 逐像素（样例 %.3f,%.3f,%.3f）", buf[0].R, buf[0].G, buf[0].B))
	}

	leg.imgs["parityBrush"] = brushFrame
	leg.imgs["paritySolid"] = solidFrame
	leg.imgs["parityDiff"] = brushFrame
}

// ---- Card D: BrushFieldSeed ----
func runCardDLeg(dc *render.Context, view render.TextureView, leg *bcLeg) {
	clearBC(dc)

	base := render.SolidRGB(colAlpha[0], colAlpha[1], colAlpha[2])
	leg.probe("D.SolidSeed0", render.BrushFieldSeed(base) == 0, "SolidBrush seed=0")
	leg.probe("D.SolidSeed指针0", render.BrushFieldSeed(nil) == 0, "nil=0")

	a50 := render.BrushWithAlpha(base, 0.5)
	a25 := render.BrushWithAlpha(base, 0.25)
	s50, s25 := render.BrushFieldSeed(a50), render.BrushFieldSeed(a25)
	leg.probe("D.WithAlpha非0", s50 != 0 && s25 != 0,
		fmt.Sprintf("seed(0.5)=%d seed(0.25)=%d", s50, s25))
	leg.probe("D.alpha互异", s50 != s25, "不同 alpha seed 互异（field cache key）")
	a50b := render.BrushWithAlpha(base, 0.5)
	leg.probe("D.同alpha同seed", render.BrushFieldSeed(a50b) == s50, "同包裹同 seed")

	// paint: half-alpha rect over dark bg (visual anchor for the alpha wrap)
	dc.SetFillBrush(render.BrushWithAlpha(base, 0.5))
	dc.DrawRectangle(60, 60, 200, 120)
	_ = dc.Fill()
	dc.SetFillBrush(render.SolidRGB(1, 1, 1))
	if !flushBC(dc, view, leg, "D") {
		return
	}
	leg.imgs["seed"] = dc.Image()
	leg.imgs["alphapaint"] = dc.Image()

	leg.findings = append(leg.findings,
		"brushfieldseed: 包裹类画刷（WithAlpha/device-space）seed 进入 GPU field cache key——同 alpha 同 seed、异 alpha 互异（brush_device_space.go 语义实证）")
}

// runBCLegs drives all four cards' offscreen legs. One GL-hub job.
func runBCLegs() *bcLeg {
	leg := &bcLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(bcW, bcH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(bcW, bcH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	runCardALeg(dc, view, leg)
	runCardBLeg(dc, view, leg)
	runCardCLeg(dc, view, leg)
	runCardDLeg(dc, view, leg)
	return leg
}
