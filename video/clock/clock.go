package clock

import (
	"fmt"
	"sync"
)

// Clock maps wall time to presentation stamps. The display asks DuePTSMS
// each tick and shows the newest queued frame at or before it; pausing
// freezes the mapping so the picture truly stops. Rate scales the walk:
// 2x advances stamps twice as fast (catch-up drops the surplus), 0.5x
// holds each frame twice as long. Time comes from an injected
// millisecond source, which keeps unit tests deterministic (production
// passes the wall clock).
type Clock struct {
	mu      sync.Mutex
	now     func() int64
	startMs int64
	anchor  int64
	paused  bool
	pauseMs int64
	heldMs  int64
	started bool
	rate    float64
}

// NewClock builds a clock over now (milliseconds). now must be monotonic.
// Rate starts at 1x.
func NewClock(now func() int64) *Clock {
	return &Clock{now: now, rate: 1}
}

// Start anchors stamp postage: the frame stamped anchor shows at once,
// later stamps follow in real time scaled by the current rate. Start
// keeps the rate (seeks re-anchor without dropping back to 1x).
func (c *Clock) Start(anchor int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.anchor = anchor
	c.startMs = c.now()
	c.heldMs = 0
	c.paused = false
	c.started = true
	if c.rate <= 0 {
		c.rate = 1
	}
}

// SetRate scales stamp progress (1 = normal, 2 = double, 0.5 = half).
// Non-positive or absurd rates are rejected so a bad caller cannot
// freeze or slingshot the picture.
func (c *Clock) SetRate(r float64) error {
	if r != r || r <= 0 || r > 8 {
		return fmt.Errorf("clock: bad rate %v (want 0 < rate <= 8)", r)
	}
	if c == nil {
		return fmt.Errorf("clock: nil clock")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rate = r
	return nil
}

// Rate reports the current walk speed.
func (c *Clock) Rate() float64 {
	if c == nil {
		return 1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return 1
	}
	return c.rate
}

// Pause freezes the picture: elapsed time stops advancing until Resume.
func (c *Clock) Pause() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started || c.paused {
		return
	}
	c.pauseMs = c.now()
	c.paused = true
}

// Resume continues after a pause; the paused span does not count.
func (c *Clock) Resume() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started || !c.paused {
		return
	}
	c.heldMs += c.now() - c.pauseMs
	c.paused = false
}

// Paused reports whether the clock is held.
func (c *Clock) Paused() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

// ElapsedMs is the playable time since Start, minus paused spans.
func (c *Clock) ElapsedMs() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return 0
	}
	end := c.now()
	if c.paused {
		end = c.pauseMs
	}
	e := end - c.startMs - c.heldMs
	if e < 0 {
		return 0
	}
	return e
}

// DuePTSMS is the newest stamp the display may show right now:
// anchor plus rate-scaled elapsed.
func (c *Clock) DuePTSMS() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return 0
	}
	end := c.now()
	if c.paused {
		end = c.pauseMs
	}
	e := end - c.startMs - c.heldMs
	if e < 0 {
		e = 0
	}
	rate := c.rate
	if rate <= 0 {
		rate = 1
	}
	return c.anchor + int64(float64(e)*rate)
}

// DriftMs tells how far a shown stamp lags the schedule (<= 0 means the
// picture is on time; positive means it is overdue).
func (c *Clock) DriftMs(shownPTSMS int64) int64 {
	return c.DuePTSMS() - shownPTSMS
}
