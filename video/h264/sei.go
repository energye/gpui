package h264

import "fmt"

// SEI supplemental messages (Annex D): timing and user-data notes the
// decoder passes through, never acts on. Unknown types survive as raw
// payloads so newer encoders never break the decode.

// SEI payload types this stage interprets; the rest pass through raw.
const (
	SEIBufferingPeriod = 0
	SEIPicTiming       = 1
	SEIUserDataUnreg   = 5
	SEIRecoveryPoint   = 6
)

// ClockTS is one pic-timing clock stamp (full-stamp fields only when
// signalled; offsets stay raw bit values).
type ClockTS struct {
	Type       uint32
	CntDropped bool
	FullStamp  bool
	Seconds    uint32
	Minutes    uint32
	Hours      uint32
	HasHMS     bool
	TimeOffset int32
	HasOffset  bool
}

// PicTiming is a parsed type-1 message. Delays exist only with HRD
// lengths from the active SPS; clocks only with pic_struct_present.
type PicTiming struct {
	HasDelays  bool
	CpbRemoval uint32
	DpbOutput  uint32
	HasStruct  bool
	PicStruct  uint32
	Clocks     []ClockTS
}

// RecoveryPoint is a parsed type-6 message.
type RecoveryPoint struct {
	FrameCnt      uint32
	ExactMatch    bool
	BrokenLink    bool
	ChangingSlice uint32
}

// SEIMessage is one sei_message: type, raw payload and the parsed view
// for known types (UUID split for user data, timing, recovery point).
type SEIMessage struct {
	Type     uint32
	Payload  []byte
	UUID     [16]byte
	UserData []byte
	Timing   PicTiming
	Recovery RecoveryPoint
}

// ParseSEI parses one SEI NALU (header byte included). vui supplies the
// HRD lengths for pic-timing (nil tolerates: delays decode as absent).
func ParseSEI(nalu []byte, vui *VUI) ([]SEIMessage, error) {
	_, _, typ, err := NALUHeader(nalu)
	if err != nil {
		return nil, err
	}
	if typ != NALSei {
		return nil, fmt.Errorf("%w: type %d is not SEI", ErrBadNALU, typ)
	}
	r := NewReader(UnescapeRBSP(nalu[1:]))
	var out []SEIMessage
	for r.MoreRBSPData() {
		pt, err := seiField(r)
		if err != nil {
			return nil, fmt.Errorf("%w: sei type: %v", ErrBadNALU, err)
		}
		sz, err := seiField(r)
		if err != nil {
			return nil, fmt.Errorf("%w: sei size: %v", ErrBadNALU, err)
		}
		pay := make([]byte, sz)
		for i := range pay {
			b, err := r.ReadBits(8)
			if err != nil {
				return nil, fmt.Errorf("%w: sei payload: %v", ErrBadNALU, err)
			}
			pay[i] = byte(b)
		}
		m := SEIMessage{Type: pt, Payload: pay}
		switch pt {
		case SEIUserDataUnreg:
			if len(pay) < 16 {
				return nil, fmt.Errorf("%w: user data %d", ErrBadNALU, len(pay))
			}
			copy(m.UUID[:], pay[:16])
			m.UserData = append([]byte(nil), pay[16:]...)
		case SEIRecoveryPoint:
			if err := parseRecovery(pay, &m.Recovery); err != nil {
				return nil, err
			}
		case SEIPicTiming:
			if err := parsePicTiming(pay, vui, &m.Timing); err != nil {
				return nil, err
			}
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: empty sei", ErrBadNALU)
	}
	return out, nil
}

// seiField reads one 0xFF-extended payloadType/payloadSize field.
func seiField(r *Reader) (uint32, error) {
	var v uint32
	for {
		b, err := r.ReadBits(8)
		if err != nil {
			return 0, err
		}
		v += b
		if b != 255 {
			return v, nil
		}
		if v > 1<<20 {
			return 0, fmt.Errorf("sei field overflow")
		}
	}
}

// parseRecovery reads a type-6 payload from raw bytes.
func parseRecovery(pay []byte, rp *RecoveryPoint) error {
	r := NewReader(pay)
	fc, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("%w: recovery cnt: %v", ErrBadNALU, err)
	}
	em, err := r.ReadBits(1)
	if err != nil {
		return fmt.Errorf("%w: exact match: %v", ErrBadNALU, err)
	}
	bl, err := r.ReadBits(1)
	if err != nil {
		return fmt.Errorf("%w: broken link: %v", ErrBadNALU, err)
	}
	cs, err := r.ReadBits(2)
	if err != nil {
		return fmt.Errorf("%w: changing slice: %v", ErrBadNALU, err)
	}
	rp.FrameCnt, rp.ExactMatch, rp.BrokenLink, rp.ChangingSlice = fc, em != 0, bl != 0, cs
	return nil
}

// parsePicTiming reads a type-1 payload; vui may be nil.
func parsePicTiming(pay []byte, vui *VUI, pt *PicTiming) error {
	r := NewReader(pay)
	var cpbLen, dpbLen int
	if vui != nil {
		h := &vui.NalHRD
		if !h.Present {
			h = &vui.VclHRD
		}
		if h.Present {
			cpbLen, dpbLen = h.CpbRemovalDelayLength, h.DpbOutputDelayLength
		}
	}
	if cpbLen > 0 {
		cpb, err := r.ReadBits(cpbLen)
		if err != nil {
			return fmt.Errorf("%w: cpb delay: %v", ErrBadNALU, err)
		}
		dpb, err := r.ReadBits(dpbLen)
		if err != nil {
			return fmt.Errorf("%w: dpb delay: %v", ErrBadNALU, err)
		}
		pt.HasDelays = true
		pt.CpbRemoval, pt.DpbOutput = cpb, dpb
	}
	if vui == nil || !vui.PicStructPresent {
		return nil
	}
	ps, err := r.ReadBits(4)
	if err != nil {
		return fmt.Errorf("%w: pic struct: %v", ErrBadNALU, err)
	}
	if ps > 8 {
		return fmt.Errorf("%w: pic struct %d", ErrBadNALU, ps)
	}
	pt.HasStruct = true
	pt.PicStruct = ps
	nclk := numClockTS(ps)
	offLen := 0
	if h := vui.NalHRD; h.Present {
		offLen = h.TimeOffsetLength
	} else if h := vui.VclHRD; h.Present {
		offLen = h.TimeOffsetLength
	}
	for i := uint32(0); i < nclk; i++ {
		f, err := r.ReadBits(1)
		if err != nil {
			return fmt.Errorf("%w: clock flag: %v", ErrBadNALU, err)
		}
		if f == 0 {
			continue
		}
		c, err := parseClockTS(r, offLen)
		if err != nil {
			return err
		}
		pt.Clocks = append(pt.Clocks, c)
	}
	return nil
}

// numClockTS maps pic_struct to clock count (Table D-1).
func numClockTS(ps uint32) uint32 {
	switch ps {
	case 0, 1, 2:
		return 1
	case 3, 4, 7:
		return 2
	case 5, 6, 8:
		return 3
	}
	return 0
}

// parseClockTS reads one clock_timestamp; offLen is time_offset_length.
func parseClockTS(r *Reader, offLen int) (ClockTS, error) {
	var c ClockTS
	fail := func(w string, err error) (ClockTS, error) {
		return ClockTS{}, fmt.Errorf("%w: %s: %v", ErrBadNALU, w, err)
	}
	ct, err := r.ReadBits(2)
	if err != nil {
		return fail("ct type", err)
	}
	c.Type = ct
	if _, err := r.ReadBits(1); err != nil { // nuit_field_based_flag
		return fail("nuit", err)
	}
	if _, err := r.ReadBits(5); err != nil { // counting_type
		return fail("counting", err)
	}
	full, err := r.ReadBits(1)
	if err != nil {
		return fail("full flag", err)
	}
	dis, err := r.ReadBits(1)
	if err != nil {
		return fail("discontinuity", err)
	}
	_ = dis
	drop, err := r.ReadBits(1)
	if err != nil {
		return fail("dropped", err)
	}
	c.CntDropped = drop != 0
	nf, err := r.ReadBits(8)
	if err != nil {
		return fail("n_frames", err)
	}
	_ = nf
	if full != 0 {
		c.FullStamp = true
		for i, dst := range []*uint32{&c.Seconds, &c.Minutes} {
			x, err := r.ReadBits(6)
			if err != nil {
				return fail(fmt.Sprintf("hms %d", i), err)
			}
			*dst = x
		}
		h, err := r.ReadBits(5)
		if err != nil {
			return fail("hours", err)
		}
		c.Hours = h
		c.HasHMS = true
	} else {
		sf, err := r.ReadBits(1)
		if err != nil {
			return fail("seconds flag", err)
		}
		if sf != 0 {
			s, err := r.ReadBits(6)
			if err != nil {
				return fail("seconds", err)
			}
			c.Seconds = s
			mf, err := r.ReadBits(1)
			if err != nil {
				return fail("minutes flag", err)
			}
			if mf != 0 {
				m, err := r.ReadBits(6)
				if err != nil {
					return fail("minutes", err)
				}
				c.Minutes = m
				hf, err := r.ReadBits(1)
				if err != nil {
					return fail("hours flag", err)
				}
				if hf != 0 {
					h, err := r.ReadBits(5)
					if err != nil {
						return fail("hours", err)
					}
					c.Hours = h
				}
			}
			c.HasHMS = true
		}
	}
	if offLen > 0 {
		o, err := r.ReadBits(offLen)
		if err != nil {
			return fail("offset", err)
		}
		// Sign-extend the fixed-width offset.
		if o&(1<<uint(offLen-1)) != 0 {
			o |= ^uint32(0) << uint(offLen)
		}
		c.TimeOffset, c.HasOffset = int32(o), true
	}
	return c, nil
}
