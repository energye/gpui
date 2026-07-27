//go:build !linux && !windows && !darwin

package platform

// NewSystemClipboard returns a memory clipboard on unsupported OS targets.
func NewSystemClipboard() Clipboard {
	return NewMemoryClipboard()
}
