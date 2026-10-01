//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package physics

import (
	"math"

	"github.com/energye/gpui/engine/core"
)

// Snap rule (S76, CharacterBody2D alignment, additive only).
//
// This file only adds new code paths; body.go (Query/Overlaps/CastRay,
// IsStandingOn/CarryRider) and platform.go (Grounded/Step) are untouched.
// Thin-segment slopes live in +Y-down tilemap units (see platform.go);
// box/circle bodies live in +Y-up math units (see body.go). The snap API
// below works in slope space (feet plus slopes), the same space Step
// integrates, so callers never convert.
//
// Godot CharacterBody2D reference (scene/2d/physics/character_body_2d.h):
// motion_mode two gears (GROUNDED/FLOATING), floor_max_angle default 45
// degrees, floor_snap_length default 0.1, floor_safe_margin default 0.08,
// up_direction default (0,-1), move_and_slide with apply_floor_snap.
// Mapping used here:
//   - MotionModeGrounded: floor snap engages, steep faces are not floor,
//     rising never snaps, falling still snaps within SnapLength.
//   - MotionModeFloating: no floor at all (top-down drift); Step numbers
//     still integrate and solid head bumps still report, but onFloor is
//     always false and no snap extends the landing.
//   - FloorNormal covers Godot floor_normal for walkable faces; vertical
//     segments have no up normal and report ok=false. Wall/ceiling sliding
//     stays with Step (head flag) and SlideDir; no new wall solver here.
//   - SafeMargin is the sink allowance below the surface (Godot
//     floor_safe_margin); SnapLength is the reach above it. The window
//     intent is game_physics --case=jump.

// MotionMode mirrors Godot CharacterBody2D motion_mode: the two gears
// GROUNDED (platformer stick) and FLOATING (top-down drift, no floor).
type MotionMode int

const (
	// MotionModeGrounded sticks to walkable floors within the snap window.
	MotionModeGrounded MotionMode = 0
	// MotionModeFloating never reports floor and never snaps.
	MotionModeFloating MotionMode = 1
)

// Valid reports whether m is one of the two known gears.
func (m MotionMode) Valid() bool {
	return m == MotionModeGrounded || m == MotionModeFloating
}

// String returns the debug key, never parsed.
func (m MotionMode) String() string {
	switch m {
	case MotionModeGrounded:
		return "grounded"
	case MotionModeFloating:
		return "floating"
	default:
		return "unknown"
	}
}

// Godot CharacterBody2D defaults (D18): floor snap 0.1, floor max angle
// 45 degrees, safe margin 0.08. Values only, no behavior: the caller owns
// the config and passes it in.
const (
	// DefaultSnapLength bridges a small gap above the floor.
	DefaultSnapLength = 0.1
	// DefaultFloorMaxAngle is 45 degrees in radians; steeper is not floor.
	DefaultFloorMaxAngle = math.Pi / 4
	// DefaultSafeMargin allows sinking this far below the surface.
	DefaultSafeMargin = 0.08
)

// UpDirection returns the world up unit (0,-1): +Y runs down in slope
// space, so up is negative Y. Godot up_direction default, same value.
func UpDirection() core.Vec2 { return core.V2(0, -1) }

// SnapConfig carries caller-owned floor tuning. Zero value is invalid;
// use DefaultSnapConfig and adjust from there.
type SnapConfig struct {
	Mode          MotionMode
	SnapLength    float64
	FloorMaxAngle float64
	SafeMargin    float64
}

// DefaultSnapConfig returns the Godot D18 defaults in grounded gear.
func DefaultSnapConfig() SnapConfig {
	return SnapConfig{
		Mode:          MotionModeGrounded,
		SnapLength:    DefaultSnapLength,
		FloorMaxAngle: DefaultFloorMaxAngle,
		SafeMargin:    DefaultSafeMargin,
	}
}

// Valid reports whether c carries a known gear with finite non-negative
// tuning. Zero snap is valid (exact contact only); negative is not.
func (c SnapConfig) Valid() bool {
	return c.Mode.Valid() &&
		finite(c.SnapLength) && c.SnapLength >= 0 &&
		finite(c.FloorMaxAngle) && c.FloorMaxAngle >= 0 &&
		finite(c.SafeMargin) && c.SafeMargin >= 0
}

// FloorNormal returns the up-pointing unit normal of s (Y <= 0, since +Y
// runs down). Flat reports (0,-1); ramps tilt toward the low side exactly
// like SlideDir reflected upright. Vertical segments have no up normal
// and report ok=false; invalid slopes report ok=false.
func FloorNormal(s Slope) (core.Vec2, bool) {
	if !s.Valid() {
		return core.Vec2{}, false
	}
	dx := s.B().X - s.A().X
	dy := s.B().Y - s.A().Y
	if dx == 0 {
		return core.Vec2{}, false
	}
	n := core.V2(dy, -dx)
	if n.Y > 0 {
		n = core.V2(-dy, dx)
	}
	l := math.Hypot(n.X, n.Y)
	if l == 0 || !finite(l) {
		return core.Vec2{}, false
	}
	out := core.V2(n.X/l, n.Y/l)
	if !finiteVec(out) || out.Y > 0 {
		return core.Vec2{}, false
	}
	return out, true
}

// SnapGrounded reports the highest walkable slope whose surface sits
// within reach of feet: at most SnapLength above (gap to bridge) or at
// most SafeMargin below (sink allowance). Steep faces never count, so a
// ramp holds while a wall does not. Floating gear reports no floor.
// Invalid configs or bad feet report false, never a panic; invalid slopes
// are skipped soft like Grounded.
func SnapGrounded(feet core.Vec2, slopes []Slope, cfg SnapConfig) (Slope, bool) {
	if !finiteVec(feet) || !cfg.Valid() {
		return Slope{}, false
	}
	if cfg.Mode != MotionModeGrounded {
		return Slope{}, false
	}
	var best Slope
	var bestY float64
	found := false
	for _, s := range slopes {
		if !s.Valid() {
			continue
		}
		if !s.Walkable(cfg.FloorMaxAngle) {
			continue
		}
		y, ok := s.GroundYAt(feet.X)
		if !ok || !finite(y) {
			continue
		}
		if feet.Y < y-cfg.SnapLength || feet.Y > y+cfg.SafeMargin {
			continue
		}
		if !found || y < bestY {
			best, bestY, found = s, y, true
		}
	}
	return best, found
}

// IsOnFloor reports whether feet stand on a walkable floor under cfg:
// SnapGrounded with the floor bit only. Floating gear is never on floor.
func IsOnFloor(feet core.Vec2, slopes []Slope, cfg SnapConfig) bool {
	_, ok := SnapGrounded(feet, slopes, cfg)
	return ok
}

// StepWithSnap moves feet by vel*dt like Step, then applies the Godot
// apply_floor_snap window: a walkable surface within SnapLength below a
// falling or still body snaps to the surface and reports onFloor. Rising
// bodies never snap (jump passes through one-way, solid still bumps head
// via Step). A Step landing on a non-walkable face touches but is not
// floor (ground cleared, onFloor false). Floating gear delegates to Step
// for motion and reports onFloor false. dt==0 only samples SnapGrounded.
// Bad feet/vel/dt, any invalid slope, or an invalid config is InvalidArg
// with feet unchanged. The input slice is never mutated. Deliberate
// difference from Godot: the snap engages whenever a walkable surface
// sits inside the bounded window, without a was-on-floor precondition.
func StepWithSnap(feet, vel core.Vec2, dt float64, slopes []Slope, cfg SnapConfig) (core.Vec2, Slope, bool, bool, error) {
	if !finiteVec(feet) || !finiteVec(vel) || !finite(dt) || dt < 0 {
		return feet, Slope{}, false, false, core.InvalidArg("physics.StepWithSnap", "arg")
	}
	for _, s := range slopes {
		if !s.Valid() {
			return feet, Slope{}, false, false, core.InvalidArg("physics.StepWithSnap", "slopes")
		}
	}
	if !cfg.Valid() {
		return feet, Slope{}, false, false, core.InvalidArg("physics.StepWithSnap", "snap")
	}
	pos, ground, grounded, head, err := Step(feet, vel, dt, slopes)
	if err != nil {
		return feet, Slope{}, false, false, err
	}
	if cfg.Mode == MotionModeFloating {
		return pos, ground, false, head, nil
	}
	if grounded {
		if ground.Walkable(cfg.FloorMaxAngle) {
			return pos, ground, true, head, nil
		}
		return pos, Slope{}, false, head, nil
	}
	if head {
		return pos, Slope{}, false, true, nil
	}
	if vel.Y < 0 {
		return pos, Slope{}, false, false, nil
	}
	if dt == 0 {
		if g, ok := SnapGrounded(feet, slopes, cfg); ok {
			return feet, g, true, false, nil
		}
		return feet, Slope{}, false, false, nil
	}
	var best Slope
	var bestY float64
	found := false
	for _, s := range slopes {
		if !s.Walkable(cfg.FloorMaxAngle) {
			continue
		}
		y, ok := s.GroundYAt(pos.X)
		if !ok || !finite(y) {
			continue
		}
		if pos.Y < y-cfg.SnapLength || pos.Y > y+cfg.SafeMargin {
			continue
		}
		if !found || y < bestY {
			best, bestY, found = s, y, true
		}
	}
	if found {
		return core.V2(pos.X, bestY), best, true, false, nil
	}
	return pos, Slope{}, false, false, nil
}
