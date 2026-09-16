package light

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type normalFactorDef struct {
	Name   string   `json:"name"`
	Light  lightDef `json:"light"`
	P      []float64 `json:"p"`
	Layer  int      `json:"layer"`
	Normal []float64 `json:"normal"`
	Height float64  `json:"height"`
	Want   float64  `json:"want"`
}

type normalApplyDef struct {
	Name    string      `json:"name"`
	W       int         `json:"w"`
	H       int         `json:"h"`
	OX      float64     `json:"ox"`
	OY      float64     `json:"oy"`
	Step    float64     `json:"step"`
	Src     [][]float64 `json:"src"`
	Layers  []int       `json:"layers"`
	Normals [][]float64 `json:"normals,omitempty"`
	Colors  [][]float64 `json:"colors,omitempty"`
	Night   []float64   `json:"night"`
	Lights  []lightDef  `json:"lights"`
	Height  float64     `json:"height"`
	Want    [][]float64 `json:"want"`
}

type normalNdotlDef struct {
	Name string    `json:"name"`
	N    []float64 `json:"n"`
	L    []float64 `json:"l"`
	Want float64   `json:"want"`
}

type normalFile struct {
	Factor []normalFactorDef `json:"factor"`
	Ndotl  []normalNdotlDef  `json:"ndotl"`
	Apply  []normalApplyDef  `json:"apply"`
}

func loadNormalFile(t *testing.T) normalFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "normal_cases.json"))
	if err != nil {
		t.Fatalf("read normal_cases.json: %v", err)
	}
	var f normalFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode normal_cases.json: %v", err)
	}
	if len(f.Factor) == 0 || len(f.Ndotl) == 0 || len(f.Apply) == 0 {
		t.Fatal("normal_cases.json misses a section")
	}
	return f
}

func buildNormalMap(t *testing.T, c normalApplyDef) *NormalMap {
	t.Helper()
	if c.Normals != nil {
		ns := make([]Normal, len(c.Normals))
		for i, v := range c.Normals {
			if len(v) != 3 {
				t.Fatalf("%s normal %d has %d numbers, want 3", c.Name, i, len(v))
			}
			ns[i] = N(v[0], v[1], v[2])
		}
		m, err := NewNormalMap(c.W, c.H, ns)
		if err != nil {
			t.Fatalf("%s: NewNormalMap: %v", c.Name, err)
		}
		return &m
	}
	if c.Colors == nil {
		t.Fatalf("%s: neither normals nor colors", c.Name)
	}
	cols := pixFromRaw(t, c.Name, c.Colors)
	m, err := NewNormalMapFromColors(c.W, c.H, cols)
	if err != nil {
		t.Fatalf("%s: NewNormalMapFromColors: %v", c.Name, err)
	}
	return &m
}

func buildNormalApplyImage(t *testing.T, c normalApplyDef) (*Image, *NormalMap, NormalScene) {
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
	var ls []Light
	for _, ld := range c.Lights {
		ls = append(ls, buildLight(t, c.Name, ld))
	}
	night := core.RGBA(c.Night[0], c.Night[1], c.Night[2], c.Night[3])
	sc, err := NewNormalScene(night, ls, c.Height)
	if err != nil {
		t.Fatalf("%s: NewNormalScene: %v", c.Name, err)
	}
	return img, buildNormalMap(t, c), sc
}

func findNormalApply(t *testing.T, f normalFile, name string) normalApplyDef {
	t.Helper()
	for _, c := range f.Apply {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("normal apply case %q missing", name)
	return normalApplyDef{}
}

// A: every frozen case reproduces its golden cosine, factor, and pixels;
// facing the lamp beats flat, flat beats facing away.
func TestNormalGolden(t *testing.T) {
	f := loadNormalFile(t)
	for _, c := range f.Ndotl {
		got := NdotL(N(c.N[0], c.N[1], c.N[2]), N(c.L[0], c.L[1], c.L[2]))
		if math.Abs(got-c.Want) > goldenEps {
			t.Errorf("%s: NdotL = %.9f, want %.9f", c.Name, got, c.Want)
		}
	}
	for _, c := range f.Factor {
		l := buildLight(t, c.Name, c.Light)
		n := N(c.Normal[0], c.Normal[1], c.Normal[2])
		got := NormalFactor(l, core.V2(c.P[0], c.P[1]), Layer(c.Layer), n, c.Height)
		if math.Abs(got-c.Want) > goldenEps {
			t.Errorf("%s: factor = %.9f, want %.9f", c.Name, got, c.Want)
		}
	}
	for _, c := range f.Apply {
		img, nm, sc := buildNormalApplyImage(t, c)
		got, err := sc.Apply(img, nm)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		if got.W != c.W || got.H != c.H {
			t.Errorf("%s: size %dx%d, want %dx%d", c.Name, got.W, got.H, c.W, c.H)
		}
		checkPixels(t, c.Name, got, c.Want)
	}
	// Direction ordering on one frozen spot: toward > flat > away.
	toward := N(0.8, 0, 0.6)
	flatN := FlatNormal()
	away := N(-0.8, 0, 0.6)
	pl, _ := NewPointLight(core.V2(4, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	p := core.V2(0, 0)
	ft := NormalFactor(pl, p, LayerWorld, toward, 3)
	ff := NormalFactor(pl, p, LayerWorld, flatN, 3)
	fa := NormalFactor(pl, p, LayerWorld, away, 3)
	if !(ft > ff && ff > fa && fa == 0) {
		t.Errorf("toward=%v flat=%v away=%v, want toward>flat>away=0", ft, ff, fa)
	}
}

// B: missing, zero, oversized, and corrupt inputs never crash; zero lights
// still show the night picture, never black; missing tilt reads as flat.
func TestNormalEdgesNoCrash(t *testing.T) {
	f := loadNormalFile(t)
	full := findNormalApply(t, f, "dome_side_3x3")
	src, nm, sc := buildNormalApplyImage(t, full)

	// Nil map on every path is InvalidArg (missing sheet code).
	if _, err := sc.Apply(src, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil map Apply = %v, want invalid-arg", err)
	}
	if _, err := sc.ApplyCPU(src, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil map ApplyCPU = %v, want invalid-arg", err)
	}
	if _, err := sc.ApplyGPU(src, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil map ApplyGPU = %v, want invalid-arg", err)
	}

	// Corrupt or mis-sized map is BadData.
	badLen := &NormalMap{W: 2, H: 2, N: make([]Normal, 3)}
	if _, err := sc.Apply(src, badLen); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("short map Apply = %v, want bad-data", err)
	}
	badNaN := &NormalMap{W: 1, H: 1, N: []Normal{{X: math.NaN()}}}
	if _, err := sc.Apply(src, badNaN); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("NaN map Apply = %v, want bad-data", err)
	}
	small, _ := NewNormalMap(2, 2, []Normal{FlatNormal(), FlatNormal(), FlatNormal(), FlatNormal()})
	if _, err := sc.Apply(src, &small); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("mis-sized map Apply = %v, want bad-data", err)
	}
	// Nil src stays InvalidArg, corrupt src BadData (same as 6.1).
	if _, err := sc.Apply(nil, nm); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil src Apply = %v, want invalid-arg", err)
	}

	// Zero lights still show night, never black, map ignored.
	zl := findNormalApply(t, f, "night_only_zero_lights")
	img, nm0, sc0 := buildNormalApplyImage(t, zl)
	got, err := sc0.Apply(img, nm0)
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
	// Zero intensity lamp is night too.
	pl, _ := NewPointLight(core.V2(0, 0), 10, core.White, 0, LayersFor(LayerWorld), nil)
	dark, _ := NewNormalScene(core.White, []Light{pl}, 4)
	one, _ := NewImage(1, 1, core.V2(0, 0), 1)
	one.Set(0, 0, core.RGB(1, 1, 1), LayerWorld)
	fm, _ := NewNormalMap(1, 1, []Normal{FlatNormal()})
	lit, err := dark.Apply(one, &fm)
	if err != nil {
		t.Fatalf("zero-intensity Apply: %v", err)
	}
	if lit.Pix[0] != core.White {
		t.Errorf("zero-intensity pixel = %v, want white night", lit.Pix[0])
	}

	// Bad constructors report and build nothing.
	badBuilds := []error{}
	_, err = NewNormalMap(0, 4, make([]Normal, 0))
	badBuilds = append(badBuilds, err)
	_, err = NewNormalScene(core.Color{R: math.NaN()}, nil, 2)
	badBuilds = append(badBuilds, err)
	_, err = NewNormalScene(core.White, nil, 0)
	badBuilds = append(badBuilds, err)
	_, err = NewNormalScene(core.White, nil, -1)
	badBuilds = append(badBuilds, err)
	_, err = NewNormalScene(core.White, nil, MaxLightRange+1)
	badBuilds = append(badBuilds, err)
	many := make([]Light, MaxLights+1)
	for i := range many {
		many[i], _ = NewDirectionalLight(core.V2(0, -1), core.White, 0.1, LayersFor(LayerWorld))
	}
	_, err = NewNormalScene(core.White, many, 2)
	badBuilds = append(badBuilds, err)
	for i, e := range badBuilds {
		if core.CodeOf(e) != core.CodeInvalidArg {
			t.Errorf("bad build[%d] = %v, want invalid-arg", i, e)
		}
	}
	// Length and sample faults are data faults, not arg faults.
	badData := []error{}
	_, err = NewNormalMap(2, 2, make([]Normal, 3))
	badData = append(badData, err)
	_, err = NewNormalMap(2, 2, []Normal{FlatNormal(), FlatNormal(), FlatNormal(), {X: math.Inf(1)}})
	badData = append(badData, err)
	_, err = NewNormalMapFromColors(2, 1, []core.Color{core.White})
	badData = append(badData, err)
	_, err = NewNormalMapFromColors(1, 1, []core.Color{{R: math.NaN()}})
	badData = append(badData, err)
	for i, e := range badData {
		if core.CodeOf(e) != core.CodeBadData {
			t.Errorf("bad data[%d] = %v, want bad-data", i, e)
		}
	}
	if _, err := NewNormalMap(5000, 5000, make([]Normal, 0)); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("5000x5000 map = %v, want out-of-memory", err)
	}
	if _, err := NewNormalMapFromColors(5000, 5000, nil); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("5000x5000 colors = %v, want out-of-memory", err)
	}

	// Hot path is total: degenerate tilt reads as flat, never NaN.
	lit2, _ := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	p := core.V2(0, 0)
	base := NormalFactor(lit2, p, LayerWorld, FlatNormal(), 4)
	for _, n := range []Normal{{}, {X: math.NaN()}, {X: math.Inf(1)}, {X: 1e308, Y: 1e308, Z: 1e308}} {
		if got := NormalFactor(lit2, p, LayerWorld, n, 4); got != base || !finite(got) {
			t.Errorf("NormalFactor(%v) = %v, want flat %v", n, got, base)
		}
		if c := LitNormalCPU(core.White, p, LayerWorld, []Light{lit2}, core.White, n, 4); !finiteColor(c) {
			t.Errorf("LitNormalCPU(%v) = %v, want finite", n, c)
		}
	}
	if got := NdotL(Normal{}, Normal{}); got != 1 || !finite(got) {
		t.Errorf("NdotL(zero,zero) = %v, want flat-on-flat 1", got)
	}
	if f := NormalFactor(lit2, p, Layer(99), FlatNormal(), 4); f != 0 {
		t.Errorf("bad layer factor = %v, want 0", f)
	}
	if f := NormalFactor(lit2, p, LayerWorld, FlatNormal(), 0); f != 0 {
		t.Errorf("zero height factor = %v, want 0", f)
	}
	if f := NormalFactor(lit2, core.Vec2{X: math.NaN()}, LayerWorld, FlatNormal(), 4); f != 0 {
		t.Errorf("NaN p factor = %v, want 0", f)
	}
	// LightDir refuses bad geometry with ok=false, never a wild vector.
	for _, tc := range []struct {
		name string
		l    Light
		p    core.Vec2
		h    float64
	}{
		{"zero dir", Light{Kind: KindDirectional, Dir: core.Vec2{}}, p, 1},
		{"nan height", lit2, p, math.NaN()},
		{"zero height", lit2, p, 0},
		{"nan p", lit2, core.Vec2{X: math.NaN()}, 1},
		{"bad kind", Light{Kind: Kind(7)}, p, 1},
	} {
		if _, ok := LightDir(tc.l, tc.p, tc.h); ok {
			t.Errorf("LightDir %s = ok, want false", tc.name)
		}
	}

	// At on bad coordinates or corrupt map never panics.
	if _, ok := nm.At(9, 9); ok {
		t.Error("At(9,9) = true, want false")
	}
	if _, ok := badLen.At(0, 0); ok {
		t.Error("corrupt At = true, want false")
	}
	if _, ok := (NormalMap{}).At(0, 0); ok {
		t.Error("zero map At = true, want false")
	}
	// Flat gray decodes to flat: (0.5,0.5,1) -> (0,0,1) exactly.
	gm, err := NewNormalMapFromColors(1, 1, []core.Color{core.RGBA(0.5, 0.5, 1, 1)})
	if err != nil {
		t.Fatalf("gray decode: %v", err)
	}
	if n, ok := gm.At(0, 0); !ok || n != FlatNormal() {
		t.Errorf("gray decode = %v,%v, want (0,0,1) true", n, ok)
	}
}

// C: both backends share one number path: CPU and GPU factors and pixels
// replay bitwise identical, the map stores and returns every normal
// bitwise, inputs never mutate, outputs never alias.
func TestNormalBoundaryIdentical(t *testing.T) {
	f := loadNormalFile(t)
	for _, c := range f.Apply {
		imgA, nmA, scA := buildNormalApplyImage(t, c)
		snapPix := append([]core.Color(nil), imgA.Pix...)
		snapLay := append([]Layer(nil), imgA.Layers...)
		snapN := append([]Normal(nil), nmA.N...)
		a, err := scA.ApplyCPU(imgA, nmA)
		if err != nil {
			t.Fatalf("%s: ApplyCPU: %v", c.Name, err)
		}
		imgB, nmB, scB := buildNormalApplyImage(t, c)
		b, err := scB.ApplyGPU(imgB, nmB)
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
		if a, b := NormalFactor(l, p, Layer(c.Layer), n, c.Height),
			NormalFactorGPU(l, p, Layer(c.Layer), n, c.Height); a != b {
			t.Fatalf("%s: factor CPU %v vs GPU %v", c.Name, a, b)
		}
		base := core.RGBA(0.7, 0.6, 0.5, 1)
		if a, b := LitNormalCPU(base, p, Layer(c.Layer), []Light{l}, core.White, n, c.Height),
			LitNormalGPU(base, p, Layer(c.Layer), []Light{l}, core.White, n, c.Height); a != b {
			t.Fatalf("%s: pixel CPU %v vs GPU %v", c.Name, a, b)
		}
	}
	// Scene mirrors agree too.
	c := findNormalApply(t, f, "dome_side_3x3")
	img, nm, sc := buildNormalApplyImage(t, c)
	p, _ := img.PosAt(0, 0)
	base, layer, _ := img.At(0, 0)
	n, _ := nm.At(0, 0)
	if a, b := sc.Lit(base, p, layer, n), sc.LitGPU(base, p, layer, n); a != b {
		t.Fatalf("scene Lit vs LitGPU: %v vs %v", a, b)
	}
	// Map boundary round-trips lossless: At returns the stored normal bit.
	for _, c := range f.Apply {
		if c.Normals == nil {
			continue
		}
		nm := buildNormalMap(t, c)
		for i, v := range c.Normals {
			want := N(v[0], v[1], v[2])
			got, ok := nm.At(i%c.W, i/c.W)
			if !ok || got != want {
				t.Fatalf("%s: normal %d round-trip = %v,%v, want %v", c.Name, i, got, ok, want)
			}
		}
	}
	// Color decode boundary: stored normal equals 2c-1 exactly.
	for _, c := range f.Apply {
		if c.Colors == nil {
			continue
		}
		nm := buildNormalMap(t, c)
		for i, v := range c.Colors {
			want := N(2*v[0]-1, 2*v[1]-1, 2*v[2]-1)
			got, ok := nm.At(i%c.W, i/c.W)
			if !ok || got != want {
				t.Fatalf("%s: color %d decode = %v,%v, want %v", c.Name, i, got, ok, want)
			}
		}
	}
}

// D: single-picture cost is measured (eight lamps over a 64x64 bump).
func TestNormalSinglePerf(t *testing.T) {
	const w, h = 64, 64
	img, err := NewImage(w, h, core.V2(0, 0), 1)
	if err != nil {
		t.Fatalf("NewImage 64x64: %v", err)
	}
	ns := make([]Normal, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, core.RGBA(0.8, 0.8, 0.8, 1), LayerWorld)
			ns[y*w+x] = N(float64(x-32)/64, float64(y-32)/64, 1.5)
		}
	}
	nm, err := NewNormalMap(w, h, ns)
	if err != nil {
		t.Fatalf("NewNormalMap 64x64: %v", err)
	}
	var ls []Light
	for i := 0; i < 8; i++ {
		l, err := NewPointLight(core.V2(float64(i*8), float64(i*4)), 24, core.RGBA(1, 0.95, 0.8, 1), 1.2, LayersFor(LayerWorld), nil)
		if err != nil {
			t.Fatalf("NewPointLight %d: %v", i, err)
		}
		ls = append(ls, l)
	}
	sc, err := NewNormalScene(core.RGBA(0.25, 0.25, 0.35, 1), ls, 8)
	if err != nil {
		t.Fatalf("NewNormalScene: %v", err)
	}
	start := time.Now()
	got, err := sc.Apply(img, &nm)
	el := time.Since(start)
	if err != nil {
		t.Fatalf("Apply 8-light 64x64: %v", err)
	}
	if got.W != w || got.H != h {
		t.Fatalf("size %dx%d, want %dx%d", got.W, got.H, w, h)
	}
	perPx := float64(el.Nanoseconds()) / float64(w*h) / float64(len(ls))
	t.Logf("normal-8x64x64: %v total (%.1f ns/px/light, %d px, %d lights)", el, perPx, w*h, len(ls))
	if el > 30*time.Second {
		t.Errorf("8-light normal 64x64 took %v, want < 30s (hang guard)", el)
	}
}

// E: long runs neither grow nor diverge; the dome keeps its shape (no
// drift, no glitter); bad pictures stay cheap.
func TestNormalLongRunStable(t *testing.T) {
	f := loadNormalFile(t)
	full := findNormalApply(t, f, "dome_side_3x3")
	mk := func() (*Image, *NormalMap, NormalScene) { return buildNormalApplyImage(t, full) }
	src0, nm0, g0 := mk()
	first, err := g0.Apply(src0, nm0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i := 0; i < 2000; i++ {
		src, nm, gg := mk()
		got, err := gg.Apply(src, nm)
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
	// Bad inputs stay cheap and consistent across a soak.
	bad := &NormalMap{W: 2, H: 2, N: make([]Normal, 3)}
	pl, _ := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	sc, _ := NewNormalScene(core.White, []Light{pl}, 4)
	src, _ := NewImage(2, 2, core.V2(0, 0), 1)
	for i := 0; i < 500; i++ {
		if _, err := sc.Apply(src, bad); core.CodeOf(err) != core.CodeBadData {
			t.Fatalf("soak bad %d = %v, want bad-data", i, err)
		}
		if _, err := sc.Apply(src, nil); core.CodeOf(err) != core.CodeInvalidArg {
			t.Fatalf("soak nil %d = %v, want invalid-arg", i, err)
		}
	}
}

// F: offscreen shapes stand in for the window (window adds the pixels):
// dome slopes fall left to right, the lee column keeps night, uniform
// fields stay uniform, decoded tilt beats flat, alpha untouched, and a
// flat overhead pixel equals the plain 6.1 number.
func TestNormalOffscreenShapes(t *testing.T) {
	f := loadNormalFile(t)

	// Dome under a left lamp: left slope brightest, right slope night.
	dome := findNormalApply(t, f, "dome_side_3x3")
	img, nm, sc := buildNormalApplyImage(t, dome)
	got, _ := sc.Apply(img, nm)
	if !(got.Pix[3].R >= 1 && got.Pix[3].R > got.Pix[4].R && got.Pix[4].R > got.Pix[5].R) {
		t.Errorf("dome row R = %v,%v,%v, want 1=left>mid>right", got.Pix[3].R, got.Pix[4].R, got.Pix[5].R)
	}
	for _, i := range []int{2, 5, 8} {
		p := got.Pix[i]
		if p.R != 0.2 || p.G != 0.2 || math.Abs(p.B-0.28) > goldenEps || p.A != 1 {
			t.Errorf("dome lee pixel %d = %v, want night-only (0.2,0.2,0.28,1)", i, p)
		}
	}
	if got.Pix[0] != got.Pix[6] || got.Pix[1] != got.Pix[7] {
		t.Error("dome top/bottom rows differ, want symmetric")
	}

	// Flat overhead field is uniform; directional field is uniform too.
	flat := findNormalApply(t, f, "flat_overhead_2x2")
	img, nm, sc = buildNormalApplyImage(t, flat)
	got, _ = sc.Apply(img, nm)
	for i := 1; i < len(got.Pix); i++ {
		if got.Pix[i] != got.Pix[0] {
			t.Errorf("flat pixel %d = %v vs %v, want uniform", i, got.Pix[i], got.Pix[0])
		}
	}
	dir := findNormalApply(t, f, "dir_uniform_2x2")
	img, nm, sc = buildNormalApplyImage(t, dir)
	got, _ = sc.Apply(img, nm)
	for i := 1; i < len(got.Pix); i++ {
		if got.Pix[i] != got.Pix[0] {
			t.Errorf("dir pixel %d = %v vs %v, want uniform", i, got.Pix[i], got.Pix[0])
		}
	}
	if p0 := got.Pix[0]; !(p0.R == p0.G && p0.B > p0.R && p0.A == 1) {
		t.Errorf("dir pixel = %v, want white lamp R==G, night B higher, A=1", p0)
	}

	// Decoded tilt facing the lamp beats decoded flat on the same row.
	dec := findNormalApply(t, f, "colors_decode_2x1")
	img, nm, sc = buildNormalApplyImage(t, dec)
	got, _ = sc.Apply(img, nm)
	if !(got.Pix[1].R > got.Pix[0].R) {
		t.Errorf("decoded tilt=%v flat=%v, want tilt brighter", got.Pix[1], got.Pix[0])
	}

	// Flat normal straight under the bulb equals the plain 6.1 pixel.
	pos := core.V2(3, 3)
	pl, _ := NewPointLight(pos, 10, core.White, 1, LayersFor(LayerWorld), nil)
	plain, _ := NewScene(core.White, []Light{pl})
	nsc, _ := NewNormalScene(core.White, []Light{pl}, 4)
	base := core.RGBA(0.7, 0.6, 0.5, 1)
	if a, b := plain.Lit(base, pos, LayerWorld), nsc.Lit(base, pos, LayerWorld, FlatNormal()); a != b {
		t.Errorf("overhead flat normal %v vs plain 6.1 %v, want equal", b, a)
	}

	// Alpha rides through everywhere.
	for _, c := range f.Apply {
		img, nm, sc := buildNormalApplyImage(t, c)
		got, _ := sc.Apply(img, nm)
		for i, p := range got.Pix {
			if p.A != 1 {
				t.Fatalf("%s pixel %d alpha = %v, want 1", c.Name, i, p.A)
			}
		}
	}
}
