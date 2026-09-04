//go:build !nogpu

package gpu

import (
	"fmt"
	"image/color"
	"math"
	"sync"
	"unsafe"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// ColorGlyphEngine lays out color glyphs (CBDT bitmaps, COLR layers) into
// batches sampling RGBA color atlas pages. Outline glyphs are skipped —
// they stay in the mask pipeline. Errors follow the mask engine contract:
// the caller falls back to CPU rendering.
type ColorGlyphEngine struct {
	mu          sync.Mutex
	atlas       *ColorGlyphAtlas
	raster      *text.ColorRasterCache
	fontIDCache map[uintptr]uint64

	quadScratch []GlyphMaskQuad

	// GPU textures for color atlas pages. Index matches atlas page index.
	pageTextures []*webgpu.Texture
	pageViews    []*webgpu.TextureView
}

// NewColorGlyphEngine creates a color glyph engine with default atlas pages.
func NewColorGlyphEngine() *ColorGlyphEngine {
	return &ColorGlyphEngine{
		atlas:  NewColorGlyphAtlas(1024, 4),
		raster: text.NewColorRasterCache(256),
	}
}

// LayoutColorGlyphs converts pre-shaped glyphs into a color batch.
// Positions come from the shaped glyphs (unhinted); quads are axis-aligned
// in user space and the batch Transform carries rotation/scale to the GPU.
func (e *ColorGlyphEngine) LayoutColorGlyphs(face text.Face, glyphs []text.ShapedGlyph, x, y float64, color render.RGBA, matrix render.Matrix, deviceScale float64) (GlyphMaskBatch, error) {
	if face == nil || len(glyphs) == 0 {
		return GlyphMaskBatch{}, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	rasterScale := glyphMaskRasterScale(matrix, deviceScale)
	fontSize := glyphMaskFontSize(face.Size(), deviceScale, rasterScale)
	source := face.Source()
	if source == nil {
		return GlyphMaskBatch{}, fmt.Errorf("color glyph: face has no FontSource")
	}
	parsed := source.Parsed()
	if parsed == nil {
		return GlyphMaskBatch{}, fmt.Errorf("color glyph: parsed font unavailable")
	}
	cf, ok := parsed.(text.ColorFont)
	if !ok || !cf.HasColorTables() {
		return GlyphMaskBatch{}, fmt.Errorf("color glyph: font has no color tables")
	}

	ppem := uint16(math.Round(fontSize))
	if ppem < 1 {
		ppem = 1
	}
	scale := 1 / (deviceScale * rasterScale)
	if !(scale > 0) {
		scale = 1
	}
	fg := renderRGBAToColorRGBA(color)
	opacity := color.A
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	fontID := e.fontID(source)

	quads := e.quadScratch[:0]
	for i := range glyphs {
		glyph := glyphs[i]
		if cf.GlyphType(uint16(glyph.GID)) == text.GlyphTypeOutline {
			continue
		}
		img, err := e.raster.Image(parsed, uint16(glyph.GID), ppem, 0, fg)
		if err != nil {
			slogger().Warn("color glyph rasterize failed", "gid", uint16(glyph.GID), "err", err)
			continue
		}
		if img.Pix == nil || img.Pix.Bounds().Dx() <= 0 || img.Pix.Bounds().Dy() <= 0 {
			continue
		}
		key := ColorGlyphKey{FontID: fontID, GlyphID: uint16(glyph.GID), PPEM: ppem, Palette: 0, FG: fg}
		region, ok := e.atlas.Get(key)
		if !ok {
			var err error
			region, err = e.atlas.Put(key, img.Pix, img.OriginX, img.OriginY)
			if err != nil {
				return GlyphMaskBatch{}, err
			}
		}
		absX := x + glyph.X
		absY := y + glyph.Y
		qx0 := float32(absX + float64(img.OriginX)*scale)
		qy0 := float32(absY - float64(img.OriginY)*scale)
		qx1 := qx0 + float32(float64(img.Pix.Bounds().Dx())*scale)
		qy1 := qy0 + float32(float64(img.Pix.Bounds().Dy())*scale)
		quads = append(quads, GlyphMaskQuad{
			X0: qx0, Y0: qy0,
			X1: qx1, Y1: qy1,
			U0: region.U0, V0: region.V0,
			U1: region.U1, V1: region.V1,
			Page: region.Page,
		})
	}
	e.quadScratch = quads

	if len(quads) == 0 {
		return GlyphMaskBatch{}, nil
	}
	page := quads[0].Page
	for i := 1; i < len(quads); i++ {
		if quads[i].Page != page {
			break
		}
	}
	return GlyphMaskBatch{
		Quads:          quads,
		Transform:      matrix,
		Color:          [4]float32{1, 1, 1, float32(opacity)},
		IsColor:        true,
		AtlasPageIndex: page,
	}, nil
}

// fontID mirrors GlyphMaskEngine.fontID so mask and color paths agree.
func (e *ColorGlyphEngine) fontID(source *text.FontSource) uint64 {
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

func renderRGBAToColorRGBA(c render.RGBA) color.RGBA {
	clamp := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 255
		}
		return uint8(v*255 + 0.5)
	}
	return color.RGBA{R: clamp(c.R), G: clamp(c.G), B: clamp(c.B), A: clamp(c.A)}
}

// SyncColorAtlasTextures uploads dirty color atlas pages to the GPU as RGBA8
// textures. Mirrors GlyphMaskEngine.SyncAtlasTextures with 4 bytes per texel.
func (e *ColorGlyphEngine) SyncColorAtlasTextures(device *webgpu.Device, queue *webgpu.Queue) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	uploads := e.atlas.DirtyUploads()
	if len(uploads) == 0 {
		return nil
	}

	for _, up := range uploads {
		idx := up.Index
		rgbaData, pageSize := e.atlas.PageRGBAData(idx)
		if rgbaData == nil || pageSize == 0 {
			continue
		}

		for len(e.pageTextures) <= idx {
			e.pageTextures = append(e.pageTextures, nil)
			e.pageViews = append(e.pageViews, nil)
		}

		size := uint32(pageSize) //nolint:gosec // atlas size always fits uint32

		if e.pageTextures[idx] == nil {
			tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
				Label:         fmt.Sprintf("glyph_color_atlas_%d", idx),
				Size:          webgpu.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1},
				MipLevelCount: 1,
				SampleCount:   1,
				Dimension:     types.TextureDimension2D,
				Format:        types.TextureFormatRGBA8Unorm,
				Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
			})
			if err != nil {
				return fmt.Errorf("create glyph color atlas texture %d: %w", idx, err)
			}
			e.pageTextures[idx] = tex

			view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
				Label:         fmt.Sprintf("glyph_color_atlas_%d_view", idx),
				Format:        types.TextureFormatRGBA8Unorm,
				Dimension:     types.TextureViewDimension2D,
				Aspect:        types.TextureAspectAll,
				MipLevelCount: 1,
			})
			if err != nil {
				return fmt.Errorf("create glyph color atlas view %d: %w", idx, err)
			}
			e.pageViews[idx] = view
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
		)

		if up.FullPage || up.W <= 0 || up.H <= 0 || up.W >= pageSize && up.H >= pageSize {
			uploadData = rgbaData
			originX, originY = 0, 0
			extentW, extentH = size, size
			bytesPerRow = size * 4
			rowsPerImg = size
		} else {
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
			rowBytes := w * 4
			alignedBPR := uint32((rowBytes + 255) &^ 255) //nolint:gosec
			if h == 1 {
				alignedBPR = uint32(rowBytes) //nolint:gosec
			}
			staging := make([]byte, int(alignedBPR)*h)
			for row := 0; row < h; row++ {
				src := ((y+row)*pageSize + x) * 4
				dst := row * int(alignedBPR)
				copy(staging[dst:dst+rowBytes], rgbaData[src:src+rowBytes])
			}
			uploadData = staging
			originX = uint32(x) //nolint:gosec
			originY = uint32(y) //nolint:gosec
			extentW = uint32(w) //nolint:gosec
			extentH = uint32(h) //nolint:gosec
			bytesPerRow = alignedBPR
			rowsPerImg = extentH
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
			return fmt.Errorf("upload glyph color atlas %d: %w", idx, err)
		}

		e.atlas.MarkClean(idx)
	}
	return nil
}

// PageTextureView returns the GPU texture view for the given color page.
// Returns nil if the page has not been uploaded.
func (e *ColorGlyphEngine) PageTextureView(index int) *webgpu.TextureView {
	e.mu.Lock()
	defer e.mu.Unlock()
	if index < 0 || index >= len(e.pageViews) {
		return nil
	}
	return e.pageViews[index]
}

// Destroy releases all GPU textures held by the engine.
func (e *ColorGlyphEngine) Destroy(device *webgpu.Device) {
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
}
