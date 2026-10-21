package main

// Shared R5 evidence helpers: snapshot pixel probes and a bitwise Golden.
// Example-local only (not a ui/ library package).

import (
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
)

type rect struct{ x, y, w, h float64 }

const (
	kindBG    = "bg"
	kindNotBG = "ink"
)

type pixelCheck struct {
	desc string
	x, y float64
	want [3]float64
	tol  float64
	kind string
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		r, g, b, valid := sampleLogical(img, dpr, c.x, c.y)
		kind := "fg"
		ok := false
		switch c.kind {
		case kindBG:
			kind = "bg"
			ok = valid && nearC(r, g, b, c.want, c.tol)
		case kindNotBG:
			kind = "ink"
			ok = valid && !nearC(r, g, b, [3]float64{0.10, 0.11, 0.13}, c.tol)
		default:
			ok = valid && nearC(r, g, b, c.want, c.tol)
		}
		fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: pixel %-16s %-4s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			c.desc, kind, c.x, c.y, r, g, b, c.want[0], c.want[1], c.want[2], ok)
		result[c.desc] = ok
	}
}

type pairCheck struct {
	desc   string
	ax, ay float64
	bx, by float64
	tol    float64
}

// runPairChecks compares two logical sample points (direct vs replay) and
// reports whether their RGB values agree within tol. This is the R5 core
// evidence: replay must be bit-equivalent in color to direct paint.
func runPairChecks(img image.Image, checks []pairCheck) (int, map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	result := make(map[string]bool, len(checks))
	okCount := 0
	for _, c := range checks {
		ar, ag, ab, av := sampleLogical(img, dpr, c.ax, c.ay)
		br, bg, bb, bv := sampleLogical(img, dpr, c.bx, c.by)
		ok := av && bv && nearC(ar, ag, ab, [3]float64{br, bg, bb}, c.tol)
		if ok {
			okCount++
		}
		fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: pair  %-16s direct=(%.3f,%.3f,%.3f) replay=(%.3f,%.3f,%.3f) diff<=%.3f ok=%v\n",
			c.desc, ar, ag, ab, br, bg, bb, c.tol, ok)
		result[c.desc] = ok
	}
	return okCount, result
}

func loadImage(path string) image.Image {
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

func sampleLogical(img image.Image, dpr, lx, ly float64) (r, g, b float64, valid bool) {
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

func nearC(r, g, b float64, want [3]float64, tol float64) bool {
	return math.Abs(r-want[0]) <= tol && math.Abs(g-want[1]) <= tol && math.Abs(b-want[2]) <= tol
}

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_r18_savelayer.png")
	base := filepath.Join(snapDir, "showcase_r18_savelayer_base.png")
	if _, err := os.Stat(base); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			fmt.Fprintf(os.Stderr, "golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit)\n", base)
			return 100, 0, false
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden: current snapshot %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "golden baseline stored: %s\n", base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "golden %s: diff=%.4f%% over %d px\n", filepath.Base(cur), diff, total)
	return diff, total, false
}

func comparePNG(basePath, curPath string, rects []rect) (pct float64, total int64, err error) {
	a, b := loadImage(basePath), loadImage(curPath)
	if a == nil || b == nil {
		return 0, 0, fmt.Errorf("decode failed")
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v", a.Bounds(), b.Bounds())
	}
	dpr := float64(a.Bounds().Dx()) / winW
	var diff int64
	for _, r := range rects {
		x0, y0 := int(r.x*dpr), int(r.y*dpr)
		x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
		for py := y0; py < y1 && py < a.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < a.Bounds().Dx(); px++ {
				ar, ag, ab, aa := a.At(px, py).RGBA()
				br, bg, bb, ba := b.At(px, py).RGBA()
				total++
				if ar != br || ag != bg || ab != bb || aa != ba {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("empty golden mask")
	}
	return float64(diff) * 100 / float64(total), total, nil
}
