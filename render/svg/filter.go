//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"image"
	"math"
	"strings"

	"github.com/energye/gpui/render"
)

// SVG filter: a <filter> element holds a chain of primitives (fe*). The
// chain is a small data-flow graph — each primitive reads named inputs
// (in / in2: "SourceGraphic", "SourceAlpha", or a previous result=) and
// writes result=. Evaluation happens fully offscreen (same family as the
// mask three-offscreen): the element renders into a source image, each
// primitive runs on an offscreen Context (Apply* rides the GPU filter
// path), and the last output composites back.
//
// Supported primitives map onto the P3.4 filter family:
//
//	feGaussianBlur → Context.ApplyBlurXY
//	feColorMatrix  → Context.ApplyColorMatrix (matrix/saturate/hueRotate/
//	                  luminanceToAlpha built per SVG spec formulas)
//	feDropShadow   → Context.ApplyDropShadow
//	feOffset       → offscreen re-draw at (dx, dy)
//	feFlood        → solid fill
//	feBlend        → DrawImage with BlendMode (normal/multiply/screen/
//	                  darken/lighten)
//	feComposite    → Porter-Duff (over/in/out/xor via BlendMode)
//	feMerge        → sequential DrawImage
//
// Named refusals (registered, design §4.1): feComposite arithmetic,
// feTile, feImage, feTurbulence, feDisplacementMap, feComponentTransfer,
// feConvolveMatrix, feMorphology, and the lighting family. A filter that
// contains an unsupported primitive is NOT applied — the element renders
// unfiltered (CSS Masking error口径) — and the refusal is a warning.

// FilterPrimitive is one fe* element of a filter chain.
type FilterPrimitive struct {
	Kind   string // "feGaussianBlur", "feColorMatrix", ...
	In     string // primary input (default: previous result / SourceGraphic)
	In2    string // secondary input (feBlend/feComposite)
	Result string // output name
	// Primitive parameters (raw parsed values).
	Numbers      []float64 // stdDeviation / dx / dy / values / kernel…
	Numbers2     []float64 // second numeric list (feColorMatrix values)
	Str1         string    // flood-color / operator / mode / type
	Str2         string    // flood-opacity as string (parsed to Number below)
	Number       float64   // single-value params
	HasNumber    bool
	FloodColor   string
	FloodOpacity float64
	MergeNodes   []string // feMergeNode in= list
}

// FilterElement represents <filter>.
type FilterElement struct {
	Attrs        Attrs
	X, Y         Length // region (filterUnits; default -10%..120% of bbox)
	W, H         Length
	Units        Units // filterUnits (default objectBoundingBox)
	ContentUnits Units // primitiveUnits (default userSpaceOnUse)
	Primitives   []*FilterPrimitive
}

func (e *FilterElement) attrs() *Attrs { return &e.Attrs }

// supportedFilterPrimitive reports whether the fe* kind maps onto the
// render filter family; unsupported kinds are named refusals.
func supportedFilterPrimitive(kind string) bool {
	switch kind {
	case "feGaussianBlur", "feOffset", "feFlood", "feColorMatrix",
		"feBlend", "feComposite", "feMerge", "feDropShadow":
		return true
	}
	return false
}

// blendModeFor maps feBlend mode / feComposite operator onto BlendMode.
// ok=false for operators without a BlendMode mapping (arithmetic).
func blendModeFor(kind, op string) (render.BlendMode, bool) {
	if kind == "feBlend" {
		switch strings.ToLower(strings.TrimSpace(op)) {
		case "normal":
			return render.BlendNormal, true
		case "multiply":
			return render.BlendMultiply, true
		case "screen":
			return render.BlendScreen, true
		case "darken":
			return render.BlendDarken, true
		case "lighten":
			return render.BlendLighten, true
		}
		return 0, false
	}
	// feComposite
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "over":
		return render.BlendNormal, true
	case "in":
		return render.BlendSourceIn, true
	case "out":
		return render.BlendSourceOut, true
	case "xor":
		return render.BlendXor, true
	}
	return 0, false // arithmetic: no k-coefficient interface (registered)
}

// saturateMatrix builds the feColorMatrix type="saturate" values (SVG 1.1).
func saturateMatrix(s float64) [20]float32 {
	return [20]float32{
		float32(0.213 + 0.787*s), float32(0.715 - 0.715*s), float32(0.072 - 0.072*s), 0, 0,
		float32(0.213 - 0.213*s), float32(0.715 + 0.285*s), float32(0.072 - 0.072*s), 0, 0,
		float32(0.213 - 0.213*s), float32(0.715 - 0.715*s), float32(0.072 + 0.928*s), 0, 0,
		0, 0, 0, 1, 0,
	}
}

// hueRotateMatrix builds feColorMatrix type="hueRotate" (SVG 1.1 formula).
func hueRotateMatrix(deg float64) [20]float32 {
	c := math.Cos(deg * 3.141592653589793 / 180)
	s := math.Sin(deg * 3.141592653589793 / 180)
	return [20]float32{
		float32(0.213 + c*0.787 - s*0.213), float32(0.715 - c*0.715 - s*0.715), float32(0.072 - c*0.072 + s*0.928), 0, 0,
		float32(0.213 - c*0.213 + s*0.143), float32(0.715 + c*0.285 + s*0.140), float32(0.072 - c*0.072 - s*0.283), 0, 0,
		float32(0.213 - c*0.787 + s*0.787), float32(0.715 - c*0.715 - s*0.715), float32(0.072 + c*0.928 + s*0.072), 0, 0,
		0, 0, 0, 1, 0,
	}
}

// luminanceToAlphaMatrix builds feColorMatrix type="luminanceToAlpha".
func luminanceToAlphaMatrix() [20]float32 {
	return [20]float32{
		0, 0, 0, 0, 0,
		0, 0, 0, 0, 0,
		0, 0, 0, 0, 0,
		float32(0.2125), float32(0.7154), float32(0.0721), 0, 0,
	}
}

// colorMatrixValues resolves feColorMatrix into a 20-value matrix.
func colorMatrixValues(fp *FilterPrimitive) ([20]float32, bool) {
	var m [20]float32
	switch strings.ToLower(strings.TrimSpace(fp.Str1)) {
	case "matrix", "":
		if len(fp.Numbers2) < 20 {
			return m, false
		}
		for i := 0; i < 20; i++ {
			m[i] = float32(fp.Numbers2[i])
		}
		return m, true
	case "saturate":
		if len(fp.Numbers2) < 1 {
			return m, false
		}
		return saturateMatrix(fp.Numbers2[0]), true
	case "huerotate":
		if len(fp.Numbers2) < 1 {
			return hueRotateMatrix(0), true
		}
		return hueRotateMatrix(fp.Numbers2[0]), true
	case "luminancetoalpha":
		return luminanceToAlphaMatrix(), true
	}
	return m, false
}

// renderFilterElement evaluates a filter chain for one element and composites
// the result back. Pipeline: the element content renders into the source
// offscreen (SourceGraphic), each primitive runs on its own offscreen Context
// (Apply* rides the GPU filter path), the last output composites back.
// Returns false when the filter must not be applied (unsupported primitive /
// dangling ref already warned) — the caller then renders the element
// unfiltered (CSS error口径).
func (d *Document) renderFilterElement(dc *render.Context, combined *render.Path, bbox render.Rect, f *FilterElement, a *Attrs, p *props, st *renderState) bool {
	for _, fp := range f.Primitives {
		if !supportedFilterPrimitive(fp.Kind) {
			return false
		}
	}
	// Filter region: filterUnits (default oBB) over the element bbox.
	bw := bbox.Max.X - bbox.Min.X
	bh := bbox.Max.Y - bbox.Min.Y
	regionCoord := func(l Length, axis LengthAxis, min float64, dim float64) float64 {
		if f.Units == UnitsUserSpaceOnUse {
			return l.Resolve(RefSpace{ViewportW: dim, ViewportH: dim}, axis)
		}
		frac := l.Value
		if l.Unit == LengthUnitPercent {
			frac = l.Value / 100
		}
		return min + frac*dim
	}
	regX := regionCoord(f.X, LengthAxisX, bbox.Min.X, bw)
	regY := regionCoord(f.Y, LengthAxisY, bbox.Min.Y, bh)
	regW := regionCoord(f.W, LengthAxisX, bbox.Min.X, bw) - regX
	regH := regionCoord(f.H, LengthAxisY, bbox.Min.Y, bh) - regY
	if regW <= 0 || regH <= 0 {
		return false
	}
	rw := int(regW)
	rh := int(regH)
	if rw <= 0 || rh <= 0 {
		return false
	}
	// SourceGraphic offscreen: content shifted so the filter region origin
	// is the offscreen origin (blur/offset spills land inside the region).
	src := render.NewContext(rw, rh)
	src.Translate(-regX, -regY)
	fillAndStroke(src, a, p, st, bbox, func() {
		src.DrawPath(combined)
	})

	// Named images: SourceGraphic / SourceAlpha / result names.
	images := map[string]*render.ImageBuf{
		"SourceGraphic": render.ImageBufFromImage(src.Image()),
	}
	alpha := image.NewRGBA(src.Image().(*imageRGBA).Rect)
	for y := 0; y < rh; y++ {
		for x := 0; x < rw; x++ {
			c0 := src.Image().(*imageRGBA).RGBAAt(x, y)
			alpha.SetRGBA(x, y, c0)
		}
	}
	images["SourceAlpha"] = render.ImageBufFromImage(alpha)

	last := ""
	// applyTo runs `apply` on a fresh offscreen over the input image and
	// returns the result as an ImageBuf (Apply* rides the GPU filter path).
	applyTo := func(buf *render.ImageBuf, apply func(*render.Context)) *render.ImageBuf {
		work := render.NewContext(rw, rh)
		work.DrawImage(buf, 0, 0)
		apply(work)
		return render.ImageBufFromImage(work.Image())
	}
	for _, fp := range f.Primitives {
		in := fp.In
		if in == "" {
			if last == "" {
				in = "SourceGraphic"
			} else {
				in = last
			}
		}
		inBuf, ok := images[in]
		if !ok {
			return false
		}
		var out *render.ImageBuf
		switch fp.Kind {
		case "feGaussianBlur":
			sx, sy := blurSigmas(fp.Numbers)
			out = applyTo(inBuf, func(w *render.Context) {
				if sx > 0 && sy > 0 {
					w.ApplyBlurXY(sx, sy)
				} else if sx > 0 {
					w.ApplyBlur(sx)
				} else if sy > 0 {
					w.ApplyBlur(sy)
				}
			})
		case "feColorMatrix":
			m, okM := colorMatrixValues(fp)
			if !okM {
				return false
			}
			out = applyTo(inBuf, func(w *render.Context) {
				w.ApplyColorMatrix(m)
			})
		case "feDropShadow":
			dx, dy, sigma := dropShadowParams(fp)
			col := colorFromHexOr(fp.FloodColor, render.RGBA{A: 1})
			out = applyTo(inBuf, func(w *render.Context) {
				w.ApplyDropShadow(dx, dy, sigma, col)
			})
		case "feOffset":
			dx, dy := offsetDeltas(fp)
			w := render.NewContext(rw, rh)
			w.DrawImage(inBuf, dx, dy)
			out = render.ImageBufFromImage(w.Image())
		case "feFlood":
			w := render.NewContext(rw, rh)
			col := colorFromHexOr(fp.FloodColor, render.RGBA{R: 1, G: 1, B: 1, A: fp.FloodOpacity})
			if fp.FloodOpacity < 1 {
				w.SetRGBA(col.R, col.G, col.B, col.A*fp.FloodOpacity)
			} else {
				w.SetRGBA(col.R, col.G, col.B, col.A)
			}
			w.DrawRectangle(0, 0, float64(rw), float64(rh))
			_ = w.Fill()
			out = render.ImageBufFromImage(w.Image())
		case "feBlend", "feComposite":
			mode, okM := blendModeFor(fp.Kind, fp.Str1)
			if !okM {
				return false
			}
			in2 := fp.In2
			if in2 == "" {
				in2 = "SourceGraphic"
			}
			in2Buf, ok2 := images[in2]
			if !ok2 {
				return false
			}
			w := render.NewContext(rw, rh)
			w.DrawImage(in2Buf, 0, 0) // destination
			opts := render.DrawImageOptions{X: 0, Y: 0, BlendMode: mode}
			w.DrawImageEx(inBuf, opts) // source with blend
			out = render.ImageBufFromImage(w.Image())
		case "feMerge":
			w := render.NewContext(rw, rh)
			for _, name := range fp.MergeNodes {
				if ib, ok2 := images[name]; ok2 {
					w.DrawImage(ib, 0, 0)
				}
			}
			out = render.ImageBufFromImage(w.Image())
		}
		if out == nil {
			return false
		}
		if fp.Result != "" {
			images[fp.Result] = out
		}
		last = fp.Result
		images["__last__"] = out
	}
	outBuf := images["__last__"]
	if outBuf == nil {
		if lastBuf, ok := images[last]; ok {
			outBuf = lastBuf
		} else {
			return false
		}
	}
	dc.DrawImage(outBuf, regX, regY)
	return true
}

// blurSigmas extracts (stdDeviationX, stdDeviationY) from the parsed list.
func blurSigmas(nums []float64) (float64, float64) {
	switch len(nums) {
	case 0:
		return 0, 0
	case 1:
		return nums[0], nums[0]
	default:
		return nums[0], nums[1]
	}
}

// dropShadowParams extracts (dx, dy, stdDeviation) from feDropShadow.
func dropShadowParams(fp *FilterPrimitive) (float64, float64, float64) {
	dx, dy, sigma := 0.0, 0.0, 0.0
	for _, attrVal := range []string{} {
		_ = attrVal
	}
	if fp.HasNumber {
		dx = fp.Number
	}
	if len(fp.Numbers) >= 1 {
		dy = fp.Numbers[0]
	}
	if len(fp.Numbers2) >= 1 {
		sigma = fp.Numbers2[0]
	}
	return dx, dy, sigma
}

// offsetDeltas extracts (dx, dy) from feOffset.
func offsetDeltas(fp *FilterPrimitive) (float64, float64) {
	if len(fp.Numbers) >= 2 {
		return fp.Numbers[0], fp.Numbers[1]
	}
	if fp.HasNumber {
		return fp.Number, 0
	}
	return 0, 0
}

// colorFromHexOr parses a hex/named color, falling back to def.
func colorFromHexOr(s string, def render.RGBA) render.RGBA {
	c, err := parseColor(s)
	if err != nil || c == nil {
		return def
	}
	r, g, b, a := c.RGBA()
	return render.RGBA{R: float64(r>>8) / 255, G: float64(g>>8) / 255, B: float64(b>>8) / 255, A: float64(a>>8) / 255}
}
