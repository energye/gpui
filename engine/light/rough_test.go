package light

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/engine/core"
)

type roughFactorDef struct {
	Name    string    `json:"name"`
	Light   lightDef  `json:"light"`
	P       []float64 `json:"p"`
	Layer   int       `json:"layer"`
	Normal  []float64 `json:"normal"`
	Rough   float64   `json:"rough"`
	Height  float64   `json:"height"`
	Weights []float64 `json:"weights"`
	Want    float64   `json:"want"`
}

type roughApplyDef struct {
	Name    string      `json:"name"`
	W       int         `json:"w"`
	H       int         `json:"h"`
	OX      float64     `json:"ox"`
	OY      float64     `json:"oy"`
	Step    float64     `json:"step"`
	Src     [][]float64 `json:"src"`
	Layers  []int       `json:"layers"`
	Normals [][]float64 `json:"normals"`
	Rough   []float64   `json:"rough"`
	Night   []float64   `json:"night"`
	Lights  []lightDef  `json:"lights"`
	Height  float64     `json:"height"`
	Weights []float64   `json:"weights"`
	Want    [][]float64 `json:"want"`
}

type roughFile struct {
	Factor []roughFactorDef `json:"factor"`
	Apply  []roughApplyDef  `json:"apply"`
}

func loadRoughFile(t *testing.T) roughFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "rough_cases.json"))
	if err != nil {
		t.Fatalf("read rough_cases.json: %v", err)
	}
	var f roughFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode rough_cases.json: %v", err)
	}
	if len(f.Factor) == 0 || len(f.Apply) == 0 {
		t.Fatal("rough_cases.json misses a section")
	}
	return f
}

func roughWeightsOf(t *testing.T, what string, raw []float64) RoughWeights {
	t.Helper()
	if len(raw) != 2 {
		t.Fatalf("%s weights has %d numbers, want 2", what, len(raw))
	}
	w, err := NewRoughWeights(raw[0], raw[1])
	if err != nil {
		t.Fatalf("%s: NewRoughWeights: %v", what, err)
	}
	return w
}

func buildRoughApplyImage(t *testing.T, c roughApplyDef) (*Image, *NormalMap, *RoughMap, RoughScene) {
	t.Helper()
	pix := pixFromRaw(t, c.Name, c.Src)
	layers := make([]Layer, len(c.Layers))
	for i, v := range c.Layers {
		layers[i] = Layer(v)
	}
	img, err := NewImageFromColors(c.W, c.H, core.V2(c.OX, c.OY), c.Step, pix, layers)
	if err != nil {
		t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
	}
	ns := make([]Normal, len(c.Normals))
	for i, v := range c.Normals {
		if len(v) != 3 {
			t.Fatalf("%s normal %d has %d numbers, want 3", c.Name, i, len(v))
		}
		ns[i] = N(v[0], v[1], v[2])
	}
	nm, err := NewNormalMap(c.W, c.H, ns)
	if err != nil {
		t.Fatalf("%s: NewNormalMap: %v", c.Name, err)
	}
	rm, err := NewRoughMap(c.W, c.H, c.Rough)
	if err != nil {
		t.Fatalf("%s: NewRoughMap: %v", c.Name, err)
	}
	var ls []Light
	for _, ld := range c.Lights {
		ls = append(ls, buildLight(t, c.Name, ld))
	}
	night := core.RGBA(c.Night[0], c.Night[1], c.Night[2], c.Night[3])
	sc, err := NewRoughScene(night, ls, c.Height, roughWeightsOf(t, c.Name, c.Weights))
	if err != nil {
		t.Fatalf("%s: NewRoughScene: %v", c.Name, err)
	}
	return img, &nm, &rm, sc
}

func findRoughApply(t *testing.T, f roughFile, name string) roughApplyDef {
	t.Helper()
	for _, c := range f.Apply {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("rough apply case %q missing", name)
	return roughApplyDef{}
}

// A: every frozen case reproduces its golden factor and pixels; facing
// the lamp beats flat, flat beats facing away, mirror beats matte.
func TestRoughGolden(t *testing.T) {
	f := loadRoughFile(t)
	for _, c := range f.Factor {
		l := buildLight(t, c.Name, c.Light)
		n := N(c.Normal[0], c.Normal[1], c.Normal[2])
		got := RoughFactor(l, core.V2(c.P[0], c.P[1]), Layer(c.Layer), n, c.Rough, c.Height, roughWeightsOf(t, c.Name, c.Weights))
		if math.Abs(got-c.Want) > goldenEps {
			t.Errorf("%s: factor = %.9f, want %.9f", c.Name, got, c.Want)
		}
	}
	for _, c := range f.Apply {
		img, nm, rm, sc := buildRoughApplyImage(t, c)
		got, err := sc.Apply(img, nm, rm)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		if got.W != c.W || got.H != c.H {
			t.Errorf("%s: size %dx%d, want %dx%d", c.Name, got.W, got.H, c.W, c.H)
		}
		checkPixels(t, c.Name, got, c.Want)
	}
	// Direction ordering on one frozen spot: toward > flat > away.
	w := DefaultRoughWeights()
	toward := N(0.8, 0, 0.6)
	flatN := FlatNormal()
	away := N(-0.8, 0, 0.6)
	pl, _ := NewPointLight(core.V2(4, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	p := core.V2(0, 0)
	ft := RoughFactor(pl, p, LayerWorld, toward, 0.2, 3, w)
	ff := RoughFactor(pl, p, LayerWorld, flatN, 0.2, 3, w)
	fa := RoughFactor(pl, p, LayerWorld, away, 0.2, 3, w)
	if !(ft > ff && ff > fa) {
		t.Errorf("toward=%v flat=%v away=%v, want toward>flat>away", ft, ff, fa)
	}
	// Same spot, same lamp: mirror outshines matte.
	pl2, _ := NewPointLight(core.V2(1, 1), 2, core.White, 1, LayersFor(LayerWorld), nil)
	fm := RoughFactor(pl2, core.V2(1, 1), LayerWorld, flatN, 0.05, 2, w)
	fr := RoughFactor(pl2, core.V2(1, 1), LayerWorld, flatN, 0.95, 2, w)
	if !(fm > fr) {
		t.Errorf("mirror=%v matte=%v, want mirror>matte", fm, fr)
	}
}

// B: missing maps fall back (flat tilt, middle matte), never crash; zero
// lights still show the night picture, never black; corrupt or mis-sized
// maps are BadData; off-sum weights are BadData, never silently fixed.
func TestRoughEdgesNoCrash(t *testing.T) {
	f := loadRoughFile(t)
	full := findRoughApply(t, f, "dome_noon_3x3")
	src, nm, rm, sc := buildRoughApplyImage(t, full)

	// Nil maps on every path fall back per pixel, never an error.
	for _, tc := range []struct {
		name string
		call func() (*Image, error)
	}{
		{"nil/nil", func() (*Image, error) { return sc.Apply(src, nil, nil) }},
		{"nil/rm", func() (*Image, error) { return sc.Apply(src, nil, rm) }},
		{"nm/nil", func() (*Image, error) { return sc.Apply(src, nm, nil) }},
	} {
		got, err := tc.call()
		if err != nil {
			t.Errorf("%s Apply: %v, want fallback", tc.name, err)
		} else if got == nil || len(got.Pix) != len(src.Pix) {
			t.Errorf("%s Apply returned %v, want full picture", tc.name, got)
		}
	}
	if _, err := sc.ApplyCPU(src, nil, nil); err != nil {
		t.Errorf("nil ApplyCPU = %v, want fallback", err)
	}
	if _, err := sc.ApplyGPU(src, nil, nil); err != nil {
		t.Errorf("nil ApplyGPU = %v, want fallback", err)
	}

	// Corrupt or mis-sized maps are BadData on every path.
	badLen := &RoughMap{W: 2, H: 2, R: []float64{0.5, 0.5, 0.5}}
	if _, err := sc.Apply(src, nm, badLen); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("short map Apply = %v, want bad-data", err)
	}
	badNaN := &RoughMap{W: 9, H: 1, R: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, math.NaN()}}
	if _, err := sc.Apply(src, nm, badNaN); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("NaN map Apply = %v, want bad-data", err)
	}
	small, _ := NewRoughMap(2, 2, []float64{0.5, 0.5, 0.5, 0.5})
	if _, err := sc.Apply(src, nm, &small); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("mis-sized map Apply = %v, want bad-data", err)
	}
	badNLen := &NormalMap{W: 2, H: 2, N: make([]Normal, 3)}
	if _, err := sc.Apply(src, badNLen, rm); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("short normal Apply = %v, want bad-data", err)
	}
	// Nil src stays InvalidArg, corrupt src BadData (same as 6.1).
	if _, err := sc.Apply(nil, nm, rm); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil src Apply = %v, want invalid-arg", err)
	}

	// Zero lights still show night, never black, maps ignored.
	zl := findRoughApply(t, f, "night_only_zero_lights")
	img, nm0, rm0, sc0 := buildRoughApplyImage(t, zl)
	got, err := sc0.Apply(img, nm0, rm0)
	if err != nil {
		t.Fatalf("zero-light Apply: %v", err)
	}
	for i, p := range got.Pix {
		if math.Abs(p.R-0.2) > goldenEps || math.Abs(p.G-0.2) > goldenEps || math.Abs(p.B-0.4) > goldenEps {
			t.Fatalf("zero-light pixel %d = %v, want night (0.2,0.2,0.4)", i, p)
		}
		if p.R == 0 && p.G == 0 && p.B == 0 {
			t.Fatalf("zero-light pixel %d is black, want night", i)
		}
	}

	// Bad constructors report and build nothing.
	if _, err := NewRoughMap(0, 4, nil); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("0-wide map = %v, want bad-data", err)
	}
	if _, err := NewRoughMap(2, 2, make([]float64, 3)); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("short map = %v, want bad-data", err)
	}
	for _, s := range [][]float64{{math.NaN(), 0.5, 0.5, 0.5}, {math.Inf(1), 0.5, 0.5, 0.5}, {-0.1, 0.5, 0.5, 0.5}, {1.1, 0.5, 0.5, 0.5}} {
		if _, err := NewRoughMap(2, 2, s); core.CodeOf(err) != core.CodeBadData {
			t.Errorf("bad sample %v = %v, want bad-data", s[0], err)
		}
	}
	if _, err := NewRoughMap(5000, 5000, nil); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("5000x5000 map = %v, want out-of-memory", err)
	}
	// Weight sum must be 1 within 0.001: drift is BadData, never fixed.
	for _, ws := range [][2]float64{{0.6, 0.5}, {0.7, 0.4}, {math.NaN(), 0.3}, {-0.1, 1.1}, {0.7, math.Inf(1)}} {
		if _, err := NewRoughWeights(ws[0], ws[1]); core.CodeOf(err) != core.CodeBadData {
			t.Errorf("weights %v = %v, want bad-data", ws, err)
		}
	}
	if _, err := NewRoughWeights(0.7, 0.3); err != nil {
		t.Errorf("weights (0.7,0.3) = %v, want ok", err)
	}
	if _, err := NewRoughWeights(0.5, 0.5); err != nil {
		t.Errorf("weights (0.5,0.5) = %v, want ok", err)
	}
	badBuilds := []error{}
	_, err = NewRoughScene(core.Color{R: math.NaN()}, nil, 2, DefaultRoughWeights())
	badBuilds = append(badBuilds, err)
	_, err = NewRoughScene(core.White, nil, 0, DefaultRoughWeights())
	badBuilds = append(badBuilds, err)
	_, err = NewRoughScene(core.White, nil, -1, DefaultRoughWeights())
	badBuilds = append(badBuilds, err)
	_, err = NewRoughScene(core.White, nil, MaxLightRange+1, DefaultRoughWeights())
	badBuilds = append(badBuilds, err)
	many := make([]Light, MaxLights+1)
	for i := range many {
		many[i], _ = NewDirectionalLight(core.V2(0, -1), core.White, 0.1, LayersFor(LayerWorld))
	}
	_, err = NewRoughScene(core.White, many, 2, DefaultRoughWeights())
	badBuilds = append(badBuilds, err)
	for i, e := range badBuilds {
		if core.CodeOf(e) != core.CodeInvalidArg {
			t.Errorf("bad build[%d] = %v, want invalid-arg", i, e)
		}
	}
	if _, err := NewRoughScene(core.White, nil, 2, RoughWeights{Diffuse: 0.6, Specular: 0.5}); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("off-sum scene = %v, want bad-data", err)
	}

	// Hot path is total: non-finite roughness reads as FlatRough,
	// out-of-range finite clamps like every other color knob in this
	// package, degenerate tilt reads as flat, off-sum weights read as
	// 0 share, never NaN anywhere.
	lit, _ := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	p := core.V2(0, 0)
	w := DefaultRoughWeights()
	base := RoughFactor(lit, p, LayerWorld, FlatNormal(), FlatRough(), 4, w)
	for _, r := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := RoughFactor(lit, p, LayerWorld, FlatNormal(), r, 4, w); got != base || !finite(got) {
			t.Errorf("RoughFactor(%v) = %v, want flat %v", r, got, base)
		}
		if c := LitRoughCPU(core.White, p, LayerWorld, []Light{lit}, core.White, FlatNormal(), r, 4, w); !finiteColor(c) {
			t.Errorf("LitRoughCPU(%v) = %v, want finite", r, c)
		}
	}
	mirror := RoughFactor(lit, p, LayerWorld, FlatNormal(), 0, 4, w)
	matte := RoughFactor(lit, p, LayerWorld, FlatNormal(), 1, 4, w)
	if got := RoughFactor(lit, p, LayerWorld, FlatNormal(), -1, 4, w); got != mirror {
		t.Errorf("RoughFactor(-1) = %v, want clamped mirror %v", got, mirror)
	}
	if got := RoughFactor(lit, p, LayerWorld, FlatNormal(), 2, 4, w); got != matte {
		t.Errorf("RoughFactor(2) = %v, want clamped matte %v", got, matte)
	}
	for _, n := range []Normal{{}, {X: math.NaN()}, {X: math.Inf(1)}} {
		if got := RoughFactor(lit, p, LayerWorld, n, 0.5, 4, w); got != base || !finite(got) {
			t.Errorf("RoughFactor(%v) = %v, want flat %v", n, got, base)
		}
	}
	off := RoughWeights{Diffuse: 0.6, Specular: 0.5}
	if f := RoughFactor(lit, p, LayerWorld, FlatNormal(), 0.5, 4, off); f != 0 {
		t.Errorf("off-sum factor = %v, want 0", f)
	}
	if f := RoughFactor(lit, p, Layer(99), FlatNormal(), 0.5, 4, w); f != 0 {
		t.Errorf("bad layer factor = %v, want 0", f)
	}
	if f := RoughFactor(lit, p, LayerWorld, FlatNormal(), 0.5, 0, w); f != 0 {
		t.Errorf("zero height factor = %v, want 0", f)
	}

	// At on bad coordinates or corrupt map never panics, reads FlatRough.
	if v, ok := rm.At(9, 9); ok || v != FlatRough() {
		t.Errorf("At(9,9) = %v,%v, want (0.5,false)", v, ok)
	}
	if v, ok := badLen.At(0, 0); ok || v != FlatRough() {
		t.Errorf("corrupt At = %v,%v, want (0.5,false)", v, ok)
	}
	if v, ok := (RoughMap{}).At(0, 0); ok || v != FlatRough() {
		t.Errorf("zero map At = %v,%v, want (0.5,false)", v, ok)
	}
	if got := RoughSpec(Normal{}, Normal{}, math.NaN()); got != RoughSpec(FlatNormal(), FlatNormal(), FlatRough()) {
		t.Errorf("RoughSpec(NaN rough) = %v, want flat answer", got)
	}
	if got := RoughSpec(N(1, 0, 0), N(-1, 0, 0), 0.1); got != 0 {
		t.Errorf("RoughSpec(backlit) = %v, want 0", got)
	}
}

// C: both backends share one number path: CPU and GPU factors and pixels
// replay bitwise identical, the maps store and return every sample
// bitwise, inputs never mutate, outputs never alias.
func TestRoughBoundaryIdentical(t *testing.T) {
	f := loadRoughFile(t)
	for _, c := range f.Apply {
		imgA, nmA, rmA, scA := buildRoughApplyImage(t, c)
		snapPix := append([]core.Color(nil), imgA.Pix...)
		snapLay := append([]Layer(nil), imgA.Layers...)
		snapN := append([]Normal(nil), nmA.N...)
		snapR := append([]float64(nil), rmA.R...)
		a, err := scA.ApplyCPU(imgA, nmA, rmA)
		if err != nil {
			t.Fatalf("%s: ApplyCPU: %v", c.Name, err)
		}
		imgB, nmB, rmB, scB := buildRoughApplyImage(t, c)
		b, err := scB.ApplyGPU(imgB, nmB, rmB)
		if err != nil {
			t.Fatalf("%s: ApplyGPU: %v", c.Name, err)
		}
		if !a.ApproxEqual(b, 0) {
			t.Fatalf("%s: CPU/GPU diverged (want bitwise identical)", c.Name)
		}
		for i := range snapPix {
			if imgA.Pix[i] != snapPix[i] || imgA.Layers[i] != snapLay[i] {
				t.Fatalf("%s: image input mutated at pixel %d", c.Name, i)
			}
			if nmA.N[i] != snapN[i] {
				t.Fatalf("%s: normal input mutated at pixel %d", c.Name, i)
			}
			if rmA.R[i] != snapR[i] {
				t.Fatalf("%s: rough input mutated at pixel %d", c.Name, i)
			}
		}
		a.Pix[0] = core.Magenta
		if imgA.Pix[0] == core.Magenta {
			t.Fatalf("%s: output aliases input", c.Name)
		}
		if b.Pix[0] == core.Magenta {
			t.Fatalf("%s: outputs share storage", c.Name)
		}
	}
	for _, c := range f.Factor {
		l := buildLight(t, c.Name, c.Light)
		p := core.V2(c.P[0], c.P[1])
		n := N(c.Normal[0], c.Normal[1], c.Normal[2])
		w := roughWeightsOf(t, c.Name, c.Weights)
		if a, b := RoughFactor(l, p, Layer(c.Layer), n, c.Rough, c.Height, w),
			RoughFactorGPU(l, p, Layer(c.Layer), n, c.Rough, c.Height, w); a != b {
			t.Fatalf("%s: factor CPU %v vs GPU %v", c.Name, a, b)
		}
		base := core.RGBA(0.7, 0.6, 0.5, 1)
		if a, b := LitRoughCPU(base, p, Layer(c.Layer), []Light{l}, core.White, n, c.Rough, c.Height, w),
			LitRoughGPU(base, p, Layer(c.Layer), []Light{l}, core.White, n, c.Rough, c.Height, w); a != b {
			t.Fatalf("%s: pixel CPU %v vs GPU %v", c.Name, a, b)
		}
	}
	// Scene mirrors agree too.
	c := findRoughApply(t, f, "dome_noon_3x3")
	img, nm, rm, sc := buildRoughApplyImage(t, c)
	p, _ := img.PosAt(0, 0)
	base, layer, _ := img.At(0, 0)
	n, _ := nm.At(0, 0)
	r, _ := rm.At(0, 0)
	if a, b := sc.Lit(base, p, layer, n, r), sc.LitGPU(base, p, layer, n, r); a != b {
		t.Fatalf("scene Lit vs LitGPU: %v vs %v", a, b)
	}
	// Map boundary round-trips lossless: At returns the stored sample.
	for _, c := range f.Apply {
		img, nm, rm, _ := buildRoughApplyImage(t, c)
		_ = img
		_ = nm
		for i, v := range c.Rough {
			got, ok := rm.At(i%c.W, i/c.W)
			if !ok || got != v {
				t.Fatalf("%s: rough %d round-trip = %v,%v, want %v", c.Name, i, got, ok, v)
			}
		}
	}
}

// D: single-picture cost is measured (eight lamps over a 64x64 bump with
// normal and roughness sheets).
func TestRoughSinglePerf(t *testing.T) {
	const w, h = 64, 64
	img, err := NewImage(w, h, core.V2(0, 0), 1)
	if err != nil {
		t.Fatalf("NewImage 64x64: %v", err)
	}
	ns := make([]Normal, w*h)
	rs := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, core.RGBA(0.8, 0.8, 0.8, 1), LayerWorld)
			ns[y*w+x] = N(float64(x-32)/64, float64(y-32)/64, 1.5)
			rs[y*w+x] = 0.2 + 0.6*float64((x+y)%5)/4
		}
	}
	nm, err := NewNormalMap(w, h, ns)
	if err != nil {
		t.Fatalf("NewNormalMap 64x64: %v", err)
	}
	rm, err := NewRoughMap(w, h, rs)
	if err != nil {
		t.Fatalf("NewRoughMap 64x64: %v", err)
	}
	var ls []Light
	for i := 0; i < 8; i++ {
		l, err := NewPointLight(core.V2(float64(i*8), float64(i*4)), 24, core.RGBA(1, 0.95, 0.8, 1), 1.2, LayersFor(LayerWorld), nil)
		if err != nil {
			t.Fatalf("NewPointLight %d: %v", i, err)
		}
		ls = append(ls, l)
	}
	sc, err := NewRoughScene(core.RGBA(0.25, 0.25, 0.35, 1), ls, 8, DefaultRoughWeights())
	if err != nil {
		t.Fatalf("NewRoughScene: %v", err)
	}
	start := time.Now()
	got, err := sc.Apply(img, &nm, &rm)
	el := time.Since(start)
	if err != nil {
		t.Fatalf("Apply 8-light 64x64: %v", err)
	}
	if got.W != w || got.H != h {
		t.Fatalf("size %dx%d, want %dx%d", got.W, got.H, w, h)
	}
	perPx := float64(el.Nanoseconds()) / float64(w*h) / float64(len(ls))
	t.Logf("rough-8x64x64: %v total (%.1f ns/px/light, %d px, %d lights)", el, perPx, w*h, len(ls))
	if el > 30*time.Second {
		t.Errorf("8-light rough 64x64 took %v, want < 30s (hang guard)", el)
	}
}

// E: long runs neither grow nor diverge; the dome keeps its shape (no
// drift, no glitter); bad pictures stay cheap and keep their code.
func TestRoughLongRunStable(t *testing.T) {
	f := loadRoughFile(t)
	full := findRoughApply(t, f, "dome_noon_3x3")
	mk := func() (*Image, *NormalMap, *RoughMap, RoughScene) { return buildRoughApplyImage(t, full) }
	src0, nm0, rm0, g0 := mk()
	first, err := g0.Apply(src0, nm0, rm0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i := 0; i < 2000; i++ {
		src, nm, rm, gg := mk()
		got, err := gg.Apply(src, nm, rm)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if !got.ApproxEqual(first, 0) {
			t.Fatalf("rep %d diverged", i)
		}
		if len(got.Pix) != len(first.Pix) {
			t.Fatalf("rep %d len %d vs %d", i, len(got.Pix), len(first.Pix))
		}
	}
	// Shape survives the soak: left slope lit, right slope night-only.
	if !(first.Pix[3].R > first.Pix[4].R && first.Pix[4].R > first.Pix[5].R) {
		t.Fatalf("dome drifted: R left=%v mid=%v right=%v", first.Pix[3].R, first.Pix[4].R, first.Pix[5].R)
	}
	// Bad inputs stay cheap and keep their code across a soak.
	bads := []*RoughMap{
		{W: 2, H: 2, R: make([]float64, 3)},
		{W: 3, H: 3, R: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, math.NaN()}},
		{W: 2, H: 2, R: []float64{0.5, 0.5, 0.5, 0.5}},
	}
	pl, _ := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	sc, _ := NewRoughScene(core.White, []Light{pl}, 4, DefaultRoughWeights())
	src, _ := NewImage(3, 3, core.V2(0, 0), 1)
	for i := 0; i < 500; i++ {
		if _, err := sc.Apply(src, nm0, bads[i%len(bads)]); core.CodeOf(err) != core.CodeBadData {
			t.Fatalf("soak bad %d = %v, want bad-data", i, err)
		}
	}
}

// F: offscreen shapes stand in for the window (window adds the pixels):
// the noon bump beats its rim by half again, night torch lights the face
// past the clothes, wood/skin/iron read apart, uniform fields stay
// uniform, alpha rides through, and the base texture is really there.
func TestRoughOffscreenShapes(t *testing.T) {
	f := loadRoughFile(t)

	// Noon dome: peak beats the rim by more than half, lee keeps night.
	dome := findRoughApply(t, f, "dome_noon_3x3")
	img, nm, rm, sc := buildRoughApplyImage(t, dome)
	got, err := sc.Apply(img, nm, rm)
	if err != nil {
		t.Fatalf("dome Apply: %v", err)
	}
	peak, rim := got.Pix[3].R, got.Pix[5].R
	if !(peak >= 1 && peak/rim > 1.5) {
		t.Errorf("dome peak=%v rim=%v ratio=%.3f, want 1=peak>1.5x rim", peak, rim, peak/rim)
	}
	if !(got.Pix[3].R > got.Pix[4].R && got.Pix[4].R > got.Pix[5].R) {
		t.Errorf("dome row R = %v,%v,%v, want left>mid>right", got.Pix[3].R, got.Pix[4].R, got.Pix[5].R)
	}
	// Lee column keeps base*night, never black.
	lee := []struct {
		i    int
		base float64
	}{{2, 0.82}, {5, 0.78}, {8, 0.82}}
	for _, tc := range lee {
		p := got.Pix[tc.i]
		if math.Abs(p.R-tc.base*0.5) > goldenEps || math.Abs(p.G-tc.base*0.5) > goldenEps ||
			math.Abs(p.B-tc.base*0.58) > goldenEps || p.A != 1 {
			t.Errorf("dome lee pixel %d = %v, want base*night (%.4f,%.4f,%.4f,1)", tc.i, p, tc.base*0.5, tc.base*0.5, tc.base*0.58)
		}
	}
	// Base texture is really there: the dome src is not one flat gray.
	if img.Pix[0] == img.Pix[1] && img.Pix[1] == img.Pix[4] {
		t.Error("dome src is uniform gray, want a real base texture")
	}

	// Night torch: face outshines the clothes, clothes stay above night.
	torch := findRoughApply(t, f, "torch_night_face_2x2")
	img, nm, rm, sc = buildRoughApplyImage(t, torch)
	got, err = sc.Apply(img, nm, rm)
	if err != nil {
		t.Fatalf("torch Apply: %v", err)
	}
	if !(got.Pix[0].R > got.Pix[1].R+0.3 && got.Pix[2].R > got.Pix[3].R+0.3) {
		t.Errorf("torch face=%v,%v clothes=%v,%v, want face brighter by 0.3+", got.Pix[0], got.Pix[2], got.Pix[1], got.Pix[3])
	}
	if !(got.Pix[1].R > 0.12 && got.Pix[3].R > 0.12) {
		t.Errorf("torch clothes=%v,%v, want above night 0.12", got.Pix[1], got.Pix[3])
	}

	// Wood matte, skin middle, iron bright: three stops read apart.
	strip := findRoughApply(t, f, "wood_skin_iron_3x1")
	img, nm, rm, sc = buildRoughApplyImage(t, strip)
	got, err = sc.Apply(img, nm, rm)
	if err != nil {
		t.Fatalf("strip Apply: %v", err)
	}
	wood, skin, iron := got.Pix[0].R, got.Pix[1].R, got.Pix[2].R
	if !(wood < skin && skin < iron) {
		t.Errorf("wood=%v skin=%v iron=%v, want wood<skin<iron", wood, skin, iron)
	}
	if !(skin-wood > 0.1 && iron-skin > 0.05) {
		t.Errorf("wood=%v skin=%v iron=%v, want gaps skin-wood>0.1 iron-skin>0.05", wood, skin, iron)
	}

	// Flat field is uniform; zero lights read night exactly.
	flat := findRoughApply(t, f, "flat_uniform_2x2")
	img, nm, rm, sc = buildRoughApplyImage(t, flat)
	got, _ = sc.Apply(img, nm, rm)
	for i := 1; i < len(got.Pix); i++ {
		if got.Pix[i] != got.Pix[0] {
			t.Errorf("flat pixel %d = %v vs %v, want uniform", i, got.Pix[i], got.Pix[0])
		}
	}
	zl := findRoughApply(t, f, "night_only_zero_lights")
	img, nm, rm, sc = buildRoughApplyImage(t, zl)
	got, _ = sc.Apply(img, nm, rm)
	for i, p := range got.Pix {
		if p.R != 0.2 || p.G != 0.2 || p.B != 0.4 || p.A != 1 {
			t.Errorf("night pixel %d = %v, want (0.2,0.2,0.4,1)", i, p)
		}
	}

	// Alpha rides through everywhere.
	for _, c := range f.Apply {
		img, nm, rm, sc := buildRoughApplyImage(t, c)
		got, _ := sc.Apply(img, nm, rm)
		for i, p := range got.Pix {
			if p.A != 1 {
				t.Fatalf("%s pixel %d alpha = %v, want 1", c.Name, i, p.A)
			}
		}
	}
}
