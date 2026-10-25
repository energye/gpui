// Evidence helpers for ui_wr_c1_boundary_nest (C1 · U21).
//
// Example-local, same rules as the other ui_wr_* windows: the gate snapshot is
// a GPU readback PNG (not a window screenshot), color probes are sampled in
// logical coordinates (DPR-aware), and the Golden baseline is compared bit for
// bit over the static mask (header + legend + body). The HUD band is excluded
// because its counters refresh during the run.
package main

import (
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
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

const snapshotPNG = "showcase_c1_boundary_nest.png"

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		r, g, b, valid := sampleLogical(img, dpr, c.x, c.y)
		label := "fg"
		if c.kind == kindBG {
			label = "bg"
		}
		ok := valid && nearC(r, g, b, c.want, c.tol)
		fmt.Fprintf(os.Stderr,
			"ui_wr_c1_boundary_nest: pixel %-18s %-3s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			c.desc, label, c.x, c.y, r, g, b, c.want[0], c.want[1], c.want[2], ok)
		result[c.desc] = ok
	}
}

// runInkRegions counts pixels that differ from the panel background. A single
// text pixel probe is brittle (glyph AA may miss the sampled point); counting
// ink over a small region proves the label actually rendered.
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
			"ui_wr_c1_boundary_nest: ink   %-18s region=(%4.0f,%4.0f %3.0fx%2.0f) ink_px=%d want>=%d ok=%v\n",
			reg.desc, reg.x, reg.y, reg.w, reg.h, ink, reg.minPx, ok)
		result[reg.desc] = ok
	}
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: pixel checks: decode %s: %v\n", path, err)
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

// evaluateGolden seeds the baseline on the first run when GPUI_ACCEPT_GOLDEN=1
// and compares bitwise on every later run. A seed run is not a PASS: the caller
// must run a second time and require 0.0000%.
func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, snapshotPNG)
	base := filepath.Join(snapDir, "showcase_c1_boundary_nest_base.png")
	if _, err := os.Stat(base); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit)\n", base)
			return 100, 0, false
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: golden: current snapshot %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: golden baseline stored: %s\n", base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: golden %s: diff=%.4f%% over %d px\n", filepath.Base(cur), diff, total)
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
