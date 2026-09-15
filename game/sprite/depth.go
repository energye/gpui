package sprite

import (
	"sort"

	"github.com/energye/gpui/game/core"
)

// DeepItem is one depth-sortable sprite (1.2 body, S41/W5).
// Depth follows the R5/game/camera convention: bigger = farther and drawn
// first (far-to-near painter); Name is only a debug key. Layer/FeetY keep
// the 2.3 rule and break ties inside the same depth.
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

// deepLess reports whether a draws before b: bigger depth first (farther
// behind), then smaller layer first, then smaller feet Y first. Ties return
// false both ways so the stable sort keeps the input order.
func deepLess(a, b DeepItem) bool {
	if a.Depth != b.Depth {
		return a.Depth > b.Depth
	}
	if a.Layer != b.Layer {
		return a.Layer < b.Layer
	}
	return a.FeetY < b.FeetY
}

// DepthSort returns the draw order for deep items: depth descending
// (far first), then layer ascending, then feet Y ascending, stable for
// ties. The input slice is never mutated; the result is a fresh slice.
// Any non-finite depth/feet returns a core InvalidArg error with a nil
// result and leaves the input untouched.
func DepthSort(items []DeepItem) ([]DeepItem, error) {
	for i := range items {
		if !finite(items[i].Depth) {
			return nil, core.InvalidArg("sprite.DepthSort", "depth")
		}
		if !finite(items[i].FeetY) {
			return nil, core.InvalidArg("sprite.DepthSort", "feetY")
		}
	}
	out := make([]DeepItem, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool { return deepLess(out[i], out[j]) })
	return out, nil
}

// IsDepthSorted reports whether items already sit in draw order (first
// draws first). Empty and single-item slices are sorted. Any non-finite
// depth/feet reports false (fail closed), never true and never a panic.
// A nil slice has no order to verify and reports false.
func IsDepthSorted(items []DeepItem) bool {
	if items == nil {
		return false
	}
	for i := range items {
		if !finite(items[i].Depth) || !finite(items[i].FeetY) {
			return false
		}
		if i > 0 && deepLess(items[i], items[i-1]) {
			return false
		}
	}
	return true
}
