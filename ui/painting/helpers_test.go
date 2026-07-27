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
