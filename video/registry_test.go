package video

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
)

// stubGrayDecoder is the VR9 plugability proof: a second decoder that
// arrives purely through RegisterDecoder, with no core change. It ignores
// input units and finishes one flat-grey synthetic picture.
type stubGrayDecoder struct {
	w, h uint32
	fed  int
}

func (s *stubGrayDecoder) DecodeNALU(nalu []byte) error {
	s.fed++
	return nil
}

func (s *stubGrayDecoder) FinishPicture() (*h264.Picture, error) {
	p, err := h264.NewPicture(s.w, s.h)
	if err != nil {
		return nil, err
	}
	for i := range p.Y {
		p.Y[i] = 180
	}
	return p, nil
}

func (s *stubGrayDecoder) Sampling() string { return color.SamplingYUV420P }

func stubGraySplit(buf []byte, lengthSize int) ([][]byte, error) {
	return [][]byte{buf}, nil
}

// TestRegistrySupported pins the capability query: the first-stage set
// plus the V2-1 H.265 header entry is present and listed, so UI asks
// first instead of guessing.
func TestRegistrySupported(t *testing.T) {
	var hasC, hasD, hasH265, hasS bool
	for _, s := range SupportedContainers() {
		if s == "mp4" {
			hasC = true
		}
	}
	for _, s := range SupportedCodecs() {
		if s == "h264" {
			hasD = true
		}
		if s == CodecH265 {
			hasH265 = true
		}
	}
	for _, s := range SupportedSamplings() {
		if s == color.SamplingYUV420P {
			hasS = true
		}
	}
	if !hasC || !hasD || !hasH265 || !hasS {
		t.Fatalf("supported containers=%v codecs=%v samplings=%v, want mp4+h264+h265+yuv420p present",
			SupportedContainers(), SupportedCodecs(), SupportedSamplings())
	}
}

// TestRegistryOpenClip pins the registry play path: the same clip that
// probes also opens and plays, and Info carries the registry names.
func TestRegistryOpenClip(t *testing.T) {
	container, codec, err := ProbeFile("testdata/vr2_720p.mp4")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if container == "" || codec == "" {
		t.Fatalf("probe names = %q/%q, want non-empty", container, codec)
	}
	p, err := OpenFile("testdata/vr2_720p.mp4", Options{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if p.Info().Container != container || p.Info().Codec != codec {
		t.Fatalf("info = %q/%q, probe = %q/%q", p.Info().Container, p.Info().Codec, container, codec)
	}
	deadline := WallDeadline(15000)
	var shown int64
	for {
		f, done := p.Poll()
		if f != nil {
			shown++
		}
		if done {
			break
		}
		if WallPast(deadline) {
			t.Fatal("play timed out before Ended")
		}
		WallSleep(5)
	}
	if shown != int64(p.Info().Frames) || shown == 0 {
		t.Fatalf("shown = %d, frames = %d", shown, p.Info().Frames)
	}
}

// TestRegistryStubPluggable pins second-decoder plugability: registering
// a new codec name makes it constructible, splittable and convertible
// through the color registry, with no core edit.
func TestRegistryStubPluggable(t *testing.T) {
	const stub = "test-stub-gray"
	RegisterDecoder(stub, func() Decoder { return &stubGrayDecoder{w: 96, h: 96} }, stubGraySplit)
	found := false
	for _, s := range SupportedCodecs() {
		if s == stub {
			found = true
		}
	}
	if !found {
		t.Fatalf("codecs %v lack %q after register", SupportedCodecs(), stub)
	}
	d, err := NewDecoder(stub)
	if err != nil {
		t.Fatalf("new stub: %v", err)
	}
	units, err := SplitUnits(stub, []byte{0x01, 0x02}, 4)
	if err != nil || len(units) != 1 {
		t.Fatalf("stub split = %v, %v", units, err)
	}
	for _, u := range units {
		if err := d.DecodeNALU(u); err != nil {
			t.Fatalf("stub feed: %v", err)
		}
	}
	pic, err := d.FinishPicture()
	if err != nil {
		t.Fatalf("stub finish: %v", err)
	}
	if pic.Width != 96 || pic.Height != 96 {
		t.Fatalf("stub size = %dx%d, want 96x96", pic.Width, pic.Height)
	}
	cf, err := color.Convert(d.Sampling(), pic.Y, pic.Cb, pic.Cr, int(pic.Width), int(pic.Height), color.Options{})
	if err != nil {
		t.Fatalf("stub color: %v", err)
	}
	if len(cf.Pix) != 96*96*4 {
		t.Fatalf("stub rgba len = %d", len(cf.Pix))
	}
	// Flat grey in, flat grey out: R/G/B close, alpha opaque.
	r, g, b, a := cf.At(48, 48)
	if a != 255 {
		t.Fatalf("stub alpha = %d, want 255", a)
	}
	if diff(r, g) > 2 || diff(g, b) > 2 {
		t.Fatalf("stub pixel not grey: %d %d %d", r, g, b)
	}
}

func diff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestRegistryUnsupported pins readable failure: junk shells and unknown
// codecs name the supported set and land in the bad-clip bucket, never a
// panic and never an empty error.
func TestRegistryUnsupported(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.bin")
	if err := os.WriteFile(junk, []byte("this is not a video shell at all, just text"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := OpenFile(junk, Options{NowMs: func() int64 { return 0 }})
	if err == nil {
		t.Fatal("junk shell opens")
	}
	if !errors.Is(err, ErrUnsupportedContainer) {
		t.Fatalf("junk err = %v, want %v", err, ErrUnsupportedContainer)
	}
	if got := Classify(err).Kind; got != KindBadClip {
		t.Fatalf("junk kind = %q, want %q", got, KindBadClip)
	}
	if _, err := NewDecoder("bogus-codec-xxx"); !errors.Is(err, ErrUnsupportedCodec) {
		t.Fatalf("bogus codec err = %v, want %v", err, ErrUnsupportedCodec)
	}
	if _, err := SplitUnits("bogus-codec-xxx", []byte{1}, 4); !errors.Is(err, ErrUnsupportedCodec) {
		t.Fatalf("bogus split err = %v, want %v", err, ErrUnsupportedCodec)
	}
	if _, _, err := ProbeFile(filepath.Join(dir, "does-not-exist.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing probe err = %v, want not-exist", err)
	}
}

// TestNoHardcodedNames is the VR9熔断 lock: the core flow (player.go)
// never switches on a container/codec/sampling name literal. Package
// import paths (video/h264, video/mp4) are the wiring the registry
// itself owns; the player only handles the names the tables hand back.
// fault.go triage labels are outside this gate on purpose.
func TestNoHardcodedNames(t *testing.T) {
	// Only string literals count: branch/switch/compare operands that
	// name a format. Import paths are registry wiring, not flow logic.
	banned := []string{"\"mp4\"", "\"h264\"", "\"yuv420p\"", "\"avc\"", "\"avc1\"", "\"hevc\"", "\"h265\"", "\"vp8\"", "\"vp9\"", "\"av1\"", "\"mkv\"", "\"webm\"", "\"mov\""}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "player.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		low := strings.ToLower(lit.Value)
		for _, b := range banned {
			if strings.Contains(low, b) {
				pos := fset.Position(lit.Pos())
				bad = append(bad, pos.String()+": "+lit.Value)
			}
		}
		return true
	})
	if len(bad) > 0 {
		t.Fatalf("player.go hardcodes format names:\n%s", strings.Join(bad, "\n"))
	}
}
