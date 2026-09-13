package clock

// Clock maps wall time to presentation stamps. The display asks DuePTSMS
// each tick and shows the newest queued frame at or before it; pausing
// freezes the mapping so the picture truly stops. Time comes from an
// injected millisecond source, which keeps unit tests deterministic
// (production passes the wall clock).
type Clock struct {
	now     func() int64
	startMs int64
	anchor  int64
	paused  bool
	pauseMs int64
	heldMs  int64
	started bool
}

// NewClock builds a clock over now (milliseconds). now must be monotonic.
func NewClock(now func() int64) *Clock {
	return &Clock{now: now}
}

// Start anchors stamp postage: the frame stamped anchor shows at once,
// later stamps follow in real time.
func (c *Clock) Start(anchor int64) {
	c.anchor = anchor
	c.startMs = c.now()
	c.heldMs = 0
	c.paused = false
	c.started = true
}

// Pause freezes the picture: elapsed time stops advancing until Resume.
func (c *Clock) Pause() {
	if c == nil || !c.started || c.paused {
		return
	}
	c.pauseMs = c.now()
	c.paused = true
}

// Resume continues after a pause; the paused span does not count.
func (c *Clock) Resume() {
	if c == nil || !c.started || !c.paused {
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
	return c.paused
}

// ElapsedMs is the playable time since Start, minus paused spans.
func (c *Clock) ElapsedMs() int64 {
	if c == nil || !c.started {
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

// DuePTSMS is the newest stamp the display may show right now.
func (c *Clock) DuePTSMS() int64 {
	if c == nil || !c.started {
		return 0
	}
	return c.anchor + c.ElapsedMs()
}

// DriftMs tells how far a shown stamp lags the schedule (<= 0 means the
// picture is on time; positive means it is overdue).
func (c *Clock) DriftMs(shownPTSMS int64) int64 {
	return c.DuePTSMS() - shownPTSMS
}
