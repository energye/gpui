//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"encoding/binary"
	"math"
	"reflect"
)

// PathEffect transforms one path into another before the stroke is expanded
// (P3.5 batch 4, SkPathEffect analogue). It is mounted on Stroke.PathEffect
// and consumed at the same sites the dash pattern uses today.
//
// Contract: implementations must be pure functions of their parameters —
// callers may cache output keyed by the path content plus the effect hash,
// and the recording layer stores the effect by reference (record-time value,
// replay-time read-only, same contract as CustomPatternBrush).
//
// FilterPath returns ok=false when the effect cannot be applied (e.g. a
// degenerate prototype); consumers then draw nothing for that stroke, which
// matches Skia (filterPath false leaves no geometry).
type PathEffect interface {
	FilterPath(src *Path) (*Path, bool)
}

// pathEffectInternal is the optional internal contract the built-in effects
// implement: a deterministic content hash for cache keys and a scaled copy
// for device-space consumers that must map user-space effect parameters by the
// CTM factor (same treatment the dash pattern gets).
type pathEffectInternal interface {
	PathEffect
	effectHash() uint64
	scaledEffect(scale float64) PathEffect
}

// effectHashOf returns a deterministic hash for any effect: built-ins use
// their parameter hash; custom effects fall back to the instance pointer so
// two different objects never share a cache entry (one object keeps reusing
// its entry, valid because the purity contract makes FilterPath stable).
func effectHashOf(e PathEffect) uint64 {
	if e == nil {
		return 0
	}
	if ie, ok := e.(pathEffectInternal); ok {
		return ie.effectHash()
	}
	// Custom effect: reference-shaped instances hash by address (stable across
	// frames per the purity contract). A value-shaped custom effect has no
	// identity a cache could key on — two different values would share one
	// entry and paint the wrong geometry — so it reports 0 (unkeyable), and
	// caches must apply it directly instead of storing it.
	v := reflect.ValueOf(e)
	switch v.Kind() {
	case reflect.Pointer, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice, reflect.UnsafePointer:
		return uint64(v.Pointer())
	default:
		return 0
	}
}

// scaleEffect returns the effect with all length parameters multiplied by
// scale (1.0 returns e unchanged). Custom effects (no internal contract) are
// passed through unscaled — documented in FilterPath's contract.
func scaleEffect(e PathEffect, scale float64) PathEffect {
	if e == nil || scale == 0 {
		return nil
	}
	if scale <= 0 {
		scale = 1
	}
	if ie, ok := e.(pathEffectInternal); ok {
		return ie.scaledEffect(scale)
	}
	if scale == 1 {
		return e
	}
	return e // custom effect: parameters are its own business
}

// PathEffectHash returns a deterministic hash for an effect, usable as a
// geometry-cache key component: built-ins hash their parameters, reference-
// shaped custom effects hash their instance. 0 means "no key" — nil, or a
// value-shaped custom effect whose identity a cache cannot track (callers must
// apply those directly; see DashGeometryCache.GetOrApplyEffect).
func PathEffectHash(e PathEffect) uint64 { return effectHashOf(e) }

// ScalePathEffect returns the effect with length parameters multiplied by
// scale, for device-space consumers that must map user-space effect units by
// the CTM factor (same treatment the dash pattern receives). Custom effects
// without the internal contract are returned unchanged.
func ScalePathEffect(e PathEffect, scale float64) PathEffect { return scaleEffect(e, scale) }

func hashF64(h *hashState, v float64) {
	h.bits(math.Float64bits(v))
}

func hashPoint(h *hashState, p Point) {
	h.bits(math.Float64bits(p.X))
	h.bits(math.Float64bits(p.Y))
}

// hashPathForEffect is a deterministic content hash (verbs + coords) used by
// the stamp effects to key their prototypes.
func hashPathForEffect(p *Path) uint64 {
	if p == nil {
		return 0
	}
	h := newHashState()
	h.bits(uint64(p.NumVerbs()))
	p.Iterate(func(verb PathVerb, c []float64) {
		h.bits(uint64(verb))
		for _, v := range c {
			h.bits(math.Float64bits(v))
		}
	})
	return h.sum()
}

// ---------------------------------------------------------------------------
// Dash effect (SkDashPathEffect analogue): wraps the existing Dash engine so
// the convenience Stroke.Dash field and the effect path share one
// implementation — no second dasher.
// ---------------------------------------------------------------------------

type dashPathEffect struct{ dash *Dash }

// NewDashPathEffect returns a dash path effect for the given pattern.
// A nil or non-dashed pattern returns nil (no effect).
func NewDashPathEffect(dash *Dash) PathEffect {
	if !dash.IsDashed() {
		return nil
	}
	return dashPathEffect{dash: dash.Clone()}
}

func (e dashPathEffect) FilterPath(src *Path) (*Path, bool) {
	if src == nil || src.NumVerbs() == 0 {
		return nil, false
	}
	out := ApplyDash(src, e.dash)
	return out, out != nil && out.NumVerbs() > 0
}

func (e dashPathEffect) effectHash() uint64 {
	h := newHashState()
	h.bits(effectKindDash)
	if e.dash != nil {
		for _, v := range e.dash.Array {
			hashF64(h, v)
		}
		hashF64(h, e.dash.Offset)
	}
	return h.sum()
}

func (e dashPathEffect) scaledEffect(scale float64) PathEffect {
	if scale == 1 || e.dash == nil {
		return e
	}
	return dashPathEffect{dash: e.dash.Scale(scale)}
}

const (
	effectKindDash = iota + 1
	effectKindCorner
	effectKindDiscrete
	effectKindPath1D
	effectKindPath2D
	effectKindSum
	effectKindCompose
)

// ---------------------------------------------------------------------------
// Corner effect (SkCornerPathEffect analogue): rounds polygon corners with
// quadratic bends, reusing Path.WithCorners (flatten + cut, one
// implementation).
// ---------------------------------------------------------------------------

type cornerPathEffect struct{ radius float64 }

// NewCornerPathEffect returns an effect rounding corners with the given
// radius. radius <= 0 returns nil (no effect).
func NewCornerPathEffect(radius float64) PathEffect {
	if !(radius > 0) || math.IsInf(radius, 0) {
		return nil
	}
	return cornerPathEffect{radius: radius}
}

func (e cornerPathEffect) FilterPath(src *Path) (*Path, bool) {
	if src == nil || src.NumVerbs() == 0 {
		return nil, false
	}
	return src.WithCorners(e.radius), true
}

func (e cornerPathEffect) effectHash() uint64 {
	h := newHashState()
	h.bits(effectKindCorner)
	hashF64(h, e.radius)
	return h.sum()
}

func (e cornerPathEffect) scaledEffect(scale float64) PathEffect {
	return cornerPathEffect{radius: e.radius * scale}
}

// ---------------------------------------------------------------------------
// Discrete effect (SkDiscretePathEffect analogue): jitters segments by
// ±deviation with control points every segLen, reusing Path.Discrete
// (deterministic alternating side offsets, one implementation).
// ---------------------------------------------------------------------------

type discretePathEffect struct{ segLen, deviation float64 }

// NewDiscretePathEffect returns an effect breaking the path into segments of
// segLen, each pushed off by up to deviation (deterministic). Invalid or
// zero-strength parameters return nil.
func NewDiscretePathEffect(segLen, deviation float64) PathEffect {
	if !(segLen > 0) || !(deviation > 0) || math.IsInf(segLen, 0) || math.IsInf(deviation, 0) {
		return nil
	}
	return discretePathEffect{segLen: segLen, deviation: deviation}
}

func (e discretePathEffect) FilterPath(src *Path) (*Path, bool) {
	if src == nil || src.NumVerbs() == 0 {
		return nil, false
	}
	return src.Discrete(e.segLen, e.deviation), true
}

func (e discretePathEffect) effectHash() uint64 {
	h := newHashState()
	h.bits(effectKindDiscrete)
	hashF64(h, e.segLen)
	hashF64(h, e.deviation)
	return h.sum()
}

func (e discretePathEffect) scaledEffect(scale float64) PathEffect {
	return discretePathEffect{segLen: e.segLen * scale, deviation: e.deviation * scale}
}

// ---------------------------------------------------------------------------
// 1D stamp effect (SkPath1DPathEffect analogue): repeats a prototype along
// the path at advance spacing, optionally aligned to the tangent.
// ---------------------------------------------------------------------------

// maxReasonableStamps bounds one stamp walk. Stepping a float can loop for
// essentially ever (advance below the sample distance's ULP never progresses),
// and an absurdly small advance would stamp billions of copies and exhaust
// memory. Skia guards the same hazard with MAX_REASONABLE_ITERATIONS (100000)
// per contour in Sk1DPathEffect::onFilterPath, rejecting the filter when it
// runs out; the 2D grid uses the same ceiling on total stamps.
const maxReasonableStamps = 100000

// Path1DStyle selects how each stamp is placed at a path position
// (SkPath1DPathEffect::Style, semantics pinned to Skia m135 source).
type Path1DStyle uint8

const (
	// Path1DTranslate moves the prototype to the sample point; the stamp's
	// coordinate origin anchors there (Skia addPath(path, pos), no bbox fixup).
	Path1DTranslate Path1DStyle = iota
	// Path1DRotate additionally rotates the prototype so its +x axis follows
	// the path tangent, anchored at the stamp origin (SkPathMeasure matrix).
	Path1DRotate
	// Path1DMorph bends the prototype along the path point by point, exactly
	// like Skia's morphpoints: a stamp point (sx, sy) maps to the path sample
	// at distance+sx, offset sy along that point's normal (x rides arc length,
	// y rides the rotated normal — the stamp size is preserved, not scaled to
	// its bbox).
	Path1DMorph
)

func (s Path1DStyle) String() string {
	switch s {
	case Path1DTranslate:
		return "translate"
	case Path1DRotate:
		return "rotate"
	case Path1DMorph:
		return "morph"
	default:
		return "unknown"
	}
}

type path1DEffect struct {
	proto   *Path
	style   Path1DStyle
	advance float64
	// offset is the normalized start distance (PostScript interpretation,
	// same reduction as Skia's constructor).
	offset float64
}

// normalize1DPhase transcribes SkPath1DPathEffect's phase cleanup: negative
// phase flips sign, values beyond advance wrap modulo advance, and the result
// is inverted into an along-path start offset (to match PostScript).
func normalize1DPhase(phase, advance float64) float64 {
	if phase < 0 {
		phase = -phase
		if phase > advance {
			phase = math.Mod(phase, advance)
		}
	} else {
		if phase > advance {
			phase = math.Mod(phase, advance)
		}
		phase = advance - phase
	}
	if phase >= advance {
		phase = 0
	}
	return phase
}

// NewPath1DPathEffect stamps proto along the path every advance units with
// the PostScript-style phase (negative phases flip, like Skia). advance <= 0,
// a nil/empty prototype, or a non-finite parameter returns nil (no effect).
func NewPath1DPathEffect(proto *Path, style Path1DStyle, advance, phase float64) PathEffect {
	if proto == nil || proto.NumVerbs() == 0 || !(advance > 0) ||
		!finiteF64(advance) || !finiteF64(phase) {
		return nil
	}
	return path1DEffect{proto: proto.Clone(), style: style, advance: advance,
		offset: normalize1DPhase(phase, advance)}
}

func finiteF64(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// FilterPath walks each contour from the normalized offset, stamping while
// distance < contour length (Skia's loop bounds — no stamp exactly at the
// endpoint unless the offset itself lands there).
func (e path1DEffect) FilterPath(src *Path) (*Path, bool) {
	if src == nil || src.NumVerbs() == 0 {
		return nil, false
	}
	const tol = 0.25
	out := NewPath()
	metrics := src.ComputeMetrics(tol)
	for _, m := range metrics {
		length := m.Length()
		if length <= 0 {
			continue
		}
		governor := maxReasonableStamps
		for d := e.offset; d < length; d += e.advance {
			governor--
			if governor < 0 {
				return nil, false
			}
			stamp, ok := e.stampAt(m, d)
			if !ok {
				continue
			}
			out.Append(stamp)
		}
	}
	return out, out.NumVerbs() > 0
}

// stampAt places one stamp whose sample sits at arc-length distance d.
func (e path1DEffect) stampAt(m PathMetric, d float64) (*Path, bool) {
	pos, tan, ok := m.TangentAt(d)
	if !ok {
		return nil, false
	}
	switch e.style {
	case Path1DMorph:
		return e.morphStamp(m, d)
	case Path1DRotate:
		angle := math.Atan2(tan.Y, tan.X)
		mx := Translate(pos.X, pos.Y).Multiply(Rotate(angle))
		return e.proto.Transform(mx), true
	default: // Path1DTranslate
		return e.proto.Transform(Translate(pos.X, pos.Y)), true
	}
}

// morphStamp maps every stamp point (sx, sy) independently: sample the path
// at d+sx (drop the stamp if beyond the end, like morphpoints returning
// false), rotate the y offset by that sample's tangent, translate to the
// sample position. Verbs keep the prototype's structure.
func (e path1DEffect) morphStamp(m PathMetric, d float64) (*Path, bool) {
	type pt struct{ x, y float64 }
	mapPair := func(sx, sy float64) (Point, bool) {
		pos, tan, ok := m.TangentAt(d + sx)
		if !ok {
			return Point{}, false
		}
		// R(tan) applied to (0, sy): cos = tan.X, sin = tan.Y.
		return Point{X: pos.X - tan.Y*sy, Y: pos.Y + tan.X*sy}, true
	}
	out := NewPath()
	bad := false
	e.proto.Iterate(func(verb PathVerb, c []float64) {
		if bad {
			return
		}
		switch verb {
		case MoveTo:
			p, ok := mapPair(c[0], c[1])
			if !ok {
				bad = true
				return
			}
			out.MoveTo(p.X, p.Y)
		case LineTo:
			p, ok := mapPair(c[0], c[1])
			if !ok {
				bad = true
				return
			}
			out.LineTo(p.X, p.Y)
		case QuadTo:
			a, ok1 := mapPair(c[0], c[1])
			b, ok2 := mapPair(c[2], c[3])
			if !ok1 || !ok2 {
				bad = true
				return
			}
			out.QuadraticTo(a.X, a.Y, b.X, b.Y)
		case ConicTo:
			a, ok1 := mapPair(c[0], c[1])
			b, ok2 := mapPair(c[2], c[3])
			if !ok1 || !ok2 {
				bad = true
				return
			}
			out.ConicTo(a.X, a.Y, b.X, b.Y, c[4])
		case CubicTo:
			a, ok1 := mapPair(c[0], c[1])
			b, ok2 := mapPair(c[2], c[3])
			g, ok3 := mapPair(c[4], c[5])
			if !ok1 || !ok2 || !ok3 {
				bad = true
				return
			}
			out.CubicTo(a.X, a.Y, b.X, b.Y, g.X, g.Y)
		case Close:
			out.Close()
		}
	})
	if bad || out.NumVerbs() == 0 {
		return nil, false
	}
	return out, true
}

func (e path1DEffect) effectHash() uint64 {
	h := newHashState()
	h.bits(effectKindPath1D)
	h.bits(uint64(e.style))
	h.bits(hashPathForEffect(e.proto))
	hashF64(h, e.advance)
	hashF64(h, e.offset)
	return h.sum()
}

func (e path1DEffect) scaledEffect(scale float64) PathEffect {
	if scale == 1 {
		return e
	}
	proto := e.proto.Transform(Scale(scale, scale))
	return path1DEffect{proto: proto, style: e.style, advance: e.advance * scale, offset: e.offset * scale}
}

// ---------------------------------------------------------------------------
// 2D grid effect (SkPath2DPathEffect analogue): tiles the prototype on a
// dx×dy grid covering the source path bounds (translation only, the same
// tiling geometry Skia uses; the source path contributes its bounds).
// ---------------------------------------------------------------------------

type path2DEffect struct {
	proto  *Path
	dx, dy float64
}

// NewPath2DPathEffect tiles proto across the source path's bounds with cell
// size dx×dy. Invalid parameters return nil.
func NewPath2DPathEffect(proto *Path, dx, dy float64) PathEffect {
	if proto == nil || proto.NumVerbs() == 0 || !(dx > 0) || !(dy > 0) ||
		math.IsInf(dx, 0) || math.IsInf(dy, 0) {
		return nil
	}
	return path2DEffect{proto: proto.Clone(), dx: dx, dy: dy}
}

func (e path2DEffect) FilterPath(src *Path) (*Path, bool) {
	if src == nil || src.NumVerbs() == 0 {
		return nil, false
	}
	b := src.BoundingBox()
	pb := e.proto.BoundingBox()
	out := NewPath()
	stamps := 0
	for gy := b.Min.Y - pb.Height(); gy <= b.Max.Y; gy += e.dy {
		for gx := b.Min.X - pb.Width(); gx <= b.Max.X; gx += e.dx {
			stamps++
			if stamps > maxReasonableStamps {
				return nil, false
			}
			out.Append(e.proto.Transform(Translate(gx-pb.Min.X, gy-pb.Min.Y)))
		}
	}
	return out, out.NumVerbs() > 0
}

func (e path2DEffect) effectHash() uint64 {
	h := newHashState()
	h.bits(effectKindPath2D)
	h.bits(hashPathForEffect(e.proto))
	hashF64(h, e.dx)
	hashF64(h, e.dy)
	return h.sum()
}

func (e path2DEffect) scaledEffect(scale float64) PathEffect {
	if scale == 1 {
		return e
	}
	return path2DEffect{proto: e.proto.Transform(Scale(scale, scale)), dx: e.dx * scale, dy: e.dy * scale}
}

// ---------------------------------------------------------------------------
// Combinators (SkSumPathEffect / SkComposePathEffect analogues).
// ---------------------------------------------------------------------------

type sumPathEffect struct{ a, b PathEffect }

// SumPathEffects applies a and b independently to the same source and unions
// the results. A nil operand yields the other one; both nil yields nil.
func SumPathEffects(a, b PathEffect) PathEffect {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return sumPathEffect{a: a, b: b}
}

func (e sumPathEffect) FilterPath(src *Path) (*Path, bool) {
	pa, oka := e.a.FilterPath(src)
	pb, okb := e.b.FilterPath(src)
	if !oka && !okb {
		return nil, false
	}
	if !oka {
		return pb, true
	}
	if !okb {
		return pa, true
	}
	out := pa.Clone()
	out.Append(pb)
	return out, out.NumVerbs() > 0
}

func (e sumPathEffect) effectHash() uint64 {
	h := newHashState()
	h.bits(effectKindSum)
	h.bits(effectHashOf(e.a))
	h.bits(effectHashOf(e.b))
	return h.sum()
}

func (e sumPathEffect) scaledEffect(scale float64) PathEffect {
	return sumPathEffect{a: scaleEffect(e.a, scale), b: scaleEffect(e.b, scale)}
}

type composePathEffect struct{ outer, inner PathEffect }

// ComposePathEffects returns outer(inner(src)): the inner effect transforms
// the path first, the outer transforms that result. A nil operand yields the
// other one; both nil yields nil. (SkComposePathEffect order: the first
// argument applies last, like mathematical composition.)
func ComposePathEffects(outer, inner PathEffect) PathEffect {
	if outer == nil {
		return inner
	}
	if inner == nil {
		return outer
	}
	return composePathEffect{outer: outer, inner: inner}
}

func (e composePathEffect) FilterPath(src *Path) (*Path, bool) {
	p, ok := e.inner.FilterPath(src)
	if !ok {
		return nil, false
	}
	return e.outer.FilterPath(p)
}

func (e composePathEffect) effectHash() uint64 {
	h := newHashState()
	h.bits(effectKindCompose)
	h.bits(effectHashOf(e.inner))
	h.bits(effectHashOf(e.outer))
	return h.sum()
}

func (e composePathEffect) scaledEffect(scale float64) PathEffect {
	return composePathEffect{outer: scaleEffect(e.outer, scale), inner: scaleEffect(e.inner, scale)}
}

// ---------------------------------------------------------------------------
// Small FNV-1a state helper (render-package-local; internal/gpu keeps its own
// hashDash untouched so existing cache keys do not shift).
// ---------------------------------------------------------------------------

type hashState struct{ acc uint64 }

func newHashState() *hashState { return &hashState{acc: 14695981039346656037} }

func (h *hashState) bits(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	for _, c := range b {
		h.acc ^= uint64(c)
		h.acc *= 1099511628211
	}
}

func (h *hashState) sum() uint64 { return h.acc }
