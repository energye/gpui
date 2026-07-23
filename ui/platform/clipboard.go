package platform

import "sync"

// Clipboard is optional host text clipboard (CapClipboard).
// Method set matches core.Clipboard so app can bridge host → Tree.
type Clipboard interface {
	ReadText() (string, bool)
	WriteText(s string) error
}

// ClipboardProvider is implemented by hosts that own a system/memory clipboard.
type ClipboardProvider interface {
	Clipboard() Clipboard
}

// MemoryClipboard is an in-process clipboard (tests / fallback when OS tools missing).
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

// FallbackClipboard wraps a primary OS clipboard and a MemoryClipboard.
// Read/Write try primary first; on failure use memory so in-process paste still works.
type FallbackClipboard struct {
	Primary  Clipboard
	Fallback *MemoryClipboard
}

// NewFallbackClipboard pairs primary with a memory fallback (never nil fallback).
func NewFallbackClipboard(primary Clipboard) *FallbackClipboard {
	return &FallbackClipboard{Primary: primary, Fallback: NewMemoryClipboard()}
}

// ReadText implements Clipboard.
func (c *FallbackClipboard) ReadText() (string, bool) {
	if c == nil {
		return "", false
	}
	if c.Primary != nil {
		if s, ok := c.Primary.ReadText(); ok {
			return s, true
		}
	}
	if c.Fallback != nil {
		return c.Fallback.ReadText()
	}
	return "", false
}

// WriteText implements Clipboard.
func (c *FallbackClipboard) WriteText(s string) error {
	if c == nil {
		return nil
	}
	var err error
	if c.Primary != nil {
		err = c.Primary.WriteText(s)
	}
	if c.Fallback != nil {
		_ = c.Fallback.WriteText(s)
	}
	// Prefer success if either path wrote; only fail when primary exists and failed
	// and we still have fallback (always ok). Return primary error for diagnostics.
	if err != nil && c.Fallback == nil {
		return err
	}
	return nil
}
