package aac

import "fmt"

// PCMFrame is one decoded AAC packet: 1024 float samples per channel,
// interleaved in Data.
type PCMFrame struct {
	SampleRate int
	Channels   int
	Samples    int // per channel (1024)
	Data       []float32
	PTSMs      int64
}

// Decoder holds an ASC-configured AAC-LC front end with overlap state.
// Configure succeeds for LC/Main/LTP; DecodePacket decodes one
// raw_data_block to PCM (landing 2).
type Decoder struct {
	cfg *Config
	asc []byte
	st  *decoderState
}

// Configure stores the esds ASC and validates the object type.
func (d *Decoder) Configure(asc []byte) error {
	if len(asc) == 0 {
		return fmt.Errorf("%w: empty asc", ErrBadASC)
	}
	c, err := ParseASC(asc, true)
	if err != nil {
		return err
	}
	switch c.ObjectType {
	case AOTMain, AOTLC, AOTLTP:
	default:
		return fmt.Errorf("aac: object %d (%s): %w", c.ObjectType, c.ProfileName(), ErrUnsupportedAOT)
	}
	if c.ChanConfig == 0 || c.Channels == 0 {
		return fmt.Errorf("%w: chan_config 0 needs PCE (landing 2)", ErrUnsupported)
	}
	d.cfg = c
	d.asc = append([]byte(nil), asc...)
	d.st = newDecoderState()
	return nil
}

// Config returns the active configuration, nil before Configure.
func (d *Decoder) Config() *Config { return d.cfg }

// DecodePacket decodes one raw_data_block to interleaved float PCM.
// Packet framing errors stay distinguishable from spectral errors.
func (d *Decoder) DecodePacket(pkt []byte, ptsMs int64) (*PCMFrame, error) {
	if d.cfg == nil {
		return nil, fmt.Errorf("%w: configure first", ErrBadASC)
	}
	if len(pkt) == 0 {
		return nil, fmt.Errorf("%w: empty packet", ErrTruncated)
	}
	if d.st == nil {
		d.st = newDecoderState()
	}
	r := newBitReaderMSB(pkt)
	frame, err := decodeRawBlock(r, d.cfg, d.st)
	if err != nil {
		return nil, err
	}
	if len(frame.channels) != d.cfg.Channels {
		return nil, fmt.Errorf("%w: got %d ch want %d", ErrBadADTS, len(frame.channels), d.cfg.Channels)
	}
	n := len(frame.channels[0])
	inter := make([]float32, n*d.cfg.Channels)
	for i := 0; i < n; i++ {
		for c := 0; c < d.cfg.Channels; c++ {
			inter[i*d.cfg.Channels+c] = float32(frame.channels[c][i])
		}
	}
	return &PCMFrame{
		SampleRate: d.cfg.SampleRate,
		Channels:   d.cfg.Channels,
		Samples:    n,
		Data:       inter,
		PTSMs:      ptsMs,
	}, nil
}
