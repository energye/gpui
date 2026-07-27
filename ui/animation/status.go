package animation

// Status is the lifecycle of a Controller (Flutter AnimationStatus subset).
type Status int

const (
	// StatusDismissed: not running, progress at start (0 for forward).
	StatusDismissed Status = iota
	// StatusForward: animating 0→1.
	StatusForward
	// StatusReverse: animating 1→0 (optional path).
	StatusReverse
	// StatusCompleted: finished at end (1 for forward, 0 for reverse).
	StatusCompleted
)

// String implements fmt.Stringer.
func (s Status) String() string {
	switch s {
	case StatusDismissed:
		return "dismissed"
	case StatusForward:
		return "forward"
	case StatusReverse:
		return "reverse"
	case StatusCompleted:
		return "completed"
	default:
		return "unknown"
	}
}
