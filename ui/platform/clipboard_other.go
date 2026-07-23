//go:build !linux && !windows && !darwin

package platform

// NewSystemClipboard returns a memory clipboard on unsupported OS targets.
func NewSystemClipboard() Clipboard {
	return NewMemoryClipboard()
}

// NewXClipClipboard is a cross-build alias.
func NewXClipClipboard() Clipboard { return NewSystemClipboard() }
