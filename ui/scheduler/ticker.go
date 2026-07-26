package scheduler

import "sync"

// Ticker is driven once per active frame (Flutter-style).
// Return false to unregister after this tick.
type Ticker interface {
	Tick(dt float64) bool
}

// TickerRegistry holds active tickers (P0 skeleton; P3 uses for animations).
type TickerRegistry struct {
	mu      sync.Mutex
	tickers []Ticker
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
func (r *TickerRegistry) TickAll(dt float64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	list := append([]Ticker(nil), r.tickers...)
	r.mu.Unlock()
	alive := make([]Ticker, 0, len(list))
	for _, t := range list {
		if t != nil && t.Tick(dt) {
			alive = append(alive, t)
		}
	}
	r.mu.Lock()
	r.tickers = alive
	r.mu.Unlock()
}
