// Evidence helpers for ui_wr_r19_snap (R19 · U21).
//
// The gate snapshot is a GPU readback PNG (not a window screenshot). Color
// probes are sampled in logical coordinates (DPR-aware); the Golden baseline
// is compared bit for bit over header + legend + body. R19's raw-vs-snapped
// contrast uses filled one-device-pixel bars: a snapped bar covers exactly
// one pixel column/row, while the same bar offset by half a device pixel
// splits its coverage over two pixels. Axis-aligned stroked hairlines are
// separately checked for crispness; the GPU backend also snaps those (see
// render/internal/gpu/stroke_hairline.go), so they are not used as blur bait.
package main

import (
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
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
	fg    [3]float64
	minFG int
}

type barProbe struct {
	desc     string
	group    string
	edge     float64 // logical left edge (vertical) / top edge (horizontal)
	sample   float64 // logical sample row (vertical) / column (horizontal)
	vertical bool
	snapped  bool
	line     [3]float64
	bg       [3]float64
}

type strokeProbe struct {
	desc     string
	group    string
	center   float64 // logical centerline
	sample   float64
	vertical bool
	line     [3]float64
	bg       [3]float64
}

type groupTally struct{ OK, Total int }

const kindBG = "bg"

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := imgDPR(img)
	for _, c := range checks {
		r, g, b, valid := sampleLogical(img, dpr, c.x, c.y)
		label := "fg"
		if c.kind == kindBG {
			label = "bg"
		}
		ok := valid && nearC(r, g, b, c.want, c.tol)
		fmt.Fprintf(os.Stderr,
			"ui_wr_r19_snap: pixel %-22s %-3s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			c.desc, label, c.x, c.y, r, g, b, c.want[0], c.want[1], c.want[2], ok)
		result[c.desc] = ok
	}
}

func runInkRegions(img image.Image, regions []inkRegion, result map[string]bool) {
	dpr := imgDPR(img)
	for _, reg := range regions {
		ink, fgPx, sampled := 0, 0, 0
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
				if reg.minFG > 0 && nearC(r, g, b, reg.fg, 0.05) {
					fgPx++
				}
			}
		}
		ok := img != nil && sampled > 0 && ink >= reg.minPx && (reg.minFG == 0 || fgPx >= reg.minFG)
		fmt.Fprintf(os.Stderr,
			"ui_wr_r19_snap: ink   %-22s region=(%4.0f,%4.0f %3.0fx%2.0f) ink_px=%d want>=%d fg_px=%d want>=%d ok=%v\n",
			reg.desc, reg.x, reg.y, reg.w, reg.h, ink, reg.minPx, fgPx, reg.minFG, ok)
		result[reg.desc] = ok
	}
}

// runBarProbes checks filled one-device-pixel bars. Snapped bars must have
// exactly one full-color pixel at the expected device column/row; raw bars
// must have no full-color pixel and two mixed pixels (the half-coverage split).
func runBarProbes(img image.Image, probes []barProbe, result map[string]bool, groups map[string]*groupTally, details *[]map[string]any) {
	dpr := imgDPR(img)
	for _, p := range probes {
		axis := int(math.Round(p.edge * dpr))
		sample := int(math.Round(p.sample * dpr))
		full, bgPx, mixed := 0, 0, 0
		centerFull := false
		rawPair := [2]int{}
		if p.snapped {
			start := axis - 2
			for i := start; i < start+5; i++ {
				r, g, b, ok := sampleBar(img, p.vertical, i, sample)
				if !ok {
					continue
				}
				if i == axis && nearC(r, g, b, p.line, 0.08) {
					centerFull = true
				}
				if nearC(r, g, b, p.line, 0.08) {
					full++
				}
				if nearC(r, g, b, p.bg, 0.05) {
					bgPx++
				}
			}
		} else {
			base := int(math.Floor(p.edge * dpr))
			rawPair = [2]int{base, base + 1}
			for i := base; i < base+2; i++ {
				r, g, b, ok := sampleBar(img, p.vertical, i, sample)
				if !ok {
					continue
				}
				if nearC(r, g, b, p.line, 0.08) {
					full++
				}
				if nearC(r, g, b, p.bg, 0.05) {
					bgPx++
				}
				if !nearC(r, g, b, p.line, 0.08) && !nearC(r, g, b, p.bg, 0.05) {
					mixed++
				}
			}
		}
		ok := false
		if p.snapped {
			ok = centerFull && full == 1 && bgPx >= 1
		} else {
			ok = full == 0 && mixed >= 2
		}
		result[p.desc] = ok
		if groups[p.group] == nil {
			groups[p.group] = &groupTally{}
		}
		groups[p.group].Total++
		if ok {
			groups[p.group].OK++
		}
		fmt.Fprintf(os.Stderr,
			"ui_wr_r19_snap: bar   %-22s group=%-8s snapped=%v edge_px=%d full=%d bg=%d mixed=%d center=%v raw_pair=%v ok=%v\n",
			p.desc, p.group, p.snapped, axis, full, bgPx, mixed, centerFull, rawPair, ok)
		*details = append(*details, map[string]any{
			"desc": p.desc, "group": p.group, "snapped": p.snapped,
			"edge_px": axis, "full_px": full, "background_px": bgPx,
			"mixed_px": mixed, "center_full": centerFull,
			"raw_pair": rawPair, "ok": ok,
		})
	}
}

// runStrokeProbes checks a snapped one-device-pixel stroke centerline: exactly
// one full-color pixel on the scanned axis, no two-pixel fat line.
func runStrokeProbes(img image.Image, probes []strokeProbe, result map[string]bool, groups map[string]*groupTally, details *[]map[string]any) {
	dpr := imgDPR(img)
	for _, p := range probes {
		axis := int(math.Floor(p.center * dpr))
		sample := int(math.Round(p.sample * dpr))
		full, bgPx, mixed := 0, 0, 0
		for i := axis - 2; i <= axis+2; i++ {
			r, g, b, ok := sampleBar(img, p.vertical, i, sample)
			if !ok {
				continue
			}
			if nearC(r, g, b, p.line, 0.08) {
				full++
			}
			if nearC(r, g, b, p.bg, 0.05) {
				bgPx++
			}
			if !nearC(r, g, b, p.line, 0.08) && !nearC(r, g, b, p.bg, 0.05) {
				mixed++
			}
		}
		ok := full == 1
		result[p.desc] = ok
		if groups[p.group] == nil {
			groups[p.group] = &groupTally{}
		}
		groups[p.group].Total++
		if ok {
			groups[p.group].OK++
		}
		fmt.Fprintf(os.Stderr,
			"ui_wr_r19_snap: line  %-22s group=%-8s center_px=%d full=%d bg=%d mixed=%d ok=%v\n",
			p.desc, p.group, axis, full, bgPx, mixed, ok)
		*details = append(*details, map[string]any{
			"desc": p.desc, "group": p.group, "center_px": axis,
			"full_px": full, "background_px": bgPx, "mixed_px": mixed, "ok": ok,
		})
	}
}

func sampleBar(img image.Image, vertical bool, axis, sample int) (r, g, b float64, ok bool) {
	if vertical {
		return sampleDevice(img, axis, sample)
	}
	return sampleDevice(img, sample, axis)
}

func sampleDevice(img image.Image, x, y int) (r, g, b float64, ok bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	bounds := img.Bounds()
	if x < bounds.Min.X || y < bounds.Min.Y || x >= bounds.Max.X || y >= bounds.Max.Y {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(x, y).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

func imgDPR(img image.Image) float64 {
	if img == nil || img.Bounds().Dx() <= 0 {
		return 1
	}
	return float64(img.Bounds().Dx()) / winW
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: pixel checks: decode %s: %v\n", path, err)
		return nil
	}
	return img
}

func sampleLogical(img image.Image, dpr, lx, ly float64) (r, g, b float64, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	px, py := int(lx*dpr), int(ly*dpr)
	return sampleDevice(img, px, py)
}

func nearC(r, g, b float64, want [3]float64, tol float64) bool {
	return math.Abs(r-want[0]) <= tol && math.Abs(g-want[1]) <= tol && math.Abs(b-want[2]) <= tol
}

// evaluateGolden seeds the baseline on the first run when GPUI_ACCEPT_GOLDEN=1
// and compares bitwise on every later run. A seed run is not a PASS.
func evaluateGolden(snapDir, curName, baseName string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, curName)
	base := filepath.Join(snapDir, baseName)
	if _, err := os.Stat(base); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit)\n", base)
			return 100, 0, false
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: golden: current snapshot %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: golden baseline stored: %s\n", base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: golden %s: diff=%.4f%% over %d px\n", filepath.Base(cur), diff, total)
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
	dpr := imgDPR(a)
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

// snapMathAt verifies the three R19 contracts at one DPR:
//   - SnapCoord(v, s)*s is an integer (fills land on pixel boundaries)
//   - SnapLine(v, s)*s is n+0.5 (1px stroke centers on a pixel center)
//   - SnapRect edges *s are integers (rect fills/borders keep their size)
func snapMathAt(scale float64) (coordOK, lineOK, rectOK bool) {
	coordOK, lineOK = true, true
	for _, v := range []float64{0, 0.25, 1.3, 2.6, 10.4, 20.5, 99.9} {
		c := rendering.SnapCoord(v, scale) * scale
		if math.Abs(c-math.Round(c)) > 1e-9 {
			coordOK = false
		}
		l := rendering.SnapLine(v, scale) * scale
		frac := l - math.Floor(l)
		if math.Abs(frac-0.5) > 1e-9 {
			lineOK = false
		}
	}
	x, y, w, h := rendering.SnapRect(1.3, 2.6, 10.4, 5.2, scale)
	rectOK = true
	for _, e := range []float64{x * scale, y * scale, (x + w) * scale, (y + h) * scale} {
		if math.Abs(e-math.Round(e)) > 1e-9 {
			rectOK = false
		}
	}
	return coordOK, lineOK, rectOK
}

type dprProbeResult struct {
	Scale        float64 `json:"scale"`
	Backend      string  `json:"backend"`
	SnappedOK    bool    `json:"snapped_ok"`
	RawOK        bool    `json:"raw_ok"`
	HairlineOK   bool    `json:"hairline_ok"`
	SnappedFull  int     `json:"snapped_full_px"`
	RawMixed     int     `json:"raw_mixed_px"`
	HairlineFull int     `json:"hairline_full_px"`
	SnappedAxis  int     `json:"snapped_col_or_row"`
	RawPair      [2]int  `json:"raw_pair"`
	HairlineAxis int     `json:"hairline_col_or_row"`
}

// runDPRProbes renders the same snapped/raw/hairline geometry offscreen at
// DPR 1/1.5/2. This is an honest engine cross-check, not a substitute for the
// real X11 window (the host panel is DPR 1 on this machine).
func runDPRProbes() ([]dprProbeResult, bool) {
	results := make([]dprProbeResult, 0, 3)
	allOK := true
	for _, scale := range []float64{1, 1.5, 2} {
		r := runDPRProbe(scale)
		results = append(results, r)
		if !r.SnappedOK || !r.RawOK || !r.HairlineOK {
			allOK = false
		}
	}
	return results, allOK
}

func runDPRProbe(scale float64) dprProbeResult {
	const W, H = 200, 120
	res := dprProbeResult{Scale: scale, Backend: "software"}
	if render.Accelerator() != nil {
		res.Backend = "gpu"
	}
	bg := [3]float64{0.10, 0.11, 0.13}
	line := [3]float64{0.30, 0.90, 0.60}
	dc := render.NewContext(W, H, render.WithDeviceScale(scale))
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.RGBA{R: bg[0], G: bg[1], B: bg[2], A: 1})

	// Filled one-device-pixel bars: snapped grid vs raw +0.5 device px.
	snapX := rendering.SnapCoord(20.4, scale)
	rawX := 70 + 0.5/scale
	snapY := rendering.SnapCoord(100.4, scale)
	rawY := 110 + 0.5/scale
	dc.SetRGBA(line[0], line[1], line[2], 1)
	dc.DrawRectangle(snapX, 8, 1/scale, 104)
	dc.DrawRectangle(rawX, 8, 1/scale, 104)
	dc.DrawRectangle(8, snapY, 184, 1/scale)
	dc.DrawRectangle(8, rawY, 184, 1/scale)
	_ = dc.Fill()

	// Hairline strokes: snapped centerline, one full device pixel.
	hairX := rendering.SnapLine(120.4, scale)
	hairY := rendering.SnapLine(90.4, scale)
	dc.SetRGBA(line[0], line[1], line[2], 1)
	dc.SetLineWidth(1 / scale)
	dc.MoveTo(hairX, 8)
	dc.LineTo(hairX, 112)
	dc.MoveTo(8, hairY)
	dc.LineTo(192, hairY)
	_ = dc.Stroke()

	img := dc.Image()
	if img == nil {
		return res
	}
	// This context is W x H logical with device scale `scale`; do not derive
	// DPR from the window width here.
	dpr := scale
	sampleDev := int(60 * dpr)
	snapAxis := int(math.Round(snapX * dpr))
	rawBase := int(math.Floor(rawX * dpr))
	hairAxis := int(math.Floor(hairX * dpr))
	res.SnappedAxis = snapAxis
	res.RawPair = [2]int{rawBase, rawBase + 1}
	res.HairlineAxis = hairAxis

	sf, _, _ := countDeviceBar(img, true, snapAxis-2, 5, sampleDev, line, bg)
	res.SnappedFull = sf
	res.SnappedOK = sf == 1 && nearDevice(img, true, snapAxis, sampleDev, line)

	rf, rb, rm := countDeviceBar(img, true, rawBase, 2, sampleDev, line, bg)
	_ = rb
	res.RawMixed = rm
	res.RawOK = rf == 0 && rm >= 2

	hf, _, _ := countDeviceBar(img, true, hairAxis-2, 5, sampleDev, line, bg)
	res.HairlineFull = hf
	res.HairlineOK = hf == 1

	// Horizontal axis must agree with the vertical result; this catches a
	// Y-only snapping regression that the vertical probes would miss.
	sampleXDev := int(60 * dpr)
	hSnapAxis := int(math.Round(snapY * dpr))
	hRawBase := int(math.Floor(rawY * dpr))
	hHairAxis := int(math.Floor(hairY * dpr))
	hsf, _, _ := countDeviceBar(img, false, hSnapAxis-2, 5, sampleXDev, line, bg)
	hrf, _, hrm := countDeviceBar(img, false, hRawBase, 2, sampleXDev, line, bg)
	hhf, _, _ := countDeviceBar(img, false, hHairAxis-2, 5, sampleXDev, line, bg)
	res.SnappedOK = res.SnappedOK && hsf == 1 && nearDevice(img, false, hSnapAxis, sampleXDev, line)
	res.RawOK = res.RawOK && hrf == 0 && hrm >= 2
	res.HairlineOK = res.HairlineOK && hhf == 1
	return res
}

func countDeviceBar(img image.Image, vertical bool, axisStart, count, sample int, line, bg [3]float64) (full, bgPx, mixed int) {
	for i := axisStart; i < axisStart+count; i++ {
		r, g, b, ok := sampleBar(img, vertical, i, sample)
		if !ok {
			continue
		}
		if nearC(r, g, b, line, 0.08) {
			full++
		}
		if nearC(r, g, b, bg, 0.05) {
			bgPx++
		}
		if !nearC(r, g, b, line, 0.08) && !nearC(r, g, b, bg, 0.05) {
			mixed++
		}
	}
	return full, bgPx, mixed
}

func nearDevice(img image.Image, vertical bool, axis, sample int, want [3]float64) bool {
	r, g, b, ok := sampleBar(img, vertical, axis, sample)
	return ok && nearC(r, g, b, want, 0.08)
}
