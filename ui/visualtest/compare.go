package visualtest

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// CompareOptions controls baseline matching tolerance.
type CompareOptions struct {
	// MaxChannelDelta is the max |a-b| per 8-bit channel for a pixel to match.
	// Default 2 (anti-alias softness).
	MaxChannelDelta uint8
	// MaxDiffRatio is the max fraction of mismatched pixels allowed (0..1).
	// Default 0.002 (0.2%). Set 0 for "any mismatch fails after channel delta".
	MaxDiffRatio float64
}

// DefaultCompare is the standard track-2 tolerance.
var DefaultCompare = CompareOptions{
	MaxChannelDelta: 2,
	MaxDiffRatio:    0.002,
}

// packageDir returns the directory of this source file (ui/visualtest).
func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}

// BaselinePath returns testdata/visual/<id>.png.
func BaselinePath(id string) string {
	return filepath.Join(packageDir(), "testdata", "visual", id+".png")
}

// OutDir returns testdata/out for failure artifacts.
func OutDir() string {
	return filepath.Join(packageDir(), "testdata", "out")
}

// updateVisual reports whether UPDATE_VISUAL is set (non-empty, not "0").
func updateVisual() bool {
	v := os.Getenv("UPDATE_VISUAL")
	return v != "" && v != "0" && v != "false"
}

// SavePNG writes img as PNG to path, creating parent dirs.
func SavePNG(path string, img image.Image) error {
	if img == nil {
		return fmt.Errorf("nil image")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// LoadPNG loads a PNG from path.
func LoadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// DiffStats holds comparison metrics.
type DiffStats struct {
	Width, Height int
	TotalPixels   int
	DiffPixels    int
	MaxDelta      int
	DiffRatio     float64
}

// CompareImages compares actual to baseline with opts.
// Returns stats and an error if outside tolerance.
func CompareImages(baseline, actual image.Image, opts CompareOptions) (DiffStats, error) {
	var stats DiffStats
	if baseline == nil {
		return stats, fmt.Errorf("nil baseline")
	}
	if actual == nil {
		return stats, fmt.Errorf("nil actual")
	}
	bb := baseline.Bounds()
	ab := actual.Bounds()
	if bb.Dx() != ab.Dx() || bb.Dy() != ab.Dy() {
		return stats, fmt.Errorf("size mismatch: baseline %dx%d actual %dx%d",
			bb.Dx(), bb.Dy(), ab.Dx(), ab.Dy())
	}
	if opts.MaxChannelDelta == 0 && opts.MaxDiffRatio == 0 {
		// keep zero delta as strict equality per channel when both unset intentionally:
		// callers should pass DefaultCompare; zero MaxChannelDelta means exact match.
	}
	w, h := bb.Dx(), bb.Dy()
	stats.Width, stats.Height = w, h
	stats.TotalPixels = w * h
	maxDelta := 0
	diffN := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			br, bg, bb8, ba := nrgbaAt(baseline, bb.Min.X+x, bb.Min.Y+y)
			ar, ag, ab8, aa := nrgbaAt(actual, ab.Min.X+x, ab.Min.Y+y)
			d := channelDelta(br, ar)
			if dg := channelDelta(bg, ag); dg > d {
				d = dg
			}
			if db := channelDelta(bb8, ab8); db > d {
				d = db
			}
			if da := channelDelta(ba, aa); da > d {
				d = da
			}
			if d > maxDelta {
				maxDelta = d
			}
			if d > int(opts.MaxChannelDelta) {
				diffN++
			}
		}
	}
	stats.DiffPixels = diffN
	stats.MaxDelta = maxDelta
	if stats.TotalPixels > 0 {
		stats.DiffRatio = float64(diffN) / float64(stats.TotalPixels)
	}
	if diffN == 0 {
		return stats, nil
	}
	if stats.DiffRatio <= opts.MaxDiffRatio {
		return stats, nil
	}
	return stats, fmt.Errorf("visual diff: %d/%d pixels (%.4f%%) exceed Δ=%d (maxΔ=%d, allow ratio=%.4f)",
		diffN, stats.TotalPixels, stats.DiffRatio*100, opts.MaxChannelDelta, maxDelta, opts.MaxDiffRatio)
}

// DiffImage highlights mismatched pixels in red on black (for debugging).
func DiffImage(baseline, actual image.Image, maxChannelDelta uint8) image.Image {
	if baseline == nil || actual == nil {
		return nil
	}
	bb := baseline.Bounds()
	ab := actual.Bounds()
	w, h := bb.Dx(), bb.Dy()
	if ab.Dx() != w || ab.Dy() != h {
		// size mismatch: solid red panel
		out := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(out, out.Bounds(), &image.Uniform{C: color.RGBA{R: 255, A: 255}}, image.Point{}, draw.Src)
		return out
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			br, bg, bb8, ba := nrgbaAt(baseline, bb.Min.X+x, bb.Min.Y+y)
			ar, ag, ab8, aa := nrgbaAt(actual, ab.Min.X+x, ab.Min.Y+y)
			d := channelDelta(br, ar)
			if dg := channelDelta(bg, ag); dg > d {
				d = dg
			}
			if db := channelDelta(bb8, ab8); db > d {
				d = db
			}
			if da := channelDelta(ba, aa); da > d {
				d = da
			}
			if d > int(maxChannelDelta) {
				out.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			} else {
				// dim baseline for context
				out.SetRGBA(x, y, color.RGBA{R: br / 3, G: bg / 3, B: bb8 / 3, A: 255})
			}
		}
	}
	return out
}

func nrgbaAt(img image.Image, x, y int) (r, g, b, a uint8) {
	rr, gg, bb, aa := img.At(x, y).RGBA()
	// 16-bit → 8-bit
	return uint8(rr >> 8), uint8(gg >> 8), uint8(bb >> 8), uint8(aa >> 8)
}

func channelDelta(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// AssertScenario compares actual against testdata/visual/<id>.png.
// With UPDATE_VISUAL=1, writes/overwrites the baseline and returns without fail.
// On mismatch, writes testdata/out/<id>_actual.png and <id>_diff.png.
func AssertScenario(t *testing.T, id string, actual image.Image, opts CompareOptions) {
	t.Helper()
	if actual == nil {
		t.Fatal("nil actual image")
	}
	basePath := BaselinePath(id)
	if updateVisual() {
		if err := SavePNG(basePath, actual); err != nil {
			t.Fatalf("UPDATE_VISUAL save %s: %v", basePath, err)
		}
		t.Logf("UPDATE_VISUAL wrote baseline %s", basePath)
		return
	}
	baseline, err := LoadPNG(basePath)
	if err != nil {
		t.Fatalf("load baseline %s: %v (set UPDATE_VISUAL=1 to create)", basePath, err)
	}
	stats, err := CompareImages(baseline, actual, opts)
	if err == nil {
		t.Logf("visual ok %s: maxΔ=%d diff=%d", id, stats.MaxDelta, stats.DiffPixels)
		return
	}
	outDir := OutDir()
	_ = os.MkdirAll(outDir, 0o755)
	actPath := filepath.Join(outDir, id+"_actual.png")
	diffPath := filepath.Join(outDir, id+"_diff.png")
	if e := SavePNG(actPath, actual); e != nil {
		t.Logf("write actual: %v", e)
	}
	if d := DiffImage(baseline, actual, opts.MaxChannelDelta); d != nil {
		if e := SavePNG(diffPath, d); e != nil {
			t.Logf("write diff: %v", e)
		}
	}
	t.Fatalf("%s: %v\n  actual: %s\n  diff:   %s\n  (UPDATE_VISUAL=1 to accept)", id, err, actPath, diffPath)
}
