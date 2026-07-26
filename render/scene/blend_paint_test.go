package scene

import (
	"testing"

	"github.com/energye/gpui/render"
)

// Ensure scene wire values are not silently equal to paint-level values for
// modes that historically caused wrong blends under raw casts.
func TestPaintBlendMode_RoundTripAndClearDivergence(t *testing.T) {
	// scene.BlendClear is 16 (after HSL); paint BlendClear is 8.
	if uint32(BlendClear) == uint32(render.BlendClear) {
		t.Fatalf("scene.BlendClear unexpectedly equals paint BlendClear (%d); raw cast would hide bugs", BlendClear)
	}
	if BlendClear.ToPaintBlendMode() != render.BlendClear {
		t.Fatalf("ToPaintBlendMode(Clear)=%v want %v", BlendClear.ToPaintBlendMode(), render.BlendClear)
	}
	if PaintBlendModeToScene(render.BlendClear) != BlendClear {
		t.Fatalf("PaintBlendModeToScene(Clear)=%v want scene.BlendClear", PaintBlendModeToScene(render.BlendClear))
	}

	// Darken: scene=4, paint Darken is after Porter-Duff block — must not match.
	if uint32(BlendDarken) == uint32(render.BlendDarken) {
		t.Fatalf("scene.BlendDarken equals paint value %d; conversion still required", BlendDarken)
	}
	if BlendDarken.ToPaintBlendMode() != render.BlendDarken {
		t.Fatalf("ToPaintBlendMode(Darken)=%v want paint Darken", BlendDarken.ToPaintBlendMode())
	}

	// First four CSS modes intentionally share values 0..3.
	for _, m := range []struct {
		s BlendMode
		p render.BlendMode
	}{
		{BlendNormal, render.BlendNormal},
		{BlendMultiply, render.BlendMultiply},
		{BlendScreen, render.BlendScreen},
		{BlendOverlay, render.BlendOverlay},
	} {
		if m.s.ToPaintBlendMode() != m.p {
			t.Fatalf("%s.ToPaintBlendMode()=%v want %v", m.s, m.s.ToPaintBlendMode(), m.p)
		}
		if uint32(m.s) != uint32(m.p) {
			t.Fatalf("%s wire value %d != paint %d (0..3 should match)", m.s, m.s, m.p)
		}
	}
}
