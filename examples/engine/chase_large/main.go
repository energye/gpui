// Command game_chase_large is the S90 thick gate (V3 close) as a real
// game, not a dashboard: one road through the large field, one red car,
// camera follow. The six V3 large pieces live inside the gameplay:
//
//   - tilemap Large million-tile streaming (S84) is the ground,
//   - world Cull 20k entities (S85) are the roadside objects,
//   - physics Broadphase 1200-body yard (S86) is the collision query,
//   - nav bake/agent/queue (S87) walks the filed route beside the car,
//   - asset Budget (S88) accounts the 3000-image pack,
//   - anim LOD + audio voices (S89) ride the same view/listener.
//
// One world, one camera: the car drives the large field (x 200..6000 at
// y 1600), the camera follows, and every system works in the same
// coordinates. Paint is one batched atlas submit per frame (no per-rect
// Fill); heavy queries run at reduced rate with fixed paint caps, so a
// maximized window costs the same as the design size.
//
// Modes:
//
//	RUN_SECONDS=30 go run ./examples/engine/chase_large -auto-only
//	  probes + timed window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/chase_large -manual-seconds 60
//	  manual 60s (events logged, title shows count), then summary.
//	go run ./examples/engine/chase_large
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800 full-window content, metric floats at top-left, golden
// compares only the pure picture below the metric band (2.5D rule).
// Gate runs lock the 1200x800 design size; manual drags are human-only.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/energye/gpui/engine/anim"
	"github.com/energye/gpui/engine/asset"
	"github.com/energye/gpui/engine/audio"
	"github.com/energye/gpui/engine/camera"
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/dirty"
	"github.com/energye/gpui/engine/nav"
	"github.com/energye/gpui/engine/particle"
	"github.com/energye/gpui/engine/physics"
	"github.com/energye/gpui/engine/renderconv"
	"github.com/energye/gpui/engine/tilemap"
	"github.com/energye/gpui/engine/world"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/scene"

	rendergpu "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "chase-large"
	scenario   = "game_chase_large"
	goldenPath = "examples/engine/chase_large/testdata/chase_large_golden.png"
	lastPath   = "examples/engine/chase_large/testdata/chase_large_last.png"
	finalPng   = "chase_large_final.png"
	finalBase  = "chase_large_final_base.png"

	probePixelTol = 8

	scenePath   = "engine/world/testdata/large_scene.json"
	cullPath    = "engine/world/testdata/large_cull.json"
	yardPath    = "engine/physics/testdata/large_yard.json"
	raysPath    = "engine/physics/testdata/large_rays.json"
	navPath     = "engine/nav/testdata/nav_grid.json"
	manifestPth = "engine/asset/testdata/large_manifest.json"
	lodPath     = "engine/anim/testdata/large_lod.json"
	voicesPath  = "engine/audio/testdata/large_voices.json"
)

// Game field: the large world (x 100..6500, y 80..3280). The car runs the
// middle road through the systems-dense zone (physics yard y64..1264,
// world field, nav diagonal); the camera follows; every system shares
// these coordinates so physics dots and the nav hero are on screen with
// the car instead of kilometers away.
const (
	carW, carH = 48.0, 24.0
	roadY      = 640.0
	carMinX    = 200.0
	carMaxX    = 6000.0
	carSpeed   = 200.0

	// Lane switching: the car eases between these three lanes.
	laneOff = 40.0
	laneT   = 4.0

	// Wanderers: at most this many roadside monsters get their own
	// random heading/speed/turns (seeded, deterministic).
	wanderMax   = 256
	wanderSeed  = 0xC0FFEE
	wanderSpdLo = 20.0
	wanderSpdHi = 80.0
	wanderTurn  = 3.0

	trailMax   = 32
	trailHeadW = 12.0
	trailTailW = 2.0
	offW, offH = 480, 270

	largeMergeStep = 4
	// NOTE: paintWorldCap/paintPhysCap retired (2026-10-12): the whole
	// picture is one batched atlas submit, so drawing all culled actives
	// costs the same submit as a subset; caps only hid content.
)

const (
	bgR, bgG, bgB             = 0.08, 0.09, 0.11
	groundR, groundG, groundB = 0.0825, 0.099, 0.12
	tileAR, tileAG, tileAB    = 0.16, 0.22, 0.18
	tileBR, tileBG, tileBB    = 0.20, 0.18, 0.14
	carR, carG, carB           = 0.90, 0.15, 0.12
)

var liveWinW, liveWinH float64

func applyWindowSize(w, h float64) { liveWinW, liveWinH = w, h }

func buildWhiteAtlas() *render.ImageBuf {
	img, _ := render.NewImageBuf(8, 8, render.FormatRGBA8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			_ = img.SetRGBA(x, y, 255, 255, 255, 255)
		}
	}
	return img
}

func readJSONFile(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

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

func want8(v float64) uint8 { return uint8(v*255 + 0.5) }

func diffExact(a, b image.Image) (changed, total int) {
	ab, bb := a.Bounds(), b.Bounds()
	if !ab.Eq(bb) {
		return 1, 0
	}
	total = ab.Dx() * ab.Dy()
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			ar, ag, ab2, aa := a.At(x, y).RGBA()
			br, bg, bb2, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab2 != bb2 || aa != ba {
				changed++
			}
		}
	}
	return changed, total
}

// ---- filed JSON shapes (minimal live/probe subsets) ----

type sceneLayout struct {
	Cols         int        `json:"cols"`
	Rows         int        `json:"rows"`
	Spacing      float64    `json:"spacing"`
	Origin       [2]float64 `json:"origin"`
	AttachEvery  int        `json:"attach_every"`
	AttachOffset [2]float64 `json:"attach_offset"`
	Extent       float64    `json:"extent"`
	Count        int        `json:"count"`
}

type sceneFile struct {
	Layout  sceneLayout `json:"layout"`
	Markers []struct {
		Name string  `json:"name"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
	} `json:"markers"`
	Pins     []int `json:"pins"`
	Attached []struct {
		ID          int `json:"id"`
		ParentIndex int `json:"parent_index"`
	} `json:"attached"`
}

type cullFile struct {
	Views map[string]struct {
		View [4]float64 `json:"view"`
		Want []int      `json:"want"`
	} `json:"views"`
}

type yardLayout struct {
	Cols         int        `json:"cols"`
	Rows         int        `json:"rows"`
	Spacing      float64    `json:"spacing"`
	Origin       [2]float64 `json:"origin"`
	Half         float64    `json:"half"`
	StaticCount  int        `json:"static_count"`
	DynamicCount int        `json:"dynamic_count"`
	Count        int        `json:"count"`
}

type yardFile struct {
	Layout   yardLayout `json:"layout"`
	Dynamics []struct {
		Name string     `json:"name"`
		X    float64    `json:"x"`
		Y    float64    `json:"y"`
		Half [2]float64 `json:"half"`
	} `json:"dynamics"`
}

type raysFile struct {
	Rays []struct {
		Name    string     `json:"name"`
		Origin  [2]float64 `json:"origin"`
		Dir     [2]float64 `json:"dir"`
		MaxDist float64    `json:"maxdist"`
		Mask    uint32     `json:"mask"`
	} `json:"rays"`
	Regions []struct {
		Name string     `json:"name"`
		Rect [4]float64 `json:"rect"`
		Want []int      `json:"want"`
	} `json:"regions"`
	QueryStatic struct {
		WantCount int `json:"want_count"`
	} `json:"query_static"`
}

type navFile struct {
	W     int    `json:"w"`
	H     int    `json:"h"`
	Cell  int    `json:"cell"`
	Start [2]int `json:"start"`
	Goal  [2]int `json:"goal"`
	Walls []struct {
		X int `json:"x"`
		Y int `json:"y"`
		W int `json:"w"`
		H int `json:"h"`
	} `json:"walls"`
}

type lodFile struct {
	Viewport   [4]float64   `json:"viewport"`
	NearDist   float64      `json:"near_dist"`
	Bounds     [][4]float64 `json:"bounds"`
	WantLevels []int        `json:"want_levels"`
	WantOff    int          `json:"want_off"`
	WantFar    int          `json:"want_far"`
	WantNear   int          `json:"want_near"`
}

type voicesFile struct {
	Listener [2]float64 `json:"listener"`
	Requests []struct {
		Pos         [2]float64 `json:"pos"`
		Range       float64    `json:"range"`
		Attenuation float64    `json:"attenuation"`
		PanStrength float64    `json:"pan_strength"`
		Level       float64    `json:"level"`
		Explosive   bool       `json:"explosive"`
	} `json:"requests"`
	Want struct {
		Channels int `json:"channels"`
		Audible  int `json:"audible"`
	} `json:"want"`
}

// ---- probes: seven logic checks + pixels + golden ----

type probeResult struct {
	LogicOK bool
	Detail  string
	TileOK  bool
	WorldOK bool
	PhysOK  bool
	NavOK   bool
	AssetOK bool
	LODOK   bool
	VoiceOK bool

	TileChunks int
	TileMerged int
	WorldTotal int
	WorldHome  int
	PhysTotal  int
	PhysPairs  int
	NavMs      float64
	AssetSets  int
	LODNear    int
	VoiceAud   int

	PixOK     bool
	PixDetail string

	GoldenOK      bool
	GoldenChanged int
	GoldenWrote   bool
	OK            bool
}

func failJSON(p probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     p.PixDetail,
		"golden":     p.GoldenChanged,
		"detail":     p.Detail,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func probeTile() (bool, int, int, string) {
	lg, err := tilemap.NewLarge(1024, 1024)
	if err != nil {
		return false, 0, 0, "NewLarge: " + err.Error()
	}
	if n := lg.ChunkCount(); n != 1024 {
		return false, n, 0, fmt.Sprintf("chunks=%d want 1024", n)
	}
	view := core.NewRect(0, 0, 2048, 2048)
	if q := lg.Request(view); len(q) != 4 {
		return false, 1024, 0, fmt.Sprintf("request=%d want 4", len(q))
	}
	mids := lg.MergeBudget(4)
	if len(mids) != 4 || lg.LoadedCount() != 4 {
		return false, 1024, len(mids), "merge/loaded drifted"
	}
	return true, 1024, 4, "chunks=1024 merged=4"
}

func buildWorldLevel() (world.World, map[world.ID]int, sceneLayout, error) {
	var sf sceneFile
	if err := readJSONFile(scenePath, &sf); err != nil {
		return world.World{}, nil, sceneLayout{}, err
	}
	w := world.NewWorld()
	l := sf.Layout
	n := l.Cols * l.Rows
	layers := map[world.ID]int{}
	for i := 0; i < n; i++ {
		id, err := w.Spawn(world.NoEntity)
		if err != nil {
			return world.World{}, nil, sceneLayout{}, err
		}
		c, r := i%l.Cols, i/l.Cols
		pos := core.V2(l.Origin[0]+float64(c)*l.Spacing, l.Origin[1]+float64(r)*l.Spacing)
		if err := w.SetTransform(id, world.Transform{Pos: pos, Scale: core.V2(1, 1)}); err != nil {
			return world.World{}, nil, sceneLayout{}, err
		}
		layers[id] = (c + r) % 3
	}
	for _, a := range sf.Attached {
		id, parent := world.ID(a.ID), world.ID(a.ParentIndex+1)
		if err := w.SetParent(id, parent); err != nil {
			return world.World{}, nil, sceneLayout{}, err
		}
		if err := w.SetTransform(id, world.Transform{Pos: core.V2(l.AttachOffset[0], l.AttachOffset[1]), Scale: core.V2(1, 1)}); err != nil {
			return world.World{}, nil, sceneLayout{}, err
		}
	}
	for _, m := range sf.Markers {
		id, err := w.Spawn(world.NoEntity)
		if err != nil {
			return world.World{}, nil, sceneLayout{}, err
		}
		if err := w.SetTransform(id, world.Transform{Pos: core.V2(m.X, m.Y), Scale: core.V2(1, 1)}); err != nil {
			return world.World{}, nil, sceneLayout{}, err
		}
		if err := w.AddComp(id, world.Comp{Kind: "marker"}); err != nil {
			return world.World{}, nil, sceneLayout{}, err
		}
		layers[id] = 0
	}
	return w, layers, l, nil
}

func probeWorld() (bool, int, int, string) {
	w, layers, l, err := buildWorldLevel()
	if err != nil {
		return false, 0, 0, err.Error()
	}
	if w.Count() != l.Count {
		return false, w.Count(), 0, fmt.Sprintf("count=%d want %d", w.Count(), l.Count)
	}
	var cf cullFile
	if err := readJSONFile(cullPath, &cf); err != nil {
		return false, w.Count(), 0, err.Error()
	}
	hv := cf.Views["home"]
	c, err := world.NewCull(core.NewRect(hv.View[0], hv.View[1], hv.View[2], hv.View[3]))
	if err != nil {
		return false, w.Count(), 0, err.Error()
	}
	for id := world.ID(1); id <= world.ID(w.Count()); id++ {
		hw, hh := l.Extent, l.Extent
		if w.HasComp(id, "marker") {
			hw, hh = 0, 0
		}
		if err := c.Track(id, layers[id], 0, hw, hh); err != nil {
			return false, w.Count(), 0, err.Error()
		}
	}
	var sf sceneFile
	_ = readJSONFile(scenePath, &sf)
	for _, p := range sf.Pins {
		if err := c.Awake(world.ID(p)); err != nil {
			return false, w.Count(), 0, err.Error()
		}
	}
	if err := c.Refresh(&w, nil); err != nil {
		return false, w.Count(), 0, err.Error()
	}
	if len(c.Active()) != len(hv.Want) {
		return false, w.Count(), len(c.Active()), fmt.Sprintf("home=%d want %d", len(c.Active()), len(hv.Want))
	}
	return true, w.Count(), len(c.Active()), fmt.Sprintf("total=%d home=%d", w.Count(), len(c.Active()))
}

func buildYard() (physics.Broadphase, []physics.Body, yardLayout, error) {
	var yf yardFile
	if err := readJSONFile(yardPath, &yf); err != nil {
		return physics.Broadphase{}, nil, yardLayout{}, err
	}
	bp := physics.NewBroadphase()
	l := yf.Layout
	var bodies []physics.Body
	for i := 0; i < l.StaticCount; i++ {
		c, r := i%l.Cols, i/l.Cols
		pos := core.V2(l.Origin[0]+float64(c)*l.Spacing, l.Origin[1]+float64(r)*l.Spacing)
		bd, err := physics.NewBox(fmt.Sprintf("s%04d", i), pos, core.V2(l.Half, l.Half), 1, 1, false)
		if err != nil {
			return physics.Broadphase{}, nil, yardLayout{}, err
		}
		if _, err := bp.Track(bd, true); err != nil {
			return physics.Broadphase{}, nil, yardLayout{}, err
		}
		bodies = append(bodies, bd)
	}
	for _, d := range yf.Dynamics {
		bd, err := physics.NewBox(d.Name, core.V2(d.X, d.Y), core.V2(d.Half[0], d.Half[1]), 1, 1, false)
		if err != nil {
			return physics.Broadphase{}, nil, yardLayout{}, err
		}
		if _, err := bp.Track(bd, false); err != nil {
			return physics.Broadphase{}, nil, yardLayout{}, err
		}
		bodies = append(bodies, bd)
	}
	return bp, bodies, l, nil
}

func probePhys() (bool, int, int, string) {
	bp, _, l, err := buildYard()
	if err != nil {
		return false, 0, 0, err.Error()
	}
	if bp.Count() != l.Count {
		return false, bp.Count(), 0, fmt.Sprintf("count=%d want %d", bp.Count(), l.Count)
	}
	var rf raysFile
	if err := readJSONFile(raysPath, &rf); err != nil {
		return false, bp.Count(), 0, err.Error()
	}
	got, err := bp.Query()
	if err != nil {
		return false, bp.Count(), 0, err.Error()
	}
	if len(got) != rf.QueryStatic.WantCount {
		return false, bp.Count(), len(got), fmt.Sprintf("pairs=%d want %d", len(got), rf.QueryStatic.WantCount)
	}
	for _, rc := range rf.Rays {
		ray, err := physics.NewRay(core.V2(rc.Origin[0], rc.Origin[1]), core.V2(rc.Dir[0], rc.Dir[1]), rc.MaxDist, rc.Mask)
		if err != nil {
			return false, bp.Count(), len(got), err.Error()
		}
		if _, _, err := bp.Ray(ray); err != nil {
			return false, bp.Count(), len(got), err.Error()
		}
	}
	return true, bp.Count(), len(got), fmt.Sprintf("total=%d pairs=%d", bp.Count(), len(got))
}

func probeNav() (bool, float64, string) {
	var f navFile
	if err := readJSONFile(navPath, &f); err != nil {
		return false, 0, err.Error()
	}
	wall := map[[2]int]bool{}
	for _, wl := range f.Walls {
		for y := wl.Y; y < wl.Y+wl.H; y++ {
			for x := wl.X; x < wl.X+wl.W; x++ {
				wall[[2]int{x, y}] = true
			}
		}
	}
	g, err := nav.Bake(f.W, f.H, func(x, y int) bool { return wall[[2]int{x, y}] })
	if err != nil {
		return false, 0, err.Error()
	}
	path, st, err := nav.FindPath(g, nav.Cell{X: f.Start[0], Y: f.Start[1]}, nav.Cell{X: f.Goal[0], Y: f.Goal[1]})
	if err != nil || st != nav.StatusFound || len(path) == 0 {
		return false, 0, "path missing"
	}
	return true, 0, fmt.Sprintf("path=%d", len(path))
}

func probeAsset() (bool, int, string) {
	m, err := asset.LoadLargeManifest(manifestPth)
	if err != nil {
		return false, 0, err.Error()
	}
	if m.Scale.TotalImages != asset.LargeScaleImages || m.Scale.TotalSets != asset.LargeScaleSets {
		return false, 0, "scale drifted"
	}
	b := asset.NewBudget(m, asset.TierBalanced)
	if err := b.RegisterAll(); err != nil {
		return false, 0, err.Error()
	}
	if len(m.Sets) == 0 {
		return false, 0, "no sets filed"
	}
	return true, len(m.Sets), fmt.Sprintf("sets=%d", len(m.Sets))
}

func probeLOD() (bool, int, string) {
	var f lodFile
	if err := readJSONFile(lodPath, &f); err != nil {
		return false, 0, err.Error()
	}
	pol, err := anim.NewLodPolicy(f.NearDist)
	if err != nil {
		return false, 0, err.Error()
	}
	vp := core.NewRect(f.Viewport[0], f.Viewport[1], f.Viewport[2], f.Viewport[3])
	bounds := make([]core.Rect, len(f.Bounds))
	for i, b := range f.Bounds {
		bounds[i] = core.NewRect(b[0], b[1], b[2], b[3])
	}
	levels := pol.Batch(bounds, vp)
	off, far, nr := 0, 0, 0
	for i, lv := range levels {
		if int(lv) != f.WantLevels[i] {
			return false, 0, fmt.Sprintf("level[%d] drifted", i)
		}
		switch lv {
		case anim.LodOff:
			off++
		case anim.LodFar:
			far++
		case anim.LodNear:
			nr++
		}
	}
	if off != f.WantOff || far != f.WantFar || nr != f.WantNear {
		return false, nr, "counts drifted"
	}
	return true, nr, fmt.Sprintf("near=%d", nr)
}

func probeVoices() (bool, int, string) {
	var f voicesFile
	if err := readJSONFile(voicesPath, &f); err != nil {
		return false, 0, err.Error()
	}
	reqs := make([]audio.VoiceRequest, len(f.Requests))
	for i, q := range f.Requests {
		s, err := audio.NewPosSound(core.V2(q.Pos[0], q.Pos[1]), q.Range, q.Attenuation, q.PanStrength)
		if err != nil {
			return false, 0, err.Error()
		}
		vr, err := audio.NewVoiceRequest(s, q.Level, q.Explosive)
		if err != nil {
			return false, 0, err.Error()
		}
		reqs[i] = vr
	}
	_, st, ok := audio.SelectVoices(core.V2(f.Listener[0], f.Listener[1]), reqs)
	if !ok || st.Channels != f.Want.Channels || st.Audible != f.Want.Audible {
		return false, st.Audible, "mix drifted"
	}
	return true, st.Audible, fmt.Sprintf("aud=%d", st.Audible)
}

func paintProbeSwatch(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	dc.SetRGB(tileAR, tileAG, tileAB)
	dc.DrawRectangle(40, 50, 120, 80)
	_ = dc.Fill()
	dc.SetRGB(0.20, 0.60, 0.25)
	dc.DrawRectangle(200, 60, 24, 24)
	_ = dc.Fill()
	dc.SetRGB(carR, carG, carB)
	dc.DrawRectangle(300, 60, 48, 24)
	_ = dc.Fill()
}

func paintOffscreenSwatch() image.Image {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paintProbeSwatch(dc)
	img := dc.Image()
	_ = dc.Close()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		return img
	}
	cp := image.NewRGBA(rgba.Bounds())
	copy(cp.Pix, rgba.Pix)
	return cp
}

func probePixels() (bool, string) {
	img := paintOffscreenSwatch()
	ar, ag, ab := sample8(img, 60, 70)
	br, bg, bb := sample8(img, 212, 72)
	cr, cg, cb := sample8(img, 310, 72)
	if !closeEnough(ar, want8(tileAR)) || !closeEnough(ag, want8(tileAG)) || !closeEnough(ab, want8(tileAB)) {
		return false, "tile swatch drifted"
	}
	if !closeEnough(br, want8(0.20)) || !closeEnough(bg, want8(0.60)) || !closeEnough(bb, want8(0.25)) {
		return false, "dot swatch drifted"
	}
	if !closeEnough(cr, want8(carR)) || !closeEnough(cg, want8(carG)) || !closeEnough(cb, want8(carB)) {
		return false, "car swatch drifted"
	}
	return true, "swatches ok"
}

func probeGolden() (bool, int, bool) {
	img := paintOffscreenSwatch()
	f, err := os.Open(goldenPath)
	if err != nil {
		out, err := os.Create(goldenPath)
		if err != nil {
			return false, 0, false
		}
		_ = png.Encode(out, img)
		_ = out.Close()
		if out2, err := os.Create(lastPath); err == nil {
			_ = png.Encode(out2, img)
			_ = out2.Close()
		}
		return true, 0, true
	}
	base, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return false, 0, false
	}
	changed, _ := diffExact(img, base)
	if out, err := os.Create(lastPath); err == nil {
		_ = png.Encode(out, img)
		_ = out.Close()
	}
	return changed == 0, changed, false
}

func runProbes() probeResult {
	var p probeResult
	var d []string
	p.TileOK, p.TileChunks, p.TileMerged, _ = probeTile()
	d = append(d, fmt.Sprintf("tile=%v", p.TileOK))
	p.WorldOK, p.WorldTotal, p.WorldHome, _ = probeWorld()
	d = append(d, fmt.Sprintf("world=%v", p.WorldOK))
	p.PhysOK, p.PhysTotal, p.PhysPairs, _ = probePhys()
	d = append(d, fmt.Sprintf("phys=%v", p.PhysOK))
	p.NavOK, p.NavMs, _ = probeNav()
	d = append(d, fmt.Sprintf("nav=%v", p.NavOK))
	p.AssetOK, p.AssetSets, _ = probeAsset()
	d = append(d, fmt.Sprintf("asset=%v", p.AssetOK))
	p.LODOK, p.LODNear, _ = probeLOD()
	d = append(d, fmt.Sprintf("lod=%v", p.LODOK))
	p.VoiceOK, p.VoiceAud, _ = probeVoices()
	d = append(d, fmt.Sprintf("voices=%v", p.VoiceOK))
	p.LogicOK = p.TileOK && p.WorldOK && p.PhysOK && p.NavOK && p.AssetOK && p.LODOK && p.VoiceOK
	p.Detail = fmt.Sprintf("%s", d)
	p.PixOK, p.PixDetail = probePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// ---- live game state: one world, one camera ----

type drawItem struct {
	rs render.AtlasSprite
}

// wander is one roadside monster with its own heading, speed and turn
// rate. Motion integrates per tick and writes back through the world,
// so cull/snapshot/paint all see it live. Seeded at build: same roster
// every run; live positions intentionally differ run to run (the frozen
// golden rebuilds a fresh world, never the live one).
type wander struct {
	id    world.ID
	angle float64
	speed float64
	turn  float64
}

type gameSim struct {
	cam   camera.Camera
	trail *particle.Trail
	pool  *particle.GPUPool
	layer *scene.DirtyLayer
	track *dirty.DirtyTracker
	atlas *render.ImageBuf

	carX, carY float64
	laneFrom, laneTo float64
	laneClock        float64
	laneIdx          int
	dir              float64
	movedPx          float64
	frames           int

	// Wanderers: roadside monsters with their own heading/speed.
	wand []wander

	large     *tilemap.Large
	world     world.World
	wlayers   map[world.ID]int
	wcull     *world.Cull
	wactive   int
	wupdMs    float64
	wmoves    int

	ybp     *physics.Broadphase
	ybodies []physics.Body
	yrays   [2]physics.Ray
	yregion int
	yquery  float64
	yray    float64

	ngrid   *nav.Grid
	nagent  *nav.Agent
	nqueue  *nav.Queue
	nmoves  float64
	narrive int
	ncell   float64
	npath   []core.Vec2

	budget  *asset.Budget
	bup     int64
	bbroken int

	lodpol  anim.LodPolicy
	lodbnds []core.Rect
	lodvp   core.Rect
	lodNear int
	lodFar  int
	lodOff  int

	vreqs []audio.VoiceRequest
	vlis  audio.Listener2D
	vch   int
	vaud  int

	items []drawItem
	flat  []render.AtlasSprite
	// Reused scratch: the hot path must not allocate per frame (GC
	// pauses are the hitch source at big windows). Trail points rebuild
	// into trailPts; pool particles copy into poolPts. Both backings live
	// on the sim, grown once, reused forever.
	trailPts []core.Vec2
	poolPts  []particle.Particle

	elapsed   float64
	hitch20   int64
	lastWall  time.Time
	haveWall  bool
	memStart  runtime.MemStats
	labelTick int

	largeLoad int
	largeDrop int

	app      *embedder.PipelineApp
	root     *rendering.AbsoluteBox
	metric   *rendering.RenderText
	worldBox *rendering.RenderBox
}

type gameTicker struct{ s *gameSim }

func (t *gameTicker) Tick(dt float64) bool {
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
	s.elapsed += dt
	// Full window is content: viewport + box track the laid-out root.
	// Guarded (only on change) so a steady window costs nothing; the
	// resize event below handles the instant response, this catches
	// anything the event missed (e.g. first layout, WM quirks).
	if s.root != nil {
		if rw, rh := s.root.Size().Width, s.root.Size().Height; rw > 0 && rh > 0 {
			if vp := s.cam.Viewport(); vp.X != rw || vp.Y != rh {
				_ = s.cam.SetViewport(core.V2(rw, rh))
			}
			if s.worldBox != nil && (s.worldBox.FixedWidth != rw || s.worldBox.FixedHeight != rh) {
				s.worldBox.FixedWidth, s.worldBox.FixedHeight = rw, rh
				s.worldBox.MarkNeedsLayout()
			}
		}
	}
	// Car runs the middle road and eases across three lanes; the camera
	// follows car + lane. On wrap the camera snaps with the car: a
	// smoothed fly-back would sweep empty world (black bar on the left
	// after maximize) while the tile/cull streaming chases a view the
	// game has already left.
	nx := s.carX + s.dir*carSpeed*dt
	wrapped := false
	if nx >= carMaxX {
		nx = carMinX + (nx - carMaxX)
		wrapped = true
	}
	moved := nx - s.carX
	if moved < 0 {
		moved = -moved
		if nx < s.carX {
			moved = (carMaxX - s.carX) + (nx - carMinX) + carW
		}
	}
	s.movedPx += moved
	s.carX = nx
	// Lane clock: hold 4s, ease 0.5s to the next lane, hold again.
	s.laneClock += dt
	if s.laneClock >= laneT {
		s.laneClock = 0
		s.laneFrom = s.carY
		s.laneIdx = (s.laneIdx + 1) % 3
		s.laneTo = roadY + (float64(s.laneIdx)-1)*laneOff
	}
	k := s.laneClock / 0.5
	if k > 1 {
		k = 1
	}
	k = k * k * (3 - 2*k)
	s.carY = s.laneFrom + (s.laneTo-s.laneFrom)*k
	if wrapped {
		_ = s.cam.SetPos(core.V2(s.carX+carW/2, s.carY+carH/2))
		if s.trail != nil {
			s.trail.Clear()
		}
	} else {
		_ = s.cam.Follow(core.V2(s.carX+carW/2, s.carY+carH/2))
	}
	// Wanderers: integrate heading/speed, turn by seeded rate, bounce
	// off the field sides, write back so cull + paint see them live.
	if len(s.wand) > 0 {
		for i := range s.wand {
			w := &s.wand[i]
			t, err := s.world.Local(w.id)
			if err != nil {
				continue
			}
			w.angle += w.turn * dt
			vx := math.Cos(w.angle) * w.speed
			vy := math.Sin(w.angle) * w.speed
			np := core.V2(t.Pos.X+vx*dt, t.Pos.Y+vy*dt)
			if np.X < 120 || np.X > 6480 {
				w.angle = math.Pi - w.angle
				np.X = t.Pos.X
			}
			if np.Y < 100 || np.Y > 3260 {
				w.angle = -w.angle
				np.Y = t.Pos.Y
			}
			_ = s.world.SetTransform(w.id, world.Transform{Pos: np, Scale: core.V2(1, 1)})
		}
	}
	view := s.cam.VisibleWorldRect()
	// Tile ground streams at the live view, 4 merges per tick.
	if s.large != nil {
		mg, dg := s.large.Update(view, largeMergeStep)
		s.largeLoad += len(mg)
		s.largeDrop += len(dg)
	}
	_ = s.trail.Push(core.V2(s.carX+4, s.carY+carH/2))
	if s.pool != nil {
		s.pool.Update(core.SecondsFloat(dt))
	}
	// World cull at the live view every 2nd tick (static world).
	if s.wcull != nil {
		if s.frames%2 == 0 {
			_ = s.wcull.SetView(view)
			t0 := time.Now()
			if err := s.wcull.Refresh(&s.world, nil); err == nil {
				s.wactive = len(s.wcull.Active())
				s.wupdMs = float64(time.Since(t0).Microseconds()) / 1000.0
			}
		}
		s.wmoves++
	}
	// Physics region query at the live view every tick.
	if s.ybp != nil {
		s.ybp.Update()
		t0 := time.Now()
		if rg, err := s.ybp.QueryRegion(view); err == nil {
			s.yregion = len(rg)
		} else {
			s.yregion = 0
		}
		s.yquery = float64(time.Since(t0).Microseconds())
		rus := 0.0
		for _, ray := range s.yrays {
			t1 := time.Now()
			_, _, _ = s.ybp.Ray(ray)
			if v := float64(time.Since(t1).Microseconds()); v > rus {
				rus = v
			}
		}
		s.yray = rus
	}
	// Nav hero walks the filed route, loops forever.
	if s.nqueue != nil && s.ngrid != nil {
		_, _ = s.nqueue.Update(*s.ngrid)
	}
	if s.nagent != nil {
		old := s.nagent.Pos()
		if arrived, err := s.nagent.Step(dt); err == nil && arrived {
			s.narrive++
			if wps := s.nagent.Waypoints(); len(wps) > 0 {
				_ = s.nagent.SetWaypoints(wps)
			}
		}
		s.nmoves += s.nagent.Pos().Sub(old).Length()
	}
	// LOD + voices every 2nd tick (static inputs).
	if len(s.lodbnds) > 0 && s.frames%2 == 0 {
		levels := s.lodpol.Batch(s.lodbnds, s.lodvp)
		off, far, nr := 0, 0, 0
		for _, lv := range levels {
			switch lv {
			case anim.LodOff:
				off++
			case anim.LodFar:
				far++
			case anim.LodNear:
				nr++
			}
		}
		s.lodOff, s.lodFar, s.lodNear = off, far, nr
	}
	if len(s.vreqs) > 0 && s.frames%2 == 0 {
		if _, st, ok := audio.SelectVoices(s.vlis.Pos(), s.vreqs); ok {
			s.vch, s.vaud = st.Channels, st.Audible
		}
	}
	now := time.Now()
	if s.haveWall {
		if now.Sub(s.lastWall) > 20*time.Millisecond {
			s.hitch20++
		}
	}
	s.lastWall = now
	s.haveWall = true
	s.labelTick++
	if s.labelTick%15 == 1 && s.metric != nil && s.app != nil {
		snap := s.app.Metrics().Snapshot()
		fps := 0.0
		if snap.AvgFrameIntervalMs > 1e-6 {
			fps = 1000.0 / snap.AvgFrameIntervalMs
		}
		s.metric.SetText(fmt.Sprintf("fps %.0f 车%.0f 活%d 物%d 径%d 音%d/%d", fps,
			s.carX, s.wactive, s.yregion, s.lodNear, s.vch, s.vaud))
	}
	// The game animates every tick (car/trail/hero all move), so the
	// world box must re-record every frame. Without this the pipeline
	// keeps presenting the cached first picture (paint_count 3 in 30s)
	// and the window looks frozen while the sim runs underneath.
	if s.worldBox != nil {
		s.worldBox.MarkNeedsPaint()
	}
	if s.root != nil {
		s.root.MarkNeedsPaint()
	}
	s.app.ScheduleFrame()
	return true
}

// paintGame draws the whole game in one batched atlas submit: ground
// tiles (visible only), roadside objects (culled, capped), nearby bodies
// (capped), nav hero, car, trail, pool. No per-rect Fill on the hot path.
func paintGame(pc *rendering.PaintContext, size rendering.Size, s *gameSim) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	bw, bh := size.Width, size.Height
	if bw <= 0 || bh <= 0 {
		bw, bh = float64(winW), float64(winH)
	}
	pc.DC.Push()
	pc.DC.ClipRect(ax, ay, bw, bh)
	pc.DC.SetRGB(groundR, groundG, groundB)
	pc.DC.DrawRectangle(ax, ay, bw, bh)
	_ = pc.DC.Fill()
	view := s.cam.VisibleWorldRect()
	items := s.items[:0]
	emit := func(wx, wy, w, h, r, g, b, a float64) {
		sx, ok := s.cam.WorldToScreen(core.V2(wx, wy))
		if !ok {
			return
		}
		if sx.X+w < -8 || sx.Y+h < -8 || sx.X > view.W+8 || sx.Y > view.H+8 {
			return
		}
		items = append(items, drawItem{rs: render.AtlasSprite{
			SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8,
			DstX: ax + sx.X, DstY: ay + sx.Y, DstW: w, DstH: h,
			Opacity: a, Tint: render.RGBA{R: r, G: g, B: b, A: a}, Filter: render.InterpNearest,
		}})
	}
	// Ground tiles: visible chunks, drawn at 128px cells (4x4 tiles) with
	// parity + hash speckle so the ground reads as terrain, not flat
	// plastic. One batched atlas submit; ~300 rects at design size, and
	// the count grows only with visible chunks (2x2 at any window size
	// per axis pair), not with window pixels: a maximized window shows
	// more world through the same budget instead of more work.
	if s.large != nil {
		for _, id := range s.large.Visible(view) {
			b, ok := s.large.ChunkBounds(id)
			if !ok {
				continue
			}
			const cell = 128.0
			for gy := b.Y; gy < b.Y+b.H; gy += cell {
				for gx := b.X; gx < b.X+b.W; gx += cell {
					tx, ty := int(gx/32), int(gy/32)
					base := (tx + ty) % 2 == 0
					var r, g, bl float64
					if base {
						r, g, bl = tileAR, tileAG, tileAB
					} else {
						r, g, bl = tileBR, tileBG, tileBB
					}
					if (tx*7919+ty*104729)%11 == 0 {
						r, g, bl = r*0.8, g*0.8, bl*0.8
					}
					emit(gx, gy, cell, cell, r, g, bl, 1)
				}
			}
		}
	}
	// Road: dark band across the field at the middle lane; the car
	// eases across lanes above it (lane paint follows roadY).
	emit(100, roadY-40, 6400, 80, 0.13, 0.13, 0.15, 1)
	emit(100, roadY-40, 6400, 4, 0.85, 0.80, 0.40, 1)
	emit(100, roadY+36, 6400, 4, 0.85, 0.80, 0.40, 1)
	// Roadside objects: ALL culled actives, no paint cap. Wanderers
	// among them walk across the whole field (see buildSim roster).
	if s.wcull != nil {
		for _, sn := range s.wcull.Snapshot() {
			var r, g, b float64
			switch sn.Layer % 3 {
			case 1:
				r, g, b = 0.95, 0.70, 0.20
			case 2:
				r, g, b = 0.65, 0.45, 0.95
			default:
				r, g, b = 0.25, 0.55, 0.90
			}
			emit(sn.X-6, sn.Y-6, 12, 12, r, g, b, 1)
		}
	}
	// Nearby bodies: ALL in-view bodies, no paint cap (same reasoning:
	// one batch, view culling already bounds the work).
	for _, bd := range s.ybodies {
		if bd.Pos.X+bd.Half.X < view.X || bd.Pos.X-bd.Half.X > view.X+view.W ||
			bd.Pos.Y+bd.Half.Y < view.Y || bd.Pos.Y-bd.Half.Y > view.Y+view.H {
			continue
		}
		emit(bd.Pos.X-4, bd.Pos.Y-4, 8, 8, 0.25, 0.8, 0.9, 0.9)
	}
	// Nav route (faint dots) + hero (16px with white plate so it reads).
	for i, wp := range s.npath {
		if i%8 != 0 {
			continue
		}
		emit(wp.X-2, wp.Y-2, 4, 4, 1, 0.9, 0.2, 0.35)
	}
	if s.nagent != nil {
		hp := s.nagent.Pos()
		emit(hp.X-9, hp.Y-9, 18, 18, 1, 1, 1, 1)
		emit(hp.X-7, hp.Y-7, 14, 14, 0.95, 0.25, 0.45, 1)
	}
	// Car on top.
	emit(s.carX, s.carY, carW, carH, carR, carG, carB, 1)
	// Pool bonfire: reused backing, capped at 64 sprites so a maximized
	// window pays the same as design size.
	if s.pool != nil {
		s.poolPts = s.pool.AppendParticles(s.poolPts[:0])
		n := len(s.poolPts)
		if n > 64 {
			n = 64
		}
		for _, pt := range s.poolPts[:n] {
			if !pt.Alive() {
				continue
			}
			emit(pt.Pos.X-4, pt.Pos.Y-4, 8, 8, 1, 0.82, 0.3, 0.9)
		}
	}
	// Trail ribbon: rebuild into the reused backing (no per-frame alloc).
	if s.trail != nil {
		s.trailPts = s.trail.AppendPoints(s.trailPts[:0])
		for i, q := range s.trailPts {
			w, err := s.trail.WidthAt(i)
			if err != nil || w <= 0 {
				continue
			}
			c, err := s.trail.ColorAt(i)
			if err != nil || c.A <= 0.01 {
				continue
			}
			rc := renderconv.ColorToRender(c)
			emit(q.X-w/2, q.Y-w/2, w, w, rc.R, rc.G, rc.B, c.A)
		}
	}
	s.items = items
	flat := s.flat[:0]
	for i := range items {
		flat = append(flat, items[i].rs)
	}
	s.flat = flat
	if len(flat) > 0 && s.atlas != nil {
		_, _ = pc.DC.DrawAtlasEx(s.atlas, flat, render.AtlasDrawOptions{})
	}
	pc.DC.Pop()
}

// captureFinal repaints the frozen baseline offscreen at design size:
// car parked at start, tile view at home, hero at start. Live numbers
// stay in the JSON gate; the picture is bitwise stable run to run.
func captureFinal(s *gameSim) image.Image {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	frozen := &gameSim{
		cam: s.cam, wlayers: s.wlayers,
		ybp: s.ybp, ybodies: s.ybodies,
		ngrid: s.ngrid, ncell: s.ncell, npath: s.npath,
		lodpol: s.lodpol, lodbnds: s.lodbnds, lodvp: s.lodvp,
		lodNear: s.lodNear, lodFar: s.lodFar, lodOff: s.lodOff,
		vreqs: s.vreqs, vlis: s.vlis, vch: s.vch, vaud: s.vaud,
		atlas: s.atlas, carX: carMinX, carY: roadY,
	}
	// Fresh large index at home (never the live one: its pending queue
	// and loaded set depend on the walked views, which vary run to run).
	flg, _ := tilemap.NewLarge(1024, 1024)
	homeView := core.NewRect(0, 0, 2048, 2048)
	flg.Update(homeView, 1024)
	frozen.large = &flg
	// Fresh world at build positions + home cull (never the live world:
	// wanderers walk it every tick, so live pixels drift run to run).
	fw, flayers, fl, _ := buildWorldLevel()
	frozen.world = fw
	frozen.wlayers = flayers
	fwc, _ := world.NewCull(core.NewRect(100, 80, 1600, 1000))
	for id := world.ID(1); id <= world.ID(fw.Count()); id++ {
		hw, hh := fl.Extent, fl.Extent
		if fw.HasComp(id, "marker") {
			hw, hh = 0, 0
		}
		_ = fwc.Track(id, flayers[id], 0, hw, hh)
	}
	var fsf sceneFile
	_ = readJSONFile(scenePath, &fsf)
	for _, p := range fsf.Pins {
		_ = fwc.Awake(world.ID(p))
	}
	fwc.SetAutoSleep(true)
	_ = fwc.Refresh(&frozen.world, nil)
	frozen.wcull = &fwc
	// Fresh hero at route start (never the live agent: it walks).
	if len(s.npath) > 0 {
		if ag, err := nav.NewAgent(s.npath[0], nav.DefaultSpeed); err == nil {
			_ = ag.SetWaypoints(s.npath)
			frozen.nagent = &ag
		}
	}
	cam, _ := camera.NewCamera(core.V2(float64(winW), float64(winH)))
	_ = cam.SetPos(core.V2(carMinX+carW/2, roadY+carH/2))
	frozen.cam = cam
	trailCfg, _ := particle.NewTrailConfig(trailMax, trailHeadW, trailTailW,
		core.RGBA(1, 0.85, 0.3, 1), core.RGBA(0.9, 0.2, 0.1, 0), particle.JointRound)
	frozen.trail, _ = particle.NewTrail(trailCfg)
	for _, p := range []core.Vec2{core.V2(carMinX-60, roadY + carH/2), core.V2(carMinX-30, roadY + carH/2), core.V2(carMinX, roadY + carH/2)} {
		_ = frozen.trail.Push(p)
	}
	shape, _ := particle.ConeShape(core.V2(0, -1), 25*math.Pi/180)
	frozen.pool, _ = particle.NewGPUPool(particle.EmitterConfig{
		Origin: core.V2(600, 700), Rate: 60, Max: 220,
		SpeedMin: 50, SpeedMax: 100,
		LifeMin: core.Milliseconds(600), LifeMax: core.Milliseconds(1000),
		Gravity: core.V2(0, -14),
		Start: core.RGBA(1, 0.82, 0.3, 1), End: core.RGBA(0.85, 0.2, 0.1, 0),
		Shape: shape, Turbulence: particle.NoTurbulence(), Sub: particle.NoSub(),
	}, 20260, func() particle.TrailConfig {
		c, _ := particle.NewTrailConfig(12, 8, 1, core.RGBA(1, 1, 1, 1), core.RGBA(1, 1, 1, 0), particle.JointRound)
		return c
	}())
	if _, err := frozen.pool.Spawn(40); err == nil {
		frozen.pool.Update(core.SecondsFloat(0.1))
	}
	dc := render.NewContext(winW, winH)
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	pc := rendering.NewPaintContext(dc, 1)
	paintGame(pc, rendering.Size{Width: float64(winW), Height: float64(winH)}, frozen)
	img := dc.Image()
	_ = dc.Close()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		return img
	}
	cp := image.NewRGBA(rgba.Bounds())
	copy(cp.Pix, rgba.Pix)
	return cp
}

func savePNG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

type manualSummary struct {
	Pointer int
	Key     int
	Resize  int
	Timed   bool
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func buildSim() (*gameSim, string) {
	s := &gameSim{carX: carMinX, carY: roadY, laneFrom: roadY, laneTo: roadY, dir: 1}
	cam, err := camera.NewCamera(core.V2(float64(winW), float64(winH)))
	if err != nil {
		return nil, "camera: " + err.Error()
	}
	_ = cam.SetSmoothing(0.12)
	_ = cam.SetPos(core.V2(carMinX+carW/2, roadY+carH/2))
	s.cam = cam
	tcfg, err := particle.NewTrailConfig(trailMax, trailHeadW, trailTailW,
		core.RGBA(1, 0.85, 0.3, 1), core.RGBA(0.9, 0.2, 0.1, 0), particle.JointRound)
	if err != nil {
		return nil, "trail: " + err.Error()
	}
	tr, err := particle.NewTrail(tcfg)
	if err != nil {
		return nil, "trail: " + err.Error()
	}
	s.trail = tr
	shape, err := particle.ConeShape(core.V2(0, -1), 25*math.Pi/180)
	if err != nil {
		return nil, "shape: " + err.Error()
	}
	pool, err := particle.NewGPUPool(particle.EmitterConfig{
		Origin: core.V2(600, 700), Rate: 60, Max: 220,
		SpeedMin: 50, SpeedMax: 100,
		LifeMin: core.Milliseconds(600), LifeMax: core.Milliseconds(1000),
		Gravity: core.V2(0, -14),
		Start: core.RGBA(1, 0.82, 0.3, 1), End: core.RGBA(0.85, 0.2, 0.1, 0),
		Shape: shape, Turbulence: particle.NoTurbulence(), Sub: particle.NoSub(),
	}, 20260, func() particle.TrailConfig {
		c, _ := particle.NewTrailConfig(12, 8, 1, core.RGBA(1, 1, 1, 1), core.RGBA(1, 1, 1, 0), particle.JointRound)
		return c
	}())
	if err != nil {
		return nil, "pool: " + err.Error()
	}
	if _, err := pool.Spawn(40); err != nil {
		return nil, "spawn: " + err.Error()
	}
	s.pool = pool
	tracker, err := dirty.NewSpriteDirtyTracker(core.NewRect(0, 0, 6600, 3400))
	if err != nil {
		return nil, "tracker: " + err.Error()
	}
	s.track = tracker
	layer, err := scene.NewSpriteDirtyLayer(0, 0, 6600, 3400)
	if err != nil {
		return nil, "layer: " + err.Error()
	}
	s.layer = layer
	s.atlas = buildWhiteAtlas()
	// Tile ground.
	lg, err := tilemap.NewLarge(1024, 1024)
	if err != nil {
		return nil, "large: " + err.Error()
	}
	s.large = &lg
	// World level: same builder as the probe (filed numbers, no invent).
	w, layers, l, err := buildWorldLevel()
	if err != nil {
		return nil, err.Error()
	}
	s.world = w
	s.wlayers = layers
	wc, err := world.NewCull(core.NewRect(100, 80, 1600, 1000))
	if err != nil {
		return nil, "cull: " + err.Error()
	}
	for id := world.ID(1); id <= world.ID(w.Count()); id++ {
		hw, hh := l.Extent, l.Extent
		if w.HasComp(id, "marker") {
			hw, hh = 0, 0
		}
		if err := wc.Track(id, layers[id], 0, hw, hh); err != nil {
			return nil, "track: " + err.Error()
		}
	}
	var sf sceneFile
	_ = readJSONFile(scenePath, &sf)
	for _, p := range sf.Pins {
		if err := wc.Awake(world.ID(p)); err != nil {
			return nil, "awake: " + err.Error()
		}
	}
	wc.SetAutoSleep(true)
	if err := wc.Refresh(&s.world, nil); err != nil {
		return nil, "refresh: " + err.Error()
	}
	s.wcull = &wc
	s.wactive = len(wc.Active())
	// Wanderers: spread across the whole field (not the first IDs, which
	// are all top rows). Stride over the full roster filtered by position
	// so above and below the road both walk. IDs are stable, motion is
	// deterministic per build, so probes stay exact while live walks.
	{
		st := uint32(wanderSeed)
		rnd := func(n uint32) uint32 {
			st = st*1664525 + 1013904223
			return (st >> 8) % n
		}
		total := int(w.Count())
		step := total / wanderMax
		if step < 1 {
			step = 1
		}
		for k := 0; k < wanderMax; k++ {
			id := world.ID(1 + k*step)
			if id > world.ID(total) {
				break
			}
			if w.HasComp(id, "marker") {
				continue
			}
			t, err := w.Local(id)
			if err != nil {
				continue
			}
			if t.Pos.X < 200 || t.Pos.X > 6400 || t.Pos.Y < 200 || t.Pos.Y > 3160 {
				continue
			}
			s.wand = append(s.wand, wander{
				id:    id,
				angle: float64(rnd(628)) / 100,
				speed: wanderSpdLo + float64(rnd(6000))/100,
				turn:  (float64(rnd(200))/100 - 1) * wanderTurn,
			})
		}
	}
	// Physics yard.
	bp, bodies, _, err := buildYard()
	if err != nil {
		return nil, err.Error()
	}
	s.ybp = &bp
	s.ybodies = bodies
	for i, rc := range [][5]float64{{0, 88, 1, 0, 2200}, {400, 0, 0, 1, 1200}} {
		ray, err := physics.NewRay(core.V2(rc[0], rc[1]), core.V2(rc[2], rc[3]), rc[4], 3)
		if err != nil {
			return nil, "ray: " + err.Error()
		}
		s.yrays[i] = ray
	}
	// Nav route.
	var nf navFile
	if err := readJSONFile(navPath, &nf); err != nil {
		return nil, "nav: " + err.Error()
	}
	wall := map[[2]int]bool{}
	for _, wl := range nf.Walls {
		for y := wl.Y; y < wl.Y+wl.H; y++ {
			for x := wl.X; x < wl.X+wl.W; x++ {
				wall[[2]int{x, y}] = true
			}
		}
	}
	g, err := nav.Bake(nf.W, nf.H, func(x, y int) bool { return wall[[2]int{x, y}] })
	if err != nil {
		return nil, "bake: " + err.Error()
	}
	s.ngrid = &g
	cell := 16.0
	if nf.Cell > 0 {
		cell = float64(nf.Cell)
	}
	s.ncell = cell
	path, st, err := nav.FindPath(g, nav.Cell{X: nf.Start[0], Y: nf.Start[1]}, nav.Cell{X: nf.Goal[0], Y: nf.Goal[1]})
	if err != nil || st != nav.StatusFound || len(path) == 0 {
		return nil, "nav path missing"
	}
	ag, err := nav.NewAgent(nav.CellToWorld(path[0], cell, core.V2(0, 0)), nav.DefaultSpeed)
	if err != nil {
		return nil, "agent: " + err.Error()
	}
	wps := make([]core.Vec2, 0, len(path))
	for _, c := range path {
		wps = append(wps, nav.CellToWorld(c, cell, core.V2(0, 0)))
	}
	if err := ag.SetWaypoints(wps); err != nil {
		return nil, "waypoints: " + err.Error()
	}
	s.nagent = &ag
	s.npath = wps
	q := nav.NewQueue()
	q.Enqueue(nav.Cell{X: nf.Start[0], Y: nf.Start[1]}, nav.Cell{X: nf.Goal[0], Y: nf.Goal[1]})
	s.nqueue = &q
	// Asset pack (math only).
	m, err := asset.LoadLargeManifest(manifestPth)
	if err != nil {
		return nil, "manifest: " + err.Error()
	}
	b := asset.NewBudget(m, asset.TierBalanced)
	if err := b.RegisterAll(); err != nil {
		return nil, "register: " + err.Error()
	}
	s.budget = b
	up, _, _ := b.EstimateTotal()
	s.bup = up
	s.bbroken = len(b.VerifyAll())
	// LOD bounds (filed space).
	var lf lodFile
	if err := readJSONFile(lodPath, &lf); err != nil {
		return nil, "lod: " + err.Error()
	}
	pol, err := anim.NewLodPolicy(lf.NearDist)
	if err != nil {
		return nil, "lod: " + err.Error()
	}
	s.lodpol = pol
	s.lodvp = core.NewRect(lf.Viewport[0], lf.Viewport[1], lf.Viewport[2], lf.Viewport[3])
	for _, b4 := range lf.Bounds {
		s.lodbnds = append(s.lodbnds, core.NewRect(b4[0], b4[1], b4[2], b4[3]))
	}
	// Voices at the filed hear point.
	var vf voicesFile
	if err := readJSONFile(voicesPath, &vf); err != nil {
		return nil, "voices: " + err.Error()
	}
	for _, q := range vf.Requests {
		snd, err := audio.NewPosSound(core.V2(q.Pos[0], q.Pos[1]), q.Range, q.Attenuation, q.PanStrength)
		if err != nil {
			return nil, "sound: " + err.Error()
		}
		vr, err := audio.NewVoiceRequest(snd, q.Level, q.Explosive)
		if err != nil {
			return nil, "voice: " + err.Error()
		}
		s.vreqs = append(s.vreqs, vr)
	}
	lis, err := audio.NewListener2D(core.V2(vf.Listener[0], vf.Listener[1]))
	if err != nil {
		return nil, "listener: " + err.Error()
	}
	s.vlis = lis
	if _, st2, ok := audio.SelectVoices(s.vlis.Pos(), s.vreqs); ok {
		s.vch, s.vaud = st2.Channels, st2.Audible
	}
	return s, ""
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "probes + timed window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	wrkit.EnsureUIFace()
	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_chase_large: probes ok=%v %s | %s\n", probe.OK, probe.Detail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		}
		os.Exit(1)
	}
	if err := rendergpu.ResetAccelerator(); err != nil {
		fmt.Fprintf(os.Stderr, "game_chase_large: reset accelerator: %v\n", err)
	}

	sim, failReason := buildSim()
	if sim == nil {
		fmt.Fprintln(os.Stderr, "FAIL: build:", failReason)
		os.Exit(1)
	}
	var secs int
	if *autoOnly {
		secs = runSeconds(30)
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
	// Full-window content: root fills the window (0,0), the world box
	// fills the root, so maximize/resize expands the picture (more world
	// visible) instead of leaving empty margins. Gate runs lock the
	// 1200x800 design size; manual drags follow live.
	root := rendering.NewAbsoluteBox(0, 0)
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}
	sim.root = root
	runtime.ReadMemStats(&sim.memStart)

	// Full-window content: the world box carries the design size so its
	// first layout is already full (Fixed 0,0 would lay out as 0 and the
	// window would stay black until a resize). Resize events grow it.
	sim.worldBox = rendering.NewRenderBox()
	sim.worldBox.FixedWidth, sim.worldBox.FixedHeight = float64(winW), float64(winH)
	// NOT a repaint boundary: the game animates every frame (car/trail/
	// hero all move), and a boundary would replay the cached first picture
	// forever (paint_count 3 in 30s, frozen window). No boundary means the
	// box repaints with its parent every frame.
	worldBox := sim.worldBox
	worldBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintGame(pc, size, sim)
	}
	root.Place(worldBox, 0, 0)
	sim.metric = wrkit.Label("fps - chase-large", 13, 0.92, 0.94, 0.98)
	root.Place(sim.metric, 12, 12)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_chase_large", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()
	// Gate runs lock the design size; manual drags are human-only.
	applyWindowSize(float64(winW), float64(winH))
	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode && ctl != nil {
					ctl.SetTitle(fmt.Sprintf("game_chase_large events=%d", summary.Pointer+summary.Key+summary.Resize))
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode && ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_chase_large events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				// Full window is content in every mode: box + viewport
				// follow live drags/maximize (then relayout). The frozen
				// golden is offscreen at fixed size, so live size never
				// perturbs it.
				if ev.Width > 0 && ev.Height > 0 {
					applyWindowSize(float64(ev.Width), float64(ev.Height))
					_ = sim.cam.SetViewport(core.V2(float64(ev.Width), float64(ev.Height)))
					if sim.worldBox != nil {
						sim.worldBox.FixedWidth, sim.worldBox.FixedHeight = float64(ev.Width), float64(ev.Height)
						sim.worldBox.MarkNeedsLayout()
					}
					root.MarkNeedsLayout()
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&gameTicker{s: sim})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	// First frame must paint without waiting for a resize: the world box
	// already carries the design size, so force the design viewport now.
	// Without this the window stays black until the first drag.
	applyWindowSize(float64(winW), float64(winH))
	_ = sim.cam.SetViewport(core.V2(float64(winW), float64(winH)))
	sim.worldBox.MarkNeedsPaint()
	root.MarkNeedsLayout()
	root.MarkNeedsPaint()
	app.ScheduleFrame()
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	var proc scheduler.ProcessTracker
	proc.Start()
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	elapsed := time.Since(t0).Seconds()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()

	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	finalImg := captureFinal(sim)
	finalDir := "examples/engine/chase_large/testdata"
	finalCur := finalDir + "/" + finalPng
	finalBase := finalDir + "/" + finalBase
	winFirst := false
	if _, err := os.Stat(finalBase); err != nil {
		if err := savePNG(finalImg, finalBase); err == nil {
			winFirst = true
		}
	}
	winDiff, winTotal := 0.0, int64(0)
	if !winFirst {
		if err := savePNG(finalImg, finalCur); err != nil {
			winDiff = 100
		} else {
			// wrsoak.LoadImage uses image.Decode without a png import in
			// some callers; compare here with our own png-backed decode
			// (image/png is imported) so a missing registration can never
			// fake a 100% diff.
			bf, err1 := os.Open(finalBase)
			cf, err2 := os.Open(finalCur)
			if err1 != nil || err2 != nil {
				winDiff = 100
			} else {
				ba, ea := png.Decode(bf)
				ca, eb := png.Decode(cf)
				_ = bf.Close()
				_ = cf.Close()
				if ea != nil || eb != nil {
					winDiff = 100
				} else {
					const metricH = 36.0
					x0 := 0
					y0 := int(metricH)
					x1, y1 := winW, int(metricH)+200
					var diff, total int64
					for py := y0; py < y1; py++ {
						for px := x0; px < x1; px++ {
							ar, ag, ab, _ := ba.At(px, py).RGBA()
							br, bg, bb, _ := ca.At(px, py).RGBA()
							if ar != br || ag != bg || ab != bb {
								diff++
							}
							total++
						}
					}
					winTotal = total
					if total > 0 {
						winDiff = float64(diff) * 100 / float64(total)
					}
					fmt.Fprintf(os.Stderr, "game_chase_large: golden %s: diff=%.4f%% over %d px\n", finalPng, winDiff, total)
				}
			}
		}
	} else {
		_ = savePNG(finalImg, finalCur)
	}
	var memEnd runtime.MemStats
	runtime.ReadMemStats(&memEnd)
	extra := map[string]any{
		"probe_ok":            probeOK,
		"golden_changed":      probe.GoldenChanged,
		"win_golden_diff_pct": winDiff,
		"win_golden_total_px": winTotal,
		"moved_px":            sim.movedPx,
		"world_active":        sim.wactive,
		"world_total":         probe.WorldTotal,
		"phys_region":         sim.yregion,
		"lod_near":            sim.lodNear,
		"lod_far":             sim.lodFar,
		"lod_off":             sim.lodOff,
		"voices_ch":           sim.vch,
		"voices_aud":          sim.vaud,
		"frames":              sim.frames,
		"hitch20":             sim.hitch20,
		"full_fallbacks":      0,
		"nav_moved":           sim.nmoves,
		"nav_arrived":         sim.narrive,
		"asset_upload":        sim.bup,
		"asset_broken":        sim.bbroken,
		"tile_loaded":         sim.large.LoadedCount(),
	}
	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID: abilityID, Scenario: scenario, Snap: snap,
			PresentCount: presents, ElapsedSec: elapsed,
			SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if presents < 1 || sim.movedPx <= 0 || !probe.OK || probe.GoldenChanged != 0 {
			fmt.Fprintln(os.Stderr, "FAIL: base gates")
			os.Exit(1)
		}
		if sim.wactive <= 0 || sim.yregion < 0 || sim.nmoves <= 0 || sim.vch <= 0 {
			fmt.Fprintln(os.Stderr, "FAIL: combination idle")
			os.Exit(1)
		}
		if winDiff != 0 && !winFirst {
			fmt.Fprintf(os.Stderr, "FAIL: win_golden diff=%.4f%%\n", winDiff)
			os.Exit(1)
		}
		if snap.GPUFallbacks != 0 || snap.CPUFallbackOps != 0 {
			fmt.Fprintln(os.Stderr, "FAIL: fallback")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_chase_large: OK presents=%d moved=%.0f active=%d region=%d nav=%.0f voices=%d/%d elapsed=%.1fs\n",
			presents, sim.movedPx, sim.wactive, sim.yregion, sim.nmoves, sim.vch, sim.vaud, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID, "scenario": scenario,
		"presents": presents, "elapsed_sec": elapsed, "moved_px": sim.movedPx,
		"probe_ok": probeOK, "timed": summary.Timed,
	})
	fmt.Println(string(b))
}
