package anim

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsEasing = 1e-9

type easingCase struct {
	Kind  string    `json:"kind"`
	Value int       `json:"value"`
	Want  []float64 `json:"want"`
}

type easingLerp struct {
	A    float64 `json:"a"`
	B    float64 `json:"b"`
	T    float64 `json:"t"`
	Kind string  `json:"kind"`
	Want float64 `json:"want"`
}

type easingFile struct {
	T     []float64    `json:"t"`
	Cases []easingCase `json:"cases"`
	Lerp  []easingLerp `json:"lerp"`
}

func loadEasingCases(t *testing.T) easingFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "easing_cases.json"))
	if err != nil {
		t.Fatalf("read easing_cases.json: %v", err)
	}
	var f easingFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode easing_cases.json: %v", err)
	}
	if len(f.Cases) == 0 || len(f.T) == 0 {
		t.Fatal("easing_cases.json has no cases")
	}
	return f
}

func mustParse(t *testing.T, name string) Kind {
	t.Helper()
	k, err := Parse(name)
	if err != nil {
		t.Fatalf("Parse %q: %v", name, err)
	}
	return k
}

// mustFindEasing returns the frozen samples for name, mirroring the
// mustFindCase helper in the sibling packages so golden lookups read the
// same way everywhere.
func mustFindEasing(t *testing.T, f easingFile, name string) easingCase {
	t.Helper()
	for _, c := range f.Cases {
		if c.Kind == name {
			return c
		}
	}
	t.Fatalf("easing_cases.json has no %s", name)
	return easingCase{}
}

// A: every frozen curve lands on the frozen numbers, lerp follows.
func TestEasingValuesFromCases(t *testing.T) {
	f := loadEasingCases(t)
	if len(f.Cases) != len(kindNames) {
		t.Fatalf("cases = %d, want %d frozen kinds", len(f.Cases), len(kindNames))
	}
	seen := map[string]bool{}
	for _, c := range f.Cases {
		k := mustParse(t, c.Kind)
		if int(k) != c.Value {
			t.Errorf("%s: value = %d, want frozen %d", c.Kind, int(k), c.Value)
		}
		if Name(k) != c.Kind {
			t.Errorf("Name(%d) = %q, want %q", int(k), Name(k), c.Kind)
		}
		if !Valid(k) {
			t.Errorf("%s: Valid = false, want true", c.Kind)
		}
		if seen[c.Kind] {
			t.Errorf("duplicate kind %q", c.Kind)
		}
		seen[c.Kind] = true
		if len(c.Want) != len(f.T) {
			t.Errorf("%s: want len = %d, want %d", c.Kind, len(c.Want), len(f.T))
			continue
		}
		for i, tt := range f.T {
			got := Ease(k, tt)
			if math.Abs(got-c.Want[i]) >= epsEasing {
				t.Errorf("%s t=%v = %.17g, want %.17g", c.Kind, tt, got, c.Want[i])
			}
		}
	}
	for _, n := range kindNames {
		if !seen[n] {
			t.Errorf("frozen kind %q missing from cases", n)
		}
	}
	for i, l := range f.Lerp {
		k := mustParse(t, l.Kind)
		if got := Lerp(l.A, l.B, l.T, k); math.Abs(got-l.Want) >= epsEasing {
			t.Errorf("lerp[%d] = %.17g, want %.17g", i, got, l.Want)
		}
		// Lerp wires Ease directly, never a second formula.
		if want := l.A + (l.B-l.A)*Ease(k, l.T); math.Abs(want-l.Want) >= epsEasing {
			t.Errorf("lerp[%d] wiring diverged: %.17g vs %.17g", i, want, l.Want)
		}
	}
}

// B: empty/zero/huge/bad clocks clamp or park, never panic or NaN.
func TestEasingEdgesNoCrash(t *testing.T) {
	kinds := AllKinds()
	// Out-of-range parks at the ends exactly.
	for _, k := range kinds {
		if got := Ease(k, -1e308); got != 0 {
			t.Errorf("%s(-huge) = %v, want 0", Name(k), got)
		}
		if got := Ease(k, 1e308); got != 1 {
			t.Errorf("%s(+huge) = %v, want 1", Name(k), got)
		}
		if got := Ease(k, math.Inf(-1)); got != 0 {
			t.Errorf("%s(-Inf) = %v, want 0", Name(k), got)
		}
		if got := Ease(k, math.Inf(1)); got != 1 {
			t.Errorf("%s(+Inf) = %v, want 1", Name(k), got)
		}
		if got := Ease(k, math.NaN()); got != 0 {
			t.Errorf("%s(NaN) = %v, want 0", Name(k), got)
		}
	}
	// Unknown kinds fall back to linear, never panic or NaN.
	for _, bad := range []Kind{Kind(-1), Kind(len(kindNames)), Kind(1e6)} {
		if Valid(bad) {
			t.Errorf("Valid(%d) = true, want false", int(bad))
		}
		if got := Name(bad); got != "unknown" {
			t.Errorf("Name(%d) = %q, want unknown", int(bad), got)
		}
		for _, tt := range []float64{-1, 0, 0.3, 0.7, 1, 2, math.NaN(), math.Inf(1)} {
			got := Ease(bad, tt)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Errorf("bad kind %d t=%v = %v, want finite", int(bad), tt, got)
			}
			// Ease outputs are always finite (ends exact, interior
			// guarded), so plain != is safe here. The sibling sort
			// package cannot do this because NaN never equals itself.
			if want := Ease(Linear, tt); got != want {
				t.Errorf("bad kind %d t=%v = %v, want linear %v", int(bad), tt, got, want)
			}
		}
	}
	// Bad names never guess a curve.
	for _, s := range []string{"", "LINEAR", "in_quad ", "nope", "unknown"} {
		if k, err := Parse(s); err == nil {
			t.Errorf("Parse %q = %v, want error", s, k)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("Parse %q code = %v, want invalid-arg", s, core.CodeOf(err))
		}
	}
	// Ends are exact for every kind, lerp ends return the inputs exactly.
	for _, k := range kinds {
		if got := Ease(k, 0); got != 0 {
			t.Errorf("%s(0) = %.17g, want 0", Name(k), got)
		}
		if got := Ease(k, 1); got != 1 {
			t.Errorf("%s(1) = %.17g, want 1", Name(k), got)
		}
		if got := Lerp(10, 20, 0, k); got != 10 {
			t.Errorf("%s lerp start = %v, want 10", Name(k), got)
		}
		if got := Lerp(10, 20, 1, k); got != 20 {
			t.Errorf("%s lerp end = %v, want 20", Name(k), got)
		}
	}
	// Dense sweep stays finite: no NaN/Inf hides between the golden t.
	for _, k := range kinds {
		for i := 0; i <= 1000; i++ {
			got := Ease(k, float64(i)/1000)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("%s t=%v = %v, want finite", Name(k), float64(i)/1000, got)
			}
		}
	}
}

// C does not apply (pure math, draws nothing): the number path must be
// lossless and replay bitwise identical instead.
func TestEasingBoundaryIdentical(t *testing.T) {
	f := loadEasingCases(t)
	for _, c := range f.Cases {
		k := mustParse(t, c.Kind)
		for i, tt := range f.T {
			a := Ease(k, tt)
			b := Ease(k, tt)
			if a != b {
				t.Fatalf("%s t=%v replay diverged: %.17g vs %.17g", c.Kind, tt, a, b)
			}
			if math.Abs(a-c.Want[i]) >= epsEasing {
				t.Errorf("%s t=%v = %.17g, want %.17g", c.Kind, tt, a, c.Want[i])
			}
		}
		// Name and Parse round-trip losslessly.
		if back, err := Parse(Name(k)); err != nil || back != k {
			t.Errorf("%s round trip = %v/%v, want %v/nil", c.Kind, back, err, k)
		}
	}
	// Lerp is exactly the Ease wiring, replayed bitwise.
	k := mustParse(t, "in_quad")
	for i := 0; i < 1000; i++ {
		tt := float64(i) / 999
		a := Lerp(-5, 7, tt, k)
		b := -5 + 12*Ease(k, tt)
		if a != b {
			t.Fatalf("lerp replay diverged at t=%v: %v vs %v", tt, a, b)
		}
	}
}

// D: 10k eased evaluations complete with a measured cost.
func TestEasingPerf10k(t *testing.T) {
	// Synthetic load only (no golden): golden samples stay in
	// easing_cases.json. Sweeping all 31 kinds keeps the load replayable.
	kinds := AllKinds()
	const n = 10000
	var acc float64
	start := time.Now()
	for i := 0; i < n; i++ {
		k := kinds[i%len(kinds)]
		// Sweep [0,1] without dividing by zero: i%n keeps t in range.
		acc += Ease(k, float64(i%n)/float64(n))
		acc += Lerp(0, 100, float64(i%n)/float64(n), k)
	}
	el := time.Since(start)
	t.Logf("easing-10k: %d eased evals x31 kinds in %v (%.1f ns/op)", n, el, float64(el.Nanoseconds())/n)
	if math.IsNaN(acc) || math.IsInf(acc, 0) {
		t.Fatal("perf accumulation went non-finite, benchmark invalid")
	}
	if acc == 0 {
		t.Error("perf loop folded to zero, benchmark invalid")
	}
}

// E: long runs do not drift (stateless replay stays bitwise identical).
func TestEasingLongRunNoDrift(t *testing.T) {
	kinds := AllKinds()
	first := make([]float64, len(kinds))
	for i, k := range kinds {
		first[i] = Ease(k, 0.37)
	}
	const reps = 200000
	for r := 0; r < reps; r++ {
		for i, k := range kinds {
			if got := Ease(k, 0.37); got != first[i] {
				t.Fatalf("rep %d %s drifted: %.17g vs %.17g", r, Name(k), got, first[i])
			}
		}
	}
	// Ends stay exact after the soak; lerp parks at NaN like Ease does.
	for _, k := range kinds {
		if got := Ease(k, 0); got != 0 || Ease(k, 1) != 1 {
			t.Errorf("%s ends drifted: 0->%v 1->%v", Name(k), got, Ease(k, 1))
		}
		if got := Lerp(4, 9, math.NaN(), k); got != 4 {
			t.Errorf("%s lerp NaN = %v, want 4", Name(k), got)
		}
	}
}

// F: offscreen golden stands in for the window (window-exempt pure math).
// The frozen samples in easing_cases.json are the evidence both backends
// share; shape assertions below pin the meaning, not just the numbers.
func TestEasingOffscreenGolden(t *testing.T) {
	f := loadEasingCases(t)
	// Golden pins the anchor: quad halves and linear identity.
	quad := mustFindEasing(t, f, "in_quad")
	if len(quad.Want) != len(f.T) {
		t.Fatalf("in_quad samples = %d, want %d", len(quad.Want), len(f.T))
	}
	for i, tt := range f.T {
		if tt == 0.5 && math.Abs(quad.Want[i]-0.25) >= epsEasing {
			t.Errorf("in_quad(0.5) golden = %v, want 0.25", quad.Want[i])
		}
	}
	linear := mustFindEasing(t, f, "linear")
	for i, tt := range f.T {
		if math.Abs(linear.Want[i]-tt) >= epsEasing {
			t.Errorf("linear golden t=%v = %v, want identity", tt, linear.Want[i])
		}
	}
	// Shape: non-overshoot families rise monotonically in [0,1].
	mono := []string{"linear", "in_sine", "out_sine", "in_out_sine",
		"in_quad", "out_quad", "in_out_quad",
		"in_cubic", "out_cubic", "in_out_cubic",
		"in_quart", "out_quart", "in_out_quart",
		"in_quint", "out_quint", "in_out_quint",
		"in_expo", "out_expo", "in_out_expo",
		"in_circ", "out_circ", "in_out_circ"}
	for _, name := range mono {
		c := mustFindEasing(t, f, name)
		for i := 1; i < len(c.Want); i++ {
			if c.Want[i]+epsEasing < c.Want[i-1] {
				t.Errorf("%s falls: t=%v %v after %v", name, f.T[i], c.Want[i], c.Want[i-1])
			}
			if c.Want[i] < -epsEasing || c.Want[i] > 1+epsEasing {
				t.Errorf("%s leaves [0,1]: t=%v %v", name, f.T[i], c.Want[i])
			}
		}
	}
	// Shape: overshoot stays bounded per family (bounce in [0,1], back
	// and elastic wider by design). One table covers the three groups so
	// a new family only adds a row.
	bounds := []struct {
		names  []string
		lo, hi float64
		tol    float64
	}{
		{[]string{"in_bounce", "out_bounce", "in_out_bounce"}, 0, 1, epsEasing},
		{[]string{"in_back", "out_back", "in_out_back"}, -0.2, 1.2, 0},
		{[]string{"in_elastic", "out_elastic", "in_out_elastic"}, -0.5, 1.5, 0},
	}
	for _, b := range bounds {
		for _, name := range b.names {
			for i, v := range mustFindEasing(t, f, name).Want {
				if v < b.lo-b.tol || v > b.hi+b.tol {
					t.Errorf("%s overshoots too far: t=%v %v", name, f.T[i], v)
				}
			}
		}
	}
	// Shape: out mirrors in (representative families), in-out hits 0.5.
	mirror := []struct{ in, out string }{
		{"in_quad", "out_quad"}, {"in_cubic", "out_cubic"}, {"in_sine", "out_sine"},
	}
	for _, m := range mirror {
		for _, tt := range []float64{0.2, 0.5, 0.8} {
			a := Ease(mustParse(t, m.out), tt)
			b := 1 - Ease(mustParse(t, m.in), 1-tt)
			if math.Abs(a-b) >= 1e-9 {
				t.Errorf("%s(%v)=%v, want 1-%s(%v)=%v", m.out, tt, a, m.in, 1-tt, b)
			}
		}
	}
	for _, name := range []string{"in_out_quad", "in_out_cubic", "in_out_sine", "in_out_bounce"} {
		if got := Ease(mustParse(t, name), 0.5); math.Abs(got-0.5) >= 1e-9 {
			t.Errorf("%s(0.5) = %v, want 0.5", name, got)
		}
	}
}
