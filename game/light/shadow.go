package light

import (
	"github.com/energye/gpui/game/core"
)

// Budgets for 6.3 shadows. Over-count occluders report OutOfMemory at
// construction; over-long shadows report InvalidArg. Never allocate then
// fail.
const (
	MaxOccluderVerts = 64
	MaxOccluders     = 256
	MaxShadowLength  = 8192.0
)

// shadowEps keeps wall faces lit: a hit at the ray ends (the bulb, the
// wall face itself) never counts as blocked.
const shadowEps = 1e-9

// Occluder is one frozen 2D shadow caster: a closed polygon loop of at
// least 3 finite vertices, row order free (clockwise or not). Always
// copied on the way in.
type Occluder struct {
	Pts []core.Vec2
}

// NewOccluder builds an Occluder copying pts. Fewer than 3 points or a
// non-finite vertex is InvalidArg; more than MaxOccluderVerts is
// OutOfMemory.
func NewOccluder(pts []core.Vec2) (Occluder, error) {
	const op = "light.NewOccluder"
	if len(pts) < 3 {
		return Occluder{}, core.InvalidArg(op, "points")
	}
	if len(pts) > MaxOccluderVerts {
		return Occluder{}, core.OutOfMemory(op, "points")
	}
	for _, p := range pts {
		if !finiteVec(p) {
			return Occluder{}, core.InvalidArg(op, "point")
		}
	}
	cp := make([]core.Vec2, len(pts))
	copy(cp, pts)
	return Occluder{Pts: cp}, nil
}

// Validate reports InvalidArg for fewer than 3 or non-finite vertices,
// OutOfMemory for over-count vertices.
func (o Occluder) Validate() error {
	const op = "light.Occluder.Validate"
	if len(o.Pts) < 3 {
		return core.InvalidArg(op, "points")
	}
	if len(o.Pts) > MaxOccluderVerts {
		return core.OutOfMemory(op, "points")
	}
	for _, p := range o.Pts {
		if !finiteVec(p) {
			return core.InvalidArg(op, "point")
		}
	}
	return nil
}

// Shadow is one frozen set of casters: every occluder blocks every light
// the same way (Godot LightOccluder2D idea, Go reworked). Occlusion in
// [0,1] says how dark the lee side gets (0 keeps the light, 1 falls back
// to night only, never black). Length caps how far past the wall the dark
// reaches; 0 casts no shadow at all (valid no-op for zero-length slides).
// Constructors copy; Apply returns fresh images.
type Shadow struct {
	Occluders []Occluder
	Occlusion float64
	Length    float64
}

// NewShadow builds a Shadow copying occluders. Occlusion outside [0,1] or
// a non-finite/negative Length or a Length beyond MaxShadowLength is
// InvalidArg; more than MaxOccluders is InvalidArg; a bad occluder fails
// the build with its own code.
func NewShadow(occluders []Occluder, occlusion, length float64) (Shadow, error) {
	const op = "light.NewShadow"
	if !finite(occlusion) || occlusion < 0 || occlusion > 1 {
		return Shadow{}, core.InvalidArg(op, "occlusion")
	}
	if !finite(length) || length < 0 || length > MaxShadowLength {
		return Shadow{}, core.InvalidArg(op, "length")
	}
	if len(occluders) > MaxOccluders {
		return Shadow{}, core.InvalidArg(op, "occluders")
	}
	for i := range occluders {
		if err := occluders[i].Validate(); err != nil {
			return Shadow{}, err
		}
	}
	cp := make([]Occluder, len(occluders))
	for i, o := range occluders {
		pts := make([]core.Vec2, len(o.Pts))
		copy(pts, o.Pts)
		cp[i] = Occluder{Pts: pts}
	}
	if cp == nil {
		cp = []Occluder{}
	}
	return Shadow{Occluders: cp, Occlusion: occlusion, Length: length}, nil
}

// Validate reports InvalidArg for a bad occlusion, length, count, or
// occluder.
func (s Shadow) Validate() error {
	const op = "light.Shadow.Validate"
	if !finite(s.Occlusion) || s.Occlusion < 0 || s.Occlusion > 1 {
		return core.InvalidArg(op, "occlusion")
	}
	if !finite(s.Length) || s.Length < 0 || s.Length > MaxShadowLength {
		return core.InvalidArg(op, "length")
	}
	if len(s.Occluders) > MaxOccluders {
		return core.InvalidArg(op, "occluders")
	}
	for i := range s.Occluders {
		if err := s.Occluders[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// segHit reports whether segment from->to crosses edge a->b strictly
// inside (past shadowEps from both ends) and, when so, the hit distance
// from `from`. Parallel and endpoint touches miss on purpose: wall faces
// stay lit, bulbs inside their own loop never self-shadow.
func segHit(from, to, a, b core.Vec2) (bool, float64) {
	d := to.Sub(from)
	e := b.Sub(a)
	denom := d.Cross(e)
	if denom == 0 || !finite(denom) {
		return false, 0
	}
	al := a.Sub(from)
	t := al.Cross(e) / denom
	u := al.Cross(d) / denom
	if !(t > shadowEps && t < 1-shadowEps) {
		return false, 0
	}
	if !(u >= -shadowEps && u <= 1+shadowEps) {
		return false, 0
	}
	dist := d.Length() * t
	if !finite(dist) {
		return false, 0
	}
	return true, dist
}

// loopHit returns the nearest wall hit distance along from->to, or false
// when the ray reaches `to` in the open. Corrupt loops are skipped
// (fail-open), never a crash.
func (s Shadow) loopHit(from, to core.Vec2) (bool, float64) {
	hit := false
	best := 0.0
	for i := range s.Occluders {
		o := s.Occluders[i]
		if o.Validate() != nil {
			continue
		}
		n := len(o.Pts)
		for k := 0; k < n; k++ {
			ok, dist := segHit(from, to, o.Pts[k], o.Pts[(k+1)%n])
			if ok && (!hit || dist < best) {
				hit = true
				best = dist
			}
		}
	}
	return hit, best
}

// occlusionAt most clamps a runtime occlusion into [0,1]; non-finite
// reads full dark, never NaN downstream.
func (s Shadow) occlusionAt() float64 {
	if !finite(s.Occlusion) {
		return 1
	}
	return clamp01(s.Occlusion)
}

// blockedShared holds the frozen geometry both backends run: true when a
// wall stands between the light and p and the dark still reaches p.
// Anything bad (empty set, zero length, bad light, non-finite p) reads
// unblocked, never panics.
func (s Shadow) blockedShared(l Light, p core.Vec2) bool {
	if !finiteVec(p) || len(s.Occluders) == 0 {
		return false
	}
	if !finite(s.Length) || s.Length <= 0 {
		return false
	}
	switch l.Kind {
	case KindPoint:
		if l.Kind != KindPoint || !finiteVec(l.Pos) {
			return false
		}
		d := p.Sub(l.Pos)
		full := d.Length()
		if !finite(full) || full <= shadowEps {
			return false
		}
		hit, at := s.loopHit(l.Pos, p)
		if !hit {
			return false
		}
		past := full - at
		return past > shadowEps && past <= s.Length+shadowEps
	case KindDirectional:
		if !finiteVec(l.Dir) || l.Dir.IsZero() {
			return false
		}
		n := l.Dir.Normalize()
		if !finiteVec(n) || n.IsZero() {
			return false
		}
		// Dir is the travel direction of the rays; cast back toward the
		// source over the shadow length.
		to := p.Sub(n.Mul(s.Length))
		if !finiteVec(to) {
			return false
		}
		hit, _ := s.loopHit(p, to)
		return hit
	default:
		return false
	}
}

// Blocked reports whether occluders shadow p from light l (CPU path).
func (s Shadow) Blocked(l Light, p core.Vec2) bool { return s.blockedShared(l, p) }

// BlockedGPU recomputes the same frozen geometry (GPU mirror): bitwise
// identical to Blocked for the same inputs.
func (s Shadow) BlockedGPU(l Light, p core.Vec2) bool { return s.blockedShared(l, p) }

// factorShared maps blocked to the dim multiplier: 1 in the open,
// 1-occlusion in the lee.
func (s Shadow) factorShared(l Light, p core.Vec2) float64 {
	if !s.blockedShared(l, p) {
		return 1
	}
	return clamp01(1 - s.occlusionAt())
}

// FactorFor returns the shadow multiplier for light l at p (CPU path).
func (s Shadow) FactorFor(l Light, p core.Vec2) float64 { return s.factorShared(l, p) }

// FactorForGPU recomputes the same multiplier (GPU mirror).
func (s Shadow) FactorForGPU(l Light, p core.Vec2) float64 { return s.factorShared(l, p) }

// litShared holds the frozen pixel sum both backends run: the 6.1 night
// plus every light, each dimmed by its own shadow multiplier. A fully
// shadowed pixel keeps base*night, never black; alpha rides through.
func (s Shadow) litShared(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
	br := sanitize01(base.R)
	bg := sanitize01(base.G)
	bb := sanitize01(base.B)
	ba := sanitize01(base.A)
	nr, ng, nb := 1.0, 1.0, 1.0
	if validColor(night) {
		nr, ng, nb = night.R, night.G, night.B
	}
	var sr, sg, sb float64
	for i := range lights {
		l := lights[i]
		if !validColor(l.Color) || !finite(l.Intensity) || l.Intensity < 0 || l.Intensity > MaxIntensity {
			continue
		}
		f := l.factorShared(p, layer)
		if f <= 0 || l.Intensity == 0 {
			continue
		}
		f *= s.factorShared(l, p)
		if f <= 0 {
			continue
		}
		sr += f * l.Intensity * l.Color.R
		sg += f * l.Intensity * l.Color.G
		sb += f * l.Intensity * l.Color.B
	}
	return core.RGBA(
		clamp01(br*nr+br*sr),
		clamp01(bg*ng+bg*sb),
		clamp01(bb*nb+bb*sb),
		ba,
	)
}

// Lit lights one pixel through lights, night, and this shadow (CPU path).
func (s Shadow) Lit(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
	return s.litShared(base, p, layer, lights, night)
}

// LitGPU recomputes the same pixel (GPU mirror): bitwise identical.
func (s Shadow) LitGPU(base core.Color, p core.Vec2, layer Layer, lights []Light, night core.Color) core.Color {
	return s.litShared(base, p, layer, lights, night)
}

// applyShared lights every pixel of src through lights+night+shadow into
// a fresh image.
func (s Shadow) applyShared(src *Image, lights []Light, night core.Color) (*Image, error) {
	const op = "light.Shadow.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	out := make([]core.Color, len(src.Pix))
	lp := make([]Layer, len(src.Layers))
	copy(lp, src.Layers)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			i := y*src.W + x
			p := core.V2(src.Origin.X+float64(x)*src.Step, src.Origin.Y+float64(y)*src.Step)
			out[i] = s.litShared(src.Pix[i], p, src.Layers[i], lights, night)
		}
	}
	return &Image{W: src.W, H: src.H, Origin: src.Origin, Step: src.Step, Pix: out, Layers: lp}, nil
}

// Apply lights src through lights and night dimmed by this shadow (CPU
// path). src never mutated. Nil src is InvalidArg, corrupt src BadData,
// invalid shadow InvalidArg.
func (s Shadow) Apply(src *Image, lights []Light, night core.Color) (*Image, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s.applyShared(src, lights, night)
}

// ApplyCPU is the explicit CPU path (same as Apply).
func (s Shadow) ApplyCPU(src *Image, lights []Light, night core.Color) (*Image, error) {
	return s.Apply(src, lights, night)
}

// ApplyGPU is the GPU mirror path: same frozen math, bitwise identical.
func (s Shadow) ApplyGPU(src *Image, lights []Light, night core.Color) (*Image, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s.applyShared(src, lights, night)
}
