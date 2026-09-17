package h265

import (
	"errors"
	"fmt"
)

// Sentinel error for slice-header parsing (V2-2 step 2).
// It joins the KindH265 triage bucket with the header errors.
var ErrBadSlice = errors.New("h265: bad slice header")

// SliceRPS is an explicit reference set coded in the slice header
// (short_term_ref_pic_set_sps_flag 0). Predicted sets point back at
// the SPS list; our clip codes all sets inline.
type SliceRPS struct {
	Predicted bool
	DeltaIdx  uint32
	Neg       uint32
	Pos       uint32
	DeltaPOC  []int32
	Used      []bool
}

// SliceHeader is the parsed slice segment header: identity (PPS,
// type, POC), reference truth (RPS counts, default refs, weights)
// and quant/filter switches the pixel stage consumes.
// Peer (read-only): libavcodec/hevc/hevcdec.c:774 hls_slice_header
// (field order, IRAP/IDR gates, RPS/weight/qp/deblock tail) +
// :175 pred_weight_table + :decode_lt_rps + ps.c:2485
// ff_hevc_compute_poc2 (POC wrap). Multilayer (layer>0) honestly
// refuses; tiles/wavefront refuse like the PPS gate.
type SliceHeader struct {
	NALType     int
	First       bool
	NoOutput    bool
	PPSID       uint32
	Dependent   bool
	SliceAddr   uint32
	Type        uint32
	PicOutput   bool
	POCLsb      uint32
	POC         int
	RPSSPSFlag  bool
	SPSRPSIdx   uint32
	RPS         *SliceRPS
	LTRefs      uint32
	TemporalMVP bool
	SAOLuma     bool
	SAOChroma   bool
	RefL0       uint32
	RefL1       uint32
	Override    bool
	// Weight table truth (PPS weighted flags + P/B slices only).
	LumaDenom    uint32
	ChromaDenom  uint32
	L0LumaUsed   []bool
	L0ChromaUsed []bool
	L1LumaUsed   []bool
	L1ChromaUsed []bool
	MergeCand    uint32
	QpDelta      int32
	SliceQP      int32
	LoopAcross   bool
	DataOffset   int // bit offset where the header ends (CABAC starts)
}

// ceilLog2 mirrors av_ceil_log2 for slice address length.
func ceilLog2(v uint32) int {
	n := 0
	for k := uint32(1); k < v; k <<= 1 {
		n++
	}
	return n
}

// computePOC wraps the POC lsb with the previous picture's TICKS value
// (pocTid0). Peer: ps.c:2485 ff_hevc_compute_poc2.
func computePOC(log2Max uint32, pocTid0, pocLsb, nalType int) int {
	maxLsb := 1 << log2Max
	prevLsb := pocTid0 % maxLsb
	prevMsb := pocTid0 - prevLsb
	var msb int
	switch {
	case pocLsb < prevLsb && prevLsb-pocLsb >= maxLsb/2:
		msb = prevMsb + maxLsb
	case pocLsb > prevLsb && pocLsb-prevLsb > maxLsb/2:
		msb = prevMsb - maxLsb
	default:
		msb = prevMsb
	}
	if nalType >= NALBlaWLp && nalType <= NALBlaNLp {
		msb = 0
	}
	return msb + pocLsb
}

// ParseSliceHeader parses one coded slice NALU (2-byte HEVC header
// included) against the known parameter sets. pocTid0 carries the
// previous picture's POC (0 after an IDR); it only matters for POC
// wrap on long clips. Multi-layer and dependent slices refuse with a
// namable error; tiles/wavefront already refuse at the PPS gate.
func ParseSliceHeader(nalu []byte, ps *ParamSets, pocTid0 int) (*SliceHeader, error) {
	typ, layer, _, err := nalHeader2(nalu)
	if err != nil {
		return nil, err
	}
	if !IsSlice(typ) {
		return nil, fmt.Errorf("%w: nal %d (%s) is not a slice", ErrBadSlice, typ, NALName(typ))
	}
	if layer != 0 {
		return nil, fmt.Errorf("%w: layer %d not supported", ErrBadSlice, layer)
	}
	r := NewReader(UnescapeRBSP(nalu[2:]))
	sh := &SliceHeader{NALType: typ}
	bit := func(name string) (bool, error) {
		b, err := r.ReadBit()
		if err != nil {
			return false, fmt.Errorf("%w: %s: %v", ErrBadSlice, name, err)
		}
		return b != 0, nil
	}
	first, err := bit("first slice")
	if err != nil {
		return nil, err
	}
	sh.First = first
	if IsIRAP(typ) {
		noop, err := bit("no output")
		if err != nil {
			return nil, err
		}
		sh.NoOutput = noop
	}
	ppsID, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: pps id: %v", ErrBadSlice, err)
	}
	if ppsID >= 64 {
		return nil, fmt.Errorf("%w: pps id %d", ErrBadSlice, ppsID)
	}
	sh.PPSID = ppsID
	if ps == nil {
		return nil, fmt.Errorf("%w: pps %d", ErrMissingPPS, ppsID)
	}
	q, ok := ps.PPS[ppsID]
	if !ok {
		return nil, fmt.Errorf("%w: pps %d", ErrMissingPPS, ppsID)
	}
	s, ok := ps.SPS[q.SPSID]
	if !ok {
		return nil, fmt.Errorf("%w: sps %d for pps %d", ErrMissingSPS, q.SPSID, ppsID)
	}
	if _, ok := ps.VPS[s.VPSID]; !ok {
		return nil, fmt.Errorf("%w: vps %d for sps %d", ErrMissingVPS, s.VPSID, s.ID)
	}
	if !first {
		if q.DependentSlices {
			dep, err := bit("dependent slice")
			if err != nil {
				return nil, err
			}
			sh.Dependent = dep
			if dep {
				return nil, fmt.Errorf("%w: dependent slices not supported", ErrBadSlice)
			}
		}
		// CTB grid the address runs on (SPS block sizes).
		ctb := uint32(1) << (s.Log2MinCB + (s.Log2MaxCB - s.Log2MinCB))
		cw := (s.Width + ctb - 1) / ctb
		ch := (s.Height + ctb - 1) / ctb
		if n := ceilLog2(cw * ch); n > 0 {
			addr, err := r.ReadBits(n)
			if err != nil {
				return nil, fmt.Errorf("%w: slice address: %v", ErrBadSlice, err)
			}
			if addr >= cw*ch {
				return nil, fmt.Errorf("%w: slice address %d >= %d", ErrBadSlice, addr, cw*ch)
			}
			sh.SliceAddr = addr
		}
	} else if q.DependentSlices {
		// First slice of a picture is never dependent; the flag is
		// simply absent on this path.
		sh.Dependent = false
	}
	if sh.Dependent {
		return nil, fmt.Errorf("%w: dependent slices not supported", ErrBadSlice)
	}
	for i := uint32(0); i < q.NumExtraBits; i++ {
		if _, err := bit("reserved bit"); err != nil {
			return nil, err
		}
	}
	st, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: slice type: %v", ErrBadSlice, err)
	}
	if st > SliceI {
		return nil, fmt.Errorf("%w: slice type %d", ErrBadSlice, st)
	}
	sh.Type = st
	if IsIRAP(typ) && st != SliceI {
		return nil, fmt.Errorf("%w: inter slice in IRAP %s", ErrBadSlice, NALName(typ))
	}
	sh.PicOutput = true
	if q.OutputFlagPresent {
		out, err := bit("pic output")
		if err != nil {
			return nil, err
		}
		sh.PicOutput = out
	}
	if s.ChromaFormat == 3 {
		return nil, fmt.Errorf("%w: separate colour plane not supported", ErrBadSlice)
	}
	if !IsIDR(typ) {
		lsb, err := r.ReadBits(int(s.Log2MaxPOCLsb))
		if err != nil {
			return nil, fmt.Errorf("%w: poc lsb: %v", ErrBadSlice, err)
		}
		sh.POCLsb = lsb
		sh.POC = computePOC(s.Log2MaxPOCLsb, pocTid0, int(lsb), typ)
	} else {
		sh.POC, sh.POCLsb = 0, 0
	}
	if !IsIDR(typ) {
		spsFlag, err := bit("rps sps flag")
		if err != nil {
			return nil, err
		}
		sh.RPSSPSFlag = spsFlag
		if !spsFlag {
			rps, err := decodeSliceRPS(r, s, ps)
			if err != nil {
				return nil, err
			}
			sh.RPS = rps
		} else {
			if s.NumShortTermRPS == 0 {
				return nil, fmt.Errorf("%w: no RPS in SPS", ErrBadSlice)
			}
			nbits := ceilLog2(s.NumShortTermRPS)
			var idx uint32
			if nbits > 0 {
				v, err := r.ReadBits(nbits)
				if err != nil {
					return nil, fmt.Errorf("%w: rps idx: %v", ErrBadSlice, err)
				}
				idx = v
			}
			if idx >= s.NumShortTermRPS {
				return nil, fmt.Errorf("%w: rps idx %d", ErrBadSlice, idx)
			}
			sh.SPSRPSIdx = idx
		}
		nlt, err := decodeLongTermRPS(r, s)
		if err != nil {
			return nil, err
		}
		sh.LTRefs = nlt
		if s.TemporalMVP {
			mvp, err := bit("temporal mvp")
			if err != nil {
				return nil, err
			}
			sh.TemporalMVP = mvp
		}
	} else {
		sh.RPSSPSFlag = false
		sh.TemporalMVP = false
	}
	if s.SAOEnabled {
		luma, err := bit("sao luma")
		if err != nil {
			return nil, err
		}
		sh.SAOLuma = luma
		if s.ChromaFormat != 0 {
			chroma, err := bit("sao chroma")
			if err != nil {
				return nil, err
			}
			sh.SAOChroma = chroma
		}
	}
	sh.RefL0, sh.RefL1 = 0, 0
	if st == SliceP || st == SliceB {
		sh.RefL0 = q.RefL0Default
		if st == SliceB {
			sh.RefL1 = q.RefL1Default
		}
		over, err := bit("ref override")
		if err != nil {
			return nil, err
		}
		sh.Override = over
		if over {
			l0, err := r.ReadUE()
			if err != nil {
				return nil, fmt.Errorf("%w: ref l0: %v", ErrBadSlice, err)
			}
			sh.RefL0 = l0 + 1
			if st == SliceB {
				l1, err := r.ReadUE()
				if err != nil {
					return nil, fmt.Errorf("%w: ref l1: %v", ErrBadSlice, err)
				}
				sh.RefL1 = l1 + 1
			}
		}
		if sh.RefL0 >= 16 || sh.RefL1 >= 16 || sh.RefL0 == 0 {
			return nil, fmt.Errorf("%w: refs %d/%d", ErrBadSlice, sh.RefL0, sh.RefL1)
		}
		if st == SliceB && sh.RefL1 == 0 {
			return nil, fmt.Errorf("%w: no L1 refs for B slice", ErrBadSlice)
		}
		if q.ListsModPresent && sh.RefL0+sh.RefL1 > 1 {
			lm0, err := bit("lists mod l0")
			if err != nil {
				return nil, err
			}
			if lm0 {
				for i := uint32(0); i < sh.RefL0; i++ {
					if _, err := r.ReadBits(ceilLog2(sh.RefL0 + sh.RefL1)); err != nil {
						return nil, fmt.Errorf("%w: list l0 %d: %v", ErrBadSlice, i, err)
					}
				}
			}
			if st == SliceB {
				lm1, err := bit("lists mod l1")
				if err != nil {
					return nil, err
				}
				if lm1 {
					for i := uint32(0); i < sh.RefL1; i++ {
						if _, err := r.ReadBits(ceilLog2(sh.RefL0 + sh.RefL1)); err != nil {
							return nil, fmt.Errorf("%w: list l1 %d: %v", ErrBadSlice, i, err)
						}
					}
				}
			}
		}
		if st == SliceB {
			mvd, err := bit("mvd l1 zero")
			if err != nil {
				return nil, err
			}
			_ = mvd
		}
		if q.CabacInitPresent {
			cabac, err := bit("cabac init")
			if err != nil {
				return nil, err
			}
			_ = cabac
		}
		if sh.TemporalMVP {
			if st == SliceB {
				if _, err := bit("collocated from l0"); err != nil {
					return nil, err
				}
			}
			if sh.RefL0 > 1 {
				c, err := r.ReadUE()
				if err != nil {
					return nil, fmt.Errorf("%w: collocated idx: %v", ErrBadSlice, err)
				}
				if c >= sh.RefL0 {
					return nil, fmt.Errorf("%w: collocated idx %d", ErrBadSlice, c)
				}
			}
		}
		if (q.WeightedPred && st == SliceP) || (q.WeightedBiPred && st == SliceB) {
			if err := parseWeightTable(r, s, sh); err != nil {
				return nil, err
			}
		}
		five, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: merge cands: %v", ErrBadSlice, err)
		}
		if five > 4 {
			return nil, fmt.Errorf("%w: merge cands %d", ErrBadSlice, five)
		}
		sh.MergeCand = 5 - five
	}
	qp, err := r.ReadSE()
	if err != nil {
		return nil, fmt.Errorf("%w: qp delta: %v", ErrBadSlice, err)
	}
	sh.QpDelta = qp
	if q.SliceChromaOffsets {
		cb, err := r.ReadSE()
		if err != nil {
			return nil, fmt.Errorf("%w: cb offset: %v", ErrBadSlice, err)
		}
		cr, err := r.ReadSE()
		if err != nil {
			return nil, fmt.Errorf("%w: cr offset: %v", ErrBadSlice, err)
		}
		if cb < -12 || cb > 12 || cr < -12 || cr > 12 {
			return nil, fmt.Errorf("%w: chroma qp %d/%d", ErrBadSlice, cb, cr)
		}
	}
	if q.DeblockControl {
		return nil, fmt.Errorf("%w: deblock control not supported", ErrBadSlice)
	}
	// Loop filter across slices: present when SAO runs or deblocking
	// runs; ours always run SAO on this path.
	sh.LoopAcross = q.LoopAcrossSlices
	if q.LoopAcrossSlices && (sh.SAOLuma || sh.SAOChroma) {
		loop, err := bit("loop across slices")
		if err != nil {
			return nil, err
		}
		sh.LoopAcross = loop
	}
	if q.TilesEnabled || q.EntropySync {
		return nil, fmt.Errorf("%w: tiles/wavefront not supported", ErrBadSlice)
	}
	if q.SliceExtPresent || q.ExtPresent {
		return nil, fmt.Errorf("%w: slice extension not supported", ErrBadSlice)
	}
	one, err := bit("alignment one")
	if err != nil {
		return nil, err
	}
	if !one {
		return nil, fmt.Errorf("%w: alignment one is 0", ErrBadSlice)
	}
	// rbsp alignment zeros to the next byte; CABAC data starts there.
	for r.pos%8 != 0 {
		z, err := bit("alignment zero")
		if err != nil {
			return nil, err
		}
		if z {
			return nil, fmt.Errorf("%w: alignment zero is 1", ErrBadSlice)
		}
	}
	sh.DataOffset = r.pos
	if r.BitsLeft() < 0 {
		return nil, fmt.Errorf("%w: overread", ErrBadSlice)
	}
	// Inferred slice QP the pixel stage quantizes with.
	bdOff := int32((s.BitDepth - 8) * 6)
	sh.SliceQP = 26 + q.PicInitQP + qp
	if sh.SliceQP > 51 || sh.SliceQP < -bdOff {
		return nil, fmt.Errorf("%w: slice qp %d", ErrBadSlice, sh.SliceQP)
	}
	return sh, nil
}

// decodeSliceRPS reads an explicit short-term set in a slice header
// (is_slice_header 1: delta_idx form when predicted).
// Peer: ps.c:113 ff_hevc_decode_short_term_rps, slice path.
func decodeSliceRPS(r *Reader, s *SPS, ps *ParamSets) (*SliceRPS, error) {
	out := &SliceRPS{}
	predict := false
	if s.NumShortTermRPS != 0 {
		b, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: rps predict: %v", ErrBadSlice, err)
		}
		predict = b != 0
	}
	out.Predicted = predict
	if predict {
		d, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: rps delta idx: %v", ErrBadSlice, err)
		}
		if d+1 > s.NumShortTermRPS {
			return nil, fmt.Errorf("%w: rps delta idx %d", ErrBadSlice, d+1)
		}
		out.DeltaIdx = d + 1
		// Picture count comes from the referenced SPS set.
		ref := s.NumShortTermRPS - out.DeltaIdx
		_ = ref
		sign, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: rps delta sign: %v", ErrBadSlice, err)
		}
		_ = sign
		abs, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: rps abs delta: %v", ErrBadSlice, err)
		}
		if abs+1 > 32768 {
			return nil, fmt.Errorf("%w: rps abs delta %d", ErrBadSlice, abs+1)
		}
		// Predicted length needs the referenced set's count, which
		// step 3 decodes for real; slices pointing here refuse until
		// then so bytes never silently misalign.
		return nil, fmt.Errorf("%w: predicted slice RPS not supported", ErrBadSlice)
	}
	neg, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: rps neg: %v", ErrBadSlice, err)
	}
	pos, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: rps pos: %v", ErrBadSlice, err)
	}
	if neg >= 16 || pos >= 16 {
		return nil, fmt.Errorf("%w: rps pics %d+%d", ErrBadSlice, neg, pos)
	}
	out.Neg, out.Pos = neg, pos
	for i := uint32(0); i < neg+pos; i++ {
		d, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: rps poc %d: %v", ErrBadSlice, i, err)
		}
		if d+1 < 1 || d+1 > 32768 {
			return nil, fmt.Errorf("%w: rps poc %d bad", ErrBadSlice, i)
		}
		u, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: rps used %d: %v", ErrBadSlice, i, err)
		}
		// Deltas run negative-then-positive; keep them signed for
		// the step-3 reference builder.
		v := int32(d + 1)
		if i < neg {
			v = -v
		}
		out.DeltaPOC = append(out.DeltaPOC, v)
		out.Used = append(out.Used, u != 0)
	}
	return out, nil
}

// decodeLongTermRPS consumes the long-term tail and reports the total
// reference count. Clips without long-term pictures (ours) return 0
// at the SPS flag check, reading no bits.
// Peer: hevcdec.c decode_lt_rps (nb_sps/nb_sh + per-pic lsb/used/msb).
func decodeLongTermRPS(r *Reader, s *SPS) (uint32, error) {
	if s == nil {
		return 0, fmt.Errorf("%w: nil sps", ErrBadSlice)
	}
	if !s.LTPresent {
		return 0, nil
	}
	var nbSPS uint32
	if len(s.LTPoc) > 0 {
		v, err := r.ReadUE()
		if err != nil {
			return 0, fmt.Errorf("%w: lt nb sps: %v", ErrBadSlice, err)
		}
		nbSPS = v
	}
	nbSH, err := r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("%w: lt nb sh: %v", ErrBadSlice, err)
	}
	if nbSPS > uint32(len(s.LTPoc)) || nbSH+nbSPS > 32 {
		return 0, fmt.Errorf("%w: lt refs %d+%d", ErrBadSlice, nbSPS, nbSH)
	}
	nbits := 0
	if len(s.LTPoc) > 1 {
		nbits = ceilLog2(uint32(len(s.LTPoc)))
	}
	for i := uint32(0); i < nbSPS+nbSH; i++ {
		if i < nbSPS {
			if nbits > 0 {
				if _, err := r.ReadBits(nbits); err != nil {
					return 0, fmt.Errorf("%w: lt idx %d: %v", ErrBadSlice, i, err)
				}
			}
		} else {
			if _, err := r.ReadBits(int(s.Log2MaxPOCLsb)); err != nil {
				return 0, fmt.Errorf("%w: lt poc %d: %v", ErrBadSlice, i, err)
			}
			if _, err := r.ReadBit(); err != nil {
				return 0, fmt.Errorf("%w: lt used %d: %v", ErrBadSlice, i, err)
			}
		}
		msb, err := r.ReadBit()
		if err != nil {
			return 0, fmt.Errorf("%w: lt msb %d: %v", ErrBadSlice, i, err)
		}
		if msb != 0 {
			if _, err := r.ReadUE(); err != nil {
				return 0, fmt.Errorf("%w: lt delta %d: %v", ErrBadSlice, i, err)
			}
		}
	}
	return nbSPS + nbSH, nil
}

// parseWeightTable reads the prediction weight table for P/B slices.
// Peer: hevcdec.c:175 pred_weight_table (denom bounds, per-ref used
// flags, default-vs-explicit weights). Only the used flags and denoms
// are kept for the gate; full tables ride along for step 3.
func parseWeightTable(r *Reader, s *SPS, sh *SliceHeader) error {
	denom, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("%w: luma denom: %v", ErrBadSlice, err)
	}
	if denom > 7 {
		return fmt.Errorf("%w: luma denom %d", ErrBadSlice, denom)
	}
	sh.LumaDenom = denom
	if s.ChromaFormat != 0 {
		d, err := r.ReadSE()
		if err != nil {
			return fmt.Errorf("%w: chroma denom delta: %v", ErrBadSlice, err)
		}
		cd := int32(denom) + d
		if cd < 0 || cd > 7 {
			return fmt.Errorf("%w: chroma denom %d", ErrBadSlice, cd)
		}
		sh.ChromaDenom = uint32(cd)
	}
	readList := func(n uint32, luma, chroma *[]bool) error {
		if n == 0 {
			return nil
		}
		lf, err := r.ReadBits(int(n))
		if err != nil {
			return fmt.Errorf("%w: luma weight flags: %v", ErrBadSlice, err)
		}
		var cf uint32
		if s.ChromaFormat != 0 {
			cf, err = r.ReadBits(int(n))
			if err != nil {
				return fmt.Errorf("%w: chroma weight flags: %v", ErrBadSlice, err)
			}
		}
		for i := uint32(0); i < n; i++ {
			bit := uint32(1) << (n - 1 - i)
			lu := lf&bit != 0
			cu := cf&bit != 0
			*luma = append(*luma, lu)
			*chroma = append(*chroma, cu)
			if lu {
				dw, err := r.ReadSE()
				if err != nil {
					return fmt.Errorf("%w: luma weight %d: %v", ErrBadSlice, i, err)
				}
				if dw < -128 || dw > 127 {
					return fmt.Errorf("%w: luma weight %d", ErrBadSlice, i)
				}
				if _, err := r.ReadSE(); err != nil {
					return fmt.Errorf("%w: luma offset %d: %v", ErrBadSlice, i, err)
				}
			}
			if cu {
				for j := 0; j < 2; j++ {
					dw, err := r.ReadSE()
					if err != nil {
						return fmt.Errorf("%w: chroma weight %d/%d: %v", ErrBadSlice, i, j, err)
					}
					if dw < -128 || dw > 127 {
						return fmt.Errorf("%w: chroma weight %d/%d", ErrBadSlice, i, j)
					}
					do, err := r.ReadSE()
					if err != nil {
						return fmt.Errorf("%w: chroma offset %d/%d: %v", ErrBadSlice, i, j, err)
					}
					if do < -(1<<17) || do > (1<<17) {
						return fmt.Errorf("%w: chroma offset %d/%d", ErrBadSlice, i, j)
					}
				}
			}
		}
		return nil
	}
	if err := readList(sh.RefL0, &sh.L0LumaUsed, &sh.L0ChromaUsed); err != nil {
		return err
	}
	if sh.Type == SliceB {
		if err := readList(sh.RefL1, &sh.L1LumaUsed, &sh.L1ChromaUsed); err != nil {
			return err
		}
	}
	return nil
}
