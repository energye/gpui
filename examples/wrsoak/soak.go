// Package wrsoak holds the shared assertion plumbing for the R15/C10 300s
// soak windows (examples/ui_wr_r15_soak, examples/ui_wr_c10_soak).
//
// Scene, gates, and thresholds stay per-window (like wrkit/wrgate split);
// only the mechanical pixel-assertion, Golden-compare, and tick helpers are
// shared so the two windows cannot drift apart. Behavior (tolerances, log
// formats, zero-tolerance Golden) matches the original per-window copies.
package wrsoak

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// ProbePoint is one pixel-assertion site: a logical window coordinate plus
// the theoretically expected color. Coordinates are resolved from the layout
// chain at runtime (never hand-computed absolutes).
type ProbePoint struct {
	X, Y float64
	Want [3]float64
	Tol  float64 // channel tolerance in 0..1 (8/255 F0, 12/255 F5)
}

// Rect is a logical-pixel rectangle (Golden mask or F6 text-density region).
type Rect struct{ X, Y, W, H float64 }

// PixelCheck is one assertion: either an exact point (Pt) or an F6 text
// region (Text + Base). Results feed EvaluateGates via scripted_ok/total.
type PixelCheck struct {
	Pt   *ProbePoint
	Text *Rect // F6: text density is a REGION property
	Base [3]float64
	Desc string
}

// LoadImage decodes a snapshot PNG (nil + stderr note on failure).
func LoadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: decode %s: %v\n", path, err)
		return nil
	}
	return img
}

// SampleLogical reads one logical coordinate (DPR-aware) from a snapshot.
func SampleLogical(img image.Image, dpr, lx, ly float64) (r, g, b float64, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	px, py := int(lx*dpr), int(ly*dpr)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

// NearC reports per-channel agreement within tol (Flutter precisionTolerance).
func NearC(r, g, b float64, want [3]float64, tol float64) bool {
	return math.Abs(r-want[0]) <= tol && math.Abs(g-want[1]) <= tol && math.Abs(b-want[2]) <= tol
}

// TextPixels counts pixels in box that differ from base (F6 text density:
// antialiased glyph pixels deviate from the panel background).
func TextPixels(img image.Image, dpr float64, box Rect, base [3]float64) int {
	if img == nil {
		return 0
	}
	x0, y0 := int(box.X*dpr), int(box.Y*dpr)
	x1, y1 := int((box.X+box.W)*dpr), int((box.Y+box.H)*dpr)
	n := 0
	for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
		for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
			r32, g32, b32, _ := img.At(px, py).RGBA()
			r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
			if math.Abs(r-base[0]) > 24.0/255 || math.Abs(g-base[1]) > 24.0/255 || math.Abs(b-base[2]) > 24.0/255 {
				n++
			}
		}
	}
	return n
}

// textPixelBudget is the minimum non-background pixel count for an F6 region
// that must contain a rendered label (an empty box scores ~0 and trips the
// gate — the label must exist).
const textPixelBudget = 120

// RunPixelChecks evaluates checks against img and records per-desc results.
// tag prefixes the stderr evidence lines (e.g. "ui_wr_r15_soak").
func RunPixelChecks(tag string, winW float64, img image.Image, checks []PixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		var ok bool
		if c.Pt != nil {
			r, g, b, valid := SampleLogical(img, dpr, c.Pt.X, c.Pt.Y)
			ok = valid && NearC(r, g, b, c.Pt.Want, c.Pt.Tol)
			fmt.Fprintf(os.Stderr, "%s: pixel %-32s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				tag, c.Desc, c.Pt.X, c.Pt.Y, r, g, b, c.Pt.Want[0], c.Pt.Want[1], c.Pt.Want[2], ok)
		} else if c.Text != nil {
			n := TextPixels(img, dpr, *c.Text, c.Base)
			ok = img != nil && n >= textPixelBudget
			fmt.Fprintf(os.Stderr, "%s: pixel %-32s text_px=%d (want >=%d) ok=%v\n",
				tag, c.Desc, n, textPixelBudget, ok)
		}
		result[c.Desc] = ok
	}
}

// EvaluateGolden compares the current snapshot against the stored baseline
// over the static mask rects (Flutter compareLists zero-tolerance semantics).
// The first run stores the baseline and reports firstRun=true (no PASS claim).
func EvaluateGolden(tag, snapDir, curName, baseName string, rects []Rect, winW float64) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, curName)
	base := filepath.Join(snapDir, baseName)
	if _, err := os.Stat(base); err != nil {
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: golden: current snapshot %s missing (%v)\n", tag, cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "%s: golden baseline stored: %s\n", tag, base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects, winW)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: golden compare: %v\n", tag, err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "%s: golden %s: diff=%.4f%% over %d px\n", tag, filepath.Base(cur), diff, total)
	return diff, total, false
}

func comparePNG(basePath, curPath string, rects []Rect, winW float64) (pct float64, total int64, err error) {
	a, b := LoadImage(basePath), LoadImage(curPath)
	if a == nil || b == nil {
		return 0, 0, fmt.Errorf("decode failed")
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v", a.Bounds(), b.Bounds())
	}
	dpr := float64(a.Bounds().Dx()) / winW
	var diff int64
	for _, r := range rects {
		x0, y0 := int(r.X*dpr), int(r.Y*dpr)
		x1, y1 := int((r.X+r.W)*dpr), int((r.Y+r.H)*dpr)
		for py := y0; py < y1 && py < a.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < a.Bounds().Dx(); px++ {
				ar, ag, ab, _ := a.At(px, py).RGBA()
				br, bg, bb, _ := b.At(px, py).RGBA()
				if ar != br || ag != bg || ab != bb {
					diff++
				}
				total++
			}
		}
	}
	if total == 0 {
		return 0, 0, nil
	}
	return float64(diff) / float64(total) * 100, total, nil
}

// FpsOf converts a mean frame interval into the steady-frame rate used by
// the persistent-tick gates (1000/interval_avg_ms).
func FpsOf(avgIntervalMs float64) float64 {
	if avgIntervalMs > 1e-6 {
		return 1000.0 / avgIntervalMs
	}
	return 0
}

// SortedKeys renders a set map deterministically (Go map order is random —
// an unsorted join would make the JSON field vary run to run).
func SortedKeys(m map[string]bool) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	out := ""
	for _, k := range ks {
		if out != "" {
			out += ","
		}
		out += k
	}
	return out
}

// Ticker adapts a closure to the scheduler ticker interface.
type Ticker struct{ On func(dt float64) }

// Tick runs the closure every tick.
func (t *Ticker) Tick(dt float64) bool {
	if t.On != nil {
		t.On(dt)
	}
	return true
}
