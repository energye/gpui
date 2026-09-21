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
	ps      *ParamSets
	dpb     *DPB
	pic     *Picture
	sps     *SPS
	mbW     int
	mbH     int
	nnzY    []int8
	nnzCb   []int8
	nnzCr   []int8
	modes   []int8
	qps     []int32
	fIDC    []uint32
	fA      []int32
	fB      []int32
	mvX     []int16
	mvY     []int16
	refIdx  []int8
	mbIntra []bool
	// Second-list motion for B slices, shadowing the list-0 arrays.
	// useM marks which lists each 4x4 uses (bit0 L0, bit1 L1); intra
	// and empty slots read zero.
	mvX1    []int16
	mvY1    []int16
	refIdx1 []int8
	mvdX1   []int16
	mvdY1   []int16
	refTmp1 []int8
	useM    []uint8
	// direct4 marks 4x4 blocks with direct-derived motion (CABAC
	// reference contexts exclude them); mbDirect marks whole B MBs
	// decoded as skip or direct (B mb_type contexts read it).
	direct4  []bool
	mbDirect []bool
	skipRun  int
	refPic   *Picture
	refList  []*Picture
	// lastStored is the aligned picture just stored to the DPB by
	// FinishPicture (nil when the frame was disposable). The returned
	// display picture may be a cropped copy; parallel workers share
	// lastStored (never mutated afterwards) as the reference input.
	// Read-only after FinishPicture; PrimeFrame tasks publish through
	// it. Nil outside a completed reference frame.
	lastStored *Picture
	// refList1 holds list 1 for B slices (nil outside B); refFor keeps
	// serving list 0 so P behaviour is untouched.
	refList1 []*Picture
	skipCnt  int
	cOff0    int32
	cOff1    int32
	qpY      int32
	// Explicit weighted prediction factors for the current slice
	// (list 0 always; list 1 for B slices in explicit mode). Identity
	// when the table signals defaults. wBiExpl selects the explicit
	// bipred formula; implicit time weights otherwise.
	wDenomL int32
	wDenomC int32
	wW0     [32]int32
	wO0     [32]int32
	wWC0    [32][2]int32
	wOC0    [32][2]int32
	wW1     [32]int32
	wO1     [32]int32
	wWC1    [32][2]int32
	wOC1    [32][2]int32
	wBiExpl bool
	// Effective scaling weights for the current slice (F9): six 4x4
	// lists (intra-Y/Cb/Cr, inter-Y/Cb/Cr) and two 8x8 luma lists
	// (intra, inter), all in raster order. Flat 16s without matrices.
	sc4      [6][16]uint8
	sc8      [2][64]uint8
	decoded  int
	slices   int
	curIsRef bool
	curIsB   bool
	// lastSEI holds the most recent supplemental messages (timing and
	// user data ride along for the player; decoding never depends on
	// them).
	lastSEI []SEIMessage
	// pocMSB/pocPrevLSB track the display-order high bits for poc type 0
	// wrapping (spec 8.2.1.1); pocHave is set by the first reference pic.
	pocMSB     int32
	pocPrevLSB int32
	pocHave    bool
	// fnOffset/fnPrev accumulate frame_num wraps for poc type 1/2
	// (spec 8.2.1.2/8.2.1.3: abs = offset + frame_num, bumped when the
	// number goes backwards). Reset by IDR.
	fnOffset int64
	fnPrev   uint32
	fnHave   bool
	// CABAC state (VR2c): arithmetic decoder, packed contexts, and the
	// per-MB side data that neighbour-dependent contexts read.
	cab     *cabacDec
	cabCtx  [1024]uint8
	refTmp  []int8
	skipped []bool
	// uniSkip marks macroblocks whose 16 4x4 motion slots are provably
	// identical (skip-uniform decode: B direct uniform in skipBUniform,
	// P skip 16x16 in decodeSkip), with the shared motion in uniDM.
	// The deblocker reads these (never writes): uniform + clean edges
	// share one zero verdict instead of four segment verdicts (S1b-E).
	// Reset per frame with skipped[]; sized with the MB arrays below.
	uniSkip []bool
	uniDM   [][2]bDirectMV
	mbI16   []bool
	cmode   []uint8
	cbpArr  []uint16
	mvdX    []int16
	mvdY    []int16
	mbSlice []int
	mbT8    []bool
	lastQPD int32
	sliceQP int32
	// lastQPDHit records whether the current macroblock decoded an
	// mb_qp_delta; untouched blocks reset lastQPD instead of inheriting
	// a stale delta.
	lastQPDHit bool
	// cabByte0 records the slice-payload byte offset backing the live
	// CABAC engine, so the PCM path can hand the byte reader back over.
	cabByte0 int
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
	case NALSei:
		var vui *VUI
		if d.sps != nil {
			vui = d.sps.VUI
		}
		msgs, err := ParseSEI(nalu, vui)
		if err != nil {
			return err
		}
		d.lastSEI = msgs
		return nil
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
	if sps.ProfileIDC != 66 && sps.ProfileIDC != 77 && sps.ProfileIDC != 100 {
		return fmt.Errorf("%w: profile %s needs a later stage", ErrStageScope, sps.Profile)
	}
	h, r, err := ParseSliceHeader(nalu, pps, sps)
	if err != nil {
		return err
	}
	if h.RedundantCnt != 0 {
		return fmt.Errorf("%w: redundant %d", ErrRedundantPic, h.RedundantCnt)
	}
	if h.FieldPic {
		which := "top"
		if h.BottomField {
			which = "bottom"
		}
		return fmt.Errorf("%w: F12 interlace field picture (%s field) needs field decoding", ErrStageScope, which)
	}
	if !sps.FrameMBsOnly {
		// Any frame inside a field-capable sequence needs field-aware
		// handling (MBAFF macroblocks or field-pair geometry) the
		// progressive engine does not implement: refuse readably
		// instead of decoding wrong pixels silently.
		if sps.MBAFF {
			return fmt.Errorf("%w: F12 interlace MBAFF frame needs field-aware decoding", ErrStageScope)
		}
		return fmt.Errorf("%w: F12 interlace sequence frame needs field-aware decoding", ErrStageScope)
	}
	d.fixPOC(h, sps)
	d.fixPOCType12(h, sps)
	if d.pic == nil {
		aw, ah := sps.AlignedWidth, sps.AlignedHeight
		if aw == 0 || ah == 0 {
			aw, ah = sps.Width, sps.Height
		}
		if aw%16 != 0 || ah%16 != 0 {
			return fmt.Errorf("%w: aligned %dx%d", ErrBadSPS, aw, ah)
		}
		mbW, mbH := int(aw/16), int(ah/16)
		n4 := mbW * 4 * mbH * 4
		nc := mbW * 2 * mbH * 2
		nmb := mbW * mbH
		// S1b-H: reuse decoder scratch across frames (same resolution).
		// Fresh decoders (or resolution changes) allocate; steady frames
		// only allocate the picture. Scratch arrays are per-decoder
		// private (never shared across B workers), so reuse is safe:
		// every MB overwrites its nnz/modes/qps/motion slots before any
		// neighbour reads them (raster order), and the per-frame reset
		// below clears refIdx/useM/direct4/mb marks. refTmp/refTmp1 get
		// zeroed in the reset block (fresh allocation zeroes them; reuse
		// must match — CABAC early-scratch reads current-MB slots).
		reuse := d.mbW == mbW && d.mbH == mbH &&
			d.nnzY != nil && len(d.nnzY) == n4 &&
			d.nnzCb != nil && len(d.nnzCb) == nc &&
			d.skipped != nil && len(d.skipped) == nmb &&
			d.mvX != nil && len(d.mvX) == n4
		pic, err := NewPicture(aw, ah)
		if err != nil {
			return err
		}
		d.pic = pic
		d.sps = sps
		d.mbW, d.mbH = mbW, mbH
		if !reuse {
			d.nnzY = make([]int8, n4)
			for i := range d.nnzY {
				d.nnzY[i] = -1
			}
			d.nnzCb = make([]int8, nc)
			d.nnzCr = make([]int8, nc)
			for i := range d.nnzCb {
				d.nnzCb[i], d.nnzCr[i] = -1, -1
			}
			d.modes = make([]int8, n4)
			for i := range d.modes {
				d.modes[i] = -1
			}
			d.qps = make([]int32, nmb)
			d.fIDC = make([]uint32, nmb)
			d.fA = make([]int32, nmb)
			d.fB = make([]int32, nmb)
			d.mvX = make([]int16, n4)
			d.mvY = make([]int16, n4)
			d.refIdx = make([]int8, n4)
			for i := range d.refIdx {
				d.refIdx[i] = -1
			}
			d.mvX1 = make([]int16, n4)
			d.mvY1 = make([]int16, n4)
			d.refIdx1 = make([]int8, n4)
			for i := range d.refIdx1 {
				d.refIdx1[i] = -1
			}
			d.mvdX1 = make([]int16, n4)
			d.mvdY1 = make([]int16, n4)
			d.useM = make([]uint8, n4)
			d.direct4 = make([]bool, n4)
			// Early reference scratch for CABAC contexts; zero reads as
			// ref 0 (never greater than zero), so no fill is needed.
			d.refTmp = make([]int8, n4)
			d.refTmp1 = make([]int8, n4)
			d.mbIntra = make([]bool, nmb)
			d.skipped = make([]bool, nmb)
			d.uniSkip = make([]bool, nmb)
			d.uniDM = make([][2]bDirectMV, nmb)
			d.mbDirect = make([]bool, nmb)
			d.mbI16 = make([]bool, nmb)
			d.cmode = make([]uint8, nmb)
			d.cbpArr = make([]uint16, nmb)
			d.mvdX = make([]int16, n4)
			d.mvdY = make([]int16, n4)
			d.mbSlice = make([]int, nmb)
			for i := range d.mbSlice {
				d.mbSlice[i] = -1
			}
			d.mbT8 = make([]bool, nmb)
		}
	}
	// New picture starts at FirstMB 0: reset per-frame motion state.
	// Multi-slice frames keep accumulating; slices all share the arrays.
	if h.FirstMB == 0 && d.decoded == 0 {
		for i := range d.refIdx {
			d.refIdx[i] = -1
		}
		for i := range d.refIdx1 {
			d.refIdx1[i] = -1
		}
		for i := range d.useM {
			d.useM[i] = 0
		}
		for i := range d.direct4 {
			d.direct4[i] = false
		}
		for i := range d.mbDirect {
			d.mbDirect[i] = false
		}
		for i := range d.modes {
			d.modes[i] = -1
		}
		for i := range d.mbIntra {
			d.mbIntra[i] = false
		}
		for i := range d.skipped {
			d.skipped[i] = false
			d.uniSkip[i] = false
			d.mbI16[i] = false
			d.cmode[i] = 0
			d.cbpArr[i] = 0
			d.mbSlice[i] = -1
		}
		for i := range d.mbT8 {
			d.mbT8[i] = false
		}
		// S1b-H: fresh allocation zeroes refTmp/refTmp1; reused scratch
		// must match (CABAC early-scratch reads current-MB slots that a
		// single-list block never writes).
		for i := range d.refTmp {
			d.refTmp[i] = 0
		}
		for i := range d.refTmp1 {
			d.refTmp1[i] = 0
		}
		// S1b-H: stale diagonals (poisonDiagSlots) and stale refs are
		// per-FRAME state, not per-MB: a reused frame inherits the
		// previous frame's diagonals/refs, and -2 (unavailable) is a
		// legal VALUE motion prediction reads (mvNeighbourL passes it
		// through as not-available, unlike -1 = intra-like). Fresh
		// allocation has zeros there (read as ref 0), so reuse must
		// clear -2s back to 0: scan refIdx/refIdx1 for poison
		// leftovers.
		//
		// S1b-H: intra modes of never-written slots are equally stale:
		// intraMostProbable reads mode slots of the ABOVE row (same
		// slice, any prior value counts — no staleness marker), so a
		// never-written slot above a coded intra block donates the OLD
		// frame's mode. Fresh allocation has -1 there (forced DC).
		//
		// S1b-H: residual presence (nnz grids) is the same story one
		// layer down: blockNC/cabacNNZAt read the LEFT/ABOVE slots'
		// stored counts (same slice, any value counts), so a stale
		// count picks the wrong VLC table / wrong CABAC context.
		// Fresh allocation has -1 everywhere (neighbourCount: no
		// neighbour; CABAC: missing marker).
		for i := range d.refIdx {
			if d.refIdx[i] == -2 {
				d.refIdx[i] = 0
			}
			if d.refIdx1[i] == -2 {
				d.refIdx1[i] = 0
			}
		}
		for i := range d.modes {
			d.modes[i] = -1
		}
		for i := range d.nnzY {
			d.nnzY[i] = -1
		}
		for i := range d.nnzCb {
			d.nnzCb[i] = -1
			d.nnzCr[i] = -1
		}
		// S1b-H: deblock params (qps/fIDC/fA/fB) are per-MB written at
		// finishPMB, and the deblocker clamps its qp reads — but a
		// never-written MB donates the OLD frame's qp/filter to edge
		// verdicts below. Fresh allocation has zeros (qp 0, filter
		// off). Reset the param layer.
		//
		// S1b-H: motion-difference history (mvd/mvd1) is the same story
		// for CABAC: cabacMVDAt reads neighbour slots' stored diffs
		// (same slice, any value counts), so a stale diff steers the
		// MVD context. Fresh allocation has zeros. Reset the diff
		// layer. (Vectors/refs/flags/modes/nnz covered above.)
		for i := range d.qps {
			d.qps[i] = 0
			d.fIDC[i] = 0
			d.fA[i] = 0
			d.fB[i] = 0
		}
		for i := range d.mvdX {
			d.mvdX[i] = 0
			d.mvdY[i] = 0
			d.mvdX1[i] = 0
			d.mvdY1[i] = 0
		}
		d.skipCnt = 0
	}
	d.qpY = 26 + pps.PicInitQP + h.QPDelta
	d.cOff0 = pps.ChromaQPOffset
	d.cOff1 = pps.ChromaQPOffset
	if pps.HasSecondChromaQP {
		d.cOff1 = pps.SecondChromaQPOffset
	}
	d.wDenomL, d.wDenomC = h.LumaDenom, h.ChromaDenom
	d.wW0, d.wO0 = h.LumaW0, h.LumaO0
	d.wWC0, d.wOC0 = h.ChromaW0, h.ChromaO0
	d.wW1, d.wO1 = h.LumaW1, h.LumaO1
	d.wWC1, d.wOC1 = h.ChromaW1, h.ChromaO1
	d.wBiExpl = h.IsB() && pps.WeightedBiPred == 1
	resolveScaling(pps, sps, &d.sc4, &d.sc8)
	d.pic.FrameNum = h.FrameNum
	d.pic.POC = h.POC
	d.pic.IsIDR = h.IsIDR
	d.curIsRef = h.NalRefIDC != 0
	d.curIsB = h.IsB()
	// Explicit marking runs before list construction (the new decode
	// consumes the lists shaped by it, like the reference decoder).
	if err := d.applyMarking(h); err != nil {
		return err
	}
	// Reference list 0: buffered pictures newest-first, reshaped by
	// the slice-header reordering steps. Multi-ref clips address
	// older or duplicated entries by index (weights follow the index).
	d.refPic = nil
	d.refList = d.refList[:0]
	d.refList1 = nil
	if h.IsP() {
		if h.IsIDR {
			// IDR P is still a refresh: no reference needed.
		} else {
			list, err := d.buildRefList0(h)
			if err != nil {
				return err
			}
			d.refList = list
			if len(d.refList) > 0 {
				d.refPic = d.refList[0]
			}
		}
		if d.refPic == nil && !h.IsIDR && d.dpb.Len() == 0 {
			// First frame must be IDR; P with no reference is corrupt.
			// F20 rides along so fault triage can name the tool.
			return fmt.Errorf("%w: P slice without reference (%w)", ErrBadSliceHeader, ErrLostReference)
		}
	}
	if h.IsB() {
		// Reference lists 0+1 in display order, reshaped by the
		// slice-header reordering steps (built here, after the reset).
		l0, l1, err := d.buildRefListsB(h)
		if err != nil {
			return err
		}
		d.refList, d.refList1 = l0, l1
		if len(l0) > 0 {
			d.refPic = l0[0]
		}
	}
	d.skipRun = -1
	total := d.mbW * d.mbH
	addr := int(h.FirstMB)
	if pps.EntropyCABAC {
		if err := d.cabacInitSlice(h, r, pps); err != nil {
			return err
		}
		return d.decodeSliceCabac(h, pps, r, addr, total)
	}
	d.cab = nil
	for addr < total {
		// Pending skips consume no bits.
		if (h.IsP() || h.IsB()) && d.skipRun > 0 {
			if h.IsB() {
				if err := d.decodeSkipB(h, addr, d.cavlcSrc(r)); err != nil {
					return fmt.Errorf("mb %d: %w", addr, err)
				}
			} else if err := d.decodeSkip(h, addr, d.cavlcSrc(r)); err != nil {
				return fmt.Errorf("mb %d: %w", addr, err)
			}
			d.decoded++
			d.skipRun--
			addr++
			continue
		}
		if (h.IsP() || h.IsB()) && d.skipRun < 0 {
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
		if !r.MoreRBSPData() && r.BitsLeft() <= 8 && !((h.IsP() || h.IsB()) && d.skipRun == 0) {
			// I slices keep the old trailing guard; P with zero run
			// still owns one coded MB in these bits.
			if !h.IsP() && !h.IsB() {
				break
			}
		}
		var err error
		if h.IsI() {
			err = d.decodeMB(r, pps, h, addr)
		} else if h.IsB() {
			err = d.decodeMBB(r, pps, h, addr)
		} else {
			err = d.decodeMBP(r, pps, h, addr)
		}
		if err != nil {
			return fmt.Errorf("mb %d: %w", addr, err)
		}
		d.decoded++
		if h.IsP() || h.IsB() {
			d.skipRun = -1
		}
		addr++
	}
	d.slices++
	return nil
}

// bDeblock* expose second-list motion to the deblocker for B pictures
// only; other pictures pass nils so the single-list path is untouched.
func (d *Decoder) bDeblockMVX1() []int16 {
	if d.curIsB {
		return d.mvX1
	}
	return nil
}

func (d *Decoder) bDeblockMVY1() []int16 {
	if d.curIsB {
		return d.mvY1
	}
	return nil
}

func (d *Decoder) bDeblockRef1() []int8 {
	if d.curIsB {
		return d.refIdx1
	}
	return nil
}

func (d *Decoder) bDeblockUseM() []uint8 {
	if d.curIsB {
		return d.useM
	}
	return nil
}

// LastSEI reports the most recent supplemental messages (nil when the
// stream carried none). The slice aliases decoder state: copy to keep.
func (d *Decoder) LastSEI() []SEIMessage {
	if d == nil {
		return nil
	}
	return d.lastSEI
}

// FinishPicture deblocks, stores to the DPB and hands over the picture.
// The frame must be exactly covered by decoded macroblocks.
// Empty or partial pictures are F20 (lost reference / truncated sample):
// the caller skips the frame and keeps playing.
func (d *Decoder) FinishPicture() (*Picture, error) {
	if d.pic == nil || d.decoded == 0 {
		return nil, fmt.Errorf("%w: no slices decoded (%w)", ErrBadSliceHeader, ErrLostReference)
	}
	if d.decoded != d.mbW*d.mbH {
		return nil, fmt.Errorf("%w: %d of %d mbs (%w)", ErrBadSliceHeader, d.decoded, d.mbW*d.mbH, ErrLostReference)
	}
	DeblockPicture(d.pic, d.qps, d.fIDC, d.fA, d.fB, d.mbW, d.mbH, d.cOff0, d.cOff1,
		d.mbIntra, d.nnzY, d.mvX, d.mvY, d.refIdx, d.refList, d.mbT8,
		d.bDeblockMVX1(), d.bDeblockMVY1(), d.bDeblockRef1(), d.refList1, d.bDeblockUseM(),
		d.uniSkip, d.uniDM)
	// Sliding-window marking: a reference picture that finds the
	// buffer full unmarks the oldest short-term before storing.
	// The victim is picked by wrapped frame number against the picture
	// being stored (raw comparison evicts the newest after a wrap).
	if d.curIsRef && d.sps != nil && d.sps.NumRefFrames > 0 {
		bits, berr := frameNumBits(d.sps)
		if berr != nil {
			return nil, berr
		}
		maxPicNum := int64(1) << uint(bits)
		for d.dpb.Len() >= int(d.sps.NumRefFrames) {
			d.dpb.evictOldest(d.pic.FrameNum, maxPicNum)
		}
	}
	if d.curIsRef {
		d.pic.archiveMotion(d)
	}
	d.dpb.Store(d.pic, d.curIsRef)
	if d.curIsRef {
		d.lastStored = d.pic
	} else {
		d.lastStored = nil
	}
	out := d.pic
	// Cropped display size: references keep the aligned picture, the
	// caller gets the cropped view (copied; uncropped returns as-is).
	if d.sps != nil && (d.sps.Width != out.Width || d.sps.Height != out.Height) {
		cr, err := out.Crop(d.sps.Width, d.sps.Height)
		if err != nil {
			return nil, err
		}
		cr.FrameNum, cr.POC, cr.IsIDR = out.FrameNum, out.POC, out.IsIDR
		out = cr
	}
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

// neighborT8 counts same-slice 8x8-transform neighbours (left + top)
// for the transform_size_8x8_flag context.
func (d *Decoder) neighborT8(addr, mbx, mby int) int {
	n := 0
	if mbx > 0 && d.cabSameSlice(addr-1) && addr-1 >= 0 && addr-1 < len(d.mbT8) && d.mbT8[addr-1] {
		n++
	}
	if mby > 0 && d.cabSameSlice(addr-d.mbW) && addr-d.mbW >= 0 && addr-d.mbW < len(d.mbT8) && d.mbT8[addr-d.mbW] {
		n++
	}
	return n
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
	return d.decodeIntraMBCore(h, pps, addr, mbx, mby, mbType, d.cavlcIntraSrc(r, pps), d.cavlcSrc(r))
}

// intraSrc provides intra syntax; CAVLC reads codes, CABAC bins. chroma
// returns the internal mode (V/H/DC/Plane = 0/1/2/3).
//
// S1b-G: zero per-MB heap, same treatment as residSrc (S1b-F): methods
// on a stack value instead of 6 heap closures per macroblock.
type intraSrc struct {
	d              *Decoder
	r              *Reader // CAVLC bitstream (nil for CABAC)
	pps            *PPS
	addr, mbx, mby int
	cabac          bool
}

// cavlcIntraSrc reads intra syntax with Exp-Golomb codes.
func (d *Decoder) cavlcIntraSrc(r *Reader, pps *PPS) intraSrc {
	return intraSrc{d: d, r: r, pps: pps}
}

// t8 reads the transform_size_8x8_flag.
func (s *intraSrc) t8() (bool, error) {
	if s.pps == nil || !s.pps.Transform8x8 {
		return false, nil
	}
	if s.cabac {
		return s.d.cabBin(399+uint16(s.d.neighborT8(s.addr, s.mbx, s.mby))) != 0, nil
	}
	b, err := s.r.ReadBits(1)
	if err != nil {
		return false, fmt.Errorf("t8 flag: %w", err)
	}
	return b != 0, nil
}

// mode reads one Intra4x4 prediction mode (the predictor is derived
// inside, like the CABAC closure before it; the pred argument stays
// for the common signature).
func (s *intraSrc) mode(bx, by, pred int) (int, error) {
	if s.cabac {
		pred = s.d.intraMostProbable(bx, by)
		if s.d.cabBin(68) != 0 {
			return pred, nil
		}
		rem := s.d.cabBin(69) + 2*s.d.cabBin(69) + 4*s.d.cabBin(69)
		if rem >= pred {
			rem++
		}
		return rem, nil
	}
	return s.d.intra4x4Mode(s.r, bx, by)
}

// mode8 reads one Intra8x8 prediction mode (same bins as 4x4 here).
func (s *intraSrc) mode8(bx, by int) (int, error) {
	if s.cabac {
		pred := s.d.intraMostProbable(bx, by)
		if s.d.cabBin(68) != 0 {
			return pred, nil
		}
		rem := s.d.cabBin(69) + 2*s.d.cabBin(69) + 4*s.d.cabBin(69)
		if rem >= pred {
			rem++
		}
		return rem, nil
	}
	return s.d.intra4x4Mode(s.r, bx, by)
}

// chroma reads the intra chroma mode.
func (s *intraSrc) chroma() (uint32, error) {
	if s.cabac {
		d := s.d
		left, top := d.cabLeftTop(s.addr, s.mbx, s.mby)
		ctx := uint16(64)
		if left >= 0 && d.mbIntra[left] && d.cmode[left] != 2 {
			ctx++
		}
		if top >= 0 && d.mbIntra[top] && d.cmode[top] != 2 {
			ctx++
		}
		if d.cabBin(ctx) == 0 {
			return 2, nil
		}
		if d.cabBin(67) == 0 {
			return 1, nil
		}
		if d.cabBin(67) == 0 {
			return 0, nil
		}
		return 3, nil
	}
	v, err := s.r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("chroma mode: %w", err)
	}
	if v > 3 {
		if clampIV {
			v = 0
		} else {
			return 0, fmt.Errorf("%w: chroma mode %d", ErrBadSliceHeader, v)
		}
	}
	// ue values run DC/H/V/Plane; internal consts run V/H/DC/Plane.
	return [4]uint32{2, 1, 0, 3}[v], nil
}

// cbp reads the intra coded-block pattern.
func (s *intraSrc) cbp() (uint32, error) {
	if s.cabac {
		return s.d.cabacCBP(s.addr, s.mbx, s.mby, true)
	}
	cbpUE, err := s.r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("cbp: %w", err)
	}
	if cbpUE > 47 {
		return 0, fmt.Errorf("%w: cbp %d", ErrBadSliceHeader, cbpUE)
	}
	return uint32(golombToIntra4x4CBP[cbpUE]), nil
}

// qpD reads mb_qp_delta.
func (s *intraSrc) qpD() (int32, error) {
	if s.cabac {
		return s.d.cabacQPDelta()
	}
	delta, err := s.r.ReadSE()
	if err != nil {
		return 0, fmt.Errorf("qp delta: %w", err)
	}
	return delta, nil
}

func (d *Decoder) decodeIntraMBCore(h *SliceHeader, pps *PPS, addr, mbx, mby int, mbType uint32, is intraSrc, rs residSrc) error {
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
	use8 := false
	if !i16 {
		var err error
		use8, err = is.t8()
		if err != nil {
			return err
		}
		if addr >= 0 && addr < len(d.mbT8) {
			d.mbT8[addr] = use8
		}
	}
	var modes [16]int
	var modes8 [4]int
	if kind == mbI4x4 && !use8 {
		stride := d.mbW * 4
		// Modes follow luma4x4BlkIdx (8x8-grouped), not raster.
		for _, b := range [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15} {
			bx, by := mbx*4+b%4, mby*4+b/4
			m, err := is.mode(bx, by, 0)
			if err != nil {
				return err
			}
			modes[b] = m
			// Publish immediately: later blocks derive their
			// most-probable mode from already-parsed neighbours.
			d.modes[by*stride+bx] = int8(m)
		}
	}
	if kind == mbI4x4 && use8 {
		stride := d.mbW * 4
		// One mode per 8x8 (TL,TR,BL,BR); each replicates to its
		// four 4x4 slots for most-probable prediction.
		for i8, b := range [4]int{0, 2, 8, 10} {
			bx, by := mbx*4+b%4, mby*4+b/4
			m, err := is.mode8(bx, by)
			if err != nil {
				return err
			}
			modes8[i8] = m
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					d.modes[(by+dy)*stride+bx+dx] = int8(m)
					modes[(by-mby*4+dy)*4+(bx-mbx*4+dx)] = m
				}
			}
		}
	}
	chromaMode, err := is.chroma()
	if err != nil {
		return err
	}
	if err := checkChromaMode(int(chromaMode), mbx, mby); err != nil {
		return err
	}
	if !i16 {
		v, err := is.cbp()
		if err != nil {
			return err
		}
		cbp = v
	}
	if cbp != 0 || i16 {
		delta, err := is.qpD()
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
	// Neighbour contexts (also read by CABAC slices): chroma mode, I16
	// flag, CBP without DC-present bits (CABAC sets those lazily while
	// decoding DC blocks), and owning slice.
	d.cmode[addr] = uint8(chromaMode)
	d.mbI16[addr] = i16
	d.cbpArr[addr] = uint16(cbp)
	d.mbSlice[addr] = d.slices
	if kind == mbI16x16 {
		if addr >= 0 && addr < len(d.mbT8) {
			d.mbT8[addr] = false
		}
		return d.reconstructI16x16With(mbx, mby, pred16, int(chromaMode), cbp, rs)
	}
	if use8 {
		return d.reconstructI8x8With(mbx, mby, modes8, int(chromaMode), cbp, rs)
	}
	return d.reconstructI4x4With(mbx, mby, modes, int(chromaMode), cbp, rs)
}

// intraMostProbable derives one block prediction mode with most-probable rule.
// Unavailable neighbours (picture edge or another slice) contribute -1
// and force DC; decoded neighbours contribute their stored mode, with
// non-Intra4x4 blocks (inter, I16x16, PCM) reading as DC (2), mirroring
// the reference pred_intra_mode cache (inter type maps to 2, only a
// zero type maps to -1). Constrained-intra gating is not applied yet.
func (d *Decoder) intraMostProbable(bx, by int) int {
	stride := d.mbW * 4
	cur := (by/4)*d.mbW + bx/4
	// Neighbours inside the macroblock under decode are trivially
	// same-slice: its slice tag publishes only after the modes parse.
	same := func(mb int) bool { return mb == cur || d.cabSameSlice(mb) }
	aC, bC := -1, -1
	if bx > 0 && same((by/4)*d.mbW+(bx-1)/4) {
		aC = 2
		if a := int(d.modes[by*stride+bx-1]); a >= 0 {
			aC = a
		}
	}
	if by > 0 && same(((by-1)/4)*d.mbW+bx/4) {
		bC = 2
		if b := int(d.modes[(by-1)*stride+bx]); b >= 0 {
			bC = b
		}
	}
	if aC < 0 || bC < 0 {
		return 2
	}
	if aC < bC {
		return aC
	}
	return bC
}

// intra4x4Mode reads one block prediction mode with most-probable rule.
func (d *Decoder) intra4x4Mode(r *Reader, bx, by int) (int, error) {
	pred := d.intraMostProbable(bx, by)
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
	d.storePCM(raw, mbx, mby)
	return nil
}

// storePCM writes raw PCM samples and marks the block's neighbour state.
func (d *Decoder) storePCM(raw []byte, mbx, mby int) {
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
	// Neighbour contexts shared with CABAC slices: PCM counts as coded
	// intra with full CBP, mirroring the reference decoder's marks.
	addr := mby*d.mbW + mbx
	d.skipped[addr] = false
	d.mbI16[addr] = true
	d.cmode[addr] = 2
	d.cbpArr[addr] = 0x1FF
	d.mbSlice[addr] = d.slices
	if addr >= 0 && addr < len(d.mbT8) {
		d.mbT8[addr] = false
	}
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

// residSrc decodes residual blocks into scan order; CAVLC and CABAC
// share one value (cabac flag picks the bitstream reader), so the
// reconstruction loops stay shared.
//
// S1b-F: zero per-MB heap. The old shape built 6 closures + struct on
// the heap per macroblock (~370B x 5184 MBs ≈ 1.9MB/frame of garbage
// on 1536x864, straight into GC). Methods on a stack value do the
// same reads with direct calls and no allocation: builders return a
// value, callers pass it down by value, nothing retains it.
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//
//	libavcodec/h264dsp.c:78-98 add_pixels/idct tables (stable function
//	pointers installed once; per-block work dispatched with no per-MB
//	allocation) + libavcodec/h264_mb.c:762 hl_decode_mb_idct_luma
//	(residual gated on coded blocks; our direct method calls); oracle
//	is VR2 exact (pixels).
type residSrc struct {
	d              *Decoder
	r              *Reader // CAVLC bitstream (nil for CABAC: engine lives in d)
	addr, mbx, mby int
	intra          bool
	cabac          bool
}

// cavlcGrouped groups raster 4x4 blocks for 8x8 CAVLC order; cavlcInv8
// maps raster inside 8x8 to zigzag scan position. Both were built per
// macroblock in cavlcSrc; now once (S1b-F).
var cavlcGrouped = [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15}

var cavlcInv8 = func() [64]int {
	var inv [64]int
	for s, p := range zigzag8x8 {
		inv[p] = s
	}
	return inv
}()

// cavlcSrc builds the CAVLC residual source for one macroblock.
func (d *Decoder) cavlcSrc(r *Reader) residSrc {
	return residSrc{d: d, r: r}
}

// dcBit reports one neighbour DC-presence bit for CABAC flag contexts.
func (s *residSrc) dcBit(naddr int, bit uint16) int {
	d := s.d
	if d.cabSameSlice(naddr) {
		if d.cbpArr[naddr]&bit != 0 {
			return 1
		}
		return 0
	}
	if s.intra {
		return 1
	}
	return 0
}

// lumaAC reads one 4x4 luma AC block (category 2 for Intra4x4/inter;
// the cat argument stays for the common signature).
func (s *residSrc) lumaAC(bx, by, cat int) ([16]int32, int, error) {
	d := s.d
	if s.cabac {
		ys, yh := d.mbW*4, d.mbH*4
		nza := d.cabacNNZAt(d.nnzY, ys, yh, 4, bx-1, by, s.intra)
		nzb := d.cabacNNZAt(d.nnzY, ys, yh, 4, bx, by-1, s.intra)
		if !d.cabacCBF(2, nza, nzb) {
			return [16]int32{}, 0, nil
		}
		return d.cabacCoeffData(2, 16, 0)
	}
	stride := d.mbW * 4
	return d.readLumaBlock(s.r, d.blockNC(d.nnzY, stride, bx, by))
}

// lumaAC15 reads one I16x16 AC block: 15 coeffs (DC excluded), placed
// one slot up so index matches true scan position.
func (s *residSrc) lumaAC15(bx, by int) ([16]int32, int, error) {
	d := s.d
	if s.cabac {
		ys, yh := d.mbW*4, d.mbH*4
		nza := d.cabacNNZAt(d.nnzY, ys, yh, 4, bx-1, by, s.intra)
		nzb := d.cabacNNZAt(d.nnzY, ys, yh, 4, bx, by-1, s.intra)
		if !d.cabacCBF(1, nza, nzb) {
			return [16]int32{}, 0, nil
		}
		return d.cabacCoeffData(1, 15, 1)
	}
	stride := d.mbW * 4
	nC := d.blockNC(d.nnzY, stride, bx, by)
	ac, err := DecodeResidualBlock(s.r, SelectTable(nC), 15, 1, false)
	if err != nil {
		return ac, 0, err
	}
	tc := 0
	for _, v := range ac {
		if v != 0 {
			tc++
		}
	}
	return ac, tc, nil
}

// lumaDC reads one 4x4 luma DC block.
func (s *residSrc) lumaDC(mbx, mby int) ([16]int32, error) {
	d := s.d
	if s.cabac {
		nza, nzb := 0, 0
		if mbx > 0 {
			nza = s.dcBit(s.addr-1, 0x100)
		} else if s.intra {
			nza = 1
		}
		if mby > 0 {
			nzb = s.dcBit(s.addr-d.mbW, 0x100)
		} else if s.intra {
			nzb = 1
		}
		if !d.cabacCBF(0, nza, nzb) {
			return [16]int32{}, nil
		}
		// A coded DC block marks presence for later neighbours.
		d.cbpArr[s.addr] |= 0x100
		out, _, err := d.cabacCoeffData(0, 16, 0)
		return out, err
	}
	stride := d.mbW * 4
	nC := d.blockNC(d.nnzY, stride, mbx*4, mby*4)
	return DecodeResidualBlock(s.r, SelectTable(nC), 16, 0, false)
}

// chromaDC reads one 2x2 chroma DC block.
func (s *residSrc) chromaDC(comp int) ([4]int32, error) {
	d := s.d
	if s.cabac {
		bit := uint16(0x40 << uint(comp))
		nza, nzb := 0, 0
		if s.mbx > 0 {
			nza = s.dcBit(s.addr-1, bit)
		} else if s.intra {
			nza = 1
		}
		if s.mby > 0 {
			nzb = s.dcBit(s.addr-d.mbW, bit)
		} else if s.intra {
			nzb = 1
		}
		if !d.cabacCBF(3, nza, nzb) {
			return [4]int32{}, nil
		}
		d.cbpArr[s.addr] |= bit
		out, _, err := d.cabacCoeffData(3, 4, 0)
		if err != nil {
			return [4]int32{}, err
		}
		var dc [4]int32
		copy(dc[:], out[:4])
		return dc, nil
	}
	dcRaw, err := DecodeResidualBlock(s.r, 0, 4, 0, true)
	if err != nil {
		return [4]int32{}, err
	}
	var dcArr [4]int32
	copy(dcArr[:], dcRaw[:4])
	return dcArr, nil
}

// chromaAC reads one 4x4 chroma AC block.
func (s *residSrc) chromaAC(mbx, mby, comp, b int) ([16]int32, int, error) {
	d := s.d
	if s.cabac {
		cs, ch := d.mbW*2, d.mbH*2
		grid := d.nnzCb
		if comp == 1 {
			grid = d.nnzCr
		}
		bx, by := mbx*2+b%2, mby*2+b/2
		nza := d.cabacNNZAt(grid, cs, ch, 2, bx-1, by, s.intra)
		nzb := d.cabacNNZAt(grid, cs, ch, 2, bx, by-1, s.intra)
		if !d.cabacCBF(4, nza, nzb) {
			return [16]int32{}, 0, nil
		}
		return d.cabacCoeffData(4, 15, 1)
	}
	cstride := d.mbW * 2
	grid := d.nnzCb
	if comp == 1 {
		grid = d.nnzCr
	}
	bx, by := mbx*2+b%2, mby*2+b/2
	nC := d.blockNC(grid, cstride, bx, by)
	ac, err := DecodeResidualBlock(s.r, SelectTable(nC), 15, 1, false)
	if err != nil {
		return ac, 0, err
	}
	tc := 0
	for _, v := range ac {
		if v != 0 {
			tc++
		}
	}
	return ac, tc, nil
}

// luma8x8 reads one 8x8 luma residual group (four 4x4 sub-blocks).
func (s *residSrc) luma8x8(mbx, mby, i8 int) ([64]int32, [4]int, error) {
	d := s.d
	if s.cabac {
		out, tc, err := d.cabacCoeffData8x8()
		if err != nil {
			return out, [4]int{}, err
		}
		return out, [4]int{tc, tc, tc, tc}, nil
	}
	stride := d.mbW * 4
	var coeff [64]int32
	var tcs [4]int
	for g := 0; g < 4; g++ {
		b := cavlcGrouped[i8*4+g]
		bx, by := mbx*4+b%4, mby*4+b/4
		nC := d.blockNC(d.nnzY, stride, bx, by)
		blk, err := DecodeResidualBlock(s.r, SelectTable(nC), 16, 0, false)
		if err != nil {
			return coeff, tcs, err
		}
		tc := 0
		for _, v := range blk {
			if v != 0 {
				tc++
			}
		}
		tcs[g] = tc
		// Publish immediately: later sub-blocks in this 8x8
		// derive nC from already-decoded neighbours.
		d.setNnz(d.nnzY, stride, bx, by, tc)
		for k := 0; k < 16; k++ {
			if blk[k] == 0 {
				continue
			}
			raster := zigzag8x8CAVLC[g*16+k]
			coeff[cavlcInv8[raster]] = blk[k]
		}
	}
	// Deblock strength reads the first 4x4 slot of each 8x8
	// as the whole-block presence: fold the four slice counts
	// into slot 0 like the reference decoder does. Later
	// neighbours' nC keeps working because only zero-ness of
	// that slot changes the table the same way it does there.
	b0 := cavlcGrouped[i8*4]
	t := tcs[0] + tcs[1] + tcs[2] + tcs[3]
	tcs[0] = t
	d.setNnz(d.nnzY, stride, mbx*4+b0%4, mby*4+b0/4, t)
	return coeff, tcs, nil
}

func (d *Decoder) reconstructI4x4With(mbx, mby int, modes [16]int, chromaMode int, cbp uint32, rs residSrc) error {
	stride := d.mbW * 4
	wY := d.sc4[0]
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
			c, total, err := rs.lumaAC(bx, by, 2)
			if err != nil {
				return err
			}
			coeff, tc = c, total
		}
		res := ITransform4x4Scaled(coeff, uint32(d.qpY), wY)
		if !addResidBlock(d.pic.Y, d.pic.Width, uint32(bx*4), uint32(by*4), pred[:], 4, res[:], 4, 4) {
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					v := int32(pred[y*4+x]) + res[y*4+x]
					d.pic.SetY(uint32(bx*4+x), uint32(by*4+y), clipPixel(v))
				}
			}
		}
		d.modes[by*stride+bx] = int8(modes[b])
		d.setNnz(d.nnzY, stride, bx, by, tc)
	}
	return d.reconstructChromaWith(mbx, mby, chromaMode, (cbp>>4)&3, rs, true)
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

// reconstructI8x8With predicts four 8x8 luma blocks, adds scaled 8x8
// residual and writes chroma.
func (d *Decoder) reconstructI8x8With(mbx, mby int, modes8 [4]int, chromaMode int, cbp uint32, rs residSrc) error {
	stride := d.mbW * 4
	w8 := d.sc8[0]
	grouped := [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15}
	for i8, b := range [4]int{0, 2, 8, 10} {
		bx, by := mbx*4+b%4, mby*4+b/4
		get := d.lumaBlockSampler(bx, by)
		pred, err := PredIntra8x8(get, int32(bx*4), int32(by*4), modes8[i8])
		if err != nil {
			return err
		}
		var coeff [64]int32
		var tcs [4]int
		if cbp&(1<<uint(i8)) != 0 {
			c, t, err := rs.luma8x8(mbx, mby, i8)
			if err != nil {
				return err
			}
			coeff, tcs = c, t
		}
		res := ITransform8x8Scaled(coeff, uint32(d.qpY), w8)
		if !addResidBlock(d.pic.Y, d.pic.Width, uint32(mbx*16+(i8%2)*8), uint32(mby*16+(i8/2)*8), pred[:], 8, res[:], 8, 8) {
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					v := int32(pred[y*8+x]) + res[y*8+x]
					d.pic.SetY(uint32(mbx*16+(i8%2)*8+x), uint32(mby*16+(i8/2)*8+y), clipPixel(v))
				}
			}
		}
		for g := 0; g < 4; g++ {
			bb := grouped[i8*4+g]
			xx, yy := mbx*4+bb%4, mby*4+bb/4
			d.setNnz(d.nnzY, stride, xx, yy, tcs[g])
		}
	}
	return d.reconstructChromaWith(mbx, mby, chromaMode, (cbp>>4)&3, rs, true)
}

func (d *Decoder) reconstructI16x16With(mbx, mby, pred16, chromaMode int, cbp uint32, rs residSrc) error {
	get := d.lumaSampler()
	pred, err := PredIntra16x16(get, int32(mbx*16), int32(mby*16), pred16)
	if err != nil {
		return err
	}
	dcRaw, err := rs.lumaDC(mbx, mby)
	if err != nil {
		return fmt.Errorf("luma dc: %w", err)
	}
	dcScaled := ITransformLumaDCScaled(dcRaw, uint32(d.qpY), d.sc4[0][0])
	dcBlk := dcScaled
	stride := d.mbW * 4
	wY := d.sc4[0]
	// AC blocks follow luma4x4BlkIdx (8x8-grouped), like Intra4x4.
	for _, b := range [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15} {
		var coeff [16]int32
		acNZ := 0
		if cbp&15 != 0 {
			bx, by := mbx*4+b%4, mby*4+b/4
			ac, total, err := rs.lumaAC15(bx, by)
			if err != nil {
				return fmt.Errorf("luma ac %d: %w", b, err)
			}
			coeff, acNZ = ac, total
		}
		res := ITransform4x4WithDCScaled(coeff, dcBlk[b], uint32(d.qpY), wY)
		bx, by := mbx*4+b%4, mby*4+b/4
		px0, py0 := (bx-mbx*4)*4, (by-mby*4)*4
		if !addResidBlock(d.pic.Y, d.pic.Width, uint32(bx*4), uint32(by*4), pred[py0*16+px0:], 16, res[:], 4, 4) {
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					v := int32(pred[((by-mby*4)*4+y)*16+(bx-mbx*4)*4+x]) + res[y*4+x]
					d.pic.SetY(uint32(bx*4+x), uint32(by*4+y), clipPixel(v))
				}
			}
		}
		d.modes[by*stride+bx] = 2
		d.setNnz(d.nnzY, stride, bx, by, acNZ)
	}
	return d.reconstructChromaWith(mbx, mby, chromaMode, (cbp>>4)&3, rs, true)
}

func (d *Decoder) reconstructChromaWith(mbx, mby, chromaMode int, cbpC uint32, rs residSrc, intra bool) error {
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
	return d.reconstructChromaBlocks(mbx, mby, cbpC, &preds[0], &preds[1], rs, intra)
}

// reconstructChromaBlocks reads chroma residual against caller-supplied
// prediction and adds it into the picture. Both intra and inter macroblocks
// share this tail; only the prediction source differs.
func (d *Decoder) reconstructChromaBlocks(mbx, mby int, cbpC uint32, predCb, predCr *[64]uint8, rs residSrc, intra bool) error {
	planes := [2][]uint8{d.pic.Cb, d.pic.Cr}
	preds := [2]*[64]uint8{predCb, predCr}
	grids := [2][]int8{d.nnzCb, d.nnzCr}
	cstride := d.mbW * 2
	// S1b-D zero fast lane: no chroma coefficients anywhere (skip /
	// zero-cbp blocks, the common case on still content). The slow
	// path below would run 8 zero-input inverse transforms
	// (ITransform4x4WithDCScaled of all-zero coeff + zero DC is all
	// zero: the dequant butterfly is linear) and add all-zero
	// residual onto prediction (clipPixel identity) — so prediction
	// IS the reconstruction. Copy rows and mark zero, no transform,
	// no bitstream reads (the slow path reads nothing either when
	// cbpC==0: chromaDC/AC sit behind cbpC>0 / ==2 gates).
	//
	// Peer (ffmpeg, read-only, ideas only, no code copied):
	//   libavcodec/h264_mb.c:762 hl_decode_mb_idct_luma
	//   (else-if sl->cbp & 15 gates the whole IDCT-add on coded
	//   blocks; zero-pattern blocks output prediction untouched;
	//   our cbpC==0 gate below); oracle is VR2 exact (pixels).
	if cbpC == 0 {
		cw := int(d.pic.Width / 2)
		ox, oy := mbx*8, mby*8
		for comp := 0; comp < 2; comp++ {
			dst := planes[comp]
			src := preds[comp]
			for y := 0; y < 8; y++ {
				copy(dst[(oy+y)*cw+ox:(oy+y)*cw+ox+8], src[y*8:(y+1)*8])
			}
			for b := 0; b < 4; b++ {
				bx, by := mbx*2+b%2, mby*2+b/2
				d.setNnz(grids[comp], cstride, bx, by, 0)
			}
		}
		return nil
	}
	qps := [2]int32{
		ChromaQP(d.qpY, d.cOff0),
		ChromaQP(d.qpY, d.cOff1),
	}
	var dcRs [2][4]int32
	if cbpC > 0 {
		for comp := 0; comp < 2; comp++ {
			dcArr, err := rs.chromaDC(comp)
			if err != nil {
				return fmt.Errorf("chroma dc: %w", err)
			}
			var w0 uint8 = 16
			if intra {
				w0 = d.sc4[1+comp][0]
			} else {
				w0 = d.sc4[4+comp][0]
			}
			dcRs[comp] = ITransformChromaDCScaled(dcArr, uint32(qps[comp]), w0)
		}
	}
	for comp := 0; comp < 2; comp++ {
		pred := preds[comp]
		dcR := dcRs[comp]
		var w [16]uint8
		if intra {
			w = d.sc4[1+comp]
		} else {
			w = d.sc4[4+comp]
		}
		for b := 0; b < 4; b++ {
			var coeff [16]int32
			acNZ := 0
			if cbpC == 2 {
				ac, total, err := rs.chromaAC(mbx, mby, comp, b)
				if err != nil {
					return fmt.Errorf("chroma ac c%d b%d: %w", comp, b, err)
				}
				coeff, acNZ = ac, total
			}
			res := ITransform4x4WithDCScaled(coeff, dcR[b], uint32(qps[comp]), w)
			bx, by := mbx*2+b%2, mby*2+b/2
			px0, py0 := (bx-mbx*2)*4, (by-mby*2)*4
			if !addResidBlock(planes[comp], d.pic.Width/2, uint32(bx*4), uint32(by*4), pred[py0*8+px0:], 8, res[:], 4, 4) {
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						v := int32(pred[((by-mby*2)*4+y)*8+(bx-mbx*2)*4+x]) + res[y*4+x]
						px, py := uint32(bx*4+x), uint32(by*4+y)
						planes[comp][py*d.pic.Width/2+px] = clipPixel(v)
					}
				}
			}
			d.setNnz(grids[comp], cstride, bx, by, acNZ)
		}
	}
	return nil
}
