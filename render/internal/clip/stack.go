//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package clip

import (
	"math"

	"github.com/energye/gpui/render/internal/image"
)

// RRectClip describes a rounded rectangle clip region.
// The Rect field holds the bounding rectangle in device coordinates,
// and Radius is the corner radius (uniform for all four corners).
type RRectClip struct {
	Rect   Rect
	Radius float64
}

// ClipStack manages hierarchical clip regions with push/pop operations.
// It maintains a stack of clip entries, where each entry can be either a
// rectangular clip, a rounded rectangle clip, or a path-based mask clip.
type ClipStack struct {
	entries []clipEntry
	bounds  Rect

	// maskCache reuses rasterized path masks across identical clips. A
	// recording can push the same viewport clip for every picture it
	// replays; rasterizing a full-canvas mask each time made R7's picture
	// cache recording CPU-bound. Keys include bounds/AA/fill rule, so
	// entries are self-validating when the canvas changes.
	maskCache      map[uint64]*MaskClipper
	maskCacheOrder []uint64
}

// clipEntry represents a single clip operation in the stack.
type clipEntry struct {
	prevBounds Rect
	mask       *MaskClipper
	maskKey    uint64
	rrect      *RRectClip // non-nil for rounded rectangle clips
	antiAlias  bool
}

// NewClipStack creates a new clip stack with the given bounds.
// The bounds represent the maximum clipping area (typically the canvas size).
func NewClipStack(bounds Rect) *ClipStack {
	return &ClipStack{
		entries: make([]clipEntry, 0, 8), // Pre-allocate for common case
		bounds:  bounds,
	}
}

// PushRect pushes a rectangular clip region onto the stack.
// The new clip bounds are the intersection of the current bounds and the given rectangle.
func (cs *ClipStack) PushRect(r Rect) {
	// Compute intersection with current bounds
	newBounds := cs.bounds.Intersect(r)

	// Push entry onto stack
	cs.entries = append(cs.entries, clipEntry{
		prevBounds: cs.bounds,
		mask:       nil, // No mask for rectangular clips
		antiAlias:  false,
	})

	// Update current bounds
	cs.bounds = newBounds
}

// PushRRect pushes a rounded rectangle clip region onto the stack.
// The new clip bounds are the intersection of the current bounds and the
// given rectangle. The radius is the corner radius (clamped to half the
// minimum dimension). If radius is zero, this is equivalent to PushRect.
func (cs *ClipStack) PushRRect(r Rect, radius float64) {
	// Clamp radius to half the smaller dimension.
	maxRadius := math.Min(r.W, r.H) / 2
	if radius > maxRadius {
		radius = maxRadius
	}
	if radius <= 0 {
		cs.PushRect(r)
		return
	}

	// Compute intersection with current bounds.
	newBounds := cs.bounds.Intersect(r)

	// Push entry with rrect data.
	cs.entries = append(cs.entries, clipEntry{
		prevBounds: cs.bounds,
		rrect:      &RRectClip{Rect: r, Radius: radius},
	})

	// Update current bounds.
	cs.bounds = newBounds
}

// PushPath pushes a path-based clip region onto the stack.
// The path (given as SOA verb+coords) is rasterized into a mask using the current bounds.
// If antiAlias is true, the mask will use anti-aliased rendering.
// fillRule selects how self-overlapping contours resolve (winding or even-odd).
func (cs *ClipStack) PushPath(verbs []PathVerb, coords []float64, antiAlias bool, fillRule FillRule) error {
	key := maskPathKey(verbs, coords, cs.bounds, antiAlias, fillRule, maskKindPath)
	mask := cs.lookupMask(key)
	if mask == nil {
		var err error
		mask, err = NewMaskClipper(verbs, coords, cs.bounds, antiAlias, fillRule)
		if err != nil {
			return err
		}
		cs.storeMask(key, mask)
	}

	// MaskClipper allocates a full-canvas coverage buffer, so mask.Bounds()
	// is the canvas, not the geometric path AABB. Track the path AABB here:
	// ClipBounds/getLocalClipBounds and the GPU scissor both need the real
	// conservative bound, and Coverage still uses the full-canvas mask.
	newBounds := cs.bounds.Intersect(pathCoordsBounds(coords))

	// Push entry onto stack
	cs.entries = append(cs.entries, clipEntry{
		prevBounds: cs.bounds,
		mask:       mask,
		maskKey:    key,
		antiAlias:  antiAlias,
	})

	// Update current bounds
	cs.bounds = newBounds

	return nil
}

// Pop removes the most recent clip region from the stack.
// If the stack is empty, this is a no-op.
func (cs *ClipStack) Pop() {
	if len(cs.entries) == 0 {
		return
	}

	// Get last entry
	lastIdx := len(cs.entries) - 1
	entry := cs.entries[lastIdx]

	// Restore previous bounds
	cs.bounds = entry.prevBounds

	// Remove entry from stack
	cs.entries = cs.entries[:lastIdx]
}

// Bounds returns the current effective clip bounds.
// This is the intersection of all pushed clip regions.
func (cs *ClipStack) Bounds() Rect {
	return cs.bounds
}

// IsVisible returns true if the point (x, y) is within the current clip region.
// For rectangular clips, this is a simple bounds check.
// For RRect clips, this checks the SDF distance.
// For mask clips, this checks if the coverage is non-zero.
func (cs *ClipStack) IsVisible(x, y float64) bool {
	// First check if point is within current bounds
	if !cs.bounds.Contains(Pt(x, y)) {
		return false
	}

	// Check all clip entries in the stack
	for i := range cs.entries {
		entry := &cs.entries[i]
		if entry.rrect != nil {
			if rrectSDF(x, y, entry.rrect) > 0 {
				return false
			}
		}
		if entry.mask != nil {
			coverage := entry.mask.Coverage(x, y)
			if coverage == 0 {
				return false
			}
		}
	}

	return true
}

// Coverage returns the combined coverage value (0-255) at the given point.
// This multiplies the coverage from all clip entries (RRect and mask) in the stack.
// Returns 0 if the point is outside the current bounds.
func (cs *ClipStack) Coverage(x, y float64) byte {
	// First check if point is within current bounds
	if !cs.bounds.Contains(Pt(x, y)) {
		return 0
	}

	// Start with full coverage
	coverage := uint16(255)

	// Multiply coverage from all clip entries
	for i := range cs.entries {
		entry := &cs.entries[i]

		// RRect SDF coverage
		if entry.rrect != nil {
			rrCov := rrectCoverage(x, y, entry.rrect)
			if rrCov == 0 {
				return 0
			}
			coverage = (coverage * uint16(rrCov)) / 255
			if coverage == 0 {
				return 0
			}
		}

		// Mask coverage
		if entry.mask != nil {
			maskCoverage := entry.mask.Coverage(x, y)
			if maskCoverage == 0 {
				return 0 // Early exit if any mask has zero coverage
			}

			// Multiply coverage: result = (coverage * maskCoverage) / 255
			coverage = (coverage * uint16(maskCoverage)) / 255

			if coverage == 0 {
				return 0 // Early exit if coverage becomes zero
			}
		}
	}

	return byte(coverage)
}

// rrectSDF computes the signed distance from point (px, py) to the rounded
// rectangle defined by rr. Negative values are inside, positive outside.
// Uses the same formula as render.sdfRRect (Inigo Quilez box SDF).
func rrectSDF(px, py float64, rr *RRectClip) float64 {
	cx := rr.Rect.X + rr.Rect.W/2
	cy := rr.Rect.Y + rr.Rect.H/2
	halfW := rr.Rect.W / 2
	halfH := rr.Rect.H / 2
	r := rr.Radius

	dx := math.Abs(px-cx) - halfW + r
	dy := math.Abs(py-cy) - halfH + r

	outside := math.Sqrt(math.Max(dx, 0)*math.Max(dx, 0) + math.Max(dy, 0)*math.Max(dy, 0))
	inside := math.Min(math.Max(dx, dy), 0)

	return outside + inside - r
}

// rrectCoverage computes anti-aliased coverage (0-255) for a point against
// a rounded rectangle clip. Uses a smoothstep transition over 0.75 pixels
// matching the SDF anti-alias width in sdf.go / sdf_render.wgsl.
func rrectCoverage(px, py float64, rr *RRectClip) byte {
	const aaWidth = 0.75
	d := rrectSDF(px, py, rr)
	if d >= aaWidth {
		return 0
	}
	if d <= -aaWidth {
		return 255
	}
	t := (d + aaWidth) / (2 * aaWidth)
	// Hermite smoothstep: 1 - (3t^2 - 2t^3)
	cov := 1 - (t * t * (3 - 2*t))
	return byte(cov * 255) //nolint:gosec // cov is in [0,1], result fits byte
}

// HasMaskClip reports whether the clip stack contains any mask-based clip
// entries (path/mask clips requiring per-pixel coverage).
func (cs *ClipStack) HasMaskClip() bool {
	for i := range cs.entries {
		if cs.entries[i].mask != nil {
			return true
		}
	}
	return false
}

// IsRectOnly reports whether the clip stack contains only rectangular clips
// (no path-based masks and no rounded rectangle clips). When true, clipping
// can be applied by restricting the destination bounds rather than per-pixel
// coverage, which preserves bitmap text quality and is significantly faster.
func (cs *ClipStack) IsRectOnly() bool {
	for i := range cs.entries {
		if cs.entries[i].mask != nil || cs.entries[i].rrect != nil {
			return false
		}
	}
	return true
}

// IsRRectOnly reports whether the clip stack contains only rectangular and/or
// rounded rectangle clips (no path-based masks). When true, clipping can be
// handled via hardware scissor rect (for the bounding box) plus analytic SDF
// in the fragment shader (for rounded corners).
func (cs *ClipStack) IsRRectOnly() bool {
	for i := range cs.entries {
		if cs.entries[i].mask != nil {
			return false
		}
	}
	return true
}

// RRectBounds returns the bounds and radius of the innermost (most recently
// pushed) rounded rectangle clip entry, or false if no RRect clip is active.
// When multiple RRect clips are nested, only the innermost is returned —
// the scissor rect already handles the outer bounds intersection.
func (cs *ClipStack) RRectBounds() (Rect, float64, bool) {
	// Scan from top of stack to find the most recent RRect.
	for i := len(cs.entries) - 1; i >= 0; i-- {
		if cs.entries[i].rrect != nil {
			rr := cs.entries[i].rrect
			return rr.Rect, rr.Radius, true
		}
	}
	return Rect{}, 0, false
}

// Depth returns the current depth of the clip stack.
// This is primarily useful for debugging and testing.
func (cs *ClipStack) Depth() int {
	return len(cs.entries)
}

// Reset clears all clip entries and restores the original bounds.
func (cs *ClipStack) Reset(bounds Rect) {
	cs.entries = cs.entries[:0]
	cs.bounds = bounds
}

// PushRectDifference subtracts rectangle r from the current clip.
// Coverage becomes zero inside r while remaining clip outside r is preserved.
func (cs *ClipStack) PushRectDifference(r Rect) error {
	key := maskRectDifferenceKey(cs.bounds, r)
	mask := cs.lookupMask(key)
	if mask == nil {
		var err error
		mask, err = NewRectDifferenceMask(cs.bounds, r)
		if err != nil {
			return err
		}
		cs.storeMask(key, mask)
	}
	// Bounds stay the same (difference does not shrink AABB in general).
	cs.entries = append(cs.entries, clipEntry{
		prevBounds: cs.bounds,
		mask:       mask,
		maskKey:    key,
		antiAlias:  false,
	})
	return nil
}

// ReplaceRect replaces the entire clip stack with a single rectangular clip
// relative to the original canvas bounds (bottom of stack / max area).
func (cs *ClipStack) ReplaceRect(canvas, r Rect) {
	cs.entries = cs.entries[:0]
	cs.bounds = canvas
	cs.PushRect(r)
}

// ReplacePath replaces the clip stack with a single path clip on the canvas.
func (cs *ClipStack) ReplacePath(canvas Rect, verbs []PathVerb, coords []float64, antiAlias bool, fillRule FillRule) error {
	cs.entries = cs.entries[:0]
	cs.bounds = canvas
	return cs.PushPath(verbs, coords, antiAlias, fillRule)
}

// pathCoordsBounds returns the conservative AABB of a path's coordinate
// points. Control points are included, so the box contains every curve. The
// clip mask itself remains full-canvas (MaskClipper indexes that buffer); this
// only narrows the stack's reported bounds.
func pathCoordsBounds(coords []float64) Rect {
	if len(coords) < 2 {
		return Rect{}
	}
	minX, minY := coords[0], coords[1]
	maxX, maxY := minX, minY
	for i := 2; i+1 < len(coords); i += 2 {
		x, y := coords[i], coords[i+1]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	return NewRect(minX, minY, maxX-minX, maxY-minY)
}

// PushPathDifference subtracts path coverage from the current clip.
// Implemented by rasterizing the path and inverting coverage inside current bounds.
func (cs *ClipStack) PushPathDifference(verbs []PathVerb, coords []float64, antiAlias bool, fillRule FillRule) error {
	key := maskPathKey(verbs, coords, cs.bounds, antiAlias, fillRule, maskKindPathDifference)
	mask := cs.lookupMask(key)
	if mask == nil {
		pos, err := NewMaskClipper(verbs, coords, cs.bounds, antiAlias, fillRule)
		if err != nil {
			return err
		}
		// Invert: coverage' = 255 - coverage for each pixel in pos mask,
		// then combine into a new mask over cs.bounds.
		w := int(cs.bounds.W + 0.5)
		h := int(cs.bounds.H + 0.5)
		if w <= 0 || h <= 0 {
			return image.ErrInvalidDimensions
		}
		inv, err := image.NewImageBuf(w, h, image.FormatGray8)
		if err != nil {
			return err
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				px := cs.bounds.X + float64(x) + 0.5
				py := cs.bounds.Y + float64(y) + 0.5
				cov := pos.Coverage(px, py)
				v := uint8(255 - int(cov))
				_ = inv.SetRGBA(x, y, v, v, v, 255)
			}
		}
		mask = &MaskClipper{mask: inv, bounds: cs.bounds}
		mask.cacheView()
		cs.storeMask(key, mask)
	}
	cs.entries = append(cs.entries, clipEntry{
		prevBounds: cs.bounds,
		mask:       mask,
		maskKey:    key,
		antiAlias:  antiAlias,
	})
	return nil
}
