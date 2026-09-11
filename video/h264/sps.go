package h264

import "fmt"

// SPS is the parsed sequence parameter set:档位, 等级, 尺寸.
type SPS struct {
	ID              uint32
	ProfileIDC      uint8
	Profile         string
	ConstraintFlags uint8
	LevelIDC        uint8
	Level           string
	ChromaFormat    uint32
	Width           uint32
	Height          uint32
	FrameMBsOnly    bool
	Interlaced      bool
	NumRefFrames    uint32
	HasCropping     bool
	CropLeft        uint32
	CropRight       uint32
	CropTop         uint32
	CropBottom      uint32
	VUIPresent      bool
	Raw             []byte
}

// ParseSPS parses one SPS NALU (header byte included).
func ParseSPS(nalu []byte) (*SPS, error) {
	forbidden, _, typ, err := NALUHeader(nalu)
	if err != nil {
		return nil, err
	}
	if forbidden {
		return nil, fmt.Errorf("%w: forbidden bit set", ErrBadSPS)
	}
	if typ != NALSPS {
		return nil, fmt.Errorf("%w: type %d is not SPS", ErrBadSPS, typ)
	}
	r := NewReader(UnescapeRBSP(nalu[1:]))
	profile, err := r.ReadBits(8)
	if err != nil {
		return nil, fmt.Errorf("%w: profile: %v", ErrBadSPS, err)
	}
	constraints, err := r.ReadBits(8)
	if err != nil {
		return nil, fmt.Errorf("%w: constraints: %v", ErrBadSPS, err)
	}
	level, err := r.ReadBits(8)
	if err != nil {
		return nil, fmt.Errorf("%w: level: %v", ErrBadSPS, err)
	}
	spsID, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: sps id: %v", ErrBadSPS, err)
	}
	s := &SPS{
		ID:              spsID,
		ProfileIDC:      uint8(profile),
		Profile:         ProfileName(uint8(profile)),
		ConstraintFlags: uint8(constraints),
		LevelIDC:        uint8(level),
		Level:           LevelString(uint8(level)),
		ChromaFormat:    1,
	}
	if isHighFamily(s.ProfileIDC) {
		chroma, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: chroma: %v", ErrBadSPS, err)
		}
		if chroma > 3 {
			return nil, fmt.Errorf("%w: chroma %d", ErrBadSPS, chroma)
		}
		s.ChromaFormat = chroma
		if chroma == 3 {
			if _, err := r.ReadBits(1); err != nil {
				return nil, fmt.Errorf("%w: separate plane: %v", ErrBadSPS, err)
			}
		}
		for _, name := range []string{"luma depth", "chroma depth"} {
			v, err := r.ReadUE()
			if err != nil {
				return nil, fmt.Errorf("%w: %s: %v", ErrBadSPS, name, err)
			}
			if v > 6 {
				return nil, fmt.Errorf("%w: %s %d", ErrBadSPS, name, v)
			}
		}
		if _, err := r.ReadBits(1); err != nil {
			return nil, fmt.Errorf("%w: qp bypass: %v", ErrBadSPS, err)
		}
		present, err := r.ReadBits(1)
		if err != nil {
			return nil, fmt.Errorf("%w: scaling present: %v", ErrBadSPS, err)
		}
		if present != 0 {
			n := 8
			if s.ChromaFormat == 3 {
				n = 12
			}
			if err := skipScalingLists(r, n); err != nil {
				return nil, err
			}
		}
	}
	if _, err := r.ReadUE(); err != nil {
		return nil, fmt.Errorf("%w: log2 max frame num: %v", ErrBadSPS, err)
	}
	pocType, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: poc type: %v", ErrBadSPS, err)
	}
	if pocType > 2 {
		return nil, fmt.Errorf("%w: poc type %d", ErrBadSPS, pocType)
	}
	switch pocType {
	case 0:
		if _, err := r.ReadUE(); err != nil {
			return nil, fmt.Errorf("%w: poc lsb: %v", ErrBadSPS, err)
		}
	case 1:
		if _, err := r.ReadBits(1); err != nil {
			return nil, fmt.Errorf("%w: poc delta always zero: %v", ErrBadSPS, err)
		}
		for i := 0; i < 2; i++ {
			if _, err := r.ReadSE(); err != nil {
				return nil, fmt.Errorf("%w: poc offset %d: %v", ErrBadSPS, i, err)
			}
		}
		cycle, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: poc cycle: %v", ErrBadSPS, err)
		}
		for i := uint32(0); i < cycle; i++ {
			if _, err := r.ReadSE(); err != nil {
				return nil, fmt.Errorf("%w: poc ref %d: %v", ErrBadSPS, i, err)
			}
		}
	}
	refs, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: ref frames: %v", ErrBadSPS, err)
	}
	s.NumRefFrames = refs
	if _, err := r.ReadBits(1); err != nil {
		return nil, fmt.Errorf("%w: gaps flag: %v", ErrBadSPS, err)
	}
	wMBs, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: width mbs: %v", ErrBadSPS, err)
	}
	hMap, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: height maps: %v", ErrBadSPS, err)
	}
	frameOnly, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: frame only: %v", ErrBadSPS, err)
	}
	s.FrameMBsOnly = frameOnly != 0
	s.Interlaced = !s.FrameMBsOnly
	if !s.FrameMBsOnly {
		if _, err := r.ReadBits(1); err != nil {
			return nil, fmt.Errorf("%w: mbaff: %v", ErrBadSPS, err)
		}
	}
	direct, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: direct8x8: %v", ErrBadSPS, err)
	}
	_ = direct
	cropFlag, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: crop flag: %v", ErrBadSPS, err)
	}
	width := (wMBs + 1) * 16
	height := (hMap + 1) * 16
	if !s.FrameMBsOnly {
		height *= 2
	}
	if cropFlag != 0 {
		s.HasCropping = true
		vals := make([]uint32, 4)
		for i := range vals {
			v, err := r.ReadUE()
			if err != nil {
				return nil, fmt.Errorf("%w: crop %d: %v", ErrBadSPS, i, err)
			}
			vals[i] = v
		}
		s.CropLeft, s.CropRight, s.CropTop, s.CropBottom = vals[0], vals[1], vals[2], vals[3]
		ux, uy := cropUnit(s.ChromaFormat, s.FrameMBsOnly)
		if vals[0]+vals[1] > 0 {
			if (vals[0]+vals[1])*ux >= width {
				return nil, fmt.Errorf("%w: crop wider than picture", ErrBadSPS)
			}
			width -= (vals[0] + vals[1]) * ux
		}
		if vals[2]+vals[3] > 0 {
			if (vals[2]+vals[3])*uy >= height {
				return nil, fmt.Errorf("%w: crop taller than picture", ErrBadSPS)
			}
			height -= (vals[2] + vals[3]) * uy
		}
	}
	if width == 0 || height == 0 || width > 8192 || height > 8192 {
		return nil, fmt.Errorf("%w: size %dx%d", ErrBadSPS, width, height)
	}
	s.Width, s.Height = width, height
	if r.MoreRBSPData() {
		if v, err := r.ReadBits(1); err == nil && v != 0 {
			s.VUIPresent = true
		}
	}
	s.Raw = append([]byte(nil), nalu...)
	return s, nil
}

func isHighFamily(profile uint8) bool {
	switch profile {
	case 100, 110, 122, 244, 44, 83, 86, 118, 128, 138, 139, 134, 135:
		return true
	}
	return false
}

func cropUnit(chroma uint32, frameOnly bool) (ux, uy uint32) {
	f := uint32(0)
	if frameOnly {
		f = 1
	}
	switch chroma {
	case 0:
		return 1, 2 - f
	case 1:
		return 2, 2 * (2 - f)
	case 2:
		return 2, 2 - f
	case 3:
		return 1, 2 - f
	default:
		return 2, 2
	}
}

func skipScalingLists(r *Reader, n int) error {
	for i := 0; i < n; i++ {
		present, err := r.ReadBits(1)
		if err != nil {
			return fmt.Errorf("%w: scaling %d present: %v", ErrBadSPS, i, err)
		}
		if present == 0 {
			continue
		}
		size := 16
		if i >= 6 {
			size = 64
		}
		last, next := int32(8), int32(8)
		for j := 0; j < size; j++ {
			if next != 0 {
				d, err := r.ReadSE()
				if err != nil {
					return fmt.Errorf("%w: scaling %d delta: %v", ErrBadSPS, i, err)
				}
				next = (last + d + 256) % 256
			}
			if next != 0 {
				last = next
			}
		}
	}
	return nil
}
