package mp4

import "errors"

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrTruncated      = errors.New("mp4: truncated file or box extends past end")
	ErrBadBox         = errors.New("mp4: bad box size or layout")
	ErrNoMoov         = errors.New("mp4: missing moov box")
	ErrNoVideoTrack   = errors.New("mp4: no video track found")
	ErrNoSampleTable  = errors.New("mp4: video track has no sample table")
	ErrBadSampleTable = errors.New("mp4: inconsistent sample table")
	ErrFragmented     = errors.New("mp4: fragmented mp4 (moof) not supported in this stage")
	ErrUnsupported    = errors.New("mp4: unsupported feature for this stage")
)

// Movie is the parsed file shell.
type Movie struct {
	MajorBrand string
	Compatible []string
	Timescale  uint32
	Duration   uint64
	DurationMs int64
	Fragmented bool
	Tracks     []*Track
	Video      *Track
}

// HasVideo reports whether a video track was found.
func (m *Movie) HasVideo() bool { return m != nil && m.Video != nil }

// EditEntry is one edit-list row.
type EditEntry struct {
	SegmentDuration uint64
	MediaTime       int64
	MediaRate       uint32
}

// Sample describes one compressed sample in decode order.
type Sample struct {
	Number   int
	Size     uint32
	Offset   uint64
	DTS      int64
	PTS      int64
	DTSMs    int64
	PTSMs    int64
	Keyframe bool
}

// Keyframe is the seek index entry.
type Keyframe struct {
	SampleNumber int
	Offset       uint64
	DTSMs        int64
	PTSMs        int64
}

// Track is one parsed trak box. Only video tracks carry sample tables.
type Track struct {
	ID           uint32
	Handler      string
	Codec        string
	CodedWidth   uint32
	CodedHeight  uint32
	Width        uint32
	Height       uint32
	Rotation     int
	Timescale    uint32
	Duration     uint64
	DurationMs   int64
	FrameRate    float64
	SampleCount  int
	Samples      []Sample
	Keyframes    []Keyframe
	EditList     []EditEntry
	AVCConfig    []byte
	PixelAspectH uint32
	PixelAspectV uint32
	HasCTTS      bool
	HasEditList  bool
}

// KeyframeCount is a convenience for metrics.
func (t *Track) KeyframeCount() int {
	if t == nil {
		return 0
	}
	return len(t.Keyframes)
}

// SampleAt returns the n-th sample in decode order (0-based).
func (t *Track) SampleAt(n int) (Sample, bool) {
	if t == nil || n < 0 || n >= len(t.Samples) {
		return Sample{}, false
	}
	return t.Samples[n], true
}

// KeyframeNear returns the last keyframe at or before the given
// presentation timestamp in milliseconds.
func (t *Track) KeyframeNear(ptsMs int64) (Keyframe, bool) {
	if t == nil || len(t.Keyframes) == 0 {
		return Keyframe{}, false
	}
	best := t.Keyframes[0]
	found := ptsMs >= best.PTSMs
	if !found {
		return t.Keyframes[0], true
	}
	for _, k := range t.Keyframes[1:] {
		if k.PTSMs <= ptsMs {
			best = k
		} else {
			break
		}
	}
	return best, true
}
