package text

import (
	"os"
	"path/filepath"
	"testing"
)

func sourceSansFamily(t *testing.T) (*FontFamily, []*FontSource) {
	t.Helper()
	files := []string{
		"SourceSans3-Regular.ttf",
		"SourceSans3-Semibold.ttf",
		"SourceSans3-Bold.ttf",
	}
	sources := make([]*FontSource, 0, len(files))
	for _, name := range files {
		path := filepath.Join("testdata", "source-sans", "TTF", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("Source Sans test font %s unavailable: %v", name, err)
		}
		src, err := NewFontSource(data)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		sources = append(sources, src)
	}
	fam, err := NewFontFamily(sources...)
	if err != nil {
		t.Fatalf("NewFontFamily: %v", err)
	}
	t.Cleanup(func() {
		for _, src := range sources {
			_ = src.Close()
		}
	})
	return fam, sources
}

func TestFontFamily_RealWeightSelection(t *testing.T) {
	fam, sources := sourceSansFamily(t)
	if fam.Name() != "Source Sans 3" {
		t.Fatalf("family=%q want Source Sans 3", fam.Name())
	}
	if got := sources[1].Name(); got != fam.Name() {
		t.Fatalf("semibold Name()=%q want typographic family %q", got, fam.Name())
	}

	cases := []struct {
		weight FontWeight
		full   string
	}{
		{WeightRegular, "Source Sans 3"},
		{WeightMedium, "Source Sans 3"},
		{WeightSemiBold, "Source Sans 3 Semibold"},
		{WeightBold, "Source Sans 3 Bold"},
		{WeightExtraBold, "Source Sans 3 Bold"},
	}
	for _, tc := range cases {
		face := fam.Face(14, WithWeight(tc.weight))
		if face == nil {
			t.Fatalf("weight %d: nil face", tc.weight)
		}
		if face.Weight() != tc.weight {
			t.Errorf("weight %d: Weight()=%d", tc.weight, face.Weight())
		}
		if got := face.Source().Parsed().FullName(); got != tc.full {
			t.Errorf("weight %d selected %q want %q", tc.weight, got, tc.full)
		}
		if FaceEmbolden(face) {
			t.Errorf("weight %d selected a real cut but Embolden=true", tc.weight)
		}
	}

	regular := fam.Face(14, WithWeight(WeightRegular))
	bold := fam.Face(14, WithWeight(WeightBold))
	rw, _ := Measure("Hamburgefonts", regular)
	bw, _ := Measure("Hamburgefonts", bold)
	if bw <= rw {
		t.Fatalf("real bold advance %.2f must be wider than regular %.2f", bw, rw)
	}
}

func TestFontFamily_StandaloneStaticStillUsesOutlineFallback(t *testing.T) {
	path := filepath.Join("testdata", "source-sans", "TTF", "SourceSans3-Regular.ttf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Source Sans regular unavailable: %v", err)
	}
	regular, err := NewFontSource(data)
	if err != nil {
		t.Fatalf("NewFontSource: %v", err)
	}
	t.Cleanup(func() { _ = regular.Close() })

	face := regular.Face(14, WithWeight(WeightBold))
	if face.Source() != regular {
		t.Fatalf("standalone source changed unexpectedly")
	}
	if !FaceEmbolden(face) {
		t.Fatalf("standalone static 700 must use synthetic outline embolden")
	}
	if face.Weight() != WeightBold {
		t.Fatalf("Weight()=%d want 700", face.Weight())
	}
	for _, v := range face.Variations() {
		if v.Tag == AxisWeight {
			t.Fatalf("static 700 must not add a wght variation: %v", face.Variations())
		}
	}
}

func TestNewFontFamily_RejectsMismatchAndDuplicateWeight(t *testing.T) {
	_, sources := sourceSansFamily(t)
	regularPath := filepath.Join("testdata", "source-sans", "TTF", "SourceSans3-Regular.ttf")
	regularData, err := os.ReadFile(regularPath)
	if err != nil {
		t.Skipf("Source Sans regular unavailable: %v", err)
	}
	regularCopy, err := NewFontSource(regularData)
	if err != nil {
		t.Fatalf("load Source Sans regular copy: %v", err)
	}
	t.Cleanup(func() { _ = regularCopy.Close() })
	if _, err := NewFontFamily(sources[0], regularCopy); err == nil {
		t.Fatal("duplicate weight must be rejected")
	}
	if _, err := NewFontFamily(); err == nil {
		t.Fatal("empty family must be rejected")
	}
	if _, err := NewFontFamily(nil); err == nil {
		t.Fatal("nil-only family must be rejected")
	}

	otherPath := filepath.Join("testdata", "ahem.ttf")
	data, err := os.ReadFile(otherPath)
	if err != nil {
		t.Skipf("Ahem test font unavailable: %v", err)
	}
	other, err := NewFontSource(data)
	if err != nil {
		t.Fatalf("load Ahem: %v", err)
	}
	t.Cleanup(func() { _ = other.Close() })
	if _, err := NewFontFamily(sources[0], other); err == nil {
		t.Fatal("different family names must be rejected")
	}
}

func TestLoadFontFamily_MissingBoldFallsBackToOutline(t *testing.T) {
	path := filepath.Join("testdata", "source-sans", "TTF", "SourceSans3-Regular.ttf")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("Source Sans regular unavailable: %v", err)
	}
	_, fam, _, err := LoadFontFamily(14,
		FontRef{Path: path},
		FontRef{Path: filepath.Join(t.TempDir(), "missing-bold.ttf")},
	)
	if err != nil {
		t.Fatalf("LoadFontFamily: %v", err)
	}
	if fam == nil {
		t.Fatal("expected a one-cut family")
	}
	face := fam.Face(14, WithWeight(WeightBold))
	if !FaceEmbolden(face) {
		t.Fatal("one-cut family at 700 must fall back to outline embolden")
	}
}
