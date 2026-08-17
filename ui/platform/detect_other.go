//go:build !linux

package platform

// DisplayBackend is a no-op stub on non-Linux (window system is fixed per OS).
type DisplayBackend int

const (
	DisplayAuto DisplayBackend = iota
	DisplayX11
	DisplayWayland
	DisplayWin32
	DisplayAppKit
)

func (b DisplayBackend) String() string {
	switch b {
	case DisplayX11:
		return "x11"
	case DisplayWayland:
		return "wayland"
	case DisplayWin32:
		return "win32"
	case DisplayAppKit:
		return "appkit"
	default:
		return "auto"
	}
}

// DetectDisplayBackend is a stub on non-Linux.
func DetectDisplayBackend() DisplayBackend { return DisplayAuto }

// HasX11Display is false on non-Linux.
func HasX11Display() bool { return false }

// HasWaylandDisplay is false on non-Linux.
func HasWaylandDisplay() bool { return false }
