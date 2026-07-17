//go:build !nogpu

package gpu

import (
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// GlyphMaskEngine manages the CPU-rasterized glyph mask atlas and produces
// GPU-ready GlyphMaskBatch data for Tier 6 rendering. It bridges the text
// shaping infrastructure (Face, Glyph) with the GPU glyph mask pipeline
// (GlyphMaskBatch, GlyphMaskQuad).
//
// Usage flow:
//  1. Call LayoutText to convert a string into a GlyphMaskBatch (shapes glyphs,
//     rasterizes missing glyphs into the R8 atlas, builds quads).
//  2. Before rendering, call SyncAtlasTextures to upload dirty atlas pages
//     to GPU textures.
//  3. Pass the resulting GlyphMaskBatch slice to RenderFrame.
//
// GlyphMaskEngine is safe for concurrent use.
type GlyphMaskEngine struct {
	mu sync.Mutex

	atlas      *text.GlyphMaskAtlas
	rasterizer *text.GlyphMaskRasterizer

	// LCD subpixel rendering configuration.
	lcdLayout text.LCDLayout
	lcdFilter text.LCDFilter

	// GPU textures for atlas pages. Index matches atlas page index.
	pageTextures []*webgpu.Texture
	pageViews    []*webgpu.TextureView

	// S4.2 upload convergence stats (last SyncAtlasTextures call).
	lastUploadBytes    int64
	lastUploadRegions  int
	lastPartialUploads int
	lastFullUploads    int
	totalUploadBytes   int64

	// opt13: reuse quad slice inside layoutGlyphs (callers must own a copy
	// before the next LayoutText — QueueGlyphMask copies into context store).
	quadScratch []GlyphMaskQuad

	// opt13: LayoutText batch cache for repeated HUD/static strings.
	layoutCache     map[glyphLayoutCacheKey]*glyphLayoutCacheEntry
	layoutCacheTick uint64
	layoutCacheSoft int
}

// glyphLayoutCacheKey identifies a LayoutText result for HUD/static reuse.
// Positions are quantized to 1/64 px; matrix must be pure translate (else no key).
type glyphLayoutCacheKey struct {
	textHash       uint64
	fontID         uint64
	sizeBits       uint32
	xQ             int32
	yQ             int32
	cr, cg, cb, ca uint32
	flags          uint16 // lcd/aliased/hint bits
}

type glyphLayoutCacheEntry struct {
	batch GlyphMaskBatch // Quads owned by entry (deep copy)
	atime uint64
}

// NewGlyphMaskEngine creates a new glyph mask engine with the default atlas
// configuration. LCD subpixel rendering is disabled by default (LCDLayoutNone).
func NewGlyphMaskEngine() *GlyphMaskEngine {
	return &GlyphMaskEngine{
		atlas:           text.NewGlyphMaskAtlasDefault(),
		rasterizer:      text.NewGlyphMaskRasterizer(),
		lcdLayout:       text.LCDLayoutNone,
		lcdFilter:       text.DefaultLCDFilter(),
		layoutCache:     make(map[glyphLayoutCacheKey]*glyphLayoutCacheEntry),
		layoutCacheSoft: 256,
	}
}

// SetLCDLayout sets the LCD subpixel layout for ClearType rendering.
// Use LCDLayoutRGB for most monitors, LCDLayoutBGR for rare BGR panels,
// or LCDLayoutNone to disable subpixel rendering (grayscale).
func (e *GlyphMaskEngine) SetLCDLayout(layout text.LCDLayout) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lcdLayout != layout {
		e.lcdLayout = layout
		// Clear atlas: existing masks were rasterized for different layout.
		e.atlas.Clear()
	}
}

// SetLCDFilter sets the LCD FIR filter for ClearType fringe reduction.
func (e *GlyphMaskEngine) SetLCDFilter(filter text.LCDFilter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lcdFilter = filter
}

// LCDLayout returns the current LCD subpixel layout.
func (e *GlyphMaskEngine) LCDLayout() text.LCDLayout {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lcdLayout
}

// LayoutText converts a text string with font face into a GPU-ready
// GlyphMaskBatch. The text is shaped into glyphs, each glyph is rasterized
// (or retrieved from cache) into the R8 alpha atlas, and GlyphMaskQuads are
// produced with screen-space positions and atlas UV coordinates.
//
// Parameters:
//   - face: font face (provides glyph iteration and metrics)
//   - s: the string to render
//   - x, y: baseline origin in user-space coordinates
//   - color: text color as gg.RGBA
//   - viewportW, viewportH: viewport dimensions for building the ortho projection
//   - matrix: the context's current transformation matrix (CTM)
//   - deviceScale: DPI scale factor (e.g., 2.0 on Retina)
//
// The returned GlyphMaskBatch contains quads in user-space coordinates. The
// Transform field is set to CTM x ortho_projection so the vertex shader
// transforms positions from user space to clip space.
func (e *GlyphMaskEngine) LayoutText(
	face text.Face,
	s string,
	x, y float64,
	color render.RGBA,
	matrix render.Matrix,
	deviceScale float64,
) (GlyphMaskBatch, error) {
	if face == nil || s == "" {
		return GlyphMaskBatch{}, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	rasterScale := glyphMaskRasterScale(matrix, deviceScale)
	fontSize := glyphMaskFontSize(face.Size(), deviceScale, rasterScale)
	fontSource := face.Source()
	if fontSource == nil {
		// MultiFace and other composites: caller should split runs (X.06).
		return GlyphMaskBatch{}, fmt.Errorf("glyph mask: face has no FontSource")
	}
	fontID := computeGlyphMaskFontID(fontSource)
	parsed := fontSource.Parsed()
	if parsed == nil {
		return GlyphMaskBatch{}, fmt.Errorf("glyph mask: parsed font unavailable")
	}

	// Detect CJK anywhere in the string (ADR-027). Mixed strings like
	// "12px: 中文" must still use CJK reduced hinting for dense strokes;
	// checking only the first rune mis-classifies Latin-prefixed body text.
	isCJK := stringContainsCJK(s)
	hinting := selectGlyphMaskHinting(fontSize, matrix, isCJK, deviceScale)

	useLCD := e.lcdLayout != text.LCDLayoutNone && selectGlyphMaskLCD(fontSize, matrix)
	lcdLayout := e.lcdLayout
	lcdFilter := e.lcdFilter

	premul := color.Premultiply()
	batchColor := [4]float32{
		float32(premul.R), float32(premul.G),
		float32(premul.B), float32(premul.A),
	}

	// S6.5: LayoutGlyphs caches Face.Glyphs results for repeated DrawString
	// (list scroll / static labels). Same cmap+advance semantics as before.
	// opt13: full batch cache for static/HUD strings (FPS line hits when value sticky).
	if key, ok := makeGlyphLayoutCacheKey(s, fontID, fontSize, batchColor, x, y, matrix, useLCD, false, hinting); ok {
		if batch, hit := e.layoutCacheGet(key); hit {
			return batch, nil
		}
		shaped := text.LayoutGlyphs(face, s)
		batch := e.layoutGlyphs(shaped, x, y, fontSize, fontID, parsed, hinting, useLCD, lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, false)
		e.layoutCachePut(key, batch)
		return batch, nil
	}
	shaped := text.LayoutGlyphs(face, s)
	return e.layoutGlyphs(shaped, x, y, fontSize, fontID, parsed, hinting, useLCD, lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, false), nil
}

// LayoutTextAliased converts a text string into a GlyphMaskBatch with binary
// (aliased) rasterization. Same pipeline as LayoutText but uses NoAAFiller
// (0/255 only) instead of AnalyticFiller (256-level AA). The aliased flag
// also sets GlyphMaskFlagAliased in the cache key so aliased and AA masks
// are cached separately.
//
// This implements Skia's SkFont::Edging::kAlias behavior for the Tier 6
// glyph mask pipeline.
func (e *GlyphMaskEngine) LayoutTextAliased(
	face text.Face,
	s string,
	x, y float64,
	color render.RGBA,
	matrix render.Matrix,
	deviceScale float64,
) (GlyphMaskBatch, error) {
	if face == nil || s == "" {
		return GlyphMaskBatch{}, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	rasterScale := glyphMaskRasterScale(matrix, deviceScale)
	fontSize := glyphMaskFontSize(face.Size(), deviceScale, rasterScale)
	fontSource := face.Source()
	fontID := computeGlyphMaskFontID(fontSource)
	parsed := fontSource.Parsed()

	isCJK := stringContainsCJK(s)
	hinting := selectGlyphMaskHinting(fontSize, matrix, isCJK, deviceScale)

	// Aliased text never uses LCD subpixel rendering — binary coverage
	// is incompatible with 3x horizontal oversampling.
	useLCD := false

	premul := color.Premultiply()
	batchColor := [4]float32{
		float32(premul.R), float32(premul.G),
		float32(premul.B), float32(premul.A),
	}

	// S6.5: shared layout-glyph cache (distinct from OT Shape cache by mode).
	shaped := text.LayoutGlyphs(face, s)

	lcdLayout := text.LCDLayoutNone
	var lcdFilter text.LCDFilter
	return e.layoutGlyphs(shaped, x, y, fontSize, fontID, parsed, hinting, useLCD, lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, true), nil
}

// LayoutShapedGlyphs lays out pre-shaped glyphs into a GlyphMaskBatch.
// Same as LayoutText but skips shaping — uses stored glyph IDs and positions.
// This implements the ADR-022 "shape once" guarantee for the GPU scene path.
// isCJK indicates whether the text contains CJK characters (ADR-027).
func (e *GlyphMaskEngine) LayoutShapedGlyphs(
	face text.Face,
	glyphs []text.ShapedGlyph,
	x, y float64,
	color render.RGBA,
	matrix render.Matrix,
	deviceScale float64,
	isCJK bool,
) (GlyphMaskBatch, error) {
	if face == nil || len(glyphs) == 0 {
		return GlyphMaskBatch{}, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	rasterScale := glyphMaskRasterScale(matrix, deviceScale)
	fontSize := glyphMaskFontSize(face.Size(), deviceScale, rasterScale)
	fontSource := face.Source()
	fontID := computeGlyphMaskFontID(fontSource)
	parsed := fontSource.Parsed()
	hinting := selectGlyphMaskHinting(fontSize, matrix, isCJK, deviceScale)
	useLCD := e.lcdLayout != text.LCDLayoutNone && selectGlyphMaskLCD(fontSize, matrix)

	premul := color.Premultiply()
	batchColor := [4]float32{
		float32(premul.R), float32(premul.G),
		float32(premul.B), float32(premul.A),
	}

	lcdFilter := e.lcdFilter
	return e.layoutGlyphs(glyphs, x, y, fontSize, fontID, parsed, hinting, useLCD, e.lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, false), nil
}

func glyphMaskRasterScale(matrix render.Matrix, deviceScale float64) float64 {
	if deviceScale <= 0 {
		deviceScale = 1.0
	}
	if matrix.B != 0 || matrix.D != 0 {
		return 1.0
	}
	scale := math.Abs(matrix.E) / deviceScale
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 1.0
	}
	return scale
}

func glyphMaskFontSize(faceSize, deviceScale, rasterScale float64) float64 {
	if deviceScale <= 0 {
		deviceScale = 1.0
	}
	if rasterScale <= 0 {
		rasterScale = 1.0
	}
	fontSize := faceSize * deviceScale * rasterScale
	if fontSize <= 0 {
		return faceSize
	}
	return fontSize
}

// snapXGrid precomputes the integer device-space X position for each glyph by
// accumulating ROUNDED advances. Rounding each glyph's absolute position
// independently would make adjacent advances jitter by ±1px and open visible
// gaps inside words ("anyway" -> "an yway"); rounding the advance makes every
// like-advance the same integer, so spacing is uniform while stems stay
// pixel-aligned and crisp. This is the standard hinted-text layout
// (FreeType/GDI integer advances).
func snapXGrid(glyphs []text.ShapedGlyph, x, deviceScale float64) []float64 {
	out := make([]float64, len(glyphs))
	pen := math.Round((x + glyphs[0].X) * deviceScale)
	for i := range glyphs {
		out[i] = pen
		if i+1 < len(glyphs) {
			pen += math.Round((glyphs[i+1].X - glyphs[i].X) * deviceScale)
		}
	}
	return out
}

// glyphPlacement computes the device-space position (returned in user space as
// absX/absY) and sub-pixel fraction for one glyph, applying hinting
// pixel-snapping. Y is snapped for any hinting (baseline / horizontal stems
// grid-fit to the pixel grid). X is snapped to the precomputed rounded-advance
// grid (snappedDevX) when snapX is set, so vertical stems stay crisp while
// spacing remains even. The fraction MUST be measured in device space: the mask
// is rasterized at device size and the quad is scaled by deviceScale at flush.
func glyphPlacement(absX, absY, deviceScale float64, hinting text.Hinting, snappedDevX float64, snapX bool) (px, py, fracX, fracY float64) {
	devX := absX * deviceScale
	devY := absY * deviceScale
	fracX = devX - math.Floor(devX)
	fracY = devY - math.Floor(devY)
	if hinting != text.HintingNone {
		fracY = 0
		absY = math.Round(devY) / deviceScale
	}
	if snapX {
		fracX = 0
		absX = snappedDevX / deviceScale
	}
	return absX, absY, fracX, fracY
}

// layoutGlyphs is the common implementation for LayoutText, LayoutTextAliased,
// and LayoutShapedGlyphs. Must be called with e.mu held.
//
// When aliased is true, glyphs are rasterized with binary coverage (0/255 only)
// using RasterizeAliased instead of RasterizeHinted, and the cache key has the
// GlyphMaskFlagAliased flag set to prevent mixing AA and aliased masks.
func (e *GlyphMaskEngine) layoutGlyphs(
	glyphs []text.ShapedGlyph,
	x, y float64,
	fontSize float64,
	fontID uint64,
	parsed text.ParsedFont,
	hinting text.Hinting,
	useLCD bool,
	lcdLayout text.LCDLayout,
	lcdFilter *text.LCDFilter,
	batchColor [4]float32,
	matrix render.Matrix,
	deviceScale float64,
	rasterScale float64,
	isCJK bool,
	aliased bool,
) GlyphMaskBatch {
	quads := e.quadScratch[:0]
	var batchIsLCD bool

	// Full hinting grid-fits stems to the integer pixel grid, so fully hinted
	// glyphs must be placed at integer device pixels (snapXGrid). LCD keeps
	// sub-pixel X (it selects the R/G/B phase), so it is excluded.
	snapX := hinting == text.HintingFull && !useLCD
	var snappedDevX []float64
	if snapX && len(glyphs) > 0 {
		snappedDevX = snapXGrid(glyphs, x, deviceScale)
	}

	for i := range glyphs {
		glyph := glyphs[i]
		// GID 0 is .notdef. CPU text.Draw skips it (no ink), only advancing the
		// pen. Rasterizing .notdef draws a tofu box, which massively inflates
		// coverage for CJK-only fonts that lack Latin glyphs (e.g. DroidSansFallback
		// mapping "12px:" to GID 0). Match CPU: skip missing glyphs.
		if glyph.GID == 0 {
			continue
		}
		// Compute the device-space placement and sub-pixel fraction for this
		// glyph, applying hinting pixel-snapping (see glyphPlacement). snapped
		// is only consulted when snapX is set.
		var snapped float64
		if snapX {
			snapped = snappedDevX[i]
		}
		absX, absY, fracX, fracY := glyphPlacement(x+glyph.X, y+glyph.Y, deviceScale, hinting, snapped, snapX)

		// Size bucket quantization (Skia pattern): under atlas pressure,
		// rasterize at a coarse bucket size and scale quads to actual size.
		// ADR-027: CJK glyphs always rasterize at exact size — bucket scaling
		// is visible on dense CJK strokes. Skia never buckets DirectMask glyphs.
		rasterSize := fontSize
		bucketScale := 1.0
		var key text.GlyphMaskKey
		if e.atlas.UnderPressure() && !isCJK {
			key = text.MakeGlyphMaskKeyBucketed(fontID, glyph.GID, fontSize, fracX, fracY)
			rasterSize = float64(key.SizeQ4) / 16.0
			if rasterSize > 0 {
				bucketScale = fontSize / rasterSize
			}
		} else {
			key = text.MakeGlyphMaskKey(fontID, glyph.GID, fontSize, fracX, fracY)
		}

		// Set mode flags in cache key so gray/LCD/aliased masks never collide.
		if aliased {
			key.Flags = text.GlyphMaskFlagAliased
		} else if useLCD {
			key.Flags = text.GlyphMaskFlagLCD
			if lcdLayout == text.LCDLayoutBGR {
				key.Flags |= text.GlyphMaskFlagLCDBGR
			}
		}

		region, rErr := e.rasterizeGlyph(key, parsed, glyph.GID, rasterSize, fracX, fracY, hinting, useLCD, aliased, *lcdFilter, lcdLayout)
		if rErr != nil {
			slogger().Warn("glyph mask rasterize failed", "gid", glyph.GID, "err", rErr)
			continue
		}

		// Empty glyph (e.g., space) — no quad needed.
		if region.Width <= 0 || region.Height <= 0 {
			continue
		}

		// Position the quad in user space using glyph bearings.
		// BearingX: offset from glyph origin to left edge of mask.
		// BearingY: offset from baseline to top edge of mask (positive = above).
		//
		// The mask was rasterized at deviceScale * rasterSize. We convert
		// mask pixel coordinates to user space by dividing by deviceScale and
		// the CTM Y scale baked into rasterSize, then scale by bucketScale to
		// match the actual display size.
		// In normal mode bucketScale=1.0 (no-op). In bucketed mode
		// bucketScale = actualSize/bucketSize (Skia strikeToSourceScale).
		scale := bucketScale / (deviceScale * rasterScale)

		// For LCD glyphs, the atlas region.Width is 3x the logical pixel width.
		// The screen quad width must use the logical width (region.Width / 3).
		regionLogicalW := region.Width
		if region.IsLCD {
			regionLogicalW = region.Width / 3
		}

		qx0 := float32(absX + float64(region.BearingX)*scale)
		qy0 := float32(absY - float64(region.BearingY)*scale) // flip Y: bearing is up, screen is down
		qx1 := qx0 + float32(float64(regionLogicalW)*scale)
		qy1 := qy0 + float32(float64(region.Height)*scale)

		if region.IsLCD {
			batchIsLCD = true
		}

		quads = append(quads, GlyphMaskQuad{
			X0: qx0, Y0: qy0,
			X1: qx1, Y1: qy1,
			U0: region.U0, V0: region.V0,
			U1: region.U1, V1: region.V1,
		})
	}
	e.quadScratch = quads

	if len(quads) == 0 {
		return GlyphMaskBatch{}
	}

	// Store device-space CTM only — ortho projection is deferred to flush time
	// when the actual render target dimensions are known (ADR-025, Skia sk_RTAdjust pattern).
	// This enables correct rendering to offscreen textures of any size.

	// Atlas dimensions for the LCD shader's texel stepping.
	atlasConfig := e.atlas.Config()
	atlasSize := float32(atlasConfig.Size)

	return GlyphMaskBatch{
		Quads:          quads,
		Transform:      matrix,
		Color:          batchColor,
		IsLCD:          batchIsLCD,
		AtlasWidth:     atlasSize,
		AtlasHeight:    atlasSize,
		AtlasPageIndex: 0, // Currently single page support (first page).
	}
}

// rasterizeGlyph dispatches glyph rasterization to the appropriate method
// based on rendering mode (LCD, aliased, or standard AA). Must be called
// with e.mu held.
func (e *GlyphMaskEngine) rasterizeGlyph(
	key text.GlyphMaskKey,
	parsed text.ParsedFont,
	gid text.GlyphID,
	size float64,
	fracX, fracY float64,
	hinting text.Hinting,
	useLCD, aliased bool,
	lcdFilter text.LCDFilter,
	lcdLayout text.LCDLayout,
) (text.GlyphMaskRegion, error) {
	switch {
	case useLCD:
		return e.rasterizeLCDGlyph(key, parsed, gid, size, fracX, fracY, hinting, lcdFilter, lcdLayout)
	case aliased:
		return e.atlas.GetOrRasterize(key, func() ([]byte, int, int, float32, float32, error) {
			result, err := e.rasterizer.RasterizeAliased(parsed, gid, size, fracX, fracY, hinting)
			if err != nil {
				return nil, 0, 0, 0, 0, err
			}
			if result == nil {
				return nil, 0, 0, 0, 0, nil // empty glyph (space)
			}
			return result.Mask, result.Width, result.Height, result.BearingX, result.BearingY, nil
		})
	default:
		return e.atlas.GetOrRasterize(key, func() ([]byte, int, int, float32, float32, error) {
			result, err := e.rasterizer.RasterizeHinted(parsed, gid, size, fracX, fracY, hinting)
			if err != nil {
				return nil, 0, 0, 0, 0, err
			}
			if result == nil {
				return nil, 0, 0, 0, 0, nil // empty glyph (space)
			}
			return result.Mask, result.Width, result.Height, result.BearingX, result.BearingY, nil
		})
	}
}

// SyncAtlasTextures uploads dirty atlas pages to the GPU as R8 textures.
// Must be called before rendering any glyph mask batches. Creates new
// textures on first use and re-uploads data when pages are modified.
//
// S4.2: prefers partial dirty-region uploads (with 256-byte row alignment)
// when the dirty area is <50% of the page; otherwise falls back to full-page
// upload. Advances the atlas frame after upload so LRU compaction can reclaim
// stale pages (Skia GrAtlasManager::postFlush pattern).
func (e *GlyphMaskEngine) SyncAtlasTextures(device *webgpu.Device, queue *webgpu.Queue) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	uploads := e.atlas.DirtyUploads()
	e.lastUploadBytes = 0
	e.lastUploadRegions = 0
	e.lastPartialUploads = 0
	e.lastFullUploads = 0
	if len(uploads) == 0 {
		// Still advance frame so compaction runs even on hit-only frames.
		e.atlas.AdvanceFrame()
		return nil
	}

	for _, up := range uploads {
		idx := up.Index
		r8Data, pageSize, _ := e.atlas.PageR8Data(idx)
		if r8Data == nil || pageSize == 0 {
			continue
		}

		// Ensure texture/view slices are large enough.
		for len(e.pageTextures) <= idx {
			e.pageTextures = append(e.pageTextures, nil)
			e.pageViews = append(e.pageViews, nil)
		}

		size := uint32(pageSize) //nolint:gosec // atlas size always fits uint32

		// Create texture on first use (always full page size).
		if e.pageTextures[idx] == nil {
			tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
				Label:         fmt.Sprintf("glyph_mask_atlas_%d", idx),
				Size:          webgpu.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1},
				MipLevelCount: 1,
				SampleCount:   1,
				Dimension:     types.TextureDimension2D,
				Format:        types.TextureFormatR8Unorm,
				Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
			})
			if err != nil {
				return fmt.Errorf("create glyph mask atlas texture %d: %w", idx, err)
			}
			e.pageTextures[idx] = tex

			view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
				Label:         fmt.Sprintf("glyph_mask_atlas_%d_view", idx),
				Format:        types.TextureFormatR8Unorm,
				Dimension:     types.TextureViewDimension2D,
				Aspect:        types.TextureAspectAll,
				MipLevelCount: 1,
			})
			if err != nil {
				return fmt.Errorf("create glyph mask atlas view %d: %w", idx, err)
			}
			e.pageViews[idx] = view
			// First create: must upload full page (texture is uninitialized).
			up.FullPage = true
			up.X, up.Y, up.W, up.H = 0, 0, pageSize, pageSize
		}

		var (
			uploadData  []byte
			originX     uint32
			originY     uint32
			extentW     uint32
			extentH     uint32
			bytesPerRow uint32
			rowsPerImg  uint32
			byteCount   int
		)

		if up.FullPage || up.W <= 0 || up.H <= 0 || up.W >= pageSize && up.H >= pageSize {
			uploadData = r8Data
			originX, originY = 0, 0
			extentW, extentH = size, size
			bytesPerRow = size
			rowsPerImg = size
			byteCount = pageSize * pageSize
			e.lastFullUploads++
		} else {
			// Partial upload with 256-byte row alignment (WebGPU multi-row rule).
			x, y, w, h := up.X, up.Y, up.W, up.H
			if x < 0 {
				x = 0
			}
			if y < 0 {
				y = 0
			}
			if x+w > pageSize {
				w = pageSize - x
			}
			if y+h > pageSize {
				h = pageSize - y
			}
			if w <= 0 || h <= 0 {
				e.atlas.MarkClean(idx)
				continue
			}
			alignedBPR := uint32((w + 255) &^ 255) //nolint:gosec
			if h == 1 {
				// Single-row copies may use tight packing.
				alignedBPR = uint32(w) //nolint:gosec
			}
			staging := make([]byte, int(alignedBPR)*h)
			for row := 0; row < h; row++ {
				src := (y+row)*pageSize + x
				dst := row * int(alignedBPR)
				copy(staging[dst:dst+w], r8Data[src:src+w])
			}
			uploadData = staging
			originX = uint32(x) //nolint:gosec
			originY = uint32(y) //nolint:gosec
			extentW = uint32(w) //nolint:gosec
			extentH = uint32(h) //nolint:gosec
			bytesPerRow = alignedBPR
			rowsPerImg = extentH
			byteCount = len(staging)
			e.lastPartialUploads++
		}

		if err := queue.WriteTexture(
			&webgpu.ImageCopyTexture{
				Texture:  e.pageTextures[idx],
				MipLevel: 0,
				Origin:   webgpu.Origin3D{X: originX, Y: originY, Z: 0},
			},
			uploadData,
			&webgpu.ImageDataLayout{
				Offset:       0,
				BytesPerRow:  bytesPerRow,
				RowsPerImage: rowsPerImg,
			},
			&webgpu.Extent3D{Width: extentW, Height: extentH, DepthOrArrayLayers: 1},
		); err != nil {
			return fmt.Errorf("upload glyph mask atlas %d: %w", idx, err)
		}

		e.lastUploadBytes += int64(byteCount)
		e.lastUploadRegions++
		e.atlas.MarkClean(idx)
	}

	e.totalUploadBytes += e.lastUploadBytes
	// Skia postFlush: advance frame after atlas work so stale pages compact.
	e.atlas.AdvanceFrame()
	return nil
}

// LastUploadStats returns S4.2 stats from the most recent SyncAtlasTextures.
func (e *GlyphMaskEngine) LastUploadStats() (bytes int64, regions, partial, full int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastUploadBytes, e.lastUploadRegions, e.lastPartialUploads, e.lastFullUploads
}

// AtlasStats returns hit/miss/entry/page counts from the underlying atlas.
func (e *GlyphMaskEngine) AtlasStats() (hits, misses uint64, entries, pages int) {
	return e.atlas.Stats()
}

// PageTextureView returns the GPU texture view for the given atlas page.
// Returns nil if the page has not been uploaded.
func (e *GlyphMaskEngine) PageTextureView(index int) *webgpu.TextureView {
	e.mu.Lock()
	defer e.mu.Unlock()
	if index < 0 || index >= len(e.pageViews) {
		return nil
	}
	return e.pageViews[index]
}

// Destroy releases all GPU textures held by the engine.
func (e *GlyphMaskEngine) Destroy(device *webgpu.Device) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, v := range e.pageViews {
		if v != nil {
			v.Release()
		}
	}
	e.pageViews = nil

	for _, t := range e.pageTextures {
		if t != nil {
			t.Release()
		}
	}
	e.pageTextures = nil

	e.atlas.Clear()
}

// Atlas returns the underlying glyph mask atlas (for testing/introspection).
func (e *GlyphMaskEngine) Atlas() *text.GlyphMaskAtlas {
	return e.atlas
}

// rasterizeLCDGlyph rasterizes a glyph with LCD subpixel rendering and stores
// the RGB coverage data in the R8 atlas at 3x width. Returns a cached region
// if already present.
func (e *GlyphMaskEngine) rasterizeLCDGlyph(
	key text.GlyphMaskKey,
	parsed text.ParsedFont,
	gid text.GlyphID,
	fontSize float64,
	fracX, fracY float64,
	hinting text.Hinting,
	filter text.LCDFilter,
	layout text.LCDLayout,
) (text.GlyphMaskRegion, error) {
	// Fast path: check cache.
	if region, ok := e.atlas.Get(key); ok {
		return region, nil
	}

	// Slow path: rasterize with LCD.
	result, err := e.rasterizer.RasterizeLCD(parsed, gid, fontSize, fracX, fracY, hinting, filter, layout)
	if err != nil {
		return text.GlyphMaskRegion{}, fmt.Errorf("lcd glyph rasterize: %w", err)
	}
	if result == nil {
		return text.GlyphMaskRegion{}, nil // empty glyph (space)
	}

	return e.atlas.PutLCD(key, result.Mask, result.Width, result.Height, result.BearingX, result.BearingY)
}

// glyphMaskHintingMaxSize is the maximum font size in device pixels for which
// hinting is auto-enabled. Above this size, outlines are smooth enough that
// grid-fitting provides no visual benefit and can introduce distortion.
const glyphMaskHintingMaxSize = 48.0

// selectGlyphMaskHinting returns the hinting mode for glyph mask rendering.
// Hinting is enabled for small text (≤48px) when the CTM is axis-aligned
// (no rotation or skew), since grid-fitting requires an aligned pixel grid.
//
// CJK text uses reduced hinting (ADR-027): full grid-fitting collapses thin
// CJK strokes. FreeType afcjk module applies Y-direction only; DirectWrite
// uses NATURAL_SYMMETRIC for unhinted CJK fonts; macOS ignores hinting entirely.
func stringContainsCJK(s string) bool {
	for _, r := range s {
		if text.IsCJKRune(r) {
			return true
		}
	}
	return false
}

func selectGlyphMaskHinting(fontSize float64, matrix render.Matrix, isCJK bool, deviceScale float64) text.Hinting {
	if matrix.B != 0 || matrix.D != 0 {
		return text.HintingNone
	}

	if fontSize > glyphMaskHintingMaxSize {
		return text.HintingNone
	}

	if isCJK {
		if deviceScale >= 2.0 {
			return text.HintingNone
		}
		return text.HintingVertical
	}

	// Full hinting grid-fits stems for crisp rendering. layoutGlyphs places
	// fully hinted glyphs on integer device pixels using rounded advances, so
	// the grid-fit stems stay pixel-aligned (crisp) while spacing stays even.
	return text.HintingFull
}

// glyphMaskLCDMaxSize is the maximum font size in device pixels for which
// LCD subpixel rendering is auto-enabled. Above this size, individual subpixels
// are large enough that per-channel alpha provides no visual benefit and the
// color fringing becomes more noticeable.
const glyphMaskLCDMaxSize = 48.0

// selectGlyphMaskLCD returns true if LCD subpixel rendering should be used.
// LCD rendering requires an axis-aligned matrix (no rotation/skew) and small
// font size (same conditions as hinting, since ClearType depends on the
// subpixel grid being axis-aligned).
func selectGlyphMaskLCD(fontSize float64, matrix render.Matrix) bool {
	// Dev override: force grayscale (disable LCD subpixel) for A/B testing.
	if os.Getenv("GOGPU_TEXT_NO_LCD") != "" {
		return false
	}
	// Rotated/skewed text: subpixel grid is not axis-aligned.
	if matrix.B != 0 || matrix.D != 0 {
		return false
	}
	// Large text: subpixels are big enough that per-channel alpha isn't needed.
	return fontSize <= glyphMaskLCDMaxSize
}

// computeGlyphMaskFontID generates a stable hash identifier for a font source.
// Uses the same approach as computeFontID in gpu_text.go.
func computeGlyphMaskFontID(source *text.FontSource) uint64 {
	if source == nil {
		return 0
	}
	h := fnv.New64a()
	parsed := source.Parsed()
	fullName := parsed.FullName()
	if fullName == "" {
		fullName = source.Name()
	}
	_, _ = fmt.Fprintf(h, "%s:%d", fullName, parsed.NumGlyphs())
	return h.Sum64()
}

func makeGlyphLayoutCacheKey(
	s string,
	fontID uint64,
	fontSize float64,
	color [4]float32,
	x, y float64,
	matrix render.Matrix,
	useLCD, aliased bool,
	hinting text.Hinting,
) (glyphLayoutCacheKey, bool) {
	// Only pure translate (HUD-style). Rotation/scale invalidates simple key.
	if matrix.A != 1 || matrix.E != 1 || matrix.B != 0 || matrix.D != 0 {
		return glyphLayoutCacheKey{}, false
	}
	// Bake translate into position.
	x += matrix.C
	y += matrix.F
	var flags uint16
	if useLCD {
		flags |= 1
	}
	if aliased {
		flags |= 2
	}
	flags |= uint16(hinting&0xFF) << 2
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return glyphLayoutCacheKey{
		textHash: h.Sum64(),
		fontID:   fontID,
		sizeBits: math.Float32bits(float32(fontSize)),
		xQ:       int32(math.Round(x * 64)),
		yQ:       int32(math.Round(y * 64)),
		cr:       math.Float32bits(color[0]),
		cg:       math.Float32bits(color[1]),
		cb:       math.Float32bits(color[2]),
		ca:       math.Float32bits(color[3]),
		flags:    flags,
	}, true
}

func (e *GlyphMaskEngine) layoutCacheGet(key glyphLayoutCacheKey) (GlyphMaskBatch, bool) {
	if e.layoutCache == nil {
		return GlyphMaskBatch{}, false
	}
	ent := e.layoutCache[key]
	if ent == nil {
		return GlyphMaskBatch{}, false
	}
	e.layoutCacheTick++
	ent.atime = e.layoutCacheTick
	// Return batch with Quads aliasing cache storage — QueueGlyphMask must copy.
	return ent.batch, true
}

func (e *GlyphMaskEngine) layoutCachePut(key glyphLayoutCacheKey, batch GlyphMaskBatch) {
	if e.layoutCache == nil {
		e.layoutCache = make(map[glyphLayoutCacheKey]*glyphLayoutCacheEntry)
	}
	// Deep-copy quads into entry storage.
	cp := make([]GlyphMaskQuad, len(batch.Quads))
	copy(cp, batch.Quads)
	stored := batch
	stored.Quads = cp
	e.layoutCacheTick++
	e.layoutCache[key] = &glyphLayoutCacheEntry{batch: stored, atime: e.layoutCacheTick}
	if e.layoutCacheSoft <= 0 {
		e.layoutCacheSoft = 256
	}
	for len(e.layoutCache) > e.layoutCacheSoft {
		// Evict oldest.
		var oldestKey glyphLayoutCacheKey
		var oldestTick uint64 = ^uint64(0)
		first := true
		for k, ent := range e.layoutCache {
			if first || ent.atime < oldestTick {
				oldestKey = k
				oldestTick = ent.atime
				first = false
			}
		}
		delete(e.layoutCache, oldestKey)
	}
}
