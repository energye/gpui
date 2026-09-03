package text

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
)

// Draw renders text to a destination image.
// Position (x, y) is the baseline origin.
// Supports sourceFace, MultiFace, and FilteredFace.
func Draw(dst draw.Image, text string, face Face, x, y float64, col color.Color) {
	if text == "" || face == nil {
		return
	}

	// Expand tabs to spaces for bitmap rendering.
	// font.Drawer maps \t to .notdef (tofu) because fonts lack a tab glyph.
	// Tab = globalTabWidth spaces (default: 8, matching CSS/Pango/POSIX).
	text = expandTabs(text)

	switch f := face.(type) {
	case *sourceFace:
		drawSourceFace(dst, text, f, x, y, col)
	case *MultiFace:
		drawMultiFace(dst, text, f, x, y, col)
	case *FilteredFace:
		drawFilteredFace(dst, text, f, x, y, col)
	}
}

// glyphRasterMode selects the rasterization coverage mode.
// Outline extraction and rasterization mode are orthogonal
// concerns (Skia pattern: SkFont::Edging is independent of font variations).
type glyphRasterMode int

const (
	rasterModeAA      glyphRasterMode = iota // 256-level analytic AA coverage
	rasterModeAliased                        // binary 0/255 coverage (Skia kAlias)
)

// glyphRasterizeFunc is the per-glyph rasterization callback used by drawGlyphs.
// It abstracts the difference between RasterizeHinted (256-level AA) and
// RasterizeAliased (binary coverage), allowing drawSourceFace and DrawAliased
// to share the glyph iteration and compositing loop.
type glyphRasterizeFunc func(
	rast *GlyphMaskRasterizer,
	pf ParsedFont,
	gid GlyphID,
	ppem float64,
	subpixelX, subpixelY float64,
	hinting Hinting,
) (*GlyphMaskResult, error)

// drawGlyphs is the shared per-glyph rendering loop for drawSourceFace and
// DrawAliased. Each glyph is individually rasterized via the provided callback,
// then composited at its precise subpixel position using draw.DrawMask.
//
// When TT bytecode hinting is active, glyph positions are computed from
// hinted advances (phantom[1].x - phantom[0].x after TT interpreter runs)
// rather than unhinted hmtx advances. This prevents outline/advance mismatch
// where hinted outlines are wider or narrower than the raw advance, causing
// letters to merge or have gaps at certain ppem values (e.g., 16px Segoe UI).
//
// This matches skrifa/FreeType/Skia: when TT hinting runs on a glyph, the
// hinted advance replaces the raw hmtx advance for positioning.
func drawGlyphs(
	dst draw.Image,
	sf *sourceFace,
	text string,
	x, y float64,
	col color.Color,
	rasterize glyphRasterizeFunc,
) {
	if vars := sf.Variations(); len(vars) > 0 {
		drawGlyphsVariable(dst, sf, text, x, y, col, vars, rasterModeAA)
		return
	}

	parsed := sf.source.Parsed()
	ppem := sf.size
	hinting := sf.config.hinting

	// Check if TT bytecode hinting is available for this font.
	// When it is, we need to use hinted advances for glyph positioning
	// to match the hinted outlines. Without this, the outline shape
	// (grid-fitted by TT interpreter) disagrees with the cursor advance
	// (raw hmtx), causing letters to merge or gap at certain sizes.
	var ttCache *ttHintCache
	if hinting != HintingNone {
		if ownFont, ok := parsed.(*ownParsedFont); ok {
			ttCache = ownFont.loadTTHintCache()
		}
	}

	rast := NewGlyphMaskRasterizer()
	src := image.NewUniform(col)

	// Vertical text (TTB/BTT): the face iterator already carries the vertical
	// position in glyph.Y (advance = vmtx height); adding the same advance to
	// advanceX would shift each glyph diagonally. Only horizontal runs advance X.
	isVertical := sf.Direction().IsVertical()

	advanceX := 0.0
	// Hinted text must be placed at integer device pixels (Skia pattern,
	// matches GPU glyphPlacement): fractional X splits 1px vertical stems
	// into two half-coverage columns in the rasterizer (FreeType shows the
	// same break at fracX=0.5), and hinted outlines are already grid-fitted
	// so Y must never be re-shifted. X snaps to the rounded-advance grid so
	// stems stay crisp while spacing stays even.
	snapPen := math.Round(x)
	for glyph := range sf.Glyphs(text) {
		adv := hintedOrRawAdvance(ttCache, glyph, ppem)
		if glyph.GID == 0 {
			// Space and other no-outline glyphs: use unhinted advance.
			// These have no TT bytecode and phantom points would be trivial.
			if !isVertical {
				advanceX += adv
				if hinting != HintingNone {
					snapPen += math.Round(adv)
				}
			}
			continue
		}

		// Position glyph using our tracked advance (which may be hinted).
		glyphX := x + advanceX
		glyphY := y + glyph.Y

		intX := math.Floor(glyphX)
		intY := math.Floor(glyphY)
		subpixelX := glyphX - intX
		subpixelY := glyphY - intY
		if hinting != HintingNone && !isVertical {
			intX = snapPen
			subpixelX = 0
			subpixelY = 0
		}

		result, err := rasterize(rast, parsed, glyph.GID, ppem, subpixelX, subpixelY, hinting)
		if err != nil || result == nil {
			if !isVertical {
				advanceX += adv
				if hinting != HintingNone {
					snapPen += math.Round(adv)
				}
			}
			continue
		}

		maskImg := &image.Alpha{
			Pix:    result.Mask,
			Stride: result.Width,
			Rect:   image.Rect(0, 0, result.Width, result.Height),
		}

		dstX := int(intX) + int(math.Round(float64(result.BearingX)))
		dstY := int(intY) - int(math.Round(float64(result.BearingY)))

		destRect := image.Rect(dstX, dstY, dstX+result.Width, dstY+result.Height)
		draw.DrawMask(dst, destRect, src, image.Point{}, maskImg, image.Point{}, draw.Over)

		// Advance cursor using hinted advance when TT hinting is active.
		if !isVertical {
			advanceX += adv
			if hinting != HintingNone {
				snapPen += math.Round(adv)
			}
		}
	}
}

// hintedOrRawAdvance returns the TT hinted advance for a glyph if available,
// otherwise falls back to the unhinted advance from the Glyphs() iterator.
//
// When TT bytecode hinting is active, the hinted advance (from phantom
// points after TT interpreter execution) matches the hinted outline shape.
// Using unhinted advances with hinted outlines causes positioning errors
// because the outline is grid-fitted but the advance is not.
//
// Reference: skrifa ScaledOutline::advance_width — returns hinted advance
// when hinting is active, raw advance otherwise.
func hintedOrRawAdvance(ttCache *ttHintCache, glyph Glyph, ppem float64) float64 {
	if ttCache != nil {
		if adv, ok := ttCache.hintedAdvanceWidth(uint16(glyph.GID), int32(ppem)); ok {
			return adv
		}
	}
	return glyph.Advance
}

// drawGlyphsVariable renders text using the own parser's gvar path for outline
// extraction with full hinting support. This path matches skrifa's unified
// load_simple architecture: gvar deltas are applied to unscaled points BEFORE
// scaling and TT bytecode hinting.
//
// Hinting priority (same as static fonts):
//  1. TT bytecode hinting with gvar-varied unscaled points
//  2. Auto-hinter on the gvar-varied outline
//  3. Grid-fit fallback
//
// The mode parameter selects coverage computation (Skia pattern: outline source
// and rasterization mode are orthogonal — variable fonts don't affect AA choice).
func drawGlyphsVariable(
	dst draw.Image,
	sf *sourceFace,
	text string,
	x, y float64,
	col color.Color,
	variations []FontVariation,
	mode glyphRasterMode,
) {
	source := sf.source
	parsed := source.Parsed()
	if _, ok := parsed.(*ownParsedFont); !ok {
		return
	}

	ppem := sf.size
	hinting := sf.config.hinting
	rast := NewGlyphMaskRasterizer()
	src := image.NewUniform(col)
	extractor := &OutlineExtractor{}

	advanceX := 0.0
	// Hinted variable text: integer device placement (matches drawGlyphs).
	snapPen := math.Round(x)
	for _, r := range text {
		if r < 0x20 && r != '\t' {
			continue
		}

		gid := GlyphID(parsed.GlyphIndex(r))
		if gid == 0 {
			// Use variable-aware advance for skipped glyphs.
			if vap, vapOK := parsed.(VariableAdvanceProvider); vapOK {
				advanceX += vap.GlyphAdvanceVar(uint16(gid), ppem, variations)
			} else {
				advanceX += parsed.GlyphAdvance(uint16(gid), ppem)
			}
			continue
		}

		glyphX := x + advanceX
		glyphY := y

		intX := math.Floor(glyphX)
		intY := math.Floor(glyphY)
		subpixelX := glyphX - intX
		subpixelY := glyphY - intY
		if hinting != HintingNone {
			intX = snapPen
			subpixelX = 0
			subpixelY = 0
		}

		// Unified gvar + hinting path (skrifa load_simple parity).
		// ExtractOutlineHintedVar applies gvar deltas THEN hinting in one pass.
		outline, _ := extractor.ExtractOutlineHintedVar(parsed, gid, ppem, hinting, variations)
		if outline == nil || outline.IsEmpty() {
			if outline != nil {
				advanceX += float64(outline.Advance)
			}
			continue
		}

		// A3 直通：hint 非 None 时优先走 hint.LightHintVar 26.6 定点 → ftgrays
		// 移植（RasterizeFT26），与 FT_Render_Glyph(light) 逐字节一致；不可用
		// 时回退老 float32 路径。CFF2 变体坐标经 variations 传入。
		var result *GlyphMaskResult
		var rErr error
		if mode == rasterModeAA && hinting != HintingNone {
			directRes, direct, dErr := rast.RasterizeHintedFT26(parsed, gid, ppem, hinting, variations)
			if dErr != nil {
				advanceX += float64(outline.Advance)
				continue
			}
			if direct {
				result = directRes
			}
		}
		if result == nil {
			switch mode {
			case rasterModeAliased:
				result, rErr = rast.RasterizeOutlineAliased(outline, subpixelX, subpixelY)
			default:
				result, rErr = rast.RasterizeOutline(outline, subpixelX, subpixelY)
			}
		}
		if rErr != nil || result == nil {
			advanceX += float64(outline.Advance)
			continue
		}

		maskImg := &image.Alpha{
			Pix:    result.Mask,
			Stride: result.Width,
			Rect:   image.Rect(0, 0, result.Width, result.Height),
		}

		dstX := int(intX) + int(math.Round(float64(result.BearingX)))
		dstY := int(intY) - int(math.Round(float64(result.BearingY)))

		destRect := image.Rect(dstX, dstY, dstX+result.Width, dstY+result.Height)
		draw.DrawMask(dst, destRect, src, image.Point{}, maskImg, image.Point{}, draw.Over)

		advanceX += float64(outline.Advance)
		if hinting != HintingNone {
			snapPen += math.Round(float64(outline.Advance))
		}
	}
}

// rasterizeHintedGlyph rasterizes a glyph with 256-level analytic AA coverage.
func rasterizeHintedGlyph(
	rast *GlyphMaskRasterizer,
	pf ParsedFont,
	gid GlyphID,
	ppem float64,
	subpixelX, subpixelY float64,
	hinting Hinting,
) (*GlyphMaskResult, error) {
	return rast.RasterizeHinted(pf, gid, ppem, subpixelX, subpixelY, hinting)
}

// rasterizeAliasedGlyph rasterizes a glyph with binary (0 or 255) coverage.
func rasterizeAliasedGlyph(
	rast *GlyphMaskRasterizer,
	pf ParsedFont,
	gid GlyphID,
	ppem float64,
	subpixelX, subpixelY float64,
	hinting Hinting,
) (*GlyphMaskResult, error) {
	return rast.RasterizeAliased(pf, gid, ppem, subpixelX, subpixelY, hinting)
}

// drawSourceFace renders text using per-glyph rasterization with fractional
// advances. Each glyph is individually rasterized via GlyphMaskRasterizer
// (256-level analytic AA with hinting), then composited at its precise
// subpixel position.
//
// This replaces the previous font.Drawer approach which used integer-rounded
// advances internally, causing letters to merge at small sizes (e.g., "Te"
// at 12px). The Glyphs() iterator now returns fractional X positions from
// HintingNone advances (ADR-039), while outline rasterization still uses
// the face's configured hinting for crisp stems.
func drawSourceFace(dst draw.Image, text string, sf *sourceFace, x, y float64, col color.Color) {
	drawGlyphs(dst, sf, text, x, y, col, rasterizeHintedGlyph)
}

// drawMultiFace renders text using a MultiFace, selecting the appropriate
// font for each rune. Consecutive runes resolved to the same face are merged
// into one draw call (mf.Runs), so mixed-script text does one rasterization
// pass per contiguous font run instead of one per rune (P5).
func drawMultiFace(dst draw.Image, text string, mf *MultiFace, x, y float64, col color.Color) {
	// Vertical (TTB/BTT): runs stack along Y (glyph.Y carries vmtx heights,
	// matching drawGlyphs); X stays fixed at the origin.
	vertical := mf.Direction().IsVertical()

	currentX := x
	currentY := y

	// Tabs already expanded to spaces by Draw() via expandTabs().
	for _, run := range mf.Runs(text) {
		switch f := run.Face.(type) {
		case *sourceFace:
			drawSourceFace(dst, run.Text, f, currentX, currentY, col)
		case *FilteredFace:
			drawFilteredFace(dst, run.Text, f, currentX, currentY, col)
		case *MultiFace:
			// Nested MultiFace (rare but possible)
			drawMultiFace(dst, run.Text, f, currentX, currentY, col)
		}

		if vertical {
			currentY += runAdvance(run.Face, run.Text)
		} else {
			currentX += runAdvance(run.Face, run.Text)
		}
	}
}

// drawFilteredFace renders text using a FilteredFace. Consecutive in-range
// runes are merged into one draw call; filtered-out runes break the run and
// contribute no advance (same layout as the previous per-rune loop).
func drawFilteredFace(dst draw.Image, text string, ff *FilteredFace, x, y float64, col color.Color) {
	vertical := ff.Direction().IsVertical()

	currentX := x
	currentY := y

	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		seg := b.String()

		// Render the segment at the current cursor position.
		switch f := ff.face.(type) {
		case *sourceFace:
			drawSourceFace(dst, seg, f, currentX, currentY, col)
		case *FilteredFace:
			drawFilteredFace(dst, seg, f, currentX, currentY, col)
		case *MultiFace:
			drawMultiFace(dst, seg, f, currentX, currentY, col)
		}

		if vertical {
			currentY += runAdvance(ff.face, seg)
		} else {
			currentX += runAdvance(ff.face, seg)
		}
		b.Reset()
	}

	// Tabs already expanded to spaces by Draw() via expandTabs().
	for _, r := range text {
		if !ff.inRanges(r) {
			flush() // Filtered rune: no draw, no advance, split the run.
			continue
		}
		b.WriteRune(r)
	}
	flush()
}

// runAdvance returns the total advance of text on face, matching how
// drawGlyphs moves the cursor: TT hinted advances when the face is a
// sourceFace with active horizontal hinting; otherwise the plain Glyphs
// iterator advances (vertical text carries vmtx heights in glyph.Advance).
func runAdvance(face Face, text string) float64 {
	sf, ok := face.(*sourceFace)
	if !ok {
		return advanceFromGlyphs(face, text)
	}
	// Vertical (TTB/BTT): drawGlyphs positions via glyph.Y (vmtx heights);
	// the TT hint cache only holds horizontal widths, so use raw advances.
	if sf.Direction().IsVertical() {
		return advanceFromGlyphs(face, text)
	}
	cache := loadFaceTTCache(sf)
	if cache == nil {
		return advanceFromGlyphs(face, text)
	}
	ppem := sf.size
	total := 0.0
	for glyph := range sf.Glyphs(text) {
		gid := uint16(glyph.GID)
		if v, ok := sf.lookupAdvance(gid, glyph.Rune, true); ok {
			total += v
			continue
		}
		var adv float64
		if hintAdv, hintOK := cache.hintedAdvanceWidth(gid, int32(ppem)); hintOK {
			adv = hintAdv
		} else {
			adv = glyph.Advance
		}
		sf.storeAdvance(gid, glyph.Rune, true, adv)
		total += adv
	}
	return total
}

// loadFaceTTCache returns the TT hint cache for a sourceFace, or nil if
// TT bytecode hinting is unavailable or disabled.
func loadFaceTTCache(sf *sourceFace) *ttHintCache {
	if sf.config.hinting == HintingNone {
		return nil
	}
	ownFont, ok := sf.source.Parsed().(*ownParsedFont)
	if !ok {
		return nil
	}
	return ownFont.loadTTHintCache()
}

// advanceFromGlyphs sums the Glyphs iterator advances (unhinted, direction-aware).
func advanceFromGlyphs(face Face, text string) float64 {
	total := 0.0
	for glyph := range face.Glyphs(text) {
		total += glyph.Advance
	}
	return total
}

// Measure returns the dimensions of text.
// Width is the horizontal advance, height is the font's line height.
func Measure(text string, face Face) (width, height float64) {
	if text == "" || face == nil {
		return 0, 0
	}

	// Get advance width
	width = face.Advance(text)

	// Get line height from metrics
	metrics := face.Metrics()
	height = metrics.LineHeight()

	return width, height
}

// DrawOptions provides advanced options for text drawing.
// Reserved for future enhancements.
type DrawOptions struct {
	// Color for the text (default: black)
	Color color.Color
}
