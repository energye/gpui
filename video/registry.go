package video

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/h265"
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

// ContainerMP4 and CodecH264 are the first-stage registry names, plus
// CodecH265 for the V2-1 header entry. They appear as literals exactly
// once each (the init below); everything else refers to these constants
// or to names the tables hand back.
const (
	ContainerMP4 = "mp4"
	CodecH264    = "h264"
	CodecH265    = "h265"
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

// containerNames snapshots the shell table's names in sorted order.
func containerNames() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(containers))
	for k := range containers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// decoderNames snapshots the codec table's names in sorted order.
func decoderNames() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(decoders))
	for k := range decoders {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// SupportedContainers lists registered shells in sorted order for
// capability queries (VR9 asks first, then opens).
func SupportedContainers() []string { return containerNames() }

// SupportedCodecs lists registered codecs in sorted order.
func SupportedCodecs() []string { return decoderNames() }

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

// probeOpen runs stat + probe loop + shell open once for both ProbeFile
// and openViaRegistry, so the two entry points never drift. Missing files
// keep their os error; parses keep their sentinel chain; only the truly
// unknown shell becomes ErrUnsupportedContainer. On shell-open failure
// container names the claiming shell (callers report it); otherwise "".
func probeOpen(path string) (movie *mp4.Movie, container, codec string, err error) {
	if IsURL(path) {
		src, serr := NewSource(path)
		if serr != nil {
			return nil, "", "", serr
		}
		defer src.Close()
		return openViaSource(src, path)
	}
	if _, err := os.Stat(path); err != nil {
		return nil, "", "", err
	}
	for _, name := range containerNames() {
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
		return movie, name, codecName, nil
	}
	return nil, "", "", fmt.Errorf("%w: %s (have %v)", ErrUnsupportedContainer, path, SupportedContainers())
}

// openViaSource probes + parses a kept-open Source (streaming path:
// the caller keeps src for sample reads; do not close here). Movie
// carries sample tables; codec names the decoder entry.
func openViaSource(src Source, nameHint string) (movie *mp4.Movie, container, codec string, err error) {
	if src == nil {
		return nil, "", "", fmt.Errorf("%w: nil source %s", ErrUnsupportedContainer, nameHint)
	}
	if !mp4ProbeSource(src) {
		return nil, "", "", fmt.Errorf("%w: %s (have %v)", ErrUnsupportedContainer, src.Name(), SupportedContainers())
	}
	movie, codecName, err := mp4OpenSource(src)
	if err != nil {
		return nil, ContainerMP4, "", err
	}
	return movie, ContainerMP4, codecName, nil
}

// ProbeSource asks which shell/codec a Source carries, without decoding.
func ProbeSource(src Source) (container, codec string, err error) {
	movie, name, codecName, err := openViaSource(src, "")
	if err != nil {
		return name, "", err
	}
	if movie == nil || movie.Video == nil {
		return name, "", fmt.Errorf("%w: %s", ErrNoVideo, src.Name())
	}
	return name, codecName, nil
}

// ProbeFile asks the registry which shell/codec a path carries, without
// decoding. It powers capability-first UI ("ask, then open") and the
// VR9 window's registry proof. Missing files keep their os error so the
// fault triage still reports file-not-found instead of unsupported.
func ProbeFile(path string) (container, codec string, err error) {
	movie, name, codecName, err := probeOpen(path)
	if err != nil {
		return name, "", err
	}
	if movie == nil || movie.Video == nil {
		return name, "", fmt.Errorf("%w: %s", ErrNoVideo, path)
	}
	return name, codecName, nil
}

// openViaRegistry probes then opens, handing the player a parsed movie
// plus the registry names to decode through. Errors keep their sentinel
// chain (mp4/h264/os) so the VR6 triage keeps its buckets; only the
// truly-unknown shell becomes ErrUnsupportedContainer.
func openViaRegistry(path string) (movie *mp4.Movie, container, codec string, err error) {
	movie, name, codecName, err := probeOpen(path)
	if err != nil {
		return nil, name, "", err
	}
	if movie == nil || movie.Video == nil || len(movie.Video.Samples) == 0 {
		return nil, name, "", fmt.Errorf("%w: %s", ErrNoVideo, path)
	}
	return movie, name, codecName, nil
}

// h264Decoder adapts the H.264 decoder to the registry interface.
type h264Decoder struct{ d *h264.Decoder }

func (h *h264Decoder) DecodeNALU(nalu []byte) error { return h.d.DecodeNALU(nalu) }

func (h *h264Decoder) FinishPicture() (*h264.Picture, error) { return h.d.FinishPicture() }

func (h *h264Decoder) Sampling() string { return color.SamplingYUV420P }

// h265Decoder adapts the V2-1 H.265 header decoder to the registry
// interface. The hvcC arrives per open (each clip carries its own sets),
// so the factory builds an empty shell and feedH265Params fills it.
type h265Decoder struct{ d *h265.Decoder }

func (h *h265Decoder) DecodeNALU(nalu []byte) error { return h.d.DecodeNALU(nalu) }

func (h *h265Decoder) FinishPicture() (*h264.Picture, error) { return h.d.FinishPicture() }

func (h *h265Decoder) Sampling() string { return color.SamplingYUV420P }

// mp4ProbeFile answers "is this file mine" from head+tail sniffing:
// mp4.Probe checks the ftyp brand or a moov box, and fast-start files
// front-load moov while plain files tail-load it, so both ends are read.
func mp4ProbeFile(path string) bool {
	if IsURL(path) {
		src, err := NewSource(path)
		if err != nil {
			return false
		}
		defer src.Close()
		return mp4ProbeSource(src)
	}
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

// mp4ProbeSource is the Source twin of mp4ProbeFile: head+tail sniff
// via ReadAt, so network sources probe with two Range GETs, never a
// full download.
func mp4ProbeSource(src Source) bool {
	if src == nil || src.Size() < 8 {
		return false
	}
	const sniff = 64 << 10
	head := make([]byte, sniff)
	n, _ := src.ReadAt(head, 0)
	if n > 0 && mp4.Probe(head[:n]) {
		return true
	}
	if src.Size() <= int64(n) {
		return false
	}
	tail := make([]byte, sniff)
	off := src.Size() - int64(len(tail))
	if off < 0 {
		off = 0
	}
	m, _ := src.ReadAt(tail, off)
	if m > 0 && mp4.Probe(tail[:m]) {
		return true
	}
	return false
}

// RejectUnits spots old-tool units inside one sample payload: data
// partitions and slice extension stop the open with a namable F17 error
// instead of guessing. Only this H.264 entry knows these types; the
// player calls through with the codec name, so no NAL knowledge leaks
// into the core flow. Non-H.264 codecs have no F17 shapes: pass.
func RejectUnits(codec string, units [][]byte, sampleNum int, path string) error {
	if codec != CodecH264 {
		return nil
	}
	for _, u := range units {
		t, ok := h264.NALType(u)
		if !ok {
			continue
		}
		switch t {
		case h264.NALSlicePartA, h264.NALSlicePartB, h264.NALSlicePartC:
			return fmt.Errorf("video: sample %d F17 data partition %s %s: %w", sampleNum, path, h264.TypeName(t), h264.ErrDataPartitioning)
		case h264.NALSliceExt:
			return fmt.Errorf("video: sample %d F17 slice extension %s: %w", sampleNum, path, h264.ErrUnsupportedNAL)
		}
	}
	return nil
}

// mp4Open parses the shell and reports the carried codec. AVC tracks
// answer the registered H.264 name, HEVC tracks (hvc1/hev1 + hvcC) answer
// the V2-1 H.265 name; a shell with neither config fails as an
// unsupported codec with the supported set named.
func mp4Open(path string) (*mp4.Movie, string, error) {
	if IsURL(path) {
		src, err := NewSource(path)
		if err != nil {
			return nil, "", err
		}
		defer src.Close()
		return mp4OpenSource(src)
	}
	movie, err := mp4.ParseFile(path)
	if err != nil {
		return nil, "", err
	}
	if movie.Video == nil {
		return movie, "", fmt.Errorf("%w: container %s without video config %s (have %v)", ErrUnsupportedCodec, ContainerMP4, path, SupportedCodecs())
	}
	if len(movie.Video.AVCConfig) > 0 {
		return movie, CodecH264, nil
	}
	if len(movie.Video.HEVCConfig) > 0 {
		return movie, CodecH265, nil
	}
	return movie, "", fmt.Errorf("%w: container %s without AVC/HEVC config %s (have %v)", ErrUnsupportedCodec, ContainerMP4, path, SupportedCodecs())
}

// mp4OpenSource parses a kept-open Source (streaming path): only box
// headers + moov travel, mdat payload stays on the server until samples
// stream. Same AVC/HEVC gate as mp4Open.
func mp4OpenSource(src Source) (*mp4.Movie, string, error) {
	movie, err := mp4.ParseReader(src, src.Size())
	if err != nil {
		return nil, "", err
	}
	if movie.Video == nil {
		return movie, "", fmt.Errorf("%w: container %s without video config %s (have %v)", ErrUnsupportedCodec, ContainerMP4, src.Name(), SupportedCodecs())
	}
	if len(movie.Video.AVCConfig) > 0 {
		return movie, CodecH264, nil
	}
	if len(movie.Video.HEVCConfig) > 0 {
		return movie, CodecH265, nil
	}
	return movie, "", fmt.Errorf("%w: container %s without AVC/HEVC config %s (have %v)", ErrUnsupportedCodec, ContainerMP4, src.Name(), SupportedCodecs())
}

func init() {
	RegisterContainer(ContainerMP4, mp4ProbeFile, mp4Open)
	RegisterDecoder(CodecH264, func() Decoder { return &h264Decoder{d: h264.NewDecoder(nil)} },
		func(buf []byte, lengthSize int) ([][]byte, error) { return h264.SplitAVCC(buf, lengthSize) })
	RegisterDecoder(CodecH265, func() Decoder { return &h265Decoder{d: h265.NewDecoder(nil)} },
		func(buf []byte, lengthSize int) ([][]byte, error) { return h265.SplitHVCC(buf, lengthSize) })
}
