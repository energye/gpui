package fx

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

type bloomCaseDef struct {
	Name      string      `json:"name"`
	Threshold float64     `json:"threshold"`
	Radius    int         `json:"radius"`
	Intensity float64     `json:"intensity"`
	W         int         `json:"w"`
	H         int         `json:"h"`
	Src       [][]float64 `json:"src"`
	Want      [][]float64 `json:"want"`
}

type vigCaseDef struct {
	Name     string      `json:"name"`
	CX       float64     `json:"cx"`
	CY       float64     `json:"cy"`
	Inner    float64     `json:"inner"`
	Outer    float64     `json:"outer"`
	Strength float64     `json:"strength"`
	W        int         `json:"w"`
	H        int         `json:"h"`
	Src      [][]float64 `json:"src"`
	Want     [][]float64 `json:"want"`
}

type lutCaseDef struct {
	Name string      `json:"name"`
	R    []float64   `json:"r"`
	G    []float64   `json:"g"`
	B    []float64   `json:"b"`
	W    int         `json:"w"`
	H    int         `json:"h"`
	Src  [][]float64 `json:"src"`
	Want [][]float64 `json:"want"`
}

type tmCaseDef struct {
	Name     string      `json:"name"`
	Mode     string      `json:"mode"`
	Exposure float64     `json:"exposure"`
	W        int         `json:"w"`
	H        int         `json:"h"`
	Src      [][]float64 `json:"src"`
	Want     [][]float64 `json:"want"`
}

type gradeCaseDef struct {
	Name      string      `json:"name"`
	Threshold float64     `json:"threshold"`
	Radius    int         `json:"radius"`
	Intensity float64     `json:"intensity"`
	CX        float64     `json:"cx"`
	CY        float64     `json:"cy"`
	Inner     float64     `json:"inner"`
	Outer     float64     `json:"outer"`
	Strength  float64     `json:"strength"`
	LUTR      []float64   `json:"lut_r"`
	LUTG      []float64   `json:"lut_g"`
	LUTB      []float64   `json:"lut_b"`
	HasLUT    bool        `json:"has_lut"`
	Mode      string      `json:"mode"`
	Exposure  float64     `json:"exposure"`
	W         int         `json:"w"`
	H         int         `json:"h"`
	Src       [][]float64 `json:"src"`
	Want      [][]float64 `json:"want"`
}

type gradeFile struct {
	Bloom    []bloomCaseDef `json:"bloom"`
	Vignette []vigCaseDef   `json:"vignette"`
	LUT      []lutCaseDef   `json:"lut"`
	Tonemap  []tmCaseDef    `json:"tonemap"`
	Grade    []gradeCaseDef `json:"grade"`
}

func loadGradeFile(t *testing.T) gradeFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "bloom_lut_cases.json"))
	if err != nil {
		t.Fatalf("read bloom_lut_cases.json: %v", err)
	}
	var f gradeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode bloom_lut_cases.json: %v", err)
	}
	if len(f.Bloom) == 0 || len(f.Vignette) == 0 || len(f.LUT) == 0 || len(f.Tonemap) == 0 || len(f.Grade) == 0 {
		t.Fatal("bloom_lut_cases.json misses a section")
	}
	return f
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

func buildGrade(t *testing.T, c gradeCaseDef) Grade {
	t.Helper()
	b, err := NewBloom(c.Threshold, c.Radius, c.Intensity)
	if err != nil {
		t.Fatalf("%s: NewBloom: %v", c.Name, err)
	}
	v, err := NewVignette(core.V2(c.CX, c.CY), c.Inner, c.Outer, c.Strength)
	if err != nil {
		t.Fatalf("%s: NewVignette: %v", c.Name, err)
	}
	tm, err := ParseTonemap(c.Mode)
	if err != nil {
		t.Fatalf("%s: ParseTonemap: %v", c.Name, err)
	}
	var lut *LUT
	if c.HasLUT {
		l, err := NewLUT(c.LUTR, c.LUTG, c.LUTB)
		if err != nil {
			t.Fatalf("%s: NewLUT: %v", c.Name, err)
		}
		lut = &l
	}
	g, err := NewGrade(b, v, lut, tm, c.Exposure)
	if err != nil {
		t.Fatalf("%s: NewGrade: %v", c.Name, err)
	}
	return g
}

// A: every frozen case reproduces its golden pixels (eps 1e-9, alpha exact).
func TestBloomLUTGolden(t *testing.T) {
	f := loadGradeFile(t)
	for _, c := range f.Bloom {
		b, err := NewBloom(c.Threshold, c.Radius, c.Intensity)
		if err != nil {
			t.Fatalf("%s: NewBloom: %v", c.Name, err)
		}
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		got, err := b.Apply(img)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		if got.W != c.W || got.H != c.H {
			t.Errorf("%s: size %dx%d, want %dx%d", c.Name, got.W, got.H, c.W, c.H)
		}
		checkPixels(t, c.Name, got, c.Want)
	}
	for _, c := range f.Vignette {
		v, err := NewVignette(core.V2(c.CX, c.CY), c.Inner, c.Outer, c.Strength)
		if err != nil {
			t.Fatalf("%s: NewVignette: %v", c.Name, err)
		}
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		got, err := v.Apply(img)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		checkPixels(t, c.Name, got, c.Want)
	}
	for _, c := range f.LUT {
		l, err := NewLUT(c.R, c.G, c.B)
		if err != nil {
			t.Fatalf("%s: NewLUT: %v", c.Name, err)
		}
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		got, err := l.Apply(img)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		checkPixels(t, c.Name, got, c.Want)
	}
	for _, c := range f.Tonemap {
		tm, err := ParseTonemap(c.Mode)
		if err != nil {
			t.Fatalf("%s: ParseTonemap: %v", c.Name, err)
		}
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		got, err := ApplyTonemap(img, tm, c.Exposure)
		if err != nil {
			t.Fatalf("%s: ApplyTonemap: %v", c.Name, err)
		}
		checkPixels(t, c.Name, got, c.Want)
	}
	for _, c := range f.Grade {
		g := buildGrade(t, c)
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		got, err := g.Apply(img)
		if err != nil {
			t.Fatalf("%s: Grade.Apply: %v", c.Name, err)
		}
		checkPixels(t, c.Name, got, c.Want)
		if gotStages := g.Stages(); len(gotStages) != 4 ||
			gotStages[0] != "bloom" || gotStages[1] != "vignette" ||
			gotStages[2] != "lut" || gotStages[3] != "tonemap" {
			t.Errorf("%s: Stages = %v, want [bloom vignette lut tonemap]", c.Name, gotStages)
		}
	}
}

// B: empty, zero, oversized, and corrupt inputs never crash; bad paths
// report InvalidArg / BadData / OutOfMemory and allocate nothing usable.
func TestBloomLUTEdgesNoCrash(t *testing.T) {
	okBloom, _ := NewBloom(0.5, 1, 1)
	okVig, _ := NewVignette(core.V2(0.5, 0.5), 0.2, 0.9, 0.5)
	okLUT, _ := NewIdentityLUT(4)
	img1, err := NewImage(1, 1)
	if err != nil {
		t.Fatalf("NewImage 1x1: %v", err)
	}
	img1.Pix[0] = core.RGB(0.5, 0.5, 0.5)

	// Nil image on every stage is InvalidArg.
	var nilImg *Image
	if _, err := okBloom.Apply(nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bloom nil Apply = %v, want invalid-arg", err)
	}
	if _, err := okVig.Apply(nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("vignette nil Apply = %v, want invalid-arg", err)
	}
	if _, err := okLUT.Apply(nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("lut nil Apply = %v, want invalid-arg", err)
	}
	if _, err := ApplyTonemap(nilImg, TonemapReinhard, 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("tonemap nil Apply = %v, want invalid-arg", err)
	}
	g, err := NewGrade(okBloom, okVig, nil, TonemapNone, 1)
	if err != nil {
		t.Fatalf("NewGrade identity: %v", err)
	}
	if _, err := g.Apply(nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("grade nil Apply = %v, want invalid-arg", err)
	}

	// Corrupt image (length mismatch) is BadData on every stage.
	bad := &Image{W: 2, H: 2, Pix: make([]core.Color, 3)}
	if _, err := okBloom.Apply(bad); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("bloom corrupt = %v, want bad-data", err)
	}
	if _, err := okVig.Apply(bad); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("vignette corrupt = %v, want bad-data", err)
	}

	// Empty tables never crash: zero LUT is InvalidArg at build and at use.
	var zeroLUT LUT
	if err := zeroLUT.Validate(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero LUT Validate = %v, want invalid-arg", err)
	}
	if _, err := zeroLUT.Apply(img1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero LUT Apply = %v, want invalid-arg", err)
	}

	// Bad constructors report invalid-arg and build nothing.
	badBuilds := []error{}
	_, err = NewBloom(math.NaN(), 1, 1)
	badBuilds = append(badBuilds, err)
	_, err = NewBloom(-0.1, 1, 1)
	badBuilds = append(badBuilds, err)
	_, err = NewBloom(0.5, -1, 1)
	badBuilds = append(badBuilds, err)
	_, err = NewBloom(0.5, MaxBloomRadius+1, 1)
	badBuilds = append(badBuilds, err)
	_, err = NewBloom(0.5, 1, math.Inf(1))
	badBuilds = append(badBuilds, err)
	_, err = NewVignette(core.V2(math.NaN(), 0.5), 0.2, 0.9, 0.5)
	badBuilds = append(badBuilds, err)
	_, err = NewVignette(core.V2(0.5, 0.5), 0.9, 0.2, 0.5)
	badBuilds = append(badBuilds, err)
	_, err = NewVignette(core.V2(0.5, 0.5), 0.2, 0.9, 2)
	badBuilds = append(badBuilds, err)
	_, err = NewLUT([]float64{0, 1}, []float64{0}, []float64{0, 1})
	badBuilds = append(badBuilds, err)
	_, err = NewLUT([]float64{0}, []float64{0}, []float64{0})
	badBuilds = append(badBuilds, err)
	_, err = NewLUT([]float64{0, math.NaN()}, []float64{0, 1}, []float64{0, 1})
	badBuilds = append(badBuilds, err)
	_, err = NewIdentityLUT(1)
	badBuilds = append(badBuilds, err)
	_, err = ParseTonemap("filmic-plus")
	badBuilds = append(badBuilds, err)
	_, err = NewGrade(okBloom, okVig, nil, Tonemap(99), 1)
	badBuilds = append(badBuilds, err)
	_, err = NewGrade(okBloom, okVig, nil, TonemapReinhard, math.NaN())
	badBuilds = append(badBuilds, err)
	for i, e := range badBuilds {
		if core.CodeOf(e) != core.CodeInvalidArg {
			t.Errorf("bad build[%d] = %v, want invalid-arg", i, e)
		}
	}
	if _, err := ApplyTonemap(img1, TonemapReinhard, -1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative exposure = %v, want invalid-arg", err)
	}

	// Oversized pictures are OutOfMemory, never an allocation panic.
	if _, err := NewImage(5000, 5000); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("5000x5000 = %v, want out-of-memory", err)
	}
	if _, err := NewImage(0, 8); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("0x8 = %v, want invalid-arg", err)
	}
	if _, err := NewImageFromColors(2, 2, make([]core.Color, 3)); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("length mismatch = %v, want bad-data", err)
	}
	nanPix := []core.Color{core.RGB(0.1, 0.1, 0.1), {R: math.NaN()}}
	if _, err := NewImageFromColors(2, 1, nanPix); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("NaN pixel = %v, want bad-data", err)
	}

	// At/Set/Clone on bad coordinates never panic.
	if _, ok := img1.At(9, 9); ok {
		t.Error("At(9,9) = true, want false")
	}
	if img1.Set(-1, 0, core.White) {
		t.Error("Set(-1,0) = true, want false")
	}
	if img1.Set(0, 0, core.Color{R: math.Inf(1)}) {
		t.Error("Set(Inf) = true, want false")
	}
	if c := nilImg.Clone(); c != nil {
		t.Error("nil Clone != nil")
	}
	if c := bad.Clone(); c != nil {
		t.Error("corrupt Clone != nil")
	}
	if Tonemap(99).String() != "none" {
		t.Error("bad Tonemap String != none")
	}
	if got := okLUT.Sample(7, 0.5); got != 0 {
		t.Errorf("bad channel Sample = %v, want 0", got)
	}
}

// C: both backends share one number path here (pure math, draws nothing):
// replays are bitwise identical, inputs never mutate, outputs never alias.
func TestBloomLUTBoundaryIdentical(t *testing.T) {
	f := loadGradeFile(t)
	build := func(c gradeCaseDef) (*Image, Grade) {
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		return img, buildGrade(t, c)
	}
	for _, c := range f.Grade {
		srcA, gA := build(c)
		snap := append([]core.Color(nil), srcA.Pix...)
		a, err := gA.Apply(srcA)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		srcB, gB := build(c)
		b, err := gB.Apply(srcB)
		if err != nil {
			t.Fatalf("%s: replay Apply: %v", c.Name, err)
		}
		if !a.ApproxEqual(b, 0) {
			t.Fatalf("%s: replay diverged (want bitwise identical)", c.Name)
		}
		for i := range snap {
			if srcA.Pix[i] != snap[i] {
				t.Fatalf("%s: input mutated at pixel %d", c.Name, i)
			}
		}
		// Output is a fresh copy: writing it cannot alias input or replay.
		a.Pix[0] = core.Magenta
		if srcA.Pix[0] == core.Magenta {
			t.Fatalf("%s: output aliases input", c.Name)
		}
		if b.Pix[0] == core.Magenta {
			t.Fatalf("%s: outputs share storage", c.Name)
		}
	}
	// Constructor copies: mutating the caller's slices cannot move the LUT.
	r := []float64{0, 1}
	gg := []float64{0, 1}
	bb := []float64{0, 1}
	l, err := NewLUT(r, gg, bb)
	if err != nil {
		t.Fatalf("NewLUT: %v", err)
	}
	r[0], gg[1], bb[0] = 0.9, 0.1, 0.9
	probe, err := NewImageFromColors(1, 1, []core.Color{core.RGB(0.25, 0.75, 0.5)})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	got, err := l.Apply(probe)
	if err != nil {
		t.Fatalf("LUT Apply: %v", err)
	}
	if math.Abs(got.Pix[0].R-0.25) > goldenEps || math.Abs(got.Pix[0].G-0.75) > goldenEps {
		t.Errorf("LUT aliases caller slices: got %v", got.Pix[0])
	}
	// Grade copies its LUT too.
	g, err := NewGrade(Bloom{}, Vignette{Center: core.V2(0.5, 0.5), Inner: 0.2, Outer: 0.9}, &l, TonemapNone, 1)
	if err != nil {
		t.Fatalf("NewGrade: %v", err)
	}
	l.R[0] = 0.9
	got2, err := g.Apply(probe)
	if err != nil {
		t.Fatalf("Grade Apply: %v", err)
	}
	if math.Abs(got2.Pix[0].R-0.25) > goldenEps {
		t.Errorf("Grade aliases caller LUT: got %v", got2.Pix[0])
	}
}

// D: full-screen grade cost is measured (1080p through the whole chain).
func TestBloomLUTFullscreenPerf(t *testing.T) {
	const w, h = 1920, 1080
	img, err := NewImage(w, h)
	if err != nil {
		t.Fatalf("NewImage 1080p: %v", err)
	}
	for y := 0; y < h; y++ {
		v := float64(y) / float64(h)
		for x := 0; x < w; x++ {
			u := float64(x) / float64(w)
			img.Pix[y*w+x] = core.RGBA(u, v, 0.5*(u+v), 1)
		}
	}
	b, _ := NewBloom(0.6, 1, 0.8)
	v, _ := NewVignette(core.V2(0.5, 0.5), 0.2, 0.9, 0.45)
	half := []float64{0, 0.5}
	l, _ := NewLUT(half, half, half)
	g, err := NewGrade(b, v, &l, TonemapReinhard, 1)
	if err != nil {
		t.Fatalf("NewGrade: %v", err)
	}
	start := time.Now()
	got, err := g.Apply(img)
	el := time.Since(start)
	if err != nil {
		t.Fatalf("Apply 1080p: %v", err)
	}
	if got.W != w || got.H != h {
		t.Fatalf("size %dx%d, want %dx%d", got.W, got.H, w, h)
	}
	fps := 1 / el.Seconds()
	t.Logf("grade-1080p: full chain in %v (%.2f fps equivalent)", el, fps)
	if el > 30*time.Second {
		t.Errorf("1080p grade took %v, want < 30s (hang guard)", el)
	}
}

// E: long runs neither grow nor diverge; bad pictures stay cheap.
func TestBloomLUTLongRunStable(t *testing.T) {
	f := loadGradeFile(t)
	var full gradeCaseDef
	for _, c := range f.Grade {
		if c.Name == "grade_full_look" {
			full = c
		}
	}
	if full.Name == "" {
		t.Fatal("grade_full_look missing")
	}
	mk := func() (*Image, Grade) {
		img, err := NewImageFromColors(full.W, full.H, pixFromRaw(t, full.Name, full.Src))
		if err != nil {
			t.Fatalf("NewImageFromColors: %v", err)
		}
		return img, buildGrade(t, full)
	}
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
	bad := &Image{W: 2, H: 2, Pix: make([]core.Color, 3)}
	b, _ := NewBloom(0.5, 1, 1)
	for i := 0; i < 500; i++ {
		if _, err := b.Apply(bad); core.CodeOf(err) != core.CodeBadData {
			t.Fatalf("soak bad %d = %v, want bad-data", i, err)
		}
	}
}

// F: offscreen shapes stand in for the window (window adds the pixels):
// glow spreads, corners fall below edges below center, tables bend, maps
// climb, alpha rides through untouched.
func TestBloomLUTOffscreenShapes(t *testing.T) {
	f := loadGradeFile(t)
	byName := func(name string) gradeCaseDef {
		for _, c := range f.Grade {
			if c.Name == name {
				return c
			}
		}
		t.Fatalf("grade case %q missing", name)
		return gradeCaseDef{}
	}
	_ = byName

	// Bloom spreads: 3x3 single white blooms every neighbour above black.
	for _, c := range f.Bloom {
		if c.Name != "bloom_r1_spread_3x3" {
			continue
		}
		b, _ := NewBloom(c.Threshold, c.Radius, c.Intensity)
		img, _ := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		got, _ := b.Apply(img)
		center := got.Pix[4].R
		if center != 1 {
			t.Errorf("spread center = %v, want 1 (clamped)", center)
		}
		for i, p := range got.Pix {
			if i == 4 {
				continue
			}
			if p.R <= 0 || p.R >= 1 {
				t.Errorf("spread neighbour %d = %v, want in (0,1)", i, p.R)
			}
			if p.A != 1 {
				t.Errorf("spread neighbour %d alpha = %v, want 1", i, p.A)
			}
		}
	}
	// Vignette ordering: center 1, edges below 1, corners lowest.
	for _, c := range f.Vignette {
		if c.Name != "vignette_center_kept" {
			continue
		}
		v, _ := NewVignette(core.V2(c.CX, c.CY), c.Inner, c.Outer, c.Strength)
		img, _ := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		got, _ := v.Apply(img)
		center, edge, corner := got.Pix[4].R, got.Pix[1].R, got.Pix[0].R
		if center != 1 {
			t.Errorf("vignette center = %v, want 1", center)
		}
		if !(corner < edge && edge < center) {
			t.Errorf("vignette order corner=%v edge=%v center=%v, want corner<edge<center", corner, edge, center)
		}
	}
	// Identity LUT keeps pixels; halve LUT halves; tonemap climbs.
	for _, c := range f.LUT {
		if c.Name == "lut_identity4" {
			l, _ := NewLUT(c.R, c.G, c.B)
			img, _ := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
			got, _ := l.Apply(img)
			for i, s := range pixFromRaw(t, c.Name, c.Src) {
				if !got.Pix[i].ApproxEqual(s, goldenEps) {
					t.Errorf("identity pixel %d moved: %v vs %v", i, got.Pix[i], s)
				}
			}
		}
	}
	for _, c := range f.Tonemap {
		if c.Name == "tonemap_reinhard_mid" {
			tm, _ := ParseTonemap(c.Mode)
			img, _ := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
			got, _ := ApplyTonemap(img, tm, c.Exposure)
			if !(got.Pix[0].R == 0.5 && got.Pix[1].R == 0 && math.Abs(got.Pix[2].R-1.0/3) < goldenEps) {
				t.Errorf("reinhard shape wrong: %v", got.Pix)
			}
			if got.Pix[2].A != 0.7 {
				t.Errorf("reinhard alpha = %v, want 0.7", got.Pix[2].A)
			}
		}
	}
	// Grade look: corners darker than center, alpha preserved.
	gc := byName("grade_full_look")
	g := buildGrade(t, gc)
	img, _ := NewImageFromColors(gc.W, gc.H, pixFromRaw(t, gc.Name, gc.Src))
	got, _ := g.Apply(img)
	center, corner := got.Pix[4].R, got.Pix[0].R
	if !(corner < center) {
		t.Errorf("grade corner=%v center=%v, want corner<center", corner, center)
	}
	// Identity grade is a no-op.
	gi := byName("grade_identity")
	ig := buildGrade(t, gi)
	srcPix := pixFromRaw(t, gi.Name, gi.Src)
	img2, _ := NewImageFromColors(gi.W, gi.H, srcPix)
	got2, _ := ig.Apply(img2)
	for i, s := range srcPix {
		if !got2.Pix[i].ApproxEqual(s, goldenEps) || got2.Pix[i].A != s.A {
			t.Errorf("identity grade pixel %d moved: %v vs %v", i, got2.Pix[i], s)
		}
	}
}
