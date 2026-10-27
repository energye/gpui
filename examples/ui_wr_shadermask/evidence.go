package main

// Example-local evidence helpers for ui_wr_shadermask: snapshot pixel probes,
// a bitwise Golden, and a live-window grab compared with the snapshot. Copied
// shape from ui_render_base_maskops (example-local, not a library package).

import (
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type rect struct{ x, y, w, h float64 }

func readPixel(img image.Image, px, py int) (r, g, b float64, valid bool) {
	if img == nil || px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

func nearC(r, g, b, wr, wg, wb, tol float64) bool {
	return math.Abs(r-wr) <= tol && math.Abs(g-wg) <= tol && math.Abs(b-wb) <= tol
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

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_shadermask.png")
	base := filepath.Join(snapDir, "showcase_shadermask_base.png")
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

// screenCheckResult is the live-window vs snapshot verdict. Attempted is false
// when -screencheck=false in manual mode; gate mode always runs it.
type screenCheckResult struct {
	Attempted bool
	OK        bool
	DiffPct   float64
	Compared  int
	GrabPath  string
	Detail    string
}

// grabClientArea captures the live X11 client area of the window with the given
// title. The snapshot is a synthetic repaint at Close, so the real window must
// be grabbed while it is still on screen.
func grabClientArea(title, outPath string) (w, h int, err error) {
	disp := os.Getenv("DISPLAY")
	if disp == "" {
		return 0, 0, fmt.Errorf("DISPLAY is empty")
	}
	info := exec.Command("xwininfo", "-name", title)
	info.Env = append(os.Environ(), "LC_ALL=C") // stable field names
	raw, err := info.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("xwininfo -name %q: %w", title, err)
	}
	var x, y, gotW, gotH int
	for _, ln := range strings.Split(string(raw), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(ln), ":")
		if !ok {
			continue
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(v))
		if convErr != nil {
			continue
		}
		switch k {
		case "Absolute upper-left X":
			x = n
		case "Absolute upper-left Y":
			y = n
		case "Width":
			gotW = n
		case "Height":
			gotH = n
		}
	}
	if gotW <= 0 || gotH <= 0 {
		return 0, 0, fmt.Errorf("xwininfo: no geometry for %q", title)
	}
	grab := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "x11grab", "-draw_mouse", "0",
		"-video_size", fmt.Sprintf("%dx%d", gotW, gotH),
		"-i", fmt.Sprintf("%s+%d,%d", disp, x, y),
		"-frames:v", "1", outPath)
	if out, grabErr := grab.CombinedOutput(); grabErr != nil {
		return 0, 0, fmt.Errorf("ffmpeg x11grab: %w: %s", grabErr, strings.TrimSpace(string(out)))
	}
	return gotW, gotH, nil
}

// compareScreenToSnapshot compares the grabbed client area against the snapshot
// over rects that exclude the live HUD (its counters change every frame).
func compareScreenToSnapshot(grabPath, snapPath string, rects []rect, tol int) (diffPct float64, compared int, err error) {
	grab, snap := loadImage(grabPath), loadImage(snapPath)
	if grab == nil || snap == nil {
		return 0, 0, fmt.Errorf("decode failed (grab=%v snap=%v)", grab != nil, snap != nil)
	}
	if grab.Bounds() != snap.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch grab=%v snap=%v (window resized?)", grab.Bounds(), snap.Bounds())
	}
	dpr := float64(snap.Bounds().Dx()) / winW
	var diff int
	for _, r := range rects {
		x0, y0 := int(r.x*dpr), int(r.y*dpr)
		x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
		for py := y0; py < y1 && py < snap.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < snap.Bounds().Dx(); px++ {
				gr, gg, gb, _ := grab.At(px, py).RGBA()
				sr, sg, sb, _ := snap.At(px, py).RGBA()
				compared++
				if channelDelta(gr, sr) > tol || channelDelta(gg, sg) > tol || channelDelta(gb, sb) > tol {
					diff++
				}
			}
		}
	}
	if compared == 0 {
		return 0, 0, fmt.Errorf("empty screen mask")
	}
	return float64(diff) * 100 / float64(compared), compared, nil
}

func channelDelta(a, b uint32) int {
	d := int(a>>8) - int(b>>8)
	if d < 0 {
		d = -d
	}
	return d
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
