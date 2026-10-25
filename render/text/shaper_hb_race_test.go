package text

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// hbRaceTestSource loads one TTF source or skips.
func hbRaceTestSource(t *testing.T) *FontSource {
	t.Helper()
	for _, p := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
		filepath.Join("testdata", "test.ttf"),
	} {
		if _, err := os.Stat(p); err == nil {
			src, err := NewFontSourceFromFile(p)
			if err != nil {
				continue
			}
			return src
		}
	}
	t.Skip("no TTF font available")
	return nil
}

// TestHbShaperConcurrentSizes pins the D2 fix: HbShaper.Shape mutates the
// SHARED cached hbFont's XScale/YScale per call (per face size). Two
// goroutines shaping the same source at different sizes race on those
// fields. Run with -race: pre-fix this reports a DATA RACE (or corrupts the
// 16.12 scale so one size renders with the other's metrics); post-fix the
// shaper serializes per-font state and both sizes shape correctly.
func TestHbShaperConcurrentSizes(t *testing.T) {
	src := hbRaceTestSource(t)
	face12 := src.Face(12)
	face48 := src.Face(48)

	shaper := NewHbShaper()
	// Warm both font entries so the race exercises the shared-instance path
	// (not first-time creation).
	shaper.Shape("warm", face12)
	shaper.Shape("warm", face48)

	ref12 := shaper.Shape("Hello gpui", face12)
	ref48 := shaper.Shape("Hello gpui", face48)
	if len(ref12) == 0 || len(ref48) == 0 {
		t.Fatalf("reference shaping empty: %d/%d", len(ref12), len(ref48))
	}

	var wg sync.WaitGroup
	for _, f := range []Face{face12, face48} {
		wg.Add(1)
		go func(face Face) {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				shaper.Shape("Hello gpui", face)
			}
		}(f)
	}
	wg.Wait()

	// Functional check (race detector aside): concurrent shaping must not
	// corrupt the scale — results must stay identical to the single-threaded
	// reference.
	got12 := shaper.Shape("Hello gpui", face12)
	got48 := shaper.Shape("Hello gpui", face48)
	for i := range ref12 {
		if i >= len(got12) || got12[i] != ref12[i] {
			t.Fatalf("12px result drifted from reference at %d (scale clobbered)", i)
		}
	}
	for i := range ref48 {
		if i >= len(got48) || got48[i] != ref48[i] {
			t.Fatalf("48px result drifted from reference at %d (scale clobbered)", i)
		}
	}
}
