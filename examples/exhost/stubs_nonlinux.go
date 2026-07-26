//go:build !linux

package exhost

import "fmt"

func openX11(w, h int, title string) (*Window, error) {
	return nil, fmt.Errorf("exhost: X11 only on linux")
}

func openWayland(w, h int, title string) (*Window, error) {
	return nil, fmt.Errorf("exhost: Wayland only on linux")
}
