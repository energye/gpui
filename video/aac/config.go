package aac

import "fmt"

// SampleRates peers ff_mpeg4audio_sample_rates (mpeg4audio_sample_rates.h):
// indices 0-12 valid, 13-15 reserved.
var SampleRates = [16]int{
	96000, 88200, 64000, 48000, 44100, 32000,
	24000, 22050, 16000, 12000, 11025, 8000, 7350, 0, 0, 0,
}

// ChanMap peers ff_mpeg4audio_channels (mpeg4audio.c): chan_config to
// channel count. Index 15 is invalid (array has 15 entries).
var ChanMap = [15]int{0, 1, 2, 3, 4, 5, 6, 8, 0, 0, 0, 7, 8, 24, 8}

// Object types (mpeg4audio.h).
const (
	AOTNull   = 0
	AOTMain   = 1
	AOTLC     = 2
	AOTSSR    = 3
	AOTLTP    = 4
	AOTSBR    = 5
	AOTScal   = 6
	AOTPS     = 29
	AOTEscape = 31
)

// Config is the parsed AudioSpecificConfig (base + extension flags).
// It peers MPEG4AudioConfig (mpeg4audio.h) for the fields the front end
// needs; GA program config (PCE) stays honest ErrUnsupported.
type Config struct {
	ObjectType     int
	SamplingIndex  int
	SampleRate     int
	ChanConfig     int
	Channels       int
	SBR            int // 1 present, 0 absent, -1 unknown
	PS             int // 1 present, 0 absent, -1 unknown
	ExtObjectType  int
	ExtSampleRate  int
	ExtSamplingIdx int
	FrameShort     bool
	FrameLength    int
}

// bitReader is a minimal MSB-first reader over ASC bytes.
type bitReader struct {
	b   []byte
	pos int // bits consumed
}

func (r *bitReader) left() int { return len(r.b)*8 - r.pos }

func (r *bitReader) show(n int) (int, bool) {
	if n <= 0 || n > 32 || r.left() < n {
		return 0, false
	}
	v := 0
	for i := 0; i < n; i++ {
		p := r.pos + i
		if r.b[p>>3]&(0x80>>(p&7)) != 0 {
			v = v<<1 | 1
		} else {
			v <<= 1
		}
	}
	return v, true
}

func (r *bitReader) read(n int) (int, bool) {
	v, ok := r.show(n)
	if ok {
		r.pos += n
	}
	return v, ok
}

func getObjectType(r *bitReader) (int, bool) {
	t, ok := r.read(5)
	if !ok {
		return 0, false
	}
	if t == AOTEscape {
		e, ok := r.read(6)
		if !ok {
			return 0, false
		}
		return 32 + e, true
	}
	return t, true
}

func getSampleRate(r *bitReader) (rate, idx int, ok bool) {
	i, ok := r.read(4)
	if !ok {
		return 0, 0, false
	}
	if i == 0x0f {
		v, ok := r.read(24)
		if !ok || v <= 0 {
			return 0, i, false
		}
		return v, i, true
	}
	if i >= len(SampleRates) || SampleRates[i] == 0 {
		return 0, i, false
	}
	return SampleRates[i], i, true
}

// ParseASC parses an AudioSpecificConfig (esds DecSpecific payload).
// syncExt enables the trailing sync-extension scan (mov/esds path uses 1,
// matching decode_audio_specific_config sync_extension=1).
func ParseASC(asc []byte, syncExt bool) (*Config, error) {
	if len(asc) == 0 {
		return nil, fmt.Errorf("%w: empty", ErrBadASC)
	}
	r := &bitReader{b: asc}
	c := &Config{SBR: -1, PS: -1}
	ot, ok := getObjectType(r)
	if !ok {
		return nil, fmt.Errorf("%w: object type: %x", ErrBadASC, asc)
	}
	c.ObjectType = ot
	rate, idx, ok := getSampleRate(r)
	if !ok {
		return nil, fmt.Errorf("%w: sampling index", ErrBadASC)
	}
	c.SampleRate, c.SamplingIndex = rate, idx
	ch, ok := r.read(4)
	if !ok {
		return nil, fmt.Errorf("%w: chan config", ErrBadASC)
	}
	c.ChanConfig = ch
	if ch < 0 || ch >= len(ChanMap) {
		return nil, fmt.Errorf("%w: chan_config %d", ErrBadASC, ch)
	}
	c.Channels = ChanMap[ch]
	if ch != 0 && c.Channels == 0 {
		return nil, fmt.Errorf("%w: chan_config %d maps to 0", ErrBadASC, ch)
	}
	// SBR/PS-leading configs wrap the real object type.
	if ot == AOTSBR || ot == AOTPS {
		if ot == AOTPS {
			c.PS = 1
		}
		c.ExtObjectType = AOTSBR
		c.SBR = 1
		erate, eidx, ok := getSampleRate(r)
		if !ok {
			return nil, fmt.Errorf("%w: ext sampling", ErrBadASC)
		}
		c.ExtSampleRate, c.ExtSamplingIdx = erate, eidx
		not, ok := getObjectType(r)
		if !ok {
			return nil, fmt.Errorf("%w: ext object", ErrBadASC)
		}
		c.ObjectType = not
		ot = not
	} else {
		c.ExtObjectType = AOTNull
	}
	// Frame length flag for the AAC family.
	switch ot {
	case AOTMain, AOTLC, AOTSSR, AOTLTP, 17:
		f, ok := r.read(1)
		if !ok {
			return nil, fmt.Errorf("%w: frame length flag", ErrBadASC)
		}
		c.FrameShort = f == 1
		if c.FrameShort {
			c.FrameLength = 960
		} else {
			c.FrameLength = 1024
		}
	default:
		c.FrameLength = 1024
	}
	if syncExt && c.ExtObjectType != AOTSBR {
		scanSyncExtension(r, c)
	}
	if c.SBR == 0 {
		c.PS = 0
	}
	if (c.PS == -1 && c.ObjectType != AOTLC) || (c.Channels&^1) != 0 {
		if c.PS == -1 {
			c.PS = 0
		}
	}
	return c, nil
}

// scanSyncExtension looks for the 0x2B7 sync extension (SBR/PS) after the
// base config, mirroring the trailing loop in ff_mpeg4audio_get_config_gb.
func scanSyncExtension(r *bitReader, c *Config) {
	for r.left() > 15 {
		v, ok := r.show(11)
		if !ok {
			return
		}
		if v == 0x2b7 {
			r.read(11)
			ot, ok := getObjectType(r)
			if !ok {
				return
			}
			c.ExtObjectType = ot
			if ot == AOTSBR {
				s, ok := r.read(1)
				if !ok {
					return
				}
				if s == 1 {
					rate, idx, ok := getSampleRate(r)
					if !ok {
						return
					}
					c.ExtSampleRate, c.ExtSamplingIdx = rate, idx
					if rate == c.SampleRate {
						c.SBR = -1
					} else {
						c.SBR = 1
					}
				} else {
					c.SBR = 0
				}
			}
			if r.left() > 11 {
				if w, ok := r.show(11); ok && w == 0x548 {
					r.read(11)
					if p, ok := r.read(1); ok && c.SBR == 1 {
						c.PS = p
					}
				}
			}
			return
		}
		r.read(1)
	}
}

// ProfileName maps the object type to the ffprobe profile spelling.
func (c *Config) ProfileName() string {
	switch c.ObjectType {
	case AOTMain:
		return "Main"
	case AOTLC:
		return "LC"
	case AOTSSR:
		return "SSR"
	case AOTLTP:
		return "LTP"
	case AOTSBR:
		return "HE-AAC"
	case AOTPS:
		return "HE-AACv2"
	default:
		return "unknown"
	}
}
