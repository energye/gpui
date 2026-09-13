package video

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// Container and decoder registries (§4.4, VR9): the player programs
// against these tables and never hardcodes a container, codec or
// sampling name. New formats arrive as new packages + Register calls;
// the core flow stays untouched. Pure Go, standard library only.
//
// The single place that names the first-stage formats is the init()
// below: container "mp4", codec "h264". Everything else (player,
// seek, windows) only handles the names the registry hands back.

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrUnsupportedContainer = errors.New("video: unsupported container")
	ErrUnsupportedCodec     = errors.New("video: unsupported codec")
)

// Decoder is the codec interface the player decodes through: feed
// compressed units, finish one picture, report the sampling the picture
// carries for the color registry. A fresh instance starts clean, so
// error isolation rebuilds via NewDecoder instead of a reset method.
type Decoder interface {
	DecodeNALU(nalu []byte) error
	FinishPicture() (*h264.Picture, error)
	Sampling() string
}

// SplitFunc cuts one container sample payload into codec units.
// H.264 splits AVCC length-prefixed payloads; future codecs register
// their own framing here so the player never switches on codec names.
type SplitFunc func(buf []byte, lengthSize int) ([][]byte, error)

type containerEntry struct {
	probe func(path string) bool
	open  func(path string) (*mp4.Movie, string, error)
}

type decoderEntry struct {
	factory func() Decoder
	split   SplitFunc
}

var (
	regMu      sync.RWMutex
	containers = map[string]containerEntry{}
	decoders   = map[string]decoderEntry{}
)

// RegisterContainer plugs a new shell format in. Probe answers "is this
// file mine" without fully parsing; open parses and reports the codec
// name carried by the file. Registering an existing name replaces it.
func RegisterContainer(name string, probe func(path string) bool, open func(path string) (*mp4.Movie, string, error)) {
	if name == "" || probe == nil || open == nil {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	containers[name] = containerEntry{probe: probe, open: open}
}

// RegisterDecoder plugs a new codec in. Registering an existing name
// replaces it (tests use this to prove the table is consulted).
func RegisterDecoder(codec string, factory func() Decoder, split SplitFunc) {
	if codec == "" || factory == nil || split == nil {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	decoders[codec] = decoderEntry{factory: factory, split: split}
}

// SupportedContainers lists registered shells in sorted order for
// capability queries (VR9 asks first, then opens).
func SupportedContainers() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(containers))
	for k := range containers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// SupportedCodecs lists registered codecs in sorted order.
func SupportedCodecs() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(decoders))
	for k := range decoders {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// SupportedSamplings lists registered color samplings (delegates to the
// color registry, so there is exactly one sampling table).
func SupportedSamplings() []string { return color.Supported() }

// NewDecoder builds a clean decoder for a registry codec name, or a
// readable error naming the supported set (zero crash on unknown).
func NewDecoder(codec string) (Decoder, error) {
	regMu.RLock()
	e, ok := decoders[codec]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q (have %v)", ErrUnsupportedCodec, codec, SupportedCodecs())
	}
	d := e.factory()
	if d == nil {
		return nil, fmt.Errorf("%w: %q factory returned nil", ErrUnsupportedCodec, codec)
	}
	return d, nil
}

// SplitUnits cuts one sample payload through the codec's splitter.
func SplitUnits(codec string, buf []byte, lengthSize int) ([][]byte, error) {
	regMu.RLock()
	e, ok := decoders[codec]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q (have %v)", ErrUnsupportedCodec, codec, SupportedCodecs())
	}
	return e.split(buf, lengthSize)
}

// ProbeFile asks the registry which shell/codec a path carries, without
// decoding. It powers capability-first UI ("ask, then open") and the
// VR9 window's registry proof. Missing files keep their os error so the
// fault triage still reports file-not-found instead of unsupported.
func ProbeFile(path string) (container, codec string, err error) {
	if _, err := os.Stat(path); err != nil {
		return "", "", err
	}
	regMu.RLock()
	names := make([]string, 0, len(containers))
	for k := range containers {
		names = append(names, k)
	}
	regMu.RUnlock()
	sort.Strings(names)
	for _, name := range names {
		regMu.RLock()
		e := containers[name]
		regMu.RUnlock()
		if !e.probe(path) {
			continue
		}
		movie, codecName, err := e.open(path)
		if err != nil {
			return name, "", err
		}
		if movie == nil || movie.Video == nil {
			return name, "", fmt.Errorf("%w: %s", ErrNoVideo, path)
		}
		return name, codecName, nil
	}
	return "", "", fmt.Errorf("%w: %s (have %v)", ErrUnsupportedContainer, path, SupportedContainers())
}

// openViaRegistry probes then opens, handing the player a parsed movie
// plus the registry names to decode through. Errors keep their sentinel
// chain (mp4/h264/os) so the VR6 triage keeps its buckets; only the
// truly-unknown shell becomes ErrUnsupportedContainer.
func openViaRegistry(path string) (movie *mp4.Movie, container, codec string, err error) {
	if _, err := os.Stat(path); err != nil {
		return nil, "", "", err
	}
	regMu.RLock()
	names := make([]string, 0, len(containers))
	for k := range containers {
		names = append(names, k)
	}
	regMu.RUnlock()
	sort.Strings(names)
	for _, name := range names {
		regMu.RLock()
		e := containers[name]
		regMu.RUnlock()
		if !e.probe(path) {
			continue
		}
		movie, codecName, err := e.open(path)
		if err != nil {
			return nil, name, "", err
		}
		if movie == nil || movie.Video == nil || len(movie.Video.Samples) == 0 {
			return nil, name, "", fmt.Errorf("%w: %s", ErrNoVideo, path)
		}
		return movie, name, codecName, nil
	}
	return nil, "", "", fmt.Errorf("%w: %s (have %v)", ErrUnsupportedContainer, path, SupportedContainers())
}

// h264Decoder adapts the H.264 decoder to the registry interface.
type h264Decoder struct{ d *h264.Decoder }

func (h *h264Decoder) DecodeNALU(nalu []byte) error { return h.d.DecodeNALU(nalu) }

func (h *h264Decoder) FinishPicture() (*h264.Picture, error) { return h.d.FinishPicture() }

func (h *h264Decoder) Sampling() string { return color.SamplingYUV420P }

// mp4ProbeFile answers "is this file mine" from head+tail sniffing:
// mp4.Probe checks the ftyp brand or a moov box, and fast-start files
// front-load moov while plain files tail-load it, so both ends are read.
func mp4ProbeFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	const sniff = 64 << 10
	head := make([]byte, sniff)
	n, _ := f.Read(head)
	if n > 0 && mp4.Probe(head[:n]) {
		return true
	}
	fi, err := f.Stat()
	if err != nil || fi.Size() <= int64(n) {
		return false
	}
	tail := make([]byte, sniff)
	off := fi.Size() - int64(len(tail))
	if off < 0 {
		off = 0
	}
	m, _ := f.ReadAt(tail, off)
	if m > 0 && mp4.Probe(tail[:m]) {
		return true
	}
	return false
}

// mp4Open parses the shell and reports the carried codec. This stage
// only carries AVC, so the codec answer is the registered H.264 name;
// a shell without AVC config fails as an unsupported codec with the
// supported set named.
func mp4Open(path string) (*mp4.Movie, string, error) {
	movie, err := mp4.ParseFile(path)
	if err != nil {
		return nil, "", err
	}
	if movie.Video == nil || len(movie.Video.AVCConfig) == 0 {
		return movie, "", fmt.Errorf("%w: container mp4 without AVC config %s (have %v)", ErrUnsupportedCodec, path, SupportedCodecs())
	}
	return movie, "h264", nil
}

func init() {
	RegisterContainer("mp4", mp4ProbeFile, mp4Open)
	RegisterDecoder("h264", func() Decoder { return &h264Decoder{d: h264.NewDecoder(nil)} },
		func(buf []byte, lengthSize int) ([][]byte, error) { return h264.SplitAVCC(buf, lengthSize) })
}
