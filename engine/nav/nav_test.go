//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package nav

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/engine/core"
)

type gridRectFull struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type gridFile struct {
	Name      string         `json:"name"`
	W         int            `json:"w"`
	H         int            `json:"h"`
	Cell      float64        `json:"cell"`
	PerFrame  int            `json:"per_frame"`
	BakeMsMax float64        `json:"bake_ms_max"`
	Start     [2]int         `json:"start"`
	Goal      [2]int         `json:"goal"`
	Walls     []gridRectFull `json:"walls"`
}

type smallCase struct {
	Name      string `json:"name"`
	Start     [2]int `json:"start"`
	Goal      [2]int `json:"goal"`
	Want      string `json:"want_status"`
	WantMinLn int    `json:"want_min_len"`
}

type cornerFile struct {
	W     int      `json:"w"`
	H     int      `json:"h"`
	Walls [][2]int `json:"walls"`
	Start [2]int   `json:"start"`
	Goal  [2]int   `json:"goal"`
	Want  string   `json:"want_status"`
}

type casesFile struct {
	Name      string      `json:"name"`
	W         int         `json:"w"`
	H         int         `json:"h"`
	Cell      float64     `json:"cell"`
	Speed     float64     `json:"speed"`
	ArriveTol float64     `json:"arrive_tol"`
	PerFrame  int         `json:"per_frame"`
	Walls     [][2]int    `json:"walls"`
	Cases     []smallCase `json:"cases"`
	Corner    cornerFile  `json:"corner"`
}

func loadGridFile(t *testing.T) gridFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "nav_grid.json"))
	if err != nil {
		t.Fatalf("read nav_grid.json: %v", err)
	}
	var f gridFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode nav_grid.json: %v", err)
	}
	if f.W != 256 || f.H != 256 || f.PerFrame != 8 || len(f.Walls) == 0 {
		t.Fatalf("nav_grid.json not the 256 bake tier: %+v", f)
	}
	return f
}

func loadCasesFile(t *testing.T) casesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "nav_cases.json"))
	if err != nil {
		t.Fatalf("read nav_cases.json: %v", err)
	}
	var f casesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode nav_cases.json: %v", err)
	}
	if len(f.Cases) != 3 || len(f.Walls) == 0 {
		t.Fatalf("nav_cases.json missing 3 cases: %+v", len(f.Cases))
	}
	return f
}

func bakeLarge(t *testing.T, f gridFile) Grid {
	t.Helper()
	g, err := Bake(f.W, f.H, func(x, y int) bool {
		for _, r := range f.Walls {
			if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
				return true
			}
		}
		return false
	})
	if err != nil {
		t.Fatalf("Bake 256: %v", err)
	}
	return g
}

func bakeSmall(t *testing.T, f casesFile) Grid {
	t.Helper()
	wall := map[[2]int]bool{}
	for _, w := range f.Walls {
		wall[w] = true
	}
	g, err := Bake(f.W, f.H, func(x, y int) bool { return wall[[2]int{x, y}] })
	if err != nil {
		t.Fatalf("Bake small: %v", err)
	}
	return g
}

// A:256大网格烘焙200ms内且逐字节一致.
func TestNavBake256Deterministic(t *testing.T) {
	f := loadGridFile(t)
	t0 := time.Now()
	g := bakeLarge(t, f)
	el := time.Since(t0)
	ms := float64(el.Microseconds()) / 1000.0
	if ms > f.BakeMsMax {
		t.Fatalf("bake 256 = %.1fms, want <= %.0fms", ms, f.BakeMsMax)
	}
	g2 := bakeLarge(t, f)
	if !g.Equal(g2) {
		t.Fatal("same input baked different grids")
	}
	b1, b2 := g.Encode(), g2.Encode()
	if len(b1) != 256*256 || len(b2) != len(b1) {
		t.Fatalf("encode len = %d/%d, want 65536", len(b1), len(b2))
	}
	for i := range b1 {
		if b1[i] != b2[i] {
			t.Fatalf("encode byte %d moved %d vs %d", i, b1[i], b2[i])
		}
	}
	back, err := Decode(f.W, f.H, b1)
	if err != nil {
		t.Fatalf("Decode roundtrip: %v", err)
	}
	if !g.Equal(back) {
		t.Fatal("decode roundtrip diverged")
	}
	t.Logf("bake-256: %.1fms (gate %.0f) solids=%d bytes=%d", ms, f.BakeMsMax, g.SolidCount(), len(b1))
}

// B:坏网格报错不崩.
func TestNavBakeEdgesNoCrash(t *testing.T) {
	for _, wh := range [][2]int{{0, 10}, {10, 0}, {-1, 5}, {2048, 4}} {
		if _, err := NewGrid(wh[0], wh[1]); core.CodeOf(err) != core.CodeInvalidArg && core.CodeOf(err) != core.CodeOutOfMemory {
			t.Errorf("NewGrid %v code = %v, want invalid-arg/out-of-memory", wh, core.CodeOf(err))
		}
		if _, err := Bake(wh[0], wh[1], nil); err == nil {
			t.Errorf("Bake %v = nil error, want fault", wh)
		}
	}
	if _, err := Decode(4, 4, make([]byte, 15)); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("short decode code = %v, want bad-data", core.CodeOf(err))
	}
	bad := append(make([]byte, 16), 0)
	bad[3] = 7
	if _, err := Decode(4, 4, bad[:16]); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("value decode code = %v, want bad-data", core.CodeOf(err))
	}
	var nilGrid Grid
	if nilGrid.Valid() || nilGrid.W() != 0 || nilGrid.Encode() != nil {
		t.Error("zero grid reports valid content")
	}
	if _, _, err := FindPath(nilGrid, Cell{}, Cell{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("path on zero grid code = %v, want invalid-arg", core.CodeOf(err))
	}
	g, err := NewGrid(4, 4)
	if err != nil {
		t.Fatalf("NewGrid 4x4: %v", err)
	}
	if _, _, err := FindPath(g, Cell{-1, 0}, Cell{1, 1}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("oob start code = %v, want invalid-arg", core.CodeOf(err))
	}
}

// C:小网格有路无路起点在墙里三种终态.
func TestNavCasesTerminal(t *testing.T) {
	f := loadCasesFile(t)
	g := bakeSmall(t, f)
	byName := map[string]smallCase{}
	for _, c := range f.Cases {
		byName[c.Name] = c
	}
	for _, want := range []string{"have_path", "no_path", "start_blocked"} {
		c, ok := byName[want]
		if !ok {
			t.Fatalf("nav_cases.json has no case %q", want)
		}
		wantSt, ok := ParseStatus(c.Want)
		if !ok {
			t.Fatalf("case %s want %q unparsable", c.Name, c.Want)
		}
		path, st, err := FindPath(g, Cell{c.Start[0], c.Start[1]}, Cell{c.Goal[0], c.Goal[1]})
		if st != wantSt {
			t.Fatalf("case %s status = %v, want %v (err %v)", c.Name, st, wantSt, err)
		}
		switch wantSt {
		case StatusFound:
			if err != nil || len(path) == 0 {
				t.Fatalf("have_path err=%v len=%d", err, len(path))
			}
			if path[0] != (Cell{c.Start[0], c.Start[1]}) || path[len(path)-1] != (Cell{c.Goal[0], c.Goal[1]}) {
				t.Fatalf("have_path endpoints = %v..%v", path[0], path[len(path)-1])
			}
			if c.WantMinLn > 0 && len(path) < c.WantMinLn {
				t.Fatalf("have_path len = %d, want >= %d", len(path), c.WantMinLn)
			}
			assertWalkable(t, g, path)
		case StatusNoPath:
			if err != nil || len(path) != 0 {
				t.Fatalf("no_path err=%v len=%d, want nil terminal", err, len(path))
			}
		case StatusStartBlocked:
			if core.CodeOf(err) != core.CodeInvalidArg || len(path) != 0 {
				t.Fatalf("start_blocked err=%v len=%d, want invalid-arg + nil", err, len(path))
			}
		}
		// Replay determinism: same query twice, same chain.
		p2, st2, _ := FindPath(g, Cell{c.Start[0], c.Start[1]}, Cell{c.Goal[0], c.Goal[1]})
		if st2 != st || len(p2) != len(path) {
			t.Fatalf("case %s replay diverged", c.Name)
		}
		for i := range path {
			if p2[i] != path[i] {
				t.Fatalf("case %s replay cell %d moved", c.Name, i)
			}
		}
	}
}

// D:斜行不穿墙角.
func TestNavCornerRefused(t *testing.T) {
	f := loadCasesFile(t)
	c := f.Corner
	wall := map[[2]int]bool{}
	for _, w := range c.Walls {
		wall[w] = true
	}
	g, err := Bake(c.W, c.H, func(x, y int) bool { return wall[[2]int{x, y}] })
	if err != nil {
		t.Fatalf("Bake corner: %v", err)
	}
	want, ok := ParseStatus(c.Want)
	if !ok {
		t.Fatalf("corner want %q unparsable", c.Want)
	}
	path, st, _ := FindPath(g, Cell{c.Start[0], c.Start[1]}, Cell{c.Goal[0], c.Goal[1]})
	if st != want {
		t.Fatalf("corner status = %v, want %v (path %v)", st, want, path)
	}
	if len(path) != 0 {
		t.Fatalf("corner path = %v, want nil (cut refused)", path)
	}
	// Same grid without the corner rule would step out diagonally at once;
	// with the rule the start cell has no legal move: all 8 neighbours are
	// walls, edges, or corner-blocked diagonals.
}

// E:寻路链合法:每步相邻且不踩墙,斜步两侧正交格须空.
func assertWalkable(t *testing.T, g Grid, path []Cell) {
	t.Helper()
	for i, c := range path {
		s, ok := g.SolidAt(c.X, c.Y)
		if !ok || s {
			t.Fatalf("path[%d] = %v blocked", i, c)
		}
		if i == 0 {
			continue
		}
		dx, dy := c.X-path[i-1].X, c.Y-path[i-1].Y
		if dx < -1 || dx > 1 || dy < -1 || dy > 1 || (dx == 0 && dy == 0) {
			t.Fatalf("path[%d] = %v jumps from %v", i, c, path[i-1])
		}
		if dx != 0 && dy != 0 {
			o1, _ := g.SolidAt(path[i-1].X+dx, path[i-1].Y)
			o2, _ := g.SolidAt(path[i-1].X, path[i-1].Y+dy)
			if o1 || o2 {
				t.Fatalf("path[%d] = %v cuts wall corner", i, c)
			}
		}
	}
}

// F:到点容差2,速度200.
func TestNavAgentArrival(t *testing.T) {
	f := loadCasesFile(t)
	if f.Speed != DefaultSpeed || f.ArriveTol != ArriveTol {
		t.Fatalf("nav_cases speed/tol = %v/%v, want %v/%v", f.Speed, f.ArriveTol, DefaultSpeed, ArriveTol)
	}
	g := bakeSmall(t, f)
	var have smallCase
	for _, c := range f.Cases {
		if c.Name == "have_path" {
			have = c
		}
	}
	cells, st, err := FindPath(g, Cell{have.Start[0], have.Start[1]}, Cell{have.Goal[0], have.Goal[1]})
	if err != nil || st != StatusFound {
		t.Fatalf("have_path setup: %v/%v", st, err)
	}
	origin := core.V2(0, 0)
	startW := CellToWorld(cells[0], f.Cell, origin)
	a, err := NewAgent(startW, f.Speed)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if err := a.SetCells(cells[1:], f.Cell, origin); err != nil {
		t.Fatalf("SetCells: %v", err)
	}
	const dt = 1.0 / 60.0
	steps := 0
	for !a.Done() && steps < 60*60 {
		arrived, err := a.Step(dt)
		if err != nil {
			t.Fatalf("Step %d: %v", steps, err)
		}
		steps++
		if arrived && !a.Done() {
			t.Fatalf("step reports arrived before done at %d", steps)
		}
	}
	if !a.Done() {
		t.Fatalf("agent never arrived in %d steps", steps)
	}
	goalW := CellToWorld(cells[len(cells)-1], f.Cell, origin)
	if a.Pos().Sub(goalW).Length() > ArriveTol+1e-9 {
		t.Fatalf("final miss = %v vs %v", a.Pos(), goalW)
	}
	t.Logf("agent: cells=%d steps=%d speed=%.0f tol=%.0f", len(cells), steps, f.Speed, f.ArriveTol)

	if _, err := NewAgent(startW, 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero speed code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := a.Step(-1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative dt code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := a.SetWaypoints([]core.Vec2{{X: 1, Y: 0}}); err != nil {
		t.Fatalf("SetWaypoints good: %v", err)
	}
}

// G:同屏100怪分摊排队,每帧8条约13帧排完.
func TestNavQueueDrain(t *testing.T) {
	gf := loadGridFile(t)
	cf := loadCasesFile(t)
	if gf.PerFrame != DefaultPerFrame || cf.PerFrame != DefaultPerFrame {
		t.Fatalf("per_frame = %d/%d, want %d", gf.PerFrame, cf.PerFrame, DefaultPerFrame)
	}
	if DrainFrames(100, 8) != 13 {
		t.Fatalf("DrainFrames(100,8) = %d, want 13", DrainFrames(100, 8))
	}
	g := bakeSmall(t, cf)
	var have smallCase
	for _, c := range cf.Cases {
		if c.Name == "have_path" {
			have = c
		}
	}
	var q Queue = NewQueue()
	if q.PerFrame() != 8 {
		t.Fatalf("default per-frame = %d, want 8", q.PerFrame())
	}
	if err := q.SetPerFrame(0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("per_frame 0 code = %v, want invalid-arg", core.CodeOf(err))
	}
	const total = 100
	for i := 0; i < total; i++ {
		sx := (have.Start[0] + i) % cf.W
		start := Cell{sx, have.Start[1]}
		if s, _ := g.SolidAt(start.X, start.Y); s {
			start = Cell{have.Start[0], have.Start[1]}
		}
		q.Enqueue(start, Cell{have.Goal[0], have.Goal[1]})
	}
	frames := 0
	for q.Pending() > 0 && frames < 100 {
		n, err := q.Update(g)
		if err != nil {
			t.Fatalf("Update frame %d: %v", frames, err)
		}
		if n > 8 {
			t.Fatalf("frame %d computed %d, want <= 8", frames, n)
		}
		frames++
	}
	if q.Pending() != 0 || q.Done() != total {
		t.Fatalf("drain pending=%d done=%d, want 0/100", q.Pending(), q.Done())
	}
	if frames != 13 {
		t.Fatalf("drain frames = %d, want 13", frames)
	}
	for _, r := range q.Results() {
		if r.Status != StatusFound || len(r.Path) == 0 {
			t.Fatalf("job %d status = %v len=%d, want found", r.ID, r.Status, len(r.Path))
		}
	}
	t.Logf("queue: 100 jobs x 8/frame = %d frames", frames)

	raw := []byte(`{"per_frame": 8}`)
	tun, err := LoadTuning(raw)
	if err != nil || tun.PerFrame != 8 {
		t.Fatalf("LoadTuning good: %v %+v", err, tun)
	}
	if _, err := LoadTuning([]byte(`{"per_frame": 0}`)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("tuning 0 code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := LoadTuning([]byte(`{bad}`)); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("tuning syntax code = %v, want bad-data", core.CodeOf(err))
	}
}
