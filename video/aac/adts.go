package aac

import "fmt"

// Header is one parsed ADTS frame header. It peers AACADTSHeaderInfo
// (adts_header.h) plus ff_adts_header_parse (adts_header.c): syncword,
// profile, sampling index, channel config, frame length, block count.
type Header struct {
	ObjectType   int
	SamplingIdx  int
	SampleRate   int
	ChanConfig   int
	FrameLength  int
	NumFrames    int
	Samples      int
	CRCAbsent    bool
	HeaderLength int
}

// ParseHeader parses the first ADTS frame header in buf (7 or 9 bytes).
func ParseHeader(buf []byte) (*Header, error) {
	if len(buf) < 7 {
		return nil, fmt.Errorf("%w: need 7 got %d", ErrTruncated, len(buf))
	}
	if buf[0] != 0xff || buf[1]&0xf0 != 0xf0 {
		return nil, fmt.Errorf("%w: syncword", ErrBadADTS)
	}
	protAbsent := buf[1]&0x01 == 1
	profile := int((buf[2] >> 6) & 0x03)
	srIdx := int((buf[2] >> 2) & 0x0f)
	if srIdx >= len(SampleRates) || SampleRates[srIdx] == 0 {
		return nil, fmt.Errorf("%w: sampling index %d", ErrBadADTS, srIdx)
	}
	chanCfg := int(((buf[2] & 0x01) << 2) | ((buf[3] >> 6) & 0x03))
	frameLen := int(buf[3]&0x03)<<11 | int(buf[4])<<3 | int((buf[5]>>5)&0x07)
	minLen := 7
	if !protAbsent {
		minLen = 9
	}
	if frameLen < minLen {
		return nil, fmt.Errorf("%w: frame length %d", ErrBadADTS, frameLen)
	}
	if len(buf) < frameLen {
		return nil, fmt.Errorf("%w: frame %d need %d", ErrTruncated, len(buf), frameLen)
	}
	numBlocks := int(buf[6] & 0x03)
	h := &Header{
		ObjectType:   profile + 1,
		SamplingIdx:  srIdx,
		SampleRate:   SampleRates[srIdx],
		ChanConfig:   chanCfg,
		FrameLength:  frameLen,
		NumFrames:    numBlocks + 1,
		Samples:      (numBlocks + 1) * 1024,
		CRCAbsent:    protAbsent,
		HeaderLength: minLen,
	}
	if h.NumFrames != 1 {
		return nil, fmt.Errorf("%w: %d raw blocks, want 1", ErrUnsupported, h.NumFrames)
	}
	return h, nil
}

// SplitADTS cuts a bare ADTS byte stream into raw_data_block payloads
// (header + CRC stripped). It mirrors the aac_decode_frame_int gate:
// every frame must open with 0xFFF, otherwise the stream is namable.
func SplitADTS(data []byte) ([][]byte, error) {
	var out [][]byte
	off := 0
	for off < len(data) {
		if len(data)-off < 7 {
			return nil, fmt.Errorf("%w: tail %d at %d", ErrTruncated, len(data)-off, off)
		}
		h, err := ParseHeader(data[off:])
		if err != nil {
			return nil, err
		}
		start := off + h.HeaderLength
		end := off + h.FrameLength
		out = append(out, data[start:end])
		off = end
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: empty", ErrBadADTS)
	}
	return out, nil
}
