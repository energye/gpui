package h264

import (
	"errors"
	"fmt"
)

// ErrStageScope marks a valid stream that belongs to a later VR2 stage
// (non-Baseline档位, P/B slices, 8x8 transform). It is never a corruption
// report: callers turn it into a readable "not yet" message.
var ErrStageScope = errors.New("h264: valid stream beyond this stage scope")

// CBP map for Intra4x4 ue values (Table 9-4 facts).
var golombToIntra4x4CBP = [48]uint8{
	47, 31, 15, 0, 23, 27, 29, 30, 7, 11, 13, 14, 39, 43, 45, 46,
	16, 3, 5, 10, 12, 19, 21, 26, 28, 35, 37, 42, 44, 1, 2, 4,
	8, 17, 18, 20, 24, 6, 9, 22, 25, 32, 33, 34, 36, 40, 38, 41,
}

// I16x16 luma prediction numbering follows the spec (Table 7-11 facts:
// Vertical=0, Horizontal=1, DC=2, Plane=3), so mb_type runs V/H/DC/Plane
// in order within each group.
var i16PredCycle = [4]int{0, 1, 2, 3}

// I16x16 chroma CBP cycle per mb_type group (Table 7-11 facts).
var i16ChromaCycle = [6]uint32{0, 16, 32, 0, 16, 32}

const (
	mbI4x4 = iota
	mbI16x16
	mbIPCM
)

// Decoder decodes I and P slices (VR2a+VR2b scope) into pictures.
// Feed every NALU of a frame in stream order; params collect on the way.
type Decoder struct {
	ps       *ParamSets
	dpb      *DPB
	pic      *Picture
	sps      *SPS
	mbW      int
	mbH      int
	nnzY     []int8
	nnzCb    []int8
	nnzCr    []int8
	modes    []int8
	qps      []int32
	fIDC     []uint32
	fA       []int32
	fB       []int32
	mvX      []int16
	mvY      []int16
	refIdx   []int8
	mbIntra  []bool
	skipRun  int
	refPic   *Picture
	refList  []*Picture
	skipCnt  int
	cOff0    int32
	cOff1    int32
	qpY      int32
	decoded  int
	slices   int
	curIsRef bool
}

// NewDecoder builds a decoder over shared parameter sets.
func NewDecoder(ps *ParamSets) *Decoder {
	if ps == nil {
		ps = NewParamSets()
	}
	return &Decoder{ps: ps, dpb: NewDPB(16)}
}

// DecodeNALU feeds one NALU: parameter sets collect, slices decode.
func (d *Decoder) DecodeNALU(nalu []byte) error {
	t, ok := NALType(nalu)
	if !ok {
		return fmt.Errorf("%w: empty unit", ErrBadNALU)
	}
	switch t {
	case NALSPS, NALPPS:
		_, err := d.ps.AddNALU(nalu)
		return err
	case NALSliceNonIDR, NALSliceIDR:
		return d.decodeSlice(nalu)
	default:
		return nil
	}
}

func peekSlicePPSID(nalu []byte) (uint32, error) {
	r := NewReader(UnescapeRBSP(nalu[1:]))
	for i := 0; i < 2; i++ {
		if _, err := r.ReadUE(); err != nil {
			return 0, err
		}
	}
	return r.ReadUE()
}

func (d *Decoder) decodeSlice(nalu []byte) error {
	ppsID, err := peekSlicePPSID(nalu)
	if err != nil {
		return err
	}
	pps, sps, err := d.ps.RequireForSlice(ppsID)
	if err != nil {
		return err
	}
	if sps.ProfileIDC != 66 {
		return fmt.Errorf("%w: profile %s needs VR2c", ErrStageScope, sps.Profile)
	}
	h, r, err := ParseSliceHeader(nalu, pps, sps)
	if err != nil {
		return err
	}
	if h.IsB() {
		return fmt.Errorf("%w: B slice needs VR2d", ErrStageScope)
	}
	if d.pic == nil {
		if sps.Width%16 != 0 || sps.Height%16 != 0 {
			return fmt.Errorf("%w: cropped %dx%d needs VR2d", ErrStageScope, sps.Width, sps.Height)
		}
		pic, err := NewPicture(sps.Width, sps.Height)
		if err != nil {
			return err
		}
		d.pic = pic
		d.sps = sps
		d.mbW, d.mbH = int(sps.Width/16), int(sps.Height/16)
		n4 := d.mbW * 4 * d.mbH * 4
		d.nnzY = make([]int8, n4)
		for i := range d.nnzY {
			d.nnzY[i] = -1
		}
		nc := d.mbW * 2 * d.mbH * 2
		d.nnzCb = make([]int8, nc)
		d.nnzCr = make([]int8, nc)
		for i := range d.nnzCb {
			d.nnzCb[i], d.nnzCr[i] = -1, -1
		}
		d.modes = make([]int8, n4)
		for i := range d.modes {
			d.modes[i] = -1
		}
		d.qps = make([]int32, d.mbW*d.mbH)
		d.fIDC = make([]uint32, d.mbW*d.mbH)
		d.fA = make([]int32, d.mbW*d.mbH)
		d.fB = make([]int32, d.mbH*d.mbW)
		d.mvX = make([]int16, n4)
		d.mvY = make([]int16, n4)
		d.refIdx = make([]int8, n4)
		for i := range d.refIdx {
			d.refIdx[i] = -1
		}
		d.mbIntra = make([]bool, d.mbW*d.mbH)
	}
	// New picture starts at FirstMB 0: reset per-frame motion state.
	// Multi-slice frames keep accumulating; slices all share the arrays.
	if h.FirstMB == 0 && d.decoded == 0 {
		for i := range d.refIdx {
			d.refIdx[i] = -1
		}
		for i := range d.modes {
			d.modes[i] = -1
		}
		for i := range d.mbIntra {
			d.mbIntra[i] = false
		}
		d.skipCnt = 0
	}
	d.qpY = 26 + pps.PicInitQP + h.QPDelta
	d.cOff0 = pps.ChromaQPOffset
	d.cOff1 = pps.ChromaQPOffset + pps.SecondChromaQPOffset
	d.pic.FrameNum = h.FrameNum
	d.pic.POC = h.POC
	d.pic.IsIDR = h.IsIDR
	d.curIsRef = h.NalRefIDC != 0
	// Reference list 0: newest stored pictures first (no reordering in
	// this stage; modification flags parsed but untreated streams fail
	// readable elsewhere). Multi-ref clips address older entries by idx.
	d.refPic = nil
	d.refList = d.refList[:0]
	if h.IsP() {
		if h.IsIDR {
			// IDR P is still a refresh: no reference needed.
		} else {
			n := int(h.RefL0Count)
			if n < 1 {
				n = 1
			}
			d.refList = d.dpb.List0(n)
			if len(d.refList) > 0 {
				d.refPic = d.refList[0]
			}
		}
		if d.refPic == nil && !h.IsIDR && d.dpb.Len() == 0 {
			// First frame must be IDR; P with no reference is corrupt.
			return fmt.Errorf("%w: P slice without reference", ErrBadSliceHeader)
		}
	}
	d.skipRun = -1
	total := d.mbW * d.mbH
	addr := int(h.FirstMB)
	for addr < total {
		// Pending skips consume no bits.
		if h.IsP() && d.skipRun > 0 {
			if err := d.decodeSkip(h, addr, r); err != nil {
				return fmt.Errorf("mb %d: %w", addr, err)
			}
			d.decoded++
			d.skipRun--
			addr++
			continue
		}
		if h.IsP() && d.skipRun < 0 {
			if !r.MoreRBSPData() && r.BitsLeft() <= 8 {
				break
			}
			run, err := r.ReadUE()
			if err != nil {
				break
			}
			d.skipRun = int(run)
			if d.skipRun > 0 {
				continue
			}
		}
		if !r.MoreRBSPData() && r.BitsLeft() <= 8 && !(h.IsP() && d.skipRun == 0) {
			// I slices keep the old trailing guard; P with zero run
			// still owns one coded MB in these bits.
			if !h.IsP() {
				break
			}
		}
		var err error
		if h.IsI() {
			err = d.decodeMB(r, pps, h, addr)
		} else {
			err = d.decodeMBP(r, pps, h, addr)
		}
		if err != nil {
			return fmt.Errorf("mb %d: %w", addr, err)
		}
		d.decoded++
		if h.IsP() {
			d.skipRun = -1
		}
		addr++
	}
	d.slices++
	return nil
}

// FinishPicture deblocks, stores to the DPB and hands over the picture.
// The frame must be exactly covered by decoded macroblocks.
func (d *Decoder) FinishPicture() (*Picture, error) {
	if d.pic == nil || d.decoded == 0 {
		return nil, fmt.Errorf("%w: no slices decoded", ErrBadSliceHeader)
	}
	if d.decoded != d.mbW*d.mbH {
		return nil, fmt.Errorf("%w: %d of %d mbs", ErrBadSliceHeader, d.decoded, d.mbW*d.mbH)
	}
	DeblockPicture(d.pic, d.qps, d.fIDC, d.fA, d.fB, d.mbW, d.mbH, d.cOff0, d.cOff1,
		d.mbIntra, d.nnzY, d.mvX, d.mvY, d.refIdx)
	d.dpb.Store(d.pic, d.curIsRef)
	out := d.pic
	d.pic = nil
	d.decoded = 0
	d.slices = 0
	return out, nil
}

func (d *Decoder) nnzAt(grid []int8, stride, x, y int) int {
	if x < 0 || y < 0 || x >= stride {
		return -1
	}
	return int(grid[y*stride+x])
}

func (d *Decoder) setNnz(grid []int8, stride, x, y, v int) {
	grid[y*stride+x] = int8(v)
}

// neighbourCount derives nC for one 4x4 block from left/up TotalCoeffs.
func neighbourCount(left, up int) int {
	switch {
	case left >= 0 && up >= 0:
		return (left + up + 1) >> 1
	case left >= 0:
		return left
	case up >= 0:
		return up
	default:
		return 0
	}
}

func (d *Decoder) decodeMB(r *Reader, pps *PPS, h *SliceHeader, addr int) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	mbType, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("mb type: %w", err)
	}
	if mbType > 25 {
		return fmt.Errorf("%w: mb type %d in I slice", ErrBadSliceHeader, mbType)
	}
	if mbType == 25 {
		return d.decodePCM(r, mbx, mby)
	}
	return d.decodeIntraMB(r, pps, h, addr, mbx, mby, mbType)
}

func (d *Decoder) decodeIntraMB(r *Reader, pps *PPS, h *SliceHeader, addr, mbx, mby int, mbType uint32) error {
	kind := mbI4x4
	pred16 := 0
	cbp := uint32(0)
	i16 := mbType != 0
	if i16 {
		kind = mbI16x16
		pred16 = i16PredCycle[(mbType-1)%4]
		group := (mbType - 1) / 4
		if mbType > 12 {
			cbp = 15
		}
		cbp |= i16ChromaCycle[group]
	}
	var modes [16]int
	if kind == mbI4x4 {
		stride := d.mbW * 4
		// Modes follow luma4x4BlkIdx (8x8-grouped), not raster.
		for _, b := range [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15} {
			bx, by := mbx*4+b%4, mby*4+b/4
			m, err := d.intra4x4Mode(r, bx, by)
			if err != nil {
				return err
			}
			modes[b] = m
			// Publish immediately: later blocks derive their
			// most-probable mode from already-parsed neighbours.
			d.modes[by*stride+bx] = int8(m)
		}
	}
	chromaModeUE, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("chroma mode: %w", err)
	}
	if chromaModeUE > 3 {
		if clampIV {
			chromaModeUE = 0
		} else {
			return fmt.Errorf("%w: chroma mode %d", ErrBadSliceHeader, chromaModeUE)
		}
	}
	// ue values run DC/H/V/Plane; internal consts run V/H/DC/Plane.
	chromaMode := [4]uint32{2, 1, 0, 3}[chromaModeUE]
	if err := checkChromaMode(int(chromaMode), mbx, mby); err != nil {
		return err
	}
	if !i16 {
		cbpUE, err := r.ReadUE()
		if err != nil {
			return fmt.Errorf("cbp: %w", err)
		}
		if cbpUE > 47 {
			return fmt.Errorf("%w: cbp %d", ErrBadSliceHeader, cbpUE)
		}
		cbp = uint32(golombToIntra4x4CBP[cbpUE])
	}
	if cbp != 0 || i16 {
		delta, err := r.ReadSE()
		if err != nil {
			return fmt.Errorf("qp delta: %w", err)
		}
		d.qpY += delta
		for d.qpY < 0 {
			d.qpY += 52
		}
		for d.qpY > 51 {
			d.qpY -= 52
		}
	}
	d.qps[addr] = d.qpY
	d.fIDC[addr] = h.DisableFilter
	d.fA[addr] = h.FilterAlpha
	d.fB[addr] = h.FilterBeta
	if d.mbIntra != nil {
		d.mbIntra[addr] = true
		d.markIntraMB(mbx, mby)
	}
	if kind == mbI16x16 {
		return d.reconstructI16x16(r, mbx, mby, pred16, int(chromaMode), cbp)
	}
	return d.reconstructI4x4(r, mbx, mby, modes, int(chromaMode), cbp)
}

// intra4x4Mode reads one block prediction mode with most-probable rule.
func (d *Decoder) intra4x4Mode(r *Reader, bx, by int) (int, error) {
	stride := d.mbW * 4
	a, b := -1, -1
	if bx > 0 {
		a = int(d.modes[by*stride+bx-1])
	}
	if by > 0 {
		b = int(d.modes[(by-1)*stride+bx])
	}
	pred := 2
	if a >= 0 && b >= 0 {
		if a < b {
			pred = a
		} else {
			pred = b
		}
	}
	flag, err := r.ReadBits(1)
	if err != nil {
		return 0, fmt.Errorf("pred flag: %w", err)
	}
	if flag != 0 {
		return pred, nil
	}
	rem, err := r.ReadBits(3)
	if err != nil {
		return 0, fmt.Errorf("rem mode: %w", err)
	}
	m := int(rem)
	if m >= pred {
		m++
	}
	return m, nil
}

func (d *Decoder) decodePCM(r *Reader, mbx, mby int) error {
	r.AlignToByte()
	raw, err := r.ReadBytes(384)
	if err != nil {
		return fmt.Errorf("pcm: %w", err)
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			d.pic.SetY(uint32(mbx*16+x), uint32(mby*16+y), raw[y*16+x])
		}
	}
	for comp := 0; comp < 2; comp++ {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				v := raw[256+comp*64+y*8+x]
				cx, cy := uint32(mbx*8+x), uint32(mby*8+y)
				if comp == 0 {
					d.pic.Cb[cy*d.pic.Width/2+cx] = v
				} else {
					d.pic.Cr[cy*d.pic.Width/2+cx] = v
				}
			}
		}
	}
	stride := d.mbW * 4
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			d.modes[(mby*4+y)*stride+mbx*4+x] = -1
			d.setNnz(d.nnzY, stride, mbx*4+x, mby*4+y, 16)
		}
	}
	cstride := d.mbW * 2
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			d.setNnz(d.nnzCb, cstride, mbx*2+x, mby*2+y, 16)
			d.setNnz(d.nnzCr, cstride, mbx*2+x, mby*2+y, 16)
		}
	}
	d.qps[mby*d.mbW+mbx] = 0
	if d.mbIntra != nil {
		d.mbIntra[mby*d.mbW+mbx] = true
		d.markIntraMB(mbx, mby)
	}
	return nil
}

func checkChromaMode(m, mbx, mby int) error {
	switch m {
	case IntraPredDC:
		return nil
	case IntraPredHorizontal:
		if mbx == 0 {
			return fmt.Errorf("%w: chroma h at left edge", ErrBadSliceHeader)
		}
	case IntraPredVertical:
		if mby == 0 {
			return fmt.Errorf("%w: chroma v at top edge", ErrBadSliceHeader)
		}
	case IntraPredPlane:
		if mbx == 0 || mby == 0 {
			return fmt.Errorf("%w: chroma plane at edge", ErrBadSliceHeader)
		}
	}
	return nil
}

func (d *Decoder) lumaSampler() sampler {
	W, H := int32(d.pic.Width), int32(d.pic.Height)
	return func(x, y int32) (uint8, bool) {
		if x < 0 || y < 0 || x >= W || y >= H {
			return 0, false
		}
		return d.pic.Y[y*W+x], true
	}
}

// lumaBlockSampler gates neighbours by decode order: future blocks read
// as unavailable so DDL/VL fall back instead of sampling zeros.
func (d *Decoder) lumaBlockSampler(curBx, curBy int) sampler {
	W, H := int32(d.pic.Width), int32(d.pic.Height)
	return func(x, y int32) (uint8, bool) {
		if x < 0 || y < 0 || x >= W || y >= H {
			return 0, false
		}
		if !d.lumaBlockDecoded(int(x/4), int(y/4), curBx, curBy) {
			return 0, false
		}
		return d.pic.Y[y*W+x], true
	}
}

// lumaBlockDecoded reports whether 4x4 block (xb,yb) is decoded before
// current block (curBx,curBy): MB raster, within MB 8x8-grouped.
func (d *Decoder) lumaBlockDecoded(xb, yb, curBx, curBy int) bool {
	mbx, mby := curBx/4, curBy/4
	nMBx, nMBy := xb/4, yb/4
	switch {
	case nMBy < mby:
		return true
	case nMBy > mby:
		return false
	case nMBx < mbx:
		return true
	case nMBx > mbx:
		return false
	}
	rank := func(b int) int {
		for i, v := range [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15} {
			if v == b {
				return i
			}
		}
		return 99
	}
	curB := (curBy%4)*4 + (curBx % 4)
	nB := (yb%4)*4 + (xb % 4)
	return rank(nB) < rank(curB)
}

func (d *Decoder) chromaSampler(plane []uint8) sampler {
	W, H := int32(d.pic.Width/2), int32(d.pic.Height/2)
	return func(x, y int32) (uint8, bool) {
		if x < 0 || y < 0 || x >= W || y >= H {
			return 0, false
		}
		return plane[y*W+x], true
	}
}

func (d *Decoder) blockNC(grid []int8, stride, bx, by int) int {
	return neighbourCount(d.nnzAt(grid, stride, bx-1, by), d.nnzAt(grid, stride, bx, by-1))
}

func (d *Decoder) reconstructI4x4(r *Reader, mbx, mby int, modes [16]int, chromaMode int, cbp uint32) error {
	stride := d.mbW * 4
	// 4x4 blocks are read 8x8-grouped (each spatial 8x8's four blocks in
	// a row), matching the residual() syntax order.
	for _, b := range [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15} {
		bx, by := mbx*4+b%4, mby*4+b/4
		get := d.lumaBlockSampler(bx, by)
		pred, err := PredIntra4x4(get, int32(bx*4), int32(by*4), modes[b])
		if err != nil {
			return err
		}
		var coeff [16]int32
		tc := 0
		i8 := (b/8)*2 + (b%4)/2
		if cbp&(1<<uint(i8)) != 0 {
			nC := d.blockNC(d.nnzY, stride, bx, by)
			c, total, err := d.readLumaBlock(r, nC)
			if err != nil {
				return err
			}
			coeff, tc = c, total
		}
		res := ITransform4x4(coeff, uint32(d.qpY))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				v := int32(pred[y*4+x]) + res[y*4+x]
				d.pic.SetY(uint32(bx*4+x), uint32(by*4+y), clipPixel(v))
			}
		}
		d.modes[by*stride+bx] = int8(modes[b])
		d.setNnz(d.nnzY, stride, bx, by, tc)
	}
	return d.reconstructChroma(r, mbx, mby, chromaMode, (cbp>>4)&3)
}

func (d *Decoder) readLumaBlock(r *Reader, nC int) ([16]int32, int, error) {
	coeff, err := DecodeResidualBlock(r, SelectTable(nC), 16, 0, false)
	if err != nil {
		return coeff, 0, err
	}
	tc := 0
	for _, v := range coeff {
		if v != 0 {
			tc++
		}
	}
	return coeff, tc, nil
}

func (d *Decoder) reconstructI16x16(r *Reader, mbx, mby, pred16, chromaMode int, cbp uint32) error {
	get := d.lumaSampler()
	pred, err := PredIntra16x16(get, int32(mbx*16), int32(mby*16), pred16)
	if err != nil {
		return err
	}
	stride := d.mbW * 4
	nCDC := d.blockNC(d.nnzY, stride, mbx*4, mby*4)
	dcRaw, err := DecodeResidualBlock(r, SelectTable(nCDC), 16, 0, false)
	if err != nil {
		return fmt.Errorf("luma dc: %w", err)
	}
	dcScaled := ITransformLumaDC(dcRaw, uint32(d.qpY))
	dcBlk := dcScaled
	// AC blocks follow luma4x4BlkIdx (8x8-grouped), like Intra4x4.
	for _, b := range [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15} {
		var coeff [16]int32
		acNZ := 0
		if cbp&15 != 0 {
			bx, by := mbx*4+b%4, mby*4+b/4
			nC := d.blockNC(d.nnzY, stride, bx, by)
			ac, err := DecodeResidualBlock(r, SelectTable(nC), 15, 1, false)
			if err != nil {
				return fmt.Errorf("luma ac %d: %w", b, err)
			}
			coeff = ac
			for _, v := range ac {
				if v != 0 {
					acNZ++
				}
			}
		}
		res := ITransform4x4WithDC(coeff, dcBlk[b], uint32(d.qpY))
		bx, by := mbx*4+b%4, mby*4+b/4
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				v := int32(pred[((by-mby*4)*4+y)*16+(bx-mbx*4)*4+x]) + res[y*4+x]
				d.pic.SetY(uint32(bx*4+x), uint32(by*4+y), clipPixel(v))
			}
		}
		d.modes[by*stride+bx] = 2
		d.setNnz(d.nnzY, stride, bx, by, acNZ)
	}
	return d.reconstructChroma(r, mbx, mby, chromaMode, (cbp>>4)&3)
}

func (d *Decoder) reconstructChroma(r *Reader, mbx, mby, chromaMode int, cbpC uint32) error {
	planes := [2][]uint8{d.pic.Cb, d.pic.Cr}
	var preds [2][64]uint8
	var err error
	for comp := 0; comp < 2; comp++ {
		get := d.chromaSampler(planes[comp])
		preds[comp], err = PredIntraChroma(get, int32(mbx*8), int32(mby*8), chromaMode)
		if err != nil {
			return err
		}
	}
	// Bitstream order is Cb DC, Cr DC, then Cb AC blocks, Cr AC blocks.
	return d.reconstructChromaBlocks(r, mbx, mby, cbpC, &preds[0], &preds[1])
}

// reconstructChromaBlocks reads chroma residual against caller-supplied
// prediction and adds it into the picture. Both intra and inter macroblocks
// share this tail; only the prediction source differs.
func (d *Decoder) reconstructChromaBlocks(r *Reader, mbx, mby int, cbpC uint32, predCb, predCr *[64]uint8) error {
	planes := [2][]uint8{d.pic.Cb, d.pic.Cr}
	preds := [2]*[64]uint8{predCb, predCr}
	grids := [2][]int8{d.nnzCb, d.nnzCr}
	cstride := d.mbW * 2
	qps := [2]int32{
		ChromaQP(d.qpY, d.cOff0),
		ChromaQP(d.qpY, d.cOff1),
	}
	var dcRs [2][4]int32
	if cbpC > 0 {
		for comp := 0; comp < 2; comp++ {
			dcRaw, err := DecodeResidualBlock(r, 0, 4, 0, true)
			if err != nil {
				return fmt.Errorf("chroma dc: %w", err)
			}
			var dcArr [4]int32
			copy(dcArr[:], dcRaw[:4])
			dcRs[comp] = ITransformChromaDC(dcArr, uint32(qps[comp]))
		}
	}
	for comp := 0; comp < 2; comp++ {
		pred := preds[comp]
		dcR := dcRs[comp]
		for b := 0; b < 4; b++ {
			var coeff [16]int32
			acNZ := 0
			if cbpC == 2 {
				bx, by := mbx*2+b%2, mby*2+b/2
				nC := d.blockNC(grids[comp], cstride, bx, by)
				ac, err := DecodeResidualBlock(r, SelectTable(nC), 15, 1, false)
				if err != nil {
					return fmt.Errorf("chroma ac c%d b%d nC=%d: %w", comp, b, nC, err)
				}
				coeff = ac
				for _, v := range ac {
					if v != 0 {
						acNZ++
					}
				}
			}
			res := ITransform4x4WithDC(coeff, dcR[b], uint32(qps[comp]))
			bx, by := mbx*2+b%2, mby*2+b/2
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					v := int32(pred[((by-mby*2)*4+y)*8+(bx-mbx*2)*4+x]) + res[y*4+x]
					px, py := uint32(bx*4+x), uint32(by*4+y)
					planes[comp][py*d.pic.Width/2+px] = clipPixel(v)
				}
			}
			d.setNnz(grids[comp], cstride, bx, by, acNZ)
		}
	}
	return nil
}
