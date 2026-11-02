// Command ui_wr_p35_persp is the P3.5 batch-3 capability real window: Matrix4
// projective math plus perspective-correct textured quads.
//
//	A  one receding ground plate textured twice — plain bilinear (left, wrong)
//	   and 1/w rational interpolation (right, right). Bands of equal world size
//	   must squeeze toward the horizon: the affine plate keeps them evenly spaced
//	   on screen, the projective one does not. The cyan measure line is the same
//	   screen height in both plates and therefore crosses different bands — the
//	   pixel probes sit exactly on that disagreement.
//	B  a depth ladder: five equal plates at growing tilt, every corner pushed
//	   through Matrix4.ProjectQuad. The last plate has its far edge behind the
//	   eye (w≤0), so the whole quad is refused and its slot stays bare.
//	C  Matrix4 tiles: identity, a lifted affine matrix (parallelogram) and the
//	   projector (trapezoid). The point inside the trapezoid's bounding box but
//	   outside the quad is probed empty — a projective map is not a parallelogram.
//
// Gate mode asserts 9 logic probes (numbers land in the JSON report), semantic
// pixel probes derived from the same projection the painters used, and a
// zero-tolerance golden over the shell content.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=6 go run ./examples/ui_wr_p35_persp -auto-only                       # gate
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_p35_persp -auto-only  # seed once
//	go run ./examples/ui_wr_p35_persp                                                # manual, stays open
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"time"

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
	winW, winH   = 1200, 800
	closeSeconds = 6
	windowTitle  = "gpui ui_wr_p35_persp — P3.5 Matrix4 与透视校正贴图"

	texW    = 64
	texH    = 40
	bandN   = 5
	bandPix = texH / bandN // texel rows per band; the last one is the separator

	offW, offH   = 320, 260
	offCX, offCY = 160.0, 130.0
)

// Card layout in body-logical coordinates: the painters place with these and
// the probes convert card-local geometry through the origin they recorded.
const (
	cardAX, cardAY, cardAW, cardAH = 8.0, 8.0, 436.0, 300.0
	cardBX, cardBY, cardBW, cardBH = 460.0, 8.0, 436.0, 300.0
	cardCX, cardCY, cardCW, cardCH = 8.0, 320.0, 888.0, 320.0
)

// cardBG is the per-card backdrop every "must be empty" probe asserts against.
var cardBG = [3]float64{0.07, 0.08, 0.10}

// bands are the five texture bands, far (index 0) to near (index 4). One table
// feeds both the texture and every colour prediction, so a probe can never
// disagree with the picture because of a duplicated constant.
var bands = [bandN][3]uint8{
	{235, 55, 55},
	{55, 235, 90},
	{70, 120, 235},
	{235, 205, 70},
	{190, 95, 235},
}

// ─────────────────────────── geometry (single source) ───────────────────────────

type geomT struct {
	focal float64
	// Card A plate: world half-extents plus the depths of the near (bottom) and
	// far (top) edge. Both plates share the numbers; only the sampling differs.
	aHW, aHH, aNear, aFar float64
	aAffX, aPersX, aY     float64
	aProbeDX, aEdgeInset  float64
	// Card B depth ladder: equal plates, growing far depth, last one behind eye.
	bHW, bHH, bNear, bY float64
	bX0, bStepX         float64
	bFars               [5]float64
	// Card C matrix tiles.
	cHalf, cY, cNear, cFar   float64
	cX                       [3]float64
	cProbeDX, cProbeDY, cPad float64
}

var G = geomT{
	focal: 100,
	aHW:   110, aHH: 150, aNear: 20, aFar: 260,
	aAffX: 108, aPersX: 320, aY: 150, aProbeDX: 20, aEdgeInset: 4,
	bHW: 34, bHH: 90, bNear: 20, bY: 175, bX0: 55, bStepX: 86,
	bFars: [5]float64{40, 110, 200, 330, -140},
	cHalf: 60, cY: 175, cNear: 40, cFar: 200,
	cX: [3]float64{156, 452, 740}, cProbeDX: 20, cProbeDY: 12, cPad: 3,
}

// plate projects a ground plate of half-extents (hw,hh) whose bottom edge sits
// at depth near and top edge at depth far, about the principal point (cx,cy),
// through the engine's own Matrix4 projector. Corners come back in the
// DrawImageQuad order (TL,TR,BR,BL) with the homogeneous weights; ok=false is
// the engine's whole-quad refusal (a corner behind the eye, non-finite input).
func plate(cx, cy, hw, hh, near, far float64) ([4]render.Point, [4]float64, bool) {
	m, ok := render.ProjectorMatrix4(G.focal, render.Pt(cx, cy), render.Point{})
	if !ok {
		return [4]render.Point{}, [4]float64{}, false
	}
	world := [4]render.Point{
		{X: -hw, Y: -hh}, {X: hw, Y: -hh},
		{X: hw, Y: hh}, {X: -hw, Y: hh},
	}
	depths := [4]float64{far, far, near, near}
	return m.ProjectQuad(world, depths)
}

// midLinePoint is the point at screen fraction t between the far (top) edge
// midpoint and the near (bottom) edge midpoint. For this symmetric trapezoid the
// constant-v contours are horizontal in BOTH samplings, so one measure line
// names one band per mode and a probe may sit off the centre line.
func midLinePoint(corners [4]render.Point, t float64) render.Point {
	top := render.Pt((corners[0].X+corners[1].X)/2, (corners[0].Y+corners[1].Y)/2)
	bot := render.Pt((corners[3].X+corners[2].X)/2, (corners[3].Y+corners[2].Y)/2)
	return render.Pt(top.X+(bot.X-top.X)*t, top.Y+(bot.Y-top.Y)*t)
}

// perspV is the exact 1/w interpolation of v along that measure line: v=0 at the
// far edge, v=1 at the near edge, weights wNear/wFar at the two ends.
func perspV(t, wNear, wFar float64) float64 {
	return t * wFar / ((1-t)*wNear + t*wFar)
}

// bandInterior maps a texture v to a band index and refuses positions within a
// texel row of a band edge or on the separator row, so a colour probe never
// reads the wrong side of a texel boundary (≈4 screen px of margin).
func bandInterior(v float64) (int, bool) {
	if v <= 0 || v >= 1 {
		return 0, false
	}
	row := int(math.Floor(v * texH))
	if row < 0 || row >= texH {
		return 0, false
	}
	in := row % bandPix
	if in < 1 || in > bandPix-3 {
		return 0, false
	}
	return row / bandPix, true
}

// bandProbePair searches the middle of the measure line — where the two
// samplings disagree most — for the fraction whose affine v and projective v
// land in DIFFERENT bands with both interior. The picture, the pixel probes and
// the offscreen CPU twin all read this one result.
func bandProbePair(wNear, wFar float64) (t, vPers float64, bAff, bPers int, ok bool) {
	best := -1.0
	for i := 140; i <= 260; i++ {
		tt := float64(i) / 400
		vp := perspV(tt, wNear, wFar)
		ba, oka := bandInterior(tt)
		bp, okp := bandInterior(vp)
		disp := math.Abs(vp - tt)
		if !oka || !okp || ba == bp || disp < 0.06 || disp <= best {
			continue
		}
		best, t, vPers, bAff, bPers, ok = disp, tt, vp, ba, bp, true
	}
	return
}

// bandProbePersp picks the first candidate fraction on the measure line whose
// projective band is interior (the ladder plates are all drawn projective).
func bandProbePersp(wNear, wFar float64) (float64, int, bool) {
	for _, tt := range []float64{0.5, 0.45, 0.55, 0.4, 0.6, 0.35, 0.65, 0.3, 0.7} {
		if b, ok := bandInterior(perspV(tt, wNear, wFar)); ok {
			return tt, b, true
		}
	}
	return 0, 0, false
}

// ─────────────────────────── texture ───────────────────────────

var tex *render.ImageBuf

// bandTexture builds the five-band plate texture from the `bands` table, with a
// dark separator row at the bottom of each band so the boundaries are readable
// on screen and identical to what bandInterior rejects.
func bandTexture() *render.ImageBuf {
	img := image.NewNRGBA(image.Rect(0, 0, texW, texH))
	for y := 0; y < texH; y++ {
		c := bands[y/bandPix]
		if y%bandPix == bandPix-1 {
			c = [3]uint8{8, 8, 12}
		}
		for x := 0; x < texW; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: c[0], G: c[1], B: c[2], A: 255})
		}
	}
	return render.ImageBufFromImage(img)
}

func bandColorF(b int) (float64, float64, float64) {
	c := bands[b]
	return float64(c[0]) / 255, float64(c[1]) / 255, float64(c[2]) / 255
}

// ─────────────────────────── logic probes ───────────────────────────

type logicResult struct {
	name string
	ok   bool
	det  string
	num  map[string]float64
}

func runLogicProbes() []logicResult {
	out := []logicResult{}
	add := func(r logicResult) { out = append(out, r) }

	// L1 the engine's projector equals the closed form it documents —
	// screen = center + world·f/(f+d), w = f+d — and ProjectQuad agrees with the
	// per-corner ProjectPoint calls it is built from.
	cx, cy := 30.0, -20.0
	m, okM := render.ProjectorMatrix4(G.focal, render.Pt(cx, cy), render.Point{})
	worstP, nSampled := 0.0, 0
	if okM {
		for _, d := range []float64{-95, -40, 0, 20, 55, 140, 330, 780} {
			for _, w := range []render.Point{{X: -180, Y: -90}, {X: 0, Y: 12}, {X: 240, Y: 160}} {
				got, gw, ok := m.ProjectPoint(w.X, w.Y, d)
				k := G.focal / (G.focal + d)
				want := render.Pt(cx+w.X*k, cy+w.Y*k)
				if !ok || math.Abs(gw-(G.focal+d)) > 1e-12 {
					worstP = math.Inf(1)
					continue
				}
				if e := math.Max(math.Abs(got.X-want.X), math.Abs(got.Y-want.Y)); e > worstP {
					worstP = e
				}
				nSampled++
			}
		}
	}
	world := [4]render.Point{{X: -60, Y: -40}, {X: 60, Y: -40}, {X: 60, Y: 40}, {X: -60, Y: 40}}
	depths := [4]float64{260, 260, 20, 20}
	qS, qW, okQ := m.ProjectQuad(world, depths)
	worstQ := 0.0
	for i := range qS {
		p, w, ok := m.ProjectPoint(world[i].X, world[i].Y, depths[i])
		if !ok || math.Abs(qS[i].X-p.X) > 1e-12 || math.Abs(qS[i].Y-p.Y) > 1e-12 || math.Abs(qW[i]-w) > 1e-12 {
			worstQ = math.Inf(1)
			break
		}
	}
	ok1 := okM && worstP <= 1e-9 && worstQ <= 1e-12 && nSampled == 24 && okQ
	add(logicResult{"projectquad_matches_closed_form", ok1,
		fmt.Sprintf("max |Δ|=%.3e over %d points (24/24 accepted), w ≡ f+d, ProjectQuad vs ProjectPoint Δ=%.1e",
			worstP, nSampled, worstQ),
		map[string]float64{"max_point_err": worstP}})

	// L2 one corner behind the eye refuses the WHOLE quad — never half a plate.
	s, wts, ok2a := m.ProjectQuad(world, [4]float64{-G.focal, -60, 20, 30})
	zeroed := s == [4]render.Point{} && wts == [4]float64{}
	_, _, ok2b := m.ProjectQuad(world, [4]float64{-140, -140, 20, 20})
	sJust, wJust, ok2c := m.ProjectQuad(world, [4]float64{-99.9, -99.9, 20, 20})
	scaleJust := math.Abs(sJust[0].X-cx) / 60
	ok2 := !ok2a && zeroed && !ok2b && ok2c && math.Abs(wJust[0]-0.1) < 1e-9 && scaleJust > 900
	add(logicResult{"behind_eye_refuses_whole_quad", ok2,
		fmt.Sprintf("w=0 → ok=false & zeroed=%v, w<0 → ok=false, w=0.1 → accepted at scale %.0f×",
			zeroed, scaleJust),
		map[string]float64{"just_in_front_scale": scaleJust}})

	// L3 non-finite inputs and a non-positive focal are rejected, never fed as
	// NaN into the draw path.
	nanC := world
	nanC[2] = render.Pt(math.NaN(), 40)
	_, _, okNaN := m.ProjectQuad(nanC, [4]float64{10, 10, 10, 10})
	_, _, okInf := m.ProjectQuad(world, [4]float64{math.Inf(1), 10, 10, 10})
	_, okFocalNeg := render.ProjectorMatrix4(-5, render.Point{}, render.Point{})
	_, okFocalNaN := render.ProjectorMatrix4(math.NaN(), render.Point{}, render.Point{})
	_, okCenterNaN := render.ProjectorMatrix4(G.focal, render.Pt(math.NaN(), 0), render.Point{})
	_, okCamNaN := render.ProjectorMatrix4(G.focal, render.Point{}, render.Pt(0, math.NaN()))
	ok3 := !okNaN && !okInf && !okFocalNeg && !okFocalNaN && !okCenterNaN && !okCamNaN
	add(logicResult{"nonfinite_inputs_rejected", ok3,
		fmt.Sprintf("accepted anyway: NaN corner=%v Inf depth=%v focal≤0=%v focal NaN=%v center NaN=%v camera NaN=%v",
			okNaN, okInf, okFocalNeg, okFocalNaN, okCenterNaN, okCamNaN), nil})

	// L4 depth orders the weights and shrinks the projected size monotonically.
	prevW, prevScale := math.Inf(-1), math.Inf(1)
	ok4 := true
	for _, d := range []float64{-50, -10, 0, 20, 50, 100, 300, 780} {
		p, gw, ok := m.ProjectPoint(120, 0, d)
		if !ok {
			ok4 = false
			break
		}
		k := G.focal / gw
		if gw <= prevW || k >= prevScale || math.Abs(p.X-(cx+120*k)) > 1e-9 {
			ok4 = false
		}
		prevW, prevScale = gw, k
	}
	add(logicResult{"weights_grow_scale_shrinks", ok4,
		"deeper corner ⇒ larger w ⇒ strictly smaller screen scale, pointwise", nil})

	// L5 equal weights collapse to the affine twin pixel-for-pixel (CPU path).
	affBytes := offscreenPlateBytes(false, false, render.InterpBilinear)
	evenBytes := offscreenPlateBytes(true, true, render.InterpBilinear)
	diffEven := -1
	if len(affBytes) == len(evenBytes) {
		diffEven = 0
		for i := range affBytes {
			if affBytes[i] != evenBytes[i] {
				diffEven++
			}
		}
	}
	add(logicResult{"equal_weights_bit_equal_affine", diffEven == 0,
		fmt.Sprintf("affine vs perspective(w≡217) differ in %d of %d bytes", diffEven, len(affBytes)),
		map[string]float64{"equal_weight_diff_bytes": float64(diffEven)}})

	// L6 the two modes really disagree offscreen, and each one matches its own
	// analytic band at the very measure-line point card A probes on screen.
	perspBytes := offscreenPlateBytes(true, false, render.InterpNearest)
	affNearest := offscreenPlateBytes(false, false, render.InterpNearest)
	diffs := 0
	if len(affNearest) == len(perspBytes) {
		for i := range affNearest {
			if affNearest[i] != perspBytes[i] {
				diffs++
			}
		}
	}
	pairT, pairVPers, pairBAff, pairBPers, pairOK := bandProbePair(G.focal+G.aNear, G.focal+G.aFar)
	if !pairOK {
		add(logicResult{"perspective_differs_from_affine", false, "no discriminating band pair found", nil})
	} else {
		px, py := offscreenProbePx(pairT)
		gotAff := pixelAt(affNearest, px, py)
		gotPers := pixelAt(perspBytes, px, py)
		disp := math.Abs(pairVPers - pairT)
		ok6 := diffs > 500 && disp > 0.06 &&
			sameColor(gotAff, bands[pairBAff]) && sameColor(gotPers, bands[pairBPers]) &&
			!sameColor(gotAff, gotPers)
		add(logicResult{"perspective_differs_from_affine", ok6,
			fmt.Sprintf("offscreen CPU plate: %d differing channels; probe (%d,%d) v_aff=%.4f→band%d rgb%v, v_pers=%.4f→band%d rgb%v",
				diffs, px, py, pairT, pairBAff, gotAff, pairVPers, pairBPers, gotPers),
			map[string]float64{"mode_diff_channels": float64(diffs), "uv_displacement": disp}})
	}

	// L7 an affine sub-block is only handed out for a plain lifted affine: a
	// projective map must refuse to be silently flattened.
	_, okAff := m.Affine()
	lift := render.Matrix4FromMatrix(render.Translate(15, -7).Multiply(render.Scale(1.4, 0.6)))
	am, okAm := lift.Affine()
	pt := render.Pt(23, 41)
	back := am.TransformPoint(pt)
	vs := lift.TransformVec4([4]float64{pt.X, pt.Y, 0, 1})
	ok7 := !okAff && okAm && !lift.IsIdentity() && render.Matrix4Identity().IsIdentity() &&
		math.Abs(back.X-vs[0]/vs[3]) <= 1e-12 && math.Abs(back.Y-vs[1]/vs[3]) <= 1e-12
	add(logicResult{"affine_refuses_projective_matrix", ok7,
		"ProjectorMatrix4.Affine()=false; lifted affine round-trips and agrees with Matrix pointwise", nil})

	// L8 inverse round-trip through the homogeneous space, plus a singular
	// matrix refused instead of returning garbage.
	inv, okInv := m.Invert()
	v0 := inv.Mul(m).TransformVec4([4]float64{37, -19, 55, 1})
	worstInv := math.Max(math.Abs(v0[0]-37), math.Abs(v0[1]+19))
	worstInv = math.Max(worstInv, math.Max(math.Abs(v0[2]-55), math.Abs(v0[3]-1)))
	// A zero affine block lifts to a rank-deficient 4×4.
	_, okSing := render.Matrix4FromMatrix(render.Matrix{}).Invert()
	ok8 := okInv && worstInv <= 1e-7 && !okSing
	add(logicResult{"invert_roundtrip_and_singular_refused", ok8,
		fmt.Sprintf("inv·m fixes (37,-19,55,1) to %.1e; zero-affine lift refused=%v", worstInv, !okSing), nil})

	// L9 the draw-entry sentinels: illegal weights and the unsupported
	// perspective+bicubic pairing surface as errors instead of wrong pixels.
	dc := render.NewContext(offW, offH)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
	cn, cw, okC := plate(offCX, offCY, G.aHW, G.aHH, G.aNear, G.aFar)
	ok9 := okC
	for _, ws := range [][4]float64{{0, 1, 1, 1}, {1, math.NaN(), 1, 1}, {1, 1, -3, 1}, {1, 1, math.Inf(1), 1}} {
		if err := dc.DrawImageQuad(tex, cn, render.QuadDrawOptions{
			Perspective: true, Weights: ws, Interpolation: render.InterpNearest, Opacity: 1,
		}); !errors.Is(err, render.ErrQuadBadWeights) {
			ok9 = false
		}
	}
	if err := dc.DrawImageQuad(tex, cn, render.QuadDrawOptions{
		Perspective: true, Weights: cw, Interpolation: render.InterpBicubic, Opacity: 1,
	}); !errors.Is(err, render.ErrQuadPerspectiveBicubic) {
		ok9 = false
	}
	// Controls: the same bicubic request without perspective stays legal, and the
	// honest perspective draw with the plate's own weights succeeds.
	if err := dc.DrawImageQuad(tex, cn, render.QuadDrawOptions{
		Interpolation: render.InterpBicubic, Opacity: 1,
	}); err != nil {
		ok9 = false
	}
	if err := dc.DrawImageQuad(tex, cn, render.QuadDrawOptions{
		Perspective: true, Weights: cw, Interpolation: render.InterpNearest, Opacity: 1,
	}); err != nil {
		ok9 = false
	}
	add(logicResult{"weight_and_interp_sentinels", ok9,
		"zero/NaN/negative/Inf weights → ErrQuadBadWeights; perspective+bicubic → ErrQuadPerspectiveBicubic; affine+bicubic and honest perspective still legal", nil})

	return out
}

// ─────────────────────────── offscreen CPU twin ───────────────────────────

func offscreenCorners() [4]render.Point {
	c, _, _ := plate(offCX, offCY, G.aHW, G.aHH, G.aNear, G.aFar)
	return c
}

// offscreenProbePx is the device pixel the band probes read on the offscreen
// twin: the measure line at fraction t, offset from the centre line like card A.
func offscreenProbePx(t float64) (int, int) {
	p := midLinePoint(offscreenCorners(), t)
	return int(math.Floor(p.X + G.aProbeDX)), int(math.Floor(p.Y))
}

// offscreenPlateBytes renders the card-A plate offscreen through the CPU path
// and returns the raw pixels. persp=false is the affine twin; even=true swaps
// the projective weights for a constant, which must reduce to the affine case.
func offscreenPlateBytes(persp, even bool, interp render.InterpolationMode) []byte {
	dc := render.NewContext(offW, offH)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
	corners, w, ok := plate(offCX, offCY, G.aHW, G.aHH, G.aNear, G.aFar)
	if !ok {
		return nil
	}
	if even {
		w = [4]float64{217, 217, 217, 217}
	}
	if err := dc.DrawImageQuad(tex, corners, render.QuadDrawOptions{
		Interpolation: interp, Opacity: 1, Perspective: persp, Weights: w,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_p35_persp: offscreen draw failed: %v\n", err)
		return nil
	}
	_ = dc.FlushGPU()
	img := dc.Image()
	raw := make([]byte, 0, offW*offH*4)
	for y := 0; y < offH; y++ {
		for x := 0; x < offW; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			raw = append(raw, byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8))
		}
	}
	return raw
}

func pixelAt(raw []byte, x, y int) [3]uint8 {
	if len(raw) < (y*offW+x+1)*4 {
		return [3]uint8{}
	}
	i := (y*offW + x) * 4
	return [3]uint8{raw[i], raw[i+1], raw[i+2]}
}

func sameColor(a, b [3]uint8) bool {
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		if d > 4 {
			return false
		}
	}
	return true
}

// ─────────────────────────── painters ───────────────────────────

var cardOrigin [3]render.Point

// sweepT animates the measure line in manual mode; gate mode freezes it on the
// probe fraction so the snapshot and the probes describe the same picture.
var sweepT = 0.49

func quadPath(corners [4]render.Point) *render.Path {
	p := render.NewPath()
	p.MoveTo(corners[0].X, corners[0].Y)
	p.LineTo(corners[1].X, corners[1].Y)
	p.LineTo(corners[2].X, corners[2].Y)
	p.LineTo(corners[3].X, corners[3].Y)
	p.Close()
	return p
}

func cardBase(pc *rendering.PaintContext, sz rendering.Size) (float64, float64) {
	dc := pc.DC
	dc.SetRGBA(cardBG[0], cardBG[1], cardBG[2], 1)
	dc.DrawRectangle(pc.OriginX, pc.OriginY, sz.Width, sz.Height)
	_ = dc.Fill()
	return pc.OriginX, pc.OriginY
}

// shift moves projected corners from card-local into window coordinates: the
// raw Canvas blit takes window space, while rendering.FillPath/StrokePath add
// the box origin themselves and must be handed card-local paths.
func shift(corners [4]render.Point, dx, dy float64) [4]render.Point {
	var out [4]render.Point
	for i, p := range corners {
		out[i] = render.Pt(p.X+dx, p.Y+dy)
	}
	return out
}

// blitPlate textures an already projected plate; persp=false is the affine twin.
func blitPlate(dc render.Canvas, corners [4]render.Point, w [4]float64, perspective bool) bool {
	if err := dc.DrawImageQuad(tex, corners, render.QuadDrawOptions{
		Interpolation: render.InterpNearest, Opacity: 1, Perspective: perspective, Weights: w,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_p35_persp: DrawImageQuad failed: %v\n", err)
		return false
	}
	return true
}

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	ox, oy := cardBase(pc, sz)

	// Both plates carry the same world geometry and the same weights — only the
	// sampling mode differs, so any pixel difference on the measure line is the
	// mode and nothing else.
	affC, affW, okA := plate(G.aAffX, G.aY, G.aHW, G.aHH, G.aNear, G.aFar)
	persC, persW, okP := plate(G.aPersX, G.aY, G.aHW, G.aHH, G.aNear, G.aFar)
	if !okA || !okP {
		return
	}
	blitPlate(pc.DC, shift(affC, ox, oy), affW, false)
	blitPlate(pc.DC, shift(persC, ox, oy), persW, true)
	for _, c := range [2][4]render.Point{affC, persC} {
		rendering.StrokePath(pc, quadPath(c), 1.0, 0.55, 0.60, 0.70, 0.5)
		p := midLinePoint(c, sweepT)
		// The line stops short of the probe column so it never paints over the
		// sampled pixel; both still name the same band (constant-v is horizontal).
		line := render.NewPath()
		line.MoveTo(p.X-92, p.Y)
		line.LineTo(p.X+G.aProbeDX-6, p.Y)
		rendering.StrokePath(pc, line, 1.5, 0.20, 0.95, 0.90, 0.85)
	}
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	ox, oy := cardBase(pc, sz)

	for i, far := range G.bFars {
		corners, w, ok := plate(G.bX0+float64(i)*G.bStepX, G.bY, G.bHW, G.bHH, G.bNear, far)
		if !ok {
			continue // behind the eye: the engine refused the whole quad
		}
		if blitPlate(pc.DC, shift(corners, ox, oy), w, true) {
			rendering.StrokePath(pc, quadPath(corners), 1.0, 0.55, 0.60, 0.70, 0.5)
		}
	}
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	cardBase(pc, sz)

	// Tile 1: identity affine block (the tile's own placement translate aside).
	// Tile 2: lifted affine — a parallelogram, opposite edges stay parallel.
	// Tile 3: projector — a trapezoid whose far edge is shorter, so its bounding
	// box has empty corners. All three are card-local: FillPath/StrokePath add
	// the box origin themselves.
	world := [4]render.Point{{X: -G.cHalf, Y: -G.cHalf}, {X: G.cHalf, Y: -G.cHalf},
		{X: G.cHalf, Y: G.cHalf}, {X: -G.cHalf, Y: G.cHalf}}
	mapAll := func(m render.Matrix) [4]render.Point {
		var out [4]render.Point
		for i, p := range world {
			out[i] = m.TransformPoint(p)
		}
		return out
	}
	drawTile := func(corners [4]render.Point, r, g, b float64) {
		p := quadPath(corners)
		rendering.FillPath(pc, p, r, g, b, 1)
		rendering.StrokePath(pc, p, 1.5, 0.85, 0.90, 0.96, 0.9)
	}
	drawTile(mapAll(render.Translate(G.cX[0], G.cY)), 0.55, 0.60, 0.68)
	drawTile(mapAll(render.Matrix{A: 1.15, B: 0.35, C: G.cX[1], D: 0.12, E: 0.95, F: G.cY}),
		0.30, 0.80, 0.45)
	c3, _, ok3 := plate(G.cX[2], G.cY, G.cHalf, G.cHalf, G.cNear, G.cFar)
	if !ok3 {
		return
	}
	drawTile(c3, 0.95, 0.45, 0.25)
	minX, minY, maxX, maxY := c3[0].X, c3[0].Y, c3[0].X, c3[0].Y
	for _, p := range c3 {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	box := render.NewPath()
	box.MoveTo(minX, minY)
	box.LineTo(maxX, minY)
	box.LineTo(maxX, maxY)
	box.LineTo(minX, maxY)
	box.Close()
	rendering.StrokePath(pc, box, 1.0, 0.45, 0.50, 0.60, 0.8)
}

// ─────────────────────────── main ───────────────────────────

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze sweep, snapshot + probes + golden")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until closed)")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = closeSeconds
	} else if secsSet {
		wrkit.RequireMinRun(secs, "P35-persp")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()
	tex = bandTexture()

	pairT, pairVPers, pairBAff, pairBPers, pairOK := bandProbePair(G.focal+G.aNear, G.focal+G.aFar)
	if !pairOK {
		fmt.Fprintln(os.Stderr, "FAIL: no discriminating band pair on card A")
		os.Exit(1)
	}
	if *autoOnly {
		sweepT = pairT // the frozen measure line is the probed one
	}

	logic := runLogicProbes()
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p35_persp: logic %-36s ok=%v %s\n", l.name, l.ok, l.det)
		if l.ok {
			logicOK++
		}
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: windowTitle,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P3.5 Matrix4 透视 — 1/w 校正贴图 / 齐次拒绝 / 投影图块", []string{
		"A 左=仿射双线性（错：等距分带）",
		"A 右=1/w 透视校正（对：向远端挤压）",
		"A 青色横线=同一屏幕高度，两盘读不同色带",
		"A 上下边缘外 4px=空白，梯形落位钉住",
		"B 深度阶梯 5 盘，far 递增透视贴图",
		"B 末盘远边在眼后（w≤0）→ 整块拒绝只剩底色",
		"C 恒等 / 提升仿射(平行四边形) / 投影(梯形)",
		"C 梯形 bbox 左上角=空白，证明非仿射",
		"门禁: 逻辑 9 条 + 像素 13 条 + 金图逐位",
	})

	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{cardAX, cardAY, cardAW, cardAH, "A 同盘双采样：仿射(左) vs 透视校正(右)", paintCardA},
		{cardBX, cardBY, cardBW, cardBH, "B 深度阶梯：眼后整块拒绝", paintCardB},
		{cardCX, cardCY, cardCW, cardCH, "C Matrix4 图块：恒等 / 仿射提升 / 投影梯形", paintCardC},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) { paint(pc, sz) }
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}
	refusedX := cardBX + G.bX0 + 4*G.bStepX
	shell.Body.LabelAt("眼后·整块拒绝", 11, refusedX-46, cardBY+26, 0.95, 0.45, 0.45)

	snapDir := filepath.Join("examples", "ui_wr_p35_persp", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p35_persp_final.png")
	basePath := filepath.Join(snapDir, "goldens", "p35_persp_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	var clock float64
	app.Scheduler().Tickers().Add(&tickFunc{on: func(dt float64) {
		if !*autoOnly {
			clock += dt
			sweepT = 0.5 + 0.45*math.Sin(clock*0.6)
		}
		shell.NoteHUDTick(dt)
		app.ScheduleFrame()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	win.Close()
	elapsed := time.Since(t0).Seconds()

	// ── pixel probes: window-logical = card origin + the card-local numbers the
	// painters used, derived from the same projection ──
	localA := func(x, y float64) (float64, float64) { return cardOrigin[0].X + x, cardOrigin[0].Y + y }
	localB := func(x, y float64) (float64, float64) { return cardOrigin[1].X + x, cardOrigin[1].Y + y }
	localC := func(x, y float64) (float64, float64) { return cardOrigin[2].X + x, cardOrigin[2].Y + y }
	emptyProbe := func(name string, x, y float64) wrkit.PixelProbe {
		return wrkit.PixelProbe{Name: name, X: x, Y: y, R: cardBG[0], G: cardBG[1], B: cardBG[2], Tol: 8}
	}

	probes := []wrkit.PixelProbe{}

	affCorners, _, okA := plate(G.aAffX, G.aY, G.aHW, G.aHH, G.aNear, G.aFar)
	persCorners, _, okP := plate(G.aPersX, G.aY, G.aHW, G.aHH, G.aNear, G.aFar)
	if okA && okP {
		// The same screen height must read band bAff through the affine sampling
		// and band bPers through the projective one.
		for i, cs := range [2][4]render.Point{affCorners, persCorners} {
			tag, b := "affine", pairBAff
			if i == 1 {
				tag, b = "projective", pairBPers
			}
			p := midLinePoint(cs, pairT)
			r, g, bl := bandColorF(b)
			x, y := localA(p.X+G.aProbeDX, p.Y)
			probes = append(probes, wrkit.PixelProbe{
				Name: fmt.Sprintf("A_measure_line_%s_band%d", tag, b), X: x, Y: y, R: r, G: g, B: bl, Tol: 40,
			})
		}
		// Placement witnesses: 4px outside the projected far and near edges must
		// be bare card — the trapezoid lands where ProjectQuad says, not where a
		// parallelogram over the same world size would have reached.
		for i, cs := range [2][4]render.Point{affCorners, persCorners} {
			tag := "affine"
			if i == 1 {
				tag = "projective"
			}
			farMid, nearMid := midLinePoint(cs, 0), midLinePoint(cs, 1)
			x, y := localA(farMid.X, farMid.Y-G.aEdgeInset)
			probes = append(probes, emptyProbe("A_"+tag+"_above_far_edge_empty", x, y))
			x, y = localA(nearMid.X, nearMid.Y+G.aEdgeInset)
			probes = append(probes, emptyProbe("A_"+tag+"_below_near_edge_empty", x, y))
		}
	}

	// B: every accepted ladder plate shows its projective band on the measure
	// line; the refused one leaves bare card.
	for i, far := range G.bFars {
		cx := G.bX0 + float64(i)*G.bStepX // card-local, like the painter
		corners, weights, ok := plate(cx, G.bY, G.bHW, G.bHH, G.bNear, far)
		if !ok {
			x, y := localB(cx, G.bY)
			probes = append(probes, emptyProbe(fmt.Sprintf("B_plate%d_far%.0f_behind_eye_empty", i, far), x, y))
			continue
		}
		t, band, okT := bandProbePersp(weights[3], weights[0])
		if !okT {
			continue
		}
		p := midLinePoint(corners, t)
		r, g, b := bandColorF(band)
		x, y := localB(p.X, p.Y)
		probes = append(probes, wrkit.PixelProbe{
			Name: fmt.Sprintf("B_plate%d_far%.0f_band%d", i, far, band), X: x, Y: y, R: r, G: g, B: b, Tol: 40,
		})
	}

	// C: tile fills, the empty space beside each quad, and the trapezoid's empty
	// bounding-box corner.
	tileCols := [][3]float64{{0.55, 0.60, 0.68}, {0.30, 0.80, 0.45}}
	for i, col := range tileCols {
		x, y := localC(G.cX[i]+G.cProbeDX, G.cY+G.cProbeDY)
		probes = append(probes, wrkit.PixelProbe{
			Name: fmt.Sprintf("C_tile%d_fill", i+1), X: x, Y: y, R: col[0], G: col[1], B: col[2], Tol: 40,
		})
		x, y = localC(G.cX[i]+G.cHalf+G.cPad, G.cY-G.cHalf-G.cPad)
		probes = append(probes, emptyProbe(fmt.Sprintf("C_tile%d_outside_fill_empty", i+1), x, y))
	}
	c3, _, ok3 := plate(G.cX[2], G.cY, G.cHalf, G.cHalf, G.cNear, G.cFar)
	if ok3 {
		mid := midLinePoint(c3, 0.5)
		x, y := localC(mid.X, mid.Y)
		probes = append(probes, wrkit.PixelProbe{Name: "C_tile3_trapezoid_fill", X: x, Y: y,
			R: 0.95, G: 0.45, B: 0.25, Tol: 40})
		minX, minY := c3[0].X, c3[0].Y
		for _, p := range c3 {
			minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		}
		x, y = localC(minX+G.cPad, minY+G.cPad)
		probes = append(probes, emptyProbe("C_tile3_bbox_corner_empty_not_parallelogram", x, y))
	}

	failedPixels := 0
	var firstPixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p35_persp: pixel %-44s FAIL %v\n", pr.Name, err)
			failedPixels++
			if firstPixelErr == nil {
				firstPixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p35_persp: pixel %-44s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "Matrix4/ProjectorMatrix4/ProjectPoint/ProjectQuad/Affine/Invert, DrawImageQuad Perspective+Weights",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal,
		"pixel_probes_total": len(probes), "pixel_probes_failed": failedPixels,
		"band_probe_t":          pairT,
		"band_affine":           pairBAff,
		"band_perspective":      pairBPers,
		"band_uv_displace":      math.Abs(pairVPers - pairT),
		"focal":                 G.focal,
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P35-persp", Scenario: "ui_wr_p35_persp", Backend: win.Backend().String(),
		Snap: app.Metrics().Snapshot(), PresentCount: app.PresentCount(), ElapsedSec: elapsed,
		SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if *autoOnly {
			os.Exit(1)
		}
	}
	if logicOK != logicTotal {
		fail("logic probes %d/%d ok", logicOK, logicTotal)
	}
	if len(probes) < 12 {
		fail("only %d pixel probes built", len(probes))
	}
	if failedPixels != 0 {
		fail("pixel probes: %d/%d failed, first: %v", failedPixels, len(probes), firstPixelErr)
	}
	if goldenErr != nil {
		fail("golden: %v", goldenErr)
	}
	if !seeded && goldenDiff != 0 {
		fail("golden diff %.4f%% want 0 over %d px", goldenDiff, goldenPx)
	}
	if *autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_p35_persp: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, len(probes)-failedPixels, len(probes),
		goldenDiff, seeded, *autoOnly, elapsed)
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
