package core

// Ticker is an animation driver registered on a Tree (gogpu ANIMATING alignment).
// Tick advances by dt seconds and returns whether the ticker should stay active.
// When still is false, the tree removes the ticker automatically after TickActive.
type Ticker interface {
	Tick(dt float64) (still bool)
}

// tickerEntry holds a registered ticker (pointer identity for Remove).
type tickerEntry struct {
	t Ticker
}

// AddTicker registers t for TickActive. Safe to call multiple times with the
// same t (deduped by identity). Marks the tree dirty so a frame is scheduled.
func (t *Tree) AddTicker(tk Ticker) {
	if t == nil || tk == nil {
		return
	}
	for _, e := range t.tickers {
		if e.t == tk {
			return
		}
	}
	t.tickers = append(t.tickers, tickerEntry{t: tk})
	t.markDirty()
}

// RemoveTicker unregisters t. No-op if not present.
func (t *Tree) RemoveTicker(tk Ticker) {
	if t == nil || tk == nil {
		return
	}
	out := t.tickers[:0]
	for _, e := range t.tickers {
		if e.t != tk {
			out = append(out, e)
		}
	}
	t.tickers = out
}

// HasActiveTickers reports whether any animation ticker is registered.
// Aligns with gogpu animations.IsAnimating() for frame scheduling.
func (t *Tree) HasActiveTickers() bool {
	return t != nil && len(t.tickers) > 0
}

// TickActive advances the tree clock and all registered tickers.
// Returns true if any ticker remains active after this step.
// Does not mark dirty by itself — tickers that change visuals should call
// MarkNeedsPaint / MarkDirty from Tick, or the host should RequestRedraw when
// still is true (app layer treats HasActiveTickers as keep-loop-alive only).
func (t *Tree) TickActive(dt float64) bool {
	if t == nil {
		return false
	}
	t.Clock().Tick(dt)
	if len(t.tickers) == 0 {
		return false
	}
	alive := t.tickers[:0]
	for _, e := range t.tickers {
		if e.t == nil {
			continue
		}
		if e.t.Tick(dt) {
			alive = append(alive, e)
		}
	}
	t.tickers = alive
	return len(t.tickers) > 0
}

// NeedsFrame reports whether a layout/paint/present pass is required.
// True when the tree is dirty or animation tickers are active (caller may still
// only paint when dirty — see app demand modes).
func (t *Tree) NeedsFrame() bool {
	if t == nil {
		return false
	}
	return t.dirty || len(t.tickers) > 0
}

// SetOnDirty registers an optional callback invoked when the tree becomes dirty
// (MarkDirty / MarkNeeds*). Used by app.Invalidator / host WakeUp.
// Callback must not re-enter tree mutation that deadlocks; keep it signal-only.
func (t *Tree) SetOnDirty(fn func()) {
	if t == nil {
		return
	}
	t.onDirty = fn
}
