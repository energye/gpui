package core

import "sync"

// Clipboard is the C-Clipbd service (copy/paste text). Hosts or tests inject an
// implementation via Tree.SetClipboard.
type Clipboard interface {
	ReadText() (string, bool)
	WriteText(s string) error
}

// MemoryClipboard is an in-process clipboard (Headless / unit tests).
type MemoryClipboard struct {
	mu   sync.Mutex
	text string
}

// NewMemoryClipboard creates an empty memory clipboard.
func NewMemoryClipboard() *MemoryClipboard { return &MemoryClipboard{} }

// ReadText implements Clipboard.
func (c *MemoryClipboard) ReadText() (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, true
}

// WriteText implements Clipboard.
func (c *MemoryClipboard) WriteText(s string) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	c.text = s
	c.mu.Unlock()
	return nil
}

// SetClipboard installs the tree-level clipboard service (nil clears).
func (t *Tree) SetClipboard(c Clipboard) {
	if t == nil {
		return
	}
	t.clipboard = c
}

// Clipboard returns the tree-level clipboard (may be nil).
func (t *Tree) Clipboard() Clipboard {
	if t == nil {
		return nil
	}
	return t.clipboard
}

// SetTheme installs the tree-default Theme (F8). Nodes under a ThemeProvider win.
// Broadcasts ThemeObserver.OnThemeChanged and marks the tree for full relayout/paint
// so chrome rebuilt from tokens can refresh (theme change contract).
func (t *Tree) SetTheme(th *Theme) {
	if t == nil {
		return
	}
	t.theme = th
	t.themeEpoch++
	t.broadcastThemeChanged(th)
	if t.root != nil {
		markSubtreeNeedsLayout(t.root)
	}
	for _, e := range t.Overlays().Entries() {
		if e.Node != nil {
			markSubtreeNeedsLayout(e.Node)
		}
	}
	t.MarkFullPaintRequired()
	t.markDirty()
}

// ThemeEpoch returns a monotonic counter incremented on each SetTheme (tests / cache keys).
func (t *Tree) ThemeEpoch() uint64 {
	if t == nil {
		return 0
	}
	return t.themeEpoch
}

// ThemeObserver is optional: controls that bake token colors into children should
// re-apply chrome when the ambient/tree theme changes.
type ThemeObserver interface {
	OnThemeChanged(th *Theme)
}

func (t *Tree) broadcastThemeChanged(th *Theme) {
	if t == nil {
		return
	}
	if t.root != nil {
		walkThemeObservers(t.root, th)
	}
	for _, e := range t.Overlays().Entries() {
		if e.Node != nil {
			walkThemeObservers(e.Node, th)
		}
	}
}

func walkThemeObservers(n Node, th *Theme) {
	if n == nil {
		return
	}
	if b := n.Base(); b != nil && b.themeHook != nil {
		b.themeHook(th)
	}
	if o, ok := n.(ThemeObserver); ok {
		o.OnThemeChanged(th)
	}
	for _, c := range n.Children() {
		walkThemeObservers(c, th)
	}
}

// NotifyThemeChanged walks n and descendants invoking ThemeObserver / theme hooks.
// Used by ConfigProvider.SetTheme and Tree.SetTheme.
func NotifyThemeChanged(n Node, th *Theme) {
	walkThemeObservers(n, th)
}

func markSubtreeNeedsLayout(n Node) {
	if n == nil {
		return
	}
	n.Base().MarkNeedsLayout()
	n.Base().MarkNeedsPaint()
	for _, c := range n.Children() {
		markSubtreeNeedsLayout(c)
	}
}

// Theme returns the tree-default Theme (may be nil).
func (t *Tree) Theme() *Theme {
	if t == nil {
		return nil
	}
	return t.theme
}

// SetOnIMEPosition registers a host callback for IME candidate placement (logical
// client pixels). Invoked after focus/caret-affecting input when CapIME is used.
func (t *Tree) SetOnIMEPosition(fn func(x, y float64)) {
	if t == nil {
		return
	}
	t.onIMEPos = fn
}

// updateIMEPosition notifies the host of the focused editor caret, if any.
func (t *Tree) updateIMEPosition() {
	if t == nil || t.onIMEPos == nil || t.focus == nil {
		return
	}
	type caretPos interface {
		CaretLocalPos() (x, y float64)
	}
	cp, ok := t.focus.(caretPos)
	if !ok {
		return
	}
	lx, ly := cp.CaretLocalPos()
	abs := AbsoluteOffset(t.focus)
	t.onIMEPos(abs.X+lx, abs.Y+ly)
}
