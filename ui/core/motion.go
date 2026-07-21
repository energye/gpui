package core

import "math"

// Clock is the animation time source (C-Motion).
// Host advances it each frame via Tick.
type Clock struct {
	// T is monotonic seconds since start (or host epoch).
	T float64
	// DT is last frame delta seconds.
	DT float64
	// ReduceMotion skips non-essential animations when true.
	ReduceMotion bool
}

// Tick advances the clock by dt seconds.
func (c *Clock) Tick(dt float64) {
	if c == nil {
		return
	}
	if dt < 0 {
		dt = 0
	}
	// Clamp pathological spikes
	if dt > 0.1 {
		dt = 0.1
	}
	c.DT = dt
	c.T += dt
}

// Now returns current time.
func (c *Clock) Now() float64 {
	if c == nil {
		return 0
	}
	return c.T
}

// EaseFunc maps t in [0,1] → eased [0,1].
type EaseFunc func(t float64) float64

// EaseLinear is identity.
func EaseLinear(t float64) float64 { return t }

// EaseOutCubic is a common UI ease-out.
func EaseOutCubic(t float64) float64 {
	t = clamp01(t)
	u := 1 - t
	return 1 - u*u*u
}

// EaseInOutCubic smoothstep-like cubic.
func EaseInOutCubic(t float64) float64 {
	t = clamp01(t)
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

func clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

// Lerp interpolates a→b by t.
func Lerp(a, b, t float64) float64 { return a + (b-a)*t }

// Anim is a one-shot or looping progress value.
type Anim struct {
	// Duration seconds; 0 → complete immediately.
	Duration float64
	// Delay before start.
	Delay float64
	// Loop when true restarts after complete.
	Loop bool
	// Ease defaults to EaseOutCubic.
	Ease EaseFunc
	// Reverse plays backward after forward when Loop (ping-pong).
	Reverse bool

	// Elapsed since start (includes delay phase as negative progress).
	elapsed float64
	// running
	started bool
	// done for non-loop
	done bool
	// reverse phase
	rev bool
}

// Start (re)starts the animation.
func (a *Anim) Start() {
	if a == nil {
		return
	}
	a.elapsed = 0
	a.started = true
	a.done = false
	a.rev = false
}

// Stop freezes at current value.
func (a *Anim) Stop() {
	if a != nil {
		a.started = false
	}
}

// Done reports completion (non-loop).
func (a *Anim) Done() bool {
	return a == nil || a.done
}

// Advance by dt; returns eased progress 0..1.
func (a *Anim) Advance(dt float64) float64 {
	if a == nil {
		return 1
	}
	if !a.started {
		if a.done {
			return 1
		}
		return 0
	}
	a.elapsed += dt
	if a.elapsed < a.Delay {
		return 0
	}
	local := a.elapsed - a.Delay
	dur := a.Duration
	if dur <= 0 {
		a.done = true
		a.started = false
		return 1
	}
	t := local / dur
	if a.Loop {
		if a.Reverse {
			// ping-pong: 0→1→0
			cycle := math.Mod(local, dur*2)
			if cycle > dur {
				t = 1 - (cycle-dur)/dur
			} else {
				t = cycle / dur
			}
		} else {
			t = math.Mod(local, dur) / dur
		}
	} else if t >= 1 {
		t = 1
		a.done = true
		a.started = false
	}
	ease := a.Ease
	if ease == nil {
		ease = EaseOutCubic
	}
	return ease(clamp01(t))
}

// Progress is the last Advance result if you cache it; convenience for one-shot.
func (a *Anim) Progress(clock *Clock) float64 {
	if clock == nil {
		return a.Advance(0)
	}
	return a.Advance(clock.DT)
}

// TreeClock returns the tree clock or a zero clock.
func (t *Tree) Clock() *Clock {
	if t == nil {
		return &Clock{}
	}
	if t.clock == nil {
		t.clock = &Clock{}
	}
	return t.clock
}

// TickClock advances the tree animation clock only (does not mark dirty).
// Demand-driven hosts must not treat a clock tick as a paint request.
// Use AddTicker + TickActive for animations; visual changes call MarkNeedsPaint.
// Deprecated path for hosts that still call TickClock: prefer TickActive.
func (t *Tree) TickClock(dt float64) {
	if t == nil {
		return
	}
	t.Clock().Tick(dt)
}
