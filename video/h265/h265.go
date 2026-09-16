package h265

import (
	"errors"
	"fmt"
)

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrBadHVCC      = errors.New("h265: bad hvcC box")
	ErrNoParamSets  = errors.New("h265: no parameter sets")
	ErrBadNALU      = errors.New("h265: bad NAL unit")
	ErrTruncated    = errors.New("h265: truncated input")
	ErrNoNALU       = errors.New("h265: no NAL units found")
	ErrNotDecodable = errors.New("h265: pixel decode not implemented in this stage (V2-1 headers only)")
)

// NAL unit types kept for header triage (ITU-T H.265 Table 7-1, only the
// entries V2-1 names; the pixel stage owns the rest).
const (
	NALVPS       = 32
	NALSPS       = 33
	NALPPS       = 34
	NALAUD       = 35
	NALPrefixSEI = 39
	NALSuffixSEI = 40
)

// TypeName returns a short name for a header NAL unit type.
func TypeName(t int) string {
	switch t {
	case NALVPS:
		return "vps"
	case NALSPS:
		return "sps"
	case NALPPS:
		return "pps"
	case NALAUD:
		return "aud"
	case NALPrefixSEI:
		return "prefixSei"
	case NALSuffixSEI:
		return "suffixSei"
	default:
		return "unknown"
	}
}

// NALType reads the 2-byte HEVC header's nal_unit_type (bits 9-14 of the
// first 16 bits: forbidden_zero_bit(1) + type(6) + layer(6) + tid(3)).
func NALType(nalu []byte) (int, bool) {
	if len(nalu) < 2 {
		return 0, false
	}
	if nalu[0]&0x80 != 0 {
		return 0, false
	}
	return int((nalu[0] >> 1) & 0x3F), true
}

// HVCC holds the parsed hvcC box: the out-of-band parameter sets plus
// the NALU length field size for MP4 sample payloads. Arrays arrive per
// NAL type (VPS/SPS/PPS/SEI); the raw box is kept for exact re-emit.
type HVCC struct {
	Profile     uint8
	ProfileName string
	Level       uint8
	LengthSize  int
	VPS         [][]byte
	SPS         [][]byte
	PPS         [][]byte
	SEI         [][]byte
	Raw         []byte
}

// ProfileName maps the HEVC profile_idc to its short name (only the
// profiles V2-1 names; unknown ids keep the numeric form).
// Peer note: the 96x96 probe clip reports profile_idc 96 (a libx265
// format-range extension value, not Main); V2-1 names it honestly and
// leaves profile gating to V2-2.
func ProfileName(idc uint8) string {
	switch idc {
	case 1:
		return "Main"
	case 2:
		return "Main 10"
	case 3:
		return "Main Still Picture"
	default:
		return fmt.Sprintf("profile-%d", idc)
	}
}

// ParseHVCC parses an hvcC box payload (the box content after type).
// Shape peer (read-only): libavcodec/hevc/parse.c:79
// ff_hevc_decode_extradata (configurationVersion 1, minimum 23 bytes
// with 0 arrays, skip 21, length-size byte, array count, then per-array
// type/count with 2-byte length-prefixed units).
func ParseHVCC(box []byte) (*HVCC, error) {
	if len(box) < 23 {
		return nil, fmt.Errorf("%w: %d bytes", ErrBadHVCC, len(box))
	}
	if box[0] != 1 {
		return nil, fmt.Errorf("%w: version %d", ErrBadHVCC, box[0])
	}
	h := &HVCC{
		Profile:     box[2],
		ProfileName: ProfileName(box[2]),
		Level:       box[12],
		LengthSize:  int(box[21]&0x03) + 1,
	}
	nArrays := int(box[22])
	off := 23
	for i := 0; i < nArrays; i++ {
		if len(box)-off < 3 {
			return nil, fmt.Errorf("%w: array %d header", ErrBadHVCC, i)
		}
		typ := int(box[off] & 0x3F)
		cnt := int(box[off+1])<<8 | int(box[off+2])
		off += 3
		for k := 0; k < cnt; k++ {
			if len(box)-off < 2 {
				return nil, fmt.Errorf("%w: array %d unit %d length", ErrBadHVCC, i, k)
			}
			n := int(box[off])<<8 | int(box[off+1])
			off += 2
			if n <= 0 || len(box)-off < n {
				return nil, fmt.Errorf("%w: array %d unit %d wants %d", ErrBadHVCC, i, k, n)
			}
			cp := append([]byte(nil), box[off:off+n]...)
			off += n
			switch typ {
			case NALVPS:
				h.VPS = append(h.VPS, cp)
			case NALSPS:
				h.SPS = append(h.SPS, cp)
			case NALPPS:
				h.PPS = append(h.PPS, cp)
			case NALPrefixSEI, NALSuffixSEI:
				h.SEI = append(h.SEI, cp)
			default:
			}
		}
	}
	if len(h.VPS) == 0 && len(h.SPS) == 0 && len(h.PPS) == 0 {
		return nil, fmt.Errorf("%w: no VPS/SPS/PPS in hvcC", ErrNoParamSets)
	}
	h.Raw = append([]byte(nil), box...)
	return h, nil
}

// SplitHVCC cuts length-prefixed NALUs (MP4 sample payload) using the
// length field size from the hvcC box (1, 2 or 4 bytes).
func SplitHVCC(sample []byte, lengthSize int) ([][]byte, error) {
	if lengthSize != 1 && lengthSize != 2 && lengthSize != 4 {
		return nil, fmt.Errorf("%w: length size %d", ErrBadNALU, lengthSize)
	}
	var out [][]byte
	off := 0
	for off < len(sample) {
		if len(sample)-off < lengthSize {
			return nil, fmt.Errorf("%w: short length at %d", ErrTruncated, off)
		}
		var n int
		switch lengthSize {
		case 1:
			n = int(sample[off])
		case 2:
			n = int(sample[off])<<8 | int(sample[off+1])
		default:
			n = int(sample[off])<<24 | int(sample[off+1])<<16 | int(sample[off+2])<<8 | int(sample[off+3])
		}
		off += lengthSize
		if n <= 0 {
			return nil, fmt.Errorf("%w: zero length at %d", ErrBadNALU, off)
		}
		if len(sample)-off < n {
			return nil, fmt.Errorf("%w: unit %d wants %d, have %d", ErrTruncated, len(out)+1, n, len(sample)-off)
		}
		cp := append([]byte(nil), sample[off:off+n]...)
		out = append(out, cp)
		off += n
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: empty sample", ErrNoNALU)
	}
	return out, nil
}
