package audio

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsPositional = 1e-9

func closeFloat(got, want float64) bool { return math.Abs(got-want) < epsPositional }

type mixCase struct {
	Name        string     `json:"name"`
	Listener    [2]float64 `json:"listener"`
	Pos         [2]float64 `json:"pos"`
	Range       float64    `json:"range"`
	Attenuation float64    `json:"attenuation"`
	PanStrength float64    `json:"pan_strength"`
	WantDist    float64    `json:"want_distance"`
	WantGain    float64    `json:"want_gain"`
	WantPan     float64    `json:"want_pan"`
	WantAudible bool       `json:"want_audible"`
	WantOK      bool       `json:"want_ok"`
}

type stereoCase struct {
	Name     string  `json:"name"`
	Mono     float64 `json:"mono"`
	Gain     float64 `json:"gain"`
	Pan      float64 `json:"pan"`
	Audible  bool    `json:"audible"`
	Distance float64 `json:"distance"`
	WantL    float64 `json:"want_l"`
	WantR    float64 `json:"want_r"`
}

type positionalFile struct {
	Mixes   []mixCase    `json:"mixes"`
	Stereos []stereoCase `json:"stereos"`
}

func loadPositionalCases(t *testing.T) positionalFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "positional_cases.json"))
	if err != nil {
		t.Fatalf("read positional_cases.json: %v", err)
	}
	var f positionalFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode positional_cases.json: %v", err)
	}
	if len(f.Mixes) == 0 || len(f.Stereos) == 0 {
		t.Fatal("positional_cases.json has no cases")
	}
	return f
}

func mustFindBy[T any](t *testing.T, list []T, key func(T) string, kind, name string) T {
	t.Helper()
	for _, c := range list {
		if key(c) == name {
			return c
		}
	}
	t.Fatalf("positional_cases.json has no %s %s", kind, name)
	var zero T
	return zero
}

func mustFindMix(t *testing.T, f positionalFile, name string) mixCase {
	t.Helper()
	return mustFindBy(t, f.Mixes, func(c mixCase) string { return c.Name }, "mix", name)
}

func mustFindStereo(t *testing.T, f positionalFile, name string) stereoCase {
	t.Helper()
	return mustFindBy(t, f.Stereos, func(c stereoCase) string { return c.Name }, "stereo", name)
}

func mustNewSound(t *testing.T, c mixCase) PosSound {
	t.Helper()
	s, err := NewPosSound(core.V2(c.Pos[0], c.Pos[1]), c.Range, c.Attenuation, c.PanStrength)
	if err != nil {
		t.Fatalf("%s: NewPosSound: %v", c.Name, err)
	}
	return s
}

func checkMix(t *testing.T, c mixCase, got Mix, ok bool) {
	t.Helper()
	if ok != c.WantOK {
		t.Errorf("%s: ok = %v, want %v", c.Name, ok, c.WantOK)
	}
	if !closeFloat(got.Distance, c.WantDist) {
		t.Errorf("%s: distance = %.17g, want %.17g", c.Name, got.Distance, c.WantDist)
	}
	if !closeFloat(got.Gain, c.WantGain) {
		t.Errorf("%s: gain = %.17g, want %.17g", c.Name, got.Gain, c.WantGain)
	}
	if !closeFloat(got.Pan, c.WantPan) {
		t.Errorf("%s: pan = %.17g, want %.17g", c.Name, got.Pan, c.WantPan)
	}
	if got.Audible != c.WantAudible {
		t.Errorf("%s: audible = %v, want %v", c.Name, got.Audible, c.WantAudible)
	}
	if got.Gain < 0 || got.Gain > 1 || got.Pan < -1 || got.Pan > 1 {
		t.Errorf("%s: out of range gain %v pan %v", c.Name, got.Gain, got.Pan)
	}
	if math.IsNaN(got.Gain) || math.IsNaN(got.Pan) || math.IsNaN(got.Distance) {
		t.Errorf("%s: NaN in mix %+v", c.Name, got)
	}
}

func checkStereo(t *testing.T, c stereoCase) {
	t.Helper()
	m := Mix{Gain: c.Gain, Pan: c.Pan, Distance: c.Distance, Audible: c.Audible}
	l, r := Stereo(c.Mono, m)
	if !closeFloat(l, c.WantL) {
		t.Errorf("stereo %s: l = %.17g, want %.17g", c.Name, l, c.WantL)
	}
	if !closeFloat(r, c.WantR) {
		t.Errorf("stereo %s: r = %.17g, want %.17g", c.Name, r, c.WantR)
	}
	if math.IsNaN(l) || math.IsNaN(r) || math.IsInf(l, 0) || math.IsInf(r, 0) {
		t.Errorf("stereo %s: non-finite l=%v r=%v", c.Name, l, r)
	}
}

func expectCode(t *testing.T, id string, fv float64, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s %v want error", id, fv)
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("%s %v code = %v, want invalid-arg", id, fv, core.CodeOf(err))
	}
}

// A:远近左右落在冻结数上，立体声左右分离对。
func TestPositionalMixFromCases(t *testing.T) {
	if DefaultAttenuation != 1 || DefaultPanStrength != 1 {
		t.Fatalf("defaults = %v/%v, want 1/1", DefaultAttenuation, DefaultPanStrength)
	}
	f := loadPositionalCases(t)
	seen := map[string]bool{}
	for _, c := range f.Mixes {
		if seen[c.Name] {
			t.Errorf("duplicate mix %q", c.Name)
		}
		seen[c.Name] = true
		s := mustNewSound(t, c)
		lis := core.V2(c.Listener[0], c.Listener[1])
		got, ok := s.Mix(lis)
		checkMix(t, c, got, ok)
	}
	for _, c := range f.Stereos {
		checkStereo(t, c)
	}
	// Wiring: frozen stereo rows match the frozen mixes through Stereo.
	// center/right_mid/left_mid tie the two tables so file and code agree.
	for _, name := range []string{"center", "right_mid", "left_mid"} {
		mc := mustFindMix(t, f, name)
		sc := mustFindStereo(t, f, name)
		s := mustNewSound(t, mc)
		m, ok := s.Mix(core.V2(mc.Listener[0], mc.Listener[1]))
		if !ok || !m.Audible {
			t.Fatalf("%s: want audible mix for wiring", name)
		}
		if !closeFloat(m.Gain, sc.Gain) || !closeFloat(m.Pan, sc.Pan) {
			t.Fatalf("%s: mix %+v diverges from stereo gain %v pan %v", name, m, sc.Gain, sc.Pan)
		}
	}
}

// B:空零超大坏数据不崩不卡死，占位加报错。
func TestPositionalEdgesNoCrash(t *testing.T) {
	// Bad constructor args are InvalidArg and store nothing. One table
	// covers the three scalar gates so a new gate only adds a row.
	badPos := []core.Vec2{{X: math.NaN()}, {Y: math.Inf(1)}, {X: math.Inf(-1), Y: 1}}
	for _, p := range badPos {
		_, err := NewPosSound(p, 100, 1, 1)
		expectCode(t, "pos", p.X+p.Y, err)
	}
	badScalars := []struct {
		id  string
		arg int
		bad []float64
	}{
		{"range", 0, []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)}},
		{"att", 1, []float64{-0.5, math.NaN(), math.Inf(1)}},
		{"panStrength", 2, []float64{-1, math.NaN(), math.Inf(1)}},
	}
	for _, g := range badScalars {
		for _, fv := range g.bad {
			args := [3]float64{100, 1, 1}
			args[g.arg] = fv
			_, err := NewPosSound(core.V2(0, 0), args[0], args[1], args[2])
			expectCode(t, g.id, fv, err)
		}
	}
	// Zero is valid: range 0 never attenuates, att 0 no falloff, pan 0 mono.
	if _, err := NewPosSound(core.V2(0, 0), 0, 1, 1); err != nil {
		t.Errorf("range 0: %v, want nil", err)
	}
	if _, err := NewPosSound(core.V2(0, 0), 100, 0, 1); err != nil {
		t.Errorf("att 0: %v, want nil", err)
	}
	if _, err := NewPosSound(core.V2(0, 0), 100, 1, 0); err != nil {
		t.Errorf("panStrength 0: %v, want nil", err)
	}
	zero := PosSound{}
	if m, ok := zero.Mix(core.V2(0, 0)); !ok || !m.Audible || m.Gain != 1 || m.Pan != 0 {
		t.Errorf("zero value mix = %+v/%v, want gain1 pan0 audible", m, ok)
	}
	// Bad listener fails closed with silence, never NaN. The silent mix
	// still carries the distance when measurable, so the check runs on the
	// shared helper instead of a second formula.
	good, err := NewPosSound(core.V2(10, 0), 100, 1, 1)
	if err != nil {
		t.Fatalf("good source: %v", err)
	}
	for _, bad := range badPos {
		m, ok := good.Mix(bad)
		if ok || m.Audible || m.Gain != 0 || m.Pan != 0 {
			t.Errorf("bad listener %v: %+v/%v, want silent + false", bad, m, ok)
		}
		if math.IsNaN(m.Gain) || math.IsNaN(m.Pan) || math.IsNaN(m.Distance) {
			t.Errorf("bad listener %v: NaN in %+v", bad, m)
		}
	}
	// Torn struct literal (bypassing the constructor) still fails closed.
	torn := PosSound{Pos: core.V2(0, 0), Range: -5, Attenuation: 1, PanStrength: 1}
	if m, ok := torn.Mix(core.V2(0, 0)); ok || m.Audible {
		t.Errorf("torn range: %+v/%v, want silent + false", m, ok)
	}
	// MixAll: bad listener fails the call, bad slots never mute the scene.
	if out, ok := MixAll(core.V2(math.NaN(), 0), []PosSound{good}); ok || out != nil {
		t.Errorf("MixAll bad listener = %v/%v, want nil + false", out, ok)
	}
	empty, ok := MixAll(core.V2(0, 0), nil)
	if !ok || empty == nil || len(empty) != 0 {
		t.Errorf("MixAll nil = %v/%v, want empty non-nil + true", empty, ok)
	}
	mixed, ok := MixAll(core.V2(0, 0), []PosSound{good, torn, good})
	if !ok || len(mixed) != 3 {
		t.Fatalf("MixAll torn = %v/%v, want 3 + true", mixed, ok)
	}
	if !mixed[0].Audible || mixed[1].Audible || !mixed[2].Audible {
		t.Errorf("MixAll torn slots = %v, want [audible silent audible]", mixed)
	}
	// Stereo: silent, zero, and bad inputs all park at 0,0. One loop
	// covers mono faults and torn mixes so a new fault only adds a case.
	aud, _ := good.Mix(core.V2(10, 0))
	stereoBad := []struct {
		name string
		mono float64
		mix  Mix
	}{
		{"zero_mono", 0, aud},
		{"silent", 0.5, Mix{Gain: 0, Pan: 1, Distance: 100, Audible: false}},
		{"nan_mono", math.NaN(), aud},
		{"inf_mono", math.Inf(1), aud},
		{"neg_inf_mono", math.Inf(-1), aud},
		{"nan_gain", 0.5, Mix{Gain: math.NaN(), Pan: 0, Distance: 0, Audible: true}},
		{"gain_over", 0.5, Mix{Gain: 2, Pan: 0, Distance: 0, Audible: true}},
		{"pan_over", 0.5, Mix{Gain: 0.5, Pan: 2, Distance: 0, Audible: true}},
		{"neg_dist", 0.5, Mix{Gain: 0.5, Pan: 0, Distance: -1, Audible: true}},
	}
	for _, c := range stereoBad {
		if l, r := Stereo(c.mono, c.mix); l != 0 || r != 0 {
			t.Errorf("stereo %s = %v/%v, want 0/0", c.name, l, r)
		}
	}
	// Huge-but-finite worlds measure far and stay silent, never loud.
	huge, err := NewPosSound(core.V2(1e308, 0), 100, 1, 1)
	if err != nil {
		t.Fatalf("huge source: %v", err)
	}
	m, ok := huge.Mix(core.V2(0, 0))
	if !ok || m.Audible || m.Gain != 0 {
		t.Errorf("huge mix = %+v/%v, want silent + true", m, ok)
	}
	if !finiteMix(m) {
		t.Errorf("huge mix non-finite %+v", m)
	}
}

func finiteMix(m Mix) bool {
	return !math.IsNaN(m.Gain) && !math.IsInf(m.Gain, 0) &&
		!math.IsNaN(m.Pan) && !math.IsInf(m.Pan, 0) &&
		!math.IsNaN(m.Distance) && !math.IsInf(m.Distance, 0)
}

// C不适用（纯算数不画画）：数路无损加逐位重放即两边同数。
func TestPositionalBoundaryIdentical(t *testing.T) {
	f := loadPositionalCases(t)
	for _, c := range f.Mixes {
		s := mustNewSound(t, c)
		lis := core.V2(c.Listener[0], c.Listener[1])
		// Render boundary is lossless for both ends.
		if back := core.Vec2FromRenderPoint(s.Pos.ToRenderPoint()); back != s.Pos {
			t.Errorf("%s: pos boundary = %v, want %v", c.Name, back, s.Pos)
		}
		if back := core.Vec2FromRenderPoint(lis.ToRenderPoint()); back != lis {
			t.Errorf("%s: listener boundary = %v, want %v", c.Name, back, lis)
		}
		a, oka := s.Mix(lis)
		b, okb := s.Mix(lis)
		if oka != okb || a != b {
			t.Errorf("%s: replay diverged %+v/%v vs %+v/%v", c.Name, a, oka, b, okb)
		}
	}
	// MixAll replays bitwise and never aliases the input.
	lis := core.V2(0, 0)
	srcs := []PosSound{}
	for _, c := range f.Mixes[:4] {
		srcs = append(srcs, mustNewSound(t, c))
	}
	snap := append([]PosSound(nil), srcs...)
	a, oka := MixAll(lis, srcs)
	b, okb := MixAll(lis, srcs)
	if !oka || !okb || len(a) != len(b) {
		t.Fatalf("MixAll replay ok=%v/%v len=%d/%d", oka, okb, len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("MixAll replay diverged at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
	for i := range srcs {
		if srcs[i] != snap[i] {
			t.Fatal("MixAll mutated the input")
		}
	}
	if len(a) > 0 {
		a[0].Gain = -999
		again, _ := MixAll(lis, srcs)
		if again[0].Gain == -999 {
			t.Error("MixAll result aliases hidden state")
		}
		if again[0] != b[0] {
			t.Error("MixAll probe write leaked into replay")
		}
	}
	// Stereo replays bitwise for every frozen row.
	for _, c := range f.Stereos {
		m := Mix{Gain: c.Gain, Pan: c.Pan, Distance: c.Distance, Audible: c.Audible}
		l1, r1 := Stereo(c.Mono, m)
		l2, r2 := Stereo(c.Mono, m)
		if l1 != l2 || r1 != r2 {
			t.Errorf("stereo %s replay diverged", c.Name)
		}
	}
}

// D:多声跑得动，耗时加声数有数。
func TestPositionalPerfMulti(t *testing.T) {
	// Synthetic load only (no golden): golden stays in
	// positional_cases.json. Seeded rand keeps the load replayable.
	r := core.NewRand(20260915)
	const n = 256
	srcs := make([]PosSound, n)
	for i := range srcs {
		s, err := NewPosSound(
			core.V2(r.RangeFloat(-1000, 1000), r.RangeFloat(-1000, 1000)),
			200, 1, 1,
		)
		if err != nil {
			t.Fatalf("source %d: %v", i, err)
		}
		srcs[i] = s
	}
	lis := core.V2(0, 0)
	const reps = 2000
	start := time.Now()
	audible := 0
	// MixAll plus one stereo sample per voice share the rep so the cost
	// covers the real per-frame path (mix then pan), not mix alone.
	for i := 0; i < reps; i++ {
		out, ok := MixAll(lis, srcs)
		if !ok || len(out) != n {
			t.Fatalf("rep %d: ok=%v len=%d", i, ok, len(out))
		}
		for _, m := range out {
			if !finiteMix(m) {
				t.Fatalf("rep %d: non-finite %+v", i, m)
			}
			l, rr := Stereo(0.5, m)
			if math.IsNaN(l) || math.IsNaN(rr) {
				t.Fatalf("rep %d: stereo NaN", i)
			}
			if m.Audible {
				audible++
				if m.Gain <= 0 || m.Gain > 1 {
					t.Fatalf("rep %d: bad gain %+v", i, m)
				}
			}
		}
	}
	el := time.Since(start)
	t.Logf("positional-256: %d reps x %d voices (%d mixes) in %v (%.1f us/rep, audible %d)", reps, n, reps*n, el, float64(el.Microseconds())/reps, audible)
	if audible == 0 {
		t.Error("perf load heard nothing, benchmark invalid")
	}
}

// E:长跑不爆不漂，坏数据不粘。
func TestPositionalLongRunStable(t *testing.T) {
	f := loadPositionalCases(t)
	mid := mustFindMix(t, f, "right_mid")
	s := mustNewSound(t, mid)
	lis := core.V2(mid.Listener[0], mid.Listener[1])
	first, ok := s.Mix(lis)
	if !ok {
		t.Fatal("right_mid want ok")
	}
	for i := 0; i < 10000; i++ {
		got, ok := s.Mix(lis)
		if !ok || got != first {
			t.Fatalf("rep %d diverged %+v vs %+v", i, got, first)
		}
		if !finiteMix(got) {
			t.Fatalf("rep %d non-finite %+v", i, got)
		}
	}
	// Walk toward the source and back: gain rises then returns, never sticks.
	steps := []struct {
		x      float64
		louder bool
	}{
		{50, false},
		{0, true},
		{50, false},
	}
	var lastGain float64
	for i, st := range steps {
		src, err := NewPosSound(core.V2(st.x, 0), 100, 1, 1)
		if err != nil {
			t.Fatalf("walk %d: %v", i, err)
		}
		m, ok := src.Mix(core.V2(0, 0))
		if !ok {
			t.Fatalf("walk %d want ok", i)
		}
		if i > 0 {
			if st.louder && !(m.Gain > lastGain) {
				t.Errorf("walk %d gain %v not louder than %v", i, m.Gain, lastGain)
			}
			if !st.louder && !(m.Gain < lastGain) {
				t.Errorf("walk %d gain %v not quieter than %v", i, m.Gain, lastGain)
			}
		}
		lastGain = m.Gain
	}
	if !closeFloat(lastGain, first.Gain) {
		t.Errorf("walk returned gain %v, want %v", lastGain, first.Gain)
	}
	// Bad data never poisons the next good mix; levels never explode.
	badLis := core.V2(math.NaN(), 0)
	if _, ok := s.Mix(badLis); ok {
		t.Error("bad listener want ok=false")
	}
	after, ok := s.Mix(lis)
	if !ok || after != first {
		t.Errorf("after bad: %+v/%v, want %+v/true", after, ok, first)
	}
	for i := 0; i < 5000; i++ {
		m, _ := s.Mix(lis)
		l, r := Stereo(1, m)
		if math.Abs(l) > 1 || math.Abs(r) > 1 {
			t.Fatalf("rep %d clips l=%v r=%v", i, l, r)
		}
	}
}

// F:离屏金对照窗（窗免，纯算数加人工听）：冻结数加形状断言，立体声数即听感。
func TestPositionalOffscreenGolden(t *testing.T) {
	f := loadPositionalCases(t)
	center := mustFindMix(t, f, "center")
	right := mustFindMix(t, f, "right_mid")
	left := mustFindMix(t, f, "left_mid")
	quiet := mustFindMix(t, f, "quiet_far")
	edge := mustFindMix(t, f, "far_edge")
	// Golden pins the anchors: center full, mid half, edge silent.
	if center.WantGain != 1 || center.WantPan != 0 || !center.WantAudible {
		t.Errorf("center golden = %+v, want gain1 pan0 audible", center)
	}
	if !closeFloat(right.WantGain, 0.5) || !closeFloat(right.WantPan, 0.5) {
		t.Errorf("right_mid golden = %+v, want gain0.5 pan0.5", right)
	}
	if edge.WantGain != 0 || edge.WantAudible {
		t.Errorf("far_edge golden = %+v, want silent", edge)
	}
	// Shape: nearer is louder; left mirrors right; up stays centered.
	if !(center.WantGain > right.WantGain && right.WantGain > quiet.WantGain && quiet.WantGain > edge.WantGain) {
		t.Errorf("gain does not fall with distance: %v %v %v %v",
			center.WantGain, right.WantGain, quiet.WantGain, edge.WantGain)
	}
	if !closeFloat(left.WantGain, right.WantGain) || !closeFloat(left.WantPan, -right.WantPan) {
		t.Errorf("left/right not mirrored: %+v vs %+v", left, right)
	}
	up := mustFindMix(t, f, "up_no_pan")
	if up.WantPan != 0 {
		t.Errorf("up pan = %v, want 0 (Y never pans)", up.WantPan)
	}
	// Shape: att 2 hushes faster than att 1 at the same spot.
	quad := mustFindMix(t, f, "quad_att2")
	if !(quad.WantGain < right.WantGain) {
		t.Errorf("att2 gain %v not quieter than att1 %v", quad.WantGain, right.WantGain)
	}
	noFall := mustFindMix(t, f, "att0_no_falloff")
	if noFall.WantGain != 1 {
		t.Errorf("att0 gain = %v, want 1 (no falloff inside range)", noFall.WantGain)
	}
	// Shape: range 0 never attenuates but still pans; pan 0 stays mono.
	r0far := mustFindMix(t, f, "range0_far")
	if !r0far.WantAudible || r0far.WantGain != 1 || r0far.WantPan != 1 {
		t.Errorf("range0_far = %+v, want gain1 pan1 audible", r0far)
	}
	mono := mustFindMix(t, f, "pan0_mono")
	if mono.WantPan != 0 {
		t.Errorf("pan0 = %+v, want pan0 mono", mono)
	}
	strong := mustFindMix(t, f, "pan2_strong")
	if !closeFloat(strong.WantPan, 0.5) {
		t.Errorf("pan2 pan = %v, want 0.5 (strong reaches full sooner)", strong.WantPan)
	}
	// Shape: stereo balance is what the ear hears. Center stays full both
	// sides, right leans right, left leans left, full right mutes left,
	// silent and zero stay silent. Levels never boost past mono (no blast).
	for _, c := range f.Stereos {
		if math.Abs(c.WantL) > math.Abs(c.Mono)+epsPositional ||
			math.Abs(c.WantR) > math.Abs(c.Mono)+epsPositional {
			t.Errorf("stereo %s boosts past mono: l=%v r=%v mono=%v", c.Name, c.WantL, c.WantR, c.Mono)
		}
	}
	lc := mustFindStereo(t, f, "center")
	if lc.WantL != lc.WantR {
		t.Errorf("center stereo l=%v r=%v, want equal", lc.WantL, lc.WantR)
	}
	lr := mustFindStereo(t, f, "right_mid")
	ll := mustFindStereo(t, f, "left_mid")
	if !(lr.WantR > lr.WantL && ll.WantL > ll.WantR) {
		t.Errorf("stereo lean wrong: right %+v left %+v", lr, ll)
	}
	rf := mustFindStereo(t, f, "range0_far_right")
	if rf.WantL != 0 || rf.WantR == 0 {
		t.Errorf("full right = l=%v r=%v, want 0 audible", rf.WantL, rf.WantR)
	}
	sil := mustFindStereo(t, f, "far_edge_silent")
	if sil.WantL != 0 || sil.WantR != 0 {
		t.Errorf("silent = l=%v r=%v, want 0/0", sil.WantL, sil.WantR)
	}
	// Waveform guard: full-scale mono never clips per channel.
	for _, c := range f.Stereos {
		m := Mix{Gain: c.Gain, Pan: c.Pan, Distance: c.Distance, Audible: c.Audible}
		l, r := Stereo(1, m)
		if math.Abs(l) > 1 || math.Abs(r) > 1 {
			t.Errorf("stereo %s full-scale clips l=%v r=%v", c.Name, l, r)
		}
	}
	// Manual listening note: the frozen L/R numbers are the audible
	// samples. Human check with headphones: center stays centered,
	// right_mid leans right, left_mid leans left, range0_far sits hard
	// right, quiet_far is faint but still placed. Waveform stays in
	// [-1,1] per channel (asserted above), so long runs never blast.
}
