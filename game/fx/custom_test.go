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

// customTolerance is frozen in custom_cases.json: goldens round-trip
// exactly, so 1e-9 is generous slack, never a hidden fudge.
const customTolerance = 1e-9

type customOneDef struct {
	Name      string      `json:"name"`
	Width     float64     `json:"width"`
	Threshold float64     `json:"threshold"`
	Amount    float64     `json:"amount"`
	Edge      float64     `json:"edge"`
	Seed      float64     `json:"seed"`
	R         float64     `json:"r"`
	G         float64     `json:"g"`
	B         float64     `json:"b"`
	W         int         `json:"w"`
	H         int         `json:"h"`
	Src       [][]float64 `json:"src"`
	Want      [][]float64 `json:"want"`
}

type customFile struct {
	Tolerance float64        `json:"tolerance"`
	Identity  []customOneDef `json:"identity"`
	Outline   []customOneDef `json:"outline"`
	Dissolve  []customOneDef `json:"dissolve"`
}

func loadCustomCases(t *testing.T) customFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "custom_cases.json"))
	if err != nil {
		t.Fatalf("read custom_cases.json: %v", err)
	}
	var f customFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode custom_cases.json: %v", err)
	}
	if len(f.Identity) == 0 || len(f.Outline) == 0 || len(f.Dissolve) == 0 {
		t.Fatal("custom_cases.json misses a section")
	}
	if f.Tolerance != customTolerance {
		t.Fatalf("tolerance = %v, want frozen %v", f.Tolerance, customTolerance)
	}
	return f
}

func mustFindCustom(t *testing.T, cases []customOneDef, name string) customOneDef {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("custom_cases.json has no case %q", name)
	return customOneDef{}
}

func buildCustom(t *testing.T, c customOneDef, kind string) Custom {
	t.Helper()
	switch kind {
	case CustomIdentity:
		return NewIdentity()
	case CustomOutline:
		h, err := NewOutline(c.Width, c.Threshold, c.R, c.G, c.B)
		if err != nil {
			t.Fatalf("%s: NewOutline: %v", c.Name, err)
		}
		return h
	case CustomDissolve:
		h, err := NewDissolve(c.Amount, c.Edge, c.Seed, c.R, c.G, c.B)
		if err != nil {
			t.Fatalf("%s: NewDissolve: %v", c.Name, err)
		}
		return h
	default:
		t.Fatalf("unknown kind %q", kind)
		return Custom{}
	}
}

func checkCustomPixels(t *testing.T, what string, got *Image, want [][]float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: Apply returned nil", what)
	}
	if len(got.Pix) != len(want) {
		t.Fatalf("%s: %d pixels, want %d", what, len(got.Pix), len(want))
	}
	for i, w := range want {
		g := got.Pix[i]
		if math.Abs(g.R-w[0]) > customTolerance || math.Abs(g.G-w[1]) > customTolerance ||
			math.Abs(g.B-w[2]) > customTolerance || g.A != w[3] {
			t.Errorf("%s pixel %d = (%.9f,%.9f,%.9f,%.4f), want (%.9f,%.9f,%.9f,%.4f)",
				what, i, g.R, g.G, g.B, g.A, w[0], w[1], w[2], w[3])
		}
	}
}

// A: every frozen case reproduces its golden pixels, and only the game
// layer is touched (no render import exists in this package by design).
func TestCustomGolden(t *testing.T) {
	f := loadCustomCases(t)
	for _, c := range f.Identity {
		h := buildCustom(t, c, CustomIdentity)
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		got, degraded, err := h.Apply(img)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		if degraded {
			t.Errorf("%s: degraded = true, want false (identity is exact)", c.Name)
		}
		if !h.IsIdentity() {
			t.Errorf("%s: IsIdentity = false, want true", c.Name)
		}
		if got.W != c.W || got.H != c.H {
			t.Errorf("%s: size %dx%d, want %dx%d", c.Name, got.W, got.H, c.W, c.H)
		}
		checkCustomPixels(t, c.Name, got, c.Want)
	}
	for _, c := range f.Outline {
		h := buildCustom(t, c, CustomOutline)
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		got, degraded, err := h.Apply(img)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		if !degraded {
			t.Errorf("%s: degraded = false, want true (CPU reference path)", c.Name)
		}
		if h.IsIdentity() {
			t.Errorf("%s: IsIdentity = true, want false", c.Name)
		}
		checkCustomPixels(t, c.Name, got, c.Want)
	}
	for _, c := range f.Dissolve {
		h := buildCustom(t, c, CustomDissolve)
		img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
		if err != nil {
			t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
		}
		got, degraded, err := h.Apply(img)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		if !degraded {
			t.Errorf("%s: degraded = false, want true (CPU reference path)", c.Name)
		}
		checkCustomPixels(t, c.Name, got, c.Want)
	}
}

// B: empty, zero, oversized, and bad-shader inputs never crash; bad paths
// report InvalidArg / BadData / NotFound / OutOfMemory / Unsupported and
// the main path still gets a drawable placeholder.
func TestCustomEdgesNoCrash(t *testing.T) {
	id := NewIdentity()
	img1, err := NewImage(1, 1)
	if err != nil {
		t.Fatalf("NewImage 1x1: %v", err)
	}
	img1.Pix[0] = core.RGB(0.5, 0.5, 0.5)

	// Nil image is InvalidArg, corrupt image BadData, on both entries.
	var nilImg *Image
	if _, _, err := id.Apply(nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Apply = %v, want invalid-arg", err)
	}
	bad := &Image{W: 2, H: 2, Pix: make([]core.Color, 3)}
	if _, _, err := id.Apply(bad); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("corrupt Apply = %v, want bad-data", err)
	}
	reg := NewRegistry()
	if _, _, err := reg.Apply(1, nilImg); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("registry nil Apply = %v, want invalid-arg", err)
	}
	if _, _, err := reg.Apply(1, bad); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("registry corrupt Apply = %v, want bad-data", err)
	}

	// Bad constructors report codes and build nothing usable.
	badBuilds := []error{}
	long := make([]byte, MaxCustomNameLen+1)
	for i := range long {
		long[i] = 'x'
	}
	_, err = NewCustom("", nil)
	badBuilds = append(badBuilds, err)
	_, err = NewCustom(string(long), map[string]float64{})
	badBuilds = append(badBuilds, err)
	_, err = NewCustom(CustomIdentity, nil)
	badBuilds = append(badBuilds, err)
	_, err = NewCustom(CustomIdentity, map[string]float64{"extra": 1})
	badBuilds = append(badBuilds, err)
	_, err = NewOutline(-1, 0.5, 1, 0, 0)
	badBuilds = append(badBuilds, err)
	_, err = NewOutline(MaxCustomOutlineWidth+1, 0.5, 1, 0, 0)
	badBuilds = append(badBuilds, err)
	_, err = NewOutline(1, 2, 1, 0, 0)
	badBuilds = append(badBuilds, err)
	_, err = NewOutline(1, 0.5, 1, 0, math.NaN())
	badBuilds = append(badBuilds, err)
	_, err = NewDissolve(-0.1, 0.1, 1, 1, 0, 0)
	badBuilds = append(badBuilds, err)
	_, err = NewDissolve(0.5, 0.6, 1, 1, 0, 0)
	badBuilds = append(badBuilds, err)
	_, err = NewDissolve(0.5, 0.1, 1.5, 1, 0, 0)
	badBuilds = append(badBuilds, err)
	_, err = NewDissolve(0.5, 0.1, -1, 1, 0, 0)
	badBuilds = append(badBuilds, err)
	for i, e := range badBuilds {
		if core.CodeOf(e) != core.CodeInvalidArg {
			t.Errorf("bad build[%d] = %v, want invalid-arg", i, e)
		}
	}
	// Unknown names are Unsupported, never a guess.
	if _, err := NewCustom("glitch-xyz", map[string]float64{}); core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("unknown name = %v, want unsupported", err)
	}
	var zero Custom
	if err := zero.Validate(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero Validate = %v, want invalid-arg", err)
	}

	// Bad shader never breaks the main path: valid src still draws a copy.
	glitch := Custom{Name: "glitch-xyz", Params: map[string]float64{}}
	holder, degraded, err := glitch.Apply(img1)
	if core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("glitch Apply = %v, want unsupported", err)
	}
	if !degraded {
		t.Error("glitch degraded = false, want true")
	}
	if holder == nil || !holder.ApproxEqual(img1, 0) {
		t.Error("glitch placeholder is not a drawable copy")
	}

	// SetParam guards: unknown keys and out-of-range values are InvalidArg
	// and leave the hook untouched.
	ol, err := NewOutline(1, 0.5, 1, 0, 0)
	if err != nil {
		t.Fatalf("NewOutline: %v", err)
	}
	before := cloneParams(ol.Params)
	if err := ol.SetParam("nope", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("SetParam unknown = %v, want invalid-arg", err)
	}
	if err := ol.SetParam("width", 99); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("SetParam width 99 = %v, want invalid-arg", err)
	}
	for k, v := range before {
		if ol.Params[k] != v {
			t.Errorf("SetParam moved %s to %v after rejection", k, ol.Params[k])
		}
	}
	if err := ol.SetParam("width", 2); err != nil || ol.Params["width"] != 2 {
		t.Errorf("SetParam width 2 = %v width = %v, want nil/2", err, ol.Params["width"])
	}
	var nilC *Custom
	if err := nilC.SetParam("width", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SetParam = %v, want invalid-arg", err)
	}

	// Registry wiring: nil table, bad handles, missing ids never panic.
	var nilReg *Registry
	if nilReg.Len() != 0 {
		t.Error("nil Len != 0")
	}
	if _, err := nilReg.Attach(id); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Attach = %v, want invalid-arg", err)
	}
	if err := nilReg.Detach(1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Detach = %v, want invalid-arg", err)
	}
	if _, _, err := nilReg.Apply(1, img1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil registry Apply = %v, want invalid-arg", err)
	}
	if _, err := reg.Attach(glitch); core.CodeOf(err) != core.CodeUnsupported {
		t.Errorf("attach glitch = %v, want unsupported", err)
	}
	if err := reg.Detach(0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("detach 0 = %v, want invalid-arg", err)
	}
	if err := reg.Detach(4242); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("detach missing = %v, want not-found", err)
	}
	holder, degraded, err = reg.Apply(4242, img1)
	if core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("apply missing = %v, want not-found", err)
	}
	if !degraded || holder == nil || !holder.ApproxEqual(img1, 0) {
		t.Error("missing-handle placeholder is not a drawable degraded copy")
	}
	if _, ok := reg.Get(4242); ok {
		t.Error("Get(missing) = true, want false")
	}

	// Full table is OutOfMemory, never a silent drop.
	full := NewRegistry()
	for i := 0; i < MaxCustomSlots; i++ {
		if _, err := full.Attach(id); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	if _, err := full.Attach(id); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("attach over budget = %v, want out-of-memory", err)
	}
}

// C: both backends share one number path here (pure math, draws nothing):
// replays are bitwise identical, inputs never mutate, outputs never alias,
// and the degraded mark always says CPU reference.
func TestCustomBoundaryIdentical(t *testing.T) {
	f := loadCustomCases(t)
	all := [][2]any{}
	for _, c := range f.Identity {
		all = append(all, [2]any{c, CustomIdentity})
	}
	for _, c := range f.Outline {
		all = append(all, [2]any{c, CustomOutline})
	}
	for _, c := range f.Dissolve {
		all = append(all, [2]any{c, CustomDissolve})
	}
	for _, pair := range all {
		c := pair[0].(customOneDef)
		kind := pair[1].(string)
		mk := func() (*Image, Custom) {
			img, err := NewImageFromColors(c.W, c.H, pixFromRaw(t, c.Name, c.Src))
			if err != nil {
				t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
			}
			return img, buildCustom(t, c, kind)
		}
		srcA, hA := mk()
		snap := append([]core.Color(nil), srcA.Pix...)
		a, degA, err := hA.Apply(srcA)
		if err != nil {
			t.Fatalf("%s: Apply: %v", c.Name, err)
		}
		srcB, hB := mk()
		b, degB, err := hB.Apply(srcB)
		if err != nil {
			t.Fatalf("%s: replay Apply: %v", c.Name, err)
		}
		if degA != degB {
			t.Fatalf("%s: degraded %v vs %v, want stable", c.Name, degA, degB)
		}
		if kind == CustomIdentity && degA {
			t.Fatalf("%s: identity degraded, want false", c.Name)
		}
		if kind != CustomIdentity && !degA {
			t.Fatalf("%s: reference path not marked degraded", c.Name)
		}
		if !a.ApproxEqual(b, 0) {
			t.Fatalf("%s: replay diverged (want bitwise identical)", c.Name)
		}
		for i := range snap {
			if srcA.Pix[i] != snap[i] {
				t.Fatalf("%s: input mutated at pixel %d", c.Name, i)
			}
		}
		a.Pix[0] = core.Magenta
		if srcA.Pix[0] == core.Magenta {
			t.Fatalf("%s: output aliases input", c.Name)
		}
		if b.Pix[0] == core.Magenta {
			t.Fatalf("%s: outputs share storage", c.Name)
		}
	}
	// Registry Get copies: mutating the copy cannot move the stored hook.
	reg := NewRegistry()
	ol, err := NewOutline(1, 0.5, 1, 0, 0)
	if err != nil {
		t.Fatalf("NewOutline: %v", err)
	}
	hid, err := reg.Attach(ol)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	got, ok := reg.Get(hid)
	if !ok {
		t.Fatal("Get attached = false")
	}
	got.Params["width"] = 7
	again, _ := reg.Get(hid)
	if again.Params["width"] != 1 {
		t.Errorf("stored hook moved to %v via Get copy", again.Params["width"])
	}
	// Registry Apply replays identical through the handle.
	ring := mustFindCustom(t, f.Outline, "outline_ring_5x5")
	img, err := NewImageFromColors(ring.W, ring.H, pixFromRaw(t, ring.Name, ring.Src))
	if err != nil {
		t.Fatalf("ring image: %v", err)
	}
	rid, err := reg.Attach(buildCustom(t, ring, CustomOutline))
	if err != nil {
		t.Fatalf("Attach ring: %v", err)
	}
	first, _, err := reg.Apply(rid, img)
	if err != nil {
		t.Fatalf("Registry.Apply: %v", err)
	}
	for i := 0; i < 50; i++ {
		re, _, err := reg.Apply(rid, img)
		if err != nil {
			t.Fatalf("replay %d: %v", i, err)
		}
		if !re.ApproxEqual(first, 0) {
			t.Fatalf("replay %d diverged", i)
		}
	}
}

// D: one full-screen hook pass is measured (1080p, synthetic load only,
// no golden: goldens stay in custom_cases.json).
func TestCustomFullscreenPerf(t *testing.T) {
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
	id := NewIdentity()
	start := time.Now()
	got, degraded, err := id.Apply(img)
	elID := time.Since(start)
	if err != nil {
		t.Fatalf("identity 1080p: %v", err)
	}
	if degraded || got.W != w || got.H != h {
		t.Fatalf("identity 1080p degraded=%v size=%dx%d", degraded, got.W, got.H)
	}
	ol, err := NewOutline(1, 0.5, 1, 0, 0)
	if err != nil {
		t.Fatalf("NewOutline: %v", err)
	}
	// Warm up once so the timed pass sees steady caches.
	if _, _, err := ol.Apply(img); err != nil {
		t.Fatalf("warmup: %v", err)
	}
	const passes = 3
	best := time.Duration(1 << 62)
	for p := 0; p < passes; p++ {
		start := time.Now()
		out, _, err := ol.Apply(img)
		el := time.Since(start)
		if err != nil {
			t.Fatalf("pass %d: %v", p, err)
		}
		if len(out.Pix) != len(img.Pix) {
			t.Fatalf("pass %d len %d vs %d", p, len(out.Pix), len(img.Pix))
		}
		if el < best {
			best = el
		}
	}
	fps := float64(time.Second) / float64(best)
	t.Logf("custom-1080p: identity in %v, outline in %v (best of %d, %.1f fps equivalent)", elID, best, passes, fps)
	if best > 30*time.Second {
		t.Errorf("1080p outline took %v, want < 30s (hang guard)", best)
	}
}

// E: long attach/detach cycles neither leak nor diverge; bad pictures stay cheap.
func TestCustomLongRunStable(t *testing.T) {
	f := loadCustomCases(t)
	ring := mustFindCustom(t, f.Outline, "outline_ring_5x5")
	mk := func() (*Image, Custom) {
		img, err := NewImageFromColors(ring.W, ring.H, pixFromRaw(t, ring.Name, ring.Src))
		if err != nil {
			t.Fatalf("NewImageFromColors: %v", err)
		}
		return img, buildCustom(t, ring, CustomOutline)
	}
	src0, h0 := mk()
	first, _, err := h0.Apply(src0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i := 0; i < 2000; i++ {
		src, h := mk()
		got, _, err := h.Apply(src)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if !got.ApproxEqual(first, 0) {
			t.Fatalf("rep %d diverged", i)
		}
	}
	// Repeated attach/detach returns to baseline with no leak.
	reg := NewRegistry()
	id := NewIdentity()
	for r := 0; r < 200; r++ {
		var ids []int
		for i := 0; i < 16; i++ {
			hid, err := reg.Attach(id)
			if err != nil {
				t.Fatalf("round %d attach %d: %v", r, i, err)
			}
			ids = append(ids, hid)
		}
		if reg.Len() != 16 {
			t.Fatalf("round %d len %d, want 16", r, reg.Len())
		}
		for _, hid := range ids {
			if err := reg.Detach(hid); err != nil {
				t.Fatalf("round %d detach %d: %v", r, hid, err)
			}
		}
		if reg.Len() != 0 {
			t.Fatalf("round %d len %d after detach, want 0", r, reg.Len())
		}
	}
	// Bad inputs stay cheap and consistent across a soak.
	bad := &Image{W: 2, H: 2, Pix: make([]core.Color, 3)}
	for i := 0; i < 500; i++ {
		if _, _, err := id.Apply(bad); core.CodeOf(err) != core.CodeBadData {
			t.Fatalf("soak bad %d = %v, want bad-data", i, err)
		}
	}
}

// F: offscreen shapes stand in for the window (window adds the pixels):
// outline rings the opaque block in red, width 0 copies, dissolve empties
// at amount 1, keeps at amount 0, and mixes keep/edge/gone in between.
func TestCustomOffscreenShapes(t *testing.T) {
	f := loadCustomCases(t)
	ring := mustFindCustom(t, f.Outline, "outline_ring_5x5")
	img, err := NewImageFromColors(ring.W, ring.H, pixFromRaw(t, ring.Name, ring.Src))
	if err != nil {
		t.Fatalf("ring image: %v", err)
	}
	got, _, err := buildCustom(t, ring, CustomOutline).Apply(img)
	if err != nil {
		t.Fatalf("ring Apply: %v", err)
	}
	// Corners (far from the block) turn outline red; interior keeps src.
	for _, i := range []int{0, 4, 20, 24} {
		if got.Pix[i] != (core.RGBA(1, 0, 0, 1)) {
			t.Errorf("ring corner %d = %v, want red outline", i, got.Pix[i])
		}
	}
	if got.Pix[12] != (core.RGBA(0.2, 0.4, 0.8, 1)) {
		t.Errorf("ring center = %v, want kept src", got.Pix[12])
	}
	// Width 0 is an exact copy.
	w0 := mustFindCustom(t, f.Outline, "outline_w0_copy_5x5")
	img0, _ := NewImageFromColors(w0.W, w0.H, pixFromRaw(t, w0.Name, w0.Src))
	got0, _, err := buildCustom(t, w0, CustomOutline).Apply(img0)
	if err != nil {
		t.Fatalf("w0 Apply: %v", err)
	}
	for i, s := range pixFromRaw(t, w0.Name, w0.Src) {
		if !got0.Pix[i].ApproxEqual(s, customTolerance) {
			t.Errorf("w0 pixel %d moved: %v vs %v", i, got0.Pix[i], s)
		}
	}
	// Dissolve amount 1 empties every pixel; amount 0 keeps every pixel.
	empty := mustFindCustom(t, f.Dissolve, "dissolve_empty_6x4")
	emptyImg, _ := NewImageFromColors(empty.W, empty.H, pixFromRaw(t, empty.Name, empty.Src))
	emptyGot, _, err := buildCustom(t, empty, CustomDissolve).Apply(emptyImg)
	if err != nil {
		t.Fatalf("empty Apply: %v", err)
	}
	for i, p := range emptyGot.Pix {
		if p.A != 0 {
			t.Errorf("empty pixel %d alpha = %v, want 0", i, p.A)
		}
	}
	full := mustFindCustom(t, f.Dissolve, "dissolve_full_6x4")
	fullSrc := pixFromRaw(t, full.Name, full.Src)
	fullImg, _ := NewImageFromColors(full.W, full.H, fullSrc)
	fullGot, _, err := buildCustom(t, full, CustomDissolve).Apply(fullImg)
	if err != nil {
		t.Fatalf("full Apply: %v", err)
	}
	for i, s := range fullSrc {
		if !fullGot.Pix[i].ApproxEqual(s, customTolerance) {
			t.Errorf("full pixel %d moved: %v vs %v", i, fullGot.Pix[i], s)
		}
	}
	// Mid amount mixes all three fates.
	mid := mustFindCustom(t, f.Dissolve, "dissolve_mid_6x4")
	midSrc := pixFromRaw(t, mid.Name, mid.Src)
	midImg, _ := NewImageFromColors(mid.W, mid.H, midSrc)
	midGot, _, err := buildCustom(t, mid, CustomDissolve).Apply(midImg)
	if err != nil {
		t.Fatalf("mid Apply: %v", err)
	}
	kept, edged, gone := 0, 0, 0
	for i, p := range midGot.Pix {
		switch {
		case p.A == 0:
			gone++
		case p == (core.RGBA(1, 0.5, 0, 1)):
			edged++
		case p.ApproxEqual(midSrc[i], customTolerance):
			kept++
		default:
			t.Errorf("mid pixel %d = %v, want kept/edge/gone", i, p)
		}
	}
	if kept == 0 || edged == 0 || gone == 0 {
		t.Errorf("mid mix kept=%d edge=%d gone=%d, want all three fates", kept, edged, gone)
	}
	t.Logf("dissolve mid: kept=%d edge=%d gone=%d", kept, edged, gone)
}
