// Package wrkit evidence helpers shared by real-window gates: zero-tolerance
// Golden comparison with baseline seeding, and semantic pixel probes sampled
// from the same snapshot.
package wrkit

import (
	"fmt"
	"os"
)

// PixelProbe is one semantic color check in logical 1200-wide client space.
// Tol is the per-channel tolerance in 8-bit steps; 0 means 3.
type PixelProbe struct {
	Name    string
	X, Y    float64
	R, G, B float64
	Tol     uint8
}

// GoldenGate compares curPath with basePath. When the baseline is absent and
// GPUI_ACCEPT_GOLDEN=1, it seeds the baseline and returns seeded=true; the
// caller must not treat that run as a PASS. All other errors are returned.
func GoldenGate(curPath, basePath string, rects []GoldenRect) (diffPct float64, totalPx int64, seeded bool, err error) {
	return GoldenGateW(curPath, basePath, 1200, rects)
}

// GoldenGateW is GoldenGate for a non-1200-wide logical window (rects use
// that window's logical coordinates).
func GoldenGateW(curPath, basePath string, logicalW float64, rects []GoldenRect) (diffPct float64, totalPx int64, seeded bool, err error) {
	if _, statErr := os.Stat(basePath); statErr != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			return 0, 0, false, fmt.Errorf("golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit)", basePath)
		}
		if err := SeedGoldenPNG(curPath, basePath); err != nil {
			return 0, 0, false, fmt.Errorf("seed golden: %w", err)
		}
		return 0, 0, true, nil
	}
	diffPct, totalPx, err = CompareGoldenPNGW(curPath, basePath, logicalW, rects)
	return diffPct, totalPx, false, err
}

// CheckPixelProbes decodes snapshot at path and checks every probe with the
// declared channel tolerance. The Golden comparison remains the exact gate;
// these probes name the visible semantics so a green cannot be only a diff=0.
func CheckPixelProbes(path string, logicalW float64, probes []PixelProbe) error {
	img, err := decodePNG(path)
	if err != nil {
		return err
	}
	b := img.Bounds()
	dpr := 1.0
	if logicalW > 0 {
		dpr = float64(b.Dx()) / logicalW
	}
	if dpr <= 0 {
		dpr = 1
	}
	for _, p := range probes {
		px := b.Min.X + int(p.X*dpr+0.5)
		py := b.Min.Y + int(p.Y*dpr+0.5)
		if px < b.Min.X || px >= b.Max.X || py < b.Min.Y || py >= b.Max.Y {
			return fmt.Errorf("%s probe outside %v", p.Name, b)
		}
		cr, cg, cb, _ := img.At(px, py).RGBA()
		gotR, gotG, gotB := uint8(cr>>8), uint8(cg>>8), uint8(cb>>8)
		wantR, wantG, wantB := uint8(p.R*255+0.5), uint8(p.G*255+0.5), uint8(p.B*255+0.5)
		tol := p.Tol
		if tol == 0 {
			tol = 3
		}
		if abs8(gotR, wantR) > int(tol) || abs8(gotG, wantG) > int(tol) || abs8(gotB, wantB) > int(tol) {
			return fmt.Errorf("%s at (%.0f,%.0f) got=(%d,%d,%d) want=(%d,%d,%d)", p.Name, p.X, p.Y, gotR, gotG, gotB, wantR, wantG, wantB)
		}
	}
	return nil
}

func abs8(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
