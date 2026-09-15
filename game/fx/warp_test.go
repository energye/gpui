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

// warpTolerance is frozen in warp_cases.json: golden offsets carry 17
// digits so replay is bitwise identical; the tolerance only guards the
// JSON text round-trip.
const warpTolerance = 1e-9

type warpNoiseDef struct {
	Seed  uint64    `json:"seed"`
	Freq  float64   `json:"freq"`
	Drift []float64 `json:"drift"`
}

type warpOffsetDef struct {
	Name     string    `json:"name"`
	Strength float64   `json:"strength"`
	Seed     uint64    `json:"seed"`
	Freq     float64   `json:"freq"`
	Drift    []float64 `json:"drift"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	T        float64   `json:"t"`
	Want     []float64 `json:"want"`
}

type warpPictureDef struct {
	W    int   `json:"w"`
	H    int   `json:"h"`
	RGBA []int `json:"rgba"`
}

type warpImageDef struct {
	Name     string    `json:"name"`
	Strength float64   `json:"strength"`
	Seed     uint64    `json:"seed"`
	Freq     float64   `json:"freq"`
	Drift    []float64 `json:"drift"`
	T        float64   `json:"t"`
	Want     []int     `json:"want"`
}

type warpFile struct {
	Tolerance    float64         `json:"tolerance"`
	DefaultNoise warpNoiseDef    `json:"default_noise"`
	Offsets      []warpOffsetDef `json:"offsets"`
	Picture      warpPictureDef  `json:"picture"`
	Warped       []warpImageDef  `json:"warped"`
}

func loadWarpCases(t *testing.T) warpFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "warp_cases.json"))
	if err != nil {
		t.Fatalf("read warp_cases.json: %v", err)
	}
	var f warpFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode warp_cases.json: %v", err)
	}
	if len(f.Offsets) == 0 || len(f.Warped) == 0 {
		t.Fatal("warp_cases.json has no offsets or warped images")
	}
	if f.Tolerance != warpTolerance {
		t.Fatalf("tolerance = %v, want frozen %v", f.Tolerance, warpTolerance)
	}
	return f
}

func mustFindOffset(t *testing.T, f warpFile, name string) warpOffsetDef {
	t.Helper()
	for _, c := range f.Offsets {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("warp_cases.json has no offset %q", name)
	return warpOffsetDef{}
}

func mustFindWarped(t *testing.T, f warpFile, name string) warpImageDef {
	t.Helper()
	for _, c := range f.Warped {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("warp_cases.json has no warped image %q", name)
	return warpImageDef{}
}

func noiseFromDef(t *testing.T, seed uint64, freq float64, drift []float64) Noise {
	t.Helper()
	if len(drift) != 2 {
		t.Fatalf("drift has %d numbers, want 2", len(drift))
	}
	return Noise{Seed: seed, Freq: freq, Drift: core.V2(drift[0], drift[1])}
}

func buildWarp(t *testing.T, c warpOffsetDef) Warp {
	t.Helper()
	w, err := NewWarp(c.Strength, noiseFromDef(t, c.Seed, c.Freq, c.Drift))
	if err != nil {
		t.Fatalf("%s: NewWarp: %v", c.Name, err)
	}
	return w
}

func sameFloat(got, want float64) bool { return math.Abs(got-want) <= warpTolerance }

// A: sample shifts point the frozen way: direction signs, golden values,
// and the strength/time shape behind them.
func TestWarpOffsetFromCases(t *testing.T) {
	f := loadWarpCases(t)
	for _, c := range f.Offsets {
		w := buildWarp(t, c)
		got := w.OffsetAt(core.V2(c.X, c.Y), c.T)
		if len(c.Want) != 2 {
			t.Fatalf("%s: want has %d numbers, want 2", c.Name, len(c.Want))
		}
		if !sameFloat(got.X, c.Want[0]) || !sameFloat(got.Y, c.Want[1]) {
			t.Errorf("%s: OffsetAt = (%v, %v), want (%v, %v)",
				c.Name, got.X, got.Y, c.Want[0], c.Want[1])
		}
		// Direction: the sign of each axis matches the frozen sign, so a
		// flipped lattice or swapped channel fails here, not just on digits.
		if (got.X > 0) != (c.Want[0] > 0) || (got.X < 0) != (c.Want[0] < 0) {
			t.Errorf("%s: X direction = %v, want sign of %v", c.Name, got.X, c.Want[0])
		}
		if (got.Y > 0) != (c.Want[1] > 0) || (got.Y < 0) != (c.Want[1] < 0) {
			t.Errorf("%s: Y direction = %v, want sign of %v", c.Name, got.Y, c.Want[1])
		}
		// Shape: one noise channel lives in [-1, 1], so each axis stays
		// within one strength.
		if math.Abs(got.X) > c.Strength || math.Abs(got.Y) > c.Strength {
			t.Errorf("%s: offset (%v, %v) escapes strength %v", c.Name, got.X, got.Y, c.Strength)
		}
		// Shape: WarpPoint is exactly p + OffsetAt, no hidden rounding.
		wp := w.WarpPoint(core.V2(c.X, c.Y), c.T)
		if wp.X != c.X+got.X || wp.Y != c.Y+got.Y {
			t.Errorf("%s: WarpPoint = (%v, %v), want p+offset (%v, %v)",
				c.Name, wp.X, wp.Y, c.X+got.X, c.Y+got.Y)
		}
		// Shape: offset scales linearly with strength (direction fixed,
		// amplitude dialed), so doubling the strength doubles the shift.
		w2, err := NewWarp(c.Strength*2, w.Noise)
		if err != nil {
			t.Fatalf("%s: NewWarp double: %v", c.Name, err)
		}
		got2 := w2.OffsetAt(core.V2(c.X, c.Y), c.T)
		if got2.X != 2*got.X || got2.Y != 2*got.Y {
			t.Errorf("%s: double-strength = (%v, %v), want 2x (%v, %v)",
				c.Name, got2.X, got2.Y, 2*got.X, 2*got.Y)
		}
	}
	// Shape: drift moves the field over time (mid-t0 vs mid-t1p5 differ),
	// while a zero drift freezes it (seed7 t=0 vs t=1.5 identical).
	a := mustFindOffset(t, f, "mid-t0")
	b := mustFindOffset(t, f, "mid-t1p5")
	if a.Want[0] == b.Want[0] && a.Want[1] == b.Want[1] {
		t.Error("drifted field frozen in time: mid-t0 == mid-t1p5")
	}
	c0 := mustFindOffset(t, f, "seed7-mid-t0")
	c1 := mustFindOffset(t, f, "seed7-mid-t1p5-frozen")
	if c0.Want[0] != c1.Want[0] || c0.Want[1] != c1.Want[1] {
		t.Error("zero drift moved: seed7 t=0 != t=1.5")
	}
	// Shape: the two axes never mirror each other on the frozen origin.
	o := mustFindOffset(t, f, "origin-t0")
	if o.Want[0] == o.Want[1] {
		t.Error("origin-t0 X == Y: channels mirror each other")
	}
}

// B: zero strength, empty, and corrupt inputs never crash; bad paths
// report codes.
func TestWarpEdgesNoCrash(t *testing.T) {
	// Zero strength is the exact identity for any point and time,
	// including non-finite inputs.
	var zero Warp
	nasty := []core.Vec2{core.V2(0, 0), core.V2(1e308, -1e308), {X: math.NaN(), Y: 1}, {X: 2, Y: math.Inf(1)}}
	for _, p := range nasty {
		for _, tm := range []float64{0, 1.5, math.NaN(), math.Inf(-1)} {
			if got := zero.OffsetAt(p, tm); got != (core.Vec2{}) {
				t.Errorf("zero OffsetAt(%v, %v) = %v, want identity", p, tm, got)
			}
			if got := zero.WarpPoint(p, tm); got.X != p.X && !(math.IsNaN(p.X) && math.IsNaN(got.X)) {
				t.Errorf("zero WarpPoint(%v, %v) = %v, want p", p, tm, got)
			}
		}
	}
	// Zero strength with a garbage field is still the identity, never a crash.
	bad := Warp{Strength: 0, Noise: Noise{Freq: 0, Drift: core.V2(math.NaN(), 1)}}
	if got := bad.OffsetAt(core.V2(3, 4), 1); got != (core.Vec2{}) {
		t.Errorf("garbage-field zero OffsetAt = %v, want identity", got)
	}
	// Non-finite live warps yield the zero offset instead of a NaN leak.
	w, err := NewWarp(6, DefaultNoise())
	if err != nil {
		t.Fatalf("NewWarp default: %v", err)
	}
	for _, p := range []core.Vec2{{X: math.NaN()}, {Y: math.Inf(1)}} {
		if got := w.OffsetAt(p, 1); got != (core.Vec2{}) {
			t.Errorf("OffsetAt(%v) = %v, want zero", p, got)
		}
	}
	if got := w.OffsetAt(core.V2(1, 2), math.NaN()); got != (core.Vec2{}) {
		t.Errorf("OffsetAt(NaN time) = %v, want zero", got)
	}
	// Bad constructors build nothing and report invalid-arg.
	dn := DefaultNoise()
	badBuilds := []struct {
		name string
		s    float64
		n    Noise
	}{
		{"neg-strength", -1, dn},
		{"nan-strength", math.NaN(), dn},
		{"inf-strength", math.Inf(1), dn},
		{"zero-freq", 1, Noise{Seed: 1, Freq: 0}},
		{"neg-freq", 1, Noise{Seed: 1, Freq: -0.5}},
		{"nan-freq", 1, Noise{Seed: 1, Freq: math.NaN()}},
		{"nan-drift", 1, Noise{Seed: 1, Freq: 0.5, Drift: core.V2(math.NaN(), 0)}},
	}
	for _, tc := range badBuilds {
		if _, err := NewWarp(tc.s, tc.n); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("%s: err = %v, want invalid-arg", tc.name, err)
		}
	}
	// Bad setters report invalid-arg and leave the warp untouched.
	before := w
	if err := w.SetStrength(-2); core.CodeOf(err) != core.CodeInvalidArg || w != before {
		t.Errorf("SetStrength(-2) = %v warp = %+v, want invalid-arg untouched", err, w)
	}
	if err := w.SetNoise(Noise{}); core.CodeOf(err) != core.CodeInvalidArg || w != before {
		t.Errorf("SetNoise(zero) = %v warp = %+v, want invalid-arg untouched", err, w)
	}
	if err := w.SetStrength(3); err != nil || w.Strength != 3 {
		t.Errorf("SetStrength(3) = %v strength = %v, want nil/3", err, w.Strength)
	}
	// Nil receivers never panic.
	var nilW *Warp
	if err := nilW.SetStrength(1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SetStrength err = %v, want invalid-arg", err)
	}
	if err := nilW.SetNoise(dn); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SetNoise err = %v, want invalid-arg", err)
	}
	// Bad pictures report codes, never a panic and never a guess.
	f := loadWarpCases(t)
	src := warpBytes(t, f.Picture)
	if _, err := WarpRGBA(src, 0, 8, 0, w); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero width err = %v, want invalid-arg", err)
	}
	if _, err := WarpRGBA(src[:len(src)-1], 8, 8, 0, w); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("short slice err = %v, want bad-data", err)
	}
	if _, err := WarpRGBA(nil, 9000, 9000, 0, w); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge picture err = %v, want out-of-memory", err)
	}
	if _, err := SampleClamped(src, 8, 0, 0, 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("clamped zero height err = %v, want invalid-arg", err)
	}
	if _, err := SampleClamped(src[:10], 8, 8, 0, 0); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("clamped short slice err = %v, want bad-data", err)
	}
	// NaN time warps to an exact copy: total on the hot path.
	dst, err := WarpRGBA(src, 8, 8, math.NaN(), w)
	if err != nil {
		t.Fatalf("NaN-time WarpRGBA: %v", err)
	}
	for i := range src {
		if dst[i] != src[i] {
			t.Fatalf("NaN-time copy differs at byte %d", i)
		}
	}
}

func warpBytes(t *testing.T, p warpPictureDef) []uint8 {
	t.Helper()
	if len(p.RGBA) != p.W*p.H*4 {
		t.Fatalf("picture has %d numbers, want %d", len(p.RGBA), p.W*p.H*4)
	}
	out := make([]uint8, len(p.RGBA))
	for i, v := range p.RGBA {
		if v < 0 || v > 255 {
			t.Fatalf("picture[%d] = %d, want 0..255", i, v)
		}
		out[i] = uint8(v) //nolint:gosec // range checked above
	}
	return out
}

// C does not need a GPU (pure numbers plus a fresh copy): both builds
// must replay bitwise identical, the source must never be written, and
// edges must pin to the border texel like clamp-to-edge.
func TestWarpBoundaryIdentical(t *testing.T) {
	f := loadWarpCases(t)
	for _, c := range f.Offsets {
		a, b := buildWarp(t, c), buildWarp(t, c)
		for i := 0; i < 1000; i++ {
			p := core.V2(c.X+float64(i%64), c.Y+float64(i/64))
			oa, ob := a.OffsetAt(p, c.T), b.OffsetAt(p, c.T)
			if oa != ob {
				t.Fatalf("%s: replay diverged at step %d: %v vs %v", c.Name, i, oa, ob)
			}
		}
	}
	src := warpBytes(t, f.Picture)
	pic := f.Picture
	mk := func() Warp {
		d, err := NewWarp(6, DefaultNoise())
		if err != nil {
			t.Fatalf("NewWarp: %v", err)
		}
		return d
	}
	first, err := WarpRGBA(src, pic.W, pic.H, 1.0, mk())
	if err != nil {
		t.Fatalf("WarpRGBA: %v", err)
	}
	pristine := append([]uint8(nil), src...)
	for i := 0; i < 50; i++ {
		got, err := WarpRGBA(src, pic.W, pic.H, 1.0, mk())
		if err != nil {
			t.Fatalf("replay %d: %v", i, err)
		}
		if len(got) != len(first) {
			t.Fatalf("replay %d length %d vs %d", i, len(got), len(first))
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("replay %d diverged at byte %d", i, j)
			}
		}
	}
	// The source is never written: the live frame stays intact while the
	// intermediate copy carries the wobble.
	for i := range src {
		if src[i] != pristine[i] {
			t.Fatalf("WarpRGBA wrote the source at byte %d", i)
		}
	}
	// Clamp-to-edge contract: outside pins to the border texel.
	edge := [][2]int{{-5, -5}, {-1, 0}, {0, -1}, {8, 8}, {99, 3}, {4, -99}}
	for _, e := range edge {
		got, err := SampleClamped(src, pic.W, pic.H, e[0], e[1])
		if err != nil {
			t.Fatalf("SampleClamped(%v): %v", e, err)
		}
		cx, cy := e[0], e[1]
		if cx < 0 {
			cx = 0
		}
		if cx >= pic.W {
			cx = pic.W - 1
		}
		if cy < 0 {
			cy = 0
		}
		if cy >= pic.H {
			cy = pic.H - 1
		}
		want, err := SampleClamped(src, pic.W, pic.H, cx, cy)
		if err != nil {
			t.Fatalf("SampleClamped(%d,%d): %v", cx, cy, err)
		}
		if got != want {
			t.Errorf("edge (%d,%d) = %v, want border %v", e[0], e[1], got, want)
		}
	}
	// Strength 0 copies the source byte for byte.
	id, err := WarpRGBA(src, pic.W, pic.H, 3.25, Warp{})
	if err != nil {
		t.Fatalf("identity WarpRGBA: %v", err)
	}
	for i := range src {
		if id[i] != src[i] {
			t.Fatalf("identity copy differs at byte %d", i)
		}
	}
}

// D: a full-HD frame warps with a measured cost (synthetic load only,
// no golden: goldens stay in warp_cases.json).
func TestWarpPerfFullscreen(t *testing.T) {
	// Synthetic load only (no golden): a diagonal gradient stands in
	// for a game frame; the frozen pictures stay in warp_cases.json.
	// Seeded values keep the load replayable.
	const w, h = 1920, 1080
	src := make([]uint8, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			src[i] = uint8((x * 255) / (w - 1))         //nolint:gosec // bounded by construction
			src[i+1] = uint8((y * 255) / (h - 1))       //nolint:gosec // bounded by construction
			src[i+2] = uint8(((x + y) * 255) / (w + h)) //nolint:gosec // bounded by construction
			src[i+3] = 255
		}
	}
	wv, err := NewWarp(6, DefaultNoise())
	if err != nil {
		t.Fatalf("NewWarp: %v", err)
	}
	// Warm up once so the timed passes see steady caches.
	if _, err := WarpRGBA(src, w, h, 0, wv); err != nil {
		t.Fatalf("warmup: %v", err)
	}
	const passes = 3
	best := time.Duration(1 << 62)
	for p := 0; p < passes; p++ {
		start := time.Now()
		dst, err := WarpRGBA(src, w, h, float64(p)*0.016, wv)
		el := time.Since(start)
		if err != nil {
			t.Fatalf("pass %d: %v", p, err)
		}
		if len(dst) != len(src) {
			t.Fatalf("pass %d length %d vs %d", p, len(dst), len(src))
		}
		if el < best {
			best = el
		}
	}
	fps := float64(time.Second) / float64(best)
	t.Logf("warp-fullhd: 1920x1080 in %v (best of %d, %.1f fps equivalent)", best, passes, fps)
	// Offset micro cost: a million sample shifts, timed.
	const n = 1000000
	start := time.Now()
	var acc core.Vec2
	for i := 0; i < n; i++ {
		acc = acc.Add(wv.OffsetAt(core.V2(float64(i%1920), float64(i/1920)), 1.0))
	}
	el := time.Since(start)
	t.Logf("warp-offset: %d shifts in %v (%.1f ns/op)", n, el, float64(el.Nanoseconds())/n)
	if acc == (core.Vec2{}) {
		t.Error("offset accumulator folded to zero, timing loop dead")
	}
}

// E: long runs neither grow nor diverge, and the picture never flowers:
// alpha stays put and no pixel is invented.
func TestWarpLongRunStable(t *testing.T) {
	f := loadWarpCases(t)
	c := mustFindOffset(t, f, "mid-t1p5")
	mk := func() Warp { return buildWarp(t, c) }
	first := mk().OffsetAt(core.V2(c.X, c.Y), c.T)
	for i := 0; i < 5000; i++ {
		if got := mk().OffsetAt(core.V2(c.X, c.Y), c.T); got != first {
			t.Fatalf("rep %d diverged: %v vs %v", i, got, first)
		}
	}
	// Alternating times replay the same alternating outputs forever.
	ts := []float64{0, 0.016, 0.5, 1.5, 10.0}
	wants := make([]core.Vec2, len(ts))
	w := mk()
	for i, tm := range ts {
		wants[i] = w.OffsetAt(core.V2(320, 200), tm)
	}
	for r := 0; r < 200; r++ {
		for i, tm := range ts {
			if got := w.OffsetAt(core.V2(320, 200), tm); got != wants[i] {
				t.Fatalf("round %d t=%v diverged: %v vs %v", r, tm, got, wants[i])
			}
		}
	}
	// Picture soak: repeated warps stay identical and closed.
	src := warpBytes(t, f.Picture)
	pic := f.Picture
	allowed := map[[4]uint8]bool{}
	for i := 0; i < len(src); i += 4 {
		allowed[[4]uint8{src[i], src[i+1], src[i+2], src[i+3]}] = true
	}
	wd, err := NewWarp(6, DefaultNoise())
	if err != nil {
		t.Fatalf("NewWarp: %v", err)
	}
	var prev []uint8
	for r := 0; r < 200; r++ {
		dst, err := WarpRGBA(src, pic.W, pic.H, float64(r)*0.016, wd)
		if err != nil {
			t.Fatalf("soak %d: %v", r, err)
		}
		if r == 0 {
			prev = dst
			continue
		}
		self, err := WarpRGBA(src, pic.W, pic.H, float64(r)*0.016, wd)
		if err != nil {
			t.Fatalf("soak %d re-warp: %v", r, err)
		}
		for i := range dst {
			if dst[i] != self[i] {
				t.Fatalf("soak %d diverged at byte %d", r, i)
			}
		}
		_ = prev
		prev = dst
	}
	last, err := WarpRGBA(src, pic.W, pic.H, 1.0, wd)
	if err != nil {
		t.Fatalf("final warp: %v", err)
	}
	for i := 0; i < len(last); i += 4 {
		if last[i+3] != 255 {
			t.Fatalf("alpha corrupted at pixel %d: %d", i/4, last[i+3])
		}
		px := [4]uint8{last[i], last[i+1], last[i+2], last[i+3]}
		if !allowed[px] {
			t.Fatalf("invented pixel %v at %d: sampler read outside the picture", px, i/4)
		}
	}
}

// F: offscreen golden stands in for the window (window intent:
// game_fx --case=warp shows the same wobble on water; this golden is
// the offscreen proof). Tolerance is 0: nearest sampling replays exact.
func TestWarpOffscreenGolden(t *testing.T) {
	f := loadWarpCases(t)
	src := warpBytes(t, f.Picture)
	pic := f.Picture
	for _, c := range f.Warped {
		w, err := NewWarp(c.Strength, noiseFromDef(t, c.Seed, c.Freq, c.Drift))
		if err != nil {
			t.Fatalf("%s: NewWarp: %v", c.Name, err)
		}
		got, err := WarpRGBA(src, pic.W, pic.H, c.T, w)
		if err != nil {
			t.Fatalf("%s: WarpRGBA: %v", c.Name, err)
		}
		if len(c.Want) != len(src) {
			t.Fatalf("%s: want has %d numbers, want %d", c.Name, len(c.Want), len(src))
		}
		bad := 0
		for i := range got {
			if int(got[i]) != c.Want[i] {
				if bad < 8 {
					t.Errorf("%s: byte %d = %d, want %d", c.Name, i, got[i], c.Want[i])
				}
				bad++
			}
		}
		if bad > 0 {
			t.Fatalf("%s: %d/%d bytes differ", c.Name, bad, len(got))
		}
	}
	// Shape: the water actually wobbles (warped differs from source) but
	// never floods (most pixels still home).
	s6 := mustFindWarped(t, f, "s6-t1")
	diff := 0
	for i := range src {
		if int(src[i]) != s6.Want[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Error("s6-t1 equals the source: no wobble at all")
	}
	if diff == len(src) {
		t.Error("s6-t1 moved every byte: flood, not wobble")
	}
	// Shape: a gentler strength moves fewer bytes than the strong one.
	s2 := mustFindWarped(t, f, "s2-t0")
	diff2 := 0
	for i := range src {
		if int(src[i]) != s2.Want[i] {
			diff2++
		}
	}
	if diff2 == 0 {
		t.Error("s2-t0 equals the source: gentle warp does nothing")
	}
	t.Logf("wobble bytes: gentle %d/%d, strong %d/%d", diff2, len(src), diff, len(src))
}
