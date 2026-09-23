package h264

// P-frame inter layer (VR2b scope): Baseline P slices, single reference,
// CAVLC residual. Covers F3 I/P part, F7 partitions down to 4x4, F8
// quarter-pel with single ref, F18 skip, plus reference management hooks.
//
// Design mirrors the slice-data flow of the reference decoder
// (skip-run state machine, per-partition motion prediction, MC then
// residual): ideas only, all code written from scratch.

// golombToInterCBP maps CBP ue values for inter MBs.
var golombToInterCBP = [48]uint8{
	0, 16, 1, 2, 4, 8, 32, 3, 5, 10, 12, 15, 47, 7, 11, 13,
	14, 6, 9, 31, 35, 37, 42, 44, 33, 34, 36, 40, 39, 43, 45, 46,
	17, 18, 20, 24, 19, 21, 26, 28, 23, 27, 29, 30, 22, 25, 38, 41,
}

func median3(a, b, c int16) int16 {
	if a > b {
		a, b = b, a
	}
	if b > c {
		b = c
		if a > b {
			b = a
		}
	}
	return b
}

func clipInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clipU8 clips v to [0,255] without branches. Bit-identical to
// clipInt(v, 0, 255) for |v| < 2^30 (all filter outputs here stay
// within a few thousand): negative values yield 0, values above 255
// yield 255, the rest pass through. Arithmetic shift plus masks only,
// so quarter-pel combine loops pay no branch mispredicts.
func clipU8(v int) int {
	t := v >> 31
	v &= ^t
	t = (255 - v) >> 31
	v |= t
	return v & 255
}

// partNotAvailable marks neighbours outside the picture: only those fall
// back to the top-left diagonal and force the median default. Intra
// neighbours report ref -1 with a zero vector.
const partNotAvailable = int8(-2)

// mvNeighbour reads one 4x4 motion slot. ref -1 means present but without
// a vector (intra or not yet decoded); partNotAvailable means outside.
func (d *Decoder) mvNeighbour(bx, by int) (mx, my int16, ref int8) {
	if bx < 0 || by < 0 || bx >= d.mbW*4 || by >= d.mbH*4 {
		return 0, 0, partNotAvailable
	}
	i := by*d.mbW*4 + bx
	r := d.refIdx[i]
	if r < 0 {
		return 0, 0, -1
	}
	return d.mvX[i], d.mvY[i], r
}

// scan8Cache maps grouped 4x4 index to motion-cache slot, mirroring the
// reference layout the diagonal predictor indexes.
var scan8Cache = [16]int{
	12, 13, 20, 21, 14, 15, 22, 23,
	28, 29, 36, 37, 30, 31, 38, 39,
}

// deadCacheSlot reports diagonal slots the reference never maintains:
// init to not-available forever, so the predictor always falls back to
// the top-left diagonal for these shapes (bottom-right 8x8, bottom 16x8,
// and matching sub-partitions).
func deadCacheSlot(n, pw int) bool {
	if n < 0 || n >= len(scan8Cache) {
		return false
	}
	s := scan8Cache[n] - 8 + pw
	return s == 16 || s == 24 || s == 32
}

// predSkipP predicts a P-skip block: a zero left or top neighbour (ref 0
// with a zero vector) forces zero motion before the median path. Missing
// neighbours count as zero too; intra neighbours report ref -1 and fall
// through to the median like the reference.
func (d *Decoder) predSkipP(mbx, mby int) (int16, int16) {
	cur := mby*d.mbW + mbx
	ax, ay, ar := d.gatedNeighbour(mbx*4-1, mby*4, cur)
	if ar == partNotAvailable || (ar == 0 && ax == 0 && ay == 0) {
		return 0, 0
	}
	bx, by, br := d.gatedNeighbour(mbx*4, mby*4-1, cur)
	if br == partNotAvailable || (br == 0 && bx == 0 && by == 0) {
		return 0, 0
	}
	return d.predMotion(0, mbx*4, mby*4, 4, 0)
}

// predMotion predicts one partition: n is its top-left grouped 4x4 index,
// (x0,y0) its 4x4 origin, w4 its width in 4x4 units.
func (d *Decoder) predMotion(n, x0, y0, w4 int, ref int8) (int16, int16) {
	ax, ay, ar := d.mvNeighbour(x0-1, y0)
	bx, by, br := d.mvNeighbour(x0, y0-1)
	cx, cy, cr := d.diagNeighbour(n, w4, x0, y0)
	if cr == partNotAvailable {
		cx, cy, cr = d.topLeftNeighbour(x0, y0)
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
	switch {
	case match > 1:
		return median3(ax, bx, cx), median3(ay, by, cy)
	case match == 1:
		if ar == ref {
			return ax, ay
		}
		if br == ref {
			return bx, by
		}
		return cx, cy
	default:
		if br == partNotAvailable && cr == partNotAvailable && ar != partNotAvailable {
			return ax, ay
		}
		return median3(ax, bx, cx), median3(ay, by, cy)
	}
}

// diagNeighbour reads the above-right diagonal for motion prediction.
// Dead cache slots always report not-available (top-left fallback); other
// positions read the picture with cross-slice neighbours gated out. Slots
// inside the macroblock under decode hold fresh stores, except the two
// top rows of the right halves, which the caller poisons at MB start
// (see poisonDiagSlots) until their partitions decode.
func (d *Decoder) diagNeighbour(n, w4, x0, y0 int) (mx, my int16, ref int8) {
	if deadCacheSlot(n, w4) {
		return 0, 0, partNotAvailable
	}
	return d.gatedNeighbour(x0+w4, y0-1, (y0/4)*d.mbW+x0/4)
}

// topLeftNeighbour reads the above-left diagonal with the same gating.
func (d *Decoder) topLeftNeighbour(x0, y0 int) (mx, my int16, ref int8) {
	return d.gatedNeighbour(x0-1, y0-1, (y0/4)*d.mbW+x0/4)
}

// mvNeighbourL reads one 4x4 motion slot for one reference list. The
// slot counts only when the partition actually predicts from that list
// (useM); otherwise it reads intra-like (-1) even with a stored index.
func (d *Decoder) mvNeighbourL(list, bx, by int) (mx, my int16, ref int8) {
	if bx < 0 || by < 0 || bx >= d.mbW*4 || by >= d.mbH*4 {
		return 0, 0, partNotAvailable
	}
	i := by*d.mbW*4 + bx
	var r int8
	if list == 0 {
		r = d.refIdx[i]
	} else {
		r = d.refIdx1[i]
	}
	if r == partNotAvailable {
		return 0, 0, partNotAvailable
	}
	if d.useM[i]&(1<<uint(list)) == 0 {
		return 0, 0, -1
	}
	if r < 0 {
		return 0, 0, -1
	}
	if list == 0 {
		return d.mvX[i], d.mvY[i], r
	}
	return d.mvX1[i], d.mvY1[i], r
}

// gatedNeighbourL gates one list-aware slot by slice, like gatedNeighbour.
func (d *Decoder) gatedNeighbourL(list, bx, by, cur int) (mx, my int16, ref int8) {
	if bx < 0 || by < 0 || bx >= d.mbW*4 || by >= d.mbH*4 {
		return 0, 0, partNotAvailable
	}
	mb := (by/4)*d.mbW + bx/4
	if mb != cur && !d.cabSameSlice(mb) {
		return 0, 0, partNotAvailable
	}
	return d.mvNeighbourL(list, bx, by)
}

// diagNeighbourL is the list-aware above-right diagonal with the same
// dead-slot and slice gating as diagNeighbour.
func (d *Decoder) diagNeighbourL(list, n, w4, x0, y0 int) (mx, my int16, ref int8) {
	if deadCacheSlot(n, w4) {
		return 0, 0, partNotAvailable
	}
	return d.gatedNeighbourL(list, x0+w4, y0-1, (y0/4)*d.mbW+x0/4)
}

// topLeftNeighbourL is the list-aware above-left diagonal.
func (d *Decoder) topLeftNeighbourL(list, x0, y0 int) (mx, my int16, ref int8) {
	return d.gatedNeighbourL(list, x0-1, y0-1, (y0/4)*d.mbW+x0/4)
}

// predMotionL predicts one B partition from one reference list, mirroring
// predMotion's matching rules with list-aware neighbours.
func (d *Decoder) predMotionL(list, n, x0, y0, w4 int, ref int8) (int16, int16) {
	ax, ay, ar := d.mvNeighbourL(list, x0-1, y0)
	bx, by, br := d.mvNeighbourL(list, x0, y0-1)
	cx, cy, cr := d.diagNeighbourL(list, n, w4, x0, y0)
	if cr == partNotAvailable {
		cx, cy, cr = d.topLeftNeighbourL(list, x0, y0)
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
	switch {
	case match > 1:
		return median3(ax, bx, cx), median3(ay, by, cy)
	case match == 1:
		if ar == ref {
			return ax, ay
		}
		if br == ref {
			return bx, by
		}
		return cx, cy
	default:
		if br == partNotAvailable && cr == partNotAvailable && ar != partNotAvailable {
			return ax, ay
		}
		return median3(ax, bx, cx), median3(ay, by, cy)
	}
}

// gatedNeighbour reads one slot, treating neighbours from another slice
// as not-available. Slots of the macroblock under decode (cur) are always
// same-slice; its tag publishes only after motion parsing.
func (d *Decoder) gatedNeighbour(bx, by, cur int) (mx, my int16, ref int8) {
	if bx < 0 || by < 0 || bx >= d.mbW*4 || by >= d.mbH*4 {
		return 0, 0, partNotAvailable
	}
	mb := (by/4)*d.mbW + bx/4
	if mb != cur && !d.cabSameSlice(mb) {
		return 0, 0, partNotAvailable
	}
	return d.mvNeighbour(bx, by)
}

// poisonDiagSlots marks the two right-half top rows not-available at
// inter-MB start: later partitions read them as not-available until the
// owning 8x8 stores overwrite them, mirroring the reference fill.
func (d *Decoder) poisonDiagSlots(mbx, mby int) {
	stride := d.mbW * 4
	d.refIdx[(mby*4+0)*stride+mbx*4+2] = partNotAvailable
	d.refIdx[(mby*4+2)*stride+mbx*4+2] = partNotAvailable
	d.refIdx1[(mby*4+0)*stride+mbx*4+2] = partNotAvailable
	d.refIdx1[(mby*4+2)*stride+mbx*4+2] = partNotAvailable
}

// pred16x8Top predicts the top half of a P_16x8 MB at 4x4 origin (x0,y0).
func (d *Decoder) pred16x8Top(x0, y0 int, ref int8) (int16, int16) {
	if _, _, br := d.mvNeighbour(x0, y0-1); br == ref {
		mx, my, _ := d.mvNeighbour(x0, y0-1)
		return mx, my
	}
	return d.predMotion(0, x0, y0, 4, ref)
}

// pred16x8Bottom predicts the bottom half at origin (x0,y0+2).
func (d *Decoder) pred16x8Bottom(x0, y0 int, ref int8) (int16, int16) {
	if _, _, ar := d.mvNeighbour(x0-1, y0); ar == ref {
		mx, my, _ := d.mvNeighbour(x0-1, y0)
		return mx, my
	}
	return d.predMotion(8, x0, y0, 4, ref)
}

// pred8x16Left predicts the left half at origin (x0,y0).
func (d *Decoder) pred8x16Left(x0, y0 int, ref int8) (int16, int16) {
	if _, _, ar := d.mvNeighbour(x0-1, y0); ar == ref {
		mx, my, _ := d.mvNeighbour(x0-1, y0)
		return mx, my
	}
	return d.predMotion(0, x0, y0, 2, ref)
}

// pred8x16Right predicts the right half; px is its 4x4 origin (mbX0+2).
// The directional neighbour is C (above-right) with D (above-left)
// standing in when C is unavailable (spec 8.4.1.3.2, the same fallback
// the median path in predMotion applies); a reference match returns it
// directly. Testing raw C alone mispredicts the rightmost column, where
// C is always out of picture (oceans s100: median (-10,65) instead of
// the correct D (-8,65), 176 luma diffs).
func (d *Decoder) pred8x16Right(px, py int, ref int8) (int16, int16) {
	mx, my, cr := d.diagNeighbour(4, 2, px, py)
	if cr == partNotAvailable {
		mx, my, cr = d.topLeftNeighbour(px, py)
	}
	if cr == ref {
		return mx, my
	}
	return d.predMotion(4, px, py, 2, ref)
}

// storeMV fills one luma rectangle (pixels) with a motion vector and its
// difference (the difference feeds CABAC neighbour contexts).
func (d *Decoder) storeMV(px, py, w, h int, mx, my, mdx, mdy int16, ref int8) {
	stride := d.mbW * 4
	for y := py / 4; y < (py+h)/4; y++ {
		for x := px / 4; x < (px+w)/4; x++ {
			i := y*stride + x
			d.mvX[i], d.mvY[i] = mx, my
			d.mvdX[i], d.mvdY[i] = mdx, mdy
			d.refIdx[i] = ref
			d.useM[i] |= useL0
		}
	}
}

// storeMV1 records one partition's list-1 motion, shadowing storeMV.
func (d *Decoder) storeMV1(px, py, w, h int, mx, my, mdx, mdy int16, ref int8) {
	stride := d.mbW * 4
	for y := py / 4; y < (py+h)/4; y++ {
		for x := px / 4; x < (px+w)/4; x++ {
			i := y*stride + x
			d.mvX1[i], d.mvY1[i] = mx, my
			d.mvdX1[i], d.mvdY1[i] = mdx, mdy
			d.refIdx1[i] = ref
			d.useM[i] |= useL1
		}
	}
}

// storeRef records one partition's reference index into scratch the
// moment it is read: later partitions of the same macroblock consult it
// for neighbour contexts. It must not touch refIdx yet — motion
// prediction still uses refIdx<0 to spot not-yet-decoded partitions.
func (d *Decoder) storeRef(bx, by, w, h int, ref int8) {
	stride := d.mbW * 4
	for y := by; y < by+h; y++ {
		for x := bx; x < bx+w; x++ {
			d.refTmp[y*stride+x] = ref
		}
	}
}

// storeRef1 is the list-1 shadow of storeRef.
func (d *Decoder) storeRef1(bx, by, w, h int, ref int8) {
	stride := d.mbW * 4
	for y := by; y < by+h; y++ {
		for x := bx; x < bx+w; x++ {
			d.refTmp1[y*stride+x] = ref
		}
	}
}

// zeroMVL clears one list's motion of a rectangle (pixels) the decoded
// partition does not use, so boundary-strength cross-list compares read
// deterministic zeros instead of a previous picture's leftovers.
func (d *Decoder) zeroMVL(px, py, w, h, list int) {
	stride := d.mbW * 4
	for y := py / 4; y < (py+h)/4; y++ {
		for x := px / 4; x < (px+w)/4; x++ {
			i := y*stride + x
			if list == 0 {
				d.mvX[i], d.mvY[i] = 0, 0
				d.mvdX[i], d.mvdY[i] = 0, 0
			} else {
				d.mvX1[i], d.mvY1[i] = 0, 0
				d.mvdX1[i], d.mvdY1[i] = 0, 0
			}
		}
	}
}

const (
	useL0 = 1
	useL1 = 2
)

// markIntraMB clears motion state for an intra MB in a P slice.
func (d *Decoder) markIntraMB(mbx, mby int) {
	stride := d.mbW * 4
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			i := (mby*4+y)*stride + mbx*4 + x
			d.refIdx[i] = -1
			d.refIdx1[i] = -1
			d.useM[i] = 0
			d.direct4[i] = false
			// CABAC motion contexts read raw MVD slots: intra must
			// contribute zero, never a stale inter difference.
			d.mvdX[i] = 0
			d.mvdY[i] = 0
			d.mvdX1[i] = 0
			d.mvdY1[i] = 0
		}
	}
	d.mbIntra[mby*d.mbW+mbx] = true
}

// fullPel clips reference access with edge extension.
func fullPel(plane []uint8, w, h, x, y int) int {
	return int(plane[clipInt(y, 0, h-1)*w+clipInt(x, 0, w-1)])
}

// halfH is the 6-tap half-pel filter horizontally.
func halfH(plane []uint8, w, h, x, y int) int {
	p0 := fullPel(plane, w, h, x-2, y)
	p1 := fullPel(plane, w, h, x-1, y)
	p2 := fullPel(plane, w, h, x, y)
	p3 := fullPel(plane, w, h, x+1, y)
	p4 := fullPel(plane, w, h, x+2, y)
	p5 := fullPel(plane, w, h, x+3, y)
	return clipU8((p0 + p5 - 5*(p1+p4) + 20*(p2+p3) + 16) >> 5)
}

// halfV is the 6-tap half-pel filter vertically.
func halfV(plane []uint8, w, h, x, y int) int {
	p0 := fullPel(plane, w, h, x, y-2)
	p1 := fullPel(plane, w, h, x, y-1)
	p2 := fullPel(plane, w, h, x, y)
	p3 := fullPel(plane, w, h, x, y+1)
	p4 := fullPel(plane, w, h, x, y+2)
	p5 := fullPel(plane, w, h, x, y+3)
	return clipU8((p0 + p5 - 5*(p1+p4) + 20*(p2+p3) + 16) >> 5)
}

// centerJ is the half-half position: cascaded 6-tap without intermediate
// rounding, single +512>>10 step.
func centerJ(plane []uint8, w, h, x, y int) int {
	var tmp [6]int
	for i := 0; i < 6; i++ {
		yy := y - 2 + i
		p0 := fullPel(plane, w, h, x-2, yy)
		p1 := fullPel(plane, w, h, x-1, yy)
		p2 := fullPel(plane, w, h, x, yy)
		p3 := fullPel(plane, w, h, x+1, yy)
		p4 := fullPel(plane, w, h, x+2, yy)
		p5 := fullPel(plane, w, h, x+3, yy)
		tmp[i] = p0 + p5 - 5*(p1+p4) + 20*(p2+p3)
	}
	sum := tmp[0] + tmp[5] - 5*(tmp[1]+tmp[4]) + 20*(tmp[2]+tmp[3])
	return clipU8((sum + 512) >> 10)
}

func avg2(a, b int) int { return (a + b + 1) >> 1 }

// lumaSample interpolates one luma sample at quarter-pel frac (0..3).
func lumaSample(plane []uint8, w, h, x, y, fx, fy int) int {
	f := fullPel(plane, w, h, x, y)
	switch {
	case fx == 0 && fy == 0:
		return f
	case fy == 0:
		hh := halfH(plane, w, h, x, y)
		switch fx {
		case 1:
			return avg2(f, hh)
		case 2:
			return hh
		default:
			return avg2(hh, fullPel(plane, w, h, x+1, y))
		}
	case fx == 0:
		hv := halfV(plane, w, h, x, y)
		switch fy {
		case 1:
			return avg2(f, hv)
		case 2:
			return hv
		default:
			return avg2(hv, fullPel(plane, w, h, x, y+1))
		}
	}
	hh := halfH(plane, w, h, x, y)
	hv := halfV(plane, w, h, x, y)
	jj := centerJ(plane, w, h, x, y)
	hh1 := halfH(plane, w, h, x, y+1)
	hv1 := halfV(plane, w, h, x+1, y)
	switch {
	case fx == 2 && fy == 2:
		return jj
	case fy == 1 && fx == 1:
		return avg2(hh, hv)
	case fy == 1 && fx == 2:
		return avg2(hh, jj)
	case fy == 1:
		if fx == 3 {
			return avg2(hh, hv1)
		}
	case fy == 2 && fx == 1:
		return avg2(hv, jj)
	case fy == 2 && fx == 3:
		return avg2(jj, hv1)
	case fy == 3 && fx == 1:
		return avg2(hv, hh1)
	case fy == 3 && fx == 2:
		return avg2(hh1, jj)
	case fy == 3 && fx == 3:
		return avg2(hh1, hv1)
	}
	return avg2(hh, hv)
}

// predictLumaBlock writes one inter partition prediction from the reference
// into out (length w*h).
func predictLumaBlock(ref *Picture, px, py, w, h int, mx, my int16, out []uint8) {
	if ref == nil {
		for i := range out {
			out[i] = 128
		}
		return
	}
	predictLumaBlockInto(ref, px, py, w, h, mx, my, out, w)
}

// predictLumaBlockInto is the strided entry (S1-W direct-write):
// output row dy lands at dst[dy*dstStride:dy*dstStride+w]. Same
// source rect, same filter, same bytes; the dense entry above is one
// call with dstStride == w. The nil-ref fill (128) also strides.
func predictLumaBlockInto(ref *Picture, px, py, w, h int, mx, my int16, dst []uint8, dstStride int) {
	if ref == nil {
		for dy := 0; dy < h; dy++ {
			base := dy * dstStride
			for dx := 0; dx < w; dx++ {
				dst[base+dx] = 128
			}
		}
		return
	}
	rw, rh := int(ref.Width), int(ref.Height)
	if predictStatsOn {
		notePredictClass(rw, rh, px, py, w, h, mx, my)
	}
	// Fast path: integer-pel motion inside the frame is a plain row copy
	// (no 6-tap filter, no per-pixel clipping). Static content — the
	// common case for screen recordings — lands here for every block.
	if sx, sy, ok := lumaIntSrc(rw, rh, px, py, w, h, mx, my); ok {
		// S1b-C width-hoisted row loops (same bytes as copyRow per
		// row): the width switch sits outside the loop so each row
		// pays two slices + inline moves only — no per-row switch,
		// no memmove call. Constant widths compile to inline moves.
		// S1b-J: the 16-wide lane (the hot one, ~70ms flat per 90f
		// profile) runs one copyBlock kernel call for the whole
		// block (peer ffmpeg ff_put_pixels16_sse2, same plain copy);
		// 8/4 lanes keep their row loops (near-zero flat).
		if w == 16 {
			copyBlockInto(dst, dstStride, ref.Y[sy*rw+sx:], rw, 16, h)
			return
		}
		switch w {
		case 8:
			for dy := 0; dy < h; dy++ {
				di := dy * dstStride
				si := (sy+dy)*rw + sx
				copy(dst[di:di+8], ref.Y[si:si+8])
			}
		case 4:
			for dy := 0; dy < h; dy++ {
				di := dy * dstStride
				si := (sy+dy)*rw + sx
				copy(dst[di:di+4], ref.Y[si:si+4])
			}
		default:
			for dy := 0; dy < h; dy++ {
				di := dy * dstStride
				si := (sy+dy)*rw + sx
				copyRow(dst[di:], ref.Y[si:], w)
			}
		}
		return
	}
	// S1 fast path: interior sub-pel blocks run the arch row kernel
	// (qpel_fast.go); edges and forced-scalar fall to the留守 below.
	if qpelFastInto(ref.Y, rw, rh, px, py, w, h, mx, my, dst, dstStride) {
		return
	}
	predictLumaBlockScalarInto(ref.Y, rw, rh, px, py, w, h, mx, my, dst, dstStride)
}

// lumaIntSrc reports the integer source origin when (mx,my) is
// integer-pel and the source rect sits fully inside the frame (same
// condition as the predictLumaBlock row-copy fast path). Single source
// for both the dense path above and the direct-write path in pdec.go;
// keep callers on this helper so the gate never drifts.
func lumaIntSrc(rw, rh, px, py, w, h int, mx, my int16) (sx, sy int, ok bool) {
	if int(mx)&3 != 0 || int(my)&3 != 0 {
		return 0, 0, false
	}
	sx = px + int(mx>>2)
	sy = py + int(my>>2)
	if sx < 0 || sy < 0 || sx+w > rw || sy+h > rh {
		return 0, 0, false
	}
	return sx, sy, true
}

// copyRow copies one w-byte row without a runtime.memmove call.
// Live partition widths are 2/4/8/16; constant-size copy compiles to
// inline moves (bounds checks eliminated, same bytes on every arch).
// Other widths keep the generic copy. Covers the thousands of small
// row copies per frame (luma/chroma integer paths, strided block
// copies) whose call overhead dominated the move itself.
func copyRow(dst, src []byte, w int) {
	switch w {
	case 2:
		copy(dst[:2], src[:2])
	case 4:
		copy(dst[:4], src[:4])
	case 8:
		copy(dst[:8], src[:8])
	case 16:
		copy(dst[:16], src[:16])
	default:
		copy(dst[:w], src[:w])
	}
}

// copyBlockStrided copies w*h bytes from src (stride srcStride, origin
// sx,sy) to dst (stride dstStride, origin dx,dy) row by row (copyRow
// per row, no memmove calls). Bit-identical to predict-then-copy; the
// win is one fewer touch of every byte (no intermediate block).
func copyBlockStrided(dst []uint8, dstStride, dx, dy int, src []uint8, srcStride, sx, sy, w, h int) {
	for r := 0; r < h; r++ {
		di := (dy+r)*dstStride + dx
		si := (sy+r)*srcStride + sx
		copyRow(dst[di:], src[si:], w)
	}
}

// chromaIntSrc mirrors lumaIntSrc for chroma (eighth-pel-exact: both
// fracs zero, same gate as predictChromaBlock).
func chromaIntSrc(w, h, px, py, cw, ch int, mx, my int16) (sx, sy int, ok bool) {
	if int(mx)&7 != 0 || int(my)&7 != 0 {
		return 0, 0, false
	}
	sx = px + int(mx>>3)
	sy = py + int(my>>3)
	if sx < 0 || sy < 0 || sx+cw > w || sy+ch > h {
		return 0, 0, false
	}
	return sx, sy, true
}

// predictLumaBlockScalar is the scalar留守 (C版留守): per-pixel 6-tap
// interpolation with per-tap edge clipping. Bit-exact reference for the
// S1 fast paths: SIMD缺席/贴边/强制标量时回落到此, 输出逐位一致由
// qpel_s1_test.go 锁死. Do not optimize here; optimize in qpel_*.
func predictLumaBlockScalar(plane []uint8, w, h, px, py, bw, bh int, mx, my int16, out []uint8) {
	predictLumaBlockScalarInto(plane, w, h, px, py, bw, bh, mx, my, out, bw)
}

// predictLumaBlockScalarInto is the strided scalar留守 (S1-W
// direct-write): same per-pixel taps as the dense留守, rows stride out.
//
// S1b-E row-band shape (bit-identical pixels): edge-touching blocks
// used to run every pixel through the per-tap-clipping scalar loop,
// ~15x the interior cost per pixel (clipInt+fullPel dominate the
// profile on the 1.5% edge blocks). Motion is linear in the pixel
// (ix = px+dx+m with m = mx>>2 floor, fx constant per block), so the
// interior pixels form a sub-run per row: rows vertically interior
// run their interior dx-run through the vector kernel (bh=1 block),
// rim pixels keep the scalar taps. The kernel gate double-checks the
// taps; a false verdict falls back to the scalar row, so a
// conservative run can only cost speed, never pixels. Peer (ffmpeg,
// read-only, ideas only): emulated-edge buffers (ours keeps exact
// per-tap clip on the rim instead).
func predictLumaBlockScalarInto(plane []uint8, w, h, px, py, bw, bh int, mx, my int16, dst []uint8, dstStride int) {
	m := int(mx) >> 2
	n := int(my) >> 2
	fx := int(mx) - (m << 2)
	fy := int(my) - (n << 2)
	for dy := 0; dy < bh; dy++ {
		base := dy * dstStride
		iy := py + dy + n
		// Vertical taps interior for this row (6-tap needs iy-2..iy+3;
		// conservative for all fy, gate verifies)?
		if iy-2 >= 0 && iy+3 < h {
			// Interior dx-run: ix(dx)=px+dx+m monotonic, need
			// ix-2>=0 && ix+3<w (superset of taps for any fx).
			a := 2 - m - px
			if a < 0 {
				a = 0
			}
			b := w - 4 - m - px
			if b > bw-1 {
				b = bw - 1
			}
			if a <= b {
				runW := b - a + 1
				// Integer run (fx==fy==0): the kernel has no
				// integer lane (its fy==0 leg assumes fx!=0), but
				// the run is tap-interior by construction, so the
				// scalar fullPel clips are no-ops: plain copy.
				if fx == 0 && fy == 0 {
					copyRow(dst[base+a:], plane[iy*w+px+a+m:], runW)
					// S1b-AE rim direct: ix/iy from hoisted m/n,
					// same pixels as scalarPel (fx/fy constant).
					for dx := 0; dx < a; dx++ {
						dst[base+dx] = scalarPelDirect(plane, w, h, px+dx+m, iy, 0, 0)
					}
					for dx := b + 1; dx < bw; dx++ {
						dst[base+dx] = scalarPelDirect(plane, w, h, px+dx+m, iy, 0, 0)
					}
					continue
				}
				if qpelBlockInto(dst[base+a:base+a+runW], runW, plane, w, h, px+a+m, iy, runW, 1, fx, fy) {
					// S1b-AE rim direct (same as above).
					for dx := 0; dx < a; dx++ {
						dst[base+dx] = scalarPelDirect(plane, w, h, px+dx+m, iy, fx, fy)
					}
					for dx := b + 1; dx < bw; dx++ {
						dst[base+dx] = scalarPelDirect(plane, w, h, px+dx+m, iy, fx, fy)
					}
					continue
				}
			}
		}
		// S1b-AE family-hoisted fallback: fx/fy block-constant, so the
		// lumaSample family switch runs once per block, not per pixel.
		// Bodies are the verbatim tap trees (same helpers, same order).
		switch {
		case fx == 0 && fy == 0:
			for dx := 0; dx < bw; dx++ {
				dst[base+dx] = uint8(fullPel(plane, w, h, px+dx+m, iy))
			}
		case fy == 0:
			for dx := 0; dx < bw; dx++ {
				dst[base+dx] = uint8(lumaSampleHor(plane, w, h, px+dx+m, iy, fx))
			}
		case fx == 0:
			for dx := 0; dx < bw; dx++ {
				dst[base+dx] = uint8(lumaSampleVer(plane, w, h, px+dx+m, iy, fy))
			}
		default:
			// S1b-AF pair dispatch (same nine verdicts as
			// lumaSampleGen, one branch per block): each pixel
			// evaluates only its pair's trees.
			switch {
			case fx == 2 && fy == 2:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen22(plane, w, h, px+dx+m, iy))
				}
			case fx == 1 && fy == 1:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen11(plane, w, h, px+dx+m, iy))
				}
			case fx == 2 && fy == 1:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen21(plane, w, h, px+dx+m, iy))
				}
			case fx == 3 && fy == 1:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen31(plane, w, h, px+dx+m, iy))
				}
			case fx == 1 && fy == 2:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen12(plane, w, h, px+dx+m, iy))
				}
			case fx == 3 && fy == 2:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen32(plane, w, h, px+dx+m, iy))
				}
			case fx == 1 && fy == 3:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen13(plane, w, h, px+dx+m, iy))
				}
			case fx == 2 && fy == 3:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen23(plane, w, h, px+dx+m, iy))
				}
			default:
				for dx := 0; dx < bw; dx++ {
					dst[base+dx] = uint8(lumaSampleGen33(plane, w, h, px+dx+m, iy))
				}
			}
		}
	}
}

// scalarPel is one pixel of the scalar留守 (extracted so the S1b-E row
// band can reuse it for rim pixels): floor quarter-pel split plus the
// full lumaSample tap tree with per-tap edge clipping.
func scalarPel(plane []uint8, w, h, px, py int, mx, my int16, dx, dy int) uint8 {
	qx := (px+dx)*4 + int(mx)
	qy := (py+dy)*4 + int(my)
	// Floor divide by 4 for possibly negative vectors.
	ix := qx >> 2
	iy := qy >> 2
	fx := qx - (ix << 2)
	fy := qy - (iy << 2)
	return uint8(lumaSample(plane, w, h, ix, iy, fx, fy))
}

// S1b-AE frac-hoisted rim (bit-identical pixels): fx/fy are
// block-constant (mx/my constant, so (px+dx)*4+mx mod 4 never changes
// with dx/dy), but scalarPel recomputed them per pixel with two
// multiplies + two shifts + two subtracts, and lumaSample re-switched
// the family per pixel. The outer scalar driver already holds m/n/fx/fy,
// so rim pixels pass ix=px+dx+m, iy=py+dy+n straight into the tap tree
// with the family hoisted once per block. scalarPel/lumaSample stay as
// the shared oracle (gated by qpel_s1_test.go all-frac pins).
func scalarPelDirect(plane []uint8, w, h, ix, iy, fx, fy int) uint8 {
	// S1b-AF: same dispatch as the fallback above, so row-band rim
	// pixels (few per edge block) also skip unused trees.
	switch {
	case fx == 0 && fy == 0:
		return uint8(fullPel(plane, w, h, ix, iy))
	case fy == 0:
		return uint8(lumaSampleHor(plane, w, h, ix, iy, fx))
	case fx == 0:
		return uint8(lumaSampleVer(plane, w, h, ix, iy, fy))
	case fx == 2 && fy == 2:
		return uint8(lumaSampleGen22(plane, w, h, ix, iy))
	case fx == 1 && fy == 1:
		return uint8(lumaSampleGen11(plane, w, h, ix, iy))
	case fx == 2 && fy == 1:
		return uint8(lumaSampleGen21(plane, w, h, ix, iy))
	case fx == 3 && fy == 1:
		return uint8(lumaSampleGen31(plane, w, h, ix, iy))
	case fx == 1 && fy == 2:
		return uint8(lumaSampleGen12(plane, w, h, ix, iy))
	case fx == 3 && fy == 2:
		return uint8(lumaSampleGen32(plane, w, h, ix, iy))
	case fx == 1 && fy == 3:
		return uint8(lumaSampleGen13(plane, w, h, ix, iy))
	case fx == 2 && fy == 3:
		return uint8(lumaSampleGen23(plane, w, h, ix, iy))
	default:
		return uint8(lumaSampleGen33(plane, w, h, ix, iy))
	}
}

// S1b-AF gen-pair specializers (bit-identical pixels): lumaSampleGen
// above evaluates all five tap trees per pixel (hh+hv+jj+hh1+hv1, ~60
// fullPel clips) then selects one verdict, but each (fx,fy) pair needs
// at most two trees (below: verbatim taps from lumaSampleGen, same
// helpers, same order). The scalar fallback dispatches once per block,
// rim pixels reuse the same pair. lumaSampleGen stays as the shared
// oracle (gated by qpel_s1_test.go all-frac pins).
func lumaSampleGen22(plane []uint8, w, h, x, y int) int {
	return centerJ(plane, w, h, x, y)
}

func lumaSampleGen11(plane []uint8, w, h, x, y int) int {
	return avg2(halfH(plane, w, h, x, y), halfV(plane, w, h, x, y))
}

func lumaSampleGen21(plane []uint8, w, h, x, y int) int {
	return avg2(halfH(plane, w, h, x, y), centerJ(plane, w, h, x, y))
}

func lumaSampleGen31(plane []uint8, w, h, x, y int) int {
	return avg2(halfH(plane, w, h, x, y), halfV(plane, w, h, x+1, y))
}

func lumaSampleGen12(plane []uint8, w, h, x, y int) int {
	return avg2(halfV(plane, w, h, x, y), centerJ(plane, w, h, x, y))
}

func lumaSampleGen32(plane []uint8, w, h, x, y int) int {
	return avg2(centerJ(plane, w, h, x, y), halfV(plane, w, h, x+1, y))
}

func lumaSampleGen13(plane []uint8, w, h, x, y int) int {
	return avg2(halfV(plane, w, h, x, y), halfH(plane, w, h, x, y+1))
}

func lumaSampleGen23(plane []uint8, w, h, x, y int) int {
	return avg2(halfH(plane, w, h, x, y+1), centerJ(plane, w, h, x, y))
}

func lumaSampleGen33(plane []uint8, w, h, x, y int) int {
	return avg2(halfH(plane, w, h, x, y+1), halfV(plane, w, h, x+1, y))
}

func lumaSampleHor(plane []uint8, w, h, x, y, fx int) int {
	f := fullPel(plane, w, h, x, y)
	hh := halfH(plane, w, h, x, y)
	switch fx {
	case 1:
		return avg2(f, hh)
	case 2:
		return hh
	default:
		return avg2(hh, fullPel(plane, w, h, x+1, y))
	}
}

func lumaSampleVer(plane []uint8, w, h, x, y, fy int) int {
	f := fullPel(plane, w, h, x, y)
	hv := halfV(plane, w, h, x, y)
	switch fy {
	case 1:
		return avg2(f, hv)
	case 2:
		return hv
	default:
		return avg2(hv, fullPel(plane, w, h, x, y+1))
	}
}

func lumaSampleGen(plane []uint8, w, h, x, y, fx, fy int) int {
	hh := halfH(plane, w, h, x, y)
	hv := halfV(plane, w, h, x, y)
	jj := centerJ(plane, w, h, x, y)
	hh1 := halfH(plane, w, h, x, y+1)
	hv1 := halfV(plane, w, h, x+1, y)
	switch {
	case fx == 2 && fy == 2:
		return jj
	case fy == 1 && fx == 1:
		return avg2(hh, hv)
	case fy == 1 && fx == 2:
		return avg2(hh, jj)
	case fy == 1:
		if fx == 3 {
			return avg2(hh, hv1)
		}
	case fy == 2 && fx == 1:
		return avg2(hv, jj)
	case fy == 2 && fx == 3:
		return avg2(jj, hv1)
	case fy == 3 && fx == 1:
		return avg2(hv, hh1)
	case fy == 3 && fx == 2:
		return avg2(hh1, jj)
	case fy == 3 && fx == 3:
		return avg2(hh1, hv1)
	}
	return avg2(hh, hv)
}

// predictChromaBlock interpolates one chroma partition into out (length
// cw*ch). The chroma vector shares the luma integer (quarter units double
// as eighth units).
func predictChromaBlock(plane []uint8, w, h, px, py, cw, ch int, mx, my int16, out []uint8) {
	predictChromaBlockInto(plane, w, h, px, py, cw, ch, mx, my, out, cw)
}

// predictChromaBlockInto is the strided entry (S1-W direct-write):
// output row dy lands at dst[dy*dstStride:dy*dstStride+cw]. Same
// source rect, same bilinear weights, same bytes; the dense entry
// above is one call with dstStride == cw.
func predictChromaBlockInto(plane []uint8, w, h, px, py, cw, ch int, mx, my int16, dst []uint8, dstStride int) {
	// Fast path: eighth-pel-exact motion inside the frame is a plain row
	// copy (bilinear weights collapse to the top-left tap).
	if sx, sy, ok := chromaIntSrc(w, h, px, py, cw, ch, mx, my); ok {
		// S1b-C width-hoisted (same shape as the luma integer path).
		switch cw {
		case 8:
			for dy := 0; dy < ch; dy++ {
				di := dy * dstStride
				si := (sy+dy)*w + sx
				copy(dst[di:di+8], plane[si:si+8])
			}
		case 4:
			for dy := 0; dy < ch; dy++ {
				di := dy * dstStride
				si := (sy+dy)*w + sx
				copy(dst[di:di+4], plane[si:si+4])
			}
		case 2:
			for dy := 0; dy < ch; dy++ {
				di := dy * dstStride
				si := (sy+dy)*w + sx
				copy(dst[di:di+2], plane[si:si+2])
			}
		default:
			for dy := 0; dy < ch; dy++ {
				di := dy * dstStride
				si := (sy+dy)*w + sx
				copyRow(dst[di:], plane[si:], cw)
			}
		}
		return
	}
	// S1b-A1 fast path: interior sub-pel blocks run the arch row kernel
	// (chroma_fast.go); edges and forced-scalar fall to the留守 below.
	if chromaFastInto(plane, w, h, px, py, cw, ch, mx, my, dst, dstStride) {
		return
	}
	for dy := 0; dy < ch; dy++ {
		base := dy * dstStride
		for dx := 0; dx < cw; dx++ {
			ex := (px+dx)*8 + int(mx)
			ey := (py+dy)*8 + int(my)
			ix := ex >> 3
			iy := ey >> 3
			fx := ex - (ix << 3)
			fy := ey - (iy << 3)
			a := fullPel(plane, w, h, ix, iy)
			b := fullPel(plane, w, h, ix+1, iy)
			c := fullPel(plane, w, h, ix, iy+1)
			d := fullPel(plane, w, h, ix+1, iy+1)
			v := ((8-fx)*(8-fy)*a + fx*(8-fy)*b + (8-fx)*fy*c + fx*fy*d + 32) >> 6
			dst[base+dx] = uint8(clipU8(v))
		}
	}
}
