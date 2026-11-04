//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package sprite

import (
	"math"
	"slices"

	"github.com/energye/gpui/engine/camera"
	"github.com/energye/gpui/engine/core"
)

// Layer is the draw band. Bigger covers smaller: world < fx < ui.
// Custom bands are plain integers; the ordering rule never changes.
type Layer int

const (
	// LayerWorld holds people, trees, ground: drawn first (behind).
	LayerWorld Layer = 0
	// LayerFX holds fire, smoke, trails: drawn over the world.
	LayerFX Layer = 1
	// LayerUI holds buttons, tips: drawn last (on top, never covered).
	LayerUI Layer = 2
)

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// ZIndexForLayer maps one draw band onto the camera z bucket axis (Godot's
// outer key): the band takes the same slot camera.YSortZ hands out, so bands sit
// on the even slots and the odd ones in between stay free for the band's
// shadows. The map is monotone, so "bigger band covers smaller" never changes;
// a band past the ±camera.YSortZRange end saturates into the last bucket,
// exactly like Godot clamping an out-of-range z_index.
func ZIndexForLayer(l Layer) int {
	return camera.ClampZIndex(camera.YSortZ(int(l)))
}

// Item is one sortable sprite: which band plus the foot bottom in world
// units. Name is only a debug key for tests and logs, never an asset id.
type Item struct {
	Name  string
	Layer Layer
	FeetY float64
}

// NewItem builds an Item. Feet Y must be finite; NaN/Inf returns a core
// InvalidArg error, never a guessed item. Layer accepts any integer so
// callers can add custom bands; Name may be empty.
func NewItem(name string, layer Layer, feetY float64) (Item, error) {
	if !finite(feetY) {
		return Item{}, core.InvalidArg("sprite.NewItem", "feetY")
	}
	return Item{Name: name, Layer: layer, FeetY: feetY}, nil
}

// yKey builds camera's key pair for one item at input position order: the band
// becomes the outer z bucket, the foot Y the in-bucket key, and order is the
// container index camera uses to break ties. It is built on the stack per
// comparison instead of as a key table, because these sorts run every frame and
// a 1000-item key slice cost 2x the sort time (P3.7 batch 2 A/B).
func (it Item) yKey(order int) camera.YSortItem {
	return camera.YSortItem{
		Pos:    core.V3(0, it.FeetY, 0),
		Order:  order,
		ZIndex: ZIndexForLayer(it.Layer),
	}
}

// compareItems orders two input positions by camera's rule (-1/0/+1).
func compareItems(items []Item, a, b int) int {
	ka, kb := items[a].yKey(a), items[b].yKey(b)
	switch {
	case camera.YSortLess(ka, kb):
		return -1
	case camera.YSortLess(kb, ka):
		return 1
	default:
		return 0
	}
}

// Sort returns the draw order for items: band ascending, then foot Y ascending
// (with camera's tolerance-based equality), ties settled by the input position.
// The band+foot formula lives in one place — camera.YSortLess — and this is a
// delegation, not a copy (P3.7 batch 2 step 2b). The input slice is never
// mutated; the result is a fresh slice the caller draws front to back in order.
// Any non-finite feet Y returns a core InvalidArg error with a nil result and
// leaves the input untouched.
func Sort(items []Item) ([]Item, error) {
	const op = "sprite.Sort"
	for i := range items {
		if !finite(items[i].FeetY) {
			return nil, core.InvalidArg(op, "feetY")
		}
	}
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	slices.SortFunc(idx, func(a, b int) int { return compareItems(items, a, b) })
	out := make([]Item, len(items))
	for rank, i := range idx {
		out[rank] = items[i]
	}
	return out, nil
}

// IsSorted reports whether items already sit in draw order (first draws
// first). Empty and single-item slices are sorted. Any non-finite feet Y
// reports false (fail closed), never true and never a panic.
func IsSorted(items []Item) bool {
	for i := range items {
		if !finite(items[i].FeetY) {
			return false
		}
	}
	for i := 1; i < len(items); i++ {
		if compareItems(items, i, i-1) < 0 {
			return false
		}
	}
	return true
}
