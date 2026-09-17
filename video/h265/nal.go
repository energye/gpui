package h265

// VCL/non-VCL NAL unit names for slice triage (V2-2 step 2).
// Peer (read-only): libavcodec/hevc/hevc.h:29 HEVCNALType enum +
// hevcdec.h:74-77 IS_IRAP/IS_IDR (type ranges, no code copied).
// V2-1's TypeName keeps naming headers only; slices get SliceTypeName
// here so h265.go stays untouched.
const (
	NALTrailN   = 0
	NALTrailR   = 1
	NALTSAN     = 2
	NALTSAR     = 3
	NALSTSAN    = 4
	NALSTSAR    = 5
	NALRadlN    = 6
	NALRadlR    = 7
	NALRaslN    = 8
	NALRaslR    = 9
	NALBlaWLp   = 16
	NALBlaWRadl = 17
	NALBlaNLp   = 18
	NALIdrWRadl = 19
	NALIdrNLp   = 20
	NALCraNut   = 21
)

// Slice type ids (ue(v) slice_type): B=0, P=1, I=2.
const (
	SliceB = 0
	SliceP = 1
	SliceI = 2
)

// IsSlice reports whether t is a coded slice segment (VCL 0-9, 16-23).
func IsSlice(t int) bool {
	return (t >= 0 && t <= 9) || (t >= 16 && t <= 23)
}

// IsIRAP reports random-access pictures (BLA/IDR/CRA + reserved 22-23).
func IsIRAP(t int) bool { return t >= NALBlaWLp && t <= 23 }

// IsIDR reports IDR pictures (poc restarts at 0, no RPS in header).
func IsIDR(t int) bool { return t == NALIdrWRadl || t == NALIdrNLp }

// IsBLA reports broken-link pictures (poc msb forced to 0).
func IsBLA(t int) bool { return t >= NALBlaWLp && t <= NALBlaNLp }

// SliceTypeName names a slice_type id for readable errors.
func SliceTypeName(t uint32) string {
	switch t {
	case SliceB:
		return "B"
	case SliceP:
		return "P"
	case SliceI:
		return "I"
	default:
		return "unknown"
	}
}

// NALName names a VCL NAL type for readable errors.
func NALName(t int) string {
	switch t {
	case NALTrailN:
		return "TRAIL_N"
	case NALTrailR:
		return "TRAIL_R"
	case NALTSAN:
		return "TSA_N"
	case NALTSAR:
		return "TSA_R"
	case NALSTSAN:
		return "STSA_N"
	case NALSTSAR:
		return "STSA_R"
	case NALRadlN:
		return "RADL_N"
	case NALRadlR:
		return "RADL_R"
	case NALRaslN:
		return "RASL_N"
	case NALRaslR:
		return "RASL_R"
	case NALBlaWLp:
		return "BLA_W_LP"
	case NALBlaWRadl:
		return "BLA_W_RADL"
	case NALBlaNLp:
		return "BLA_N_LP"
	case NALIdrWRadl:
		return "IDR_W_RADL"
	case NALIdrNLp:
		return "IDR_N_LP"
	case NALCraNut:
		return "CRA_NUT"
	default:
		return TypeName(t)
	}
}
