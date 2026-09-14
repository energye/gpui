package popover_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/kit/popover"
	"github.com/energye/gpui/ui/rendering"
)

// Layout matrix: Exact/Min/Max each run once via Layout(.
func TestPopover_LayoutMatrix_ExactMinMax(t *testing.T) {
	mk := func() *popover.Popover {
		p := popover.NewPopover("Matrix trigger")
		p.SetTitle("Matrix title text")
		p.SetContent("helper line with enough width")
		return p
	}
	exact := mk()
	got := exact.Layout(rendering.Tight(320, 120))
	if math.Abs(got.Width-320) > 0.5 || math.Abs(got.Height-120) > 0.5 {
		t.Fatalf("Exact=%v want 320x120", got)
	}
	minP := mk()
	gotMin := minP.Layout(rendering.Constraints{MinWidth: 500, MaxWidth: rendering.Unbounded, MinHeight: 160, MaxHeight: rendering.Unbounded})
	if gotMin.Width < 500-0.5 || gotMin.Height < 160-0.5 {
		t.Fatalf("Min=%v want >=500x160", gotMin)
	}
	maxP := popover.NewPopover("A very long trigger label that would exceed a narrow max width budget")
	maxP.SetTitle("A very long popover title that forces the ideal wider than max")
	maxP.SetContent("A similarly long helper line that forces the ideal wider than max")
	gotMax := maxP.Layout(rendering.Constraints{MaxWidth: 220, MaxHeight: 90})
	if gotMax.Width > 220+0.5 || gotMax.Height > 90+0.5 {
		t.Fatalf("Max=%v want <=220x90", gotMax)
	}
	if gotMax.Width <= 0 || gotMax.Height <= 0 {
		t.Fatalf("Max degenerate %v", gotMax)
	}
	// Panel ideal respects title floor 177 + padding 24.
	pp := popover.NewPopover("x")
	pp.SetTitle("T")
	pp.SetContent("C")
	pp.Layout(rendering.Loose(800, 600))
	if w := pp.PanelLaidOut().Width; w < 177+24-0.5 {
		t.Fatalf("panel width=%v want >=201", w)
	}
}
