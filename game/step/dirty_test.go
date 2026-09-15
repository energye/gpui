package step

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type dirtyMove struct {
	From []float64 `json:"from"`
	To   []float64 `json:"to"`
}

type dirtyCase struct {
	Name      string      `json:"name"`
	Moves     []dirtyMove `json:"moves"`
	WantRects [][]float64 `json:"want_rects"`
	WantFull  bool        `json:"want_full"`
	WantCount int         `json:"want_count"`
}

type dirtyFile struct {
	Bounds   []float64   `json:"bounds"`
	Cases    []dirtyCase `json:"cases"`
	MaxRects int         `json:"max_dirty_rects"`
	Perf     struct {
		Sprites int `json:"sprites"`
		Reps    int `json:"reps"`
	} `json:"perf"`
	Longrun struct {
		Frames        int `json:"frames"`
		MovesPerFrame int `json:"moves_per_frame"`
	} `json:"longrun"`
}

func loadDirtyCases(t *testing.T) dirtyFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "dirty_cases.json"))
	if err != nil {
		t.Fatalf("read dirty_cases.json: %v", err)
	}
	var f dirtyFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode dirty_cases.json: %v", err)
	}
	if len(f.Bounds) != 4 || len(f.Cases) == 0 {
		t.Fatal("dirty_cases.json misses bounds or cases")
	}
	return f
}

func mustFindDirtyCase(t *testing.T, f dirtyFile, name string) dirtyCase {
	t.Helper()
	for _, c := range f.Cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("dirty_cases.json has no case %q", name)
	return dirtyCase{}
}

func dirtyBounds(t *testing.T, f dirtyFile) core.Rect {
	t.Helper()
	return core.NewRect(f.Bounds[0], f.Bounds[1], f.Bounds[2], f.Bounds[3])
}

func dirtyRect(t *testing.T, v []float64, what string) core.Rect {
	t.Helper()
	if len(v) != 4 {
		t.Fatalf("%s has %d numbers, want 4", what, len(v))
	}
	return core.NewRect(v[0], v[1], v[2], v[3])
}

func mustDirtyTracker(t *testing.T, bounds core.Rect, sprite bool) *DirtyTracker {
	t.Helper()
	var tr *DirtyTracker
	var err error
	if sprite {
		tr, err = NewSpriteDirtyTracker(bounds)
	} else {
		tr, err = NewDirtyTracker(bounds)
	}
	if err != nil {
		t.Fatalf("NewDirtyTracker(%v): %v", bounds, err)
	}
	return tr
}

func applyDirtyMoves(t *testing.T, tr *DirtyTracker, c dirtyCase) {
	t.Helper()
	for i, m := range c.Moves {
		from := dirtyRect(t, m.From, c.Name+"/from")
		to := dirtyRect(t, m.To, c.Name+"/to")
		tr.MarkMoved(from, to)
		_ = i
	}
}

func checkDirtyRects(t *testing.T, tag string, got []core.Rect, want [][]float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: rects = %d, want %d (%v)", tag, len(got), len(want), got)
	}
	for i := range got {
		w := want[i]
		if got[i].X != w[0] || got[i].Y != w[1] || got[i].W != w[2] || got[i].H != w[3] {
			t.Errorf("%s: rect[%d] = %v, want %v", tag, i, got[i], w)
		}
	}
}

func expectDirtyCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

// A:动哪更哪:每组移动只脏旧并新,不多不少;静帧零脏;17块退整屏.
func TestDirtyMovesFromCases(t *testing.T) {
	f := loadDirtyCases(t)
	if f.MaxRects != MaxDirtyRects {
		t.Fatalf("MaxDirtyRects = %d, want frozen %d", MaxDirtyRects, f.MaxRects)
	}
	bounds := dirtyBounds(t, f)
	for _, c := range f.Cases {
		tr := mustDirtyTracker(t, bounds, false)
		applyDirtyMoves(t, tr, c)
		if tr.NeedsFull() != c.WantFull {
			t.Errorf("%s: full = %v, want %v", c.Name, tr.NeedsFull(), c.WantFull)
		}
		if tr.DirtyCount() != c.WantCount {
			t.Errorf("%s: count = %d, want %d", c.Name, tr.DirtyCount(), c.WantCount)
		}
		if c.WantFull {
			if tr.DirtyRects() != nil {
				t.Errorf("%s: full frame must report nil rects", c.Name)
			}
			continue
		}
		checkDirtyRects(t, c.Name, tr.DirtyRects(), c.WantRects)
		var wantArea float64
		for _, w := range c.WantRects {
			wantArea += w[2] * w[3]
		}
		if math.Abs(tr.DirtyArea()-wantArea) > 1e-9 {
			t.Errorf("%s: area = %v, want %v", c.Name, tr.DirtyArea(), wantArea)
		}
		if cov := tr.Coverage(); cov < 0 || cov > 1 {
			t.Errorf("%s: coverage = %v, want in [0,1]", c.Name, cov)
		}
	}
	// Sprite layer updates independently: static stays clean while the car moves.
	sp := mustFindDirtyCase(t, f, "sprite_two_moves")
	staticTr := mustDirtyTracker(t, bounds, false)
	spriteTr := mustDirtyTracker(t, bounds, true)
	if !spriteTr.IsSprite() || staticTr.IsSprite() {
		t.Fatal("sprite flag wrong: sprite must be sprite, static must not")
	}
	applyDirtyMoves(t, spriteTr, sp)
	if staticTr.DirtyCount() != 0 || staticTr.NeedsFull() {
		t.Errorf("static layer dirty = %d full = %v, want 0/false", staticTr.DirtyCount(), staticTr.NeedsFull())
	}
	checkDirtyRects(t, "sprite_two_moves/sprite", spriteTr.DirtyRects(), sp.WantRects)
}

// B:空零超大坏输入不崩不卡死;全动退整屏后可恢复.
func TestDirtyEdgesNoCrash(t *testing.T) {
	f := loadDirtyCases(t)
	bounds := dirtyBounds(t, f)
	// Bad bounds never build.
	for _, b := range []core.Rect{
		core.NewRect(0, 0, 0, 100),
		core.NewRect(0, 0, -10, 100),
		core.NewRect(math.NaN(), 0, 100, 100),
		core.NewRect(0, 0, math.Inf(1), 100),
	} {
		if _, err := NewDirtyTracker(b); err == nil {
			t.Errorf("NewDirtyTracker(%v) want error", b)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("NewDirtyTracker(%v) code = %v, want invalid-arg", b, core.CodeOf(err))
		}
		if _, err := NewSpriteDirtyTracker(b); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("NewSpriteDirtyTracker(%v) code = %v, want invalid-arg", b, core.CodeOf(err))
		}
	}
	expectDirtyCode(t, "empty bounds", func() error { _, err := NewDirtyTracker(core.Rect{}); return err }(), core.CodeInvalidArg)
	tr := mustDirtyTracker(t, bounds, false)
	// Empty and non-finite marks keep nothing.
	for _, r := range []core.Rect{
		{},
		core.NewRect(10, 10, 0, 10),
		core.NewRect(math.NaN(), 0, 8, 8),
		core.NewRect(0, 0, 8, math.Inf(1)),
		core.NewRect(5000, 5000, 8, 8),
	} {
		if tr.MarkDirty(r) {
			t.Errorf("MarkDirty(%v) kept, want ignore", r)
		}
		if tr.MarkMoved(r, r) {
			t.Errorf("MarkMoved self (%v) kept, want ignore", r)
		}
	}
	// Identical boxes mean no motion.
	still := core.NewRect(50, 50, 32, 32)
	if tr.MarkMoved(still, still) {
		t.Error("MarkMoved identical kept, want ignore")
	}
	// Spawn and despawn dirty the live side only.
	if !tr.MarkMoved(core.Rect{}, still) {
		t.Error("spawn MarkMoved kept nothing, want the live box")
	}
	if !tr.MarkMoved(still, core.Rect{}) {
		t.Error("despawn MarkMoved kept nothing, want the live box")
	}
	if tr.DirtyCount() != 2 {
		t.Fatalf("spawn+despawn count = %d, want 2", tr.DirtyCount())
	}
	tr.Clear()
	// Full fallback: the frozen many-moves case trips, then recovers.
	many := mustFindDirtyCase(t, f, "full_fallback_many")
	applyDirtyMoves(t, tr, many)
	if !tr.NeedsFull() || tr.DirtyRects() != nil || tr.DirtyCount() != 0 {
		t.Fatal("many moves must fall back to full with nil rects")
	}
	if tr.MarkDirty(core.NewRect(1, 1, 4, 4)) {
		t.Error("MarkDirty after full kept, want ignore")
	}
	tr.Clear()
	if tr.NeedsFull() || tr.DirtyCount() != 0 {
		t.Fatal("Clear after full must recover to clean")
	}
	st := tr.Stats()
	if st.Frames != 2 || st.FullFallbacks != 1 {
		t.Errorf("stats after 2 frames = %+v, want frames 2 full 1", st)
	}
	// Nil tracker never panics.
	var nilTr *DirtyTracker
	if nilTr.MarkDirty(still) || nilTr.MarkMoved(still, still) {
		t.Error("nil Mark kept, want ignore")
	}
	nilTr.MarkFull()
	nilTr.Clear()
	if nilTr.NeedsFull() || nilTr.DirtyCount() != 0 || nilTr.DirtyArea() != 0 || nilTr.Coverage() != 0 {
		t.Error("nil getters moved off zero, want parked")
	}
	if nilTr.DirtyRects() != nil {
		t.Error("nil DirtyRects != nil, want nil")
	}
	if (nilTr.Stats() != DirtyStats{}) {
		t.Error("nil Stats != zero, want parked")
	}
	if nilTr.Bounds() != (core.Rect{}) || nilTr.IsSprite() {
		t.Error("nil Bounds/IsSprite wrong, want zero/false")
	}
}

// C:纯算数不画画:双建重放逐位一致,发射切片拷贝隔离,输入不改.
func TestDirtyBoundaryIdentical(t *testing.T) {
	f := loadDirtyCases(t)
	bounds := dirtyBounds(t, f)
	for _, c := range f.Cases {
		build := func() *DirtyTracker {
			tr := mustDirtyTracker(t, bounds, false)
			applyDirtyMoves(t, tr, c)
			return tr
		}
		a, b := build(), build()
		if a.NeedsFull() != b.NeedsFull() || a.DirtyCount() != b.DirtyCount() {
			t.Fatalf("%s: replay full/count %v/%d vs %v/%d", c.Name, a.NeedsFull(), a.DirtyCount(), b.NeedsFull(), b.DirtyCount())
		}
		ga, gb := a.DirtyRects(), b.DirtyRects()
		if len(ga) != len(gb) {
			t.Fatalf("%s: replay len %d vs %d", c.Name, len(ga), len(gb))
		}
		for i := range ga {
			if ga[i] != gb[i] {
				t.Fatalf("%s: replay diverged at rect %d: %v vs %v", c.Name, i, ga[i], gb[i])
			}
		}
		if !c.WantFull && len(ga) > 0 {
			ga[0] = core.NewRect(-999, -999, 1, 1)
			again := build()
			if again.DirtyRects()[0] != gb[0] {
				t.Fatalf("%s: emitted slice aliases tracker storage", c.Name)
			}
		}
		// Pure helper matches the stateful path rect for rect.
		if !c.WantFull {
			for i, m := range c.Moves {
				u, ok := DirtyForMove(dirtyRect(t, m.From, "from"), dirtyRect(t, m.To, "to"))
				if !ok {
					t.Fatalf("%s: move[%d] helper false, want union", c.Name, i)
				}
				clipped, ok := u.Intersection(bounds)
				if !ok {
					t.Fatalf("%s: move[%d] union outside bounds", c.Name, i)
				}
				if clipped != gb[i] {
					t.Fatalf("%s: move[%d] helper %v vs kept %v", c.Name, i, clipped, gb[i])
				}
			}
		}
	}
	// Helper edge parity: identical still, empty both, bad numbers.
	still := core.NewRect(5, 5, 8, 8)
	if _, ok := DirtyForMove(still, still); ok {
		t.Error("DirtyForMove identical = true, want false")
	}
	if _, ok := DirtyForMove(core.Rect{}, core.Rect{}); ok {
		t.Error("DirtyForMove empty/empty = true, want false")
	}
	if _, ok := DirtyForMove(core.NewRect(math.NaN(), 0, 8, 8), still); ok {
		t.Error("DirtyForMove NaN = true, want false")
	}
	if u, ok := DirtyForMove(core.Rect{}, still); !ok || u != still {
		t.Errorf("DirtyForMove spawn = %v,%v, want %v,true", u, ok, still)
	}
}

// D:脏块数帧率有数:千精灵突发退整屏耗时与稳态小脏帧耗时一起记.
func TestDirtyPerfCounts(t *testing.T) {
	f := loadDirtyCases(t)
	if f.Perf.Sprites <= 0 || f.Perf.Reps <= 0 {
		t.Fatal("perf params missing, want frozen sprites and reps")
	}
	bounds := dirtyBounds(t, f)
	// Burst: a thousand distinct moves in one frame must fall back to full.
	burst := mustDirtyTracker(t, bounds, true)
	start := time.Now()
	for i := 0; i < f.Perf.Sprites; i++ {
		x := float64((i * 37) % 760)
		y := float64((i * 53) % 560)
		burst.MarkMoved(core.NewRect(x, y, 16, 16), core.NewRect(x+8, y, 16, 16))
	}
	el := time.Since(start)
	if !burst.NeedsFull() {
		t.Fatalf("burst %d moves full = false, want true", f.Perf.Sprites)
	}
	t.Logf("dirty-burst: %d moves in %v (full fallback, rects nil)", f.Perf.Sprites, el)
	burst.Clear()
	// Steady: few dirty boxes per frame stay partial across reps.
	steady := mustDirtyTracker(t, bounds, true)
	start = time.Now()
	var totalRects int
	for r := 0; r < f.Perf.Reps; r++ {
		base := float64((r * 11) % 700)
		steady.MarkMoved(core.NewRect(base, 100, 32, 32), core.NewRect(base+8, 100, 32, 32))
		steady.MarkMoved(core.NewRect(200, base, 24, 24), core.NewRect(200, base+8, 24, 24))
		if steady.NeedsFull() {
			t.Fatalf("rep %d steady tripped full, want partial", r)
		}
		totalRects += steady.DirtyCount()
		steady.Clear()
	}
	el = time.Since(start)
	st := steady.Stats()
	t.Logf("dirty-steady: %d frames %d rects in %v (%.1f ns/frame maxRects=%d)", f.Perf.Reps, totalRects, el, float64(el.Nanoseconds())/float64(f.Perf.Reps), st.MaxRects)
	if st.Frames != f.Perf.Reps || st.TotalRects != totalRects || st.FullFallbacks != 0 || st.MaxRects != 2 {
		t.Errorf("steady stats = %+v, want frames %d rects %d full 0 max 2", st, f.Perf.Reps, totalRects)
	}
}

// E:长跑不漏:数千帧追车双重放一致,整屏往返回基线.
func TestDirtyLongRunStable(t *testing.T) {
	f := loadDirtyCases(t)
	if f.Longrun.Frames <= 0 || f.Longrun.MovesPerFrame <= 0 {
		t.Fatal("longrun params missing, want frozen frames and moves")
	}
	bounds := dirtyBounds(t, f)
	run := func() DirtyStats {
		tr := mustDirtyTracker(t, bounds, true)
		x := 100.0
		for i := 0; i < f.Longrun.Frames; i++ {
			for k := 0; k < f.Longrun.MovesPerFrame; k++ {
				nx := x + float64(4+k*2)
				if nx > 700 {
					nx = 100
				}
				tr.MarkMoved(core.NewRect(x, 200, 48, 24), core.NewRect(nx, 200, 48, 24))
				x = nx
			}
			if tr.NeedsFull() {
				t.Fatalf("frame %d long-run tripped full, want partial", i)
			}
			tr.Clear()
		}
		return tr.Stats()
	}
	a, b := run(), run()
	if a != b {
		t.Fatalf("long replay diverged: %+v vs %+v", a, b)
	}
	if a.Frames != f.Longrun.Frames || a.FullFallbacks != 0 {
		t.Errorf("long stats = %+v, want frames %d full 0", a, f.Longrun.Frames)
	}
	if a.TotalRects != f.Longrun.Frames*f.Longrun.MovesPerFrame || a.MaxRects != f.Longrun.MovesPerFrame {
		t.Errorf("long stats = %+v, want rects %d max %d", a, f.Longrun.Frames*f.Longrun.MovesPerFrame, f.Longrun.MovesPerFrame)
	}
	// Full frames drain back to baseline over repeated trips.
	tr := mustDirtyTracker(t, bounds, false)
	many := mustFindDirtyCase(t, f, "full_fallback_many")
	for i := 0; i < 200; i++ {
		applyDirtyMoves(t, tr, many)
		if !tr.NeedsFull() {
			t.Fatalf("cycle %d want full", i)
		}
		tr.Clear()
		if tr.NeedsFull() || tr.DirtyCount() != 0 {
			t.Fatalf("cycle %d did not drain to clean", i)
		}
	}
	st := tr.Stats()
	if st.Frames != 200 || st.FullFallbacks != 200 || st.TotalRects != 0 {
		t.Errorf("drain stats = %+v, want frames 200 full 200 rects 0", st)
	}
}

// F:离屏金对照窗(真窗game_step--case=dirty随P2建,此处为离屏证据):
// 冻结并外加形状断言,追车旧并新全被盖住,裁剪不出界,整屏nil.
func TestDirtyOffscreenGolden(t *testing.T) {
	f := loadDirtyCases(t)
	bounds := dirtyBounds(t, f)
	single := mustFindDirtyCase(t, f, "single_move")
	if len(single.WantRects) != 1 || single.WantRects[0][0] != 10 || single.WantRects[0][1] != 10 || single.WantRects[0][2] != 64 || single.WantRects[0][3] != 32 {
		t.Fatalf("single_move frozen = %v, want [[10 10 64 32]]", single.WantRects)
	}
	// Shape: kept union contains both the trail and the arrival.
	chase := mustFindDirtyCase(t, f, "chase_union")
	tr := mustDirtyTracker(t, bounds, false)
	applyDirtyMoves(t, tr, chase)
	kept := tr.DirtyRects()
	if len(kept) != 1 {
		t.Fatalf("chase kept = %d, want 1", len(kept))
	}
	for _, m := range chase.Moves {
		from := dirtyRect(t, m.From, "from")
		to := dirtyRect(t, m.To, "to")
		if !kept[0].ContainsRect(from) || !kept[0].ContainsRect(to) {
			t.Errorf("chase kept %v misses from %v or to %v", kept[0], from, to)
		}
	}
	// Shape: clipped edge never leaves the bounds.
	edge := mustFindDirtyCase(t, f, "clipped_edge")
	te := mustDirtyTracker(t, bounds, false)
	applyDirtyMoves(t, te, edge)
	keptEdge := te.DirtyRects()
	if len(keptEdge) != 1 {
		t.Fatalf("edge kept = %d, want 1", len(keptEdge))
	}
	if !bounds.ContainsRect(keptEdge[0]) {
		t.Errorf("edge kept %v escapes bounds %v", keptEdge[0], bounds)
	}
	// Shape: full fallback reports nil, and sprite order stays stable.
	many := mustFindDirtyCase(t, f, "full_fallback_many")
	tm := mustDirtyTracker(t, bounds, false)
	applyDirtyMoves(t, tm, many)
	if !tm.NeedsFull() || tm.DirtyRects() != nil {
		t.Error("many moves want full with nil rects")
	}
	sp := mustFindDirtyCase(t, f, "sprite_two_moves")
	if sp.WantCount != 2 || len(sp.WantRects) != 2 {
		t.Fatalf("sprite_two_moves frozen count = %d/%d, want 2/2", sp.WantCount, len(sp.WantRects))
	}
	ts := mustDirtyTracker(t, bounds, true)
	applyDirtyMoves(t, ts, sp)
	checkDirtyRects(t, "sprite order", ts.DirtyRects(), sp.WantRects)
	// Window intent: game_step --case=dirty draws this chase union per frame
	// and gates on partial presents; the window itself lands with P2.
}
