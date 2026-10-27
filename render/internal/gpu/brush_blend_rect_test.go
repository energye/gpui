//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

func blendProbeRect(x, y, w, h float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(x, y)
	p.LineTo(x+w, y)
	p.LineTo(x+w, y+h)
	p.LineTo(x, y+h)
	p.Close()
	return p
}

func blendProbeTriangle() *render.Path {
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(40, 0)
	p.LineTo(0, 40)
	p.Close()
	return p
}

func blendProbePaint(mode render.BlendMode) *render.Paint {
	p := render.NewPaint()
	p.SetBrush(render.SolidBrush{Color: render.RGBA{R: 1, A: 1}})
	p.BlendMode = mode
	return p
}

// TestStagedBlendUnsupported guards the GPU gate for non-Normal brush fills:
// fixed-function modes ride the staged rect blit, everything else falls back.
func TestStagedBlendUnsupported(t *testing.T) {
	rect := blendProbeRect(0, 0, 64, 64)
	tri := blendProbeTriangle()

	cases := []struct {
		name string
		mode render.BlendMode
		path *render.Path
		want bool
	}{
		{"normal_rect", render.BlendNormal, rect, false},
		{"normal_triangle", render.BlendNormal, tri, false},
		{"dstin_rect", render.BlendDestinationIn, rect, false},
		{"dstout_rect", render.BlendDestinationOut, rect, false},
		{"dstin_triangle", render.BlendDestinationIn, tri, true},
		{"multiply_rect", render.BlendMultiply, rect, true},
		{"nil_path_normal", render.BlendNormal, nil, false},
		{"nil_path_dstin", render.BlendDestinationIn, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stagedBlendUnsupported(blendProbePaint(tc.mode), tc.path)
			if got != tc.want {
				t.Fatalf("stagedBlendUnsupported(%v) = %v, want %v", tc.mode, got, tc.want)
			}
		})
	}
}

// TestFillBrushNativeRejectsNonRectFixedBlend pins the fail-closed side of the
// gate: a destination-dependent blend on non-rect geometry must take the CPU
// path rather than blend a quad that does not cover its own bounds.
func TestFillBrushNativeRejectsNonRectFixedBlend(t *testing.T) {
	rc := &GPURenderContext{}
	err := rc.fillBrushNative(render.GPURenderTarget{}, blendProbeTriangle(), blendProbePaint(render.BlendDestinationIn))
	if err != render.ErrFallbackToCPU {
		t.Fatalf("fillBrushNative(triangle, DstIn) = %v, want ErrFallbackToCPU", err)
	}
}

// TestFillBrushNativeRectFixedBlendGate checks the opposite side without a
// device: an axis-aligned rect with a fixed blend passes the gate (the real
// GPU output is covered by render/draw_shader_mask_gpu_test.go).
func TestFillBrushNativeRectFixedBlendGate(t *testing.T) {
	if stagedBlendUnsupported(blendProbePaint(render.BlendDestinationIn), blendProbeRect(0, 0, 64, 64)) {
		t.Fatal("rect + DstIn should be allowed on the staged GPU path")
	}
	if !paintSupportsGPUFixedBlend(blendProbePaint(render.BlendDestinationIn)) {
		t.Fatal("DstIn should have a fixed-function blend state")
	}
}
