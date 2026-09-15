package step

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// MaxScale caps one SetTimeScale input. Larger parks at the cap so a bad
// slider never explodes the fixed loop (250ms*8 stays bounded; Fixed
// clamps the fed world delta again).
const MaxScale = 8.0

// Clock is layered game time: one real frame splits into a scaled world
// delta and an unscaled UI delta. Pause stops the world only; the UI
// keeps moving. Only core numbers are used.
type Clock struct {
	scale  float64
	paused bool
	world  core.Duration
	ui     core.Duration
}

// NewClock builds an unpaused clock at 1x with zero totals.
func NewClock() Clock { return Clock{scale: 1} }

// normalizeScale pins s to [0, MaxScale]. NaN, zero, and negative park
// at 0 (world stops instead of running backwards); +Inf and over-cap
// park at MaxScale. Unlike NewFixed it never errors.
func normalizeScale(s float64) float64 {
	if !(s > 0) {
		return 0
	}
	if math.IsInf(s, 1) {
		return MaxScale
	}
	if s > MaxScale {
		return MaxScale
	}
	return s
}

// SetTimeScale stores s clamped to [0, MaxScale]. Nil clocks do nothing.
func (c *Clock) SetTimeScale(s float64) {
	if c == nil {
		return
	}
	c.scale = normalizeScale(s)
}

// TimeScale returns the stored scale, or 0 on a nil clock.
func (c *Clock) TimeScale() float64 {
	if c == nil {
		return 0
	}
	return c.scale
}

// SetPaused stores the pause flag. Nil clocks do nothing.
func (c *Clock) SetPaused(b bool) {
	if c == nil {
		return
	}
	c.paused = b
}

// Paused reports whether the world is stopped. Nil clocks report false.
func (c *Clock) Paused() bool {
	if c == nil {
		return false
	}
	return c.paused
}

// Pause stops the world; the UI keeps moving. Nil clocks do nothing.
func (c *Clock) Pause() {
	if c == nil {
		return
	}
	c.paused = true
}

// Resume restarts the world. Nil clocks do nothing.
func (c *Clock) Resume() {
	if c == nil {
		return
	}
	c.paused = false
}

// Split folds one real frame into world and UI deltas without state.
// The frame clamps to [0, MaxFrame] first (shared with Fixed); the UI
// takes the clamped frame, the world takes the scaled frame or 0 when
// paused. World may exceed MaxFrame at scale > 1; feed it into
// Fixed.Advance which clamps again so a hitch never spirals.
func Split(frame core.Duration, scale float64, paused bool) (world, ui core.Duration) {
	clamped := frame.Clamp(0, MaxFrame)
	ui = clamped
	if paused {
		return 0, ui
	}
	return clamped.Scale(normalizeScale(scale)), ui
}

// Advance splits one real frame and adds both deltas to the totals.
// Nil clocks report 0, 0 and change nothing.
func (c *Clock) Advance(frame core.Duration) (world, ui core.Duration) {
	if c == nil {
		return 0, 0
	}
	world, ui = Split(frame, c.scale, c.paused)
	c.world += world
	c.ui += ui
	return world, ui
}

// WorldElapsed returns the scaled world total. Nil clocks return 0.
func (c *Clock) WorldElapsed() core.Duration {
	if c == nil {
		return 0
	}
	return c.world
}

// UIElapsed returns the unscaled UI total. Nil clocks return 0.
func (c *Clock) UIElapsed() core.Duration {
	if c == nil {
		return 0
	}
	return c.ui
}

// Reset clears both totals, keeping scale and pause. Nil clocks do nothing.
func (c *Clock) Reset() {
	if c == nil {
		return
	}
	c.world = 0
	c.ui = 0
}
