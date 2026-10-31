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

import "sync"

// Default atlas settings.
const (
	// MinAtlasSize is the minimum atlas dimension (256x256).
	MinAtlasSize = 256

	// DefaultShelfPadding is the padding between shelves.
	DefaultShelfPadding = 1
)

// AtlasRegion represents a rectangular region in a texture atlas.
type AtlasRegion struct {
	// X is the left edge of the region.
	X int
	// Y is the top edge of the region.
	Y int
	// Width is the region width.
	Width int
	// Height is the region height.
	Height int
}

// IsValid returns true if the region has valid dimensions.
func (r AtlasRegion) IsValid() bool {
	return r.Width > 0 && r.Height > 0
}

// Contains returns true if the point (x, y) is inside the region.
func (r AtlasRegion) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// shelf represents a horizontal shelf in the shelf-packing algorithm.
type shelf struct {
	y       int // Top Y coordinate of this shelf
	height  int // Height of this shelf (tallest item so far)
	nextX   int // Next available X position on this shelf
	padding int // Padding between items
}

// RectAllocator implements a simple shelf-packing algorithm for
// allocating rectangular regions within a fixed-size area.
//
// The shelf-packing algorithm works by dividing the atlas into
// horizontal "shelves". Each new rectangle is placed on the current
// shelf if it fits, or a new shelf is created below.
type RectAllocator struct {
	mu sync.Mutex

	// Atlas dimensions
	width  int
	height int

	// Shelves (sorted by Y position)
	shelves []*shelf

	// Current shelf for allocation
	currentShelf int

	// Padding between items and shelves
	padding int

	// Statistics
	allocCount int
	usedArea   int
}

// NewRectAllocator creates a new rectangular region allocator.
func NewRectAllocator(width, height, padding int) *RectAllocator {
	if width < MinAtlasSize {
		width = MinAtlasSize
	}
	if height < MinAtlasSize {
		height = MinAtlasSize
	}
	if padding < 0 {
		padding = 0
	}

	return &RectAllocator{
		width:   width,
		height:  height,
		shelves: make([]*shelf, 0, 16),
		padding: padding,
	}
}

// Allocate finds space for a rectangle of the given size.
// Returns an invalid region if the rectangle cannot be allocated.
func (a *RectAllocator) Allocate(width, height int) AtlasRegion {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Validate size
	if width <= 0 || height <= 0 {
		return AtlasRegion{}
	}

	// Add padding to requested size
	paddedWidth := width + a.padding
	paddedHeight := height + a.padding

	// Check if it fits in the atlas at all
	if paddedWidth > a.width || paddedHeight > a.height {
		return AtlasRegion{}
	}

	// Try to fit on existing shelves
	for i, s := range a.shelves {
		if a.fitsOnShelf(s, paddedWidth, paddedHeight) {
			return a.allocateOnShelf(i, width, height, paddedWidth)
		}
	}

	// Create a new shelf
	return a.allocateNewShelf(width, height, paddedWidth, paddedHeight)
}

// fitsOnShelf checks if a rectangle fits on the given shelf.
func (a *RectAllocator) fitsOnShelf(s *shelf, paddedWidth, paddedHeight int) bool {
	// Check horizontal space
	if s.nextX+paddedWidth > a.width {
		return false
	}
	// Check if height is compatible (can be taller if shelf is tall enough,
	// but we can't make the shelf taller if items are already on it)
	if paddedHeight > s.height && s.nextX > 0 {
		return false
	}
	return true
}

// allocateOnShelf allocates space on an existing shelf.
func (a *RectAllocator) allocateOnShelf(shelfIndex, width, height, paddedWidth int) AtlasRegion {
	s := a.shelves[shelfIndex]

	region := AtlasRegion{
		X:      s.nextX,
		Y:      s.y,
		Width:  width,
		Height: height,
	}

	// Update shelf state
	s.nextX += paddedWidth
	if height+a.padding > s.height {
		s.height = height + a.padding
	}

	a.allocCount++
	a.usedArea += width * height

	return region
}

// allocateNewShelf creates a new shelf and allocates the rectangle on it.
func (a *RectAllocator) allocateNewShelf(width, height, paddedWidth, paddedHeight int) AtlasRegion {
	// Calculate Y position for new shelf
	newY := 0
	if len(a.shelves) > 0 {
		lastShelf := a.shelves[len(a.shelves)-1]
		newY = lastShelf.y + lastShelf.height
	}

	// Check if there's vertical space
	if newY+paddedHeight > a.height {
		return AtlasRegion{}
	}

	// Create new shelf
	newShelf := &shelf{
		y:       newY,
		height:  paddedHeight,
		nextX:   paddedWidth,
		padding: a.padding,
	}
	a.shelves = append(a.shelves, newShelf)

	region := AtlasRegion{
		X:      0,
		Y:      newY,
		Width:  width,
		Height: height,
	}

	a.allocCount++
	a.usedArea += width * height

	return region
}

// Reset clears all allocations, making the entire area available again.
func (a *RectAllocator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.shelves = a.shelves[:0]
	a.currentShelf = 0
	a.allocCount = 0
	a.usedArea = 0
}

// UsedArea returns the total area of allocated rectangles.
func (a *RectAllocator) UsedArea() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.usedArea
}

// Utilization returns the fraction of area used (0.0 to 1.0).
func (a *RectAllocator) Utilization() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()

	totalArea := a.width * a.height
	if totalArea == 0 {
		return 0
	}
	return float64(a.usedArea) / float64(totalArea)
}

// AllocCount returns the number of successful allocations.
func (a *RectAllocator) AllocCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.allocCount
}
