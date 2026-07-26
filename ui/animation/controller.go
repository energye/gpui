// Package animation provides a minimal Flutter-style AnimationController (P3).
package animation

import (
	"math"
	"sync"

	"github.com/energye/gpui/ui/scheduler"
)

// Controller drives a 0..1 value over DurationSec via the ticker registry.
// When the animation completes (or Stop is called with remove), Tick returns false
// so the registry auto-unregisters (no zombie tickers).
type Controller struct {
	mu sync.Mutex

	durationSec float64
	elapsed     float64
	value       float64 // 0..1
	running     bool
	repeat      bool

	onValue func(v float64)
	reg     *scheduler.TickerRegistry
}

// NewController creates a controller. durationSec <= 0 defaults to 1s.
func NewController(durationSec float64) *Controller {
	if durationSec <= 0 {
		durationSec = 1
	}
	return &Controller{durationSec: durationSec}
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

// OnValue sets the per-tick callback (may call MarkNeedsPaint).
func (c *Controller) OnValue(fn func(v float64)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.onValue = fn
	c.mu.Unlock()
}

// Value returns the current 0..1 progress.
func (c *Controller) Value() float64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
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

// Start registers with reg and begins advancing from 0 (or continues if already running).
func (c *Controller) Start(reg *scheduler.TickerRegistry) {
	if c == nil || reg == nil {
		return
	}
	c.mu.Lock()
	c.reg = reg
	c.running = true
	c.elapsed = 0
	c.value = 0
	fn := c.onValue
	c.mu.Unlock()
	reg.Add(c)
	if fn != nil {
		fn(0)
	}
}

// Stop unregisters and marks not running.
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
	if c.repeat {
		// Phase loops in [0,1).
		if c.durationSec > 0 {
			c.value = math.Mod(c.elapsed/c.durationSec, 1)
			if c.value < 0 {
				c.value += 1
			}
		}
		v := c.value
		fn := c.onValue
		c.mu.Unlock()
		if fn != nil {
			fn(v)
		}
		return true
	}
	// One-shot.
	if c.elapsed >= c.durationSec {
		c.value = 1
		c.running = false
		v := c.value
		fn := c.onValue
		c.mu.Unlock()
		if fn != nil {
			fn(v)
		}
		return false // auto-remove from registry
	}
	c.value = c.elapsed / c.durationSec
	v := c.value
	fn := c.onValue
	c.mu.Unlock()
	if fn != nil {
		fn(v)
	}
	return true
}
