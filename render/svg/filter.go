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
// Named refusals (registered, design §4.1): feComposite arithmetic (until
// 4b-2), feTile, feImage, feTurbulence (until 4b-3), feDisplacementMap,
// feComponentTransfer, feConvolveMatrix (until 4b-3), feMorphology, and the
// lighting family (until 4b-2). A filter that contains an unsupported
// primitive is NOT applied — the element renders unfiltered (CSS Masking
// error口径) — and the refusal is a warning.

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

	// P3.8 4b-1: filter primitive subregion (x/y/width/height in
	// primitiveUnits). Per-attr fallback: filter region (userSpaceOnUse) or
	// 0/0/1/1 fractions of the object bbox (objectBoundingBox).
	SX, SY, SW, SH       Length
	SXSet, SYSet, SWSet, SHSet bool
	// feMorphology: operator (default erode) + radius list (resvg rule:
	// 1 value → both axes, 2 → x/y, otherwise invalid → 0/0 → transparent).
	Operator string
	Radius   []float64
	// feDisplacementMap: channel selectors (0=R, 1=G, 2=B, 3=A; default A).
	XSel, YSel int
	Scale      float64
	HasScale   bool
	// feComponentTransfer: per-channel feFuncR/G/B/A (nil = identity).
	Funcs [4]*TransferFunc
	// feImage: href (element ref "#id" resolves at render; data: URI decodes
	// at parse into Img), preserveAspectRatio raw.
	Href   string
	Aspect string
	Img    *ImageElement
	// P3.8 4b-2: feDiffuseLighting/feSpecularLighting + light source child;
	// feComposite arithmetic coefficients.
	SurfaceScale     float64
	DiffuseConstant  float64
	SpecularConstant float64
	SpecularExponent float64
	LightingColor    string
	Light            *LightSource
	K1, K2, K3, K4   float64
	// P3.8 4b-3: feTurbulence (Str1 carries type; the fractalNoise match is
	// exact like usvg) + feConvolveMatrix. kernelUnitLength is parsed by
	// resvg but unused — not carried.
	BaseFrequency []float64
	NumOctaves    float64
	HasNumOctaves bool
	Seed          float64
	HasSeed       bool
	StitchTiles   bool
	Order         []float64
	KernelMatrix  []float64
	Divisor       float64
	HasDivisor    bool
	Bias          float64
	HasBias       bool
	TargetX, TargetY int
	HasTargetX, HasTargetY bool
	EdgeMode      string
	PreserveAlpha bool
}

// LightSource is one feDistantLight/fePointLight/feSpotLight child.
type LightSource struct {
	Type                             int // 0=distant, 1=point, 2=spot
	Azimuth, Elevation               float64
	X, Y, Z                          float64
	PointsAtX, PointsAtY, PointsAtZ  float64
	SpotSpecularExponent             float64
	LimitingConeAngle                float64
	HasLimitingCone                  bool
}

// TransferFunc is one feFuncR/G/B/A transfer function of feComponentTransfer
// (SVG Filter Effects §10: identity/table/discrete/linear/gamma).
type TransferFunc struct {
	Type        string // normalized lowercase; unknown → identity
	TableValues []float64
	Slope       float64 // linear (default 1)
	Intercept   float64 // linear
	Amplitude   float64 // gamma (default 1)
	Exponent    float64 // gamma (default 1)
	Offset      float64 // gamma
}

// FilterElement represents <filter>.
type FilterElement struct {
	Attrs        Attrs
	X, Y         Length // region (filterUnits; default -10%..120% of bbox)
	W, H         Length
	Units        Units // filterUnits (default objectBoundingBox)
	ContentUnits Units // primitiveUnits (default userSpaceOnUse)
	LinearRGB    bool  // color-interpolation-filters (spec default true)
	// FloodColor/FloodOpacity carry the <filter> element's own raw values —
	// a feFlood flood-color="inherit" reads them (non-inheritable attributes
	// inherit from the direct parent only, usvg resolve_inherit).
	FloodColor   string
	FloodOpacity string
	Primitives   []*FilterPrimitive
}

func (e *FilterElement) attrs() *Attrs { return &e.Attrs }

// supportedFilterPrimitive reports whether the fe* kind maps onto the
// render filter family; unsupported kinds are named refusals.
func supportedFilterPrimitive(kind string) bool {
	switch kind {
	case "feGaussianBlur", "feOffset", "feFlood", "feColorMatrix",
		"feBlend", "feComposite", "feMerge", "feDropShadow",
		"feMorphology", "feTile", "feDisplacementMap", "feComponentTransfer", "feImage",
		"feDiffuseLighting", "feSpecularLighting",
		"feTurbulence", "feConvolveMatrix":
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

// subRegion is one filter primitive's subregion in user-space canvas
// coordinates (the offscreen maps it 1:1 with the (regX, regY) origin
// subtracted). resvg carries the same per-result region for feTile cells and
// the feOffset inheritance quirk.
type subRegion struct{ x, y, w, h float64 }

// sameAs compares on rounded ints (resvg resolves subregions to IntRect).
func (s subRegion) sameAs(o subRegion) bool {
	same := func(a, b float64) bool { return int(math.Round(a)) == int(math.Round(b)) }
	return same(s.x, o.x) && same(s.y, o.y) && same(s.w, o.w) && same(s.h, o.h)
}

// renderFilterElement evaluates a filter chain for one element and composites
// the result back. Pipeline: the element content renders into the source
// offscreen (SourceGraphic), each primitive runs on its own offscreen Context
// (Apply* rides the GPU filter path), the last output composites back.
// Returns false when the filter must not be applied (unsupported primitive /
// dangling ref already warned) — the caller then renders the element
// unfiltered (CSS error口径).
func (d *Document) renderFilterElement(dc *render.Context, combined *render.Path, bbox render.Rect, f *FilterElement, a *Attrs, p *props, filterID string, st *renderState) bool {
	// feImage recursion breaker: while this filter is being evaluated, a
	// referenced subtree carrying the same filter renders unfiltered
	// (resvg's in-conversion cache hit returns an empty filter — same
	// observable semantics, see renderState.filterRefs).
	if st.filterRefs == nil {
		st.filterRefs = map[string]bool{}
	}
	st.filterRefs[filterID] = true
	defer delete(st.filterRefs, filterID)

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
	// Region size ceiling: the SVG spec bounds filter regions only by the
	// element's percentages, and resvg's huge-radius cases hand us regions
	// far beyond any sane surface. Above the ceiling the filter is not
	// applied (the element paints unfiltered — the CSS error stance this
	// package already uses), instead of allocating gigabytes offscreen.
	if rw > maxFilterRegionSide || rh > maxFilterRegionSide || rw*rh > maxFilterRegionPixels {
		return false
	}
	// Per-primitive subregion machinery (resvg filter/mod.rs orchestration):
	// every result carries its region; a primitive whose subregion differs
	// from the filter region gets its output clipped to it (feOffset is the
	// registered exception — it inherits its input's region unclipped).
	full := subRegion{regX, regY, regW, regH}
	regionOf := map[string]subRegion{"SourceGraphic": full, "SourceAlpha": full}
	resolveSubRegion := func(fp *FilterPrimitive) subRegion {
		// userSpaceOnUse primitiveUnits: per-attr fallback to the filter
		// region; objectBoundingBox: fractions of the element bbox with
		// per-attr fallback to 0/0/1/1 (resvg resolve_primitive_region).
		frac := func(l Length, set bool, defFrac float64, dim float64) float64 {
			if !set {
				return defFrac * dim
			}
			v := l.Value
			if l.Unit == LengthUnitPercent {
				v /= 100
			}
			return v * dim
		}
		user := func(l Length, set bool, def float64, axis LengthAxis, dim float64) float64 {
			if !set {
				return def
			}
			return l.Resolve(RefSpace{ViewportW: dim, ViewportH: dim}, axis)
		}
		if f.ContentUnits == UnitsObjectBoundingBox {
			return subRegion{
				x: bbox.Min.X + frac(fp.SX, fp.SXSet, 0, bw),
				y: bbox.Min.Y + frac(fp.SY, fp.SYSet, 0, bh),
				w: frac(fp.SW, fp.SWSet, 1, bw),
				h: frac(fp.SH, fp.SHSet, 1, bh),
			}
		}
		return subRegion{
			x: user(fp.SX, fp.SXSet, regX, LengthAxisX, bw),
			y: user(fp.SY, fp.SYSet, regY, LengthAxisY, bh),
			w: user(fp.SW, fp.SWSet, regW, LengthAxisX, bw),
			h: user(fp.SH, fp.SHSet, regH, LengthAxisY, bh),
		}
	}
	clipToSubRegion := func(buf *render.ImageBuf, sr subRegion) *render.ImageBuf {
		cw := newSVGOffscreen(rw, rh)
		defer cw.Close()
		cw.ClipRect(sr.x-regX, sr.y-regY, sr.w, sr.h)
		cw.DrawImage(buf, 0, 0)
		out := render.ImageBufFromImage(cw.Image())
		buf.Dispose()
		return out
	}

	// SourceGraphic offscreen: content shifted so the filter region origin
	// is the offscreen origin (blur/offset spills land inside the region).
	src := newSVGOffscreen(rw, rh)
	defer src.Close()
	src.Translate(-regX, -regY)
	fillAndStroke(src, a, p, st, bbox, func() {
		src.DrawPath(combined)
	})

	// Named images: SourceGraphic / SourceAlpha / result names.
	// src.Image() materializes a fresh RGBA every call (FlushGPU + full
	// copy) — hoist it once; the alpha twin is a per-row copy of the same
	// pixels (bit-identical to the previous per-pixel loop, without the
	// 2×rw×rh redundant materializations that dominated bake time).
	srcImg, _ := src.Image().(*image.RGBA)
	images := map[string]*render.ImageBuf{
		"SourceGraphic": render.ImageBufFromImage(srcImg),
	}
	alpha := image.NewRGBA(srcImg.Rect)
	for y := 0; y < rh; y++ {
		row := srcImg.Pix[y*srcImg.Stride:]
		dstRow := alpha.Pix[y*alpha.Stride:]
		copy(dstRow[:rw*4], row[:rw*4])
	}
	images["SourceAlpha"] = render.ImageBufFromImage(alpha)

	last := ""
	// applyTo runs `apply` on a fresh offscreen over the input image and
	// returns the result as an ImageBuf (Apply* rides the CPU filter path; the
	// offscreen is CPUOnly, matching the deterministic Document.Render bake).
	applyTo := func(buf *render.ImageBuf, apply func(*render.Context)) *render.ImageBuf {
		work := newSVGOffscreen(rw, rh)
		defer work.Close()
		work.DrawImage(buf, 0, 0)
		apply(work)
		return render.ImageBufFromImage(work.Image())
	}
	for _, fp := range f.Primitives {
		in := fp.In
		if in == "" {
			// resvg resolve_input: a non-first primitive's default input is
			// the PREVIOUS primitive's output, even when that result is
			// unnamed (images["__last__"] carries it).
			if _, okL := images["__last__"]; okL {
				in = "__last__"
			} else {
				in = "SourceGraphic"
			}
		}
		inBuf, ok := images[in]
		if !ok {
			disposeAll(images)
			disposeAll(images)
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
				disposeAll(images)
				disposeAll(images)
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
			w := newSVGOffscreen(rw, rh)
			defer w.Close()
			w.DrawImage(inBuf, dx, dy)
			out = render.ImageBufFromImage(w.Image())
		case "feFlood":
			// Spec default: black. flood-color is NOT inheritable, but an
			// explicit "inherit" keyword takes the DIRECT parent's (the
			// <filter> element's) value — usvg parse.rs resolve_inherit
			// non-inheritable branch, one level only, missing → black.
			// flood-opacity="inherit" follows the same rule in usvg and
			// stays a registered gap here (no corpus coverage).
			colStr := fp.FloodColor
			if strings.TrimSpace(colStr) == "inherit" {
				colStr = f.FloodColor
				if strings.TrimSpace(colStr) == "inherit" {
					colStr = "" // nested inherit on <filter>: simplification → black
				}
			}
			w := newSVGOffscreen(rw, rh)
			defer w.Close()
			col := colorFromHexOr(colStr, render.RGBA{A: 1}) // opaque black
			w.SetRGBA(col.R, col.G, col.B, col.A*fp.FloodOpacity)
			w.DrawRectangle(0, 0, float64(rw), float64(rh))
			_ = w.Fill()
			out = render.ImageBufFromImage(w.Image())
		case "feBlend", "feComposite":
			if fp.Kind == "feComposite" && strings.ToLower(strings.TrimSpace(fp.Str1)) == "arithmetic" {
				// resvg composite.rs: premul-domain k-coefficients, RGB
				// clamped to the new alpha, alpha≈0 → transparent.
				in2a := fp.In2
				if in2a == "" {
					in2a = "SourceGraphic"
				}
				in2A, okA := images[in2a]
				if !okA {
					disposeAll(images)
					return false
				}
				out = applyTo(inBuf, func(w *render.Context) {
					w.ApplyCompositeArithmetic(
						float32(fp.K1), float32(fp.K2), float32(fp.K3), float32(fp.K4), in2A, f.LinearRGB)
				})
				break
			}
			mode, okM := blendModeFor(fp.Kind, fp.Str1)
			if !okM {
				disposeAll(images)
				disposeAll(images)
				return false
			}
			in2 := fp.In2
			if in2 == "" {
				in2 = "SourceGraphic"
			}
			in2Buf, ok2 := images[in2]
			if !ok2 {
				disposeAll(images)
				disposeAll(images)
				return false
			}
			w := newSVGOffscreen(rw, rh)
			defer w.Close()
			w.DrawImage(in2Buf, 0, 0) // destination
			opts := render.DrawImageOptions{X: 0, Y: 0, BlendMode: mode}
			w.DrawImageEx(inBuf, opts) // source with blend
			out = render.ImageBufFromImage(w.Image())
		case "feMerge":
			w := newSVGOffscreen(rw, rh)
			defer w.Close()
			for _, name := range fp.MergeNodes {
				if name == "" {
					// feMergeNode without `in` = previous primitive's result
					// (resolve_input default, same rule as fp.In above).
					if _, okL := images["__last__"]; okL {
						name = "__last__"
					} else {
						name = "SourceGraphic"
					}
				}
				if ib, ok2 := images[name]; ok2 {
					w.DrawImage(ib, 0, 0)
				}
			}
			out = render.ImageBufFromImage(w.Image())
		case "feMorphology":
			// usvg convert_morphology: the radius resolves against the
			// primitiveUnits scale (userSpaceOnUse = 1; oBB = element bbox —
			// also the missing-attribute default); zeros reset to 1 and
			// negatives keep the default, so the result is always positive.
			defX, defY := 1.0, 1.0
			if f.ContentUnits == UnitsObjectBoundingBox {
				defX, defY = bw, bh
			}
			rx, ry := morphologyRadii(fp.Radius, defX, defY)
			out = applyTo(inBuf, func(w *render.Context) {
				irx, iry := int(math.Ceil(rx)), int(math.Ceil(ry))
				if fp.Operator == "dilate" {
					w.ApplyDilateXY(irx, iry)
				} else {
					w.ApplyErodeXY(irx, iry)
				}
			})
		case "feTile":
			// resvg apply_tile: the INPUT's region is the tile cell; the
			// whole filter region is filled with repeats of it (ApplyTile
			// phase anchors the cell at its own origin — resvg-exact).
			cell, okC := regionOf[in]
			if !okC {
				cell = full
			}
			w := newSVGOffscreen(rw, rh)
			defer w.Close()
			w.DrawImage(inBuf, 0, 0)
			w.ApplyTile([4]float32{
				float32(cell.x - regX), float32(cell.y - regY),
				float32(cell.x - regX + cell.w), float32(cell.y - regY + cell.h),
			})
			out = render.ImageBufFromImage(w.Image())
		case "feDisplacementMap":
			in2 := fp.In2
			if in2 == "" {
				in2 = "SourceGraphic"
			}
			in2Buf, ok2 := images[in2]
			if !ok2 {
				disposeAll(images)
				return false
			}
			scale := 0.0
			if fp.HasScale {
				scale = fp.Scale
			}
			out = applyTo(inBuf, func(w *render.Context) {
				w.ApplyDisplacementMap(fp.XSel, fp.YSel, int(math.Round(scale)), in2Buf)
			})
		case "feComponentTransfer":
			// resvg apply_component_transfer: demultiply → per-channel
			// transfer → premultiply — exactly ApplyColorFilter's storage
			// round trip; the five SVG types compile into 256-entry LUTs.
			lutR, lutG, lutB, lutA := compileTransferLUTs(fp.Funcs)
			out = applyTo(inBuf, func(w *render.Context) {
				w.ApplyColorFilter(render.ColorFilterFunc(func(r, g, b, a float32) (float32, float32, float32, float32) {
					return lutApply(lutR, r), lutApply(lutG, g), lutApply(lutB, b), lutApply(lutA, a)
				}))
			})
		case "feImage":
			sr := resolveSubRegion(fp)
			out = d.renderFeImagePrimitive(rw, rh, regX, regY, fp, sr, st)
		case "feDiffuseLighting", "feSpecularLighting":
			spec := fp.Kind == "feSpecularLighting"
			if spec && (fp.SpecularExponent < 1 || fp.SpecularExponent > 128) {
				// resvg usvg: specularExponent outside [1,128] → dummy
				// primitive → transparent output.
				w := newSVGOffscreen(rw, rh)
				defer w.Close()
				out = render.ImageBufFromImage(w.Image())
			} else {
				params := render.SVGLightingParams{
					Specular:         spec,
					SurfaceScale:     float32(fp.SurfaceScale),
					DiffuseConstant:  float32(fp.DiffuseConstant),
					SpecularConstant: float32(fp.SpecularConstant),
					SpecularExponent: float32(fp.SpecularExponent),
					// lighting-color defaults to white (spec).
					LightColor: colorFromHexOr(fp.LightingColor, render.RGBA{R: 1, G: 1, B: 1, A: 1}),
					LinearRGB:  f.LinearRGB,
				}
				if ls := fp.Light; ls != nil {
					params.LightType = ls.Type
					params.Azimuth = float32(ls.Azimuth)
					params.Elevation = float32(ls.Elevation)
					params.LightX, params.LightY, params.LightZ = float32(ls.X), float32(ls.Y), float32(ls.Z)
					params.PointsAtX, params.PointsAtY, params.PointsAtZ = float32(ls.PointsAtX), float32(ls.PointsAtY), float32(ls.PointsAtZ)
					params.SpotSpecularExponent = float32(ls.SpotSpecularExponent)
					params.LimitingConeAngle = float32(ls.LimitingConeAngle)
					params.HasLimitingCone = ls.HasLimitingCone
				}
				out = applyTo(inBuf, func(w *render.Context) {
					w.ApplySVGLighting(params)
				})
			}
		case "feTurbulence":
			// resvg apply_turbulence: the noise generates over the FULL
			// filter region buffer (subregion clipping is the generic
			// machinery below) and has no input. Coordinates: resvg samples
			// at (pixel + region_offset) / canvas scale; this engine's
			// filter buffers are user-space 1:1 (region origin subtracted at
			// setup), so the sample coordinate is pixel + region origin.
			w := newSVGOffscreen(rw, rh)
			defer w.Close()
			w.ApplySVGTurbulence(turbulenceParams(fp, regX, regY, f.LinearRGB))
			out = render.ImageBufFromImage(w.Image())
		case "feConvolveMatrix":
			if p, okP := convolveMatrixParams(fp, f.LinearRGB); okP {
				out = applyTo(inBuf, func(w *render.Context) {
					w.ApplySVGConvolveMatrix(p)
				})
			} else {
				// usvg convert_convolve_matrix → None → dummy primitive
				// (transparent black flood): divisor≈0, target out of
				// [0,order), or kernelMatrix count ≠ order_x·order_y.
				w := newSVGOffscreen(rw, rh)
				defer w.Close()
				out = render.ImageBufFromImage(w.Image())
			}
		}
		if out == nil {
			disposeAll(images)
			return false
		}
		// Subregion bookkeeping: clip the output (feOffset inherits its
		// input's region and is not clipped — resvg's registered quirk) and
		// record the result region for feTile cells / downstream inheritance.
		sr := resolveSubRegion(fp)
		if fp.Kind == "feOffset" {
			if r, okR := regionOf[in]; okR {
				sr = r
			}
		} else if !sr.sameAs(full) {
			out = clipToSubRegion(out, sr)
		}
		if fp.Result != "" {
			images[fp.Result] = out
			regionOf[fp.Result] = sr
		}
		last = fp.Result
		images["__last__"] = out
		regionOf["__last__"] = sr
	}
	outBuf := images["__last__"]
	if outBuf == nil {
		if lastBuf, ok := images[last]; ok {
			outBuf = lastBuf
		} else {
			disposeAll(images)
			disposeAll(images)
			disposeAll(images)
			return false
		}
	}
	dc.DrawImage(outBuf, regX, regY)
	disposeAll(images)
	return true
}

// disposeAll releases every intermediate ImageBuf's GPU texture. The
// evaluator builds one buf per primitive per element; without this the
// texture registry pins them until process end (batch-6 conformance run
// OOMed at case ~200 on exactly this).
func disposeAll(images map[string]*render.ImageBuf) {
	for _, buf := range images {
		if buf != nil {
			buf.Dispose()
		}
	}
}

// Filter-region ceilings: 4096 px per side / 4096×4096 total area — the
// same guard the COLRv1 raster uses. Cases beyond it paint unfiltered.
const (
	maxFilterRegionSide   = 4096
	maxFilterRegionPixels = 4096 * 4096
)

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
	// Canonical layout (parseFilterPrimitive feOffset branch): Number=dx,
	// Numbers[0]=dy — the same contract dropShadowParams reads.
	dx, dy := 0.0, 0.0
	if fp.HasNumber {
		dx = fp.Number
	}
	if len(fp.Numbers) >= 1 {
		dy = fp.Numbers[0]
	}
	return dx, dy
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

// morphologyRadii applies the resvg usvg convert_morphology radius rules
// (verbatim): a 1-value list → both axes, 2 values → x/y, any other count →
// 0; zeros reset to 1 (the Chrome/Safari quirk usvg mirrors — "not
// specified in the spec"); a negative on either axis keeps the DEFAULT on
// both axes; the result multiplies by the primitiveUnits scale (defX/defY:
// userSpaceOnUse = 1, objectBoundingBox = element bbox — the same value a
// missing attribute falls back to). The evaluator therefore never sees a
// non-positive radius.
func morphologyRadii(nums []float64, defX, defY float64) (float64, float64) {
	rx, ry := defX, defY
	if len(nums) > 0 {
		x, y := 0.0, 0.0
		switch len(nums) {
		case 1:
			x, y = nums[0], nums[0]
		case 2:
			x, y = nums[0], nums[1]
		}
		if !(math.Signbit(x) || math.Signbit(y)) {
			if x == 0 {
				x = 1
			}
			if y == 0 {
				y = 1
			}
			rx, ry = x*defX, y*defY
		}
	}
	return rx, ry
}

// turbulenceParams resolves feTurbulence into the public parameter set
// (usvg convert_turbulence verbatim): baseFrequency is a 1–2 value list
// (invalid/negative-by-sign → both 0), numOctaves defaults 1 with
// sign-negative → 0 and rounding, seed truncates, type matches
// "fractalNoise" exactly (anything else → turbulence).
func turbulenceParams(fp *FilterPrimitive, regX, regY float64, linearRGB bool) render.SVGTurbulenceParams {
	bfx, bfy := 0.0, 0.0
	switch len(fp.BaseFrequency) {
	case 1:
		bfx, bfy = fp.BaseFrequency[0], fp.BaseFrequency[0]
	case 2:
		bfx, bfy = fp.BaseFrequency[0], fp.BaseFrequency[1]
	}
	if math.Signbit(bfx) || math.Signbit(bfy) {
		bfx, bfy = 0, 0
	}
	octaves := 1.0
	if fp.HasNumOctaves {
		octaves = fp.NumOctaves
	}
	if math.Signbit(octaves) {
		octaves = 0
	}
	o := int(math.Round(octaves))
	// resvg casts the rounded count to u32 (saturating); a hostile doc with
	// a billions-of-octaves value would hang it the same way — the ceiling
	// keeps the engine live and is far beyond any sane octave count.
	if o > 256 {
		o = 256
	}
	seed := int32(0)
	if fp.HasSeed {
		s := float64(float32(fp.Seed)) // usvg parses the seed as f32
		if s > math.MaxInt32 {
			seed = math.MaxInt32
		} else if s < math.MinInt32 {
			seed = math.MinInt32
		} else {
			seed = int32(s) // trunc() — 1.5 → 1
		}
	}
	return render.SVGTurbulenceParams{
		OffsetX:        float32(regX),
		OffsetY:        float32(regY),
		BaseFrequencyX: float32(bfx),
		BaseFrequencyY: float32(bfy),
		NumOctaves:     o,
		Seed:           seed,
		StitchTiles:    fp.StitchTiles,
		FractalNoise:   fp.Str1 == "fractalNoise",
		LinearRGB:      linearRGB,
	}
}

// convolveMatrixParams resolves feConvolveMatrix into the public parameter
// set (usvg convert_convolve_matrix verbatim). ok=false reproduces the
// usvg → dummy-primitive cases: divisor≈0, targetX/Y outside [0,order), or
// a kernelMatrix whose count ≠ order_x·order_y.
func convolveMatrixParams(fp *FilterPrimitive, linearRGB bool) (render.SVGConvolveMatrixParams, bool) {
	// order: first value → columns, second → rows (default rows = columns);
	// non-positive values keep the 3×3 default (usvg rule).
	cols, rows := 3, 3
	if len(fp.Order) > 0 {
		x := int(fp.Order[0])
		y := x
		if len(fp.Order) > 1 {
			y = int(fp.Order[1])
		}
		if x > 0 && y > 0 {
			cols, rows = x, y
		}
	}
	// kernelMatrix: used only when the count matches cols*rows exactly
	// (usvg); otherwise it stays empty → ConvolveMatrixData::new fails →
	// dummy.
	matrix := fp.KernelMatrix
	if len(matrix) != cols*rows {
		matrix = nil
	}
	// divisor: explicit attribute, else the kernel sum rounded to 1e-6 with
	// the approx-zero → 1.0 fallback (usvg).
	divisor := 0.0
	if fp.HasDivisor {
		divisor = fp.Divisor
		if svgF32ApproxZeroWide(divisor) {
			return render.SVGConvolveMatrixParams{}, false
		}
	} else {
		sum := float32(0)
		for _, kv := range matrix {
			sum += float32(kv)
		}
		sum = float32(math.Round(float64(sum)*1e6) / 1e6)
		if svgF32ApproxZeroWide(float64(sum)) {
			sum = 1.0
		}
		divisor = float64(sum)
	}
	targetX, targetY := cols/2, rows/2
	if fp.HasTargetX {
		if fp.TargetX < 0 || fp.TargetX >= cols {
			return render.SVGConvolveMatrixParams{}, false
		}
		targetX = fp.TargetX
	}
	if fp.HasTargetY {
		if fp.TargetY < 0 || fp.TargetY >= rows {
			return render.SVGConvolveMatrixParams{}, false
		}
		targetY = fp.TargetY
	}
	if len(matrix) != cols*rows { // usvg ConvolveMatrixData::new → None
		return render.SVGConvolveMatrixParams{}, false
	}
	edge := render.SVGEdgeModeDuplicate
	switch fp.EdgeMode {
	case "none":
		edge = render.SVGEdgeModeNone
	case "wrap":
		edge = render.SVGEdgeModeWrap
	}
	kernel := make([]float32, len(matrix))
	for i, kv := range matrix {
		kernel[i] = float32(kv)
	}
	return render.SVGConvolveMatrixParams{
		Columns:       cols,
		Rows:          rows,
		Kernel:        kernel,
		Divisor:       float32(divisor),
		Bias:          float32(fp.Bias),
		TargetX:       targetX,
		TargetY:       targetY,
		EdgeMode:      edge,
		PreserveAlpha: fp.PreserveAlpha,
		LinearRGB:     linearRGB,
	}, true
}

// svgF32ApproxZeroWide is the svg-layer handle on the internal
// ApproxZeroUlps(4)-on-f32 check (see internal/filter).
func svgF32ApproxZeroWide(v float64) bool {
	return math.Abs(v) <= 5.605193966189168e-45 // 4 × f32 denormal ULP
}

// compileTransferLUTs bakes the four feFunc* transfer functions into
// 256-entry LUTs (nil = channel unchanged: identity, or an empty
// tableValues list — resvg is_dummy semantics). Formulas are resvg
// component_transfer.rs verbatim (SVG Filter Effects §10); the output
// clamps to [0,1] and truncates to u8 like resvg's `as u8`.
func compileTransferLUTs(funcs [4]*TransferFunc) (r, g, b, a []uint8) {
	compile := func(fn *TransferFunc) []uint8 {
		if fn == nil {
			return nil
		}
		switch fn.Type {
		case "table", "discrete":
			if len(fn.TableValues) == 0 {
				return nil
			}
		case "linear", "gamma":
		default: // identity (unknown types normalize at parse)
			return nil
		}
		t := make([]uint8, 256)
		for i := 0; i < 256; i++ {
			c := float64(i) / 255
			var v float64
			switch fn.Type {
			case "table":
				n := len(fn.TableValues) - 1
				k := int(math.Floor(c * float64(n)))
				if k > n {
					k = n
				}
				if k == n {
					v = fn.TableValues[k]
				} else {
					vk, vk1 := fn.TableValues[k], fn.TableValues[k+1]
					v = vk + (c-float64(k)/float64(n))*float64(n)*(vk1-vk)
				}
			case "discrete":
				n := len(fn.TableValues)
				k := int(math.Floor(c * float64(n)))
				if k > n-1 {
					k = n - 1
				}
				v = fn.TableValues[k]
			case "linear":
				v = fn.Slope*c + fn.Intercept
			case "gamma":
				v = fn.Amplitude*math.Pow(c, fn.Exponent) + fn.Offset
			}
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			t[i] = uint8(v * 255)
		}
		return t
	}
	return compile(funcs[0]), compile(funcs[1]), compile(funcs[2]), compile(funcs[3])
}

// lutApply looks one straight channel up in a transfer LUT (nil = identity).
func lutApply(tab []uint8, v float32) float32 {
	if len(tab) != 256 {
		return v
	}
	return float32(tab[int(v*255+0.5)]) / 255
}

// renderFeImagePrimitive produces the feImage output: an external/data-URI
// raster fitted into the primitive subregion (resvg convert_inner), or the
// referenced element subtree rendered in its own user space translated to
// the subregion origin (resvg apply_image). Missing/invalid href → a
// transparent image (resvg dummy-primitive stance).
func (d *Document) renderFeImagePrimitive(rw, rh int, regX, regY float64, fp *FilterPrimitive, sr subRegion, st *renderState) *render.ImageBuf {
	w := newSVGOffscreen(rw, rh)
	defer w.Close()
	if fp.Img != nil {
		if buf := fp.Img.renderBuf(); buf != nil {
			w.Push()
			w.Translate(sr.x-regX, sr.y-regY)
			w.ClipRect(0, 0, sr.w, sr.h)
			w.Transform(viewportTransform(sr.w, sr.h, ViewBox{0, 0, float64(fp.Img.ImgW), float64(fp.Img.ImgH)}, fp.Img.Par))
			w.DrawImage(buf, 0, 0)
			w.Pop()
			buf.Dispose()
			fp.Img.buf = nil // the cached buf is now disposed — re-evals re-decode
		}
	} else if ref := strings.TrimSpace(fp.Href); strings.HasPrefix(ref, "#") {
		if target := d.ElementByID(strings.TrimPrefix(ref, "#")); target != nil {
			w.Push()
			w.Translate(sr.x-regX, sr.y-regY)
			// The referenced subtree renders with the CURRENT filter state:
			// a subtree whose own filter is in-progress renders unfiltered
			// (st.filterRefs — the resvg self-recursive termination).
			inner := *st
			renderElement(w, target, evalProps(rootProps(), *target.attrs(), st.doc), &inner)
			w.Pop()
		}
	}
	return render.ImageBufFromImage(w.Image())
}
