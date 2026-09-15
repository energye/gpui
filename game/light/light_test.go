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

// Tolerance is explicit: pure float64 math replays bitwise, JSON goldens
// round-trip exactly, so 1e-9 is generous slack, never a hidden fudge.
const goldenEps = 1e-9

type cookieDef struct {
	W    int       `json:"w"`
	H    int       `json:"h"`
	Mask []float64 `json:"mask"`
}

type lightDef struct {
	Kind      string     `json:"kind"`
	Pos       []float64  `json:"pos"`
	Dir       []float64  `json:"dir"`
	Range     float64    `json:"range"`
	Color     []float64  `json:"color"`
	Intensity float64    `json:"intensity"`
	Layers    uint32     `json:"layers"`
	Cookie    *cookieDef `json:"cookie,omitempty"`
}

type factorCaseDef struct {
	Name  string    `json:"name"`
	Light lightDef  `json:"light"`
	P     []float64 `json:"p"`
	Layer int       `json:"layer"`
	Want  float64   `json:"want"`
}

type applyCaseDef struct {
	Name   string      `json:"name"`
	W      int         `json:"w"`
	H      int         `json:"h"`
	OX     float64     `json:"ox"`
	OY     float64     `json:"oy"`
	Step   float64     `json:"step"`
	Src    [][]float64 `json:"src"`
	Layers []int       `json:"layers"`
	Night  []float64   `json:"night"`
	Lights []lightDef  `json:"lights"`
	Want   [][]float64 `json:"want"`
}

type lightFile struct {
	Factor []factorCaseDef `json:"factor"`
	Apply  []applyCaseDef  `json:"apply"`
}

func loadLightFile(t *testing.T) lightFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "light_cases.json"))
	if err != nil {
		t.Fatalf("read light_cases.json: %v", err)
	}
	var f lightFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode light_cases.json: %v", err)
	}
	if len(f.Factor) == 0 || len(f.Apply) == 0 {
		t.Fatal("light_cases.json misses a section")
	}
	return f
}

func buildLight(t *testing.T, what string, d lightDef) Light {
	t.Helper()
	col := core.RGBA(d.Color[0], d.Color[1], d.Color[2], d.Color[3])
	var ck *Cookie
	if d.Cookie != nil {
		c, err := NewCookie(d.Cookie.W, d.Cookie.H, d.Cookie.Mask)
		if err != nil {
			t.Fatalf("%s: NewCookie: %v", what, err)
		}
		ck = &c
	}
	if d.Kind == "directional" {
		l, err := NewDirectionalLight(core.V2(d.Dir[0], d.Dir[1]), col, d.Intensity, d.Layers)
		if err != nil {
			t.Fatalf("%s: NewDirectionalLight: %v", what, err)
		}
		return l
	}
	l, err := NewPointLight(core.V2(d.Pos[0], d.Pos[1]), d.Range, col, d.Intensity, d.Layers, ck)
	if err != nil {
		t.Fatalf("%s: NewPointLight: %v", what, err)
	}
	return l
}

func pixFromRaw(t *testing.T, what string, raw [][]float64) []core.Color {
	t.Helper()
	out := make([]core.Color, len(raw))
	for i, v := range raw {
		if len(v) != 4 {
			t.Fatalf("%s pixel %d has %d numbers, want 4", what, i, len(v))
		}
		out[i] = core.RGBA(v[0], v[1], v[2], v[3])
	}
	return out
}

func buildApplyImage(t *testing.T, c applyCaseDef) (*Image, Scene) {
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
	sc, err := NewScene(night, ls)
	if err != nil {
		t.Fatalf("%s: NewScene: %v", c.Name, err)
	}
	return img, sc
}

func checkPixels(t *testing.T, what string, got *Image, want [][]float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: Apply returned nil", what)
	}
	if len(got.Pix) != len(want) {
		t.Fatalf("%s: %d pixels, want %d", what, len(got.Pix), len(want))
	}
	for i, w := range want {
		g := got.Pix[i]
		if math.Abs(g.R-w[0]) > goldenEps || math.Abs(g.G-w[1]) > goldenEps ||
			math.Abs(g.B-w[2]) > goldenEps || g.A != w[3] {
			t.Errorf("%s pixel %d = (%.9f,%.9f,%.9f,%.4f), want (%.9f,%.9f,%.9f,%.4f)",
				what, i, g.R, g.G, g.B, g.A, w[0], w[1], w[2], w[3])
		}
	}
}

func findApply(t *testing.T, f lightFile, name string) applyCaseDef {
	t.Helper()
	for _, c := range f.Apply {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("apply case %q missing", name)
	return applyCaseDef{}
}

// A: every frozen case reproduces its golden factor and pixels.
func TestLightGolden(t *testing.T) {
	f := loadLightFile(t)
	for _, c := range f.Factor {
		l := buildLight(t, c.Name, c.Light)
		got := l.FactorAt(core.V2(c.P[0], c.P[1]), Layer(c.Layer))
		if math.Abs(got-c.Want) > goldenEps {
			t.Errorf("%s: factor = %.9f, want %.9f", c.Name, got, c.Want)
		}
	}
	for _, c := range f.Apply {
		img, sc := buildApplyImage(t, c)
		got, err := sc.Apply(img)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		if got.W != c.W || got.H != c.H {
			t.Errorf("%s: size %dx%d, want %dx%d", c.Name, got.W, got.H, c.W, c.H)
		}
		checkPixels(t, c.Name, got, c.Want)
	}
}

// B: empty, zero, oversized, and corrupt inputs never crash; zero lights
// still show the night picture, never black; missing cookie falls back.
func TestLightEdgesNoCrash(t *testing.T) {
	night := core.RGBA(0.2, 0.2, 0.4, 1)
	sc, err := NewScene(night, nil)
	if err != nil {
		t.Fatalf("NewScene empty: %v", err)
	}
	img, err := NewImage(2, 2, core.V2(0, 0), 1)
	if err != nil {
		t.Fatalf("NewImage 2x2: %v", err)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, core.RGB(1, 1, 1), LayerWorld)
		}
	}
	got, err := sc.Apply(img)
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

	// Nil image on every path is InvalidArg.
	var nilImg *Image
	if _, err := sc.Apply(nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Apply = %v, want invalid-arg", err)
	}
	if _, err := sc.ApplyCPU(nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ApplyCPU = %v, want invalid-arg", err)
	}
	if _, err := sc.ApplyGPU(nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ApplyGPU = %v, want invalid-arg", err)
	}

	// Corrupt image is BadData.
	bad := &Image{W: 2, H: 2, Origin: core.V2(0, 0), Step: 1, Pix: make([]core.Color, 3), Layers: make([]Layer, 3)}
	if _, err := sc.Apply(bad); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("corrupt Apply = %v, want bad-data", err)
	}

	// Bad constructors report invalid-arg and build nothing.
	badBuilds := []error{}
	_, err = NewPointLight(core.V2(math.NaN(), 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	badBuilds = append(badBuilds, err)
	_, err = NewPointLight(core.V2(0, 0), 0, core.White, 1, LayersFor(LayerWorld), nil)
	badBuilds = append(badBuilds, err)
	_, err = NewPointLight(core.V2(0, 0), MaxLightRange+1, core.White, 1, LayersFor(LayerWorld), nil)
	badBuilds = append(badBuilds, err)
	_, err = NewPointLight(core.V2(0, 0), 10, core.White, MaxIntensity+1, LayersFor(LayerWorld), nil)
	badBuilds = append(badBuilds, err)
	_, err = NewPointLight(core.V2(0, 0), 10, core.RGBA(2, 0, 0, 1), 1, LayersFor(LayerWorld), nil)
	badBuilds = append(badBuilds, err)
	_, err = NewDirectionalLight(core.V2(0, 0), core.White, 1, LayersFor(LayerWorld))
	badBuilds = append(badBuilds, err)
	_, err = NewDirectionalLight(core.V2(math.Inf(1), 0), core.White, 1, LayersFor(LayerWorld))
	badBuilds = append(badBuilds, err)
	_, err = NewCookie(0, 4, make([]float64, 0))
	badBuilds = append(badBuilds, err)
	_, err = NewScene(core.Color{R: math.NaN()}, nil)
	badBuilds = append(badBuilds, err)
	many := make([]Light, MaxLights+1)
	for i := range many {
		many[i], _ = NewDirectionalLight(core.V2(0, -1), core.White, 0.1, LayersFor(LayerWorld))
	}
	_, err = NewScene(core.White, many)
	badBuilds = append(badBuilds, err)
	_, err = ParseKind("spot")
	badBuilds = append(badBuilds, err)
	for i, e := range badBuilds {
		if core.CodeOf(e) != core.CodeInvalidArg {
			t.Errorf("bad build[%d] = %v, want invalid-arg", i, e)
		}
	}
	// Cookie length and sample faults are data faults, not arg faults.
	if _, err := NewCookie(2, 2, make([]float64, 3)); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("cookie length = %v, want bad-data", err)
	}
	if _, err := NewCookie(2, 2, []float64{0, 1, 0, math.NaN()}); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("cookie sample = %v, want bad-data", err)
	}
	if _, err := NewCookie(MaxCookieDimension+1, 1, make([]float64, MaxCookieDimension+1)); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("oversize cookie = %v, want out-of-memory", err)
	}
	if _, err := NewImage(5000, 5000, core.V2(0, 0), 1); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("5000x5000 = %v, want out-of-memory", err)
	}
	if _, err := NewImage(0, 8, core.V2(0, 0), 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("0x8 = %v, want invalid-arg", err)
	}
	if _, err := NewImage(2, 2, core.V2(0, 0), 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("step 0 = %v, want invalid-arg", err)
	}
	nanPix := []core.Color{core.White, {R: math.NaN()}}
	if _, err := NewImageFromColors(2, 1, core.V2(0, 0), 1, nanPix, []Layer{LayerWorld, LayerWorld}); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("NaN pixel = %v, want bad-data", err)
	}

	// Hot path is total: NaN/Inf/out-of-range never panics, never NaN.
	pl, _ := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	for _, p := range []core.Vec2{{X: math.NaN()}, {X: math.Inf(1)}, {X: 1e308, Y: 1e308}} {
		if f := pl.FactorAt(p, LayerWorld); !finite(f) {
			t.Errorf("FactorAt(%v) = %v, want finite", p, f)
		}
		if c := LitCPU(core.White, p, LayerWorld, []Light{pl}, core.White); !finiteColor(c) {
			t.Errorf("LitCPU(%v) = %v, want finite", p, c)
		}
	}
	if f := pl.FactorAt(core.V2(0, 0), Layer(99)); f != 0 {
		t.Errorf("bad layer factor = %v, want 0", f)
	}

	// Missing cookie falls back to bare bulb (same as nil).
	fullMask := []float64{1, 1, 1, 1}
	bare, _ := NewPointLight(core.V2(1, 1), 4, core.White, 1, LayersFor(LayerWorld), nil)
	ck, _ := NewCookie(2, 2, fullMask)
	with, _ := NewPointLight(core.V2(1, 1), 4, core.White, 1, LayersFor(LayerWorld), &ck)
	p := core.V2(2, 1)
	if a, b := bare.FactorAt(p, LayerWorld), with.FactorAt(p, LayerWorld); math.Abs(a-b) > goldenEps {
		t.Errorf("full cookie factor %v vs bare %v, want equal", b, a)
	}
	// Corrupt cookie pointer falls back too, never dark.
	corrupt := bare
	corrupt.Cookie = &Cookie{W: 2, H: 2, Mask: []float64{0, 0, 0}}
	if f := corrupt.FactorAt(core.V2(1, 1), LayerWorld); f != 1 {
		t.Errorf("corrupt cookie factor = %v, want 1 (fallback)", f)
	}

	// At/Set/Clone on bad coordinates never panic.
	if _, _, ok := img.At(9, 9); ok {
		t.Error("At(9,9) = true, want false")
	}
	if img.Set(-1, 0, core.White, LayerWorld) {
		t.Error("Set(-1,0) = true, want false")
	}
	if img.Set(0, 0, core.Color{R: math.Inf(1)}, LayerWorld) {
		t.Error("Set(Inf) = true, want false")
	}
	if c := nilImg.Clone(); c != nil {
		t.Error("nil Clone != nil")
	}
	if c := bad.Clone(); c != nil {
		t.Error("corrupt Clone != nil")
	}
	if KindPoint.String() != "point" || KindDirectional.String() != "directional" {
		t.Error("Kind.String wrong")
	}
	zeroCk := Cookie{}
	if _, ok := zeroCk.At(0, 0); ok {
		t.Error("zero cookie At = true, want false")
	}
	if got := zeroCk.Sample(core.V2(0.5, 0.5)); got != 0 {
		t.Errorf("zero cookie Sample = %v, want 0", got)
	}
}

// C: both backends share one number path: CPU and GPU factors and pixels
// replay bitwise identical, inputs never mutate, outputs never alias.
func TestLightBoundaryIdentical(t *testing.T) {
	f := loadLightFile(t)
	for _, c := range f.Apply {
		imgA, scA := buildApplyImage(t, c)
		snapPix := append([]core.Color(nil), imgA.Pix...)
		snapLay := append([]Layer(nil), imgA.Layers...)
		a, err := scA.ApplyCPU(imgA)
		if err != nil {
			t.Fatalf("%s: ApplyCPU: %v", c.Name, err)
		}
		imgB, scB := buildApplyImage(t, c)
		b, err := scB.ApplyGPU(imgB)
		if err != nil {
			t.Fatalf("%s: ApplyGPU: %v", c.Name, err)
		}
		if !a.ApproxEqual(b, 0) {
			t.Fatalf("%s: CPU/GPU diverged (want bitwise identical)", c.Name)
		}
		for i := range snapPix {
			if imgA.Pix[i] != snapPix[i] || imgA.Layers[i] != snapLay[i] {
				t.Fatalf("%s: input mutated at pixel %d", c.Name, i)
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
		if a, b := l.FactorAt(p, Layer(c.Layer)), l.FactorGPU(p, Layer(c.Layer)); a != b {
			t.Fatalf("%s: factor CPU %v vs GPU %v", c.Name, a, b)
		}
		base := core.RGBA(0.7, 0.6, 0.5, 1)
		if a, b := LitCPU(base, p, Layer(c.Layer), []Light{l}, core.White),
			LitGPU(base, p, Layer(c.Layer), []Light{l}, core.White); a != b {
			t.Fatalf("%s: pixel CPU %v vs GPU %v", c.Name, a, b)
		}
	}
	// Scene mirrors agree too.
	c := findApply(t, f, "torch_person_not_bg")
	img, sc := buildApplyImage(t, c)
	p, _ := img.PosAt(0, 0)
	base, layer, _ := img.At(0, 0)
	if a, b := sc.Lit(base, p, layer), sc.LitGPU(base, p, layer); a != b {
		t.Fatalf("scene Lit vs LitGPU: %v vs %v", a, b)
	}
}

// D: multi-light cost is measured (eight torches over a 64x64 night).
func TestLightMultiPerf(t *testing.T) {
	const w, h = 64, 64
	img, err := NewImage(w, h, core.V2(0, 0), 1)
	if err != nil {
		t.Fatalf("NewImage 64x64: %v", err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, core.RGBA(0.8, 0.8, 0.8, 1), LayerWorld)
		}
	}
	var ls []Light
	for i := 0; i < 8; i++ {
		l, err := NewPointLight(core.V2(float64(i*8), float64(i*4)), 24, core.RGBA(1, 0.95, 0.8, 1), 1.2, LayersFor(LayerWorld), nil)
		if err != nil {
			t.Fatalf("NewPointLight %d: %v", i, err)
		}
		ls = append(ls, l)
	}
	sc, err := NewScene(core.RGBA(0.25, 0.25, 0.35, 1), ls)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	start := time.Now()
	got, err := sc.Apply(img)
	el := time.Since(start)
	if err != nil {
		t.Fatalf("Apply 8-light 64x64: %v", err)
	}
	if got.W != w || got.H != h {
		t.Fatalf("size %dx%d, want %dx%d", got.W, got.H, w, h)
	}
	perPx := float64(el.Nanoseconds()) / float64(w*h) / float64(len(ls))
	t.Logf("light-8x64x64: %v total (%.1f ns/px/light, %d px, %d lights)", el, perPx, w*h, len(ls))
	if el > 30*time.Second {
		t.Errorf("8-light 64x64 took %v, want < 30s (hang guard)", el)
	}
}

// E: long runs neither grow nor diverge; bad pictures stay cheap.
func TestLightLongRunStable(t *testing.T) {
	f := loadLightFile(t)
	full := findApply(t, f, "torch_cone_slit")
	mk := func() (*Image, Scene) { return buildApplyImage(t, full) }
	src0, g0 := mk()
	first, err := g0.Apply(src0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i := 0; i < 2000; i++ {
		src, gg := mk()
		got, err := gg.Apply(src)
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
	// Bad inputs stay cheap and consistent across a soak.
	bad := &Image{W: 2, H: 2, Origin: core.V2(0, 0), Step: 1, Pix: make([]core.Color, 3), Layers: make([]Layer, 3)}
	pl, _ := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	sc, _ := NewScene(core.White, []Light{pl})
	for i := 0; i < 500; i++ {
		if _, err := sc.Apply(bad); core.CodeOf(err) != core.CodeBadData {
			t.Fatalf("soak bad %d = %v, want bad-data", i, err)
		}
	}
}

// F: offscreen shapes stand in for the window (window adds the pixels):
// center beats corner, lit beats night-only, person beats background,
// slit center beats slit side, directional is uniform, alpha untouched.
func TestLightOffscreenShapes(t *testing.T) {
	f := loadLightFile(t)
	byName := func(name string) applyCaseDef {
		for _, c := range f.Apply {
			if c.Name == name {
				return c
			}
		}
		t.Fatalf("apply case %q missing", name)
		return applyCaseDef{}
	}

	// Point gradient: center pixel is brightest, corners dimmer.
	pt := byName("point_white_3x3")
	img, sc := buildApplyImage(t, pt)
	got, _ := sc.Apply(img)
	center, corner := got.Pix[4].R, got.Pix[0].R
	if !(center >= corner && center == 1 && corner < 1 && corner > 0.9) {
		t.Errorf("point gradient center=%v corner=%v, want 1=center>corner>0.9", center, corner)
	}

	// Zero lights: night dims but never blacks, alpha exact.
	nz := byName("night_only_zero_lights")
	img, sc = buildApplyImage(t, nz)
	got, _ = sc.Apply(img)
	for i, p := range got.Pix {
		if p.R != 0.2 || p.G != 0.2 || p.B != 0.4 || p.A != 1 {
			t.Errorf("night pixel %d = %v, want (0.2,0.2,0.4,1)", i, p)
		}
	}

	// Layer split: world pixel lit above night, UI pixel stays night.
	sp := byName("layer_split_world_only")
	img, sc = buildApplyImage(t, sp)
	got, _ = sc.Apply(img)
	if !(got.Pix[0].R > got.Pix[1].R && got.Pix[1].R == 0.3) {
		t.Errorf("split lit=%v unlit=%v, want lit>0.3=unlit", got.Pix[0], got.Pix[1])
	}

	// Torch slit: middle column beats the sides on the same row (green
	// channel carries it; red clamps to 1 on both).
	sl := byName("torch_cone_slit")
	img, sc = buildApplyImage(t, sl)
	got, _ = sc.Apply(img)
	if !(got.Pix[4].G > got.Pix[3].G && got.Pix[4].G > got.Pix[5].G) {
		t.Errorf("slit mid=%v left=%v right=%v, want mid brightest", got.Pix[4], got.Pix[3], got.Pix[5])
	}

	// Torch person vs background: world pixels beat fx pixels.
	tp := byName("torch_person_not_bg")
	img, sc = buildApplyImage(t, tp)
	got, _ = sc.Apply(img)
	if !(got.Pix[0].R > got.Pix[1].R && got.Pix[2].R > got.Pix[3].R) {
		t.Errorf("torch person=%v,%v bg=%v,%v, want person>bg", got.Pix[0], got.Pix[2], got.Pix[1], got.Pix[3])
	}
	if got.Pix[1].R != 0.225 {
		t.Errorf("torch bg = %v, want night-only 0.225", got.Pix[1])
	}

	// Directional over fx: every pixel equal (uniform, no gradient).
	dn := byName("directional_warm_night")
	img, sc = buildApplyImage(t, dn)
	got, _ = sc.Apply(img)
	for i := 1; i < len(got.Pix); i++ {
		if got.Pix[i] != got.Pix[0] {
			t.Errorf("directional pixel %d = %v vs %v, want uniform", i, got.Pix[i], got.Pix[0])
		}
	}

	// Colored light bends channels: red outruns green and blue.
	cl := byName("colored_red_on_white")
	img, sc = buildApplyImage(t, cl)
	got, _ = sc.Apply(img)
	p0 := got.Pix[0]
	if !(p0.R > p0.G && p0.G > p0.B-0.2 && p0.A == 1) {
		t.Errorf("colored pixel = %v, want red-dominant", p0)
	}
}
