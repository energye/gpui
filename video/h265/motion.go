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
// read as temporal candidates (both lists, like a stored frame).
type picState struct {
	pic  *Picture
	poc  int
	rpl0 []refPic
	rpl1 []refPic
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

// buildRPL assembles L0 for one P/B slice: used short-term deltas in RPS
// order (negatives = before, rest = after), truncated to the header
// count, then list-modification reorder. Long-term tails refuse: the
// header keeps only the count, and this clip sends none.
func (d *interDec) buildRPL(sh *SliceHeader) ([]refPic, error) {
	if sh.Type != SliceP && sh.Type != SliceB {
		return nil, fmt.Errorf("%w: bad slice for L0", ErrBadSlice)
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
	tmp = padRPL(tmp, n)
	return tmp, nil
}

// padRPL cycles the default order out to n entries like the peer's
// while loop (our clips always fill exactly; padding never fires).
func padRPL(tmp []refPic, n int) []refPic {
	for len(tmp) < n {
		tmp = append(tmp, tmp...)
	}
	if len(tmp) > n {
		tmp = tmp[:n]
	}
	return tmp
}

// buildRPL1 assembles L1 for one B slice: used short-term deltas in
// reverse RPS order (after = future first, then before), truncated to
// the header count, then L1 list-modification reorder. P slices carry
// no L1 and report nil without error so the shared B harness can call
// both builders on every inter frame.
func (d *interDec) buildRPL1(sh *SliceHeader) ([]refPic, error) {
	if sh.Type != SliceB {
		return nil, nil
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
	tmp := append(append([]refPic{}, aft...), bef...)
	if len(tmp) == 0 {
		return nil, fmt.Errorf("%w: zero L1 refs", ErrBadSlice)
	}
	n := int(sh.RefL1)
	if n < len(tmp) {
		tmp = tmp[:n]
	}
	if len(sh.ListsModL1) > 0 {
		if len(sh.ListsModL1) != n {
			return nil, fmt.Errorf("%w: list mod L1 %d vs %d", ErrBadSlice, len(sh.ListsModL1), n)
		}
		out := make([]refPic, n)
		for i, e := range sh.ListsModL1 {
			if int(e) >= len(tmp) {
				return nil, fmt.Errorf("%w: list L1 entry %d", ErrBadSlice, e)
			}
			out[i] = tmp[e]
		}
		tmp = out
	}
	tmp = padRPL(tmp, n)
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
func (d *interDec) store(pic *Picture, poc int, rpl0, rpl1 []refPic, grid *mvGrid) {
	d.pics[poc] = &picState{pic: pic, poc: poc, rpl0: append([]refPic{}, rpl0...), rpl1: append([]refPic{}, rpl1...), grid: grid}
	d.order = append(d.order, poc)
	keep := map[int]bool{poc: true}
	for _, r := range rpl0 {
		keep[r.poc] = true
	}
	for _, r := range rpl1 {
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
// frame's list: long-term mismatch gives nothing, equal POC gaps copy,
// else scale (our clips are all short-term).
func (d *interDec) checkMVSet(colMVx, colMVy int32, colPOC int, colRPL []refPic, refidxCol int, curRPL []refPic, curPOC, refIdxLX int) (int32, int32, bool) {
	if refidxCol >= len(colRPL) || refIdxLX >= len(curRPL) {
		return 0, 0, false
	}
	if colRPL[refidxCol].longTerm != curRPL[refIdxLX].longTerm {
		return 0, 0, false
	}
	colDiff := colPOC - colRPL[refidxCol].poc
	curDiff := curPOC - curRPL[refIdxLX].poc
	if colDiff == curDiff || colDiff == 0 {
		return colMVx, colMVy, true
	}
	rx, ry := mvScale(colMVx, colMVy, colDiff, curDiff)
	return rx, ry, true
}

// hasFutureRef mirrors the check_diffpicount gate: any current-list
// reference past the current picture.
func hasFutureRef(rpl0, rpl1 []refPic, curPOC int) bool {
	for _, r := range rpl0 {
		if r.poc > curPOC {
			return true
		}
	}
	for _, r := range rpl1 {
		if r.poc > curPOC {
			return true
		}
	}
	return false
}

// temporalMerge mirrors derive_temporal_colocated_mvs for target list
// X: intra colocated blocks give nothing; L1-only blocks route
// through their L1; L0-only route through L0; BI blocks follow the
// future-reference rule against the colocated list flag.
func (d *interDec) temporalMerge(col *picState, colX, colY int, curRPL []refPic, curPOC, X, refIdxLX int, rpl0, rpl1 []refPic, colocL1 bool) (int32, int32, bool) {
	f := d.gridAt(col.grid, colX, colY)
	if f == nil || f.pred == motionIntra {
		return 0, 0, false
	}
	if f.pred&motionL0 == 0 {
		return d.checkMVSet(f.x1, f.y1, col.poc, col.rpl1, f.ref1, curRPL, curPOC, refIdxLX)
	}
	if f.pred == motionL0 {
		return d.checkMVSet(f.x0, f.y0, col.poc, col.rpl0, f.ref0, curRPL, curPOC, refIdxLX)
	}
	lx := 0
	if hasFutureRef(rpl0, rpl1, curPOC) {
		if !colocL1 {
			lx = 1
		}
	} else if X != 0 {
		lx = 1
	}
	if lx == 0 {
		return d.checkMVSet(f.x0, f.y0, col.poc, col.rpl0, f.ref0, curRPL, curPOC, refIdxLX)
	}
	return d.checkMVSet(f.x1, f.y1, col.poc, col.rpl1, f.ref1, curRPL, curPOC, refIdxLX)
}

// temporalCand picks the bottom-right then center colocated blocks
// (16-snapped, same-CTB-row gate), like temporal_luma_motion_vector.
func (d *interDec) temporalCand(col *picState, x0, y0, w, h int, curRPL []refPic, curPOC, X, refIdxLX int, rpl0, rpl1 []refPic, colocL1 bool) (int32, int32, bool) {
	if col == nil {
		return 0, 0, false
	}
	x, y := x0+w, y0+h
	if (y0>>d.log2CTB) == (y>>d.log2CTB) && y < int(d.sps.Height) && x < int(d.sps.Width) {
		x, y = x&^15, y&^15
		if mx, my, ok := d.temporalMerge(col, x, y, curRPL, curPOC, X, refIdxLX, rpl0, rpl1, colocL1); ok {
			return mx, my, true
		}
	}
	x, y = (x0+(w>>1))&^15, (y0+(h>>1))&^15
	return d.temporalMerge(col, x, y, curRPL, curPOC, X, refIdxLX, rpl0, rpl1, colocL1)
}

// combPairs mirrors l0_l1_cand_idx: combined BI candidate order.
var combPairs = [12][2]int{
	{0, 1}, {1, 0}, {0, 2}, {2, 0}, {1, 2}, {2, 1},
	{0, 3}, {3, 0}, {1, 3}, {3, 1}, {2, 3}, {3, 2},
}

// spatialMerge builds the candidate list (A1/B1/B0/A0/B2 with the
// peer's dedup gates, temporal, B-combine, zero fill). Pure reads, so
// the whole list builds and the index picks (the peer early-returns at
// the same entry).
func (d *interDec) spatialMerge(g *mvGrid, sh *SliceHeader, x0, y0, w, h int, rpl0, rpl1 []refPic, curPOC int, col *picState) []mvCand {
	var list []mvCand
	maxC := int(sh.MergeCand)
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
	// Temporal candidates (B takes both lists, refs pinned to 0).
	colocL1 := !sh.ColocFromL0
	if sh.TemporalMVP && len(list) < maxC {
		mx0, my0, ok0 := d.temporalCand(col, x0, y0, w, h, rpl0, curPOC, 0, 0, rpl0, rpl1, colocL1)
		var tx1, ty1 int32
		ok1 := false
		if sh.Type == SliceB {
			tx1, ty1, ok1 = d.temporalCand(col, x0, y0, w, h, rpl1, curPOC, 1, 0, rpl0, rpl1, colocL1)
		}
		if ok0 || ok1 {
			c := mvCand{pred: 0, ref0: 0, ref1: 0}
			if ok0 {
				c.pred |= motionL0
				c.x0, c.y0 = mx0, my0
			}
			if ok1 {
				c.pred |= motionL1
				c.x1, c.y1 = tx1, ty1
			}
			list = append(list, c)
		}
	}
	nbOrig := len(list)
	// Combined BI candidates (B slices with room).
	if sh.Type == SliceB && nbOrig > 1 && len(list) < maxC {
		for ci := 0; len(list) < maxC && ci < nbOrig*(nbOrig-1) && ci < len(combPairs); ci++ {
			ai, bi := combPairs[ci][0], combPairs[ci][1]
			if ai >= len(list) || bi >= len(list) {
				continue
			}
			a, b := list[ai], list[bi]
			if a.pred&motionL0 == 0 || b.pred&motionL1 == 0 {
				continue
			}
			sameRef := pocOf(rpl0, a.ref0) == pocOf(rpl1, b.ref1)
			sameMV := a.x0 == b.x1 && a.y0 == b.y1
			if sameRef && sameMV {
				continue
			}
			list = append(list, mvCand{pred: motionBI, ref0: a.ref0, x0: a.x0, y0: a.y0, ref1: b.ref1, x1: b.x1, y1: b.y1})
		}
	}
	// Zero fill (B pins BI with the shorter list cycling).
	zeroPred, nbRefs := motionL0, len(rpl0)
	if sh.Type == SliceB {
		zeroPred = motionBI
		nbRefs = len(rpl0)
		if len(rpl1) < nbRefs {
			nbRefs = len(rpl1)
		}
	}
	zero := 0
	for len(list) < maxC {
		ri := 0
		if zero < nbRefs {
			ri = zero
		}
		list = append(list, mvCand{pred: zeroPred, ref0: ri, ref1: ri})
		zero++
	}
	return list
}

// split2N1 is gone: mergeMode only takes part 0, so multi-PU part
// gates never fire (kept as an honest refusal, not silent support).

// mergeMode mirrors ff_hevc_luma_mv_merge_mode (single-MCL 8x8
// redirect included): derive the list, pick the index, demote stray
// BI on 12-pel blocks.
func (d *interDec) mergeMode(g *mvGrid, sh *SliceHeader, x0, y0, w, h, log2cb, partIdx, mergeIdx int, rpl0, rpl1 []refPic, curPOC int, col *picState) (mvCand, error) {
	if partIdx != 0 {
		return mvCand{}, fmt.Errorf("%w: merge part %d", ErrNotDecodable, partIdx)
	}
	if int(d.pps.Log2ParallelMerge) > 2 && (1<<log2cb) == 8 {
		// Single-MCL redirect: part 0 starts at the CU origin, so
		// only the size widens (candidate spots move with it).
		w, h = 1<<log2cb, 1<<log2cb
	}
	list := d.spatialMerge(g, sh, x0, y0, w, h, rpl0, rpl1, curPOC, col)
	if mergeIdx >= len(list) {
		return mvCand{}, fmt.Errorf("%w: merge idx %d", ErrBadSlice, mergeIdx)
	}
	c := list[mergeIdx]
	if c.pred == motionBI && w+h == 12 {
		c.pred = motionL0
	}
	return c, nil
}

// sameListPOC mirrors mv_mp_mode_mx: the neighbour's own-list POC
// for its stored index must equal the current list's POC for the
// target index. Neighbour lists ride the current frame's RPLs (the
// grid only covers the frame being derived).
func sameListPOC(g *mvGrid, x, y, lx, X, er, refIdx int, curRPL, rpl0, rpl1 []refPic, d *interDec) bool {
	_, _, _, _, _, _, _ = g, x, y, lx, X, er, d
	var nbRPL []refPic
	if lx == 0 {
		nbRPL = rpl0
	} else {
		nbRPL = rpl1
	}
	if er >= len(nbRPL) || refIdx >= len(curRPL) {
		return false
	}
	return nbRPL[er].poc == curRPL[refIdx].poc
}

// distScale mirrors dist_scale: neighbour-list POC against the
// current-list POC, scale on mismatch (all short-term here).
// elist selects the neighbour list (rpl0/rpl1), X is unused beyond
// curRPL (kept for the peer's shape), ref indexes curRPL.
func (d *interDec) distScale(g *mvGrid, x, y, elist, refCurr, ref int, curRPL []refPic, curPOC int, rpl0, rpl1 []refPic) (int32, int32, bool) {
	_, _, _ = refCurr, rpl0, rpl1
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
	var nbRPL []refPic
	if elist == 0 {
		nbRPL = rpl0
	} else {
		nbRPL = rpl1
	}
	if er >= len(nbRPL) || ref >= len(curRPL) {
		return 0, 0, false
	}
	picE := nbRPL[er].poc
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
// stamps final vectors back into the PUs. Intra CUs paint intra and
// carry no PUs; their syntax-time modes stay for the pixel stage.
func (d *interDec) deriveFrame(fs *FrameSyntax, sh *SliceHeader, rpl0, rpl1 []refPic, col *picState, col1 *picState, grid *mvGrid) ([]puMotion, error) {
	var out []puMotion
	if sh.Type == SliceB && len(rpl1) == 0 {
		return nil, fmt.Errorf("%w: B with no L1", ErrBadSlice)
	}
	for ci := range fs.CUs {
		cu := &fs.CUs[ci]
		cb := 1 << cu.Log2Size
		switch cu.Pred {
		case PredIntra:
			if len(cu.PUs) > 0 {
				return nil, fmt.Errorf("%w: intra PUs in inter derive", ErrBadSlice)
			}
			d.fillGrid(grid, cu.X0, cu.Y0, cb, cb, mvCand{pred: motionIntra})
		case PredSkip:
			if len(cu.PUs) != 1 {
				return nil, fmt.Errorf("%w: skip PUs %d", ErrBadSlice, len(cu.PUs))
			}
			pu := &cu.PUs[0]
			c, err := d.mergeMode(grid, sh, cu.X0, cu.Y0, cb, cb, int(cu.Log2Size), 0, int(pu.MergeIdx), rpl0, rpl1, sh.POC, col)
			if err != nil {
				return nil, err
			}
			if sh.Type != SliceB && c.pred != motionL0 {
				return nil, fmt.Errorf("%w: skip pred %d", ErrNotDecodable, c.pred)
			}
			d.fillGrid(grid, cu.X0, cu.Y0, cb, cb, c)
			setPUFinal(pu, c)
			out = append(out, puMotion{x0: cu.X0, y0: cu.Y0, w: cb, h: cb, mv: c})
		case PredInter:
			rects := interPURects(cu.X0, cu.Y0, cb, cu.Part)
			if len(rects) != len(cu.PUs) {
				return nil, fmt.Errorf("%w: PUs %d vs %d rects", ErrBadSlice, len(cu.PUs), len(rects))
			}
			// Pass 1: merge PUs fill the grid first (peer fills
			// right after each PU, but merge entries never depend
			// on later AMVP vectors; AMVP neighbours must see
			// every merge vector in this CU). Order of `out`
			// still follows decode order below.
			merged := map[int]puMotion{}
			for i, r := range rects {
				pu := &cu.PUs[i]
				if !pu.Merge {
					continue
				}
				m, err := d.mergeMode(grid, sh, r[0], r[1], r[2], r[3], int(cu.Log2Size), i, int(pu.MergeIdx), rpl0, rpl1, sh.POC, col)
				if err != nil {
					return nil, err
				}
				if sh.Type != SliceB && m.pred != motionL0 {
					return nil, fmt.Errorf("%w: inter pred %d", ErrNotDecodable, m.pred)
				}
				d.fillGrid(grid, r[0], r[1], r[2], r[3], m)
				setPUFinal(pu, m)
				merged[i] = puMotion{x0: r[0], y0: r[1], w: r[2], h: r[3], mv: m}
			}
			// Pass 2: AMVP PUs read the full merge field.
			for i, r := range rects {
				pu := &cu.PUs[i]
				if pu.Merge {
					out = append(out, merged[i])
					continue
				}
				var c mvCand
				m, err := d.amvpPU(grid, sh, r, int(cu.Log2Size), i, pu, rpl0, rpl1, sh.POC, col, col1)
				if err != nil {
					return nil, err
				}
				c = m
				if sh.Type != SliceB && c.pred != motionL0 {
					return nil, fmt.Errorf("%w: inter pred %d", ErrNotDecodable, c.pred)
				}
				d.fillGrid(grid, r[0], r[1], r[2], r[3], c)
				setPUFinal(pu, c)
				out = append(out, puMotion{x0: r[0], y0: r[1], w: r[2], h: r[3], mv: c})
			}
		}
	}
	return out, nil
}

// setPUFinal stamps derived vectors back into the PU syntax.
func setPUFinal(pu *PUInfo, c mvCand) {
	pu.Pred = uint8(c.pred)
	pu.RefIdx, pu.MVX, pu.MVY = uint8(c.ref0), c.x0, c.y0
	pu.Ref1, pu.MV1X, pu.MV1Y = uint8(c.ref1), c.x1, c.y1
}

// amvpPU derives one non-merge PU per active list (peer
// hevc_luma_mv_mvp_mode call order: L0 then L1; MVD adds on).
func (d *interDec) amvpPU(g *mvGrid, sh *SliceHeader, r [4]int, log2cb, partIdx int, pu *PUInfo, rpl0, rpl1 []refPic, curPOC int, col, col1 *picState) (mvCand, error) {
	colocL1 := !sh.ColocFromL0
	c := mvCand{}
	active := pu.Pred
	if sh.Type != SliceB {
		active = PredL0
	}
	// PUs ride the peer's InterPredIdc wire order (PRED_L0=0,
	// PRED_L1=1, PRED_BI=2): L0-active means L0 only or BI, L1-
	// active means L1 only or BI. PredL0(0) must not be tested
	// with != against the BI value (BI!=L0), so the gates use
	// explicit per-value matches.
	l0Active := active == PredL0 || active == PredBI
	l1Active := active == PredL1 || active == PredBI
	if l0Active {
		c.pred |= motionL0
		c.ref0 = int(pu.RefIdx)
		px, py, err := d.mvpMode(g, sh, r[0], r[1], r[2], r[3], int(pu.RefIdx), int(pu.MVPFlag), rpl0, curPOC, col, 0, rpl0, rpl1, colocL1)
		if err != nil {
			return mvCand{}, err
		}
		c.x0, c.y0 = clipMV(px+pu.MVDX), clipMV(py+pu.MVDY)
	}
	if l1Active {
		c.pred |= motionL1
		c.ref1 = int(pu.Ref1)
		px, py, err := d.mvpMode(g, sh, r[0], r[1], r[2], r[3], int(pu.Ref1), int(pu.MVPFlag1), rpl1, curPOC, col1, 1, rpl0, rpl1, colocL1)
		if err != nil {
			return mvCand{}, err
		}
		c.x1, c.y1 = clipMV(px+pu.MVD1X), clipMV(py+pu.MVD1Y)
	}
	return c, nil
}

// mvpMode mirrors ff_hevc_luma_mv_mvp_mode for one list (P passes
// L0; B passes each active list): left group (same-pic then scaled),
// above group, temporal on flag match. Only the predictor returns;
// the caller adds the parsed MVD.
func (d *interDec) mvpMode(g *mvGrid, sh *SliceHeader, x0, y0, w, h, refIdx int, mvpFlag int, curRPL []refPic, curPOC int, col *picState, X int, rpl0, rpl1 []refPic, colocL1 bool) (int32, int32, error) {
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
		// Same-picture pull: the neighbour's own list must match
		// the current list (peer's mv_mp_mode_mx compares
		// refPicList[pred_flag_index] against refPicList[ref_idx_curr]).
		if !sameListPOC(g, x, y, lx, X, er, refIdx, curRPL, rpl0, rpl1, d) {
			return 0, 0, false
		}
		return mvx, mvy, true
	}
	mxScaled := func(x, y, lx int) (int32, int32, bool) {
		f := d.gridAt(g, x, y)
		if f == nil || f.pred&(1<<lx) == 0 {
			return 0, 0, false
		}
		return d.distScale(g, x, y, lx, X, refIdx, curRPL, curPOC, rpl0, rpl1)
	}
	var mxA [2]int32
	var mxB [2]int32
	aOK := true
	avA0 := bottomLeft && yA0 < int(d.sps.Height) && d.zAvail(x0, y0, xA0, yA0) && d.gridAt(g, xA0, yA0) != nil && d.gridAt(g, xA0, yA0).pred != motionIntra
	avA1 := d.avail(g, left, xA1, yA1) != nil
	scaledL0 := avA0 || avA1
	// Peer try order is X-first: for X=0 L0 then L1, for X=1 L1
	// then L0 (mvs.c pred_flag_index_l0/l1). Hardcoding L0-first
	// picks the wrong predictor on L1 (probe poc1 32,16 L1 took
	// scaled L0 (0,0) instead of scaled L1 (0,48) -> final (0,-44)
	// vs peer (0,4)).
	lx0, lx1 := 0, 1
	if X == 1 {
		lx0, lx1 = 1, 0
	}
	// Left group: same-picture per spot (A0 lx0/lx1, A1 lx0/lx1),
	// then scaled per spot — the peer's exact try order.
	if avA0 {
		if x, y, ok := mxSame(xA0, yA0, lx0); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
		if x, y, ok := mxSame(xA0, yA0, lx1); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
	}
	if avA1 {
		if x, y, ok := mxSame(xA1, yA1, lx0); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
		if x, y, ok := mxSame(xA1, yA1, lx1); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
	}
	if avA0 {
		if x, y, ok := mxScaled(xA0, yA0, lx0); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
		if x, y, ok := mxScaled(xA0, yA0, lx1); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
	}
	if avA1 {
		if x, y, ok := mxScaled(xA1, yA1, lx0); ok {
			mxA[0], mxA[1] = x, y
			goto bCand
		}
		if x, y, ok := mxScaled(xA1, yA1, lx1); ok {
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
		if x, y, ok := mxSame(xB0, yB0, lx0); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
		if x, y, ok := mxSame(xB0, yB0, lx1); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
	}
	if avB1 {
		if x, y, ok := mxSame(xB1, yB1, lx0); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
		if x, y, ok := mxSame(xB1, yB1, lx1); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
	}
	if avB2 {
		if x, y, ok := mxSame(xB2, yB2, lx0); ok {
			mxB[0], mxB[1] = x, y
			goto scaleF
		}
		if x, y, ok := mxSame(xB2, yB2, lx1); ok {
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
			if x, y, ok := mxScaled(xB0, yB0, lx0); ok {
				mxB[0], mxB[1], bOK = x, y, true
			} else if x, y, ok := mxScaled(xB0, yB0, lx1); ok {
				mxB[0], mxB[1], bOK = x, y, true
			}
		}
		if !bOK && avB1 {
			if x, y, ok := mxScaled(xB1, yB1, lx0); ok {
				mxB[0], mxB[1], bOK = x, y, true
			} else if x, y, ok := mxScaled(xB1, yB1, lx1); ok {
				mxB[0], mxB[1], bOK = x, y, true
			}
		}
		if !bOK && avB2 {
			if x, y, ok := mxScaled(xB2, yB2, lx0); ok {
				mxB[0], mxB[1], bOK = x, y, true
			} else if x, y, ok := mxScaled(xB2, yB2, lx1); ok {
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
	// Zero-predictor short-circuit: an empty spatial list still
	// consults temporal at flag 0 (peer's gate hits at numMVPCandLX
	// 0), and a single spatial entry consults at flag 1 — never
	// error before the temporal gate runs. No colocated frame yet
	// decodes as zero (probe: ncand 1 c0 0,0 at the first AMVP of
	// each frame); a present-but-unavailable temporal falls back to
	// zero too (peer's mvpcand_list[2] is zero-initialised, so an
	// out-of-range flag reads (0,0): probe MVP x 0 y 32 flag 1
	// ncand 1 c0 0,24 picks zero, final (40,0) = zero + mvd).
	if len(list) < 2 && sh.TemporalMVP && mvpFlag == len(list) {
		if col == nil {
			list = append(list, [2]int32{0, 0})
		} else if mx, my, ok := d.temporalCand(col, x0, y0, w, h, curRPL, curPOC, X, refIdx, rpl0, rpl1, colocL1); ok {
			list = append(list, [2]int32{mx, my})
		} else {
			list = append(list, [2]int32{0, 0})
		}
	}
	if mvpFlag >= len(list) {
		if mvpFlag < 2 {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("%w: mvp %d of %d", ErrBadSlice, mvpFlag, len(list))
	}
	return list[mvpFlag][0], list[mvpFlag][1], nil
}
