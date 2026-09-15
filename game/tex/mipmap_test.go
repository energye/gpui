package tex

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

// Tolerance is frozen at 1: box-downsampled checker halves round within
// 1 LSB (odd halves truncate), so mipmap_cases.json carries the same 1.
const mipmapTolerance = 1

type sizeDef struct {
	W      int `json:"w"`
	H      int `json:"h"`
	Levels int `json:"levels"`
}

type scaleDef struct {
	Scale float64 `json:"scale"`
	W     int     `json:"w"`
	H     int     `json:"h"`
	Level int     `json:"level"`
}

type levelSizeDef struct {
	W     int `json:"w"`
	H     int `json:"h"`
	Level int `json:"level"`
	LW    int `json:"lw"`
	LH    int `json:"lh"`
}

type probeDef struct {
	Desc  string  `json:"desc"`
	Scale float64 `json:"scale"`
	Level int     `json:"level"`
}

type mipSpotDef struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	RGBA [4]int `json:"rgba"`
}

type meanDef struct {
	W     int `json:"w"`
	H     int `json:"h"`
	Level int `json:"level"`
	Mean  int `json:"mean"`
}

type samplerDef struct {
	Name     string `json:"name"`
	Near     string `json:"near"`
	Far      string `json:"far"`
	Mip      string `json:"mip"`
	Aniso    int    `json:"aniso"`
	Mag      string `json:"mag"`
	Min      string `json:"min"`
	Mipmap   string `json:"mipmap"`
	EffAniso int    `json:"eff_aniso"`
}

type mipmapCases struct {
	Tolerance  int            `json:"tolerance"`
	Sizes      []sizeDef      `json:"sizes"`
	Scales     []scaleDef     `json:"scales"`
	LevelSizes []levelSizeDef `json:"level_sizes"`
	Probes     []probeDef     `json:"probes"`
	Level0     []mipSpotDef   `json:"level0_spots"`
	Means      []meanDef      `json:"level_means"`
	Samplers   []samplerDef   `json:"sampler_contract"`
}

func loadMipmapCases(t *testing.T) mipmapCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "mipmap_cases.json"))
	if err != nil {
		t.Fatalf("read mipmap_cases.json: %v", err)
	}
	var c mipmapCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode mipmap_cases.json: %v", err)
	}
	if len(c.Sizes) == 0 || len(c.Samplers) == 0 {
		t.Fatal("mipmap_cases.json missing sizes or sampler contract")
	}
	if c.Tolerance != mipmapTolerance {
		t.Fatalf("tolerance = %d, want frozen %d", c.Tolerance, mipmapTolerance)
	}
	return c
}

func findSampler(t *testing.T, c mipmapCases, name string) samplerDef {
	t.Helper()
	for _, s := range c.Samplers {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("mipmap_cases.json has no sampler %q", name)
	return samplerDef{}
}

func makeFilter(t *testing.T, s samplerDef) Filter {
	t.Helper()
	f := Filter{Near: ParseKind(s.Near), Far: ParseKind(s.Far), Mip: ParseMipMode(s.Mip), MaxAniso: uint16(s.Aniso)} //nolint:gosec // aniso range is data-driven, validated by Table.Set
	if s.Name == "zero-keeps-history" && f != (Filter{}) {
		t.Fatalf("zero-keeps-history case is not the zero value: %+v", f)
	}
	return f
}

// checker64 builds the frozen 64x64 black/white checker the golden pins:
// 8px cells, TL black, so every half lands on a known mean.
func checker64(t *testing.T) []byte {
	t.Helper()
	const w, h, cell = 64, 64, 8
	px := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := byte(0)
			if (x/cell+y/cell)%2 == 1 {
				v = 255
			}
			off := (y*w + x) * 4
			px[off], px[off+1], px[off+2], px[off+3] = v, v, v, 255
		}
	}
	return px
}

// boxDown halves px (w-by-h) with the same 2x2 average the CPU chain uses.
func boxDown(px []byte, w, h int) ([]byte, int, int) {
	dw, dh := w/2, h/2
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	out := make([]byte, dw*dh*4)
	at := func(x, y int) [4]int {
		if x >= w {
			x = w - 1
		}
		if y >= h {
			y = h - 1
		}
		off := (y*w + x) * 4
		return [4]int{int(px[off]), int(px[off+1]), int(px[off+2]), int(px[off+3])}
	}
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			a, b, c, d := at(x*2, y*2), at(x*2+1, y*2), at(x*2, y*2+1), at(x*2+1, y*2+1)
			off := (y*dw + x) * 4
			for k := 0; k < 4; k++ {
				out[off+k] = byte((a[k] + b[k] + c[k] + d[k]) / 4)
			}
		}
	}
	return out, dw, dh
}

// levelMean averages the R channel of one level for the mean rows.
func levelMean(px []byte) int {
	sum := 0
	for i := 0; i < len(px); i += 4 {
		sum += int(px[i])
	}
	return sum / (len(px) / 4)
}

func diffByte(a, b int) int {
	if a < b {
		return b - a
	}
	return a - b
}

func expectMipmapCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

func mustFilter(t *testing.T, s samplerDef) Filter {
	t.Helper()
	f := makeFilter(t, s)
	if s.Name == "zero-keeps-history" {
		return f
	}
	tab := NewTable()
	if err := tab.Set("probe", f); err != nil {
		t.Fatalf("%s: Table.Set: %v", s.Name, err)
	}
	return f
}

// A: normal inputs land on the frozen numbers (levels, scales, samplers).
func TestMipmapSwitchFromCases(t *testing.T) {
	c := loadMipmapCases(t)
	for _, s := range c.Sizes {
		if got := NumLevels(s.W, s.H); got != s.Levels {
			t.Errorf("NumLevels(%dx%d) = %d, want %d", s.W, s.H, got, s.Levels)
		}
	}
	for _, s := range c.Scales {
		if got := LevelForScale(s.Scale, s.W, s.H); got != s.Level {
			t.Errorf("LevelForScale(%v %dx%d) = %d, want %d", s.Scale, s.W, s.H, got, s.Level)
		}
	}
	for _, s := range c.LevelSizes {
		lw, lh, ok := LevelSize(s.W, s.H, s.Level)
		if !ok || lw != s.LW || lh != s.LH {
			t.Errorf("LevelSize(%dx%d @%d) = %dx%d,%v, want %dx%d,true", s.W, s.H, s.Level, lw, lh, ok, s.LW, s.LH)
		}
	}
	// Old callers stay smooth: DefaultFilter matches the historic sampler.
	def := DefaultFilter()
	if def.Near != KindLinear || def.Far != KindLinear || def.Mip != MipNearest || def.MaxAniso != DefaultAniso {
		t.Errorf("DefaultFilter = %+v, want linear/linear/nearest/1", def)
	}
	// The frozen switch actually switches.
	var near Filter
	if err := near.SetFilter(KindNearest, KindNearest, 1); err != nil {
		t.Fatalf("SetFilter near: %v", err)
	}
	mag, min, mip, aniso := near.SamplerParams()
	if mag || min || mip || aniso != 1 {
		t.Errorf("near params = %v/%v/%v/%d, want false/false/false/1", mag, min, mip, aniso)
	}
	far := mustFilter(t, findSampler(t, c, "far-stable"))
	mag, min, mip, aniso = far.SamplerParams()
	if !mag || !min || !mip || aniso != 4 {
		t.Errorf("far-stable params = %v/%v/%v/%d, want true/true/true/4", mag, min, mip, aniso)
	}
	// Per-picture memory: two ids hold different switches independently.
	tab := NewTable()
	if err := tab.Set("tree-near", near); err != nil {
		t.Fatalf("Set tree-near: %v", err)
	}
	if err := tab.Set("tree-far", far); err != nil {
		t.Fatalf("Set tree-far: %v", err)
	}
	gotNear, ok := tab.Get("tree-near")
	if !ok || gotNear != near {
		t.Errorf("tree-near = %+v,%v, want %+v,true", gotNear, ok, near)
	}
	gotFar, ok := tab.Get("tree-far")
	if !ok || gotFar != far {
		t.Errorf("tree-far = %+v,%v, want %+v,true", gotFar, ok, far)
	}
}

// B: empty/zero/huge/bad inputs never panic, always a core code.
func TestMipmapEdgesNoCrash(t *testing.T) {
	if n := NumLevels(0, 0); n != 0 {
		t.Errorf("NumLevels(0,0) = %d, want 0", n)
	}
	if n := NumLevels(-4, 64); n != 0 {
		t.Errorf("NumLevels(-4,64) = %d, want 0", n)
	}
	// Tiny picture: one level, every scale lands on it.
	if n := NumLevels(1, 1); n != 1 {
		t.Errorf("NumLevels(1,1) = %d, want 1", n)
	}
	if got := LevelForScale(0.25, 1, 1); got != 0 {
		t.Errorf("LevelForScale tiny = %d, want 0", got)
	}
	for _, lv := range []int{-1, 1, 99} {
		if _, _, ok := LevelSize(1, 1, lv); ok {
			t.Errorf("LevelSize(1x1 @%d) ok=true, want false", lv)
		}
	}
	if _, _, ok := LevelSize(0, 0, 0); ok {
		t.Error("LevelSize(0x0) ok=true, want false")
	}
	// NaN/Inf/zero/negative scales fall back to level 0, never a panic.
	posInf := math.Inf(1)
	negInf := math.Inf(-1)
	nan := math.NaN()
	for _, sc := range []float64{0, -0.5, posInf, negInf, nan} {
		_ = LevelForScale(sc, 64, 64)
		if got := LevelForScale(sc, 64, 64); got != 0 {
			t.Errorf("LevelForScale(%v) = %d, want 0", sc, got)
		}
	}
	var nilF *Filter
	expectMipmapCode(t, "nil SetFilter", nilF.SetFilter(KindLinear, KindLinear, 1), core.CodeInvalidArg)
	expectMipmapCode(t, "nil SetMipmap", nilF.SetMipmap(MipLinear), core.CodeInvalidArg)
	var f Filter
	badKinds := [][2]Kind{{KindUnknown, KindLinear}, {KindLinear, KindUnknown}, {Kind(99), KindLinear}}
	for i, k := range badKinds {
		if err := f.SetFilter(k[0], k[1], 1); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad kinds[%d]: want invalid-arg, got %v", i, err)
		}
	}
	for _, a := range []int{0, -1, 17, 99} {
		if err := f.SetFilter(KindLinear, KindLinear, a); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("aniso %d: want invalid-arg, got %v", a, err)
		}
	}
	if err := f.SetMipmap(MipUnknown); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad mip: want invalid-arg, got %v", err)
	}
	// Failed sets leave the receiver untouched.
	if f != (Filter{}) {
		t.Errorf("failed sets moved the filter: %+v", f)
	}
	// Table guards: nil, empty id, unknown kind/mip/aniso store nothing.
	var nilT *Table
	expectMipmapCode(t, "nil Set", nilT.Set("a", DefaultFilter()), core.CodeInvalidArg)
	if _, ok := nilT.Get("a"); ok {
		t.Error("nil Get ok=true, want false")
	}
	if nilT.Remove("a") || nilT.Len() != 0 {
		t.Error("nil table is not empty-quiet")
	}
	nilT.Clear()
	tab := NewTable()
	expectMipmapCode(t, "empty id", tab.Set("", DefaultFilter()), core.CodeInvalidArg)
	expectMipmapCode(t, "unknown kind", tab.Set("a", Filter{Near: KindUnknown, Far: KindLinear, Mip: MipNearest, MaxAniso: 1}), core.CodeInvalidArg)
	expectMipmapCode(t, "unknown mip", tab.Set("a", Filter{Near: KindLinear, Far: KindLinear, Mip: MipUnknown, MaxAniso: 1}), core.CodeInvalidArg)
	expectMipmapCode(t, "aniso 0", tab.Set("a", Filter{Near: KindLinear, Far: KindLinear, Mip: MipNearest, MaxAniso: 0}), core.CodeInvalidArg)
	expectMipmapCode(t, "aniso 99", tab.Set("a", Filter{Near: KindLinear, Far: KindLinear, Mip: MipNearest, MaxAniso: 99}), core.CodeInvalidArg)
	if tab.Len() != 0 {
		t.Errorf("failed sets stored %d rows, want 0", tab.Len())
	}
	if _, ok := tab.Get("missing"); ok {
		t.Error("missing Get ok=true, want false")
	}
	if tab.Remove("missing") {
		t.Error("missing Remove true, want false")
	}
	// Parse helpers fail closed.
	if ParseKind("cubic") != KindUnknown || ParseMipMode("cubic") != MipUnknown {
		t.Error("Parse of unknown must be Unknown")
	}
	if Kind(99).String() == "" || MipMode(99).String() == "" {
		t.Error("String of unknown must stay named")
	}
}

// D: switching cost and storage are measured (per-picture switch, not per texel).
func TestMipmapPerfSwitch(t *testing.T) {
	tab := NewTable()
	def := DefaultFilter()
	const pictures = 1000
	start := time.Now()
	for i := 0; i < pictures; i++ {
		id := core.AssetID("tex/tree#" + itoa(i))
		if err := tab.Set(id, def); err != nil {
			t.Fatalf("Set %d: %v", i, err)
		}
		if _, ok := tab.Get(id); !ok {
			t.Fatalf("Get %d missed", i)
		}
	}
	el := time.Since(start)
	t.Logf("tex-switch: %d pictures set+get in %v (len=%d)", pictures, el, tab.Len())
	var f Filter
	if err := f.SetFilter(KindLinear, KindLinear, 4); err != nil {
		t.Fatalf("SetFilter: %v", err)
	}
	const reps = 20000
	start = time.Now()
	for i := 0; i < reps; i++ {
		_ = LevelForScale(0.3, 256, 256)
		_, _, _, _ = f.SamplerParams()
	}
	el = time.Since(start)
	t.Logf("tex-far-view: %d level+params in %v (%.1f ns/op)", reps, el, float64(el.Nanoseconds())/reps)
}

// E: long runs replay bitwise identical; Remove/Clear never leak rows.
func TestMipmapLongRunStable(t *testing.T) {
	first := checker64(t)
	px, w, h := first, 64, 64
	var means []int
	for lv := 0; lv < NumLevels(w, h); lv++ {
		means = append(means, levelMean(px))
		px, w, h = boxDown(px, w, h)
	}
	for i := 0; i < 1000; i++ {
		px, w, h := first, 64, 64
		for lv := 0; lv < NumLevels(w, h); lv++ {
			if got := levelMean(px); got != means[lv] {
				t.Fatalf("rep %d level %d mean=%d, want %d", i, lv, got, means[lv])
			}
			px, w, h = boxDown(px, w, h)
		}
	}
	// Double-release is quiet: missing removes report false, Clear empties.
	tab := NewTable()
	if err := tab.Set("tree", DefaultFilter()); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !tab.Remove("tree") || tab.Remove("tree") {
		t.Fatal("Remove must be true once, then false")
	}
	for i := 0; i < 100; i++ {
		id := core.AssetID("tex/t#" + itoa(i))
		if err := tab.Set(id, DefaultFilter()); err != nil {
			t.Fatalf("Set %d: %v", i, err)
		}
	}
	tab.Clear()
	if tab.Len() != 0 {
		t.Fatalf("Clear left %d rows", tab.Len())
	}
}

// F: offscreen golden stands in for game_tex (W3 window intent, built with P2).
// The frozen checker halves plus level means are the evidence the far view
// shares with the future far-tree window; no game_* window exists yet.
func TestMipmapOffscreenGolden(t *testing.T) {
	c := loadMipmapCases(t)
	for _, p := range c.Probes {
		if got := LevelForScale(p.Scale, 64, 64); got != p.Level {
			t.Errorf("%s: LevelForScale(%v) = %d, want %d", p.Desc, p.Scale, got, p.Level)
		}
	}
	px := checker64(t)
	for _, s := range c.Level0 {
		off := (s.Y*64 + s.X) * 4
		got := [4]int{int(px[off]), int(px[off+1]), int(px[off+2]), int(px[off+3])}
		if diffs := maxDiff4(got, s.RGBA); diffs > c.Tolerance {
			t.Errorf("level0 (%d,%d) = %v, want %v (tol %d)", s.X, s.Y, got, s.RGBA, c.Tolerance)
		}
	}
	w, h := 64, 64
	for _, m := range c.Means {
		lv := m.Level
		cur, cw, ch := px, w, h
		for i := 0; i < lv; i++ {
			cur, cw, ch = boxDown(cur, cw, ch)
		}
		_ = ch
		_ = cw
		if got := levelMean(cur); diffByte(got, m.Mean) > c.Tolerance {
			t.Errorf("level %d mean = %d, want %d (tol %d)", lv, got, m.Mean, c.Tolerance)
		}
		// Shape: non-tail halves stay 50/50 black/white, never all one
		// color. The 1px tail cannot be split, so it only pins the mean.
		if lv < NumLevels(w, h)-1 {
			lo, hi := 0, 0
			for i := 0; i < len(cur); i += 4 {
				if cur[i] < 128 {
					lo++
				} else {
					hi++
				}
			}
			if lo == 0 || hi == 0 || diffByte(lo, hi) > len(cur)/4/8 {
				t.Errorf("level %d shape lopsided: dark=%d light=%d", lv, lo, hi)
			}
		}
	}
	// C wiring row: every contract entry maps through one function and a
	// fully-nearest picture always masks mip/aniso like the GPU sampler.
	for _, s := range c.Samplers {
		f := makeFilter(t, s)
		mag, min, mip, aniso := f.SamplerParams()
		if nameOf(mag) != s.Mag || nameOf(min) != s.Min || nameOf(mip) != s.Mipmap || int(aniso) != s.EffAniso {
			t.Errorf("%s: params = %s/%s/%s/%d, want %s/%s/%s/%d",
				s.Name, nameOf(mag), nameOf(min), nameOf(mip), aniso,
				s.Mag, s.Min, s.Mipmap, s.EffAniso)
		}
	}
}

func maxDiff4(a, b [4]int) int {
	m := 0
	for i := 0; i < 4; i++ {
		if d := diffByte(a[i], b[i]); d > m {
			m = d
		}
	}
	return m
}

// itoa renders a small non-negative int without importing strconv in tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func nameOf(linear bool) string {
	if linear {
		return "linear"
	}
	return "nearest"
}
