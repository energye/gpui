package camera

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// Projector maps world positions plus depth to screen positions.
// Only core numbers are used; render types are converted once by the
// caller at the render boundary with Vec2.ToRenderPoint.
type Projector struct {
	focal  float64
	center core.Vec2
	camera core.Vec2
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteVec(v core.Vec2) bool { return finite(v.X) && finite(v.Y) }

// NewProjector builds a Projector.
// Focal must be finite and positive; center and camera must be finite.
// Anything else returns a core InvalidArg error, never a guessed projector.
func NewProjector(focal float64, center, camera core.Vec2) (Projector, error) {
	if !finite(focal) || focal <= 0 {
		return Projector{}, core.InvalidArg("camera.NewProjector", "focal")
	}
	if !finiteVec(center) || !finiteVec(camera) {
		return Projector{}, core.InvalidArg("camera.NewProjector", "center/camera")
	}
	return Projector{focal: focal, center: center, camera: camera}, nil
}

// Focal returns the eye-to-screen distance the projector was built with.
func (p Projector) Focal() float64 { return p.focal }

// Center returns the screen point where depth 0 lands.
func (p Projector) Center() core.Vec2 { return p.center }

// Camera returns the world point sitting under Center at depth 0.
func (p Projector) Camera() core.Vec2 { return p.camera }

// DepthToScale maps depth to a screen scale.
// Depth 0 returns 1. Behind the eye (focal+depth <= 0) or any
// non-finite depth returns ok=false with scale 0.
func (p Projector) DepthToScale(depth float64) (scale float64, ok bool) {
	if !finite(depth) {
		return 0, false
	}
	denom := p.focal + depth
	if math.IsInf(denom, 0) || !(denom > 0) {
		return 0, false
	}
	scale = p.focal / denom
	if !finite(scale) {
		return 0, false
	}
	return scale, true
}

// Project maps one world point at depth to a screen point.
// It also returns the scale used, so callers can size sprites.
// Undrawable input (bad depth, NaN/Inf world) returns ok=false with
// zero screen and zero scale, never NaN and never a panic.
func (p Projector) Project(world core.Vec2, depth float64) (screen core.Vec2, scale float64, ok bool) {
	if !finiteVec(world) {
		return core.Vec2{}, 0, false
	}
	scale, ok = p.DepthToScale(depth)
	if !ok {
		return core.Vec2{}, 0, false
	}
	screen = core.V2(
		p.center.X+(world.X-p.camera.X)*scale,
		p.center.Y+(world.Y-p.camera.Y)*scale,
	)
	if !finiteVec(screen) {
		return core.Vec2{}, 0, false
	}
	return screen, scale, true
}

// ProjectQuad maps four corners plus per-corner depths to four screen
// corners. The result feeds render.DrawImageQuad corner for corner after
// one ToRenderPoint each. Any undrawable corner fails the whole quad
// (ok=false) so the caller never draws half a trapezoid.
func (p Projector) ProjectQuad(corners [4]core.Vec2, depths [4]float64) (screens [4]core.Vec2, scales [4]float64, ok bool) {
	for i := 0; i < 4; i++ {
		s, sc, okOne := p.Project(corners[i], depths[i])
		if !okOne {
			return [4]core.Vec2{}, [4]float64{}, false
		}
		screens[i] = s
		scales[i] = sc
	}
	return screens, scales, true
}

// Unproject inverts Project: screen plus depth back to world.
// It reports ok=false for the same inputs Project rejects, so a
// Project/Unproject round trip closes within float tolerance.
func (p Projector) Unproject(screen core.Vec2, depth float64) (world core.Vec2, ok bool) {
	if !finiteVec(screen) {
		return core.Vec2{}, false
	}
	scale, ok := p.DepthToScale(depth)
	if !ok || scale == 0 {
		return core.Vec2{}, false
	}
	world = core.V2(
		p.camera.X+(screen.X-p.center.X)/scale,
		p.camera.Y+(screen.Y-p.center.Y)/scale,
	)
	if !finiteVec(world) {
		return core.Vec2{}, false
	}
	return world, true
}
