package camera

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Layer is one parallax depth band: how far it drifts with the camera,
// plus an optional seamless repeat.
//
// Only core numbers are used. A Layer draws nothing; Shift computes the
// drifted world position and Screen feeds it into a Projector, whose
// screen corners go to the existing trapezoid/triangle draws
// (render.DrawImageQuad, DrawVertices/DrawMesh).
type Layer struct {
	factor core.Vec2
	offset core.Vec2
	mirror core.Vec2
}

// NewLayer builds a Layer.
//
// Factor is the follow ratio per axis: 1 locks to the world, 0 pins to
// the screen (far sky), 0.5 drifts at half speed, values above 1 run
// ahead (foreground). Any finite value is accepted, including negatives
// (counter-drift effects).
//
// Offset is an extra shift in world units.
//
// Mirror is the repeat period in world units per axis; 0 disables repeat
// on that axis. Negative or non-finite components are a core InvalidArg
// error, never a silently wrapped layer.
func NewLayer(factor, offset, mirror core.Vec2) (Layer, error) {
	if !finiteVec(factor) {
		return Layer{}, core.InvalidArg("camera.NewLayer", "factor")
	}
	if !finiteVec(offset) {
		return Layer{}, core.InvalidArg("camera.NewLayer", "offset")
	}
	if !finiteVec(mirror) {
		return Layer{}, core.InvalidArg("camera.NewLayer", "mirror")
	}
	if mirror.X < 0 || mirror.Y < 0 {
		return Layer{}, core.InvalidArg("camera.NewLayer", "mirror")
	}
	return Layer{factor: factor, offset: offset, mirror: mirror}, nil
}

// Factor returns the follow ratio per axis.
func (l Layer) Factor() core.Vec2 { return l.factor }

// Offset returns the extra shift in world units.
func (l Layer) Offset() core.Vec2 { return l.offset }

// Mirror returns the repeat period per axis (0 disables that axis).
func (l Layer) Mirror() core.Vec2 { return l.mirror }

// wrapOne folds x into [0, m) when m > 0; otherwise x passes through.
// NaN/Inf input yields ok=false instead of NaN. Callers pass validated
// inputs, so the single finite(x) gate up front covers both paths and
// the mirror dispatch below only decides wrap versus pass-through.
func wrapOne(x, m float64) (float64, bool) {
	if !finite(x) {
		return 0, false
	}
	if m == 0 {
		return x, true
	}
	if !finite(m) || m <= 0 {
		return 0, false
	}
	return x - math.Floor(x/m)*m, true
}

// Shift returns the drifted world position for world seen from camera:
//
//	shift = world + camera*(1-factor) + offset
//
// wrapped per axis into [0, mirror) when that mirror component is set.
// Factor 1 cancels the camera term (world-locked); factor 0 keeps the
// full camera term so Screen later subtracts it back out (screen-pinned).
// Undrawable input (NaN/Inf world or camera) returns ok=false with a
// zero position, never NaN and never a panic.
func (l Layer) Shift(world, camera core.Vec2) (core.Vec2, bool) {
	if !finiteVec(world) || !finiteVec(camera) {
		return core.Vec2{}, false
	}
	if !finiteVec(l.factor) || !finiteVec(l.offset) || !finiteVec(l.mirror) {
		return core.Vec2{}, false
	}
	x := world.X + camera.X*(1-l.factor.X) + l.offset.X
	y := world.Y + camera.Y*(1-l.factor.Y) + l.offset.Y
	wx, ok := wrapOne(x, l.mirror.X)
	if !ok {
		return core.Vec2{}, false
	}
	wy, ok := wrapOne(y, l.mirror.Y)
	if !ok {
		return core.Vec2{}, false
	}
	out := core.V2(wx, wy)
	if !finiteVec(out) {
		return core.Vec2{}, false
	}
	return out, true
}

// Screen maps world at depth seen from camera to a screen point.
//
// It shifts first, then projects: build the Projector with the same
// camera each frame so factor 1 layers land exactly where a direct
// Project call lands. It also returns the perspective scale, so callers
// can size sprites. Any undrawable step (bad shift or bad depth)
// returns ok=false with zero screen and zero scale.
func (l Layer) Screen(world core.Vec2, depth float64, camera core.Vec2, proj Projector) (core.Vec2, float64, bool) {
	shifted, ok := l.Shift(world, camera)
	if !ok {
		return core.Vec2{}, 0, false
	}
	return proj.Project(shifted, depth)
}

// ScreenQuad maps four corners plus per-corner depths to four screen
// corners for render.DrawImageQuad. Any undrawable corner fails the whole
// quad (ok=false) so the caller never draws half a trapezoid.
func (l Layer) ScreenQuad(corners [4]core.Vec2, depths [4]float64, camera core.Vec2, proj Projector) ([4]core.Vec2, [4]float64, bool) {
	var screens [4]core.Vec2
	var scales [4]float64
	for i := 0; i < 4; i++ {
		s, sc, ok := l.Screen(corners[i], depths[i], camera, proj)
		if !ok {
			return [4]core.Vec2{}, [4]float64{}, false
		}
		screens[i] = s
		scales[i] = sc
	}
	return screens, scales, true
}
