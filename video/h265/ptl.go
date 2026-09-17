package h265

import (
	"errors"
	"fmt"
)

// Sentinel errors for full parameter-set parsing (V2-2 step 1).
// Messages stay in plain English so callers can match with errors.Is.
var (
	ErrBadVPS     = errors.New("h265: bad VPS")
	ErrBadSPS     = errors.New("h265: bad SPS")
	ErrBadPPS     = errors.New("h265: bad PPS")
	ErrMissingVPS = errors.New("h265: missing VPS")
	ErrMissingSPS = errors.New("h265: missing SPS")
	ErrMissingPPS = errors.New("h265: missing PPS")
)

// PTL is the profile/tier/level block shared by VPS and SPS.
// Peer (read-only): libavcodec/hevc/ps.c:262 decode_profile_tier_level +
// :337 parse_ptl (field order, reserved skip, inbld flag).
type PTL struct {
	ProfileSpace uint32
	TierFlag     bool
	ProfileIDC   uint32
	Compat       [32]bool
	Progressive  bool
	Interlaced   bool
	NonPacked    bool
	FrameOnly    bool
	LevelIDC     uint8
	ProfileNameV string
}

// readPTL reads one profile_tier_level(1, maxSubLayers) block.
func readPTL(r *Reader, maxSubLayers uint32) (*PTL, error) {
	p := &PTL{}
	v, err := r.ReadBits(2)
	if err != nil {
		return nil, fmt.Errorf("%w: profile space: %v", ErrBadSPS, err)
	}
	p.ProfileSpace = v
	t, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: tier: %v", ErrBadSPS, err)
	}
	p.TierFlag = t != 0
	idc, err := r.ReadBits(5)
	if err != nil {
		return nil, fmt.Errorf("%w: profile idc: %v", ErrBadSPS, err)
	}
	p.ProfileIDC = idc
	for i := 0; i < 32; i++ {
		b, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: compat %d: %v", ErrBadSPS, i, err)
		}
		p.Compat[i] = b != 0
	}
	// Effective profile follows the compatibility flags when idc is 0.
	eff := p.ProfileIDC
	if eff == 0 {
		for i := 1; i < 32; i++ {
			if p.Compat[i] {
				eff = uint32(i)
				break
			}
		}
	}
	flags := []struct {
		name string
		dst  *bool
	}{
		{"progressive", &p.Progressive},
		{"interlaced", &p.Interlaced},
		{"nonpacked", &p.NonPacked},
		{"frameonly", &p.FrameOnly},
	}
	for _, f := range flags {
		b, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrBadSPS, f.name, err)
		}
		*f.dst = b != 0
	}
	inSet := func(ids ...uint32) bool {
		for _, id := range ids {
			if p.ProfileIDC == id || (id < 32 && p.Compat[id]) {
				return true
			}
		}
		return false
	}
	switch {
	case inSet(4, 5, 6, 7, 8, 9, 10):
		for _, n := range []string{"12bit", "10bit", "8bit", "422", "420", "mono", "intra", "onepic", "lowerbr"} {
			if _, err := r.ReadBit(); err != nil {
				return nil, fmt.Errorf("%w: constraint %s: %v", ErrBadSPS, n, err)
			}
		}
		if inSet(5, 9, 10) {
			if _, err := r.ReadBit(); err != nil {
				return nil, fmt.Errorf("%w: 14bit: %v", ErrBadSPS, err)
			}
			if err := skipBits(r, 33, "reserved33"); err != nil {
				return nil, err
			}
		} else {
			if err := skipBits(r, 34, "reserved34"); err != nil {
				return nil, err
			}
		}
	case p.ProfileIDC == 2 || p.Compat[2]:
		if err := skipBits(r, 7, "reserved7"); err != nil {
			return nil, err
		}
		if _, err := r.ReadBit(); err != nil {
			return nil, fmt.Errorf("%w: onepic: %v", ErrBadSPS, err)
		}
		if err := skipBits(r, 35, "reserved35"); err != nil {
			return nil, err
		}
	default:
		if err := skipBits(r, 43, "reserved43"); err != nil {
			return nil, err
		}
	}
	if inSet(1, 2, 3, 4, 5, 9) {
		b, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: inbld: %v", ErrBadSPS, err)
		}
		_ = b
	} else {
		if _, err := r.ReadBit(); err != nil {
			return nil, fmt.Errorf("%w: reserved1: %v", ErrBadSPS, err)
		}
	}
	lvl, err := r.ReadBits(8)
	if err != nil {
		return nil, fmt.Errorf("%w: level: %v", ErrBadSPS, err)
	}
	p.LevelIDC = uint8(lvl)
	p.ProfileNameV = ProfileName(uint8(eff))
	// Sublayer PTL present flags: our clips use a single sublayer, but
	// the bits must still be consumed when more layers are declared.
	if maxSubLayers > 1 {
		for i := uint32(0); i < maxSubLayers-1; i++ {
			sp, err := r.ReadBit()
			if err != nil {
				return nil, fmt.Errorf("%w: sub profile %d: %v", ErrBadSPS, i, err)
			}
			sl, err := r.ReadBit()
			if err != nil {
				return nil, fmt.Errorf("%w: sub level %d: %v", ErrBadSPS, i, err)
			}
			_ = sp
			_ = sl
		}
		for i := maxSubLayers - 1; i < 8; i++ {
			if _, err := r.ReadBits(2); err != nil {
				return nil, fmt.Errorf("%w: sub reserved %d: %v", ErrBadSPS, i, err)
			}
		}
		// Sublayer bodies only appear when a present flag was set;
		// single-layer clips never reach here, multi-layer clips
		// report honest unsupported in vps/sps (rare path).
		return nil, fmt.Errorf("%w: multi-layer sub PTL not supported", ErrBadSPS)
	}
	return p, nil
}

// skipBits consumes n single bits (for reserved runs longer than the
// Reader's 32-bit single-read limit).
func skipBits(r *Reader, n int, name string) error {
	for i := 0; i < n; i++ {
		if _, err := r.ReadBit(); err != nil {
			return fmt.Errorf("%w: %s bit %d: %v", ErrBadSPS, name, i, err)
		}
	}
	return nil
}
