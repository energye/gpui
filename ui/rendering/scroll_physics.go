package rendering

import "math"

// ScrollPhysics applies boundary conditions and optional ballistic fling
// (Flutter ScrollPhysics subset — FScroll-PHYSICS minimal).
//
// Not claimed: BouncingScrollPhysics spring, platform pixel-perfect curves,
// multi-axis simultaneous physics, overscroll glow.
type ScrollPhysics interface {
	// AdjustPosition clamps or transforms a proposed scroll offset into [min,max].
	// When max < min (unset range), only min is enforced if finite.
	AdjustPosition(position, min, max float64) float64
	// CreateBallistic starts a fling from velocity (px/s, content scroll space).
	// Returns nil when velocity is negligible or physics does not fling.
	CreateBallistic(velocity, position, min, max float64) BallisticSimulation
}

// BallisticSimulation advances an inertial scroll until stop or boundary.
type BallisticSimulation interface {
	// Step integrates by dt seconds; returns new position and whether the sim ended.
	Step(dt float64) (position float64, done bool)
	// Velocity is the current px/s (may be 0 when done).
	Velocity() float64
}

// ClampingScrollPhysics hard-clamps to [min,max] and flings with constant
// deceleration until rest or edge (Android-like clamp, no bounce).
type ClampingScrollPhysics struct {
	// Deceleration is positive magnitude of friction in px/s² (default 6000).
	Deceleration float64
	// Tolerance stops the fling when |velocity| falls below this (default 50 px/s).
	Tolerance float64
}

// DefaultClampingScrollPhysics returns sensible fling defaults.
func DefaultClampingScrollPhysics() *ClampingScrollPhysics {
	return &ClampingScrollPhysics{Deceleration: 6000, Tolerance: 50}
}

func (p *ClampingScrollPhysics) decel() float64 {
	if p == nil || p.Deceleration <= 0 {
		return 6000
	}
	return p.Deceleration
}

func (p *ClampingScrollPhysics) tol() float64 {
	if p == nil || p.Tolerance <= 0 {
		return 50
	}
	return p.Tolerance
}

// AdjustPosition implements ScrollPhysics (hard clamp).
func (p *ClampingScrollPhysics) AdjustPosition(position, min, max float64) float64 {
	if position < min {
		return min
	}
	if max >= min && position > max {
		return max
	}
	return position
}

// CreateBallistic implements ScrollPhysics.
func (p *ClampingScrollPhysics) CreateBallistic(velocity, position, min, max float64) BallisticSimulation {
	if math.Abs(velocity) < p.tol() {
		return nil
	}
	pos := p.AdjustPosition(position, min, max)
	return &frictionBallistic{
		pos:   pos,
		vel:   velocity,
		min:   min,
		max:   max,
		decel: p.decel(),
		tol:   p.tol(),
	}
}

type frictionBallistic struct {
	pos, vel, min, max, decel, tol float64
}

func (b *frictionBallistic) Velocity() float64 {
	if b == nil {
		return 0
	}
	return b.vel
}

func (b *frictionBallistic) Step(dt float64) (float64, bool) {
	if b == nil {
		return 0, true
	}
	if dt < 0 {
		dt = 0
	}
	if math.Abs(b.vel) < b.tol {
		b.vel = 0
		b.pos = clampScroll(b.pos, b.min, b.max)
		return b.pos, true
	}
	sign := 1.0
	if b.vel < 0 {
		sign = -1
	}
	newVel := b.vel - sign*b.decel*dt
	// Crossed zero → stop.
	if newVel*b.vel < 0 {
		b.vel = 0
		b.pos = clampScroll(b.pos, b.min, b.max)
		return b.pos, true
	}
	b.vel = newVel
	b.pos += b.vel * dt
	if b.pos <= b.min {
		b.pos = b.min
		b.vel = 0
		return b.pos, true
	}
	if b.max >= b.min && b.pos >= b.max {
		b.pos = b.max
		b.vel = 0
		return b.pos, true
	}
	return b.pos, false
}

func clampScroll(pos, min, max float64) float64 {
	if pos < min {
		return min
	}
	if max >= min && pos > max {
		return max
	}
	return pos
}

// NeverScrollPhysics rejects all motion (keeps position; no fling).
type NeverScrollPhysics struct{}

func (NeverScrollPhysics) AdjustPosition(position, min, max float64) float64 {
	return clampScroll(position, min, max)
}

func (NeverScrollPhysics) CreateBallistic(velocity, position, min, max float64) BallisticSimulation {
	return nil
}
