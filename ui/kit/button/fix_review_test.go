package button_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
)

// TestButton_FixReview_DisabledStyleOverride: custom-disabled-bg demo —
// business Style / semantic root hooks tune the disabled gray instead of
// being swallowed (Default 0.1, Dashed 0.4 keep their gray + no click).
func TestButton_FixReview_DisabledStyleOverride(t *testing.T) {
	mk := func(typ button.ButtonType, bg float64) *button.Button {
		b := button.NewButton("Default Button")
		b.SetType(typ)
		b.SetDisabled(true)
		b.SetStyle(button.Style{
			Bg:    render.RGBA{R: bg, G: bg, B: bg, A: 1},
			UseBg: true,
		})
		b.Layout(rendering.Loose(1000, 1000))
		return b
	}
	def := mk(button.ButtonDefault, 0.1)
	if got := def.Fill(); got.R < 0.05 || got.R > 0.15 {
		t.Fatalf("default disabled fill=%v want ~0.1 gray", got)
	}
	dash := mk(button.ButtonDashed, 0.4)
	if got := dash.Fill(); got.R < 0.35 || got.R > 0.45 {
		t.Fatalf("dashed disabled fill=%v want ~0.4 gray", got)
	}
	for _, b := range []*button.Button{def, dash} {
		w, h := b.LaidOut().Width, b.LaidOut().Height
		if b.PointerDown(w/2, h/2) {
			t.Fatal("disabled must swallow press even with style override")
		}
	}
}

// TestButton_FixReview_NeutralTextHover: default text button hovers on
// colorBgTextHover (rgba(0,0,0,0.06)), not the lighter FillTertiary.
func TestButton_FixReview_NeutralTextHover(t *testing.T) {
	b := button.NewButton("Text Button")
	b.SetType(button.ButtonText)
	b.Layout(rendering.Loose(1000, 1000))
	w, h := b.LaidOut().Width, b.LaidOut().Height
	b.PointerMove(w/2, h/2)
	if got := b.Fill(); got.A < 0.05 || got.A > 0.07 {
		t.Fatalf("neutral text hover fill=%v want alpha ~0.06", got)
	}
}

// TestButton_FixReview_LoadingReservesSlot: loading reserves the leading
// slot even while a P1 delay hides the spinner (C7) — the label must not
// jump when the ring appears.
func TestButton_FixReview_LoadingReservesSlot(t *testing.T) {
	b := button.NewButton("Loading")
	b.SetLoadingDelay(60 * time.Second)
	b.SetLoading(true)
	b.Layout(rendering.Loose(1000, 1000))
	plain := button.NewButton("Loading")
	plain.Layout(rendering.Loose(1000, 1000))
	if b.LaidOut().Width <= plain.LaidOut().Width {
		t.Fatalf("loading width %.1f must exceed plain %.1f (reserved slot)",
			b.LaidOut().Width, plain.LaidOut().Width)
	}
	if b.HasSpinner() {
		t.Fatal("delay must hide the spinner before the deadline")
	}
}

// TestButton_FixReview_LoadingFocusable: disabled AND loading both leave
// the Tab order (§6.9.0 keyboard rule).
func TestButton_FixReview_LoadingFocusable(t *testing.T) {
	b := button.NewButton("Loading")
	b.SetLoading(true)
	if b.Focusable() {
		t.Fatal("loading button must not be focusable")
	}
	if b.FocusNode().Enabled {
		t.Fatal("loading focus node must be disabled")
	}
	d := button.NewButton("Disabled")
	d.SetDisabled(true)
	if d.Focusable() {
		t.Fatal("disabled button must not be focusable")
	}
}

// TestButton_FixReview_WaveEffectHook: Inset/Shake hooks install and paint
// paths record their snapshot inputs.
func TestButton_FixReview_WaveEffectHook(t *testing.T) {
	b := button.NewButton("Inset")
	b.SetWaveEffect(button.WaveEffectInset)
	if b.WaveEffect() != button.WaveEffectInset {
		t.Fatalf("wave effect=%v want inset", b.WaveEffect())
	}
	b.SetWaveEffect(button.WaveEffectShake)
	if b.WaveEffect() != button.WaveEffectShake {
		t.Fatalf("wave effect=%v want shake", b.WaveEffect())
	}
}

// TestButton_FixReview_StyleGeometry: Style radius/padding overrides win;
// semantic root radius/padding are the fallback.
func TestButton_FixReview_StyleGeometry(t *testing.T) {
	b := button.NewButton("Object")
	b.SetStyle(button.Style{Radius: 2, UseRadius: true, Padding: 20, UsePadding: true})
	b.Layout(rendering.Loose(1000, 1000))
	if got := b.Radius(); got != 2 {
		t.Fatalf("style radius=%.1f want 2", got)
	}
	if got := b.PaddingInline(); got != 20 {
		t.Fatalf("style padding=%.1f want 20", got)
	}
	c := button.NewButton("Function")
	c.SetSemanticStyle(button.SemanticRoot, button.Style{Radius: 3, UseRadius: true})
	c.Layout(rendering.Loose(1000, 1000))
	if got := c.Radius(); got != 3 {
		t.Fatalf("semantic radius=%.1f want 3", got)
	}
}

// TestButton_OnScreenDemand: SetOnScreen is the shell's visibility control
// verb — off-screen spinners leave the ticker registry (no frames, phase
// kept), scrolling back resumes demand with no re-attach handshake.
func TestButton_OnScreenDemand(t *testing.T) {
	reg := &scheduler.TickerRegistry{}
	b := button.NewButton("Loading")
	b.SetLoading(true)
	b.Layout(rendering.Loose(1000, 1000))
	if !b.OnScreen() {
		t.Fatal("new button must default to on-screen")
	}
	b.Attach(reg)
	if !reg.HasActive() {
		t.Fatal("attached spinner must hold the registry")
	}
	b.SetOnScreen(false)
	if b.OnScreen() {
		t.Fatal("SetOnScreen(false) must report off-screen")
	}
	if reg.HasActive() {
		t.Fatal("off-screen spinner must leave the registry")
	}
	reg.TickAll(1.0 / 60)
	if reg.HasActive() {
		t.Fatal("off-screen spinner must stay out after ticks")
	}
	b.SetOnScreen(true)
	if !b.OnScreen() {
		t.Fatal("SetOnScreen(true) must report on-screen")
	}
	if !reg.HasActive() {
		t.Fatal("back on screen must resume the registry without re-attach")
	}
	if !b.WantsFrame() {
		t.Fatal("visible spinner must want frames")
	}
	b.Detach()
	if reg.HasActive() {
		t.Fatal("detach must release the registry")
	}
}
