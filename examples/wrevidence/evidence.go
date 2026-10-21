// Package wrevidence provides snapshot pixel probes and bitwise Golden
// comparison for the ui_render_base_* / ui_render_graphics/* real windows.
//
// Example-local only (not a ui/ library package). The ui_wr_* R/C windows
// keep their own copy because they run under the wr-close skill; this package
// centralizes the same evidence rules for the render-axis windows.
package wrevidence

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
)

// Rect is a logical-pixel rectangle used by the Golden mask.
type Rect struct{ X, Y, W, H float64 }

// PixelCheck is one GPU-snapshot color probe.
// Kind is "" (plain color), KindBG (background color), or KindNotBG (ink
// against Want, which must be the panel background for that probe).
type PixelCheck struct {
	Desc string
	X, Y float64
	Want [3]float64
	Tol  float64
	Kind string
}

const (
	KindBG    = "bg"
	KindNotBG = "ink"
)

// PairCheck compares two pixels (e.g. direct vs replay) at the same local
// coordinate in two regions.
type PairCheck struct {
	Desc   string
	AX, AY float64
	BX, BY float64
	Tol    float64
}

// Load reads a PNG. It exits the caller-friendly way on error; callers in
// examples already use os.Exit for missing GPU output.
func Load(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: open snapshot %s: %v\n", path, err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: decode snapshot %s: %v\n", path, err)
		return nil
	}
	return img
}

func rgbaAt(img image.Image, x, y int) (float64, float64, float64) {
	c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	return float64(c.R) / 255.0, float64(c.G) / 255.0, float64(c.B) / 255.0
}

func dist(a, b [3]float64) float64 {
	return math.Max(math.Abs(a[0]-b[0]), math.Max(math.Abs(a[1]-b[1]), math.Abs(a[2]-b[2])))
}

// CheckPixels runs color probes and logs one line per probe.
func CheckPixels(img image.Image, checks []PixelCheck, prefix string) map[string]bool {
	out := make(map[string]bool, len(checks))
	if img == nil {
		return out
	}
	for _, c := range checks {
		x, y := int(c.X), int(c.Y)
		got := [3]float64{}
		got[0], got[1], got[2] = rgbaAt(img, x, y)
		ok := false
		label := "fg"
		switch c.Kind {
		case KindBG:
			label = "bg"
			ok = dist(got, c.Want) <= c.Tol
		case KindNotBG:
			label = "ink"
			ok = dist(got, c.Want) > c.Tol
		default:
			ok = dist(got, c.Want) <= c.Tol
		}
		out[c.Desc] = ok
		fmt.Fprintf(os.Stderr, "%s: pixel %-22s %-3s @(%4d,%4d) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			prefix, c.Desc, label, x, y, got[0], got[1], got[2], c.Want[0], c.Want[1], c.Want[2], ok)
	}
	return out
}

// CheckPairs compares two snapshot pixels and logs one line per pair.
func CheckPairs(img image.Image, pairs []PairCheck, prefix string) (int, int) {
	ok := 0
	if img == nil {
		return 0, len(pairs)
	}
	for _, p := range pairs {
		ax, ay := int(p.AX), int(p.AY)
		bx, by := int(p.BX), int(p.BY)
		ar, ag, ab := rgbaAt(img, ax, ay)
		br, bg, bb := rgbaAt(img, bx, by)
		a := [3]float64{ar, ag, ab}
		b := [3]float64{br, bg, bb}
		good := dist(a, b) <= p.Tol
		if good {
			ok++
		}
		fmt.Fprintf(os.Stderr, "%s: pair  %-22s a=(%.3f,%.3f,%.3f) b=(%.3f,%.3f,%.3f) diff<=%.3f ok=%v\n",
			prefix, p.Desc, ar, ag, ab, br, bg, bb, p.Tol, good)
	}
	return ok, len(pairs)
}

// Golden compares the current snapshot against a zero-tolerance masked
// baseline. When the baseline is absent and GPUI_ACCEPT_GOLDEN=1, it copies
// the snapshot to the baseline and returns first=true. The caller must still
// run twice and require 0.0000% on the non-seed run.
func Golden(snapPath, basePath string, rects []Rect, prefix string) (diffPct float64, totalPx int64, first bool) {
	if snapPath == "" {
		fmt.Fprintf(os.Stderr, "%s: FAIL: empty snapshot path\n", prefix)
		return 100, 0, false
	}
	if _, err := os.Stat(basePath); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") == "1" {
			if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "%s: FAIL: mkdir golden: %v\n", prefix, err)
				return 100, 0, false
			}
			b, rerr := os.ReadFile(snapPath)
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "%s: FAIL: read snapshot for seed: %v\n", prefix, rerr)
				return 100, 0, false
			}
			if werr := os.WriteFile(basePath, b, 0o644); werr != nil {
				fmt.Fprintf(os.Stderr, "%s: FAIL: write golden: %v\n", prefix, werr)
				return 100, 0, false
			}
			fmt.Fprintf(os.Stderr, "%s: golden baseline stored: %s\n", prefix, basePath)
			return 0, 0, true
		}
		fmt.Fprintf(os.Stderr, "%s: golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed)\n", prefix, basePath)
		return 100, 0, false
	}

	snap := Load(snapPath)
	base := Load(basePath)
	if snap == nil || base == nil {
		return 100, 0, false
	}
	sb, bb := snap.Bounds(), base.Bounds()
	var diff int64
	for _, r := range rects {
		x0 := int(math.Floor(r.X))
		y0 := int(math.Floor(r.Y))
		x1 := int(math.Ceil(r.X + r.W))
		y1 := int(math.Ceil(r.Y + r.H))
		reg := image.Rect(x0, y0, x1, y1).Intersect(sb).Intersect(bb)
		if reg.Empty() {
			continue
		}
		for y := reg.Min.Y; y < reg.Max.Y; y++ {
			for x := reg.Min.X; x < reg.Max.X; x++ {
				sr, sg, sb2, sa := snap.At(x, y).RGBA()
				br, bg, bb2, ba := base.At(x, y).RGBA()
				totalPx++
				if sr != br || sg != bg || sb2 != bb2 || sa != ba {
					diff++
				}
			}
		}
	}
	if totalPx == 0 {
		fmt.Fprintf(os.Stderr, "%s: FAIL: golden mask has zero pixels\n", prefix)
		return 100, 0, false
	}
	diffPct = 100.0 * float64(diff) / float64(totalPx)
	fmt.Fprintf(os.Stderr, "%s: golden %s: diff=%.4f%% over %d px\n",
		prefix, filepath.Base(snapPath), diffPct, totalPx)
	return diffPct, totalPx, false
}
