package kit

import "github.com/energye/gpui/ui/core"

// tickerLifecycle binds a core.Ticker to a Tree while the control is mounted.
// Used by Spin/Skeleton so Off-screen Tabs content does not keep ANIMATING alive.
//
// Contract:
//   - Attach / OnMount → BindTicker when active
//   - OnUnmount → RemoveTicker
//   - Tick returns false when bound but unmounted (auto-drop from registry)
type tickerLifecycle struct {
	tree   *core.Tree
	ticker core.Ticker
}

func (l *tickerLifecycle) attach(t *core.Tree, tk core.Ticker, active bool) {
	if l == nil || t == nil || tk == nil {
		return
	}
	l.tree = t
	l.ticker = tk
	t.BindTicker(tk, active)
}

func (l *tickerLifecycle) setActive(active bool) {
	if l == nil || l.tree == nil || l.ticker == nil {
		return
	}
	if active {
		l.tree.AddTicker(l.ticker)
	} else {
		l.tree.RemoveTicker(l.ticker)
	}
}

func (l *tickerLifecycle) unmount() {
	if l == nil || l.tree == nil || l.ticker == nil {
		return
	}
	l.tree.RemoveTicker(l.ticker)
}

// stillMounted reports whether a bound ticker should keep running.
// Unbound (unit tests) → true so Tick still advances when Active.
// Bound + unmounted → false so the demand loop can idle.
func (l *tickerLifecycle) stillMounted(nodeTree *core.Tree) bool {
	if l == nil || l.tree == nil {
		return true
	}
	return nodeTree != nil
}
