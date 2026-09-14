package camera

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// MinZoom floors each zoom component. Zero/negative zoom clamps here,
// never to zero, so View stays invertible.
const MinZoom = 1e-6

// Camera is a pure-math 2D lens: position, zoom, angle, shake offset,
// position limits, follow smoothing, screen anchor, viewport size.
// It draws nothing; View feeds the existing render draws.
type Camera struct {
	pos       core.Vec2
	zoom      core.Vec2
	rotation  float64
	shake     core.Vec2
	limit     core.Rect
	smoothing float64
	anchor    core.Vec2
	viewport  core.Vec2
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func normalizeRot(rad float64) float64 {
	return math.Atan2(math.Sin(rad), math.Cos(rad))
}

func finiteRect(r core.Rect) bool {
	return finite(r.X) && finite(r.Y) && finite(r.W) && finite(r.H)
}

// NewCamera builds a centered camera over viewport (screen size).
// Viewport must be finite; negative components clamp to 0.
// NaN/Inf viewport returns a core InvalidArg error.
func NewCamera(viewport core.Vec2) (Camera, error) {
	if !finiteVec(viewport) {
		return Camera{}, core.InvalidArg("camera.NewCamera", "viewport")
	}
	cam := Camera{zoom: core.V2(1, 1), anchor: core.V2(0.5, 0.5)}
	// Validated above; SetViewport owns the negative-to-0 clamp.
	_ = cam.SetViewport(viewport)
	return cam, nil
}

// Pos returns the clamped camera center in world units.
func (c Camera) Pos() core.Vec2 { return c.pos }

// Zoom returns the zoom scale (each component >= MinZoom).
func (c Camera) Zoom() core.Vec2 { return c.zoom }

// Rotation returns the camera angle in radians, normalized to [-pi, pi].
func (c Camera) Rotation() float64 { return c.rotation }

// Shake returns the current shake offset in world units.
func (c Camera) Shake() core.Vec2 { return c.shake }

// Limit returns the position clamp box; empty means disabled.
func (c Camera) Limit() core.Rect { return c.limit }

// Smoothing returns the Follow lerp factor: 0 snaps, (0,1] eases.
func (c Camera) Smoothing() float64 { return c.smoothing }

// Anchor returns the screen anchor fraction in [0,1] each.
func (c Camera) Anchor() core.Vec2 { return c.anchor }

// Viewport returns the screen size in pixels (each >= 0).
func (c Camera) Viewport() core.Vec2 { return c.viewport }

// SetViewport replaces the screen size; negatives clamp to 0.
func (c *Camera) SetViewport(v core.Vec2) error {
	if !finiteVec(v) {
		return core.InvalidArg("camera.SetViewport", "viewport")
	}
	c.viewport = core.V2(math.Max(v.X, 0), math.Max(v.Y, 0))
	return nil
}

// SetPos moves the center; it clamps into Limit when Limit is set.
func (c *Camera) SetPos(p core.Vec2) error {
	if !finiteVec(p) {
		return core.InvalidArg("camera.SetPos", "pos")
	}
	c.pos = c.clampPos(p)
	return nil
}

// SetZoom replaces the scale; zero/negative components clamp to MinZoom.
func (c *Camera) SetZoom(z core.Vec2) error {
	if !finiteVec(z) {
		return core.InvalidArg("camera.SetZoom", "zoom")
	}
	c.zoom = core.V2(math.Max(z.X, MinZoom), math.Max(z.Y, MinZoom))
	return nil
}

// SetRotation replaces the angle; any finite value normalizes to [-pi, pi].
func (c *Camera) SetRotation(rad float64) error {
	if !finite(rad) {
		return core.InvalidArg("camera.SetRotation", "rotation")
	}
	c.rotation = normalizeRot(rad)
	return nil
}

// SetShake replaces the shake offset; callers drive it (trauma curve,
// seeded random), the camera only adds and decays it.
func (c *Camera) SetShake(offset core.Vec2) error {
	if !finiteVec(offset) {
		return core.InvalidArg("camera.SetShake", "shake")
	}
	c.shake = offset
	return nil
}

// DecayShake shrinks the shake offset by factor in [0,1] (1 clears).
// Out-of-range factors clamp; NaN/Inf is an error.
func (c *Camera) DecayShake(factor float64) error {
	if !finite(factor) {
		return core.InvalidArg("camera.DecayShake", "factor")
	}
	c.shake = c.shake.Mul(1 - clampFloat(factor, 0, 1))
	return nil
}

// SetLimit replaces the center clamp box; empty disables clamping.
// NaN/Inf components are an error; negative sizes stay empty (disabled).
func (c *Camera) SetLimit(r core.Rect) error {
	if !finiteRect(r) {
		return core.InvalidArg("camera.SetLimit", "limit")
	}
	c.limit = r
	c.pos = c.clampPos(c.pos)
	return nil
}

// SetSmoothing sets the Follow factor: 0 snaps, (0,1] eases.
// Out-of-range values clamp to [0,1]; NaN/Inf is an error.
func (c *Camera) SetSmoothing(f float64) error {
	if !finite(f) {
		return core.InvalidArg("camera.SetSmoothing", "smoothing")
	}
	c.smoothing = clampFloat(f, 0, 1)
	return nil
}

// SetAnchor sets which screen point the center maps to, as a fraction
// of the viewport in [0,1] each ((0,0) top-left, (0.5,0.5) centered).
// Out-of-range components clamp; NaN/Inf is an error.
func (c *Camera) SetAnchor(a core.Vec2) error {
	if !finiteVec(a) {
		return core.InvalidArg("camera.SetAnchor", "anchor")
	}
	c.anchor = core.V2(clampFloat(a.X, 0, 1), clampFloat(a.Y, 0, 1))
	return nil
}

// Follow moves toward target (clamped into Limit first). With smoothing 0
// it snaps; otherwise pos lerps by the smoothing factor. Bad targets are
// an error and move nothing.
func (c *Camera) Follow(target core.Vec2) error {
	if !finiteVec(target) {
		return core.InvalidArg("camera.Follow", "target")
	}
	want := c.clampPos(target)
	if c.smoothing == 0 {
		c.pos = want
		return nil
	}
	c.pos = c.pos.Lerp(want, c.smoothing)
	return nil
}

func (c Camera) clampPos(p core.Vec2) core.Vec2 {
	if c.limit.IsEmpty() {
		return p
	}
	return core.V2(
		clampFloat(p.X, c.limit.X, c.limit.X+c.limit.W),
		clampFloat(p.Y, c.limit.Y, c.limit.Y+c.limit.H),
	)
}

// EffectivePos is the clamped center plus the shake offset. Limits pin
// the center; shake may peek outside for a frame by design.
// Pos is always clamped at write time (SetPos/SetLimit/Follow), so this
// is a direct add with no second clamp on the hot path.
func (c Camera) EffectivePos() core.Vec2 {
	return c.pos.Add(c.shake)
}

// View returns the world-to-screen matrix:
//
//	screen = Anchor*Viewport + Rot(-rotation) * Zoom * (world - EffectivePos)
//
// Zoom never hits zero (see MinZoom), so the result is always finite and
// invertible for finite state; setters already reject NaN/Inf.
func (c Camera) View() core.Mat2D {
	eff := c.EffectivePos()
	origin := core.V2(c.anchor.X*c.viewport.X, c.anchor.Y*c.viewport.Y)
	return core.Translate2D(origin.X, origin.Y).Mul(
		core.Rotate2D(-c.rotation).Mul(
			core.Scale2D(c.zoom.X, c.zoom.Y).Mul(
				core.Translate2D(-eff.X, -eff.Y))))
}

// WorldToScreen maps one world point to screen pixels.
// Bad input returns ok=false with a zero point, never NaN.
func (c Camera) WorldToScreen(world core.Vec2) (screen core.Vec2, ok bool) {
	if !finiteVec(world) {
		return core.Vec2{}, false
	}
	out := c.View().TransformPoint(world)
	if !finiteVec(out) {
		return core.Vec2{}, false
	}
	return out, true
}

// ScreenToWorld inverts WorldToScreen. Bad input returns ok=false.
func (c Camera) ScreenToWorld(screen core.Vec2) (world core.Vec2, ok bool) {
	if !finiteVec(screen) {
		return core.Vec2{}, false
	}
	inv, ok := c.View().Invert()
	if !ok {
		return core.Vec2{}, false
	}
	out := inv.TransformPoint(screen)
	if !finiteVec(out) {
		return core.Vec2{}, false
	}
	return out, true
}

// VisibleWorldRect returns the axis-aligned world bound of the viewport,
// for tile/chunk culling. With rotation it is a conservative bound.
// Empty viewports see only the center point.
func (c Camera) VisibleWorldRect() core.Rect {
	eff := c.EffectivePos()
	if c.viewport.X <= 0 || c.viewport.Y <= 0 {
		return core.NewRect(eff.X, eff.Y, 0, 0)
	}
	corners := [4]core.Vec2{
		{}, {X: c.viewport.X}, {X: c.viewport.X, Y: c.viewport.Y}, {Y: c.viewport.Y},
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, s := range corners {
		w, ok := c.ScreenToWorld(s)
		if !ok {
			return core.NewRect(eff.X, eff.Y, 0, 0)
		}
		minX = math.Min(minX, w.X)
		minY = math.Min(minY, w.Y)
		maxX = math.Max(maxX, w.X)
		maxY = math.Max(maxY, w.Y)
	}
	return core.NewRect(minX, minY, maxX-minX, maxY-minY)
}
