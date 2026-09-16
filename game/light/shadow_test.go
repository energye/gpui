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

type shadowLightDef struct {
	Kind      string    `json:"kind"`
	Pos       []float64 `json:"pos"`
	Dir       []float64 `json:"dir"`
	Range     float64   `json:"range"`
	Color     []float64 `json:"color"`
	Intensity float64   `json:"intensity"`
	Layers    uint32    `json:"layers"`
}

type shadowFactorDef struct {
	Name      string         `json:"name"`
	Light     shadowLightDef `json:"light"`
	Occluders [][][2]float64 `json:"occluders"`
	Occlusion float64        `json:"occlusion"`
	Length    float64        `json:"length"`
	P         []float64      `json:"p"`
	Want      float64        `json:"want"`
}

type shadowApplyDef struct {
	Name      string           `json:"name"`
	W         int              `json:"w"`
	H         int              `json:"h"`
	OX        float64          `json:"ox"`
	OY        float64          `json:"oy"`
	Step      float64          `json:"step"`
	Src       [][]float64      `json:"src"`
	Layers    []int            `json:"layers"`
	Night     []float64        `json:"night"`
	Lights    []shadowLightDef `json:"lights"`
	Occluders [][][2]float64   `json:"occluders"`
	Occlusion float64          `json:"occlusion"`
	Length    float64          `json:"length"`
	Want      [][]float64      `json:"want"`
}

type shadowFile struct {
	Factor []shadowFactorDef `json:"factor"`
	Apply  []shadowApplyDef  `json:"apply"`
}

func loadShadowFile(t *testing.T) shadowFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "shadow_cases.json"))
	if err != nil {
		t.Fatalf("read shadow_cases.json: %v", err)
	}
	var f shadowFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode shadow_cases.json: %v", err)
	}
	if len(f.Factor) == 0 || len(f.Apply) == 0 {
		t.Fatal("shadow_cases.json misses a section")
	}
	return f
}

func buildShadowLight(t *testing.T, what string, d shadowLightDef) Light {
	t.Helper()
	col := core.RGBA(d.Color[0], d.Color[1], d.Color[2], d.Color[3])
	if d.Kind == "directional" {
		l, err := NewDirectionalLight(core.V2(d.Dir[0], d.Dir[1]), col, d.Intensity, d.Layers)
		if err != nil {
			t.Fatalf("%s: NewDirectionalLight: %v", what, err)
		}
		return l
	}
	l, err := NewPointLight(core.V2(d.Pos[0], d.Pos[1]), d.Range, col, d.Intensity, d.Layers, nil)
	if err != nil {
		t.Fatalf("%s: NewPointLight: %v", what, err)
	}
	return l
}

// buildShadowCase keeps corrupt loops as-is on purpose: the loader never
// repairs inputs, the engine must fail open instead.
func buildShadowCase(c shadowFactorDef) Shadow {
	var os []Occluder
	for _, loop := range c.Occluders {
		var pts []core.Vec2
		for _, q := range loop {
			pts = append(pts, core.V2(q[0], q[1]))
		}
		os = append(os, Occluder{Pts: pts})
	}
	return Shadow{Occluders: os, Occlusion: c.Occlusion, Length: c.Length}
}

func buildShadowApply(t *testing.T, c shadowApplyDef) (*Image, []Light, core.Color, Shadow) {
	t.Helper()
	pix := make([]core.Color, len(c.Src))
	for i, v := range c.Src {
		if len(v) != 4 {
			t.Fatalf("%s pixel %d has %d numbers, want 4", c.Name, i, len(v))
		}
		pix[i] = core.RGBA(v[0], v[1], v[2], v[3])
	}
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
		ls = append(ls, buildShadowLight(t, c.Name, ld))
	}
	night := core.RGBA(c.Night[0], c.Night[1], c.Night[2], c.Night[3])
	return img, ls, night, buildShadowCase(shadowFactorDef{Occluders: c.Occluders, Occlusion: c.Occlusion, Length: c.Length})
}

func findShadowFactor(t *testing.T, f shadowFile, name string) shadowFactorDef {
	t.Helper()
	for _, c := range f.Factor {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("factor case %q missing", name)
	return shadowFactorDef{}
}

func findShadowApply(t *testing.T, f shadowFile, name string) shadowApplyDef {
	t.Helper()
	for _, c := range f.Apply {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("apply case %q missing", name)
	return shadowApplyDef{}
}

// A: every frozen case reproduces its golden multiplier and pixels.
func TestShadowGolden(t *testing.T) {
	f := loadShadowFile(t)
	for _, c := range f.Factor {
		l := buildShadowLight(t, c.Name, c.Light)
		s := buildShadowCase(c)
		if got := s.FactorFor(l, core.V2(c.P[0], c.P[1])); math.Abs(got-c.Want) > goldenEps {
			t.Errorf("%s: factor = %.9f, want %.9f", c.Name, got, c.Want)
		}
		if blocked := s.Blocked(l, core.V2(c.P[0], c.P[1])); blocked != (c.Want < 1) {
			t.Errorf("%s: blocked = %v, want %v (want %.4f)", c.Name, blocked, c.Want < 1, c.Want)
		}
	}
	for _, c := range f.Apply {
		img, ls, night, s := buildShadowApply(t, c)
		got, err := s.Apply(img, ls, night)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		if len(got.Pix) != len(c.Want) {
			t.Fatalf("%s: %d pixels, want %d", c.Name, len(got.Pix), len(c.Want))
		}
		for i, w := range c.Want {
			g := got.Pix[i]
			if math.Abs(g.R-w[0]) > goldenEps || math.Abs(g.G-w[1]) > goldenEps ||
				math.Abs(g.B-w[2]) > goldenEps || g.A != w[3] {
				t.Errorf("%s pixel %d = (%.9f,%.9f,%.9f,%.4f), want (%.9f,%.9f,%.9f,%.4f)",
					c.Name, i, g.R, g.G, g.B, g.A, w[0], w[1], w[2], w[3])
			}
		}
	}
}

// B: empty sets, zero-length shadows, and corrupt polygons never crash;
// the open always stays lit.
func TestShadowEdgesNoCrash(t *testing.T) {
	wall, err := NewOccluder([]core.Vec2{core.V2(2, -1), core.V2(3, -1), core.V2(3, 1), core.V2(2, 1)})
	if err != nil {
		t.Fatalf("NewOccluder wall: %v", err)
	}
	pl, err := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	if err != nil {
		t.Fatalf("NewPointLight: %v", err)
	}
	night := core.RGBA(0.2, 0.2, 0.4, 1)

	// No occluders: every pixel reads the plain 6.1 light, never darker.
	open, err := NewShadow(nil, 1, 1000)
	if err != nil {
		t.Fatalf("NewShadow empty: %v", err)
	}
	img, err := NewImage(3, 1, core.V2(0, 0), 1)
	if err != nil {
		t.Fatalf("NewImage 3x1: %v", err)
	}
	for x := 0; x < 3; x++ {
		img.Set(x, 0, core.White, LayerWorld)
	}
	got, err := open.Apply(img, []Light{pl}, night)
	if err != nil {
		t.Fatalf("open Apply: %v", err)
	}
	plain, err := NewScene(night, []Light{pl})
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	ref, err := plain.Apply(img)
	if err != nil {
		t.Fatalf("plain Apply: %v", err)
	}
	if !got.ApproxEqual(ref, 0) {
		t.Fatal("open shadow differs from plain light, want identical")
	}

	// Zero-length shadow: the wall stands there but casts nothing.
	flat, err := NewShadow([]Occluder{wall}, 1, 0)
	if err != nil {
		t.Fatalf("NewShadow zero length: %v", err)
	}
	if f := flat.FactorFor(pl, core.V2(5, 0)); f != 1 {
		t.Errorf("zero-length factor = %v, want 1", f)
	}
	got, err = flat.Apply(img, []Light{pl}, night)
	if err != nil {
		t.Fatalf("zero-length Apply: %v", err)
	}
	if !got.ApproxEqual(ref, 0) {
		t.Fatal("zero-length shadow differs from plain light, want identical")
	}

	// Bad polygons fail at construction and never reach the hot path.
	badBuilds := []error{}
	_, err = NewOccluder(nil)
	badBuilds = append(badBuilds, err)
	_, err = NewOccluder([]core.Vec2{core.V2(0, 0), core.V2(1, 1)})
	badBuilds = append(badBuilds, err)
	_, err = NewOccluder([]core.Vec2{core.V2(0, 0), core.V2(math.NaN(), 0), core.V2(1, 1)})
	badBuilds = append(badBuilds, err)
	_, err = NewShadow([]Occluder{wall}, -0.5, 1000)
	badBuilds = append(badBuilds, err)
	_, err = NewShadow([]Occluder{wall}, math.NaN(), 1000)
	badBuilds = append(badBuilds, err)
	_, err = NewShadow([]Occluder{wall}, 1, -1)
	badBuilds = append(badBuilds, err)
	_, err = NewShadow([]Occluder{wall}, 1, MaxShadowLength+1)
	badBuilds = append(badBuilds, err)
	many := make([]Occluder, MaxOccluders+1)
	for i := range many {
		many[i] = wall
	}
	_, err = NewShadow(many, 1, 100)
	badBuilds = append(badBuilds, err)
	_, err = NewShadow([]Occluder{{Pts: []core.Vec2{core.V2(0, 0), core.V2(1, 0)}}}, 1, 100)
	badBuilds = append(badBuilds, err)
	for i, e := range badBuilds {
		if core.CodeOf(e) != core.CodeInvalidArg {
			t.Errorf("bad build[%d] = %v, want invalid-arg", i, e)
		}
	}
	huge := make([]core.Vec2, MaxOccluderVerts+1)
	for i := range huge {
		huge[i] = core.V2(float64(i), 0)
	}
	if _, err := NewOccluder(huge); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("oversize occluder = %v, want out-of-memory", err)
	}

	// Nil and corrupt images keep their 6.1 codes on every path.
	full, err := NewShadow([]Occluder{wall}, 1, 1000)
	if err != nil {
		t.Fatalf("NewShadow: %v", err)
	}
	var nilImg *Image
	if _, err := full.Apply(nilImg, []Light{pl}, night); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Apply = %v, want invalid-arg", err)
	}
	if _, err := full.ApplyCPU(nilImg, []Light{pl}, night); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ApplyCPU = %v, want invalid-arg", err)
	}
	if _, err := full.ApplyGPU(nilImg, []Light{pl}, night); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ApplyGPU = %v, want invalid-arg", err)
	}
	bad := &Image{W: 2, H: 2, Origin: core.V2(0, 0), Step: 1, Pix: make([]core.Color, 3), Layers: make([]Layer, 3)}
	if _, err := full.Apply(bad, []Light{pl}, night); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("corrupt Apply = %v, want bad-data", err)
	}
	if err := full.Validate(); err != nil {
		t.Errorf("Validate full = %v, want nil", err)
	}
	// Zero Shadow is valid: empty set, no darkening, no reach (no-op).
	if err := (Shadow{}).Validate(); err != nil {
		t.Errorf("Validate zero = %v, want nil (empty no-op)", err)
	}
	if err := (Shadow{Occlusion: 2}).Validate(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("Validate occlusion 2 = %v, want invalid-arg", err)
	}

	// Hot path is total: NaN/Inf geometry never panics, never NaN, and
	// reads unblocked (fail-open).
	for _, p := range []core.Vec2{{X: math.NaN()}, {X: math.Inf(1)}, {X: 1e308, Y: 1e308}} {
		if full.Blocked(pl, p) {
			t.Errorf("Blocked(%v) = true, want false", p)
		}
		if f := full.FactorFor(pl, p); f != 1 {
			t.Errorf("FactorFor(%v) = %v, want 1", p, f)
		}
		if c := full.Lit(core.White, p, LayerWorld, []Light{pl}, night); !finiteColor(c) {
			t.Errorf("Lit(%v) = %v, want finite", p, c)
		}
	}
	nanLight := pl
	nanLight.Pos = core.Vec2{X: math.NaN()}
	if full.Blocked(nanLight, core.V2(5, 0)) {
		t.Error("Blocked(NaN-pos light) = true, want false")
	}
	dl, _ := NewDirectionalLight(core.V2(1, 0), core.White, 1, LayersFor(LayerWorld))
	zeroDir := dl
	zeroDir.Dir = core.Vec2{}
	if full.Blocked(zeroDir, core.V2(5, 0)) {
		t.Error("Blocked(zero-dir light) = true, want false")
	}
	unknown := dl
	unknown.Kind = Kind(99)
	if full.Blocked(unknown, core.V2(5, 0)) {
		t.Error("Blocked(unknown kind) = true, want false")
	}
	// A struct-literal shadow with out-of-range fields clamps instead of
	// leaking NaN or negatives.
	raw := Shadow{Occluders: []Occluder{wall}, Occlusion: 5, Length: 1000}
	if f := raw.FactorFor(pl, core.V2(5, 0)); f != 0 {
		t.Errorf("raw occlusion 5 factor = %v, want 0 (clamped)", f)
	}
	rawnan := Shadow{Occluders: []Occluder{wall}, Occlusion: math.NaN(), Length: 1000}
	if f := rawnan.FactorFor(pl, core.V2(5, 0)); f != 0 {
		t.Errorf("raw occlusion NaN factor = %v, want 0 (fail-dark clamped)", f)
	}
}

// C: both backends share one number path: factors, pixels, and scenes
// replay bitwise identical, inputs never mutate, outputs never alias.
func TestShadowBoundaryIdentical(t *testing.T) {
	f := loadShadowFile(t)
	for _, c := range f.Apply {
		imgA, lsA, nightA, sA := buildShadowApply(t, c)
		snapPix := append([]core.Color(nil), imgA.Pix...)
		snapLay := append([]Layer(nil), imgA.Layers...)
		a, err := sA.ApplyCPU(imgA, lsA, nightA)
		if err != nil {
			t.Fatalf("%s: ApplyCPU: %v", c.Name, err)
		}
		imgB, lsB, nightB, sB := buildShadowApply(t, c)
		b, err := sB.ApplyGPU(imgB, lsB, nightB)
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
		l := buildShadowLight(t, c.Name, c.Light)
		s := buildShadowCase(c)
		p := core.V2(c.P[0], c.P[1])
		if x, y := s.Blocked(l, p), s.BlockedGPU(l, p); x != y {
			t.Fatalf("%s: Blocked CPU %v vs GPU %v", c.Name, x, y)
		}
		if x, y := s.FactorFor(l, p), s.FactorForGPU(l, p); x != y {
			t.Fatalf("%s: factor CPU %v vs GPU %v", c.Name, x, y)
		}
		base := core.RGBA(0.7, 0.6, 0.5, 1)
		if x, y := s.Lit(base, p, Layer(0), []Light{l}, core.White),
			s.LitGPU(base, p, Layer(0), []Light{l}, core.White); x != y {
			t.Fatalf("%s: pixel CPU %v vs GPU %v", c.Name, x, y)
		}
	}
}

// D: multi-occluder cost is measured (one torch, 32 wall squares, 64x64).
func TestShadowMultiPerf(t *testing.T) {
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
	var occs []Occluder
	for i := 0; i < 32; i++ {
		x0 := float64((i%8)*8 + 1)
		y0 := float64((i/8)*16 + 2)
		o, err := NewOccluder([]core.Vec2{
			core.V2(x0, y0), core.V2(x0+4, y0), core.V2(x0+4, y0+6), core.V2(x0, y0+6),
		})
		if err != nil {
			t.Fatalf("NewOccluder %d: %v", i, err)
		}
		occs = append(occs, o)
	}
	pl, err := NewPointLight(core.V2(32, 32), 48, core.RGBA(1, 0.95, 0.8, 1), 1.2, LayersFor(LayerWorld), nil)
	if err != nil {
		t.Fatalf("NewPointLight: %v", err)
	}
	s, err := NewShadow(occs, 1, 1000)
	if err != nil {
		t.Fatalf("NewShadow: %v", err)
	}
	night := core.RGBA(0.25, 0.25, 0.35, 1)
	start := time.Now()
	got, err := s.Apply(img, []Light{pl}, night)
	el := time.Since(start)
	if err != nil {
		t.Fatalf("Apply 32-wall 64x64: %v", err)
	}
	if got.W != w || got.H != h {
		t.Fatalf("size %dx%d, want %dx%d", got.W, got.H, w, h)
	}
	perPx := float64(el.Nanoseconds()) / float64(w*h) / float64(len(occs))
	t.Logf("shadow-32x64x64: %v total (%.1f ns/px/occluder, %d px, %d occluders)", el, perPx, w*h, len(occs))
	if el > 60*time.Second {
		t.Errorf("32-wall 64x64 took %v, want < 60s (hang guard)", el)
	}
}

// E: long runs neither grow nor diverge; bad pictures stay cheap.
func TestShadowLongRunStable(t *testing.T) {
	f := loadShadowFile(t)
	full := findShadowApply(t, f, "shadow_wall_3x1")
	mk := func() (*Image, []Light, core.Color, Shadow) { return buildShadowApply(t, full) }
	src0, ls0, n0, s0 := mk()
	first, err := s0.Apply(src0, ls0, n0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i := 0; i < 2000; i++ {
		src, ls, n, s := mk()
		got, err := s.Apply(src, ls, n)
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
	for i := 0; i < 500; i++ {
		if _, err := s0.Apply(bad, []Light{pl}, n0); core.CodeOf(err) != core.CodeBadData {
			t.Fatalf("soak bad %d = %v, want bad-data", i, err)
		}
	}
}

// F: offscreen shapes stand in for the window (window adds the pixels):
// the wall face stays lit, the lee falls to night, the open matches the
// plain torch, half shadow lands halfway, alpha untouched.
func TestShadowOffscreenShapes(t *testing.T) {
	f := loadShadowFile(t)

	// Wall row: bulb and face burn at 1, the lee sits exactly on night.
	wall := findShadowApply(t, f, "shadow_wall_3x1")
	img, ls, night, s := buildShadowApply(t, wall)
	got, err := s.Apply(img, ls, night)
	if err != nil {
		t.Fatalf("wall Apply: %v", err)
	}
	if got.Pix[0] != core.RGBA(1, 1, 1, 1) || got.Pix[1] != core.RGBA(1, 1, 1, 1) {
		t.Errorf("wall open = %v,%v, want white,white (face stays lit)", got.Pix[0], got.Pix[1])
	}
	if got.Pix[2] != core.RGBA(0.2, 0.2, 0.4, 1) {
		t.Errorf("wall lee = %v, want night (0.2,0.2,0.4,1)", got.Pix[2])
	}

	// No occluders: identical to the plain 6.1 scene, symmetric falloff.
	none := findShadowApply(t, f, "shadow_none_2x2")
	img, ls, night, s = buildShadowApply(t, none)
	got, err = s.Apply(img, ls, night)
	if err != nil {
		t.Fatalf("none Apply: %v", err)
	}
	plain, err := NewScene(night, ls)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	ref, err := plain.Apply(img)
	if err != nil {
		t.Fatalf("plain Apply: %v", err)
	}
	if !got.ApproxEqual(ref, 0) {
		t.Fatal("open shadow differs from plain light, want identical")
	}
	if got.Pix[1] != got.Pix[2] {
		t.Errorf("falloff asymmetric: %v vs %v", got.Pix[1], got.Pix[2])
	}
	if !(got.Pix[0].R > got.Pix[1].R && got.Pix[1].R > got.Pix[3].R) {
		t.Errorf("falloff not decreasing: %v %v %v", got.Pix[0], got.Pix[1], got.Pix[3])
	}

	// Half occlusion lands halfway between lit and night.
	half := findShadowApply(t, f, "shadow_half_2x1")
	img, ls, night, s = buildShadowApply(t, half)
	got, err = s.Apply(img, ls, night)
	if err != nil {
		t.Fatalf("half Apply: %v", err)
	}
	direct, _ := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	litR := direct.FactorAt(core.V2(1, 0), LayerWorld)
	wantR := 1*0.2 + 1*litR*0.5
	if math.Abs(got.Pix[1].R-wantR) > goldenEps {
		t.Errorf("half lee R = %v, want lit/2+night = %v", got.Pix[1].R, wantR)
	}
	if got.Pix[1].A != 1 {
		t.Errorf("half alpha = %v, want 1 (untouched)", got.Pix[1].A)
	}

	// Direction looks right: lee is dark, source side is lit.
	lee := findShadowFactor(t, f, "dir_lee_dark")
	src := findShadowFactor(t, f, "dir_source_side_lit")
	dl := buildShadowLight(t, "dir", lee.Light)
	sl := buildShadowCase(lee)
	if sl.FactorFor(dl, core.V2(lee.P[0], lee.P[1])) != 0 {
		t.Error("directional lee reads lit, want dark")
	}
	if sl.FactorFor(dl, core.V2(src.P[0], src.P[1])) != 1 {
		t.Error("directional source side reads dark, want lit")
	}
	_ = src
}
