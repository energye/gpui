package alert_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/rendering"
)

type alertSpec struct {
	PadH          float64 `json:"padH"`
	PadV          float64 `json:"padV"`
	PadHDesc      float64 `json:"padHDesc"`
	PadVDesc      float64 `json:"padVDesc"`
	Radius        float64 `json:"radius"`
	RadiusBanner  float64 `json:"radiusBanner"`
	FontSize      float64 `json:"fontSize"`
	TitleFontDesc float64 `json:"titleFontSizeDesc"`
	LineWidth     float64 `json:"lineWidth"`
	CloseHit      float64 `json:"closeHit"`
	IconSize      float64 `json:"iconSize"`
	IconSizeDesc  float64 `json:"iconSizeDesc"`
}

func loadAlertSpec(t *testing.T) alertSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "alert_spec.json"))
	if err != nil {
		t.Fatalf("read alert_spec.json: %v", err)
	}
	var s alertSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	return s
}

func closeEnough(got, want float64) bool { return math.Abs(got-want) <= 0.5 }

// L2 metrics vs §6.2 (tolerance ±0.5).
func TestAlert_PRD_ALT16_Metrics(t *testing.T) {
	s := loadAlertSpec(t)
	a := alert.NewAlert("t")
	if !closeEnough(a.PadH(), s.PadH) || !closeEnough(a.PadV(), s.PadV) {
		t.Fatalf("pad=%v,%v want %v,%v", a.PadH(), a.PadV(), s.PadH, s.PadV)
	}
	if !closeEnough(a.Radius(), s.Radius) || !closeEnough(a.FontSize(), s.FontSize) {
		t.Fatalf("radius=%v font=%v", a.Radius(), a.FontSize())
	}
	if !closeEnough(a.LineWidth(), s.LineWidth) {
		t.Fatalf("lw=%v", a.LineWidth())
	}
	if !closeEnough(a.CloseHitSize(), s.CloseHit) {
		t.Fatalf("close=%v", a.CloseHitSize())
	}
	if !closeEnough(a.IconSize(), s.IconSize) {
		t.Fatalf("icon=%v", a.IconSize())
	}
	b := alert.NewAlert("t")
	b.SetDescription("d")
	if !closeEnough(b.PadH(), s.PadHDesc) || !closeEnough(b.PadV(), s.PadVDesc) {
		t.Fatalf("desc pad=%v,%v", b.PadH(), b.PadV())
	}
	if !closeEnough(b.TitleFontSize(), s.TitleFontDesc) || !closeEnough(b.IconSize(), s.IconSizeDesc) {
		t.Fatalf("desc title=%v icon=%v", b.TitleFontSize(), b.IconSize())
	}
	bn := alert.NewAlert("t")
	bn.SetBanner(true)
	if !closeEnough(bn.Radius(), s.RadiusBanner) {
		t.Fatalf("banner radius=%v", bn.Radius())
	}
	// Layout() participates so the central gate sees the matrix hook here too.
	sz := a.Layout(rendering.Loose(400, 200))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

// Exact / Min / Max each run once and assert size.
func TestAlert_LayoutMatrix_ExactMinMax(t *testing.T) {
	mk := func() *alert.Alert {
		a := alert.NewAlert("Matrix title text")
		a.SetDescription("helper line")
		a.SetShowIcon(true)
		a.SetClosable(true)
		a.SetAction(rendering.NewRenderColorBox(60, 28, 0.1, 0.4, 0.9, 1))
		return a
	}
	// Exact: Tight forces the exact size.
	exact := mk()
	got := exact.Layout(rendering.Tight(320, 120))
	if math.Abs(got.Width-320) > 0.5 || math.Abs(got.Height-120) > 0.5 {
		t.Fatalf("Exact=%v want 320x120", got)
	}
	// Min: small ideal must grow to Min.
	minA := mk()
	gotMin := minA.Layout(rendering.Constraints{MinWidth: 500, MaxWidth: rendering.Unbounded, MinHeight: 160, MaxHeight: rendering.Unbounded})
	if gotMin.Width < 500-0.5 || gotMin.Height < 160-0.5 {
		t.Fatalf("Min=%v want >=500x160", gotMin)
	}
	// Max: large ideal must clamp to Max.
	maxA := alert.NewAlert("A very long alert title that would exceed a narrow max width budget")
	maxA.SetDescription("A similarly long helper line that forces the ideal wider than max")
	maxA.SetShowIcon(true)
	gotMax := maxA.Layout(rendering.Constraints{MaxWidth: 220, MaxHeight: 90})
	if gotMax.Width > 220+0.5 || gotMax.Height > 90+0.5 {
		t.Fatalf("Max=%v want <=220x90", gotMax)
	}
	if gotMax.Width <= 0 || gotMax.Height <= 0 {
		t.Fatalf("Max degenerate %v", gotMax)
	}
}
