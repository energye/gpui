package rendering_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// layoutTransform builds a laid-out RenderTransform of fixed size with one color child.
func layoutTransform(t *testing.T, w, h, rot, sx, sy float64) *rendering.RenderTransform {
	t.Helper()
	child := rendering.NewRenderColorBox(w, h, 1, 0, 0, 1)
	tr := rendering.NewRenderTransform(child)
	tr.FixedWidth, tr.FixedHeight = w, h
	tr.SetRotation(rot)
	tr.SetScale(sx, sy)
	root := rendering.NewAbsoluteBox(w*3, h*3)
	root.Place(tr, 0, 0)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: w * 3, Height: h * 3}, true)
	if tr.Size().Width != w || tr.Size().Height != h {
		t.Fatalf("layout size=%v want %vx%v", tr.Size(), w, h)
	}
	return tr
}

// TestRenderTransform_HitTest_Identity keeps AABB behavior when rot=0 scale=1.
func TestRenderTransform_HitTest_Identity(t *testing.T) {
	tr := layoutTransform(t, 40, 40, 0, 1, 1)

	// Interior → child color box.
	if hit := tr.HitTest(rendering.Point{X: 20, Y: 20}); hit == nil {
		t.Fatal("identity center should hit")
	}
	// Outside box → miss.
	if hit := tr.HitTest(rendering.Point{X: -1, Y: 10}); hit != nil {
		t.Fatal("identity outside should miss")
	}
	if hit := tr.HitTest(rendering.Point{X: 50, Y: 10}); hit != nil {
		t.Fatal("identity past right edge should miss")
	}
	// Corner interior still hits.
	if hit := tr.HitTest(rendering.Point{X: 1, Y: 1}); hit == nil {
		t.Fatal("identity near corner should hit")
	}
}

// TestRenderTransform_HitTest_Rotation drives shipped HitTest under 45° rotation:
// parent-local AABB corner is a miss (maps outside local box); a point on the
// painted rotated geometry (mapped from local center / near edge mid) hits.
func TestRenderTransform_HitTest_Rotation(t *testing.T) {
	const w, h = 100.0, 100.0
	tr := layoutTransform(t, w, h, math.Pi/4, 1, 1)

	// Untransformed AABB corner (0,0): under 45° about center, inverse maps
	// outside the local content box → must MISS (AABB-only would HIT).
	if hit := tr.HitTest(rendering.Point{X: 0, Y: 0}); hit != nil {
		t.Fatalf("AABB corner (0,0) after 45° should miss (got %T); inverse-CTM required", hit)
	}
	if hit := tr.HitTest(rendering.Point{X: 99, Y: 99}); hit != nil {
		t.Fatalf("AABB far corner after 45° should miss (got %T)", hit)
	}

	// Local center paints at parent center — always a hit.
	if hit := tr.HitTest(rendering.Point{X: 50, Y: 50}); hit == nil {
		t.Fatal("center should hit under rotation")
	}

	// Interior local point near the top-left corner paints outside the untransformed
	// AABB under 45° (forward: local(5,5) → parent ≈ (50, -13.64)). Must HIT —
	// pure AABB would miss because Y < 0.
	// Use a slightly inset parent Y so FP does not land exactly on the local corner.
	outsideAABB := rendering.Point{X: 50, Y: -10}
	if outsideAABB.Y >= 0 && outsideAABB.Y < h {
		t.Fatal("test fixture must be outside untransformed AABB")
	}
	if hit := tr.HitTest(outsideAABB); hit == nil {
		t.Fatalf("painted geometry at parent (%.2f,%.2f) outside AABB should hit", outsideAABB.X, outsideAABB.Y)
	}

	// Beyond the rotated square's painted extent (past the corner tip at y≈-20.7).
	far := rendering.Point{X: 50, Y: 50 - 50*math.Sqrt2 - 20}
	if hit := tr.HitTest(far); hit != nil {
		t.Fatalf("beyond painted extent (%.2f,%.2f) should miss", far.X, far.Y)
	}
}

// TestRenderTransform_HitTest_Scale: uniform scale>1 expands painted hit region
// beyond the layout AABB; scale<1 shrinks it so AABB edge interior can miss.
func TestRenderTransform_HitTest_Scale(t *testing.T) {
	const w, h = 40.0, 40.0

	// Scale 2 about center: local (0,0) → parent (-20,-20) relative to layout box origin.
	// Parent (-10,-10) is outside [0,40] AABB but maps into local → HIT.
	trBig := layoutTransform(t, w, h, 0, 2, 2)
	if hit := trBig.HitTest(rendering.Point{X: -10, Y: -10}); hit == nil {
		t.Fatal("scale=2: point outside AABB but on scaled geometry should hit")
	}
	if hit := trBig.HitTest(rendering.Point{X: 20, Y: 20}); hit == nil {
		t.Fatal("scale=2: center should hit")
	}
	// Far outside scaled extent.
	if hit := trBig.HitTest(rendering.Point{X: -30, Y: -30}); hit != nil {
		t.Fatal("scale=2: beyond scaled extent should miss")
	}

	// Scale 0.5 about center: layout AABB corner (0,0) maps to local outside half-box → MISS.
	// (AABB-only would still hit.)
	trSmall := layoutTransform(t, w, h, 0, 0.5, 0.5)
	if hit := trSmall.HitTest(rendering.Point{X: 0, Y: 0}); hit != nil {
		t.Fatalf("scale=0.5: AABB corner should miss (got %T); pure AABB would hit", hit)
	}
	if hit := trSmall.HitTest(rendering.Point{X: 20, Y: 20}); hit == nil {
		t.Fatal("scale=0.5: center should still hit")
	}
	// Near center but still inside scaled half (local extent maps to parent [10,30]).
	if hit := trSmall.HitTest(rendering.Point{X: 15, Y: 15}); hit == nil {
		t.Fatal("scale=0.5: point inside scaled box should hit")
	}
}

// TestRenderTransform_HitTest_RotationAndScale combines both (still via shipped HitTest).
func TestRenderTransform_HitTest_RotationAndScale(t *testing.T) {
	tr := layoutTransform(t, 80, 80, math.Pi/6, 1.5, 1.5)
	// Center always maps to itself under center-based transform.
	if hit := tr.HitTest(rendering.Point{X: 40, Y: 40}); hit == nil {
		t.Fatal("center should hit under rot+scale")
	}
	// Far away must miss.
	if hit := tr.HitTest(rendering.Point{X: 500, Y: 500}); hit != nil {
		t.Fatal("far point should miss")
	}
}
