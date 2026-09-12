package h264

import "fmt"

// B-slice spatial direct mode (frames only): motion comes from neighbours
// plus the colocated block of the first list-1 reference, never from the
// bitstream. Design mirrors the reference decoder's frame path
// (neighbour unsigned-minimum, single-match shortcut, stationary-colocated
// zeroing); all logic is reimplemented in pure Go.

// bDirectMV holds one list's derived direct motion.
type bDirectMV struct {
	ref    int8
	mx, my int16
	use    bool
}

// minNonNeg returns the smallest non-negative reference index, or -1 when
// every neighbour is missing or intra (unsigned-minimum semantics).
func minNonNeg(refs ...int8) int8 {
	best := int8(-1)
	for _, r := range refs {
		if r < 0 {
			continue
		}
		if best < 0 || r < best {
			best = r
		}
	}
	return best
}

// bDirectMB derives spatial-direct motion for one 16x16 MB at (mbx, mby).
// colPic is the first list-1 reference (nil only when the list is empty,
// which fails readable). Temporal direct stops readable for a later stage.
func (d *Decoder) bDirectMB(mbx, mby int, h *SliceHeader, colPic *Picture) ([2]bDirectMV, error) {
	x0, y0 := mbx*4, mby*4
	return d.bDirectAt(x0, y0, 0, 4, x0, y0, h, colPic)
}

// bDirectB8 derives spatial-direct motion for one 8x8 sub-block at 4x4
// origin (qx, qy): neighbours at the sub-block origin, colocated read at
// the sub-block's own top-left 4x4.
func (d *Decoder) bDirectB8(qx, qy int, h *SliceHeader, colPic *Picture) ([2]bDirectMV, error) {
	return d.bDirectAt(qx, qy, 0, 2, qx, qy, h, colPic)
}

// bDirectAt is the shared spatial-direct core: (x0, y0) is the 4x4
// origin for neighbour reads (n/w4 shape the diagonal rule),
// (colX4, colY4) the colocated 4x4 address in colPic.
func (d *Decoder) bDirectAt(x0, y0, n, w4, colX4, colY4 int, h *SliceHeader, colPic *Picture) ([2]bDirectMV, error) {
	var out [2]bDirectMV
	if !h.DirectSpatial {
		return out, fmt.Errorf("%w: temporal direct needs a later stage", ErrStageScope)
	}
	if colPic == nil {
		return out, fmt.Errorf("%w: direct without list-1 picture", ErrBadSliceHeader)
	}
	for list := 0; list < 2; list++ {
		ax, ay, ar := d.mvNeighbourL(list, x0-1, y0)
		bx, by, br := d.mvNeighbourL(list, x0, y0-1)
		cx, cy, cr := d.diagNeighbourL(list, n, w4, x0, y0)
		if cr == partNotAvailable {
			cx, cy, cr = d.topLeftNeighbourL(list, x0, y0)
		}
		ref := minNonNeg(ar, br, cr)
		if ref < 0 {
			out[list] = bDirectMV{ref: -1}
			continue
		}
		match := 0
		if ar == ref {
			match++
		}
		if br == ref {
			match++
		}
		if cr == ref {
			match++
		}
		var mx, my int16
		switch {
		case match > 1:
			mx, my = median3(ax, bx, cx), median3(ay, by, cy)
		case ar == ref:
			mx, my = ax, ay
		case br == ref:
			mx, my = bx, by
		default:
			mx, my = cx, cy
		}
		out[list] = bDirectMV{ref: ref, mx: mx, my: my, use: true}
	}
	if !out[0].use && !out[1].use {
		out[0] = bDirectMV{ref: 0, use: true}
		out[1] = bDirectMV{ref: 0, use: true}
	}
	// Both-zero neighbour motion needs no colocated read: the result is
	// zero motion either way.
	if out[0].mx == 0 && out[0].my == 0 && out[1].mx == 0 && out[1].my == 0 {
		return out, nil
	}
	cr0, cx0, cy0, cr1, cx1, cy1, ok := colPic.coloc(colX4, colY4)
	stationary := false
	if ok {
		if cr0 == 0 && abs16(cx0) <= 1 && abs16(cy0) <= 1 {
			stationary = true
		} else if cr0 < 0 && cr1 == 0 && abs16(cx1) <= 1 && abs16(cy1) <= 1 {
			stationary = true
		}
	}
	if stationary {
		if out[0].ref == 0 {
			out[0].mx, out[0].my = 0, 0
		}
		if out[1].ref == 0 {
			out[1].mx, out[1].my = 0, 0
		}
	}
	return out, nil
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}
