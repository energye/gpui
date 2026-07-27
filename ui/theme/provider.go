package theme

import "sync"

// Provider holds the active token set with an optional override stack.
//
// Command-style API: Push/Pop for local scopes (e.g. dialog chrome).
// Thread-safe for concurrent read; Push/Pop intended on UI thread.
type Provider struct {
	mu    sync.RWMutex
	base  Tokens
	stack []Tokens
}

// NewProvider creates a provider with the given base tokens.
// If base is zero-ish (FontSize==0), DefaultTokens is used.
func NewProvider(base Tokens) *Provider {
	if base.FontSize <= 0 {
		base = DefaultTokens()
	}
	return &Provider{base: base}
}

// Default is the process-wide provider (lazy DefaultTokens).
var Default = NewProvider(DefaultTokens())

// Current returns the top-of-stack tokens, or base if the stack is empty.
func (p *Provider) Current() Tokens {
	if p == nil {
		return DefaultTokens()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if n := len(p.stack); n > 0 {
		return p.stack[n-1]
	}
	return p.base
}

// Base returns the root tokens (ignoring overrides).
func (p *Provider) Base() Tokens {
	if p == nil {
		return DefaultTokens()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.base
}

// SetBase replaces the root tokens (does not clear the override stack).
func (p *Provider) SetBase(t Tokens) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.base = t
	p.mu.Unlock()
}

// Push pushes an override (typically a modified copy of Current()).
func (p *Provider) Push(t Tokens) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.stack = append(p.stack, t)
	p.mu.Unlock()
}

// Pop removes the top override. No-op if empty.
func (p *Provider) Pop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if n := len(p.stack); n > 0 {
		p.stack = p.stack[:n-1]
	}
	p.mu.Unlock()
}

// Depth returns override stack depth.
func (p *Provider) Depth() int {
	if p == nil {
		return 0
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.stack)
}

// With runs fn under tokens t and restores the previous stack depth.
func (p *Provider) With(t Tokens, fn func()) {
	if p == nil {
		if fn != nil {
			fn()
		}
		return
	}
	p.Push(t)
	defer p.Pop()
	if fn != nil {
		fn()
	}
}
