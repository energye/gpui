//go:build !nogpu

package gpu

// Temporary probe for the residual rotate-text convergence work (会话摘要 #1):
// 「同进程无 clip CPU 渲染 vs mask 光栅」最小对照。
//
// Compares, in one process, two renderings of the EXACT same geometry the
// render_text_transform rotate45 cell draws ("Hello gg!" @ 24px, translate to
// (55,600), rotate 45°):
//
//   A) CPU render WITHOUT the cell clip (render.Context.DrawString, no ClipRect)
//   B) rasterizeTransformMask(devicePath) composited onto the same background
//      with the same color using the CPU premul source-over formula
//
// If A == B per-pixel, the mask rasterization itself is faithful and the
// residual must come from the clip or the GPU blend stage. If A != B, the
// mask rasterization diverges from the CPU fill (光栅舍入).

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// replicateOutlinePath mirrors render.Context.textOutlinePath using only
// exported render/text APIs (same extraction, HintingNone, Y-down segments).
func replicateOutlinePath(t *testing.T, face text.Face, s string) *render.Path {
	t.Helper()
	src := face.Source()
	if src == nil {
		t.Fatal("face has no source")
	}
	extractor := text.NewOutlineExtractor()
	parsed := src.Parsed()
	fontSize := face.Size()

	path := render.NewPath()
	hasContour := false
	for _, sg := range text.Shape(s, face) {
		o, err := extractor.ExtractOutline(parsed, sg.GID, fontSize)
		if err != nil || o == nil || o.IsEmpty() {
			continue
		}
		gx := 0.0 + sg.X
		for _, seg := range o.Segments {
			switch seg.Op {
			case text.OutlineOpMoveTo:
				if hasContour {
					path.Close()
				}
				path.MoveTo(gx+float64(seg.Points[0].X), float64(seg.Points[0].Y))
				hasContour = true
			case text.OutlineOpLineTo:
				path.LineTo(gx+float64(seg.Points[0].X), float64(seg.Points[0].Y))
			case text.OutlineOpQuadTo:
				path.QuadraticTo(
					gx+float64(seg.Points[0].X), float64(seg.Points[0].Y),
					gx+float64(seg.Points[1].X), float64(seg.Points[1].Y))
			case text.OutlineOpCubicTo:
				path.CubicTo(
					gx+float64(seg.Points[0].X), float64(seg.Points[0].Y),
					gx+float64(seg.Points[1].X), float64(seg.Points[1].Y),
					gx+float64(seg.Points[2].X), float64(seg.Points[2].Y))
			}
		}
	}
	if hasContour {
		path.Close()
	}
	return path
}

func TestIsolateMaskVsCPUNoClip(t *testing.T) {
	face := glyphMaskTestFont(t, 24)
	if face == nil {
		t.Skip("no font")
	}

	// rotate45 cell in render_text_transform: row2 col0
	// cx = 25, cy = 70 + 2*210 + 2*5 = 500; text origin (55, 600)
	const textOx, textOy = 55.0, 600.0

	// devicePath identical to tryGPUTransformMask's: user path →
	// Transform(CTM) → deviceMatrix (identity at scale 1).
	user := replicateOutlinePath(t, face, "Hello gg!")
	ctm := render.Translate(textOx, textOy).Multiply(render.Rotate(math.Pi / 4))
	devicePath := user.Transform(ctm)

	mask, bx, by, bw, bh, err := rasterizeTransformMask(devicePath)
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	t.Logf("mask bbox=(%d,%d) %dx%d", bx, by, bw, bh)

	// CPU reference, same geometry, NO clip.
	// Same background (0.95) and text color (0.12) as the example.
	dc := render.NewContext(900, 700)
	dc.ClearWithColor(render.RGBA{R: 0.95, G: 0.95, B: 0.95, A: 1})
	dc.SetFont(face)
	dc.SetRGB(0.12, 0.12, 0.12)
	dc.Translate(textOx, textOy)
	dc.Rotate(math.Pi / 4)
	dc.DrawString("Hello gg!", 0, 0)

	img := dc.Image()
	outDir := t.TempDir()
	if os.Getenv("ISOLATE_OUT") != "" {
		outDir = os.Getenv("ISOLATE_OUT")
	}
	_ = dc.SavePNG(filepath.Join(outDir, "cpu_noclip.png"))

	// Expected CPU pixel per mask alpha using the CPU premul source-over
	// formula (blendCoverageSolid): src premul = color*A*cov; dst premul = bg
	// (A=1); out = src + dst*(1-srcAlpha); store uint8(out*255) truncated.
	const colorA = 1.0
	diff := 0
	hist := [4]int{} // 0-31, 32-63, 64-159, >=160
	var signed []int
	maxd := 0
	for j := 0; j < bh; j++ {
		for i := 0; i < bw; i++ {
			cov := mask[j*bw+i]
			srcAlpha := colorA * float64(cov) / 255
			// premul source-over: out = src + dst*(1-srcAlpha); both gray.
			exp := 0.12*srcAlpha + 0.95*(1-srcAlpha)
			exp8 := uint8(clamp255f(exp * 255))
			gc := lumAt(img, bx+i, by+j)
			if gc == int(exp8) {
				continue
			}
			d := gc - int(exp8)
			diff++
			signed = append(signed, d)
			ad := int(math.Abs(float64(d)))
			if ad < 32 {
				hist[0]++
			} else if ad < 64 {
				hist[1]++
			} else if ad < 160 {
				hist[2]++
			} else {
				hist[3]++
			}
			if ad > maxd {
				maxd = ad
			}
		}
	}
	sum := 0
	for _, v := range signed {
		sum += v
	}
	mean := 0.0
	if len(signed) > 0 {
		mean = float64(sum) / float64(len(signed))
	}
	var variance float64
	for _, v := range signed {
		d := float64(v) - mean
		variance += d * d
	}
	std := 0.0
	if len(signed) > 1 {
		std = math.Sqrt(variance / float64(len(signed)-1))
	}
	fmt.Printf("ISOLATE mask-vs-CPU(noclip) rotate45: diff=%d <=31:%d 32-63:%d 64-159:%d >=160:%d maxd=%d mean=%.2f std=%.2f\n",
		diff, hist[0], hist[1], hist[2], hist[3], maxd, mean, std)

	// Dump the mask composited with the same formula to a PNG for
	// python cross-check (mask composition step).
	comp := render.NewContext(900, 700)
	comp.ClearWithColor(render.RGBA{R: 0.95, G: 0.95, B: 0.95, A: 1})
	for j := 0; j < bh; j++ {
		for i := 0; i < bw; i++ {
			cov := mask[j*bw+i]
			srcAlpha := colorA * float64(cov) / 255
			exp := 0.12*srcAlpha + 0.95*(1-srcAlpha)
			v := uint8(clamp255f(exp * 255))
			comp.SetPixel(bx+i, by+j, render.RGBA{R: float64(v) / 255, G: float64(v) / 255, B: float64(v) / 255, A: 1})
		}
	}
	_ = comp.SavePNG(filepath.Join(outDir, "mask_comp.png"))
	t.Logf("artifacts: %s", outDir)
}

func clamp255f(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 255 {
		return 255
	}
	return x
}

// lumAt returns the gray luminance (0-255) of an NRGBA/RGBA image pixel.
func lumAt(img image.Image, x, y int) int {
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return 0
	}
	r, g, b, _ := img.At(x, y).RGBA()
	return int((0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)))
}

var _ = os.Stdout