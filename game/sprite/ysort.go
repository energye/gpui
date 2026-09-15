package sprite

import (
	"math"
	"sort"

	"github.com/energye/gpui/game/core"
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

// less reports whether a draws before b: smaller layer first, then
// smaller feet Y first. Ties return false both ways so the stable sort
// keeps the input order and equal heights never flicker.
func less(a, b Item) bool {
	if a.Layer != b.Layer {
		return a.Layer < b.Layer
	}
	return a.FeetY < b.FeetY
}

// Sort returns the draw order for items: layer ascending, then feet Y
// ascending, stable for ties. The input slice is never mutated; the result
// is a fresh slice the caller draws front to back in order. Any non-finite
// feet Y returns a core InvalidArg error with a nil result and leaves the
// input untouched.
func Sort(items []Item) ([]Item, error) {
	for i := range items {
		if !finite(items[i].FeetY) {
			return nil, core.InvalidArg("sprite.Sort", "feetY")
		}
	}
	out := make([]Item, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool { return less(out[i], out[j]) })
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
		if i > 0 && less(items[i], items[i-1]) {
			return false
		}
	}
	return true
}
