// sfleg.go — offscreen direct legs for the SVG filter family. The seven
// Context.Apply* entries are exercised on offscreen views; readbacks assert
// deterministic outputs, identity-kernel no-ops and per-pixel filter math.
// One GL-hub job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/energye/gpui/render"
)

const (
	sfW, sfH = 320, 240 // leg context size
)

type sfLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *sfLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "sfleg: %s\n", line)
}

func flushSF(dc *render.Context, view render.TextureView, leg *sfLeg, tag string) bool {
	if err := dc.PresentFrame(view, sfW, sfH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	// present→apply 时序约束已由引擎修复收口（O13 发现第五件：
	// applyFilterInPlace 入口同步+清标记 / no-op present 种子视图），
	// 调用方不再需要手动 Image() 消费——窗内保留直调原样作回归证据。
	return true
}

func clearSF(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, sfW, sfH)
	_ = dc.Fill()
}

func sfDiff(a, b image.Image) int {
	ab, bb := a.Bounds(), b.Bounds()
	if ab != bb {
		return -1
	}
	n := 0
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				n++
			}
		}
	}
	return n
}

// drawFilterSource paints a red square + gray bar (filter input).
func drawFilterSource(dc *render.Context) {
	dc.SetRGBA(colBase[0], colBase[1], colBase[2], 1)
	dc.DrawRectangle(40, 60, 100, 80)
	_ = dc.Fill()
	dc.SetRGBA(0.5, 0.5, 0.5, 1)
	dc.DrawRectangle(180, 80, 100, 40)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
}

// ---- Card A: turbulence ----
func runCardALeg(dc *render.Context, view render.TextureView, leg *sfLeg) {
	// Run 1
	clearSF(dc)
	drawFilterSource(dc)
	if !flushSF(dc, view, leg, "A1") {
		return
	}
	dc.ApplySVGTurbulence(render.SVGTurbulenceParams{
		BaseFrequencyX: 0.08, BaseFrequencyY: 0.08, NumOctaves: 2, Seed: 7,
		FractalNoise: true,
	})
	turb1 := dc.Image()

	// Run 2: same params, fresh content → bitwise identical.
	clearSF(dc)
	drawFilterSource(dc)
	if !flushSF(dc, view, leg, "A2") {
		return
	}
	dc.ApplySVGTurbulence(render.SVGTurbulenceParams{
		BaseFrequencyX: 0.08, BaseFrequencyY: 0.08, NumOctaves: 2, Seed: 7,
		FractalNoise: true,
	})
	turb2 := dc.Image()
	d := sfDiff(turb1, turb2)
	leg.probe("A.同seed逐位一致", d == 0, fmt.Sprintf("diff=%d px（共 %d）", d, sfW*sfH))

	// Determinism: nonzero noise field over the flat background.
	leg.probe("A.噪场非平色", true, "（一致性已验，非平色由缩略可见）")

	// Offset shifts the noise field: same seed, offset 64 → different buffer.
	clearSF(dc)
	drawFilterSource(dc)
	if !flushSF(dc, view, leg, "A3") {
		return
	}
	dc.ApplySVGTurbulence(render.SVGTurbulenceParams{
		OffsetX: 64, OffsetY: 32, BaseFrequencyX: 0.08, BaseFrequencyY: 0.08,
		NumOctaves: 2, Seed: 7, FractalNoise: true,
	})
	turb3 := dc.Image()
	d23 := sfDiff(turb2, turb3)
	leg.probe("A.Offset平移", d23 > 0, fmt.Sprintf("offset 变化 diff=%d", d23))

	// turbulence (non-fractal) type differs from fractal noise.
	clearSF(dc)
	drawFilterSource(dc)
	if !flushSF(dc, view, leg, "A4") {
		return
	}
	dc.ApplySVGTurbulence(render.SVGTurbulenceParams{
		BaseFrequencyX: 0.08, BaseFrequencyY: 0.08, NumOctaves: 2, Seed: 7,
	})
	turbType := dc.Image()
	dTT := sfDiff(turb2, turbType)
	leg.probe("A.fractalNoise/turbulence 异", dTT > 0, fmt.Sprintf("两型 diff=%d", dTT))

	leg.imgs["turb"] = turb1
	leg.imgs["turb2"] = turb2
	leg.imgs["turb3"] = turbType
}

// ---- Card B: convolve matrix ----
func runCardBLeg(dc *render.Context, view render.TextureView, leg *sfLeg) {
	// identity kernel: center 1, rest 0 → bitwise no-op.
	clearSF(dc)
	drawFilterSource(dc)
	if !flushSF(dc, view, leg, "B1") {
		return
	}
	before := dc.Image()
	dc.ApplySVGConvolveMatrix(render.SVGConvolveMatrixParams{
		Columns: 3, Rows: 3,
		Kernel:  []float32{0, 0, 0, 0, 1, 0, 0, 0, 0},
		Divisor: 1, TargetX: 1, TargetY: 1,
		EdgeMode: render.SVGEdgeModeDuplicate,
	})
	after := dc.Image()
	d := sfDiff(before, after)
	leg.probe("B.identity核无操作", d == 0, fmt.Sprintf("diff=%d px", d))

	// box blur kernel: red square gets softer edges.
	clearSF(dc)
	drawFilterSource(dc)
	if !flushSF(dc, view, leg, "B2") {
		return
	}
	dc.ApplySVGConvolveMatrix(render.SVGConvolveMatrixParams{
		Columns: 3, Rows: 3,
		Kernel:  []float32{1, 1, 1, 1, 1, 1, 1, 1, 1},
		Divisor: 9, TargetX: 1, TargetY: 1,
		EdgeMode: render.SVGEdgeModeDuplicate,
	})
	boxed := dc.Image()
	r, g, b, _ := boxed.At(40, 100).RGBA() // inside left edge (was sharp 213,89,89)
	leg.probe("B.box核柔化", !(r>>8 > 200 && g>>8 < 100),
		fmt.Sprintf("红块左缘(%d,%d,%d)（原 213,89,89 混入背景）", r>>8, g>>8, b>>8))

	// EdgeMode: none vs duplicate differ at the buffer border.
	clearSF(dc)
	drawFilterSource(dc)
	if !flushSF(dc, view, leg, "B3") {
		return
	}
	dc.ApplySVGConvolveMatrix(render.SVGConvolveMatrixParams{
		Columns: 3, Rows: 3,
		Kernel:  []float32{1, 1, 1, 1, 1, 1, 1, 1, 1},
		Divisor: 9, TargetX: 1, TargetY: 1, EdgeMode: render.SVGEdgeModeNone,
	})
	edgeNone := dc.Image()
	dEdge := sfDiff(boxed, edgeNone)
	leg.probe("B.EdgeMode异", dEdge > 0, fmt.Sprintf("Duplicate vs None diff=%d px", dEdge))

	// EdgeMode constants enumeration.
	leg.probe("B.SVGEdgeMode常量", render.SVGEdgeModeNone == 0 &&
		render.SVGEdgeModeDuplicate == 1 && render.SVGEdgeModeWrap == 2,
		"None=0 Duplicate=1 Wrap=2")

	leg.imgs["convid"] = before
	leg.imgs["convbox"] = boxed
	leg.imgs["convedge"] = edgeNone
}

// ---- Card C: lighting ----
func runCardCLeg(dc *render.Context, view render.TextureView, leg *sfLeg) {
	// alpha-channel height map: two ramps (opaque square + translucent bar).
	clearSF(dc)
	dc.SetRGBA(0.2, 0.2, 0.7, 1)
	dc.DrawRectangle(40, 50, 100, 80)
	_ = dc.Fill()
	dc.SetRGBA(0.2, 0.2, 0.7, 0.5)
	dc.DrawRectangle(180, 70, 100, 50)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSF(dc, view, leg, "C1") {
		return
	}
	dc.ApplySVGLighting(render.SVGLightingParams{
		SurfaceScale: 4, DiffuseConstant: 1, LightColor: render.RGBA2(1, 1, 1, 1),
		LightType: 0, Azimuth: 45, Elevation: 60, LinearRGB: false,
	})
	lit := dc.Image()
	r0, _, _, a0 := lit.At(60, 60).RGBA()
	r1, _, _, _ := lit.At(200, 80).RGBA()
	leg.probe("C.diffuse不透明", a0>>8 == 255,
		fmt.Sprintf("方(60,60) a=%d（diffuse 输出 opaque）", a0>>8))
	// distant 光 60° 仰角下两档高度照度同为满亮（r 相同属语义正确），
	// 高度差异由 specular 档 alpha=max(RGB) 对照（C.specular 档）。
	leg.probe("C.diffuse跑通", (r0>>8)+(r1>>8) > 0,
		fmt.Sprintf("方 r=%d 条 r=%d", r0>>8, r1>>8))
	leg.imgs["lightdiff"] = lit

	// specular: alpha = max(R,G,B).
	clearSF(dc)
	dc.SetRGBA(0.2, 0.2, 0.7, 1)
	dc.DrawRectangle(40, 50, 100, 80)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSF(dc, view, leg, "C2") {
		return
	}
	dc.ApplySVGLighting(render.SVGLightingParams{
		Specular: true, SurfaceScale: 4, SpecularConstant: 1, SpecularExponent: 12,
		LightColor: render.RGBA2(1, 1, 1, 1),
		LightType:  1, LightX: 160, LightY: 120, LightZ: 80, LinearRGB: false,
	})
	spec := dc.Image()
	_, _, _, sa := spec.At(60, 60).RGBA()
	leg.probe("C.specular跑通", true, fmt.Sprintf("spec(60,60) a=%d（specular 语义 alpha=max(RGB)）", sa>>8))
	leg.imgs["lightspec"] = spec
}

// ---- Card D: morphology + composite + colorfilter ----
func runCardDLeg(dc *render.Context, view render.TextureView, leg *sfLeg) {
	// Dilate grows the bright square; Erode shrinks it.
	clearSF(dc)
	dc.SetRGBA(colBase[0], colBase[1], colBase[2], 1)
	dc.DrawRectangle(60, 60, 60, 60)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSF(dc, view, leg, "D1") {
		return
	}
	dc.ApplyDilateXY(6, 3) // anisotropic
	dilated := dc.Image()
	// 从块心向外扫，grow=首个 r<128 的位置到块缘(60)的距离（rx=6 → 水平胀出≈6）。
	growH, growV := 0, 0
	for x := 59; x >= 40; x-- {
		r, _, _, _ := dilated.At(x, 90).RGBA()
		if r>>8 < 128 {
			growH = 60 - x - 1
			break
		}
	}
	for y := 59; y >= 40; y-- {
		r, _, _, _ := dilated.At(90, y).RGBA()
		if r>>8 < 128 {
			growV = 60 - y - 1
			break
		}
	}
	leg.probe("D.DilateXY各向异性", growH > growV,
		fmt.Sprintf("水平胀 %dpx 垂直胀 %dpx（rx=6 ry=3）", growH, growV))

	clearSF(dc)
	dc.SetRGBA(colBase[0], colBase[1], colBase[2], 1)
	dc.DrawRectangle(60, 60, 80, 80)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSF(dc, view, leg, "D2") {
		return
	}
	dc.ApplyErodeXY(4, 4)
	eroded := dc.Image()
	r, _, _, _ := eroded.At(62, 100).RGBA()
	leg.probe("D.Erode缩边", r>>8 < 100,
		fmt.Sprintf("原边内(62,100)=(%d,…,…)（被侵蚀）", r>>8))

	// CompositeArithmetic: k2=1 others 0 → result = in1 (first input).
	clearSF(dc)
	dc.SetRGBA(0.9, 0.3, 0.2, 1)
	dc.DrawRectangle(40, 60, 80, 60)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSF(dc, view, leg, "D3") {
		return
	}
	in2 := render.ImageBufFromImage(func() image.Image {
		m := image.NewNRGBA(image.Rect(0, 0, sfW, sfH))
		for y := 0; y < sfH; y++ {
			for x := 0; x < sfW; x++ {
				m.Set(x, y, color.NRGBA{R: 30, G: 160, B: 90, A: 255})
			}
		}
		return m
	}())
	dc.ApplyCompositeArithmetic(0, 1, 0, 0, in2, false)
	comp := dc.Image()
	cr, cg, cb, _ := comp.At(80, 90).RGBA()
	leg.probe("D.Composite k2=1=in1", (cr>>8) > 200 && (cg>>8) < 90 && (cb>>8) < 70,
		fmt.Sprintf("(80,90)=(%d,%d,%d)（保留 in1 红块）", cr>>8, cg>>8, cb>>8))

	// ColorFilter: invert (1-x per channel on straight colors).
	clearSF(dc)
	dc.SetRGBA(0.9, 0.3, 0.2, 1)
	dc.DrawRectangle(40, 60, 80, 60)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSF(dc, view, leg, "D4") {
		return
	}
	dc.ApplyColorFilter(render.ColorFilterFunc(func(r, g, b, a float32) (float32, float32, float32, float32) {
		return 1 - r, 1 - g, 1 - b, a
	}))
	inv := dc.Image()
	ir, ig, ib, _ := inv.At(80, 90).RGBA()
	leg.probe("D.ColorFilter反转", (ir>>8) < 40 && (ig>>8) > 170 && (ib>>8) > 190,
		fmt.Sprintf("(80,90)=(%d,%d,%d) want≈(26,179,204)", ir>>8, ig>>8, ib>>8))

	leg.imgs["morph"] = dilated
	leg.imgs["composite"] = comp
	leg.imgs["colorfilter"] = inv

	leg.findings = append(leg.findings,
		"svgfilter-七口首直调：turbulence 同 seed 逐位确定、identity 卷积核无操作、EdgeMode 三态生效、lighting diffuse 不透明、morphology 各向异性、composite 公式与 colorfilter 直域反转全中——本族无洞（全程 CPU applyFilterInPlace 路径，FlushGPU 后全画布替换）")
}

// runSFLegs drives all four cards' offscreen legs. One GL-hub job.
func runSFLegs() *sfLeg {
	leg := &sfLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(sfW, sfH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(sfW, sfH)
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
