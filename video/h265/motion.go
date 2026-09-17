package h265

import (
	"fmt"
)

// Motion derivation for P slices: reference lists, merge/AMVP vectors
// and per-picture motion grids feeding temporal candidates.
//
// Peer (read-only, ideas only, no code copied):
//   mvs.c neighbour/z-scan/MER availability, mv_scale/dist_scale,
//   spatial merge, temporal colocated, merge_mode, MVP mode +
//   refs.c frame_rps short-term order, slice_rpl default order, list
//   modification and collocated pick +
//   hevcdec.c hls_prediction_unit PU walk order and grid fill.
// Only bin-adjacent orders, gates and numbers travel; B slices and
// long-term pictures refuse honestly (this clip has neither).

// Motion prediction flags (peer PredFlag order).
const (
	motionIntra = 0
	motionL0    = 1
	motionL1    = 2
	motionBI    = 3
)

// mvCand is one motion hypothesis: prediction flag, per-list index
// into the current RPL and quarter-pel vectors.
type mvCand struct {
	pred       int
	ref0, ref1 int
	x0, y0     int32
	x1, y1     int32
}

// refPic is one reference-list entry: display POC plus long-term mark.
type refPic struct {
	poc      int
	longTerm bool
}

// mvGrid is a picture's motion field at min-PU resolution, zeroed to
// intra; decode fills inter/skip PUs in z order like tab_mvf.
type mvGrid struct {
	w, h int
	f    []mvCand
}

// picState is a decoded picture plus the motion truth later pictures
// read as temporal candidates (own L0 included, like a stored frame).
type picState struct {
	pic  *Picture
	poc  int
	rpl0 []refPic
	grid *mvGrid
}

// interDec decodes P frames against its picture store. Pictures enter
// after the loop filter: references are filtered, like the in-loop
// chain. One instance owns the sequence.
type interDec struct {
	sps *SPS
	pps *PPS
	// Geometry derived from the SPS (peer ps.c tail).
	log2CTB, log2MinCB, log2MinPU, log2MinTB int
	minPUW, minPUH, ctbSize, ctbW, tbMask    int
	pics                                     map[int]*picState
	order                                    []int
}

// newInterDec binds a sequence; geometry follows ps.c derivations
// (CTB = max CB, min PU = min CB - 1, ceil CTB grid, TB mask).
func newInterDec(sps *SPS, pps *PPS) (*interDec, error) {
	if sps == nil || pps == nil {
		return nil, fmt.Errorf("%w: nil sets", ErrBadSlice)
	}
	if sps.Log2MaxCB < sps.Log2MinCB || sps.Log2MinCB < 3 || sps.Log2MaxCB > 6 {
		return nil, fmt.Errorf("%w: cb %d-%d", ErrBadSlice, sps.Log2MinCB, sps.Log2MaxCB)
	}
	if sps.Width == 0 || sps.Height == 0 {
		return nil, fmt.Errorf("%w: bad size", ErrBadSPS)
	}
	d := &interDec{sps: sps, pps: pps, pics: map[int]*picState{}}
	d.log2CTB = int(sps.Log2MaxCB)
	d.log2MinCB = int(sps.Log2MinCB)
	d.log2MinPU = d.log2MinCB - 1
	d.log2MinTB = int(sps.Log2MinTB)
	d.ctbSize = 1 << d.log2CTB
	d.ctbW = (int(sps.Width) + d.ctbSize - 1) / d.ctbSize
	d.minPUW = int(sps.Width) >> d.log2MinPU
	d.minPUH = int(sps.Height) >> d.log2MinPU
	if d.minPUW < 1 || d.minPUH < 1 {
		return nil, fmt.Errorf("%w: pu grid", ErrBadSPS)
	}
	d.tbMask = (1 << (d.log2CTB - d.log2MinTB)) - 1
	return d, nil
}

// maxBuf caps the store (peer sliding window; SPS buffering).
func (d *interDec) maxBuf() int {
	m := int(d.sps.MaxBuffering)
	if m < 1 {
		m = 1
	}
	if m > 16 {
		m = 16
	}
	return m
}

// buildRPL assembles L0 for one P slice: used short-term deltas in RPS
// order (negatives = before, rest = after), truncated to the header
// count, then list-modification reorder. Long-term tails refuse: the
// header keeps only the count, and this clip sends none.
func (d *interDec) buildRPL(sh *SliceHeader) ([]refPic, error) {
	if sh.Type == SliceB {
		return nil, fmt.Errorf("%w: B slices", ErrNotDecodable)
	}
	if sh.LTRefs > 0 {
		return nil, fmt.Errorf("%w: long-term refs", ErrNotDecodable)
	}
	var bef, aft []refPic
	if sh.RPS != nil {
		neg := int(sh.RPS.Neg)
		for i, dp := range sh.RPS.DeltaPOC {
			if !sh.RPS.Used[i] {
				continue
			}
			rp := refPic{poc: sh.POC + int(dp)}
			if i < neg {
				bef = append(bef, rp)
			} else {
				aft = append(aft, rp)
			}
		}
	}
	tmp := append(append([]refPic{}, bef...), aft...)
	if len(tmp) == 0 {
		return nil, fmt.Errorf("%w: zero refs", ErrBadSlice)
	}
	n := int(sh.RefL0)
	if n < len(tmp) {
		tmp = tmp[:n]
	}
	if len(sh.ListsModL0) > 0 {
		if len(sh.ListsModL0) != n {
			return nil, fmt.Errorf("%w: list mod %d vs %d", ErrBadSlice, len(sh.ListsModL0), n)
		}
		out := make([]refPic, n)
		for i, e := range sh.ListsModL0 {
			if int(e) >= len(tmp) {
				return nil, fmt.Errorf("%w: list entry %d", ErrBadSlice, e)
			}
			out[i] = tmp[e]
		}
		tmp = out
	}
	// Pad short lists by cycling the order like the peer's while loop
	// (our clip always fills exactly; padding never fires here).
	for len(tmp) < n {
		tmp = append(tmp, tmp...)
	}
	tmp = tmp[:n]
	return tmp, nil
}

// resolve maps an RPL entry to its stored picture.
func (d *interDec) resolve(rp refPic) (*picState, error) {
	p, ok := d.pics[rp.poc]
	if !ok || p.pic == nil {
		return nil, fmt.Errorf("%w: missing ref POC %d", ErrBadSlice, rp.poc)
	}
	return p, nil
}

// store files a filtered picture with its motion truth, evicting the
// lowest unreferenced POC past the buffering cap.
func (d *interDec) store(pic *Picture, poc int, rpl0 []refPic, grid *mvGrid) {
	d.pics[poc] = &picState{pic: pic, poc: poc, rpl0: append([]refPic{}, rpl0...), grid: grid}
	d.order = append(d.order, poc)
	keep := map[int]bool{poc: true}
	for _, r := range rpl0 {
		keep[r.poc] = true
	}
	for len(d.pics) > d.maxBuf() {
		victim := -1
		for _, o := range d.order {
			if _, ok := d.pics[o]; !ok {
				continue
			}
			if !keep[o] {
				victim = o
				break
			}
		}
		if victim < 0 {
			break
		}
		delete(d.pics, victim)
	}
}

// gridAt reads the covering motion field; nil when outside (gated by
// the candidate flags first, like TAB_MVF_PU).
func (d *interDec) gridAt(g *mvGrid, x, y int) *mvCand {
	if x < 0 || y < 0 || x >= int(d.sps.Width) || y >= int(d.sps.Height) {
		return nil
	}
	xp, yp := x>>d.log2MinPU, y>>d.log2MinPU
	if xp >= g.w || yp >= g.h {
		return nil
	}
	return &g.f[yp*g.w+xp]
}

// fillGrid paints one PU rect into the motion field (peer tab_mvf
// fill runs right after each prediction unit, in decode order).
func (d *interDec) fillGrid(g *mvGrid, x0, y0, w, h int, c mvCand) {
	for y := y0; y < y0+h; y += 1 << d.log2MinPU {
		for x := x0; x < x0+w; x += 1 << d.log2MinPU {
			if f := d.gridAt(g, x, y); f != nil {
				*f = c
			}
		}
	}
}

// naFlags mirrors set_neighbour_available for one PU (single tile:
// tile edges are the picture edges; CTB flags come from the PU's own
// CTB, which never crosses a PU).
func (d *interDec) naFlags(x0, y0, w, h int) (left, up, upLeft, upRight, bottomLeft bool) {
	x0b := x0 & (d.ctbSize - 1)
	y0b := y0 & (d.ctbSize - 1)
	cx, cy := x0>>d.log2CTB, y0>>d.log2CTB
	ctbUp := cy > 0
	ctbLeft := cx > 0
	ctbUpLeft := ctbUp && ctbLeft
	ctbUpRight := cy > 0 && cx+1 < d.ctbW
	up = ctbUp || y0b != 0
	left = ctbLeft || x0b != 0
	if x0b != 0 || y0b != 0 {
		upLeft = left && up
	} else {
		upLeft = ctbUpLeft
	}
	sap := up
	if x0b+w == d.ctbSize {
		sap = ctbUpRight && y0b == 0
	}
	upRight = sap && x0+w < int(d.sps.Width)
	if y0+h >= int(d.sps.Height) {
		bottomLeft = false
	} else {
		bottomLeft = left
	}
	return left, up, upLeft, upRight, bottomLeft
}

// zAvail mirrors z_scan_block_avail: cross-CTB neighbours decode
// earlier except same-row right (masked compare decides there); inside
// one CTB the Morton order of min-TB units decides.
func (d *interDec) zAvail(xCurr, yCurr, xN, yN int) bool {
	xCc, yCc := xCurr>>d.log2CTB, yCurr>>d.log2CTB
	xNc, yNc := xN>>d.log2CTB, yN>>d.log2CTB
	if yNc < yCc || xNc < xCc {
		return true
	}
	xc := (xCurr >> d.log2MinTB) & d.tbMask
	yc := (yCurr >> d.log2MinTB) & d.tbMask
	xn := (xN >> d.log2MinTB) & d.tbMask
	yn := (yN >> d.log2MinTB) & d.tbMask
	return morton(xn, yn) <= morton(xc, yc)
}

// sameMER mirrors is_diff_mer (true when both luma spots share one
// merge region of the PPS parallel level).
func (d *interDec) sameMER(xN, yN, xP, yP int) bool {
	pl := int(d.pps.Log2ParallelMerge)
	return xN>>pl == xP>>pl && yN>>pl == yP>>pl
}

// avail is the AVAILABLE macro: flag set and neighbour not intra.
func (d *interDec) avail(g *mvGrid, flag bool, x, y int) *mvCand {
	if !flag {
		return nil
	}
	f := d.gridAt(g, x, y)
	if f == nil || f.pred == motionIntra {
		return nil
	}
	return f
}

// sameMV is compare_mv_ref_idx: equal flags plus equal refs/vectors
// on the active lists.
func sameMV(a, b mvCand) bool {
	if a.pred != b.pred {
		return false
	}
	switch a.pred {
	case motionBI:
		return a.ref0 == b.ref0 && a.x0 == b.x0 && a.y0 == b.y0 &&
			a.ref1 == b.ref1 && a.x1 == b.x1 && a.y1 == b.y1
	case motionL0:
		return a.ref0 == b.ref0 && a.x0 == b.x0 && a.y0 == b.y0
	case motionL1:
		return a.ref1 == b.ref1 && a.x1 == b.x1 && a.y1 == b.y1
	}
	return false
}

func clipInt8(v int) int {
	if v < -128 {
		return -128
	}
	if v > 127 {
		return 127
	}
	return v
}

func clip12(v int) int {
	if v < -2048 {
		return -2048
	}
	if v > 2047 {
		return 2047
	}
	return v
}

func clipMV(v int32) int32 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return v
}

// mvScale mirrors mv_scale: POC-distance scaling with int8-clipped
// diffs, the 0x4000 reciprocal step and int16-clipped output.
func mvScale(dx, dy int32, td, tb int) (int32, int32) {
	td, tb = clipInt8(td), clipInt8(tb)
	tx := (0x4000 + absInt(td/2)) / td
	sf := clip12((tb*tx + 32) >> 6)
	ox := int(sf) * int(dx)
	oy := int(sf) * int(dy)
	neg := 0
	if ox < 0 {
		neg = 1
	}
	rx := int32((ox + 127 + neg) >> 8)
	neg = 0
	if oy < 0 {
		neg = 1
	}
	ry := int32((oy + 127 + neg) >> 8)
	return clipMV(rx), clipMV(ry)
}

// checkMVSet mirrors check_mvset for list X against the colocated
// frame's list: long-term mismatch zeroes, equal POC gaps copy,
// else scale (P clip: everything is short-term).
func (d *interDec) checkMVSet(col *picState, colX, colY, listCol, refidxCol int, curRPL []refPic, curPOC, X, refIdxLX int) (int32, int32, bool) {
	g := col.grid
	f := d.gridAt(g, colX, colY)
	if f == nil {
		return 0, 0, false
	}
	var colMVx, colMVy int32
	if listCol == 0 {
		colMVx, colMVy = f.x0, f.y0
	} else {
		colMVx, colMVy = f.x1, f.y1
	}
	colRPL := col.rpl0
	if listCol == 1 {
		return 0, 0, false // B-only second list; P never asks
	}
	if refidxCol >= len(colRPL) || refIdxLX >= len(curRPL) {
		return 0, 0, false
	}
	colDiff := col.poc - colRPL[refidxCol].poc
	curDiff := curPOC - curRPL[refIdxLX].poc
	if colDiff == curDiff || colDiff == 0 {
		return colMVx, colMVy, true
	}
	rx, ry := mvScale(colMVx, colMVy, colDiff, curDiff)
	return rx, ry, true
}

// temporalMerge mirrors derive_temporal_colocated_mvs for X=0 (P
// slices): intra colocated blocks give nothing; L1-only blocks route
// through their L1; L0/BI route through L0. target is the current
// list-0 index the vector scales to (merge always 0, MVP the PU ref).
func (d *interDec) temporalMerge(col *picState, colX, colY int, curRPL []refPic, curPOC, target int) (int32, int32, bool) {
	f := d.gridAt(col.grid, colX, colY)
	if f == nil || f.pred == motionIntra {
		return 0, 0, false
	}
	lx := 0
	if f.pred&motionL0 == 0 {
		lx = 1
	}
	ri := f.ref0
	if lx == 1 {
		ri = f.ref1
	}
	return d.checkMVSet(col, colX, colY, lx, ri, curRPL, curPOC, 0, target)
}

// temporalCand picks the bottom-right then center colocated blocks
// (16-snapped, same-CTB-row gate), like temporal_luma_motion_vector.
func (d *interDec) temporalCand(col *picState, x0, y0, w, h int, curRPL []refPic, curPOC, target int) (int32, int32, bool) {
	if col == nil {
		return 0, 0, false
	}
	x, y := x0+w, y0+h
	if (y0>>d.log2CTB) == (y>>d.log2CTB) && y < int(d.sps.Height) && x < int(d.sps.Width) {
		x, y = x&^15, y&^15
		if mx, my, ok := d.temporalMerge(col, x, y, curRPL, curPOC, target); ok {
			return mx, my, true
		}
	}
	x, y = (x0+(w>>1))&^15, (y0+(h>>1))&^15
	return d.temporalMerge(col, x, y, curRPL, curPOC, target)
}

// spatialMerge builds the candidate list (A1/B1/B0/A0/B2 with the
// peer's dedup gates, temporal L0, zero fill). Pure reads, so the
// whole list builds and the index picks (the peer early-returns at
// the same entry).
func (d *interDec) spatialMerge(g *mvGrid, sh *SliceHeader, x0, y0, w, h int, curRPL []refPic, curPOC int, col *picState) []mvCand {
	var list []mvCand
	maxC := int(sh.MergeCand)
	nbRefs := len(curRPL)
	left, up, upLeft, upRight, bottomLeft := d.naFlags(x0, y0, w, h)
	xA1, yA1 := x0-1, y0+h-1
	xB1, yB1 := x0+w-1, y0-1
	xB0, yB0 := x0+w, y0-1
	xA0, yA0 := x0-1, y0+h
	xB2, yB2 := x0-1, y0-1
	var availA1, availB1 *mvCand
	// A1 (left): the part-1 multi-PU gate never fires here
	// (mergeMode only takes part 0); same-MER still blocks.
	if !d.sameMER(xA1, yA1, x0, y0) {
		if f := d.avail(g, left, xA1, yA1); f != nil {
			availA1 = f
			list = append(list, *f)
		}
	}
	// B1 (above).
	if !d.sameMER(xB1, yB1, x0, y0) {
		if f := d.avail(g, up, xB1, yB1); f != nil {
			if availA1 == nil || !sameMV(*f, *availA1) {
				availB1 = f
				list = append(list, *f)
			} else {
				availB1 = f
			}
		}
	}
	_ = availB1
	// B0 (above right): picture bound + z-scan + MER gates.
	if f := d.avail(g, upRight, xB0, yB0); f != nil && xB0 < int(d.sps.Width) &&
		d.zAvail(x0, y0, xB0, yB0) && !d.sameMER(xB0, yB0, x0, y0) {
		if availB1 == nil || !sameMV(*f, *availB1) {
			list = append(list, *f)
		}
	}
	// A0 (below left).
	if f := d.avail(g, bottomLeft, xA0, yA0); f != nil && yA0 < int(d.sps.Height) &&
		d.zAvail(x0, y0, xA0, yA0) && !d.sameMER(xA0, yA0, x0, y0) {
		if availA1 == nil || !sameMV(*f, *availA1) {
			list = append(list, *f)
		}
	}
	// B2 (above left): only while the list is short.
	if f := d.avail(g, upLeft, xB2, yB2); f != nil && !d.sameMER(xB2, yB2, x0, y0) && len(list) != 4 {
		dup := false
		if availA1 != nil && sameMV(*f, *availA1) {
			dup = true
		}
		if availB1 != nil && sameMV(*f, *availB1) {
			dup = true
		}
		if !dup {
			list = append(list, *f)
		}
	}
	// Temporal L0 (P slices never take the L1/B-combine branches).
	if sh.TemporalMVP && len(list) < maxC {
		if mx, my, ok := d.temporalCand(col, x0, y0, w, h, curRPL, curPOC, 0); ok {
			list = append(list, mvCand{pred: motionL0, x0: mx, y0: my})
		}
	}
	// Zero fill to the max count (P: L0 only, ref cycles nb_refs).
	zero := 0
	for len(list) < maxC {
		ri := 0
		if zero < nbRefs {
			ri = zero
		}
		list = append(list, mvCand{pred: motionL0, ref0: ri})
		zero++
	}
	return list
}

// split2N1 is gone: mergeMode only takes part 0, so multi-PU part
// gates never fire (kept as an honest refusal, not silent support).

// mergeMode mirrors ff_hevc_luma_mv_merge_mode (single-MCL 8x8
// redirect included): derive the list, pick the index, demote stray
// BI on 12-pel blocks.
func (d *interDec) mergeMode(g *mvGrid, sh *SliceHeader, x0, y0, w, h, log2cb, partIdx, mergeIdx int, curRPL []refPic, curPOC int, col *picState) (mvCand, error) {
	if partIdx != 0 {
		return mvCand{}, fmt.Errorf("%w: merge part %d", ErrNotDecodable, partIdx)
	}
	if int(d.pps.Log2ParallelMerge) > 2 && (1<<log2cb) == 8 {
		// Single-MCL redirect: part 0 starts at the CU origin, so
		// only the size widens (candidate spots move with it).
		w, h = 1<<log2cb, 1<<log2cb
	}
	list := d.spatialMerge(g, sh, x0, y0, w, h, curRPL, curPOC, col)
	if mergeIdx >= len(list) {
		return mvCand{}, fmt.Errorf("%w: merge idx %d", ErrBadSlice, mergeIdx)
	}
	c := list[mergeIdx]
	if c.pred == motionBI && w+h == 12 {
		c.pred = motionL0
	}
	return c, nil
}

// distScale mirrors dist_scale: same-picture ref POC compare, scale
// on mismatch (long-term pairs copy; this clip is all short-term).
func (d *interDec) distScale(g *mvGrid, x, y, elist, refCurr, ref int, curRPL []refPic, curPOC int) (int32, int32, bool) {
	f := d.gridAt(g, x, y)
	if f == nil {
		return 0, 0, false
	}
	var mvx, mvy int32
	var er int
	if elist == 0 {
		mvx, mvy, er = f.x0, f.y0, f.ref0
	} else {
		mvx, mvy, er = f.x1, f.y1, f.ref1
	}
	if er >= len(curRPL) || ref >= len(curRPL) {
		return 0, 0, false
	}
	picE := curRPL[er].poc
	picC := curRPL[ref].poc
	if picE != picC {
		diff := curPOC - picE
		if diff == 0 {
			diff = 1
		}
		mvx, mvy = mvScale(mvx, mvy, diff, curPOC-picC)
	}
	return mvx, mvy, true
}

// puMotion is one PU's derived motion for the MC stage.
type puMotion struct {
	x0, y0, w, h int
	mv           mvCand
}

// deriveFrame walks CUs in decode order deriving every PU's motion
// (peer hls_prediction_unit call order per split), fills the grid and
// stamps final vectors back into the PUs. Multi-PU merge past part 0
// refuses: this clip is all single-PU and the part-1 MER gates are
// unproven on wire.
func (d *interDec) deriveFrame(fs *FrameSyntax, sh *SliceHeader, curRPL []refPic, col *picState, grid *mvGrid) ([]puMotion, error) {
	var out []puMotion
	for ci := range fs.CUs {
		cu := &fs.CUs[ci]
		cb := 1 << cu.Log2Size
		switch cu.Pred {
		case PredSkip:
			if len(cu.PUs) != 1 {
				return nil, fmt.Errorf("%w: skip PUs %d", ErrBadSlice, len(cu.PUs))
			}
			pu := &cu.PUs[0]
			c, err := d.mergeMode(grid, sh, cu.X0, cu.Y0, cb, cb, int(cu.Log2Size), 0, int(pu.MergeIdx), curRPL, sh.POC, col)
			if err != nil {
				return nil, err
			}
			if c.pred != motionL0 {
				return nil, fmt.Errorf("%w: skip pred %d", ErrNotDecodable, c.pred)
			}
			d.fillGrid(grid, cu.X0, cu.Y0, cb, cb, c)
			pu.RefIdx, pu.MVX, pu.MVY = uint8(c.ref0), c.x0, c.y0
			out = append(out, puMotion{x0: cu.X0, y0: cu.Y0, w: cb, h: cb, mv: c})
		case PredInter:
			rects := interPURects(cu.X0, cu.Y0, cb, cu.Part)
			if len(rects) != len(cu.PUs) {
				return nil, fmt.Errorf("%w: PUs %d vs %d rects", ErrBadSlice, len(cu.PUs), len(rects))
			}
			for i, r := range rects {
				pu := &cu.PUs[i]
				var c mvCand
				if pu.Merge {
					m, err := d.mergeMode(grid, sh, r[0], r[1], r[2], r[3], int(cu.Log2Size), i, int(pu.MergeIdx), curRPL, sh.POC, col)
					if err != nil {
						return nil, err
					}
					c = m
				} else {
					px, py, err := d.mvpMode(grid, sh, r[0], r[1], r[2], r[3], int(pu.RefIdx), int(pu.MVPFlag), curRPL, sh.POC, col)
					if err != nil {
						return nil, err
					}
					c = mvCand{pred: motionL0, ref0: int(pu.RefIdx), x0: clipMV(px + pu.MVDX), y0: clipMV(py + pu.MVDY)}
				}
				if c.pred != motionL0 {
					return nil, fmt.Errorf("%w: inter pred %d", ErrNotDecodable, c.pred)
				}
				d.fillGrid(grid, r[0], r[1], r[2], r[3], c)
				pu.RefIdx, pu.MVX, pu.MVY = uint8(c.ref0), c.x0, c.y0
				out = append(out, puMotion{x0: r[0], y0: r[1], w: r[2], h: r[3], mv: c})
			}
		}
	}
	return out, nil
}

// mvpMode mirrors ff_hevc_luma_mv_mvp_mode for LX=0 (P slices): left
// group (same-pic then scaled), above group, temporal on flag match.
// Only the predictor returns; the caller adds the parsed MVD.
func (d *interDec) mvpMode(g *mvGrid, sh *SliceHeader, x0, y0, w, h, refIdx int, mvpFlag int, curRPL []refPic, curPOC int, col *picState) (int32, int32, error) {
	left, up, upLeft, upRight, bottomLeft := d.naFlags(x0, y0, w, h)
	xA0, yA0 := x0-1, y0+h
	xA1, yA1 := x0-1, y0+h-1
	xB0, yB0 := x0+w, y0-1
	xB1, yB1 := x0+w-1, y0-1
	xB2, yB2 := x0-1, y0-1
	// Same-picture pulls (mx): the neighbour's list-lx ref POC must
	// equal the target ref POC (either neighbour list may match).
	mxSame := func(x, y, lx int) (int32, int32, bool) {
		f := d.gridAt(g, x, y)
		if f == nil || f.pred&(1<<lx) == 0 {
			return 0, 0, false
		}
		var er int
		var mvx, mvy int32
		if lx == 0 {
			er, mvx, mvy = f.ref0, f.x0, f.y0
		} else {
			er, mvx, mvy = f.ref1, f.x1, f.y1
		}
		if er >= len(curRPL) || refIdx >= len(curRPL) {
			return 0, 0, false
		}
		if curRPL[er].poc != curRPL[refIdx].poc {
			return 0, 0, false
		}
		return mvx, mvy, true
	}
	mxScaled := func(x, y, lx int) (int32, int32, bool) {
		f := d.gridAt(g, x, y)
		if f == nil || f.pred&(1<<lx) == 0 {
			return 0, 0, false
		}
		return d.distScale(g, x, y, lx, 0, refIdx, curRPL, curPOC)
	}
	var mxA [2]int32
	var mxB [2]int32
	aOK := true
	avA0 := bottomLeft && yA0 < int(d.sps.Height) && d.zAvail(x0, y0, xA0, yA0) && d.gridAt(g, xA0, yA0) != nil && d.gridAt(g, xA0, yA0).pred != motionIntra
	avA1 := d.avail(g, left, xA1, yA1) != nil
	scaledL0 := avA0 || avA1
	// Left group: same-picture per spot (A0 L0/L1, A1 L0/L1),
	// then scaled per spot — the peer's exact try order.
	if avA0 {
		if x, y, ok := mxSame(xA0, yA0, 0); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
		if x, y, ok := mxSame(xA0, yA0, 1); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
	}
	if avA1 {
		if x, y, ok := mxSame(xA1, yA1, 0); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
		if x, y, ok := mxSame(xA1, yA1, 1); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
	}
	if avA0 {
		if x, y, ok := mxScaled(xA0, yA0, 0); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
		if x, y, ok := mxScaled(xA0, yA0, 1); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
	}
	if avA1 {
		if x, y, ok := mxScaled(xA1, yA1, 0); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
		if x, y, ok := mxScaled(xA1, yA1, 1); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
	}
	aOK = false
bCand:
	avB0 := upRight && xB0 < int(d.sps.Width) && d.zAvail(x0, y0, xB0, yB0) && d.gridAt(g, xB0, yB0) != nil && d.gridAt(g, xB0, yB0).pred != motionIntra
	avB1 := d.avail(g, up, xB1, yB1) != nil
	avB2 := d.avail(g, upLeft, xB2, yB2) != nil
	bOK := true
	if avB0 {
		if x, y, ok := mxSame(xB0, yB0, 0); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
		if x, y, ok := mxSame(xB0, yB0, 1); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
	}
	if avB1 {
		if x, y, ok := mxSame(xB1, yB1, 0); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
		if x, y, ok := mxSame(xB1, yB1, 1); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
	}
	if avB2 {
		if x, y, ok := mxSame(xB2, yB2, 0); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
		if x, y, ok := mxSame(xB2, yB2, 1); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
	}
	bOK = false
scaleF:
	if !scaledL0 {
		if bOK {
			aOK = true
			mxA = mxB
		}
		bOK = false
		if avB0 {
			if x, y, ok := mxScaled(xB0, yB0, 0); ok {
				mxB[0], mxB[1], bOK = x, y, true
			} else if x, y, ok := mxScaled(xB0, yB0, 1); ok {
				mxB[0], mxB[1], bOK = x, y, true
			}
		}
		if !bOK && avB1 {
			if x, y, ok := mxScaled(xB1, yB1, 0); ok {
				mxB[0], mxB[1], bOK = x, y, true
			} else if x, y, ok := mxScaled(xB1, yB1, 1); ok {
				mxB[0], mxB[1], bOK = x, y, true
			}
		}
		if !bOK && avB2 {
			if x, y, ok := mxScaled(xB2, yB2, 0); ok {
				mxB[0], mxB[1], bOK = x, y, true
			} else if x, y, ok := mxScaled(xB2, yB2, 1); ok {
				mxB[0], mxB[1], bOK = x, y, true
			}
		}
	}
	var list [][2]int32
	if aOK {
		list = append(list, mxA)
	}
	if bOK && (!aOK || mxA != mxB) {
		list = append(list, mxB)
	}
	// Temporal candidate scales to the PU's own ref (checkMVSet
	// inside targets refIdx, like the peer's ref_idx argument).
	if len(list) < 2 && sh.TemporalMVP && mvpFlag == len(list) {
		if mx, my, ok := d.temporalCand(col, x0, y0, w, h, curRPL, curPOC, refIdx); ok {
			list = append(list, [2]int32{mx, my})
		}
	}
	if mvpFlag >= len(list) {
		return 0, 0, fmt.Errorf("%w: mvp %d of %d", ErrBadSlice, mvpFlag, len(list))
	}
	return list[mvpFlag][0], list[mvpFlag][1], nil
}
