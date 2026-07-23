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
