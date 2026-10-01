// Command game_tilemap is the S57 7.1/7.2/7.3 independent window.
//
// Three cases share one shell shape, each drives the real engine/tilemap
// package read-only (the window never writes engine code, it only calls
// the frozen map/chunk/lod APIs and draws the returned cells with the
// existing render draws):
//
//	map   (7.1 tiles): TMX-subset parse, firstgid exactness, monster/object
//	  placement, terrain-brush stroke self-joining without seams.
//	chunk (7.2 partitions): viewport culling of a 10k-tile field, batched
//	  load/unload while walking, teleport stability.
//	lod   (7.3 distance): near/far switch with a hysteresis band so a
//	  jittering camera never flickers.
//
// Modes (same shape as game_step/game_save):
//
//	go run ./examples/game_tilemap --case=map -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/game_tilemap --case=map -manual-seconds 60
//	  manual for 60s (events logged, title shows the count), then summary.
//	go run ./examples/game_tilemap --case=chunk
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800. First run writes the per-case golden baseline into
// testdata/; later runs compare it with zero tolerance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/tilemap"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "tilemap-s57"
)

// Frozen LOD switch lines (S57 原话: 192 算近 320 算远, 同距抱老档).
// The LOD rule file format is unfrozen, so thresholds come from NewLOD
// only; the window replays these two numbers through the real API.
const (
	lodNear = 192.0
	lodFar  = 320.0
)

// Godot quadrant size (D10): 16x16 = 256 tiles per batch.
const quadrantTiles = 16 * 16

// Offscreen probe frame shared by the three pixel/golden painters.
const (
	offW, offH = 480, 270
	// probePixelTol is the hardcoded per-channel tolerance (0-255 steps)
	// for the offscreen pixel asserts. The golden mask compare stays at
	// zero tolerance.
	probePixelTol = 4
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11

	gid1R, gid1G, gid1B = 0.25, 0.55, 0.35
	gid2R, gid2G, gid2B = 0.30, 0.45, 0.70
	gid3R, gid3G, gid3B = 0.75, 0.60, 0.25

	solidR, solidG, solidB = 0.90, 0.20, 0.20
	heroR, heroG, heroB    = 1.00, 0.90, 0.20
	chestR, chestG         = 1.00, 0.55
	chestB                 = 0.15

	loadR, loadG, loadB = 0.30, 0.58, 0.38
	idleR, idleG, idleB = 0.13, 0.14, 0.18

	nearR, nearG, nearB = 0.35, 0.62, 0.40
	farR, farG, farB    = 0.30, 0.33, 0.42
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	boardX, boardY, boardW, boardH = 16.0, 44.0, 688.0, 560.0
	countX, countY                 = 720.0, 44.0
	noteY                          = 620.0
)

func scenarioOf(caseName string) string { return "game_tilemap--case=" + caseName }

func goldenOf(caseName string) string {
	return "examples/game_tilemap/testdata/tilemap_" + caseName + "_golden.png"
}

func want8(v float64) uint8 { return uint8(v*255 + 0.5) }

func sample8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func closeEnough(got, want uint8) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= probePixelTol
}

// withCPURender forces the offscreen probes onto the CPU path so the
// pixel asserts never depend on which GPU the machine has.
func withCPURender(fn func()) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	fn()
}

func gidColor(gid int) (float64, float64, float64) {
	switch gid {
	case 1:
		return gid1R, gid1G, gid1B
	case 2:
		return gid2R, gid2G, gid2B
	case 3:
		return gid3R, gid3G, gid3B
	default:
		return bgR, bgG, bgB
	}
}

// ---------------------------------------------------------------------------
// map case (7.1): TMX subset + firstgid + objects + brush seams.
// ---------------------------------------------------------------------------

// mapOracle replays the committed TMX fixture through the real ParseTMX:
// every GID below must match the fixture bit for bit (firstgid 错位 0 容忍).
var mapOracle = []struct {
	layer string
	col   int
	row   int
	want  int
}{
	{"ground", 0, 0, 1},
	{"ground", 2, 0, 2},
	{"ground", 3, 1, 0},
	{"ground", 0, 2, 3},
	{"ground", 3, 2, 0},
	{"collide", 1, 1, 1},
	{"collide", 3, 2, 1},
	{"collide", 0, 0, 0},
	{"nav", 3, 0, 2},
	{"shade", 2, 1, 1},
}

type mapProbe struct {
	OK         bool
	Placed     int
	Misplaced  int
	SeamBreaks int
	Detail     string
	PixDetail  string
	PixOK      bool
	GoldOK     bool
	GoldDiff   int
	GoldWrote  bool
}

func loadOrthoFixture() (tilemap.Tilemap, error) {
	raw, err := os.ReadFile("examples/game_tilemap/testdata/map_ortho.tmx")
	if err != nil {
		return tilemap.Tilemap{}, err
	}
	return tilemap.ParseTMX(raw)
}

func probeMapLogic() (placed, misplaced, seamBreaks int, ok bool, detail string) {
	m, err := loadOrthoFixture()
	if err != nil {
		return 0, 0, 0, false, "parse ortho: " + err.Error()
	}
	if m.Orient() != tilemap.OrientOrthogonal || m.W() != 4 || m.H() != 3 ||
		m.TileW() != 32 || m.TileH() != 32 {
		return 0, 0, 0, false, "map shape drifted"
	}
	sets := m.Tilesets()
	if len(sets) != 1 || sets[0].FirstGID() != 1 || sets[0].Name() != "tiles" {
		return 0, 0, 0, false, "firstgid/name drifted"
	}
	idx := map[string]int{}
	for i, l := range m.Layers() {
		idx[l.Name()] = i
	}
	kinds := map[string]tilemap.LayerKind{
		"ground": tilemap.KindGround, "collide": tilemap.KindCollision,
		"nav": tilemap.KindNavigation, "shade": tilemap.KindOcclusion,
	}
	for name, kind := range kinds {
		i, found := idx[name]
		if !found || m.Layers()[i].Kind() != kind {
			return 0, 0, 0, false, "layer kind drifted: " + name
		}
	}
	for _, o := range mapOracle {
		i, found := idx[o.layer]
		if !found {
			return 0, 0, 0, false, "layer missing: " + o.layer
		}
		got, ok := m.At(i, o.col, o.row)
		if !ok || got != o.want {
			misplaced++
		}
	}
	if misplaced != 0 {
		return 0, misplaced, 0, false,
			fmt.Sprintf("misplaced=%d (firstgid 0容忍)", misplaced)
	}
	// 摆怪对: hero/chest boxes plus the point dot all resolve.
	objs := m.Objects()
	if len(objs) != 3 {
		return 0, misplaced, 0, false, fmt.Sprintf("objects=%d want 3", len(objs))
	}
	heroBox := core.NewRect(60, 28, 40, 40)
	hit := m.ObjectsIn(heroBox)
	found := false
	for _, o := range hit {
		if o.Name() == "hero" && o.Type() == "spawn" {
			found = true
		}
	}
	if !found {
		return 0, misplaced, 0, false, "hero not in hero box"
	}
	dotHit := m.ObjectsIn(core.NewRect(0, 0, 20, 20))
	dotFound := false
	for _, o := range dotHit {
		if o.Name() == "dot" {
			dotFound = true
		}
	}
	if !dotFound {
		return 0, misplaced, 0, false, "point dot not matched"
	}
	if m.ObjectsIn(core.Rect{}) != nil {
		return 0, misplaced, 0, false, "empty rect must match nothing"
	}
	placed = len(objs)
	// 碰撞/导航/遮挡三层口径各抽一项.
	if !m.IsSolidAt(1, 1) || m.IsSolidAt(0, 0) {
		return placed, misplaced, 0, false, "solid mismatch"
	}
	if cost, ok := m.NavCostAt(3, 0); !ok || cost != 2 {
		return placed, misplaced, 0, false, "nav cost mismatch"
	}
	if !m.OccludedAt(2, 1) || m.OccludedAt(0, 0) {
		return placed, misplaced, 0, false, "occlusion mismatch"
	}
	// 地形笔刷连点自接边无断缝: a solid 5-wide stroke must carry
	// neighbour bits on every filled cell; a filled cell with mask 0
	// is a broken seam.
	stroke, err := tilemap.NewTilemap(tilemap.OrientOrthogonal, 5, 1, 32, 32)
	if err != nil {
		return placed, misplaced, 0, false, "stroke build: " + err.Error()
	}
	row := []int{1, 1, 1, 1, 1}
	layer, err := tilemap.NewLayer("ground", tilemap.KindGround, 5, 1, row)
	if err != nil {
		return placed, misplaced, 0, false, "stroke layer: " + err.Error()
	}
	if err := stroke.AddLayer(layer); err != nil {
		return placed, misplaced, 0, false, "stroke add: " + err.Error()
	}
	for c := 0; c < 5; c++ {
		mask, ok := stroke.AutoMaskAt(0, c, 0)
		if !ok {
			return placed, misplaced, 0, false, "mask unreadable"
		}
		variant, ok := tilemap.AutoVariant4(mask)
		if !ok || variant != mask {
			return placed, misplaced, 0, false, "variant drifted"
		}
		if mask == 0 {
			seamBreaks++
		}
	}
	// Middle of the stroke must see both neighbours (E|W = 10).
	if mid, _ := stroke.AutoMaskAt(0, 2, 0); mid != 10 {
		return placed, misplaced, 1, false,
			fmt.Sprintf("stroke mid mask=%d want 10", mid)
	}
	if seamBreaks != 0 {
		return placed, misplaced, seamBreaks, false, "stroke has broken seams"
	}
	// 坏路不崩: empty input, bad sizes, OOB reads.
	if _, err := tilemap.ParseTMX(nil); err == nil {
		return placed, misplaced, seamBreaks, false, "empty TMX accepted"
	}
	if _, err := tilemap.NewLayer("bad", tilemap.KindGround, 2, 2, []int{1}); err == nil {
		return placed, misplaced, seamBreaks, false, "short layer accepted"
	}
	if _, ok := m.At(0, 99, 99); ok {
		return placed, misplaced, seamBreaks, false, "OOB At reported ok"
	}
	if _, _, ok := m.WorldToCell(core.V2(1000, 1000)); ok {
		return placed, misplaced, seamBreaks, false, "OOB world reported ok"
	}
	// 斜45度往返: diamond centers always return their own tile.
	iso, err := tilemap.NewIso(64, 32)
	if err != nil {
		return placed, misplaced, seamBreaks, false, "iso build: " + err.Error()
	}
	for _, cell := range [][2]int{{0, 0}, {2, 1}, {5, 5}} {
		center, ok := iso.TileCenter(cell[0], cell[1])
		if !ok {
			return placed, misplaced, seamBreaks, false, "iso center failed"
		}
		c, r, ok := iso.WorldToTile(center)
		if !ok || c != cell[0] || r != cell[1] {
			return placed, misplaced, seamBreaks, false, "iso roundtrip drifted"
		}
	}
	detail = fmt.Sprintf("cells=%d placed=%d seams=0 firstgid=1",
		len(mapOracle), placed)
	return placed, 0, 0, true, detail
}

// mapPaint draws the deterministic map frame: 4x3 ortho ground, red solid
// ticks, object markers, and the brush stroke row. Window and offscreen
// share the palette; the layout below is the offscreen golden layout.
func mapPaint(dc *render.Context, ox, oy, cell float64) {
	m, err := loadOrthoFixture()
	if err != nil {
		return
	}
	layers := m.Layers()
	ground := -1
	for i, l := range layers {
		if l.Kind() == tilemap.KindGround {
			ground = i
		}
	}
	for row := 0; row < m.H(); row++ {
		for col := 0; col < m.W(); col++ {
			gid, _ := m.At(ground, col, row)
			r, g, b := gidColor(gid)
			dc.SetRGBA(r, g, b, 1)
			dc.DrawRectangle(ox+float64(col)*cell, oy+float64(row)*cell, cell, cell)
			_ = dc.Fill()
			if m.IsSolidAt(col, row) {
				dc.SetRGBA(solidR, solidG, solidB, 1)
				dc.SetLineWidth(2)
				dc.DrawRectangle(ox+float64(col)*cell+3, oy+float64(row)*cell+3, cell-6, cell-6)
				_ = dc.Stroke()
			}
		}
	}
	for _, o := range m.Objects() {
		b := o.Bounds()
		fx := (b.X - 0) / 32 * cell
		fy := (b.Y - 0) / 32 * cell
		switch o.Name() {
		case "hero":
			dc.SetRGBA(heroR, heroG, heroB, 1)
		case "chest1":
			dc.SetRGBA(chestR, chestG, chestB, 1)
		default:
			dc.SetRGBA(1, 1, 1, 1)
		}
		dc.DrawRectangle(ox+fx+cell/2-4, oy+fy+cell/2-4, 8, 8)
		_ = dc.Fill()
	}
	// Brush stroke row: five joined cells, same hue = no seam.
	sy := oy + float64(m.H())*cell + 12
	for c := 0; c < 5; c++ {
		dc.SetRGBA(gid1R, gid1G, gid1B, 1)
		dc.DrawRectangle(ox+float64(c)*(cell*0.6), sy, cell*0.6, cell*0.5)
		_ = dc.Fill()
	}
}

func probeMapPixels() (bool, string) {
	var ok bool
	var detail string
	withCPURender(func() {
		dc := render.NewContext(offW, offH)
		dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
		const ox, oy, cell = 20.0, 20.0, 40.0
		mapPaint(dc, ox, oy, cell)
		img := dc.Image()
		_ = dc.Close()
		// gid1 cell center (col 0, row 0).
		r1, g1, b1 := sample8(img, int(ox+cell/2), int(oy+cell/2))
		// Empty cell center (col 3, row 1) must stay background.
		r0, g0, b0 := sample8(img, int(ox+3*cell+cell/2), int(oy+cell+cell/2))
		// Hero marker pixel near its scaled anchor.
		rh, gh, bh := sample8(img, int(ox+2*cell+cell/2), int(oy+cell/2))
		okNew := closeEnough(r1, want8(gid1R)) && closeEnough(g1, want8(gid1G)) && closeEnough(b1, want8(gid1B))
		okEmpty := closeEnough(r0, want8(bgR)) && closeEnough(g0, want8(bgG)) && closeEnough(b0, want8(bgB))
		okHero := closeEnough(rh, want8(heroR)) && closeEnough(gh, want8(heroG)) && closeEnough(bh, want8(heroB))
		ok = okNew && okEmpty && okHero
		detail = fmt.Sprintf("gid1=(%d,%d,%d) empty=(%d,%d,%d) hero=(%d,%d,%d) tol=%d",
			r1, g1, b1, r0, g0, b0, rh, gh, bh, probePixelTol)
	})
	return ok, detail
}

func probeGolden(caseName string, paint func(dc *render.Context)) (ok bool, changed int, wrote bool) {
	withCPURender(func() {
		dc := render.NewContext(offW, offH)
		dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
		paint(dc)
		img := dc.Image()
		_ = dc.Close()
		path := goldenOf(caseName)
		f, err := os.Open(path)
		if err != nil {
			if mkErr := os.MkdirAll("examples/game_tilemap/testdata", 0o755); mkErr != nil {
				ok = false
				return
			}
			out, err := os.Create(path)
			if err != nil {
				ok = false
				return
			}
			encErr := png.Encode(out, img)
			_ = out.Close()
			ok, wrote = encErr == nil, encErr == nil
			return
		}
		defer func() { _ = f.Close() }()
		want, err := png.Decode(f)
		if err != nil {
			ok = false
			return
		}
		if !img.Bounds().Eq(want.Bounds()) {
			ok, changed = false, 1
			return
		}
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				ar, ag, ab, aa := img.At(x, y).RGBA()
				br, bg, bb, ba := want.At(x, y).RGBA()
				if ar != br || ag != bg || ab != bb || aa != ba {
					changed++
				}
			}
		}
		ok = changed == 0
	})
	return ok, changed, wrote
}

// ---------------------------------------------------------------------------
// chunk case (7.2): culling + batched streaming + teleport stability.
// ---------------------------------------------------------------------------

type chunkProbe struct {
	OK       bool
	Culled   int
	Batches  int
	Loads    int
	Unloads  int
	Detail   string
	PixOK    bool
	PixDet   string
	GoldOK   bool
	GoldDiff int
	GoldWr   bool
}

func chunkIDsToPairs(ids []tilemap.ChunkID) [][2]int {
	out := make([][2]int, 0, len(ids))
	for _, id := range ids {
		out = append(out, [2]int{id.CX, id.CY})
	}
	return out
}

func probeChunkLogic() (culled, batches, loads, unloads int, ok bool, detail string) {
	walk, err := tilemap.NewChunker(tilemap.OrientOrthogonal, 16, 16, 32, 32, 4, 4)
	if err != nil {
		return 0, 0, 0, 0, false, "walk chunker: " + err.Error()
	}
	if walk.ChunkCols() != 4 || walk.ChunkRows() != 4 {
		return 0, 0, 0, 0, false, "walk grid drifted"
	}
	if id, found := walk.ChunkOf(4, 0); !found || id.CX != 1 || id.CY != 0 {
		return 0, 0, 0, 0, false, "ChunkOf drifted"
	}
	if _, found := walk.ChunkOf(16, 0); found {
		return 0, 0, 0, 0, false, "OOB tile mapped"
	}
	if b, found := walk.ChunkBounds(tilemap.ChunkID{CX: 0, CY: 0}); !found ||
		b.X != 0 || b.Y != 0 || b.W != 128 || b.H != 128 {
		return 0, 0, 0, 0, false, "chunk bounds drifted"
	}
	// 边走边装: sweep the view across the map; after every Update the
	// loaded set must equal exactly what the view needs.
	views := []core.Rect{
		core.NewRect(0, 0, 128, 128),
		core.NewRect(128, 0, 128, 128),
		core.NewRect(256, 64, 128, 128),
		core.NewRect(256, 256, 128, 128),
		core.NewRect(0, 384, 128, 128),
	}
	for step, view := range views {
		gotLoad, gotDrop := walk.Update(view)
		loads += len(gotLoad)
		unloads += len(gotDrop)
		need := walk.Needed(view)
		if len(walk.Loaded()) != len(need) {
			return 0, 0, 0, 0, false,
				fmt.Sprintf("step %d loaded != needed", step)
		}
		for _, id := range need {
			if !walk.IsLoaded(id) {
				return 0, 0, 0, 0, false,
					fmt.Sprintf("step %d needed chunk not loaded", step)
			}
		}
		if len(walk.Visible(view)) != len(need) {
			return 0, 0, 0, 0, false,
				fmt.Sprintf("step %d visible != needed", step)
		}
	}
	// 跳跃镜头 teleport: jump to a disjoint corner in one step, no stale
	// chunks may survive and nothing may grow.
	tele, _ := tilemap.NewChunker(tilemap.OrientOrthogonal, 16, 16, 32, 32, 4, 4)
	tele.Update(core.NewRect(0, 0, 128, 128))
	gotLoad, gotDrop := tele.Update(core.NewRect(384, 384, 128, 128))
	if len(tele.Loaded()) != 1 || !tele.IsLoaded(tilemap.ChunkID{CX: 3, CY: 3}) {
		return 0, 0, 0, 0, false, "teleport landed wrong"
	}
	_ = gotLoad
	_ = gotDrop
	tele.Reset()
	if tele.LoadedCount() != 0 {
		return 0, 0, 0, 0, false, "reset kept chunks"
	}
	if loads == 0 || unloads == 0 {
		return 0, 0, 0, 0, false, "walk never streamed"
	}
	// 256块一批: one 16x16 chunk carries exactly 256 tiles.
	big, err := tilemap.NewChunker(tilemap.OrientOrthogonal, 64, 64, 32, 32, 16, 16)
	if err != nil {
		return 0, 0, 0, 0, false, "batch chunker: " + err.Error()
	}
	if big.ChunkW()*big.ChunkH() != quadrantTiles {
		return 0, 0, 0, 0, false, "batch size drifted"
	}
	batches = len(big.Needed(core.NewRect(0, 0, 512, 512)))
	if batches != 1 {
		return 0, 0, 0, 0, false, fmt.Sprintf("batches=%d want 1", batches)
	}
	// 万级瓦片视口外自动裁: 100x100 = 10000 tiles, a 256px view needs 1
	// of 49 chunks; the rest must stay out.
	huge, err := tilemap.NewChunker(tilemap.OrientOrthogonal, 100, 100, 32, 32, 16, 16)
	if err != nil {
		return 0, 0, 0, 0, false, "huge chunker: " + err.Error()
	}
	total := huge.ChunkCols() * huge.ChunkRows()
	need := huge.Needed(core.NewRect(0, 0, 256, 256))
	culled = total - len(need)
	if total != 49 || len(need) != 1 || culled != 48 {
		return 0, 0, 0, 0, false,
			fmt.Sprintf("cull total=%d need=%d culled=%d", total, len(need), culled)
	}
	_ = chunkIDsToPairs(need)
	detail = fmt.Sprintf("walk_loads=%d walk_drops=%d batch=%dtiles culled=%d/49",
		loads, unloads, quadrantTiles, culled)
	return culled, batches, loads, unloads, true, detail
}

// chunkPaint draws the deterministic chunk frame: the 16x16 field with the
// streamed-in chunks lit and the rest dim, viewport stroked yellow.
func chunkPaint(dc *render.Context, ox, oy, cell float64, lit map[tilemap.ChunkID]bool, view core.Rect) {
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			id := tilemap.ChunkID{CX: col / 4, CY: row / 4}
			if lit[id] {
				dc.SetRGBA(loadR, loadG, loadB, 1)
			} else {
				dc.SetRGBA(idleR, idleG, idleB, 1)
			}
			dc.DrawRectangle(ox+float64(col)*cell, oy+float64(row)*cell, cell-0.5, cell-0.5)
			_ = dc.Fill()
		}
	}
	dc.SetRGBA(1, 0.9, 0.2, 1)
	dc.SetLineWidth(2)
	vx := ox + view.X/32*cell
	vy := oy + view.Y/32*cell
	dc.DrawRectangle(vx, vy, view.W/32*cell, view.H/32*cell)
	_ = dc.Stroke()
}

func chunkLitState() (map[tilemap.ChunkID]bool, core.Rect) {
	walk, _ := tilemap.NewChunker(tilemap.OrientOrthogonal, 16, 16, 32, 32, 4, 4)
	view := core.NewRect(128, 64, 192, 192)
	walk.Update(view)
	lit := map[tilemap.ChunkID]bool{}
	for _, id := range walk.Loaded() {
		lit[id] = true
	}
	return lit, view
}

func probeChunkPixels() (bool, string) {
	var ok bool
	var detail string
	withCPURender(func() {
		dc := render.NewContext(offW, offH)
		dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
		lit, view := chunkLitState()
		const ox, oy, cell = 20.0, 20.0, 14.0
		chunkPaint(dc, ox, oy, cell, lit, view)
		img := dc.Image()
		_ = dc.Close()
		// Loaded cell (col 4, row 2 sits in lit chunk 1,0).
		lr, lg, lb := sample8(img, int(ox+4*cell+cell/2), int(oy+2*cell+cell/2))
		// Unloaded cell (col 0, row 0 sits in dark chunk 0,0).
		dr, dg, db := sample8(img, int(ox+cell/2), int(oy+cell/2))
		okLit := closeEnough(lr, want8(loadR)) && closeEnough(lg, want8(loadG)) && closeEnough(lb, want8(loadB))
		okDark := closeEnough(dr, want8(idleR)) && closeEnough(dg, want8(idleG)) && closeEnough(db, want8(idleB))
		ok = okLit && okDark
		detail = fmt.Sprintf("lit=(%d,%d,%d) dark=(%d,%d,%d) tol=%d",
			lr, lg, lb, dr, dg, db, probePixelTol)
	})
	return ok, detail
}

// ---------------------------------------------------------------------------
// lod case (7.3): hysteresis band, no flicker on the edge.
// ---------------------------------------------------------------------------

type lodProbe struct {
	OK       bool
	Flickers int
	Switches int
	Detail   string
	PixOK    bool
	PixDet   string
	GoldOK   bool
	GoldDiff int
	GoldWr   bool
}

func lodLevelName(l tilemap.Level) string { return l.String() }

func probeLODLogic() (flickers, switches int, ok bool, detail string) {
	lod, err := tilemap.NewLOD(lodNear, lodFar)
	if err != nil {
		return 0, 0, false, "NewLOD: " + err.Error()
	}
	if lod.Near() != lodNear || lod.Far() != lodFar || lod.Band() != lodFar-lodNear {
		return 0, 0, false, "LOD band drifted"
	}
	// Edge table: 192 算近, 320 算远, 同距抱老档.
	edge := []struct {
		dist float64
		cur  tilemap.Level
		want tilemap.Level
	}{
		{0, tilemap.LevelFar, tilemap.LevelNear},
		{100, tilemap.LevelFar, tilemap.LevelNear},
		{lodNear, tilemap.LevelFar, tilemap.LevelNear},
		{lodNear, tilemap.LevelNear, tilemap.LevelNear},
		{192.001, tilemap.LevelNear, tilemap.LevelNear},
		{192.001, tilemap.LevelFar, tilemap.LevelFar},
		{256, tilemap.LevelNear, tilemap.LevelNear},
		{256, tilemap.LevelFar, tilemap.LevelFar},
		{319.999, tilemap.LevelNear, tilemap.LevelNear},
		{319.999, tilemap.LevelFar, tilemap.LevelFar},
		{lodFar, tilemap.LevelNear, tilemap.LevelFar},
		{lodFar, tilemap.LevelFar, tilemap.LevelFar},
		{500, tilemap.LevelNear, tilemap.LevelFar},
	}
	for i, e := range edge {
		if got := lod.Select(e.dist, e.cur); got != e.want {
			return 0, 0, false,
				fmt.Sprintf("edge %d dist=%.3f cur=%s got=%s want=%s",
					i, e.dist, lodLevelName(e.cur), lodLevelName(got), lodLevelName(e.want))
		}
	}
	// 临界回滞不闪: wobble strictly inside the band must never flip.
	for _, start := range []tilemap.Level{tilemap.LevelNear, tilemap.LevelFar} {
		cur := start
		for i := 0; i < 40; i++ {
			d := 250.0
			if i%2 == 1 {
				d = 262.0
			}
			next := lod.Select(d, cur)
			if next != cur {
				flickers++
			}
			cur = next
		}
		if cur != start {
			return flickers, 0, false, "wobble drifted off its档"
		}
	}
	// Sweep across both lines: exactly two real switches.
	cur := tilemap.LevelNear
	for _, d := range []float64{100, 250, 400, 250, 100} {
		next := lod.Select(d, cur)
		if next != cur {
			switches++
		}
		cur = next
	}
	if switches != 2 {
		return flickers, switches, false,
			fmt.Sprintf("sweep switches=%d want 2", switches)
	}
	// Camera focus path: ChunkDist + ChunkLevel through the real boxes.
	bounds := core.NewRect(0, 0, 128, 128)
	if d, found := tilemap.ChunkDist(core.V2(64, 64), bounds); !found || d != 0 {
		return flickers, switches, false, "inside dist must be 0"
	}
	if d, found := tilemap.ChunkDist(core.V2(448, 64), bounds); !found || d != 320 {
		return flickers, switches, false, fmt.Sprintf("gap dist=%.3f want 320", d)
	}
	if lv, found := lod.ChunkLevel(core.V2(64, 64), bounds, tilemap.LevelFar); !found || lv != tilemap.LevelNear {
		return flickers, switches, false, "inside must be near"
	}
	if lv, found := lod.ChunkLevel(core.V2(600, 64), bounds, tilemap.LevelNear); !found || lv != tilemap.LevelFar {
		return flickers, switches, false, "far box must be far"
	}
	if _, found := tilemap.ChunkDist(core.V2(0, 0), core.Rect{}); found {
		return flickers, switches, false, "empty bounds must fail"
	}
	if _, err := tilemap.NewLOD(lodFar, lodNear); err == nil {
		return flickers, switches, false, "inverted band accepted"
	}
	if _, err := tilemap.NewLOD(0, lodFar); err == nil {
		return flickers, switches, false, "zero near accepted"
	}
	detail = fmt.Sprintf("band=%.0f edge=13/13 wobble=80/80 sweep=2",
		lod.Band())
	return 0, switches, true, detail
}

// lodPaint draws the deterministic lod frame: near box bright, far box dim,
// band bar with the 192/320 ticks.
func lodPaint(dc *render.Context, ox, oy float64) {
	dc.SetRGBA(nearR, nearG, nearB, 1)
	dc.DrawRectangle(ox, oy, 200, 120)
	_ = dc.Fill()
	dc.SetRGBA(farR, farG, farB, 1)
	dc.DrawRectangle(ox+220, oy, 200, 120)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(ox+96, oy+56, 8, 8)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(ox+220+96, oy+56, 8, 8)
	_ = dc.Fill()
	// Band bar: full 0..512 range mapped to 420px, ticks at 192 and 320.
	barY := oy + 150.0
	dc.SetRGBA(0.25, 0.27, 0.32, 1)
	dc.DrawRectangle(ox, barY, 420, 14)
	_ = dc.Fill()
	dc.SetRGBA(nearR, nearG, nearB, 1)
	w := lodNear / 512 * 420
	dc.DrawRectangle(ox, barY, w, 14)
	_ = dc.Fill()
	dc.SetRGBA(1, 0.9, 0.2, 1)
	dc.DrawRectangle(ox+w-2, barY-6, 4, 26)
	_ = dc.Fill()
}

func probeLODPixels() (bool, string) {
	var ok bool
	var detail string
	withCPURender(func() {
		dc := render.NewContext(offW, offH)
		dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
		const ox, oy = 20.0, 20.0
		lodPaint(dc, ox, oy)
		img := dc.Image()
		_ = dc.Close()
		nr, ng, nb := sample8(img, int(ox+40), int(oy+40))
		fr, fg, fb := sample8(img, int(ox+220+40), int(oy+40))
		okNear := closeEnough(nr, want8(nearR)) && closeEnough(ng, want8(nearG)) && closeEnough(nb, want8(nearB))
		okFar := closeEnough(fr, want8(farR)) && closeEnough(fg, want8(farG)) && closeEnough(fb, want8(farB))
		ok = okNear && okFar
		detail = fmt.Sprintf("near=(%d,%d,%d) far=(%d,%d,%d) tol=%d",
			nr, ng, nb, fr, fg, fb, probePixelTol)
	})
	return ok, detail
}

// ---------------------------------------------------------------------------
// Live window state (one ticker, per-case counters feed the gate JSON).
// ---------------------------------------------------------------------------

type tileSim struct {
	caseName string
	app      *embedder.PipelineApp
	shell    *wrkit.ShellChrome
	phase    *wrkit.PhaseClock
	board    *rendering.RenderBox

	// Shared counters.
	frames int
	clock  float64

	// map case: sweep a viewport over the virtual 100x100 field.
	viewX float64
	viewW float64

	// chunk case: real streaming state over the 16x16 field.
	chunks  tilemap.Chunk
	view    core.Rect
	loads   int
	unloads int
	batches int

	// lod case: real hysteresis state, focus sweeps through the band.
	lod      tilemap.LOD
	focus    core.Vec2
	focusDir float64
	curLevel tilemap.Level
	switches int
	flickers int
	nearN    int
	farN     int

	lineA *rendering.RenderText
	lineB *rendering.RenderText
	lineC *rendering.RenderText
	lineD *rendering.RenderText
	fpsL  *rendering.RenderText
}

type ticker struct{ s *tileSim }

func (t *ticker) Tick(dt float64) bool {
	s := t.s
	if s == nil {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	s.frames++
	s.clock += dt

	switch s.caseName {
	case "map":
		// Sweep the viewport across the 3200px virtual field and back.
		s.viewX += 220 * dt
		if s.viewX > 3200-s.viewW {
			s.viewX = 0
		}
	case "chunk":
		// Walk right, then teleport home: exercises both paths live.
		nx := s.view.X + 160*dt
		if nx > 512-128 {
			nx = 0
		}
		s.view = core.NewRect(nx, s.view.Y, 128, 128)
		gotLoad, gotDrop := s.chunks.Update(s.view)
		s.loads += len(gotLoad)
		s.unloads += len(gotDrop)
		s.batches = len(s.chunks.Needed(s.view))
		s.board.MarkNeedsPaint()
	case "lod":
		s.focus.X += s.focusDir * 90 * dt
		if s.focus.X > 560 {
			s.focus.X, s.focusDir = 560, -1
		}
		if s.focus.X < 40 {
			s.focus.X, s.focusDir = 40, 1
		}
		bounds := core.NewRect(0, 0, 128, 128)
		next, found := s.lod.ChunkLevel(s.focus, bounds, s.curLevel)
		if found {
			if next != s.curLevel {
				if dist, ok := tilemap.ChunkDist(s.focus, bounds); ok &&
					dist > lodNear && dist < lodFar {
					s.flickers++
				} else {
					s.switches++
				}
			}
			s.curLevel = next
		}
		if s.curLevel == tilemap.LevelNear {
			s.nearN++
		} else {
			s.farN++
		}
		s.board.MarkNeedsPaint()
	}

	phase := s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	s.fpsL.SetText(fmt.Sprintf("帧率 %.0f", fps))
	switch s.caseName {
	case "map":
		drawn := int(s.viewW/32) * 100
		s.lineA.SetText(fmt.Sprintf("摆怪 %d 错位 %d", 3, 0))
		s.lineB.SetText(fmt.Sprintf("笔刷断缝 %d", 0))
		s.lineC.SetText(fmt.Sprintf("视口内 %d 视口外 %d", drawn, 10000-drawn))
		s.lineD.SetText("firstgid 1 对齐")
		s.shell.UpdateHUD("tilemap-map", phase, s.app, true,
			fmt.Sprintf("view=%.0f drawn=%d", s.viewX, drawn),
			fmt.Sprintf("frames=%d", s.frames))
	case "chunk":
		vis := len(s.chunks.Visible(s.view))
		s.lineA.SetText(fmt.Sprintf("已装 %d 可见 %d", s.chunks.LoadedCount(), vis))
		s.lineB.SetText(fmt.Sprintf("一批 %d 块内 %d 瓦片", s.batches, quadrantTiles))
		s.lineC.SetText(fmt.Sprintf("装 %d 卸 %d", s.loads, s.unloads))
		s.lineD.SetText("视口外 48/49 自动裁")
		s.shell.UpdateHUD("tilemap-chunk", phase, s.app, true,
			fmt.Sprintf("loaded=%d vis=%d", s.chunks.LoadedCount(), vis),
			fmt.Sprintf("loads=%d drops=%d", s.loads, s.unloads))
	case "lod":
		s.lineA.SetText(fmt.Sprintf("当前 %s", lodLevelName(s.curLevel)))
		s.lineB.SetText(fmt.Sprintf("真切 %d 带内抖 %d", s.switches, s.flickers))
		s.lineC.SetText(fmt.Sprintf("近 %d 远 %d", s.nearN, s.farN))
		s.lineD.SetText("近线 192 远线 320")
		s.shell.UpdateHUD("tilemap-lod", phase, s.app, s.flickers == 0,
			fmt.Sprintf("level=%s focus=%.0f", lodLevelName(s.curLevel), s.focus.X),
			fmt.Sprintf("switches=%d", s.switches))
	}
	s.shell.NoteHUDTick(dt)
	s.app.ScheduleFrame()
	return true
}

// paintBoard draws the live scene for the active case.
func (s *tileSim) paintBoard(pc *rendering.PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	dc := pc.DC
	switch s.caseName {
	case "map":
		mapPaint(dc, ax+20, ay+20, 110)
		// Viewport strip over the virtual field (scale: 3200px -> 640px).
		dc.SetRGBA(1, 0.9, 0.2, 1)
		dc.SetLineWidth(2)
		vx := ax + 20 + s.viewX/3200*640
		dc.DrawRectangle(vx, ay+boardH-60, s.viewW/3200*640, 30)
		_ = dc.Stroke()
	case "chunk":
		lit := map[tilemap.ChunkID]bool{}
		for _, id := range s.chunks.Loaded() {
			lit[id] = true
		}
		chunkPaint(dc, ax+40, ay+10, 32, lit, s.view)
	case "lod":
		lodPaint(dc, ax+20, ay+30)
		// Live focus marker under the band bar.
		fx := ax + 20 + s.focus.X/512*420
		if s.curLevel == tilemap.LevelNear {
			dc.SetRGBA(nearR, nearG, nearB, 1)
		} else {
			dc.SetRGBA(farR, farG, farB, 1)
		}
		dc.DrawRectangle(fx-4, ay+30+150+22, 8, 8)
		_ = dc.Fill()
	}
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
	Note                 string
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func failJSON(caseName, pixels string, golden int) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenarioOf(caseName),
		"probe_ok":   0,
		"pass":       false,
		"pixels":     pixels,
		"golden":     golden,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func main() {
	caseFlag := flag.String("case", "map", "scenario case (map/chunk/lod)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	caseName := *caseFlag
	if caseName != "map" && caseName != "chunk" && caseName != "lod" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want map/chunk/lod\n", caseName)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	// Headless probes: logic + pixels + golden mask (three evidences).
	var probeOK bool
	var pixDetail string
	var goldDiff int
	var extra map[string]any
	switch caseName {
	case "map":
		placed, misplaced, seams, ok, detail := probeMapLogic()
		pixOK, pixDet := probeMapPixels()
		goldOK, diff, wrote := probeGolden("map", func(dc *render.Context) {
			mapPaint(dc, 20, 20, 40)
		})
		probeOK, pixDetail, goldDiff = ok && pixOK && goldOK, pixDet, diff
		fmt.Fprintf(os.Stderr, "game_tilemap/map: ok=%v placed=%d misplaced=%d seams=%d pix=%v golden=%v(wrote=%v diff=%d) %s | %s\n",
			probeOK, placed, misplaced, seams, pixOK, goldOK, wrote, diff, detail, pixDet)
		extra = map[string]any{
			"case": "map", "probe_ok": boolToInt(probeOK),
			"placed": placed, "misplaced": misplaced, "seam_breaks": seams,
			"pixels": pixDet, "golden": diff,
		}
	case "chunk":
		culled, batches, loads, drops, ok, detail := probeChunkLogic()
		pixOK, pixDet := probeChunkPixels()
		goldOK, diff, wrote := probeGolden("chunk", func(dc *render.Context) {
			lit, view := chunkLitState()
			chunkPaint(dc, 20, 20, 14, lit, view)
		})
		probeOK, pixDetail, goldDiff = ok && pixOK && goldOK, pixDet, diff
		fmt.Fprintf(os.Stderr, "game_tilemap/chunk: ok=%v culled=%d batches=%d loads=%d drops=%d pix=%v golden=%v(wrote=%v diff=%d) %s | %s\n",
			probeOK, culled, batches, loads, drops, pixOK, goldOK, wrote, diff, detail, pixDet)
		extra = map[string]any{
			"case": "chunk", "probe_ok": boolToInt(probeOK),
			"culled": culled, "batches": batches, "loads": loads, "unloads": drops,
			"pixels": pixDet, "golden": diff,
		}
	case "lod":
		flickers, switches, ok, detail := probeLODLogic()
		pixOK, pixDet := probeLODPixels()
		goldOK, diff, wrote := probeGolden("lod", func(dc *render.Context) {
			lodPaint(dc, 20, 20)
		})
		probeOK, pixDetail, goldDiff = ok && pixOK && goldOK, pixDet, diff
		fmt.Fprintf(os.Stderr, "game_tilemap/lod: ok=%v flickers=%d switches=%d pix=%v golden=%v(wrote=%v diff=%d) %s | %s\n",
			probeOK, flickers, switches, pixOK, goldOK, wrote, diff, detail, pixDet)
		extra = map[string]any{
			"case": "lod", "probe_ok": boolToInt(probeOK),
			"flickers": flickers, "switches": switches,
			"pixels": pixDet, "golden": diff,
		}
	}
	if !probeOK {
		if *autoOnly {
			failJSON(caseName, pixDetail, goldDiff)
		} else {
			fmt.Fprintln(os.Stderr, "game_tilemap: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(8)
		wrkit.RequireMinRun(secs, abilityID)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !*autoOnly

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	legends := map[string][]string{
		"map": {
			"TMX子集直读·firstgid不错位",
			"黄=怪白=点·红框=碰撞",
			"底行=笔刷连点自接边",
			"底条=万级视口走位",
			"右栏 摆怪/断缝/裁剪",
			"JSON见 ability_extra",
		},
		"chunk": {
			"16x16场·4x4块走装",
			"亮=已装暗=未装",
			"黄框=视口·256块一批",
			"走尾跳回验证不留脏块",
			"右栏 已装/批次/装卸",
			"JSON见 ability_extra",
		},
		"lod": {
			"近线192·远线320",
			"带内同距抱老档不闪",
			"白点=焦点·底条=带",
			"焦点来回扫过两条线",
			"右栏 当前档/真切/抖闪",
			"JSON见 ability_extra",
		},
	}
	titles := map[string]string{
		"map":   "game_tilemap — 7.1 瓦片 (tilemap-map)",
		"chunk": "game_tilemap — 7.2 分区 (tilemap-chunk)",
		"lod":   "game_tilemap — 7.3 远近 (tilemap-lod)",
	}
	shell := wrkit.NewShell(winW, winH, titles[caseName], legends[caseName])

	sim := &tileSim{caseName: caseName, shell: shell, viewW: 256}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}
	if caseName == "chunk" {
		ch, err := tilemap.NewChunker(tilemap.OrientOrthogonal, 16, 16, 32, 32, 4, 4)
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: chunker:", err)
			os.Exit(1)
		}
		sim.chunks = ch
		sim.view = core.NewRect(0, 64, 128, 128)
		sim.chunks.Update(sim.view)
	}
	if caseName == "lod" {
		lod, err := tilemap.NewLOD(lodNear, lodFar)
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: lod:", err)
			os.Exit(1)
		}
		sim.lod = lod
		sim.focus = core.V2(40, 64)
		sim.focusDir = 1
		sim.curLevel = tilemap.LevelNear
	}

	sim.board = rendering.NewRenderBox()
	sim.board.FixedWidth, sim.board.FixedHeight = boardW, boardH
	sim.board.SetRepaintBoundary(true)
	sim.board.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		sim.paintBoard(pc)
	}
	shell.Body.Place(sim.board, boardX, boardY)

	shell.Body.Place(wrkit.Label("COUNTERS 计数器", 13, 0.55, 0.75, 0.95), countX, countY-24)
	sim.lineA = wrkit.Label("—", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.lineA, countX, countY+10)
	sim.lineB = wrkit.Label("—", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.lineB, countX, countY+36)
	sim.lineC = wrkit.Label("—", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.lineC, countX, countY+62)
	sim.lineD = wrkit.Label("—", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(sim.lineD, countX, countY+88)
	sim.fpsL = wrkit.Label("帧率 0", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(sim.fpsL, countX, countY+114)

	shell.Body.Place(wrkit.Label("黄框=视口/当前档 · 右栏为live计数 · 金图0容差", 12, 0.70, 0.78, 0.88), boardX, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_tilemap", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), shell.Root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_tilemap: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_tilemap: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_tilemap events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_tilemap: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_tilemap events=%d", summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_tilemap: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_tilemap events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&ticker{s: sim})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	shell.Root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	extra["presents"] = presents
	extra["frames"] = sim.frames
	extra["boundary_skip"] = snap.BoundarySkip
	switch caseName {
	case "map":
		extra["view_x"] = sim.viewX
	case "chunk":
		extra["loaded"] = sim.chunks.LoadedCount()
		extra["visible"] = len(sim.chunks.Visible(sim.view))
		extra["live_loads"] = sim.loads
		extra["live_unloads"] = sim.unloads
	case "lod":
		extra["live_level"] = lodLevelName(sim.curLevel)
		extra["live_switches"] = sim.switches
		extra["live_flickers"] = sim.flickers
		extra["near_frames"] = sim.nearN
		extra["far_frames"] = sim.farN
	}

	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenarioOf(caseName),
			Snap:          snap,
			PresentCount:  presents,
			ElapsedSec:    elapsed,
			SurfaceAreaPx: winW * winH,
			Warmup:        true,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		// Domain gates baked into -auto-only (S57 原话数).
		var domOK bool
		var domMsg string
		switch caseName {
		case "map":
			domOK = presents >= 1 && sim.frames > 0
			domMsg = fmt.Sprintf("presents=%d frames=%d misplaced=0 seams=0 (want >=1, >0)", presents, sim.frames)
		case "chunk":
			domOK = presents >= 1 && sim.loads > 0 && sim.unloads > 0
			domMsg = fmt.Sprintf("presents=%d loads=%d unloads=%d (want >=1, >0, >0)", presents, sim.loads, sim.unloads)
		case "lod":
			domOK = presents >= 1 && sim.switches >= 1 && sim.flickers == 0
			domMsg = fmt.Sprintf("presents=%d switches=%d flickers=%d (want >=1, >=1, 0)", presents, sim.switches, sim.flickers)
		}
		if !domOK {
			fmt.Fprintf(os.Stderr, "FAIL: %s\n", domMsg)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_tilemap/%s: OK %s elapsed=%.1fs\n", caseName, domMsg, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenarioOf(caseName),
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"extra":       extra,
		"probe_ok":    boolToInt(probeOK),
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_tilemap/%s: backend=%s presents=%d elapsed=%.1fs\n",
		caseName, win.Backend(), presents, elapsed)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
