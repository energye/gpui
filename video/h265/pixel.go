package h265

// Pixel reconstruction for I slices: dequant + inverse transform +
// intra prediction + add, walked leaf by leaf in decode order.
//
// Peer (read-only, ideas only, no code copied):
//   hevc/cabac.c ff_hevc_hls_residual_coding tail (QP derivation,
//   level scaling, transform choice, add_residual) +
//   hevc/dsp_template.c dequant/transform_4x4_luma/idct_4/8/16/32/dc
//   (shifts, matrix orientation) + hevc/dsp.c transform[32][32]
//   (numbers live in transform_tables.go) +
//   hevc/pred_template.c intra_pred/ref filters/planar/dc/angular
//   (neighbor rules, filter gates, formulas) +
//   hevc/mvs.c ff_hevc_set_neighbour_available (candidate rules).
// Loop filters (deblock/SAO) ride separately.

import (
	"fmt"
)

// Small numeric tables (standard values, same discipline as the scan
// and CABAC tables: numbers travel, logic is fresh).

// levelScale is the H.265 Table 8-14 base scale per qp%6.
var levelScale = [6]int{40, 45, 51, 57, 64, 72}

// chromaQP maps qpi 30..43 to qpc for 4:2:0 (peer qp_c table).
var chromaQP = [14]int{29, 30, 31, 32, 33, 33, 34, 34, 35, 35, 36, 36, 37, 37}

// intraPredAngle holds modes 2..34 (peer pred_angular table).
var intraPredAngle = [33]int{
	32, 26, 21, 17, 13, 9, 5, 2, 0, -2, -5, -9, -13, -17, -21, -26, -32,
	-26, -21, -17, -13, -9, -5, -2, 0, 2, 5, 9, 13, 17, 21, 26, 32,
}

// invAngle holds the projection factors for negative angles
// (peer table, indexed mode-11 for modes 11..25).
var invAngle = [15]int{
	-4096, -1638, -910, -630, -482, -390, -315, -256, -315, -390, -482,
	-630, -910, -1638, -4096,
}

func clipPix(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

func clip16(v int) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return int16(v)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func log2size(size int) int {
	n := 0
	for s := size; s > 1; s >>= 1 {
		n++
	}
	return n
}

// qpFor derives the quant parameter for one residual block: luma uses
// qpY directly, chroma maps through the 4:2:0 table (8-bit: no offset).
func qpFor(cIdx int, qpY int32) int {
	if cIdx == 0 {
		return int(qpY)
	}
	qpi := int(qpY)
	if qpi < 0 {
		qpi = 0
	}
	if qpi > 51 {
		qpi = 51
	}
	if qpi < 30 {
		return qpi
	}
	if qpi > 43 {
		return qpi - 6
	}
	return chromaQP[qpi-30]
}

// dequant scales raster levels in place into transform input:
// (level*scale*16+add)>>shift clipped to int16 (no scaling lists on
// this path, so the matrix factor stays 16 everywhere). shift follows
// the peer: bitDepth+log2-5 (8-bit: log2+3).
func dequant(coeffs []int16, log2t, qp int) {
	shift := 8 + log2t - 5
	add := 1 << (shift - 1)
	scale := levelScale[qp%6] << uint(qp/6)
	for i, v := range coeffs {
		if v == 0 {
			continue
		}
		if v < 0 {
			coeffs[i] = -clip16((int(-v)*scale*16 + add) >> uint(shift))
			continue
		}
		coeffs[i] = clip16((int(v)*scale*16 + add) >> uint(shift))
	}
}

// maxXY reports the largest x/y holding a nonzero coefficient
// (-1 when the block is empty).
func maxXY(coeffs []int16, size int) int {
	m := -1
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if coeffs[y*size+x] != 0 {
				if x > m {
					m = x
				}
				if y > m {
					m = y
				}
			}
		}
	}
	return m
}

// idctDC fills the block from the DC coefficient only.
func idctDC(coeffs []int16, size int) {
	shift := 14 - 8
	add := 1 << (shift - 1)
	c := (int(coeffs[0]) + 1) >> 1
	c = (c + add) >> uint(shift)
	for i := range coeffs {
		coeffs[i] = int16(c)
	}
}

// idctGeneric runs the two-pass separable inverse transform with the
// subsampled matrix rows (out[i] = sum_j T[j*step][i]*in[j]) and the
// peer's shifts (7, then 20-bitDepth).
func idctGeneric(coeffs []int16, size int) {
	step := 32 / size
	tmp := make([]int16, size*size)
	for col := 0; col < size; col++ {
		for i := 0; i < size; i++ {
			acc := 0
			for j := 0; j < size; j++ {
				acc += int(hevcTransform[j*step][i]) * int(coeffs[j*size+col])
			}
			tmp[i*size+col] = int16((acc + 64) >> 7)
		}
	}
	for row := 0; row < size; row++ {
		for i := 0; i < size; i++ {
			acc := 0
			for j := 0; j < size; j++ {
				acc += int(hevcTransform[j*step][i]) * int(tmp[row*size+j])
			}
			coeffs[row*size+i] = int16((acc + 2048) >> 12)
		}
	}
}

// transform4x4Luma is the special luma 4x4 path (peer TR_4x4_LUMA,
// two passes with shifts 7 and 12).
func transform4x4Luma(coeffs []int16) {
	mk := func(a, b, c, d int) (int, int, int, int) {
		c0 := a + c
		c1 := c + d
		c2 := a - d
		c3 := 74 * b
		o2 := 74 * (a - c + d)
		o0 := 29*c0 + 55*c1 + c3
		o1 := 55*c2 - 29*c1 + c3
		o3 := 55*c0 + 29*c2 - c3
		return o0, o1, o2, o3
	}
	for x := 0; x < 4; x++ {
		o0, o1, o2, o3 := mk(int(coeffs[x]), int(coeffs[4+x]), int(coeffs[8+x]), int(coeffs[12+x]))
		coeffs[x] = int16((o0 + 64) >> 7)
		coeffs[4+x] = int16((o1 + 64) >> 7)
		coeffs[8+x] = int16((o2 + 64) >> 7)
		coeffs[12+x] = int16((o3 + 64) >> 7)
	}
	for y := 0; y < 4; y++ {
		o0, o1, o2, o3 := mk(int(coeffs[y*4]), int(coeffs[y*4+1]), int(coeffs[y*4+2]), int(coeffs[y*4+3]))
		coeffs[y*4] = int16((o0 + 2048) >> 12)
		coeffs[y*4+1] = int16((o1 + 2048) >> 12)
		coeffs[y*4+2] = int16((o2 + 2048) >> 12)
		coeffs[y*4+3] = int16((o3 + 2048) >> 12)
	}
}

// inverseTransform scales (peer dequant shape: bitDepth+log2-5 with
// Table 8-14 scale, no scaling lists here) then runs the inverse
// transform over raster levels into spatial residual (in place).
// luma4 selects the intra luma 4x4 path.
func inverseTransform(coeffs []int16, log2t, qp int, luma4 bool) {
	shift := 8 + log2t - 5
	add := 1 << (shift - 1)
	scale := levelScale[qp%6] << uint(qp/6)
	for i, v := range coeffs {
		if v == 0 {
			continue
		}
		coeffs[i] = clip16((int(v)*scale*16 + add) >> uint(shift))
	}
	size := 1 << log2t
	// Intra luma 4x4 always rides the DST path (peer checks it before
	// the DC fast path, so even a lone DC coefficient transforms
	// through transform_4x4_luma, never idct_dc).
	if luma4 {
		transform4x4Luma(coeffs)
		return
	}
	m := maxXY(coeffs, size)
	if m < 0 {
		return
	}
	if m == 0 {
		idctDC(coeffs, size)
		return
	}
	idctGeneric(coeffs, size)
}

// morton interleaves 4 bits (z-scan address inside one CTB).
func morton(x, y int) int {
	a := 0
	for i := 0; i < 4; i++ {
		a |= ((x >> uint(i)) & 1) << (2 * uint(i))
		a |= ((y >> uint(i)) & 1) << (2*uint(i) + 1)
	}
	return a
}

// avail holds intra neighbor availability for one block.
type avail struct {
	left, up, upLeft, upRight, bottomLeft bool
}

// zBefore reports whether the neighbor block at (xN,yN) is decoded
// before the current block at (xC,yC): above/left CTBs first, else
// Morton order inside the CTB. log2Ctb is the SPS CTB size (the old
// clip used 64, the B-clip uses 32; hardcoding 64 marks bottom-left
// neighbours from not-yet-decoded CTB rows available).
func zBefore(xN, yN, xC, yC, picW, picH, log2Ctb int) bool {
	if xN < 0 || yN < 0 || xN >= picW || yN >= picH {
		return false
	}
	ctb := 1 << uint(log2Ctb)
	xNc, yNc := xN/ctb, yN/ctb
	xCc, yCc := xC/ctb, yC/ctb
	if yNc != yCc {
		return yNc < yCc
	}
	if xNc != xCc {
		return xNc < xCc
	}
	const tb = 4
	n := morton((xN-xNc*ctb)/tb, (yN-yNc*ctb)/tb)
	c := morton((xC-xCc*ctb)/tb, (yC-yCc*ctb)/tb)
	return n < c
}

// neighbours derives candidates for a luma-coord rect (w,h in luma
// samples) on a tile-free single-slice picture. log2Ctb is the SPS
// CTB size.
// Peer: mvs.c set_neighbour_available + the z-scan gates in intra_pred.
func neighbours(x0, y0, w, h, picW, picH, log2Ctb int) avail {
	ctb := 1 << uint(log2Ctb)
	cx, cy := x0/ctb, y0/ctb
	x0b, y0b := x0%ctb, y0%ctb
	ctbLeft := cx > 0
	ctbUp := cy > 0
	a := avail{}
	a.up = ctbUp || y0b != 0
	a.left = ctbLeft || x0b != 0
	if x0b != 0 || y0b != 0 {
		a.upLeft = a.left && a.up
	} else {
		a.upLeft = ctbLeft && ctbUp
	}
	if x0b+w == ctb {
		a.upRight = cy > 0 && x0/ctb+1 < (picW+ctb-1)/ctb && y0b == 0
	} else {
		a.upRight = a.up
	}
	if x0+w >= picW {
		a.upRight = false
	}
	endY := cy*ctb + ctb
	if endY > picH {
		endY = picH
	}
	if y0+h >= endY {
		a.bottomLeft = false
	} else {
		a.bottomLeft = a.left
	}
	const tb = 4
	if a.bottomLeft && !zBefore(x0-tb, y0+h, x0, y0, picW, picH, log2Ctb) {
		a.bottomLeft = false
	}
	if a.upRight && !zBefore(x0+w, y0-tb, x0, y0, picW, picH, log2Ctb) {
		a.upRight = false
	}
	return a
}

// refSet gathers one component's reference samples: corner at [0],
// main at [1:size+1], extension at [size+1:2*size+1]. (px,py) address
// the component plane; sizes clip to that plane. Unavailable spans use
// the non-constrained substitution sequence.
func refSet(plane []byte, stride, picW, picH, px, py, size int, a avail) ([]int, []int) {
	top := make([]int, 4*size+1)
	left := make([]int, 4*size+1)
	at := func(x, y int) int { return int(plane[y*stride+x]) }
	// Clipped extension spans in component units.
	bl := py + 2*size
	if bl > picH {
		bl = picH
	}
	bottomSize := bl - (py + size)
	if bottomSize < 0 {
		bottomSize = 0
	}
	tr := px + 2*size
	if tr > picW {
		tr = picW
	}
	rightSize := tr - (px + size)
	if rightSize < 0 {
		rightSize = 0
	}
	if a.upLeft {
		top[0] = at(px-1, py-1)
		left[0] = top[0]
	}
	if a.up {
		for i := 0; i < size; i++ {
			top[1+i] = at(px+i, py-1)
		}
	}
	if a.upRight {
		for i := 0; i < rightSize && i < size; i++ {
			top[1+size+i] = at(px+size+i, py-1)
		}
		for i := rightSize; i < size; i++ {
			top[1+size+i] = top[size+rightSize]
		}
	}
	if a.left {
		for i := 0; i < size; i++ {
			left[1+i] = at(px-1, py+i)
		}
	}
	if a.bottomLeft {
		for i := 0; i < bottomSize && i < size; i++ {
			left[1+size+i] = at(px-1, py+size+i)
		}
		for i := bottomSize; i < size; i++ {
			left[1+size+i] = left[size+bottomSize]
		}
	}
	// Substitution (peer order, non-constrained path).
	if !a.bottomLeft {
		if a.left {
			for i := 0; i < size; i++ {
				left[1+size+i] = left[size]
			}
		} else if a.upLeft {
			for i := 0; i < 2*size; i++ {
				left[1+i] = left[0]
				top[1+i] = left[0]
			}
			a.left = true
		} else if a.up {
			left[0] = top[1]
			for i := 0; i < 2*size; i++ {
				left[1+i] = left[0]
			}
			a.upLeft = true
			a.left = true
		} else if a.upRight {
			for i := 0; i < size; i++ {
				top[1+i] = top[1+size]
			}
			left[0] = top[1+size]
			for i := 0; i < 2*size; i++ {
				left[1+i] = left[0]
			}
			a.up = true
			a.upLeft = true
			a.left = true
		} else {
			left[0] = 128
			for i := 0; i < 2*size; i++ {
				top[1+i] = 128
				left[1+i] = 128
			}
		}
	}
	if !a.left {
		for i := 0; i < size; i++ {
			left[1+i] = left[1+size]
		}
	}
	if !a.upLeft {
		left[0] = left[1]
	}
	if !a.up {
		for i := 0; i < size; i++ {
			top[1+i] = left[0]
		}
	}
	if !a.upRight {
		for i := 0; i < size; i++ {
			top[1+size+i] = top[size]
		}
	}
	top[0] = left[0]
	return top, left
}

// filterRefs applies the 3-tap or strong reference filter per the peer
// gate (luma path; our SPS never disables smoothing, and the strong
// branch additionally needs a 32x32 luma block).
func filterRefs(top, left []int, size, log2t, mode int, strong bool) ([]int, []int) {
	if mode == IntraDC || size == 4 {
		return top, left
	}
	thresh := [3]int{7, 1, 0}[log2t-3]
	minDist := absInt(mode - 26)
	if d := absInt(mode - 10); d < minDist {
		minDist = d
	}
	if minDist <= thresh {
		return top, left
	}
	if strong && log2t == 5 {
		thr := 1 << (8 - 5)
		if absInt(top[0]+top[64]-2*top[32]) < thr && absInt(left[0]+left[64]-2*left[32]) < thr {
			ft := make([]int, len(top))
			fl := make([]int, len(left))
			copy(fl, left)
			ft[0] = top[0]
			ft[64] = top[64]
			for i := 0; i < 63; i++ {
				ft[1+i] = ((64-(i+1))*top[0] + (i+1)*top[64] + 32) >> 6
				fl[1+i] = ((64-(i+1))*left[0] + (i+1)*left[64] + 32) >> 6
			}
			return ft, fl
		}
	}
	n := 2 * size
	ft := make([]int, len(top))
	fl := make([]int, len(left))
	fl[n] = left[n]
	ft[n] = top[n]
	for i := n - 1; i >= 1; i-- {
		fl[i] = (left[i+1] + 2*left[i] + left[i-1] + 2) >> 2
		ft[i] = (top[i+1] + 2*top[i] + top[i-1] + 2) >> 2
	}
	c := (left[1] + 2*left[0] + top[1] + 2) >> 2
	fl[0], ft[0] = c, c
	return ft, fl
}

// predictIntra fills a size*size block from refs for one mode.
func predictIntra(out []byte, top, left []int, size, mode, cIdx int) {
	switch mode {
	case IntraPlanar:
		l2 := log2size(size)
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				out[y*size+x] = byte(((size-1-x)*left[1+y] + (x+1)*top[size+1] +
					(size-1-y)*top[1+x] + (y+1)*left[size+1] + size) >> uint(l2+1))
			}
		}
	case IntraDC:
		dc := size
		for i := 0; i < size; i++ {
			dc += left[1+i] + top[1+i]
		}
		dc >>= uint(log2size(size) + 1)
		for i := range out {
			out[i] = byte(dc)
		}
		if cIdx == 0 && size < 32 {
			out[0] = byte((left[1] + 2*dc + top[1] + 2) >> 2)
			for x := 1; x < size; x++ {
				out[x] = byte((top[1+x] + 3*dc + 2) >> 2)
			}
			for y := 1; y < size; y++ {
				out[y*size] = byte((left[1+y] + 3*dc + 2) >> 2)
			}
		}
	default:
		predictAngular(out, top, left, size, mode, cIdx)
	}
}

// predictAngular is the directional path (peer pred_angular shape:
// vertical modes project rows onto top, horizontal onto left).
func predictAngular(out []byte, top, left []int, size, mode, cIdx int) {
	angle := intraPredAngle[mode-2]
	if mode >= 18 {
		ref := make([]int, 8*size+9)
		base := 4*size + 4
		get := func(i int) int {
			if i < -base {
				return ref[0]
			}
			if i >= len(ref)-base {
				return ref[len(ref)-1]
			}
			return ref[base+i]
		}
		set := func(i, v int) {
			if i >= -base && i < len(ref)-base {
				ref[base+i] = v
			}
		}
		for i := -1; i <= 2*size; i++ {
			set(i, top[1+i])
		}
		last := (size * angle) >> 5
		if angle < 0 && last < -1 {
			for x := 0; x <= size; x++ {
				set(x-1, top[1+x-1])
			}
			// Peer extends its ref_tmp[last..-1]; our ref runs one slot
			// ahead of the peer's (ref[i] holds sample offset i, the
			// peer's ref[i] holds offset i-1), so the same projected
			// samples land at x-1 here.
			for x := last; x <= -1; x++ {
				set(x-1, left[0+((x*invAngle[mode-11]+128)>>8)])
			}
		}
		for y := 0; y < size; y++ {
			idx := ((y + 1) * angle) >> 5
			fact := ((y + 1) * angle) & 31
			for x := 0; x < size; x++ {
				if fact != 0 {
					a := get(x + idx)
					b := get(x + idx + 1)
					out[y*size+x] = byte(((32-fact)*a + fact*b + 16) >> 5)
				} else {
					out[y*size+x] = byte(get(x + idx))
				}
			}
		}
		if mode == 26 && cIdx == 0 && size < 32 {
			for y := 0; y < size; y++ {
				out[y*size] = clipPix(top[1] + ((left[1+y] - left[0]) >> 1))
			}
		}
		return
	}
	ref := make([]int, 8*size+9)
	base := 4*size + 4
	get := func(i int) int {
		if i < -base {
			return ref[0]
		}
		if i >= len(ref)-base {
			return ref[len(ref)-1]
		}
		return ref[base+i]
	}
	set := func(i, v int) {
		if i >= -base && i < len(ref)-base {
			ref[base+i] = v
		}
	}
	for i := -1; i <= 2*size; i++ {
		set(i, left[1+i])
	}
	last := (size * angle) >> 5
	if angle < 0 && last < -1 {
		for x := 0; x <= size; x++ {
			set(x-1, left[1+x-1])
		}
		// Same one-slot shift as the vertical path (see above).
		for x := last; x <= -1; x++ {
			set(x-1, top[0+((x*invAngle[mode-11]+128)>>8)])
		}
	}
	for x := 0; x < size; x++ {
		idx := ((x + 1) * angle) >> 5
		fact := ((x + 1) * angle) & 31
		for y := 0; y < size; y++ {
			if fact != 0 {
				a := get(y + idx)
				b := get(y + idx + 1)
				out[y*size+x] = byte(((32-fact)*a + fact*b + 16) >> 5)
			} else {
				out[y*size+x] = byte(get(y + idx))
			}
		}
	}
	if mode == 10 && cIdx == 0 && size < 32 {
		for x := 0; x < size; x++ {
			out[x] = clipPix(left[1] + ((top[1+x] - top[0]) >> 1))
		}
	}
}

// recon holds the working picture plus syntax for one I frame.
type recon struct {
	pic *Picture
	fs  *FrameSyntax
	tus map[tuKey]*TUInfo
}

type tuKey struct {
	x, y, log2, c int
}

// Reconstruct builds the pre-filter picture for an I-frame syntax:
// every leaf is predicted, residuals dequantized, transformed and
// added in decode order. Loop filters ride separately.
func Reconstruct(fs *FrameSyntax, s *SPS) (*Picture, error) {
	if fs == nil || fs.SH == nil {
		return nil, fmt.Errorf("%w: nil syntax", ErrBadSlice)
	}
	if s == nil {
		return nil, fmt.Errorf("%w: nil sps", ErrBadSPS)
	}
	pic := NewPicture(int(s.Width), int(s.Height))
	if pic == nil {
		return nil, fmt.Errorf("%w: bad size", ErrBadSPS)
	}
	r := &recon{pic: pic, fs: fs, tus: map[tuKey]*TUInfo{}}
	for i := range fs.TUs {
		t := &fs.TUs[i]
		r.tus[tuKey{t.X0, t.Y0, t.Log2Size, t.CIdx}] = t
	}
	for _, l := range fs.Leaves {
		if err := r.leaf(l, s); err != nil {
			return nil, err
		}
	}
	return pic, nil
}

// leaf reconstructs one transform leaf across its components.
func (r *recon) leaf(l TULeaf, s *SPS) error {
	pic := r.pic
	size := 1 << l.Log2Size
	// Luma always predicts (peer predicts before the cbf gate).
	pred := make([]byte, size*size)
	a := neighbours(l.X0, l.Y0, size, size, pic.Width, pic.Height, int(s.Log2MaxCB))
	top, left := refSet(pic.Y, pic.Width, pic.Width, pic.Height, l.X0, l.Y0, size, a)
	top, left = filterRefs(top, left, size, l.Log2Size, int(l.LumaMode), s.SmoothIntra)
	predictIntra(pred, top, left, size, int(l.LumaMode), 0)
	var res []int16
	if l.CbfLuma {
		t, ok := r.tus[tuKey{l.X0, l.Y0, l.Log2Size, 0}]
		if !ok {
			return fmt.Errorf("%w: missing luma TU %d,%d,%d", ErrBadSlice, l.X0, l.Y0, size)
		}
		res = append([]int16(nil), t.Coeffs...)
		inverseTransform(res, l.Log2Size, qpFor(0, t.QP), l.Log2Size == 2)
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			v := int(pred[y*size+x])
			if res != nil {
				v += int(res[y*size+x])
			}
			pic.Y[(l.Y0+y)*pic.Width+l.X0+x] = clipPix(v)
		}
	}
	// Chroma (4:2:0): big blocks share the TU origin, 4x4 luma blocks
	// share one chroma block at the CU origin on the last leaf.
	if s.ChromaFormat != 1 {
		return nil
	}
	cw := (pic.Width + 1) / 2
	chH := (pic.Height + 1) / 2
	if l.Log2Size > 2 {
		cs := size / 2
		px, py := l.X0/2, l.Y0/2
		ca := neighbours(l.X0, l.Y0, size, size, pic.Width, pic.Height, int(s.Log2MaxCB))
		for _, c := range []int{1, 2} {
			plane := pic.Cb
			if c == 2 {
				plane = pic.Cr
			}
			ctop, cleft := refSet(plane, cw, cw, chH, px, py, cs, ca)
			cp := make([]byte, cs*cs)
			predictIntra(cp, ctop, cleft, cs, int(l.ChromaMode), c)
			var cres []int16
			if (c == 1 && l.CbfCb) || (c == 2 && l.CbfCr) {
				t, ok := r.tus[tuKey{l.X0, l.Y0, l.Log2Size - 1, c}]
				if !ok {
					return fmt.Errorf("%w: missing chroma TU %d,%d,%d,c%d", ErrBadSlice, l.X0, l.Y0, size, c)
				}
				cres = append([]int16(nil), t.Coeffs...)
				inverseTransform(cres, l.Log2Size-1, qpFor(c, t.QP), false)
			}
			for y := 0; y < cs; y++ {
				for x := 0; x < cs; x++ {
					v := int(cp[y*cs+x])
					if cres != nil {
						v += int(cres[y*cs+x])
					}
					plane[(py+y)*cw+px+x] = clipPix(v)
				}
			}
		}
		return nil
	}
	if l.Log2Size == 2 && l.Blk == 3 {
		px, py := l.CbX/2, l.CbY/2
		ca := neighbours(l.CbX, l.CbY, 8, 8, pic.Width, pic.Height, int(s.Log2MaxCB))
		for _, c := range []int{1, 2} {
			plane := pic.Cb
			if c == 2 {
				plane = pic.Cr
			}
			ctop, cleft := refSet(plane, cw, cw, chH, px, py, 4, ca)
			cp := make([]byte, 16)
			predictIntra(cp, ctop, cleft, 4, int(l.ChromaMode), c)
			var cres []int16
			if (c == 1 && l.CbfCb) || (c == 2 && l.CbfCr) {
				t, ok := r.tus[tuKey{l.CbX, l.CbY, 2, c}]
				if !ok {
					return fmt.Errorf("%w: missing small chroma TU %d,%d,c%d", ErrBadSlice, l.CbX, l.CbY, c)
				}
				cres = append([]int16(nil), t.Coeffs...)
				inverseTransform(cres, 2, qpFor(c, t.QP), false)
			}
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					v := int(cp[y*4+x])
					if cres != nil {
						v += int(cres[y*4+x])
					}
					plane[(py+y)*cw+px+x] = clipPix(v)
				}
			}
		}
	}
	return nil
}
