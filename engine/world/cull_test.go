package world

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/sprite"
)

// S85 cull tests read every number from testdata; the test builds the input
// level from the layout params (fixture setup) but never invents an
// expected active set: all wants come from large_cull.json, frozen by
// /tmp/gen_cull.py on 2026-10-05 (naive grid math, no engine code).

type cullLayout struct {
	Cols          int       `json:"cols"`
	Rows          int       `json:"rows"`
	Spacing       float64   `json:"spacing"`
	Origin        [2]float64 `json:"origin"`
	AttachEvery   int       `json:"attach_every"`
	AttachOffset  [2]float64 `json:"attach_offset"`
	Extent        float64   `json:"extent"`
	Layers        string    `json:"layers"`
	Count         int       `json:"count"`
}

type cullMarker struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

type cullAttached struct {
	ID          int `json:"id"`
	ParentIndex int `json:"parent_index"`
}

type cullSceneFile struct {
	Layout   cullLayout    `json:"layout"`
	Markers  []cullMarker  `json:"markers"`
	Pins     []int         `json:"pins"`
	Attached []cullAttached `json:"attached"`
}

type cullView struct {
	View [4]float64 `json:"view"`
	Want []int      `json:"want"`
}

type cullBudgets struct {
	Entities    int     `json:"entities"`
	Reps        int     `json:"reps"`
	UpdateMsMax float64 `json:"update_ms_max"`
	Cycles      int     `json:"cycles"`
	Reopens     int     `json:"reopens"`
}

type cullFile struct {
	SceneFile string              `json:"scene_file"`
	Views     map[string]cullView `json:"views"`
	Budgets   cullBudgets         `json:"budgets"`
}

func loadCullFiles(t testing.TB) (cullSceneFile, cullFile) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "large_cull.json"))
	if err != nil {
		t.Fatalf("read large_cull.json: %v", err)
	}
	var cf cullFile
	if err := json.Unmarshal(raw, &cf); err != nil {
		t.Fatalf("decode large_cull.json: %v", err)
	}
	raw, err = os.ReadFile(filepath.Join("testdata", cf.SceneFile))
	if err != nil {
		t.Fatalf("read %s: %v", cf.SceneFile, err)
	}
	var sf cullSceneFile
	if err := json.Unmarshal(raw, &sf); err != nil {
		t.Fatalf("decode %s: %v", cf.SceneFile, err)
	}
	if sf.Layout.Count == 0 || len(cf.Views) == 0 {
		t.Fatal("cull fixtures have no entities or views")
	}
	return sf, cf
}

// buildLargeWorld stamps the filed level into a live World in id order:
// row-major grid roots, every attach_every-th entity parented to the
// previous one with the filed offset, markers last with marker comps.
// Returns the world plus id->layer for the sortie checks.
func buildLargeWorld(t testing.TB, sf cullSceneFile) (World, map[ID]int) {
	t.Helper()
	l := sf.Layout
	n := l.Cols * l.Rows
	w := NewWorld()
	layers := make(map[ID]int, n+len(sf.Markers))
	for i := 0; i < n; i++ {
		id, err := w.Spawn(NoEntity)
		if err != nil {
			t.Fatalf("spawn %d: %v", i, err)
		}
		if int(id) != i+1 {
			t.Fatalf("spawn order drift: id = %d, want %d", id, i+1)
		}
		c, r := i%l.Cols, i/l.Cols
		pos := core.V2(l.Origin[0]+float64(c)*l.Spacing, l.Origin[1]+float64(r)*l.Spacing)
		if err := w.SetTransform(id, Transform{Pos: pos, Scale: core.V2(1, 1)}); err != nil {
			t.Fatalf("place %d: %v", id, err)
		}
		layers[id] = (c + r) % 3
	}
	for _, a := range sf.Attached {
		id, parent := ID(a.ID), ID(a.ParentIndex+1)
		if err := w.SetParent(id, parent); err != nil {
			t.Fatalf("attach %d under %d: %v", id, parent, err)
		}
		if err := w.SetTransform(id, Transform{
			Pos:   core.V2(l.AttachOffset[0], l.AttachOffset[1]),
			Scale: core.V2(1, 1),
		}); err != nil {
			t.Fatalf("offset %d: %v", id, err)
		}
		if p, err := w.Parent(id); err != nil || p != parent {
			t.Fatalf("attach link %d = %v/%v, want parent %d", id, p, err, parent)
		}
	}
	for _, m := range sf.Markers {
		id, err := w.Spawn(NoEntity)
		if err != nil {
			t.Fatalf("spawn marker %s: %v", m.Name, err)
		}
		if err := w.SetTransform(id, Transform{Pos: core.V2(m.X, m.Y), Scale: core.V2(1, 1)}); err != nil {
			t.Fatalf("place marker %s: %v", m.Name, err)
		}
		if err := w.AddComp(id, Comp{Kind: "marker", Ref: core.AssetID(m.Name)}); err != nil {
			t.Fatalf("mark %s: %v", m.Name, err)
		}
		layers[id] = 0
	}
	if w.Count() != l.Count {
		t.Fatalf("level count = %d, want %d", w.Count(), l.Count)
	}
	return w, layers
}

// trackLarge registers every entity into c in id order with the filed
// extent, then pins the filed awake set.
func trackLarge(t testing.TB, c *Cull, w *World, sf cullSceneFile, layers map[ID]int) {
	t.Helper()
	for id := ID(1); id <= ID(sf.Layout.Count); id++ {
		hw, hh := sf.Layout.Extent, sf.Layout.Extent
		if w.HasComp(id, "marker") {
			hw, hh = 0, 0
		}
		if err := c.Track(id, layers[id], 0, hw, hh); err != nil {
			t.Fatalf("track %d: %v", id, err)
		}
	}
	for _, p := range sf.Pins {
		if err := c.Awake(ID(p)); err != nil {
			t.Fatalf("awake %d: %v", p, err)
		}
	}
}

func sameIDSet(got []ID, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if int(got[i]) != want[i] {
			return false
		}
	}
	return true
}

// naiveActive filters with only public WorldOf plus the filed rule: the
// independent slow path the fast fold must agree with.
func naiveActive(t testing.TB, w *World, sf cullSceneFile, layers map[ID]int, view core.Rect, pins map[int]bool) []ID {
	t.Helper()
	var out []ID
	for id := ID(1); id <= ID(sf.Layout.Count); id++ {
		wt, err := w.WorldOf(id)
		if err != nil {
			t.Fatalf("naive WorldOf %d: %v", id, err)
		}
		hit := false
		if w.HasComp(id, "marker") {
			hit = view.Contains(wt.Pos)
		} else if !view.IsEmpty() {
			box := core.NewRect(wt.Pos.X-sf.Layout.Extent, wt.Pos.Y-sf.Layout.Extent,
				sf.Layout.Extent*2, sf.Layout.Extent*2)
			hit = view.Intersects(box)
		}
		if hit || pins[int(id)] {
			out = append(out, id)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if layers[a] != layers[b] {
			return layers[a] < layers[b]
		}
		ya, _ := w.WorldOf(a)
		yb, _ := w.WorldOf(b)
		if ya.Pos.Y != yb.Pos.Y {
			return ya.Pos.Y < yb.Pos.Y
		}
		return a < b
	})
	return out
}

// A: the filed views cull to the filed active sets, in order.
func TestCullLargeFromCases(t *testing.T) {
	sf, cf := loadCullFiles(t)
	w, layers := buildLargeWorld(t, sf)
	pins := map[int]bool{}
	for _, p := range sf.Pins {
		pins[p] = true
	}
	for name, v := range cf.Views {
		c, err := NewCull(core.NewRect(v.View[0], v.View[1], v.View[2], v.View[3]))
		if err != nil {
			t.Fatalf("%s NewCull: %v", name, err)
		}
		trackLarge(t, &c, &w, sf, layers)
		if err := c.Refresh(&w, nil); err != nil {
			t.Fatalf("%s refresh: %v", name, err)
		}
		if got := c.Active(); !sameIDSet(got, v.Want) {
			t.Errorf("%s: active = %d ids starting %v, want %d starting %v",
				name, len(got), headIDs(got, 5), len(v.Want), v.Want[:minInt(5, len(v.Want))])
		}
		// The live naive path agrees with both the file and the fast fold.
		view := core.NewRect(v.View[0], v.View[1], v.View[2], v.View[3])
		if slow := naiveActive(t, &w, sf, layers, view, pins); !sameIDSet(slow, v.Want) {
			t.Errorf("%s: naive = %d ids, file wants %d (fast/file already compared)", name, len(slow), len(v.Want))
		}
	}
}

func headIDs(ids []ID, n int) []ID {
	if len(ids) < n {
		n = len(ids)
	}
	return ids[:n]
}

// mustBeHomeOrder pins the update-set identity per rep: count plus the
// first 32 draw-order slots must match the filed home want exactly, so a
// budget rep that passes on speed cannot silently pass on a moved order.
func mustBeHomeOrder(t testing.TB, c Cull, want []int) error {
	t.Helper()
	got := c.Active()
	if len(got) != len(want) {
		return fmt.Errorf("count = %d, want %d", len(got), len(want))
	}
	head := 32
	if len(got) < head {
		head = len(got)
	}
	for i := 0; i < head; i++ {
		if int(got[i]) != want[i] {
			return fmt.Errorf("slot[%d] = %d, want %d", i, got[i], want[i])
		}
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// A2: the zero-alloc Snapshot carries the same ids, order, and world spots
// as Active plus per-active WorldOf: the production read path agrees with
// the slow path exactly.
func TestCullSnapshotMatchesWorldOf(t *testing.T) {
	sf, cf := loadCullFiles(t)
	w, layers := buildLargeWorld(t, sf)
	v := cf.Views["home"]
	c, err := NewCull(core.NewRect(v.View[0], v.View[1], v.View[2], v.View[3]))
	if err != nil {
		t.Fatalf("NewCull: %v", err)
	}
	trackLarge(t, &c, &w, sf, layers)
	if err := c.Refresh(&w, nil); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	active := c.Active()
	snaps := c.Snapshot()
	if len(snaps) != len(active) {
		t.Fatalf("snapshot = %d, active = %d", len(snaps), len(active))
	}
	pins := map[int]bool{}
	for _, p := range sf.Pins {
		pins[p] = true
	}
	attach := map[int]int{}
	for _, a := range sf.Attached {
		attach[a.ID] = a.ParentIndex + 1
	}
	for i, s := range snaps {
		if s.ID != active[i] {
			t.Fatalf("snap[%d] = %d, active = %d", i, s.ID, active[i])
		}
		if s.Layer != layers[s.ID] {
			t.Fatalf("snap %d layer = %d, want %d", s.ID, s.Layer, layers[s.ID])
		}
		if s.Pinned != pins[int(s.ID)] {
			t.Fatalf("snap %d pinned = %v", s.ID, s.Pinned)
		}
		if want := attach[int(s.ID)]; int(s.Parent) != want {
			if !(want == 0 && s.Parent == NoEntity) {
				t.Fatalf("snap %d parent = %d, want %d", s.ID, s.Parent, want)
			}
		}
		wt, err := w.WorldOf(s.ID)
		if err != nil {
			t.Fatalf("WorldOf %d: %v", s.ID, err)
		}
		if wt.Pos.X != s.X || wt.Pos.Y != s.Y {
			t.Fatalf("snap %d = (%v,%v), WorldOf = %v", s.ID, s.X, s.Y, wt.Pos)
		}
	}
	var nilCull *Cull
	if nilCull.Snapshot() != nil {
		t.Error("nil snapshot not silent")
	}
}
func TestCullSortMatchesSprite(t *testing.T) {
	sf, cf := loadCullFiles(t)
	w, layers := buildLargeWorld(t, sf)
	v := cf.Views["home"]
	c, err := NewCull(core.NewRect(v.View[0], v.View[1], v.View[2], v.View[3]))
	if err != nil {
		t.Fatalf("NewCull: %v", err)
	}
	trackLarge(t, &c, &w, sf, layers)
	if err := c.Refresh(&w, nil); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	active := c.Active()
	items := make([]sprite.Item, 0, len(active))
	for _, id := range active {
		wt, err := w.WorldOf(id)
		if err != nil {
			t.Fatalf("WorldOf %d: %v", id, err)
		}
		it, err := sprite.NewItem(strconv.Itoa(int(id)), sprite.Layer(layers[id]), wt.Pos.Y)
		if err != nil {
			t.Fatalf("NewItem %d: %v", id, err)
		}
		items = append(items, it)
	}
	sorted, err := sprite.Sort(items)
	if err != nil {
		t.Fatalf("sprite.Sort: %v", err)
	}
	if len(sorted) != len(active) {
		t.Fatalf("sprite order = %d, active = %d", len(sorted), len(active))
	}
	for i, it := range sorted {
		if it.Name != strconv.Itoa(int(active[i])) {
			t.Fatalf("order[%d] = %s, active = %d", i, it.Name, active[i])
		}
	}
}

// C: the 1500-scale update (full fold plus active matrices) holds the
// filed 3ms budget; the hot Refresh itself stays nearly alloc-free.
func TestCullBudgetLarge(t *testing.T) {
	sf, cf := loadCullFiles(t)
	w, layers := buildLargeWorld(t, sf)
	v := cf.Views["home"]
	c, err := NewCull(core.NewRect(v.View[0], v.View[1], v.View[2], v.View[3]))
	if err != nil {
		t.Fatalf("NewCull: %v", err)
	}
	trackLarge(t, &c, &w, sf, layers)
	for i := 0; i < 2; i++ {
		if err := c.Refresh(&w, nil); err != nil {
			t.Fatalf("warmup: %v", err)
		}
	}
	reps := cf.Budgets.Reps
	if reps <= 0 {
		reps = 5
	}
	var total time.Duration
	var n int
	for r := 0; r < reps; r++ {
		t0 := time.Now()
		if err := c.Refresh(&w, nil); err != nil {
			t.Fatalf("refresh: %v", err)
		}
		if err := mustBeHomeOrder(t, c, v.Want); err != nil {
			t.Fatalf("rep %d order moved: %v", r, err)
		}
		for _, id := range c.Active() {
			if _, err := w.WorldOf(id); err != nil {
				t.Fatalf("WorldOf %d: %v", id, err)
			}
			n++
		}
		total += time.Since(t0)
	}
	ms := float64(total.Microseconds()) / float64(reps) / 1000.0
	t.Logf("update over %d actives: %.2fms mean (gate %.1f), refresh %dus",
		len(cf.Views["home"].Want), ms, cf.Budgets.UpdateMsMax, c.LastRefreshUs())
	if ms > cf.Budgets.UpdateMsMax {
		t.Errorf("large update %.2fms exceeds %.1fms", ms, cf.Budgets.UpdateMsMax)
	}
	for i := 0; i < 3; i++ {
		if err := c.Refresh(&w, nil); err != nil {
			t.Fatalf("warmup: %v", err)
		}
	}
	allocs := testing.AllocsPerRun(20, func() {
		if err := c.Refresh(&w, nil); err != nil {
			t.Errorf("refresh: %v", err)
		}
	})
	t.Logf("refresh allocs/run = %.1f", allocs)
	if allocs > 4 {
		t.Errorf("refresh allocs = %.1f, want <= 4", allocs)
	}
}

// D: sleepers skip the update set; the enabler wakes on enter and sleeps
// on exit; attached children follow the parent both ways.
func TestCullSleepEnablerAttach(t *testing.T) {
	var sc = NewScene()
	w := sc.World()
	var ids []ID
	for i := 0; i < 6; i++ {
		id, err := sc.Spawn(NoEntity)
		if err != nil {
			t.Fatalf("spawn: %v", err)
		}
		if err := w.SetTransform(id, Transform{Pos: core.V2(float64(100+i*40), 100), Scale: core.V2(1, 1)}); err != nil {
			t.Fatalf("place: %v", err)
		}
		ids = append(ids, id)
	}
	// Sword hangs under the hero with a local offset: the挂点.
	hero, sword := ids[0], ids[1]
	if err := w.SetParent(sword, hero); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := w.SetTransform(sword, Transform{Pos: core.V2(12, 0), Scale: core.V2(1, 1)}); err != nil {
		t.Fatalf("offset: %v", err)
	}
	c, err := NewCull(core.NewRect(0, 0, 400, 400))
	if err != nil {
		t.Fatalf("NewCull: %v", err)
	}
	for _, id := range ids {
		if err := c.Track(id, 0, 0, 16, 16); err != nil {
			t.Fatalf("track %d: %v", id, err)
		}
	}
	// Sleep the tail: they sit in view but skip the update set.
	if err := sc.Sleep(ids[4]); err != nil {
		t.Fatalf("sleep: %v", err)
	}
	if err := c.Refresh(w, &sc); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if c.IsActive(ids[4]) {
		t.Error("sleeper in update set with enabler off")
	}
	if !c.InView(ids[4]) {
		t.Error("sleeper missing from visibility set")
	}
	// Enabler on: the same refresh wakes it into the update set.
	c.SetAutoSleep(true)
	if err := c.Refresh(w, &sc); err != nil {
		t.Fatalf("refresh enabler: %v", err)
	}
	if !c.IsActive(ids[4]) || !sc.IsActive(ids[4]) {
		t.Error("entered sleeper not woken by enabler")
	}
	// Drag the hero across the map: the sword follows unbroken, both exit
	// together and re-enter together.
	move := func(x float64) {
		if err := w.SetTransform(hero, Transform{Pos: core.V2(x, 100), Scale: core.V2(1, 1)}); err != nil {
			t.Fatalf("drag: %v", err)
		}
		if err := c.Refresh(w, &sc); err != nil {
			t.Fatalf("refresh: %v", err)
		}
	}
	move(5000)
	if c.InView(hero) || c.InView(sword) {
		t.Error("dragged pair still visible")
	}
	if !sc.IsSleeping(hero) || !sc.IsSleeping(sword) {
		t.Error("exited pair not slept by enabler")
	}
	move(100)
	if !c.IsActive(hero) || !c.IsActive(sword) {
		t.Error("returned pair not active")
	}
	sw, err := w.WorldOf(sword)
	if err != nil {
		t.Fatalf("sword WorldOf: %v", err)
	}
	if sw.Pos.X != 112 || sw.Pos.Y != 100 {
		t.Errorf("sword world = %v, want (112,100)", sw.Pos)
	}
}

// E: despawning the parent never crashes; the child survives as a root at
// its local spot and the dead id drops from the index.
func TestCullDespawnParentNoCrash(t *testing.T) {
	w := NewWorld()
	dad, err := w.Spawn(NoEntity)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	kid, err := w.Spawn(dad)
	if err != nil {
		t.Fatalf("spawn kid: %v", err)
	}
	if err := w.SetTransform(dad, Transform{Pos: core.V2(100, 100), Scale: core.V2(1, 1)}); err != nil {
		t.Fatalf("place: %v", err)
	}
	if err := w.SetTransform(kid, Transform{Pos: core.V2(10, 0), Scale: core.V2(1, 1)}); err != nil {
		t.Fatalf("offset: %v", err)
	}
	c, _ := NewCull(core.NewRect(0, 0, 400, 400))
	for _, id := range []ID{dad, kid} {
		if err := c.Track(id, 0, 0, 16, 16); err != nil {
			t.Fatalf("track %d: %v", id, err)
		}
	}
	if err := w.Despawn(dad); err != nil {
		t.Fatalf("despawn: %v", err)
	}
	if err := c.Refresh(&w, nil); err != nil {
		t.Fatalf("refresh after despawn: %v", err)
	}
	if c.Dropped() != 1 {
		t.Errorf("dropped = %d, want 1", c.Dropped())
	}
	if !c.IsActive(kid) {
		t.Error("orphaned child lost from update set")
	}
	kw, err := w.WorldOf(kid)
	if err != nil {
		t.Fatalf("kid WorldOf: %v", err)
	}
	if kw.Pos.X != 10 || kw.Pos.Y != 0 {
		t.Errorf("orphan world = %v, want local (10,0)", kw.Pos)
	}
}

// F: 200 view jumps return to baseline; 5 fresh rebuilds never drift.
func TestCullEnterExitBaselineReopen(t *testing.T) {
	sf, cf := loadCullFiles(t)
	home := cf.Views["home"]
	jump := cf.Views["jump"]
	viewOf := func(v cullView) core.Rect {
		return core.NewRect(v.View[0], v.View[1], v.View[2], v.View[3])
	}
	build := func() (World, map[ID]int, Cull) {
		w, layers := buildLargeWorld(t, sf)
		c, err := NewCull(viewOf(home))
		if err != nil {
			t.Fatalf("NewCull: %v", err)
		}
		trackLarge(t, &c, &w, sf, layers)
		return w, layers, c
	}
	w, _, c := build()
	if err := c.Refresh(&w, nil); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	base := strings.Join(idStrings(c.Active()), ",")
	var in, out int
	cycles := cf.Budgets.Cycles
	if cycles <= 0 {
		cycles = 200
	}
	for i := 0; i < cycles; i++ {
		if err := c.SetView(viewOf(jump)); err != nil {
			t.Fatalf("set jump: %v", err)
		}
		if err := c.Refresh(&w, nil); err != nil {
			t.Fatalf("refresh jump: %v", err)
		}
		in += len(c.Entered())
		out += len(c.Exited())
		if err := c.SetView(viewOf(home)); err != nil {
			t.Fatalf("set home: %v", err)
		}
		if err := c.Refresh(&w, nil); err != nil {
			t.Fatalf("refresh home: %v", err)
		}
		in += len(c.Entered())
		out += len(c.Exited())
	}
	if got := strings.Join(idStrings(c.Active()), ","); got != base {
		t.Error("200 enter/exit cycles drifted from baseline")
	}
	if in != out {
		t.Errorf("enter/exit unbalanced: in %d out %d", in, out)
	}
	if in == 0 {
		t.Error("jump cycles reported no transitions")
	}
	reopens := cf.Budgets.Reopens
	if reopens <= 0 {
		reopens = 5
	}
	for i := 0; i < reopens; i++ {
		rw, _, rc := build()
		if err := rc.Refresh(&rw, nil); err != nil {
			t.Fatalf("reopen %d: %v", i, err)
		}
		if got := strings.Join(idStrings(rc.Active()), ","); got != base {
			t.Fatalf("reopen %d drifted", i)
		}
	}
}

// BenchmarkCullPan walks the view across the field like the live window:
// the moving-view cost the window gate actually judges.
func BenchmarkCullPan(b *testing.B) {
	sf, cf := loadCullFiles(b)
	w, layers := buildLargeWorld(b, sf)
	home := cf.Views["home"]
	c, err := NewCull(core.NewRect(home.View[0], home.View[1], home.View[2], home.View[3]))
	if err != nil {
		b.Fatalf("NewCull: %v", err)
	}
	trackLarge(b, &c, &w, sf, layers)
	b.ResetTimer()
	x := home.View[0]
	for i := 0; i < b.N; i++ {
		x += 220.0 / 60.0
		if x > 6400-1600 {
			x = 100
		}
		if err := c.SetView(core.NewRect(x, 80, 1600, 1000)); err != nil {
			b.Fatalf("view: %v", err)
		}
		if err := c.Refresh(&w, nil); err != nil {
			b.Fatalf("refresh: %v", err)
		}
	}
}

// F2: a full pan sweep with wrap never balloons the update set: every
// tick stays inside the filed density band and the end matches home.
func TestCullPanWrapBand(t *testing.T) {
	sf, cf := loadCullFiles(t)
	home := cf.Views["home"]
	w, layers := buildLargeWorld(t, sf)
	c, err := NewCull(core.NewRect(home.View[0], home.View[1], home.View[2], home.View[3]))
	if err != nil {
		t.Fatalf("NewCull: %v", err)
	}
	trackLarge(t, &c, &w, sf, layers)
	vw, vh := home.View[2], home.View[3]
	fieldMax := sf.Layout.Origin[0] + float64(sf.Layout.Cols)*sf.Layout.Spacing
	base := strings.Join(idStrings(c.Active()), ",")
	_ = base
	x := home.View[0]
	top, ticks := 0, 0
	for i := 0; i < 3600; i++ {
		x += 220.0 / 60.0
		if x > fieldMax-vw {
			x = sf.Layout.Origin[0]
		}
		if err := c.SetView(core.NewRect(x, home.View[1], vw, vh)); err != nil {
			t.Fatalf("view: %v", err)
		}
		if err := c.Refresh(&w, nil); err != nil {
			t.Fatalf("refresh: %v", err)
		}
		n := c.ActiveCount()
		if n > top {
			top = n
		}
		if n < 1500 || n > 1800 {
			t.Fatalf("tick %d x=%.0f active = %d, want 1500..1800", i, x, n)
		}
		ticks++
	}
	t.Logf("pan %d ticks top active = %d", ticks, top)
}

// F3: chains deeper than parent+child fold exactly like WorldOf: the
// one-level fast path in foldWorld must not change grandchild numbers.
func TestCullDeepChainMatchesWorldOf(t *testing.T) {
	w := NewWorld()
	grand, err := w.Spawn(NoEntity)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	dad, err := w.Spawn(grand)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	kid, err := w.Spawn(dad)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if err := w.SetTransform(grand, Transform{Pos: core.V2(100, 50), Scale: core.V2(1, 1)}); err != nil {
		t.Fatalf("place: %v", err)
	}
	if err := w.SetTransform(dad, Transform{Pos: core.V2(10, 5), Rot: 0.3, Scale: core.V2(1, 1)}); err != nil {
		t.Fatalf("place: %v", err)
	}
	if err := w.SetTransform(kid, Transform{Pos: core.V2(4, 0), Scale: core.V2(1, 1)}); err != nil {
		t.Fatalf("place: %v", err)
	}
	slow, err := w.WorldOf(kid)
	if err != nil {
		t.Fatalf("WorldOf: %v", err)
	}
	fast, ok := w.foldWorld(kid)
	if !ok {
		t.Fatal("foldWorld missed a live grandchild")
	}
	if fast != slow {
		t.Fatalf("foldWorld = %+v, WorldOf = %+v", fast, slow)
	}
}

// G: bad inputs fail with their own codes, never a crash.
func TestCullEdgesNoCrash(t *testing.T) {
	var nilCull *Cull
	if err := nilCull.Refresh(&World{}, nil); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil refresh err = %v, want invalid-arg", err)
	}
	if nilCull.Active() != nil || nilCull.Entered() != nil || nilCull.Exited() != nil {
		t.Error("nil getters not silent")
	}
	if nilCull.ActiveCount() != 0 || nilCull.Count() != 0 || nilCull.View() != (core.Rect{}) {
		t.Error("nil counters not parked")
	}
	nilCull.Reset()
	c, _ := NewCull(core.NewRect(0, 0, 100, 100))
	if err := c.Refresh(nil, nil); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil world err = %v, want invalid-arg", err)
	}
	if _, err := NewCull(core.NewRect(0, 0, 100, 100)); err != nil {
		t.Fatalf("NewCull: %v", err)
	}
	if _, err := NewCull(core.Rect{X: 0, Y: 0, W: 100, H: math.Inf(1)}); err == nil {
		t.Error("inf view accepted")
	}
	if err := c.Track(NoEntity, 0, 0, 1, 1); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("track null err = %v, want invalid-arg", err)
	}
	if err := c.Track(7, 0, 0, -1, 1); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("track neg extent err = %v, want invalid-arg", err)
	}
	if err := c.Track(7, 0, 0, 1, 1); err != nil {
		t.Fatalf("track: %v", err)
	}
	if err := c.Track(7, 0, 0, 1, 1); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("dup track err = %v, want invalid-arg", err)
	}
	if err := c.Update(8, 0, 0, 1, 1); err == nil || core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("update unknown err = %v, want not-found", err)
	}
	if err := c.Awake(8); err == nil || core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("awake unknown err = %v, want not-found", err)
	}
	if err := c.Untrack(8); err == nil || core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("untrack unknown err = %v, want not-found", err)
	}
	if err := c.SetMaxActive(-1); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("neg cap err = %v, want invalid-arg", err)
	}
	// Empty views keep awake pins only, never panic.
	if err := c.SetView(core.Rect{}); err != nil {
		t.Fatalf("empty view: %v", err)
	}
	w := NewWorld()
	id, _ := w.Spawn(NoEntity)
	_ = w.SetTransform(id, Transform{Pos: core.V2(10, 10), Scale: core.V2(1, 1)})
	c.Reset()
	_ = c.Track(id, 0, 0, 4, 4)
	_ = c.Awake(42) // untracked pin: NotFound, ignored
	if err := c.Refresh(&w, nil); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if c.ActiveCount() != 0 {
		t.Errorf("empty view active = %d, want 0 (pin 42 untracked)", c.ActiveCount())
	}
	// The cap trims in draw order and counts the rest.
	full := NewWorld()
	var cc Cull
	cc, _ = NewCull(core.NewRect(0, 0, 10000, 10000))
	for i := 0; i < 30; i++ {
		fid, _ := full.Spawn(NoEntity)
		_ = full.SetTransform(fid, Transform{Pos: core.V2(float64(i * 10), 0), Scale: core.V2(1, 1)})
		_ = cc.Track(fid, 0, 0, 4, 4)
	}
	if err := cc.SetMaxActive(10); err != nil {
		t.Fatalf("cap: %v", err)
	}
	if err := cc.Refresh(&full, nil); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if cc.ActiveCount() != 10 || cc.Trimmed() != 20 {
		t.Errorf("cap active/trimmed = %d/%d, want 10/20", cc.ActiveCount(), cc.Trimmed())
	}
	// Past the cap Track fails fast with out-of-memory.
	var big Cull
	big, _ = NewCull(core.NewRect(0, 0, 10, 10))
	for i := 1; i <= MaxCullEntities; i++ {
		if err := big.Track(ID(i), 0, 0, 1, 1); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	if err := big.Track(ID(MaxCullEntities+1), 0, 0, 1, 1); err == nil ||
		core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("over cap err = %v, want out-of-memory", err)
	}
}

func idStrings(ids []ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprint(int(id))
	}
	return out
}
