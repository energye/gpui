package kit

// Package-level helpers for Feedback controls.

func abs01(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
