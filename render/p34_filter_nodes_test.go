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

// P3.4 filter-family node matrix: every ImageFilter node (plus the paint-level
// MaskFilter) gets one card-sized scene and one semantic contract. The same
// assertions run on the GPU leg and on a CPU-only leg, so a node that only
// exists on one leg fails here (semantics, not bit parity — blur alignment is
// P4.2's contract).
const (
	p34W, p34H = 288, 160
)

type p34Check struct {
	name string
	box  image.Rectangle
	want func(r, g, b, a int) bool
}

type p34Node struct {
	name   string
	draw   func(*render.Context)
	checks []p34Check
}

func p34DarkBG(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.14, A: 1})
}

func p34Clear(dc *render.Context) { dc.ClearWithColor(render.RGBA{}) }

func p34Rect(dc *render.Context, x, y, w, h float64) {
	dc.DrawRectangle(x, y, w, h)
	if err := dc.Fill(); err != nil {
		panic(err)
	}
}

func p34Nodes() []p34Node {
	return []p34Node{
		{
			name: "blur",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 1, 1, 1)
				p34Rect(dc, 40, 50, 60, 60)
				dc.ApplyBlur(8)
			},
			checks: []p34Check{
				{"core", image.Rect(60, 70, 80, 90), func(r, g, b, a int) bool { return r > 170 }},
				{"edge_falloff", image.Rect(44, 70, 52, 90), func(r, g, b, a int) bool { return r > 60 && r < 230 }},
			},
		},
		{
			name: "blurxy",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 1, 1, 1)
				p34Rect(dc, 40, 70, 160, 20)
				dc.ApplyBlurXY(0, 10)
			},
			checks: []p34Check{
				{"vertical_smear", image.Rect(110, 58, 130, 66), func(r, g, b, a int) bool { return r > 40 && r < 220 }},
				{"no_horizontal_spread", image.Rect(20, 72, 32, 88), func(r, g, b, a int) bool { return r < 40 }},
			},
		},
		{
			name: "dropshadow",
			draw: func(dc *render.Context) {
				p34Clear(dc)
				dc.SetRGBA(1, 0.2, 0.2, 1)
				p34Rect(dc, 50, 40, 70, 70)
				dc.ApplyDropShadow(20, 20, 4, render.RGBA{A: 0.8})
			},
			checks: []p34Check{
				{"content", image.Rect(70, 60, 90, 80), func(r, g, b, a int) bool { return r > 150 }},
				{"shadow", image.Rect(126, 88, 136, 102), func(r, g, b, a int) bool { return a > 80 && r < 30 }},
			},
		},
		{
			name: "colormatrix",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 0, 0, 1)
				p34Rect(dc, 50, 50, 60, 60)
				m := [20]float32{
					1, 0, 0, 0, 0,
					0, 1, 0, 0, 0,
					0, 0, 1, 0, 0,
					0, 0, 0, 1, 0,
				}
				m[0], m[2] = 0, 1   // r' = b
				m[10], m[12] = 1, 0 // b' = r
				dc.ApplyColorMatrix(m)
			},
			checks: []p34Check{
				{"swapped_to_blue", image.Rect(70, 70, 90, 90), func(r, g, b, a int) bool { return b > 150 && r < 80 }},
			},
		},
		{
			name: "grayscale",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 0, 0, 1)
				p34Rect(dc, 50, 50, 60, 60)
				dc.ApplyGrayscale()
			},
			checks: []p34Check{
				{"luma_equal_channels", image.Rect(70, 70, 90, 90), func(r, g, b, a int) bool {
					return absInt(r-g) <= 4 && absInt(g-b) <= 4 && r > 30 && r < 90
				}},
			},
		},
		{
			name: "invert",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(0, 0, 1, 1)
				p34Rect(dc, 50, 50, 60, 60)
				dc.ApplyInvert()
			},
			checks: []p34Check{
				{"blue_to_yellow", image.Rect(70, 70, 90, 90), func(r, g, b, a int) bool { return r > 200 && g > 200 && b < 70 }},
			},
		},
		{
			name: "erode",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 1, 1, 1)
				p34Rect(dc, 60, 50, 60, 60)
				dc.ApplyErode(5)
			},
			checks: []p34Check{
				{"corner_eaten", image.Rect(60, 52, 65, 58), func(r, g, b, a int) bool { return r < 60 }},
				{"body_kept", image.Rect(80, 70, 100, 90), func(r, g, b, a int) bool { return r > 170 }},
			},
		},
		{
			name: "dilate",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 1, 1, 1)
				p34Rect(dc, 80, 50, 60, 60)
				dc.ApplyDilate(5)
			},
			checks: []p34Check{
				{"grown", image.Rect(78, 52, 88, 58), func(r, g, b, a int) bool { return r > 150 }},
				{"far_clear", image.Rect(20, 20, 32, 32), func(r, g, b, a int) bool { return r < 60 }},
			},
		},
		{
			name: "offset",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 1, 1, 1)
				p34Rect(dc, 40, 60, 50, 50)
				dc.ApplyOffset(120, 0)
			},
			checks: []p34Check{
				{"moved_copy", image.Rect(160, 70, 190, 90), func(r, g, b, a int) bool { return r > 150 }},
				{"origin_vacated", image.Rect(50, 70, 70, 90), func(r, g, b, a int) bool { return r < 60 }},
			},
		},
		{
			name: "alphathreshold",
			draw: func(dc *render.Context) {
				p34Clear(dc)
				dc.SetRGBA(1, 1, 1, 0.5)
				p34Rect(dc, 40, 50, 60, 60)
				dc.SetRGBA(1, 1, 1, 0.95)
				p34Rect(dc, 160, 50, 60, 60)
				dc.ApplyAlphaThreshold(180)
			},
			checks: []p34Check{
				{"low_alpha_zeroed", image.Rect(60, 70, 80, 90), func(r, g, b, a int) bool { return a < 40 }},
				{"high_alpha_kept", image.Rect(180, 70, 200, 90), func(r, g, b, a int) bool { return a > 220 }},
			},
		},
		{
			name: "lighting",
			draw: func(dc *render.Context) {
				p34Clear(dc)
				dc.SetRGBA(1, 1, 1, 1)
				dc.DrawCircle(144, 80, 45)
				if err := dc.Fill(); err != nil {
					panic(err)
				}
				dc.ApplyLighting([3]float32{-30, -40, 60}, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 1, 2)
			},
			checks: []p34Check{
				{"lit_top_left", image.Rect(112, 62, 130, 78), func(r, g, b, a int) bool { return a > 200 && r > 40 }},
				{"darker_bottom_right", image.Rect(160, 92, 178, 108), func(r, g, b, a int) bool { return a > 0 }},
			},
		},
		{
			name: "tile",
			draw: func(dc *render.Context) {
				p34Clear(dc)
				dc.SetRGBA(1, 1, 1, 1)
				p34Rect(dc, 20, 20, 20, 20)
				dc.ApplyTile([4]float32{10, 10, 50, 50})
			},
			checks: []p34Check{
				{"cell_copy", image.Rect(104, 104, 114, 114), func(r, g, b, a int) bool { return a > 200 }},
				{"gap_between_cells", image.Rect(84, 84, 94, 94), func(r, g, b, a int) bool { return a < 40 }},
			},
		},
		{
			name: "bloom",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 1, 1, 1)
				p34Rect(dc, 120, 60, 48, 48)
				dc.ApplyBloom(render.BloomParams{Threshold: 0.6, Sigma: 6, Intensity: 0.8})
			},
			checks: []p34Check{
				{"core", image.Rect(130, 75, 150, 90), func(r, g, b, a int) bool { return r > 200 }},
				{"halo", image.Rect(106, 76, 116, 94), func(r, g, b, a int) bool { return r > 36 }},
			},
		},
		{
			name: "displacement",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 1, 1, 1)
				p34Rect(dc, 60, 60, 60, 60)
				m := image.NewRGBA(image.Rect(0, 0, p34W, p34H))
				for i := 0; i < len(m.Pix); i += 4 {
					m.Pix[i], m.Pix[i+3] = 255, 255
				}
				dc.ApplyDisplacementMap(0, 0, 40, render.ImageBufFromImage(m))
			},
			checks: []p34Check{
				{"shifted_left_up", image.Rect(46, 46, 56, 56), func(r, g, b, a int) bool { return r > 150 }},
				{"vacated_corner", image.Rect(112, 112, 124, 124), func(r, g, b, a int) bool { return r < 60 }},
			},
		},
		{
			name: "merge",
			draw: func(dc *render.Context) {
				p34Clear(dc)
				dc.SetRGBA(0.2, 1, 0.4, 1)
				p34Rect(dc, 30, 50, 90, 90)
				dc.ApplyMerge(
					[]render.ImageFilterNode{{Kind: render.ImageFilterOffset, IntOffsetX: 140}},
					[]render.ImageFilterNode{{Kind: render.ImageFilterInvert}},
				)
			},
			checks: []p34Check{
				{"offset_layer", image.Rect(175, 70, 200, 100), func(r, g, b, a int) bool { return r < 120 && g > 150 && b > 60 }},
				{"invert_layer", image.Rect(50, 65, 90, 95), func(r, g, b, a int) bool { return r > 130 && b > 130 }},
			},
		},
		{
			name: "maskfilter",
			draw: func(dc *render.Context) {
				p34DarkBG(dc)
				dc.SetRGBA(1, 0.6, 0.2, 1)
				dc.SetPaintMaskFilter(4)
				p34Rect(dc, 40, 50, 120, 100)
				dc.SetPaintMaskFilter(0)
				dc.SetRGBA(1, 0.6, 0.2, 1)
				p34Rect(dc, 190, 55, 60, 80)
			},
			checks: []p34Check{
				{"soft_edge", image.Rect(34, 90, 40, 110), func(r, g, b, a int) bool { return r > 60 && r < 230 }},
				{"hard_edge_outside", image.Rect(184, 90, 188, 110), func(r, g, b, a int) bool { return r < 60 }},
			},
		},
	}
}

func p34BoxAvg(img image.Image, box image.Rectangle) (r, g, b, a int) {
	var sr, sg, sb, sa, n int
	bounds := img.Bounds()
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
				continue
			}
			pr, pg, pb, pa := img.At(x, y).RGBA()
			sr += int(pr >> 8)
			sg += int(pg >> 8)
			sb += int(pb >> 8)
			sa += int(pa >> 8)
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0, 0
	}
	return sr / n, sg / n, sb / n, sa / n
}

func p34RunLeg(t *testing.T, n p34Node, opts ...render.ContextOption) image.Image {
	t.Helper()
	dc := render.NewContext(p34W, p34H, opts...)
	defer dc.Close()
	n.draw(dc)
	img := dc.Image()
	if img == nil {
		t.Fatalf("nil image")
	}
	return img
}

// TestP34FilterNodesCPU pins every node's contract on the CPU leg.
func TestP34FilterNodesCPU(t *testing.T) {
	for _, n := range p34Nodes() {
		n := n
		t.Run(n.name, func(t *testing.T) {
			img := p34RunLeg(t, n, render.WithCPUOnly())
			check := func(name string, box image.Rectangle, want func(int, int, int, int) bool) {
				r, g, b, a := p34BoxAvg(img, box)
				if !want(r, g, b, a) {
					t.Errorf("%s %s avg=(%d,%d,%d,%d) box=%v", n.name, name, r, g, b, a, box)
				}
			}
			for _, c := range n.checks {
				check(c.name, c.box, c.want)
			}
		})
	}
}

// TestP34FilterNodesGPU pins the same contracts on the GPU leg (skipped when no
// device is available).
func TestP34FilterNodesGPU(t *testing.T) {
	requireFilterOpsGPU(t)
	for _, n := range p34Nodes() {
		n := n
		t.Run(n.name, func(t *testing.T) {
			img := p34RunLeg(t, n)
			for _, c := range n.checks {
				r, g, b, a := p34BoxAvg(img, c.box)
				if !c.want(r, g, b, a) {
					t.Errorf("%s %s avg=(%d,%d,%d,%d) box=%v", n.name, c.name, r, g, b, a, c.box)
				}
			}
		})
	}
}

var _ = fmt.Sprintf
