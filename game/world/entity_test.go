package world

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsEntity = 1e-9

type nodeCase struct {
	Name   string     `json:"name"`
	Parent string     `json:"parent"`
	Pos    [2]float64 `json:"pos"`
	Rot    float64    `json:"rot"`
	Scale  [2]float64 `json:"scale"`
}

type wantCase struct {
	Name  string     `json:"name"`
	Pos   [2]float64 `json:"pos"`
	Rot   float64    `json:"rot"`
	Scale [2]float64 `json:"scale"`
	Mat   [6]float64 `json:"mat"`
}

type chainCase struct {
	Name  string     `json:"name"`
	Nodes []nodeCase `json:"nodes"`
	Want  []wantCase `json:"want_world"`
}

type compAdd struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type compCase struct {
	Entity string    `json:"entity"`
	Add    []compAdd `json:"add"`
}

type entityFile struct {
	Chains []chainCase `json:"chains"`
	Comps  []compCase  `json:"comps"`
	Perf   struct {
		Entities int `json:"entities"`
		Reps     int `json:"reps"`
	} `json:"perf"`
	Longrun struct {
		Cycles int `json:"cycles"`
		Batch  int `json:"batch"`
	} `json:"longrun"`
}

func loadEntityCases(t *testing.T) entityFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "entity_cases.json"))
	if err != nil {
		t.Fatalf("read entity_cases.json: %v", err)
	}
	var f entityFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode entity_cases.json: %v", err)
	}
	if len(f.Chains) == 0 || len(f.Comps) == 0 {
		t.Fatal("entity_cases.json has no cases")
	}
	return f
}

func mustSpawn(t *testing.T, w *World, parent ID) ID {
	t.Helper()
	id, err := w.Spawn(parent)
	if err != nil {
		t.Fatalf("Spawn(%d): %v", parent, err)
	}
	if id == NoEntity || !w.Alive(id) {
		t.Fatalf("Spawn(%d) = %d, want a live id", parent, id)
	}
	return id
}

func buildChain(t *testing.T, w *World, ch chainCase) map[string]ID {
	t.Helper()
	ids := map[string]ID{}
	for _, n := range ch.Nodes {
		var parent ID
		if n.Parent != "" {
			var ok bool
			parent, ok = ids[n.Parent]
			if !ok {
				t.Fatalf("%s: parent %q spawns after its child", ch.Name, n.Parent)
			}
		}
		id := mustSpawn(t, w, parent)
		if _, dup := ids[n.Name]; dup {
			t.Fatalf("%s: duplicate node %q", ch.Name, n.Name)
		}
		ids[n.Name] = id
		loc := Transform{Pos: core.V2(n.Pos[0], n.Pos[1]), Rot: n.Rot, Scale: core.V2(n.Scale[0], n.Scale[1])}
		if err := w.SetTransform(id, loc); err != nil {
			t.Fatalf("%s/%s SetTransform: %v", ch.Name, n.Name, err)
		}
	}
	return ids
}

func near(a, b float64) bool { return math.Abs(a-b) < epsEntity }

func checkWant(t *testing.T, tag string, w *World, ids map[string]ID, want wantCase) {
	t.Helper()
	id, ok := ids[want.Name]
	if !ok {
		t.Fatalf("%s: unknown node %q", tag, want.Name)
	}
	got, err := w.WorldOf(id)
	if err != nil {
		t.Fatalf("%s/%s WorldOf: %v", tag, want.Name, err)
	}
	if !near(got.Pos.X, want.Pos[0]) || !near(got.Pos.Y, want.Pos[1]) {
		t.Errorf("%s/%s world pos = %v, want %v", tag, want.Name, got.Pos, want.Pos)
	}
	if !near(got.Rot, want.Rot) {
		t.Errorf("%s/%s world rot = %.17g, want %.17g", tag, want.Name, got.Rot, want.Rot)
	}
	if !near(got.Scale.X, want.Scale[0]) || !near(got.Scale.Y, want.Scale[1]) {
		t.Errorf("%s/%s world scale = %v, want %v", tag, want.Name, got.Scale, want.Scale)
	}
	m, err := w.WorldMatrix(id)
	if err != nil {
		t.Fatalf("%s/%s WorldMatrix: %v", tag, want.Name, err)
	}
	c := [6]float64{m.A, m.B, m.C, m.D, m.E, m.F}
	for i := range c {
		if !near(c[i], want.Mat[i]) {
			t.Errorf("%s/%s world mat = %v, want %v", tag, want.Name, c, want.Mat)
			break
		}
	}
}

func expectEntityCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

// A:父子变换继承对:平移叠加、缩放连乘、旋转带着转,矩阵和分解两边都落在冻结数上.
func TestEntityHierarchyFromCases(t *testing.T) {
	f := loadEntityCases(t)
	for _, ch := range f.Chains {
		w := NewWorld()
		ids := buildChain(t, &w, ch)
		if w.Count() != len(ch.Nodes) {
			t.Errorf("%s: count = %d, want %d", ch.Name, w.Count(), len(ch.Nodes))
		}
		if w.Spawned() != uint64(len(ch.Nodes)) {
			t.Errorf("%s: spawned = %d, want %d", ch.Name, w.Spawned(), len(ch.Nodes))
		}
		for _, n := range ch.Nodes {
			id := ids[n.Name]
			loc, err := w.Local(id)
			if err != nil {
				t.Fatalf("%s/%s Local: %v", ch.Name, n.Name, err)
			}
			if loc.Pos.X != n.Pos[0] || loc.Pos.Y != n.Pos[1] || loc.Rot != n.Rot ||
				loc.Scale.X != n.Scale[0] || loc.Scale.Y != n.Scale[1] {
				t.Errorf("%s/%s local moved on store", ch.Name, n.Name)
			}
			gotParent, err := w.Parent(id)
			if err != nil {
				t.Fatalf("%s/%s Parent: %v", ch.Name, n.Name, err)
			}
			if gotParent != ids[n.Parent] {
				t.Errorf("%s/%s parent = %d, want %d", ch.Name, n.Name, gotParent, ids[n.Parent])
			}
		}
		for _, want := range ch.Want {
			checkWant(t, ch.Name, &w, ids, want)
		}
	}
	// Comps hang in add order with the frozen kinds and asset refs.
	w := NewWorld()
	for _, cc := range f.Comps {
		id := mustSpawn(t, &w, NoEntity)
		for _, a := range cc.Add {
			if err := w.AddComp(id, Comp{Kind: a.Kind, Ref: core.AssetID(a.Ref)}); err != nil {
				t.Fatalf("%s AddComp %q: %v", cc.Entity, a.Kind, err)
			}
			if !w.HasComp(id, a.Kind) {
				t.Errorf("%s HasComp(%q) = false after add", cc.Entity, a.Kind)
			}
		}
		got, err := w.Comps(id)
		if err != nil {
			t.Fatalf("%s Comps: %v", cc.Entity, err)
		}
		if len(got) != len(cc.Add) {
			t.Fatalf("%s comps = %d, want %d", cc.Entity, len(got), len(cc.Add))
		}
		for i, a := range cc.Add {
			if got[i].Kind != a.Kind || string(got[i].Ref) != a.Ref {
				t.Errorf("%s comp[%d] = %+v, want {%s %s}", cc.Entity, i, got[i], a.Kind, a.Ref)
			}
		}
		// Remove drops the first match; a second remove reports not-found.
		first := cc.Add[0].Kind
		if err := w.RemoveComp(id, first); err != nil {
			t.Errorf("%s RemoveComp(%q): %v", cc.Entity, first, err)
		}
		if len(cc.Add) > 1 && !w.HasComp(id, cc.Add[1].Kind) {
			t.Errorf("%s lost %q after removing %q", cc.Entity, cc.Add[1].Kind, first)
		}
		if err := w.RemoveComp(id, first); len(cc.Add) == 1 {
			if core.CodeOf(err) != core.CodeNotFound {
				t.Errorf("%s second RemoveComp(%q) code = %v, want not-found", cc.Entity, first, core.CodeOf(err))
			}
		}
		expectEntityCode(t, cc.Entity+" remove missing", w.RemoveComp(id, "no-such-comp"), core.CodeNotFound)
	}
}

// B:删爹不崩:孩子落回根守住本地数,坏链坏数错码分清,nil接收器不炸.
func TestEntityEdgesNoCrash(t *testing.T) {
	f := loadEntityCases(t)
	ch := f.Chains[0]
	w := NewWorld()
	ids := buildChain(t, &w, ch)
	root, child, grand := ids["root"], ids["child"], ids["grand"]

	// Delete the father: children fall back to roots with locals intact.
	if err := w.Despawn(root); err != nil {
		t.Fatalf("Despawn(root): %v", err)
	}
	if w.Alive(root) {
		t.Error("root still alive after Despawn")
	}
	if p, _ := w.Parent(child); p != NoEntity {
		t.Errorf("orphaned child parent = %d, want root", p)
	}
	cl, _ := w.Local(child)
	cw, err := w.WorldOf(child)
	if err != nil {
		t.Fatalf("orphaned child WorldOf: %v", err)
	}
	if cw != cl {
		t.Errorf("orphaned child world = %+v, want local %+v", cw, cl)
	}
	gw, _ := w.WorldOf(grand)
	if !near(gw.Pos.X, 5) || !near(gw.Pos.Y, 7) {
		t.Errorf("grand world after root loss = %v, want (5,7)", gw.Pos)
	}
	kids, err := w.Children(child)
	if err != nil || len(kids) != 1 || kids[0] != grand {
		t.Errorf("orphaned child kids = %v,%v, want [grand]", kids, err)
	}

	// Missing ids fail closed with not-found, never a crash.
	expectEntityCode(t, "despawn missing", w.Despawn(9999), core.CodeNotFound)
	expectEntityCode(t, "despawn twice", w.Despawn(root), core.CodeNotFound)
	if _, err := w.Spawn(9999); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("spawn under dead code = %v, want not-found", core.CodeOf(err))
	}
	expectEntityCode(t, "parent of dead", func() error { _, err := w.Parent(root); return err }(), core.CodeNotFound)
	expectEntityCode(t, "world of dead", func() error { _, err := w.WorldOf(root); return err }(), core.CodeNotFound)

	// Bad links are rejected, never guessed.
	w2 := NewWorld()
	a := mustSpawn(t, &w2, NoEntity)
	b := mustSpawn(t, &w2, a)
	c := mustSpawn(t, &w2, b)
	expectEntityCode(t, "self parent", w2.SetParent(a, a), core.CodeInvalidArg)
	expectEntityCode(t, "cycle to grandchild", w2.SetParent(a, c), core.CodeInvalidArg)
	expectEntityCode(t, "cycle to child", w2.SetParent(a, b), core.CodeInvalidArg)
	expectEntityCode(t, "parent missing", w2.SetParent(a, 9999), core.CodeNotFound)
	expectEntityCode(t, "child missing", w2.SetParent(9999, a), core.CodeNotFound)
	if err := w2.SetParent(b, a); err != nil {
		t.Errorf("re-link to same parent: %v, want no-op nil", err)
	}
	if err := w2.SetParent(c, NoEntity); err != nil {
		t.Fatalf("detach: %v", err)
	}
	if p, _ := w2.Parent(c); p != NoEntity {
		t.Errorf("detached parent = %d, want root", p)
	}
	if err := w2.SetParent(c, a); err != nil {
		t.Fatalf("reattach: %v", err)
	}

	// Bad numbers store nothing; bad comps are refused.
	before, _ := w2.Local(a)
	for _, bad := range []Transform{
		{Pos: core.V2(math.NaN(), 0), Scale: core.V2(1, 1)},
		{Pos: core.V2(0, math.Inf(1)), Scale: core.V2(1, 1)},
		{Rot: math.NaN(), Scale: core.V2(1, 1)},
		{Scale: core.V2(1, math.Inf(-1))},
	} {
		expectEntityCode(t, "bad transform", w2.SetTransform(a, bad), core.CodeInvalidArg)
	}
	if after, _ := w2.Local(a); after != before {
		t.Error("rejected transform moved the stored local")
	}
	expectEntityCode(t, "empty comp kind", w2.AddComp(a, Comp{}), core.CodeInvalidArg)
	expectEntityCode(t, "empty remove kind", w2.RemoveComp(a, ""), core.CodeInvalidArg)
	tmp := mustSpawn(t, &w2, NoEntity)
	if err := w2.Despawn(tmp); err != nil {
		t.Fatalf("despawn tmp: %v", err)
	}
	expectEntityCode(t, "add on dead", w2.AddComp(tmp, Comp{Kind: "sprite"}), core.CodeNotFound)
	if w2.HasComp(tmp, "sprite") || w2.HasComp(a, "") {
		t.Error("HasComp true on dead entity or empty kind")
	}

	// Nil world never panics: writers refuse, getters park.
	var nw *World
	if _, err := nw.Spawn(NoEntity); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Spawn code = %v, want invalid-arg", core.CodeOf(err))
	}
	expectEntityCode(t, "nil Despawn", nw.Despawn(1), core.CodeInvalidArg)
	expectEntityCode(t, "nil SetParent", nw.SetParent(1, NoEntity), core.CodeInvalidArg)
	expectEntityCode(t, "nil SetTransform", nw.SetTransform(1, IdentityTransform()), core.CodeInvalidArg)
	expectEntityCode(t, "nil AddComp", nw.AddComp(1, Comp{Kind: "x"}), core.CodeInvalidArg)
	expectEntityCode(t, "nil RemoveComp", nw.RemoveComp(1, "x"), core.CodeInvalidArg)
	if nw.Alive(1) || nw.Count() != 0 || nw.Spawned() != 0 || nw.HasComp(1, "x") {
		t.Error("nil getters left zero, want parked")
	}
	for _, err := range []error{
		func() error { _, err := nw.Parent(1); return err }(),
		func() error { _, err := nw.Children(1); return err }(),
		func() error { _, err := nw.Local(1); return err }(),
		func() error { _, err := nw.WorldOf(1); return err }(),
		func() error { _, err := nw.WorldMatrix(1); return err }(),
		func() error { _, err := nw.Comps(1); return err }(),
	} {
		if err == nil {
			t.Error("nil getter want error")
		}
	}
	nw.Clear()
}

// C does not apply (pure math, draws nothing): same file replays bitwise
// identical, and core boundary conversions are lossless instead.
func TestEntityBoundaryIdentical(t *testing.T) {
	f := loadEntityCases(t)
	build := func() (*World, map[string]map[string]ID) {
		w := NewWorld()
		m := map[string]map[string]ID{}
		for _, ch := range f.Chains {
			m[ch.Name] = buildChain(t, &w, ch)
		}
		return &w, m
	}
	a, am := build()
	b, bm := build()
	for _, ch := range f.Chains {
		for _, want := range ch.Want {
			ma, errA := a.WorldMatrix(am[ch.Name][want.Name])
			mb, errB := b.WorldMatrix(bm[ch.Name][want.Name])
			if errA != nil || errB != nil {
				t.Fatalf("%s/%s matrix: %v %v", ch.Name, want.Name, errA, errB)
			}
			if ma != mb {
				t.Fatalf("%s/%s replay diverged: %+v vs %+v", ch.Name, want.Name, ma, mb)
			}
			la, _ := a.WorldOf(am[ch.Name][want.Name])
			lb, _ := b.WorldOf(bm[ch.Name][want.Name])
			if la != lb {
				t.Fatalf("%s/%s world replay diverged", ch.Name, want.Name)
			}
		}
	}
	// Boundary round-trips through core helpers only (no render import
	// here): game hands these exact values to the draw side.
	v := core.V2(15, 27)
	if core.Vec2FromRenderPoint(v.ToRenderPoint()) != v {
		t.Error("Vec2 boundary round-trip moved the point")
	}
	m, _ := a.WorldMatrix(am[f.Chains[0].Name]["grand"])
	if core.Mat2DFromRenderMatrix(m.ToRenderMatrix()) != m {
		t.Error("Mat2D boundary round-trip moved the matrix")
	}
	// Comps read back a private copy: mutating it never touches the store.
	id := mustSpawn(t, a, NoEntity)
	if err := a.AddComp(id, Comp{Kind: "sprite", Ref: "tex/hero"}); err != nil {
		t.Fatalf("AddComp: %v", err)
	}
	got, _ := a.Comps(id)
	got[0].Kind = "mutated"
	again, _ := a.Comps(id)
	if again[0].Kind != "sprite" {
		t.Error("Comps exposed the store, want a fresh copy")
	}
	kids, _ := a.Children(am[f.Chains[1].Name]["root"])
	kids[0] = NoEntity
	fresh, _ := a.Children(am[f.Chains[1].Name]["root"])
	if fresh[0] == NoEntity {
		t.Error("Children exposed the store, want a fresh copy")
	}
}

// D:千实体跑得动,耗时有数.
func TestEntityPerfThousand(t *testing.T) {
	f := loadEntityCases(t)
	n, reps := f.Perf.Entities, f.Perf.Reps
	if n <= 0 || reps <= 0 {
		t.Fatal("perf params missing, want frozen entities and reps")
	}
	w := NewWorld()
	root := mustSpawn(t, &w, NoEntity)
	ids := make([]ID, 0, n)
	ids = append(ids, root)
	// Synthetic star load only (no golden): golden stays in the json file.
	for i := 1; i < n; i++ {
		id := mustSpawn(t, &w, root)
		loc := Transform{Pos: core.V2(float64(i%64), float64(i/64)), Rot: float64(i%360) * math.Pi / 180, Scale: core.V2(1, 1)}
		if err := w.SetTransform(id, loc); err != nil {
			t.Fatalf("perf SetTransform: %v", err)
		}
		ids = append(ids, id)
	}
	if w.Count() != n {
		t.Fatalf("count = %d, want %d", w.Count(), n)
	}
	var acc float64
	start := time.Now()
	for r := 0; r < reps; r++ {
		for _, id := range ids {
			m, err := w.WorldMatrix(id)
			if err != nil {
				t.Fatalf("perf WorldMatrix(%d): %v", id, err)
			}
			acc += m.C + m.F
		}
	}
	el := time.Since(start)
	ops := int64(reps * n)
	t.Logf("entity-perf: %d entities x %d reps (%d matrices) in %v (%.1f ns/op)", n, reps, ops, el, float64(el.Nanoseconds())/float64(ops))
	if math.IsNaN(acc) || math.IsInf(acc, 0) || acc == 0 {
		t.Error("perf accumulation invalid, benchmark meaningless")
	}
}

// E:反复生灭不涨:活数回到基线,幸存者逐位一致,发号只涨不回头.
func TestEntityLongRunNoLeak(t *testing.T) {
	f := loadEntityCases(t)
	cycles, batch := f.Longrun.Cycles, f.Longrun.Batch
	if cycles <= 0 || batch <= 0 {
		t.Fatal("longrun params missing, want frozen cycles and batch")
	}
	w := NewWorld()
	base := buildChain(t, &w, f.Chains[0])
	type snap struct {
		local Transform
		world Transform
		mat   core.Mat2D
		comps []Comp
	}
	snapshot := func() map[ID]snap {
		out := map[ID]snap{}
		for _, id := range base {
			l, _ := w.Local(id)
			wd, _ := w.WorldOf(id)
			m, _ := w.WorldMatrix(id)
			c, _ := w.Comps(id)
			out[id] = snap{l, wd, m, c}
		}
		return out
	}
	before := snapshot()
	baseCount, baseSpawned := w.Count(), w.Spawned()
	var maxID ID
	for _, id := range base {
		if id > maxID {
			maxID = id
		}
	}
	root := base["root"]
	for i := 0; i < cycles; i++ {
		born := make([]ID, 0, batch)
		for j := 0; j < batch; j++ {
			id, err := w.Spawn(root)
			if err != nil {
				t.Fatalf("cycle %d spawn: %v", i, err)
			}
			if id <= maxID {
				t.Fatalf("cycle %d reused id %d (max %d)", i, id, maxID)
			}
			if err := w.AddComp(id, Comp{Kind: "temp"}); err != nil {
				t.Fatalf("cycle %d add: %v", i, err)
			}
			born = append(born, id)
		}
		for _, id := range born {
			if err := w.Despawn(id); err != nil {
				t.Fatalf("cycle %d despawn(%d): %v", id, i, err)
			}
			if w.Alive(id) {
				t.Fatalf("cycle %d: %d alive after despawn", i, id)
			}
		}
	}
	if w.Count() != baseCount {
		t.Errorf("after soak count = %d, want base %d", w.Count(), baseCount)
	}
	if w.Spawned() != baseSpawned+uint64(cycles*batch) {
		t.Errorf("spawned = %d, want ledger %d", w.Spawned(), baseSpawned+uint64(cycles*batch))
	}
	after := snapshot()
	for id, s := range before {
		g, ok := after[id]
		if !ok {
			t.Fatalf("survivor %d lost in soak", id)
		}
		if g.local != s.local || g.world != s.world || g.mat != s.mat {
			t.Errorf("survivor %d moved in soak", id)
		}
		if fmt.Sprintf("%v", g.comps) != fmt.Sprintf("%v", s.comps) {
			t.Errorf("survivor %d comps moved in soak", id)
		}
	}
	// Clear drops the living but never rewinds issuance.
	top := w.Spawned()
	w.Clear()
	if w.Count() != 0 {
		t.Errorf("after Clear count = %d, want 0", w.Count())
	}
	fresh, err := w.Spawn(NoEntity)
	if err != nil {
		t.Fatalf("spawn after Clear: %v", err)
	}
	if uint64(fresh) <= top {
		t.Errorf("post-Clear id %d rewinds issuance (spawned %d)", fresh, top)
	}
}

// F:离屏金对照窗(W3窗免,纯算数):冻结数加形状断言.
func TestEntityOffscreenGolden(t *testing.T) {
	f := loadEntityCases(t)
	for _, ch := range f.Chains {
		ww := NewWorld()
		ids := buildChain(t, &ww, ch)
		for _, want := range ch.Want {
			checkWant(t, "golden "+ch.Name, &ww, ids, want)
		}
	}
	// Shape: a root world matrix is exactly its local matrix.
	w2 := NewWorld()
	r2 := buildChain(t, &w2, f.Chains[3])
	rl, _ := w2.Local(r2["root"])
	rm, _ := w2.WorldMatrix(r2["root"])
	if rm != LocalMatrix(rl) {
		t.Errorf("root matrix = %+v, want local %+v", rm, LocalMatrix(rl))
	}
	// Shape: 90-degree rotation turns the child offset onto -Y/+X.
	w3 := NewWorld()
	r3 := buildChain(t, &w3, f.Chains[2])
	cw, _ := w3.WorldOf(r3["child"])
	if cw.Pos.Y != 4 || math.Abs(cw.Pos.X) >= epsEntity {
		t.Errorf("rot90 child = %v, want (~0,4)", cw.Pos)
	}
	gw, _ := w3.WorldOf(r3["grand"])
	if gw.Pos.X >= cw.Pos.X {
		t.Errorf("rot90 grand x = %v not left of child x = %v", gw.Pos.X, cw.Pos.X)
	}
	if !near(gw.Pos.X, -2) || !near(gw.Pos.Y, 4) {
		t.Errorf("rot90 grand = %v, want (-2,4)", gw.Pos)
	}
	// Shape: detach and root-loss both land the child on its local (5,0).
	w4 := NewWorld()
	r4 := buildChain(t, &w4, f.Chains[0])
	if err := w4.SetParent(r4["child"], NoEntity); err != nil {
		t.Fatalf("detach: %v", err)
	}
	dw, _ := w4.WorldOf(r4["child"])
	if dw.Pos != core.V2(5, 0) {
		t.Errorf("detached child = %v, want (5,0)", dw.Pos)
	}
	dg, _ := w4.WorldOf(r4["grand"])
	if dg.Pos != core.V2(5, 7) {
		t.Errorf("detached grand = %v, want (5,7)", dg.Pos)
	}
	w5 := NewWorld()
	r5 := buildChain(t, &w5, f.Chains[0])
	if err := w5.Despawn(r5["root"]); err != nil {
		t.Fatalf("despawn root: %v", err)
	}
	ow, _ := w5.WorldOf(r5["child"])
	if ow.Pos != core.V2(5, 0) {
		t.Errorf("orphaned child = %v, want (5,0)", ow.Pos)
	}
	og, _ := w5.WorldOf(r5["grand"])
	if og.Pos != core.V2(5, 7) {
		t.Errorf("orphaned grand = %v, want (5,7)", og.Pos)
	}
}
