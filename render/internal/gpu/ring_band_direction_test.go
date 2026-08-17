//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/stroke"
)

// TestTessellateAA_RingHoleBandDirection guards the clip-stroke inner-edge AA
// fix (render_clipping ex5): a stroked closed path expands to a ring (outer +
// inner contour); the inner contour's exterior band must face the hole
// (stencil==0 region) so the stroke's inner edge gets a partial-coverage
// gradient instead of a hard full-alpha step. Single contours must stay
// unchanged (their exterior band faces away from the fill).
func TestTessellateAA_RingHoleBandDirection(t *testing.T) {
	// Ring via stroke expansion of the ex5 eye outline (3px stroke).
	p := &render.Path{}
	p.MoveTo(600, 120)
	p.CubicTo(650, 50, 700, 50, 750, 120)
	p.CubicTo(700, 180, 650, 180, 600, 120)
	p.Close()
	verbs := convertPathVerbsToStroke(p.Verbs())
	style := stroke.Stroke{Width: 3, Cap: stroke.LineCapRound, Join: stroke.LineJoinRound, MiterLimit: 4}
	expander := stroke.NewStrokeExpander(style)
	outVerbs, outCoords := expander.Expand(verbs, p.Coords())
	fillPath := strokeResultToPath(outVerbs, outCoords)

	tess := NewFanTessellator()
	tess.TessellateAA(fillPath)

	if len(tess.contourAreas) != 2 {
		t.Fatalf("expanded ring should have 2 contours, got %d", len(tess.contourAreas))
	}
	// Both band meshes must be non-empty.
	if len(tess.bandVerts) == 0 || len(tess.innerBandVerts) == 0 {
		t.Fatal("expected exterior and interior band geometry")
	}

	// Top arc: the stroke inner (hole) boundary sits at y≈69. The hole
	// contour's exterior band must extend TOWARD the hole (y>69); its
	// interior band must stay on the stroke side (y<69).
	var extBelow, innerBelow int
	for i := 0; i+2 < len(tess.bandVerts); i += 3 {
		x, y, d := tess.bandVerts[i], tess.bandVerts[i+1], tess.bandVerts[i+2]
		if x > 670 && x < 680 && y > 68 && y < 70 && d < 0 {
			extBelow++
			if y <= 69 {
				t.Errorf("hole exterior band points INTO the stroke at (%.1f,%.1f) d=%.2f (want toward hole y>69)", x, y, d)
			}
		}
	}
	for i := 0; i+2 < len(tess.innerBandVerts); i += 3 {
		x, y, d := tess.innerBandVerts[i], tess.innerBandVerts[i+1], tess.innerBandVerts[i+2]
		if x > 670 && x < 680 && y > 68 && y < 70 && d > 0 {
			innerBelow++
			if y > 69 {
				t.Errorf("hole interior band points INTO the hole at (%.1f,%.1f) d=%.2f (want toward stroke y<69)", x, y, d)
			}
		}
	}
	if extBelow == 0 || innerBelow == 0 {
		t.Fatalf("expected band verts near x≈675 y≈69 top-arc inner edge (ext=%d inner=%d)", extBelow, innerBelow)
	}
}

// TestTessellateAA_ConvexBandUnaffected verifies a single contour (diamond)
// keeps its exterior band facing AWAY from the fill after the hole-orient
// fix: band d<0 verts above the top edge (y < boundary), inner band d>0
// below (y > boundary). The interior half is emitted for every non-corner
// segment — just-inside pixels get their partial coverage so straight edges
// fade smoothly like the convex renderer (ui_render_graphics/basic ③ lines
// vs ⑥ stroke: a binary interior makes stroked edges look stepped).
func TestTessellateAA_ConvexBandUnaffected(t *testing.T) {
	d := &render.Path{}
	d.MoveTo(50, 20) // top
	d.LineTo(80, 50) // right
	d.LineTo(50, 80) // bottom
	d.LineTo(20, 50) // left
	d.Close()

	tess := NewFanTessellator()
	tess.TessellateAA(d)
	if len(tess.contourAreas) != 1 {
		t.Fatalf("diamond should be a single contour, got %d", len(tess.contourAreas))
	}
	// Top edge's A-end corner (x≈50, y≈20): exterior d<0 verts sit above the
	// boundary (y < 20 — away from the fill); interior d>0 verts below (y > 20).
	// Band vertices live at the quad ends, so probe the corner region. The 90°
	// corner ends are eroded by the sharp-corner interior treatment, but the
	// interior half must still exist somewhere along the straight edges (the
	// smooth both-side fade the convex renderer also produces).
	var extAbove, innerNear int
	for i := 0; i+2 < len(tess.bandVerts); i += 3 {
		x, y, d := tess.bandVerts[i], tess.bandVerts[i+1], tess.bandVerts[i+2]
		if x > 48 && x < 56 && y > 17 && y < 23 && d < 0 {
			extAbove++
			if y >= 20 {
				t.Errorf("convex exterior band points INTO fill at (%.1f,%.1f)", x, y)
			}
		}
	}
	for i := 0; i+2 < len(tess.innerBandVerts); i += 3 {
		_, y, d := tess.innerBandVerts[i], tess.innerBandVerts[i+1], tess.innerBandVerts[i+2]
		if y > 20 && d > 0 {
			innerNear++
		}
	}
	if extAbove == 0 {
		t.Fatal("expected diamond top-edge exterior band verts")
	}
	if innerNear == 0 {
		t.Fatal("expected diamond interior band verts (smooth just-inside fade)")
	}
}
