package scheduler

import (
	"sync"
	"sync/atomic"
)

// Ticker is driven once per active frame (Flutter-style).
// Return false to unregister after this tick.
type Ticker interface {
	Tick(dt float64) bool
}

// FrameWanter is an optional per-tick frame-demand expression. Tickers
// that do not implement it keep the legacy behavior (registered means every
// tick requests a frame). Returning false on state-only ticks (blink phase,
// HUD budgets) lets the frame loop stay event-driven; frames then follow
// dirtiness instead of ticker aliveness.
type FrameWanter interface {
	WantsFrame() bool
}

// TickerRegistry holds active tickers (P0 skeleton; P3 uses for animations).
type TickerRegistry struct {
	mu      sync.Mutex
	tickers []Ticker
	// frameWanted is the OR of the last TickAll's per-ticker demand:
	// non-implementers count as true (legacy behavior).
	frameWanted atomic.Bool
}

// Add registers a ticker if not already present.
func (r *TickerRegistry) Add(t Ticker) {
	if r == nil || t == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.tickers {
		if x == t {
			return
		}
	}
	r.tickers = append(r.tickers, t)
}

// Remove unregisters a ticker.
func (r *TickerRegistry) Remove(t Ticker) {
	if r == nil || t == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.tickers[:0]
	for _, x := range r.tickers {
		if x != t {
			out = append(out, x)
		}
	}
	r.tickers = out
}

// HasActive reports whether any ticker is registered.
func (r *TickerRegistry) HasActive() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tickers) > 0
}

// TickAll advances all tickers with dt seconds; drops those returning false.
// It also recomputes the frame-demand flag (see FrameWanted).
func (r *TickerRegistry) TickAll(dt float64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	list := append([]Ticker(nil), r.tickers...)
	r.mu.Unlock()
	alive := make([]Ticker, 0, len(list))
	wanted := false
	for _, t := range list {
		if t == nil {
			continue
		}
		if !t.Tick(dt) {
			continue
		}
		alive = append(alive, t)
		if w, ok := t.(FrameWanter); !ok || w.WantsFrame() {
			wanted = true
		}
	}
	r.mu.Lock()
	r.tickers = alive
	r.mu.Unlock()
	r.frameWanted.Store(wanted)
}

// FrameWanted reports whether the last TickAll saw frame demand from any
// surviving ticker. False before the first tick and whenever every ticker
// either unregistered or opted out via FrameWanter.
func (r *TickerRegistry) FrameWanted() bool {
	if r == nil {
		return false
	}
	return r.frameWanted.Load()
}
