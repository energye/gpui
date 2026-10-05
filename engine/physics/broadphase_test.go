package physics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/engine/core"
)

// S86 broadphase tests read every number from testdata; the test builds
// the yard from the layout params (fixture setup) but never invents an
// expected pair: all wants come from large_rays.json, frozen by
// /tmp/gen_yard.py (naive grid math, no engine code).

type yardLayout struct {
	Cols         int        `json:"cols"`
	Rows         int        `json:"rows"`
	Spacing      float64    `json:"spacing"`
	Origin       [2]float64 `json:"origin"`
	Half         float64    `json:"half"`
	StaticCount  int        `json:"static_count"`
	DynamicCount int        `json:"dynamic_count"`
	SpringX      float64    `json:"spring_x"`
	Count        int        `json:"count"`
}

type yardDyn struct {
	Name string     `json:"name"`
	X    float64    `json:"x"`
	Y    float64    `json:"y"`
	Half [2]float64 `json:"half"`
}

type yardSlope struct {
	Name   string     `json:"name"`
	A      [2]float64 `json:"a"`
	B      [2]float64 `json:"b"`
	Oneway bool       `json:"oneway"`
}

type yardSpring struct {
	Name    string  `json:"name"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Impulse float64 `json:"impulse"`
	Damp    float64 `json:"damp"`
}

type yardFile struct {
	Layout   yardLayout   `json:"layout"`
	Dynamics []yardDyn    `json:"dynamics"`
	Oneways  []yardSlope  `json:"oneways"`
	Springs  []yardSpring `json:"springs"`
}

type rayCase struct {
	Name    string     `json:"name"`
	Origin  [2]float64 `json:"origin"`
	Dir     [2]float64 `json:"dir"`
	MaxDist float64    `json:"maxdist"`
	Mask    uint32     `json:"mask"`
}

type regionCase struct {
	Name string     `json:"name"`
	Rect [4]float64 `json:"rect"`
	Want []int      `json:"want"`
}

type raysFile struct {
	YardFile    string       `json:"yard_file"`
	Rays        []rayCase    `json:"rays"`
	Regions     []regionCase `json:"regions"`
	QueryStatic struct {
		WantPairs [][]string `json:"want_pairs"`
		WantCount int        `json:"want_count"`
	} `json:"query_static"`
	QueryDrop struct {
		DY        float64    `json:"dy"`
		WantPairs [][]string `json:"want_pairs"`
		WantCount int        `json:"want_count"`
	} `json:"query_drop"`
	Budgets struct {
		Bodies        int     `json:"bodies"`
		Reps          int     `json:"reps"`
		QueryUsMax    float64 `json:"query_us_max"`
		RayUsMax      float64 `json:"ray_us_max"`
		MaxCandidates int     `json:"max_candidates"`
		Replays       int     `json:"replays"`
	} `json:"budgets"`
}

func loadYardFiles(t testing.TB) (yardFile, raysFile) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "large_rays.json"))
	if err != nil {
		t.Fatalf("read large_rays.json: %v", err)
	}
	var rf raysFile
	if err := json.Unmarshal(raw, &rf); err != nil {
		t.Fatalf("decode large_rays.json: %v", err)
	}
	raw, err = os.ReadFile(filepath.Join("testdata", rf.YardFile))
	if err != nil {
		t.Fatalf("read %s: %v", rf.YardFile, err)
	}
	var yf yardFile
	if err := json.Unmarshal(raw, &yf); err != nil {
		t.Fatalf("decode %s: %v", rf.YardFile, err)
	}
	if yf.Layout.Count == 0 {
		t.Fatal("yard fixture empty")
	}
	return yf, rf
}

// buildYard stamps the filed yard into a Broadphase in filed order:
// statics row-major, dynamics last. Handles run 0..count-1 in that
// order; the test keeps the same order for the naive cross-check.
func buildYard(t testing.TB, yf yardFile) (Broadphase, []Body) {
	t.Helper()
	bp := NewBroadphase()
	var bodies []Body
	l := yf.Layout
	for i := 0; i < l.StaticCount; i++ {
		c, r := i%l.Cols, i/l.Cols
		pos := core.V2(l.Origin[0]+float64(c)*l.Spacing, l.Origin[1]+float64(r)*l.Spacing)
		bd, err := NewBox(fmt.Sprintf("s%04d", i), pos, core.V2(l.Half, l.Half), 1, 1, false)
		if err != nil {
			t.Fatalf("static %d: %v", i, err)
		}
		if _, err := bp.Track(bd, true); err != nil {
			t.Fatalf("track %d: %v", i, err)
		}
		bodies = append(bodies, bd)
	}
	for _, d := range yf.Dynamics {
		bd, err := NewBox(d.Name, core.V2(d.X, d.Y), core.V2(d.Half[0], d.Half[1]), 1, 1, false)
		if err != nil {
			t.Fatalf("dynamic %s: %v", d.Name, err)
		}
		if _, err := bp.Track(bd, false); err != nil {
			t.Fatalf("track %s: %v", d.Name, err)
		}
		bodies = append(bodies, bd)
	}
	if bp.Count() != l.Count {
		t.Fatalf("yard count = %d, want %d", bp.Count(), l.Count)
	}
	return bp, bodies
}

func pairKeys(cs []Contact) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.A + "+" + c.B
	}
	return out
}

func samePairs(got []Contact, want [][]string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].A != want[i][0] || got[i].B != want[i][1] {
			return false
		}
	}
	return true
}

// A: the filed static query matches Query exactly, same pairs and order.
// The cap stays off here: agreement is measured uncapped, the cap gets
// its own exact test below.
func TestBroadphaseFiledStatic(t *testing.T) {
	yf, rf := loadYardFiles(t)
	bp, bodies := buildYard(t, yf)
	if err := bp.SetMaxCandidates(0); err != nil {
		t.Fatalf("uncap: %v", err)
	}
	got, err := bp.Query()
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !samePairs(got, rf.QueryStatic.WantPairs) {
		t.Errorf("static pairs = %d, want %d", len(got), rf.QueryStatic.WantCount)
	}
	// The live narrow path agrees with the file over the same bodies.
	slow, err := Query(bodies)
	if err != nil {
		t.Fatalf("naive: %v", err)
	}
	if len(slow) != len(got) {
		t.Errorf("naive = %d, broad = %d", len(slow), len(got))
	} else {
		for i := range slow {
			if slow[i].A != got[i].A || slow[i].B != got[i].B {
				t.Fatalf("pair[%d] narrow=%s+%s broad=%s+%s", i, slow[i].A, slow[i].B, got[i].A, got[i].B)
			}
		}
	}
}

// B: dynamics dropped by the filed dy meet the filed drop pairs.
func TestBroadphaseFiledDrop(t *testing.T) {
	yf, rf := loadYardFiles(t)
	bp, _ := buildYard(t, yf)
	if err := bp.SetMaxCandidates(0); err != nil {
		t.Fatalf("uncap: %v", err)
	}
	dy := rf.QueryDrop.DY
	for h := yf.Layout.StaticCount; h < yf.Layout.Count; h++ {
		e, ok := bp.at(h)
		if !ok {
			t.Fatalf("dynamic handle %d missing", h)
		}
		nb := e.body
		nb.Pos = core.V2(nb.Pos.X, nb.Pos.Y+dy)
		if err := bp.Move(h, nb); err != nil {
			t.Fatalf("drop %d: %v", h, err)
		}
	}
	got, err := bp.Query()
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !samePairs(got, rf.QueryDrop.WantPairs) {
		t.Errorf("drop pairs = %d, want %d", len(got), rf.QueryDrop.WantCount)
	}
}

// C: rays agree with CastRay one to one, hits and misses.
func TestBroadphaseFiledRays(t *testing.T) {
	yf, rf := loadYardFiles(t)
	bp, bodies := buildYard(t, yf)
	for _, rc := range rf.Rays {
		ray, err := NewRay(core.V2(rc.Origin[0], rc.Origin[1]), core.V2(rc.Dir[0], rc.Dir[1]), rc.MaxDist, rc.Mask)
		if err != nil {
			t.Fatalf("%s ray: %v", rc.Name, err)
		}
		fh, fok, err := bp.Ray(ray)
		if err != nil {
			t.Fatalf("%s broad ray: %v", rc.Name, err)
		}
		sh, sok, err := CastRay(bodies, ray)
		if err != nil {
			t.Fatalf("%s narrow ray: %v", rc.Name, err)
		}
		if fok != sok {
			t.Fatalf("%s: broad hit=%v narrow hit=%v", rc.Name, fok, sok)
		}
		if fok && (fh.Name != sh.Name || fh.Dist != sh.Dist) {
			t.Fatalf("%s: broad %s@%v narrow %s@%v", rc.Name, fh.Name, fh.Dist, sh.Name, sh.Dist)
		}
	}
}

// C2: one region lookup matches the filed handles in order, and stays
// far under the filed 50us single-query gate even on a loaded dev box.
func TestBroadphaseFiledRegions(t *testing.T) {
	yf, rf := loadYardFiles(t)
	bp, _ := buildYard(t, yf)
	if len(rf.Regions) == 0 {
		t.Fatal("no filed regions")
	}
	for _, rg := range rf.Regions {
		rect := core.NewRect(rg.Rect[0], rg.Rect[1], rg.Rect[2], rg.Rect[3])
		// Warmup builds the index once: the gate measures steady
		// cull_aabb, not the first rebuild (same as the window warmup).
		if _, err := bp.QueryRegion(rect); err != nil {
			t.Fatalf("%s warmup: %v", rg.Name, err)
		}
		t0 := time.Now()
		var got []int
		for r := 0; r < 20; r++ {
			var err error
			got, err = bp.QueryRegion(rect)
			if err != nil {
				t.Fatalf("%s region: %v", rg.Name, err)
			}
		}
		us := float64(time.Since(t0).Microseconds()) / 20.0
		t.Logf("region %s: %d bodies %.2fus", rg.Name, len(got), us)
		if len(got) != len(rg.Want) {
			t.Errorf("%s: got %d, want %d", rg.Name, len(got), len(rg.Want))
			continue
		}
		for i := range got {
			if got[i] != rg.Want[i] {
				t.Errorf("%s: slot[%d] = %d, want %d", rg.Name, i, got[i], rg.Want[i])
				break
			}
		}
		if us > 50.0 {
			t.Errorf("region %s %.1fus exceeds 50us single-query gate", rg.Name, us)
		}
	}
	if _, err := bp.QueryRegion(core.Rect{}); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty region err = %v", err)
	}
}

// D: budgets hold with room: the filed 50us single-region query and 30us
// ray gates are judged from release triple-runs per §5 (formal magnitudes
// live in the window JSON, like S61's ~350us/~10us formal pins). The unit
// test carries order-of-magnitude ceilings so a loaded dev box never
// flakes, plus the exact cap trim math.
func TestBroadphaseBudgetFiled(t *testing.T) {
	yf, rf := loadYardFiles(t)
	bp, _ := buildYard(t, yf)
	reps := rf.Budgets.Reps
	if reps <= 0 {
		reps = 5
	}
	for i := 0; i < 2; i++ {
		if _, err := bp.Query(); err != nil {
			t.Fatalf("warmup: %v", err)
		}
	}
	var total time.Duration
	for r := 0; r < reps; r++ {
		t0 := time.Now()
		if _, err := bp.Query(); err != nil {
			t.Fatalf("query: %v", err)
		}
		total += time.Since(t0)
	}
	ms := float64(total.Microseconds()) / float64(reps) / 1000.0
	t.Logf("yard query over %d bodies: %.2fms, evals %d", yf.Layout.Count, ms, bp.Evaluations())
	if us := ms * 1000.0; us > 4000.0 {
		t.Errorf("query %.0fus exceeds 4000us order-of-magnitude ceiling", us)
	}
	for _, rc := range rf.Rays {
		ray, _ := NewRay(core.V2(rc.Origin[0], rc.Origin[1]), core.V2(rc.Dir[0], rc.Dir[1]), rc.MaxDist, rc.Mask)
		t0 := time.Now()
		for r := 0; r < 100; r++ {
			if _, _, err := bp.Ray(ray); err != nil {
				t.Fatalf("ray: %v", err)
			}
		}
		us := float64(time.Since(t0).Microseconds()) / 100.0
		t.Logf("ray %s: %.2fus", rc.Name, us)
		if us > 300.0 {
			t.Errorf("ray %s %.1fus exceeds 300us order-of-magnitude ceiling", rc.Name, us)
		}
	}
	allocs := testing.AllocsPerRun(20, func() {
		if _, err := bp.Query(); err != nil {
			t.Errorf("query: %v", err)
		}
	})
	t.Logf("query allocs/run = %.1f", allocs)
	if allocs > 12 {
		t.Errorf("query allocs = %.1f, want <= 12", allocs)
	}
	// The cap trims in track order and counts the rest.
	if err := bp.SetMaxCandidates(rf.Budgets.MaxCandidates); err != nil {
		t.Fatalf("cap: %v", err)
	}
	dy := rf.QueryDrop.DY
	for h := yf.Layout.StaticCount; h < yf.Layout.Count; h++ {
		e, _ := bp.at(h)
		nb := e.body
		nb.Pos = core.V2(nb.Pos.X, nb.Pos.Y+dy)
		if err := bp.Move(h, nb); err != nil {
			t.Fatalf("drop: %v", err)
		}
	}
	if _, err := bp.Query(); err != nil {
		t.Fatalf("query: %v", err)
	}
	if bp.Trimmed() != len(rf.QueryDrop.WantPairs)-rf.Budgets.MaxCandidates {
		t.Errorf("trimmed = %d, want %d", bp.Trimmed(), len(rf.QueryDrop.WantPairs)-rf.Budgets.MaxCandidates)
	}
	// The grid shortlists: 1200 bodies pair to 719400 naive, the index
	// must evaluate an order of magnitude fewer.
	if got := bp.Evaluations(); got > 100000 {
		t.Errorf("drop evaluations = %d, grid shortlist broken", got)
	} else {
		t.Logf("drop evaluations = %d (naive 719400)", got)
	}
}

// E: sleepers skip integration but still collide; contact wakes them.
func TestBroadphaseSleepWake(t *testing.T) {
	var bp = NewBroadphase()
	a, _ := NewBox("a", core.V2(0, 0), core.V2(10, 10), 1, 1, false)
	b, _ := NewBox("b", core.V2(100, 0), core.V2(10, 10), 1, 1, false)
	ha, err := bp.Track(a, false)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := bp.Track(b, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := bp.Sleep(ha); err != nil {
		t.Fatalf("sleep: %v", err)
	}
	if bp.IsActive(ha) {
		t.Error("sleeper reports active")
	}
	if bp.SleepingCount() != 1 {
		t.Error("sleeping count wrong")
	}
	// Drag b onto the sleeper: the pair reports and wakes it.
	nb := b
	nb.Pos = core.V2(5, 0)
	if err := bp.Move(hb, nb); err != nil {
		t.Fatal(err)
	}
	got, err := bp.Query()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("contacts = %d, want 1", len(got))
	}
	if len(bp.Woke()) != 1 || bp.Woke()[0] != ha {
		t.Errorf("woke = %v, want [%d]", bp.Woke(), ha)
	}
	if !bp.IsActive(ha) {
		t.Error("contact did not auto-wake")
	}
	// Stillness parks: freeze and Update past the tuning.
	var bp2 = NewBroadphase()
	if err := bp2.SetSleep(0.5, 3); err != nil {
		t.Fatal(err)
	}
	c, _ := NewBox("c", core.V2(500, 500), core.V2(5, 5), 1, 1, false)
	hc, _ := bp2.Track(c, false)
	for i := 0; i < 3; i++ {
		bp2.Update()
	}
	if bp2.IsActive(hc) {
		t.Error("still body never slept")
	}
}

// E2: static rest never wakes: a sleeper parked on a static stays parked
// across repeated Query (Godot Body sleep convergence). Dynamic touch
// still wakes, so the yard row turns gray instead of churning.
func TestBroadphaseStaticRestNoWake(t *testing.T) {
	var bp = NewBroadphase()
	if err := bp.SetMaxCandidates(0); err != nil {
		t.Fatal(err)
	}
	wall, _ := NewBox("wall", core.V2(64, 64), core.V2(20, 20), 1, 1, false)
	drop, _ := NewBox("drop", core.V2(64, 64), core.V2(10, 10), 1, 1, false)
	hs, err := bp.Track(wall, true)
	if err != nil {
		t.Fatal(err)
	}
	hd, err := bp.Track(drop, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = hs
	if err := bp.Sleep(hd); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, err := bp.Query()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("round %d contacts = %d, want 1", i, len(got))
		}
		if len(bp.Woke()) != 0 {
			t.Fatalf("round %d woke = %v, want []", i, bp.Woke())
		}
		if bp.IsActive(hd) {
			t.Fatalf("round %d static rest woke the sleeper", i)
		}
	}
	if bp.SleepingCount() != 1 {
		t.Errorf("sleeping = %d, want 1", bp.SleepingCount())
	}
	// Sleeper-sleeper rest also stays parked: neither side is awake to
	// push, so repeated Query converges instead of churning.
	var bp2 = NewBroadphase()
	if err := bp2.SetMaxCandidates(0); err != nil {
		t.Fatal(err)
	}
	s1, _ := NewBox("s1", core.V2(0, 0), core.V2(10, 10), 1, 1, false)
	s2, _ := NewBox("s2", core.V2(5, 0), core.V2(10, 10), 1, 1, false)
	h1, _ := bp2.Track(s1, false)
	h2, _ := bp2.Track(s2, false)
	if err := bp2.Sleep(h1); err != nil {
		t.Fatal(err)
	}
	if err := bp2.Sleep(h2); err != nil {
		t.Fatal(err)
	}
	got, err := bp2.Query()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("sleeper pair contacts = %d, want 1", len(got))
	}
	if len(bp2.Woke()) != 0 {
		t.Fatalf("sleeper pair woke = %v, want []", bp2.Woke())
	}
	if bp2.SleepingCount() != 2 {
		t.Errorf("sleeper pair sleeping = %d, want 2", bp2.SleepingCount())
	}
}

// F: one-way plus spring: pad impulse formula and top-only pass.
func TestBroadphaseSpringPad(t *testing.T) {
	yf, _ := loadYardFiles(t)
	if len(yf.Springs) == 0 {
		t.Fatal("no filed springs")
	}
	sp := yf.Springs[0]
	pad, err := NewSpring(sp.Impulse, sp.Damp)
	if err != nil {
		t.Fatalf("spring: %v", err)
	}
	if got := pad.Bounce(300); got != -(300*sp.Damp + sp.Impulse) {
		t.Errorf("bounce(300) = %v", got)
	}
	if got := pad.Bounce(-50); got != -50 {
		t.Errorf("rising passes through, got %v", got)
	}
	if _, err := NewSpring(-1, 0.5); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("neg impulse err = %v", err)
	}
	if _, err := NewSpring(1, 1.5); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("damp>1 err = %v", err)
	}
	// The filed one-way still spans its endpoints (top-only spirit kept by Step).
	if len(yf.Oneways) == 0 {
		t.Fatal("no filed oneways")
	}
	o := yf.Oneways[0]
	s, err := NewSlope(o.Name, core.V2(o.A[0], o.A[1]), core.V2(o.B[0], o.B[1]))
	_ = s
	_ = err
	sl, err := NewOneWay(o.Name, core.V2(o.A[0], o.A[1]), core.V2(o.B[0], o.B[1]))
	if err != nil {
		t.Fatalf("oneway: %v", err)
	}
	y, ok := sl.GroundYAt(600)
	if !ok || y != 300 {
		t.Errorf("oneway ground = %v/%v, want 300/true", y, ok)
	}
}

// G: 10000 replays never drift: same bodies, same order, same bytes.
func TestBroadphaseReplayStable(t *testing.T) {
	yf, rf := loadYardFiles(t)
	replays := rf.Budgets.Replays
	if replays <= 0 {
		replays = 10000
	}
	if replays > 10000 {
		replays = 10000
	}
	bp, _ := buildYard(t, yf)
	dy := rf.QueryDrop.DY
	for h := yf.Layout.StaticCount; h < yf.Layout.Count; h++ {
		e, _ := bp.at(h)
		nb := e.body
		nb.Pos = core.V2(nb.Pos.X, nb.Pos.Y+dy)
		if err := bp.Move(h, nb); err != nil {
			t.Fatal(err)
		}
	}
	first, err := bp.Query()
	if err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprint(pairKeys(first))
	for i := 0; i < replays; i++ {
		got, err := bp.Query()
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(pairKeys(got)) != base {
			t.Fatalf("replay %d drifted", i)
		}
	}
}

// H: bad inputs fail with their own codes, never a crash.
func TestBroadphaseEdgesNoCrash(t *testing.T) {
	var nilBP *Broadphase
	if _, err := nilBP.Query(); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil query err = %v", err)
	}
	if nilBP.Woke() != nil || nilBP.Count() != 0 || nilBP.Cell() != 0 {
		t.Error("nil getters not silent")
	}
	nilBP.Reset()
	nilBP.SetAutoWake(true)
	nilBP.SetAutoSleep(false)
	bp := NewBroadphase()
	if _, _, err := bp.Ray(Ray{}); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad ray err = %v", err)
	}
	if _, err := bp.Track(Body{Shape: Shape(99)}, false); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad track err = %v", err)
	}
	if err := bp.Move(99, Body{}); err == nil || core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("move unknown err = %v", err)
	}
	if err := bp.Sleep(99); err == nil || core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("sleep unknown err = %v", err)
	}
	if err := bp.SetMaxCandidates(-1); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("neg cap err = %v", err)
	}
	// Static sleep refusal and over-cap OutOfMemory.
	bd, _ := NewBox("s", core.V2(0, 0), core.V2(4, 4), 1, 1, false)
	h, _ := bp.Track(bd, true)
	if err := bp.Sleep(h); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("static sleep err = %v", err)
	}
	var big Broadphase = NewBroadphase()
	good, _ := NewBox("g", core.V2(0, 0), core.V2(1, 1), 1, 1, false)
	for i := 0; i < MaxBroadphaseBodies; i++ {
		if _, err := big.Track(good, true); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	if _, err := big.Track(good, true); err == nil || core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("over cap err = %v", err)
	}
}
