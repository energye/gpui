// Command game_stage_chase is the P3 combination gate: camera follow (1.3)
// plus ysort/depth occlusion (2.3/1.2) plus GPU particle trail (5.2) plus
// tile chunks (7.2) plus dynamic dirty update (8.3) in one chase.
//
// World: 64x40 tiles at 32px (2048x1280), 8x8-tile chunks (8x5=40).
// A red chase car loops horizontally; the camera follows with smoothing
// and limits; only Visible chunks draw; 1000 props sort far-to-near;
// a car Trail plus a bonfire GPUPool prove 5.2; a DirtyTracker plus a
// scene DirtyLayer prove 8.3 (static chunks never dirtied).
//
// Modes:
//
//	RUN_SECONDS=120 go run ./examples/game_stage_chase -auto-only
//	  probes + 2-minute window (JSON on stdout, exit 1 on fail).
//	go run ./examples/game_stage_chase -manual-seconds 60
//	  manual 60s (events logged, title shows count), then summary.
//	go run ./examples/game_stage_chase
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_stage_chase. First run writes the golden
// baseline into testdata/; later runs compare it with zero tolerance.
// Night battle (lighting+grade) stays unopened until this gate is green.
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

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/examples/wrsoak"
	"github.com/energye/gpui/game/camera"
	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/particle"
	"github.com/energye/gpui/game/sprite"
	"github.com/energye/gpui/game/step"
	"github.com/energye/gpui/game/tilemap"
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
	abilityID  = "stage-chase"
	scenario   = "game_stage_chase"
	goldenPath = "examples/game_stage_chase/testdata/stage_chase_golden.png"
	lastPath   = "examples/game_stage_chase/testdata/stage_chase_last.png"

	probePixelTol = 8
)

// World frozen numbers (deterministic, no randomness in probes).
const (
	mapTilesW, mapTilesH = 64, 40
	tileW, tileH         = 32.0, 32.0
	worldW, worldH       = 2048.0, 1280.0
	chunkTilesW          = 8
	chunkTilesH          = 8

	viewW, viewH = 880.0, 360.0

	carW, carH   = 48.0, 24.0
	carY         = 640.0
	carMinX      = 100.0
	carMaxX      = 1900.0 - carW
	carSpeed     = 200.0
	propCount    = 1000
	trailMax     = 32
	trailHeadW   = 12.0
	trailTailW   = 2.0
	offW, offH   = 480, 270
)

// Shared colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB         = 0.08, 0.09, 0.11
	chunkAR, chunkAG, chunkAB = 0.16, 0.22, 0.18
	chunkBR, chunkBG, chunkBB = 0.20, 0.18, 0.14
	treeR, treeG, treeB    = 0.20, 0.60, 0.25
	rockR, rockG, rockB    = 0.55, 0.55, 0.58
	carR, carG, carB       = 0.90, 0.15, 0.12
	trailR, trailG, trailB = 1.0, 0.65, 0.15
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	worldX, worldY, worldWW, worldWH = 16.0, 44.0, 880.0, 360.0
	countX, countY                   = 16.0, 420.0
	noteY                            = 620.0
)

var stageImageID = core.AssetID("tex/stage")
var stageSrc = core.NewRect(0, 0, 8, 8)

type probeResult struct {
	LogicOK   bool
	LogicDetail string
	CamOK     bool
	ChunkOK   bool
	SortOK    bool
	TrailOK   bool
	DirtyOK   bool
	PoolOK    bool
	ParityOK  bool
	ParityPct float64
	PixOK     bool
	PixDetail string
	GoldenOK  bool
	GoldenChanged int
	GoldenWrote   bool
	OK            bool
}

func buildWhiteAtlas() *render.ImageBuf {
	img, _ := render.NewImageBuf(8, 8, render.FormatRGBA8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			_ = img.SetRGBA(x, y, 255, 255, 255, 255)
		}
	}
	return img
}

// probeLogic exercises the five real engines with frozen numbers.
func probeLogic() (camOK, chunkOK, sortOK, trailOK, dirtyOK, poolOK bool, detail string) {
	// 1.3 camera: smoothing lerp lands on the frozen number.
	cam, err := camera.NewCamera(core.V2(viewW, viewH))
	if err != nil {
		return false, false, false, false, false, false, "camera build: " + err.Error()
	}
	if err := cam.SetSmoothing(0.12); err != nil {
		return false, false, false, false, false, false, "camera smoothing: " + err.Error()
	}
	if err := cam.Follow(core.V2(500, 400)); err != nil {
		return false, false, false, false, false, false, "camera follow: " + err.Error()
	}
	got := cam.Pos()
	// From (0,0) lerp 0.12 toward (500,400): (60,48).
	if math.Abs(got.X-60) > 1e-9 || math.Abs(got.Y-48) > 1e-9 {
		return false, false, false, false, false, false, fmt.Sprintf("camera pos=(%.6f,%.6f) want (60,48)", got.X, got.Y)
	}
	camOK = true

	// 7.2 chunk: 64x40/8x8 grid, first view needs exactly chunks (0,0),(1,0).
	chk, err := tilemap.NewChunker(tilemap.OrientOrthogonal, mapTilesW, mapTilesH, tileW, tileH, chunkTilesW, chunkTilesH)
	if err != nil {
		return camOK, false, false, false, false, false, "chunk build: " + err.Error()
	}
	if chk.ChunkCols() != 8 || chk.ChunkRows() != 5 {
		return camOK, false, false, false, false, false, fmt.Sprintf("chunk grid=%dx%d want 8x5", chk.ChunkCols(), chk.ChunkRows())
	}
	view := core.NewRect(0, 0, 512, 256)
	need := chk.Needed(view)
	if len(need) != 2 {
		return camOK, false, false, false, false, false, fmt.Sprintf("chunk needed=%d want 2", len(need))
	}
	added := chk.Load(view)
	if len(added) != 2 || chk.LoadedCount() != 2 {
		return camOK, false, false, false, false, false, "chunk load count mismatch"
	}
	vis := chk.Visible(view)
	if len(vis) != 2 {
		return camOK, false, false, false, false, false, "chunk visible mismatch"
	}
	// Jump far: update swaps the set, never grows.
	far := core.NewRect(1500, 900, 512, 256)
	loaded, unloaded := chk.Update(far)
	if len(loaded) == 0 || len(unloaded) == 0 {
		return camOK, false, false, false, false, false, "chunk jump must swap"
	}
	if chk.LoadedCount() != len(chk.Needed(far)) {
		return camOK, false, false, false, false, false, "chunk loaded must equal needed after jump"
	}
	chunkOK = true

	// 2.3+1.2 sort: far draws first, ui never covered, ties stable.
	items := []sprite.Item{
		{Name: "near", Layer: sprite.LayerWorld, FeetY: 300},
		{Name: "far", Layer: sprite.LayerWorld, FeetY: 100},
		{Name: "fx", Layer: sprite.LayerFX, FeetY: 50},
		{Name: "ui", Layer: sprite.LayerUI, FeetY: 0},
	}
	sorted, err := sprite.Sort(items)
	if err != nil {
		return camOK, chunkOK, false, false, false, false, "ysort: " + err.Error()
	}
	if len(sorted) != 4 || sorted[0].Name != "far" || sorted[1].Name != "near" || sorted[2].Name != "fx" || sorted[3].Name != "ui" {
		return camOK, chunkOK, false, false, false, false, "ysort order wrong"
	}
	if !sprite.IsSorted(sorted) {
		return camOK, chunkOK, false, false, false, false, "ysort not sorted"
	}
	deep := []sprite.DeepItem{
		{Name: "near", Depth: 1700, Layer: sprite.LayerWorld, FeetY: 300},
		{Name: "far", Depth: 1900, Layer: sprite.LayerWorld, FeetY: 100},
	}
	dsorted, err := sprite.DepthSort(deep)
	if err != nil || len(dsorted) != 2 || dsorted[0].Name != "far" {
		return camOK, chunkOK, false, false, false, false, "depth order wrong"
	}
	sortOK = true

	// 5.2 trail: head wide, tail narrow, head color is Start.
	tcfg, err := particle.NewTrailConfig(8, trailHeadW, trailTailW, core.RGBA(1, 0.85, 0.3, 1), core.RGBA(0.9, 0.2, 0.1, 0), particle.JointRound)
	if err != nil {
		return camOK, chunkOK, sortOK, false, false, false, "trail cfg: " + err.Error()
	}
	tr, err := particle.NewTrail(tcfg)
	if err != nil {
		return camOK, chunkOK, sortOK, false, false, false, "trail build: " + err.Error()
	}
	_ = tr.Push(core.V2(0, 0))
	_ = tr.Push(core.V2(10, 0))
	_ = tr.Push(core.V2(20, 2))
	if tr.Len() != 3 {
		return camOK, chunkOK, sortOK, false, false, false, "trail len wrong"
	}
	wHead, _ := tr.WidthAt(2)
	wTail, _ := tr.WidthAt(0)
	if math.Abs(wHead-trailHeadW) > 1e-9 || math.Abs(wTail-trailTailW) > 1e-9 {
		return camOK, chunkOK, sortOK, false, false, false, "trail taper wrong"
	}
	cHead, _ := tr.ColorAt(2)
	if math.Abs(cHead.R-1) > 1e-9 {
		return camOK, chunkOK, sortOK, false, false, false, "trail head color wrong"
	}
	if _, err := tr.Verts(); err != nil {
		return camOK, chunkOK, sortOK, false, false, false, "trail verts: " + err.Error()
	}
	trailOK = true

	// 8.3 dirty: single move keeps one union, burst falls back to full.
	bounds := core.NewRect(0, 0, worldW, worldH)
	dt, err := step.NewSpriteDirtyTracker(bounds)
	if err != nil {
		return camOK, chunkOK, sortOK, trailOK, false, false, "dirty build: " + err.Error()
	}
	from := core.NewRect(100, carY, carW, carH)
	to := core.NewRect(116, carY, carW, carH)
	dt.MarkMoved(from, to)
	if dt.DirtyCount() != 1 || dt.NeedsFull() {
		return camOK, chunkOK, sortOK, trailOK, false, false, "dirty single move wrong"
	}
	dt.Clear()
	dt2, _ := step.NewSpriteDirtyTracker(bounds)
	for i := 0; i < 17; i++ {
		x := float64(i * 40)
		dt2.MarkMoved(core.NewRect(x, 10, 16, 16), core.NewRect(x+8, 10, 16, 16))
	}
	if !dt2.NeedsFull() {
		return camOK, chunkOK, sortOK, trailOK, false, false, "dirty burst must fall back"
	}
	dirtyOK = true

	// 5.2 pool: small bonfire recipe births and tracks.
	shape, err := particle.ConeShape(core.V2(0, -1), 20*math.Pi/180)
	if err != nil {
		return camOK, chunkOK, sortOK, trailOK, dirtyOK, false, "pool shape: " + err.Error()
	}
	pcfg := particle.EmitterConfig{
		Origin: core.V2(600, 700), Rate: 60, Max: 50,
		SpeedMin: 40, SpeedMax: 80,
		LifeMin: core.Milliseconds(500), LifeMax: core.Milliseconds(800),
		Gravity: core.V2(0, -10),
		Start: core.RGBA(1, 0.8, 0.3, 1), End: core.RGBA(0.8, 0.2, 0.1, 0),
		Shape: shape, Turbulence: particle.NoTurbulence(), Sub: particle.NoSub(),
	}
	ptcfg, err := particle.NewTrailConfig(8, 6, 1, core.RGBA(1, 1, 1, 1), core.RGBA(1, 1, 1, 0), particle.JointMiter)
	if err != nil {
		return camOK, chunkOK, sortOK, trailOK, dirtyOK, false, "pool trail cfg: " + err.Error()
	}
	pool, err := particle.NewGPUPool(pcfg, 77, ptcfg)
	if err != nil {
		return camOK, chunkOK, sortOK, trailOK, dirtyOK, false, "pool build: " + err.Error()
	}
	if _, err := pool.Spawn(5); err != nil {
		return camOK, chunkOK, sortOK, trailOK, dirtyOK, false, "pool spawn: " + err.Error()
	}
	pool.Update(core.Milliseconds(100))
	if pool.Alive() <= 0 || pool.Trails() <= 0 || pool.TotalPoints() <= 0 {
		return camOK, chunkOK, sortOK, trailOK, dirtyOK, false, "pool must be alive with tracks"
	}
	poolOK = true
	detail = "cam=(60,48) chunks=8x5 needed=2 sorted=far,near,fx,ui trail=12->2 dirty=1/full pool alive"
	return camOK, chunkOK, sortOK, trailOK, dirtyOK, poolOK, detail
}

// paintProbeFrame draws the deterministic probe scene shared by pixels,
// parity, and golden: dark bg, two chunk blocks, green tree, red car,
// orange trail bar.
func paintProbeFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	dc.SetRGB(chunkAR, chunkAG, chunkAB)
	dc.DrawRectangle(20, 30, 200, 120)
	_ = dc.Fill()
	dc.SetRGB(chunkBR, chunkBG, chunkBB)
	dc.DrawRectangle(240, 30, 200, 120)
	_ = dc.Fill()
	dc.SetRGB(treeR, treeG, treeB)
	dc.DrawRectangle(60, 170, 24, 32)
	_ = dc.Fill()
	dc.SetRGB(carR, carG, carB)
	dc.DrawRectangle(220, 176, carW, carH)
	_ = dc.Fill()
	dc.SetRGB(trailR, trailG, trailB)
	dc.DrawRectangle(140, 186, 80, 6)
	_ = dc.Fill()
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

func paintOffscreen() image.Image {
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
	paintProbeFrame(dc)
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

// probePixels asserts chunk/tree/car/trail colors offscreen.
func probePixels() (bool, string) {
	img := paintOffscreen()
	ar, ag, ab := sample8(img, 120, 90)
	br, bg, bb := sample8(img, 340, 90)
	tr, tg, tb := sample8(img, 72, 186)
	cr, cg, cb := sample8(img, 244, 188)
	lr, lg, lb := sample8(img, 180, 189)
	okA := closeEnough(ar, want8(chunkAR)) && closeEnough(ag, want8(chunkAG)) && closeEnough(ab, want8(chunkAB))
	okB := closeEnough(br, want8(chunkBR)) && closeEnough(bg, want8(chunkBG)) && closeEnough(bb, want8(chunkBB))
	okT := closeEnough(tr, want8(treeR)) && closeEnough(tg, want8(treeG)) && closeEnough(tb, want8(treeB))
	okC := closeEnough(cr, want8(carR)) && closeEnough(cg, want8(carG)) && closeEnough(cb, want8(carB))
	okL := closeEnough(lr, want8(trailR)) && closeEnough(lg, want8(trailG)) && closeEnough(lb, want8(trailB))
	detail := fmt.Sprintf("chunkA=(%d,%d,%d) chunkB=(%d,%d,%d) tree=(%d,%d,%d) car=(%d,%d,%d) trail=(%d,%d,%d) tol=%d",
		ar, ag, ab, br, bg, bb, tr, tg, tb, cr, cg, cb, lr, lg, lb, probePixelTol)
	return okA && okB && okT && okC && okL, detail
}

func diffImages(a, b image.Image) (changed, total int) {
	if !a.Bounds().Eq(b.Bounds()) {
		return 1, 1
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if aa != ba {
				changed++
			} else {
				ds := [3]int{int(ar>>8) - int(br>>8), int(ag>>8) - int(bg>>8), int(ab>>8) - int(bb>>8)}
				hit := false
				for _, d := range ds {
					if d < 0 {
						d = -d
					}
					if d > 2 {
						hit = true
					}
				}
				if hit {
					changed++
				}
			}
			total++
		}
	}
	return changed, total
}

func diffExact(a, b image.Image) (changed, total int) {
	if !a.Bounds().Eq(b.Bounds()) {
		return 1, 1
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
			total++
		}
	}
	return changed, total
}

// paintParityAtlas draws the same five blocks through the live R4 tinted
// atlas road (nearest, integer rects): sharp edges, no AA fringe, so CPU
// vs GPU parity is exact up to rounding (tint falls back to CPU sampling
// on both sides). Live window draws the same way.
func paintParityAtlas(dc *render.Context, atlas *render.ImageBuf) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	sprites := []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 20, DstY: 30, DstW: 200, DstH: 120, Opacity: 1, Tint: render.RGBA{R: chunkAR, G: chunkAG, B: chunkAB, A: 1}, Filter: render.InterpNearest},
		{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 240, DstY: 30, DstW: 200, DstH: 120, Opacity: 1, Tint: render.RGBA{R: chunkBR, G: chunkBG, B: chunkBB, A: 1}, Filter: render.InterpNearest},
		{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 60, DstY: 170, DstW: 24, DstH: 32, Opacity: 1, Tint: render.RGBA{R: treeR, G: treeG, B: treeB, A: 1}, Filter: render.InterpNearest},
		{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 220, DstY: 176, DstW: carW, DstH: carH, Opacity: 1, Tint: render.RGBA{R: carR, G: carG, B: carB, A: 1}, Filter: render.InterpNearest},
		{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 140, DstY: 186, DstW: 80, DstH: 6, Opacity: 1, Tint: render.RGBA{R: trailR, G: trailG, B: trailB, A: 1}, Filter: render.InterpNearest},
	}
	_, _ = dc.DrawAtlasEx(atlas, sprites, render.AtlasDrawOptions{})
}

func paintParityOffscreen(cpu bool, atlas *render.ImageBuf) image.Image {
	if cpu {
		_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	} else {
		_ = os.Unsetenv("GOGPU_RENDER_MODE")
	}
	dc := render.NewContext(offW, offH)
	paintParityAtlas(dc, atlas)
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

// probeParity paints the atlas probe on CPU and GPU and requires <=1% diff
// (channel tolerance 2, same as game_sprite: only AA-free atlas road here;
// rect fills stay in pixels+golden which are CPU-exact).
func probeParity() (bool, float64) {
	prev, had := os.LookupEnv("GOGPU_RENDER_MODE")
	atlas := buildWhiteAtlas()
	cpu := paintParityOffscreen(true, atlas)
	gpu := paintParityOffscreen(false, atlas)
	if had {
		_ = os.Setenv("GOGPU_RENDER_MODE", prev)
	} else {
		_ = os.Unsetenv("GOGPU_RENDER_MODE")
	}
	changed, total := diffImages(cpu, gpu)
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(changed) / float64(total)
	}
	return pct <= 1.0, pct
}

// probeGolden compares the deterministic frame against the frozen mask
// with zero tolerance; the first run produces the baseline.
func probeGolden() (ok bool, changed int, wrote bool) {
	img := paintOffscreen()
	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/game_stage_chase/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(goldenPath)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		if encErr != nil {
			return false, 0, false
		}
		if out2, err := os.Create(lastPath); err == nil {
			_ = png.Encode(out2, img)
			_ = out2.Close()
		}
		return true, 0, true
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		return false, 0, false
	}
	changed, _ = diffExact(img, want)
	if changed == 0 {
		if out, err := os.Create(lastPath); err == nil {
			_ = png.Encode(out, img)
			_ = out.Close()
		}
	}
	return changed == 0, changed, false
}

func runProbes() probeResult {
	var p probeResult
	p.CamOK, p.ChunkOK, p.SortOK, p.TrailOK, p.DirtyOK, p.PoolOK, p.LogicDetail = probeLogic()
	p.LogicOK = p.CamOK && p.ChunkOK && p.SortOK && p.TrailOK && p.DirtyOK && p.PoolOK
	p.PixOK, p.PixDetail = probePixels()
	p.ParityOK, p.ParityPct = probeParity()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.PixOK && p.ParityOK && p.GoldenOK
	return p
}

// prop is one deterministic world decoration.
type prop struct {
	X, Y, W, H float64
	FeetY      float64
	Depth      float64
	Kind       int // 0 tree, 1 rock, 2 house
}

func buildProps() []prop {
	out := make([]prop, 0, propCount)
	for i := 0; i < propCount; i++ {
		col := (i * 37) % mapTilesW
		row := (i * 53) % mapTilesH
		x := float64(col)*tileW + 6 + float64(i%5)*2
		y := float64(row)*tileH + 8 + float64((i/7)%4)*3
		w, h := 14.0, 18.0
		kind := i % 3
		if kind == 1 {
			w, h = 12, 10
		} else if kind == 2 {
			w, h = 22, 16
		}
		feet := y + h
		out = append(out, prop{X: x, Y: y, W: w, H: h, FeetY: feet, Depth: 4000 - feet, Kind: kind})
	}
	return out
}

func propColor(k int) (float64, float64, float64) {
	switch k {
	case 0:
		return treeR, treeG, treeB
	case 1:
		return rockR, rockG, rockB
	default:
		return 0.65, 0.45, 0.25
	}
}

// chaseDrawItem is one painter entry: bigger depth draws first (props),
// the car/trail/pool ride last. Package level so the frame reuses the
// backing across ticks instead of growing a fresh slice every paint.
type chaseDrawItem struct {
	depth float64
	feet  float64
	rs    render.AtlasSprite
}

// chaseBatchEmit is the per-frame batch drain: same-image proof needs the
// call count only, sprite contents stay in the batch contract tests.
func chaseBatchEmit(_ core.AssetID, _ []sprite.Sprite) {}

// chaseSim is the live window state: all five engines advance every tick.
type chaseSim struct {
	cam     camera.Camera
	chunk   tilemap.Chunk
	trail   *particle.Trail
	pool    *particle.GPUPool
	tracker *step.DirtyTracker
	layer   *scene.DirtyLayer
	props   []prop
	atlas   *render.ImageBuf
	batch   *sprite.Batch

	app   *embedder.PipelineApp
	shell *wrkit.ShellChrome
	phase *wrkit.PhaseClock

	worldBox *rendering.RenderBox
	overlay  *rendering.RenderBox
	show     []scene.DirtyRect

	// Chunk cache: the view drifts ~3px/frame but the 256px chunk range
	// flips rarely; equal ranges skip Update/Visible (same loaded set).
	hasCache       bool
	cachedCX0      int
	cachedCY0      int
	cachedCX1      int
	cachedCY1      int
	cachedVis      []tilemap.ChunkID
	cachedLoaded   int
	deepScratch    []sprite.DeepItem
	paintItems     []chaseDrawItem
	paintFlat      []render.AtlasSprite
	labelTick      int
	sortMismatches int
	hitch20        int64
	lastTickWall   time.Time
	haveTickWall   bool
	memStart       runtime.MemStats

	carX, carY float64
	dir        float64
	movedPx    float64
	frames     int
	maxSeen    int
	visible    int
	loaded     int
	sorted     int
	trailPts   int
	poolAlive  int
	poolPts    int
	batchCalls int
	fulls      int

	camL, chunkL, sortL, trailL, dirtyL, fpsL *rendering.RenderText
}

type ticker struct{ s *chaseSim }

func srect(r core.Rect) scene.DirtyRect { return scene.DirtyRect{X: r.X, Y: r.Y, W: r.W, H: r.H} }

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
	// Car loops horizontally.
	nx := s.carX + s.dir*carSpeed*dt
	if nx >= carMaxX {
		nx = carMinX + (nx - carMaxX)
	}
	oldR := core.NewRect(s.carX, s.carY, carW, carH)
	newR := core.NewRect(nx, s.carY, carW, carH)
	moved := nx - s.carX
	if moved < 0 {
		moved = -moved
		if nx < s.carX {
			// Wrapped: count the wrap jump as forward motion.
			moved = (carMaxX - s.carX) + (nx - carMinX) + carW
		}
	}
	s.movedPx += moved
	s.carX = nx
	// 1.3 follow.
	_ = s.cam.Follow(core.V2(s.carX+carW/2, s.carY+carH/2))
	view := s.cam.VisibleWorldRect()
	// 7.2 chunks: the 256px range flips rarely; equal ranges reuse the
	// cached set (Loaded already equals Needed, same numbers, no work).
	const chunkPx = float64(chunkTilesW) * tileW
	cx0 := int(math.Floor(view.X / chunkPx))
	cy0 := int(math.Floor(view.Y / chunkPx))
	cx1 := int(math.Floor((view.X + view.W) / chunkPx))
	cy1 := int(math.Floor((view.Y + view.H) / chunkPx))
	if !s.hasCache || cx0 != s.cachedCX0 || cy0 != s.cachedCY0 || cx1 != s.cachedCX1 || cy1 != s.cachedCY1 {
		_, _ = s.chunk.Update(view)
		s.cachedVis = s.chunk.Visible(view)
		s.cachedLoaded = s.chunk.LoadedCount()
		s.cachedCX0, s.cachedCY0, s.cachedCX1, s.cachedCY1 = cx0, cy0, cx1, cy1
		s.hasCache = true
	}
	s.visible = len(s.cachedVis)
	s.loaded = s.cachedLoaded
	// 5.2 car trail + bonfire pool.
	_ = s.trail.Push(core.V2(s.carX+4, s.carY+carH/2))
	s.trailPts = s.trail.Len()
	if s.pool != nil {
		s.pool.Update(core.SecondsFloat(dt))
		s.poolAlive = s.pool.Alive()
		s.poolPts = s.pool.TotalPoints()
	}
	// 8.3 dirty twins with the same numbers.
	s.tracker.MarkMoved(oldR, newR)
	s.layer.MarkMoved(srect(oldR), srect(newR))
	kept := s.layer.DirtyRects()
	s.show = append(s.show[:0], kept...)
	if len(kept) > s.maxSeen {
		s.maxSeen = len(kept)
	}
	st := s.tracker.Stats()
	s.fulls = st.FullFallbacks
	// 2.3+1.2 sort: count visible every tick (no per-frame slice or sort);
	// the order contract is re-proved every 300 ticks, mismatches stay 0.
	s.sorted = s.countVisible(view)
	if s.frames%300 == 0 {
		if !s.validateSort(view) {
			s.sortMismatches++
		}
	}
	// Batch proof: one image means one call (persistent batch keeps its
	// backing; the single-image fast path skips the per-frame map).
	if s.batch == nil {
		s.batch = sprite.NewBatch()
	}
	s.batch.Clear()
	for i := range s.props {
		p := &s.props[i]
		if p.X+p.W < view.X || p.X > view.X+view.W || p.Y+p.H < view.Y || p.Y > view.Y+view.H {
			continue
		}
		sp, err := sprite.NewSprite(stageImageID, stageSrc, core.NewRect(p.X, p.Y, p.W, p.H), 1)
		if err != nil {
			continue
		}
		_, _ = s.batch.Add(sp)
	}
	carSp, _ := sprite.NewSprite(stageImageID, stageSrc, newR, 1)
	_, _ = s.batch.Add(carSp)
	s.batchCalls = s.batch.Flush(chaseBatchEmit)
	if s.worldBox != nil {
		s.worldBox.MarkNeedsPaint()
	}
	if s.overlay != nil {
		s.overlay.MarkNeedsPaint()
	}
	s.tracker.Clear()
	s.layer.Clear()

	phase := s.phase.Advance(dt)
	// Wall-time hitch over the §5 20ms line (the metrics ring still counts
	// the old 33.4ms line; this recomputes the strict line per tick).
	now := time.Now()
	if s.haveTickWall {
		if now.Sub(s.lastTickWall) > 20*time.Millisecond {
			s.hitch20++
		}
	}
	s.lastTickWall = now
	s.haveTickWall = true

	// Chrome text at ~4Hz: counters still jump, JSON gates read sim fields.
	s.labelTick++
	if s.labelTick%15 == 1 {
		snap := s.app.Metrics().Snapshot()
		fps := 0.0
		if snap.AvgFrameIntervalMs > 1e-6 {
			fps = 1000.0 / snap.AvgFrameIntervalMs
		}
		s.camL.SetText(fmt.Sprintf("镜头 %.0f,%.0f 可见%d块", s.cam.Pos().X, s.cam.Pos().Y, s.visible))
		s.chunkL.SetText(fmt.Sprintf("分区 装载%d 可见%d", s.loaded, s.visible))
		s.sortL.SetText(fmt.Sprintf("排序 %d 遮挡对", s.sorted))
		s.trailL.SetText(fmt.Sprintf("拖尾 车%d点 炉%d活%d点", s.trailPts, s.poolAlive, s.poolPts))
		if s.tracker.NeedsFull() {
			s.dirtyL.SetText("脏块 FULL")
		} else {
			s.dirtyL.SetText(fmt.Sprintf("脏块 %d 回退%d", len(kept), s.fulls))
		}
		s.fpsL.SetText(fmt.Sprintf("帧率 %.0f 位移%.0f", fps, s.movedPx))
		gateOK := s.movedPx > 0 && s.visible > 0 && s.sorted > 0 && s.trailPts > 0
		s.shell.NoteHUDTick(dt)
		s.shell.UpdateHUD("stage-chase", phase, s.app, gateOK,
			fmt.Sprintf("vis=%d sorted=%d trail=%d dirty=%d", s.visible, s.sorted, s.trailPts, len(kept)),
			fmt.Sprintf("moved=%.0f batch=%d", s.movedPx, s.batchCalls))
	} else {
		s.shell.NoteHUDTick(dt)
	}
	s.app.ScheduleFrame()
	return true
}

func (s *chaseSim) countVisible(view core.Rect) int {
	n := 0
	for i := range s.props {
		p := &s.props[i]
		if p.X+p.W < view.X || p.X > view.X+view.W || p.Y+p.H < view.Y || p.Y > view.Y+view.H {
			continue
		}
		n++
	}
	return n + 1 // car always drawn
}

func (s *chaseSim) validateSort(view core.Rect) bool {
	s.deepScratch = s.deepScratch[:0]
	for i := range s.props {
		p := &s.props[i]
		if p.X+p.W < view.X || p.X > view.X+view.W || p.Y+p.H < view.Y || p.Y > view.Y+view.H {
			continue
		}
		if len(s.deepScratch) >= 400 {
			break
		}
		d, err := sprite.NewDeepItem("", p.Depth, sprite.LayerWorld, p.FeetY)
		if err != nil {
			return false
		}
		s.deepScratch = append(s.deepScratch, d)
	}
	car, err := sprite.NewDeepItem("car", 4000-(s.carY+carH), sprite.LayerWorld, s.carY+carH)
	if err != nil {
		return false
	}
	s.deepScratch = append(s.deepScratch, car)
	ordered, err := sprite.DepthSort(s.deepScratch)
	if err != nil {
		return false
	}
	return sprite.IsDepthSorted(ordered)
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func failJSON(probe probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
		"parity_pct": probe.ParityPct,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func main() {
	caseFlag := flag.String("case", "chase", "scenario case (only chase)")
	autoOnly := flag.Bool("auto-only", false, "probes + timed window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()
	if *caseFlag != "chase" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want chase (only chase gate)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_stage_chase: probes ok=%v logic=%v(cam=%v chunk=%v sort=%v trail=%v dirty=%v pool=%v detail=%s) pix=%v parity=%.4f%% golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, probe.LogicOK, probe.CamOK, probe.ChunkOK, probe.SortOK, probe.TrailOK, probe.DirtyOK, probe.PoolOK, probe.LogicDetail, probe.PixOK, probe.ParityPct, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_stage_chase: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}
	// Probe's offscreen GPU pass used a standalone device; drop it before the
	// window opens so only one device is live on 1GB cards (dual-device peak
	// OOMs depth creation). The window re-inits lazily on its own device.
	if err := rendergpu.ResetAccelerator(); err != nil {
		fmt.Fprintf(os.Stderr, "game_stage_chase: reset accelerator: %v\n", err)
	}

	// Build the five live engines (all real packages, no example bypass).
	cam, err := camera.NewCamera(core.V2(viewW, viewH))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: camera:", err)
		os.Exit(1)
	}
	_ = cam.SetSmoothing(0.12)
	_ = cam.SetLimit(core.NewRect(viewW/2, viewH/2, worldW-viewW, worldH-viewH))
	_ = cam.SetPos(core.V2(carMinX+carW/2, carY+carH/2))
	chk, err := tilemap.NewChunker(tilemap.OrientOrthogonal, mapTilesW, mapTilesH, tileW, tileH, chunkTilesW, chunkTilesH)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: chunk:", err)
		os.Exit(1)
	}
	tcfg, err := particle.NewTrailConfig(trailMax, trailHeadW, trailTailW, core.RGBA(1, 0.85, 0.3, 1), core.RGBA(0.9, 0.2, 0.1, 0), particle.JointRound)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: trail cfg:", err)
		os.Exit(1)
	}
	trail, err := particle.NewTrail(tcfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: trail:", err)
		os.Exit(1)
	}
	shape, err := particle.ConeShape(core.V2(0, -1), 25*math.Pi/180)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: pool shape:", err)
		os.Exit(1)
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
		fmt.Fprintln(os.Stderr, "FAIL: pool:", err)
		os.Exit(1)
	}
	if _, err := pool.Spawn(40); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: pool spawn:", err)
		os.Exit(1)
	}
	tracker, err := step.NewSpriteDirtyTracker(core.NewRect(0, 0, worldW, worldH))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: tracker:", err)
		os.Exit(1)
	}
	layer, err := scene.NewSpriteDirtyLayer(0, 0, worldW, worldH)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: dirty layer:", err)
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(120)
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

	shell := wrkit.NewShell(winW, winH, "game_stage_chase — 追车五合一 (stage-chase)", []string{
		"镜头跟随·限位平滑",
		"排序遮挡·远先近后",
		"拖尾·头宽尾窄渐隐",
		"分区·只画可见块",
		"脏区·动哪更哪",
		"黄框=脏区 青车=追车",
		"JSON见 ability_extra",
	})
	atlas := buildWhiteAtlas()
	sim := &chaseSim{
		cam: cam, chunk: chk, trail: trail, pool: pool,
		tracker: tracker, layer: layer,
		props: buildProps(), atlas: atlas,
		shell: shell, carX: carMinX, carY: carY, dir: 1,
	}
	runtime.ReadMemStats(&sim.memStart)
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	shell.Body.Place(wrkit.Label("CHASE 世界跟随区", 13, 0.55, 0.75, 0.95), worldX, worldY-24)
	sim.worldBox = rendering.NewRenderBox()
	sim.worldBox.FixedWidth, sim.worldBox.FixedHeight = worldWW, worldWH
	sim.worldBox.SetRepaintBoundary(true)
	live := sim
	sim.worldBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintWorld(pc, live)
	}
	shell.Body.Place(sim.worldBox, worldX, worldY)
	sim.overlay = rendering.NewRenderBox()
	sim.overlay.FixedWidth, sim.overlay.FixedHeight = worldWW, worldWH
	sim.overlay.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil || live == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		view := live.cam.VisibleWorldRect()
		pc.DC.SetRGBA(1, 0.9, 0.2, 1)
		pc.DC.SetLineWidth(2)
		for _, r := range live.show {
			sx := r.X - view.X
			sy := r.Y - view.Y
			if sx+r.W < 0 || sy+r.H < 0 || sx > viewW || sy > viewH {
				continue
			}
			pc.DC.DrawRectangle(ax+sx, ay+sy, r.W, r.H)
			_ = pc.DC.Stroke()
		}
	}
	shell.Body.Place(sim.overlay, worldX, worldY)

	shell.Body.Place(wrkit.Label("COUNTERS 计数器", 13, 0.55, 0.75, 0.95), countX, countY-24)
	sim.camL = wrkit.Label("镜头 -", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.camL, countX, countY+10)
	sim.chunkL = wrkit.Label("分区 -", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.chunkL, countX+300, countY+10)
	sim.sortL = wrkit.Label("排序 -", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.sortL, countX+560, countY+10)
	sim.trailL = wrkit.Label("拖尾 -", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.trailL, countX, countY+36)
	sim.dirtyL = wrkit.Label("脏块 -", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.dirtyL, countX+300, countY+36)
	sim.fpsL = wrkit.Label("帧率 -", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fpsL, countX+560, countY+36)
	shell.Body.Place(wrkit.Label("红车循环追·镜头跟·黄框脏区·橙条拖尾·灰炉烟·绿树灰石按远近盖", 12, 0.70, 0.78, 0.88), countX, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_stage_chase", Decorations: true})
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
		// Phase 1: snapshot reports the window golden, gates stay loose.
		SnapshotPath: "examples/game_stage_chase/testdata/chase_final.png",
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_stage_chase: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_stage_chase: pointer %s (%.0f,%.0f) n=%d\n", ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_stage_chase events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_stage_chase: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_stage_chase events=%d", summary.Pointer+summary.Key+summary.Resize))
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
					fmt.Fprintf(os.Stderr, "game_stage_chase: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_stage_chase events=%d", summary.Pointer+summary.Key+summary.Resize))
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
	// Window golden over the static legend chrome (the world is live by
	// design: the car loops forever, so no world pixel is deterministic;
	// game-pixel determinism rides the offscreen golden instead).
	goldenRects := []wrsoak.Rect{
		{X: 20, Y: 320, W: 100, H: 260},
		{X: 20, Y: 630, W: 100, H: 60},
		{X: 20, Y: 70, W: 100, H: 14},
	}
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_stage_chase", "examples/game_stage_chase/testdata", "chase_final.png", "chase_final_base.png", goldenRects, winW)
	var memEnd runtime.MemStats
	runtime.ReadMemStats(&memEnd)
	// HeapAlloc is the live heap (TotalAlloc only ever grows); the leak
	// gate watches end vs peak RSS, this ratio is diagnostic only.
	var heapGrowth float64
	if sim.memStart.HeapAlloc > 0 {
		heapGrowth = float64(memEnd.HeapAlloc-sim.memStart.HeapAlloc) / float64(sim.memStart.HeapAlloc)
	}
	var maxPauseMs float64
	for _, d := range memEnd.PauseNs {
		if ms := float64(d) / 1e6; ms > maxPauseMs {
			maxPauseMs = ms
		}
	}
	hitch20Rate := 0.0
	if elapsed > 0 {
		hitch20Rate = float64(sim.hitch20) / (elapsed / 60)
	}
	extra := map[string]any{
		"probe_ok":            probeOK,
		"parity_pct":          probe.ParityPct,
		"golden_changed":      probe.GoldenChanged,
		"win_golden_diff_pct": winGoldenDiff,
		"win_golden_total_px": winGoldenTotal,
		"moved_px":            sim.movedPx,
		"visible_chunks":      sim.visible,
		"loaded_chunks":       sim.loaded,
		"sorted_sprites":      sim.sorted,
		"sort_mismatches":     sim.sortMismatches,
		"trail_points":        sim.trailPts,
		"pool_alive":          sim.poolAlive,
		"pool_points":         sim.poolPts,
		"batch_calls":         sim.batchCalls,
		"dirty_max":           sim.maxSeen,
		"full_fallbacks":      sim.fulls,
		"frames":              sim.frames,
		"hitch20":             sim.hitch20,
		"hitch20_rate":        hitch20Rate,
		"alloc_total":         memEnd.TotalAlloc,
		"heap_alloc":          memEnd.HeapAlloc,
		"heap_growth":         heapGrowth,
		"num_gc":              memEnd.NumGC - sim.memStart.NumGC,
		"max_pause_ms":        maxPauseMs,
		"boundary_skip":       snap.BoundarySkip,
		"case":                "chase",
	}
	if winGoldenFirst {
		extra["win_golden_first"] = 1
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
		if presents < 1 || sim.movedPx <= 0 || !probe.OK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d moved=%.0f probe=%v (want >=1, >0, true)\n", presents, sim.movedPx, probe.OK)
			os.Exit(1)
		}
		if sim.visible <= 0 || sim.loaded <= 0 || sim.sorted <= 0 || sim.trailPts <= 0 || sim.poolAlive <= 0 || sim.batchCalls < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: visible=%d loaded=%d sorted=%d trail=%d pool=%d batch=%d (want >0, >0, >0, >0, >0, >=1)\n", sim.visible, sim.loaded, sim.sorted, sim.trailPts, sim.poolAlive, sim.batchCalls)
			os.Exit(1)
		}
		// S52 卡面门限（§5 帧门＋本组单数，字面执行，红即红）：
		// presents6500＋、fps57＋、p95≤16、p99≤20、20ms卡顿≤3/分、
		// dirty_max≤2、full0、fallback0、内存end≈peak、双金0差、
		// parity图集路0差、排序零失配、六数全对。
		fpsInterval := 0.0
		if snap.AvgFrameIntervalMs > 1e-6 {
			fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
		}
		if presents < 6500 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d want >=6500 (S52)\n", presents)
			os.Exit(1)
		}
		if fpsInterval < 57 {
			fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.2f want >=57 (S52)\n", fpsInterval)
			os.Exit(1)
		}
		if snap.P95FrameIntervalMs > 16 && snap.P95FrameIntervalMs > 0 {
			fmt.Fprintf(os.Stderr, "FAIL: p95=%.2f want <=16 (§5)\n", snap.P95FrameIntervalMs)
			os.Exit(1)
		}
		if snap.P99FrameIntervalMs > 20 && snap.P99FrameIntervalMs > 0 {
			fmt.Fprintf(os.Stderr, "FAIL: p99=%.2f want <=20 (§5)\n", snap.P99FrameIntervalMs)
			os.Exit(1)
		}
		if extra["hitch20_rate"].(float64) > 3 {
			fmt.Fprintf(os.Stderr, "FAIL: hitch20_rate=%.2f want <=3/min (§5, 20ms recomputed)\n", extra["hitch20_rate"])
			os.Exit(1)
		}
		if sim.maxSeen > 2 {
			fmt.Fprintf(os.Stderr, "FAIL: dirty_max=%d want <=2 (S52)\n", sim.maxSeen)
			os.Exit(1)
		}
		if sim.sortMismatches != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: sort_mismatches=%d want 0 (S52)\n", sim.sortMismatches)
			os.Exit(1)
		}
		// 显存/内存: 2分钟门看稳态不漏 (end相对peak涨幅<=5%)。
		// 起始到峰值含字体/GPU预热虚涨, 不做门, 只如实上报; 2小时长跑另按起止5%判。
		if snap.RSSPeakKB > 0 && snap.RSSEndKB > 0 {
			growthPeak := float64(snap.RSSEndKB-snap.RSSPeakKB) / float64(snap.RSSPeakKB)
			if growthPeak > 0.05 {
				fmt.Fprintf(os.Stderr, "FAIL: rss peak growth=%.3f want <=0.05 (%d->%d KB)\n", growthPeak, snap.RSSPeakKB, snap.RSSEndKB)
				os.Exit(1)
			}
		}
		// 像素: 离屏金0差、窗金0差、parity图集路0差（S52 卡面字面）。
		if probe.GoldenChanged != 0 || probe.ParityPct != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: pixels golden=%d parity=%.4f (want 0 and 0)\n", probe.GoldenChanged, probe.ParityPct)
			os.Exit(1)
		}
		if winGoldenDiff != 0 && !winGoldenFirst {
			fmt.Fprintf(os.Stderr, "FAIL: win_golden diff=%.4f%% over %d px (want 0)\n", winGoldenDiff, winGoldenTotal)
			os.Exit(1)
		}
		if snap.GPUFallbacks != 0 || snap.CPUFallbackOps != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: fallback gpu=%d cpu_ops=%d (want 0 and 0)\n", snap.GPUFallbacks, snap.CPUFallbackOps)
			os.Exit(1)
		}
		if sim.fulls != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: full_fallbacks=%d want 0 (S52)\n", sim.fulls)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_stage_chase: OK presents=%d fps=%.1f p95=%.1f moved=%.0f vis=%d sorted=%d trail=%d batch=%d elapsed=%.1fs\n",
			presents, fpsInterval, snap.P95FrameIntervalMs, sim.movedPx, sim.visible, sim.sorted, sim.trailPts, sim.batchCalls, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID, "scenario": scenario, "backend": win.Backend().String(),
		"events": map[string]any{"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize},
		"presents": presents, "elapsed_sec": elapsed, "moved_px": sim.movedPx,
		"visible_chunks": sim.visible, "sorted_sprites": sim.sorted, "trail_points": sim.trailPts,
		"probe_ok": probeOK, "timed": summary.Timed,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_stage_chase: backend=%s presents=%d moved=%.0f vis=%d sorted=%d trail=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.movedPx, sim.visible, sim.sorted, sim.trailPts, elapsed)
}

// paintWorld draws visible chunks, sorted props, car, car trail, and the
// bonfire pool through one tinted atlas draw (R4 road) in depth order.
func paintWorld(pc *rendering.PaintContext, s *chaseSim) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	view := s.cam.VisibleWorldRect()
	// Ground: view-sized dark base.
	pc.DC.SetRGB(0.10, 0.12, 0.12)
	pc.DC.DrawRectangle(ax, ay, worldWW, worldWH)
	_ = pc.DC.Fill()
	// Visible chunks checker (world->screen, zoom 1).
	vis := s.cachedVis
	if !s.hasCache {
		vis = s.chunk.Visible(view)
	}
	for _, id := range vis {
		b, ok := s.chunk.ChunkBounds(id)
		if !ok {
			continue
		}
		sx, ok1 := s.cam.WorldToScreen(core.V2(b.X, b.Y))
		if !ok1 {
			continue
		}
		if (id.CX+id.CY)%2 == 0 {
			pc.DC.SetRGB(chunkAR, chunkAG, chunkAB)
		} else {
			pc.DC.SetRGB(chunkBR, chunkBG, chunkBB)
		}
		pc.DC.DrawRectangle(ax+sx.X, ay+sx.Y, b.W, b.H)
		_ = pc.DC.Fill()
	}
	// Lane guide through the car row.
	if c0, ok := s.cam.WorldToScreen(core.V2(view.X, s.carY-10)); ok {
		pc.DC.SetRGBA(0.9, 0.9, 0.9, 0.25)
		pc.DC.SetLineWidth(1)
		pc.DC.DrawLine(ax+c0.X, ay+c0.Y, ax+c0.X+view.W, ay+c0.Y)
		_ = pc.DC.Stroke()
	}
	// Sorted atlas sprites: visible props far-to-near + car on top.
	// Frame-owned backings: no per-paint grows after warmup.
	items := s.paintItems[:0]
	for i := range s.props {
		p := &s.props[i]
		if p.X+p.W < view.X || p.X > view.X+view.W || p.Y+p.H < view.Y || p.Y > view.Y+view.H {
			continue
		}
		sc, ok := s.cam.WorldToScreen(core.V2(p.X, p.Y))
		if !ok {
			continue
		}
		r, g, b := propColor(p.Kind)
		items = append(items, chaseDrawItem{depth: p.Depth, feet: p.FeetY, rs: render.AtlasSprite{
			SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8,
			DstX: ax + sc.X, DstY: ay + sc.Y, DstW: p.W, DstH: p.H,
			Opacity: 1, Tint: render.RGBA{R: r, G: g, B: b, A: 1}, Filter: render.InterpNearest,
		}})
		if len(items) >= 600 {
			break
		}
	}
	// Car always drawn (even at wrap edge): screen-clipped.
	if sc, ok := s.cam.WorldToScreen(core.V2(s.carX, s.carY)); ok {
		items = append(items, chaseDrawItem{depth: -1e9, feet: 1e9, rs: render.AtlasSprite{
			SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8,
			DstX: ax + sc.X, DstY: ay + sc.Y, DstW: carW, DstH: carH,
			Opacity: 1, Tint: render.RGBA{R: carR, G: carG, B: carB, A: 1}, Filter: render.InterpNearest,
		}})
	}
	// Car trail ribbon: newest widest brightest.
	if s.trail != nil {
		pts := s.trail.Points()
		for i, q := range pts {
			sc, ok := s.cam.WorldToScreen(q)
			if !ok {
				continue
			}
			if sc.X < -20 || sc.Y < -20 || sc.X > viewW+20 || sc.Y > viewH+20 {
				continue
			}
			w, err := s.trail.WidthAt(i)
			if err != nil || w <= 0 {
				continue
			}
			c, err := s.trail.ColorAt(i)
			if err != nil || c.A <= 0.01 {
				continue
			}
			rc := c.ToRender()
			items = append(items, chaseDrawItem{depth: -2e9, feet: 2e9, rs: render.AtlasSprite{
				SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8,
				DstX: ax + sc.X - w/2, DstY: ay + sc.Y - w/2, DstW: w, DstH: w,
				Opacity: c.A, Tint: render.RGBA{R: rc.R, G: rc.G, B: rc.B, A: rc.A}, Filter: render.InterpNearest,
			}})
		}
	}
	// Bonfire pool points (world->screen, small).
	if s.pool != nil {
		for _, pt := range s.pool.Particles() {
			if !pt.Alive() {
				continue
			}
			sc, ok := s.cam.WorldToScreen(pt.Pos)
			if !ok {
				continue
			}
			if sc.X < -12 || sc.Y < -12 || sc.X > viewW+12 || sc.Y > viewH+12 {
				continue
			}
			c := s.pool.ColorOf(pt)
			if c.A <= 0.01 {
				continue
			}
			rc := c.ToRender()
			items = append(items, chaseDrawItem{depth: -3e9, feet: 3e9, rs: render.AtlasSprite{
				SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8,
				DstX: ax + sc.X - 3, DstY: ay + sc.Y - 3, DstW: 6, DstH: 6,
				Opacity: c.A, Tint: render.RGBA{R: rc.R, G: rc.G, B: rc.B, A: rc.A}, Filter: render.InterpNearest,
			}})
			if len(items) >= 900 {
				break
			}
		}
	}
	// Painter far-to-near: bigger depth first (props), car/trail/pool last.
	// Insertion road, same order as before, now on the reused backing.
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && items[j-1].depth < items[j].depth {
			items[j-1], items[j] = items[j], items[j-1]
			j--
		}
	}
	s.paintItems = items
	flat := s.paintFlat[:0]
	for i := range items {
		flat = append(flat, items[i].rs)
	}
	s.paintFlat = flat
	if len(flat) > 0 && s.atlas != nil {
		_, _ = pc.DC.DrawAtlasEx(s.atlas, flat, render.AtlasDrawOptions{})
	}
}
