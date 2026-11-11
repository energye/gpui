//go:build !(js && wasm) && !nogpu

package render

import "testing"

// TestPresentPreferNonBlocking pins the I5 v2 scope: the non-blocking present
// order is native Wayland only. X11/Windows/macOS keep FIFO (their present
// path does not wait on Mesa's Wayland WSI release), and go/gles Wayland is
// C7 (EGL present blocking), not this item.
func TestPresentPreferNonBlocking(t *testing.T) {
	cases := []struct {
		name string
		p    PresentPlatform
		want Backend
		got  bool
	}{
		{"wayland-native", PresentPlatformWayland, BackendNative, true},
		{"wayland-go", PresentPlatformWayland, BackendGo, false},
		{"x11-native", PresentPlatformX11, BackendNative, false},
		{"win32-native", PresentPlatformWin32, BackendNative, false},
	}
	for _, c := range cases {
		if got := presentPreferNonBlocking(c.p, c.want); got != c.got {
			t.Errorf("%s: got %v want %v", c.name, got, c.got)
		}
	}
}
