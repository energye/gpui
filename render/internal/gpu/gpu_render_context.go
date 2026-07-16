//go:build !nogpu

package gpu

import (
	"fmt"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/stroke"
	"github.com/energye/gpui/render/text"
)

// GPURenderContext holds per-gg.Context GPU state: pending draw commands,
// clip state, frame tracking, and its own render session. Each gg.Context
// lazily creates one GPURenderContext, ensuring isolated pending command
// queues and independent LoadOp tracking.
//
// This follows the enterprise pattern: Skia SurfaceFillContext.fOpsTask
// (per-surface), Flutter EntityPass (per-layer), Vello Scene (per-call).
//
// GPURenderContext references the shared GPUShared for device, pipelines,
// and atlas engines but never owns them.
type GPURenderContext struct {
	shared *GPUShared // reference to shared resources (NOT owned)

	// Per-context render session (owns frame textures: MSAA, depth, resolve).
	session *GPURenderSession

	// Per-context pending command queues.
	pendingShapes         []SDFRenderShape
	pendingConvexCommands []ConvexDrawCommand
	// Scratch backing for QueueColoredMesh (avoid per-triangle heap allocs).
	// Slices in pending ConvexDrawCommand point into these until Flush clears.
	convexMeshPts             []render.Point
	convexMeshVCs             [][4]float32
	pendingStencilPaths       []StencilPathCommand
	pendingImageCommands      []ImageDrawCommand
	pendingGPUTextureCommands []GPUTextureDrawCommand
	pendingTextBatches        []TextBatch
	pendingGlyphMaskBatches   []GlyphMaskBatch
	baseLayer                 *GPUTextureDrawCommand
	pendingTarget             render.GPURenderTarget
	hasPendingTarget          bool

	// Per-context clip state.
	clipRect        *[4]uint32
	clipRRect       *ClipParams
	clipPath        *render.Path // arbitrary clip path for depth clipping (GPU-CLIP-003a)
	scissorSegments []scissorSegment
	scissorGroups   []ScissorGroup
	scissorRects    [][4]uint32
	scissorClips    []ClipParams

	// Per-tier batch seal flags prevent merging across scissor boundaries.
	// Two separate flags needed: a single shared flag would be consumed by
	// the first Queue call (e.g., QueueText), allowing the second tier
	// (e.g., QueueGlyphMask) to merge across the same boundary.
	textBatchSealed  bool // seals QueueText (MSDF Tier 4)
	glyphBatchSealed bool // seals QueueGlyphMask (Tier 6)

	// Per-context frame tracking (fixes LoadOp corruption).
	// When frameRendered is true, subsequent render passes use LoadOpLoad.
	// Reset by BeginFrame() at the start of each frame.
	frameRendered bool
	lastView      *webgpu.TextureView

	// Per-context scene stats (for Auto pipeline mode).
	sceneStats render.SceneStats

	// G.04 / residual brush bootstrap diagnostic (ColorAt stage + GPU blit).
	brushBootstrapReason string
	pipelineMode         render.PipelineMode

	// Anti-aliasing state for GPU rendering (propagated from Context).
	antiAlias bool

	// Shared command encoder for single-command-buffer frames (ADR-017).
	// When set, Flush records render passes into this encoder instead of
	// creating its own + submitting. The caller owns Finish + Submit.
	sharedEncoder *webgpu.CommandEncoder
}

// LastSubmitPathStats returns S6.2 encode/submit counters from the last Flush.
func (rc *GPURenderContext) LastSubmitPathStats() SubmitPathStats {
	if rc == nil || rc.session == nil {
		return SubmitPathStats{}
	}
	return rc.session.LastSubmitPathStats()
}

// LastBatchDrawStats returns S6.3 post-coalesce draw/quad counters from the last Flush.
func (rc *GPURenderContext) LastBatchDrawStats() BatchDrawStats {
	if rc == nil || rc.session == nil {
		return BatchDrawStats{}
	}
	return rc.session.LastBatchDrawStats()
}

// PendingCount returns the total number of pending commands (for testing).
func (rc *GPURenderContext) PendingCount() int {
	n := len(rc.pendingShapes) + len(rc.pendingConvexCommands) +
		len(rc.pendingStencilPaths) + len(rc.pendingImageCommands) + len(rc.pendingGPUTextureCommands) +
		len(rc.pendingTextBatches) + len(rc.pendingGlyphMaskBatches)
	if rc.baseLayer != nil {
		n++
	}
	return n
}

// SetPipelineMode sets the pipeline mode for this context's operations.
func (rc *GPURenderContext) SetPipelineMode(mode render.PipelineMode) {
	rc.pipelineMode = mode
}

// SetAntiAlias sets the anti-aliasing state for GPU rendering.
// When false, SDF shapes use binary step coverage instead of smoothstep.
func (rc *GPURenderContext) SetAntiAlias(enabled bool) {
	rc.antiAlias = enabled
}

// SetClipRect records a scissor rect change for this context.
func (rc *GPURenderContext) SetClipRect(x, y, w, h uint32) {
	rect := [4]uint32{x, y, w, h}
	rc.clipRect = &rect
	rc.recordScissorSegment(&rect)
}

// ClearClipRect removes the scissor rect for this context.
func (rc *GPURenderContext) ClearClipRect() {
	rc.clipRect = nil
	rc.recordScissorSegment(nil)
}

// SetClipRRect sets the rounded rectangle clip for this context.
func (rc *GPURenderContext) SetClipRRect(x, y, w, h, radius float32) {
	rc.clipRRect = &ClipParams{
		RectX1:  x,
		RectY1:  y,
		RectX2:  x + w,
		RectY2:  y + h,
		Radius:  radius,
		Enabled: 1.0,
	}
	rc.recordScissorSegment(rc.clipRect)
}

// ClearClipRRect removes the rounded rectangle clip for this context.
func (rc *GPURenderContext) ClearClipRRect() {
	rc.clipRRect = nil
	rc.recordScissorSegment(rc.clipRect)
}

// SetClipPath sets an arbitrary clip path for depth-based clipping (GPU-CLIP-003a).
// The path must be in device-space coordinates. When set, subsequent draws are
// clipped to the path region via the depth buffer. The path is fan-tessellated
// and rendered to the depth buffer before content; content fragments test against
// the clip depth so only pixels within the clipped region pass.
func (rc *GPURenderContext) SetClipPath(path *render.Path) {
	rc.clipPath = path
	rc.recordScissorSegment(rc.clipRect)
}

// ClearClipPath removes the arbitrary clip path, restoring full rendering.
func (rc *GPURenderContext) ClearClipPath() {
	rc.clipPath = nil
	rc.recordScissorSegment(rc.clipRect)
}

// BeginFrame resets per-frame state so the first render pass clears the surface.
func (rc *GPURenderContext) BeginFrame() {
	rc.clipRect = nil
	rc.clipPath = nil
	rc.frameRendered = false
	rc.lastView = nil
	rc.textBatchSealed = false
	rc.glyphBatchSealed = false
}

// SetSharedEncoder sets a shared command encoder for single-command-buffer
// frames (ADR-017). When set, Flush() records render passes into this encoder
// instead of creating its own and submitting. The caller is responsible for
// encoder.Finish() + queue.Submit() after all contexts have flushed.
// Pass a zero-value CommandEncoder (IsNil() == true) to restore normal
// per-context submit behavior.
func (rc *GPURenderContext) SetSharedEncoder(encoder gpucontext.CommandEncoder) {
	if encoder.IsNil() {
		rc.sharedEncoder = nil
		return
	}
	rc.sharedEncoder = (*webgpu.CommandEncoder)(encoder.Pointer())
}

// CreateEncoder creates a new command encoder for shared use across contexts.
// Returns a zero-value CommandEncoder (IsNil() == true) if the session is not
// initialized or encoder creation fails.
func (rc *GPURenderContext) CreateEncoder() gpucontext.CommandEncoder {
	if rc.session == nil {
		return gpucontext.CommandEncoder{}
	}
	enc, err := rc.session.device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{
		Label: "shared_frame_encoder",
	})
	if err != nil {
		return gpucontext.CommandEncoder{}
	}
	return gpucontext.NewCommandEncoder(unsafe.Pointer(enc)) //nolint:gosec // Go spec Rule 1 (ADR-018)
}

// SubmitEncoder finishes the shared encoder and submits the command buffer.
func (rc *GPURenderContext) SubmitEncoder(encoder gpucontext.CommandEncoder) error {
	if rc.session == nil {
		return fmt.Errorf("GPU session not initialized")
	}
	if encoder.IsNil() {
		return fmt.Errorf("nil command encoder")
	}
	enc := (*webgpu.CommandEncoder)(encoder.Pointer())
	cmdBuf, err := enc.Finish()
	if err != nil {
		return fmt.Errorf("finish shared encoder: %w", err)
	}
	if _, err := rc.session.queue.Submit(cmdBuf); err != nil {
		rc.session.device.FreeCommandBuffer(cmdBuf)
		return fmt.Errorf("submit shared encoder: %w", err)
	}
	return nil
}

// SceneStats returns the accumulated scene statistics for this context.
func (rc *GPURenderContext) SceneStats() render.SceneStats {
	return rc.sceneStats
}

// noteBrushBootstrap records an explicit ColorAt→GPU-blit bootstrap reason (G.04).
func (rc *GPURenderContext) noteBrushBootstrap(reason string) {
	if rc == nil || reason == "" {
		return
	}
	rc.brushBootstrapReason = reason
}

// TakeBrushBootstrapReason returns and clears the last brush bootstrap reason.
func (rc *GPURenderContext) TakeBrushBootstrapReason() string {
	if rc == nil {
		return ""
	}
	r := rc.brushBootstrapReason
	rc.brushBootstrapReason = ""
	return r
}

// QueueShape accumulates an SDF shape for batch dispatch.
func (rc *GPURenderContext) QueueShape(target render.GPURenderTarget, shape render.DetectedShape, paint *render.Paint, stroked bool) error {
	// If target changed, flush previous batch first.
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if err := rc.Flush(rc.pendingTarget); err != nil {
			return err
		}
	}

	rs, ok := DetectedShapeToRenderShape(shape, paint, stroked)
	if !ok {
		return render.ErrFallbackToCPU
	}

	// Skip zero-alpha shapes — premultiplied SrcOver with (0,0,0,0) is a
	// mathematical no-op but wastes GPU bandwidth and can interfere with
	// MSAA sample coverage weighting (BUG-SDF-001: transparent fill makes
	// subsequent stroke invisible). Enterprise pattern: Skia nothingToDraw()
	// (SkPaint.cpp:273), Cairo nothing_to_do() (cairo-surface.c:2148).
	if rs.ColorA == 0 {
		return nil
	}

	rc.pendingShapes = append(rc.pendingShapes, rs)

	rc.pendingTarget = target
	rc.hasPendingTarget = true
	return nil
}

// QueueConvex accumulates a convex polygon for batch dispatch.
func (rc *GPURenderContext) QueueConvex(target render.GPURenderTarget, cmd ConvexDrawCommand) {
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if fErr := rc.Flush(rc.pendingTarget); fErr != nil {
			slogger().Warn("auto-flush failed", "err", fErr)
		}
	}
	rc.pendingConvexCommands = append(rc.pendingConvexCommands, cmd)
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// QueueColoredMesh queues a triangle mesh with optional per-vertex colors
// via the convex fast-path.
// positions are pixel-space points; colors are straight RGBA (premultiplied here).
// triangleList=true groups positions as independent triangles; false = fan.
//
// Hot path (DrawMesh / 3D): ONE TriangleList command for the whole mesh
// (not N tri-commands), backed by reusable scratch. SkipAA solid verts.
func (rc *GPURenderContext) QueueColoredMesh(target render.GPURenderTarget, positions []render.Point, colors []render.RGBA, triangleList bool) {
	if len(positions) < 3 {
		return
	}
	nOut := 0
	if triangleList {
		nOut = (len(positions) / 3) * 3
	} else {
		// Expand fan to triangle list once: (0,i,i+1)
		nTri := len(positions) - 2
		if nTri <= 0 {
			return
		}
		nOut = nTri * 3
	}
	if nOut < 3 {
		return
	}
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if fErr := rc.Flush(rc.pendingTarget); fErr != nil {
			slogger().Warn("auto-flush failed", "err", fErr)
		}
	}

	useVC := len(colors) == len(positions)
	need := nOut

	var pts []render.Point
	var vcs [][4]float32
	ptsBase := 0
	vcBase := 0
	if cap(rc.convexMeshPts)-len(rc.convexMeshPts) >= need {
		ptsBase = len(rc.convexMeshPts)
		rc.convexMeshPts = rc.convexMeshPts[:ptsBase+need]
		pts = rc.convexMeshPts
		if useVC {
			if cap(rc.convexMeshVCs)-len(rc.convexMeshVCs) < need {
				vcs = make([][4]float32, need)
				vcBase = 0
			} else {
				vcBase = len(rc.convexMeshVCs)
				rc.convexMeshVCs = rc.convexMeshVCs[:vcBase+need]
				vcs = rc.convexMeshVCs
			}
		}
	} else if len(rc.convexMeshPts) == 0 {
		capN := need
		if capN < 512 {
			capN = 512
		}
		rc.convexMeshPts = make([]render.Point, need, capN)
		pts = rc.convexMeshPts
		ptsBase = 0
		if useVC {
			rc.convexMeshVCs = make([][4]float32, need, capN)
			vcs = rc.convexMeshVCs
			vcBase = 0
		}
	} else {
		pts = make([]render.Point, need)
		ptsBase = 0
		if useVC {
			vcs = make([][4]float32, need)
			vcBase = 0
		}
	}

	toPremul := func(c render.RGBA) [4]float32 {
		return [4]float32{
			float32(c.R * c.A),
			float32(c.G * c.A),
			float32(c.B * c.A),
			float32(c.A),
		}
	}

	// Pack triangle-list geometry into contiguous scratch.
	if triangleList {
		copy(pts[ptsBase:ptsBase+need], positions[:need])
		if useVC {
			for i := 0; i < need; i++ {
				vcs[vcBase+i] = toPremul(colors[i])
			}
		}
	} else {
		o := 0
		for i := 1; i+1 < len(positions); i++ {
			pts[ptsBase+o+0] = positions[0]
			pts[ptsBase+o+1] = positions[i]
			pts[ptsBase+o+2] = positions[i+1]
			if useVC {
				vcs[vcBase+o+0] = toPremul(colors[0])
				vcs[vcBase+o+1] = toPremul(colors[i])
				vcs[vcBase+o+2] = toPremul(colors[i+1])
			}
			o += 3
		}
	}

	solid := [4]float32{0, 0, 0, 1}
	if useVC {
		// Average first triangle as solid fallback for pipelines that ignore VC.
		c0 := vcs[vcBase+0]
		c1 := vcs[vcBase+1]
		c2 := vcs[vcBase+2]
		solid = [4]float32{
			(c0[0] + c1[0] + c2[0]) / 3,
			(c0[1] + c1[1] + c2[1]) / 3,
			(c0[2] + c1[2] + c2[2]) / 3,
			(c0[3] + c1[3] + c2[3]) / 3,
		}
	} else if len(colors) > 0 {
		solid = toPremul(colors[0])
	}

	cmd := ConvexDrawCommand{
		Points:       pts[ptsBase : ptsBase+need : ptsBase+need],
		Color:        solid,
		SkipAA:       true,
		TriangleList: true,
	}
	if useVC {
		cmd.VertexColors = vcs[vcBase : vcBase+need : vcBase+need]
	}
	rc.pendingConvexCommands = append(rc.pendingConvexCommands, cmd)
	rc.pendingTarget = target
	rc.hasPendingTarget = true
	rc.sceneStats.ShapeCount++
}

// QueueStencil accumulates a stencil path for batch dispatch.
func (rc *GPURenderContext) QueueStencil(target render.GPURenderTarget, cmd StencilPathCommand) {
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if fErr := rc.Flush(rc.pendingTarget); fErr != nil {
			slogger().Warn("auto-flush failed", "err", fErr)
		}
	}
	rc.pendingStencilPaths = append(rc.pendingStencilPaths, cmd)
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// QueueText accumulates an MSDF text batch for dispatch.
// Adjacent batches with identical visual properties (transform, color, atlas,
// MSDF parameters) are coalesced into a single batch to minimize GPU draw calls (ADR-031).
func (rc *GPURenderContext) QueueText(target render.GPURenderTarget, batch TextBatch) {
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if fErr := rc.Flush(rc.pendingTarget); fErr != nil {
			slogger().Warn("auto-flush failed", "err", fErr)
		}
	}
	// Coalesce with last pending batch if same visual properties (ADR-031).
	// Skip merging if a scissor boundary was crossed since the last batch was
	// queued (textBatchSealed=true); this keeps text batches within the
	// correct scissor group so text is not clipped by a sibling element's rect.
	if n := len(rc.pendingTextBatches); n > 0 && !rc.textBatchSealed {
		last := &rc.pendingTextBatches[n-1]
		if last.CanMerge(batch) {
			last.Quads = append(last.Quads, batch.Quads...)
			rc.pendingTarget = target
			rc.hasPendingTarget = true
			return
		}
	}
	rc.textBatchSealed = false // new batch started; allow future merges within same scissor region
	rc.pendingTextBatches = append(rc.pendingTextBatches, batch)
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// QueueImageDraw accumulates an image draw command for Tier 3 dispatch.
// Destination is a CTM-transformed quad given as TL, TR, BR, BL corners in
// device pixels. Parameters are kept primitive to avoid import cycles.
func (rc *GPURenderContext) QueueImageDraw(target render.GPURenderTarget, pixelData []byte, genID uint64, imgWidth, imgHeight, imgStride int,
	tlX, tlY, trX, trY, brX, brY, blX, blY, opacity float32, viewportW, viewportH uint32,
	u0, v0, u1, v1 float32,
	nearest bool,
) {
	minX := min4f(tlX, trX, brX, blX)
	minY := min4f(tlY, trY, brY, blY)
	maxX := max4f(tlX, trX, brX, blX)
	maxY := max4f(tlY, trY, brY, blY)
	cmd := ImageDrawCommand{
		PixelData:      pixelData,
		GenerationID:   genID,
		ImgWidth:       imgWidth,
		ImgHeight:      imgHeight,
		ImgStride:      imgStride,
		DstX:           minX,
		DstY:           minY,
		DstW:           maxX - minX,
		DstH:           maxY - minY,
		TLX:            tlX,
		TLY:            tlY,
		TRX:            trX,
		TRY:            trY,
		BRX:            brX,
		BRY:            brY,
		BLX:            blX,
		BLY:            blY,
		Opacity:        opacity,
		ViewportWidth:  viewportW,
		ViewportHeight: viewportH,
		U0:             u0,
		V0:             v0,
		U1:             u1,
		V1:             v1,
		Nearest:        nearest,
	}
	rc.queueImageCmd(target, cmd)
}

func min4f(a, b, c, d float32) float32 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	if d < m {
		m = d
	}
	return m
}

func max4f(a, b, c, d float32) float32 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	if d > m {
		m = d
	}
	return m
}

// queueImageCmd accumulates an image draw command for Tier 3 dispatch.
func (rc *GPURenderContext) queueImageCmd(target render.GPURenderTarget, cmd ImageDrawCommand) {
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if fErr := rc.Flush(rc.pendingTarget); fErr != nil {
			slogger().Warn("auto-flush failed", "err", fErr)
		}
	}
	rc.pendingImageCommands = append(rc.pendingImageCommands, cmd)
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// QueueBaseLayer sets the compositor base layer — a textured quad drawn BEFORE
// all tiers in the render pass. Last call wins. Used for CPU pixmap compositing
// in zero-readback rendering (ADR-015, Flutter OffsetLayer pattern).
func (rc *GPURenderContext) QueueBaseLayer(target render.GPURenderTarget, view gpucontext.TextureView,
	dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32,
) {
	rc.baseLayer = &GPUTextureDrawCommand{
		View: view, DstX: dstX, DstY: dstY, DstW: dstW, DstH: dstH,
		Opacity: opacity, ViewportWidth: vpW, ViewportHeight: vpH,
	}
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// QueueGPUTextureDraw queues a GPU-to-GPU texture compositing command.
// The texture view is sampled directly — zero CPU readback, zero upload.
func (rc *GPURenderContext) QueueGPUTextureDraw(target render.GPURenderTarget, view gpucontext.TextureView,
	dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32,
) {
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if fErr := rc.Flush(rc.pendingTarget); fErr != nil {
			slogger().Warn("auto-flush failed", "err", fErr)
		}
	}
	rc.pendingGPUTextureCommands = append(rc.pendingGPUTextureCommands, GPUTextureDrawCommand{
		View: view, DstX: dstX, DstY: dstY, DstW: dstW, DstH: dstH,
		Opacity: opacity, ViewportWidth: vpW, ViewportHeight: vpH,
	})
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// QueueGlyphMask accumulates a glyph mask batch for dispatch.
// Adjacent batches with identical visual properties (transform, color, LCD mode,
// atlas page) are coalesced into a single batch to minimize GPU draw calls (ADR-031).
func (rc *GPURenderContext) QueueGlyphMask(target render.GPURenderTarget, batch GlyphMaskBatch) {
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if fErr := rc.Flush(rc.pendingTarget); fErr != nil {
			slogger().Warn("auto-flush failed", "err", fErr)
		}
	}
	// Coalesce with last pending batch if same visual properties (ADR-031).
	// Skip merging if a scissor boundary was crossed since the last batch was
	// queued (glyphBatchSealed=true); this keeps glyph batches within the
	// correct scissor group so text is not clipped by a sibling element's rect.
	if n := len(rc.pendingGlyphMaskBatches); n > 0 && !rc.glyphBatchSealed {
		last := &rc.pendingGlyphMaskBatches[n-1]
		if last.CanMerge(batch) {
			last.Quads = append(last.Quads, batch.Quads...)
			rc.pendingTarget = target
			rc.hasPendingTarget = true
			return
		}
	}
	rc.glyphBatchSealed = false
	rc.pendingGlyphMaskBatches = append(rc.pendingGlyphMaskBatches, batch)
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// DrawText shapes and queues text for MSDF rendering (Tier 4).
func (rc *GPURenderContext) DrawText(target render.GPURenderTarget, face any, s string, x, y float64, color render.RGBA, matrix render.Matrix, deviceScale float64) error {
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

	rc.shared.mu.Lock()
	rc.shared.ensureTextEngine()
	engine := rc.shared.textEngine
	rc.shared.mu.Unlock()

	batch, err := engine.LayoutText(textFace, s, x, y, color, matrix, deviceScale)
	if err != nil {
		slogger().Debug("DrawText: LayoutText failed", "err", err, "text", s)
		return render.ErrFallbackToCPU
	}
	if len(batch.Quads) == 0 {
		return nil
	}

	rc.QueueText(target, batch)
	return nil
}

// DrawGlyphMaskText shapes and queues text for glyph mask rendering (Tier 6).
func (rc *GPURenderContext) DrawGlyphMaskText(target render.GPURenderTarget, face any, s string, x, y float64, color render.RGBA, matrix render.Matrix, deviceScale float64) error {
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

	rc.shared.mu.Lock()
	rc.shared.ensureGlyphMaskEngine()
	engine := rc.shared.glyphMaskEngine
	rc.shared.mu.Unlock()

	batch, err := engine.LayoutText(textFace, s, x, y, color, matrix, deviceScale)
	if err != nil {
		slogger().Debug("DrawGlyphMaskText: LayoutText failed", "err", err, "text", s, "w", target.Width, "h", target.Height)
		return render.ErrFallbackToCPU
	}
	if len(batch.Quads) == 0 {
		return nil
	}

	rc.QueueGlyphMask(target, batch)
	return nil
}

// DrawGlyphMaskTextAliased shapes and queues text for aliased (binary coverage)
// glyph mask rendering. Same pipeline as DrawGlyphMaskText but rasterizes with
// NoAAFiller (0/255 only) instead of AnalyticFiller (256-level AA).
func (rc *GPURenderContext) DrawGlyphMaskTextAliased(target render.GPURenderTarget, face any, s string, x, y float64, color render.RGBA, matrix render.Matrix, deviceScale float64) error {
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

	rc.shared.mu.Lock()
	rc.shared.ensureGlyphMaskEngine()
	engine := rc.shared.glyphMaskEngine
	rc.shared.mu.Unlock()

	batch, err := engine.LayoutTextAliased(textFace, s, x, y, color, matrix, deviceScale)
	if err != nil {
		slogger().Debug("DrawGlyphMaskTextAliased: LayoutTextAliased failed", "err", err, "text", s, "w", target.Width, "h", target.Height)
		return render.ErrFallbackToCPU
	}
	if len(batch.Quads) == 0 {
		return nil
	}

	rc.QueueGlyphMask(target, batch)
	return nil
}

// DrawShapedGlyphMaskText renders pre-shaped glyphs through the glyph mask pipeline.
// Same as DrawGlyphMaskText but skips shaping — uses stored glyph positions directly.
func (rc *GPURenderContext) DrawShapedGlyphMaskText(target render.GPURenderTarget, face any, glyphs []text.ShapedGlyph, x, y float64, color render.RGBA, matrix render.Matrix, deviceScale float64) error {
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

	rc.shared.mu.Lock()
	rc.shared.ensureGlyphMaskEngine()
	engine := rc.shared.glyphMaskEngine
	rc.shared.mu.Unlock()

	isCJK := len(glyphs) > 0 && glyphs[0].IsCJK
	batch, err := engine.LayoutShapedGlyphs(textFace, glyphs, x, y, color, matrix, deviceScale, isCJK)
	if err != nil {
		return render.ErrFallbackToCPU
	}
	if len(batch.Quads) == 0 {
		return nil
	}

	rc.QueueGlyphMask(target, batch)
	return nil
}

// FillPath queues a filled path for GPU rendering.
func (rc *GPURenderContext) FillPath(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	// L.06: prefer cover-inline R8 (convex / stencil-then-cover) when MaskAware
	// texture is live. Advanced paints fall back to fillMaskedAsImage.
	if paint != nil && paint.MaskCoverage != nil {
		if rc.tryFillMaskedConvexInline(target, path, paint) {
			return nil
		}
		// Solid SourceOver + GPU mask: fall through to stencil-then-cover with
		// cover-pass R8 sampling (same mask bind group as convex/SDF).
		if !(isGPUSolidPaint(paint) && paintUsesSourceOver(paint) && paintSupportsGPUFixedBlend(paint) && rc.shared.HasGPUMask()) {
			return rc.fillMaskedAsImage(target, path, paint)
		}
		// continue into GPU solid fill path below
	}
	if !isGPUSolidPaint(paint) {
		if paintSupportsGPUAdvancedBlend(paint) {
			return rc.fillAdvancedBlendAsImage(target, path, paint)
		}
		// P0-2: native GPU gradient / image-pattern fill (no large-area ColorAt).
		if err := rc.fillBrushNative(target, path, paint); err == nil {
			return nil
		}
		return rc.fillBrushAsImage(target, path, paint)
	}
	if !paintSupportsGPUFixedBlend(paint) {
		if paintSupportsGPUAdvancedBlend(paint) {
			return rc.fillAdvancedBlendAsImage(target, path, paint)
		}
		return render.ErrFallbackToCPU
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}

	// Q.03: when AA is off, snap path verts to device pixels (Skia Graphite-style).
	if !rc.antiAlias && path != nil {
		path = snapPathToPixelGrid(path)
	}

	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++

	// If in Compute mode, delegate to VelloAccelerator.
	if rc.pipelineMode == render.PipelineModeCompute {
		rc.shared.mu.Lock()
		va := rc.shared.velloAccel
		rc.shared.mu.Unlock()
		if va != nil && va.CanCompute() {
			va.SetAntiAlias(rc.antiAlias)
			return va.FillPath(target, path, paint)
		}
	}

	// If target changed, flush previous batch first.
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if err := rc.Flush(rc.pendingTarget); err != nil {
			return err
		}
	}

	color := getColorFromPaint(paint)
	premulR := float32(color.R * color.A)
	premulG := float32(color.G * color.A)
	premulB := float32(color.B * color.A)
	premulA := float32(color.A)

	// Try convex fast-path (NonZero fill rule only).
	// The convex renderer uses centroid fan tessellation without stencil buffer,
	// which is structurally equivalent to NonZero fill. For EvenOdd paths
	// (e.g., stroke-expanded ring outlines), the stencil-then-cover path below
	// must be used — it correctly implements EvenOdd via stencil bit inversion.
	// Skia Ganesh gates its convex fast-path on isSimpleFill() for the same reason.
	// S6.6: ConvexPathCache avoids re-walking path + IsConvex on retained frames.
	if paint.FillRule != render.FillRuleEvenOdd {
		var points []render.Point
		var ok bool
		if cache := rc.shared.ConvexPathCache(); cache != nil {
			points, ok = cache.GetOrClassify(path)
		} else {
			points, ok = extractConvexPolygon(path)
		}
		if ok {
			slogger().Debug("FillPath: convex fast-path", "points", len(points), "fillRule", paint.FillRule)
			cmd := ConvexDrawCommand{
				Points:    points,
				Color:     [4]float32{premulR, premulG, premulB, premulA},
				BlendMode: paintBlendMode(paint),
			}
			rc.QueueConvex(target, cmd)
			return nil
		}
	}

	// Fall back to stencil-then-cover (S4.3: reuse tessellation via pathGeomCache).
	var fanVerts []float32
	var coverQuad [12]float32
	aaOff := !rc.antiAlias
	fr := paint.FillRule
	if cache := rc.shared.PathGeomCache(); cache != nil {
		if v, cq, ok := cache.GetOrTessellate(path, fr, aaOff); ok {
			fanVerts, coverQuad = v, cq
		}
	}
	if fanVerts == nil {
		tess := NewFanTessellator()
		tess.TessellatePath(path)
		fanVerts = tess.Vertices()
		if len(fanVerts) == 0 {
			return nil
		}
		coverQuad = tess.CoverQuad()
	}
	if len(fanVerts) == 0 {
		return nil
	}

	cmd := StencilPathCommand{
		Vertices:  fanVerts, // already owned copy from cache/miss path
		CoverQuad: coverQuad,
		Color:     [4]float32{premulR, premulG, premulB, premulA},
		FillRule:  paint.FillRule,
		BlendMode: paintBlendMode(paint),
	}
	rc.QueueStencil(target, cmd)
	return nil
}

// StrokePath renders a stroked path by expanding to filled outline.
func (rc *GPURenderContext) StrokePath(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	// R2: non-solid strokes expand to filled outlines then route through FillPath
	// (native gradient/pattern or bootstrap). Solid keeps fixed/advanced blend gates.
	if isGPUSolidPaint(paint) {
		if !paintSupportsGPUFixedBlend(paint) && !paintSupportsGPUAdvancedBlend(paint) {
			return render.ErrFallbackToCPU
		}
	} else {
		// Non-solid: SourceOver bootstrap/native or advanced dual-tex only.
		if !paintUsesSourceOver(paint) && !paintSupportsGPUAdvancedBlend(paint) {
			return render.ErrFallbackToCPU
		}
	}

	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++

	// In PipelineModeCompute, delegate strokes to Vello compute pipeline.
	// Default (Auto) uses stencil-then-cover which handles target.View correctly.
	// Vello compute currently lacks GPU-direct output (writes to CPU pixmap only),
	// so broad routing breaks FlushGPUWithView — see TASK-GG-STROKE-REGRESSION-374.
	if rc.pipelineMode == render.PipelineModeCompute {
		rc.shared.mu.Lock()
		va := rc.shared.velloAccel
		rc.shared.mu.Unlock()
		if va != nil && va.CanCompute() {
			va.SetAntiAlias(rc.antiAlias)
			return va.StrokePath(target, path, paint)
		}
	}

	if path.NumVerbs() == 0 {
		return nil
	}

	// Q.03: snap stroke geometry when AA disabled.
	pathToStroke := path
	if !rc.antiAlias {
		pathToStroke = snapPathToPixelGrid(path)
	}
	var dashHash uint64
	if paint.IsDashed() {
		dash := paint.EffectiveDash()
		if dash != nil && dash.IsDashed() {
			transformScale := paint.TransformScale
			if transformScale <= 0 {
				transformScale = 1.0
			}
			dashHash = hashDash(dash)
			// S6.6: dash geometry cache; apply on snapped path (AA-off correctness).
			if dc := rc.shared.DashGeomCache(); dc != nil {
				pathToStroke = dc.GetOrApply(pathToStroke, dash, transformScale)
			} else {
				d := dash
				if transformScale > 1.0 {
					d = dash.Scale(transformScale)
				}
				pathToStroke = render.ApplyDash(pathToStroke, d)
			}
			if pathToStroke == nil || pathToStroke.NumVerbs() == 0 {
				return nil
			}
		}
	}

	// S4.3/S6.6: stroke expansion cache keyed by path + style + dash.
	skey := makeStrokeCacheKey(pathToStroke, paint, !rc.antiAlias, dashHash)
	var fillPath *render.Path
	if sc := rc.shared.StrokeGeomCache(); sc != nil {
		if cached, ok := sc.Get(skey); ok {
			fillPath = cached
		}
	}
	if fillPath == nil {
		strokeVerbs := convertPathVerbsToStroke(pathToStroke.Verbs())
		style := stroke.Stroke{
			Width:      effectiveStrokeWidth(paint),
			Cap:        stroke.LineCap(paint.EffectiveLineCap()),
			Join:       stroke.LineJoin(paint.EffectiveLineJoin()),
			MiterLimit: paint.EffectiveMiterLimit(),
		}
		expander := stroke.NewStrokeExpander(style)
		outVerbs, outCoords := expander.Expand(strokeVerbs, pathToStroke.Coords())
		if len(outVerbs) == 0 {
			return nil
		}
		fillPath = strokeResultToPath(outVerbs, outCoords)
		if sc := rc.shared.StrokeGeomCache(); sc != nil && fillPath != nil {
			sc.Put(skey, fillPath)
		}
	}

	// EvenOdd correctly handles both stroke topologies:
	//   - Smooth paths (round-rects, circles): 2-contour ring (outer CW + inner CCW).
	//     Each pixel in the hollow center is toggled twice → stencil=0 → empty.
	//   - Sharp paths (rectangles): inner-pivot V-shapes create self-intersections.
	//     V-shape area is toggled twice → stencil=0 → correctly hollow.
	// The stencil EvenOdd pipeline uses IncrementWrap+WriteMask=0x01 (parity toggle)
	// instead of StencilOperationInvert, which has a driver bug on AMD D3D12 (#374).
	// NonZero would miscount V-shape area as winding=2 → solid fill (wrong for sharp paths).
	strokePaint := *paint
	strokePaint.FillRule = render.FillRuleEvenOdd
	return rc.FillPath(target, fillPath, &strokePaint)
}

// FillShape accumulates a filled shape for batch dispatch.
func (rc *GPURenderContext) FillShape(target render.GPURenderTarget, shape render.DetectedShape, paint *render.Paint) error {
	// L.06: prefer SDF cover-inline R8 when MaskAware texture is live.
	if paint != nil && paint.MaskCoverage != nil {
		if rc.tryFillMaskedSDFInline(target, shape, paint) {
			return nil
		}
		pth := detectedShapeToPath(shape)
		if pth == nil {
			return render.ErrFallbackToCPU
		}
		return rc.fillMaskedAsImage(target, pth, paint)
	}

	rc.sceneStats.ShapeCount++

	if !isGPUSolidPaint(paint) {
		p := detectedShapeToPath(shape)
		if p == nil {
			return render.ErrFallbackToCPU
		}
		if err := rc.fillBrushNative(target, p, paint); err == nil {
			return nil
		}
		return rc.fillBrushAsImage(target, p, paint)
	}
	// SDF pipelines are SourceOver-only; PD / advanced modes route via FillPath.
	if !paintUsesSourceOver(paint) {
		p := detectedShapeToPath(shape)
		if p == nil {
			return render.ErrFallbackToCPU
		}
		return rc.FillPath(target, p, paint)
	}
	if !rc.shared.gpuReady {
		return rc.shared.cpuFallback.FillShape(target, shape, paint)
	}

	// If in Compute mode, delegate to VelloAccelerator.
	if rc.pipelineMode == render.PipelineModeCompute {
		rc.shared.mu.Lock()
		va := rc.shared.velloAccel
		rc.shared.mu.Unlock()
		if va != nil && va.CanCompute() {
			va.SetAntiAlias(rc.antiAlias)
			return va.FillShape(target, shape, paint)
		}
	}

	return rc.QueueShape(target, shape, paint, false)
}

// StrokeShape accumulates a stroked shape for batch dispatch.
func (rc *GPURenderContext) StrokeShape(target render.GPURenderTarget, shape render.DetectedShape, paint *render.Paint) error {
	rc.sceneStats.ShapeCount++

	if !rc.shared.gpuReady {
		return rc.shared.cpuFallback.StrokeShape(target, shape, paint)
	}

	// R3: dashed / non-SO / thin strokes → geometric StrokePath (GPU expand+fill)
	// instead of hard CPU fallback. SDF annular path stays SourceOver solid ≥2px.
	if paint.IsDashed() || !paintUsesSourceOver(paint) || effectiveStrokeWidth(paint) < 2.0 || !isGPUSolidPaint(paint) {
		p := detectedShapeToPath(shape)
		if p == nil {
			return render.ErrFallbackToCPU
		}
		return rc.StrokePath(target, p, paint)
	}

	if rc.pipelineMode == render.PipelineModeCompute {
		rc.shared.mu.Lock()
		va := rc.shared.velloAccel
		rc.shared.mu.Unlock()
		if va != nil && va.CanCompute() {
			va.SetAntiAlias(rc.antiAlias)
			return va.StrokeShape(target, shape, paint)
		}
	}

	return rc.QueueShape(target, shape, paint, true)
}

// ensureLCDDestBase uploads the current CPU pixmap as a GPU base layer so
// two-pass LCD ClearType can blend against real destination colors.
// encodeSubmitReadbackGrouped always LoadOpClears the offscreen RT, so LCD
// must re-seed dest from the pixmap (ClearWithColor / prior readback) each
// flush when no other baseLayer is set.
// Returns a release func for the temporary texture/view (call after Flush render).
func (rc *GPURenderContext) ensureLCDDestBase(target render.GPURenderTarget, hasLCD bool) func() {
	if !hasLCD || rc.baseLayer != nil {
		return nil
	}
	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 || len(target.Data) < tw*th*4 {
		return nil
	}
	device := rc.shared.Device()
	queue := rc.shared.Queue()
	if device == nil || queue == nil {
		return nil
	}
	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label: "lcd_dest_base",
		Size: webgpu.Extent3D{
			Width: uint32(tw), Height: uint32(th), DepthOrArrayLayers: 1, //nolint:gosec
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatRGBA8Unorm,
		Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil
	}
	view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label:         "lcd_dest_base_view",
		Format:        types.TextureFormatRGBA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Release()
		return nil
	}
	// Align pitch for multi-row WriteTexture.
	tight := uint32(tw * 4) //nolint:gosec
	aligned := alignTextureBytesPerRow(tight)
	upload := target.Data[:tw*th*4]
	if aligned != tight && th > 1 {
		padded := make([]byte, int(aligned)*th)
		for y := 0; y < th; y++ {
			copy(padded[y*int(aligned):y*int(aligned)+tw*4], upload[y*tw*4:(y+1)*tw*4])
		}
		upload = padded
	}
	if err := queue.WriteTexture(
		&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
		upload,
		&webgpu.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: uint32(th)},        //nolint:gosec
		&webgpu.Extent3D{Width: uint32(tw), Height: uint32(th), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		view.Release()
		tex.Release()
		return nil
	}
	rc.baseLayer = &GPUTextureDrawCommand{
		View:           gpucontext.NewTextureView(unsafe.Pointer(view)), //nolint:gosec
		DstX:           0,
		DstY:           0,
		DstW:           float32(tw),
		DstH:           float32(th),
		Opacity:        1,
		ViewportWidth:  uint32(tw), //nolint:gosec
		ViewportHeight: uint32(th), //nolint:gosec
	}
	return func() {
		view.Release()
		tex.Release()
	}
}

// Flush dispatches all pending commands for this context via the render session.
func (rc *GPURenderContext) Flush(target render.GPURenderTarget) error { //nolint:cyclop,gocognit,gocyclo,funlen // sequential resource setup + group dispatch
	pending := rc.PendingCount()
	if pending == 0 {
		// rasterAtlas: CPU shapes already in pixmap, upload to offscreen texture.
		if !target.View.IsNil() && rc.shared.strategy == strategyRasterAtlas {
			return rc.uploadPixmapToView(target)
		}
		return rc.flushVello(target)
	}

	rc.shared.mu.Lock()
	// Lazy GPU initialization.
	if rc.shared.device == nil {
		if err := rc.shared.ensureGPU(); err != nil {
			rc.shared.mu.Unlock()
			slogger().Warn("GPU init failed, using CPU fallback", "err", err)
			return render.ErrFallbackToCPU
		}
	}
	rc.shared.ensurePipelines()

	device := rc.shared.device
	queue := rc.shared.queue
	sdfPipeline := rc.shared.sdfRenderPipeline
	convexRend := rc.shared.convexRenderer
	stencilRend := rc.shared.stencilRenderer
	textEng := rc.shared.textEngine
	glyphEng := rc.shared.glyphMaskEngine
	rc.shared.mu.Unlock()

	// Ensure session exists with all renderers.
	if rc.session == nil {
		rc.session = NewGPURenderSession(device, queue, rc.shared.SampleCount())
		rc.session.SetSDFPipeline(sdfPipeline)
		rc.session.SetConvexRenderer(convexRend)
		rc.session.SetStencilRenderer(stencilRend)
	}

	// Propagate per-frame anti-aliasing state to session.
	rc.session.antiAlias = rc.antiAlias

	// Transfer per-context frame tracking to session before rendering.
	rc.session.SetFrameState(rc.frameRendered, rc.lastView)

	// Build scissor groups from the timeline.
	// S6.2: groups reference pending command slices directly (no deep-copy).
	// Pending queues are reset only AFTER atlas sync + RenderFrameGrouped so
	// sub-slices remain valid for the whole encode/submit path.
	groups := rc.buildScissorGroups()
	rc.hasPendingTarget = false
	rc.sceneStats = render.SceneStats{}

	// Collect all text and glyph mask batches for atlas sync without extra copies
	// when a single group owns the full pending text/glyph queues.
	var allTextBatches []TextBatch
	var allGlyphMaskBatches []GlyphMaskBatch
	if len(groups) == 1 {
		allTextBatches = groups[0].TextBatches
		allGlyphMaskBatches = groups[0].GlyphMaskBatches
	} else {
		for i := range groups {
			allTextBatches = append(allTextBatches, groups[i].TextBatches...)
			allGlyphMaskBatches = append(allGlyphMaskBatches, groups[i].GlyphMaskBatches...)
		}
	}

	// Upload dirty MSDF atlases to the GPU before rendering text.
	if len(allTextBatches) > 0 && textEng != nil {
		rc.shared.mu.Lock()
		err := rc.syncTextAtlases()
		rc.shared.mu.Unlock()
		if err != nil {
			slogger().Warn("atlas sync failed", "err", err)
			for i := range groups {
				groups[i].TextBatches = nil
			}
		}
	}

	// Upload dirty glyph mask atlas pages.
	if len(allGlyphMaskBatches) > 0 && glyphEng != nil {
		rc.shared.mu.Lock()
		err := rc.syncGlyphMaskAtlases(allGlyphMaskBatches)
		rc.shared.mu.Unlock()
		if err != nil {
			slogger().Warn("glyph mask atlas sync failed", "err", err)
			for i := range groups {
				groups[i].GlyphMaskBatches = nil
			}
		}
	}

	// Propagate shared atlas texture to this session (may differ from session's own).
	// This ensures offscreen sessions see the atlas even if they didn't sync it.
	if rc.shared.sharedAtlasView != nil {
		rc.session.SetTextAtlasRef(rc.shared.sharedAtlasTex, rc.shared.sharedAtlasView)
	}

	// Propagate glyph mask atlas page views for offscreen sessions.
	// Same pattern as MSDF atlas — engine is shared, views must reach each session.
	if len(allGlyphMaskBatches) > 0 && glyphEng != nil {
		for i, batch := range allGlyphMaskBatches {
			view := glyphEng.PageTextureView(batch.AtlasPageIndex)
			if view != nil {
				rc.session.SetGlyphMaskAtlasView(i, view, batch.IsLCD)
			}
		}
	}

	// Two-pass LCD needs real destination colors on the GPU. On the first
	// frame LoadOpClear is transparent; upload the CPU pixmap (ClearWithColor
	// background) as base layer so darken+add samples correct dest.
	hasLCDBatch := false
	for i := range allGlyphMaskBatches {
		if allGlyphMaskBatches[i].IsLCD {
			hasLCDBatch = true
			break
		}
	}
	lcdDestRelease := rc.ensureLCDDestBase(target, hasLCDBatch)
	if lcdDestRelease != nil {
		defer lcdDestRelease()
	}

	baseLayer := rc.baseLayer
	rc.baseLayer = nil

	// L.06: bind full-surface R8 mask for convex cover-inline sampling when active.
	if err := rc.session.PrepareFrameMask(rc.shared); err != nil {
		slogger().Warn("prepare frame mask failed", "err", err)
	}

	err := rc.session.RenderFrameGrouped(target, groups, baseLayer, rc.sharedEncoder)
	if err != nil {
		total := 0
		for i := range groups {
			total += len(groups[i].SDFShapes) + len(groups[i].ConvexCommands) + len(groups[i].StencilPaths) +
				len(groups[i].ImageCommands) + len(groups[i].TextBatches) + len(groups[i].GlyphMaskBatches)
		}
		slogger().Warn("render session error",
			"groups", len(groups), "totalItems", total, "err", err)
	}

	// S6.2: drop pending command ownership after encode/submit consumed the slices.
	clear(rc.pendingShapes)
	clear(rc.pendingConvexCommands)
	clear(rc.pendingStencilPaths)
	clear(rc.pendingImageCommands)
	clear(rc.pendingGPUTextureCommands)
	clear(rc.pendingTextBatches)
	clear(rc.pendingGlyphMaskBatches)
	clear(rc.scissorSegments)
	rc.pendingShapes = rc.pendingShapes[:0]
	rc.pendingConvexCommands = rc.pendingConvexCommands[:0]
	rc.convexMeshPts = rc.convexMeshPts[:0]
	rc.convexMeshVCs = rc.convexMeshVCs[:0]
	rc.pendingStencilPaths = rc.pendingStencilPaths[:0]
	rc.pendingImageCommands = rc.pendingImageCommands[:0]
	rc.pendingGPUTextureCommands = rc.pendingGPUTextureCommands[:0]
	rc.pendingTextBatches = rc.pendingTextBatches[:0]
	rc.pendingGlyphMaskBatches = rc.pendingGlyphMaskBatches[:0]
	rc.scissorSegments = rc.scissorSegments[:0]

	// Read back frame tracking from session.
	rc.frameRendered, rc.lastView = rc.session.FrameState()

	return err
}

// flushVello flushes Vello compute if it has pending paths and the effective
// pipeline mode is Compute. In Auto mode with few shapes, SelectPipeline
// returns RenderPass — Vello paths would be lost. Guard ensures Vello is
// flushed only when the mode actually routes paths there.
func (rc *GPURenderContext) flushVello(target render.GPURenderTarget) error {
	effectiveMode := rc.effectivePipelineMode()
	rc.shared.mu.Lock()
	va := rc.shared.velloAccel
	rc.shared.mu.Unlock()
	if va != nil && va.PendingCount() > 0 && effectiveMode == render.PipelineModeCompute {
		if err := va.Flush(target); err != nil {
			slogger().Debug("vello compute flush failed", "err", err)
		}
	}
	rc.sceneStats = render.SceneStats{}
	return nil
}

// uploadPixmapToView uploads CPU-rasterized pixmap content to an offscreen
// GPU texture on rasterAtlas strategy. Skia Graphite pattern: shapes are
// CPU-rasterized, then uploaded via WriteTexture (no render pass needed).
func (rc *GPURenderContext) uploadPixmapToView(target render.GPURenderTarget) error {
	rc.sceneStats = render.SceneStats{}

	queue := rc.shared.Queue()
	if queue == nil || len(target.Data) == 0 {
		return nil
	}

	wgpuView := (*webgpu.TextureView)(target.View.Pointer())
	if wgpuView == nil {
		return nil
	}
	tex := wgpuView.Texture()
	if tex == nil {
		return nil
	}

	w, h := uint32(target.Width), uint32(target.Height) //nolint:gosec // bounded by pixmap

	// Pixmap is RGBA, offscreen texture is BGRA8Unorm — swizzle R↔B.
	bgra := make([]byte, len(target.Data))
	for i := 0; i < len(target.Data); i += 4 {
		bgra[i+0] = target.Data[i+2]
		bgra[i+1] = target.Data[i+1]
		bgra[i+2] = target.Data[i+0]
		bgra[i+3] = target.Data[i+3]
	}

	return queue.WriteTexture(
		&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
		bgra,
		&webgpu.ImageDataLayout{BytesPerRow: w * 4, RowsPerImage: h},
		&webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	)
}

// effectivePipelineMode determines the actual mode for this flush.
func (rc *GPURenderContext) effectivePipelineMode() render.PipelineMode {
	mode := rc.pipelineMode
	if mode == render.PipelineModeAuto {
		rc.shared.mu.Lock()
		hasCompute := rc.shared.velloAccel != nil && rc.shared.velloAccel.CanCompute()
		rc.shared.mu.Unlock()
		mode = render.SelectPipeline(rc.sceneStats, hasCompute)
	}
	return mode
}

// CreateOffscreenTexture allocates a GPU texture for offscreen rendering.
// Checks deviceReady (not gpuReady): texture allocation needs a live device,
// not shape pipelines. On rasterAtlas, gpuReady is false but device is alive.
// Skia Graphite: TextureProxy::Make() works under kRasterAtlas.
func (rc *GPURenderContext) CreateOffscreenTexture(w, h int) (gpucontext.TextureView, func()) {
	if rc.shared == nil {
		slogger().Warn("CreateOffscreenTexture: shared is nil")
		return gpucontext.TextureView{}, nil
	}
	if !rc.shared.deviceReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil {
			slogger().Warn("CreateOffscreenTexture: ensureGPU failed", "error", err)
			return gpucontext.TextureView{}, nil
		}
		if !rc.shared.deviceReady {
			slogger().Warn("CreateOffscreenTexture: device not ready after ensureGPU")
			return gpucontext.TextureView{}, nil
		}
	}
	device := rc.shared.Device()
	if device == nil {
		slogger().Warn("CreateOffscreenTexture: device is nil")
		return gpucontext.TextureView{}, nil
	}

	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "offscreen_cache",
		Size:          webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec // bounded
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageCopyDst | types.TextureUsageTextureBinding,
	})
	if err != nil {
		slogger().Warn("CreateOffscreenTexture: CreateTexture failed",
			"error", err, "width", w, "height", h,
			"format", "BGRA8Unorm", "usage", "RenderAttachment|CopySrc|TextureBinding")
		return gpucontext.TextureView{}, nil
	}

	view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label:         "offscreen_cache_view",
		Format:        types.TextureFormatBGRA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		slogger().Warn("CreateOffscreenTexture: CreateTextureView failed",
			"error", err, "width", w, "height", h)
		tex.Release()
		return gpucontext.TextureView{}, nil
	}

	release := func() {
		view.Release()
		tex.Release()
	}
	return gpucontext.NewTextureView(unsafe.Pointer(view)), release //nolint:gosec // Go spec Rule 1 (ADR-018)
}

// Close releases this context's GPU resources. Shared resources are NOT
// released — they are owned by GPUShared.
func (rc *GPURenderContext) Close() {
	if rc.session != nil {
		rc.session.Destroy()
		rc.session = nil
	}
	rc.pendingShapes = nil
	rc.pendingConvexCommands = nil
	rc.pendingStencilPaths = nil
	rc.pendingImageCommands = nil
	rc.pendingGPUTextureCommands = nil
	rc.baseLayer = nil
	rc.pendingTextBatches = nil
	rc.pendingGlyphMaskBatches = nil
	rc.hasPendingTarget = false
	rc.clipRect = nil
	rc.clipRRect = nil
	rc.clipPath = nil
	rc.scissorSegments = nil
	rc.scissorGroups = nil
	rc.scissorRects = nil
	rc.scissorClips = nil
	rc.textBatchSealed = false
	rc.glyphBatchSealed = false
	rc.sceneStats = render.SceneStats{}
}

// recordScissorSegment records a scissor state change in the timeline.
// It seals both text tiers so the next QueueGlyphMask/QueueText starts
// a new batch instead of merging across the scissor boundary.
func (rc *GPURenderContext) recordScissorSegment(rect *[4]uint32) {
	rc.textBatchSealed = true
	rc.glyphBatchSealed = true
	seg := scissorSegment{
		sdfCount:     len(rc.pendingShapes),
		convexCount:  len(rc.pendingConvexCommands),
		stencilCount: len(rc.pendingStencilPaths),
		imageCount:   len(rc.pendingImageCommands),
		gpuTexCount:  len(rc.pendingGPUTextureCommands),
		textCount:    len(rc.pendingTextBatches),
		glyphCount:   len(rc.pendingGlyphMaskBatches),
	}
	if rect != nil {
		seg.rect = *rect
		seg.hasRect = true
	}
	if rc.clipRRect != nil {
		seg.clipRRect = *rc.clipRRect
		seg.hasClipRRect = true
	}
	seg.clipPath = rc.clipPath
	rc.scissorSegments = append(rc.scissorSegments, seg)
}

// buildScissorGroups builds scissor groups from the pending commands and timeline.
func (rc *GPURenderContext) buildScissorGroups() []ScissorGroup {
	needed := len(rc.scissorSegments) + 1
	if cap(rc.scissorGroups) < needed {
		rc.scissorGroups = make([]ScissorGroup, 0, needed)
	}
	if cap(rc.scissorRects) < needed {
		rc.scissorRects = make([][4]uint32, 0, needed)
	}
	if cap(rc.scissorClips) < needed {
		rc.scissorClips = make([]ClipParams, 0, needed)
	}
	clear(rc.scissorGroups)
	clear(rc.scissorRects)
	clear(rc.scissorClips)
	groups := rc.scissorGroups[:0]
	rects := rc.scissorRects[:0]
	clips := rc.scissorClips[:0]

	if len(rc.scissorSegments) == 0 {
		groups = append(groups, ScissorGroup{
			Rect:               nil,
			SDFShapes:          rc.pendingShapes,
			ConvexCommands:     rc.pendingConvexCommands,
			StencilPaths:       rc.pendingStencilPaths,
			ImageCommands:      rc.pendingImageCommands,
			GPUTextureCommands: rc.pendingGPUTextureCommands,
			TextBatches:        rc.pendingTextBatches,
			GlyphMaskBatches:   rc.pendingGlyphMaskBatches,
		})
		rc.scissorGroups = groups
		rc.scissorRects = rects
		rc.scissorClips = clips
		return groups
	}

	firstSeg := rc.scissorSegments[0]
	if firstSeg.sdfCount > 0 || firstSeg.convexCount > 0 || firstSeg.stencilCount > 0 ||
		firstSeg.imageCount > 0 || firstSeg.gpuTexCount > 0 || firstSeg.textCount > 0 || firstSeg.glyphCount > 0 {
		groups = append(groups, ScissorGroup{
			Rect:               nil,
			SDFShapes:          rc.pendingShapes[:firstSeg.sdfCount],
			ConvexCommands:     rc.pendingConvexCommands[:firstSeg.convexCount],
			StencilPaths:       rc.pendingStencilPaths[:firstSeg.stencilCount],
			ImageCommands:      rc.pendingImageCommands[:firstSeg.imageCount],
			GPUTextureCommands: rc.pendingGPUTextureCommands[:firstSeg.gpuTexCount],
			TextBatches:        rc.pendingTextBatches[:firstSeg.textCount],
			GlyphMaskBatches:   rc.pendingGlyphMaskBatches[:firstSeg.glyphCount],
		})
	}

	for i, seg := range rc.scissorSegments {
		var endSDF, endConvex, endStencil, endImage, endGPUTex, endText, endGlyph int
		if i+1 < len(rc.scissorSegments) {
			next := rc.scissorSegments[i+1]
			endSDF = next.sdfCount
			endConvex = next.convexCount
			endStencil = next.stencilCount
			endImage = next.imageCount
			endGPUTex = next.gpuTexCount
			endText = next.textCount
			endGlyph = next.glyphCount
		} else {
			endSDF = len(rc.pendingShapes)
			endConvex = len(rc.pendingConvexCommands)
			endStencil = len(rc.pendingStencilPaths)
			endImage = len(rc.pendingImageCommands)
			endGPUTex = len(rc.pendingGPUTextureCommands)
			endText = len(rc.pendingTextBatches)
			endGlyph = len(rc.pendingGlyphMaskBatches)
		}

		if seg.sdfCount == endSDF && seg.convexCount == endConvex &&
			seg.stencilCount == endStencil && seg.imageCount == endImage &&
			seg.gpuTexCount == endGPUTex && seg.textCount == endText && seg.glyphCount == endGlyph {
			continue
		}

		var groupRect *[4]uint32
		if seg.hasRect {
			rects = append(rects, seg.rect)
			groupRect = &rects[len(rects)-1]
		}
		var groupClip *ClipParams
		if seg.hasClipRRect {
			clips = append(clips, seg.clipRRect)
			groupClip = &clips[len(clips)-1]
		}
		groups = append(groups, ScissorGroup{
			Rect:               groupRect,
			ClipRRect:          groupClip,
			ClipPath:           seg.clipPath,
			SDFShapes:          rc.pendingShapes[seg.sdfCount:endSDF],
			ConvexCommands:     rc.pendingConvexCommands[seg.convexCount:endConvex],
			StencilPaths:       rc.pendingStencilPaths[seg.stencilCount:endStencil],
			ImageCommands:      rc.pendingImageCommands[seg.imageCount:endImage],
			GPUTextureCommands: rc.pendingGPUTextureCommands[seg.gpuTexCount:endGPUTex],
			TextBatches:        rc.pendingTextBatches[seg.textCount:endText],
			GlyphMaskBatches:   rc.pendingGlyphMaskBatches[seg.glyphCount:endGlyph],
		})
	}

	rc.scissorGroups = groups
	rc.scissorRects = rects
	rc.scissorClips = clips
	return groups
}

// syncTextAtlases uploads dirty MSDF atlas pages. Must be called with shared.mu held.
func (rc *GPURenderContext) syncTextAtlases() error {
	s := rc.shared
	dirtyIndices := s.textEngine.DirtyAtlases()
	if len(dirtyIndices) == 0 {
		return nil
	}

	for _, idx := range dirtyIndices {
		rgbaData, size, _ := s.textEngine.AtlasRGBAData(idx)
		if rgbaData == nil || size == 0 {
			continue
		}

		atlasSize := uint32(size) //nolint:gosec // atlas size always fits uint32

		tex, err := s.device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         fmt.Sprintf("msdf_atlas_%d", idx),
			Size:          webgpu.Extent3D{Width: atlasSize, Height: atlasSize, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatRGBA8Unorm,
			Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
		})
		if err != nil {
			return fmt.Errorf("create atlas texture %d: %w", idx, err)
		}

		view, err := s.device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Label:         fmt.Sprintf("msdf_atlas_%d_view", idx),
			Format:        types.TextureFormatRGBA8Unorm,
			Dimension:     types.TextureViewDimension2D,
			Aspect:        types.TextureAspectAll,
			MipLevelCount: 1,
		})
		if err != nil {
			tex.Release()
			return fmt.Errorf("create atlas texture view %d: %w", idx, err)
		}

		if err := s.queue.WriteTexture(
			&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
			rgbaData,
			&webgpu.ImageDataLayout{
				Offset:       0,
				BytesPerRow:  atlasSize * 4,
				RowsPerImage: atlasSize,
			},
			&webgpu.Extent3D{Width: atlasSize, Height: atlasSize, DepthOrArrayLayers: 1},
		); err != nil {
			tex.Release()
			return fmt.Errorf("upload atlas texture %d: %w", idx, err)
		}

		// Store atlas in GPUShared (shared across all contexts).
		if s.sharedAtlasView != nil {
			s.sharedAtlasView.Release()
		}
		if s.sharedAtlasTex != nil {
			s.sharedAtlasTex.Release()
		}
		s.sharedAtlasTex = tex
		s.sharedAtlasView = view
		s.textEngine.MarkClean(idx)
	}
	return nil
}

// syncGlyphMaskAtlases uploads dirty R8 atlas pages. Must be called with shared.mu held.
func (rc *GPURenderContext) syncGlyphMaskAtlases(batches []GlyphMaskBatch) error {
	s := rc.shared
	if err := s.glyphMaskEngine.SyncAtlasTextures(s.device, s.queue); err != nil {
		return err
	}

	hasLCD := false
	for i := range batches {
		if batches[i].IsLCD {
			hasLCD = true
			break
		}
	}

	if err := rc.session.ensureGlyphMaskPipeline(hasLCD); err != nil {
		return err
	}

	for i, batch := range batches {
		view := s.glyphMaskEngine.PageTextureView(batch.AtlasPageIndex)
		if view == nil {
			slogger().Warn("glyph mask atlas page not synced — text skipped",
				"pageIndex", batch.AtlasPageIndex, "batchIndex", i, "quads", len(batch.Quads))
			continue
		}
		rc.session.SetGlyphMaskAtlasView(i, view, batch.IsLCD)
	}
	return nil
}

// tryFillMaskedConvexInline routes solid convex fills through the convex cover
// pipeline with L.06 R8 mask sampling in the fragment shader (true cover-inline).
// Returns true when the draw was queued (caller must not fall back).
func (rc *GPURenderContext) tryFillMaskedConvexInline(target render.GPURenderTarget, path *render.Path, paint *render.Paint) bool {
	if path == nil || paint == nil || paint.MaskCoverage == nil {
		return false
	}
	if !isGPUSolidPaint(paint) || !paintUsesSourceOver(paint) {
		return false
	}
	if paint.FillRule == render.FillRuleEvenOdd {
		return false
	}
	if !rc.shared.HasGPUMask() {
		return false
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return false
		}
	}
	points, ok := extractConvexPolygon(path)
	if !ok {
		return false
	}
	color := getColorFromPaint(paint)
	cmd := ConvexDrawCommand{
		Points: points,
		Color: [4]float32{
			float32(color.R * color.A),
			float32(color.G * color.A),
			float32(color.B * color.A),
			float32(color.A),
		},
		BlendMode: paintBlendMode(paint),
	}
	rc.QueueConvex(target, cmd)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return true
}

// tryFillMaskedSDFInline routes solid SDF shapes through the SDF cover pipeline
// with L.06 R8 mask sampling in the fragment shader (true cover-inline).
func (rc *GPURenderContext) tryFillMaskedSDFInline(target render.GPURenderTarget, shape render.DetectedShape, paint *render.Paint) bool {
	if paint == nil || paint.MaskCoverage == nil {
		return false
	}
	if !isGPUSolidPaint(paint) || !paintUsesSourceOver(paint) {
		return false
	}
	if !rc.shared.HasGPUMask() {
		return false
	}
	switch shape.Kind {
	case render.ShapeCircle, render.ShapeEllipse, render.ShapeRect, render.ShapeRRect:
	default:
		return false
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return false
		}
	}
	if err := rc.QueueShape(target, shape, paint, false); err != nil {
		return false
	}
	return true
}
