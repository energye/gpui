package physics

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type bodyDef struct {
	Name    string     `json:"name"`
	Shape   string     `json:"shape"`
	Pos     [2]float64 `json:"pos"`
	Half    [2]float64 `json:"half"`
	Radius  float64    `json:"radius"`
	Layer   uint32     `json:"layer"`
	Mask    uint32     `json:"mask"`
	Trigger bool       `json:"trigger"`
}

type overlapDef struct {
	A    string `json:"a"`
	B    string `json:"b"`
	Want bool   `json:"want"`
}

type collideDef struct {
	A    string `json:"a"`
	B    string `json:"b"`
	Want bool   `json:"want"`
}

type queryDef struct {
	Name        string     `json:"name"`
	Bodies      []string   `json:"bodies"`
	Want        [][]string `json:"want"`
	WantTrigger []bool     `json:"want_trigger"`
}

type bodyFile struct {
	Bodies   []bodyDef    `json:"bodies"`
	Overlaps []overlapDef `json:"overlaps"`
	Collides []collideDef `json:"collides"`
	Queries  []queryDef   `json:"queries"`
}

func loadBodyCases(t *testing.T) bodyFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "body_cases.json"))
	if err != nil {
		t.Fatalf("read body_cases.json: %v", err)
	}
	var f bodyFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode body_cases.json: %v", err)
	}
	if len(f.Bodies) == 0 || len(f.Overlaps) == 0 || len(f.Queries) == 0 {
		t.Fatal("body_cases.json has no bodies, overlaps, or queries")
	}
	return f
}

func buildBody(t *testing.T, d bodyDef) Body {
	t.Helper()
	pos := core.V2(d.Pos[0], d.Pos[1])
	switch d.Shape {
	case "box":
		b, err := NewBox(d.Name, pos, core.V2(d.Half[0], d.Half[1]), d.Layer, d.Mask, d.Trigger)
		if err != nil {
			t.Fatalf("NewBox %q: %v", d.Name, err)
		}
		return b
	case "circle":
		b, err := NewCircle(d.Name, pos, d.Radius, d.Layer, d.Mask, d.Trigger)
		if err != nil {
			t.Fatalf("NewCircle %q: %v", d.Name, err)
		}
		return b
	default:
		t.Fatalf("unknown shape %q for %q", d.Shape, d.Name)
		return Body{}
	}
}

func bodyMap(t *testing.T, f bodyFile) map[string]Body {
	t.Helper()
	m := map[string]Body{}
	for _, d := range f.Bodies {
		if _, dup := m[d.Name]; dup {
			t.Fatalf("duplicate body %q", d.Name)
		}
		m[d.Name] = buildBody(t, d)
	}
	return m
}

func mustFindQuery(t *testing.T, f bodyFile, name string) queryDef {
	t.Helper()
	for _, q := range f.Queries {
		if q.Name == name {
			return q
		}
	}
	t.Fatalf("body_cases.json has no query %q", name)
	return queryDef{}
}

func mustFindBody(t *testing.T, m map[string]Body, what, name string) Body {
	t.Helper()
	b, ok := m[name]
	if !ok {
		t.Fatalf("%s unknown body %q", what, name)
	}
	return b
}

func queryBodies(t *testing.T, m map[string]Body, q queryDef) []Body {
	t.Helper()
	bodies := make([]Body, 0, len(q.Bodies))
	for _, n := range q.Bodies {
		bodies = append(bodies, mustFindBody(t, m, "query "+q.Name, n))
	}
	return bodies
}

// expectBothWays asserts f agrees on both argument orders (overlap and
// mask checks are symmetric by contract).
func expectBothWays(t *testing.T, what string, a, b Body, want bool, f func(Body, Body) bool) {
	t.Helper()
	if got := f(a, b); got != want {
		t.Errorf("%s %s+%s = %v, want %v", what, a.Name, b.Name, got, want)
	}
	if got := f(b, a); got != want {
		t.Errorf("%s %s+%s swapped = %v, want %v", what, b.Name, a.Name, got, want)
	}
}

// expectSilent asserts a query reports nothing with no error.
func expectSilent(t *testing.T, what string, bodies []Body) {
	t.Helper()
	if got, err := Query(bodies); err != nil || len(got) != 0 {
		t.Errorf("%s Query = %v/%v, want empty/nil", what, got, err)
	}
}

// A:撞和触发对:几何相交加层分组加触发标记全落在冻结数上.
func TestBodyOverlapsFromCases(t *testing.T) {
	f := loadBodyCases(t)
	m := bodyMap(t, f)
	for _, o := range f.Overlaps {
		a := mustFindBody(t, m, "overlap", o.A)
		b := mustFindBody(t, m, "overlap", o.B)
		expectBothWays(t, "overlap", a, b, o.Want, Overlaps)
	}
	for _, c := range f.Collides {
		a := mustFindBody(t, m, "collide", c.A)
		b := mustFindBody(t, m, "collide", c.B)
		expectBothWays(t, "collide", a, b, c.Want, CanCollide)
	}
	for _, q := range f.Queries {
		bodies := queryBodies(t, m, q)
		got, err := Query(bodies)
		if err != nil {
			t.Errorf("query %s: %v", q.Name, err)
			continue
		}
		if len(got) != len(q.Want) {
			t.Errorf("query %s contacts = %v, want %v", q.Name, contacts(got), q.Want)
			continue
		}
		for i, w := range q.Want {
			if len(w) != 2 || got[i].A != w[0] || got[i].B != w[1] {
				t.Errorf("query %s pair[%d] = %s+%s, want %s+%s", q.Name, i, got[i].A, got[i].B, w[0], w[1])
			}
			if i < len(q.WantTrigger) && got[i].Trigger != q.WantTrigger[i] {
				t.Errorf("query %s pair[%d] trigger = %v, want %v", q.Name, i, got[i].Trigger, q.WantTrigger[i])
			}
			if got[i].AI >= got[i].BI {
				t.Errorf("query %s pair[%d] indices %d/%d not ordered", q.Name, i, got[i].AI, got[i].BI)
			}
		}
	}
}

func contacts(cs []Contact) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.A + "+" + c.B
	}
	return out
}

// sameBody compares two bodies NaN-aware: NaN never equals itself, so a
// plain != would cry mutation on untouched NaN inputs.
func sameBody(a, b Body) bool {
	return a.Name == b.Name && a.Shape == b.Shape &&
		sameFloat(a.Pos.X, b.Pos.X) && sameFloat(a.Pos.Y, b.Pos.Y) &&
		sameFloat(a.Half.X, b.Half.X) && sameFloat(a.Half.Y, b.Half.Y) &&
		sameFloat(a.Radius, b.Radius) &&
		a.Layer == b.Layer && a.Mask == b.Mask && a.Trigger == b.Trigger
}

func sameFloat(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return a == b
}

// B:同位空零超大坏数据全不崩不卡死,占位加报错.
func TestBodyEdgesNoCrash(t *testing.T) {
	// Empty and nil queries report nothing, never an error.
	expectSilent(t, "nil", nil)
	expectSilent(t, "empty", []Body{})
	solo, err := NewBox("solo", core.V2(0, 0), core.V2(5, 5), 1, 1, false)
	if err != nil {
		t.Fatalf("solo: %v", err)
	}
	expectSilent(t, "single", []Body{solo})

	// Same position still overlaps (no divide, no NaN), masked pair stays quiet.
	dup, err := NewBox("dup", core.V2(0, 0), core.V2(5, 5), 1, 1, false)
	if err != nil {
		t.Fatalf("dup: %v", err)
	}
	if !Overlaps(solo, dup) {
		t.Error("same-position boxes report no overlap, want true")
	}
	ghost, err := NewBox("ghost", core.V2(0, 0), core.Vec2{}, 0, 0, false)
	if err != nil {
		// Zero half is legal; only a constructor rejection needs a code check.
		if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("zero-half code = %v, want invalid-arg", core.CodeOf(err))
		}
	} else {
		if !ghost.Valid() {
			t.Error("zero-half box reports invalid, want valid point")
		}
		if _, ok := Bounds(ghost); !ok {
			t.Error("zero-half Bounds ok=false, want true")
		}
	}

	// Zero-size points overlap only when they share the exact spot.
	p0, _ := NewBox("p0", core.V2(3, 4), core.Vec2{}, 1, 1, false)
	p1, _ := NewBox("p1", core.V2(3, 4), core.Vec2{}, 1, 1, false)
	p2, _ := NewBox("p2", core.V2(3.5, 4), core.Vec2{}, 1, 1, false)
	if !Overlaps(p0, p1) {
		t.Error("same-spot points report no overlap, want true")
	}
	if Overlaps(p0, p2) {
		t.Error("offset points report overlap, want false")
	}
	c0, _ := NewCircle("c0", core.V2(0, 0), 0, 1, 1, false)
	if !Overlaps(p0, p0) {
		t.Error("self overlap = false, want true")
	}
	if Overlaps(c0, p2) {
		t.Error("far point-circle reports overlap, want false")
	}

	// Bad constructors store nothing and name the fault.
	badPos := []core.Vec2{{X: math.NaN()}, {Y: math.Inf(1)}, {X: math.Inf(-1), Y: 1}}
	for _, p := range badPos {
		if _, err := NewBox("bad", p, core.V2(1, 1), 1, 1, false); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad pos %v code = %v, want invalid-arg", p, core.CodeOf(err))
		}
		if _, err := NewCircle("bad", p, 1, 1, 1, false); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad circle pos %v code = %v, want invalid-arg", p, core.CodeOf(err))
		}
	}
	for _, h := range []core.Vec2{{X: -1, Y: 1}, {X: 1, Y: -2}, {X: math.NaN()}, {Y: math.Inf(1)}} {
		if _, err := NewBox("bad", core.V2(0, 0), h, 1, 1, false); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad half %v code = %v, want invalid-arg", h, core.CodeOf(err))
		}
	}
	for _, r := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := NewCircle("bad", core.V2(0, 0), r, 1, 1, false); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad radius %v code = %v, want invalid-arg", r, core.CodeOf(err))
		}
	}

	// Hand-built bad bodies fail closed: no overlap, no bounds, query errors.
	bad := []Body{
		{Name: "nan-pos", Shape: ShapeBox, Pos: core.V2(math.NaN(), 0), Half: core.V2(1, 1), Layer: 1, Mask: 1},
		{Name: "neg-half", Shape: ShapeBox, Pos: core.V2(0, 0), Half: core.V2(-1, 1), Layer: 1, Mask: 1},
		{Name: "nan-r", Shape: ShapeCircle, Pos: core.V2(0, 0), Radius: math.NaN(), Layer: 1, Mask: 1},
		{Name: "neg-r", Shape: ShapeCircle, Pos: core.V2(0, 0), Radius: -3, Layer: 1, Mask: 1},
		{Name: "weird", Shape: Shape(9), Pos: core.V2(0, 0), Half: core.V2(1, 1), Layer: 1, Mask: 1},
	}
	for _, b := range bad {
		if b.Valid() {
			t.Errorf("%s reports valid, want false", b.Name)
		}
		if Overlaps(b, solo) || Overlaps(solo, b) {
			t.Errorf("%s overlaps a good body, want false", b.Name)
		}
		if _, ok := Bounds(b); ok {
			t.Errorf("%s Bounds ok=true, want false", b.Name)
		}
		mix := []Body{solo, b}
		snap := append([]Body(nil), mix...)
		if out, err := Query(mix); err == nil || out != nil {
			t.Errorf("%s Query = %v/%v, want nil + error", b.Name, out, err)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("%s Query code = %v, want invalid-arg", b.Name, core.CodeOf(err))
		}
		for i := range mix {
			if !sameBody(mix[i], snap[i]) {
				t.Errorf("%s Query mutated input", b.Name)
				break
			}
		}
	}

	// Huge-but-finite bodies never panic; order-insensitive masks stay quiet.
	huge, _ := NewBox("huge", core.V2(1e308, 0), core.V2(1e308, 1), 1, 1, false)
	tiny, _ := NewBox("tiny", core.V2(-1e308, 0), core.V2(1, 1), 1, 1, false)
	_ = Overlaps(huge, tiny)
	_ = Overlaps(tiny, huge)
	if _, ok := Bounds(huge); !ok {
		t.Error("huge Bounds ok=false, want true")
	}
	nomask, _ := NewBox("nomask", core.V2(0, 0), core.V2(10, 10), 0, 0, false)
	if CanCollide(solo, nomask) || CanCollide(nomask, solo) {
		t.Error("zero-mask CanCollide = true, want false")
	}
	if got, err := Query([]Body{solo, nomask}); err != nil || len(got) != 0 {
		t.Errorf("zero-mask Query = %v/%v, want empty/nil", got, err)
	}
	// Empty names are legal debug keys and stay distinguishable by index.
	anon0, _ := NewBox("", core.V2(0, 0), core.V2(5, 5), 1, 1, false)
	anon1, _ := NewBox("", core.V2(0, 0), core.V2(5, 5), 1, 1, false)
	got, err := Query([]Body{anon0, anon1})
	if err != nil || len(got) != 1 || got[0].AI != 0 || got[0].BI != 1 {
		t.Errorf("anon Query = %v/%v, want one 0/1 contact", got, err)
	}
}

// C:纯算数不画画,两边同数靠边界往返无损加逐位重放(C不适用像素,记边界等价).
func TestBodyBoundaryIdentical(t *testing.T) {
	f := loadBodyCases(t)
	m := bodyMap(t, f)
	// Render boundary is lossless for every frozen center.
	for name, b := range m {
		if back := core.Vec2FromRenderPoint(b.Pos.ToRenderPoint()); back != b.Pos {
			t.Errorf("%s boundary = %v, want %v", name, back, b.Pos)
		}
		if r, ok := Bounds(b); ok {
			if back := core.Vec2FromRenderPoint(r.Center().ToRenderPoint()); back != r.Center() {
				t.Errorf("%s bounds center boundary diverged", name)
			}
		} else {
			t.Errorf("%s Bounds ok=false, want true", name)
		}
	}
	// Same query replays contact-for-contact, flag-for-flag.
	bodies := queryBodies(t, m, mustFindQuery(t, f, "mixed"))
	snap := append([]Body(nil), bodies...)
	a, err := Query(bodies)
	if err != nil {
		t.Fatalf("mixed Query: %v", err)
	}
	b, err := Query(bodies)
	if err != nil {
		t.Fatalf("mixed replay: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("replay len = %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("replay diverged at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
	for i := range bodies {
		if bodies[i] != snap[i] {
			t.Fatal("query mutated the input")
		}
	}
	// Result is fresh: writing it cannot alias the input.
	if len(a) > 0 {
		probe := append([]Contact(nil), a...)
		a[0].A = "mutated-probe"
		if bodies[a[0].AI].Name == "mutated-probe" {
			t.Error("result aliases input")
		}
		a = probe
	}
}

// D:百盒查询跑得动,耗时调用有数.
func TestBodyPerfHundred(t *testing.T) {
	// Synthetic load only (no golden): golden stays in body_cases.json.
	// Seeded rand keeps the load replayable.
	r := core.NewRand(20260915)
	const n = 100
	bodies := make([]Body, n)
	for i := 0; i < n; i++ {
		pos := core.V2(r.RangeFloat(-500, 500), r.RangeFloat(-500, 500))
		b, err := NewBox("p", pos, core.V2(8, 8), 1, 1, i%10 == 0)
		if err != nil {
			t.Fatalf("box %d: %v", i, err)
		}
		bodies[i] = b
	}
	const reps = 2000
	var pairs int
	start := time.Now()
	for i := 0; i < reps; i++ {
		got, err := Query(bodies)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		pairs += len(got)
	}
	el := time.Since(start)
	t.Logf("body-100: %d queries x %d boxes in %v (%.1f us/query, %d pairs total)", reps, n, el, float64(el.Microseconds())/reps, pairs)
}

// E:长跑不穿:万次重放逐位一致,走格往返不粘.
func TestBodyLongRunStable(t *testing.T) {
	f := loadBodyCases(t)
	m := bodyMap(t, f)
	bodies := queryBodies(t, m, mustFindQuery(t, f, "mixed"))
	first, err := Query(bodies)
	if err != nil {
		t.Fatalf("mixed Query: %v", err)
	}
	snap := append([]Body(nil), bodies...)
	for i := 0; i < 10000; i++ {
		got, err := Query(bodies)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if len(got) != len(first) {
			t.Fatalf("rep %d len = %d, want %d", i, len(got), len(first))
		}
		for j := range first {
			if got[j] != first[j] {
				t.Fatalf("rep %d drifted at %d: %+v vs %+v", i, j, got[j], first[j])
			}
		}
	}
	for i := range bodies {
		if bodies[i] != snap[i] {
			t.Fatal("10k queries mutated the input")
		}
	}
	// A hero walks through a zone and back: touch flips and returns, the
	// trigger flag follows the zone, never the walker.
	wall, _ := NewBox("wall", core.V2(0, 0), core.V2(5, 5), 1, 1, false)
	coin, _ := NewBox("coin", core.V2(0, 0), core.V2(5, 5), 1, 1, true)
	walks := []struct {
		name    string
		other   Body
		trigger bool
	}{
		{name: "wall", other: wall, trigger: false},
		{name: "coin", other: coin, trigger: true},
	}
	for _, w := range walks {
		for _, x := range []float64{-20, 0, -20} {
			hero, _ := NewBox("hero", core.V2(x, 0), core.V2(5, 5), 1, 1, false)
			got, err := Query([]Body{hero, w.other})
			if err != nil {
				t.Fatalf("%s walk %v: %v", w.name, x, err)
			}
			if x == 0 {
				if len(got) != 1 || got[0].Trigger != w.trigger {
					t.Errorf("%s walk x=%v = %v, want one pair trigger=%v", w.name, x, got, w.trigger)
				}
			} else if len(got) != 0 {
				t.Errorf("%s walk x=%v pairs = %d, want 0", w.name, x, len(got))
			}
		}
	}
}

// F:离屏金对照窗(W1免窗,game_physics后建):冻结数加形状断言.
func TestBodyOffscreenGolden(t *testing.T) {
	f := loadBodyCases(t)
	m := bodyMap(t, f)
	// Golden numbers stay frozen.
	for _, o := range f.Overlaps {
		if got := Overlaps(m[o.A], m[o.B]); got != o.Want {
			t.Fatalf("golden overlap %s+%s = %v, want %v", o.A, o.B, got, o.Want)
		}
	}
	// Shape: edge touch counts (stand on platform), a hair gap does not.
	if !Overlaps(m["box_a"], m["box_touch"]) {
		t.Error("edge touch = false, want true (stand counts as touch)")
	}
	if Overlaps(m["box_a"], m["box_gap"]) {
		t.Error("0.5 gap = true, want false")
	}
	// Shape: circle hugs the box edge; one step out lets go.
	if !Overlaps(m["box_c"], m["circle_edge"]) {
		t.Error("circle edge = false, want true")
	}
	if Overlaps(m["box_c"], m["circle_out"]) {
		t.Error("circle out = true, want false")
	}
	// Shape: geometry says touch, masks say no: query stays silent.
	if !Overlaps(m["box_a"], m["ghost"]) {
		t.Error("box+ghost geometry = false, want true")
	}
	if CanCollide(m["box_a"], m["ghost"]) {
		t.Error("box+ghost masks = true, want false")
	}
	if got, _ := Query([]Body{m["hero"], m["ghost"]}); len(got) != 0 {
		t.Errorf("masked query = %v, want empty", got)
	}
	// Shape: trigger is reported, never dropped, flagged for the caller.
	trig, err := Query([]Body{m["hero"], m["coin_near"]})
	if err != nil || len(trig) != 1 || !trig[0].Trigger {
		t.Errorf("trigger pair = %v/%v, want one flagged contact", trig, err)
	}
	solid, err := Query([]Body{m["hero"], m["wall"]})
	if err != nil || len(solid) != 1 || solid[0].Trigger {
		t.Errorf("solid pair = %v/%v, want one unflagged contact", solid, err)
	}
	// Shape: box bounds are exact spans; circle bounds are diameters.
	if r, _ := Bounds(m["hero"]); r.X != -5 || r.Y != -5 || r.W != 10 || r.H != 10 {
		t.Errorf("hero bounds = %v, want {-5 -5 10 10}", r)
	}
	if r, _ := Bounds(m["circle_a"]); r.X != -5 || r.Y != -5 || r.W != 10 || r.H != 10 {
		t.Errorf("circle bounds = %v, want {-5 -5 10 10}", r)
	}
	// Mixed golden order is input order i<j with flags attached.
	mixed := mustFindQuery(t, f, "mixed")
	got, err := Query(queryBodies(t, m, mixed))
	if err != nil {
		t.Fatalf("mixed Query: %v", err)
	}
	if len(got) != len(mixed.Want) {
		t.Fatalf("mixed golden len = %d, want %d", len(got), len(mixed.Want))
	}
	for i, w := range mixed.Want {
		if got[i].A != w[0] || got[i].B != w[1] || got[i].Trigger != mixed.WantTrigger[i] {
			t.Errorf("mixed[%d] = %s+%s trig=%v, want %s+%s trig=%v",
				i, got[i].A, got[i].B, got[i].Trigger, w[0], w[1], mixed.WantTrigger[i])
		}
	}
}
