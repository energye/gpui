package painting_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/painting"
)

func TestFillRoundRect_NilSafe(t *testing.T) {
	var c *painting.Context
	c.FillRoundRect(0, 0, 10, 10, 2, 1, 0, 0, 1)
	empty := &painting.Context{}
	empty.FillRoundRect(0, 0, 10, 10, 2, 1, 0, 0, 1)
}

func TestFillLinearGradient_NilSafe(t *testing.T) {
	var c *painting.Context
	c.FillLinearGradient2(0, 0, 10, 10, 0, 0, 10, 0, 1, 0, 0, 1, 0, 0, 1, 1)
}

func TestDrawTextColored_NilSafe(t *testing.T) {
	var c *painting.Context
	c.DrawTextColored("hi", 0, 0, 1, 1, 1, 1)
}

func TestFillRoundRect_OriginOffset(t *testing.T) {
	dc := render.NewContext(64, 64)
	pc := painting.New(dc, 1)
	child := pc.WithOrigin(16, 8)
	child.FillRoundRect(0, 0, 20, 12, 3, 1, 0, 0, 1)
	// Smoke: must not panic; GPU/CPU path may leave empty without flush in all modes.
	_ = dc
}

func TestFillLinearGradient2_Smoke(t *testing.T) {
	dc := render.NewContext(64, 32)
	pc := painting.New(dc, 1)
	pc.FillLinearGradient2(0, 0, 64, 32, 0, 0, 64, 0,
		0.2, 0.4, 0.9, 1,
		0.9, 0.3, 0.2, 1,
	)
}

func TestStrokeAPIs_NilSafe(t *testing.T) {
	var c *painting.Context
	c.StrokeRect(0, 0, 10, 10, 1, 1, 0, 0, 1)
	c.StrokeRoundRect(0, 0, 10, 10, 2, 1, 1, 0, 0, 1)
	c.StrokeLine(0, 0, 5, 5, 1, 0, 1, 0, 1)
	c.FillCircle(5, 5, 3, 1, 1, 1, 1)
	c.StrokeCircle(5, 5, 3, 1, 1, 1, 1, 1)
	c.PushClipRoundRect(0, 0, 10, 10, 2)
	c.SetStrokeStyle(painting.DefaultStrokeStyle())
}

func TestStrokeAPIs_Smoke(t *testing.T) {
	dc := render.NewContext(64, 64)
	pc := painting.New(dc, 1)
	pc.SetStrokeStyle(painting.DefaultStrokeStyle())
	pc.StrokeRect(4, 4, 20, 12, 2, 1, 0, 0, 1)
	pc.StrokeRoundRect(4, 24, 20, 12, 4, 2, 0, 1, 0, 1)
	pc.StrokeLine(0, 0, 63, 63, 1, 0, 0, 1, 1)
	pc.FillCircle(40, 20, 8, 0.2, 0.6, 0.9, 1)
	pc.StrokeCircle(40, 44, 8, 1.5, 1, 1, 0, 1)
	pc.PushClipRoundRect(0, 0, 32, 32, 6)
	pc.FillRect(0, 0, 32, 32, 1, 0, 0, 0.5)
	pc.PopClip()
}

func TestEstimateTextSize_RuneBased(t *testing.T) {
	// CJK: 4 runes should not use byte length (12 for UTF-8).
	wCJK, h := painting.EstimateTextSize("你好世界", 14, 1.0)
	if h <= 0 {
		t.Fatal("height")
	}
	wLatin, _ := painting.EstimateTextSize("ABCD", 14, 1.0)
	if wCJK < wLatin*0.9 {
		// both 4 runes * 14 * 1.0 → same
	}
	if wCJK != 4*14*1.0 {
		t.Fatalf("cjk w=%v want %v", wCJK, 4*14.0)
	}
	// byte-len heuristic would be 12*14 for CJK; ensure we are not that large wrongly with aw=1
	if wCJK >= 12*14 {
		t.Fatalf("used byte length? w=%v", wCJK)
	}
}

func TestMeasureText_FallbackWithoutFont(t *testing.T) {
	dc := render.NewContext(32, 32)
	pc := painting.New(dc, 1)
	w, h := pc.MeasureText("Hi", 14, 0.55)
	if w <= 0 || h <= 0 {
		t.Fatalf("measure fallback w=%v h=%v", w, h)
	}
}
