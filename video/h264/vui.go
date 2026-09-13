package h264

import "fmt"

// VUI carries the SPS display-side markings (Annex E): shape, colour
// range, frame rate and buffer delays. Absent sections read the spec
// defaults (square pixels, studio/limited range, no timing), so callers
// never branch on presence.

// HRD holds the buffer-delay lengths an SEI pic-timing message needs.
// Rate/size tables are parsed and dropped: this stage only times frames.
type HRD struct {
	Present               bool
	CpbRemovalDelayLength int // bits (length_minus1 + 1)
	DpbOutputDelayLength  int
	TimeOffsetLength      int
}

// VUI is the parsed vui_parameters block. Only timing/colour/shape and
// the HRD lengths survive; reserved and encoder-tuning fields are
// skipped in place.
type VUI struct {
	SARWidth, SARHeight uint32
	SARPresent          bool
	VideoFormat         uint32
	FullRange           bool
	ColourPrimaries     uint32
	ColourTransfer      uint32
	ColourMatrix        uint32
	ColourPresent       bool
	ChromaLocTop        uint32
	ChromaLocBottom     uint32
	ChromaLocPresent    bool
	TimingUnits         uint32
	TimingScale         uint32
	FixedRate           bool
	TimingPresent       bool
	NalHRD              HRD
	VclHRD              HRD
	PicStructPresent    bool
	NumReorderFrames    uint32
	RestrictPresent     bool
}

// SAR returns the display pixel shape (1:1 unless signalled).
func (v *VUI) SAR() (uint32, uint32) {
	if v == nil || !v.SARPresent || v.SARWidth == 0 || v.SARHeight == 0 {
		return 1, 1
	}
	return v.SARWidth, v.SARHeight
}

// FPS returns frames per second from timing_info (0 when unsignalled).
func (v *VUI) FPS() float64 {
	if v == nil || !v.TimingPresent || v.TimingUnits == 0 {
		return 0
	}
	return float64(v.TimingScale) / float64(2*v.TimingUnits)
}

// CpbDpbPresent reports whether buffer delays may appear in SEI.
func (v *VUI) CpbDpbPresent() bool {
	return v != nil && (v.NalHRD.Present || v.VclHRD.Present)
}

// parseVUI reads vui_parameters at the current bit position (the
// vui_present flag already consumed).
func parseVUI(r *Reader) (*VUI, error) {
	v := &VUI{}
	b, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: aspect present: %v", ErrBadSPS, err)
	}
	if b != 0 {
		idc, err := r.ReadBits(8)
		if err != nil {
			return nil, fmt.Errorf("%w: aspect idc: %v", ErrBadSPS, err)
		}
		if idc == 255 {
			w, err := r.ReadBits(16)
			if err != nil {
				return nil, fmt.Errorf("%w: sar w: %v", ErrBadSPS, err)
			}
			h, err := r.ReadBits(16)
			if err != nil {
				return nil, fmt.Errorf("%w: sar h: %v", ErrBadSPS, err)
			}
			v.SARWidth, v.SARHeight = w, h
		} else if idc > 0 && idc < 17 {
			w, h := sarTable(idc)
			v.SARWidth, v.SARHeight = w, h
		} else if idc != 0 {
			return nil, fmt.Errorf("%w: aspect idc %d", ErrBadSPS, idc)
		}
		if idc != 0 {
			v.SARPresent = true
		}
	}
	if err := skipFlag(r, "overscan"); err != nil {
		return nil, err
	}
	b, err = r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: video signal: %v", ErrBadSPS, err)
	}
	if b != 0 {
		f, err := r.ReadBits(3)
		if err != nil {
			return nil, fmt.Errorf("%w: video format: %v", ErrBadSPS, err)
		}
		v.VideoFormat = f
		fr, err := r.ReadBits(1)
		if err != nil {
			return nil, fmt.Errorf("%w: full range: %v", ErrBadSPS, err)
		}
		v.FullRange = fr != 0
		c, err := r.ReadBits(1)
		if err != nil {
			return nil, fmt.Errorf("%w: colour desc: %v", ErrBadSPS, err)
		}
		if c != 0 {
			for i, dst := range []*uint32{&v.ColourPrimaries, &v.ColourTransfer, &v.ColourMatrix} {
				x, err := r.ReadBits(8)
				if err != nil {
					return nil, fmt.Errorf("%w: colour %d: %v", ErrBadSPS, i, err)
				}
				*dst = x
			}
			v.ColourPresent = true
		}
	}
	b, err = r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: chroma loc: %v", ErrBadSPS, err)
	}
	if b != 0 {
		t, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: chroma top: %v", ErrBadSPS, err)
		}
		bo, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: chroma bottom: %v", ErrBadSPS, err)
		}
		v.ChromaLocTop, v.ChromaLocBottom = t, bo
		v.ChromaLocPresent = true
	}
	b, err = r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: timing present: %v", ErrBadSPS, err)
	}
	if b != 0 {
		u, err := r.ReadBits(32)
		if err != nil {
			return nil, fmt.Errorf("%w: timing units: %v", ErrBadSPS, err)
		}
		s, err := r.ReadBits(32)
		if err != nil {
			return nil, fmt.Errorf("%w: timing scale: %v", ErrBadSPS, err)
		}
		f, err := r.ReadBits(1)
		if err != nil {
			return nil, fmt.Errorf("%w: fixed rate: %v", ErrBadSPS, err)
		}
		v.TimingUnits, v.TimingScale = u, s
		v.FixedRate = f != 0
		v.TimingPresent = true
	}
	for i, dst := range []*HRD{&v.NalHRD, &v.VclHRD} {
		p, err := r.ReadBits(1)
		if err != nil {
			return nil, fmt.Errorf("%w: hrd %d present: %v", ErrBadSPS, i, err)
		}
		if p == 0 {
			continue
		}
		if err := parseHRD(r, dst); err != nil {
			return nil, err
		}
	}
	if v.NalHRD.Present || v.VclHRD.Present {
		if _, err := r.ReadBits(1); err != nil { // low_delay_hrd_flag
			return nil, fmt.Errorf("%w: low delay hrd: %v", ErrBadSPS, err)
		}
	}
	p, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: pic struct present: %v", ErrBadSPS, err)
	}
	v.PicStructPresent = p != 0
	b, err = r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: restriction present: %v", ErrBadSPS, err)
	}
	if b == 0 {
		return v, nil
	}
	v.RestrictPresent = true
	if _, err := r.ReadBits(1); err != nil { // motion_vectors_over_pic_boundaries_flag
		return nil, fmt.Errorf("%w: mv over boundaries: %v", ErrBadSPS, err)
	}
	for i := 0; i < 4; i++ {
		if _, err := r.ReadUE(); err != nil {
			return nil, fmt.Errorf("%w: restriction %d: %v", ErrBadSPS, i, err)
		}
	}
	nr, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: reorder frames: %v", ErrBadSPS, err)
	}
	v.NumReorderFrames = nr
	if _, err := r.ReadUE(); err != nil {
		return nil, fmt.Errorf("%w: max buffer: %v", ErrBadSPS, err)
	}
	return v, nil
}

// skipFlag consumes an optional one-bit flag plus its one-bit payload
// (overscan_appropriate_flag shape).
func skipFlag(r *Reader, what string) error {
	b, err := r.ReadBits(1)
	if err != nil {
		return fmt.Errorf("%w: %s present: %v", ErrBadSPS, what, err)
	}
	if b != 0 {
		if _, err := r.ReadBits(1); err != nil {
			return fmt.Errorf("%w: %s value: %v", ErrBadSPS, what, err)
		}
	}
	return nil
}

// parseHRD reads hrd_parameters, keeping only the delay lengths.
func parseHRD(r *Reader, h *HRD) error {
	n, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("%w: cpb count: %v", ErrBadSPS, err)
	}
	if n > 31 {
		return fmt.Errorf("%w: cpb count %d", ErrBadSPS, n)
	}
	if _, err := r.ReadBits(8); err != nil { // bit_rate_scale + cpb_size_scale
		return fmt.Errorf("%w: hrd scales: %v", ErrBadSPS, err)
	}
	for i := uint32(0); i <= n; i++ {
		if _, err := r.ReadUE(); err != nil {
			return fmt.Errorf("%w: bit rate %d: %v", ErrBadSPS, i, err)
		}
		if _, err := r.ReadUE(); err != nil {
			return fmt.Errorf("%w: cpb size %d: %v", ErrBadSPS, i, err)
		}
		if _, err := r.ReadBits(1); err != nil {
			return fmt.Errorf("%w: cbr %d: %v", ErrBadSPS, i, err)
		}
	}
	cpb, err := r.ReadBits(5)
	if err != nil {
		return fmt.Errorf("%w: cpb delay length: %v", ErrBadSPS, err)
	}
	dpb, err := r.ReadBits(5)
	if err != nil {
		return fmt.Errorf("%w: dpb delay length: %v", ErrBadSPS, err)
	}
	to, err := r.ReadBits(5)
	if err != nil {
		return fmt.Errorf("%w: time offset length: %v", ErrBadSPS, err)
	}
	h.CpbRemovalDelayLength = int(cpb) + 1
	h.DpbOutputDelayLength = int(dpb) + 1
	h.TimeOffsetLength = int(to) + 1
	h.Present = true
	return nil
}

// sarTable maps aspect_ratio_idc 1..16 to width/height (Table E-1).
func sarTable(idc uint32) (uint32, uint32) {
	switch idc {
	case 1:
		return 1, 1
	case 2:
		return 12, 11
	case 3:
		return 10, 11
	case 4:
		return 16, 11
	case 5:
		return 40, 33
	case 6:
		return 24, 11
	case 7:
		return 20, 11
	case 8:
		return 32, 11
	case 9:
		return 80, 33
	case 10:
		return 18, 11
	case 11:
		return 15, 11
	case 12:
		return 64, 33
	case 13:
		return 160, 99
	case 14:
		return 4, 3
	case 15:
		return 3, 2
	case 16:
		return 2, 1
	}
	return 1, 1
}
