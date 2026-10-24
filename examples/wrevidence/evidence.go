// Package wrevidence provides snapshot pixel probes, retained-packet replay
// capture and bitwise Golden comparison for the real-window examples
// (ui_render_base_* / ui_render_graphics/* and examples/video/*).
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
	"sync/atomic"
	"time"

	"github.com/energye/gpui/ui/embedder"
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

// Retained tolerances, measured rather than guessed (VR8 + ffmpeg, 2026-10-23):
//
//	InkTol  = 5%%/255 off the window clear color. Text antialiasing never
//	        covers a whole rect, so this separates "content is here" from
//	        "background".
//	MoveTol = one 8-bit level, counted at or above the floor (a sample that
//	        moved by exactly one level is moved). A testsrc clip's
//	        frame-to-frame mean abs difference is ~2/255 per channel, so a 2%%
//	        floor would miss real movement; two replays of the *same* decoded
//	        frame differ by exactly zero, which is what makes one level a
//	        discriminator instead of noise.
const (
	RetainedInkTol  = 0.05
	RetainedMoveTol = 1.0 / 255.0
)

// RetainedGridX / RetainedGridY are the fixed sample density per rect.
const (
	retainedGridX = 40
	retainedGridY = 20
)

// RetainedPair is a pair of PNGs replayed from sealed retained packets
// (embedder.PipelineApp.RetainedSnapshot). It is the pixel proof for the
// retained path: a window present is zero-readback and opts.SnapshotPath
// repaints the LIVE tree, so only a packet replay shows what the sealed packet
// actually shipped.
type RetainedPair struct {
	a, b  image.Image
	scale float64
}

// OpenRetainedPair decodes the two replays. Either capture failing to land is
// reported as an error string (the caller's gate must not read that as "0 ink").
// winW is the window's logical width; image width / winW restores the device
// scale, so rect coordinates can stay in the layout space the app used.
func OpenRetainedPair(aPath, bPath string, aOK, bOK bool, winW int) (*RetainedPair, string) {
	if !aOK || !bOK {
		return nil, fmt.Sprintf("retained 快照缺片 a=%v b=%v", aOK, bOK)
	}
	a, eb := decodeFile(aPath)
	if eb != "" {
		return nil, eb
	}
	b, eb := decodeFile(bPath)
	if eb != "" {
		return nil, eb
	}
	if a.Bounds() != b.Bounds() {
		return nil, fmt.Sprintf("两张快照尺寸不一致 %v vs %v", a.Bounds(), b.Bounds())
	}
	if winW <= 0 || a.Bounds().Dx() <= 0 {
		return nil, fmt.Sprintf("快照宽度 %d 对不上窗口宽度 %d", a.Bounds().Dx(), winW)
	}
	return &RetainedPair{a: a, b: b,
		scale: float64(a.Bounds().Dx()) / float64(winW)}, ""
}

// RectEvidence samples a retainedGridX×retainedGridY grid over r in both
// replays and reports the fraction of samples carrying ink over clear in each
// one, plus the fraction that moved between them. Ink proves the content put
// pixels on the retained path; movement proves the layer repaints in place
// rather than freezing on one frame.
func (p *RetainedPair) RectEvidence(r Rect, clear [3]float64) (inkA, inkB, move float64) {
	var inkSamplesA, inkSamplesB, moveSamples, total float64
	for j := 0; j < retainedGridY; j++ {
		for i := 0; i < retainedGridX; i++ {
			lx := r.X + r.W*((float64(i)+0.5)/float64(retainedGridX))
			ly := r.Y + r.H*((float64(j)+0.5)/float64(retainedGridY))
			px, py := int(lx*p.scale), int(ly*p.scale)
			c1 := rgbAt(p.a, px, py)
			c2 := rgbAt(p.b, px, py)
			total++
			if spread(c1, clear) > RetainedInkTol {
				inkSamplesA++
			}
			if spread(c2, clear) > RetainedInkTol {
				inkSamplesB++
			}
			if spread(c1, c2) >= RetainedMoveTol {
				moveSamples++
			}
		}
	}
	if total == 0 {
		return 0, 0, 0
	}
	return inkSamplesA / total, inkSamplesB / total, moveSamples / total
}

func decodeFile(path string) (image.Image, string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Sprintf("打开 %s 失败: %v", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Sprintf("解码 %s 失败: %v", path, err)
	}
	return img, ""
}

func rgbAt(img image.Image, x, y int) [3]float64 {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return [3]float64{}
	}
	x = min(max(x, b.Min.X), b.Max.X-1)
	y = min(max(y, b.Min.Y), b.Max.Y-1)
	c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	return [3]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255}
}

// spread is the worst per-channel distance between two colors, matching this
// package's tolerance convention (max over channels, not euclidean).
func spread(a, b [3]float64) float64 {
	return math.Max(math.Abs(a[0]-b[0]), math.Max(math.Abs(a[1]-b[1]), math.Abs(a[2]-b[2])))
}

// RetainedCapture fires one retained-packet replay (embedder.
// PipelineApp.RetainedSnapshot) and remembers whether it produced a PNG.
//
// The replay is a raster job that the UI thread has to submit, so start runs it
// on its own goroutine: a caller that blocks the UI thread also blocks the
// frame carrying its own request.
type RetainedCapture struct {
	busy atomic.Bool
	ok   atomic.Bool
}

// Start fires the replay unless one is already in flight and reports whether it
// fired (a full composite plus a PNG write is not worth stacking).
func (c *RetainedCapture) Start(app *embedder.PipelineApp, path string) bool {
	if !c.busy.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		c.ok.Store(app.RetainedSnapshot(path))
		c.busy.Store(false)
	}()
	return true
}

// Wait blocks until this capture is no longer in flight, bounded by d. Close
// uses it: the replay touches the window's render context, which is gone after
// the window closes. Pass a bound longer than embedder.RetainedSnapshot's
// internal 3s submit bound — a shorter wait returns while the replay is still
// queued and the window can be torn down under it. The video windows use 4s.
func (c *RetainedCapture) Wait(d time.Duration) {
	deadline := time.Now().Add(d)
	for c.busy.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

// OK reports whether a replay landed and wrote its PNG.
func (c *RetainedCapture) OK() bool { return c.ok.Load() }
