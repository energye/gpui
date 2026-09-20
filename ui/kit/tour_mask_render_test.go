package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestTourMask_GeometryMatchesCases(t *testing.T) {
	cases := loadTourCases(t)
	geo := cases["geometry"].(map[string]any)
	hole := geo["hole"].(map[string]any)
	tgt := hole["target"].(map[string]any)
	target := kit.TourMaskRect{
		X: tgt["x"].(float64), Y: tgt["y"].(float64),
		W: tgt["w"].(float64), H: tgt["h"].(float64),
	}
	got := kit.ComputeTourMaskHole(target, kit.DefaultTourMaskGap())
	if got.X != hole["want_x"].(float64) || got.Y != hole["want_y"].(float64) ||
		got.W != hole["want_w"].(float64) || got.H != hole["want_h"].(float64) {
		t.Fatalf("hole = %.1f,%.1f,%.1f,%.1f", got.X, got.Y, got.W, got.H)
	}
	if got.Radius != hole["gap_radius"].(float64) {
		t.Fatalf("radius = %v want 2", got.Radius)
	}
	center := geo["panel_center"].(map[string]any)
	pc := kit.ComputeTourMaskPanel(1200, 800, kit.TourMaskHoleRect{}, kit.TourMaskRect{}, kit.TourMaskPlacementCenter, 1001)
	if pc.X != center["want_x"].(float64) || pc.Y != center["want_y"].(float64) || pc.W != center["want_w"].(float64) {
		t.Fatalf("center panel = %.1f,%.1f,%.1f", pc.X, pc.Y, pc.W)
	}
	pb := kit.ComputeTourMaskPanel(1200, 800, got, target, kit.TourMaskPlacementBottom, 1001)
	if pb.W != 520 {
		t.Fatalf("bottom panel w = %v want 520", pb.W)
	}
	if pb.Y <= got.Y+got.H {
		t.Fatalf("bottom panel must sit below hole, y=%v hole bottom=%v", pb.Y, got.Y+got.H)
	}
	if z := kit.ResolveTourMaskZ(kit.DefaultTourMaskProps()); z != 1001 {
		t.Fatalf("tour z = %d want 1001 above modal 1000", z)
	}
	if ind := kit.ResolveTourMaskIndicatorText(0, 3, "zh-CN"); ind != "1 / 3" {
		t.Fatalf("indicator = %q", ind)
	}
}
