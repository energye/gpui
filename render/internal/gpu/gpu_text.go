//go:build !nogpu

package gpu

import (
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/msdf"
)

// GPUTextEngine manages MSDF atlas generation and text layout for GPU
// rendering. It bridges the text shaping infrastructure (Face, Glyph) with
// the GPU text pipeline (TextBatch, TextQuad).
//
// Usage flow:
//  1. Call LayoutText to convert a string into a TextBatch (shapes glyphs,
//     generates MSDF textures into atlas, builds quads).
//  2. Before Flush, call DirtyAtlases/AtlasRGBAData/MarkClean to upload
//     modified atlas pages to GPU textures.
//  3. Pass the resulting TextBatch slice to RenderFrame.
//
// GPUTextEngine is safe for concurrent use.
type GPUTextEngine struct {
	mu sync.Mutex

	atlasManager    *msdf.AtlasManager
	cjkAtlasManager *msdf.AtlasManager // ADR-027: separate atlas for CJK (128px reference)
	extractor       *text.OutlineExtractor

	// msdfSize is the MSDF texture size per glyph cell (in pixels).
	msdfSize    int
	msdfSizeCJK int // ADR-027: 128px for CJK display text

	// pxRange is the MSDF distance range in pixels (typically 4.0).
	pxRange float32

	// Grow-only quad scratch (avoids per-DrawString alloc on dynamic HUD).
	quadScratch []TextQuad

	// R7.5-style layout templates: stable strings skip reshape; high-churn labels
	// never populate the cache (see text.IsHighChurnLabel).
	layoutCache     map[glyphLayoutTemplateKey]*msdfLayoutTemplateEntry
	layoutCacheTick uint64
	layoutCacheSoft int
	layoutCacheHits uint64
	layoutCacheMiss uint64
}

// msdfLayoutTemplateEntry caches MSDF quads at a base origin for rebase.
type msdfLayoutTemplateEntry struct {
	baseX, baseY float64
	quads        []TextQuad
	atlasIndex   int
	atlasSize    float32
	atime        uint64
}

// NewGPUTextEngine creates a new GPU text engine with default configuration.
func NewGPUTextEngine() *GPUTextEngine {
	const glyphSize = 64
	const glyphSizeCJK = 128 // ADR-027: 2x for dense CJK strokes (MapLibre pattern)
	const pxRange = 4.0

	cfg := msdf.DefaultAtlasConfig()
	cfg.GlyphSize = glyphSize
	mgr, _ := msdf.NewAtlasManager(cfg)
	genCfg := mgr.Generator().Config()
	genCfg.Range = pxRange
	mgr.Generator().SetConfig(genCfg)

	// ADR-027: CJK display text uses 128px reference with 2048 texture.
	// Single atlas manager with larger reference — CJK glyphs that reach MSDF
	// (>64px display text) are rare and benefit from 2x resolution.
	// Body text CJK (≤64px) routes to Tier 6 bitmap, never reaches here.
	cjkCfg := msdf.DefaultAtlasConfig()
	cjkCfg.GlyphSize = glyphSizeCJK
	cjkCfg.Size = 2048
	cjkMgr, _ := msdf.NewAtlasManager(cjkCfg)
	cjkGenCfg := cjkMgr.Generator().Config()
	cjkGenCfg.Range = pxRange
	cjkMgr.Generator().SetConfig(cjkGenCfg)

	return &GPUTextEngine{
		atlasManager:    mgr,
		cjkAtlasManager: cjkMgr,
		extractor:       text.NewOutlineExtractor(),
		msdfSize:        glyphSize,
		msdfSizeCJK:     glyphSizeCJK,
		pxRange:         pxRange,
		layoutCache:     make(map[glyphLayoutTemplateKey]*msdfLayoutTemplateEntry),
		layoutCacheSoft: 256,
	}
}

// LayoutText converts a text string with font face into a GPU-ready TextBatch.
// The text is shaped into glyphs, each glyph's MSDF is generated and packed
// into the atlas, and TextQuads are produced with user-space positions and
// atlas UV coordinates.
//
// Parameters:
//   - face: font face (provides glyph iteration and metrics)
//   - s: the string to render
//   - x, y: baseline origin in user-space coordinates
//   - color: text color as gg.RGBA
//   - viewportW, viewportH: viewport dimensions for building the ortho projection
//   - matrix: the context's current transformation matrix (CTM)
//
// The returned TextBatch contains quads in user-space coordinates. The
// Transform field is set to CTM x ortho_projection so the vertex shader
// transforms positions from user space to clip space. This ensures that
// Scale, Rotate, and Skew transforms applied to the drawing context affect
// text rendering correctly. The MSDF fragment shader's fwidth() automatically
// adapts to the composed transform for correct anti-aliasing.
//
// The deviceScale parameter scales the logical font size to physical pixels
// (e.g., 2.0 on a Retina display). This produces a higher screenPxRange in
// the MSDF shader, yielding crisper text on HiDPI displays. Quad positions
// remain in logical coordinates because the CTM already handles device scaling.
// Pass 1.0 for standard (non-HiDPI) rendering.
func (e *GPUTextEngine) LayoutText(
	face text.Face,
	s string,
	x, y float64,
	color render.RGBA,
	matrix render.Matrix,
	deviceScale float64,
) (TextBatch, error) {
	if face == nil || s == "" {
		return TextBatch{}, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	logicalSize := face.Size()
	if logicalSize <= 0 {
		logicalSize = 16 // fallback: never zero
	}
	fontSource := face.Source()
	fontID := computeFontID(fontSource)

	// ADR-027: detect CJK anywhere → select CJK atlas when mixed script.
	isCJK := false
	for _, r := range s {
		if text.IsCJKRune(r) {
			isCJK = true
			break
		}
	}
	activeAtlas := e.atlasManager
	atlasIndex := 0
	if isCJK {
		activeAtlas = e.cjkAtlasManager
		atlasIndex = cjkAtlasOffset
	}
	atlasConfig := activeAtlas.Config()

	// Layout template (pure translate): static HUD/list labels hit without reshape.
	// High-churn telemetry bypasses put (unique keys every frame).
	if key, ok := makeGlyphLayoutTemplateKey(s, fontID, logicalSize, deviceScale, false, false, text.HintingNone, matrix); ok {
		if batch, hit := e.msdfLayoutTemplateGet(key, x, y, color, matrix, atlasIndex, float32(atlasConfig.Size)); hit {
			return batch, nil
		}
	}

	quads := e.quadScratch[:0]
	var glyphCount, outlineSkip, atlasSkip, boundsSkip int

	// Scale ratio: outline is extracted at msdfSize, quad positions are in user space.
	// Use logicalSize because the CTM already handles device scaling.
	refSize := float64(e.msdfSize)
	refSizeCJK := float64(e.msdfSizeCJK)
	var ratio float64

	for glyph := range face.Glyphs(s) {
		glyphCount++

		// Match CPU text.Draw: GID 0 is .notdef — advance-only, no ink.
		if glyph.GID == 0 {
			continue
		}

		// ADR-027: CJK display text uses 128px reference for dense strokes.
		glyphRefSize := refSize
		glyphAtlas := activeAtlas
		glyphMsdfSize := e.msdfSize
		if text.IsCJKRune(glyph.Rune) {
			glyphRefSize = refSizeCJK
			glyphAtlas = e.cjkAtlasManager
			glyphMsdfSize = e.msdfSizeCJK
			ratio = logicalSize / refSizeCJK
		} else {
			ratio = logicalSize / refSize
		}

		key := msdf.GlyphKey{
			FontID:  fontID,
			GlyphID: uint16(glyph.GID),    //nolint:gosec // GlyphID is uint16
			Size:    int16(glyphMsdfSize), //nolint:gosec // msdfSize fits int16
		}

		// Hot path: reuse atlas cell without re-extracting outlines every frame
		// (dynamic HUD re-layouts unique strings but reuses digit/letter cells).
		region, ok := glyphAtlas.Lookup(key)
		if !ok {
			outline, err := e.extractor.ExtractOutline(fontSource.Parsed(), glyph.GID, glyphRefSize)
			if err != nil || outline == nil || outline.IsEmpty() {
				outlineSkip++
				continue
			}
			var gerr error
			region, gerr = glyphAtlas.Get(key, outline)
			if gerr != nil {
				slogger().Warn("MSDF atlas get failed", "gid", glyph.GID, "err", gerr)
				atlasSkip++
				continue
			}
		}

		// Skip empty/degenerate regions (e.g. space characters).
		if region.PlaneMaxX <= region.PlaneMinX || region.PlaneMaxY <= region.PlaneMinY {
			boundsSkip++
			continue
		}

		qx0 := float32(x + glyph.X + float64(region.PlaneMinX)*ratio)
		qx1 := float32(x + glyph.X + float64(region.PlaneMaxX)*ratio)
		qy0 := float32(y + float64(region.PlaneMinY)*ratio)
		qy1 := float32(y + float64(region.PlaneMaxY)*ratio)

		quads = append(quads, TextQuad{
			X0: qx0, Y0: qy0,
			X1: qx1, Y1: qy1,
			U0: region.U0, V0: region.V0,
			U1: region.U1, V1: region.V1,
		})
	}
	e.quadScratch = quads

	slogger().Debug("LayoutText result",
		"text", s, "glyphs", glyphCount,
		"quads", len(quads),
		"outlineSkip", outlineSkip, "atlasSkip", atlasSkip, "boundsSkip", boundsSkip)

	if len(quads) == 0 {
		return TextBatch{}, nil
	}

	// Caller-owned copy: template cache and multi-batch HUD must not alias scratch.
	out := make([]TextQuad, len(quads))
	copy(out, quads)

	batch := TextBatch{
		Quads:      out,
		Color:      color,
		Transform:  matrix,
		AtlasIndex: atlasIndex,
		PxRange:    e.pxRange,
		AtlasSize:  float32(atlasConfig.Size),
	}

	if key, ok := makeGlyphLayoutTemplateKey(s, fontID, logicalSize, deviceScale, false, false, text.HintingNone, matrix); ok {
		if !text.IsHighChurnLabel(s) {
			e.msdfLayoutTemplatePut(key, x, y, batch)
		}
	}
	return batch, nil
}

func (e *GPUTextEngine) msdfLayoutTemplateGet(
	key glyphLayoutTemplateKey,
	x, y float64,
	color render.RGBA,
	matrix render.Matrix,
	atlasIndex int,
	atlasSize float32,
) (TextBatch, bool) {
	if e.layoutCache == nil {
		return TextBatch{}, false
	}
	ent := e.layoutCache[key]
	if ent == nil {
		e.layoutCacheMiss++
		return TextBatch{}, false
	}
	dx := x - ent.baseX
	dy := y - ent.baseY
	// Pixel-safe rebase only for integer device deltas (deviceScale baked into positions).
	if dx != 0 || dy != 0 {
		// MSDF quads are in logical user space; require integer-pixel translation.
		if dx != float64(int64(dx)) || dy != float64(int64(dy)) {
			e.layoutCacheMiss++
			return TextBatch{}, false
		}
	}
	e.layoutCacheTick++
	ent.atime = e.layoutCacheTick
	e.layoutCacheHits++
	quads := make([]TextQuad, len(ent.quads))
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
	return TextBatch{
		Quads:      quads,
		Color:      color,
		Transform:  matrix,
		AtlasIndex: atlasIndex,
		PxRange:    e.pxRange,
		AtlasSize:  atlasSize,
	}, true
}

func (e *GPUTextEngine) msdfLayoutTemplatePut(key glyphLayoutTemplateKey, x, y float64, batch TextBatch) {
	if e.layoutCache == nil {
		e.layoutCache = make(map[glyphLayoutTemplateKey]*msdfLayoutTemplateEntry)
	}
	if len(batch.Quads) == 0 {
		return
	}
	cp := make([]TextQuad, len(batch.Quads))
	copy(cp, batch.Quads)
	e.layoutCacheTick++
	e.layoutCache[key] = &msdfLayoutTemplateEntry{
		baseX:      x,
		baseY:      y,
		quads:      cp,
		atlasIndex: batch.AtlasIndex,
		atlasSize:  batch.AtlasSize,
		atime:      e.layoutCacheTick,
	}
	if e.layoutCacheSoft <= 0 {
		e.layoutCacheSoft = 256
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

// cjkAtlasOffset is the index offset for CJK atlas pages.
// Latin atlas pages: 0..N-1, CJK atlas pages: cjkAtlasOffset..cjkAtlasOffset+M-1.
const cjkAtlasOffset = 100

// DirtyAtlases returns indices of atlases that have been modified since
// the last MarkClean call and need GPU upload. Includes both Latin and CJK atlases.
func (e *GPUTextEngine) DirtyAtlases() []int {
	e.mu.Lock()
	defer e.mu.Unlock()
	dirty := e.atlasManager.DirtyAtlases()
	for _, idx := range e.cjkAtlasManager.DirtyAtlases() {
		dirty = append(dirty, idx+cjkAtlasOffset)
	}
	return dirty
}

// AtlasRGBAData returns the atlas pixel data converted from RGB (3 bytes/pixel)
// to RGBA (4 bytes/pixel) suitable for GPU texture upload. Also returns the
// atlas dimensions. Indices ≥ cjkAtlasOffset refer to CJK atlas pages.
func (e *GPUTextEngine) AtlasRGBAData(index int) (data []byte, width, height int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var atlas *msdf.Atlas
	if index >= cjkAtlasOffset {
		atlas = e.cjkAtlasManager.GetAtlas(index - cjkAtlasOffset)
	} else {
		atlas = e.atlasManager.GetAtlas(index)
	}
	if atlas == nil {
		return nil, 0, 0
	}
	rgba := rgbToRGBA(atlas.Data, atlas.Size, atlas.Size)
	return rgba, atlas.Size, atlas.Size
}

// MarkClean marks an atlas as uploaded to GPU.
func (e *GPUTextEngine) MarkClean(index int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if index >= cjkAtlasOffset {
		e.cjkAtlasManager.MarkClean(index - cjkAtlasOffset)
	} else {
		e.atlasManager.MarkClean(index)
	}
}

// AtlasSize returns the atlas texture size (width = height).
func (e *GPUTextEngine) AtlasSize() int {
	return e.atlasManager.Config().Size
}

// PxRange returns the MSDF pixel range.
func (e *GPUTextEngine) PxRange() float32 {
	return e.pxRange
}

// GlyphCount returns the total number of cached glyphs across all atlases.
func (e *GPUTextEngine) GlyphCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.atlasManager.GlyphCount()
}

// computeFontID generates a stable hash identifier for a font source.
// Uses the full font name (includes subfamily like "Regular"/"Bold") and
// number of glyphs as a lightweight fingerprint. The full name is critical
// to distinguish fonts within the same family (e.g., "Go Regular" vs "Go Bold")
// that share the same family name and glyph count.
func computeFontID(source *text.FontSource) uint64 {
	if source == nil {
		return 0
	}
	h := fnv.New64a()
	parsed := source.Parsed()
	fullName := parsed.FullName()
	if fullName == "" {
		fullName = source.Name() // fallback to family name
	}
	_, _ = fmt.Fprintf(h, "%s:%d", fullName, parsed.NumGlyphs())
	return h.Sum64()
}
