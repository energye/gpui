package h264

import "errors"

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrNoNALU           = errors.New("h264: no NAL units found")
	ErrBadNALU          = errors.New("h264: bad NAL unit")
	ErrBadSPS           = errors.New("h264: bad SPS")
	ErrBadPPS           = errors.New("h264: bad PPS")
	ErrMissingSPS       = errors.New("h264: slice needs an SPS that was never seen")
	ErrMissingPPS       = errors.New("h264: slice needs a PPS that was never seen")
	ErrSliceGroups      = errors.New("h264: slice groups (F17) not supported in this stage")
	ErrDataPartitioning = errors.New("h264: data partitioning (F17) not supported in this stage")
	ErrUnsupportedNAL   = errors.New("h264: unsupported NAL unit for this stage")
	ErrBadAVCC          = errors.New("h264: bad avcC box")
	ErrNoParamSets      = errors.New("h264: no parameter sets")
	ErrBadSliceHeader   = errors.New("h264: bad slice header")
	ErrUnsupportedLevel = errors.New("h264: unsupported level")
	ErrTruncated        = errors.New("h264: truncated input")
)

// NAL unit types (ITU-T H.264 Table 7-1).
const (
	NALSliceNonIDR = 1
	NALSlicePartA  = 2
	NALSlicePartB  = 3
	NALSlicePartC  = 4
	NALSliceIDR    = 5
	NALSei         = 6
	NALSPS         = 7
	NALPPS         = 8
	NALAUD         = 9
	NALEoS         = 10
	NALEoB         = 11
	NALFiller      = 12
	NALPrefix      = 14
	NALSliceExt    = 20
)

// TypeName returns a short name for a NAL unit type.
func TypeName(t int) string {
	switch t {
	case NALSliceNonIDR:
		return "slice"
	case NALSlicePartA:
		return "partA"
	case NALSlicePartB:
		return "partB"
	case NALSlicePartC:
		return "partC"
	case NALSliceIDR:
		return "idr"
	case NALSei:
		return "sei"
	case NALSPS:
		return "sps"
	case NALPPS:
		return "pps"
	case NALAUD:
		return "aud"
	case NALEoS:
		return "eos"
	case NALEoB:
		return "eob"
	case NALFiller:
		return "filler"
	case NALPrefix:
		return "prefix"
	case NALSliceExt:
		return "sliceExt"
	default:
		return "unknown"
	}
}

// TypeNameCN returns the Chinese label for a NAL unit type,
// for example-local display. Engine JSON keeps English keys.
func TypeNameCN(t int) string {
	switch t {
	case NALSliceNonIDR:
		return "切片"
	case NALSlicePartA:
		return "分区A"
	case NALSlicePartB:
		return "分区B"
	case NALSlicePartC:
		return "分区C"
	case NALSliceIDR:
		return "关键帧"
	case NALSei:
		return "补充信息"
	case NALSPS:
		return "片头参数"
	case NALPPS:
		return "图参数"
	case NALAUD:
		return "分隔符"
	case NALEoS:
		return "序列结束"
	case NALEoB:
		return "流结束"
	case NALFiller:
		return "填充"
	case NALPrefix:
		return "前缀"
	case NALSliceExt:
		return "扩展切片"
	default:
		return "未知"
	}
}

// NALType returns the unit type of a raw NALU (header byte low 5 bits).
// ok is false for empty units.
func NALType(nalu []byte) (t int, ok bool) {
	if len(nalu) == 0 {
		return 0, false
	}
	return int(nalu[0] & 0x1F), true
}

// ProfileName maps profile_idc to the common档位 name.
// Unknown ids keep a numeric label so callers can report them.
func ProfileName(idc uint8) string {
	switch idc {
	case 66:
		return "Baseline"
	case 77:
		return "Main"
	case 100:
		return "High"
	case 44:
		return "CAVLC444"
	case 83:
		return "ScalableBaseline"
	case 86:
		return "ScalableHigh"
	case 110:
		return "High10"
	case 118:
		return "High444Predictive"
	case 122:
		return "High422"
	case 244:
		return "High444"
	default:
		return "Unknown"
	}
}

// IsBaselineMainHigh reports whether idc is one of the three first-stage档位.
func IsBaselineMainHigh(idc uint8) bool {
	return idc == 66 || idc == 77 || idc == 100
}

// LevelSupported reports whether level_idc is in the first-stage range 1-5.2.
func LevelSupported(idc uint8) bool {
	switch idc {
	case 10, 11, 12, 13, 20, 21, 22, 30, 31, 32, 40, 41, 42, 50, 51, 52:
		return true
	}
	return false
}

// LevelString renders level_idc (e.g. 31) as dotted text (e.g. "3.1").
func LevelString(idc uint8) string {
	major := idc / 10
	minor := idc % 10
	if major == 0 {
		return "unknown"
	}
	return string(rune('0'+major)) + "." + string(rune('0'+minor))
}
