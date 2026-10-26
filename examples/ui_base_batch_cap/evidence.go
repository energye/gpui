// Pixel/Golden evidence for ui_base_batch_cap.
//
// Rules match the other base windows: gate snapshots are GPU readback PNGs
// (not window captures), probes are sampled in logical coordinates, and the
// Golden compare is bitwise over the content mask (metric strip excluded).
package main

import (
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
)

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	desc string
	x, y float64
	want [3]float64
	tol  float64
	kind string
}

type inkRegion struct {
	desc  string
	x, y  float64
	w, h  float64
	bg    [3]float64
	tol   float64
	minPx int
}

const kindBG = "bg"

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		r, g, b, valid := sampleLogical(img, dpr, c.x, c.y)
		ok := valid && nearC(r, g, b, c.want, c.tol)
		fmt.Fprintf(os.Stderr,
			"ui_base_batch_cap: pixel %-18s %-3s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			c.desc, c.kind, c.x, c.y, r, g, b, c.want[0], c.want[1], c.want[2], ok)
		result[c.desc] = ok
	}
}

func runInkRegions(img image.Image, regions []inkRegion, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, reg := range regions {
		ink, sampled := 0, 0
		x0, y0 := int(reg.x*dpr), int(reg.y*dpr)
		x1, y1 := int((reg.x+reg.w)*dpr), int((reg.y+reg.h)*dpr)
		var bounds image.Rectangle
		if img != nil {
			bounds = img.Bounds()
		}
		for py := y0; py < y1; py++ {
			for px := x0; px < x1; px++ {
				if px < bounds.Min.X || py < bounds.Min.Y || px >= bounds.Max.X || py >= bounds.Max.Y {
					continue
				}
				r32, g32, b32, _ := img.At(px, py).RGBA()
				r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
				sampled++
				if !nearC(r, g, b, reg.bg, reg.tol) {
					ink++
				}
			}
		}
		ok := img != nil && sampled > 0 && ink >= reg.minPx
		fmt.Fprintf(os.Stderr,
			"ui_base_batch_cap: ink   %-18s region=(%4.0f,%4.0f %3.0fx%2.0f) ink_px=%d want>=%d ok=%v\n",
			reg.desc, reg.x, reg.y, reg.w, reg.h, ink, reg.minPx, ok)
		result[reg.desc] = ok
	}
}

func loadImage(path string) image.Image {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_base_batch_cap: pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_base_batch_cap: pixel checks: decode %s: %v\n", path, err)
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

// evaluateGolden seeds base from cur on GPUI_ACCEPT_GOLDEN=1, otherwise
// compares bitwise. A seed run is not a PASS: run again and require 0.0000%.
func evaluateGolden(cur, base string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	if cur == "" || base == "" {
		return 100, 0, false
	}
	if _, err := os.Stat(base); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			fmt.Fprintf(os.Stderr, "ui_base_batch_cap: golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed)\n", base)
			return 100, 0, false
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ui_base_batch_cap: golden: current %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err != nil {
			return 100, 0, false
		}
		fmt.Fprintf(os.Stderr, "ui_base_batch_cap: golden baseline stored: %s\n", base)
		return 0, 0, true
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_base_batch_cap: golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "ui_base_batch_cap: golden %s: diff=%.4f%% over %d px\n", base, diff, total)
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
