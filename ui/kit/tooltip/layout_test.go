package tooltip_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/tooltip"
	"github.com/energye/gpui/ui/rendering"
)

type tooltipSpec struct {
	PadX       float64 `json:"padX"`
	PadY       float64 `json:"padY"`
	Radius     float64 `json:"radius"`
	FontSize   float64 `json:"fontSize"`
	MaxWidth   float64 `json:"maxWidth"`
	ArrowSize  float64 `json:"arrowSize"`
	Gap        float64 `json:"gap"`
	PortalZ    int     `json:"portalZ"`
	EnterDelay float64 `json:"enterDelay"`
	LeaveDelay float64 `json:"leaveDelay"`
	TriggerH   float64 `json:"triggerH"`
	Tolerance  float64 `json:"tolerance"`
}

func loadTooltipSpec(t *testing.T) tooltipSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "tooltip_spec.json"))
	if err != nil {
		t.Fatalf("read tooltip_spec.json: %v", err)
	}
	var s tooltipSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	return s
}

func closeEnough(got, want, tol float64) bool { return math.Abs(got-want) <= tol }

// L2 metrics vs §6.2 (tolerance from testdata).
func TestTooltip_PRD_TIP18_Metrics(t *testing.T) {
	s := loadTooltipSpec(t)
	tp := tooltip.NewTooltip("metrics")
	if !closeEnough(tp.PadX(), s.PadX, s.Tolerance) || !closeEnough(tp.PadY(), s.PadY, s.Tolerance) {
		t.Fatalf("pad=%v,%v want %v,%v", tp.PadX(), tp.PadY(), s.PadX, s.PadY)
	}
	if !closeEnough(tp.Radius(), s.Radius, s.Tolerance) || !closeEnough(tp.FontSize(), s.FontSize, s.Tolerance) {
		t.Fatalf("radius=%v font=%v", tp.Radius(), tp.FontSize())
	}
	if !closeEnough(tp.MaxWidth(), s.MaxWidth, s.Tolerance) || !closeEnough(tp.ArrowSize(), s.ArrowSize, s.Tolerance) {
		t.Fatalf("maxWidth=%v arrow=%v", tp.MaxWidth(), tp.ArrowSize())
	}
	if !closeEnough(tp.Gap(), s.Gap, s.Tolerance) {
		t.Fatalf("gap=%v", tp.Gap())
	}
	if tp.ZIndex() != s.PortalZ {
		t.Fatalf("z=%d want %d", tp.ZIndex(), s.PortalZ)
	}
	if !closeEnough(tp.MouseEnterDelay(), s.EnterDelay, s.Tolerance) || !closeEnough(tp.MouseLeaveDelay(), s.LeaveDelay, s.Tolerance) {
		t.Fatalf("delay=%v/%v", tp.MouseEnterDelay(), tp.MouseLeaveDelay())
	}
	if !closeEnough(tp.TriggerHeight(), s.TriggerH, s.Tolerance) {
		t.Fatalf("triggerH=%v", tp.TriggerHeight())
	}
	sz := tp.Layout(rendering.Loose(500, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

// Exact / Min / Max each run once and assert size.
func TestTooltip_LayoutMatrix_ExactMinMax(t *testing.T) {
	mk := func() *tooltip.Tooltip {
		tp := tooltip.NewTooltip("Matrix tip text")
		tp.SetTriggerLabel("Matrix trigger")
		return tp
	}
	exact := mk()
	got := exact.Layout(rendering.Tight(320, 120))
	if math.Abs(got.Width-320) > 0.5 || math.Abs(got.Height-120) > 0.5 {
		t.Fatalf("Exact=%v want 320x120", got)
	}
	minT := mk()
	gotMin := minT.Layout(rendering.Constraints{MinWidth: 500, MaxWidth: rendering.Unbounded, MinHeight: 160, MaxHeight: rendering.Unbounded})
	if gotMin.Width < 500-0.5 || gotMin.Height < 160-0.5 {
		t.Fatalf("Min=%v want >=500x160", gotMin)
	}
	maxT := tooltip.NewTooltip("A very long tooltip title that would exceed a narrow max width budget")
	maxT.SetTriggerLabel("A similarly long trigger label that forces the ideal wider than max")
	gotMax := maxT.Layout(rendering.Constraints{MaxWidth: 220, MaxHeight: 90})
	if gotMax.Width > 220+0.5 || gotMax.Height > 90+0.5 {
		t.Fatalf("Max=%v want <=220x90", gotMax)
	}
	if gotMax.Width <= 0 || gotMax.Height <= 0 {
		t.Fatalf("Max degenerate %v", gotMax)
	}
}
