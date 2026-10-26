// Package wrkit golden helpers: bitwise PNG comparison for real-window
// evidence. Golden files are seeded once from a reviewed snapshot and then
// compared with zero tolerance; a seed run is never a PASS by itself.
package wrkit

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// GoldenRect is a logical-coordinate rectangle (1200x800 client space) that
// participates in a golden comparison. Rects outside the image are clamped.
type GoldenRect struct {
	X, Y, W, H float64
}

// ShellContentRects masks the shared wrkit shell: legend (260x656 at 12,60)
// and body (904x656 at 284,60). The 48px top bar and 72px bottom HUD are
// excluded because they carry live FPS/phase text that must not enter a
// bitwise golden.
func ShellContentRects() []GoldenRect {
	return []GoldenRect{
		{X: 12, Y: 60, W: 260, H: 656},
		{X: 284, Y: 60, W: 904, H: 656},
	}
}

// ShellFullRects additionally includes the static 1200x48 top bar. Use it
// only when the title/phase chip is frozen as well.
func ShellFullRects() []GoldenRect {
	return []GoldenRect{
		{X: 0, Y: 0, W: 1200, H: 48},
		{X: 12, Y: 60, W: 260, H: 656},
		{X: 284, Y: 60, W: 904, H: 656},
	}
}

// CompareGoldenPNG compares curPath against basePath over rects in the shared
// 1200-wide logical space. Equality is exact (8-bit RGBA channels, zero
// tolerance). It returns the differing percentage and the number of compared
// pixels.
func CompareGoldenPNG(curPath, basePath string, rects []GoldenRect) (diffPct float64, totalPx int64, err error) {
	return CompareGoldenPNGW(curPath, basePath, 1200, rects)
}

// CompareGoldenPNGW is CompareGoldenPNG for a window whose logical client
// width is not 1200 (e.g. the 900x300 textinput window). Rects stay in that
// window's logical space; the device scale is derived from logicalW.
func CompareGoldenPNGW(curPath, basePath string, logicalW float64, rects []GoldenRect) (diffPct float64, totalPx int64, err error) {
	cur, err := decodePNG(curPath)
	if err != nil {
		return 0, 0, fmt.Errorf("current snapshot: %w", err)
	}
	base, err := decodePNG(basePath)
	if err != nil {
		return 0, 0, fmt.Errorf("baseline: %w", err)
	}
	if cur.Bounds() != base.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch: current %v baseline %v", cur.Bounds(), base.Bounds())
	}
	b := cur.Bounds()
	if len(rects) == 0 {
		rects = []GoldenRect{{X: 0, Y: 0, W: float64(b.Dx()), H: float64(b.Dy())}}
	}
	if logicalW <= 0 {
		logicalW = float64(b.Dx())
	}
	dpr := float64(b.Dx()) / logicalW
	if dpr <= 0 {
		dpr = 1
	}
	var diff int64
	for _, r := range rects {
		x0 := int(r.X*dpr + 0.5)
		y0 := int(r.Y*dpr + 0.5)
		x1 := int((r.X+r.W)*dpr + 0.5)
		y1 := int((r.Y+r.H)*dpr + 0.5)
		if x0 < b.Min.X {
			x0 = b.Min.X
		}
		if y0 < b.Min.Y {
			y0 = b.Min.Y
		}
		if x1 > b.Max.X {
			x1 = b.Max.X
		}
		if y1 > b.Max.Y {
			y1 = b.Max.Y
		}
		for py := y0; py < y1; py++ {
			for px := x0; px < x1; px++ {
				ar, ag, ab, aa := cur.At(px, py).RGBA()
				br, bg, bb, ba := base.At(px, py).RGBA()
				totalPx++
				if ar != br || ag != bg || ab != bb || aa != ba {
					diff++
				}
			}
		}
	}
	if totalPx == 0 {
		return 0, 0, fmt.Errorf("empty golden mask")
	}
	return float64(diff) * 100 / float64(totalPx), totalPx, nil
}

// SeedGoldenPNG copies curPath to basePath after review. It creates the parent
// directory. The caller must run again without GPUI_ACCEPT_GOLDEN and require
// a 0.0000% diff before claiming a green.
func SeedGoldenPNG(curPath, basePath string) error {
	data, err := os.ReadFile(curPath)
	if err != nil {
		return fmt.Errorf("current snapshot: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(basePath, data, 0o644)
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}
