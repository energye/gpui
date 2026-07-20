//go:build linux && !nogpu

package main

import (
	"fmt"
	"math"
	"os"
	"sync"

	"github.com/energye/gpui/render"
)

// drawStats is overlay metrics for the HUD.
type drawStats struct {
	Frame   uint64
	FPS     float64
	Seconds float64
}

var (
	hudFontOnce sync.Once
	hudFontPath string
)

func ensureHUDFont(dc *render.Context) {
	hudFontOnce.Do(func() {
		for _, p := range []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
			"/usr/share/fonts/truetype/freefont/FreeSans.ttf",
			"/usr/share/fonts/opentype/noto/NotoSans-Regular.ttf",
		} {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				hudFontPath = p
				break
			}
		}
	})
	if hudFontPath != "" {
		_ = dc.LoadFontFace(hudFontPath, 15)
	}
}

// drawCompositeFrame is a mid-weight animation inspired by mem_anim S12
// (cards + orbs + bars + soft panels). Intentionally self-contained so this
// example does not import the full mem_anim scenario matrix.
func drawCompositeFrame(dc *render.Context, w, h int, t float64, st drawStats) {
	fw, fh := float64(w), float64(h)

	// Background gradient-ish bands
	dc.SetRGB(0.07, 0.08, 0.12)
	dc.DrawRectangle(0, 0, fw, fh)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		yy := fh * (0.05 + 0.15*float64(i))
		pulse := 0.5 + 0.5*math.Sin(t*0.8+float64(i)*0.4)
		dc.SetRGBA(0.12+0.05*pulse, 0.14, 0.22, 0.35)
		dc.DrawRectangle(0, yy, fw, fh*0.08)
		_ = dc.Fill()
	}

	// Floating orbs
	for i := 0; i < 12; i++ {
		ang := t*0.7 + float64(i)*0.52
		cx := fw*0.5 + math.Cos(ang)*fw*0.28
		cy := fh*0.42 + math.Sin(ang*1.3)*fh*0.22
		r := 14.0 + 8*math.Sin(t*1.2+float64(i))
		dc.SetRGBA(
			0.3+0.5*math.Sin(float64(i)),
			0.5+0.3*math.Cos(float64(i)*0.7),
			0.9,
			0.55,
		)
		dc.DrawCircle(cx, cy, r)
		_ = dc.Fill()
	}

	// Card stack
	for i := 0; i < 5; i++ {
		ox := 36.0 + float64(i)*18 + 10*math.Sin(t+float64(i)*0.5)
		oy := 48.0 + float64(i)*22
		cw, ch := fw*0.42, 72.0
		dc.SetRGBA(0.16, 0.18, 0.26, 0.92)
		dc.DrawRoundedRectangle(ox, oy, cw, ch, 12)
		_ = dc.Fill()
		dc.SetRGBA(0.35, 0.55, 0.95, 0.9)
		dc.DrawRoundedRectangle(ox+12, oy+16, 40, 40, 8)
		_ = dc.Fill()
		prog := 0.2 + 0.8*(0.5+0.5*math.Sin(t*1.5+float64(i)))
		dc.SetRGBA(0.25, 0.28, 0.35, 1)
		dc.DrawRoundedRectangle(ox+64, oy+32, cw-88, 10, 4)
		_ = dc.Fill()
		dc.SetRGBA(0.2, 0.85, 0.55, 1)
		dc.DrawRoundedRectangle(ox+64, oy+32, (cw-88)*prog, 10, 4)
		_ = dc.Fill()
	}

	// Right panel lattice
	baseX := fw * 0.55
	baseY := fh * 0.12
	cols, rows := 8, 6
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			u := float64(x) / float64(cols-1)
			v := float64(y) / float64(rows-1)
			px := baseX + u*fw*0.4 + 6*math.Sin(t*2+v*4)
			py := baseY + v*fh*0.55 + 6*math.Cos(t*1.7+u*3)
			sz := 6.0 + 3*math.Sin(t+u*5+v*3)
			dc.SetRGBA(0.9, 0.4+0.4*u, 0.3+0.5*v, 0.75)
			dc.DrawCircle(px, py, sz)
			_ = dc.Fill()
			if x+1 < cols {
				u2 := float64(x+1) / float64(cols-1)
				px2 := baseX + u2*fw*0.4 + 6*math.Sin(t*2+v*4)
				dc.SetRGBA(0.5, 0.6, 0.8, 0.25)
				dc.SetLineWidth(1)
				dc.MoveTo(px, py)
				dc.LineTo(px2, py)
				_ = dc.Stroke()
			}
		}
	}

	// Top HUD: fps / frame / runtime seconds
	dc.SetRGBA(0.05, 0.06, 0.1, 0.82)
	dc.DrawRoundedRectangle(12, 10, math.Min(fw-24, 420), 34, 8)
	_ = dc.Fill()
	line := fmt.Sprintf("fps=%.1f  frame=%d  t=%.1fs", st.FPS, st.Frame, st.Seconds)
	ensureHUDFont(dc)
	dc.SetRGB(0.92, 0.95, 1.0)
	dc.DrawString(line, 24, 32)

	// Bottom ticker bar
	dc.SetRGBA(0.1, 0.12, 0.18, 0.95)
	dc.DrawRectangle(0, fh-36, fw, 36)
	_ = dc.Fill()
	sweep := math.Mod(t*120, fw+80) - 40
	dc.SetRGBA(0.3, 0.7, 1.0, 0.85)
	dc.DrawRoundedRectangle(sweep, fh-28, 80, 20, 6)
	_ = dc.Fill()

	dc.SetRGBA(1, 1, 1, 0.08)
	dc.DrawRectangle(0, 0, fw, 2)
	_ = dc.Fill()
}
