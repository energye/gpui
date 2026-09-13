package color

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// vectorFile mirrors testdata/vr3_vectors.json. Both stimuli and
// expectations live in the file; the test only reads and compares.
type vectorFile struct {
	Cases []vectorCase `json:"cases"`
}

type vectorCase struct {
	Desc          string  `json:"desc"`
	W             int     `json:"w"`
	H             int     `json:"h"`
	Y             []uint8 `json:"y"`
	Cb            []uint8 `json:"cb"`
	Cr            []uint8 `json:"cr"`
	FullRange     bool    `json:"full_range"`
	Matrix        uint32  `json:"matrix"`
	MatrixPresent bool    `json:"matrix_present"`
	WantRGBA      []uint8 `json:"want_rgba"`
}

func loadVectors(t *testing.T) []vectorCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "vr3_vectors.json"))
	if err != nil {
		t.Fatalf("vectors missing: %v", err)
	}
	var vf vectorFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatalf("vectors bad: %v", err)
	}
	if len(vf.Cases) == 0 {
		t.Fatal("vectors empty")
	}
	return vf.Cases
}

// VR3 gate: every spec vector converts byte-exact (tolerance lives in the
// window gate against ffmpeg; the unit gate pins our own formula).
func TestVectorsExact(t *testing.T) {
	maxDiff := 0
	for _, vc := range loadVectors(t) {
		opt := Options{FullRange: vc.FullRange, Matrix: vc.Matrix, MatrixPresent: vc.MatrixPresent}
		f, err := Convert(SamplingYUV420P, vc.Y, vc.Cb, vc.Cr, vc.W, vc.H, opt)
		if err != nil {
			t.Fatalf("%s: %v", vc.Desc, err)
		}
		if len(f.Pix) != len(vc.WantRGBA) {
			t.Fatalf("%s: pix %d want %d", vc.Desc, len(f.Pix), len(vc.WantRGBA))
		}
		for i := range f.Pix {
			d := int(f.Pix[i]) - int(vc.WantRGBA[i])
			if d < 0 {
				d = -d
			}
			if d > maxDiff {
				maxDiff = d
			}
			if d != 0 {
				t.Fatalf("%s: byte %d = %d want %d", vc.Desc, i, f.Pix[i], vc.WantRGBA[i])
			}
		}
		// Alpha channel is always opaque.
		for i := 3; i < len(f.Pix); i += 4 {
			if f.Pix[i] != 255 {
				t.Fatalf("%s: alpha %d", vc.Desc, f.Pix[i])
			}
		}
	}
	t.Logf("vectors=%d max_per_channel_diff=%d", len(loadVectors(t)), maxDiff)
}

// Into with a caller buffer matches the allocating path bit for bit.
func TestConvertIntoMatchesConvert(t *testing.T) {
	for _, vc := range loadVectors(t) {
		opt := Options{FullRange: vc.FullRange, Matrix: vc.Matrix, MatrixPresent: vc.MatrixPresent}
		a, err := Convert(SamplingYUV420P, vc.Y, vc.Cb, vc.Cr, vc.W, vc.H, opt)
		if err != nil {
			t.Fatalf("%s: %v", vc.Desc, err)
		}
		dst := make([]byte, vc.W*vc.H*4)
		if err := ConvertInto(SamplingYUV420P, dst, vc.Y, vc.Cb, vc.Cr, vc.W, vc.H, opt); err != nil {
			t.Fatalf("%s into: %v", vc.Desc, err)
		}
		for i := range a.Pix {
			if dst[i] != a.Pix[i] {
				t.Fatalf("%s: into byte %d differs", vc.Desc, i)
			}
		}
	}
}

// The dispatch table is consulted: custom samplings plug in (VR9), the
// default name stays registered, unknown names fail naming themselves.
func TestRegistry(t *testing.T) {
	found := false
	for _, s := range Supported() {
		if s == SamplingYUV420P {
			found = true
		}
	}
	if !found {
		t.Fatalf("supported %v lacks %q", Supported(), SamplingYUV420P)
	}
	called := false
	Register("test-only-gray", func(dst, y, cb, cr []byte, w, h int, opt Options) error {
		called = true
		for i := 0; i < w*h; i++ {
			dst[i*4], dst[i*4+1], dst[i*4+2], dst[i*4+3] = y[i], y[i], y[i], 255
		}
		return nil
	})
	y := []uint8{10, 20, 30, 40}
	f, err := Convert("test-only-gray", y, []uint8{128}, []uint8{128}, 2, 2, Options{})
	if err != nil {
		t.Fatalf("custom sampling: %v", err)
	}
	if !called || f.Pix[0] != 10 || f.Pix[4] != 20 {
		t.Fatalf("custom converter not consulted: %+v", f.Pix)
	}
	_, err = Convert("yuv999", y, []uint8{128}, []uint8{128}, 2, 2, Options{})
	if !errors.Is(err, ErrUnsupportedSampling) {
		t.Fatalf("sampling err = %v, want ErrUnsupportedSampling", err)
	}
}

// BT.470BG shares the BT.601 taps; exotic matrices fail naming the matrix.
func TestMatrixPolicy(t *testing.T) {
	y := []uint8{180, 180, 180, 180}
	cb, cr := []uint8{100}, []uint8{160}
	a, err := Convert(SamplingYUV420P, y, cb, cr, 2, 2, Options{Matrix: MatrixSMPTE170M, MatrixPresent: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Convert(SamplingYUV420P, y, cb, cr, 2, 2, Options{Matrix: MatrixBT470BG, MatrixPresent: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatalf("470BG differs from 170M at %d", i)
		}
	}
	for _, m := range []uint32{MatrixRGB, MatrixFCC, MatrixSMPTE240M, 99} {
		_, err := Convert(SamplingYUV420P, y, cb, cr, 2, 2, Options{Matrix: m, MatrixPresent: true})
		if !errors.Is(err, ErrUnsupportedMatrix) {
			t.Fatalf("matrix %d err = %v, want ErrUnsupportedMatrix", m, err)
		}
	}
	// Absent colour description defaults to BT.601, same as explicit 170M.
	c, err := Convert(SamplingYUV420P, y, cb, cr, 2, 2, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range a.Pix {
		if a.Pix[i] != c.Pix[i] {
			t.Fatalf("default differs from 170M at %d", i)
		}
	}
}

func TestBadSizes(t *testing.T) {
	ok := func() (y, cb, cr []byte) {
		return make([]byte, 4*2), []byte{128}, []byte{128}
	}
	y, cb, cr := ok()
	if _, err := Convert(SamplingYUV420P, y, cb, cr, 3, 2, Options{}); !errors.Is(err, ErrBadSize) {
		t.Fatalf("odd width err = %v", err)
	}
	if _, err := Convert(SamplingYUV420P, y[:3], cb, cr, 2, 2, Options{}); !errors.Is(err, ErrBadPlanes) {
		t.Fatalf("short Y err = %v", err)
	}
	if err := ConvertInto(SamplingYUV420P, make([]byte, 7), y, cb, cr, 2, 2, Options{}); !errors.Is(err, ErrBadSize) {
		t.Fatalf("short dst err = %v", err)
	}
}

// Hot path reuses the caller buffer with zero fresh allocations.
func TestIntoZeroAlloc(t *testing.T) {
	w, h := 64, 64
	y := make([]byte, w*h)
	cb := make([]byte, w*h/4)
	cr := make([]byte, w*h/4)
	dst := make([]byte, w*h*4)
	for i := range y {
		y[i] = uint8(i)
	}
	for i := range cb {
		cb[i], cr[i] = 100, 160
	}
	n := testing.AllocsPerRun(20, func() {
		_ = ConvertInto(SamplingYUV420P, dst, y, cb, cr, w, h, Options{})
	})
	if n != 0 {
		t.Fatalf("ConvertInto allocs = %v, want 0", n)
	}
}

func BenchmarkConvert420Into(b *testing.B) {
	w, h := 320, 240
	y := make([]byte, w*h)
	cb := make([]byte, w*h/4)
	cr := make([]byte, w*h/4)
	dst := make([]byte, w*h*4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ConvertInto(SamplingYUV420P, dst, y, cb, cr, w, h, Options{})
	}
}

func BenchmarkConvert420Into1080p(b *testing.B) {
	w, h := 1920, 1080
	y := make([]byte, w*h)
	cb := make([]byte, w*h/4)
	cr := make([]byte, w*h/4)
	dst := make([]byte, w*h*4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ConvertInto(SamplingYUV420P, dst, y, cb, cr, w, h, Options{})
	}
}

func BenchmarkConvert420(b *testing.B) {
	w, h := 320, 240
	y := make([]byte, w*h)
	cb := make([]byte, w*h/4)
	cr := make([]byte, w*h/4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Convert(SamplingYUV420P, y, cb, cr, w, h, Options{})
	}
}
