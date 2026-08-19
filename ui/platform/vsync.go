package platform

// DefaultAnimTickSeconds is the fallback frame period when no VSyncWaiter is available (~60 Hz).
const DefaultAnimTickSeconds = 1.0 / 60.0

// HostVSync returns h as VSyncWaiter if implemented.
func HostVSync(h Host) VSyncWaiter {
	if h == nil {
		return nil
	}
	v, _ := h.(VSyncWaiter)
	return v
}

// FrameNotifier is the compositor "frame presented" notification abstraction.
// A Host implementing it drives frame pacing from the display server's
// "a frame you committed has been shown" notice instead of the client-side
// DRM vblank fallback (ENGINE_FRAME_PRESENT_STANDARD.md 块2):
//   - Wayland: wl_surface.frame callback (compositor vblank, no client wait)
//   - X11: XPresent PresentCompleteNotify (X server display-complete notice)
// The scheduler prefers FrameNotifier over VSyncWaiter and never starts the
// DRM listener while one is present (single pacing source, no double stamp).
type FrameNotifier interface {
	// RequestFrameNotify asks for the next "frame presented" notification.
	// Call after a frame has been submitted for presentation. The notice
	// arrives as an EventFramePresented from WaitEvents. While a request is
	// already in flight the call is a no-op.
	RequestFrameNotify()
}

// HostFrameNotifier returns h as FrameNotifier if implemented.
func HostFrameNotifier(h Host) FrameNotifier {
	if h == nil {
		return nil
	}
	f, _ := h.(FrameNotifier)
	return f
}
