//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// YSort25D orders true-2.5D objects the way Godot's canvas y-sort does it. The
// outer key is the node's effective z bucket: relative z accumulates along the
// parent chain and clamps to ±YSortZRange at every level, absolute z ignores the
// chain (Godot renderer_canvas_cull.cpp:145-150 and :431-436, range from
// rendering_server_enums.h:40-41). The inner key is the origin Y compared with
// tolerance-based equality, and a tie breaks on the container's DFS collection
// index — the two branches of Godot's ItemYSort functor verbatim
// (renderer_canvas_cull.h:117-127: is_equal_approx on ysort_xform.origin.y, else
// ysort_index). Sorting is scoped to one container: nodes of different
// containers are never ordered against each other (Godot re-sorts only inside a
// y-sort owner and never across a CanvasLayer).
//
// Unfrozen 2026-11-04 (P3.7 batch 2, user-approved): the symbols below were
// frozen additive-only since 2026-10-04. A 2×3 key compared with exact != could
// not carry z priority and could flicker on sub-pixel feet, so the key became
// two-level and the default key dropped the x+z whisper (that variant survives,
// named, and is now selectable per sort run). Switched in the same commit: the
// camera case table and examples/engine/25d_basis (measured: their draw order is
// unchanged). The sprite/render copies still carry their own comparators and are
// scheduled to delegate here in batch 2 step 2b.
package camera

import (
	"math"
	"sort"

	"github.com/energye/gpui/engine/core"
)

// MaxYSort25D caps one sort pass, copied from the demo's 4000-node guard
// (misc/2.5d/addons/node25d/y_sort_25d.gd:30-33: the z range only holds 4000).
const MaxYSort25D = 4000

// YSortZBase is the first z slot; YSortZStep leaves odd slots for shadows.
const (
	YSortZBase = -4000
	YSortZStep = 2
)

// YSortZRange is the clamp half-width of one effective z index: Godot keeps a
// canvas item z between -4096 and 4096 and clamps at every level of the parent
// chain, so a deep chain can never run away past the range.
const YSortZRange = 4096

// ySortCmpEpsilon is Godot's CMP_EPSILON (core/math/math_defs.h:50), which the
// y-sort comparator uses through Math::is_equal_approx.
const ySortCmpEpsilon = 1e-5

// YSortKeyFn turns one position into the in-bucket key. nil means YSortKeyY.
type YSortKeyFn func(core.Vec3) float64

// YSortItem is one sortable object inside one container: its position in the
// container frame, the container order that breaks ties, and its z bucket.
// ZAsRelative only carries meaning for tree input, where YSortTree resolves the
// parent chain; a flat item is already resolved.
type YSortItem struct {
	Pos         core.Vec3
	Order       int
	ZIndex      int
	ZAsRelative bool
}

// YSortKeyY is the standard in-bucket key: the node's origin Y, the demo's own
// y_sort (node_25d.gd:134-135) and what Godot's ItemYSort reads out of the
// accumulated transform.
func YSortKeyY(p core.Vec3) float64 { return p.Y }

// YSortKeySlightXZ is the demo's y_sort_slight_xz variant (node_25d.gd:137-140):
// Y plus a whisper of X+Z, which orders same-height objects geometrically instead
// of falling back to the container index. Opt-in through YSortWithKey /
// YSortTreeWithKey.
func YSortKeySlightXZ(p core.Vec3) float64 { return p.Y + 0.001*(p.X+p.Z) }

// ySortEqualApprox is Godot's Math::is_equal_approx for two keys
// (core/math/math_funcs.h:552-562): exact equality first (the ±Inf branch), then
// |b-a| < max(CMP_EPSILON, CMP_EPSILON*|a|). Two feet a sub-pixel apart must not
// flip order from frame to frame, yet the tolerance stays far below any real
// position difference.
func ySortEqualApprox(a, b float64) bool {
	if a == b {
		return true
	}
	tol := ySortCmpEpsilon * math.Abs(a)
	if tol < ySortCmpEpsilon {
		tol = ySortCmpEpsilon
	}
	return math.Abs(b-a) < tol
}

// clampYSortZ saturates one z value into [-YSortZRange, YSortZRange].
func clampYSortZ(z int) int {
	if z > YSortZRange {
		return YSortZRange
	}
	if z < -YSortZRange {
		return -YSortZRange
	}
	return z
}

// ClampZIndex saturates one z index into [-YSortZRange, YSortZRange] — the same
// CLAMP Godot applies at every level of a z chain
// (renderer_canvas_cull.cpp:431-436). Module libraries that map their own
// priority axis onto this z axis (sprite's draw bands, for instance) go through
// here so one saturation rule lives in one place.
func ClampZIndex(z int) int { return clampYSortZ(z) }

// YSortLessWithKey reports whether a draws behind b: z bucket first (Godot puts
// items of one z_index into one draw bucket and sorts by Y only inside it), then
// the key with tolerance-based equality, then the container order. Two different
// Order values always decide the winner, so the outcome never depends on the
// sorting algorithm being stable. The equality branch is not transitive (a chain
// of sub-epsilon steps can outrun the tolerance) — Godot's comparator has the
// same property, and what it can straddle stays far below one pixel.
func YSortLessWithKey(a, b YSortItem, key YSortKeyFn) bool {
	if a.ZIndex != b.ZIndex {
		return a.ZIndex < b.ZIndex
	}
	if key == nil {
		key = YSortKeyY
	}
	ka, kb := key(a.Pos), key(b.Pos)
	if !ySortEqualApprox(ka, kb) {
		return ka < kb
	}
	return a.Order < b.Order
}

// YSortLess is the standard comparator: z bucket, then pure Y, then order.
func YSortLess(a, b YSortItem) bool { return YSortLessWithKey(a, b, YSortKeyY) }

// YSortNode is one node of one y-sort container. Parent is an index into the
// same slice, -1 for a root. Pos is expressed in the container frame — Godot
// sorts on the accumulated transform origin, so a caller holding parent-relative
// transforms resolves them first and passes that accumulated value in here.
// Z accumulates along the parent chain when ZAsRelative, clamping at every
// level; the DFS visit order is what breaks key ties.
type YSortNode struct {
	Parent      int
	Pos         core.Vec3
	ZIndex      int
	ZAsRelative bool
}

// YSortEffectiveZ resolves one node's z bucket from its parent chain
// (relative z accumulates and clamps per level; absolute z ignores the chain).
// A bad index, a non-finite position or a cycle returns a core error and 0.
func YSortEffectiveZ(nodes []YSortNode, i int) (int, error) {
	const op = "camera.YSortEffectiveZ"
	if i < 0 || i >= len(nodes) {
		return 0, core.InvalidArg(op, "index")
	}
	if err := ySortValidateNode(nodes[i], op); err != nil {
		return 0, err
	}
	// Walk up with a step budget: more steps than nodes means a cycle.
	chain := make([]int, 0, 8)
	cur := i
	for guard := 0; cur >= 0; guard++ {
		if guard > len(nodes) {
			return 0, core.InvalidArg(op, "cycle")
		}
		chain = append(chain, cur)
		p := nodes[cur].Parent
		if p == cur {
			return 0, core.InvalidArg(op, "cycle")
		}
		cur = p
	}
	z := 0
	for k := len(chain) - 1; k >= 0; k-- {
		n := nodes[chain[k]]
		if n.ZAsRelative {
			z = clampYSortZ(z + n.ZIndex)
		} else {
			z = clampYSortZ(n.ZIndex)
		}
	}
	return z, nil
}

// ySortValidateNode rejects a non-finite position (fail closed, never a guess).
func ySortValidateNode(n YSortNode, op string) error {
	if !finite(n.Pos.X) || !finite(n.Pos.Y) || !finite(n.Pos.Z) {
		return core.InvalidArg(op, "pos")
	}
	return nil
}

// YSortTreeWithKey flattens one container depth-first (roots in slice order,
// children in slice order; pass the host itself as a root to give it the Godot
// slot 0) and returns the draw order as indices into nodes: far first, near
// last. Flattening is DFS, but the compare does not cluster subtrees: the key is
// each node's own position in the container frame, exactly like Godot's
// ysort_xform.origin.y, so a descendant with a far Y sorts late and interleaves
// with other branches — the DFS index only breaks ties. More than MaxYSort25D
// nodes, a bad parent index, a cycle or a non-finite position returns a core
// error and a nil slice.
func YSortTreeWithKey(nodes []YSortNode, key YSortKeyFn) ([]int, error) {
	return ySortTree("camera.YSortTreeWithKey", nodes, key)
}

// YSortTree is YSortTreeWithKey with the standard pure-Y key.
func YSortTree(nodes []YSortNode) ([]int, error) {
	return ySortTree("camera.YSortTree", nodes, YSortKeyY)
}

func ySortTree(op string, nodes []YSortNode, key YSortKeyFn) ([]int, error) {
	if len(nodes) > MaxYSort25D {
		return nil, core.OutOfMemory(op, "nodes")
	}
	children := make([][]int, len(nodes))
	var roots []int
	for i, n := range nodes {
		if err := ySortValidateNode(n, op); err != nil {
			return nil, err
		}
		switch {
		case n.Parent == -1:
			roots = append(roots, i)
		case n.Parent < 0 || n.Parent >= len(nodes):
			return nil, core.InvalidArg(op, "parent")
		default:
			children[n.Parent] = append(children[n.Parent], i)
		}
	}
	items := make([]YSortItem, 0, len(nodes))
	nodeOf := make([]int, 0, len(nodes)) // items[k] came from nodes[nodeOf[k]]
	seen := make([]bool, len(nodes))
	var walk func(idx, parentZ int)
	walk = func(idx, parentZ int) {
		if seen[idx] {
			return // a second visit means a cycle; reported by the count check below
		}
		seen[idx] = true
		z := clampYSortZ(parentZ + nodes[idx].ZIndex)
		if !nodes[idx].ZAsRelative {
			z = clampYSortZ(nodes[idx].ZIndex)
		}
		items = append(items, YSortItem{
			Pos:         nodes[idx].Pos,
			Order:       len(items), // DFS position = the tie breaker
			ZIndex:      z,
			ZAsRelative: nodes[idx].ZAsRelative,
		})
		nodeOf = append(nodeOf, idx)
		for _, c := range children[idx] {
			walk(c, z)
		}
	}
	for _, r := range roots {
		walk(r, 0)
	}
	if len(items) != len(nodes) {
		// A short visit set means a cycle, or a chain that never reaches a root.
		return nil, core.InvalidArg(op, "cycle")
	}
	order := ySortOrder(len(items), func(i, j int) bool { return YSortLessWithKey(items[i], items[j], key) })
	out := make([]int, len(items))
	for pos, itemIdx := range order {
		out[pos] = nodeOf[itemIdx] // draw order, expressed as node indices
	}
	return out, nil
}

// YSortWithKey orders flat items back to front without mutating the input. The
// items are one sibling set, so ZIndex is used as given (no parent chain to
// accumulate). Over MaxYSort25D items returns a core OutOfMemory error and nil,
// mirroring the demo's refusal to sort past 4000 nodes.
func YSortWithKey(items []YSortItem, key YSortKeyFn) ([]YSortItem, error) {
	return ySortFlat("camera.YSortWithKey", items, key)
}

// YSort is YSortWithKey with the standard pure-Y key.
func YSort(items []YSortItem) ([]YSortItem, error) {
	return ySortFlat("camera.YSort", items, YSortKeyY)
}

func ySortFlat(op string, items []YSortItem, key YSortKeyFn) ([]YSortItem, error) {
	if len(items) > MaxYSort25D {
		return nil, core.OutOfMemory(op, "items")
	}
	out := make([]YSortItem, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool { return YSortLessWithKey(out[i], out[j], key) })
	return out, nil
}

// ySortOrder returns the 0..n-1 permutation sorted far-first by less.
func ySortOrder(n int, less func(i, j int) bool) []int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return less(order[i], order[j]) })
	return order
}

// YSortZ maps a sorted rank to its z slot: base plus rank times step. The odd
// slots in between are reserved for shadows by convention — no engine code
// consumes them yet (see shadow25d: the 25d_basis window still parks shadows on
// a fixed ground line, not on the owner's slot). This slotting is the demo's
// z_index *assignment* trick (y_sort_25d.gd:43-48 rewrites every node's z so the
// engine's own z bucketing reproduces the y order), not the comparator above.
func YSortZ(rank int) int { return YSortZBase + rank*YSortZStep }
