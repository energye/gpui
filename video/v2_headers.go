package video

import (
	"fmt"
	"os"

	"github.com/energye/gpui/video/h265"
	"github.com/energye/gpui/video/mp4"
)

// H265Headers is the V2-1 header evidence: box truth plus hvcC shape,
// read through the registry without decoding pixels. Windows show these
// numbers; V2-2 will turn them into pictures.
type H265Headers struct {
	Codec      string
	Width      int
	Height     int
	Profile    string
	Level      int
	LengthSize int
	VPS        int
	SPS        int
	PPS        int
	Samples    int
	Units      int
}

// ProbeH265Headers reads the H.265 box headers of path through the same
// registry open as the player (probe + shell parse + hvcC parse + sample
// split). It never decodes pixels; slice payloads are not touched beyond
// framing. Non-H.265 clips fail with the registry codec error.
func ProbeH265Headers(path string) (*H265Headers, error) {
	movie, container, codec, err := probeOpen(path)
	if err != nil {
		return nil, err
	}
	if codec != CodecH265 || container != ContainerMP4 {
		return nil, fmt.Errorf("%w: %s is %s/%s, want mp4/h265", ErrUnsupportedCodec, path, container, codec)
	}
	v := movie.Video
	if v == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoVideo, path)
	}
	h, err := h265.ParseHVCC(v.HEVCConfig)
	if err != nil {
		return nil, fmt.Errorf("video: header params %s: %w", path, err)
	}
	out := &H265Headers{
		Codec:      codec,
		Width:      int(v.Width),
		Height:     int(v.Height),
		Profile:    h.ProfileName,
		Level:      int(h.Level),
		LengthSize: h.LengthSize,
		VPS:        len(h.VPS),
		SPS:        len(h.SPS),
		PPS:        len(h.PPS),
		Samples:    len(v.Samples),
	}
	if out.Width == 0 {
		out.Width = int(v.CodedWidth)
	}
	if out.Height == 0 {
		out.Height = int(v.CodedHeight)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	for _, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			return nil, fmt.Errorf("video: sample %d unreadable %s: %w", s.Number, path, mp4.ErrTruncated)
		}
		units, err := h265.SplitHVCC(buf, h.LengthSize)
		if err != nil {
			return nil, fmt.Errorf("video: sample %d split %s: %w", s.Number, path, err)
		}
		out.Units += len(units)
	}
	return out, nil
}
