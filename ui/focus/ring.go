package focus

// FocusRingOutset is the default logical-pixel outset for a focus ring (≈1.5 design tokens).
const FocusRingOutset = 1.5

// FocusRingRect returns an outset rectangle around (x,y,w,h) for painting a focus
// ring. Does not allocate layers or saveLayer — callers stroke/fill in their paint.
func FocusRingRect(x, y, w, h, outset float64) (rx, ry, rw, rh float64) {
	if outset < 0 {
		outset = FocusRingOutset
	}
	return x - outset, y - outset, w + 2*outset, h + 2*outset
}
