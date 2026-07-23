package core

import "fmt"

// HitPaintIssue describes a layout node that violates hit == paint geometry.
type HitPaintIssue struct {
	Node    Node
	TypeID  string
	Message string
}

// AuditHitPaintContract walks the tree after Layout and reports nodes where
// child offsets place content outside the parent box, or Pressable/Decorated
// patterns that historically diverged hit vs paint.
//
// Contract (Flutter RO-like):
//   - Paint: childPC.Origin = parentOrigin + child.Offset()
//   - Hit:   local = parentPoint - child.Offset()
//   - AbsoluteOffset = sum of Offset along ancestor chain
//
// Therefore Offset is the single source of truth; this audit catches layout
// that wrote Offset inconsistently with Size (e.g. center-with-loose-max).
func AuditHitPaintContract(root Node) []HitPaintIssue {
	if root == nil {
		return nil
	}
	var out []HitPaintIssue
	var walk func(n Node)
	walk = func(n Node) {
		if n == nil {
			return
		}
		b := n.Base()
		sz := b.Size()
		for _, c := range b.children {
			cb := c.Base()
			off := cb.Offset()
			csz := cb.Size()
			// Scroll uses negative offsets intentionally; skip known scroll types.
			tid := c.TypeID()
			_ = tid
			// Child top-left outside parent local bounds (except intentional expand hit).
			if off.X < -0.5 || off.Y < -0.5 {
				// allow small float error; negative Y is wrong for non-scroll
				if n.TypeID() != "primitive.ScrollViewport" {
					out = append(out, HitPaintIssue{
						Node: c, TypeID: c.TypeID(),
						Message: fmt.Sprintf("child offset %v outside parent (negative)", off),
					})
				}
			}
			// Content drawn below parent bottom while parent claims small height:
			// classic Pressable loose-MaxHeight center bug.
			if csz.Height > 0 && sz.Height > 0 && off.Y+0.5 > sz.Height {
				out = append(out, HitPaintIssue{
					Node: c, TypeID: c.TypeID(),
					Message: fmt.Sprintf("child offset.Y=%.1f + size outside parent height=%.1f (paint below hit box)", off.Y, sz.Height),
				})
			}
			if csz.Width > 0 && sz.Width > 0 && off.X+0.5 > sz.Width {
				out = append(out, HitPaintIssue{
					Node: c, TypeID: c.TypeID(),
					Message: fmt.Sprintf("child offset.X=%.1f outside parent width=%.1f", off.X, sz.Width),
				})
			}
			walk(c)
		}
	}
	walk(root)
	return out
}

// PaintOriginFor returns the absolute paint origin of n (sum of offsets).
// Identical to AbsoluteOffset — exposed for tests/docs that speak "paint origin".
func PaintOriginFor(n Node) Point { return AbsoluteOffset(n) }
