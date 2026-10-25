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
	"errors"
	"image"
	"image/color"
	"sync"
)

// ErrColorAtlasFull is returned when no color atlas page can fit a glyph.
// Callers fall back to CPU rendering; the atlas stays valid.
var ErrColorAtlasFull = errors.New("gpu: color glyph atlas is full")

// ColorGlyphKey identifies one cached color glyph raster.
type ColorGlyphKey struct {
	FontID  uint64
	GlyphID uint16
	PPEM    uint16
	Palette int
	FG      color.RGBA
}

// ColorGlyphRegion is a packed RGBA block with page-local normalized UVs.
type ColorGlyphRegion struct {
	U0, V0, U1, V1 float32
	Page           int
	Width, Height  int
	BearingX       float32
	BearingY       float32
}

// ColorGlyphDirtyUpload describes one page region needing GPU upload.
type ColorGlyphDirtyUpload struct {
	Index      int
	X, Y, W, H int
	FullPage   bool
}

type colorGlyphPage struct {
	size   int
	pix    []byte
	alloc  *RectAllocator
	dirty  bool
	dirtyX int
	dirtyY int
	dirtyW int
	dirtyH int
}

// ColorGlyphAtlas packs premultiplied RGBA color glyphs into square pages.
// Shelf packing reuses RectAllocator; one dirty rect per page mirrors the
// mask atlas upload contract (partial vs full page decided by the uploader).
type ColorGlyphAtlas struct {
	mu       sync.Mutex
	pages    []*colorGlyphPage
	pageSize int
	maxPages int
	entries  map[ColorGlyphKey]ColorGlyphRegion
	// lastUse[key] is the put/get stamp; eviction picks the coldest page.
	lastUse map[ColorGlyphKey]uint64
	useStamp uint64
}

// NewColorGlyphAtlas creates an atlas with pageSize×pageSize RGBA pages.
func NewColorGlyphAtlas(pageSize, maxPages int) *ColorGlyphAtlas {
	if pageSize < MinAtlasSize {
		pageSize = MinAtlasSize
	}
	if maxPages < 1 {
		maxPages = 1
	}
	return &ColorGlyphAtlas{
		pageSize: pageSize,
		maxPages: maxPages,
		entries:  make(map[ColorGlyphKey]ColorGlyphRegion),
		lastUse:  make(map[ColorGlyphKey]uint64),
	}
}

// Get returns the packed region for a cached glyph and refreshes its LRU
// stamp.
func (a *ColorGlyphAtlas) Get(key ColorGlyphKey) (ColorGlyphRegion, bool) {
	a.mu.Lock()
	if _, ok := a.entries[key]; ok {
		a.useStamp++
		a.lastUse[key] = a.useStamp
	}
	defer a.mu.Unlock()
	region, ok := a.entries[key]
	return region, ok
}

// Put packs an RGBA glyph block, returning its region. Repeat Puts of the
// same key return the existing region without copying.
func (a *ColorGlyphAtlas) Put(key ColorGlyphKey, img *image.RGBA, bearingX, bearingY float32) (ColorGlyphRegion, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if region, ok := a.entries[key]; ok {
		a.useStamp++
		a.lastUse[key] = a.useStamp
		return region, nil
	}
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	if w <= 0 || h <= 0 || w > a.pageSize || h > a.pageSize {
		return ColorGlyphRegion{}, ErrColorAtlasFull
	}
	for i, page := range a.pages {
		if region, ok := a.putOnPage(i, page, key, img, w, h, bearingX, bearingY); ok {
			return region, nil
		}
	}
	if len(a.pages) >= a.maxPages {
		// C2: evict the coldest page and retry once (Skia GrStrikeCache
		// purges when full) instead of failing forever. The mask atlas's
		// evictTail does the same for alpha glyphs.
		if a.evictColdestPageLocked() {
			for i, page := range a.pages {
				if region, ok := a.putOnPage(i, page, key, img, w, h, bearingX, bearingY); ok {
					a.useStamp++
					a.lastUse[key] = a.useStamp
					return region, nil
				}
			}
		}
		return ColorGlyphRegion{}, ErrColorAtlasFull
	}
	page := &colorGlyphPage{
		size:  a.pageSize,
		pix:   make([]byte, a.pageSize*a.pageSize*4),
		alloc: NewRectAllocator(a.pageSize, a.pageSize, DefaultShelfPadding),
	}
	a.pages = append(a.pages, page)
	if region, ok := a.putOnPage(len(a.pages)-1, page, key, img, w, h, bearingX, bearingY); ok {
		a.useStamp++
		a.lastUse[key] = a.useStamp
		return region, nil
	}
	return ColorGlyphRegion{}, ErrColorAtlasFull
}

// evictColdestPageLocked drops the page whose entries have the coldest
// lastUse stamps and removes their keys. Reports whether a page was evicted.
// Caller must hold a.mu.
func (a *ColorGlyphAtlas) evictColdestPageLocked() bool {
	if len(a.pages) == 0 {
		return false
	}
	// Sum lastUse per page; coldest sum loses.
	pageStamp := make([]uint64, len(a.pages))
	for key, region := range a.entries {
		st := a.lastUse[key]
		if int(region.Page) < len(pageStamp) {
			pageStamp[region.Page] += st
		}
	}
	victim := 0
	for i := 1; i < len(pageStamp); i++ {
		if pageStamp[i] < pageStamp[victim] {
			victim = i
		}
	}
	for key, region := range a.entries {
		if int(region.Page) == victim {
			delete(a.entries, key)
			delete(a.lastUse, key)
		}
	}
	// Drop the page (pending dirty uploads for it become stale — the region
	// is gone, so the uploader must skip by page index).
	a.pages = append(a.pages[:victim], a.pages[victim+1:]...)
	// Reindex remaining regions/pages.
	for key, region := range a.entries {
		if int(region.Page) > victim {
			region.Page--
			a.entries[key] = region
		}
	}
	return true
}

func (a *ColorGlyphAtlas) putOnPage(index int, page *colorGlyphPage, key ColorGlyphKey, img *image.RGBA, w, h int, bearingX, bearingY float32) (ColorGlyphRegion, bool) {
	area := page.alloc.Allocate(w, h)
	if !area.IsValid() {
		return ColorGlyphRegion{}, false
	}
	src := img.Pix
	srcStride := img.Stride
	for row := 0; row < h; row++ {
		dst := ((area.Y+row)*page.size + area.X) * 4
		copy(page.pix[dst:dst+w*4], src[row*srcStride:row*srcStride+w*4])
	}
	a.expandDirty(page, area.X, area.Y, w, h)
	size := float32(page.size)
	region := ColorGlyphRegion{
		U0: float32(area.X) / size, V0: float32(area.Y) / size,
		U1: float32(area.X+w) / size, V1: float32(area.Y+h) / size,
		Page:     index,
		Width:    w,
		Height:   h,
		BearingX: bearingX,
		BearingY: bearingY,
	}
	a.entries[key] = region
	return region, true
}

func (a *ColorGlyphAtlas) expandDirty(page *colorGlyphPage, x, y, w, h int) {
	if !page.dirty {
		page.dirty = true
		page.dirtyX, page.dirtyY, page.dirtyW, page.dirtyH = x, y, w, h
		return
	}
	x0 := min(page.dirtyX, x)
	y0 := min(page.dirtyY, y)
	x1 := max(page.dirtyX+page.dirtyW, x+w)
	y1 := max(page.dirtyY+page.dirtyH, y+h)
	page.dirtyX, page.dirtyY, page.dirtyW, page.dirtyH = x0, y0, x1-x0, y1-y0
}

// DirtyUploads lists pages needing GPU upload.
func (a *ColorGlyphAtlas) DirtyUploads() []ColorGlyphDirtyUpload {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []ColorGlyphDirtyUpload
	for i, page := range a.pages {
		if !page.dirty {
			continue
		}
		out = append(out, ColorGlyphDirtyUpload{Index: i, X: page.dirtyX, Y: page.dirtyY, W: page.dirtyW, H: page.dirtyH})
	}
	return out
}

// MarkClean clears the dirty flag after a successful upload.
func (a *ColorGlyphAtlas) MarkClean(index int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if index >= 0 && index < len(a.pages) {
		a.pages[index].dirty = false
	}
}

// PageRGBAData returns the raw page bytes and page size for upload.
func (a *ColorGlyphAtlas) PageRGBAData(index int) (data []byte, size int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if index < 0 || index >= len(a.pages) {
		return nil, 0
	}
	return a.pages[index].pix, a.pages[index].size
}

// PageCount returns the number of allocated pages.
func (a *ColorGlyphAtlas) PageCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pages)
}
