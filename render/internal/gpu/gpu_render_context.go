//go:build !nogpu

package gpu

import (
	"fmt"
	"image"
	"math"
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

	// deviceGen is GPUShared.deviceGen at last session (re)build.
	deviceGen uint64

	// Per-context render session (owns frame textures: MSAA, depth, resolve).
	session *GPURenderSession

	// Per-context pending command queues.
	pendingShapes         []SDFRenderShape
	pendingConvexCommands []ConvexDrawCommand
	// Scratch backing for QueueColoredMesh (avoid per-triangle heap allocs).
	// Slices in pending ConvexDrawCommand point into these until Flush clears.
	convexMeshPts []render.Point
	convexMeshVCs [][4]float32
	// Pre-packed GPU verts (opt19): TriangleList mesh packs once at Queue.
	convexMeshPacked []byte
	// opt22: uint16 index scratch for indexed mesh commands (DrawIndexed).
	convexMeshIdx             []uint16
	pendingStencilPaths       []StencilPathCommand
	pendingImageCommands      []ImageDrawCommand
	pendingGPUTextureCommands []GPUTextureDrawCommand
	// Brush cover results retained until after Flush (no-readback N1/N2).
	pendingBrushCoverResults []brushCoverResult
	pendingTextBatches       []TextBatch
	pendingGlyphMaskBatches  []GlyphMaskBatch
	// Owned backing store for glyph mask quads (LayoutText may reuse scratch).
	glyphMaskQuadStore []GlyphMaskQuad
	baseLayer          *GPUTextureDrawCommand
	pendingTarget      render.GPURenderTarget
	hasPendingTarget   bool
	// presentStash holds View-nil parent draws deferred across layer RT work (F1).
	// Avoids mid-frame offscreen Flush of the whole base scene when entering a layer.
	presentStash presentPendingStash

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

	// Set when Flush absorbed pending into target.Data (View-nil). Context
	// ApplyImageFilterGraph must re-seed from pixmap afterward (Multiply+filter).
	flushedPendingToData bool
	lastView             *webgpu.TextureView

	// Per-context scene stats (for Auto pipeline mode).
	sceneStats render.SceneStats

	// G.04 / residual brush bootstrap diagnostic (ColorAt stage + GPU blit).
	brushBootstrapReason string
	// lastBrushPathKind: session-inline (true GPU) vs GPU* bootstrap (v3.9).
	lastBrushPathKind brushPathKind
	pipelineMode      render.PipelineMode

	// Anti-aliasing state for GPU rendering (propagated from Context).
	antiAlias bool
	// preferSampleCount1 forces the per-context session to use 1x samples.
	// Effect RTs (glow/filter offscreens) do not need 4x MSAA resolve every frame.
	preferSampleCount1 bool

	// Shared command encoder for single-command-buffer frames (ADR-017).
	// When set, Flush records render passes into this encoder instead of
	// creating its own + submitting. The caller owns Finish + Submit.
	sharedEncoder *webgpu.CommandEncoder

	// Deferred advanced-blend layer pops (Multiply/Screen/…). PopLayer during
	// draw often has no surface View yet (present View arrives at
	// PresentFrameFull). We keep the layer RT and dual-tex at Flush time when
	// dest View exists — zero mid-draw PollWait readback.
	pendingAdvancedLayers []pendingAdvancedLayer

	// frameScratch is a BGRA offscreen with CopySrc|TextureBinding used when
	// advanced blends must sample dest. Swapchain textures are RENDER_ATTACHMENT
	// only and cannot be dual-tex sources.
	frameScratchTex  *webgpu.Texture
	frameScratchView *webgpu.TextureView
	frameScratchW    int
	frameScratchH    int
	layerReleaseHold []func()
	// opt42: dual-tex resolve scratch (avoid per-resolve make on L3 blend path).
	dualTexViewOpsScratch []dualTexViewBlendOp

	// Pool of offscreen layer RTs by size to avoid per-PushLayer alloc/OOM.
	offscreenPool map[[2]int][]offscreenPooled
}

type offscreenPooled struct {
	tex  *webgpu.Texture
	view *webgpu.TextureView
}

// pendingAdvancedLayer is a PopLayer advanced blend resolved at Flush.
type pendingAdvancedLayer struct {
	srcView gpucontext.TextureView
	srcW    int
	srcH    int
	damage  image.Rectangle
	mode    render.BlendMode
	opacity float64
	release func()
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

// SetPreferSampleCount1 forces 1x MSAA for this context's GPU session.
// Used by continuous effect offscreens (glow/blur) where 4x resolve dominates cost.
func (rc *GPURenderContext) SetPreferSampleCount1(enabled bool) {
	if rc == nil {
		return
	}
	if rc.preferSampleCount1 == enabled {
		return
	}
	rc.preferSampleCount1 = enabled
	// Recreate session on next flush with the new sample count.
	if rc.session != nil {
		rc.session.Destroy()
		rc.session = nil
	}
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

// drainLayerReleaseHold frees deferred advanced-layer RT releases.
func (rc *GPURenderContext) drainLayerReleaseHold() {
	if rc == nil {
		return
	}
	for _, rel := range rc.layerReleaseHold {
		if rel != nil {
			rel()
		}
	}
	rc.layerReleaseHold = rc.layerReleaseHold[:0]
}

// BeginFrame resets per-frame state so the first render pass clears the surface.
// HasPresentView reports whether this context is bound to a window/surface
// present path (non-nil last surface view). Offscreen unit tests use View-nil
// FlushGPU and should prefer immediate advanced-layer CPU composite (D05).
func (rc *GPURenderContext) HasPresentView() bool {
	if rc == nil {
		return false
	}
	return rc.lastView != nil
}

func (rc *GPURenderContext) MidFrameDataFlush() bool {
	return rc != nil && rc.flushedPendingToData
}

func (rc *GPURenderContext) ClearMidFrameDataFlush() {
	if rc != nil {
		rc.flushedPendingToData = false
	}
}

// DigCmdBufStats exposes session CB retain stats for mem digs.
func (rc *GPURenderContext) DigCmdBufStats() (prev int, usedSurface bool) {
	if rc == nil || rc.session == nil {
		return -1, false
	}
	return rc.session.DigCmdBufStats()
}

func (rc *GPURenderContext) BeginFrame() {
	// Free previous frame command buffers (prevCmdBufs). Without this, windowed
	// PresentFrame paths never call SetSurfaceTarget, so CBs accumulate every
	// frame and native RSS climbs (~hundreds of KB/s on PKS solid_only).
	if rc.session != nil {
		rc.session.BeginFrame()
	}
	rc.clipRect = nil
	rc.clipPath = nil
	rc.frameRendered = false
	rc.lastView = nil
	rc.textBatchSealed = false
	rc.glyphBatchSealed = false
	// Drop any advanced layers not resolved last frame (avoid VRAM leak).
	if len(rc.pendingAdvancedLayers) > 0 {
		for i := range rc.pendingAdvancedLayers {
			if rc.pendingAdvancedLayers[i].release != nil {
				rc.pendingAdvancedLayers[i].release()
			}
		}
		rc.pendingAdvancedLayers = rc.pendingAdvancedLayers[:0]
	}
	for _, rel := range rc.layerReleaseHold {
		if rel != nil {
			rel()
		}
	}
	rc.layerReleaseHold = rc.layerReleaseHold[:0]
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
	// Retain until next session.BeginFrame (matches finishSurfaceSubmit).
	rc.session.prevCmdBufs = append(rc.session.prevCmdBufs, cmdBuf)
	return nil
}

// SceneStats returns the accumulated scene statistics for this context.
func (rc *GPURenderContext) SceneStats() render.SceneStats {
	return rc.sceneStats
}

// brushPathKind classifies the last successful non-solid brush fill path.
type brushPathKind int

const (
	brushPathNone brushPathKind = iota
	// brushPathSessionInline: main-pass stencil+cover (true GPU, not bootstrap).
	brushPathSessionInline
	// brushPathGPUStar: retain/readback/field×R8/ColorAt stage (GPU* bootstrap).
	brushPathGPUStar
)

func (rc *GPURenderContext) markBrushSessionInline() {
	if rc != nil {
		rc.lastBrushPathKind = brushPathSessionInline
	}
}

func (rc *GPURenderContext) markBrushGPUStar() {
	if rc != nil {
		rc.lastBrushPathKind = brushPathGPUStar
	}
}

// noteBrushBootstrapIfGPUStar records bootstrap only for GPU* paths, not
// session-inline native covers (v3.9). Clears lastBrushPathKind.
func (rc *GPURenderContext) noteBrushBootstrapIfGPUStar(reason string) {
	if rc == nil {
		return
	}
	if rc.lastBrushPathKind == brushPathGPUStar {
		rc.noteBrushBootstrap(reason)
	}
	rc.lastBrushPathKind = brushPathNone
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

// presentPendingStash stores present-deferred GPU commands (no surface View yet).
type presentPendingStash struct {
	active        bool
	shapes        []SDFRenderShape
	convex        []ConvexDrawCommand
	convexMeshPts []render.Point
	convexMeshVCs [][4]float32
	// opt22: owned copies of PackedVerts / Indices so parent mesh survives
	// layer Queue overwriting rc.convexMeshPacked.
	convexMeshPacked []byte
	convexMeshIdx    []uint16
	stencil          []StencilPathCommand
	images           []ImageDrawCommand
	gpuTex           []GPUTextureDrawCommand
	text             []TextBatch
	glyph            []GlyphMaskBatch
	scissorSegments  []scissorSegment
	baseLayer        *GPUTextureDrawCommand
}

// relocateConvexMeshData copies PackedVerts/Indices into dstPacked/dstIdx and
// rewrites command slice headers to point at the destination (stash or rc).
func relocateConvexMeshData(cmds []ConvexDrawCommand, from int, dstPacked *[]byte, dstIdx *[]uint16) {
	for i := from; i < len(cmds); i++ {
		cmd := &cmds[i]
		if n := len(cmd.PackedVerts); n > 0 {
			off := len(*dstPacked)
			*dstPacked = append(*dstPacked, cmd.PackedVerts...)
			cmd.PackedVerts = (*dstPacked)[off : off+n : off+n]
		}
		if n := len(cmd.Indices); n > 0 {
			off := len(*dstIdx)
			*dstIdx = append(*dstIdx, cmd.Indices...)
			cmd.Indices = (*dstIdx)[off : off+n : off+n]
		}
	}
}

func (rc *GPURenderContext) stashPresentPending() {
	if rc == nil || !rc.hasPendingTarget {
		return
	}
	// Merge into existing stash (multiple layer enter/exit within a frame).
	// Scissor segment counts are absolute indices into the pending queues — when
	// merging a second parent batch, re-base the new segments by the already-
	// stashed command lengths so buildScissorGroups never sees inverted ranges.
	s := &rc.presentStash
	baseSDF := len(s.shapes)
	baseConvex := len(s.convex)
	baseStencil := len(s.stencil)
	baseImage := len(s.images)
	baseGPUTex := len(s.gpuTex)
	baseText := len(s.text)
	baseGlyph := len(s.glyph)

	s.shapes = append(s.shapes, rc.pendingShapes...)
	s.convex = append(s.convex, rc.pendingConvexCommands...)
	// opt22: deep-copy packed mesh bytes/indices into stash-owned storage so
	// subsequent layer QueueColoredMesh* cannot overwrite parent PackedVerts.
	// baseConvex is the pre-append length (declared above with scissor bases).
	relocateConvexMeshData(s.convex, baseConvex, &s.convexMeshPacked, &s.convexMeshIdx)
	s.convexMeshPts = append(s.convexMeshPts, rc.convexMeshPts...)
	s.convexMeshVCs = append(s.convexMeshVCs, rc.convexMeshVCs...)
	s.stencil = append(s.stencil, rc.pendingStencilPaths...)
	s.images = append(s.images, rc.pendingImageCommands...)
	s.gpuTex = append(s.gpuTex, rc.pendingGPUTextureCommands...)
	s.text = append(s.text, rc.pendingTextBatches...)
	s.glyph = append(s.glyph, rc.pendingGlyphMaskBatches...)
	if len(rc.scissorSegments) > 0 {
		if baseSDF|baseConvex|baseStencil|baseImage|baseGPUTex|baseText|baseGlyph == 0 {
			s.scissorSegments = append(s.scissorSegments, rc.scissorSegments...)
		} else {
			for i := range rc.scissorSegments {
				seg := rc.scissorSegments[i]
				seg.sdfCount += baseSDF
				seg.convexCount += baseConvex
				seg.stencilCount += baseStencil
				seg.imageCount += baseImage
				seg.gpuTexCount += baseGPUTex
				seg.textCount += baseText
				seg.glyphCount += baseGlyph
				s.scissorSegments = append(s.scissorSegments, seg)
			}
		}
	}
	if s.baseLayer == nil {
		s.baseLayer = rc.baseLayer
	}
	s.active = true

	rc.pendingShapes = rc.pendingShapes[:0]
	rc.pendingConvexCommands = rc.pendingConvexCommands[:0]
	rc.convexMeshPts = rc.convexMeshPts[:0]
	rc.convexMeshVCs = rc.convexMeshVCs[:0]
	rc.convexMeshPacked = rc.convexMeshPacked[:0]
	rc.convexMeshIdx = rc.convexMeshIdx[:0]
	rc.pendingStencilPaths = rc.pendingStencilPaths[:0]
	rc.pendingImageCommands = rc.pendingImageCommands[:0]
	rc.pendingGPUTextureCommands = rc.pendingGPUTextureCommands[:0]
	rc.pendingTextBatches = rc.pendingTextBatches[:0]
	rc.pendingGlyphMaskBatches = rc.pendingGlyphMaskBatches[:0]
	rc.glyphMaskQuadStore = rc.glyphMaskQuadStore[:0]
	rc.scissorSegments = rc.scissorSegments[:0]
	rc.baseLayer = nil
	rc.hasPendingTarget = false
	rc.textBatchSealed = true
	rc.glyphBatchSealed = true
}

// prependSlice puts prefix before dst without the double-alloc
// append(append([]T{}, prefix...), dst...) pattern. Reuses dst capacity when
// possible (in-place shift) so present-stash unstash does not thrash the heap
// every frame with full-scene particle/mesh command lists.
func prependSlice[T any](dst, prefix []T) []T {
	if len(prefix) == 0 {
		return dst
	}
	if len(dst) == 0 {
		return append(dst[:0], prefix...)
	}
	need := len(prefix) + len(dst)
	if cap(dst) >= need {
		old := len(dst)
		dst = dst[:need]
		copy(dst[len(prefix):], dst[:old])
		copy(dst[:len(prefix)], prefix)
		return dst
	}
	out := make([]T, need)
	copy(out, prefix)
	copy(out[len(prefix):], dst)
	return out
}

func (rc *GPURenderContext) unstashPresentPending() {
	if rc == nil || !rc.presentStash.active {
		return
	}
	s := &rc.presentStash
	// Stash is earlier in z-order: put it before current pending.
	// Current scissor counts index into the post-stash queues only — shift them
	// by the stashed prefix lengths before concatenating timelines.
	offSDF := len(s.shapes)
	offConvex := len(s.convex)
	offStencil := len(s.stencil)
	offImage := len(s.images)
	offGPUTex := len(s.gpuTex)
	offText := len(s.text)
	offGlyph := len(s.glyph)
	if len(rc.scissorSegments) > 0 && (offSDF|offConvex|offStencil|offImage|offGPUTex|offText|offGlyph) != 0 {
		for i := range rc.scissorSegments {
			rc.scissorSegments[i].sdfCount += offSDF
			rc.scissorSegments[i].convexCount += offConvex
			rc.scissorSegments[i].stencilCount += offStencil
			rc.scissorSegments[i].imageCount += offImage
			rc.scissorSegments[i].gpuTexCount += offGPUTex
			rc.scissorSegments[i].textCount += offText
			rc.scissorSegments[i].glyphCount += offGlyph
		}
	}

	rc.pendingShapes = prependSlice(rc.pendingShapes, s.shapes)
	rc.pendingConvexCommands = prependSlice(rc.pendingConvexCommands, s.convex)
	// Move stash-owned packed mesh into rc scratch and re-point (opt22).
	// Prefix (stashed) commands need relocate from 0..len(s.convex); any
	// already-pending cmds after the prefix keep their own rc packing.
	if len(s.convex) > 0 {
		// First copy all current pending packed after stash prefix into a temp
		// path: relocate entire pending set into a fresh rc buffer.
		newPacked := rc.convexMeshPacked[:0]
		if cap(newPacked) < 1 {
			newPacked = make([]byte, 0, len(s.convexMeshPacked)+256)
		}
		newIdx := rc.convexMeshIdx[:0]
		relocateConvexMeshData(rc.pendingConvexCommands, 0, &newPacked, &newIdx)
		rc.convexMeshPacked = newPacked
		rc.convexMeshIdx = newIdx
	}
	rc.convexMeshPts = prependSlice(rc.convexMeshPts, s.convexMeshPts)
	rc.convexMeshVCs = prependSlice(rc.convexMeshVCs, s.convexMeshVCs)
	rc.pendingStencilPaths = prependSlice(rc.pendingStencilPaths, s.stencil)
	rc.pendingImageCommands = prependSlice(rc.pendingImageCommands, s.images)
	rc.pendingGPUTextureCommands = prependSlice(rc.pendingGPUTextureCommands, s.gpuTex)
	rc.pendingTextBatches = prependSlice(rc.pendingTextBatches, s.text)
	rc.pendingGlyphMaskBatches = prependSlice(rc.pendingGlyphMaskBatches, s.glyph)
	rc.scissorSegments = prependSlice(rc.scissorSegments, s.scissorSegments)
	if rc.baseLayer == nil {
		rc.baseLayer = s.baseLayer
	}
	// Reset stash (keep capacity for next frame).
	s.active = false
	s.shapes = s.shapes[:0]
	s.convex = s.convex[:0]
	s.convexMeshPts = s.convexMeshPts[:0]
	s.convexMeshVCs = s.convexMeshVCs[:0]
	s.convexMeshPacked = s.convexMeshPacked[:0]
	s.convexMeshIdx = s.convexMeshIdx[:0]
	s.stencil = s.stencil[:0]
	s.images = s.images[:0]
	s.gpuTex = s.gpuTex[:0]
	s.text = s.text[:0]
	s.glyph = s.glyph[:0]
	s.scissorSegments = s.scissorSegments[:0]
	s.baseLayer = nil
}

// PrepareTarget is the exported entry for Context.setGPUClipRect / callers that
// must bind the command stream before recording scissor segments.
func (rc *GPURenderContext) PrepareTarget(target render.GPURenderTarget) error {
	return rc.prepareTarget(target)
}

// prepareTarget switches the active GPU target. F1: defer View-nil parent flushes
// by stashing when entering a layer RT (avoids mid-frame full-scene offscreen encode).
func (rc *GPURenderContext) prepareTarget(target render.GPURenderTarget) error {
	if rc == nil {
		return nil
	}
	if rc.hasPendingTarget && !sameTarget(&rc.pendingTarget, &target) {
		if rc.pendingTarget.View.IsNil() && !target.View.IsNil() {
			rc.stashPresentPending()
		} else if err := rc.Flush(rc.pendingTarget); err != nil {
			return err
		}
	}
	rc.pendingTarget = target
	rc.hasPendingTarget = true
	return nil
}

func (rc *GPURenderContext) QueueShape(target render.GPURenderTarget, shape render.DetectedShape, paint *render.Paint, stroked bool) error {
	if err := rc.prepareTarget(target); err != nil {
		return err
	}
	rc.ensureDrawOrder(drawTierSDF)

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
	return nil
}

// QueueConvex accumulates a convex polygon for batch dispatch.
func (rc *GPURenderContext) QueueConvex(target render.GPURenderTarget, cmd ConvexDrawCommand) {
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.ensureDrawOrder(drawTierConvex)
	rc.pendingConvexCommands = append(rc.pendingConvexCommands, cmd)
}

// QueueColoredMesh queues a triangle mesh with optional per-vertex colors
// via the convex fast-path.
// positions are pixel-space points; colors are straight RGBA (premultiplied here).
// triangleList=true groups positions as independent triangles; false = fan.
//
// Hot path (DrawMesh / 3D): ONE TriangleList command for the whole mesh
// (not N tri-commands), backed by reusable scratch. SkipAA solid verts.
//
// opt19: pack GPU vertex bytes once here (PackedVerts). Flush only WriteBuffers
// the pre-packed blob — no second Points/VertexColors → stride walk.
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
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.ensureDrawOrder(drawTierConvex)

	useVC := len(colors) == len(positions)
	need := nOut
	nBytes := need * convexMeshVertexStride

	// Grow-only packed vertex scratch (primary mesh payload, opt33 12B).
	pkBase := len(rc.convexMeshPacked)
	if cap(rc.convexMeshPacked) < pkBase+nBytes {
		capN := (pkBase + nBytes) * 2
		if capN < 512*convexMeshVertexStride {
			capN = 512 * convexMeshVertexStride
		}
		np := make([]byte, pkBase, capN)
		copy(np, rc.convexMeshPacked)
		rc.convexMeshPacked = np
	}
	rc.convexMeshPacked = rc.convexMeshPacked[:pkBase+nBytes]
	pk := rc.convexMeshPacked

	toPremul := func(c render.RGBA) [4]float32 {
		return [4]float32{
			float32(c.R * c.A),
			float32(c.G * c.A),
			float32(c.B * c.A),
			float32(c.A),
		}
	}
	solidColor := [4]float32{0, 0, 0, 1}
	if !useVC && len(colors) > 0 {
		solidColor = toPremul(colors[0])
	} else if !useVC {
		solidColor = [4]float32{0, 0, 0, 1}
	}

	writeAt := func(vi int, p render.Point, col [4]float32) {
		off := pkBase + vi*convexMeshVertexStride
		writeConvexMeshVertex(pk[off:], float32(p.X), float32(p.Y), col)
	}

	if triangleList {
		if useVC {
			for i := 0; i < need; i++ {
				writeAt(i, positions[i], toPremul(colors[i]))
			}
			// Solid fallback = mean of first triangle.
			c0 := toPremul(colors[0])
			c1 := toPremul(colors[1])
			c2 := toPremul(colors[2])
			solidColor = [4]float32{
				(c0[0] + c1[0] + c2[0]) / 3,
				(c0[1] + c1[1] + c2[1]) / 3,
				(c0[2] + c1[2] + c2[2]) / 3,
				(c0[3] + c1[3] + c2[3]) / 3,
			}
		} else {
			for i := 0; i < need; i++ {
				writeAt(i, positions[i], solidColor)
			}
		}
	} else {
		// Fan → triangle list, pack directly.
		o := 0
		for i := 1; i+1 < len(positions); i++ {
			var c0, c1, c2 [4]float32
			if useVC {
				c0, c1, c2 = toPremul(colors[0]), toPremul(colors[i]), toPremul(colors[i+1])
			} else {
				c0, c1, c2 = solidColor, solidColor, solidColor
			}
			writeAt(o+0, positions[0], c0)
			writeAt(o+1, positions[i], c1)
			writeAt(o+2, positions[i+1], c2)
			o += 3
		}
		if useVC {
			c0 := toPremul(colors[0])
			c1 := toPremul(colors[1])
			c2 := toPremul(colors[2])
			solidColor = [4]float32{
				(c0[0] + c1[0] + c2[0]) / 3,
				(c0[1] + c1[1] + c2[1]) / 3,
				(c0[2] + c1[2] + c2[2]) / 3,
				(c0[3] + c1[3] + c2[3]) / 3,
			}
		}
	}

	cmd := ConvexDrawCommand{
		Color:        solidColor,
		SkipAA:       true,
		TriangleList: true,
		PackedVerts:  pk[pkBase : pkBase+nBytes : pkBase+nBytes],
	}
	rc.pendingConvexCommands = append(rc.pendingConvexCommands, cmd)
	rc.pendingTarget = target
	rc.hasPendingTarget = true
	rc.sceneStats.ShapeCount++
}

// QueueColoredMeshIndexed queues a mesh with unique vertices + uint16 indices
// (opt22). Avoids CPU expand of indexed DrawMesh (disc fans etc.) so WriteBuffer
// uploads only unique verts. Indices are copied into convexMeshIdx scratch.
//
// opt23: hot path drops O(n) pre-validation (DrawMesh supplies in-range indices),
// uses a tight pack loop (coverage=1 fixed), and keeps grow-only scratch.
func (rc *GPURenderContext) QueueColoredMeshIndexed(target render.GPURenderTarget, positions []render.Point, colors []render.RGBA, indices []uint16) {
	if len(positions) < 3 || len(indices) < 3 {
		return
	}
	nIdx := len(indices) / 3 * 3
	if nIdx < 3 {
		return
	}
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.ensureDrawOrder(drawTierConvex)

	nVerts := len(positions)
	nBytes := nVerts * convexMeshVertexStride
	pkBase := len(rc.convexMeshPacked)
	if cap(rc.convexMeshPacked) < pkBase+nBytes {
		capN := (pkBase + nBytes) * 2
		if capN < 512*convexMeshVertexStride {
			capN = 512 * convexMeshVertexStride
		}
		np := make([]byte, pkBase, capN)
		copy(np, rc.convexMeshPacked)
		rc.convexMeshPacked = np
	}
	rc.convexMeshPacked = rc.convexMeshPacked[:pkBase+nBytes]
	pk := rc.convexMeshPacked[pkBase : pkBase+nBytes]

	useVC := len(colors) == len(positions)
	solidColor := packMeshVertsCoverage1(pk, positions, colors, useVC)

	// Copy indices into grow-only scratch (owned until Flush / stash relocate).
	idxBase := len(rc.convexMeshIdx)
	if cap(rc.convexMeshIdx) < idxBase+nIdx {
		capN := (idxBase + nIdx) * 2
		if capN < 512 {
			capN = 512
		}
		ni := make([]uint16, idxBase, capN)
		copy(ni, rc.convexMeshIdx)
		rc.convexMeshIdx = ni
	}
	rc.convexMeshIdx = rc.convexMeshIdx[:idxBase+nIdx]
	copy(rc.convexMeshIdx[idxBase:], indices[:nIdx])

	cmd := ConvexDrawCommand{
		Color:        solidColor,
		SkipAA:       true,
		TriangleList: true,
		PackedVerts:  rc.convexMeshPacked[pkBase : pkBase+nBytes : pkBase+nBytes],
		Indices:      rc.convexMeshIdx[idxBase : idxBase+nIdx : idxBase+nIdx],
	}
	rc.pendingConvexCommands = append(rc.pendingConvexCommands, cmd)
	rc.pendingTarget = target
	rc.hasPendingTarget = true
	rc.sceneStats.ShapeCount++
}

// packMeshVertsCoverage1 writes SkipAA mesh verts into dst (opt33: 12B layout).
// dst must be len(positions)*convexMeshVertexStride. Coverage is implicit 1.0
// in vs_mesh. Returns a solid fallback color (mean of first triangle when useVC).
func packMeshVertsCoverage1(dst []byte, positions []render.Point, colors []render.RGBA, useVC bool) [4]float32 {
	n := len(positions)
	solid := [4]float32{0, 0, 0, 1}
	if !useVC {
		if len(colors) > 0 {
			c := colors[0]
			a := float32(c.A)
			solid = [4]float32{float32(c.R) * a, float32(c.G) * a, float32(c.B) * a, a}
		}
		colBits := packColorUnorm8x4RGBA(solid[0], solid[1], solid[2], solid[3])
		for i := 0; i < n; i++ {
			off := i * convexMeshVertexStride
			*(*uint32)(unsafe.Pointer(&dst[off+0])) = math.Float32bits(float32(positions[i].X)) //nolint:gosec
			*(*uint32)(unsafe.Pointer(&dst[off+4])) = math.Float32bits(float32(positions[i].Y)) //nolint:gosec
			*(*uint32)(unsafe.Pointer(&dst[off+8])) = colBits                                   //nolint:gosec
		}
		return solid
	}
	// opt31+opt33+opt40: quant via packColorUnorm8x4RGBA (same bits as inline).
	var s0, s1, s2, s3 float32
	for i := 0; i < n; i++ {
		c := colors[i]
		a := float32(c.A)
		r := float32(c.R) * a
		g := float32(c.G) * a
		b := float32(c.B) * a
		if i < 3 {
			s0 += r
			s1 += g
			s2 += b
			s3 += a
		}
		off := i * convexMeshVertexStride
		*(*uint32)(unsafe.Pointer(&dst[off+0])) = math.Float32bits(float32(positions[i].X)) //nolint:gosec
		*(*uint32)(unsafe.Pointer(&dst[off+4])) = math.Float32bits(float32(positions[i].Y)) //nolint:gosec
		*(*uint32)(unsafe.Pointer(&dst[off+8])) = packColorUnorm8x4RGBA(r, g, b, a)         //nolint:gosec
	}
	if n >= 3 {
		return [4]float32{s0 / 3, s1 / 3, s2 / 3, s3 / 3}
	}
	return solid
}

// QueueStencil accumulates a stencil path for batch dispatch.
func (rc *GPURenderContext) QueueStencil(target render.GPURenderTarget, cmd StencilPathCommand) {
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.ensureDrawOrder(drawTierStencil)
	rc.pendingStencilPaths = append(rc.pendingStencilPaths, cmd)
}

// QueueText accumulates an MSDF text batch for dispatch.
// Adjacent batches with identical visual properties (transform, color, atlas,
// MSDF parameters) are coalesced into a single batch to minimize GPU draw calls (ADR-031).
func (rc *GPURenderContext) QueueText(target render.GPURenderTarget, batch TextBatch) {
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.ensureDrawOrder(drawTierText)
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
	contentDirty bool,
) {
	rc.ensureDrawOrder(drawTierImage)
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
		ContentDirty:   contentDirty,
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
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.pendingImageCommands = append(rc.pendingImageCommands, cmd)
}

// QueueBaseLayer sets the compositor base layer — a textured quad drawn BEFORE
// all tiers in the render pass. Last call wins. Used for CPU pixmap compositing
// in zero-readback rendering (ADR-015, Flutter OffsetLayer pattern).
func (rc *GPURenderContext) QueueBaseLayer(target render.GPURenderTarget, view gpucontext.TextureView,
	dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32,
) {
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.baseLayer = &GPUTextureDrawCommand{
		View: view, DstX: dstX, DstY: dstY, DstW: dstW, DstH: dstH,
		Opacity: opacity, ViewportWidth: vpW, ViewportHeight: vpH,
	}
}

// brushCoverResult owns a per-draw stencil-cover texture until Flush finishes.
type brushCoverResult struct {
	tex  *webgpu.Texture
	view *webgpu.TextureView
	// recycle, if set, returns tex/view to a pool instead of Release.
	recycle func(tex *webgpu.Texture, view *webgpu.TextureView)
}

func (rc *GPURenderContext) retainBrushCoverResult(tex *webgpu.Texture, view *webgpu.TextureView) {
	rc.retainBrushCoverResultRecycle(tex, view, nil)
}

func (rc *GPURenderContext) retainBrushCoverResultRecycle(tex *webgpu.Texture, view *webgpu.TextureView, recycle func(*webgpu.Texture, *webgpu.TextureView)) {
	if rc == nil || tex == nil || view == nil {
		return
	}
	rc.pendingBrushCoverResults = append(rc.pendingBrushCoverResults, brushCoverResult{tex: tex, view: view, recycle: recycle})
}

func (rc *GPURenderContext) releaseBrushCoverResults() {
	if rc == nil {
		return
	}
	for i := range rc.pendingBrushCoverResults {
		r := &rc.pendingBrushCoverResults[i]
		if r.recycle != nil {
			r.recycle(r.tex, r.view)
			r.tex, r.view, r.recycle = nil, nil, nil
			continue
		}
		if r.view != nil {
			r.view.Release()
			r.view = nil
		}
		if r.tex != nil {
			r.tex.Release()
			r.tex = nil
		}
	}
	rc.pendingBrushCoverResults = rc.pendingBrushCoverResults[:0]
}

// queueBrushCoverTexture composites a retained brush cover texture into the
// session via GPU-to-GPU draw (no CPU readback / re-upload).
func (rc *GPURenderContext) queueBrushCoverTexture(
	target render.GPURenderTarget,
	view *webgpu.TextureView,
	x0, y0, x1, y1 float32,
	vpW, vpH uint32,
) {
	if rc == nil || view == nil {
		return
	}
	rc.QueueGPUTextureDraw(target, gpucontext.NewTextureView(unsafe.Pointer(view)), //nolint:gosec
		x0, y0, x1-x0, y1-y0, 1.0, vpW, vpH)
}

// QueueGPUTextureDraw queues a GPU-to-GPU texture compositing command.
// The texture view is sampled directly — zero CPU readback, zero upload.
func (rc *GPURenderContext) QueueGPUTextureDraw(target render.GPURenderTarget, view gpucontext.TextureView,
	dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32,
) {
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.ensureDrawOrder(drawTierGPUTex)
	rc.pendingGPUTextureCommands = append(rc.pendingGPUTextureCommands, GPUTextureDrawCommand{
		View: view, DstX: dstX, DstY: dstY, DstW: dstW, DstH: dstH,
		U0: 0, V0: 0, U1: 1, V1: 1,
		Opacity: opacity, ViewportWidth: vpW, ViewportHeight: vpH,
	})
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// QueueGPUTextureDrawUV is QueueGPUTextureDraw with an explicit source UV rect
// (normalized 0..1). Empty/invalid UV falls back to the full texture.
func (rc *GPURenderContext) QueueGPUTextureDrawUV(target render.GPURenderTarget, view gpucontext.TextureView,
	dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32,
	u0, v0, u1, v1 float32,
) {
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.ensureDrawOrder(drawTierGPUTex)
	if u1 <= u0 || v1 <= v0 {
		u0, v0, u1, v1 = 0, 0, 1, 1
	}
	rc.pendingGPUTextureCommands = append(rc.pendingGPUTextureCommands, GPUTextureDrawCommand{
		View: view, DstX: dstX, DstY: dstY, DstW: dstW, DstH: dstH,
		U0: u0, V0: v0, U1: u1, V1: v1,
		Opacity: opacity, ViewportWidth: vpW, ViewportHeight: vpH,
	})
	rc.pendingTarget = target
	rc.hasPendingTarget = true
}

// QueueGlyphMask accumulates a glyph mask batch for dispatch.
// Adjacent batches with identical visual properties (transform, color, LCD mode,
// atlas page) are coalesced into a single batch to minimize GPU draw calls (ADR-031).
func (rc *GPURenderContext) QueueGlyphMask(target render.GPURenderTarget, batch GlyphMaskBatch) {
	if err := rc.prepareTarget(target); err != nil {
		slogger().Warn("auto-flush failed", "err", err)
	}
	rc.ensureDrawOrder(drawTierGlyph)
	// Own a copy of quads so GlyphMaskEngine layout scratch / cache aliases stay safe.
	if n := len(batch.Quads); n > 0 {
		start := len(rc.glyphMaskQuadStore)
		rc.glyphMaskQuadStore = append(rc.glyphMaskQuadStore, batch.Quads...)
		batch.Quads = rc.glyphMaskQuadStore[start:]
	}
	// Coalesce with last pending batch if same visual properties (ADR-031).
	// Skip merging if a scissor boundary was crossed since the last batch was
	// queued (glyphBatchSealed=true); this keeps glyph batches within the
	// correct scissor group so text is not clipped by a sibling element's rect.
	if n := len(rc.pendingGlyphMaskBatches); n > 0 && !rc.glyphBatchSealed {
		last := &rc.pendingGlyphMaskBatches[n-1]
		if last.CanMerge(batch) {
			// batch.Quads already in store; just extend last range if contiguous.
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

// queueGlyphMaskSplit expands multi-page glyph batches then queues each page.
// Required once the atlas grows past page 0: UVs are page-local and the GPU
// bind group samples one R8 page texture per batch.
func (rc *GPURenderContext) queueGlyphMaskSplit(target render.GPURenderTarget, batch GlyphMaskBatch) {
	for _, b := range SplitGlyphMaskBatchByPage(batch) {
		if len(b.Quads) == 0 {
			continue
		}
		rc.QueueGlyphMask(target, b)
	}
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

	rc.queueGlyphMaskSplit(target, batch)
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

	rc.queueGlyphMaskSplit(target, batch)
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

	rc.queueGlyphMaskSplit(target, batch)
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
		// Solid SourceOver + GPU mask: stencil-then-cover with cover-pass R8.
		// If the mask plane is not live yet (SetMask without MaskAware bind),
		// bootstrap via fillMaskedAsImage so SetMask+PushMaskLayer stays correct.
		if !(isGPUSolidPaint(paint) && paintUsesSourceOver(paint) && paintSupportsGPUFixedBlend(paint) && rc.shared.HasGPUMask()) {
			return rc.fillMaskedAsImage(target, path, paint)
		}
		// HasGPUMask but cover-inline failed: still force masked bootstrap.
		// Falling through to unmasked solid fill ignores SetMask (WithSetMask).
		return rc.fillMaskedAsImage(target, path, paint)
	}
	if !isGPUSolidPaint(paint) {
		// GPU-FIRST order (do not reorder / demote):
		//  1) advanced dual-tex GPU
		//  2) fillBrushNative: span/field/convex/pattern-rect GPU, then field×coverage / coverage+ColorAt GPU*
		//  3) fillBrushAsImage full software stage + GPU blit (GPU*)
		//  4) only if both fail → ErrFallbackToCPU (caller may pure-CPU)
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

	if err := rc.prepareTarget(target); err != nil {
		return err
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
	// Hairline / 1px axis-aligned strokes: snap to pixel centers so ink lands
	// on the intended device row/column (caret, HiDPI hairline).
	if shouldSnapHairline(paint) {
		pathToStroke = snapHairlineStrokePath(pathToStroke)
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
		// Match FillPath/text paths: lazy-init shared GPU before silent CPU fallback.
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return rc.shared.cpuFallback.FillShape(target, shape, paint)
		}
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
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return rc.shared.cpuFallback.StrokeShape(target, shape, paint)
		}
	}

	// R3: dashed / non-SO / non-solid → geometric StrokePath (GPU expand+fill).
	// Thin solid SourceOver strokes (Ant 1px rings/borders) stay on SDF annular
	// coverage AA — expand+fill path produces hard edges that look non-Ant in
	// the window (CPU software AA in ui_ant_compare looked fine by comparison).
	// Floor matches CPU sdfMinStrokeWidth (1.0); sub-1px still expands.
	w := effectiveStrokeWidth(paint)
	if paint.IsDashed() || !paintUsesSourceOver(paint) || w < 1.0 || !isGPUSolidPaint(paint) {
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
	// Track View-nil flushes that consume pending into pixmap Data so filter
	// graphs can seed from the full surface (advanced blend mid-flush / D140).
	hadPending := rc.PendingCount() > 0
	viewNil := target.View.IsNil()
	defer func() {
		// After a successful encode path, pending queues are cleared. If we had
		// work and targeted the CPU-backed surface, pixmap Data is authoritative.
		if hadPending && viewNil && rc.PendingCount() == 0 {
			rc.flushedPendingToData = true
		}
	}()
	// F1: restore present-deferred parent draws.
	// Base/CPU FlushGPU uses View-nil targets — must unstash here or white/cyan
	// base draws remain stashed forever and advanced layer resolve sees black (D05).
	if target.View.IsNil() && rc.presentStash.active {
		rc.unstashPresentPending()
	}
	// Layer RTs also carry a non-nil View; never inject the stash into a layer
	// self-flush (pendingTarget matches the layer View being flushed).
	// opt21: layer self-flush while parent is present-stashed can encode+enqueue
	// and coalesce with Present (or the next surface Submit) instead of a mid-frame
	// Queue.Submit per PopLayer (PKS Screen+Multiply ~2 extra submits/frame).
	opt21DeferLayerSubmit := false
	if !target.View.IsNil() && rc.presentStash.active {
		layerSelfFlush := rc.hasPendingTarget && !rc.pendingTarget.View.IsNil() && sameTarget(&rc.pendingTarget, &target)
		if layerSelfFlush {
			opt21DeferLayerSubmit = true
		} else if !rc.hasPendingTarget || rc.pendingTarget.View.IsNil() {
			rc.unstashPresentPending()
		}
	}
	pending := rc.PendingCount()

	if pending == 0 {
		if len(rc.pendingAdvancedLayers) > 0 {
			return rc.resolvePendingAdvancedLayers(target)
		}
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
	} else if !gpuFilterGraphRegistered {
		// opt41: device already live — only register filter graph once.
		// Skip ensureGPU body on the warm present path (was every Flush).
		rc.shared.registerFilterGraphIfNeeded()
	}
	rc.shared.ensurePipelines()

	device := rc.shared.device
	queue := rc.shared.queue
	sdfPipeline := rc.shared.sdfRenderPipeline
	convexRend := rc.shared.convexRenderer
	stencilRend := rc.shared.stencilRenderer
	textEng := rc.shared.textEngine
	glyphEng := rc.shared.glyphMaskEngine
	sharedGen := rc.shared.deviceGen
	rc.shared.mu.Unlock()

	// After AutoRecover / SetDeviceProvider, shared pipelines are Destroy()'d and
	// deviceGen bumps. Session still holds the released *Device + stale pipeline
	// pointers → CreateShaderModule "resource already released". Rebuild session.
	if rc.session != nil && (rc.deviceGen != sharedGen || rc.session.device != device) {
		rc.session.Destroy()
		rc.session = nil
		rc.frameRendered = false
		rc.lastView = nil
	}

	// Ensure session exists with all renderers.
	if rc.session == nil {
		sc := rc.shared.SampleCount()
		if rc.preferSampleCount1 {
			sc = 1
		}
		rc.session = NewGPURenderSession(device, queue, sc)
		rc.deviceGen = sharedGen
		// Effect/offscreen surfaces (preferSampleCount1) MUST own sampleCount-matched
		// shape pipelines. Injecting the shared stencil/sdf/convex renderers thrashes
		// createPipelines every alternating main↔effect flush: each session has its
		// own mask/clip BGL objects, so coverPipeMaskLayout mismatches and rebuilds
		// WGSL+pipelines (~40% CPU on continuous glow). Also sampleCount may differ
		// (shared=4 vs effect=1). Main/window contexts keep shared pipelines.
		if rc.preferSampleCount1 {
			rc.session.MarkOwnsShapePipelines()
		} else {
			rc.session.SetSDFPipeline(sdfPipeline)
			rc.session.SetConvexRenderer(convexRend)
			rc.session.SetStencilRenderer(stencilRend)
		}
	} else if !rc.preferSampleCount1 {
		// Same device: re-bind only when GPUShared replaced pipeline objects
		// (avoids pipelinesReady=false every frame).
		if sdfPipeline != nil && rc.session.sdfPipeline != sdfPipeline {
			rc.session.SetSDFPipeline(sdfPipeline)
		}
		if convexRend != nil && rc.session.convexRenderer != convexRend {
			rc.session.SetConvexRenderer(convexRend)
		}
		if stencilRend != nil && rc.session.stencilRenderer != stencilRend {
			rc.session.SetStencilRenderer(stencilRend)
		}
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

	// F1: when advanced layer pops are pending, render the frame into frameScratch
	// (CopySrc|TextureBinding|RenderAttachment) so dual-tex can sample dest.
	// Swapchain views lack COPY_SRC and cannot be dual-tex destinations.
	surfaceView := target.View
	useScratch := len(rc.pendingAdvancedLayers) > 0 && !surfaceView.IsNil()
	if useScratch {
		sw, sh := target.Width, target.Height
		if target.ViewWidth > 0 && target.ViewHeight > 0 {
			sw, sh = int(target.ViewWidth), int(target.ViewHeight)
		}
		if sw > 0 && sh > 0 {
			if serr := rc.ensureFrameScratch(sw, sh); serr != nil {
				useScratch = false
			} else {
				target.View = gpucontext.NewTextureView(unsafe.Pointer(rc.frameScratchView)) //nolint:gosec
				// Force LoadOpClear on scratch for this frame.
				rc.frameRendered = false
				rc.lastView = nil
				if rc.session != nil {
					rc.session.SetFrameState(false, nil)
				}
			}
		} else {
			useScratch = false
		}
	}

	// F1: full-frame single-submit (base+dual-tex+surface blit one encoder) is
	// still disabled — enabling regressed TestF1_AdvancedLayerPresentView* pixels
	// (Multiply white / Screen black). Live path remains multi-submit MultiInto
	// out-RT + blit (opt32). Re-enable only with dest-correct proof + F1 green.
	// opt39 instead applies convex vertex sticky (see buildConvexResources).
	singleSubmit := false && useScratch && rc.sharedEncoder == nil && len(rc.pendingAdvancedLayers) > 0 &&
		rc.session != nil && rc.session.device != nil && rc.session.queue != nil
	var frameEnc *webgpu.CommandEncoder
	if singleSubmit {
		enc, eerr := rc.session.device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "f1_frame_enc"})
		if eerr == nil && enc != nil {
			frameEnc = enc
			rc.sharedEncoder = enc
		} else {
			singleSubmit = false
		}
	}

	// opt21: defer mid-frame layer RT Submit into lead queue (no sharedEncoder).
	// Clear immediately after encode so nested resolve/blit paths still submit.
	if opt21DeferLayerSubmit && rc.session != nil && rc.sharedEncoder == nil {
		rc.session.SetDeferSurfaceSubmit(true)
	}
	err := rc.session.RenderFrameGrouped(target, groups, baseLayer, rc.sharedEncoder)
	if opt21DeferLayerSubmit && rc.session != nil {
		rc.session.SetDeferSurfaceSubmit(false)
	}
	if err != nil {
		total := 0
		for i := range groups {
			total += len(groups[i].SDFShapes) + len(groups[i].ConvexCommands) + len(groups[i].StencilPaths) +
				len(groups[i].ImageCommands) + len(groups[i].TextBatches) + len(groups[i].GlyphMaskBatches)
		}
		slogger().Warn("render session error",
			"groups", len(groups), "totalItems", total, "err", err)
	}
	// Base pass (including HUD/text) is encoded. Sync session frame state so a
	// nested resolve Flush uses LoadOpLoad instead of clearing scratch.
	// Then drop base command lists — resolve only submits blend composite blits.
	// Re-encoding the full scene in resolve was wiping HUD text: outer Flush had
	// forced rc.frameRendered=false for scratch, and nested Flush restored that
	// Clear state even after the base pass had already painted text.
	if rc.session != nil {
		rc.frameRendered, rc.lastView = rc.session.FrameState()
	}
	if err == nil && len(rc.pendingAdvancedLayers) > 0 {
		rc.dropEncodedPendingCommands()
		if aerr := rc.resolvePendingAdvancedLayersEnc(target, frameEnc); aerr != nil {
			err = aerr
			// dual-tex failure: fall back to non-single-submit residual path.
			singleSubmit = false
		}
	}
	// Blit scratch → swapchain when advanced path used the intermediate.
	if err == nil && useScratch && !surfaceView.IsNil() && rc.frameScratchView != nil {
		vpW := uint32(target.Width)  //nolint:gosec
		vpH := uint32(target.Height) //nolint:gosec
		if target.ViewWidth > 0 && target.ViewHeight > 0 {
			vpW, vpH = target.ViewWidth, target.ViewHeight
		}
		blitTarget := target
		blitTarget.View = surfaceView
		rc.frameRendered = false
		rc.lastView = nil
		if rc.session != nil {
			rc.session.SetFrameState(false, nil)
		}
		// Queue blit as base-layer style texture draw on a clean pending set.
		// (pending shape lists already cleared below only after this block — so
		// clear command lists first for a blit-only encode.)
		clear(rc.pendingShapes)
		clear(rc.pendingConvexCommands)
		clear(rc.pendingStencilPaths)
		clear(rc.pendingImageCommands)
		clear(rc.pendingGPUTextureCommands)
		clear(rc.pendingTextBatches)
		clear(rc.pendingGlyphMaskBatches)
		rc.pendingShapes = rc.pendingShapes[:0]
		rc.pendingConvexCommands = rc.pendingConvexCommands[:0]
		rc.pendingStencilPaths = rc.pendingStencilPaths[:0]
		rc.pendingImageCommands = rc.pendingImageCommands[:0]
		rc.pendingGPUTextureCommands = rc.pendingGPUTextureCommands[:0]
		rc.pendingTextBatches = rc.pendingTextBatches[:0]
		rc.pendingGlyphMaskBatches = rc.pendingGlyphMaskBatches[:0]
		rc.glyphMaskQuadStore = rc.glyphMaskQuadStore[:0]
		rc.scissorSegments = rc.scissorSegments[:0]
		rc.hasPendingTarget = false

		rc.QueueGPUTextureDraw(blitTarget,
			gpucontext.NewTextureView(unsafe.Pointer(rc.frameScratchView)), //nolint:gosec
			0, 0, float32(vpW), float32(vpH), 1.0, vpW, vpH)

		if singleSubmit && frameEnc != nil {
			// Encode surface blit into frameEnc without nested Flush singleSubmit.
			rc.sharedEncoder = frameEnc
			groups2 := rc.buildScissorGroups()
			base2 := rc.baseLayer
			rc.baseLayer = nil
			if berr := rc.session.RenderFrameGrouped(blitTarget, groups2, base2, frameEnc); berr != nil {
				err = berr
			}
			rc.sharedEncoder = nil
			// Drop pending after encode.
			rc.pendingGPUTextureCommands = rc.pendingGPUTextureCommands[:0]
			rc.hasPendingTarget = false
			cmd, ferr := frameEnc.Finish()
			if ferr != nil {
				err = ferr
			} else if cmd != nil {
				// opt39: coalesce opt21 deferred layer CBs + frame CB (leads first).
				// submitWithLeading retains cmd in prevCmdBufs on success.
				if serr := rc.session.submitWithLeading(cmd); serr != nil && err == nil {
					err = serr
				}
			}
			frameEnc = nil
			// Layer RTs sampled by dual-tex are safe to free after submit.
			rc.drainLayerReleaseHold()
		} else {
			if frameEnc != nil {
				// Abandon single-submit encoder; submit whatever was recorded first.
				rc.sharedEncoder = nil
				if cmd, ferr := frameEnc.Finish(); ferr == nil && cmd != nil {
					_ = rc.session.submitWithLeading(cmd)
				}
				frameEnc = nil
			}
			// Normal separate submit for blit.
			if ferr := rc.Flush(blitTarget); ferr != nil {
				err = ferr
			}
			// resolve deferred holds if dual-tex used external enc then fell back.
			rc.drainLayerReleaseHold()
		}
	} else if frameEnc != nil {
		// Advanced resolved into scratch but no blit (shouldn't happen with useScratch).
		rc.sharedEncoder = nil
		cmd, ferr := frameEnc.Finish()
		if ferr != nil && err == nil {
			err = ferr
		} else if cmd != nil {
			if serr := rc.session.submitWithLeading(cmd); serr != nil && err == nil {
				err = serr
			}
		}
		rc.drainLayerReleaseHold()
	}
	if frameEnc != nil {
		rc.sharedEncoder = nil
	}
	// Safety: if advanced path ran without useScratch, holds may still be pending.
	if len(rc.layerReleaseHold) > 0 {
		rc.drainLayerReleaseHold()
	}

	// Cover textures must outlive RenderFrameGrouped bind/sample; free after submit.
	rc.releaseBrushCoverResults()

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
	rc.convexMeshPacked = rc.convexMeshPacked[:0]
	rc.convexMeshIdx = rc.convexMeshIdx[:0]
	rc.pendingStencilPaths = rc.pendingStencilPaths[:0]
	rc.pendingImageCommands = rc.pendingImageCommands[:0]
	rc.pendingGPUTextureCommands = rc.pendingGPUTextureCommands[:0]
	rc.pendingTextBatches = rc.pendingTextBatches[:0]
	rc.pendingGlyphMaskBatches = rc.pendingGlyphMaskBatches[:0]
	rc.glyphMaskQuadStore = rc.glyphMaskQuadStore[:0]
	rc.scissorSegments = rc.scissorSegments[:0]

	// Read back frame tracking from session.
	rc.frameRendered, rc.lastView = rc.session.FrameState()

	return err
}

// dropEncodedPendingCommands clears base draw queues after they have been
// consumed by RenderFrameGrouped. Keeps pendingAdvancedLayers and layer holds.
// Used so advanced-blend resolve only encodes composite blits (LoadOpLoad).
func (rc *GPURenderContext) dropEncodedPendingCommands() {
	if rc == nil {
		return
	}
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
	rc.convexMeshPacked = rc.convexMeshPacked[:0]
	rc.convexMeshIdx = rc.convexMeshIdx[:0]
	rc.pendingStencilPaths = rc.pendingStencilPaths[:0]
	rc.pendingImageCommands = rc.pendingImageCommands[:0]
	rc.pendingGPUTextureCommands = rc.pendingGPUTextureCommands[:0]
	rc.pendingTextBatches = rc.pendingTextBatches[:0]
	rc.pendingGlyphMaskBatches = rc.pendingGlyphMaskBatches[:0]
	rc.glyphMaskQuadStore = rc.glyphMaskQuadStore[:0]
	rc.scissorSegments = rc.scissorSegments[:0]
	rc.baseLayer = nil
	rc.hasPendingTarget = false
	rc.textBatchSealed = true
	rc.glyphBatchSealed = true
}

// QueueAdvancedLayerComposite defers a PopLayer advanced blend until Flush has a
// live destination View (PresentFrame surface). Keeps src RT alive via release.
func (rc *GPURenderContext) QueueAdvancedLayerComposite(
	srcView gpucontext.TextureView, srcW, srcH int,
	damage image.Rectangle, mode render.BlendMode, opacity float64,
	release func(),
) {
	if rc == nil || srcView.IsNil() {
		if release != nil {
			release()
		}
		return
	}
	rc.pendingAdvancedLayers = append(rc.pendingAdvancedLayers, pendingAdvancedLayer{
		srcView: srcView,
		srcW:    srcW,
		srcH:    srcH,
		damage:  damage,
		mode:    mode,
		opacity: opacity,
		release: release,
	})
}

// ensureFrameScratch allocates/resizes a BGRA intermediate for advanced blend dest sampling.
func (rc *GPURenderContext) ensureFrameScratch(w, h int) error {
	if rc == nil || w <= 0 || h <= 0 {
		return render.ErrFallbackToCPU
	}
	if rc.frameScratchTex != nil && rc.frameScratchW == w && rc.frameScratchH == h && rc.frameScratchView != nil {
		return nil
	}
	if rc.frameScratchView != nil {
		rc.frameScratchView.Release()
		rc.frameScratchView = nil
	}
	if rc.frameScratchTex != nil {
		rc.frameScratchTex.Release()
		rc.frameScratchTex = nil
	}
	rc.shared.mu.Lock()
	device := rc.shared.device
	rc.shared.mu.Unlock()
	if device == nil {
		return render.ErrFallbackToCPU
	}
	usage := types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageCopyDst | types.TextureUsageTextureBinding
	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "adv_blend_frame_scratch",
		Size:          webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         usage,
	})
	if err != nil {
		return err
	}
	view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label:         "adv_blend_frame_scratch_view",
		Format:        types.TextureFormatBGRA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Release()
		return err
	}
	rc.frameScratchTex = tex
	rc.frameScratchView = view
	rc.frameScratchW, rc.frameScratchH = w, h
	return nil
}

// resolvePendingAdvancedLayers composites deferred advanced-blend layers at
// Present Flush. When the active target is frameScratch (or any texture with
// CopySrc|TextureBinding), dual-tex samples dest+src without CPU PollWait.
// Damage-tight UV is used for both dual-tex and the opacity fallback blit.
func (rc *GPURenderContext) resolvePendingAdvancedLayers(target render.GPURenderTarget) error {
	return rc.resolvePendingAdvancedLayersEnc(target, nil)
}

func (rc *GPURenderContext) resolvePendingAdvancedLayersEnc(target render.GPURenderTarget, enc *webgpu.CommandEncoder) error {
	if rc == nil || len(rc.pendingAdvancedLayers) == 0 {
		return nil
	}
	layers := rc.pendingAdvancedLayers
	rc.pendingAdvancedLayers = nil

	// View-nil Image/FlushGPU path: dual-tex Multiply with empty alpha was wiping
	// base (D05 black outside). Materialize each layer RT and CPU-blend into
	// target.Data (base already flushed into Data by RenderFrameGrouped).
	// Present FlushGPUWithView keeps dual-tex below.
	if target.View.IsNil() && len(target.Data) >= target.Width*target.Height*4 && target.Width > 0 && target.Height > 0 {
		var firstErr error
		for i := range layers {
			pl := &layers[i]
			if pl.srcView.IsNil() {
				if pl.release != nil {
					pl.release()
				}
				continue
			}
			rgba, err := rc.ReadbackViewRGBA(pl.srcView, pl.srcW, pl.srcH)
			if pl.release != nil {
				pl.release()
			}
			if err != nil || len(rgba) < pl.srcW*pl.srcH*4 {
				if firstErr == nil && err != nil {
					firstErr = err
				}
				continue
			}
			if err := cpuCompositeAdvancedLayer(target.Data, target.Width, target.Height, rgba, pl.srcW, pl.srcH, pl.mode, pl.opacity, pl.damage); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	// FlushGPU (View-nil / CPU readback target): still composite via frameScratch,
	// then copy result into target.Data so sampleRGBA / Image paths see blends.
	readbackToData := false
	if target.View.IsNil() {
		tw, th := target.Width, target.Height
		if tw <= 0 || th <= 0 {
			for i := range layers {
				if layers[i].release != nil {
					layers[i].release()
				}
			}
			return render.ErrFallbackToCPU
		}
		if err := rc.ensureFrameScratch(tw, th); err != nil {
			for i := range layers {
				if layers[i].release != nil {
					layers[i].release()
				}
			}
			return err
		}
		// First paint base pixmap into scratch if we have CPU data (prior draws).
		// Callers often FlushGPU after PopLayer with empty pending; base already
		// lives in target.Data from previous flushes.
		target.View = gpucontext.NewTextureView(unsafe.Pointer(rc.frameScratchView)) //nolint:gosec
		target.ViewWidth = uint32(tw)                                                //nolint:gosec
		target.ViewHeight = uint32(th)                                               //nolint:gosec
		readbackToData = true
		// Seed scratch from CPU pixmap so dual-tex has a real dest.
		if len(target.Data) >= tw*th*4 {
			_ = rc.uploadPixmapToView(target)
		}
	}

	vpW := uint32(target.Width)  //nolint:gosec
	vpH := uint32(target.Height) //nolint:gosec
	if target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}
	tw, th := int(vpW), int(vpH)
	full := image.Rect(0, 0, tw, th)

	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	cache := &rc.shared.dualTexBlend
	rc.shared.mu.Unlock()

	dstTex := (*webgpu.TextureView)(target.View.Pointer()).Texture()
	canDual := device != nil && queue != nil && dstTex != nil

	// F1: batch advanced layers into one dual-tex Submit writing directly into
	// frameScratch (dest). Falls back to opacity blit if dual-tex fails.
	holds := rc.layerReleaseHold[:0]
	viewOps := rc.dualTexViewOpsScratch[:0]
	if cap(viewOps) < len(layers) {
		viewOps = make([]dualTexViewBlendOp, 0, len(layers))
	}
	type fallbackLayer struct {
		pl     pendingAdvancedLayer
		bounds image.Rectangle
		op     float32
	}
	var fallbacks []fallbackLayer

	for i := range layers {
		pl := &layers[i]
		if pl.srcView.IsNil() {
			if pl.release != nil {
				pl.release()
			}
			continue
		}
		op := float32(pl.opacity)
		if op < 0 {
			op = 0
		}
		if op > 1 {
			op = 1
		}
		bounds := pl.damage
		if bounds.Empty() {
			bounds = image.Rect(0, 0, pl.srcW, pl.srcH)
		}
		bounds = bounds.Inset(-2).Intersect(full).Intersect(image.Rect(0, 0, pl.srcW, pl.srcH))
		if bounds.Empty() {
			if pl.release != nil {
				holds = append(holds, pl.release)
			}
			continue
		}
		srcWGPU := (*webgpu.TextureView)(pl.srcView.Pointer())
		var srcTex *webgpu.Texture
		if srcWGPU != nil {
			srcTex = srcWGPU.Texture()
		}
		if canDual && srcTex != nil && srcWGPU != nil {
			viewOps = append(viewOps, dualTexViewBlendOp{
				srcView: srcWGPU,
				bounds:  bounds,
				mode:    pl.mode,
				opacity: op,
			})
			if pl.release != nil {
				holds = append(holds, pl.release)
			}
		} else {
			fallbacks = append(fallbacks, fallbackLayer{pl: *pl, bounds: bounds, op: op})
		}
	}

	rc.dualTexViewOpsScratch = viewOps
	var err error
	// Live path: dualTexAdvancedBlendViewsRegionSized / multi bundle → out RT → blit.
	// opt12: batch all layers into one dual-tex Submit when possible.
	type outBlit struct {
		view    *webgpu.TextureView
		tex     *webgpu.Texture
		bounds  image.Rectangle
		opacity float32
	}
	var outs []outBlit
	// opt32: optional shared encoder for dual-tex multi + composite blit (one Finish).
	var compositeEnc *webgpu.CommandEncoder
	if len(viewOps) > 0 {
		dstView := (*webgpu.TextureView)(target.View.Pointer())
		recordEnc := enc
		if recordEnc == nil && device != nil {
			if ce, eerr := device.CreateCommandEncoder(dualTexCompositeEncoderDesc); eerr == nil {
				compositeEnc = ce
				recordEnc = ce
			}
		}
		// Prefer IntoEncoder (opt32 / shared frame enc). Fall back to R7.3
		// separate dual-tex CB + leading coalesce, then per-op path.
		var mout []dualTexViewBlendOut
		var derr error
		if recordEnc != nil {
			mout, derr = dualTexAdvancedBlendViewsMultiIntoEncoder(
				device, queue, cache, dstView, viewOps, tw, th, recordEnc)
		} else {
			derr = fmt.Errorf("dual-tex multi: no encoder")
		}
		if derr != nil {
			if compositeEnc != nil {
				compositeEnc.DiscardEncoding()
				compositeEnc = nil
			}
			// R7.3: finish dual-tex multi without Submit; coalesce with following
			// blit Flush into one Queue.Submit (multi CB, ordered).
			bundle, berr := dualTexAdvancedBlendViewsMultiBundle(device, queue, cache, dstView, viewOps, tw, th, false)
			if berr != nil {
				err = berr
				// Fallback: per-op path (still correct, more submits).
				for i := range viewOps {
					op := viewOps[i]
					outTex, outView, oerr := dualTexAdvancedBlendViewsRegionSized(
						device, queue, cache, dstView, op.srcView, op.bounds, op.mode, tw, th)
					if oerr != nil {
						err = oerr
						continue
					}
					outs = append(outs, outBlit{view: outView, tex: outTex, bounds: op.bounds, opacity: op.opacity})
				}
			} else {
				for _, mo := range bundle.Outs {
					outs = append(outs, outBlit{view: mo.view, tex: mo.tex, bounds: mo.bounds, opacity: mo.opacity})
				}
				if bundle.Cmd != nil && rc.session != nil {
					rc.session.EnqueueLeadingSubmit(bundle.Cmd, bundle.Cleanup)
				} else if bundle.Cleanup != nil {
					bundle.Cleanup()
				}
			}
		} else {
			for _, mo := range mout {
				outs = append(outs, outBlit{view: mo.view, tex: mo.tex, bounds: mo.bounds, opacity: mo.opacity})
			}
			// Dual-tex passes live on compositeEnc/enc — no separate lead CB.
		}
	}

	// If viewsRegion produced nothing and dual-tex path failed, demote to opacity blits.
	if len(outs) == 0 {
		for i := range layers {
			pl := &layers[i]
			if pl.srcView.IsNil() {
				continue
			}
			bounds := pl.damage
			if bounds.Empty() {
				bounds = image.Rect(0, 0, pl.srcW, pl.srcH)
			}
			bounds = bounds.Inset(-2).Intersect(full).Intersect(image.Rect(0, 0, pl.srcW, pl.srcH))
			if bounds.Empty() {
				continue
			}
			op := float32(pl.opacity)
			if op < 0 {
				op = 0
			}
			if op > 1 {
				op = 1
			}
			fallbacks = append(fallbacks, fallbackLayer{pl: *pl, bounds: bounds, op: op})
		}
	}

	for i := range outs {
		o := &outs[i]
		u0, v0, u1, v1 := float32(0), float32(0), float32(1), float32(1)
		gv := gpucontext.NewTextureView(unsafe.Pointer(o.view)) //nolint:gosec
		rc.QueueGPUTextureDrawUV(target, gv,
			float32(o.bounds.Min.X), float32(o.bounds.Min.Y),
			float32(o.bounds.Dx()), float32(o.bounds.Dy()),
			o.opacity, vpW, vpH, u0, v0, u1, v1)
		// Keep outTex/view alive until after Flush samples them.
		tex, view := o.tex, o.view
		bw, bh := o.bounds.Dx(), o.bounds.Dy()
		holds = append(holds, func() {
			if cache != nil && tex != nil && view != nil {
				cache.putOutBGRA(tex, view, bw, bh)
				return
			}
			if view != nil {
				view.Release()
			}
			if tex != nil {
				tex.Release()
			}
		})
	}

	for i := range fallbacks {
		fb := &fallbacks[i]
		u0 := float32(fb.bounds.Min.X) / float32(fb.pl.srcW)
		v0 := float32(fb.bounds.Min.Y) / float32(fb.pl.srcH)
		u1 := float32(fb.bounds.Max.X) / float32(fb.pl.srcW)
		v1 := float32(fb.bounds.Max.Y) / float32(fb.pl.srcH)
		rc.QueueGPUTextureDrawUV(target, fb.pl.srcView,
			float32(fb.bounds.Min.X), float32(fb.bounds.Min.Y),
			float32(fb.bounds.Dx()), float32(fb.bounds.Dy()),
			fb.op, vpW, vpH, u0, v0, u1, v1)
	}
	rc.layerReleaseHold = holds

	if rc.PendingCount() > 0 {
		// pendingAdvancedLayers already nil — no recursion into resolve.
		if compositeEnc != nil && rc.session != nil {
			// opt32: encode composite blit into the same encoder as dual-tex multi,
			// one Finish + submitWithLeading (layers + dual+blit).
			rc.sharedEncoder = compositeEnc
			ferr := rc.Flush(target)
			rc.sharedEncoder = nil
			if ferr != nil {
				compositeEnc.DiscardEncoding()
				compositeEnc = nil
				if err == nil {
					err = ferr
				}
			} else {
				cmd, ferr := compositeEnc.Finish()
				compositeEnc = nil
				if ferr != nil {
					if err == nil {
						err = ferr
					}
				} else if serr := rc.session.submitWithLeading(cmd); serr != nil && err == nil {
					err = serr
				}
			}
		} else if enc != nil && rc.session != nil {
			// opt39: external single-submit encoder owns Finish/Submit.
			// Encode dual-tex out→scratch blits into enc; do not Finish here.
			rc.sharedEncoder = enc
			ferr := rc.Flush(target)
			rc.sharedEncoder = nil
			if ferr != nil && err == nil {
				err = ferr
			}
		} else {
			// R7.3 path: Flush surface/blit CB; session coalesces deferred dual-tex multi.
			if ferr := rc.Flush(target); ferr != nil && err == nil {
				err = ferr
			}
		}
	} else if compositeEnc != nil && rc.session != nil {
		// Dual-tex multi encoded but no blit queued — still Finish+submit.
		cmd, ferr := compositeEnc.Finish()
		compositeEnc = nil
		if ferr != nil {
			if err == nil {
				err = ferr
			}
		} else if serr := rc.session.submitWithLeading(cmd); serr != nil && err == nil {
			err = serr
		}
	} else if rc.session != nil {
		// Dual-tex multi finished but no composite blit queued — still submit.
		if ferr := rc.session.FlushLeadingSubmitsOnly(); ferr != nil && err == nil {
			err = ferr
		}
	}
	if compositeEnc != nil {
		compositeEnc.DiscardEncoding()
		compositeEnc = nil
	}
	// When encoding into an external single-submit encoder, layer RT textures
	// must stay alive until Finish/Submit. Caller drains layerReleaseHold after.
	if enc == nil {
		rc.drainLayerReleaseHold()
	}
	if readbackToData && !target.View.IsNil() && len(target.Data) > 0 {
		if rgba, rerr := rc.ReadbackViewRGBA(target.View, target.Width, target.Height); rerr == nil {
			// ReadbackViewRGBA is RGBA; pixmap Data is premultiplied RGBA.
			n := len(target.Data)
			if len(rgba) < n {
				n = len(rgba)
			}
			copy(target.Data[:n], rgba[:n])
		} else if err == nil {
			err = rerr
		}
	}
	return err
}

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
// Reuses pooled textures by (w,h) to avoid VRAM OOM from per-layer alloc.
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
	key := [2]int{w, h}
	if rc.offscreenPool == nil {
		rc.offscreenPool = make(map[[2]int][]offscreenPooled)
	}
	if bucket := rc.offscreenPool[key]; len(bucket) > 0 {
		item := bucket[len(bucket)-1]
		rc.offscreenPool[key] = bucket[:len(bucket)-1]
		release := func() {
			// return to pool (cap 4 per size)
			if rc.offscreenPool == nil {
				rc.offscreenPool = make(map[[2]int][]offscreenPooled)
			}
			b := rc.offscreenPool[key]
			if len(b) < 8 {
				rc.offscreenPool[key] = append(b, item)
				return
			}
			item.view.Release()
			item.tex.Release()
		}
		return gpucontext.NewTextureView(unsafe.Pointer(item.view)), release //nolint:gosec
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

	item := offscreenPooled{tex: tex, view: view}
	release := func() {
		if rc.offscreenPool == nil {
			rc.offscreenPool = make(map[[2]int][]offscreenPooled)
		}
		b := rc.offscreenPool[key]
		if len(b) < 8 {
			rc.offscreenPool[key] = append(b, item)
			return
		}
		view.Release()
		tex.Release()
	}
	return gpucontext.NewTextureView(unsafe.Pointer(view)), release //nolint:gosec // Go spec Rule 1 (ADR-018)
}

// Close releases this context's GPU resources. Shared resources are NOT
// released — they are owned by GPUShared.
// PurgeSurfaceResources drops surface-sized GPU attachments and offscreen
// pools without unregistering the context or destroying shared pipelines.
func (rc *GPURenderContext) PurgeSurfaceResources() {
	if rc == nil {
		return
	}
	if rc.session != nil {
		rc.session.PurgeSurfaceTextures()
	}
	rc.drainOffscreenPool()
	rc.releaseBrushCoverResults()
}

func (rc *GPURenderContext) Close() {
	if rc.shared != nil {
		rc.shared.unregisterContext(rc)
	}
	if rc.session != nil {
		rc.session.Destroy()
		rc.session = nil
	}
	// Layer offscreen pool (CreateOffscreenTexture "offscreen_cache") pins device
	// VRAM across AutoRecover if not drained — measured 3×tex+view after abandon.
	rc.drainOffscreenPool()
	if rc.frameScratchView != nil {
		rc.frameScratchView.Release()
		rc.frameScratchView = nil
	}
	if rc.frameScratchTex != nil {
		rc.frameScratchTex.Release()
		rc.frameScratchTex = nil
	}
	rc.frameScratchW, rc.frameScratchH = 0, 0
	rc.pendingShapes = nil
	rc.pendingConvexCommands = nil
	rc.pendingStencilPaths = nil
	rc.pendingImageCommands = nil
	rc.pendingGPUTextureCommands = nil
	rc.releaseBrushCoverResults()
	rc.baseLayer = nil
	rc.pendingTextBatches = nil
	rc.pendingGlyphMaskBatches = nil
	rc.glyphMaskQuadStore = nil
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

// drainOffscreenPool releases every pooled layer RT texture/view.
func (rc *GPURenderContext) drainOffscreenPool() {
	if rc == nil || rc.offscreenPool == nil {
		return
	}
	for key, bucket := range rc.offscreenPool {
		for i := range bucket {
			if bucket[i].view != nil {
				bucket[i].view.Release()
				bucket[i].view = nil
			}
			if bucket[i].tex != nil {
				bucket[i].tex.Release()
				bucket[i].tex = nil
			}
		}
		delete(rc.offscreenPool, key)
	}
	rc.offscreenPool = nil
}

// Draw-order tiers match GPURenderSession.recordGroupDraws pass order.
// Within a scissor group, lower tiers always draw before higher tiers. When a
// later API call queues a lower tier while higher-tier work is already pending,
// seal a scissor segment so the new work becomes a later group (painter order).
const (
	drawTierSDF     = 1
	drawTierConvex  = 2
	drawTierStencil = 3
	drawTierImage   = 4
	drawTierGPUTex  = 5
	drawTierText    = 6
	drawTierGlyph   = 7
)

func (rc *GPURenderContext) maxPendingDrawTier() int {
	if rc == nil {
		return 0
	}
	switch {
	case len(rc.pendingGlyphMaskBatches) > 0:
		return drawTierGlyph
	case len(rc.pendingTextBatches) > 0:
		return drawTierText
	case len(rc.pendingGPUTextureCommands) > 0:
		return drawTierGPUTex
	case len(rc.pendingImageCommands) > 0:
		return drawTierImage
	case len(rc.pendingStencilPaths) > 0:
		return drawTierStencil
	case len(rc.pendingConvexCommands) > 0:
		return drawTierConvex
	case len(rc.pendingShapes) > 0:
		return drawTierSDF
	default:
		return 0
	}
}

// ensureDrawOrder seals the current timeline when queueing tier would otherwise
// be drawn under already-pending higher-tier geometry (Skia-style painter order).
func (rc *GPURenderContext) ensureDrawOrder(tier int) {
	if rc == nil || tier <= 0 {
		return
	}
	if rc.maxPendingDrawTier() > tier {
		rc.recordScissorSegment(rc.clipRect)
	}
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
		clampEnd := func(n, e int) int {
			if e < 0 {
				return 0
			}
			if e > n {
				return n
			}
			return e
		}
		groups = append(groups, ScissorGroup{
			Rect:               nil,
			SDFShapes:          rc.pendingShapes[:clampEnd(len(rc.pendingShapes), firstSeg.sdfCount)],
			ConvexCommands:     rc.pendingConvexCommands[:clampEnd(len(rc.pendingConvexCommands), firstSeg.convexCount)],
			StencilPaths:       rc.pendingStencilPaths[:clampEnd(len(rc.pendingStencilPaths), firstSeg.stencilCount)],
			ImageCommands:      rc.pendingImageCommands[:clampEnd(len(rc.pendingImageCommands), firstSeg.imageCount)],
			GPUTextureCommands: rc.pendingGPUTextureCommands[:clampEnd(len(rc.pendingGPUTextureCommands), firstSeg.gpuTexCount)],
			TextBatches:        rc.pendingTextBatches[:clampEnd(len(rc.pendingTextBatches), firstSeg.textCount)],
			GlyphMaskBatches:   rc.pendingGlyphMaskBatches[:clampEnd(len(rc.pendingGlyphMaskBatches), firstSeg.glyphCount)],
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
		// Defensive clamp: corrupted scissor timelines (stash merge bugs) must
		// not panic the frame — empty the inverted range instead.
		clampRange := func(n, a, b int) (int, int) {
			if a < 0 {
				a = 0
			}
			if b < 0 {
				b = 0
			}
			if a > n {
				a = n
			}
			if b > n {
				b = n
			}
			if a > b {
				a, b = b, a
			}
			return a, b
		}
		s0, s1 := clampRange(len(rc.pendingShapes), seg.sdfCount, endSDF)
		c0, c1 := clampRange(len(rc.pendingConvexCommands), seg.convexCount, endConvex)
		t0, t1 := clampRange(len(rc.pendingStencilPaths), seg.stencilCount, endStencil)
		i0, i1 := clampRange(len(rc.pendingImageCommands), seg.imageCount, endImage)
		g0, g1 := clampRange(len(rc.pendingGPUTextureCommands), seg.gpuTexCount, endGPUTex)
		x0, x1 := clampRange(len(rc.pendingTextBatches), seg.textCount, endText)
		y0, y1 := clampRange(len(rc.pendingGlyphMaskBatches), seg.glyphCount, endGlyph)
		groups = append(groups, ScissorGroup{
			Rect:               groupRect,
			ClipRRect:          groupClip,
			ClipPath:           seg.clipPath,
			SDFShapes:          rc.pendingShapes[s0:s1],
			ConvexCommands:     rc.pendingConvexCommands[c0:c1],
			StencilPaths:       rc.pendingStencilPaths[t0:t1],
			ImageCommands:      rc.pendingImageCommands[i0:i1],
			GPUTextureCommands: rc.pendingGPUTextureCommands[g0:g1],
			TextBatches:        rc.pendingTextBatches[x0:x1],
			GlyphMaskBatches:   rc.pendingGlyphMaskBatches[y0:y1],
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
		// Positive reclaim: free dual-tex snap pools then retry once. Under resize
		// churn those pools can hold tens of MB and starve glyph_mask_atlas_0.
		s.dualTexBlend.releasePooledVRAM()
		if s.device != nil {
			_ = s.device.WaitIdle()
		}
		if err2 := s.glyphMaskEngine.SyncAtlasTextures(s.device, s.queue); err2 != nil {
			return err2
		}
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
	// Temporarily prefer fillMaskedAsImage (SetMask+layer correctness).
	// Cover-inline R8 path can skip mask sampling on layer RTs (WithSetMask).
	return false

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
	// Temporarily prefer fillMaskedAsImage (SetMask+layer correctness).
	// Cover-inline R8 path can skip mask sampling on layer RTs (WithSetMask).
	return false

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
