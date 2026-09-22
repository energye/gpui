package kit

import (
	"testing"

	"github.com/energye/gpui/ui/kit/internal/scope"
)

func TestIcon_PRD_State(t *testing.T) {
	resetIconSources()
	// ICO-05 spin advances.
	ctx := scope.DefaultCtx()
	sp := DefaultIconProps("sync")
	sp.Spin = true
	in := BuildIcon(ctx, sp)
	a0 := in.EffectiveAngle()
	if !in.Tick(0.45) {
		t.Fatalf("spin tick should advance")
	}
	if in.EffectiveAngle() == a0 {
		t.Fatalf("angle did not advance")
	}
	// ICO-06 reduced motion freezes.
	rm := ctx.WithMotion(scope.MotionConfig{Enabled: true, ReducedMotion: true})
	fr := BuildIcon(rm, sp)
	fa := fr.EffectiveAngle()
	if fr.Tick(0.45) {
		t.Fatalf("reduced-motion tick must return false")
	}
	if fr.EffectiveAngle() != fa {
		t.Fatalf("reduced-motion angle moved")
	}
	// ICO-07 rotate 180 participates.
	rp := DefaultIconProps("check")
	rp.Rotate = 180
	ri := BuildIcon(ctx, rp)
	if ri.EffectiveAngle() != 180 {
		t.Fatalf("rotate angle=%v want 180", ri.EffectiveAngle())
	}
	// ICO-09 decorative defaults: no tab, hit defer, no role.
	d := NewIcon("check")
	if d.TabStop() {
		t.Fatalf("pure icon must not take tab")
	}
	if !d.HitDefer() {
		t.Fatalf("decorative icon must hit-defer")
	}
	if d.Role() != "" {
		t.Fatalf("decorative icon role must be empty")
	}
	// ICO-18 labeled icon exposes img role.
	lp := DefaultIconProps("home")
	lp.AriaLabel = "home icon"
	li := BuildIcon(ctx, lp)
	if li.Role() != "img" || li.HitDefer() {
		t.Fatalf("labeled icon must be img without defer")
	}
	// Idle tick returns false so hosts drop it.
	idle := NewIcon("check")
	if idle.Tick(0.1) {
		t.Fatalf("idle tick must return false")
	}
}
