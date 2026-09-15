package scene_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/ui/scene"
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

func dirtyRect(t *testing.T, v []float64, what string) scene.DirtyRect {
	t.Helper()
	if len(v) != 4 {
		t.Fatalf("%s has %d numbers, want 4", what, len(v))
	}
	return scene.DirtyRect{X: v[0], Y: v[1], W: v[2], H: v[3]}
}

func mustDirtyLayer(t *testing.T, f dirtyFile, sprite bool) *scene.DirtyLayer {
	t.Helper()
	var l *scene.DirtyLayer
	var err error
	if sprite {
		l, err = scene.NewSpriteDirtyLayer(f.Bounds[0], f.Bounds[1], f.Bounds[2], f.Bounds[3])
	} else {
		l, err = scene.NewDirtyLayer(f.Bounds[0], f.Bounds[1], f.Bounds[2], f.Bounds[3])
	}
	if err != nil {
		t.Fatalf("NewDirtyLayer(%v): %v", f.Bounds, err)
	}
	return l
}

func applyDirtyMoves(t *testing.T, l *scene.DirtyLayer, c dirtyCase) {
	t.Helper()
	for _, m := range c.Moves {
		l.MarkMoved(dirtyRect(t, m.From, c.Name+"/from"), dirtyRect(t, m.To, c.Name+"/to"))
	}
}

func checkDirtyRects(t *testing.T, tag string, got []scene.DirtyRect, want [][]float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: rects = %d, want %d (%v)", tag, len(got), len(want), got)
	}
	for i := range got {
		w := want[i]
		if got[i].X != w[0] || got[i].Y != w[1] || got[i].W != w[2] || got[i].H != w[3] {
			t.Errorf("%s: rect[%d] = %+v, want %v", tag, i, got[i], w)
		}
	}
}

// A:动哪更哪:每组移动只脏旧并新,不多不少;静帧零脏;17块退整屏.
func TestDirtyMovesFromCases(t *testing.T) {
	f := loadDirtyCases(t)
	if f.MaxRects != scene.MaxDirtyRects {
		t.Fatalf("MaxDirtyRects = %d, want frozen %d", scene.MaxDirtyRects, f.MaxRects)
	}
	for _, c := range f.Cases {
		l := mustDirtyLayer(t, f, false)
		applyDirtyMoves(t, l, c)
		if l.NeedsFull() != c.WantFull {
			t.Errorf("%s: full = %v, want %v", c.Name, l.NeedsFull(), c.WantFull)
		}
		if l.DirtyCount() != c.WantCount {
			t.Errorf("%s: count = %d, want %d", c.Name, l.DirtyCount(), c.WantCount)
		}
		if c.WantFull {
			if l.DirtyRects() != nil {
				t.Errorf("%s: full frame must report nil rects", c.Name)
			}
			continue
		}
		checkDirtyRects(t, c.Name, l.DirtyRects(), c.WantRects)
		var wantArea float64
		for _, w := range c.WantRects {
			wantArea += w[2] * w[3]
		}
		if math.Abs(l.DirtyArea()-wantArea) > 1e-9 {
			t.Errorf("%s: area = %v, want %v", c.Name, l.DirtyArea(), wantArea)
		}
		if cov := l.Coverage(); cov < 0 || cov > 1 {
			t.Errorf("%s: coverage = %v, want in [0,1]", c.Name, cov)
		}
	}
	// Sprite layer updates independently: static stays clean while the car moves.
	sp := mustFindDirtyCase(t, f, "sprite_two_moves")
	staticL := mustDirtyLayer(t, f, false)
	spriteL := mustDirtyLayer(t, f, true)
	if !spriteL.IsSprite() || staticL.IsSprite() {
		t.Fatal("sprite flag wrong: sprite must be sprite, static must not")
	}
	if spriteL.Kind() != "dirty_sprite" || staticL.Kind() != "dirty" {
		t.Fatalf("kinds = %q/%q, want dirty_sprite/dirty", spriteL.Kind(), staticL.Kind())
	}
	applyDirtyMoves(t, spriteL, sp)
	if staticL.DirtyCount() != 0 || staticL.NeedsFull() {
		t.Errorf("static layer dirty = %d full = %v, want 0/false", staticL.DirtyCount(), staticL.NeedsFull())
	}
	checkDirtyRects(t, "sprite_two_moves/sprite", spriteL.DirtyRects(), sp.WantRects)
	// Dirty layers join the retained tree without touching picture caching.
	scene.ResetLayerIDGen()
	root := scene.NewContainerLayer()
	root.Add(spriteL)
	if n := scene.Walk(root, nil); n != 2 {
		t.Errorf("walk with dirty child = %d, want 2", n)
	}
}

// B:空零超大坏输入不崩不卡死;全动退整屏后可恢复.
func TestDirtyEdgesNoCrash(t *testing.T) {
	f := loadDirtyCases(t)
	// Bad bounds never build.
	for _, b := range [][4]float64{
		{0, 0, 0, 100},
		{0, 0, -10, 100},
		{math.NaN(), 0, 100, 100},
		{0, 0, math.Inf(1), 100},
	} {
		if _, err := scene.NewDirtyLayer(b[0], b[1], b[2], b[3]); err == nil {
			t.Errorf("NewDirtyLayer(%v) want error", b)
		} else if scene.DirtyCodeOf(err) != scene.DirtyCodeInvalidArg {
			t.Errorf("NewDirtyLayer(%v) code = %v, want invalid-arg", b, scene.DirtyCodeOf(err))
		}
		if _, err := scene.NewSpriteDirtyLayer(b[0], b[1], b[2], b[3]); scene.DirtyCodeOf(err) != scene.DirtyCodeInvalidArg {
			t.Errorf("NewSpriteDirtyLayer(%v) code = %v, want invalid-arg", b, scene.DirtyCodeOf(err))
		}
	}
	if _, err := scene.NewDirtyLayer(0, 0, 0, 0); scene.DirtyCodeOf(err) != scene.DirtyCodeInvalidArg {
		t.Errorf("empty bounds code = %v, want invalid-arg", scene.DirtyCodeOf(err))
	}
	l := mustDirtyLayer(t, f, false)
	// Empty and non-finite marks keep nothing.
	for _, r := range []scene.DirtyRect{
		{},
		{X: 10, Y: 10, W: 0, H: 10},
		{X: math.NaN(), Y: 0, W: 8, H: 8},
		{X: 0, Y: 0, W: 8, H: math.Inf(1)},
		{X: 5000, Y: 5000, W: 8, H: 8},
	} {
		if l.MarkDirty(r) {
			t.Errorf("MarkDirty(%+v) kept, want ignore", r)
		}
		if l.MarkMoved(r, r) {
			t.Errorf("MarkMoved self (%+v) kept, want ignore", r)
		}
	}
	// Identical boxes mean no motion.
	still := scene.DirtyRect{X: 50, Y: 50, W: 32, H: 32}
	if l.MarkMoved(still, still) {
		t.Error("MarkMoved identical kept, want ignore")
	}
	// Spawn and despawn dirty the live side only.
	if !l.MarkMoved(scene.DirtyRect{}, still) {
		t.Error("spawn MarkMoved kept nothing, want the live box")
	}
	if !l.MarkMoved(still, scene.DirtyRect{}) {
		t.Error("despawn MarkMoved kept nothing, want the live box")
	}
	if l.DirtyCount() != 2 {
		t.Fatalf("spawn+despawn count = %d, want 2", l.DirtyCount())
	}
	l.Clear()
	// Full fallback: the frozen many-moves case trips, then recovers.
	many := mustFindDirtyCase(t, f, "full_fallback_many")
	applyDirtyMoves(t, l, many)
	if !l.NeedsFull() || l.DirtyRects() != nil || l.DirtyCount() != 0 {
		t.Fatal("many moves must fall back to full with nil rects")
	}
	if l.MarkDirty(scene.DirtyRect{X: 1, Y: 1, W: 4, H: 4}) {
		t.Error("MarkDirty after full kept, want ignore")
	}
	l.Clear()
	if l.NeedsFull() || l.DirtyCount() != 0 {
		t.Fatal("Clear after full must recover to clean")
	}
	st := l.Stats()
	if st.Frames != 2 || st.FullFallbacks != 1 {
		t.Errorf("stats after 2 frames = %+v, want frames 2 full 1", st)
	}
	// Nil layer never panics.
	var nilL *scene.DirtyLayer
	if nilL.MarkDirty(still) || nilL.MarkMoved(still, still) {
		t.Error("nil Mark kept, want ignore")
	}
	nilL.MarkFull()
	nilL.Clear()
	nilL.Add(nil)
	if nilL.NeedsFull() || nilL.DirtyCount() != 0 || nilL.DirtyArea() != 0 || nilL.Coverage() != 0 {
		t.Error("nil getters moved off zero, want parked")
	}
	if nilL.DirtyRects() != nil || nilL.Children() != nil {
		t.Error("nil slices != nil, want nil")
	}
	if (nilL.Stats() != scene.DirtyStats{}) {
		t.Error("nil Stats != zero, want parked")
	}
	if nilL.LayerID() != 0 || nilL.IsSprite() || nilL.Bounds() != (scene.DirtyRect{}) {
		t.Error("nil identity wrong, want 0/false/zero")
	}
	if nilL.Kind() != "dirty" {
		t.Errorf("nil Kind = %q, want dirty", nilL.Kind())
	}
}

// C:两边齐(纯层不画画):双建重放逐位一致,发射切片拷贝隔离.
func TestDirtyBoundaryIdentical(t *testing.T) {
	f := loadDirtyCases(t)
	for _, c := range f.Cases {
		build := func() *scene.DirtyLayer {
			l := mustDirtyLayer(t, f, false)
			applyDirtyMoves(t, l, c)
			return l
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
				t.Fatalf("%s: replay diverged at rect %d: %+v vs %+v", c.Name, i, ga[i], gb[i])
			}
		}
		if !c.WantFull && len(ga) > 0 {
			ga[0] = scene.DirtyRect{X: -999, Y: -999, W: 1, H: 1}
			again := build()
			if again.DirtyRects()[0] != gb[0] {
				t.Fatalf("%s: emitted slice aliases layer storage", c.Name)
			}
		}
	}
}

// D:脏块数帧率有数:千精灵突发退整屏耗时与稳态小脏帧耗时一起记.
func TestDirtyPerfCounts(t *testing.T) {
	f := loadDirtyCases(t)
	if f.Perf.Sprites <= 0 || f.Perf.Reps <= 0 {
		t.Fatal("perf params missing, want frozen sprites and reps")
	}
	// Burst: a thousand distinct moves in one frame must fall back to full.
	burst := mustDirtyLayer(t, f, true)
	start := time.Now()
	for i := 0; i < f.Perf.Sprites; i++ {
		x := float64((i * 37) % 760)
		y := float64((i * 53) % 560)
		burst.MarkMoved(
			scene.DirtyRect{X: x, Y: y, W: 16, H: 16},
			scene.DirtyRect{X: x + 8, Y: y, W: 16, H: 16},
		)
	}
	el := time.Since(start)
	if !burst.NeedsFull() {
		t.Fatalf("burst %d moves full = false, want true", f.Perf.Sprites)
	}
	t.Logf("dirty-burst: %d moves in %v (full fallback, rects nil)", f.Perf.Sprites, el)
	burst.Clear()
	// Steady: few dirty boxes per frame stay partial across reps.
	steady := mustDirtyLayer(t, f, true)
	start = time.Now()
	totalRects := 0
	for r := 0; r < f.Perf.Reps; r++ {
		base := float64((r * 11) % 700)
		steady.MarkMoved(
			scene.DirtyRect{X: base, Y: 100, W: 32, H: 32},
			scene.DirtyRect{X: base + 8, Y: 100, W: 32, H: 32},
		)
		steady.MarkMoved(
			scene.DirtyRect{X: 200, Y: base, W: 24, H: 24},
			scene.DirtyRect{X: 200, Y: base + 8, W: 24, H: 24},
		)
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
	run := func() scene.DirtyStats {
		l := mustDirtyLayer(t, f, true)
		x := 100.0
		for i := 0; i < f.Longrun.Frames; i++ {
			for k := 0; k < f.Longrun.MovesPerFrame; k++ {
				nx := x + float64(4+k*2)
				if nx > 700 {
					nx = 100
				}
				l.MarkMoved(
					scene.DirtyRect{X: x, Y: 200, W: 48, H: 24},
					scene.DirtyRect{X: nx, Y: 200, W: 48, H: 24},
				)
				x = nx
			}
			if l.NeedsFull() {
				t.Fatalf("frame %d long-run tripped full, want partial", i)
			}
			l.Clear()
		}
		return l.Stats()
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
	l := mustDirtyLayer(t, f, false)
	many := mustFindDirtyCase(t, f, "full_fallback_many")
	for i := 0; i < 200; i++ {
		applyDirtyMoves(t, l, many)
		if !l.NeedsFull() {
			t.Fatalf("cycle %d want full", i)
		}
		l.Clear()
		if l.NeedsFull() || l.DirtyCount() != 0 {
			t.Fatalf("cycle %d did not drain to clean", i)
		}
	}
	st := l.Stats()
	if st.Frames != 200 || st.FullFallbacks != 200 || st.TotalRects != 0 {
		t.Errorf("drain stats = %+v, want frames 200 full 200 rects 0", st)
	}
}

// F:离屏金对照窗(真窗game_step--case=dirty随P2建,此处为离屏证据):
// 冻结并外加形状断言,追车旧并新全被盖住,裁剪不出界,整屏nil.
func TestDirtyOffscreenGolden(t *testing.T) {
	f := loadDirtyCases(t)
	single := mustFindDirtyCase(t, f, "single_move")
	if len(single.WantRects) != 1 || single.WantRects[0][0] != 10 || single.WantRects[0][1] != 10 || single.WantRects[0][2] != 64 || single.WantRects[0][3] != 32 {
		t.Fatalf("single_move frozen = %v, want [[10 10 64 32]]", single.WantRects)
	}
	// Shape: kept union covers both the trail and the arrival.
	chase := mustFindDirtyCase(t, f, "chase_union")
	l := mustDirtyLayer(t, f, false)
	applyDirtyMoves(t, l, chase)
	kept := l.DirtyRects()
	if len(kept) != 1 {
		t.Fatalf("chase kept = %d, want 1", len(kept))
	}
	for _, m := range chase.Moves {
		from := dirtyRect(t, m.From, "from")
		to := dirtyRect(t, m.To, "to")
		if _, ok := kept[0].Intersect(from); !ok {
			t.Errorf("chase kept %+v misses from %+v", kept[0], from)
		} else {
			u := kept[0].Union(scene.DirtyRect{})
			_ = u
		}
		if _, ok := kept[0].Intersect(to); !ok {
			t.Errorf("chase kept %+v misses to %+v", kept[0], to)
		}
		// Union containment: kept must fully contain both boxes.
		if from.X < kept[0].X || from.Y < kept[0].Y || from.X+from.W > kept[0].X+kept[0].W || from.Y+from.H > kept[0].Y+kept[0].H {
			t.Errorf("chase kept %+v does not contain from %+v", kept[0], from)
		}
		if to.X < kept[0].X || to.Y < kept[0].Y || to.X+to.W > kept[0].X+kept[0].W || to.Y+to.H > kept[0].Y+kept[0].H {
			t.Errorf("chase kept %+v does not contain to %+v", kept[0], to)
		}
	}
	// Shape: clipped edge never leaves the bounds.
	edge := mustFindDirtyCase(t, f, "clipped_edge")
	le := mustDirtyLayer(t, f, false)
	applyDirtyMoves(t, le, edge)
	keptEdge := le.DirtyRects()
	if len(keptEdge) != 1 {
		t.Fatalf("edge kept = %d, want 1", len(keptEdge))
	}
	bx, by, bw, bh := f.Bounds[0], f.Bounds[1], f.Bounds[2], f.Bounds[3]
	k := keptEdge[0]
	if k.X < bx || k.Y < by || k.X+k.W > bx+bw || k.Y+k.H > by+bh {
		t.Errorf("edge kept %+v escapes bounds %v", k, f.Bounds)
	}
	// Shape: full fallback reports nil, and sprite order stays stable.
	many := mustFindDirtyCase(t, f, "full_fallback_many")
	lm := mustDirtyLayer(t, f, false)
	applyDirtyMoves(t, lm, many)
	if !lm.NeedsFull() || lm.DirtyRects() != nil {
		t.Error("many moves want full with nil rects")
	}
	sp := mustFindDirtyCase(t, f, "sprite_two_moves")
	if sp.WantCount != 2 || len(sp.WantRects) != 2 {
		t.Fatalf("sprite_two_moves frozen count = %d/%d, want 2/2", sp.WantCount, len(sp.WantRects))
	}
	ls := mustDirtyLayer(t, f, true)
	applyDirtyMoves(t, ls, sp)
	checkDirtyRects(t, "sprite order", ls.DirtyRects(), sp.WantRects)
	// Window intent: game_step --case=dirty draws this chase union per frame
	// and gates on partial presents; the window itself lands with P2.
}
