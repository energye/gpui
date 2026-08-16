//go:build !nogpu

// Package gpu: kTransformedMask text semantic — whole-string alpha masks
// CPU-rasterized by the same Skia-AAA software filler the CPU Tier2 outline
// path uses, uploaded to the glyph-mask atlas and drawn as one textured quad.
// The GPU output matches the CPU rendering bit-exactly (the vector-outline
// cover pass cannot reproduce Skia's scanline trapezoid accumulation on
// densely overlapping edges, which is why rotated text used to diverge).

package gpu

import (
	"unsafe"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/raster"
	"github.com/energye/gpui/render/text"
)

// DrawGlyphMaskTransformText implements render.GPUTransformMaskTextAccelerator.
//
// devicePath is the whole string's outline in device (pixel) space — the exact
// geometry the CPU Tier2 software fill consumes. The mask is rasterized here by
// the same Skia-AAA software filler (render.SoftwareRenderer with default
// 4x-A A scanline settings), integer-aligned so the raster's pixel grid matches
// the surface grid, then cached in the glyph-mask atlas under a content
// fingerprint and queued as a single device-space quad.
func (rc *GPURenderContext) DrawGlyphMaskTransformText(
	target render.GPURenderTarget,
	face any,
	s string,
	x, y float64,
	color render.RGBA,
	matrix render.Matrix,
	deviceScale float64,
	devicePath *render.Path,
) error {
	if devicePath == nil || s == "" {
		return nil
	}
	textFace, ok := face.(text.Face)
	if !ok || textFace == nil {
		return render.ErrFallbackToCPU
	}
	rc.sceneStats.TextCount++

	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}

	// Integer-align the mask origin: Skia-AAA scans integer pixel rows, so
	// shifting the path by an integral offset keeps the mask pixels identical
	// to the pixels the CPU Tier2 fill produces on the real surface.
	mask, bx, by, bw, bh, err := rasterizeTransformMask(devicePath)
	if err != nil {
		return render.ErrFallbackToCPU
	}

	rc.shared.mu.Lock()
	rc.shared.ensureGlyphMaskEngine()
	engine := rc.shared.glyphMaskEngine
	rc.shared.mu.Unlock()

	region, err := engine.PutTransformMask(textFace, s, deviceScale, matrix, mask, bw, bh)
	if err != nil || region.Width <= 0 || region.Height <= 0 {
		return render.ErrFallbackToCPU
	}

	// Device-space quad with identity CTM (ortho projection is applied at
	// flush time), sampling the mask region from the glyph-mask atlas.
	quad := GlyphMaskQuad{
		X0: float32(bx), Y0: float32(by),
		X1: float32(bx + bw), Y1: float32(by + bh),
		U0: region.U0, V0: region.V0,
		U1: region.U1, V1: region.V1,
		Page: region.AtlasIndex,
	}
	batch := GlyphMaskBatch{
		Quads:          []GlyphMaskQuad{quad},
		Transform:      render.Matrix{A: 1, E: 1}, // identity: quads are device-space
		Color:          premulRGBA(color),
		AtlasPageIndex: region.AtlasIndex,
	}
	rc.queueGlyphMaskSplit(target, batch)
	return nil
}

// rasterizeTransformMask CPU-rasterizes a device-space outline path into an
// integer-aligned alpha mask using the exact Skia-AAA analytic scanline filler
// the CPU Tier2 outline path uses (raster.AnalyticFiller with the same
// aaShift/clip/flatten settings as SoftwareRenderer's analytic branch). It
// deliberately bypasses SoftwareRenderer.Fill's Auto mode — that mode may pick
// the globally-registered AdaptiveFiller (tile supersampling) once any
// render/gpu or render/raster import is alive, which would diverge from the
// CPU reference output. Returns the mask row-major and the device-space origin
// it corresponds to.
func rasterizeTransformMask(devicePath *render.Path) (mask []byte, bx, by, bw, bh int, err error) {
	b := devicePath.Bounds()
	bw, bh = b.Dx(), b.Dy()
	if bw <= 0 || bh <= 0 || bw > 4096 || bh > 4096 {
		return nil, 0, 0, 0, 0, render.ErrFallbackToCPU
	}
	bx, by = b.Min.X, b.Min.Y
	shifted := devicePath
	if bx != 0 || by != 0 {
		// Translation only: TransformPoint is (A·x + B·y + C, D·x + E·y + F),
		// so the Y-axis scale lives in E, not D.
		shifted = devicePath.Transform(render.Matrix{A: 1, E: 1, C: -float64(bx), F: -float64(by)})
	}

	// Same edge-builder configuration as SoftwareRenderer's analytic branch
	// (render/software.go Fill): 4x AA shift, 0.1px flatten tolerance,
	// 2px clip margin, adaptive curve flattening.
	eb := raster.NewEdgeBuilder(2)
	eb.SetFlattenTolerance(0.1)
	eb.SetClipRect(&raster.Rect{MinX: -2, MinY: -2, MaxX: float32(bw) + 2, MaxY: float32(bh) + 2})
	eb.SetFlattenCurves(true)
	defer eb.SetFlattenCurves(false)

	verbs := shifted.Verbs()
	if len(verbs) == 0 {
		return make([]byte, bw*bh), bx, by, bw, bh, nil
	}
	verbBytes := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(verbs))), len(verbs))
	eb.BuildFromPathF64(verbBytes, shifted.Coords())
	if eb.IsEmpty() {
		return make([]byte, bw*bh), bx, by, bw, bh, nil
	}

	af := raster.NewAnalyticFiller(bw, bh)
	mask = make([]byte, bw*bh)
	af.Fill(eb, raster.FillRuleNonZero, func(y int, runs *raster.AlphaRuns) {
		if y < 0 || y >= bh {
			return
		}
		row := y * bw
		for x, alpha := range runs.Iter() {
			if alpha != 0 && x >= 0 && x < bw {
				mask[row+x] = alpha
			}
		}
	})
	return mask, bx, by, bw, bh, nil
}
