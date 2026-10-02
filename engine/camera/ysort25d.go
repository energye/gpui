//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// YSort25D orders true-2.5D objects back to front, the Godot misc/2.5d
// way (S79). Lower y draws first (behind); the demo's y_sort_slight_xz
// adds 0.001*(x+z) so equal-height objects keep a stable order instead
// of flickering. Sorted ranks map to z slots starting at -4000 stepping
// by 2, leaving the odd slots in between for shadows; over 4000 objects
// refuses to sort rather than overflowing the z range.
package camera

import (
	"sort"

	"github.com/energye/gpui/engine/core"
)

// MaxYSort25D caps one sort pass, copied from the demo's 4000-node guard.
const MaxYSort25D = 4000

// YSortZBase is the first z slot; YSortZStep leaves odd slots for shadows.
const (
	YSortZBase = -4000
	YSortZStep = 2
)

// YSortItem is one sortable object: its 3D position plus its input order.
// Order breaks full ties so the result is deterministic.
type YSortItem struct {
	Pos   core.Vec3
	Order int
}

// YSortKey ranks one position: y plus a whisper of x+z, exactly like the
// demo's y_sort_slight_xz.
func YSortKey(p core.Vec3) float64 {
	return p.Y + 0.001*(p.X+p.Z)
}

// YSortLess reports whether a draws behind b (lower key first;
// ties fall back to input order, never a coin flip).
func YSortLess(a, b YSortItem) bool {
	ka, kb := YSortKey(a.Pos), YSortKey(b.Pos)
	if ka != kb {
		return ka < kb
	}
	return a.Order < b.Order
}

// YSort orders items back to front without mutating the input.
// More than MaxYSort25D items returns a core OutOfMemory error and nil,
// mirroring the demo's refusal to sort past 4000 nodes.
func YSort(items []YSortItem) ([]YSortItem, error) {
	const op = "camera.YSort"
	if len(items) > MaxYSort25D {
		return nil, core.OutOfMemory(op, "items")
	}
	out := make([]YSortItem, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool { return YSortLess(out[i], out[j]) })
	return out, nil
}

// YSortZ maps a sorted rank to its z slot: base plus rank times step.
func YSortZ(rank int) int { return YSortZBase + rank*YSortZStep }
