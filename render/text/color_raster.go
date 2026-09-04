package text

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"

	xdraw "golang.org/x/image/draw"

	"github.com/energye/gpui/render/text/emoji"
)

// ErrNotColorGlyph is returned when a color raster is requested for a
// glyph the font renders as a plain outline.
var ErrNotColorGlyph = errors.New("text: glyph is not a color glyph")

// ColorRasterImage is one GPU-ready color glyph: premultiplied RGBA pixels
// plus baseline-relative placement at the requested PPEM.
type ColorRasterImage struct {
	Pix     *image.RGBA
	OriginX float32
	OriginY float32
	Advance float64
}

type colorRasterKey struct {
	font    ParsedFont
	glyphID uint16
	ppem    uint16
	palette int
	fg      color.RGBA
}

// ColorRasterCache rasterizes color glyphs once per (font, glyph, size,
// palette, foreground) and reuses the RGBA after that. Skia's color-glyph
// cache equivalent: CBDT bitmaps are decoded and scaled, COLR layers are
// flattened, both into premultiplied RGBA the GPU can upload directly.
type ColorRasterCache struct {
	mu      sync.Mutex
	entries map[colorRasterKey]*ColorRasterImage
	maxSize int
	raster  *GlyphMaskRasterizer
}

// NewColorRasterCache creates a cache holding up to maxSize entries.
func NewColorRasterCache(maxSize int) *ColorRasterCache {
	return &ColorRasterCache{
		entries: make(map[colorRasterKey]*ColorRasterImage),
		maxSize: maxSize,
		raster:  NewGlyphMaskRasterizer(),
	}
}

// Image returns the cached RGBA for a color glyph, rasterizing on miss.
// Foreground (text-color) COLR layers resolve to fg; CBDT glyphs ignore it.
// The mutex guards the map only: rasterization runs unlocked so concurrent
// misses do not serialize behind PNG decode/scale/COLR flattening.
func (c *ColorRasterCache) Image(font ParsedFont, glyphID uint16, ppem uint16, palette int, fg color.RGBA) (*ColorRasterImage, error) {
	cf, ok := font.(ColorFont)
	if !ok {
		return nil, ErrNotColorGlyph
	}
	key := colorRasterKey{font: font, glyphID: glyphID, ppem: ppem, palette: palette, fg: fg}
	c.mu.Lock()
	if img, ok := c.entries[key]; ok {
		c.mu.Unlock()
		return img, nil
	}
	c.mu.Unlock()
	img, err := c.rasterize(cf, font, glyphID, ppem, palette, fg)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.entries[key]; ok {
		return existing, nil
	}
	if len(c.entries) >= c.maxSize {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}
	c.entries[key] = img
	return img, nil
}

func (c *ColorRasterCache) rasterize(cf ColorFont, font ParsedFont, glyphID uint16, ppem uint16, palette int, fg color.RGBA) (*ColorRasterImage, error) {
	switch cf.GlyphType(glyphID) {
	case GlyphTypeBitmap:
		return rasterizeColorBitmap(cf, font, glyphID, ppem)
	case GlyphTypeCOLR:
		return c.rasterizeCOLR(cf, font, glyphID, ppem, palette, fg)
	default:
		return nil, fmt.Errorf("%w: gid %d", ErrNotColorGlyph, glyphID)
	}
}

func rasterizeColorBitmap(cf ColorFont, font ParsedFont, glyphID uint16, ppem uint16) (*ColorRasterImage, error) {
	bmp, err := cf.BitmapGlyph(glyphID, ppem)
	if err != nil {
		return nil, err
	}
	src, err := bmp.Decode()
	if err != nil {
		return nil, err
	}
	scale := float64(ppem) / float64(bmp.PPEM)
	if scale <= 0 {
		scale = 1
	}
	dw := int(float64(bmp.Width) * scale)
	dh := int(float64(bmp.Height) * scale)
	if dw <= 0 || dh <= 0 {
		return nil, fmt.Errorf("text: color bitmap scales to empty: %dx%d", dw, dh)
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return &ColorRasterImage{
		Pix:     dst,
		OriginX: bmp.OriginX * float32(scale),
		OriginY: bmp.OriginY * float32(scale),
		Advance: font.GlyphAdvance(glyphID, float64(ppem)),
	}, nil
}

func (c *ColorRasterCache) rasterizeCOLR(cf ColorFont, font ParsedFont, glyphID uint16, ppem uint16, palette int, fg color.RGBA) (*ColorRasterImage, error) {
	cg, err := cf.COLRGlyph(glyphID, palette)
	if err != nil {
		return nil, err
	}
	if len(cg.Layers) == 0 {
		return nil, fmt.Errorf("text: COLR glyph %d has no layers", glyphID)
	}
	type placed struct {
		mask *image.Alpha
		x, y int
		col  color.RGBA
	}
	var layers []placed
	baseY := 0
	for _, layer := range cg.Layers {
		res, err := c.raster.RasterizeHinted(font, GlyphID(layer.GlyphID), float64(ppem), 0, 0, HintingNone)
		if err != nil {
			return nil, err
		}
		if res == nil || res.Width <= 0 || res.Height <= 0 {
			continue
		}
		mask := image.NewAlpha(image.Rect(0, 0, res.Width, res.Height))
		copy(mask.Pix, res.Mask)
		layers = append(layers, placed{mask: mask, x: int(math.Round(float64(res.BearingX))), y: int(math.Round(float64(res.BearingY))), col: colrLayerColor(layer, fg)})
		if layers[len(layers)-1].y > baseY {
			baseY = layers[len(layers)-1].y
		}
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("text: COLR glyph %d rasterizes empty", glyphID)
	}
	w, h := 0, 0
	for _, l := range layers {
		if l.x+l.mask.Bounds().Dx() > w {
			w = l.x + l.mask.Bounds().Dx()
		}
		if baseY-l.y+l.mask.Bounds().Dy() > h {
			h = baseY - l.y + l.mask.Bounds().Dy()
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for _, l := range layers {
		r := image.Rect(l.x, baseY-l.y, l.x+l.mask.Bounds().Dx(), baseY-l.y+l.mask.Bounds().Dy())
		draw.DrawMask(dst, r, image.NewUniform(l.col), image.Point{}, l.mask, image.Point{}, draw.Over)
	}
	return &ColorRasterImage{
		Pix:     dst,
		OriginX: 0,
		OriginY: float32(baseY),
		Advance: font.GlyphAdvance(glyphID, float64(ppem)),
	}, nil
}

func colrLayerColor(layer emoji.ColorLayer, fg color.RGBA) color.RGBA {
	if layer.IsForeground() {
		return fg
	}
	return layer.Color.ToRGBA()
}
