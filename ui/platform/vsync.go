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
