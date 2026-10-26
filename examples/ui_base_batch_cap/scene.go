// Scene + measurement state for ui_base_batch_cap.
//
// Two primitive-dense modes share one window:
//   - prims:  DrawAtlasEx with 1k/5k/10k atlas quads (2.5D sprite base).
//   - vector: FillPath with 100/300/600 curved paths (design-tool base,
//     re-tessellation cost is the measured work).
//
// The example only places/controls content; all drawing goes through the
// public render canvas API. Content is deterministic for a frozen phase so
// the Golden baseline can be bitwise.
package main

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

const (
	modePrims  = 0
	modeVector = 1
)

// content geometry: keep a margin so background probes are stable.
const (
	contentMarginX = 24.0
	contentTop     = 40.0 // metric strip floats over content; masked in Golden
	contentBottom  = 24.0
)

// primSteps is the row-13 sprite ladder fixed by the matrix (1k/5k/10k).
var primSteps = []int{1000, 5000, 10000}

// vectorSteps is the path-dense ladder for mode ②. 24 cubic segments per
// path means 2.4k/7.2k/14.4k curves on screen at each step.
var vectorSteps = []int{100, 300, 600}

const vectorSegments = 24

// phaseBits converts float64 phase to/from atomic bits (paint thread reads,
// controller writes).
type scene struct {
	mode    atomic.Int32
	n       atomic.Int32
	phase   atomic.Uint64
	frozen  atomic.Bool
	prewarm atomic.Bool
	// frozenPaints increments after a paintBatch that observed frozen=true.
	// Snapshot barriers wait for this instead of guessing how many presents
	// are still in flight with the pre-freeze phase.
	frozenPaints atomic.Uint64

	atlas   *render.ImageBuf
	sprites map[int][]render.AtlasSprite
	paths   map[int][]*render.Path
}

func newScene() *scene {
	s := &scene{
		sprites: make(map[int][]render.AtlasSprite, len(primSteps)),
		paths:   make(map[int][]*render.Path, len(vectorSteps)),
	}
	s.atlas = buildAtlas()
	if s.atlas != nil {
		for _, n := range primSteps {
			s.sprites[n] = buildSprites(n)
		}
	}
	for _, n := range vectorSteps {
		s.paths[n] = buildPaths(n)
	}
	return s
}

// buildAtlas makes a 32x32 RGBA atlas: four 16x16 solid color cells. Tint is
// deliberately avoided (whole batch would fall back to CPU sampling); color
// comes from the source cell instead.
func buildAtlas() *render.ImageBuf {
	img, err := render.NewImageBuf(32, 32, render.FormatRGBA8)
	if err != nil {
		return nil
	}
	palette := [4][3]uint8{
		{255, 90, 70},   // coral
		{70, 200, 255},  // cyan
		{120, 235, 120}, // green
		{245, 210, 90},  // amber
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			c := palette[(y/16)*2+(x/16)]
			_ = img.SetRGBA(x, y, c[0], c[1], c[2], 255)
		}
	}
	return img
}

// gridFor returns the grid used to lay n items in the content box.
func gridFor(n int) (cols, rows int, cellW, cellH float64) {
	boxW := winW - contentMarginX*2
	boxH := winH - contentTop - contentBottom
	cols = int(math.Ceil(math.Sqrt(float64(n) * boxW / boxH)))
	if cols < 1 {
		cols = 1
	}
	rows = (n + cols - 1) / cols
	cellW = boxW / float64(cols)
	cellH = boxH / float64(rows)
	return
}

// buildSprites precomputes n atlas quads (no per-frame allocation).
func buildSprites(n int) []render.AtlasSprite {
	cols, _, cellW, cellH := gridFor(n)
	out := make([]render.AtlasSprite, 0, n)
	size := math.Min(cellW, cellH) * 0.72
	if size < 2 {
		size = 2
	}
	for i := 0; i < n; i++ {
		col, row := i%cols, i/cols
		cx := contentMarginX + (float64(col)+0.5)*cellW
		cy := contentTop + (float64(row)+0.5)*cellH
		cell := (i * 7) % 4
		out = append(out, render.AtlasSprite{
			SrcX: float64((cell % 2) * 16), SrcY: float64((cell / 2) * 16),
			SrcW: 16, SrcH: 16,
			DstX: cx - size/2, DstY: cy - size/2, DstW: size, DstH: size,
			Opacity: 1,
			Filter:  render.InterpNearest,
		})
	}
	return out
}

// buildPaths precomputes n curved paths centered on (0,0). Each path is a
// 24-segment wavy closed curve (cubic segments), sized to its grid cell.
func buildPaths(n int) []*render.Path {
	_, _, cellW, cellH := gridFor(n)
	radius := math.Min(cellW, cellH) * 0.42
	if radius < 2 {
		radius = 2
	}
	out := make([]*render.Path, 0, n)
	for i := 0; i < n; i++ {
		p := render.NewPath()
		// Star-ish closed contour: radius oscillates 3 times per turn, with
		// control points offset tangentially to keep real cubic curves.
		step := 2 * math.Pi / vectorSegments
		rAt := func(a float64) float64 {
			return radius * (0.72 + 0.28*math.Sin(3*a+float64(i%7)*0.17))
		}
		p.MoveTo(rAt(0), 0)
		for k := 1; k <= vectorSegments; k++ {
			a0 := float64(k-1) * step
			a1 := float64(k) * step
			r0, r1 := rAt(a0), rAt(a1)
			am := (a0 + a1) / 2
			rm := (r0 + r1) / 2 * 1.18 // pull control points outward
			c1x, c1y := rm*math.Cos(am-step*0.18), rm*math.Sin(am-step*0.18)
			c2x, c2y := rm*math.Cos(am+step*0.18), rm*math.Sin(am+step*0.18)
			p.CubicTo(c1x, c1y, c2x, c2y, r1*math.Cos(a1), r1*math.Sin(a1))
		}
		p.Close()
		out = append(out, p)
	}
	return out
}

// set switches density/mode (controller goroutine).
func (s *scene) set(mode int32, n int) {
	s.mode.Store(mode)
	s.n.Store(int32(n))
}

// setFrozen freezes animation at phase 0 for a deterministic snapshot.
func (s *scene) setFrozen() {
	s.phase.Store(math.Float64bits(0))
	s.frozen.Store(true)
}

// setRunning resumes animation.
func (s *scene) setRunning() {
	s.frozen.Store(false)
}

// setPrewarm marks the unmeasured warm-up cycle (label only; the ladders and
// pixel content are identical).
func (s *scene) setPrewarm(v bool) { s.prewarm.Store(v) }

func (s *scene) currentPhase() float64 {
	return math.Float64frombits(s.phase.Load())
}

func (s *scene) advance(dt float64) {
	if s.frozen.Load() {
		return
	}
	old := s.currentPhase()
	s.phase.Store(math.Float64bits(old + dt*0.6))
}

// paintBatch draws the current mode; called on the UI thread.
func (s *scene) paintBatch(pc *rendering.PaintContext, size rendering.Size, fps float64, presents int64) {
	if pc == nil || pc.DC == nil {
		return
	}
	frozen := s.frozen.Load()
	dc := pc.DC
	w, h := size.Width, size.Height
	if w <= 0 {
		w = winW
	}
	if h <= 0 {
		h = winH
	}
	dc.SetRGB(0.055, 0.065, 0.085)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	mode := s.mode.Load()
	n := int(s.n.Load())
	phase := s.currentPhase()
	switch mode {
	case modeVector:
		s.paintVector(pc, n, phase)
	default:
		s.paintPrims(pc, n, phase)
	}

	// Metric strip floats over the content (Golden masks the top band).
	if face := wrkit.FaceAt(13); face != nil {
		dc.SetFont(face)
	}
	dc.SetRGBA(0.78, 0.88, 0.98, 1)
	modeName := "prims"
	if mode == modeVector {
		modeName = "vector"
	}
	prefix := ""
	if s.prewarm.Load() {
		prefix = "PREWARM "
	}
	dc.DrawString(paintLabel(prefix, modeName, n, fps, presents), 10, 22)
	if frozen {
		s.frozenPaints.Add(1)
	}
}

// paintLabel builds the overlay line (kept out of the paint core so tests can
// read the exact fields).
func paintLabel(prefix, mode string, n int, fps float64, presents int64) string {
	if mode == "" {
		mode = "prims"
	}
	return fmt.Sprintf("BATCH %s%s n=%d fps=%.0f presents=%d", prefix, mode, n, fps, presents)
}

func (s *scene) paintPrims(pc *rendering.PaintContext, n int, phase float64) {
	if s.atlas == nil {
		return
	}
	list := s.sprites[n]
	if len(list) == 0 {
		return
	}
	// Camera pan: one global transform moves the whole batch (no per-sprite
	// rewrite), so the measured work is drawing N quads, not slice upkeep.
	pc.DC.Push()
	pc.DC.Translate(math.Sin(phase)*6, math.Cos(phase*0.7)*4)
	_, _ = pc.DC.DrawAtlasEx(s.atlas, list, render.AtlasDrawOptions{})
	pc.DC.Pop()
}

func (s *scene) paintVector(pc *rendering.PaintContext, n int, phase float64) {
	list := s.paths[n]
	if len(list) == 0 {
		return
	}
	cols, _, cellW, cellH := gridFor(n)
	palette := [4][3]float64{
		{0.95, 0.36, 0.30},
		{0.30, 0.75, 0.98},
		{0.42, 0.90, 0.48},
		{0.96, 0.82, 0.35},
	}
	for i, p := range list {
		if p == nil {
			continue
		}
		col, row := i%cols, i/cols
		cx := contentMarginX + (float64(col)+0.5)*cellW
		cy := contentTop + (float64(row)+0.5)*cellH
		c := palette[i%4]
		pc.DC.SetRGBA(c[0], c[1], c[2], 0.92)
		pc.DC.Push()
		pc.DC.Translate(cx, cy)
		pc.DC.Rotate(phase*0.35 + float64(i%11)*0.07)
		_ = pc.DC.FillPath(p)
		pc.DC.Pop()
	}
}
