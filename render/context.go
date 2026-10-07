//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"image"
	"sync/atomic"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render/internal/clip"
	"github.com/energye/gpui/render/text"
)

// Context is the main drawing context.
// It maintains a pixmap, current path, paint state, and transformation stack.
// Context implements io.Closer for proper resource cleanup.
//
// When deviceScale > 1.0 (HiDPI/Retina), the Context maintains a larger physical
// pixmap while exposing logical dimensions to user code. Drawing operations use
// logical coordinates; the Context applies a base scale transform transparently.
type Context struct {
	width    int // logical width (user-facing)
	height   int // logical height (user-facing)
	pixmap   *Pixmap
	renderer Renderer

	// HiDPI support.
	// R1-3: atomic — the raster thread rewrites the scale at the present
	// boundary (SetDeviceScale) while UI-side readers (snapshot path,
	// diagnostics) load it concurrently. Plain float64 was a data race.
	deviceScale atomic.Uint64 // math.Float64bits: physical px per logical px (default 1.0)

	// Current state
	path        *Path
	paint       *Paint
	face        text.Face       // Current font face for text drawing
	clipStack   *clip.ClipStack // Clipping stack
	gpuClipPath *Path           // device-space clip path for GPU depth clipping (GPU-CLIP-003a)

	// Transform and state stack
	matrix         Matrix // user transform (starts as Identity, user-space only)
	deviceMatrix   Matrix // device scale transform (Identity when scale=1.0, NEVER modified by user)
	stack          []Matrix
	clipStackDepth []int // Tracks clip stack depth for each Push/Pop

	// Layer support
	layerStack *layerStack // Layer stack for compositing
	basePixmap *Pixmap     // Base pixmap when layers are active
	// layerGPUReleases holds deferred GPU layer texture releases until after
	// Flush completes the DrawGPUTexture composite that samples them (P0-1).
	layerGPUReleases []func()

	// Mask support
	mask      *Mask   // Current alpha mask
	maskStack []*Mask // Mask stack for Push/Pop

	// cached R8 plane for mask/difference clips (GPU MaskAware path).
	// Rebuilt when clipMaskGPUGen or user mask pointer changes.
	clipMaskGPU     *Mask
	clipMaskGPUGen  int // bumped on every clip stack mutation
	clipMaskGPUAt   int // gen when cache was built
	clipMaskGPUUser *Mask

	// Per-frame damage tracking (ADR-021 Level 1).
	// List of per-operation bounding boxes — NOT a single union rect.
	// Each Fill/Stroke adds its own rect. Passed as-is to PresentWithDamage
	// for per-rect OS blit. Merged to bounding box if count exceeds threshold.
	frameDamageRects      []image.Rectangle
	damageTrackingEnabled bool

	// offscreenPassDepth counts active offscreen recording sub-passes
	// (BeginOffscreenPass). While > 0, the surface clip must not leak into
	// the sub-pass stream (see resetGPUClipForPass).
	offscreenPassDepth int

	// passScratch / passMain / passRect implement the retained-record CPU
	// scratch swap (see context_pass_scratch.go). passMain != nil means a
	// swap is active and c.pixmap currently aliases passScratch.
	// passScratchDirty: set when a CPU-fallback draw lands in scratch
	// during a pass; Begin clears only then, Commit skips the
	// zero-scan/upload when unset. Stays set after an upload (content
	// present, needs clear before reuse).
	passScratch      *Pixmap
	passMain         *Pixmap
	passRect         image.Rectangle
	passScratchDirty bool

	// Pipeline mode
	pipelineMode PipelineMode // GPU pipeline selection mode

	// Rasterizer mode
	rasterizerMode RasterizerMode // CPU rasterizer selection mode

	// Anti-aliasing
	antiAlias      bool   // anti-aliasing enabled (default: true)
	antiAliasStack []bool // Push/Pop stack for antiAlias state
	dither         bool   // P.09 ordered dither after resolve

	// Text rendering
	textMode         TextMode               // text strategy selection (default: Auto)
	lcdLayout        LCDLayout              // LCD/ClearType layout (default None)
	textDecoration   TextDecoration         // underline/line-through/overline (X.08)
	outlineExtractor *text.OutlineExtractor // lazy: for transform-aware text (Strategy B)
	glyphCache       *text.GlyphCache       // lazy: cached glyph outlines for drawStringAsOutlines
	colorRasterCache *text.ColorRasterCache // lazy: reused color RGBA cache for the CPU color fallback

	// Per-context GPU render context (isolated pending commands, clips, frame tracking).
	// Lazily created when GPURenderContextProvider is available.
	// Typed as gpuContextOps (defined in this package) to avoid circular import
	// with internal/gpu while maintaining type safety.
	gpuCtx            gpuContextOps
	gpuFallbackWarned bool // true after first global fallback warning (avoid log spam)

	pathStats RenderPathStats

	// Scratch for DrawVertices / DrawMesh (avoid per-call make on hot paths).
	vertDevScratch    []Point
	meshExpPosScratch []Point
	meshExpColScratch []RGBA
	meshSolidScratch  []RGBA

	// GPU filter publish (F.03 zero-readback effect RT).
	// After ApplyBlur/ApplyImageFilterGraph on the GPU texture path, result
	// lives here for DrawGPUTexture compositing; pixmap is lazily materialised.
	filterGPUView        gpucontext.TextureView
	filterGPUW           int
	filterGPUH           int
	filterGPURelease     func()
	filterGPUReleasePrev func() // one-frame delayed free (present sampling)
	pixmapFilterStale    bool
	filterSrcScratch     []byte
	// Retained offscreen used as ApplyBlur GPU seed (no per-call CreateOffscreen).
	filterSrcView    gpucontext.TextureView
	filterSrcW       int
	filterSrcH       int
	filterSrcRelease func()

	// After FlushGPUWithView* the latest pixels live on lastFlushedView; pixmap
	// may lag until Image/SavePNG/readback. viewContentAheadOfPixmap tracks that.
	viewContentAheadOfPixmap bool
	lastFlushedView          gpucontext.TextureView
	lastFlushedViewW         int
	lastFlushedViewH         int

	// effectSurface: continuous effect offscreen (SetEffectSurface).
	// F14: FlushGPU publishes into a pooled TextureBinding RT and attaches
	// filterGPUView for zero-readback DrawGPUTexture present; pixmap stays stale
	// until Export/Image materialize.
	effectSurface bool

	// midFrameNilFlush is set when FlushGPU (nil view) absorbed pending draws
	// into the pixmap. ApplyImageFilterGraph must seed from pixmap afterward
	// (D105/D140), not treat remaining pending as the full surface.
	midFrameNilFlush bool

	// Lifecycle
	closed bool // Indicates whether Close has been called
}

// RenderPathStats counts how draw operations were routed for this Context.
type RenderPathStats struct {
	GPUOps                int    // ops successfully queued/executed on GPU path
	CPUFallbackOps        int    // ops that had a GPU path available but fell back to CPU
	LastCPUFallbackReason string // diagnostic: most recent fallback reason
	// FrameFlushes counts FlushGPU / FlushGPUWithView* invocations since the
	// last ResetRenderPathStats or BeginFrame.
	FrameFlushes int
	// BrushBootstrapOps counts ColorAt-stage → GPU blit fills (G.04 / residual).
	// Not a hard cpu_fb — GPU still owns the composite; reason is explicit.
	BrushBootstrapOps        int
	LastBrushBootstrapReason string
}

