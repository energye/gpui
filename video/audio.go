package video

import (
	"fmt"
	"os"
	"sync"

	"github.com/energye/gpui/video/aac"
	"github.com/energye/gpui/video/mp4"
)

// CodecAAC is the A1 audio registry name. It appears as a literal exactly
// once (the init below); everything else refers to this constant.
const CodecAAC = "aac"

// AudioDecoder is the audio codec interface A1 decodes through: configure
// once from the esds ASC, then feed one raw_data_block per packet. A fresh
// instance starts clean and decodes to real PCM.
type AudioDecoder interface {
	Configure(asc []byte) error
	DecodePacket(pkt []byte, ptsMs int64) (*aac.PCMFrame, error)
	Config() *aac.Config
}

type audioEntry struct {
	factory func() AudioDecoder
}

var (
	audioMu       sync.RWMutex
	audioDecoders = map[string]audioEntry{}
)

// RegisterAudioDecoder plugs a new audio codec in. Registering an existing
// name replaces it (tests use this to prove the table is consulted).
func RegisterAudioDecoder(codec string, factory func() AudioDecoder) {
	if codec == "" || factory == nil {
		return
	}
	audioMu.Lock()
	defer audioMu.Unlock()
	audioDecoders[codec] = audioEntry{factory: factory}
}

// SupportedAudioCodecs lists registered audio codecs in sorted order.
func SupportedAudioCodecs() []string {
	audioMu.RLock()
	defer audioMu.RUnlock()
	out := make([]string, 0, len(audioDecoders))
	for k := range audioDecoders {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// NewAudioDecoder builds a clean audio decoder for a registry codec name.
func NewAudioDecoder(codec string) (AudioDecoder, error) {
	audioMu.RLock()
	e, ok := audioDecoders[codec]
	audioMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q (have %v)", ErrUnsupportedCodec, codec, SupportedAudioCodecs())
	}
	d := e.factory()
	if d == nil {
		return nil, fmt.Errorf("%w: %q factory returned nil", ErrUnsupportedCodec, codec)
	}
	return d, nil
}

// aacAudioDecoder adapts the AAC front end to the audio registry.
type aacAudioDecoder struct{ d *aac.Decoder }

func (a *aacAudioDecoder) Configure(asc []byte) error { return a.d.Configure(asc) }

func (a *aacAudioDecoder) DecodePacket(pkt []byte, ptsMs int64) (*aac.PCMFrame, error) {
	return a.d.DecodePacket(pkt, ptsMs)
}

func (a *aacAudioDecoder) Config() *aac.Config { return a.d.Config() }

// AudioInfo describes one parsed AAC track (A1 demux evidence).
type AudioInfo struct {
	Path       string
	Codec      string
	Profile    string
	SampleRate int
	Channels   int
	Samples    int
	DurationMs int64
	ASC        []byte
}

// ProbeAudio parses the shell and reports the AAC track without decoding.
// Video presence is not required here; silent clips report ErrNoAudio.
func ProbeAudio(path string) (AudioInfo, error) {
	movie, err := mp4.ParseFile(path)
	if err != nil {
		return AudioInfo{}, err
	}
	return audioInfoFromMovie(movie, path)
}

// ProbeAudioSource is the Source twin of ProbeAudio.
func ProbeAudioSource(src Source) (AudioInfo, error) {
	if src == nil {
		return AudioInfo{}, fmt.Errorf("%w: nil source", ErrNoAudio)
	}
	movie, err := mp4.ParseReader(src, src.Size())
	if err != nil {
		return AudioInfo{}, err
	}
	return audioInfoFromMovie(movie, src.Name())
}

func audioInfoFromMovie(movie *mp4.Movie, path string) (AudioInfo, error) {
	if movie == nil || movie.Audio == nil {
		return AudioInfo{}, fmt.Errorf("%w: %s", ErrNoAudio, path)
	}
	a := movie.Audio
	if a.Codec != "mp4a" || len(a.ASC) == 0 {
		return AudioInfo{}, fmt.Errorf("%w: %s codec %q", ErrNoAudio, path, a.Codec)
	}
	cfg, err := aac.ParseASC(a.ASC, true)
	if err != nil {
		return AudioInfo{}, err
	}
	return AudioInfo{
		Path:       path,
		Codec:      CodecAAC,
		Profile:    cfg.ProfileName(),
		SampleRate: cfg.SampleRate,
		Channels:   cfg.Channels,
		Samples:    a.SampleCount,
		DurationMs: a.DurationMs,
		ASC:        append([]byte(nil), a.ASC...),
	}, nil
}

// ReadAudioPacket reads one raw AAC packet payload (one raw_data_block)
// by packet index (0-based). Caller feeds it to AudioDecoder.
func ReadAudioPacket(path string, n int) ([]byte, int64, error) {
	movie, err := mp4.ParseFile(path)
	if err != nil {
		return nil, 0, err
	}
	if movie.Audio == nil {
		return nil, 0, fmt.Errorf("%w: %s", ErrNoAudio, path)
	}
	s, ok := movie.Audio.SampleAt(n)
	if !ok {
		return nil, 0, fmt.Errorf("%w: packet %d of %d", ErrBadClip, n, movie.Audio.SampleCount)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	buf := make([]byte, s.Size)
	if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
		return nil, 0, err
	}
	return buf, s.PTSMs, nil
}

func init() {
	RegisterAudioDecoder(CodecAAC, func() AudioDecoder { return &aacAudioDecoder{d: &aac.Decoder{}} })
}
