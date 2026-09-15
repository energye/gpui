package physics

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// slopeSnap is the contact tolerance for Grounded and Step only. Overlaps
// keeps its exact edge rule; standing allows this snap so a rider carried
// by identical float deltas stays grounded across steps. Name differs from
// 14.1b standingSnap on purpose: both live in one package.
const slopeSnap = 1e-9

// Slope is one thin walk surface: segment A-B plus the one-way flag.
// +Y runs down (tilemap CellToWorld units); above means smaller Y.
// A one-way slope only lands from above, a solid slope also reports a
// head bump when crossed from below. The segment is thin: top and bottom
// share one Y, so Grounded alone cannot tell the side; Step tells by
// motion direction.
type Slope struct {
	name   string
	a      core.Vec2
	b      core.Vec2
	oneWay bool
}

// NewSlope builds a solid slope from a to b.
func NewSlope(name string, a, b core.Vec2) (Slope, error) {
	if !finiteVec(a) || !finiteVec(b) {
		return Slope{}, core.InvalidArg("physics.NewSlope", "pos")
	}
	if a == b {
		return Slope{}, core.InvalidArg("physics.NewSlope", "pos")
	}
	return Slope{name: name, a: a, b: b}, nil
}

// NewOneWay builds a one-way slope from a to b. Any angle is accepted;
// the usual case is horizontal. Landing matches NewSlope, the rising
// pass-through never reports a head bump.
func NewOneWay(name string, a, b core.Vec2) (Slope, error) {
	if !finiteVec(a) || !finiteVec(b) {
		return Slope{}, core.InvalidArg("physics.NewOneWay", "pos")
	}
	if a == b {
		return Slope{}, core.InvalidArg("physics.NewOneWay", "pos")
	}
	return Slope{name: name, a: a, b: b, oneWay: true}, nil
}

// Name returns the debug key, never parsed.
func (s Slope) Name() string { return s.name }

// A returns the first endpoint.
func (s Slope) A() core.Vec2 { return s.a }

// B returns the second endpoint.
func (s Slope) B() core.Vec2 { return s.b }

// OneWay reports whether the slope is top-only.
func (s Slope) OneWay() bool { return s.oneWay }

// Valid reports whether s carries finite distinct endpoints.
func (s Slope) Valid() bool {
	return finiteVec(s.a) && finiteVec(s.b) && s.a != s.b
}

// GroundYAt returns the surface Y at column x. Vertical segments are not
// a function of x and report ok=false; x outside the inclusive endpoint
// span reports ok=false. Endpoints count as inside.
func (s Slope) GroundYAt(x float64) (float64, bool) {
	if !s.Valid() || !finite(x) {
		return 0, false
	}
	if s.a.X == s.b.X {
		return 0, false
	}
	lo, hi := s.a.X, s.b.X
	if lo > hi {
		lo, hi = hi, lo
	}
	if x < lo || x > hi {
		return 0, false
	}
	t := (x - s.a.X) / (s.b.X - s.a.X)
	y := s.a.Y + (s.b.Y-s.a.Y)*t
	if !finite(y) {
		return 0, false
	}
	return y, true
}

// Angle returns the slope angle from horizontal in [0, pi/2]: 0 is flat,
// pi/2 is vertical. Invalid slopes report ok=false.
func (s Slope) Angle() (float64, bool) {
	if !s.Valid() {
		return 0, false
	}
	dx := math.Abs(s.b.X - s.a.X)
	dy := math.Abs(s.b.Y - s.a.Y)
	angle := math.Atan2(dy, dx)
	if !finite(angle) {
		return 0, false
	}
	return angle, true
}

// Walkable reports whether the slope is gentle enough to stand on:
// angle <= maxAngle with exact comparison. Bad slopes or a bad maxAngle
// (non-finite or negative) report false. The caller owns maxAngle.
func (s Slope) Walkable(maxAngle float64) bool {
	if !s.Valid() || !finite(maxAngle) || maxAngle < 0 {
		return false
	}
	angle, ok := s.Angle()
	if !ok {
		return false
	}
	return angle <= maxAngle
}

// SlideDir returns the downhill unit vector (+Y down). Flat segments have
// no downhill and report ok=false; vertical segments slide straight down
// (0,1). Invalid slopes report ok=false.
func (s Slope) SlideDir() (core.Vec2, bool) {
	if !s.Valid() {
		return core.Vec2{}, false
	}
	dx := s.b.X - s.a.X
	dy := s.b.Y - s.a.Y
	if dx == 0 {
		return core.V2(0, 1), true
	}
	if dy == 0 {
		return core.Vec2{}, false
	}
	var dir core.Vec2
	if s.b.Y > s.a.Y {
		dir = s.b.Sub(s.a)
	} else {
		dir = s.a.Sub(s.b)
	}
	n := math.Hypot(dir.X, dir.Y)
	if n == 0 || !finite(n) {
		return core.Vec2{}, false
	}
	out := core.V2(dir.X/n, dir.Y/n)
	if !finiteVec(out) {
		return core.Vec2{}, false
	}
	return out, true
}

// FeetOf returns the sole point of b: box center plus half height, circle
// center plus radius. It only reads b; invalid bodies report ok=false.
func FeetOf(b Body) (core.Vec2, bool) {
	if !b.Valid() {
		return core.Vec2{}, false
	}
	switch b.Shape {
	case ShapeBox:
		feet := core.V2(b.Pos.X, b.Pos.Y+b.Half.Y)
		if !finiteVec(feet) {
			return core.Vec2{}, false
		}
		return feet, true
	case ShapeCircle:
		feet := core.V2(b.Pos.X, b.Pos.Y+b.Radius)
		if !finiteVec(feet) {
			return core.Vec2{}, false
		}
		return feet, true
	default:
		return core.Vec2{}, false
	}
}

// WithFeet returns a copy of b whose sole sits at feet: Pos.X follows the
// feet column, Pos.Y backs off by half height or radius. The input is
// never mutated; bad bodies or bad feet report ok=false.
func WithFeet(b Body, feet core.Vec2) (Body, bool) {
	if !b.Valid() || !finiteVec(feet) {
		return Body{}, false
	}
	nb := b
	switch b.Shape {
	case ShapeBox:
		nb.Pos = core.V2(feet.X, feet.Y-b.Half.Y)
	case ShapeCircle:
		nb.Pos = core.V2(feet.X, feet.Y-b.Radius)
	default:
		return Body{}, false
	}
	if !nb.Valid() {
		return Body{}, false
	}
	return nb, true
}

// Grounded reports the highest slope whose surface sits within slopeSnap
// of feet at the feet column. Invalid slopes are skipped (soft, like
// tilemap At); ties keep the first input. One-way counts the same as
// solid here: directionless touch only, Step decides the side.
func Grounded(feet core.Vec2, slopes []Slope) (Slope, bool) {
	if !finiteVec(feet) {
		return Slope{}, false
	}
	var best Slope
	var bestY float64
	found := false
	for _, s := range slopes {
		if !s.Valid() {
			continue
		}
		y, ok := s.GroundYAt(feet.X)
		if !ok {
			continue
		}
		if math.Abs(feet.Y-y) > slopeSnap {
			continue
		}
		if !found || y < bestY {
			best, bestY, found = s, y, true
		}
	}
	return best, found
}

// Step moves feet by vel*dt and resolves one thin-segment contact.
// Falling or still (vel.Y >= 0): the highest crossed surface between the
// old and new columns snaps to the surface and reports grounded. Rising
// (vel.Y < 0): staying on the same slope still reports grounded, a solid
// slope crossed from below reports hitHead with the feet clamped to the
// ceiling, one-way slopes never report head. dt==0 only samples Grounded.
// Any invalid slope fails closed with InvalidArg like Query; bad feet,
// vel, or dt (non-finite or dt < 0) is InvalidArg with feet unchanged.
// The input slice is never mutated. Walking a slope needs velocity along
// the slope (or gravity next step); pure horizontal motion into a slope
// side does not auto-snap.
func Step(feet, vel core.Vec2, dt float64, slopes []Slope) (core.Vec2, Slope, bool, bool, error) {
	if !finiteVec(feet) || !finiteVec(vel) || !finite(dt) || dt < 0 {
		return feet, Slope{}, false, false, core.InvalidArg("physics.Step", "arg")
	}
	for _, s := range slopes {
		if !s.Valid() {
			return feet, Slope{}, false, false, core.InvalidArg("physics.Step", "slopes")
		}
	}
	if dt == 0 {
		g, ok := Grounded(feet, slopes)
		return feet, g, ok, false, nil
	}
	nx := feet.X + vel.X*dt
	ny := feet.Y + vel.Y*dt
	if !finite(nx) || !finite(ny) {
		return feet, Slope{}, false, false, core.InvalidArg("physics.Step", "move")
	}
	next := core.V2(nx, ny)
	if vel.Y >= 0 {
		var best Slope
		var bestY float64
		found := false
		for _, s := range slopes {
			y, ok := s.GroundYAt(nx)
			if !ok {
				continue
			}
			if feet.Y <= y+slopeSnap && ny >= y-slopeSnap {
				if !found || y < bestY {
					best, bestY, found = s, y, true
				}
			}
		}
		if found {
			return core.V2(nx, bestY), best, true, false, nil
		}
		if g, ok := Grounded(next, slopes); ok {
			return next, g, true, false, nil
		}
		return next, Slope{}, false, false, nil
	}
	for _, s := range slopes {
		yOld, okOld := s.GroundYAt(feet.X)
		yNew, okNew := s.GroundYAt(nx)
		if okOld && okNew &&
			math.Abs(feet.Y-yOld) <= slopeSnap && math.Abs(ny-yNew) <= slopeSnap {
			if g, ok := Grounded(next, slopes); ok {
				_ = g
				return next, s, true, false, nil
			}
		}
	}
	var headY float64
	foundHead := false
	for _, s := range slopes {
		if s.oneWay {
			continue
		}
		y, ok := s.GroundYAt(nx)
		if !ok {
			continue
		}
		if yOld, okOld := s.GroundYAt(feet.X); okOld && math.Abs(feet.Y-yOld) <= slopeSnap {
			continue
		}
		if feet.Y >= y-slopeSnap && ny <= y+slopeSnap {
			if !foundHead || y > headY {
				headY, foundHead = y, true
			}
		}
	}
	if foundHead {
		return core.V2(nx, headY), Slope{}, false, true, nil
	}
	return next, Slope{}, false, false, nil
}
