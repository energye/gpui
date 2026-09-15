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

const epsTilemap = 1e-9

type atCase struct {
	Layer string `json:"layer"`
	Col   int    `json:"col"`
	Row   int    `json:"row"`
	Want  int    `json:"want"`
}

type cellCase struct {
	Col   int        `json:"col"`
	Row   int        `json:"row"`
	World [2]float64 `json:"world"`
}

type worldCase struct {
	World [2]float64 `json:"world"`
	Col   int        `json:"col"`
	Row   int        `json:"row"`
}

type solidCase struct {
	Col  int  `json:"col"`
	Row  int  `json:"row"`
	Want bool `json:"want"`
}

type navCase struct {
	Col  int  `json:"col"`
	Row  int  `json:"row"`
	Cost int  `json:"cost"`
	OK   bool `json:"ok"`
}

type occlCase struct {
	Col  int  `json:"col"`
	Row  int  `json:"row"`
	Want bool `json:"want"`
}

type maskCase struct {
	Col  int `json:"col"`
	Row  int `json:"row"`
	Mask int `json:"mask"`
}

type variantCase struct {
	Mask    int  `json:"mask"`
	Variant int  `json:"variant"`
	OK      bool `json:"ok"`
}

type objCase struct {
	ID     int        `json:"id"`
	Name   string     `json:"name"`
	Type   string     `json:"type"`
	Bounds [4]float64 `json:"bounds"`
}

type objInCase struct {
	Rect [4]float64 `json:"rect"`
	Want []string   `json:"want"`
}

type orthoCases struct {
	TMX        string         `json:"tmx"`
	Orient     string         `json:"orient"`
	MapW       int            `json:"map_w"`
	MapH       int            `json:"map_h"`
	TileW      float64        `json:"tile_w"`
	TileH      float64        `json:"tile_h"`
	Layers     []string       `json:"layers"`
	Kinds      map[string]int `json:"kinds"`
	At         []atCase       `json:"at"`
	Cells      []cellCase     `json:"cells"`
	Worlds     []worldCase    `json:"worlds"`
	OOBWorlds  [][2]float64   `json:"oob_worlds"`
	Solids     []solidCase    `json:"solids"`
	Navs       []navCase      `json:"navs"`
	Occlusions []occlCase     `json:"occlusions"`
	Masks      []maskCase     `json:"masks"`
	Variants   []variantCase  `json:"variants"`
	Objects    []objCase      `json:"objects"`
	ObjectsIn  []objInCase    `json:"objects_in"`
}

type isoTileCase struct {
	Col    int        `json:"col"`
	Row    int        `json:"row"`
	Origin [2]float64 `json:"origin"`
	Center [2]float64 `json:"center"`
}

type isoContainsCase struct {
	Col   int        `json:"col"`
	Row   int        `json:"row"`
	World [2]float64 `json:"world"`
	Want  bool       `json:"want"`
}

type isoCases struct {
	TMX      string            `json:"tmx"`
	TileW    float64           `json:"tile_w"`
	TileH    float64           `json:"tile_h"`
	Tiles    []isoTileCase     `json:"tiles"`
	Worlds   []worldCase       `json:"worlds"`
	Contains []isoContainsCase `json:"contains"`
}

type tilemapFile struct {
	Ortho orthoCases `json:"ortho"`
	Iso   isoCases   `json:"iso"`
}

func loadTilemapCases(t *testing.T) tilemapFile {
	t.Helper()
	raw := mustReadTestdata(t, "tilemap_cases.json")
	var f tilemapFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode tilemap_cases.json: %v", err)
	}
	if len(f.Ortho.At) == 0 || len(f.Iso.Tiles) == 0 {
		t.Fatal("tilemap_cases.json has no cases")
	}
	return f
}

func mustReadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}

func loadOrthoMap(t *testing.T) (Tilemap, tilemapFile) {
	t.Helper()
	cases := loadTilemapCases(t)
	raw := mustReadTestdata(t, cases.Ortho.TMX)
	m, err := ParseTMX(raw)
	if err != nil {
		t.Fatalf("ParseTMX ortho: %v", err)
	}
	return m, cases
}

func loadIsoMap(t *testing.T) (Tilemap, tilemapFile) {
	t.Helper()
	cases := loadTilemapCases(t)
	raw := mustReadTestdata(t, cases.Iso.TMX)
	m, err := ParseTMX(raw)
	if err != nil {
		t.Fatalf("ParseTMX iso: %v", err)
	}
	return m, cases
}

func closeVec(got core.Vec2, want [2]float64) bool {
	return math.Abs(got.X-want[0]) < epsTilemap && math.Abs(got.Y-want[1]) < epsTilemap
}

func objNames(objs []Object) []string {
	names := make([]string, len(objs))
	for i, o := range objs {
		names[i] = o.Name()
	}
	return names
}

func equalStr(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	m := map[string]int{}
	for _, s := range got {
		m[s]++
	}
	for _, s := range want {
		m[s]--
		if m[s] < 0 {
			return false
		}
	}
	return true
}

// A:格子号对象摆位碰撞导航自动拼斜45度全落在冻结数上.
func TestTilemapFromCases(t *testing.T) {
	m, cases := loadOrthoMap(t)
	o := cases.Ortho
	if m.Orient() != OrientOrthogonal || m.W() != o.MapW || m.H() != o.MapH ||
		m.TileW() != o.TileW || m.TileH() != o.TileH {
		t.Fatalf("ortho header = %v %dx%d %.0fx%.0f, want orthogonal %dx%d %.0fx%.0f",
			m.Orient(), m.W(), m.H(), m.TileW(), m.TileH(), o.MapW, o.MapH, o.TileW, o.TileH)
	}
	if len(m.Layers()) != len(o.Layers) {
		t.Fatalf("layers = %d, want %d", len(m.Layers()), len(o.Layers))
	}
	for _, n := range o.Layers {
		idx, ok := m.LayerIndex(n)
		if !ok {
			t.Errorf("missing layer %q", n)
			continue
		}
		if int(m.Layers()[idx].Kind()) != o.Kinds[n] {
			t.Errorf("layer %q kind = %v, want %v", n, m.Layers()[idx].Kind(), o.Kinds[n])
		}
	}
	for i, k := range o.At {
		idx, ok := m.LayerIndex(k.Layer)
		if !ok {
			t.Fatalf("at[%d] unknown layer %q", i, k.Layer)
		}
		got, ok := m.At(idx, k.Col, k.Row)
		if !ok || got != k.Want {
			t.Errorf("at[%d] %s(%d,%d) = %v/%v, want %v/true", i, k.Layer, k.Col, k.Row, got, ok, k.Want)
		}
	}
	for i, k := range o.Cells {
		got, ok := m.CellToWorld(k.Col, k.Row)
		if !ok || !closeVec(got, k.World) {
			t.Errorf("cell[%d] (%d,%d) = %v/%v, want %v/true", i, k.Col, k.Row, got, ok, k.World)
		}
		if b, ok := m.CellBounds(k.Col, k.Row); !ok ||
			math.Abs(b.X-k.World[0]) >= epsTilemap || math.Abs(b.Y-k.World[1]) >= epsTilemap ||
			b.W != o.TileW || b.H != o.TileH {
			t.Errorf("bounds[%d] = %v/%v, want origin %v size %.0fx%.0f", i, b, ok, k.World, o.TileW, o.TileH)
		}
	}
	for i, k := range o.Worlds {
		c, r, ok := m.WorldToCell(core.V2(k.World[0], k.World[1]))
		if !ok || c != k.Col || r != k.Row {
			t.Errorf("world[%d] %v = (%d,%d)/%v, want (%d,%d)/true", i, k.World, c, r, ok, k.Col, k.Row)
		}
	}
	for i, w := range o.OOBWorlds {
		if c, r, ok := m.WorldToCell(core.V2(w[0], w[1])); ok {
			t.Errorf("oob world[%d] %v = (%d,%d)/true, want false", i, w, c, r)
		}
	}
	for i, k := range o.Solids {
		if got := m.IsSolidAt(k.Col, k.Row); got != k.Want {
			t.Errorf("solid[%d] (%d,%d) = %v, want %v", i, k.Col, k.Row, got, k.Want)
		}
	}
	for i, k := range o.Navs {
		cost, ok := m.NavCostAt(k.Col, k.Row)
		if ok != k.OK || (ok && cost != k.Cost) {
			t.Errorf("nav[%d] (%d,%d) = %v/%v, want %v/%v", i, k.Col, k.Row, cost, ok, k.Cost, k.OK)
		}
	}
	for i, k := range o.Occlusions {
		if got := m.OccludedAt(k.Col, k.Row); got != k.Want {
			t.Errorf("occl[%d] (%d,%d) = %v, want %v", i, k.Col, k.Row, got, k.Want)
		}
	}
	ground, ok := m.LayerIndex("ground")
	if !ok {
		t.Fatal("no ground layer for automask")
	}
	for i, k := range o.Masks {
		got, ok := m.AutoMaskAt(ground, k.Col, k.Row)
		if !ok || got != k.Mask {
			t.Errorf("mask[%d] (%d,%d) = %v/%v, want %v/true", i, k.Col, k.Row, got, ok, k.Mask)
		}
		if v, ok := AutoVariant4(got); !ok || v != got {
			t.Errorf("variant[%d] mask %v -> %v/%v, want identity", i, got, v, ok)
		}
	}
	for i, k := range o.Variants {
		v, ok := AutoVariant4(k.Mask)
		if ok != k.OK || (ok && v != k.Variant) {
			t.Errorf("variant[%d] %v = %v/%v, want %v/%v", i, k.Mask, v, ok, k.Variant, k.OK)
		}
	}
	if len(m.Objects()) != len(o.Objects) {
		t.Fatalf("objects = %d, want %d", len(m.Objects()), len(o.Objects))
	}
	byName := map[string]Object{}
	for _, ob := range m.Objects() {
		byName[ob.Name()] = ob
	}
	for i, k := range o.Objects {
		got, ok := byName[k.Name]
		if !ok {
			t.Errorf("object[%d] missing %q", i, k.Name)
			continue
		}
		if got.ID() != k.ID || got.Type() != k.Type {
			t.Errorf("object[%d] = id %v type %q, want id %v type %q", i, got.ID(), got.Type(), k.ID, k.Type)
		}
		b := got.Bounds()
		if math.Abs(b.X-k.Bounds[0]) >= epsTilemap || math.Abs(b.Y-k.Bounds[1]) >= epsTilemap ||
			math.Abs(b.W-k.Bounds[2]) >= epsTilemap || math.Abs(b.H-k.Bounds[3]) >= epsTilemap {
			t.Errorf("object[%d] bounds = %v, want %v", i, b, k.Bounds)
		}
	}
	for i, k := range o.ObjectsIn {
		rect := core.NewRect(k.Rect[0], k.Rect[1], k.Rect[2], k.Rect[3])
		got := objNames(m.ObjectsIn(rect))
		if !equalStr(got, k.Want) {
			t.Errorf("objects_in[%d] %v = %v, want %v", i, k.Rect, got, k.Want)
		}
	}

	// Isometric half: origins, centers, owning diamonds, edge inclusion.
	isoMap, cases2 := loadIsoMap(t)
	if isoMap.Orient() != OrientIsometric {
		t.Fatalf("iso orient = %v, want isometric", isoMap.Orient())
	}
	iso, err := NewIso(cases2.Iso.TileW, cases2.Iso.TileH)
	if err != nil {
		t.Fatalf("NewIso: %v", err)
	}
	for i, k := range cases2.Iso.Tiles {
		got, ok := iso.TileToWorld(k.Col, k.Row)
		if !ok || !closeVec(got, k.Origin) {
			t.Errorf("iso tile[%d] (%d,%d) origin = %v/%v, want %v/true", i, k.Col, k.Row, got, ok, k.Origin)
		}
		c, ok := iso.TileCenter(k.Col, k.Row)
		if !ok || !closeVec(c, k.Center) {
			t.Errorf("iso tile[%d] (%d,%d) center = %v/%v, want %v/true", i, k.Col, k.Row, c, ok, k.Center)
		}
		if b, ok := iso.TileBounds(k.Col, k.Row); !ok || !closeVec(core.V2(b.X, b.Y), k.Origin) ||
			b.W != cases2.Iso.TileW || b.H != cases2.Iso.TileH {
			t.Errorf("iso tile[%d] bounds = %v/%v, want origin %v", i, b, ok, k.Origin)
		}
		// Tilemap delegates to the same iso math.
		if got2, ok := isoMap.CellToWorld(k.Col, k.Row); !ok || got2 != got {
			t.Errorf("iso map cell[%d] = %v/%v, want iso %v", i, got2, ok, got)
		}
	}
	for i, k := range cases2.Iso.Worlds {
		c, r, ok := iso.WorldToTile(core.V2(k.World[0], k.World[1]))
		if !ok || c != k.Col || r != k.Row {
			t.Errorf("iso world[%d] %v = (%d,%d)/%v, want (%d,%d)/true", i, k.World, c, r, ok, k.Col, k.Row)
		}
	}
	for i, k := range cases2.Iso.Contains {
		if got := iso.DiamondContains(k.Col, k.Row, core.V2(k.World[0], k.World[1])); got != k.Want {
			t.Errorf("iso contains[%d] (%d,%d) %v = %v, want %v", i, k.Col, k.Row, k.World, got, k.Want)
		}
	}
}

// B:空零超大坏数据缺块全不崩不卡死,占位加报错.
func TestTilemapEdgesNoCrash(t *testing.T) {
	m, _ := loadOrthoMap(t)
	// OOB cells are placeholders: empty gid, not solid, no cost, no shade.
	for _, c := range [][2]int{{-1, 0}, {4, 0}, {0, -1}, {0, 3}, {99, 99}} {
		if g, ok := m.At(0, c[0], c[1]); ok || g != 0 {
			t.Errorf("oob At(%d,%d) = %v/%v, want 0/false", c[0], c[1], g, ok)
		}
		if _, ok := m.CellToWorld(c[0], c[1]); ok {
			t.Errorf("oob CellToWorld(%d,%d) ok=true, want false", c[0], c[1])
		}
		if m.IsSolidAt(c[0], c[1]) {
			t.Errorf("oob IsSolid(%d,%d) = true, want false", c[0], c[1])
		}
		if _, ok := m.NavCostAt(c[0], c[1]); ok {
			t.Errorf("oob Nav(%d,%d) ok=true, want false", c[0], c[1])
		}
		if m.OccludedAt(c[0], c[1]) {
			t.Errorf("oob Occl(%d,%d) = true, want false", c[0], c[1])
		}
		if _, ok := m.AutoMaskAt(0, c[0], c[1]); ok {
			t.Errorf("oob Mask(%d,%d) ok=true, want false", c[0], c[1])
		}
	}
	if _, ok := m.At(-1, 0, 0); ok {
		t.Error("bad layer -1 ok=true, want false")
	}
	if _, ok := m.At(99, 0, 0); ok {
		t.Error("bad layer 99 ok=true, want false")
	}
	if _, ok := m.AutoMaskAt(99, 0, 0); ok {
		t.Error("bad mask layer ok=true, want false")
	}
	for _, bad := range []int{-1, 16, 99} {
		if _, ok := AutoVariant4(bad); ok {
			t.Errorf("variant %v ok=true, want false", bad)
		}
	}
	// Empty/zero constructors are rejected, never guessed.
	if _, err := NewTilemap(OrientOrthogonal, 0, 3, 32, 32); err == nil {
		t.Error("zero mapW want error")
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero mapW code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewTilemap(Orientation(9), 4, 3, 32, 32); err == nil {
		t.Error("bad orient want error")
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad orient code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewTilemap(OrientOrthogonal, 4, 3, 0, 32); err == nil {
		t.Error("zero tileW want error")
	}
	if _, err := NewIso(0, 32); err == nil {
		t.Error("zero iso tileW want error")
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero iso code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewIso(math.NaN(), 32); err == nil {
		t.Error("NaN iso want error")
	}
	if _, err := NewLayer("", KindGround, 4, 3, make([]int, 12)); err == nil {
		t.Error("empty layer name want error")
	}
	if _, err := NewLayer("x", LayerKind(9), 4, 3, make([]int, 12)); err == nil {
		t.Error("bad kind want error")
	}
	if _, err := NewLayer("x", KindGround, 4, 3, []int{1, 2}); err == nil {
		t.Error("short gids want error")
	}
	if _, err := NewLayer("x", KindGround, 2, 2, []int{1, -1, 0, 0}); err == nil {
		t.Error("negative gid want error")
	}
	if _, err := NewObject(0, "x", "t", core.NewRect(0, 0, 1, 1)); err == nil {
		t.Error("zero id want error")
	}
	if _, err := NewObject(1, "x", "t", core.NewRect(math.NaN(), 0, 1, 1)); err == nil {
		t.Error("NaN bounds want error")
	}
	// Duplicate names and ids are rejected.
	dupLayer, _ := NewLayer("ground", KindGround, m.W(), m.H(), make([]int, m.W()*m.H()))
	if err := m.AddLayer(dupLayer); err == nil {
		t.Error("dup layer want error")
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("dup layer code = %v, want invalid-arg", core.CodeOf(err))
	}
	dupObj, _ := NewObject(1, "dup", "t", core.NewRect(0, 0, 1, 1))
	if err := m.AddObject(dupObj); err == nil {
		t.Error("dup object want error")
	}
	// Bad world inputs fail closed with zero outputs.
	for _, bad := range []core.Vec2{{X: math.NaN()}, {Y: math.Inf(1)}, {X: math.Inf(-1), Y: 1}} {
		if _, _, ok := m.WorldToCell(bad); ok {
			t.Errorf("bad world %v ok=true, want false", bad)
		}
	}
	if got := m.ObjectsIn(core.Rect{}); got != nil {
		t.Errorf("empty rect ObjectsIn = %v, want nil", got)
	}
	if got := m.ObjectsIn(core.NewRect(math.NaN(), 0, 10, 10)); got != nil {
		t.Errorf("NaN rect ObjectsIn = %v, want nil", got)
	}
	iso, _ := NewIso(64, 32)
	if iso.DiamondContains(0, 0, core.V2(math.NaN(), 0)) {
		t.Error("NaN diamond = true, want false")
	}
	if _, _, ok := iso.WorldToTile(core.V2(math.Inf(1), 0)); ok {
		t.Error("Inf iso world ok=true, want false")
	}
	// File faults: malformed counts are BadData, reserved shapes are Unsupported.
	for _, f := range []struct {
		file string
		code core.Code
	}{
		{"bad_count.tmx", core.CodeBadData},
		{"unsupported_hex.tmx", core.CodeUnsupported},
		{"unsupported_base64.tmx", core.CodeUnsupported},
		{"unsupported_ellipse.tmx", core.CodeUnsupported},
	} {
		raw := mustReadTestdata(t, f.file)
		if _, err := ParseTMX(raw); err == nil {
			t.Errorf("%s want error", f.file)
		} else if core.CodeOf(err) != f.code {
			t.Errorf("%s code = %v, want %v", f.file, core.CodeOf(err), f.code)
		}
	}
	if _, err := ParseTMX(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty TMX code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := ParseTMX([]byte("<map></map>")); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("short XML code = %v, want bad-data", core.CodeOf(err))
	}
}

// C:纯算数不画画,两边同数靠边界往返无损加逐位重放.
func TestTilemapBoundaryIdentical(t *testing.T) {
	m, cases := loadOrthoMap(t)
	// Render boundary is lossless for every frozen cell origin.
	for _, k := range cases.Ortho.Cells {
		got, ok := m.CellToWorld(k.Col, k.Row)
		if !ok {
			t.Fatalf("cell (%d,%d) ok=false", k.Col, k.Row)
		}
		if back := core.Vec2FromRenderPoint(got.ToRenderPoint()); back != got {
			t.Errorf("cell (%d,%d) boundary = %v, want %v", k.Col, k.Row, back, got)
		}
		// Round trip closes: world -> cell -> world.
		c, r, ok := m.WorldToCell(got)
		if !ok || c != k.Col || r != k.Row {
			t.Errorf("ortho round trip %v -> (%d,%d)/%v, want (%d,%d)", got, c, r, ok, k.Col, k.Row)
		}
		again, _ := m.CellToWorld(k.Col, k.Row)
		if again != got {
			t.Errorf("replay diverged: %v vs %v", again, got)
		}
	}
	// Iso centers round trip through the diamond picker.
	iso, _ := NewIso(cases.Iso.TileW, cases.Iso.TileH)
	for _, k := range cases.Iso.Tiles {
		c, ok := iso.TileCenter(k.Col, k.Row)
		if !ok {
			t.Fatalf("iso center (%d,%d) ok=false", k.Col, k.Row)
		}
		if back := core.Vec2FromRenderPoint(c.ToRenderPoint()); back != c {
			t.Errorf("iso boundary (%d,%d) = %v, want %v", k.Col, k.Row, back, c)
		}
		gc, gr, ok := iso.WorldToTile(c)
		if !ok || gc != k.Col || gr != k.Row {
			t.Errorf("iso round trip %v -> (%d,%d)/%v, want (%d,%d)", c, gc, gr, ok, k.Col, k.Row)
		}
	}
	// Same file parses to the same map twice (no hidden state).
	raw := mustReadTestdata(t, cases.Ortho.TMX)
	a, err := ParseTMX(raw)
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}
	b, err := ParseTMX(raw)
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}
	for i := range a.Layers() {
		ga, gb := a.Layers()[i].GIDs(), b.Layers()[i].GIDs()
		for j := range ga {
			if ga[j] != gb[j] {
				t.Fatalf("parse replay diverged layer %d cell %d", i, j)
			}
		}
	}
	// Layer GIDs are copies: writing the result cannot alias the map.
	l0 := m.Layers()[0].GIDs()
	l0[0] = -999
	if g, _ := m.At(0, 0, 0); g == -999 {
		t.Error("GIDs aliases map storage")
	}
}

// D:大图 queries 跑得动,帧率调用有数.
func TestTilemapPerfLarge(t *testing.T) {
	// Synthetic load only (no golden): golden stays in tilemap_cases.json.
	// Seeded rand keeps the load replayable.
	const W, H = 128, 128
	m, err := NewTilemap(OrientOrthogonal, W, H, 32, 32)
	if err != nil {
		t.Fatalf("NewTilemap: %v", err)
	}
	r := core.NewRand(20260915)
	gids := make([]int, W*H)
	for i := range gids {
		if r.Float64() < 0.7 {
			gids[i] = int(r.RangeInt(1, 4))
		}
	}
	ground, err := NewLayer("ground", KindGround, W, H, gids)
	if err != nil {
		t.Fatalf("NewLayer: %v", err)
	}
	if err := m.AddLayer(ground); err != nil {
		t.Fatalf("AddLayer: %v", err)
	}
	coll := make([]int, W*H)
	for i := range coll {
		if r.Float64() < 0.05 {
			coll[i] = 1
		}
	}
	cl, err := NewLayer("collide", KindCollision, W, H, coll)
	if err != nil {
		t.Fatalf("NewLayer collide: %v", err)
	}
	if err := m.AddLayer(cl); err != nil {
		t.Fatalf("AddLayer collide: %v", err)
	}
	groundIdx, _ := m.LayerIndex("ground")
	const reps = 2000
	start := time.Now()
	solids := 0
	for i := 0; i < reps; i++ {
		col := int(r.RangeInt(0, W))
		row := int(r.RangeInt(0, H))
		if _, ok := m.At(groundIdx, col, row); !ok {
			t.Fatalf("rep %d At ok=false", i)
		}
		if _, ok := m.AutoMaskAt(groundIdx, col, row); !ok {
			t.Fatalf("rep %d Mask ok=false", i)
		}
		if m.IsSolidAt(col, row) {
			solids++
		}
		if _, _, ok := m.WorldToCell(core.V2(float64(col*32+7), float64(row*32+9))); !ok {
			t.Fatalf("rep %d WorldToCell ok=false", i)
		}
	}
	el := time.Since(start)
	t.Logf("tilemap-128: %d ops x 4 queries on %dx%d in %v (%.1f us/op, solids %d)", reps, W, H, el, float64(el.Microseconds())/reps, solids)
}

// E:反复进出长跑不涨不漂不粘.
func TestTilemapLongRunStable(t *testing.T) {
	m, cases := loadOrthoMap(t)
	ground, _ := m.LayerIndex("ground")
	// 10k mask replays stay bitwise identical and never mutate the map.
	first, _ := m.AutoMaskAt(ground, 1, 1)
	snap, _ := m.At(ground, 1, 1)
	for i := 0; i < 10000; i++ {
		got, ok := m.AutoMaskAt(ground, 1, 1)
		if !ok || got != first {
			t.Fatalf("rep %d mask = %v/%v, want %v/true", i, got, ok, first)
		}
	}
	if g, _ := m.At(ground, 1, 1); g != snap {
		t.Fatal("10k masks mutated the map")
	}
	// Walk across a wall and back: solid flips and returns, never sticks.
	for _, c := range [][2]int{{0, 0}, {1, 1}, {0, 0}} {
		want := c[0] == 1 && c[1] == 1
		if got := m.IsSolidAt(c[0], c[1]); got != want {
			t.Errorf("walk (%d,%d) solid = %v, want %v", c[0], c[1], got, want)
		}
	}
	// Reparse 200 times: same bytes, same cells, no drift.
	raw := mustReadTestdata(t, cases.Ortho.TMX)
	for i := 0; i < 200; i++ {
		again, err := ParseTMX(raw)
		if err != nil {
			t.Fatalf("reparse %d: %v", i, err)
		}
		for _, k := range cases.Ortho.At {
			idx, _ := again.LayerIndex(k.Layer)
			if g, _ := again.At(idx, k.Col, k.Row); g != k.Want {
				t.Fatalf("reparse %d %s(%d,%d) = %v, want %v", i, k.Layer, k.Col, k.Row, g, k.Want)
			}
		}
	}
	// Iso long walk: centers always map back to their own tile.
	iso, _ := NewIso(cases.Iso.TileW, cases.Iso.TileH)
	for i, k := range cases.Iso.Tiles {
		c, ok := iso.TileCenter(k.Col, k.Row)
		if !ok {
			t.Fatalf("tile[%d] center ok=false", i)
		}
		for rep := 0; rep < 1000; rep++ {
			gc, gr, ok := iso.WorldToTile(c)
			if !ok || gc != k.Col || gr != k.Row {
				t.Fatalf("tile[%d] rep %d -> (%d,%d)/%v", i, rep, gc, gr, ok)
			}
		}
	}
}

// F:离屏金对照窗 (W1免窗,game_tilemap后建):冻结数加形状断言.
func TestTilemapOffscreenGolden(t *testing.T) {
	m, cases := loadOrthoMap(t)
	// Golden numbers stay frozen.
	for _, k := range cases.Ortho.At {
		idx, _ := m.LayerIndex(k.Layer)
		if g, _ := m.At(idx, k.Col, k.Row); g != k.Want {
			t.Fatalf("golden %s(%d,%d) = %v, want %v", k.Layer, k.Col, k.Row, g, k.Want)
		}
	}
	// Shape: orthogonal grid is axis aligned with exact tile strides.
	a, _ := m.CellToWorld(0, 0)
	b, _ := m.CellToWorld(1, 0)
	c, _ := m.CellToWorld(0, 1)
	if b.X-a.X != cases.Ortho.TileW || b.Y != a.Y {
		t.Errorf("x stride = %v, want (%v,0)", b.Sub(a), cases.Ortho.TileW)
	}
	if c.Y-a.Y != cases.Ortho.TileH || c.X != a.X {
		t.Errorf("y stride = %v, want (0,%v)", c.Sub(a), cases.Ortho.TileH)
	}
	if area, _ := m.CellBounds(0, 0); area.W*area.H != cases.Ortho.TileW*cases.Ortho.TileH {
		t.Errorf("cell area = %v, want %.0f", area.Area(), cases.Ortho.TileW*cases.Ortho.TileH)
	}
	// Shape: full-surround mask is 15, isolated is 0, edge is partial.
	ground, _ := m.LayerIndex("ground")
	if full, _ := m.AutoMaskAt(ground, 1, 1); full != 15 {
		t.Errorf("surrounded mask = %v, want 15", full)
	}
	if iso, _ := m.AutoMaskAt(ground, 3, 2); iso != 0 {
		t.Errorf("isolated mask = %v, want 0", iso)
	}
	// Shape: collision never claims an empty art hole as walkable confusion:
	// (2,1) has art but no solid; (1,1) has both.
	if m.IsSolidAt(2, 1) {
		t.Error("(2,1) solid = true, want false (art without collision)")
	}
	if !m.IsSolidAt(1, 1) {
		t.Error("(1,1) solid = false, want true")
	}
	// Iso shape: diamond area is half the box, centers sit half-tile in.
	iso, _ := NewIso(cases.Iso.TileW, cases.Iso.TileH)
	box, _ := iso.TileBounds(0, 0)
	if box.W != cases.Iso.TileW || box.H != cases.Iso.TileH {
		t.Errorf("iso box = %v, want %.0fx%.0f", box, cases.Iso.TileW, cases.Iso.TileH)
	}
	origin, _ := iso.TileToWorld(0, 0)
	center, _ := iso.TileCenter(0, 0)
	if center.X-origin.X != cases.Iso.TileW/2 || center.Y-origin.Y != cases.Iso.TileH/2 {
		t.Errorf("iso center offset = %v, want (%.1f,%.1f)", center.Sub(origin), cases.Iso.TileW/2, cases.Iso.TileH/2)
	}
	// Neighbour origins step by half tiles on both axes.
	east, _ := iso.TileToWorld(1, 0)
	if east.X-origin.X != cases.Iso.TileW/2 || east.Y-origin.Y != cases.Iso.TileH/2 {
		t.Errorf("iso east step = %v, want (%.1f,%.1f)", east.Sub(origin), cases.Iso.TileW/2, cases.Iso.TileH/2)
	}
	south, _ := iso.TileToWorld(0, 1)
	if south.X-origin.X != -cases.Iso.TileW/2 || south.Y-origin.Y != cases.Iso.TileH/2 {
		t.Errorf("iso south step = %v, want (%.1f,%.1f)", south.Sub(origin), -cases.Iso.TileW/2, cases.Iso.TileH/2)
	}
}
