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
// which fails readable). Temporal direct derives from the top-left
// colocated block (the 16x16 frame path); per-8x8 callers use
// bDirectMBBlocks / bTempDirect8 instead.
func (d *Decoder) bDirectMB(mbx, mby int, h *SliceHeader, colPic *Picture) ([2]bDirectMV, error) {
	x0, y0 := mbx*4, mby*4
	if !h.DirectSpatial {
		return d.bTempDirect8(x0, y0, h, colPic)
	}
	dm, err := d.bDirectNeighbors(x0, y0, 0, 4, h)
	if err != nil {
		return dm, err
	}
	return d.applyDirectStationary(dm, x0, y0, colPic), nil
}

// bDirectMBBlocks derives skip motion for one 16x16 MB, per 8x8 from the
// top-left colocated 4x4 of each 8x8. Per-8x8 top-left matches the oracle
// on the 1080p/2K temporal clips (58/2/33 px); outer corners (x8*3) and
// single-MB both test worse here, so skip stays top-left per 8x8.
//
// B_Direct 16x16 explicit (single motion for the whole MB) does not use
// this; it derives once via bDirectMB below.
func (d *Decoder) bDirectMBBlocks(mbx, mby int, h *SliceHeader, colPic *Picture) ([4][2]bDirectMV, error) {
	var out [4][2]bDirectMV
	if h.DirectSpatial {
		x0, y0 := mbx*4, mby*4
		base, err := d.bDirectNeighbors(x0, y0, 0, 4, h)
		if err != nil {
			return out, err
		}
		for i := 0; i < 4; i++ {
			qx := x0 + (i%2)*2
			qy := y0 + (i/2)*2
			out[i] = d.applyDirectStationary(base, qx, qy, colPic)
		}
		return out, nil
	}
	x0, y0 := mbx*4, mby*4
	for i := 0; i < 4; i++ {
		qx := x0 + (i%2)*2
		qy := y0 + (i/2)*2
		dm, err := d.bTempDirect8(qx, qy, h, colPic)
		if err != nil {
			return out, err
		}
		out[i] = dm
	}
	return out, nil
}

// bDirectMBLevel derives the macroblock-level spatial-direct motion once
// for a B_8x8 block: every direct 8x8 sub-block shares it (plus its own
// stationary-colocated check), instead of deriving per sub-block from
// just-decoded explicit neighbours inside the same macroblock.
//
// Temporal direct has no macroblock-level derivation (each 8x8 scales
// its own colocated block); the B_8x8 caller derives per sub-block
// with bTempDirect8 instead, so this stays spatial-only.
func (d *Decoder) bDirectMBLevel(mbx, mby int, h *SliceHeader) ([2]bDirectMV, error) {
	x0, y0 := mbx*4, mby*4
	return d.bDirectNeighbors(x0, y0, 0, 4, h)
}

// bDirectB8 copies the macroblock-level direct motion onto one 8x8
// sub-block at 4x4 origin (qx, qy), applying the sub-block's own
// stationary-colocated check. colPic is the first list-1 reference.
func (d *Decoder) bDirectB8(dm [2]bDirectMV, qx, qy int, colPic *Picture) [2]bDirectMV {
	return d.applyDirectStationary(dm, qx, qy, colPic)
}

// bDirectNeighbors is the shared spatial-direct neighbour core: (x0, y0)
// is the 4x4 origin for neighbour reads (n/w4 shape the diagonal rule).
// Stationary-colocated zeroing is separate (applyDirectStationary).
func (d *Decoder) bDirectNeighbors(x0, y0, n, w4 int, h *SliceHeader) ([2]bDirectMV, error) {
	var out [2]bDirectMV
	if !h.DirectSpatial {
		return out, fmt.Errorf("%w: temporal direct needs a later stage", ErrStageScope)
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
	return out, nil
}

// applyDirectStationary applies the stationary-colocated check at
// colocated 4x4 (colX4, colY4): a near-zero colocated block zeroes both
// derived motions. Both-zero neighbour motion needs no colocated read:
// the result is zero motion either way.
func (d *Decoder) applyDirectStationary(out [2]bDirectMV, colX4, colY4 int, colPic *Picture) [2]bDirectMV {
	if out[0].mx == 0 && out[0].my == 0 && out[1].mx == 0 && out[1].my == 0 {
		return out
	}
	if colPic == nil {
		return out
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
	return out
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}

// bTempDirect8 derives temporal-direct motion for one 8x8 sub-block
// whose top-left 4x4 sits at (qx, qy): scale the colocated block's
// motion by display-order distances (spec 8.4.1.2.2, frames-only
// short-term path). Both lists are always used, list 1 pinned at ref
// 0; an intra/missing colocated block yields zero motion at ref 0.
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//
//	libavcodec/h264_direct.c:36-59 get_scale_factor (td/tb clipping,
//	tx rounding, 256 on td==0; our tempScale below) +
//	:496-736 pred_temp_direct_motion frame path (colocated L0-first
//	branch, map_col_to_list0 remap, per-8x8 top-left colocated read,
//	L1 ref 0, intra-colocated zero; our branch + map + scale below)
//	hooked via :737 ff_h264_pred_direct_motion dispatch (our
//	bDirectMBBlocks temporal leg above). Ideas only; oracle is ffmpeg
//	YUV pixels on the 1080p/2K temporal clips.
func (d *Decoder) bTempDirect8(qx, qy int, h *SliceHeader, colPic *Picture) ([2]bDirectMV, error) {
	var out [2]bDirectMV
	if d.sps == nil || d.sps.MBAFF || !d.sps.FrameMBsOnly {
		return out, fmt.Errorf("%w: temporal direct needs frame pictures", ErrStageScope)
	}
	if len(d.refList1) == 0 || d.refList1[0] == nil {
		return out, fmt.Errorf("%w: temporal direct without list-1 picture", ErrBadSliceHeader)
	}
	if len(d.refList) == 0 || d.refList[0] == nil {
		return out, fmt.Errorf("%w: temporal direct without list-0 picture", ErrBadSliceHeader)
	}
	cr0, cx0, cy0, cr1, cx1, cy1, ok := colPic.coloc(qx, qy)
	if !ok || (cr0 < 0 && cr1 < 0) {
		out[0] = bDirectMV{ref: 0, use: true}
		out[1] = bDirectMV{ref: 0, use: true}
		return out, nil
	}
	// Colocated L0-first branch: its reference (by FrameNum) remaps
	// into the current list 0; a missing map falls back to 0 like the
	// reference decoder's zero-filled colmap.
	var mvx, mvy int16
	if cr0 >= 0 {
		mvx, mvy = cx0, cy0
	} else {
		mvx, mvy = cx1, cy1
	}
	fn0, fn1 := colPic.colocFN(qx, qy)
	colFN := fn0
	if cr0 < 0 {
		colFN = fn1
	}
	ref0 := d.mapColToL0(colFN)
	p0 := d.refList[ref0]
	if p0 == nil {
		return out, fmt.Errorf("%w: temporal direct mapped nil ref", ErrBadSliceHeader)
	}
	scale := tempScale(h.POC, p0.POC, d.refList1[0].POC)
	m0x := int16((int32(scale)*int32(mvx) + 128) >> 8)
	m0y := int16((int32(scale)*int32(mvy) + 128) >> 8)
	out[0] = bDirectMV{ref: ref0, mx: m0x, my: m0y, use: true}
	out[1] = bDirectMV{ref: 0, mx: m0x - mvx, my: m0y - mvy, use: true}
	return out, nil
}

// mapColToL0 remaps a colocated reference FrameNum into the current
// list-0 index (0 when the picture already left the buffer).
func (d *Decoder) mapColToL0(colFN int32) int8 {
	if colFN < 0 {
		return 0
	}
	for i, p := range d.refList {
		if p != nil && int32(p.FrameNum) == colFN {
			return int8(i)
		}
	}
	return 0
}

// tempScale is the colocated-MV scale for temporal direct (ffmpeg
// get_scale_factor, frames path): td/tb clipped to int8 range, 256 on
// degenerate td, 10-bit clipped result. Long-term pictures stop
// readable elsewhere (this stage tracks short-term only).
func tempScale(poc, poc0, poc1 int32) int32 {
	td := poc1 - poc0
	if td < -128 {
		td = -128
	} else if td > 127 {
		td = 127
	}
	if td == 0 {
		return 256
	}
	tb := poc - poc0
	if tb < -128 {
		tb = -128
	} else if tb > 127 {
		tb = 127
	}
	tx := (16384 + (abs32(td) >> 1)) / td
	s := (tb*tx + 32) >> 6
	if s < -1024 {
		return -1024
	} else if s > 1023 {
		return 1023
	}
	return s
}
