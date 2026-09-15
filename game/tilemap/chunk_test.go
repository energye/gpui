package tilemap

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsChunk = 1e-9

type chunkGrid struct {
	Orient string  `json:"orient"`
	MapW   int     `json:"map_w"`
	MapH   int     `json:"map_h"`
	TileW  float64 `json:"tile_w"`
	TileH  float64 `json:"tile_h"`
	ChunkW int     `json:"chunk_w"`
	ChunkH int     `json:"chunk_h"`
	Cols   int     `json:"cols"`
	Rows   int     `json:"rows"`
	TMX    string  `json:"tmx"`
}

type chunkOfCase struct {
	Col int `json:"col"`
	Row int `json:"row"`
	CX  int `json:"cx"`
	CY  int `json:"cy"`
}

type chunkBoundsCase struct {
	CX   int        `json:"cx"`
	CY   int        `json:"cy"`
	Rect [4]float64 `json:"rect"`
}

type chunkNeededCase struct {
	Name string     `json:"name"`
	View [4]float64 `json:"view"`
	Want [][2]int   `json:"want"`
}

type chunkStepCase struct {
	View      [4]float64 `json:"view"`
	LoadedNew [][2]int   `json:"loaded_new"`
	LoadedAll [][2]int   `json:"loaded_all"`
	Visible   [][2]int   `json:"visible,omitempty"`
	Dropped   [][2]int   `json:"dropped,omitempty"`
}

type chunkJumpCase struct {
	From         [4]float64 `json:"from"`
	To           [4]float64 `json:"to"`
	WantLoaded   [][2]int   `json:"want_loaded"`
	WantUnloaded [][2]int   `json:"want_unloaded"`
}

type chunkRoundtrip struct {
	A [4]float64 `json:"a"`
	B [4]float64 `json:"b"`
}

type chunkIso struct {
	MapW            int        `json:"map_w"`
	MapH            int        `json:"map_h"`
	TileW           float64    `json:"tile_w"`
	TileH           float64    `json:"tile_h"`
	ChunkW          int        `json:"chunk_w"`
	ChunkH          int        `json:"chunk_h"`
	Cols            int        `json:"cols"`
	Rows            int        `json:"rows"`
	Chunk00Contains [2]float64 `json:"chunk00_contains_center"`
}

type chunkTail struct {
	MapW     int        `json:"map_w"`
	MapH     int        `json:"map_h"`
	TileW    float64    `json:"tile_w"`
	TileH    float64    `json:"tile_h"`
	ChunkW   int        `json:"chunk_w"`
	ChunkH   int        `json:"chunk_h"`
	Cols     int        `json:"cols"`
	Rows     int        `json:"rows"`
	TailID   [2]int     `json:"tail_id"`
	TailRect [4]float64 `json:"tail_rect"`
}

type chunkFile struct {
	Grid              chunkGrid         `json:"grid"`
	Of                []chunkOfCase     `json:"of"`
	OOBTiles          [][2]int          `json:"oob_tiles"`
	Bounds            []chunkBoundsCase `json:"bounds"`
	Needed            []chunkNeededCase `json:"needed"`
	LoadSeq           []chunkStepCase   `json:"load_seq"`
	UnloadSeq         []chunkStepCase   `json:"unload_seq"`
	Jump              chunkJumpCase     `json:"jump"`
	TeleportRoundtrip chunkRoundtrip    `json:"teleport_roundtrip"`
	Iso               chunkIso          `json:"iso"`
	Tail              chunkTail         `json:"tail"`
}

func loadChunkCases(t *testing.T) chunkFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "chunk_cases.json"))
	if err != nil {
		t.Fatalf("read chunk_cases.json: %v", err)
	}
	var f chunkFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode chunk_cases.json: %v", err)
	}
	if len(f.Of) == 0 || len(f.Needed) == 0 {
		t.Fatal("chunk_cases.json has no cases")
	}
	return f
}

func mustChunker(t *testing.T) (Chunk, chunkFile) {
	t.Helper()
	f := loadChunkCases(t)
	g := f.Grid
	c, err := NewChunker(OrientOrthogonal, g.MapW, g.MapH, g.TileW, g.TileH, g.ChunkW, g.ChunkH)
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}
	if c.ChunkCols() != g.Cols || c.ChunkRows() != g.Rows {
		t.Fatalf("grid = %dx%d, want %dx%d", c.ChunkCols(), c.ChunkRows(), g.Cols, g.Rows)
	}
	return c, f
}

func chunkRect(v [4]float64) core.Rect {
	return core.NewRect(v[0], v[1], v[2], v[3])
}

func chunkIDs(pairs [][2]int) []ChunkID {
	out := make([]ChunkID, len(pairs))
	for i, p := range pairs {
		out[i] = ChunkID{CX: p[0], CY: p[1]}
	}
	return out
}

func equalChunkIDs(got, want []ChunkID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func closeChunkRect(got core.Rect, want [4]float64) bool {
	return math.Abs(got.X-want[0]) < epsChunk && math.Abs(got.Y-want[1]) < epsChunk &&
		math.Abs(got.W-want[2]) < epsChunk && math.Abs(got.H-want[3]) < epsChunk
}

func mustFindChunkNeeded(t *testing.T, f chunkFile, name string) chunkNeededCase {
	t.Helper()
	for _, k := range f.Needed {
		if k.Name == name {
			return k
		}
	}
	t.Fatalf("missing needed case %q", name)
	return chunkNeededCase{}
}

// A:镜头内外装卸对:格子归属、块包络、视口相交、装卸序列全落在冻结数上.
func TestChunkFromCases(t *testing.T) {
	c, f := mustChunker(t)
	for i, k := range f.Of {
		got, ok := c.ChunkOf(k.Col, k.Row)
		if !ok || got != (ChunkID{CX: k.CX, CY: k.CY}) {
			t.Errorf("of[%d] (%d,%d) = %v/%v, want (%d,%d)/true", i, k.Col, k.Row, got, ok, k.CX, k.CY)
		}
	}
	for _, tc := range f.OOBTiles {
		if got, ok := c.ChunkOf(tc[0], tc[1]); ok {
			t.Errorf("oob tile (%d,%d) = %v/true, want false", tc[0], tc[1], got)
		}
	}
	for i, k := range f.Bounds {
		got, ok := c.ChunkBounds(ChunkID{CX: k.CX, CY: k.CY})
		if !ok || !closeChunkRect(got, k.Rect) {
			t.Errorf("bounds[%d] (%d,%d) = %v/%v, want %v/true", i, k.CX, k.CY, got, ok, k.Rect)
		}
	}
	for _, k := range f.Needed {
		if got := c.Needed(chunkRect(k.View)); !equalChunkIDs(got, chunkIDs(k.Want)) {
			t.Errorf("needed %s %v = %v, want %v", k.Name, k.View, got, chunkIDs(k.Want))
		}
	}
	// Load sequence: newly loaded deltas plus the running loaded set.
	for i, s := range f.LoadSeq {
		got := c.Load(chunkRect(s.View))
		if !equalChunkIDs(got, chunkIDs(s.LoadedNew)) {
			t.Errorf("load[%d] new = %v, want %v", i, got, chunkIDs(s.LoadedNew))
		}
		if all := c.Loaded(); !equalChunkIDs(all, chunkIDs(s.LoadedAll)) {
			t.Errorf("load[%d] all = %v, want %v", i, all, chunkIDs(s.LoadedAll))
		}
		if len(s.Visible) > 0 {
			if vis := c.Visible(chunkRect(s.View)); !equalChunkIDs(vis, chunkIDs(s.Visible)) {
				t.Errorf("load[%d] visible = %v, want %v", i, vis, chunkIDs(s.Visible))
			}
		}
		for _, id := range chunkIDs(s.LoadedAll) {
			if !c.IsLoaded(id) {
				t.Errorf("load[%d] IsLoaded(%v) = false, want true", i, id)
			}
		}
	}
	for i, s := range f.UnloadSeq {
		got := c.Unload(chunkRect(s.View))
		if !equalChunkIDs(got, chunkIDs(s.Dropped)) {
			t.Errorf("unload[%d] dropped = %v, want %v", i, got, chunkIDs(s.Dropped))
		}
		if all := c.Loaded(); !equalChunkIDs(all, chunkIDs(s.LoadedAll)) {
			t.Errorf("unload[%d] all = %v, want %v", i, all, chunkIDs(s.LoadedAll))
		}
	}
	// Jump: one Update swaps the far ends with no leftovers.
	j := f.Jump
	c.Reset()
	if c.LoadedCount() != 0 {
		t.Fatal("Reset did not clear")
	}
	c.Load(chunkRect(j.From))
	gotL, gotU := c.Update(chunkRect(j.To))
	if !equalChunkIDs(gotL, chunkIDs(j.WantLoaded)) {
		t.Errorf("jump loaded = %v, want %v", gotL, chunkIDs(j.WantLoaded))
	}
	if !equalChunkIDs(gotU, chunkIDs(j.WantUnloaded)) {
		t.Errorf("jump unloaded = %v, want %v", gotU, chunkIDs(j.WantUnloaded))
	}
	if all := c.Loaded(); !equalChunkIDs(all, chunkIDs(j.WantLoaded)) {
		t.Errorf("jump all = %v, want %v", all, chunkIDs(j.WantLoaded))
	}
	// Iso half: same operations on diamonds, chunk (0,0) owns the (0,0) center.
	iso, err := NewChunker(OrientIsometric, f.Iso.MapW, f.Iso.MapH, f.Iso.TileW, f.Iso.TileH, f.Iso.ChunkW, f.Iso.ChunkH)
	if err != nil {
		t.Fatalf("NewChunker iso: %v", err)
	}
	if iso.ChunkCols() != f.Iso.Cols || iso.ChunkRows() != f.Iso.Rows {
		t.Fatalf("iso grid = %dx%d, want %dx%d", iso.ChunkCols(), iso.ChunkRows(), f.Iso.Cols, f.Iso.Rows)
	}
	b00, ok := iso.ChunkBounds(ChunkID{})
	if !ok {
		t.Fatal("iso chunk (0,0) ok=false")
	}
	if !b00.Contains(core.V2(f.Iso.Chunk00Contains[0], f.Iso.Chunk00Contains[1])) {
		t.Errorf("iso chunk (0,0) %v misses center %v", b00, f.Iso.Chunk00Contains)
	}
	// Tail half: 5x5 tiles in 2x2 chunks leaves a 1-tile tail block.
	tail, err := NewChunker(OrientOrthogonal, f.Tail.MapW, f.Tail.MapH, f.Tail.TileW, f.Tail.TileH, f.Tail.ChunkW, f.Tail.ChunkH)
	if err != nil {
		t.Fatalf("NewChunker tail: %v", err)
	}
	if tail.ChunkCols() != f.Tail.Cols || tail.ChunkRows() != f.Tail.Rows {
		t.Fatalf("tail grid = %dx%d, want %dx%d", tail.ChunkCols(), tail.ChunkRows(), f.Tail.Cols, f.Tail.Rows)
	}
	tb, ok := tail.ChunkBounds(ChunkID{CX: f.Tail.TailID[0], CY: f.Tail.TailID[1]})
	if !ok || !closeChunkRect(tb, f.Tail.TailRect) {
		t.Errorf("tail chunk = %v/%v, want %v/true", tb, ok, f.Tail.TailRect)
	}
}

// B:跳跃镜头空零超大坏数据全不崩不卡死,缺块占位加报错.
func TestChunkEdgesNoCrash(t *testing.T) {
	c, f := mustChunker(t)
	// Empty and NaN/Inf views see nothing and change nothing.
	before := c.LoadedCount()
	for i, v := range [][4]float64{
		{0, 0, 0, 0}, {10, 10, 0, 50}, {10, 10, 50, 0},
		{math.NaN(), 0, 64, 64}, {0, math.Inf(1), 64, 64}, {0, 0, math.NaN(), 64},
	} {
		if got := c.Needed(chunkRect(v)); len(got) != 0 {
			t.Errorf("bad view[%d] needed = %v, want empty", i, got)
		}
		if got := c.Load(chunkRect(v)); len(got) != 0 {
			t.Errorf("bad view[%d] load = %v, want empty", i, got)
		}
		if got := c.Visible(chunkRect(v)); len(got) != 0 {
			t.Errorf("bad view[%d] visible = %v, want empty", i, got)
		}
	}
	if c.LoadedCount() != before {
		t.Error("bad views changed the loaded set")
	}
	// Empty view unloads everything (finite, sees nothing); NaN keeps it.
	c.Load(chunkRect([4]float64{0, 0, 128, 128}))
	if n := c.LoadedCount(); n == 0 {
		t.Fatal("setup load drew nothing")
	}
	if got := c.Unload(core.Rect{}); len(got) == 0 {
		t.Error("empty view unload dropped nothing, want all")
	}
	if c.LoadedCount() != 0 {
		t.Errorf("after empty unload = %d, want 0", c.LoadedCount())
	}
	c.Load(chunkRect([4]float64{0, 0, 128, 128}))
	before = c.LoadedCount()
	if got := c.Unload(core.NewRect(math.NaN(), 0, 64, 64)); len(got) != 0 {
		t.Errorf("NaN unload = %v, want empty", got)
	}
	if c.LoadedCount() != before {
		t.Error("NaN unload changed the loaded set")
	}
	l, u := c.Update(core.NewRect(math.Inf(1), 0, 64, 64))
	if len(l) != 0 || len(u) != 0 {
		t.Errorf("Inf update = %v/%v, want empty", l, u)
	}
	if c.LoadedCount() != before {
		t.Error("Inf update changed the loaded set")
	}
	// Unknown ids report ok=false with zero.
	for _, id := range []ChunkID{{CX: -1}, {CY: -1}, {CX: 99, CY: 99}, {CX: f.Grid.Cols, CY: 0}} {
		if _, ok := c.ChunkBounds(id); ok {
			t.Errorf("bad id %v ok=true, want false", id)
		}
		if c.IsLoaded(id) {
			t.Errorf("bad id %v loaded=true, want false", id)
		}
	}
	// Constructors reject empty/zero instead of guessing.
	for _, tc := range []struct {
		name       string
		orient     Orientation
		mapW, mapH int
		tileW      float64
		chunkW     int
		code       core.Code
	}{
		{"bad orient", Orientation(9), 16, 16, 32, 4, core.CodeInvalidArg},
		{"zero map", OrientOrthogonal, 0, 16, 32, 4, core.CodeInvalidArg},
		{"zero tile", OrientOrthogonal, 16, 16, 0, 4, core.CodeInvalidArg},
		{"nan tile", OrientOrthogonal, 16, 16, math.NaN(), 4, core.CodeInvalidArg},
		{"zero chunk", OrientOrthogonal, 16, 16, 32, 0, core.CodeInvalidArg},
	} {
		_, err := NewChunker(tc.orient, tc.mapW, tc.mapH, tc.tileW, 32, tc.chunkW, 4)
		if err == nil {
			t.Errorf("%s want error", tc.name)
		} else if core.CodeOf(err) != tc.code {
			t.Errorf("%s code = %v, want %v", tc.name, core.CodeOf(err), tc.code)
		}
	}
	// A grid beyond MaxChunks is OutOfMemory, never a hang.
	if _, err := NewChunker(OrientOrthogonal, 1<<20, 1<<11, 32, 32, 1, 1); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge grid code = %v, want out-of-memory", core.CodeOf(err))
	}
	// Chunks bigger than the map collapse to one chunk.
	big, err := NewChunker(OrientOrthogonal, 3, 3, 32, 32, 99, 99)
	if err != nil {
		t.Fatalf("big chunk: %v", err)
	}
	if big.ChunkCols() != 1 || big.ChunkRows() != 1 {
		t.Errorf("big chunk grid = %dx%d, want 1x1", big.ChunkCols(), big.ChunkRows())
	}
	if n := big.Needed(chunkRect([4]float64{0, 0, 96, 96})); !equalChunkIDs(n, []ChunkID{{}}) {
		t.Errorf("big chunk needed = %v, want [(0,0)]", n)
	}
	// Nil grids never panic: loads empty, counts zero.
	var nilGrid *Chunk
	if got := nilGrid.Load(chunkRect([4]float64{0, 0, 64, 64})); len(got) != 0 {
		t.Errorf("nil load = %v, want empty", got)
	}
	if l, u := nilGrid.Update(chunkRect([4]float64{0, 0, 64, 64})); len(l) != 0 || len(u) != 0 {
		t.Errorf("nil update = %v/%v, want empty", l, u)
	}
	if n := nilGrid.Unload(chunkRect([4]float64{0, 0, 64, 64})); len(n) != 0 {
		t.Errorf("nil unload = %v, want empty", n)
	}
	if nilGrid.IsLoaded(ChunkID{}) || nilGrid.LoadedCount() != 0 || len(nilGrid.Loaded()) != 0 {
		t.Error("nil grid reports loaded, want empty")
	}
	nilGrid.Reset()
}

// C:纯算数不画画,两边同数靠边界往返无损加逐位重放.
func TestChunkBoundaryIdentical(t *testing.T) {
	c, f := mustChunker(t)
	// Render boundary keeps every frozen chunk box lossless.
	for _, k := range f.Bounds {
		got, ok := c.ChunkBounds(ChunkID{CX: k.CX, CY: k.CY})
		if !ok {
			t.Fatalf("chunk (%d,%d) ok=false", k.CX, k.CY)
		}
		if back := core.Vec2FromRenderPoint(core.V2(got.X, got.Y).ToRenderPoint()); back != (core.Vec2{X: got.X, Y: got.Y}) {
			t.Errorf("chunk (%d,%d) origin boundary = %v, want (%v,%v)", k.CX, k.CY, back, got.X, got.Y)
		}
	}
	// Same view twice lists the same chunks (no hidden state).
	k := mustFindChunkNeeded(t, f, "center128")
	a, b := c.Needed(chunkRect(k.View)), c.Needed(chunkRect(k.View))
	if !equalChunkIDs(a, b) {
		t.Fatalf("needed replay diverged: %v vs %v", a, b)
	}
	// Load twice loads once: the second pass adds nothing.
	view := chunkRect(f.LoadSeq[0].View)
	first := c.Load(view)
	second := c.Load(view)
	if len(first) == 0 || len(second) != 0 {
		t.Errorf("reload = %v then %v, want new then empty", first, second)
	}
	// C2: real camera viewport (S08 VisibleWorldRect) feeds the same path:
	// a 256x256 screen centered on the 16x16 map sees the middle 2x2 chunks.
	// game/tilemap never imports game/camera; the rect crosses as core.Rect.
	// (Values mirror camera center math: view=(cx-128,cy-128,256,256).)
	camView := core.NewRect(256-128, 256-128, 256, 256)
	if got := c.Needed(camView); !equalChunkIDs(got, chunkIDs([][2]int{{1, 1}, {2, 1}, {1, 2}, {2, 2}})) {
		t.Errorf("camera-fed needed = %v, want middle 2x2", got)
	}
	// Loaded copies cannot alias the grid.
	snap := c.Loaded()
	snap[0] = ChunkID{CX: 99, CY: 99}
	if c.IsLoaded(ChunkID{CX: 99, CY: 99}) {
		t.Error("Loaded aliases grid storage")
	}
	if again := c.Loaded(); !equalChunkIDs(again, chunkIDs(f.LoadSeq[0].LoadedAll)) {
		t.Errorf("loaded after alias write = %v, want %v", again, chunkIDs(f.LoadSeq[0].LoadedAll))
	}
}

// D:快移帧率有数:Needed加Update在64x64图8x8块上跑得动.
func TestChunkPerfFastMove(t *testing.T) {
	c, err := NewChunker(OrientOrthogonal, 64, 64, 32, 32, 8, 8)
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}
	// Seeded rand keeps the sweep replayable.
	r := core.NewRand(20260915)
	const reps = 2000
	views := make([]core.Rect, reps)
	for i := range views {
		views[i] = core.NewRect(float64(r.RangeInt(0, 64*32-256)), float64(r.RangeInt(0, 64*32-256)), 256, 256)
	}
	start := time.Now()
	visible := 0
	for _, v := range views {
		loaded, unloaded := c.Update(v)
		_ = loaded
		_ = unloaded
		visible += len(c.Visible(v))
	}
	el := time.Since(start)
	t.Logf("chunk-64: %d update+visible on 64x64/8x8 in %v (%.1f us/op, visible %d)", reps, el, float64(el.Microseconds())/reps, visible)
	if visible == 0 {
		t.Error("perf sweep saw nothing, benchmark invalid")
	}
}

// E:跑大圈不涨:镜头来回跳千次,装载集合恒等于视口所需.
func TestChunkLongRunStable(t *testing.T) {
	_, f := mustChunker(t)
	c, err := NewChunker(OrientOrthogonal, f.Grid.MapW, f.Grid.MapH, f.Grid.TileW, f.Grid.TileH, f.Grid.ChunkW, f.Grid.ChunkH)
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}
	a, b := chunkRect(f.TeleportRoundtrip.A), chunkRect(f.TeleportRoundtrip.B)
	for i := 0; i < 1000; i++ {
		l, u := c.Update(a)
		_ = l
		_ = u
		if all := c.Loaded(); !equalChunkIDs(all, c.Needed(a)) {
			t.Fatalf("rep %d A loaded = %v, want needed", i, all)
		}
		l, u = c.Update(b)
		_ = l
		_ = u
		if all := c.Loaded(); !equalChunkIDs(all, c.Needed(b)) {
			t.Fatalf("rep %d B loaded = %v, want needed", i, all)
		}
	}
	if n := c.LoadedCount(); n != len(c.Needed(b)) {
		t.Errorf("after 2k jumps loaded = %d, want %d", n, len(c.Needed(b)))
	}
	// Rebuild twice: same views load the same chunks, no drift.
	d, _ := NewChunker(OrientOrthogonal, f.Grid.MapW, f.Grid.MapH, f.Grid.TileW, f.Grid.TileH, f.Grid.ChunkW, f.Grid.ChunkH)
	d.Load(a)
	e, _ := NewChunker(OrientOrthogonal, f.Grid.MapW, f.Grid.MapH, f.Grid.TileW, f.Grid.TileH, f.Grid.ChunkW, f.Grid.ChunkH)
	e.Load(a)
	if !equalChunkIDs(d.Loaded(), e.Loaded()) {
		t.Errorf("rebuild diverged: %v vs %v", d.Loaded(), e.Loaded())
	}
}

// F:离屏金对照窗(W2意向game_tilemap--case=chunk,先离屏对比):冻结数加形状断言.
func TestChunkOffscreenGolden(t *testing.T) {
	c, f := mustChunker(t)
	// Golden numbers stay frozen.
	for _, k := range f.Needed {
		if got := c.Needed(chunkRect(k.View)); !equalChunkIDs(got, chunkIDs(k.Want)) {
			t.Fatalf("golden %s = %v, want %v", k.Name, got, chunkIDs(k.Want))
		}
	}
	// Shape: 16x16 tiles in 4x4 chunks is a 4x4 grid, stride one tile block.
	if c.ChunkCols() != 4 || c.ChunkRows() != 4 {
		t.Errorf("grid = %dx%d, want 4x4", c.ChunkCols(), c.ChunkRows())
	}
	a, _ := c.ChunkBounds(ChunkID{})
	b, _ := c.ChunkBounds(ChunkID{CX: 1})
	if b.X-a.X != f.Grid.TileW*float64(f.Grid.ChunkW) || b.Y != a.Y {
		t.Errorf("x stride = %v, want %v", b.X-a.X, f.Grid.TileW*float64(f.Grid.ChunkW))
	}
	below, _ := c.ChunkBounds(ChunkID{CY: 1})
	if below.Y-a.Y != f.Grid.TileH*float64(f.Grid.ChunkH) || below.X != a.X {
		t.Errorf("y stride = %v, want %v", below.Y-a.Y, f.Grid.TileH*float64(f.Grid.ChunkH))
	}
	if area := a.W * a.H; area != 128*128 {
		t.Errorf("chunk area = %v, want 16384", area)
	}
	// Shape: tile (15,15) sits in the last chunk, tile (0,0) in the first.
	if last, ok := c.ChunkOf(15, 15); !ok || last != (ChunkID{CX: 3, CY: 3}) {
		t.Errorf("tile (15,15) = %v/%v, want (3,3)/true", last, ok)
	}
	if first, ok := c.ChunkOf(0, 0); !ok || first != (ChunkID{}) {
		t.Errorf("tile (0,0) = %v/%v, want (0,0)/true", first, ok)
	}
	// Shape: TMX subset still parses (7.1 unbroken) and matches the grid.
	raw, err := os.ReadFile(filepath.Join("testdata", f.Grid.TMX))
	if err != nil {
		t.Fatalf("read %s: %v", f.Grid.TMX, err)
	}
	m, err := ParseTMX(raw)
	if err != nil {
		t.Fatalf("ParseTMX chunk map: %v", err)
	}
	if m.W() != f.Grid.MapW || m.H() != f.Grid.MapH || m.TileW() != f.Grid.TileW || m.TileH() != f.Grid.TileH {
		t.Errorf("chunk map = %dx%d %.0fx%.0f, want 16x16 32x32", m.W(), m.H(), m.TileW(), m.TileH())
	}
	// Every tile maps into exactly one chunk of this grid.
	for row := 0; row < m.H(); row++ {
		for col := 0; col < m.W(); col++ {
			if _, ok := c.ChunkOf(col, row); !ok {
				t.Fatalf("tile (%d,%d) has no chunk", col, row)
			}
		}
	}
}
