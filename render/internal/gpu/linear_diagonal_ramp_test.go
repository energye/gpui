//go:build !nogpu

package gpu

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// TestLinearDiagonalRampMatchesColorAt pins the C1 terminus cut 2: a
// diagonal linear gradient over an AA rect now routes through the GPU
// ramp×mask pipeline (brush dispatch reorder); its output must match the
// exact ColorAt oracle within ramp quantization (±2/255) at interior points
// and stay background outside the rect.
func TestLinearDiagonalRampMatchesColorAt(t *testing.T) {
	shared := NewGPUShared()
	shared.mu.Lock()
	err := shared.ensureGPU()
	shared.mu.Unlock()
	if err != nil || !shared.gpuReady {
		t.Skipf("no GPU available: %v", err)
	}
	rc := &GPURenderContext{shared: shared}

	const W, H = 200, 140
	data := make([]byte, W*H*4)
	target := render.GPURenderTarget{Width: W, Height: H, Data: data, Stride: W * 4}

	// Diagonal: (20,20) → (180,120), Pad extend, over rect (40,30)-(160,110).
	brush := render.NewLinearGradientBrush(20, 20, 180, 120).
		AddColorStop(0, render.RGBA{R: 1, A: 1}).
		AddColorStop(0.5, render.RGBA{G: 1, A: 1}).
		AddColorStop(1, render.RGBA{B: 1, A: 1})
	paint := &render.Paint{}
	path := render.NewPath()
	path.MoveTo(40, 30)
	path.LineTo(160, 30)
	path.LineTo(160, 110)
	path.LineTo(40, 110)
	path.Close()

	if err := rc.fillLinearGradientFieldMasked(target, path, paint, brush); err != nil {
		t.Fatalf("ramp fill: %v", err)
	}
	// Flush pending GPU ops into the Data-backed target (readback contract),
	// mirroring pipeline_wiring_test's rc.Flush(target).
	if err := rc.Flush(target); err != nil {
		t.Fatalf("flush: %v", err)
	}
	img := imageFromData(target.Data, W, H)

	at := func(x, y int) (r, g, b uint8) {
		r32, g32, b32, _ := img.At(x, y).RGBA()
		return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8)
	}
	check := func(x, y int) {
		r, g, b := at(x, y)
		c := brush.ColorAt(float64(x)+0.5, float64(y)+0.5)
		want := [3]uint8{uint8(clamp255(c.R * c.A * 255)), uint8(clamp255(c.G * c.A * 255)), uint8(clamp255(c.B * c.A * 255))}
		d := math.Abs(float64(r)-float64(want[0]))
		if dg := math.Abs(float64(g) - float64(want[1])); dg > d {
			d = dg
		}
		if db := math.Abs(float64(b) - float64(want[2])); db > d {
			d = db
		}
		if d > 2 {
			t.Errorf("(%d,%d): ramp=(%d,%d,%d) oracle=(%d,%d,%d) maxd=%.0f", x, y, r, g, b, want[0], want[1], want[2], d)
		}
	}
	// Interior points across the diagonal span.
	check(60, 50)
	check(100, 70)
	check(140, 90)
	check(80, 60)
	check(120, 80)
	// Background outside the rect stays clear.
	if r, g, b := at(10, 10); r != 0 || g != 0 || b != 0 {
		t.Errorf("background outside rect painted: (%d,%d,%d)", r, g, b)
	}
	_ = image.Rect(0, 0, 1, 1)
}

// imageFromData wraps a premultiplied RGBA byte slice as an image for probes.
func imageFromData(data []byte, w, h int) image.Image {
	return &dataImage{data: data, w: w, h: h}
}

type dataImage struct {
	data []byte
	w, h int
}

func (d *dataImage) ColorModel() color.Model { return color.NRGBAModel }
func (d *dataImage) Bounds() image.Rectangle { return image.Rect(0, 0, d.w, d.h) }
func (d *dataImage) At(x, y int) color.Color {
	off := (y*d.w + x) * 4
	// Premultiplied RGBA in, NRGBA out (un-premultiply; probes pass A=1
	// interiors and A=0 background where premul==straight).
	a := d.data[off+3]
	if a == 0 {
		return color.NRGBA{0, 0, 0, 0}
	}
	r := uint8((uint32(d.data[off+0]) * 255) / uint32(a))
	g := uint8((uint32(d.data[off+1]) * 255) / uint32(a))
	b := uint8((uint32(d.data[off+2]) * 255) / uint32(a))
	return color.NRGBA{r, g, b, a}
}
