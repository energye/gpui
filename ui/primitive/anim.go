package primitive

import "math"

// FloatAnim is a composable 1D property animation for demand-frame UI.
//
// Kit controls (Switch thumb, Progress polish, etc.) own a FloatAnim and
// register Tick via Tree.AddTicker — no per-control animation engine.
//
//	a := &FloatAnim{Current: 0, Duration: 0.2}
//	a.SetTarget(1)           // animate toward 1
//	tree.AddTicker(a)        // or wrap in control.Tick that calls a.Tick
//
// Easing defaults to ease-out cubic (Ant-like motion feel).
type FloatAnim struct {
	Current  float64
	Target   float64
	From     float64
	Duration float64 // seconds; <=0 snaps immediately
	elapsed  float64
	active   bool
	// Easing maps t∈[0,1] → [0,1]. nil → EaseOutCubic.
	Easing func(t float64) float64
	// OnUpdate optional; called when Current changes.
	OnUpdate func(v float64)
}

// EaseOutCubic is the default UI easing (fast start, soft end).
func EaseOutCubic(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	u := 1 - t
	return 1 - u*u*u
}

// EaseInOutCubic is a smoother bidirectional ease.
func EaseInOutCubic(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

// Snap jumps Current and Target without animation.
func (a *FloatAnim) Snap(v float64) {
	if a == nil {
		return
	}
	a.Current = v
	a.Target = v
	a.From = v
	a.elapsed = 0
	a.active = false
	if a.OnUpdate != nil {
		a.OnUpdate(v)
	}
}

// SetTarget starts (or restarts) an animation toward to.
// Uses a.Duration; if Duration<=0, snaps.
func (a *FloatAnim) SetTarget(to float64) {
	if a == nil {
		return
	}
	if a.Duration <= 0 || a.Current == to {
		a.Snap(to)
		return
	}
	a.From = a.Current
	a.Target = to
	a.elapsed = 0
	a.active = true
}

// SetTargetDuration is SetTarget with an explicit duration override.
func (a *FloatAnim) SetTargetDuration(to, duration float64) {
	if a == nil {
		return
	}
	prev := a.Duration
	a.Duration = duration
	a.SetTarget(to)
	a.Duration = prev
}

// Active reports whether an animation is in progress.
func (a *FloatAnim) Active() bool { return a != nil && a.active }

// Tick advances the animation. Returns still=true while animating.
// Implements core.Ticker when used as a tree ticker.
func (a *FloatAnim) Tick(dt float64) (still bool) {
	if a == nil || !a.active {
		return false
	}
	if dt < 0 {
		dt = 0
	}
	a.elapsed += dt
	t := 1.0
	if a.Duration > 0 {
		t = a.elapsed / a.Duration
	}
	if t >= 1 {
		a.Current = a.Target
		a.active = false
		if a.OnUpdate != nil {
			a.OnUpdate(a.Current)
		}
		return false
	}
	ease := a.Easing
	if ease == nil {
		ease = EaseOutCubic
	}
	e := ease(t)
	a.Current = a.From + (a.Target-a.From)*e
	if a.OnUpdate != nil {
		a.OnUpdate(a.Current)
	}
	return true
}

// Lerp is a pure linear interpolate helper for one-shot use.
func Lerp(a, b, t float64) float64 {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return a + (b-a)*t
}

// Clamp01 clamps t to [0,1].
func Clamp01(t float64) float64 {
	return math.Max(0, math.Min(1, t))
}
