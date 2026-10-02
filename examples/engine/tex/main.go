// Command game_tex is the 3.2/3.3 texture dual-case independent window.
//
// Far view (mipmap switch) and background streaming (stream ledger) share
// one window, each case driving the real engine/tex package read-only:
//
//	go run ./examples/engine/tex --case=far -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/tex --case=stream -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/tex --case=far -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/engine/tex
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_tex. First run writes the per-case golden
// baseline into testdata/; later runs compare it with zero tolerance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"math"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/tex"
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
	abilityID  = "tex"

	frozenMipmapPath = "engine/tex/testdata/mipmap_cases.json"
	frozenStreamPath = "engine/tex/testdata/stream_cases.json"
	texDataDir       = "engine/tex/testdata"

	goldenFarPath    = "examples/engine/tex/testdata/tex_far_golden.png"
	goldenStreamPath = "examples/engine/tex/testdata/tex_stream_golden.png"

	// Gate ceilings baked into -auto-only (see README for the numbers).
	// Foreground Poll must stay microsecond-scale while workers decode;
	// one big picture must stream in the millisecond magnitude.
	maxPollTotalMs = 100
	maxBigBgMs     = 50

	// sweepRate moves the far-view scale 1.0 -> 0.3 -> 1.0 so an 8s auto
	// run crosses at least one mipmap level step.
	sweepRate = 0.35
	scaleNear = 1.0
	scaleFar  = 0.3
)

// Shared scene colors (window paint and offscreen goldens use the same).
const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11

	frameR, frameG, frameB = 0.75, 0.78, 0.85
	readyR, readyG, readyB = 0.25, 0.85, 0.45
	loadR, loadG, loadB    = 0.95, 0.80, 0.30
)

// Body-local layout (legacy card coords, kept for probe reference;
// live window is full-window content, see metricStripH).
const (
	cardX0, cardX1, cardX2 = 16.0, 312.0, 608.0
	cardY                  = 44.0
	cardW, cardH           = 280.0, 420.0
	ghostThumb             = 140.0
	noteY                  = 480.0

	offW, offH = 480, 270
)

// 2.5D摆法：满窗即内容，指标浮左上，Golden排除指标带。
const metricStripH = 32.0

// Frozen file shapes (subset of the engine truth; numbers stay in the files).
type mipSizeDef struct {
	W      int `json:"w"`
	H      int `json:"h"`
	Levels int `json:"levels"`
}

type mipScaleDef struct {
	Scale float64 `json:"scale"`
	W     int     `json:"w"`
	H     int     `json:"h"`
	Level int     `json:"level"`
}

type mipLevelSizeDef struct {
	W     int `json:"w"`
	H     int `json:"h"`
	Level int `json:"level"`
	LW    int `json:"lw"`
	LH    int `json:"lh"`
}

type mipProbeDef struct {
	Desc  string  `json:"desc"`
	Scale float64 `json:"scale"`
	Level int     `json:"level"`
}

type mipSpotDef struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	RGBA [4]int `json:"rgba"`
}

type mipMeanDef struct {
	W     int `json:"w"`
	H     int `json:"h"`
	Level int `json:"level"`
	Mean  int `json:"mean"`
}

type mipSamplerDef struct {
	Name     string `json:"name"`
	Near     string `json:"near"`
	Far      string `json:"far"`
	Mip      string `json:"mip"`
	Aniso    int    `json:"aniso"`
	Mag      string `json:"mag"`
	Min      string `json:"min"`
	Mipmap   string `json:"mipmap"`
	EffAniso int    `json:"eff_aniso"`
}

type mipmapFrozen struct {
	Tolerance  int               `json:"tolerance"`
	Sizes      []mipSizeDef      `json:"sizes"`
	Scales     []mipScaleDef     `json:"scales"`
	LevelSizes []mipLevelSizeDef `json:"level_sizes"`
	Probes     []mipProbeDef     `json:"probes"`
	Level0     []mipSpotDef      `json:"level0_spots"`
	Means      []mipMeanDef      `json:"level_means"`
	Samplers   []mipSamplerDef   `json:"sampler_contract"`
}

type streamDef struct {
	ID     string `json:"id"`
	File   string `json:"file"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Blocks int    `json:"blocks"`
	Upload int    `json:"upload"`
	Pixels int    `json:"pixels"`
}

type streamSpot struct {
	File string `json:"file"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	RGBA [4]int `json:"rgba"`
}

type streamPlaceholder struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Blocks int    `json:"blocks"`
	Upload int    `json:"upload"`
	Pixels int    `json:"pixels"`
	RGBA   [4]int `json:"rgba"`
}

type streamFrozen struct {
	Tolerance           int               `json:"tolerance"`
	Format              string            `json:"format"`
	MaxStreams          int               `json:"max_streams"`
	MaxStreamBytes      int               `json:"max_stream_bytes"`
	MaxStreamAssetBytes int               `json:"max_stream_asset_bytes"`
	MaxIDLen            int               `json:"max_id_len"`
	Placeholder         streamPlaceholder `json:"placeholder"`
	Streams             []streamDef       `json:"streams"`
	Spots               []streamSpot      `json:"spots"`
}

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	LogicOK, PixOK, GoldenOK bool
	LogicDetail, PixDetail   string
	GoldenChanged            int
	GoldenWrote              bool
	// Case extras feeding the auto gate JSON.
	Scale03Level int
	PollTotalMs  float64
	PollAvgUs    float64
	BigBgMs      float64
	ReadyCount   int
	ReadyTotal   int
	GhostMissing bool
	BgEqual      bool
	OK           bool
}

func loadMipmapFrozen() (mipmapFrozen, error) {
	var c mipmapFrozen
	raw, err := os.ReadFile(frozenMipmapPath)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	return c, nil
}

func loadStreamFrozen() (streamFrozen, error) {
	var c streamFrozen
	raw, err := os.ReadFile(frozenStreamPath)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	return c, nil
}

func wantBytes(col core.Color) [4]int {
	r, g, b, a := col.ToBytes()
	return [4]int{int(r), int(g), int(b), int(a)}
}

func boolName(linear bool) string {
	if linear {
		return "linear"
	}
	return "nearest"
}

// checker64 rebuilds the frozen 64x64 black/white pattern the engine golden
// pins: 8px cells, top-left black, so every half lands on a known mean.
func checker64() []byte {
	const w, h, cell = 64, 64, 8
	px := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := byte(0)
			if (x/cell+y/cell)%2 == 1 {
				v = 255
			}
			off := (y*w + x) * 4
			px[off], px[off+1], px[off+2], px[off+3] = v, v, v, 255
		}
	}
	return px
}

// boxDown halves px (w-by-h) with the 2x2 average the CPU chain uses,
// mirroring the engine test helper (edge clamps, min 1).
func boxDown(px []byte, w, h int) ([]byte, int, int) {
	dw, dh := w/2, h/2
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	out := make([]byte, dw*dh*4)
	at := func(x, y int) [4]int {
		if x >= w {
			x = w - 1
		}
		if y >= h {
			y = h - 1
		}
		off := (y*w + x) * 4
		return [4]int{int(px[off]), int(px[off+1]), int(px[off+2]), int(px[off+3])}
	}
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			a, b, c, d := at(x*2, y*2), at(x*2+1, y*2), at(x*2, y*2+1), at(x*2+1, y*2+1)
			off := (y*dw + x) * 4
			for k := 0; k < 4; k++ {
				out[off+k] = byte((a[k] + b[k] + c[k] + d[k]) / 4)
			}
		}
	}
	return out, dw, dh
}

func meanR(px []byte) int {
	sum := 0
	for i := 0; i < len(px); i += 4 {
		sum += int(px[i])
	}
	return sum / (len(px) / 4)
}

func diffByte(a, b int) int {
	if a < b {
		return b - a
	}
	return a - b
}

// probeFarLogic drives the real mipmap switch against the frozen file:
// level math, the 1.0->0.3 far step, the sampler contract, per-picture Table.
func probeFarLogic() (bool, int, string) {
	c, err := loadMipmapFrozen()
	if err != nil {
		return false, 0, "frozen read: " + err.Error()
	}
	if c.Tolerance != 1 {
		return false, 0, fmt.Sprintf("tolerance = %d, want frozen 1", c.Tolerance)
	}
	for _, s := range c.Sizes {
		if got := tex.NumLevels(s.W, s.H); got != s.Levels {
			return false, 0, fmt.Sprintf("NumLevels(%dx%d) = %d, want %d", s.W, s.H, got, s.Levels)
		}
	}
	for _, s := range c.Scales {
		if got := tex.LevelForScale(s.Scale, s.W, s.H); got != s.Level {
			return false, 0, fmt.Sprintf("LevelForScale(%v %dx%d) = %d, want %d", s.Scale, s.W, s.H, got, s.Level)
		}
	}
	for _, s := range c.LevelSizes {
		lw, lh, ok := tex.LevelSize(s.W, s.H, s.Level)
		if !ok || lw != s.LW || lh != s.LH {
			return false, 0, fmt.Sprintf("LevelSize(%dx%d @%d) = %dx%d,%v, want %dx%d,true",
				s.W, s.H, s.Level, lw, lh, ok, s.LW, s.LH)
		}
	}
	// The S56 headline: 1x down to 0.3x must land on a stable deeper level.
	lv03 := tex.LevelForScale(scaleFar, 64, 64)
	if lv03 != 1 {
		return false, lv03, fmt.Sprintf("LevelForScale(0.3) = %d, want 1", lv03)
	}
	// Sampler contract: every frozen entry maps through one function.
	for _, s := range c.Samplers {
		f := tex.Filter{Near: tex.ParseKind(s.Near), Far: tex.ParseKind(s.Far),
			Mip: tex.ParseMipMode(s.Mip), MaxAniso: uint16(s.Aniso)} //nolint:gosec // data-driven, validated by Table below
		if s.Name == "zero-keeps-history" {
			if f != (tex.Filter{}) {
				return false, lv03, "zero-keeps-history is not the zero value"
			}
			continue
		}
		tab := tex.NewTable()
		if err := tab.Set("probe", f); err != nil {
			return false, lv03, s.Name + ": Table.Set: " + err.Error()
		}
		mag, min, mip, aniso := f.SamplerParams()
		if boolName(mag) != s.Mag || boolName(min) != s.Min || boolName(mip) != s.Mipmap || int(aniso) != s.EffAniso {
			return false, lv03, fmt.Sprintf("%s: params = %s/%s/%s/%d, want %s/%s/%s/%d",
				s.Name, boolName(mag), boolName(min), boolName(mip), aniso,
				s.Mag, s.Min, s.Mipmap, s.EffAniso)
		}
	}
	// The frozen far-stable switch actually switches.
	var near tex.Filter
	if err := near.SetFilter(tex.KindNearest, tex.KindNearest, 1); err != nil {
		return false, lv03, "SetFilter near: " + err.Error()
	}
	var far tex.Filter
	if err := far.SetFilter(tex.KindLinear, tex.KindLinear, 4); err != nil {
		return false, lv03, "SetFilter far: " + err.Error()
	}
	if err := far.SetMipmap(tex.MipLinear); err != nil {
		return false, lv03, "SetMipmap: " + err.Error()
	}
	mag, min, mip, aniso := far.SamplerParams()
	if !mag || !min || !mip || aniso != 4 {
		return false, lv03, fmt.Sprintf("far-stable = %v/%v/%v/%d, want true/true/true/4", mag, min, mip, aniso)
	}
	// Per-picture memory: two ids hold different switches independently.
	tab := tex.NewTable()
	if err := tab.Set("tree-near", near); err != nil {
		return false, lv03, "Set tree-near: " + err.Error()
	}
	if err := tab.Set("tree-far", far); err != nil {
		return false, lv03, "Set tree-far: " + err.Error()
	}
	gotNear, ok := tab.Get("tree-near")
	if !ok || gotNear != near {
		return false, lv03, "tree-near replay diverged"
	}
	gotFar, ok := tab.Get("tree-far")
	if !ok || gotFar != far {
		return false, lv03, "tree-far replay diverged"
	}
	return true, lv03, fmt.Sprintf("levels=%d scales=%d samplers=%d lv03=%d",
		len(c.Sizes), len(c.Scales), len(c.Samplers), lv03)
}

// probeStreamLogic drives the real Stream ledger against the frozen file:
// background decode, microsecond Poll, millisecond big picture, magenta ghost.
func probeStreamLogic() (bool, string, probeResult) {
	var extra probeResult
	c, err := loadStreamFrozen()
	if err != nil {
		return false, "frozen read: " + err.Error(), extra
	}
	if tex.MaxStreams != c.MaxStreams || tex.MaxStreamBytes != c.MaxStreamBytes ||
		tex.MaxStreamAssetBytes != c.MaxStreamAssetBytes || tex.MaxStreamIDLen != c.MaxIDLen {
		return false, "stream budgets diverge from stream_cases.json", extra
	}
	if tex.FormatBC1RGBAUnorm.String() != c.Format {
		return false, fmt.Sprintf("format = %q, want %q", tex.FormatBC1RGBAUnorm.String(), c.Format), extra
	}
	s := tex.NewStream()
	raws := map[string][]byte{}
	for _, want := range c.Streams {
		raw, err := os.ReadFile(texDataDir + "/" + want.File)
		if err != nil {
			return false, "read " + want.File + ": " + err.Error(), extra
		}
		raws[want.ID] = raw
		if err := s.Request(core.AssetID(want.ID), raw); err != nil {
			return false, want.ID + ": Request: " + err.Error(), extra
		}
		if st, err := s.Poll(core.AssetID(want.ID)); err != nil {
			return false, want.ID + ": Poll after Request: " + err.Error(), extra
		} else if st != tex.StateLoading && st != tex.StateReady {
			return false, fmt.Sprintf("%s: Poll = %v, want loading/ready", want.ID, st), extra
		}
	}
	for _, want := range c.Streams {
		im, err := s.Wait(core.AssetID(want.ID))
		if err != nil {
			return false, want.ID + ": Wait: " + err.Error(), extra
		}
		if im.Width() != want.Width || im.Height() != want.Height ||
			im.BlockCount() != want.Blocks || im.UploadSize() != want.Upload || im.PixelSize() != want.Pixels {
			return false, fmt.Sprintf("%s: size drift", want.ID), extra
		}
		// Background and sync decode stay bitwise identical.
		direct, err := tex.ParseKTX2(raws[want.ID])
		if err != nil {
			return false, want.ID + ": ParseKTX2: " + err.Error(), extra
		}
		if !im.Equal(direct) {
			return false, want.ID + ": background diverged from sync", extra
		}
		extra.BgEqual = true
	}
	extra.ReadyCount = s.Count()
	extra.ReadyTotal = len(c.Streams)
	if extra.ReadyCount != extra.ReadyTotal {
		return false, fmt.Sprintf("ready = %d, want %d", extra.ReadyCount, extra.ReadyTotal), extra
	}
	// Foreground Polls stay microsecond-scale while workers decode.
	const tiles = 64
	batch := tex.NewStream()
	checker := raws["stream/checker"]
	t0 := time.Now()
	for i := 0; i < tiles; i++ {
		if err := batch.Request(core.AssetID(fmt.Sprintf("poll/tile_%03d", i)), checker); err != nil {
			return false, "poll batch Request: " + err.Error(), extra
		}
	}
	pollT0 := time.Now()
	for i := 0; i < tiles; i++ {
		if _, err := batch.Poll(core.AssetID(fmt.Sprintf("poll/tile_%03d", i))); err != nil {
			return false, "poll batch Poll: " + err.Error(), extra
		}
	}
	pollEl := time.Since(pollT0)
	_ = t0
	extra.PollTotalMs = float64(pollEl.Microseconds()) / 1000.0
	if tiles > 0 {
		extra.PollAvgUs = float64(pollEl.Microseconds()) / tiles
	}
	if pollEl > maxPollTotalMs*time.Millisecond {
		return false, fmt.Sprintf("foreground polls took %v, play would stall", pollEl), extra
	}
	// One big picture streams in the millisecond magnitude.
	bigT0 := time.Now()
	big := tex.NewStream()
	bigID := core.AssetID("stream/big")
	if err := big.Request(bigID, raws["stream/big"]); err != nil {
		return false, "big Request: " + err.Error(), extra
	}
	if _, err := big.Wait(bigID); err != nil {
		return false, "big Wait: " + err.Error(), extra
	}
	extra.BigBgMs = float64(time.Since(bigT0).Microseconds()) / 1000.0
	if time.Since(bigT0) > maxBigBgMs*time.Millisecond {
		return false, fmt.Sprintf("big background took %v, want millisecond magnitude", time.Since(bigT0)), extra
	}
	// Missing file serves the shared magenta placeholder, never nil.
	ghost := tex.NewStream()
	if err := ghost.RequestFile("stream/ghost", texDataDir+"/no_such.ktx2"); err != nil {
		return false, "ghost RequestFile: " + err.Error(), extra
	}
	if _, err := ghost.Wait("stream/ghost"); err == nil {
		return false, "ghost Wait want an error", extra
	}
	if st, err := ghost.Poll("stream/ghost"); err == nil || st != tex.StateMissing {
		return false, fmt.Sprintf("ghost Poll = %v/%v, want missing/not-found", st, err), extra
	}
	if got, ok := ghost.Get("stream/ghost"); ok || got == nil || got != tex.PlaceholderImage() {
		return false, "ghost Get want the shared magenta", extra
	}
	if st, err := ghost.Poll("stream/never"); err == nil || st != tex.StateEmpty {
		return false, "unknown Poll want empty/not-found", extra
	}
	extra.GhostMissing = true
	detail := fmt.Sprintf("ready=%d/%d poll_total=%.3fms poll_avg=%.1fus big_bg=%.3fms ghost=missing",
		extra.ReadyCount, extra.ReadyTotal, extra.PollTotalMs, extra.PollAvgUs, extra.BigBgMs)
	return true, detail, extra
}

// probeTexPixels asserts the shared pixel evidence offscreen: frozen spots
// land exact, quadrant seams stay sharp (no bleed), the placeholder is
// opaque magenta on every pixel, and far halves keep their frozen means.
func probeTexPixels() (bool, string) {
	c, err := loadStreamFrozen()
	if err != nil {
		return false, "frozen read: " + err.Error()
	}
	if c.Tolerance != 0 {
		return false, fmt.Sprintf("tolerance = %d, want frozen 0", c.Tolerance)
	}
	for _, sp := range c.Spots {
		im, err := tex.LoadKTX2(texDataDir + "/" + sp.File)
		if err != nil {
			return false, sp.File + ": " + err.Error()
		}
		got, ok := im.At(sp.X, sp.Y)
		if !ok {
			return false, fmt.Sprintf("%s (%d,%d): At ok=false", sp.File, sp.X, sp.Y)
		}
		if wantBytes(got) != sp.RGBA {
			return false, fmt.Sprintf("%s (%d,%d) = %v, want %v", sp.File, sp.X, sp.Y, wantBytes(got), sp.RGBA)
		}
	}
	// Seam: the checker quadrants meet sharp at the block seam, no bleed.
	edge, err := tex.LoadKTX2(texDataDir + "/checker_8x8.ktx2")
	if err != nil {
		return false, "checker: " + err.Error()
	}
	pairs := [][2][2]int{{{3, 3}, {4, 3}}, {{3, 4}, {4, 4}}, {{3, 0}, {4, 0}}, {{0, 3}, {0, 4}}}
	for i, p := range pairs {
		a, _ := edge.At(p[0][0], p[0][1])
		b, _ := edge.At(p[1][0], p[1][1])
		if a == b {
			return false, fmt.Sprintf("edge[%d] shares %v, want a sharp seam", i, a)
		}
	}
	// Placeholder: opaque magenta everywhere, frozen geometry.
	ph := tex.PlaceholderImage()
	if ph == nil {
		return false, "PlaceholderImage nil"
	}
	if ph.Width() != c.Placeholder.Width || ph.Height() != c.Placeholder.Height ||
		ph.BlockCount() != c.Placeholder.Blocks || ph.UploadSize() != c.Placeholder.Upload ||
		ph.PixelSize() != c.Placeholder.Pixels {
		return false, "placeholder geometry drifted"
	}
	phPx := ph.Pixels()
	for y := 0; y < c.Placeholder.Height; y++ {
		for x := 0; x < c.Placeholder.Width; x++ {
			off := (y*c.Placeholder.Width + x) * 4
			got := [4]int{int(phPx[off]), int(phPx[off+1]), int(phPx[off+2]), int(phPx[off+3])}
			if got != c.Placeholder.RGBA {
				return false, fmt.Sprintf("placeholder (%d,%d) = %v, want %v", x, y, got, c.Placeholder.RGBA)
			}
		}
	}
	// Far means: halves stay 50/50, never one flat color (no shimmer source).
	mc, err := loadMipmapFrozen()
	if err != nil {
		return false, "mipmap frozen read: " + err.Error()
	}
	px := checker64()
	for _, m := range mc.Means {
		cur, w, h := px, 64, 64
		for i := 0; i < m.Level; i++ {
			cur, w, h = boxDown(cur, w, h)
		}
		_, _ = w, h
		if got := meanR(cur); diffByte(got, m.Mean) > mc.Tolerance {
			return false, fmt.Sprintf("level %d mean = %d, want %d", m.Level, got, m.Mean)
		}
	}
	detail := fmt.Sprintf("spots=%d edges=%d placeholder=magenta means=%d tol=0/1",
		len(c.Spots), len(pairs), len(mc.Means))
	return true, detail
}

// fillCells paints RGBA bytes as cell-sized rects (deterministic golden helper).
func fillCells(dc *render.Context, px []byte, w, h int, ox, oy, cell float64) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			dc.SetRGBA(float64(px[off])/255, float64(px[off+1])/255, float64(px[off+2])/255, 1)
			dc.DrawRectangle(ox+float64(x)*cell, oy+float64(y)*cell, cell, cell)
			_ = dc.Fill()
		}
	}
}

func strokeFrame(dc *render.Context, x, y, w, h float64) {
	dc.SetRGBA(frameR, frameG, frameB, 1)
	dc.SetLineWidth(1)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Stroke()
}

// paintGoldenFar draws the deterministic far mask: decoded checker at L0,
// its box halves, and the magenta placeholder, each framed.
func paintGoldenFar(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	im, err := tex.LoadKTX2(texDataDir + "/checker_8x8.ktx2")
	if err != nil {
		return
	}
	px := im.Pixels()
	fillCells(dc, px, 8, 8, 16, 16, 8)
	strokeFrame(dc, 16, 16, 64, 64)
	half, _, _ := boxDown(px, 8, 8)
	fillCells(dc, half, 4, 4, 96, 16, 16)
	strokeFrame(dc, 96, 16, 64, 64)
	quarter, _, _ := boxDown(half, 4, 4)
	fillCells(dc, quarter, 2, 2, 176, 16, 32)
	strokeFrame(dc, 176, 16, 64, 64)
	fillCells(dc, tex.PlaceholderImage().Pixels(), 4, 4, 256, 16, 16)
	strokeFrame(dc, 256, 16, 64, 64)
}

// paintGoldenStream draws the deterministic stream mask: the three frozen
// pictures, the placeholder, and the framed ghost card.
func paintGoldenStream(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	red, err := tex.LoadKTX2(texDataDir + "/solid_red_4x4.ktx2")
	if err != nil {
		return
	}
	fillCells(dc, red.Pixels(), 4, 4, 16, 16, 16)
	strokeFrame(dc, 16, 16, 64, 64)
	checker, err := tex.LoadKTX2(texDataDir + "/checker_8x8.ktx2")
	if err != nil {
		return
	}
	fillCells(dc, checker.Pixels(), 8, 8, 96, 16, 8)
	strokeFrame(dc, 96, 16, 64, 64)
	big, err := tex.LoadKTX2(texDataDir + "/solid_256x256.ktx2")
	if err != nil {
		return
	}
	top := make([]byte, 64*64*4)
	full := big.Pixels()
	for y := 0; y < 64; y++ {
		copy(top[y*64*4:(y+1)*64*4], full[y*256*4:y*256*4+64*4])
	}
	fillCells(dc, top, 64, 64, 176, 16, 1)
	strokeFrame(dc, 176, 16, 64, 64)
	fillCells(dc, tex.PlaceholderImage().Pixels(), 4, 4, 256, 16, 16)
	strokeFrame(dc, 256, 16, 64, 64)
	fillCells(dc, tex.PlaceholderImage().Pixels(), 4, 4, 336, 16, 16)
	dc.SetRGBA(0.1, 0.1, 0.12, 1)
	dc.SetLineWidth(3)
	dc.DrawRectangle(336, 16, 64, 64)
	_ = dc.Stroke()
}

// probeGolden compares the deterministic case frame against the frozen mask
// with zero tolerance; the first run produces the baseline.
func probeGolden(caseName, goldenPath string) (ok bool, changed int, wrote bool) {
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
	if caseName == "far" {
		paintGoldenFar(dc)
	} else {
		paintGoldenStream(dc)
	}
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/engine/tex/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(goldenPath)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		return encErr == nil, 0, encErr == nil
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		return false, 0, false
	}
	if !img.Bounds().Eq(want.Bounds()) {
		return false, 1, false
	}
	// Golden裁掉顶部指标带：浮层指标行不参比。
	stripPx := int(metricStripH * float64(img.Bounds().Dy()) / float64(winH))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		if y-img.Bounds().Min.Y < stripPx {
			continue
		}
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			ar, ag, ab, aa := img.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}
	return changed == 0, changed, false
}

// runProbes collects the three evidences: logic, pixels, golden mask.
func runProbes(caseName string) probeResult {
	var p probeResult
	if caseName == "far" {
		p.LogicOK, p.Scale03Level, p.LogicDetail = probeFarLogic()
		p.ReadyTotal = 1
		if p.LogicOK {
			p.ReadyCount = 1
		}
	} else {
		var extra probeResult
		var ok bool
		var detail string
		ok, detail, extra = probeStreamLogic()
		p.LogicOK, p.LogicDetail = ok, detail
		p.PollTotalMs, p.PollAvgUs, p.BigBgMs = extra.PollTotalMs, extra.PollAvgUs, extra.BigBgMs
		p.ReadyCount, p.ReadyTotal = extra.ReadyCount, extra.ReadyTotal
		p.GhostMissing, p.BgEqual = extra.GhostMissing, extra.BgEqual
	}
	p.PixOK, p.PixDetail = probeTexPixels()
	var wrote bool
	goldenPath := goldenStreamPath
	if caseName == "far" {
		goldenPath = goldenFarPath
	}
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden(caseName, goldenPath)
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// levelImg is one downsampled level: pixels plus dimensions.
type levelImg struct {
	px []byte
	w  int
	h  int
}

// texSim is the live window state: full-window scene + floating metrics.
type texSim struct {
	caseName string
	app      *embedder.PipelineApp
	root     *rendering.AbsoluteBox
	phase    *wrkit.PhaseClock
	fullBox  *rendering.RenderBox
	metric   *rendering.RenderText

	// Far sweep: 1.0 -> 0.3 -> 1.0 through the real LevelForScale.
	scale    float64
	scaleDir float64
	scaleMin float64
	level    int
	switches int
	levels   []levelImg
	nearPx   []byte
	farBox   *rendering.RenderBox
	monT     float64
	monX     float64
	frames    int

	// Stream ledger: background Requests, foreground Polls.
	stream  *tex.Stream
	ids     []string
	states  []tex.State
	ready   int
	ghostSt tex.State
	pollAvg float64
	pollN   int64
	// Big-picture background timing crosses a goroutine boundary:
	// atomics only, so -race stays clean.
	bigMsBits atomic.Int64
	bigDone   atomic.Bool
	cardPx    []levelImg
}

func bigBgMs(s *texSim) float64 {
	if s == nil || !s.bigDone.Load() {
		return -1
	}
	return math.Float64frombits(uint64(s.bigMsBits.Load())) //nolint:gosec // bits stored by Float64bits
}

type ticker struct{ s *texSim }

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
	// Monster walk advances in both cases (window only).
	s.monT += dt
	s.monX = math.Mod(s.monT*60, winW+128) - 64
	if s.caseName == "far" {
		s.scale += s.scaleDir * sweepRate * dt
		if s.scale <= scaleFar {
			s.scale, s.scaleDir = scaleFar, 1
		}
		if s.scale >= scaleNear {
			s.scale, s.scaleDir = scaleNear, -1
		}
		if s.scale < s.scaleMin {
			s.scaleMin = s.scale
		}
		lv := tex.LevelForScale(s.scale, 64, 64)
		if lv != s.level {
			s.level = lv
			s.switches++
		}
		s.fullBox.MarkNeedsPaint()
		if s.metric != nil {
			snap := s.app.Metrics().Snapshot()
			fps := 0.0
			if snap.AvgFrameIntervalMs > 1e-6 {
				fps = 1000.0 / snap.AvgFrameIntervalMs
			}
			s.metric.SetText(fmt.Sprintf("scale=%.2f L%d sw=%d fps=%.0f", s.scale, s.level, s.switches, fps))
			s.metric.MarkNeedsPaint()
		}
	} else {
		pollT0 := time.Now()
		ready := 0
		for i, id := range s.ids {
			st, _ := s.stream.Poll(core.AssetID(id))
			s.states[i] = st
			if st == tex.StateReady {
				ready++
				if s.cardPx[i].px == nil {
					if im, ok := s.stream.Get(core.AssetID(id)); ok && im != nil {
						s.cardPx[i].px = im.Pixels()
						s.cardPx[i].w, s.cardPx[i].h = im.Width(), im.Height()
					}
				}
			}
		}
		s.ready = ready
		gst, _ := s.stream.Poll("stream/ghost")
		s.ghostSt = gst
		elUs := float64(time.Since(pollT0).Microseconds())
		s.pollN++
		s.pollAvg += (elUs - s.pollAvg) / float64(s.pollN)
		s.fullBox.MarkNeedsPaint()
		if s.metric != nil {
			snap := s.app.Metrics().Snapshot()
			fps := 0.0
			if snap.AvgFrameIntervalMs > 1e-6 {
				fps = 1000.0 / snap.AvgFrameIntervalMs
			}
			s.metric.SetText(fmt.Sprintf("ready=%d/%d %s poll=%.1f fps=%.0f", s.ready, len(s.ids), s.ghostSt.String(), s.pollAvg, fps))
			s.metric.MarkNeedsPaint()
		}
	}
	_ = s.phase.Advance(dt)
	s.app.ScheduleFrame()
	return true
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

func failJSON(caseName string, probe probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   "game_tex--case=" + caseName,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

// paintLiveCells draws RGBA bytes into a live RenderBox paint callback.
// The source is downsampled to at most thumbMax pixels per side first:
// per-pixel vector rectangles cost one draw call each, so painting a
// 256x256 source cell-by-cell (65536 calls) stalls the frame loop and
// the window drops to ~14fps. Downsampling keeps the live card honest
// (same pixels, fewer calls) while probes use the full-res source.
func paintLiveCells(pc *rendering.PaintContext, px []byte, w, h int, dw, dh float64) {
	if pc == nil || pc.DC == nil || len(px) == 0 || w <= 0 || h <= 0 {
		return
	}
	const thumbMax = 64
	sw, sh := w, h
	if sw > thumbMax || sh > thumbMax {
		sw = thumbMax
		if sh > thumbMax {
			sh = thumbMax
		}
	}
	ax, ay := pc.Abs(0, 0)
	cw, ch := dw/float64(sw), dh/float64(sh)
	for y := 0; y < sh; y++ {
		sy := y * h / sh
		for x := 0; x < sw; x++ {
			sx := x * w / sw
			off := (sy*w + sx) * 4
			pc.DC.SetRGBA(float64(px[off])/255, float64(px[off+1])/255, float64(px[off+2])/255, 1)
			pc.DC.DrawRectangle(ax+float64(x)*cw, ay+float64(y)*ch, cw+0.5, ch+0.5)
			_ = pc.DC.Fill()
		}
	}
}

func main() {
	caseFlag := flag.String("case", "", "scenario case (far or stream)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	if *caseFlag != "far" && *caseFlag != "stream" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want far or stream (dual case)\n", *caseFlag)
		os.Exit(1)
	}
	caseName := *caseFlag
	scenario := "game_tex--case=" + caseName
	wrkit.EnsureUIFace()

	probe := runProbes(caseName)
	fmt.Fprintf(os.Stderr, "game_tex[%s]: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) %s | %s\n",
		caseName, probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
		probe.GoldenChanged, probe.LogicDetail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(caseName, probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_tex: selftest FAIL, not opening window")
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

	sim := &texSim{caseName: caseName}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}
	sim.root = root
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	if caseName == "far" {
		sim.scale, sim.scaleDir, sim.scaleMin = scaleNear, -1, scaleNear
		sim.level = tex.LevelForScale(scaleNear, 64, 64)
		// Level pixels mirror the CPU box chain the engine doc names;
		// level choice itself comes from the real tex.LevelForScale.
		base := checker64()
		w, h := 64, 64
		sim.levels = make([]levelImg, 0, 7)
		px := base
		for {
			cp := append([]byte(nil), px...)
			sim.levels = append(sim.levels, levelImg{px: cp, w: w, h: h})
			if w == 1 && h == 1 {
				break
			}
			px, w, h = boxDown(px, w, h)
		}
		ck, err := tex.LoadKTX2(texDataDir + "/checker_8x8.ktx2")
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: load checker:", err)
			os.Exit(1)
		}
		sim.nearPx = ck.Pixels()

		// 满窗即内容：整窗为远近过滤场景（过滤+压缩色块+大图底+巡逻怪加厚）。
		sim.fullBox = rendering.NewRenderBox()
		sim.fullBox.FixedWidth, sim.fullBox.FixedHeight = winW, winH
		sim.fullBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			lv := sim.level
			if lv < 0 {
				lv = 0
			}
			if lv >= len(sim.levels) {
				lv = len(sim.levels) - 1
			}
			paintLiveCells(pc, sim.levels[lv].px, sim.levels[lv].w, sim.levels[lv].h, size.Width, size.Height)
			mx, my := sim.monX, size.Height-90
			pc.DC.SetRGBA(0.16, 0.14, 0.20, 1)
			pc.DC.DrawRectangle(mx, my, 72, 52)
			_ = pc.DC.Fill()
			pc.DC.SetRGBA(1, 0.25, 0.20, 1)
			pc.DC.DrawRectangle(mx+50, my+14, 12, 12)
			_ = pc.DC.Fill()
		}
		_ = cardX0
		_ = cardX1
		_ = cardX2
		root.Place(sim.fullBox, 0, 0)
		sim.metric = wrkit.Label("scale=1.00", 13, 0.92, 0.94, 0.98)
		root.Place(sim.metric, 8, 8)
	} else {
		c, err := loadStreamFrozen()
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: frozen:", err)
			os.Exit(1)
		}
		sim.stream = tex.NewStream()
		for _, want := range c.Streams {
			sim.ids = append(sim.ids, want.ID)
			if err := sim.stream.RequestFile(core.AssetID(want.ID), texDataDir+"/"+want.File); err != nil {
				fmt.Fprintln(os.Stderr, "FAIL: RequestFile:", err)
				os.Exit(1)
			}
		}
		if err := sim.stream.RequestFile("stream/ghost", texDataDir+"/no_such.ktx2"); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: ghost RequestFile:", err)
			os.Exit(1)
		}
		sim.states = make([]tex.State, len(sim.ids))
		for i := range sim.states {
			sim.states[i] = tex.StateLoading
		}
		sim.ghostSt = tex.StateLoading
		sim.cardPx = make([]levelImg, len(sim.ids))
		// Big picture streams off the play path; the ticker Polls it.
		// Timing crosses threads via atomics only.
		go func() {
			bt0 := time.Now()
			_, _ = sim.stream.Wait("stream/big")
			ms := float64(time.Since(bt0).Microseconds()) / 1000.0
			sim.bigMsBits.Store(int64(math.Float64bits(ms))) //nolint:gosec // round-trip through Float64frombits
			sim.bigDone.Store(true)
		}()

		// 满窗即内容：整窗为流式场景（首就绪大图打底+缺图品红角标+巡逻怪加厚）。
		sim.fullBox = rendering.NewRenderBox()
		sim.fullBox.FixedWidth, sim.fullBox.FixedHeight = winW, winH
		sim.fullBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			bg := tex.PlaceholderImage().Pixels()
			bw, bh := 4, 4
			for _, cp := range sim.cardPx {
				if cp.px != nil {
					bg, bw, bh = cp.px, cp.w, cp.h
					break
				}
			}
			paintLiveCells(pc, bg, bw, bh, size.Width, size.Height)
			mx, my := sim.monX, size.Height-90
			pc.DC.SetRGBA(0.16, 0.14, 0.20, 1)
			pc.DC.DrawRectangle(mx, my, 72, 52)
			_ = pc.DC.Fill()
			pc.DC.SetRGBA(1, 0.25, 0.20, 1)
			pc.DC.DrawRectangle(mx+50, my+14, 12, 12)
			_ = pc.DC.Fill()
			// Ghost corner: magenta placeholder chip stays visible.
			paintLiveCells(pc, tex.PlaceholderImage().Pixels(), 4, 4, 96, 96)
		}
		root.Place(sim.fullBox, 0, 0)
		sim.metric = wrkit.Label("ready=0", 13, 0.92, 0.94, 0.98)
		root.Place(sim.metric, 8, 8)
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_tex", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_tex[%s]: close (%s)\n", caseName, win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_tex[%s]: pointer %s (%.0f,%.0f) n=%d\n",
						caseName, ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_tex events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_tex[%s]: key n=%d\n", caseName, summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_tex events=%d", summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					sim.fullBox.FixedWidth, sim.fullBox.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsPaint()
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_tex[%s]: resize %dx%d n=%d\n", caseName, ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_tex events=%d", summary.Pointer+summary.Key+summary.Resize))
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
	root.MarkNeedsPaint()
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
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	extra := map[string]any{
		"case":          caseName,
		"probe_ok":      probeOK,
		"boundary_skip": snap.BoundarySkip,
		"pixels":        probe.PixDetail,
		"golden":        probe.GoldenChanged,
	}
	if caseName == "far" {
		extra["scale_min"] = sim.scaleMin
		extra["scale03_level"] = probe.Scale03Level
		extra["level_switches"] = sim.switches
		extra["level"] = sim.level
	} else {
		extra["ready"] = sim.ready
		extra["ready_total"] = len(sim.ids)
		extra["ghost"] = sim.ghostSt.String()
		extra["poll_avg_us"] = sim.pollAvg
		extra["big_bg_ms"] = bigBgMs(sim)
		extra["probe_poll_total_ms"] = probe.PollTotalMs
		extra["probe_big_bg_ms"] = probe.BigBgMs
		extra["frames"] = sim.frames
	}

	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenario,
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
		if caseName == "far" {
			if presents < 1 || !probe.OK || probe.Scale03Level != 1 || sim.switches < 1 || sim.scaleMin > 0.35 {
				fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v lv03=%d switches=%d scalemin=%.2f (want >=1, true, 1, >=1, <=0.35)\n",
					presents, probe.OK, probe.Scale03Level, sim.switches, sim.scaleMin)
				os.Exit(1)
			}
		} else {
			if presents < 1 || !probe.OK || sim.ready != len(sim.ids) || sim.ghostSt != tex.StateMissing {
				fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v ready=%d/%d ghost=%s (want >=1, true, all, missing)\n",
					presents, probe.OK, sim.ready, len(sim.ids), sim.ghostSt.String())
				os.Exit(1)
			}
		}
		fmt.Fprintf(os.Stderr, "game_tex[%s]: OK presents=%d elapsed=%.1fs\n", caseName, presents, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"probe_ok":    probeOK,
		"extra":       extra,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_tex[%s]: backend=%s presents=%d elapsed=%.1fs\n",
		caseName, win.Backend(), presents, elapsed)
}
