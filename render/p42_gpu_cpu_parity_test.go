//go:build !nogpu

package render_test

import (
	"fmt"
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// P4.2 GPU/CPU consistency gate.
//
// The headline contract is the filter kernels: outside the 1px antialiasing
// band the GPU leg and the CPU leg must agree within p42InteriorTol. The AA
// band is the SDF coverage line (evidence/SDF/coverage-line.md): the GPU now
// samples a precomputed analytic coverage LUT instead of the old 0.75px
// smoothstep, so the band error dropped from 18 to <=6 and is bounded here by
// p42EdgeTol = 8. That 8 is the single-variable LUT floor (~5.5/255 spread at
// a fixed distance), not a remaining defect: a hard edge can never be
// bit-identical across the two legs without a second LUT axis.
const (
	p42W, p42H     = 96, 96
	p42InteriorTol = 2
	p42EdgeTol     = 8
)

type p42Scene struct {
	name string
	draw func(*render.Context)
}

func p42Scenes() []p42Scene {
	bg := func(dc *render.Context) {
		dc.ClearWithColor(render.RGBA{R: 0.06, G: 0.08, B: 0.12, A: 1})
	}
	sq := func(dc *render.Context, x, y, w, h, cr, cg, cb, ca float64) {
		dc.SetRGBA(cr, cg, cb, ca)
		dc.DrawRectangle(x, y, w, h)
		_ = dc.Fill()
	}
	white := func(dc *render.Context) { dc.SetRGBA(1, 1, 1, 1) }
	return []p42Scene{
		{"blur2", func(dc *render.Context) { bg(dc); white(dc); sq(dc, 24, 24, 48, 48, 1, 1, 1, 1); dc.ApplyBlur(2) }},
		{"blur4", func(dc *render.Context) { bg(dc); sq(dc, 24, 24, 48, 48, 1, 1, 1, 1); dc.ApplyBlur(4) }},
		{"blur8", func(dc *render.Context) { bg(dc); sq(dc, 24, 24, 48, 48, 1, 1, 1, 1); dc.ApplyBlur(8) }},
		{"blur16", func(dc *render.Context) { bg(dc); sq(dc, 24, 24, 48, 48, 1, 1, 1, 1); dc.ApplyBlur(16) }},
		{"blur24", func(dc *render.Context) { bg(dc); sq(dc, 24, 24, 48, 48, 1, 1, 1, 1); dc.ApplyBlur(24) }},
		{"blur3_5", func(dc *render.Context) { bg(dc); sq(dc, 24, 24, 48, 48, 1, 1, 1, 1); dc.ApplyBlur(3.5) }},
		{"blur12_7", func(dc *render.Context) { bg(dc); sq(dc, 24, 24, 48, 48, 1, 1, 1, 1); dc.ApplyBlur(12.7) }},
		{"blurxy_6_2", func(dc *render.Context) { bg(dc); sq(dc, 24, 32, 48, 32, 1, 1, 1, 1); dc.ApplyBlurXY(6, 2) }},
		{"shadow_colored", func(dc *render.Context) {
			bg(dc)
			sq(dc, 24, 20, 40, 40, 1, 1, 1, 1)
			dc.ApplyDropShadow(10, 10, 8, render.RGBA{R: 0.2, G: 0.4, B: 1, A: 0.7})
		}},
		{"circle_blur", func(dc *render.Context) {
			bg(dc)
			white(dc)
			dc.DrawCircle(48, 48, 24)
			_ = dc.Fill()
			dc.ApplyBlur(6)
		}},
		{"blurxy", func(dc *render.Context) { bg(dc); sq(dc, 24, 32, 48, 32, 1, 1, 1, 1); dc.ApplyBlurXY(0, 8) }},
		{"dropshadow", func(dc *render.Context) {
			bg(dc)
			sq(dc, 24, 20, 40, 40, 1, 0.3, 0.3, 1)
			dc.ApplyDropShadow(12, 12, 4, render.RGBA{A: 0.8})
		}},
		{"linear_ramp", func(dc *render.Context) {
			dc.ClearWithColor(render.RGBA{A: 1})
			g := render.NewLinearGradientBrush(8, 48, 88, 48).
				AddColorStop(0, render.RGBA{R: 1, A: 1}).
				AddColorStop(1, render.RGBA{B: 1, A: 1})
			dc.SetFillBrush(g)
			dc.DrawRectangle(8, 24, 80, 48)
			_ = dc.Fill()
		}},
		{"radial_ramp", func(dc *render.Context) {
			dc.ClearWithColor(render.RGBA{A: 1})
			g := render.NewRadialGradientBrush(48, 48, 0, 40).
				AddColorStop(0, render.RGBA{R: 1, A: 1}).
				AddColorStop(1, render.RGBA{B: 1, A: 1})
			dc.SetFillBrush(g)
			dc.DrawRectangle(8, 8, 80, 80)
			_ = dc.Fill()
		}},
		{"alpha_square", func(dc *render.Context) { bg(dc); sq(dc, 20, 20, 56, 56, 1, 1, 1, 0.5) }},
		{"invert_half_alpha", func(dc *render.Context) {
			bg(dc)
			sq(dc, 20, 20, 56, 56, 1, 0, 0, 0.5)
			dc.ApplyInvert()
		}},
		{"grayscale", func(dc *render.Context) { bg(dc); sq(dc, 20, 20, 56, 56, 1, 0.2, 0.1, 1); dc.ApplyGrayscale() }},
		{"mask_filter", func(dc *render.Context) {
			bg(dc)
			dc.SetRGBA(1, 0.6, 0.2, 1)
			dc.SetPaintMaskFilter(4)
			dc.DrawRectangle(20, 20, 56, 56)
			_ = dc.Fill()
			dc.SetPaintMaskFilter(0)
		}},
		{"bloom", func(dc *render.Context) {
			bg(dc)
			sq(dc, 32, 32, 32, 32, 1, 1, 1, 1)
			dc.ApplyBloom(render.BloomParams{Threshold: 0.6, Sigma: 5, Intensity: 0.8})
		}},
	}
}

// p42EdgeBand marks the antialiasing band: pixels within 2px of a sharp
// transition (central-difference > 12 levels in any channel) in either image.
// Blur ramps are smooth per-pixel and therefore stay "interior" — that is the
// point: the kernel parity is judged on the ramp itself, and only the coverage
// transition is excused.
func p42EdgeBand(img image.Image) []bool {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	mark := make([]bool, w*h)
	at := func(x, y int) [4]int {
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x >= w {
			x = w - 1
		}
		if y >= h {
			y = h - 1
		}
		r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return [4]int{int(r >> 8), int(g >> 8), int(bl >> 8), int(a >> 8)}
	}
	sharp := func(x, y int) bool {
		l, r := at(x-1, y), at(x+1, y)
		u, d := at(x, y-1), at(x, y+1)
		for i := 0; i < 4; i++ {
			if absInt(l[i]-r[i]) > 12 || absInt(u[i]-d[i]) > 12 {
				return true
			}
		}
		return false
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !sharp(x, y) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					mark[ny*w+nx] = true
				}
			}
		}
	}
	return mark
}

func TestP42GPUCPUFilterParity(t *testing.T) {
	requireFilterOpsGPU(t)
	for _, sc := range p42Scenes() {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			gpu := render.NewContext(p42W, p42H)
			sc.draw(gpu)
			gi := gpu.Image()
			_ = gpu.Close()

			cpu := render.NewContext(p42W, p42H, render.WithCPUOnly())
			sc.draw(cpu)
			ci := cpu.Image()
			_ = cpu.Close()

			edge := p42EdgeBand(gi)
			edgeCPU := p42EdgeBand(ci)
			interiorMax, edgeMax, edgeCount := 0, 0, 0
			b := gi.Bounds()
			for y := 0; y < b.Dy(); y++ {
				for x := 0; x < b.Dx(); x++ {
					ar, ag, ab, aa := gi.At(x, y).RGBA()
					br, bg, bb, ba := ci.At(x, y).RGBA()
					d := 0
					for _, v := range [4]int{int(ar>>8) - int(br>>8), int(ag>>8) - int(bg>>8), int(ab>>8) - int(bb>>8), int(aa>>8) - int(ba>>8)} {
						if v < 0 {
							v = -v
						}
						if v > d {
							d = v
						}
					}
					if edge[y*b.Dx()+x] || edgeCPU[y*b.Dx()+x] {
						edgeCount++
						if d > edgeMax {
							edgeMax = d
						}
						continue
					}
					if d > interiorMax {
						interiorMax = d
					}
				}
			}
			fmt.Printf("P42 %-16s interior_max=%d edge_max=%d edge_px=%d\n", sc.name, interiorMax, edgeMax, edgeCount)
			if interiorMax > p42InteriorTol {
				t.Fatalf("interior max delta %d > %d (filter kernels drifted apart)", interiorMax, p42InteriorTol)
			}
			if edgeMax > p42EdgeTol {
				t.Fatalf("edge max delta %d > %d (AA band regressed beyond the registered SDF gap)", edgeMax, p42EdgeTol)
			}
		})
	}
}
