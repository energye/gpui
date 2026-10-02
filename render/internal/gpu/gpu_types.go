//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// texViewNative adapts a hal.TextureView to res.Native. Destroy is
// idempotent at the webgpu layer (released-guard), so registering the same
// view for multiple commands is safe.
type texViewNative struct{ v hal.TextureView }

func (n texViewNative) Release() {
	if n.v != nil {
		n.v.Destroy()
	}
}

// GPUTextureDrawCommand represents a GPU-to-GPU texture compositing command.
// Unlike ImageDrawCommand (CPU pixel upload), this draws a pre-existing GPU
// texture view directly — zero CPU readback, zero re-upload.
//
// The View is a res.View: either a SourceKey resolved at flush time to
// the current active instance (deferred, never a stale snapshot) or a strong
// Ref to a direct resource. This mirrors the Skia GrSurfaceProxyView
// direct-bind pattern with deferred/instantiated forms.
type GPUTextureDrawCommand struct {
	View       res.View
	DstX, DstY float32
	DstW, DstH float32
	// Optional source UV rect in normalized texture space. Zero U1/V1 means full
	// texture (0,0)-(1,1). Used for damage-tight layer composites (F1).
	U0, V0, U1, V1 float32
	Opacity        float32
	ViewportWidth  uint32
	ViewportHeight uint32
	// YUV planes variant (P3-A layer 4): when IsYUV is set this command
	// draws an NV12 frame (View = Y=R8 full height, UVView = RG8
	// interleaved half height, same normalized UVs) through the YUV
	// convert pipeline instead of the single-texture path. View keeps
	// its shared queue role (counts, seals, damage, Y resolve); the UV
	// view rides alongside and resolves at flush time.
	UVView res.View
	IsYUV  bool
	// Rigid quad corners (transform compositing): CTM-transformed TL/TR/BR/BL
	// in device pixels. Zero corners = axis-aligned fallback using Dst rect
	// (bit-identical to before). Lets rigid bodies record once and replay
	// rotated/scaled every frame instead of re-recording vector content.
	TLX, TLY float32
	TRX, TRY float32
	BRX, BRY float32
	BLX, BLY float32
}

// scissorSegment records a scissor state change along with the cumulative
// pending counts at the time of the change. Used to slice pending arrays
// into per-scissor groups during Flush().
type scissorSegment struct {
	rect         [4]uint32    // scissor rect (valid when hasRect=true)
	hasRect      bool         // false = full framebuffer
	clipRRect    ClipParams   // RRect clip (valid when hasClipRRect=true)
	hasClipRRect bool         // false = no RRect clip
	clipPath     *render.Path // arbitrary clip path for depth clipping (GPU-CLIP-003a)
	sdfCount     int          // len(pendingShapes) at time of change
	convexCount  int          // len(pendingConvexCommands) at time of change
	stencilCount int          // len(pendingStencilPaths) at time of change
	imageCount   int          // len(pendingImageCommands) at time of change
	gpuTexCount  int          // len(pendingGPUTextureCommands) at time of change
	textCount    int          // len(pendingTextBatches) at time of change
	glyphCount   int          // len(pendingGlyphMaskBatches) at time of change
}

// extractConvexPolygon checks if a path is a single closed contour made entirely
// of line segments that form a convex polygon. If so, it returns the polygon
// points. If the path contains curves, multiple subpaths, or is not convex,
// it returns nil, false.
//
// This enables Tier 2a (convex fast-path) for paths like triangles, pentagons,
// and other convex shapes that don't need stencil-then-cover.
func extractConvexPolygon(path *render.Path) ([]render.Point, bool) {
	if path.NumVerbs() < 3 {
		return nil, false
	}

	var points []render.Point
	moveCount := 0
	closed := false
	hasCurves := false

	path.Iterate(func(verb render.PathVerb, coords []float64) {
		if hasCurves {
			return
		}
		switch verb {
		case render.MoveTo:
			moveCount++
			if moveCount > 1 {
				hasCurves = true // abuse flag for early exit
				return
			}
			points = append(points, render.Pt(coords[0], coords[1]))
		case render.LineTo:
			points = append(points, render.Pt(coords[0], coords[1]))
		case render.QuadTo, render.CubicTo:
			hasCurves = true
		case render.Close:
			closed = true
		}
	})

	if hasCurves || !closed || moveCount != 1 || len(points) < 3 {
		return nil, false
	}

	if !IsConvex(points) {
		return nil, false
	}

	return points, true
}

// convertPathVerbsToStroke and strokeResultToPath are also shared utilities
// used by both SDFAccelerator and GPURenderContext.
// They remain in their original location (vello_accelerator.go or similar).
