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
	"slices"

	"github.com/energye/gpui/engine/camera"
	"github.com/energye/gpui/engine/core"
)

// DeepItem is one depth-sortable sprite.
// Depth follows the R5/engine/camera convention: bigger = farther and drawn
// first (far-to-near painter); Name is only a debug key. Layer/FeetY keep
// the 2.3 rule and break ties inside the same depth — that tie rule is not
// copied here any more, it is camera's (P3.7 batch 2 step 2b).
type DeepItem struct {
	Name  string
	Depth float64
	Layer Layer
	FeetY float64
}

// NewDeepItem builds a DeepItem. Depth and FeetY must be finite; NaN/Inf
// returns a core InvalidArg error, never a guessed item.
func NewDeepItem(name string, depth float64, layer Layer, feetY float64) (DeepItem, error) {
	if !finite(depth) {
		return DeepItem{}, core.InvalidArg("sprite.NewDeepItem", "depth")
	}
	if !finite(feetY) {
		return DeepItem{}, core.InvalidArg("sprite.NewDeepItem", "feetY")
	}
	return DeepItem{Name: name, Depth: depth, Layer: layer, FeetY: feetY}, nil
}

// SetDepth changes the depth value. NaN/Inf returns InvalidArg and the
// original value is untouched.
func SetDepth(s *DeepItem, depth float64) error {
	if s == nil {
		return core.InvalidArg("sprite.SetDepth", "item")
	}
	if !finite(depth) {
		return core.InvalidArg("sprite.SetDepth", "depth")
	}
	s.Depth = depth
	return nil
}

// yKey builds camera's key pair for one deep item at input position order
// (band → z bucket, foot Y → in-bucket key, order → tie breaker). Built on the
// stack per comparison: these sorts run every frame, so no key table is
// allocated (same reasoning as Item.yKey).
func (d DeepItem) yKey(order int) camera.YSortItem {
	return camera.YSortItem{
		Pos:    core.V3(0, d.FeetY, 0),
		Order:  order,
		ZIndex: ZIndexForLayer(d.Layer),
	}
}

// compareDeep orders two input positions: depth descending first (this repo's
// extension axis), then camera's rule inside one depth (-1/0/+1).
func compareDeep(items []DeepItem, a, b int) int {
	if da, db := items[a].Depth, items[b].Depth; da != db {
		if da > db {
			return -1
		}
		return 1
	}
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

// DepthSort returns the draw order for deep items: depth descending (far first),
// then the camera rule inside one depth (band bucket, foot Y with camera's
// tolerance-based equality, input position as the tie breaker). Depth itself is
// this repo's extension axis — Godot has no 2D distance key — so it stays the
// outer level here and nothing else about the order is decided locally.
// The input slice is never mutated; the result is a fresh slice. Any non-finite
// depth/feet returns a core InvalidArg error with a nil result and leaves the
// input untouched.
func DepthSort(items []DeepItem) ([]DeepItem, error) {
	const op = "sprite.DepthSort"
	for i := range items {
		if !finite(items[i].Depth) {
			return nil, core.InvalidArg(op, "depth")
		}
		if !finite(items[i].FeetY) {
			return nil, core.InvalidArg(op, "feetY")
		}
	}
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	slices.SortFunc(idx, func(a, b int) int { return compareDeep(items, a, b) })
	out := make([]DeepItem, len(items))
	for rank, i := range idx {
		out[rank] = items[i]
	}
	return out, nil
}

// IsDepthSorted reports whether items already sit in draw order (first draws
// first). Empty and single-item slices are sorted. Any non-finite depth/feet
// reports false (fail closed), never true and never a panic.
// A nil slice has no order to verify and reports false.
func IsDepthSorted(items []DeepItem) bool {
	if items == nil {
		return false
	}
	for i := range items {
		if !finite(items[i].Depth) || !finite(items[i].FeetY) {
			return false
		}
	}
	for i := 1; i < len(items); i++ {
		if compareDeep(items, i, i-1) < 0 {
			return false
		}
	}
	return true
}
