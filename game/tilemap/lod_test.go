package tilemap

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsLOD = 1e-9

type lodRange struct {
	Near float64 `json:"near"`
	Far  float64 `json:"far"`
	Band float64 `json:"band"`
}

type lodSelectCase struct {
	Dist float64 `json:"dist"`
	Cur  string  `json:"cur"`
	Want string  `json:"want"`
}

type lodDistCase struct {
	Focus [2]float64 `json:"focus"`
	Rect  [4]float64 `json:"rect"`
	Want  float64    `json:"want"`
	OK    bool       `json:"ok"`
}

type lodChunkCase struct {
	Name   string     `json:"name"`
	Focus  [2]float64 `json:"focus"`
	Bounds [4]float64 `json:"bounds"`
	Cur    string     `json:"cur"`
	Want   string     `json:"want"`
}

type lodWobble struct {
	Bounds   [4]float64 `json:"bounds"`
	BandA    [2]float64 `json:"band_a"`
	BandB    [2]float64 `json:"band_b"`
	Near     [2]float64 `json:"near"`
	Far      [2]float64 `json:"far"`
	EdgeNear [2]float64 `json:"edge_near"`
	EdgeFar  [2]float64 `json:"edge_far"`
}

type lodCameraCase struct {
	Name     string     `json:"name"`
	View     [4]float64 `json:"view"`
	Bounds   [4]float64 `json:"bounds"`
	WantDist float64    `json:"want_dist"`
	Cur      string     `json:"cur"`
	Want     string     `json:"want"`
}

type lodFile struct {
	LOD         lodRange        `json:"lod"`
	Select      []lodSelectCase `json:"select"`
	Dist        []lodDistCase   `json:"dist"`
	Chunks      []lodChunkCase  `json:"chunks"`
	Wobble      lodWobble       `json:"wobble"`
	CameraViews []lodCameraCase `json:"camera_views"`
}

func lodLevelOf(s string) (Level, bool) {
	switch s {
	case "near":
		return LevelNear, true
	case "far":
		return LevelFar, true
	}
	return Level(9), false
}

func loadLODCases(t *testing.T) lodFile {
	t.Helper()
	raw := mustReadTestdata(t, "lod_cases.json")
	var f lodFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode lod_cases.json: %v", err)
	}
	if len(f.Select) == 0 || len(f.Dist) == 0 || len(f.Chunks) == 0 {
		t.Fatal("lod_cases.json has no cases")
	}
	return f
}

func mustLOD(t *testing.T) (LOD, lodFile) {
	t.Helper()
	f := loadLODCases(t)
	l, err := NewLOD(f.LOD.Near, f.LOD.Far)
	if err != nil {
		t.Fatalf("NewLOD: %v", err)
	}
	if l.Near() != f.LOD.Near || l.Far() != f.LOD.Far || l.Band() != f.LOD.Band {
		t.Fatalf("lod = %.3f/%.3f band %.3f, want %.3f/%.3f band %.3f",
			l.Near(), l.Far(), l.Band(), f.LOD.Near, f.LOD.Far, f.LOD.Band)
	}
	return l, f
}

func lodVec(p [2]float64) core.Vec2 { return core.V2(p[0], p[1]) }

func closeLODDist(got, want float64) bool { return math.Abs(got-want) < epsLOD }

func mustFindLODChunk(t *testing.T, f lodFile, name string) lodChunkCase {
	t.Helper()
	for _, k := range f.Chunks {
		if k.Name == name {
			return k
		}
	}
	t.Fatalf("missing lod chunk case %q", name)
	return lodChunkCase{}
}

// A:远简近精切换对:距离选档、点到块距离、整块选档全落在冻结数上.
func TestLODFromCases(t *testing.T) {
	l, f := mustLOD(t)
	for i, k := range f.Select {
		cur, ok := lodLevelOf(k.Cur)
		if !ok {
			t.Fatalf("select[%d] bad cur %q", i, k.Cur)
		}
		want, ok := lodLevelOf(k.Want)
		if !ok {
			t.Fatalf("select[%d] bad want %q", i, k.Want)
		}
		if got := l.Select(k.Dist, cur); got != want {
			t.Errorf("select[%d] dist %v cur %v = %v, want %v", i, k.Dist, cur, got, want)
		}
	}
	for i, k := range f.Dist {
		got, ok := ChunkDist(lodVec(k.Focus), chunkRect(k.Rect))
		if ok != k.OK || (ok && !closeLODDist(got, k.Want)) {
			t.Errorf("dist[%d] focus %v rect %v = %v/%v, want %v/%v", i, k.Focus, k.Rect, got, ok, k.Want, k.OK)
		}
	}
	for _, k := range f.Chunks {
		cur, _ := lodLevelOf(k.Cur)
		want, _ := lodLevelOf(k.Want)
		got, ok := l.ChunkLevel(lodVec(k.Focus), chunkRect(k.Bounds), cur)
		if !ok || got != want {
			t.Errorf("chunk %s focus %v = %v/%v, want %v/true", k.Name, k.Focus, got, ok, want)
		}
	}
}

// B:临界不闪加坏路不崩:带内保旧档,空零坏数占位加报错,未冻格式先报占位.
func TestLODEdgesNoCrash(t *testing.T) {
	l, f := mustLOD(t)
	// Same distance inside the band keeps the old level: the no-flicker core.
	keepN := mustFindLODChunk(t, f, "band_keep_near")
	keepF := mustFindLODChunk(t, f, "band_keep_far")
	if keepN.Focus != keepF.Focus || keepN.Bounds != keepF.Bounds {
		t.Fatal("band pair must share geometry to prove hysteresis")
	}
	if keepN.Want == keepF.Want {
		t.Fatal("band pair must want different levels for the same distance")
	}
	// Bad thresholds are rejected, never a guessed band.
	for _, tc := range []struct {
		name      string
		near, far float64
		code      core.Code
	}{
		{"zero near", 0, 320, core.CodeInvalidArg},
		{"negative", -1, 320, core.CodeInvalidArg},
		{"equal", 320, 320, core.CodeInvalidArg},
		{"reversed", 320, 192, core.CodeInvalidArg},
		{"nan near", math.NaN(), 320, core.CodeInvalidArg},
		{"inf far", 192, math.Inf(1), core.CodeInvalidArg},
	} {
		if _, err := NewLOD(tc.near, tc.far); err == nil {
			t.Errorf("%s want error", tc.name)
		} else if core.CodeOf(err) != tc.code {
			t.Errorf("%s code = %v, want %v", tc.name, core.CodeOf(err), tc.code)
		}
	}
	// Bad distances keep the current level, never panic.
	for i, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := l.Select(bad, LevelNear); got != LevelNear {
			t.Errorf("bad dist[%d] near = %v, want near", i, got)
		}
		if got := l.Select(bad, LevelFar); got != LevelFar {
			t.Errorf("bad dist[%d] far = %v, want far", i, got)
		}
	}
	// Invalid current level inside the band splits at the midpoint, never panics.
	mid := (f.LOD.Near + f.LOD.Far) / 2
	if got := l.Select(f.LOD.Near+1, Level(9)); got != LevelNear {
		t.Errorf("invalid cur below mid = %v, want near", got)
	}
	if got := l.Select(f.LOD.Far-1, Level(9)); got != LevelFar {
		t.Errorf("invalid cur above mid = %v, want far", got)
	}
	if got := l.Select(mid, Level(9)); got != LevelFar {
		t.Errorf("invalid cur at mid = %v, want far", got)
	}
	// Bad geometry keeps the level with ok=false.
	badFocus := []core.Vec2{{X: math.NaN()}, {Y: math.Inf(1)}}
	for i, bf := range badFocus {
		if _, ok := ChunkDist(bf, chunkRect([4]float64{0, 0, 128, 128})); ok {
			t.Errorf("bad focus[%d] ok=true, want false", i)
		}
		if got, ok := l.ChunkLevel(bf, chunkRect([4]float64{0, 0, 128, 128}), LevelNear); ok || got != LevelNear {
			t.Errorf("bad focus[%d] level = %v/%v, want near/false", i, got, ok)
		}
	}
	for i, bad := range [][4]float64{
		{math.NaN(), 0, 128, 128}, {0, 0, math.Inf(1), 128}, {0, 0, 0, 0}, {0, 0, -10, 10},
	} {
		if _, ok := ChunkDist(core.V2(64, 64), chunkRect(bad)); ok {
			t.Errorf("bad rect[%d] ok=true, want false", i)
		}
	}
	// Zero LOD never panics (callers must use NewLOD for a real band).
	var zero LOD
	_ = zero.Select(10, LevelNear)
	_ = zero.Band()
	// Level names stay frozen for logs and JSON.
	if LevelNear.String() != "near" || LevelFar.String() != "far" || Level(9).String() != "unknown" {
		t.Error("level strings drifted, want near/far/unknown")
	}
	// TMX subset still holds: reserved shapes are Unsupported, never guessed.
	for _, tc := range []struct {
		file string
		code core.Code
	}{
		{"unsupported_hex.tmx", core.CodeUnsupported},
		{"unsupported_base64.tmx", core.CodeUnsupported},
		{"unsupported_ellipse.tmx", core.CodeUnsupported},
		{"bad_count.tmx", core.CodeBadData},
	} {
		raw := mustReadTestdata(t, tc.file)
		if _, err := ParseTMX(raw); err == nil {
			t.Errorf("%s want error", tc.file)
		} else if core.CodeOf(err) != tc.code {
			t.Errorf("%s code = %v, want %v", tc.file, core.CodeOf(err), tc.code)
		}
	}
}

// C:纯算数不画画,两边同数靠边界往返无损加逐位重放,视口只进core数.
func TestLODBoundaryIdentical(t *testing.T) {
	l, f := mustLOD(t)
	// Render boundary keeps every frozen focus and box origin lossless.
	for _, k := range f.Dist {
		focus := lodVec(k.Focus)
		if back := core.Vec2FromRenderPoint(focus.ToRenderPoint()); back != focus {
			t.Fatalf("focus %v boundary = %v", k.Focus, back)
		}
		box := chunkRect(k.Rect)
		origin := core.V2(box.X, box.Y)
		if back := core.Vec2FromRenderPoint(origin.ToRenderPoint()); back != origin {
			t.Fatalf("rect %v origin boundary = %v", k.Rect, back)
		}
		// Same inputs twice give the same distance and level.
		a, oka := ChunkDist(focus, box)
		b, okb := ChunkDist(focus, box)
		if oka != okb || a != b {
			t.Fatalf("dist replay diverged: %v/%v vs %v/%v", a, oka, b, okb)
		}
		cur, _ := lodLevelOf("near")
		if x, _ := l.ChunkLevel(focus, box, cur); x != func() Level { y, _ := l.ChunkLevel(focus, box, cur); return y }() {
			t.Fatal("level replay diverged")
		}
	}
	// Two selectors from the same numbers agree bit for bit.
	other, err := NewLOD(f.LOD.Near, f.LOD.Far)
	if err != nil {
		t.Fatalf("rebuild LOD: %v", err)
	}
	for _, k := range f.Select {
		cur, _ := lodLevelOf(k.Cur)
		if l.Select(k.Dist, cur) != other.Select(k.Dist, cur) {
			t.Fatalf("rebuild diverged at dist %v", k.Dist)
		}
	}
	// C2: real camera viewport (S08 VisibleWorldRect) feeds the same path:
	// focus is the view center, the rect crosses as core.Rect.
	// game/tilemap never imports game/camera.
	// (Values mirror camera center math: focus=(x+w/2,y+h/2).)
	for _, k := range f.CameraViews {
		view := chunkRect(k.View)
		focus := view.Center()
		dist, ok := ChunkDist(focus, chunkRect(k.Bounds))
		if !ok || !closeLODDist(dist, k.WantDist) {
			t.Errorf("camera %s dist = %v/%v, want %v/true", k.Name, dist, ok, k.WantDist)
		}
		cur, _ := lodLevelOf(k.Cur)
		want, _ := lodLevelOf(k.Want)
		if got, ok := l.ChunkLevel(focus, chunkRect(k.Bounds), cur); !ok || got != want {
			t.Errorf("camera %s level = %v/%v, want %v/true", k.Name, got, ok, want)
		}
	}
}

// D:切换耗时有数:万块选档在帧预算内跑得动.
func TestLODPerfSwitch(t *testing.T) {
	l, f := mustLOD(t)
	// Synthetic load only (no golden): golden stays in lod_cases.json.
	// Seeded rand keeps the sweep replayable.
	r := core.NewRand(20260915)
	const reps = 20000
	dists := make([]float64, reps)
	curs := make([]Level, reps)
	for i := range dists {
		dists[i] = r.RangeFloat(0, 600)
		if r.RangeInt(0, 2) == 0 {
			curs[i] = LevelNear
		} else {
			curs[i] = LevelFar
		}
	}
	bounds := chunkRect(f.Wobble.Bounds)
	focusNear := lodVec(f.Wobble.Near)
	start := time.Now()
	near, far := 0, 0
	for i := range dists {
		got := l.Select(dists[i], curs[i])
		if got == LevelNear {
			near++
		} else {
			far++
		}
		if gl, ok := l.ChunkLevel(focusNear, bounds, curs[i]); !ok {
			t.Fatal("perf ChunkLevel ok=false")
		} else if gl == LevelNear {
			near++
		} else {
			far++
		}
	}
	el := time.Since(start)
	t.Logf("lod-switch: %d select+chunk on band %.0f-%.0f in %v (%.1f ns/op, near %d far %d)",
		reps, f.LOD.Near, f.LOD.Far, el, float64(el.Nanoseconds())/float64(2*reps), near, far)
	if near == 0 || far == 0 {
		t.Error("perf sweep saw one side only, benchmark invalid")
	}
}

// E:长跑不抖:带内来回万次不翻,过线只切一次,双重放逐位一致.
func TestLODLongRunStable(t *testing.T) {
	l, f := mustLOD(t)
	bounds := chunkRect(f.Wobble.Bounds)
	bandA, bandB := lodVec(f.Wobble.BandA), lodVec(f.Wobble.BandB)
	nearF, farF := lodVec(f.Wobble.Near), lodVec(f.Wobble.Far)
	edgeFar := lodVec(f.Wobble.EdgeFar)
	// Inside the band 5000 round trips never flip either side.
	cur := LevelNear
	for i := 0; i < 5000; i++ {
		var ok bool
		if cur, ok = l.ChunkLevel(bandA, bounds, cur); !ok || cur != LevelNear {
			t.Fatalf("rep %d bandA near flipped to %v/%v", i, cur, ok)
		}
		if cur, ok = l.ChunkLevel(bandB, bounds, cur); !ok || cur != LevelNear {
			t.Fatalf("rep %d bandB near flipped to %v/%v", i, cur, ok)
		}
	}
	cur = LevelFar
	for i := 0; i < 5000; i++ {
		var ok bool
		if cur, ok = l.ChunkLevel(bandA, bounds, cur); !ok || cur != LevelFar {
			t.Fatalf("rep %d bandA far flipped to %v/%v", i, cur, ok)
		}
		if cur, ok = l.ChunkLevel(bandB, bounds, cur); !ok || cur != LevelFar {
			t.Fatalf("rep %d bandB far flipped to %v/%v", i, cur, ok)
		}
	}
	// Crossing the far edge then wobbling inside switches exactly once.
	cur = LevelNear
	switches := 0
	prev := cur
	for i := 0; i < 1000; i++ {
		next, ok := l.ChunkLevel(edgeFar, bounds, cur)
		if !ok {
			t.Fatalf("rep %d edge ok=false", i)
		}
		if next != prev {
			switches++
		}
		prev, cur = next, next
		next, ok = l.ChunkLevel(bandA, bounds, cur)
		if !ok {
			t.Fatalf("rep %d band ok=false", i)
		}
		if next != prev {
			switches++
		}
		prev, cur = next, next
	}
	if switches != 1 {
		t.Errorf("edge wobble switches = %d, want 1", switches)
	}
	if cur != LevelFar {
		t.Errorf("after wobble = %v, want far", cur)
	}
	// Far and near anchors still land after the soak.
	if got, _ := l.ChunkLevel(farF, bounds, cur); got != LevelFar {
		t.Errorf("far anchor = %v, want far", got)
	}
	if got, _ := l.ChunkLevel(nearF, bounds, LevelFar); got != LevelNear {
		t.Errorf("near anchor = %v, want near", got)
	}
	// Rebuild twice: same focus path gives the same levels, no drift.
	path := []core.Vec2{bandA, bandB, nearF, farF, edgeFar}
	a, b := LevelNear, LevelNear
	for i := 0; i < 2000; i++ {
		p := path[i%len(path)]
		na, oka := l.ChunkLevel(p, bounds, a)
		nb, okb := l.ChunkLevel(p, bounds, b)
		if !oka || !okb || na != nb {
			t.Fatalf("rep %d rebuild diverged: %v/%v vs %v/%v", i, na, oka, nb, okb)
		}
		a, b = na, nb
	}
}

// F:离屏金对照窗(W3意向game_tilemap--case=lod,先离屏对比):冻结数加形状断言.
func TestLODOffscreenGolden(t *testing.T) {
	l, f := mustLOD(t)
	// Golden numbers stay frozen.
	for _, k := range f.Select {
		cur, _ := lodLevelOf(k.Cur)
		want, _ := lodLevelOf(k.Want)
		if got := l.Select(k.Dist, cur); got != want {
			t.Fatalf("golden select %v/%v = %v, want %v", k.Dist, cur, got, want)
		}
	}
	for _, k := range f.Dist {
		if got, ok := ChunkDist(lodVec(k.Focus), chunkRect(k.Rect)); !ok && k.OK {
			t.Fatalf("golden dist %v ok=false, want true", k.Focus)
		} else if ok && !closeLODDist(got, k.Want) {
			t.Fatalf("golden dist %v = %v, want %v", k.Focus, got, k.Want)
		}
	}
	// Shape: thresholds order the band, hysteresis needs a real width.
	if !(l.Near() > 0) || !(l.Far() > l.Near()) || l.Band() != l.Far()-l.Near() || l.Band() <= 0 {
		t.Errorf("band shape = %.3f/%.3f/%.3f, want 0 < near < far", l.Near(), l.Far(), l.Band())
	}
	// Shape: nearer focus measures smaller on the same box.
	bounds := chunkRect(f.Wobble.Bounds)
	dNear, _ := ChunkDist(lodVec(f.Wobble.Near), bounds)
	dBand, _ := ChunkDist(lodVec(f.Wobble.BandA), bounds)
	dFar, _ := ChunkDist(lodVec(f.Wobble.Far), bounds)
	if !(dNear < dBand && dBand < dFar) {
		t.Errorf("dist order = %v < %v < %v, want increasing", dNear, dBand, dFar)
	}
	// Shape: the same 128x128 box has area 16384, like the 7.2 chunk box.
	if area := bounds.W * bounds.H; area != 16384 {
		t.Errorf("box area = %v, want 16384", area)
	}
	// Shape: 7.2 grid still feeds LOD: chunk (0,0) near the map center is
	// detailed, the opposite corner from a far focus is simplified.
	grid, err := NewChunker(OrientOrthogonal, 16, 16, 32, 32, 4, 4)
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}
	b00, ok := grid.ChunkBounds(ChunkID{})
	if !ok {
		t.Fatal("chunk (0,0) ok=false")
	}
	farFocus := lodVec(f.Wobble.Far)
	nearFocus := b00.Center()
	if got, _ := l.ChunkLevel(nearFocus, b00, LevelFar); got != LevelNear {
		t.Errorf("grid center level = %v, want near", got)
	}
	last, ok := grid.ChunkBounds(ChunkID{CX: 3, CY: 3})
	if !ok {
		t.Fatal("chunk (3,3) ok=false")
	}
	if got, _ := l.ChunkLevel(farFocus, last, LevelNear); got != LevelFar {
		t.Errorf("grid far level = %v, want far", got)
	}
	// Shape: far_diag geometry is the true far-side prove-out.
	diag := mustFindLODChunk(t, f, "far_diag")
	dcur, _ := lodLevelOf(diag.Cur)
	dwant, _ := lodLevelOf(diag.Want)
	if got, _ := l.ChunkLevel(lodVec(diag.Focus), chunkRect(diag.Bounds), dcur); got != dwant {
		t.Errorf("far_diag level = %v, want %v", got, dwant)
	}
}
