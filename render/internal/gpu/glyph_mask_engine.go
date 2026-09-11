//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"sync"
	"unsafe"

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

	// R7.5: origin-free layout template cache for scroll/HUD reuse.
	// Geometry is cached without absolute x/y/color; safe rebase translates quads.
	// Templates hold atlas UVs — layoutCacheAtlasGen tracks atlas.Generation() and
	// drops the cache when pages are reset/cleared (compact / LRU page reclaim).
	layoutCache         map[glyphLayoutTemplateKey]*glyphLayoutTemplateEntry
	layoutCacheTick     uint64
	layoutCacheSoft     int
	layoutCacheHits     uint64
	layoutCacheMiss     uint64
	layoutCacheAtlasGen uint64

	// opt26: cache computeGlyphMaskFontID by FontSource pointer (name+glyphCount hash).
	fontIDCache map[uintptr]uint64
}

// glyphLayoutTemplateKey identifies shaped+rasterized glyph geometry independent
// of draw origin and color (R7.5). Matrix must be pure translate to participate.
type glyphLayoutTemplateKey struct {
	textHash uint64
	fontID   uint64
	sizeBits uint32
	dsBits   uint32 // deviceScale
	flags    uint16 // lcd/aliased/hint bits
}

type glyphLayoutTemplateEntry struct {
	baseX, baseY   float64
	deviceScale    float64
	g0X, g0Y       float64 // shaped offset of first non-.notdef glyph (snap origin)
	canSnapRebase  bool    // full-hint !LCD with uniform shaped Y
	uniformShapedY bool
	quads          []GlyphMaskQuad
	isLCD          bool
	atlasW, atlasH float32
	page           int
	atime          uint64
}

// NewGlyphMaskEngine creates a new glyph mask engine with the default atlas
// configuration. LCD subpixel rendering is disabled by default (LCDLayoutNone).
func NewGlyphMaskEngine() *GlyphMaskEngine {
	return &GlyphMaskEngine{
		atlas:           text.NewGlyphMaskAtlasDefault(),
		rasterizer:      text.NewGlyphMaskRasterizer(),
		lcdLayout:       text.LCDLayoutNone,
		lcdFilter:       text.DefaultLCDFilter(),
		layoutCache:     make(map[glyphLayoutTemplateKey]*glyphLayoutTemplateEntry),
		layoutCacheSoft: 512,
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
		// R7.5: templates reference atlas UVs — drop them with the atlas.
		e.dropLayoutTemplateCache()
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
//   - color: text color as render.RGBA
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

	p, err := e.resolveGlyphMaskParams(face, s, color, matrix, deviceScale, false)
	if err != nil {
		return GlyphMaskBatch{}, err
	}
	fontSize, fontID, parsed := p.fontSize, p.fontID, p.parsed
	rasterScale := p.rasterScale
	isCJK, hinting := p.isCJK, p.hinting
	useLCD, lcdLayout, lcdFilter := p.useLCD, p.lcdLayout, p.lcdFilter
	batchColor := p.batchColor

	if cf, ok := colorFontOf(parsed); ok {
		for _, r := range s {
			if gid := parsed.GlyphIndex(r); cf.GlyphType(gid) != text.GlyphTypeOutline {
				return GlyphMaskBatch{}, fmt.Errorf("glyph mask: color glyph %d, use color path", gid)
			}
		}
	}

	// opt24: try layout template BEFORE LayoutGlyphs — shaped is unused on hit
	// (layoutTemplateGet only needs key+origin). Static HUD/list strings skip
	// shape entirely; dynamic strings still shape on miss.
	if key, ok := makeGlyphLayoutTemplateKey(s, fontID, fontSize, deviceScale, useLCD, false, hinting, matrix); ok {
		if batch, hit := e.layoutTemplateGet(key, nil, x, y, batchColor, matrix); hit {
			return batch, nil
		}
		// S6.5: LayoutGlyphs caches Face.Glyphs (shape-level).
		// R7.5: origin-free layout template + safe quad rebase for scroll/HUD.
		// Skip template put for high-churn telemetry strings (unique every frame).
		shaped := text.LayoutGlyphs(face, s)
		batch := e.layoutGlyphs(shaped, x, y, fontSize, fontID, parsed, hinting, useLCD, lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, false, false)
		if !text.IsHighChurnLabel(s) {
			e.layoutTemplatePut(key, shaped, x, y, deviceScale, hinting, useLCD, batch)
		}
		return batch, nil
	}
	shaped := text.LayoutGlyphs(face, s)
	return e.layoutGlyphs(shaped, x, y, fontSize, fontID, parsed, hinting, useLCD, lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, false, false), nil
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

	// Aliased text never uses LCD subpixel rendering — binary coverage
	// is incompatible with 3x horizontal oversampling.
	p, err := e.resolveGlyphMaskParams(face, s, color, matrix, deviceScale, true)
	if err != nil {
		return GlyphMaskBatch{}, err
	}
	fontSize, fontID, parsed := p.fontSize, p.fontID, p.parsed
	rasterScale := p.rasterScale
	isCJK, hinting := p.isCJK, p.hinting
	useLCD, lcdLayout, lcdFilter := p.useLCD, p.lcdLayout, p.lcdFilter
	batchColor := p.batchColor

	if cf, ok := colorFontOf(parsed); ok {
		for _, r := range s {
			if gid := parsed.GlyphIndex(r); cf.GlyphType(gid) != text.GlyphTypeOutline {
				return GlyphMaskBatch{}, fmt.Errorf("glyph mask: color glyph %d, use color path", gid)
			}
		}
	}

	// opt24: template hit before shape (same as LayoutText).
	if key, ok := makeGlyphLayoutTemplateKey(s, fontID, fontSize, deviceScale, useLCD, true, hinting, matrix); ok {
		if batch, hit := e.layoutTemplateGet(key, nil, x, y, batchColor, matrix); hit {
			return batch, nil
		}
		// S6.5 shape cache + R7.5 layout template (aliased flag in key).
		shaped := text.LayoutGlyphs(face, s)
		batch := e.layoutGlyphs(shaped, x, y, fontSize, fontID, parsed, hinting, useLCD, lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, true, false)
		if !text.IsHighChurnLabel(s) {
			e.layoutTemplatePut(key, shaped, x, y, deviceScale, hinting, useLCD, batch)
		}
		return batch, nil
	}
	shaped := text.LayoutGlyphs(face, s)
	return e.layoutGlyphs(shaped, x, y, fontSize, fontID, parsed, hinting, useLCD, lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, true, false), nil
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

	p, err := e.resolveGlyphMaskParams(face, "", color, matrix, deviceScale, false)
	if err != nil {
		return GlyphMaskBatch{}, err
	}
	fontSize, fontID, parsed := p.fontSize, p.fontID, p.parsed
	rasterScale := p.rasterScale
	// p.isCJK was derived from an empty string (the shaped path carries no
	// source text); the caller's per-batch flag wins.
	hinting := p.hinting
	useLCD, lcdLayout, lcdFilter := p.useLCD, p.lcdLayout, p.lcdFilter
	batchColor := p.batchColor
	if cf, ok := colorFontOf(parsed); ok {
		for i := range glyphs {
			if cf.GlyphType(uint16(glyphs[i].GID)) != text.GlyphTypeOutline {
				return GlyphMaskBatch{}, fmt.Errorf("glyph mask: color glyph %d, use color path", uint16(glyphs[i].GID))
			}
		}
	}
	return e.layoutGlyphs(glyphs, x, y, fontSize, fontID, parsed, hinting, useLCD, lcdLayout, &lcdFilter, batchColor, matrix, deviceScale, rasterScale, isCJK, false, true), nil
}

// colorFontOf returns the color backend when the font carries CBDT/COLR
// tables. Ordinary fonts return false with zero allocation, keeping the
// hot mask path free of per-call slice building.
func colorFontOf(parsed text.ParsedFont) (text.ColorFont, bool) {
	cf, ok := parsed.(text.ColorFont)
	if !ok || !cf.HasColorTables() {
		return nil, false
	}
	return cf, true
}

// glyphMaskParams carries the common layout parameters resolved from a face
// and CTM, shared by LayoutText / LayoutTextAliased / LayoutShapedGlyphs so
// the three entry points cannot drift apart (hinting, LCD, color, metrics).
type glyphMaskParams struct {
	rasterScale float64
	fontSize    float64
	fontID      uint64
	parsed      text.ParsedFont
	isCJK       bool
	hinting     text.Hinting
	useLCD      bool
	lcdLayout   text.LCDLayout
	lcdFilter   text.LCDFilter
	batchColor  [4]float32
}

// resolveGlyphMaskParams derives the per-text-run rendering parameters from
// the face configuration and the current CTM: raster scale (from the matrix),
// glyph-mask font size, font ID + parsed font for rasterization, CJK
// detection, hinting (face-config, Skia single-cache semantic), LCD mode
// (disabled for aliased runs — binary coverage is incompatible with 3x
// horizontal subpixel oversampling) and the premultiplied batch color.
func (e *GlyphMaskEngine) resolveGlyphMaskParams(face text.Face, s string, color render.RGBA, matrix render.Matrix, deviceScale float64, aliased bool) (glyphMaskParams, error) {
	rasterScale := glyphMaskRasterScale(matrix, deviceScale)
	fontSize := glyphMaskFontSize(face.Size(), deviceScale, rasterScale)
	fontSource := face.Source()
	if fontSource == nil {
		// MultiFace and other composites: caller should split runs (X.06).
		return glyphMaskParams{}, fmt.Errorf("glyph mask: face has no FontSource")
	}
	fontID := e.fontID(fontSource)
	parsed := fontSource.Parsed()
	if parsed == nil {
		return glyphMaskParams{}, fmt.Errorf("glyph mask: parsed font unavailable")
	}
	isCJK := stringContainsCJK(s)
	hinting := selectGlyphMaskHinting(fontSize, matrix, isCJK, deviceScale, face.Hinting())
	useLCD := e.lcdLayout != text.LCDLayoutNone && selectGlyphMaskLCD(fontSize, matrix)
	if aliased {
		useLCD = false
	}
	premul := color.Premultiply()
	batchColor := [4]float32{
		float32(premul.R), float32(premul.G),
		float32(premul.B), float32(premul.A),
	}
	return glyphMaskParams{
		rasterScale: rasterScale,
		fontSize:    fontSize,
		fontID:      fontID,
		parsed:      parsed,
		isCJK:       isCJK,
		hinting:     hinting,
		useLCD:      useLCD,
		lcdLayout:   e.lcdLayout,
		lcdFilter:   e.lcdFilter,
		batchColor:  batchColor,
	}, nil
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
// pixel-snapping. Y is always snapped to the pixel grid (FreeType renders on
// an integer baseline for every mode, including no-hint). X is snapped to the
// precomputed rounded-advance grid (snappedDevX) when snapX is set, so vertical
// stems stay crisp while spacing remains even; LCD leaves X fractional to pick
// the RGB subpixel phase. The fraction MUST be measured in device space: the
// mask is rasterized at device size and the quad is scaled by deviceScale at
// flush.
//
// devScaleX can differ from devScaleY for scaled CTMs: X sub-pixel phase is
// measured at the raster resolution (deviceScale*rasterScale) so the mask's
// pixel grid aligns with CPU text.Draw's continuous placement, while Y keeps
// the axis deviceScale (integer baseline, CPU parity at scaled sizes).
func glyphPlacement(absX, absY, devScaleX, devScaleY float64, hinting text.Hinting, snappedDevX float64, snapX bool) (px, py, fracX, fracY float64) {
	devX := absX * devScaleX
	devY := absY * devScaleY
	fracX = devX - math.Floor(devX)
	fracY = devY - math.Floor(devY)
	fracY = 0
	absY = math.Round(devY) / devScaleY
	if snapX {
		fracX = 0
		absX = snappedDevX / devScaleX
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
	// honorShapedX places glyphs at their shaped X (kerning, ligatures,
	// mark attachment) instead of walking hinted advances. True only for
	// the pre-shaped submit (LayoutShapedGlyphs); the string entries keep
	// the hint-advance pen walk for CPU text.Draw parity.
	honorShapedX bool,
) GlyphMaskBatch {
	quads := e.quadScratch[:0]
	var batchIsLCD bool

	// Hinted non-LCD masks place glyphs on an integer pixel grid using the
	// SAME pen as CPU text.Draw (drawGlyphs): snapPen starts at round(x)
	// and advances by round(hinted advance) per glyph. The hinted advance
	// (region.Advance) can differ from the raw hmtx advance (TT bytecode
	// hinting adjusts it, e.g. Noto CJK 'w' at 20px: hinted 16.04 vs hmtx
	// 12.12); using the hmtx value put GPU glyphs up to a pixel off the CPU
	// bitmap. Unhinted/LCD text keeps the shaped (hmtx) advance grid.
	//
	// For a scaled/transformed CTM the snap grid is DEVICE space
	// (deviceScale×rasterScale per user px): rounding on the pre-transform
	// user grid lets the CTM multiply the error (Scale(2,2) shifted glyphs
	// a full pixel right, the old ×rasterSize re-scale also halved scaled
	// advances). The advance then comes from the SAME TT hint cache as CPU
	// drawGlyphs (HintedAdvanceWidth, int-truncated ppem — the rasterizer's
	// float-ppem advance grids differ at non-integer sizes and drift after
	// rounding), so scaled snapPen matches CPU bit-exactly.
	pixelGrid := matrix.B == 0 && matrix.D == 0 && matrix.A == 1 && matrix.E == 1
	devScaleX := deviceScale
	if !pixelGrid {
		devScaleX = deviceScale * rasterScale
	}
	snapX := hinting != text.HintingNone && !useLCD
	// Shaped submit on the pixel grid is declared here because the grid
	// allocation below must know whether this batch takes the shaped
	// branch (which never reads the rounded-advance grid).
	shapedSnap := honorShapedX && pixelGrid
	pen := math.Round(x * devScaleX)
	var snappedDevX []float64
	// The shaped branch positions from its own anchor math, so skip the
	// per-batch grid allocation there; behavior is unchanged.
	if !snapX && !shapedSnap && len(glyphs) > 0 && pixelGrid {
		snappedDevX = snapXGrid(glyphs, x, deviceScale)
	}
	// Shaped submit on the pixel grid anchors on the first glyph: the batch
	// origin is snapped once and each glyph's shaped offset is rounded
	// absolutely. This keeps the run span identical to the caret table
	// (kerning, ligatures, suffix/CJK boundary within half a pixel, no
	// cumulative drift); equal fractional advances therefore alternate by
	// 1px with period 1/frac (e.g. 10.288px pitch shows 10,11,10… — the
	// visible fix is script-appropriate fallback so advances are near
	// integers, not a different rounding). Other modes keep the pen walk.
	anchorX := 0.0
	shapedOrigin := 0.0
	if shapedSnap && len(glyphs) > 0 {
		anchorX = glyphs[0].X
		shapedOrigin = math.Round((x + anchorX) * devScaleX)
	}

	for i := range glyphs {
		glyph := glyphs[i]
		// GID 0 is .notdef. CPU text.Draw skips it (no ink) but still advances
		// the hint pen with the (unhinted) advance — TT phantom-only outlines
		// give the integer advance; non-TT fonts fall back to the shaped diff.
		if glyph.GID == 0 {
			if shapedSnap {
				// Shaped positions need no pen advance.
				continue
			}
			if snapX && i+1 < len(glyphs) {
				if pixelGrid {
					pen += math.Round((glyphs[i+1].X - glyphs[i].X) * deviceScale)
				} else if ha, ok := text.HintedAdvanceWidth(parsed, glyph.GID, fontSize); ok {
					pen += math.Round(ha)
				} else {
					pen += math.Round((glyphs[i+1].X - glyphs[i].X) * devScaleX)
				}
			}
			continue
		}
		// Compute the device-space placement and sub-pixel fraction for this
		// glyph, applying hinting pixel-snapping (see glyphPlacement). snapped
		// is only consulted when snapX is set.
		var snapped float64
		if shapedSnap {
			snapped = shapedOrigin + math.Round((glyph.X-anchorX)*devScaleX)
		} else if snapX {
			snapped = pen
		} else if len(snappedDevX) == len(glyphs) {
			snapped = snappedDevX[i]
		} else {
			// Unhinted scaled/transformed CTM: continuous device position
			// (fractional X picks the raster sub-pixel phase).
			snapped = (x + glyph.X) * devScaleX
		}
		absX, absY, fracX, fracY := glyphPlacement(x+glyph.X, y+glyph.Y, devScaleX, deviceScale, hinting, snapped, snapX)

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

		// Advance the hint grid pen with the HINTED advance, exactly like CPU
		// text.Draw (snapPen += round(outline.Advance)) — including empty
		// glyphs (spaces) which carry an advance but no ink. region.Advance is
		// the hinted advance when hinting is active; empty glyphs (spaces,
		// rasterized as a zero region) fall back to the shaped hmtx advance,
		// which equals the hinted one for ink-less glyphs. This keeps every
		// glyph x-position identical to the CPU bitmap on multi-glyph lines.
		if !shapedSnap && snapX && i+1 < len(glyphs) {
			if pixelGrid {
				adv := region.Advance
				if region.Width <= 0 || region.Height <= 0 {
					adv = float32(glyphs[i+1].X - glyphs[i].X)
				} else {
					// region.Advance is measured at rasterSize (which is
					// fontSize×rasterScale, or a coarser bucket under atlas
					// pressure). The pen must advance in SOURCE-face units:
					// the quad is placed in user space and the CTM (including
					// the uniform scale folded into rasterScale) scales it once
					// at render time. Using the raster-size advance here would
					// double the scale and open huge gaps between glyphs
					// (e.g. Scale(2,2) rendered "Hello gg!" with 2× spacing).
					adv *= float32(fontSize / (rasterScale * rasterSize))
				}
				pen += math.Round(float64(adv) * deviceScale)
			} else {
				// Scaled/transformed CTM: CPU drawGlyphs parity — same TT
				// hint cache (int-truncated ppem), same round() per advance.
				// Non-TT fonts (or unhintable glyphs) fall back to the
				// rasterizer's float-ppem advance (or the shaped diff for
				// empty glyphs).
				adv := float64(region.Advance) * (fontSize / rasterSize)
				if ha, ok := text.HintedAdvanceWidth(parsed, glyph.GID, fontSize); ok {
					adv = ha
				} else if region.Width <= 0 || region.Height <= 0 {
					adv = (glyphs[i+1].X - glyphs[i].X) * devScaleX
				}
				pen += math.Round(adv)
			}
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
			Page: region.AtlasIndex,
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

	// Primary page for single-page batches; multi-page batches must be expanded
	// via SplitGlyphMaskBatchByPage before Queue (DrawGlyphMaskText does this).
	page := quads[0].Page
	for i := 1; i < len(quads); i++ {
		if quads[i].Page != page {
			// Mixed pages — leave AtlasPageIndex as first page; callers split.
			break
		}
	}

	return GlyphMaskBatch{
		Quads:          quads,
		Transform:      matrix,
		Color:          batchColor,
		IsLCD:          batchIsLCD,
		AtlasWidth:     atlasSize,
		AtlasHeight:    atlasSize,
		AtlasPageIndex: page,
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
		return e.atlas.GetOrRasterize(key, func() ([]byte, int, int, float32, float32, float32, error) {
			result, err := e.rasterizer.RasterizeAliased(parsed, gid, size, fracX, fracY, hinting)
			if err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}
			if result == nil {
				return nil, 0, 0, 0, 0, 0, nil // empty glyph (space)
			}
			return result.Mask, result.Width, result.Height, result.BearingX, result.BearingY, result.Advance, nil
		})
	default:
		return e.atlas.GetOrRasterize(key, func() ([]byte, int, int, float32, float32, float32, error) {
			result, err := e.rasterizer.RasterizeHinted(parsed, gid, size, fracX, fracY, hinting)
			if err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}
			if result == nil {
				return nil, 0, 0, 0, 0, 0, nil // empty glyph (space)
			}
			return result.Mask, result.Width, result.Height, result.BearingX, result.BearingY, result.Advance, nil
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
	e.dropLayoutTemplateCache()
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

	// LCD layout keeps the shaped X phase (snapX=false) — pen advance unused.
	return e.atlas.PutLCD(key, result.Mask, result.Width, result.Height, result.BearingX, result.BearingY, 0)
}

// selectGlyphMaskHinting returns the hinting mode for glyph mask rendering.
// Hinting is enabled for axis-aligned text at any size (grid-fitting requires
// an aligned pixel grid). It is disabled only for rotated/skewed text and
// CJK HiDPI — see the rules in the function body.
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

func selectGlyphMaskHinting(fontSize float64, matrix render.Matrix, isCJK bool, deviceScale float64, faceHinting text.Hinting) text.Hinting {
	// The GPU glyph-mask rasterizer consumes the SAME hinting as the CPU
	// text.Draw path — the face's configured hinting (WithHinting, default
	// HintingFull) — Skia's single-glyph-cache semantic where CPU and GPU
	// share one strikemaker configuration. No size/script/DPI policy is
	// applied here: any such rule would diverge GPU output from the CPU
	// bitmap. Grid-fitting is dropped only when the pixel grid is not
	// axis-aligned (rotated/skewed CTM), where hinting cannot apply.
	//
	// 回退开关：GOGPU_TEXT_NO_HINT 强制 None（与 GOGPU_TEXT_NO_LCD 同模式）。
	if os.Getenv("GOGPU_TEXT_NO_HINT") != "" {
		return text.HintingNone
	}

	// Rotated/skewed text: grid-fitting requires an axis-aligned pixel grid.
	if matrix.B != 0 || matrix.D != 0 {
		return text.HintingNone
	}

	// CPU text.Draw parity: use the face configuration verbatim.
	return faceHinting
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
// Same identity as historical fmt.Fprintf("%s:%d", name, numGlyphs) FNV64a.
func computeGlyphMaskFontID(source *text.FontSource) uint64 {
	if source == nil {
		return 0
	}
	parsed := source.Parsed()
	fullName := parsed.FullName()
	if fullName == "" {
		fullName = source.Name()
	}
	// Avoid fmt.Fprintf on the hot path (opt26): Write name + ':' + decimal digits.
	h := fnv.New64a()
	_, _ = h.Write([]byte(fullName))
	_, _ = h.Write([]byte{':'})
	n := parsed.NumGlyphs()
	if n == 0 {
		_, _ = h.Write([]byte{'0'})
	} else {
		var dec [20]byte
		i := len(dec)
		for n > 0 {
			i--
			dec[i] = byte('0' + n%10)
			n /= 10
		}
		_, _ = h.Write(dec[i:])
	}
	return h.Sum64()
}

// fontID returns computeGlyphMaskFontID with per-engine pointer cache (opt26).
func (e *GlyphMaskEngine) fontID(source *text.FontSource) uint64 {
	if source == nil {
		return 0
	}
	p := uintptr(unsafe.Pointer(source))
	if e.fontIDCache != nil {
		if id, ok := e.fontIDCache[p]; ok {
			return id
		}
	} else {
		e.fontIDCache = make(map[uintptr]uint64, 4)
	}
	id := computeGlyphMaskFontID(source)
	e.fontIDCache[p] = id
	return id
}

func makeGlyphLayoutTemplateKey(
	s string,
	fontID uint64,
	fontSize float64,
	deviceScale float64,
	useLCD, aliased bool,
	hinting text.Hinting,
	matrix render.Matrix,
) (glyphLayoutTemplateKey, bool) {
	// Only pure translate (HUD/list scroll). Rotation/scale need full layout.
	if matrix.A != 1 || matrix.E != 1 || matrix.B != 0 || matrix.D != 0 {
		return glyphLayoutTemplateKey{}, false
	}
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
	return glyphLayoutTemplateKey{
		textHash: h.Sum64(),
		fontID:   fontID,
		sizeBits: math.Float32bits(float32(fontSize)),
		dsBits:   math.Float32bits(float32(deviceScale)),
		flags:    flags,
	}, true
}

// layoutTemplateOriginOffsets extracts shaped offsets used as snap anchors.
func layoutTemplateOriginOffsets(shaped []text.ShapedGlyph) (g0X, g0Y float64, uniformY bool) {
	uniformY = true
	found := false
	var firstY float64
	for i := range shaped {
		if shaped[i].GID == 0 {
			continue
		}
		if !found {
			g0X = shaped[i].X
			g0Y = shaped[i].Y
			firstY = shaped[i].Y
			found = true
			continue
		}
		if shaped[i].Y != firstY {
			uniformY = false
		}
	}
	return g0X, g0Y, uniformY
}

// layoutRebaseDelta returns a uniform quad translation that preserves glyph
// masks (pixel-safe). Allowed when:
//  1. delta is an integer number of device pixels (any hint/LCD mode), or
//  2. full-hint !LCD snap path with uniform shaped Y (list scroll arbitrary dy).
func layoutRebaseDelta(ent *glyphLayoutTemplateEntry, x, y float64) (dx, dy float64, ok bool) {
	if ent == nil || len(ent.quads) == 0 {
		return 0, 0, false
	}
	ds := ent.deviceScale
	if ds <= 0 {
		ds = 1
	}

	// Fast path: whole device-pixel move preserves subpixel phase for every glyph.
	if ddx, okX := integerDeviceDelta(ent.baseX, x, ds); okX {
		if ddy, okY := integerDeviceDelta(ent.baseY, y, ds); okY {
			return ddx, ddy, true
		}
	}

	// Full-hint snap path: positions are grid-fitted; X shift is always uniform.
	// Y shift is uniform when all shaped Y offsets match (typical LTR runs).
	if ent.canSnapRebase {
		dx = (math.Round((x+ent.g0X)*ds) - math.Round((ent.baseX+ent.g0X)*ds)) / ds
		if ent.uniformShapedY {
			dy = (math.Round((y+ent.g0Y)*ds) - math.Round((ent.baseY+ent.g0Y)*ds)) / ds
			return dx, dy, true
		}
		// Non-uniform Y: only accept integer device Y (already failed above) or exact baseY.
		if y == ent.baseY {
			return dx, 0, true
		}
	}

	// Exact origin match (HUD sticky) — color-only reuse still pays off.
	if x == ent.baseX && y == ent.baseY {
		return 0, 0, true
	}
	return 0, 0, false
}

func integerDeviceDelta(base, next, ds float64) (delta float64, ok bool) {
	d := (next - base) * ds
	r := math.Round(d)
	if math.Abs(d-r) > 1e-6 {
		return 0, false
	}
	return r / ds, true
}

func (e *GlyphMaskEngine) layoutTemplateGet(
	key glyphLayoutTemplateKey,
	_ []text.ShapedGlyph,
	x, y float64,
	color [4]float32,
	matrix render.Matrix,
) (GlyphMaskBatch, bool) {
	e.syncLayoutTemplateCacheWithAtlas()
	if e.layoutCache == nil {
		return GlyphMaskBatch{}, false
	}
	ent := e.layoutCache[key]
	if ent == nil {
		e.layoutCacheMiss++
		return GlyphMaskBatch{}, false
	}
	dx, dy, ok := layoutRebaseDelta(ent, x, y)
	if !ok {
		e.layoutCacheMiss++
		return GlyphMaskBatch{}, false
	}
	e.layoutCacheTick++
	ent.atime = e.layoutCacheTick
	e.layoutCacheHits++
	// Keep every atlas page warm so compact() does not zero masks that this
	// template still references (templates do not call atlas.Get per glyph).
	if e.atlas != nil {
		e.atlas.TouchPage(ent.page)
		seen := map[int]struct{}{ent.page: {}}
		for i := range ent.quads {
			p := ent.quads[i].Page
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			e.atlas.TouchPage(p)
		}
	}

	// Caller-owned copy: tests and multi-batch HUD may hold two LayoutText
	// results; do not return engine.quadScratch (would alias/clobber).
	quads := make([]GlyphMaskQuad, len(ent.quads))
	if dx == 0 && dy == 0 {
		copy(quads, ent.quads)
	} else {
		fdx, fdy := float32(dx), float32(dy)
		for i, q := range ent.quads {
			q.X0 += fdx
			q.X1 += fdx
			q.Y0 += fdy
			q.Y1 += fdy
			quads[i] = q
		}
	}
	return GlyphMaskBatch{
		Quads:          quads,
		Transform:      matrix,
		Color:          color,
		IsLCD:          ent.isLCD,
		AtlasWidth:     ent.atlasW,
		AtlasHeight:    ent.atlasH,
		AtlasPageIndex: ent.page,
	}, true
}

func (e *GlyphMaskEngine) layoutTemplatePut(
	key glyphLayoutTemplateKey,
	shaped []text.ShapedGlyph,
	x, y float64,
	deviceScale float64,
	hinting text.Hinting,
	useLCD bool,
	batch GlyphMaskBatch,
) {
	e.syncLayoutTemplateCacheWithAtlas()
	if e.layoutCache == nil {
		e.layoutCache = make(map[glyphLayoutTemplateKey]*glyphLayoutTemplateEntry)
	}
	if len(batch.Quads) == 0 {
		return
	}
	cp := make([]GlyphMaskQuad, len(batch.Quads))
	copy(cp, batch.Quads)
	g0X, g0Y, uniformY := layoutTemplateOriginOffsets(shaped)
	canSnap := hinting == text.HintingFull && !useLCD
	e.layoutCacheTick++
	e.layoutCache[key] = &glyphLayoutTemplateEntry{
		baseX:          x,
		baseY:          y,
		deviceScale:    deviceScale,
		g0X:            g0X,
		g0Y:            g0Y,
		canSnapRebase:  canSnap,
		uniformShapedY: uniformY,
		quads:          cp,
		isLCD:          batch.IsLCD,
		atlasW:         batch.AtlasWidth,
		atlasH:         batch.AtlasHeight,
		page:           batch.AtlasPageIndex,
		atime:          e.layoutCacheTick,
	}
	if e.layoutCacheSoft <= 0 {
		e.layoutCacheSoft = 512
	}
	for len(e.layoutCache) > e.layoutCacheSoft {
		var oldestKey glyphLayoutTemplateKey
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

// syncLayoutTemplateCacheWithAtlas drops layout templates when the glyph atlas
// generation advances (page reset / Clear). Stale entries would keep UVs into
// recycled or zeroed atlas regions and corrupt HUD/static text after compact.
func (e *GlyphMaskEngine) syncLayoutTemplateCacheWithAtlas() {
	if e.atlas == nil {
		return
	}
	gen := e.atlas.Generation()
	if e.layoutCacheAtlasGen == gen {
		return
	}
	e.dropLayoutTemplateCache()
	e.layoutCacheAtlasGen = gen
}

func (e *GlyphMaskEngine) dropLayoutTemplateCache() {
	if len(e.layoutCache) > 0 {
		e.layoutCache = make(map[glyphLayoutTemplateKey]*glyphLayoutTemplateEntry)
	} else if e.layoutCache == nil {
		e.layoutCache = make(map[glyphLayoutTemplateKey]*glyphLayoutTemplateEntry)
	}
	// Keep soft limit / tick counters; only UV-bearing entries are invalid.
}

// LayoutTemplateCacheStats returns R7.5 layout template hit/miss counters.
func (e *GlyphMaskEngine) LayoutTemplateCacheStats() (hits, misses uint64, entries int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.layoutCacheHits, e.layoutCacheMiss, len(e.layoutCache)
}

// ResetLayoutTemplateCacheStats clears hit/miss counters (entries retained).
func (e *GlyphMaskEngine) ResetLayoutTemplateCacheStats() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.layoutCacheHits = 0
	e.layoutCacheMiss = 0
}

// PutTransformMask stores a whole-string alpha mask (kTransformedMask
// semantic: rotated/sheared text or the CPU Tier2 outline path) in the
// glyph-mask atlas. The mask is CPU-rasterized by the same Skia-AAA software
// filler the CPU uses, so the GPU quad reproduces the CPU output bit-exactly.
// The key is a content fingerprint (font + string + size + CTM + deviceScale),
// so repeated draws reuse the stored region. bearing/advance are zero — the
// whole-string quad is positioned by its device-space bounds, not per-glyph.
func (e *GlyphMaskEngine) PutTransformMask(
	face text.Face,
	s string,
	deviceScale float64,
	matrix render.Matrix,
	mask []byte,
	w, h int,
) (text.GlyphMaskRegion, error) {
	if face == nil || s == "" || w <= 0 || h <= 0 {
		return text.GlyphMaskRegion{}, render.ErrFallbackToCPU
	}
	source := face.Source()
	if source == nil {
		return text.GlyphMaskRegion{}, render.ErrFallbackToCPU
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	key := transformMaskKey(e.fontID(source), source.Name(), s, face.Size(), deviceScale, matrix)
	return e.atlas.Put(key, mask, w, h, 0, 0, 0)
}

// transformMaskKey builds a content fingerprint for a whole-string transform
// mask. The 96-bit space (64-bit FontID + 16-bit GlyphID + SizeQ4 + flags)
// makes collisions with glyph entries practically impossible; the string and
// CTM are folded into the hash so each distinct text+transform caches its
// own mask.
func transformMaskKey(fontID uint64, fontName, s string, fontSize, deviceScale float64, matrix render.Matrix) text.GlyphMaskKey {
	h := fnv.New64a()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], fontID)
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(fontName))
	_, _ = h.Write([]byte(s))
	for _, v := range [8]float64{fontSize, deviceScale, matrix.A, matrix.B, matrix.C, matrix.D, matrix.E, matrix.F} {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		_, _ = h.Write(buf[:])
	}
	fp := h.Sum64()
	var sizeQ4 int16
	switch {
	case fontSize < 0:
		sizeQ4 = 0
	case fontSize > 2047:
		sizeQ4 = 32767
	default:
		sizeQ4 = int16(fontSize * 16.0) //nolint:gosec // bounds checked above
	}
	return text.GlyphMaskKey{
		FontID:  fp,
		GlyphID: uint16(fp >> 48),
		SizeQ4:  sizeQ4,
	}
}
