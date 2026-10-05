package tilemap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/engine/core"
)

type largeGrid struct {
	MapW   int     `json:"map_w"`
	MapH   int     `json:"map_h"`
	TileW  float64 `json:"tile_w"`
	TileH  float64 `json:"tile_h"`
	ChunkW int     `json:"chunk_w"`
	ChunkH int     `json:"chunk_h"`
	Cols   int     `json:"cols"`
	Rows   int     `json:"rows"`
	Count  int     `json:"count"`
	TMX    string  `json:"tmx"`
}

type largeOf struct {
	Col int `json:"col"`
	Row int `json:"row"`
	CX  int `json:"cx"`
	CY  int `json:"cy"`
}

type largeView struct {
	Name       string     `json:"name"`
	View       [4]float64 `json:"view"`
	WantChunks [][2]int   `json:"want_chunks"`
}

type largeMapChecks struct {
	W          int      `json:"w"`
	H          int      `json:"h"`
	At00       int      `json:"at_0_0"`
	Objects    int      `json:"objects"`
	SpawnNames []string `json:"spawn_names"`
}

type largeFile struct {
	Grid          largeGrid      `json:"grid"`
	BuildMsMax    float64        `json:"build_ms_max"`
	MergePerFrame int            `json:"merge_per_frame"`
	FrameMsMax    float64        `json:"frame_ms_max"`
	Of            []largeOf      `json:"of"`
	Views         []largeView    `json:"views"`
	MapChecks     largeMapChecks `json:"map_checks"`
}

func loadLargeCases(t *testing.T) largeFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "large_budget.json"))
	if err != nil {
		t.Fatalf("read large_budget.json: %v", err)
	}
	var f largeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode large_budget.json: %v", err)
	}
	if f.Grid.Count == 0 {
		t.Fatal("large_budget.json has no grid")
	}
	return f
}

func sameChunkSet(got []ChunkID, want [][2]int) bool {
	if len(got) != len(want) {
		return false
	}
	m := make(map[ChunkID]struct{}, len(got))
	for _, id := range got {
		m[id] = struct{}{}
	}
	for _, w := range want {
		if _, ok := m[ChunkID{CX: w[0], CY: w[1]}]; !ok {
			return false
		}
	}
	return true
}

// A: index geometry over the million-tile map comes from the frozen file.
func TestLargeFromCases(t *testing.T) {
	f := loadLargeCases(t)
	g := f.Grid
	start := time.Now()
	lg, err := NewLargeSized(OrientOrthogonal, g.MapW, g.MapH, g.TileW, g.TileH, g.ChunkW, g.ChunkH)
	if err != nil {
		t.Fatalf("NewLargeSized: %v", err)
	}
	if el := time.Since(start); el.Seconds()*1000 > f.BuildMsMax {
		t.Errorf("index build %v exceeds %.0fms", el, f.BuildMsMax)
	}
	if lg.Cols() != g.Cols || lg.Rows() != g.Rows || lg.ChunkCount() != g.Count {
		t.Errorf("grid = %dx%d count %d, want %dx%d count %d",
			lg.Cols(), lg.Rows(), lg.ChunkCount(), g.Cols, g.Rows, g.Count)
	}
	for _, c := range f.Of {
		id, ok := lg.ChunkOf(c.Col, c.Row)
		if !ok || id.CX != c.CX || id.CY != c.CY {
			t.Errorf("ChunkOf(%d,%d) = %v/%v, want (%d,%d)",
				c.Col, c.Row, id, ok, c.CX, c.CY)
		}
	}
	for _, v := range f.Views {
		view := core.NewRect(v.View[0], v.View[1], v.View[2], v.View[3])
		if got := lg.Needed(view); !sameChunkSet(got, v.WantChunks) {
			t.Errorf("%s: needed = %v, want %v", v.Name, got, v.WantChunks)
		}
	}
}

// B: two-phase streaming honors the per-frame budget; jumps never strand.
func TestLargeMergeBudget(t *testing.T) {
	f := loadLargeCases(t)
	g := f.Grid
	lg, err := NewLargeSized(OrientOrthogonal, g.MapW, g.MapH, g.TileW, g.TileH, g.ChunkW, g.ChunkH)
	if err != nil {
		t.Fatalf("NewLargeSized: %v", err)
	}
	view := core.NewRect(0, 0, 2048, 2048)
	need := lg.Needed(view)
	if len(need) != 4 {
		t.Fatalf("wide view needed = %d, want 4", len(need))
	}
	queued := lg.Request(view)
	if len(queued) != 4 || lg.PendingCount() != 4 {
		t.Fatalf("request = %d pending %d, want 4/4", len(queued), lg.PendingCount())
	}
	// Same view twice never double-queues.
	if again := lg.Request(view); len(again) != 0 {
		t.Errorf("re-request = %d, want 0", len(again))
	}
	start := time.Now()
	merged := lg.MergeBudget(2)
	el := time.Since(start)
	if len(merged) != 2 || lg.LoadedCount() != 2 || lg.PendingCount() != 2 {
		t.Errorf("merge2 = %d loaded %d pending %d, want 2/2/2",
			len(merged), lg.LoadedCount(), lg.PendingCount())
	}
	if el.Seconds()*1000 > f.FrameMsMax {
		t.Errorf("merge frame %v exceeds %.0fms", el, f.FrameMsMax)
	}
	rest := lg.MergeBudget(f.MergePerFrame)
	if len(rest) != 2 || lg.LoadedCount() != 4 || lg.PendingCount() != 0 {
		t.Errorf("merge rest = %d loaded %d pending %d, want 2/4/0",
			len(rest), lg.LoadedCount(), lg.PendingCount())
	}
	// Unloaded chunks read as missing (magenta), loaded ones draw.
	if !lg.IsMissing(ChunkID{CX: 5, CY: 5}) {
		t.Error("far chunk not missing, want placeholder")
	}
	if lg.IsMissing(need[0]) {
		t.Errorf("loaded chunk %v reads missing", need[0])
	}
	if vis := lg.Visible(view); !sameChunkSet(vis, [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}}) {
		t.Errorf("visible = %v, want 2x2", vis)
	}
	// Jump across the map: stale chunks drop in the same step, nothing grows.
	jump := core.NewRect(31744, 31744, 1024, 1024)
	mg, dropped := lg.Update(jump, f.MergePerFrame)
	if len(mg) != 1 || len(dropped) != 4 {
		t.Errorf("jump merge=%d drop=%d, want 1/4", len(mg), len(dropped))
	}
	if lg.LoadedCount() != 1 || !lg.IsLoaded(ChunkID{CX: 31, CY: 31}) {
		t.Errorf("after jump loaded = %v, want [(31,31)]", lg.Loaded())
	}
	// Bad views change nothing, never panic.
	before := lg.LoadedCount()
	if m, d := lg.Update(core.Rect{}, 4); len(m) != 0 || len(d) != 0 {
		t.Errorf("empty view update = %d/%d, want 0/0", len(m), len(d))
	}
	if lg.LoadedCount() != before {
		t.Error("empty view changed loaded set")
	}
	if m := lg.MergeBudget(0); len(m) != 0 {
		t.Errorf("merge 0 = %d, want 0", len(m))
	}
	if m := lg.MergeBudget(-3); len(m) != 0 {
		t.Errorf("merge -3 = %d, want 0", len(m))
	}
	var nilLarge *Large
	if nilLarge.Needed(view) != nil || nilLarge.Visible(view) != nil ||
		nilLarge.Loaded() != nil || nilLarge.IsLoaded(ChunkID{}) ||
		nilLarge.IsMissing(ChunkID{}) || nilLarge.Request(view) != nil ||
		nilLarge.MergeBudget(2) != nil || nilLarge.LoadedCount() != 0 ||
		nilLarge.PendingCount() != 0 || nilLarge.Requests() != 0 || nilLarge.Merges() != 0 {
		t.Error("nil large not silent")
	}
	nilLarge.Reset()
}

// C: oversized grids fail fast; bad sizes are InvalidArg.
func TestLargeEdgesNoCrash(t *testing.T) {
	if _, err := NewLarge(0, 1024); err == nil || core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero width err = %v, want invalid-arg", err)
	}
	if _, err := NewLargeSized(OrientOrthogonal, 1024, 1024, 32, 32, 0, 32); err == nil ||
		core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero chunk err = %v, want invalid-arg", err)
	}
	if _, err := NewLarge(4096, 4096); err == nil || core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("16M tiles err = %v, want out-of-memory", err)
	}
	if _, err := NewLarge(2048, 2048); err == nil || core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("4M tiles err = %v, want out-of-memory", err)
	}
	var nilLarge *Large
	if got := nilLarge.W(); got != 0 {
		t.Errorf("nil W = %d, want 0", got)
	}
}

// D: the large TMX parses under the frozen subset: firstgid exact,
// spawns present, brush self-joins without seams.
func TestLargeMapParse(t *testing.T) {
	f := loadLargeCases(t)
	raw, err := os.ReadFile(filepath.Join("testdata", f.Grid.TMX))
	if err != nil {
		t.Fatalf("read %s: %v", f.Grid.TMX, err)
	}
	m, err := ParseTMX(raw)
	if err != nil {
		t.Fatalf("ParseTMX: %v", err)
	}
	mc := f.MapChecks
	if m.W() != mc.W || m.H() != mc.H {
		t.Errorf("map = %dx%d, want %dx%d", m.W(), m.H(), mc.W, mc.H)
	}
	if gid, ok := m.At(0, 0, 0); !ok || gid != mc.At00 {
		t.Errorf("At(0,0,0) = %d/%v, want %d", gid, ok, mc.At00)
	}
	objs := m.Objects()
	if len(objs) != mc.Objects {
		t.Errorf("objects = %d, want %d", len(objs), mc.Objects)
	}
	names := map[string]bool{}
	for _, o := range objs {
		names[o.Name()] = true
	}
	for _, n := range mc.SpawnNames {
		if !names[n] {
			t.Errorf("spawn %q missing in %v", n, objs)
		}
	}
	// Brush self-join: a 2x2 solid block has no seam breaks (mask corners
	// join, AutoVariant4 stays in 0..15 on the ground layer).
	for r := 0; r < 2; r++ {
		for c := 0; c < 2; c++ {
			mask, ok := m.AutoMaskAt(0, c, r)
			if !ok || mask < 0 || mask > 15 {
				t.Errorf("AutoMaskAt(0,%d,%d) = %d/%v, want 0..15", c, r, mask, ok)
			}
		}
	}
}
