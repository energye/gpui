package behavior

// Performance facade (F0-5 §6.4): budgets plus isolation helpers.
// It never measures GPU itself; it owns the thresholds and the
// cache/virtual bookkeeping that product windows assert against.

// Budgets mirrors ENGINE §2.2.2 steady-window expectations.
type Budgets struct {
	MinFPSWall float64
	MaxP95Ms   float64
}

// DefaultBudgets is the motion-window gate: fps>=55, p95<=22ms.
func DefaultBudgets() Budgets { return Budgets{MinFPSWall: 55, MaxP95Ms: 22} }

// CheckFPS reports whether the steady window meets the budget.
func (b Budgets) CheckFPS(fpsWall, p95Ms float64) bool {
	return fpsWall >= b.MinFPSWall && p95Ms <= b.MaxP95Ms
}

// CheckIsolation reports the repaint-boundary contract: the animated
// node is dirty while its static sibling stays clean.
func CheckIsolation(animatedDirty, siblingDirty bool) bool {
	return animatedDirty && !siblingDirty
}

// CacheBudget is a tiny LRU: full budgets evict the oldest key.
// Product image caches wrap this; F0 only needs the eviction rule.
type CacheBudget struct {
	Cap   int
	keys  []string
	items map[string]struct{}
}

// NewCacheBudget builds a budget holding at most cap entries.
func NewCacheBudget(cap int) *CacheBudget {
	if cap <= 0 {
		cap = 1
	}
	return &CacheBudget{Cap: cap, items: map[string]struct{}{}}
}

// Set inserts key, evicting the oldest when full. Returns the evicted
// key, or "" when nothing was evicted.
func (c *CacheBudget) Set(key string) string {
	if c == nil {
		return ""
	}
	if _, ok := c.items[key]; ok {
		return ""
	}
	var evicted string
	if len(c.keys) >= c.Cap {
		evicted = c.keys[0]
		c.keys = c.keys[1:]
		delete(c.items, evicted)
	}
	c.keys = append(c.keys, key)
	c.items[key] = struct{}{}
	return evicted
}

// Len reports held entries.
func (c *CacheBudget) Len() int {
	if c == nil {
		return 0
	}
	return len(c.keys)
}

// Has reports membership.
func (c *CacheBudget) Has(key string) bool {
	if c == nil {
		return false
	}
	_, ok := c.items[key]
	return ok
}

// VirtualBoundOK checks the long-list contract: bound rows serve total
// rows with bind_count far below item_count (at least 8x fewer binds).
func VirtualBoundOK(total, bound int) bool {
	if total <= 0 || bound < 0 {
		return false
	}
	if bound == 0 {
		return total > 0
	}
	return total >= bound*8
}
