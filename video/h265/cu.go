package h265

import (
	"fmt"
)

// Intra mode ids and part mode ids (syntax layer).
const (
	IntraPlanar = 0
	IntraDC     = 1
	IntraAng26  = 26
)

const (
	Part2Nx2N = 0
	Part2NxN  = 1
	PartNx2N  = 2
	PartNxN   = 3
	Part2NxnU = 4
	Part2NxnD = 5
	PartnLx2N = 6
	PartnRx2N = 7
)

// Prediction modes for the inter stage (I slices stay PredIntra).
const (
	PredIntra = 0
	PredInter = 1
	PredSkip  = 2
)

// SAO type ids (match the peer enum: off/band/edge).
const (
	SAOOff  = 0
	SAOBand = 1
	SAOEdge = 2
)

// Scan ids used by the residual path.
const (
	ScanDiag  = 0
	ScanHoriz = 1
	ScanVert  = 2
)

// SAOParams holds one CTB's filter choice (values merge-resolved).
type SAOParams struct {
	MergeLeft bool
	MergeUp   bool
	Type      [3]uint8
	Abs       [3][4]uint8
	Sign      [3][4]bool
	BandPos   [3]uint8
	EOClass   [3]uint8
}

// PUInfo is one inter prediction unit's motion truth (quarter-pel
// MV/MVD; merge PUs carry only the index until the merge stage runs).
type PUInfo struct {
	X0, Y0     int
	W, H       int
	Merge      bool
	MergeIdx   uint8
	RefIdx     uint8
	MVX, MVY   int32
	MVDX, MVDY int32
	MVPFlag    uint8
}

// CUInfo is one coding unit's syntax truth for the pixel stage.
// Pred selects intra (I slices and intra CUs in P slices) vs inter
// (PredInter) vs skip (PredSkip); PUs rides only for inter/skip.
type CUInfo struct {
	X0, Y0     int
	Log2Size   int
	Part       uint8
	Pred       uint8
	PUs        []PUInfo
	NoResid    bool
	Luma       [4]uint8
	Chroma     uint8
	ChromaLuma uint8
	QpDelta    int32
}

// TUInfo is one transform unit's coefficient truth (raster levels,
// pre-dequant; the pixel stage scales and transforms). QP carries the
// luma QP in force for this block (slice QP plus decoded deltas).
type TUInfo struct {
	X0, Y0   int
	Log2Size int
	CIdx     int
	QP       int32
	Coeffs   []int16
}

// TULeaf is one visited transform leaf: every transform_unit call lands
// here, including all-zero leaves (no TUInfo rides for those, but the
// pixel stage still predicts them). Order matches the decode walk.
// CbX/CbY carry the transform-tree root (CU origin) for the small-TU
// chroma path, which predicts at the root on the last leaf.
type TULeaf struct {
	X0, Y0     int
	CbX, CbY   int
	Log2Size   int
	Blk        int
	CbfLuma    bool
	CbfCb      bool
	CbfCr      bool
	LumaMode   uint8
	ChromaMode uint8
	QP         int32
}

// FrameSyntax is the full IDR syntax: headers, CUs, TUs, SAO table.
type FrameSyntax struct {
	SH       *SliceHeader
	CUs      []CUInfo
	TUs      []TUInfo
	Leaves   []TULeaf
	SAO      []SAOParams
	CTUs     int
	Consumed int
	TailZero bool
	// TailFirstOne/PayloadBits debug the trailing gate: bit index of
	// the first 1 past Consumed (-1 when clean) and total payload bits.
	TailFirstOne int
	PayloadBits  int
}

// synParser walks one intra slice's CABAC payload into a FrameSyntax.
// Peer (read-only): hevcdec.c hls_coding_quadtree/hls_coding_unit/
// hls_transform_tree/hls_transform_unit/intra_prediction_unit/
// hls_sao_param/luma_intra_pred_mode + cabac.c syntax decoders.
// Only the bin order and context indices travel.
type synParser struct {
	cab          *cabacDec
	st           [199]uint8
	sps          *SPS
	pps          *PPS
	sh           *SliceHeader
	depth        []uint8 // per min-CB, for split_cu contexts
	ipm          []uint8 // per min-PU, for MPM derivation
	skip         []uint8 // per min-CB, for skip_flag contexts (inter)
	minCB        int
	minPU        int
	gridW        int // min-CB columns
	puW          int // min-PU columns
	picW         int
	picH         int
	ctb          int // log2 CTB size
	qpY          int32
	qpCoded      bool
	curPredIntra bool // transform tree scans diagonal unless intra
	curMaxDepth  int
	curSplit     bool // intra NxN split of the CU under walk
	// traceTU logs (x0,y0,log2t,cIdx,nbits) per residual call when set.
	traceTU *[]string
	frame   *FrameSyntax
}

func (p *synParser) bin(ctx int) (int, error) {
	if ctx < 0 || ctx >= 199 {
		return 0, fmt.Errorf("%w: ctx %d", ErrBadSlice, ctx)
	}
	v, err := p.cab.binLog(&p.st[ctx], ctx)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (p *synParser) bypass() int { return p.cab.bypassLog() }

func (p *synParser) bypassN(n int) (uint32, error) { return p.cab.bypassBinsLog(n) }

// ParseFrameSyntax parses one slice NALU fully into syntax.
// I and P slices ride here; B returns the honest pixel-stage error.
func ParseFrameSyntax(nalu []byte, ps *ParamSets, pocTid0 int) (*FrameSyntax, error) {
	sh, err := ParseSliceHeader(nalu, ps, pocTid0)
	if err != nil {
		return nil, err
	}
	if sh.Type != SliceI && sh.Type != SliceP {
		return nil, fmt.Errorf("%w: %s slices", ErrNotDecodable, SliceTypeName(sh.Type))
	}
	q, ok := ps.PPS[sh.PPSID]
	if !ok {
		return nil, fmt.Errorf("%w: pps %d", ErrMissingPPS, sh.PPSID)
	}
	s, ok := ps.SPS[q.SPSID]
	if !ok {
		return nil, fmt.Errorf("%w: sps %d", ErrMissingSPS, q.SPSID)
	}
	raw := UnescapeRBSP(nalu[2:])
	if sh.DataOffset%8 != 0 || sh.DataOffset/8 > len(raw) {
		return nil, fmt.Errorf("%w: data offset %d", ErrBadSlice, sh.DataOffset)
	}
	payload := append([]byte(nil), raw[sh.DataOffset/8:]...)
	payload = append(payload, 0, 0, 0, 0) // standard overread pad
	cab, err := newCabacDec(payload)
	if err != nil {
		return nil, err
	}
	p := &synParser{
		cab: cab, sps: s, pps: q, sh: sh,
		minCB: 1 << s.Log2MinCB,
		minPU: 1 << (s.Log2MinCB - 1),
		picW:  int(s.Width), picH: int(s.Height),
		ctb:   int(s.Log2MaxCB),
		qpY:   sh.SliceQP,
		frame: &FrameSyntax{SH: sh},
	}
	// Peer: ps.c inferred geometry (log2_ctb = min_cb + diff;
	// ceiling grids; qp offset from bit depth). Our 96x96 clip is a
	// multiple of every grid so plain and ceiling division agree;
	// ceiling keeps larger clips honest.
	if int64(s.Log2MinCB)+int64(s.Log2MaxCB-s.Log2MinCB) != int64(s.Log2MaxCB) {
		return nil, fmt.Errorf("%w: ctb size", ErrBadSPS)
	}
	p.gridW = (p.picW + p.minCB - 1) / p.minCB
	gridH := (p.picH + p.minCB - 1) / p.minCB
	p.puW = (p.picW + p.minPU - 1) / p.minPU
	puH := (p.picH + p.minPU - 1) / p.minPU
	p.depth = make([]uint8, p.gridW*gridH)
	p.skip = make([]uint8, p.gridW*gridH)
	p.ipm = make([]uint8, p.puW*puH)
	for i := range p.ipm {
		p.ipm[i] = IntraDC
	}
	var state [199]uint8
	initCtx(&state, sh.Type, false, sh.SliceQP)
	p.st = state
	ctbSize := 1 << p.ctb
	ctbW := (p.picW + ctbSize - 1) / ctbSize
	ctbH := (p.picH + ctbSize - 1) / ctbSize
	more := true
	p.frame.TailFirstOne = -2
	lastFrame = p.frame
	for rs := 0; rs < ctbW*ctbH && more; rs++ {
		rx, ry := rs%ctbW, rs/ctbW
		if err := p.saoParam(rx, ry); err != nil {
			return nil, err
		}
		p.frame.CTUs++
		more, err = p.quadtree(rx*ctbSize, ry*ctbSize, p.ctb, 0)
		if err != nil {
			// Report the tail position even on mid-slice errors.
			used := cab.consumedBits()
			p.frame.Consumed = used
			p.frame.PayloadBits = (len(payload) - 4) * 8
			p.frame.TailFirstOne = -2
			return nil, err
		}
	}
	if more {
		return nil, fmt.Errorf("%w: slice not terminated", ErrBadSlice)
	}
	// Tail accounting: the terminate bin already ended coding. Real
	// encoders byte-pad the tail arbitrarily (fresh x265 encodes end
	// mid-0x5E etc., never textbook stop-bit-plus-zeros, and the peer
	// never validates it either), so the only honest failure here is
	// reading past the payload. Exact stop position is pinned by the
	// gate test (consumed/payload), which catches any bin-count drift.
	used := cab.consumedBits()
	rawBits := (len(payload) - 4) * 8
	if used > rawBits {
		return nil, fmt.Errorf("%w: overread tail", ErrBadSlice)
	}
	tailZero := true
	firstOne := -1
	for i := used; i < rawBits; i++ {
		if payload[i/8]>>(uint(7-(i%8)))&1 != 0 {
			tailZero = false
			if firstOne < 0 {
				firstOne = i
			}
		}
	}
	p.frame.Consumed = used
	p.frame.TailZero = tailZero
	p.frame.TailFirstOne = firstOne
	p.frame.PayloadBits = rawBits
	return p.frame, nil
}

// DebugParse runs ParseFrameSyntax but returns the partial frame on
// error (for tests diagnosing bit positions).
func DebugParse(nalu []byte, ps *ParamSets) (*FrameSyntax, error) {
	fs, err := ParseFrameSyntax(nalu, ps, 0)
	if err != nil && lastFrame != nil {
		return lastFrame, err
	}
	return fs, err
}

var lastFrame *FrameSyntax

// saoParam parses one CTB's SAO choice (merge-resolved for the filter).
func (p *synParser) saoParam(rx, ry int) error {
	var sa SAOParams
	if p.sh.SAOLuma || p.sh.SAOChroma {
		if rx > 0 {
			v, err := p.bin(ctxSaoMergeFlag)
			if err != nil {
				return err
			}
			sa.MergeLeft = v != 0
		}
		if ry > 0 && !sa.MergeLeft {
			v, err := p.bin(ctxSaoMergeFlag)
			if err != nil {
				return err
			}
			sa.MergeUp = v != 0
		}
	}
	ctbW := (p.picW + (1 << p.ctb) - 1) / (1 << p.ctb)
	at := func(dx, dy int) *SAOParams {
		return &p.frame.SAO[(ry+dy)*ctbW+(rx+dx)]
	}
	nCh := 1
	if p.sps.ChromaFormat != 0 {
		nCh = 3
	}
	// Merged CTBs copy the neighbour's table with no bins on the wire
	// (peer SET_SAO macro args stay unevaluated on the merge path).
	if sa.MergeLeft {
		src := at(-1, 0)
		sa = *src
		sa.MergeLeft = true
		p.frame.SAO = append(p.frame.SAO, sa)
		return nil
	}
	if sa.MergeUp {
		src := at(0, -1)
		sa = *src
		sa.MergeUp = true
		p.frame.SAO = append(p.frame.SAO, sa)
		return nil
	}
	for c := 0; c < nCh; c++ {
		apply := (c == 0 && p.sh.SAOLuma) || (c > 0 && p.sh.SAOChroma)
		if !apply {
			sa.Type[c] = SAOOff
			continue
		}
		if c == 2 {
			sa.Type[2] = sa.Type[1]
			sa.EOClass[2] = sa.EOClass[1]
		} else {
			v, err := p.bin(ctxSaoTypeIdx)
			if err != nil {
				return err
			}
			if v == 0 {
				sa.Type[c] = SAOOff
			} else if p.bypass() == 0 {
				sa.Type[c] = SAOBand
			} else {
				sa.Type[c] = SAOEdge
			}
		}
		if sa.Type[c] == SAOOff {
			continue
		}
		for i := 0; i < 4; i++ {
			v, err := p.saoAbs()
			if err != nil {
				return err
			}
			sa.Abs[c][i] = v
		}
		if sa.Type[c] == SAOBand {
			for i := 0; i < 4; i++ {
				if sa.Abs[c][i] != 0 {
					sa.Sign[c][i] = p.bypass() != 0
				}
			}
			v, err := p.bypassN(5)
			if err != nil {
				return err
			}
			sa.BandPos[c] = uint8(v)
		} else if c != 2 {
			v, err := p.bypassN(2)
			if err != nil {
				return err
			}
			sa.EOClass[c] = uint8(v)
		}
	}
	p.frame.SAO = append(p.frame.SAO, sa)
	return nil
}

// saoAbs reads one truncated-unary offset (8-bit depth: max 31).
func (p *synParser) saoAbs() (uint8, error) {
	max := (1 << (8 - 5)) - 1
	var i int
	for i = 0; i < max && p.bypass() != 0; i++ {
	}
	return uint8(i), nil
}

// quadtree walks one coding quadtree; it reports whether more slice
// data follows (end_of_slice flag at CTB edges).
func (p *synParser) quadtree(x0, y0, log2cb, depth int) (bool, error) {
	cb := 1 << log2cb
	inBounds := x0+cb <= p.picW && y0+cb <= p.picH
	split := log2cb > int(p.sps.Log2MinCB)
	if inBounds && log2cb > int(p.sps.Log2MinCB) {
		v, err := p.splitCU(x0, y0, depth)
		if err != nil {
			return false, err
		}
		split = v
	} else if !inBounds {
		// Edge: flag absent, forced until the CB fits. The peer's
		// tab_ct_depth is only written at leaf CUs (set_ct_depth in
		// hls_coding_unit), so forced path writes nothing here.
		split = log2cb > int(p.sps.Log2MinCB)
	}
	if p.pps.CUQPDelta && log2cb >= p.ctb-int(p.pps.DiffQPDelayDepth) {
		p.qpCoded = false
	}
	if split {
		half := cb >> 1
		x1, y1 := x0+half, y0+half
		more, err := p.quadtree(x0, y0, log2cb-1, depth+1)
		if err != nil {
			return false, err
		}
		if more && x1 < p.picW {
			more, err = p.quadtree(x1, y0, log2cb-1, depth+1)
			if err != nil {
				return false, err
			}
		}
		if more && y1 < p.picH {
			more, err = p.quadtree(x0, y1, log2cb-1, depth+1)
			if err != nil {
				return false, err
			}
		}
		if more && x1 < p.picW && y1 < p.picH {
			more, err = p.quadtree(x1, y1, log2cb-1, depth+1)
			if err != nil {
				return false, err
			}
		}
		if !more {
			return false, nil
		}
		// more==true: the peer reports whether CTUs remain to the
		// right/below; the CTB loop continues while both hold.
		return (x1+half) < p.picW || (y1+half) < p.picH, nil
	}
	if err := p.codingUnit(x0, y0, log2cb); err != nil {
		return false, err
	}
	p.fillDepth(x0, y0, 1<<log2cb, depth)
	ctbSize := 1 << p.ctb
	xEnd := (x0+cb)%ctbSize == 0 || x0+cb >= p.picW
	yEnd := (y0+cb)%ctbSize == 0 || y0+cb >= p.picH
	if xEnd && yEnd {
		// CTB edge: read end_of_slice_flag (0 = more CTUs, 1 = end).
		// The peer decodes the flag but only honors it at CTB edges;
		// middle CTUs continue while the flag says so.
		more := p.cab.terminate() == 0
		return more, nil
	}
	return true, nil
}

// splitCU reads split_coding_unit_flag with the depth-map context.
// Peer: cabac.c ff_hevc_split_coding_unit_flag_decode (x0b/y0b edge
// masks + tab_ct_depth neighbours + depth compare). Zero-filled
// neighbours (unparsed CTBs) compare false, same as the peer.
func (p *synParser) splitCU(x0, y0, depth int) (bool, error) {
	xcb := x0 / p.minCB
	ycb := y0 / p.minCB
	inc := 0
	if xcb > 0 && int(p.depth[ycb*p.gridW+xcb-1]) > depth {
		inc++
	}
	if ycb > 0 && int(p.depth[(ycb-1)*p.gridW+xcb]) > depth {
		inc++
	}
	v, err := p.bin(ctxSplitCodingUnitFlag + inc)
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

// codingUnit parses one CU: transquant-bypass flag first,
// then the intra path (I slices) or the inter path (P slices;
// intra CUs inside P reuse the intra body below).
func (p *synParser) codingUnit(x0, y0, log2cb int) error {
	var cu CUInfo
	cu.X0, cu.Y0, cu.Log2Size = x0, y0, log2cb
	if p.pps.TransquantBypass {
		// Peer reads the flag but our PPS gate already refused
		// bypass-enabled clips... except this clip disables the
		// tool (trace transquant_bypass 0), so no bin rides here.
		// Kept as a guard for foreign clips.
		return fmt.Errorf("%w: transquant bypass tool on", ErrBadSlice)
	}
	if p.sh.Type != SliceI {
		return p.codingUnitInter(x0, y0, log2cb, &cu)
	}
	return p.codingUnitIntra(x0, y0, log2cb, &cu)
}

// codingUnitIntra is the I-slice body: part mode, luma/chroma modes,
// then the transform tree. Untouched by the inter stage.
func (p *synParser) codingUnitIntra(x0, y0, log2cb int, cu *CUInfo) error {
	cu.Pred = PredIntra
	p.curPredIntra = true
	// I slices are always intra; part mode splits only at min CB.
	cu.Part = Part2Nx2N
	if log2cb == int(p.sps.Log2MinCB) {
		v, err := p.partMode(log2cb)
		if err != nil {
			return err
		}
		cu.Part = v
	}
	split := cu.Part == PartNxN
	nSide := 1
	if split {
		nSide = 2
	}
	// Luma modes: peer reads all prev flags first, then the per-PB
	// mpm/rem in raster (two loops); a single interleaved loop would
	// misorder bins for NxN CUs.
	prev := [4]int{}
	for i := 0; i < nSide; i++ {
		for j := 0; j < nSide; j++ {
			v, err := p.bin(ctxPrevIntraLumaPredFlag)
			if err != nil {
				return err
			}
			prev[2*i+j] = v
		}
	}
	for i := 0; i < nSide; i++ {
		for j := 0; j < nSide; j++ {
			var mode uint8
			if prev[2*i+j] != 0 {
				m, err := p.mpmIdx()
				if err != nil {
					return err
				}
				mode = p.deriveMPM(x0+j*(1<<(log2cb-nSide+1)), y0+i*(1<<(log2cb-nSide+1)), m)
			} else {
				r, err := p.bypassN(5)
				if err != nil {
					return err
				}
				mode = p.deriveREM(x0+j*(1<<(log2cb-nSide+1)), y0+i*(1<<(log2cb-nSide+1)), uint8(r))
			}
			cu.Luma[2*i+j] = mode
			p.fillIPM(x0+j*(1<<(log2cb-nSide+1)), y0+i*(1<<(log2cb-nSide+1)), 1<<(log2cb-nSide+1), mode)
		}
	}
	// Chroma mode once per CU (4:2:0).
	v, err := p.bin(ctxIntraChromaPredMode)
	if err != nil {
		return err
	}
	cm := uint8(4)
	if v != 0 {
		b, err := p.bypassN(2)
		if err != nil {
			return err
		}
		cm = uint8(b)
	}
	cu.Chroma = cm
	luma0 := cu.Luma[0]
	table := [4]uint8{0, 26, 10, 1}
	if cm != 4 {
		if luma0 == table[cm] {
			cu.ChromaLuma = 34
		} else {
			cu.ChromaLuma = table[cm]
		}
	} else {
		cu.ChromaLuma = luma0
	}
	p.frame.CUs = append(p.frame.CUs, *cu)
	// Transform tree over the whole CB; depth limit is the intra
	// depth (peer max_trafo_depth; NxN CUs add one, kept below).
	maxDepth := int(p.sps.MaxTIDepthIntra)
	if split {
		maxDepth++
	}
	p.curMaxDepth = maxDepth
	p.curSplit = split
	return p.transformTree(x0, y0, x0, y0, log2cb, log2cb, 0, 0, [2]bool{}, [2]bool{})
}

// codingUnitInter parses one P-slice CU: skip flag, then pred/part,
// then per-PU merge or motion data, then the residual gate.
// Peer (read-only): hevcdec.c hls_coding_unit inter branch +
// hls_prediction_unit (skip/merge/mvp call order) + cabac.c skip,
// pred_mode, part_mode, merge, ref_idx, mvd, mvp, no_residual decoders.
// Only bin order and context indices travel; merge/MVP derivation
// and motion compensation ride in the pixel stage.
func (p *synParser) codingUnitInter(x0, y0, log2cb int, cu *CUInfo) error {
	cb := 1 << log2cb
	// Skip flag with left/above contexts (single-slice picture:
	// neighbours are available off the picture edge only).
	sk, err := p.skipFlag(x0, y0)
	if err != nil {
		return err
	}
	p.fillSkip(x0, y0, cb, sk)
	if sk {
		cu.Pred = PredSkip
		p.curPredIntra = false
		pu := PUInfo{X0: x0, Y0: y0, W: cb, H: cb, Merge: true}
		if p.sh.MergeCand > 1 {
			m, err := p.mergeIdx()
			if err != nil {
				return err
			}
			pu.MergeIdx = m
		}
		cu.PUs = []PUInfo{pu}
		p.fillIPM(x0, y0, cb, IntraDC)
		p.frame.CUs = append(p.frame.CUs, *cu)
		// Skip CUs carry no residual bins at all (peer returns
		// right after the prediction unit + boundary strengths).
		p.curMaxDepth = 0
		return nil
	}
	// Pred mode: 1 = intra (reuses the intra body), 0 = inter.
	pm, err := p.bin(ctxPredModeFlag)
	if err != nil {
		return err
	}
	if pm != 0 {
		return p.codingUnitIntra(x0, y0, log2cb, cu)
	}
	cu.Pred = PredInter
	p.curPredIntra = false
	part, err := p.partModeInter(log2cb)
	if err != nil {
		return err
	}
	cu.Part = part
	rects := interPURects(x0, y0, cb, part)
	if len(rects) == 0 {
		return fmt.Errorf("%w: inter part %d", ErrBadSlice, part)
	}
	for _, r := range rects {
		pu := PUInfo{X0: r[0], Y0: r[1], W: r[2], H: r[3]}
		mf, err := p.bin(ctxMergeFlag)
		if err != nil {
			return err
		}
		if mf != 0 {
			pu.Merge = true
			if p.sh.MergeCand > 1 {
				m, err := p.mergeIdx()
				if err != nil {
					return err
				}
				pu.MergeIdx = m
			}
			cu.PUs = append(cu.PUs, pu)
			continue
		}
		// AMVP path (P slices: L0 only, no inter_pred_idc on wire).
		if p.sh.RefL0 > 1 {
			ri, err := p.refIdxL0(int(p.sh.RefL0))
			if err != nil {
				return err
			}
			pu.RefIdx = ri
		}
		dx, dy, err := p.mvdCoding()
		if err != nil {
			return err
		}
		pu.MVDX, pu.MVDY = dx, dy
		mv, err := p.bin(ctxMvpLxFlag)
		if err != nil {
			return err
		}
		pu.MVPFlag = uint8(mv)
		cu.PUs = append(cu.PUs, pu)
	}
	p.fillIPM(x0, y0, cb, IntraDC)
	// Residual gate (peer hls_coding_unit): the flag rides only when
	// the CU is inter and NOT single-PU merge 2Nx2N. Intra reuses the
	// intra body (no flag, tree follows); single-PU merge 2Nx2N also
	// skips the flag with the tree following (rqt_root_cbf=1 init).
	// Flag value 1 means the tree follows, 0 means bare.
	rqtRoot := true
	if !(cu.Part == Part2Nx2N && len(cu.PUs) == 1 && cu.PUs[0].Merge) {
		nr, err := p.bin(ctxNoResidualDataFlag)
		if err != nil {
			return err
		}
		rqtRoot = nr != 0
	}
	if !rqtRoot {
		cu.NoResid = true
		p.frame.CUs = append(p.frame.CUs, *cu)
		p.curMaxDepth = 0
		return nil
	}
	p.frame.CUs = append(p.frame.CUs, *cu)
	p.curMaxDepth = int(p.sps.MaxTIDepthInter)
	p.curSplit = false
	return p.transformTree(x0, y0, x0, y0, log2cb, log2cb, 0, 0, [2]bool{}, [2]bool{})
}

// skipFlag reads cu_skip_flag with left/above contexts. The peer's
// x0/y0 are CTB-relative (av_zero_extend), so the left neighbour is
// gated by the in-CTB offset OR the left-CTB flag, not by the
// picture edge alone. H265SKIPDBG=1 traces (x0,y0,inc,value).
func (p *synParser) skipFlag(x0, y0 int) (bool, error) {
	ctbMask := (1 << p.ctb) - 1
	x0b, y0b := x0&ctbMask, y0&ctbMask
	rx, ry := x0>>p.ctb, y0>>p.ctb
	xcb, ycb := x0/p.minCB, y0/p.minCB
	inc := 0
	if rx > 0 || x0b != 0 {
		if xcb > 0 && p.skip[ycb*p.gridW+xcb-1] != 0 {
			inc++
		}
	}
	if ry > 0 || y0b != 0 {
		if ycb > 0 && p.skip[(ycb-1)*p.gridW+xcb] != 0 {
			inc++
		}
	}
	v, err := p.bin(ctxSkipFlag + inc)
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

// fillSkip stores the skip flag over the CB's min-CB cells.
func (p *synParser) fillSkip(x0, y0, size int, sk bool) {
	var v uint8
	if sk {
		v = 1
	}
	for y := y0; y < y0+size && y < p.picH; y += p.minCB {
		for x := x0; x < x0+size && x < p.picW; x += p.minCB {
			if x >= 0 && y >= 0 {
				p.skip[(y/p.minCB)*p.gridW+x/p.minCB] = v
			}
		}
	}
}

// partModeInter reads part_mode for inter CUs (peer
// ff_hevc_part_mode_decode; our SPS disables AMP, other clips with
// AMP sizes refuse honestly below).
func (p *synParser) partModeInter(log2cb int) (uint8, error) {
	v, err := p.bin(ctxPartMode)
	if err != nil {
		return 0, err
	}
	if v != 0 {
		return Part2Nx2N, nil
	}
	if log2cb == int(p.sps.Log2MinCB) {
		v, err := p.bin(ctxPartMode + 1)
		if err != nil {
			return 0, err
		}
		if v != 0 {
			return Part2NxN, nil
		}
		if log2cb == 3 {
			return PartNx2N, nil
		}
		v, err = p.bin(ctxPartMode + 2)
		if err != nil {
			return 0, err
		}
		if v != 0 {
			return PartNx2N, nil
		}
		return PartNxN, nil
	}
	if !p.sps.AMPEnabled {
		v, err := p.bin(ctxPartMode + 1)
		if err != nil {
			return 0, err
		}
		if v != 0 {
			return Part2NxN, nil
		}
		return PartNx2N, nil
	}
	return 0, fmt.Errorf("%w: AMP part modes", ErrBadSlice)
}

// interPURects splits a CB into PU rects [x,y,w,h] for one part mode.
func interPURects(x0, y0, cb int, part uint8) [][4]int {
	switch part {
	case Part2Nx2N:
		return [][4]int{{x0, y0, cb, cb}}
	case Part2NxN:
		return [][4]int{{x0, y0, cb, cb / 2}, {x0, y0 + cb/2, cb, cb / 2}}
	case PartNx2N:
		return [][4]int{{x0, y0, cb / 2, cb}, {x0 + cb/2, y0, cb / 2, cb}}
	case PartNxN:
		h := cb / 2
		return [][4]int{
			{x0, y0, h, h}, {x0 + h, y0, h, h},
			{x0, y0 + h, h, h}, {x0 + h, y0 + h, h, h},
		}
	}
	return nil
}

// mergeIdx reads merge_idx (first bin CABAC, remainder bypass).
func (p *synParser) mergeIdx() (uint8, error) {
	v, err := p.bin(ctxMergeIdx)
	if err != nil {
		return 0, err
	}
	i := v
	for i != 0 && i < int(p.sh.MergeCand)-1 {
		if p.bypass() == 0 {
			break
		}
		i++
	}
	return uint8(i), nil
}

// refIdxL0 reads ref_idx_l0 (contexts for the first two, bypass rest).
func (p *synParser) refIdxL0(numRefs int) (uint8, error) {
	max := numRefs - 1
	mc := max
	if mc > 2 {
		mc = 2
	}
	i := 0
	for i < mc {
		v, err := p.bin(ctxRefIdxL0 + i)
		if err != nil {
			return 0, err
		}
		if v == 0 {
			return uint8(i), nil
		}
		i++
	}
	if i == 2 {
		for i < max {
			if p.bypass() == 0 {
				break
			}
			i++
		}
	}
	return uint8(i), nil
}

// mvdCoding reads one PU's motion-vector difference (peer
// ff_hevc_hls_mvd_coding order: x/y greater0, greater1, then
// magnitude/sign bypass per component).
func (p *synParser) mvdCoding() (int32, int32, error) {
	x0, err := p.bin(ctxAbsMvdGreater0Flag)
	if err != nil {
		return 0, 0, err
	}
	y0, err := p.bin(ctxAbsMvdGreater0Flag)
	if err != nil {
		return 0, 0, err
	}
	x, y := x0, y0
	if x != 0 {
		// Peer reads greater1 on the second context of its pair
		// (OFFSET+1), same fixed ctx for both components.
		g, err := p.bin(ctxAbsMvdGreater1Flag + 1)
		if err != nil {
			return 0, 0, err
		}
		x += g
	}
	if y != 0 {
		g, err := p.bin(ctxAbsMvdGreater1Flag + 1)
		if err != nil {
			return 0, 0, err
		}
		y += g
	}
	dx, err := p.mvdComponent(x)
	if err != nil {
		return 0, 0, err
	}
	dy, err := p.mvdComponent(y)
	if err != nil {
		return 0, 0, err
	}
	return dx, dy, nil
}

// mvdComponent reads one signed MVD component from its class.
// Class 1 is a single sign bypass (0->+1, 1->-1). Class 2 is the
// bypass Exp-Golomb: ret starts 2, prefix ones at k=1.. add 1<<k,
// then k suffix bits, then the sign bypass (peer's mvd_decode).
func (p *synParser) mvdComponent(cls int) (int32, error) {
	switch cls {
	case 0:
		return 0, nil
	case 1:
		if p.bypass() != 0 {
			return -1, nil
		}
		return 1, nil
	}
	ret := 2
	k := 1
	for k < 31 && p.bypass() != 0 {
		ret += 1 << uint(k)
		k++
	}
	if k == 31 {
		return 0, fmt.Errorf("%w: mvd overflow", ErrBadSlice)
	}
	for k--; k >= 0; k-- {
		ret += p.bypass() << uint(k)
	}
	if p.bypass() != 0 {
		return -int32(ret), nil
	}
	return int32(ret), nil
}

// qpAllows reports whether a CU may code cu_qp_delta (QP block grid).
func (p *synParser) qpAllows(x0, y0, log2cb int) bool {
	qg := p.ctb - int(p.pps.DiffQPDelayDepth)
	if qg < 0 {
		return false
	}
	q := 1 << uint(qg)
	_ = log2cb
	return x0%q == 0 && y0%q == 0
}

// partMode reads part_mode (intra path; AMP branch kept for shape).
func (p *synParser) partMode(log2cb int) (uint8, error) {
	v, err := p.bin(ctxPartMode)
	if err != nil {
		return 0, err
	}
	if v != 0 {
		return Part2Nx2N, nil
	}
	if log2cb == int(p.sps.Log2MinCB) {
		return PartNxN, nil
	}
	v, err = p.bin(ctxPartMode + 1)
	if err != nil {
		return 0, err
	}
	if v != 0 {
		return Part2NxN, nil
	}
	return PartNx2N, nil
}

// mpmIdx reads the 2-bin bypass MPM index.
func (p *synParser) mpmIdx() (uint8, error) {
	var i uint8
	for i < 2 && p.bypass() != 0 {
		i++
	}
	return i, nil
}

// deriveMPM builds the 3-candidate list from left/above modes.
func (p *synParser) deriveMPM(x0, y0 int, idx uint8) uint8 {
	left := uint8(IntraDC)
	above := uint8(IntraDC)
	if x0 > 0 {
		left = p.ipmAt(x0-p.minPU, y0)
	}
	yCTB := (y0 >> p.ctb) << p.ctb
	if y0 > 0 && y0-1 >= yCTB {
		above = p.ipmAt(x0, y0-p.minPU)
	}
	var cand [3]uint8
	if left == above {
		if left < 2 {
			cand = [3]uint8{IntraPlanar, IntraDC, IntraAng26}
		} else {
			cand[0] = left
			cand[1] = 2 + uint8((int(left)-2-1+32)&31)
			cand[2] = 2 + uint8((int(left)-2+1)&31)
		}
	} else {
		cand[0], cand[1] = left, above
		switch {
		case cand[0] != IntraPlanar && cand[1] != IntraPlanar:
			cand[2] = IntraPlanar
		case cand[0] != IntraDC && cand[1] != IntraDC:
			cand[2] = IntraDC
		default:
			cand[2] = IntraAng26
		}
	}
	if idx > 2 {
		return IntraDC
	}
	return cand[idx]
}

// deriveREM maps a 5-bit remainder past the sorted candidates.
func (p *synParser) deriveREM(x0, y0 int, rem uint8) uint8 {
	left := uint8(IntraDC)
	above := uint8(IntraDC)
	if x0 > 0 {
		left = p.ipmAt(x0-p.minPU, y0)
	}
	yCTB := (y0 >> p.ctb) << p.ctb
	if y0 > 0 && y0-1 >= yCTB {
		above = p.ipmAt(x0, y0-p.minPU)
	}
	var cand [3]uint8
	if left == above {
		if left < 2 {
			cand = [3]uint8{IntraPlanar, IntraDC, IntraAng26}
		} else {
			cand[0] = left
			cand[1] = 2 + uint8((int(left)-2-1+32)&31)
			cand[2] = 2 + uint8((int(left)-2+1)&31)
		}
	} else {
		cand[0], cand[1] = left, above
		switch {
		case cand[0] != IntraPlanar && cand[1] != IntraPlanar:
			cand[2] = IntraPlanar
		case cand[0] != IntraDC && cand[1] != IntraDC:
			cand[2] = IntraDC
		default:
			cand[2] = IntraAng26
		}
	}
	if cand[0] > cand[1] {
		cand[0], cand[1] = cand[1], cand[0]
	}
	if cand[0] > cand[2] {
		cand[0], cand[2] = cand[2], cand[0]
	}
	if cand[1] > cand[2] {
		cand[1], cand[2] = cand[2], cand[1]
	}
	m := rem
	for i := 0; i < 3; i++ {
		if m >= cand[i] {
			m++
		}
	}
	return m
}

func (p *synParser) ipmAt(x, y int) uint8 {
	if x < 0 || y < 0 || x >= p.picW || y >= p.picH {
		return IntraDC
	}
	return p.ipm[(y/p.minPU)*p.puW+x/p.minPU]
}

func (p *synParser) fillIPM(x0, y0, size int, mode uint8) {
	for y := y0; y < x0*0+y0+size && y < p.picH; y += p.minPU {
		for x := x0; x < x0+size && x < p.picW; x += p.minPU {
			if x >= 0 && y >= 0 {
				p.ipm[(y/p.minPU)*p.puW+x/p.minPU] = mode
			}
		}
	}
}

// fillDepth stores the CT depth over the in-picture part of a CB
// (peer set_ct_depth shape; edge-forced splits included).
func (p *synParser) fillDepth(x0, y0, size, depth int) {
	for y := y0; y < y0+size && y < p.picH; y += p.minCB {
		for x := x0; x < x0+size && x < p.picW; x += p.minCB {
			if x >= 0 && y >= 0 {
				p.depth[(y/p.minCB)*p.gridW+x/p.minCB] = uint8(depth)
			}
		}
	}
}

// transformTree walks the TU quadtree storing coefficient blocks.
// Coding-unit origin (cbX,cbY) rides along for the small-TU chroma path.
func (p *synParser) transformTree(x0, y0, xBase, yBase, log2cb, log2t, depth, blk int, cbfCB, cbfCR [2]bool) error {
	split := false
	// Peer gate (hls_transform_tree): split bin only when the TU fits
	// in the max, exceeds the min, stays under the CU's depth and is
	// not the forced NxN depth-0 split. Inter adds one forced case:
	// with zero inter depth the split bin never rides and the TU
	// splits iff the CB uses a non-2Nx2N part (peer's inter_split).
	interSplit := !p.curPredIntra && depth == 0 && p.sps.MaxTIDepthInter == 0 &&
		len(p.frame.CUs) > 0 && p.frame.CUs[len(p.frame.CUs)-1].Part != Part2Nx2N
	if log2t <= int(p.sps.Log2MaxTB) && log2t > int(p.sps.Log2MinTB) &&
		depth < p.curMaxDepth && !(p.curSplit && depth == 0) && !interSplit {
		v, err := p.bin(ctxSplitTransformFlag + 5 - log2t)
		if err != nil {
			return err
		}
		split = v != 0
	} else if log2t > int(p.sps.Log2MaxTB) || (p.curSplit && depth == 0) || interSplit {
		split = true
	}
	if p.sps.ChromaFormat != 0 && (log2t > 2 || p.sps.ChromaFormat == 3) {
		if depth == 0 || cbfCB[0] {
			v, err := p.bin(ctxCbfCbCr + depth)
			if err != nil {
				return err
			}
			cbfCB[0] = v != 0
			if p.sps.ChromaFormat == 2 && (!split && log2t == 3) {
				// 4:2:2 second chroma block (absent on our path).
				return fmt.Errorf("%w: 422 second chroma block", ErrBadSlice)
			}
		}
		if depth == 0 || cbfCR[0] {
			v, err := p.bin(ctxCbfCbCr + depth)
			if err != nil {
				return err
			}
			cbfCR[0] = v != 0
			if p.sps.ChromaFormat == 2 && (!split && log2t == 3) {
				return fmt.Errorf("%w: 422 second chroma block", ErrBadSlice)
			}
		}
	}
	if split {
		half := 1 << (log2t - 1)
		for i, pos := range [][2]int{{x0, y0}, {x0 + half, y0}, {x0, y0 + half}, {x0 + half, y0 + half}} {
			if err := p.transformTree(pos[0], pos[1], xBase, yBase, log2cb, log2t-1, depth+1, i, cbfCB, cbfCR); err != nil {
				return err
			}
		}
		return nil
	}
	// Luma flag: intra path always codes it, matching the peer's
	// pred_mode gate (intra, or inter with depth/cbf cause).
	// Peer: pred_mode==INTRA || trafo_depth!=0 || cbf_cb || cbf_cr.
	// When the gate is shut (inter depth-0 all-zero chroma) the
	// value stays 1 without consuming a bin (peer's cbf_luma=1
	// init); the transform below then runs with cbf 1/0/0.
	needLuma := p.curPredIntra || depth != 0 || cbfCB[0] || cbfCR[0]
	cbfLuma := true
	if needLuma {
		v, err := p.bin(ctxCbfLuma + boolToInt(depth == 0))
		if err != nil {
			return err
		}
		cbfLuma = v != 0
	}
	return p.transformUnit(x0, y0, xBase, yBase, log2cb, log2t, blk, cbfLuma, cbfCB[0], cbfCR[0])
}

// transformUnit parses one TU: the QP delta rides here (first TU
// with coefficients codes it), then one residual block per cbf.
// Chroma TUs share the luma origin (peer passes xBase/yBase through;
// the 4:2:0 subsample applies at prediction/placement, not syntax).
// blk selects the small-TU chroma path (parsed once at blk 3).
func (p *synParser) transformUnit(x0, y0, xBase, yBase, log2cb, log2t, blk int, cbfLuma, cbfCb, cbfCr bool) error {
	// Peer codes the delta in the first TU with coefficients per QP
	// group (is_coded reset in quadtree for large CUs); no position
	// gate rides here.
	if (cbfLuma || cbfCb || cbfCr) && p.pps.CUQPDelta && !p.qpCoded {
		abs, err := p.qpDeltaAbs()
		if err != nil {
			return err
		}
		d := int32(abs)
		if d != 0 {
			if p.bypass() != 0 {
				d = -d
			}
		}
		if d < -26 || d > 25 {
			return fmt.Errorf("%w: cu qp delta %d", ErrBadSlice, d)
		}
		p.qpY = p.qpY + d
		p.qpCoded = true
		_ = xBase
		_ = yBase
		_ = log2cb
	}
	// Every leaf lands in the leaf table (all-zero leaves carry no
	// TUInfo, but the pixel stage still predicts them). Modes mirror
	// the peer's tu state: luma from the covering PU, chroma from the
	// CU's mapped mode; QP is the value in force for this TU.
	// Inter leaves carry mode 255 (no intra direction; the motion
	// stage owns them) and always scan diagonal.
	cu := p.frame.CUs[len(p.frame.CUs)-1]
	lm := p.intraLumaMode(x0, y0, log2t)
	if !p.curPredIntra {
		lm = 255
	}
	p.frame.Leaves = append(p.frame.Leaves, TULeaf{
		X0: x0, Y0: y0, CbX: xBase, CbY: yBase, Log2Size: log2t, Blk: blk,
		CbfLuma: cbfLuma, CbfCb: cbfCb, CbfCr: cbfCr,
		LumaMode: lm, ChromaMode: cu.ChromaLuma,
		QP: p.qpY,
	})
	if cbfLuma {
		coeffs, err := p.residual(x0, y0, log2t, p.scanFor(log2t, 0, lm), 0)
		if err != nil {
			return err
		}
		p.frame.TUs = append(p.frame.TUs, TUInfo{X0: x0, Y0: y0, Log2Size: log2t, CIdx: 0, QP: p.qpY, Coeffs: coeffs})
	}
	if p.sps.ChromaFormat != 0 && log2t > 2 {
		lc := p.frame.CUs[len(p.frame.CUs)-1].ChromaLuma
		// Peer gates the chroma scan on the luma TU size (log2t),
		// not the subsampled chroma size.
		if cbfCb {
			coeffs, err := p.residual(x0, y0, log2t-1, p.scanFor(log2t, 1, lc), 1)
			if err != nil {
				return err
			}
			p.frame.TUs = append(p.frame.TUs, TUInfo{X0: x0, Y0: y0, Log2Size: log2t - 1, CIdx: 1, QP: p.qpY, Coeffs: coeffs})
		}
		if cbfCr {
			coeffs, err := p.residual(x0, y0, log2t-1, p.scanFor(log2t, 2, lc), 2)
			if err != nil {
				return err
			}
			p.frame.TUs = append(p.frame.TUs, TUInfo{X0: x0, Y0: y0, Log2Size: log2t - 1, CIdx: 2, QP: p.qpY, Coeffs: coeffs})
		}
	}
	// Small-TU chroma (log2t 2, 4x4 luma): the single 4x4 chroma block
	// rides on the CU origin and is parsed once (peer blk_idx 3 path).
	if p.sps.ChromaFormat == 1 && log2t == 2 && blk == 3 {
		lc := p.frame.CUs[len(p.frame.CUs)-1].ChromaLuma
		if cbfCb {
			coeffs, err := p.residual(xBase, yBase, 2, p.scanFor(2, 1, lc), 1)
			if err != nil {
				return err
			}
			p.frame.TUs = append(p.frame.TUs, TUInfo{X0: xBase, Y0: yBase, Log2Size: 2, CIdx: 1, QP: p.qpY, Coeffs: coeffs})
		}
		if cbfCr {
			coeffs, err := p.residual(xBase, yBase, 2, p.scanFor(2, 2, lc), 2)
			if err != nil {
				return err
			}
			p.frame.TUs = append(p.frame.TUs, TUInfo{X0: xBase, Y0: yBase, Log2Size: 2, CIdx: 2, QP: p.qpY, Coeffs: coeffs})
		}
	}
	return nil
}

// intraLumaMode reads the PU mode covering (x0,y0) for scan choice.
func (p *synParser) intraLumaMode(x0, y0, log2t int) uint8 {
	_ = log2t
	return p.ipmAt(x0, y0)
}

// scanFor picks the scan for one TU: inter blocks always ride
// diagonal (peer keeps SCAN_DIAG unless intra small-block modes
// redirect); intra defers to the mode map.
func (p *synParser) scanFor(log2t, cIdx int, mode uint8) int {
	if !p.curPredIntra {
		return ScanDiag
	}
	return p.scanIdx(mode, log2t, cIdx)
}

// scanIdx picks the scan for an intra TU (small blocks follow the mode).
func (p *synParser) scanIdx(mode uint8, log2t, cIdx int) int {
	if log2t >= 4 {
		return ScanDiag
	}
	_ = cIdx
	if mode >= 6 && mode <= 14 {
		return ScanVert
	}
	if mode >= 22 && mode <= 30 {
		return ScanHoriz
	}
	return ScanDiag
}

// qpDeltaAbs reads cu_qp_delta_abs (prefix CABAC, suffix bypass).
func (p *synParser) qpDeltaAbs() (uint32, error) {
	var prefix uint32
	inc := 0
	for prefix < 5 {
		v, err := p.bin(ctxCuQpDelta + inc)
		if err != nil {
			return 0, err
		}
		if v == 0 {
			break
		}
		prefix++
		inc = 1
	}
	if prefix < 5 {
		return prefix, nil
	}
	var suffix uint32
	k := 0
	for k < 7 && p.bypass() != 0 {
		suffix += 1 << uint(k)
		k++
	}
	if k == 7 {
		return 0, fmt.Errorf("%w: qp delta overflow", ErrBadSlice)
	}
	for k > 0 {
		k--
		suffix += uint32(p.bypass()) << uint(k)
	}
	return prefix + suffix, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
