package platform

import (
	"fmt"
	"sync"
)

type fallbackClipboard struct {
	mu   sync.RWMutex
	data map[string]string
}

var fallbackClip = &fallbackClipboard{data: make(map[string]string)}

// FallbackClipboard returns the process-global in-memory clipboard.
// Used when the native selection is unavailable (nil Window.Clipboard)
// or as a fast self-read mirror for X11/Wayland ownData.
func FallbackClipboard() Clipboard { return fallbackClip }

func (f *fallbackClipboard) Get(kind string) (string, error) {
	if kind == "" {
		kind = "text/plain"
	}
	f.mu.RLock()
	v, ok := f.data[kind]
	f.mu.RUnlock()
	if !ok || v == "" {
		return "", fmt.Errorf("clipboard is empty")
	}
	return v, nil
}

func (f *fallbackClipboard) Set(kind, data string) error {
	if kind == "" {
		kind = "text/plain"
	}
	f.mu.Lock()
	if f.data == nil {
		f.data = make(map[string]string)
	}
	f.data[kind] = data
	f.mu.Unlock()
	return nil
}

// ClearFallbackForTest clears the fallback (test only).
func ClearFallbackForTest() {
	fallbackClip.mu.Lock()
	fallbackClip.data = make(map[string]string)
	fallbackClip.mu.Unlock()
}

// SetFallbackForTest sets fallback directly (test only).
func SetFallbackForTest(kind, data string) {
	_ = fallbackClip.Set(kind, data)
}
