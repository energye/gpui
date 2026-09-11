package h264

import "fmt"

// AVCC holds the parsed avcC box: the out-of-band parameter sets plus
// the NALU length field size for MP4 sample payloads.
type AVCC struct {
	Profile     uint8
	ProfileName string
	Compat      uint8
	Level       uint8
	LengthSize  int
	SPS         [][]byte
	PPS         [][]byte
	Raw         []byte
}

// ParseAVCC parses an avcC box payload (the box content after type).
func ParseAVCC(box []byte) (*AVCC, error) {
	if len(box) < 7 {
		return nil, fmt.Errorf("%w: %d bytes", ErrBadAVCC, len(box))
	}
	if box[0] != 1 {
		return nil, fmt.Errorf("%w: version %d", ErrBadAVCC, box[0])
	}
	a := &AVCC{
		Profile:     box[1],
		ProfileName: ProfileName(box[1]),
		Compat:      box[2],
		Level:       box[3],
		LengthSize:  int(box[4]&0x03) + 1,
	}
	nSPS := int(box[5] & 0x1F)
	off := 6
	for i := 0; i < nSPS; i++ {
		if len(box)-off < 2 {
			return nil, fmt.Errorf("%w: sps %d length", ErrBadAVCC, i)
		}
		n := int(box[off])<<8 | int(box[off+1])
		off += 2
		if n <= 0 || len(box)-off < n {
			return nil, fmt.Errorf("%w: sps %d wants %d", ErrBadAVCC, i, n)
		}
		a.SPS = append(a.SPS, append([]byte(nil), box[off:off+n]...))
		off += n
	}
	if len(box)-off < 1 {
		return nil, fmt.Errorf("%w: missing pps count", ErrBadAVCC)
	}
	nPPS := int(box[off])
	off++
	for i := 0; i < nPPS; i++ {
		if len(box)-off < 2 {
			return nil, fmt.Errorf("%w: pps %d length", ErrBadAVCC, i)
		}
		n := int(box[off])<<8 | int(box[off+1])
		off += 2
		if n <= 0 || len(box)-off < n {
			return nil, fmt.Errorf("%w: pps %d wants %d", ErrBadAVCC, i, n)
		}
		a.PPS = append(a.PPS, append([]byte(nil), box[off:off+n]...))
		off += n
	}
	if len(a.SPS) == 0 {
		return nil, fmt.Errorf("%w: no sps in avcC", ErrNoParamSets)
	}
	a.Raw = append([]byte(nil), box...)
	return a, nil
}
