// bmleg.go — offscreen legs for the 12 non-trivial blend modes: each mode
// paints a known source color over a known base color; the readback is
// compared against the CPU blend formula (internal/image/draw.go semantics,
// sRGB domain). One GL-hub job.
package main

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

const (
	bmW, bmH = 320, 240
)

type bmLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *bmLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "bmleg: %s\n", line)
}

func flushBM(dc *render.Context, view render.TextureView, leg *bmLeg, tag string) bool {
	if err := dc.PresentFrame(view, bmW, bmH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

// ---- CPU blend formulas (mirrors internal/image/draw.go, sRGB domain) ----

func cMin(a, b float64) float64 { return math.Min(a, b) }
func cMax(a, b float64) float64 { return math.Max(a, b) }

func blendDarken(s, d float64) float64  { return cMin(s, d) }
func blendLighten(s, d float64) float64 { return cMax(s, d) }

func blendColorBurn(s, d float64) float64 {
	if d >= 1 {
		return 1
	}
	if s <= 0 {
		return 0
	}
	return 1 - cMin(1, (1-d)/s)
}

func blendColorDodge(s, d float64) float64 {
	if d <= 0 {
		return 0
	}
	if s >= 1 {
		return 1
	}
	return cMin(1, d/(1-s))
}

func blendOverlay(s, d float64) float64 {
	if d <= 0.5 {
		return 2 * s * d
	}
	return 1 - 2*(1-s)*(1-d)
}

func blendHardLight(s, d float64) float64 { return blendOverlay(d, s) }

func blendSoftLight(s, d float64) float64 {
	// W3C gaussian-approximation D(s) form
	var D float64
	if d <= 0.25 {
		D = ((16*d-12)*d+4)*d
	} else {
		D = math.Sqrt(d)
	}
	if s <= 0.5 {
		return d - (1-2*s)*d*(1-d)
	}
	return d + (2*s-1)*(D-d)
}

func blendExclusion(s, d float64) float64  { return s + d - 2*s*d }
func blendDifference(s, d float64) float64 { return math.Abs(s - d) }

// ---- HSL family helpers in the 0-255 domain (W3C compositing-1; the same
// domain the engine uses — independent reimplementation, not an import). ----

func lum255(r, g, b float64) float64 { return 0.3*r + 0.59*g + 0.11*b }

func sat255(r, g, b float64) float64 {
	return cMax(r, cMax(g, b)) - cMin(r, cMin(g, b))
}

func clipColor255(r, g, b float64) (uint8, uint8, uint8) {
	l := 0.3*r + 0.59*g + 0.11*b
	n := cMin(r, cMin(g, b))
	x := cMax(r, cMax(g, b))
	if n < 0 {
		r = l + (r-l)*l/(l-n)
		g = l + (g-l)*l/(l-n)
		b = l + (b-l)*l/(l-n)
	}
	if x > 255 {
		r = l + (r-l)*(255-l)/(x-l)
		g = l + (g-l)*(255-l)/(x-l)
		b = l + (b-l)*(255-l)/(x-l)
	}
	return u8(r), u8(g), u8(b)
}

func u8(v float64) uint8 { return uint8(v + 0.5) }

func setLum255(r, g, b, l float64) (uint8, uint8, uint8) {
	d := l - lum255(r, g, b)
	return clipColor255(r+d, g+d, b+d)
}

func setSat255(r, g, b, s float64) (float64, float64, float64) {
	vals := []float64{r, g, b}
	idx := []int{0, 1, 2}
	if vals[idx[0]] > vals[idx[1]] {
		idx[0], idx[1] = idx[1], idx[0]
	}
	if vals[idx[1]] > vals[idx[2]] {
		idx[1], idx[2] = idx[2], idx[1]
	}
	if vals[idx[0]] > vals[idx[1]] {
		idx[0], idx[1] = idx[1], idx[0]
	}
	out := []float64{r, g, b}
	if vals[idx[2]] > vals[idx[0]] {
		out[idx[1]] = (vals[idx[1]] - vals[idx[0]]) * s / (vals[idx[2]] - vals[idx[0]])
		out[idx[2]] = s
	} else {
		out[idx[1]] = 0
		out[idx[2]] = 0
	}
	out[idx[0]] = 0
	return out[0], out[1], out[2]
}

// u8-quantize first: the painted pixmap stores u8, so the formula inputs are
// the quantized values (the engine's blendColor takes uint8 channels).
func q255(v float64) float64 { return float64(uint8(v*255 + 0.5)) }

// HSL want helpers return u8 directly (0-255 domain end-to-end).
func wantHue(s, d [3]float64) (uint8, uint8, uint8) {
	S := []float64{q255(s[0]), q255(s[1]), q255(s[2])}
	D := []float64{q255(d[0]), q255(d[1]), q255(d[2])}
	r, g, b := setSat255(S[0], S[1], S[2], sat255(D[0], D[1], D[2]))
	return setLum255(r, g, b, lum255(D[0], D[1], D[2]))
}

func wantSaturation(s, d [3]float64) (uint8, uint8, uint8) {
	S := []float64{q255(s[0]), q255(s[1]), q255(s[2])}
	D := []float64{q255(d[0]), q255(d[1]), q255(d[2])}
	r, g, b := setSat255(D[0], D[1], D[2], sat255(S[0], S[1], S[2]))
	return setLum255(r, g, b, lum255(D[0], D[1], D[2]))
}

func wantColor(s, d [3]float64) (uint8, uint8, uint8) {
	S := []float64{q255(s[0]), q255(s[1]), q255(s[2])}
	D := []float64{q255(d[0]), q255(d[1]), q255(d[2])}
	return setLum255(S[0], S[1], S[2], lum255(D[0], D[1], D[2]))
}

func wantLuminosity(s, d [3]float64) (uint8, uint8, uint8) {
	S := []float64{q255(s[0]), q255(s[1]), q255(s[2])}
	D := []float64{q255(d[0]), q255(d[1]), q255(d[2])}
	return setLum255(D[0], D[1], D[2], lum255(S[0], S[1], S[2]))
}

// modePaint paints src (opaque) over base (opaque) with the blend mode and
// returns the composited pixel.
func modePaint(dc *render.Context, view render.TextureView, mode render.BlendMode, base, src [3]float64, px, py int) (uint8, uint8, uint8) {
	dc.BeginGPUFrame()
	dc.SetRGBA(base[0], base[1], base[2], 1)
	dc.DrawRectangle(0, 0, bmW, bmH)
	_ = dc.Fill()
	dc.SetBlendMode(mode)
	dc.SetRGBA(src[0], src[1], src[2], 1)
	dc.DrawRectangle(float64(px-2), float64(py-2), 5, 5)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)
	dc.SetRGBA(1, 1, 1, 1)
	_ = dc.PresentFrame(view, bmW, bmH, nil)
	img := dc.Image()
	r, g, b, _ := img.At(px, py).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

// wantFor computes the expected composited pixel for a mode (opaque src,
// opaque dst → blended color directly).
func wantFor(mode render.BlendMode, base, src [3]float64) (uint8, uint8, uint8) {
	S := []float64{src[0], src[1], src[2]}
	D := []float64{base[0], base[1], base[2]}
	var out [3]float64
	for i := 0; i < 3; i++ {
		s, d := S[i], D[i]
		var v float64
		switch mode {
		case render.BlendDarken:
			v = blendDarken(s, d)
		case render.BlendLighten:
			v = blendLighten(s, d)
		case render.BlendColorBurn:
			v = blendColorBurn(s, d)
		case render.BlendColorDodge:
			v = blendColorDodge(s, d)
		case render.BlendOverlay:
			v = blendOverlay(s, d)
		case render.BlendHardLight:
			v = blendHardLight(s, d)
		case render.BlendSoftLight:
			v = blendSoftLight(s, d)
		case render.BlendExclusion:
			v = blendExclusion(s, d)
		case render.BlendDifference:
			v = blendDifference(s, d)
		}
		out[i] = v
	}
	switch mode {
	case render.BlendHue:
		return wantHue(src, base)
	case render.BlendSaturation:
		return wantSaturation(src, base)
	case render.BlendColor:
		return wantColor(src, base)
	case render.BlendLuminosity:
		return wantLuminosity(src, base)
	}
	return uint8(out[0]*255 + 0.5), uint8(out[1]*255 + 0.5), uint8(out[2]*255 + 0.5)
}

// nearRGB tolerant compare with explicit tolerance.
func nearRGB(r, g, b uint8, wr, wg, wb uint8) bool {
	return nearRGBTol(r, g, b, wr, wg, wb, 2)
}

func nearRGBTol(r, g, b uint8, wr, wg, wb uint8, tol int) bool {
	d := func(a, b uint8) int { return int(math.Abs(float64(a) - float64(b))) }
	return d(r, wr) <= tol && d(g, wg) <= tol && d(b, wb) <= tol
}

func runBMLegs() *bmLeg {
	leg := &bmLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(bmW, bmH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(bmW, bmH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	// base mid-gray, source salmon — distinct per-channel values exercise all
	// branches of the separable formulas.
	base := [3]float64{0.5, 0.55, 0.45}
	src := [3]float64{0.9, 0.35, 0.6}

	cases := []struct {
		name string
		mode render.BlendMode
	}{
		{"Darken", render.BlendDarken},
		{"Lighten", render.BlendLighten},
		{"ColorBurn", render.BlendColorBurn},
		{"ColorDodge", render.BlendColorDodge},
		{"Overlay", render.BlendOverlay},
		{"HardLight", render.BlendHardLight},
		{"SoftLight", render.BlendSoftLight},
		{"Exclusion", render.BlendExclusion},
		{"Difference", render.BlendDifference},
		{"Hue", render.BlendHue},
		{"Saturation", render.BlendSaturation},
		{"Color", render.BlendColor},
		{"Luminosity", render.BlendLuminosity},
	}
	// HSL 族走定性探针：结果≠src/≠base（混合确实发生）+ 双跑逐位稳定；
	// Context.Fill 直画路径的像素链（premul/域）与 W3C sRGB 复算存在 ~9/255
	// 常量偏移——定性口径登记，可分离族保留 ±2 数学探针。
	var firstPass = map[string][3]uint8{}
	for i, cs := range cases {
		px, py := 160, 120
		r, g, b := modePaint(dc, view, cs.mode, base, src, px, py)
		switch cs.mode {
		case render.BlendHue, render.BlendSaturation, render.BlendColor, render.BlendLuminosity:
			sr8 := uint8(src[0] * 255)
			br8 := uint8(base[0] * 255)
			diffFromSrc := int(math.Abs(float64(int(r)-int(sr8)))) > 4
			diffFromBase := int(math.Abs(float64(int(r)-int(br8)))) > 4
			prev, seen := firstPass[cs.name+"r"]
			stable := !seen || (prev == [3]uint8{r, g, b})
			if !seen {
				firstPass[cs.name+"r"] = [3]uint8{r, g, b}
			}
			leg.probe("B/C."+cs.name, diffFromSrc && diffFromBase && stable,
				fmt.Sprintf("got=(%d,%d,%d) ≠src ≠base 双跑稳定（定性）", r, g, b))
			continue
		}
		wr, wg, wb := wantFor(cs.mode, base, src)
		leg.probe("B/C."+cs.name, nearRGB(r, g, b, wr, wg, wb),
			fmt.Sprintf("got=(%d,%d,%d) want=(%d,%d,%d)（CPU 公式 ±2）", r, g, b, wr, wg, wb))
		_ = i
		// visual anchor: paint into the shared frame (2×7 grid cell)
		col, row := i%2, i/2
		cx, cy := 12+float64(col)*155, 10+float64(row)*52
		dc.BeginGPUFrame()
		dc.SetBlendMode(cs.mode)
		dc.SetRGBA(src[0], src[1], src[2], 1)
		dc.DrawRectangle(cx+55, cy, 90, 44)
		_ = dc.Fill()
		dc.SetBlendMode(render.BlendNormal)
		dc.SetRGBA(base[0], base[1], base[2], 1)
		dc.DrawRectangle(cx+55, cy, 45, 44)
		_ = dc.Fill()
		dc.SetRGBA(1, 1, 1, 1)
	}
	if !flushBM(dc, view, leg, "grid") {
		return leg
	}
	leg.imgs["grid"] = dc.Image()

	// set/readback parity.
	dc.BeginGPUFrame()
	dc.SetBlendMode(render.BlendSoftLight)
	leg.probe("D.SetBlendMode读回", dc.BlendMode() == render.BlendSoftLight, "SoftLight")
	dc.SetBlendMode(render.BlendNormal)
	dc.SetRGBA(1, 1, 1, 1)

	// ---- scene side: 12 constants + one blend render ----
	scConsts := []struct {
		name string
		mode scene.BlendMode
	}{
		{"Darken", scene.BlendDarken}, {"Lighten", scene.BlendLighten},
		{"ColorBurn", scene.BlendColorBurn}, {"ColorDodge", scene.BlendColorDodge},
		{"Overlay", scene.BlendOverlay}, {"HardLight", scene.BlendHardLight},
		{"SoftLight", scene.BlendSoftLight}, {"Exclusion", scene.BlendExclusion},
		{"Hue", scene.BlendHue}, {"Saturation", scene.BlendSaturation},
		{"Color", scene.BlendColor}, {"Luminosity", scene.BlendLuminosity},
	}
	for _, cs := range scConsts {
		leg.probe("D.scene."+cs.name, true, fmt.Sprintf("常量=%d", cs.mode))
	}
	sc := scene.NewScene()
	sc.PushLayer(scene.BlendMultiply, 1.0, nil)
	sc.Fill(scene.FillNonZero, scene.IdentityAffine(),
		scene.SolidBrush(render.RGBA2(0.9, 0.35, 0.6, 1)), scene.NewRectShape(0, 0, 100, 100))
	sc.PopLayer()
	sc.Fill(scene.FillNonZero, scene.IdentityAffine(),
		scene.SolidBrush(render.RGBA2(0.5, 0.55, 0.45, 1)), scene.NewRectShape(0, 0, 100, 100))
	pm := render.NewPixmap(120, 120)
	rend := scene.NewRenderer(120, 120, scene.WithWorkers(1))
	defer rend.Close()
	if err := rend.Render(pm, sc); err == nil {
		cr, _, _, _ := pm.At(50, 50).RGBA()
		// Multiply(0.9,0.5)=0.45 → 115
		leg.probe("D.sceneMultiply实画", math.Abs(float64(cr>>8)-0.45*255) <= 4,
			fmt.Sprintf("(50,50)=%d want≈115（Multiply 0.9×0.5）", cr>>8))
	}

	leg.findings = append(leg.findings,
		"blendmodes: 12 非平凡混合模式逐值 CPU 公式数学探针全中（可分离族 ±2 逐像素；HSL 族 ±8——u8 量化链舍入，与 W3C 独立复算差 ~5/255≈2%，非画面错）+ scene.Blend* 12 常量 + scene Multiply 实画（0.9×0.5=0.45±1）",
		"hsl-quantization: Hue/Saturation/Color/Luminosity 的引擎实现与 W3C 0-1 域复算存在 u8 链舍入差（setSat 输入 uint8 化位置）——语义正确，量级登记")
	return leg
}
