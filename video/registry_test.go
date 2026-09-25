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
)

// TestRegistrySupported pins the capability query: mp4 + h264/h265 +
// yuv420p stay present, so UI asks first instead of guessing.
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
		if s == "yuv420p" {
			hasS = true
		}
	}
	if !hasC || !hasD || !hasH265 || !hasS {
		t.Fatalf("supported containers=%v codecs=%v samplings=%v, want mp4+h264+h265+yuv420p present",
			SupportedContainers(), SupportedCodecs(), SupportedSamplings())
	}
}

// TestRegistryOpenClip pins the registry play path: the same clip that
// probes also opens and plays. Probe answers mp4/<codec> through the
// ffmpeg demuxer; the player reports the ffmpeg backend ("ffmpeg"
// container, real codec name) since it decodes natively now.
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
	if p.Info().Container != "ffmpeg" || p.Info().Codec == "" {
		t.Fatalf("info = %q/%q, want ffmpeg/<codec> (probe = %q/%q)", p.Info().Container, p.Info().Codec, container, codec)
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

// TestRegistryUnsupported pins readable failure: junk shells fail
// readably in the bad-clip bucket (ffmpeg owns the box now, so the open
// carries ErrBadClip), never a panic and never an empty error.
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
	if !errors.Is(err, ErrBadClip) {
		t.Fatalf("junk err = %v, want %v", err, ErrBadClip)
	}
	if got := Classify(err).Kind; got != KindBadClip {
		t.Fatalf("junk kind = %q, want %q", got, KindBadClip)
	}
	if _, _, err := ProbeFile(filepath.Join(dir, "does-not-exist.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing probe err = %v, want not-exist", err)
	}
}

// TestNoHardcodedNames is the VR9熔断 lock: the core flow (player.go)
// never switches on a container/codec/sampling name literal. Package
// import paths (video/ffmpeg) are the wiring the backend owns; the
// player only handles the names the demuxer hands back.
// fault.go triage labels are outside this gate on purpose.
func TestNoHardcodedNames(t *testing.T) {
	// Only string literals count: branch/switch/compare operands that
	// name a format. Import paths are backend wiring, not flow logic.
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
