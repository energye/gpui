package step

import (
	"github.com/energye/gpui/game/core"
)

// MaxFrame caps one Advance input. A hitch longer than this parks at the
// cap so the loop never spirals into hundreds of catch-up ticks.
// 250ms follows Fix Your Timestep (Gaffer classic: frameTime = min(0.25)).
const MaxFrame = 250 * core.Millisecond

// Fixed is one fixed logic beat plus the render blend factor.
//
// The caller feeds each real frame into Advance, runs the returned number
// of fixed logic ticks, then blends the last two logic states with Alpha
// for the picture. Dt never changes mid-run; the ledger is integer
// milliseconds so slow and fast machines agree.
type Fixed struct {
	dt      core.Duration
	accum   core.Duration
	steps   uint64
	elapsed core.Duration
}

// NewFixed builds a loop with tick width dt. dt must be positive;
// zero or negative dt is a core InvalidArg error and stores nothing.
func NewFixed(dt core.Duration) (Fixed, error) {
	if dt <= 0 {
		return Fixed{}, core.InvalidArg("step.NewFixed", "dt")
	}
	return Fixed{dt: dt}, nil
}

// Dt returns the frozen tick width, or 0 on a nil loop.
func (f *Fixed) Dt() core.Duration {
	if f == nil {
		return 0
	}
	return f.dt
}

// Steps returns how many logic ticks Advance has produced in total.
func (f *Fixed) Steps() uint64 {
	if f == nil {
		return 0
	}
	return f.steps
}

// Elapsed returns steps*dt: logic time consumed so far.
func (f *Fixed) Elapsed() core.Duration {
	if f == nil {
		return 0
	}
	return f.elapsed
}

// Accum returns the leftover frame time waiting for the next tick.
func (f *Fixed) Accum() core.Duration {
	if f == nil {
		return 0
	}
	return f.accum
}

// Alpha returns accum/dt in [0,1): the blend factor for the picture.
// Advance keeps accum below dt and Reset clears it, so the division is
// already in range; nil and broken loops park at 0.
func (f *Fixed) Alpha() float64 {
	if f == nil || f.dt <= 0 {
		return 0
	}
	return float64(f.accum) / float64(f.dt)
}

// Advance folds one real frame into the loop and reports how many fixed
// ticks the caller must run. Negative frames park at 0, frames beyond
// MaxFrame clamp to MaxFrame. Nil loops report 0 and change nothing.
func (f *Fixed) Advance(frame core.Duration) int {
	if f == nil || f.dt <= 0 {
		return 0
	}
	f.accum += frame.Clamp(0, MaxFrame)
	n := int(f.accum / f.dt)
	f.accum -= core.Duration(n) * f.dt
	f.steps += uint64(n)
	f.elapsed += core.Duration(n) * f.dt
	return n
}

// Reset clears the remainder plus the totals, keeping Dt.
// Nil loops do nothing.
func (f *Fixed) Reset() {
	if f == nil {
		return
	}
	f.accum = 0
	f.steps = 0
	f.elapsed = 0
}

func clampAlpha(a float64) float64 {
	// !(a > 0) covers NaN, negative, and zero in one check.
	if !(a > 0) {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}

// Interp blends prev to curr with alpha. Alpha clamps to [0,1] first
// (NaN parks at prev); the ends pass through untouched. Prev and curr
// follow IEEE-754 without a guard, like anim.Lerp.
func Interp(prev, curr, alpha float64) float64 {
	alpha = clampAlpha(alpha)
	if alpha == 0 {
		return prev
	}
	if alpha == 1 {
		return curr
	}
	return prev + (curr-prev)*alpha
}

// InterpFloat is the explicit float64 blend; identical to Interp.
// Both names stay frozen so asset files and code read the same way.
func InterpFloat(prev, curr, alpha float64) float64 {
	return Interp(prev, curr, alpha)
}

// InterpVec blends two positions with alpha. Alpha handling matches
// Interp (clamped, NaN parks at prev); the ends pass through untouched.
func InterpVec(prev, curr core.Vec2, alpha float64) core.Vec2 {
	alpha = clampAlpha(alpha)
	if alpha == 0 {
		return prev
	}
	if alpha == 1 {
		return curr
	}
	return prev.Lerp(curr, alpha)
}
