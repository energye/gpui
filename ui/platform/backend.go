package platform

import "fmt"

// Backend is a platform window backend registered via Register. Each platform
// (X11/Wayland/Win32/AppKit) provides one; the engine picks by Kind or by
// display detection.
//
// The backend is deliberately split into three concerns (see the plan):
//   - Create:  build a native window (own the event loop sources).
//   - Adopt:   bind an existing native surface/handle (embedding scenarios).
//   - Window:  the produced object owns Host (events/size/scale) plus optional
//     IME/Clipboard capabilities discovered at bind time.
type Backend interface {
	// Kind is the platform this backend serves.
	Kind() PlatformKind
	// Create opens a new native window and returns it bound to this backend.
	Create(opts Options) (*Window, error)
	// Adopt binds existing native handles (e.g. an embedding host window)
	// and returns a Window wired to them. Returns an error when the handles
	// are not recognized by this backend.
	Adopt(ns NativeSurface) (*Window, error)
}

// registry holds the registered backends, keyed by PlatformKind.
var registry = map[PlatformKind]Backend{}

// Register installs a backend for kind. Later registrations replace earlier
// ones (tests / alternative implementations may override).
func Register(kind PlatformKind, b Backend) {
	if b == nil {
		delete(registry, kind)
		return
	}
	registry[kind] = b
}

// backendFor returns the registered backend for kind.
func backendFor(kind PlatformKind) (Backend, error) {
	b, ok := registry[kind]
	if !ok || b == nil {
		return nil, fmt.Errorf("platform: no backend registered for %s", kind)
	}
	return b, nil
}

// registeredKinds returns all registered platform kinds (deterministic order
// by PlatformKind value).
func registeredKinds() []PlatformKind {
	out := make([]PlatformKind, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	// Stable order (PlatformKind is iota).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// toPlatformKind maps a display backend choice to a PlatformKind.
func toPlatformKind(d DisplayBackend) PlatformKind {
	switch d {
	case DisplayX11:
		return PlatformX11
	case DisplayWayland:
		return PlatformWayland
	case DisplayWin32:
		return PlatformWin32
	case DisplayAppKit:
		return PlatformAppKit
	default:
		return PlatformNone
	}
}
