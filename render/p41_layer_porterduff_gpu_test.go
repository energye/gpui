//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// p41Pixel returns the raw RGBA at (x,y) from a Context snapshot.
func p41Pixel(img image.Image, x, y int) (int, int, int, int) {
	r, g, b, a := img.At(x, y).RGBA()
	return int(r >> 8), int(g >> 8), int(b >> 8), int(a >> 8)
}

// TestP41LayerPlusGPUComposites is the engine-hole #3 red test: a Porter-Duff
// non-normal layer whose content lives on the GPU RT must composite into the
// parent (Skia saveLayer/restore draws the layer image with the paint blend).
// Before the fix the windowed present dropped the layer content (B card showed
// the bare blue backdrop); this test pins the same composition headlessly.
func TestP41LayerPlusGPUComposites(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	// Draw the base through the GPU queue (ClearWithColor only writes the CPU
	// pixmap; a GPU composite would then blend against an empty target).
	dc.SetRGBA(0, 0, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	if err := dc.Fill(); err != nil {
		t.Fatalf("base fill: %v", err)
	}
	dc.PushLayer(render.BlendPlus, 1)
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(16, 16, 32, 32)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	dc.PopLayer()
	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	r, g, b, a := p41Pixel(img, 32, 32)
	// plus: src (255,0,0,255) + dst (0,0,255,255) clamped.
	if r < 250 || g != 0 || b < 250 || a != 255 {
		t.Fatalf("layer plus composite = %d,%d,%d,%d, want ~255,0,255,255", r, g, b, a)
	}
	outR, outG, outB, _ := p41Pixel(img, 2, 2)
	if outR != 0 || outG != 0 || outB != 255 {
		t.Fatalf("outside = %d,%d,%d, want 0,0,255", outR, outG, outB)
	}
}

// TestP41LayerPlusMeshVertices drives the same composition through the public
// mesh path the window uses (paint blend -> withMeshBlend layer wrapper).
func TestP41LayerPlusMeshVertices(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.SetRGBA(0, 0, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	if err := dc.Fill(); err != nil {
		t.Fatalf("base fill: %v", err)
	}
	pts := []render.Point{{X: 8, Y: 8}, {X: 56, Y: 8}, {X: 32, Y: 56}}
	cols := []render.RGBA{{R: 1, A: 1}, {G: 1, A: 1}, {B: 1, A: 1}}
	dc.SetRGBA(1, 1, 1, 1)
	dc.SetBlendMode(render.BlendPlus)
	dc.DrawVertices(pts, cols, render.VertexModeTriangles)
	dc.SetBlendMode(render.BlendNormal)
	img := dc.Image()
	r, g, b, a := p41Pixel(img, 32, 24)
	if a == 0 {
		t.Fatalf("alpha 0 at centroid (mesh missing)")
	}
	// plus over blue: blue channel saturates at 255, red/green gain the mesh.
	if r == 0 && g == 0 {
		t.Fatalf("centroid = %d,%d,%d,%d, want plus composite over blue", r, g, b, a)
	}
}

// TestP41LayerPorterDuffFamily pins the full fixed-function Porter-Duff family
// on the layer composite path: the GPU queued blend blit must match the CPU
// premul reference for every mode (Plus/Xor/SrcIn/DstIn/SrcOut/Clear/...).
func outsideUnchanged(mode render.BlendMode) bool {
	switch mode {
	case render.BlendPlus, render.BlendXor, render.BlendDestinationOver, render.BlendSourceAtop:
		return true
	}
	return false
}

func TestP41LayerPorterDuffFamily(t *testing.T) {
	requireFilterOpsGPU(t)
	modes := []render.BlendMode{
		render.BlendPlus, render.BlendXor, render.BlendSourceIn, render.BlendDestinationIn,
		render.BlendSourceOut, render.BlendDestinationOut, render.BlendClear,
		render.BlendSourceAtop, render.BlendDestinationAtop, render.BlendDestinationOver,
	}
	const w, h = 32, 32
	draw := func(dc *render.Context, mode render.BlendMode) {
		dc.SetRGBA(0, 0, 1, 1)
		dc.DrawRectangle(0, 0, w, h)
		if err := dc.Fill(); err != nil {
			t.Fatalf("base fill: %v", err)
		}
		dc.PushLayer(mode, 1)
		dc.SetRGBA(1, 0, 0, 1)
		dc.DrawRectangle(8, 8, 16, 16)
		if err := dc.Fill(); err != nil {
			t.Fatalf("layer fill: %v", err)
		}
		dc.PopLayer()
	}
	for _, mode := range modes {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			gpu := render.NewContext(w, h)
			defer gpu.Close()
			draw(gpu, mode)
			gi := gpu.Image()

			cpu := render.NewContext(w, h, render.WithCPUOnly())
			defer cpu.Close()
			draw(cpu, mode)
			ci := cpu.Image()

			pts := [][2]int{{16, 16}}
			if outsideUnchanged(mode) {
				// Destructive modes (SrcIn/DstIn/SrcOut/Clear/DstAtop) transform the
				// whole surface on the GPU (layer RT = full surface, Skia restore
				// contract) while the CPU composite is still content-bounded; that
				// pre-existing CPU/GPU divergence is registered under P4.2, so only
				// the outside-invariant modes are compared here.
				pts = append(pts, [2]int{4, 4}, [2]int{2, 28})
			}
			for _, pt := range pts {
				gr, gg, gb, ga := p41Pixel(gi, pt[0], pt[1])
				cr, cg, cb, ca := p41Pixel(ci, pt[0], pt[1])
				if absInt(gr-cr) > 1 || absInt(gg-cg) > 1 || absInt(gb-cb) > 1 || absInt(ga-ca) > 1 {
					t.Fatalf("%v at %v: gpu=%d,%d,%d,%d cpu=%d,%d,%d,%d",
						mode, pt, gr, gg, gb, ga, cr, cg, cb, ca)
				}
			}
		})
	}
}
