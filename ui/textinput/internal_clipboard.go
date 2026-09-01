package textinput

import "github.com/energye/gpui/ui/platform"

// effectiveClipboard returns the box clipboard or the process-global
// fallback. Never nil. Converges to platform.FallbackClipboard as single
// source of truth (was duplicated in textinput and x11).
func effectiveClipboard(c platform.Clipboard) platform.Clipboard {
	if c != nil {
		return c
	}
	return platform.FallbackClipboard()
}
