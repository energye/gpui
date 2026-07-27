// Package animation provides a Flutter-style AnimationController (P3/P5e).
package animation

import (
	"math"
	"sync"

	"github.com/energye/gpui/ui/scheduler"
)

// Controller drives a 0..1 value over DurationSec via the ticker registry.
// When the animation completes (or Stop is called), Tick returns false so the
// registry auto-unregisters (no zombie tickers / F17).
//
// Value() is the curved progress (after Curve). Linear progress is internal.
type Controller struct {
	mu sync.Mutex

	durationSec float64
	elapsed     float64
	// linear is uncurved progress in [0,1].
	linear float64
	// value is curved progress reported to listeners.
	value   float64
	running bool
	repeat  bool
	reverse bool // when true, linear goes 1→0

	curve  Curve
	status Status

	onValue  func(v float64)
	onStatus func(s Status)
	reg      *scheduler.TickerRegistry
}

// NewController creates a controller. durationSec <= 0 defaults to 1s.
// Default curve is Linear; status is Dismissed.
func NewController(durationSec float64) *Controller {
	if durationSec <= 0 {
		durationSec = 1
	}
	return &Controller{
		durationSec: durationSec,
		curve:       CurveLinear,
		status:      StatusDismissed,
	}
}

// SetCurve sets the progress curve (nil → Linear).
func (c *Controller) SetCurve(curve Curve) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if curve == nil {
		curve = CurveLinear
	}
	c.curve = curve
	c.value = applyCurve(c.curve, c.linear)
	c.mu.Unlock()
}

// Curve returns the active curve (never nil after NewController).
func (c *Controller) Curve() Curve {
	if c == nil {
		return CurveLinear
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.curve == nil {
		return CurveLinear
	}
	return c.curve
}

// SetRepeat enables looping (spinner). When true, Tick never auto-stops.
func (c *Controller) SetRepeat(v bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.repeat = v
	c.mu.Unlock()
}

// OnValue sets the per-tick callback (may call MarkNeedsPaint / emit mutations).
// Receives the curved value in [0,1].
func (c *Controller) OnValue(fn func(v float64)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.onValue = fn
	c.mu.Unlock()
}

// OnStatus sets a callback for status transitions.
func (c *Controller) OnStatus(fn func(s Status)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.onStatus = fn
	c.mu.Unlock()
}

// Status returns the current lifecycle status.
func (c *Controller) Status() Status {
	if c == nil {
		return StatusDismissed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// Value returns the current curved 0..1 progress.
func (c *Controller) Value() float64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// LinearProgress returns uncurved progress in [0,1] (tests / advanced).
func (c *Controller) LinearProgress() float64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.linear
}

// IsRunning reports whether the controller is active.
func (c *Controller) IsRunning() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Start registers with reg and begins advancing from 0 forward.
func (c *Controller) Start(reg *scheduler.TickerRegistry) {
	if c == nil || reg == nil {
		return
	}
	c.mu.Lock()
	c.reg = reg
	c.running = true
	c.reverse = false
	c.elapsed = 0
	c.linear = 0
	c.value = applyCurve(c.curve, 0)
	fn := c.onValue
	c.mu.Unlock()
	c.setStatus(StatusForward)
	reg.Add(c)
	if fn != nil {
		fn(0)
	}
}

// Reverse starts or continues animating from current linear progress toward 0.
// Registers with reg if not already running.
func (c *Controller) Reverse(reg *scheduler.TickerRegistry) {
	if c == nil {
		return
	}
	if reg == nil {
		c.mu.Lock()
		reg = c.reg
		c.mu.Unlock()
	}
	if reg == nil {
		return
	}
	c.mu.Lock()
	c.reg = reg
	c.running = true
	c.reverse = true
	// Map current linear to elapsed for reverse: elapsed = (1-linear)*duration
	if c.durationSec > 0 {
		c.elapsed = (1 - c.linear) * c.durationSec
	}
	v := applyCurve(c.curve, c.linear)
	c.value = v
	fn := c.onValue
	wasRunning := false
	// check if already in registry — always Add is ok if registry dedups? safer Remove+Add
	c.mu.Unlock()
	c.setStatus(StatusReverse)
	reg.Remove(c)
	reg.Add(c)
	_ = wasRunning
	if fn != nil {
		fn(v)
	}
}

// Stop unregisters and marks not running (status → Dismissed).
func (c *Controller) Stop() {
	if c == nil {
		return
	}
	c.mu.Lock()
	reg := c.reg
	c.running = false
	c.mu.Unlock()
	if reg != nil {
		reg.Remove(c)
	}
	c.setStatus(StatusDismissed)
}

func (c *Controller) setStatus(s Status) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.status == s {
		c.mu.Unlock()
		return
	}
	c.status = s
	fn := c.onStatus
	c.mu.Unlock()
	if fn != nil {
		fn(s)
	}
}

// Tick implements scheduler.Ticker.
func (c *Controller) Tick(dt float64) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return false
	}
	if dt < 0 {
		dt = 0
	}
	c.elapsed += dt
	dur := c.durationSec
	if dur <= 0 {
		dur = 1
	}

	if c.repeat {
		// Phase loops in [0,1).
		c.linear = math.Mod(c.elapsed/dur, 1)
		if c.linear < 0 {
			c.linear += 1
		}
		if c.reverse {
			c.linear = 1 - c.linear
		}
		c.value = applyCurve(c.curve, c.linear)
		v := c.value
		fn := c.onValue
		c.mu.Unlock()
		if fn != nil {
			fn(v)
		}
		return true
	}

	// One-shot forward or reverse.
	if c.elapsed >= dur {
		if c.reverse {
			c.linear = 0
		} else {
			c.linear = 1
		}
		c.value = applyCurve(c.curve, c.linear)
		c.running = false
		v := c.value
		fn := c.onValue
		c.mu.Unlock()
		if fn != nil {
			fn(v)
		}
		c.setStatus(StatusCompleted)
		return false // auto-remove from registry (F17)
	}

	t := c.elapsed / dur
	if c.reverse {
		c.linear = 1 - t
	} else {
		c.linear = t
	}
	c.value = applyCurve(c.curve, c.linear)
	v := c.value
	fn := c.onValue
	c.mu.Unlock()
	if fn != nil {
		fn(v)
	}
	return true
}
