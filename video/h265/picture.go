package h265

// Picture is one decoded 8-bit 4:2:0 frame: luma plus quarter-size
// chroma planes. Stride equals width (no padding); the bridge crops
// or scales on display. V2-2 step 2 owns allocation; the pixel stage
// fills the planes.
type Picture struct {
	Width  int
	Height int
	Y      []byte
	Cb     []byte
	Cr     []byte
}

// NewPicture allocates a blank picture (all planes zeroed).
func NewPicture(w, h int) *Picture {
	if w <= 0 || h <= 0 || w > 16888 || h > 16888 {
		return nil
	}
	cw, ch := (w+1)/2, (h+1)/2
	return &Picture{
		Width:  w,
		Height: h,
		Y:      make([]byte, w*h),
		Cb:     make([]byte, cw*ch),
		Cr:     make([]byte, cw*ch),
	}
}

// AtY reads one luma sample (no bounds check; hot path).
func (p *Picture) AtY(x, y int) byte { return p.Y[y*p.Width+x] }

// DPB is the decoded picture buffer: reference storage sized by the
// SPS (MaxBuffering, 16 max). Pictures stay until the sliding window
// evicts them; the pixel stage owns eviction in step 3.
type DPB struct {
	Max   int
	Pics  []*Picture
	Order []int // POCs in decode order, for the step-3 reorder gate
}

// NewDPB builds an empty buffer (max clamped to 1..16).
func NewDPB(max uint32) *DPB {
	m := int(max)
	if m < 1 {
		m = 1
	}
	if m > 16 {
		m = 16
	}
	return &DPB{Max: m}
}

// Len reports stored pictures.
func (d *DPB) Len() int { return len(d.Pics) }
