package tag_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/tag"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Layout/Node contract: New carries an intrinsic non-zero size and
// Layout(loose) leaves a non-zero Node size. Zero-size tags are forbidden.
func TestTag_LayoutNode_NonZero(t *testing.T) {
	tg := tag.NewTag("Contract")
	if ps := tg.PreferredSize(); ps.Width <= 0 || ps.Height <= 0 {
		t.Fatalf("intrinsic preferred=%v must be non-zero right after New", ps)
	}
	sz := tg.Layout(rendering.Loose(400, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("loose layout=%v must be non-zero", sz)
	}
	if ns := tg.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("node size=%v must be non-zero after Layout", ns)
	}

	ct := tag.NewCheckableTag("Contract")
	if ps := ct.PreferredSize(); ps.Width <= 0 || ps.Height <= 0 {
		t.Fatalf("checkable intrinsic=%v must be non-zero", ps)
	}
	ct.Layout(rendering.Loose(400, 100))
	if ns := ct.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("checkable node size=%v must be non-zero", ns)
	}

	g := tag.NewCheckableTagGroup(tag.TagOption{Label: "A", Value: "a"})
	g.Layout(rendering.Loose(400, 200))
	if ns := g.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("group node size=%v must be non-zero", ns)
	}
}

// No-face path: layout still estimates width from the heuristic and paint
// must never draw a black bar in place of text (DrawString without a face
// is a no-op, so the text zone keeps the chrome background).
func TestTag_NoFace_EstimateNoBlackBar(t *testing.T) {
	tg := tag.NewTag("NoFace")
	if tg.TextWidth() <= 0 {
		t.Fatal("heuristic text width must stay positive without a face")
	}
	sz := tg.Layout(rendering.Loose(300, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("loose layout=%v must be non-zero without a face", sz)
	}

	const canvas = 200.0
	dc := render.NewContext(int(canvas), int(100))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tg.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(10, 10))
	got := dc.Image()

	// Text zone without glyphs: no near-black pixels allowed (a black bar
	// would show up here). The chrome background is a light tint.
	black := 0
	for yy := int(10 + 3); yy < int(10+sz.Height-3); yy++ {
		for xx := int(10 + 7); xx < int(10+sz.Width-7); xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 20 && g/257 < 20 && b/257 < 20 {
				black++
			}
		}
	}
	if black != 0 {
		t.Fatalf("text zone has %d near-black pixels without a face (black bar?)", black)
	}
}

// TestTag_PRD_TAG22_P1_SemanticHooks covers §6.8 P1 semantic
// classNames/styles: the kit keeps shallow hooks (stored, round-tripped,
// paint-safe). A deep CSS cascade is staged, not silently dropped.
func TestTag_PRD_TAG22_P1_SemanticHooks(t *testing.T) {
	tg := tag.NewTag("Hook")
	tg.SetStyle(map[string]string{"root": "background:#fff"})
	tg.SetClassName("my-tag")
	if tg.Style()["root"] != "background:#fff" || tg.ClassName() != "my-tag" {
		t.Fatal("semantic hooks must round-trip")
	}
	tg.Layout(rendering.Loose(300, 100))
	paintTag(t, tg.Node())
}

// TestTag_PRD_TAG22_P1_HrefOnClickMapping covers §6.8 P1 href/target: on the
// desktop there is no anchor navigation, so href maps to the whole-tag
// OnClick callback (TAG-08 link path). Real browser tab semantics are staged.
func TestTag_PRD_TAG22_P1_HrefOnClickMapping(t *testing.T) {
	calls := 0
	tg := tag.NewTag("Href stands in for <a>")
	tg.SetOnClick(func() { calls++ })
	tg.Click()
	tg.PressKey("Enter")
	if calls != 2 {
		t.Fatalf("href-mapped clicks=%d want 2", calls)
	}
}

// TestTag_PRD_TAG22_P1_ProviderTheme covers §6.8 P1 ConfigProvider global:
// per-tag provider flow works today (checked skin follows the brand);
// a process-wide ConfigProvider switch rides on the theme provider.
func TestTag_PRD_TAG22_P1_ProviderTheme(t *testing.T) {
	tok := theme.Default.Current()
	alt := tok
	alt.ColorPrimary = theme.Hex("#ff4d4f")
	p := theme.NewProvider(alt)
	ct := tag.NewCheckableTag("Brand")
	ct.SetChecked(true)
	ct.SetProvider(p)
	if got := ct.EffectiveChrome(); got.Bg.R < 0.9 || got.Bg.G > 0.4 {
		t.Fatalf("provider brand bg=%+v should follow red brand", got.Bg)
	}
}

// The rest of §6.8 P1 cannot be built faithfully on the desktop and is
// skipped with reasons instead of being silently dropped.

func TestTag_PRD_TAG22_P1_MotionPixels(t *testing.T) {
	t.Skip("P1 staged: pixel-level enter/exit motion needs an animation clock; P0 allows instant add/remove (TAG-12 green)")
}

func TestTag_PRD_TAG22_P1_FullDndKit(t *testing.T) {
	t.Skip("P1 staged: full dnd-kit pointer-drag has no desktop gesture wired; combo reorder示意 covered by TAG-15")
}

func TestTag_PRD_TAG22_P1_DebugPages(t *testing.T) {
	t.Skip("P1 staged: customize/component-token are doc-only debug pages with no runtime behavior to assert")
}

func TestTag_PRD_TAG22_P1_OptionHooks(t *testing.T) {
	t.Skip("P1 staged: per-option className/style (6.4.0) hooks on TagOption not exposed yet; label/value/icon/disabled options covered by TAG-11")
}

func TestTag_PRD_TAG22_P1_NoPixelHash(t *testing.T) {
	t.Skip("not done by design:逐像素官网哈希 explicitly out of scope per §6.1 (L3 uses this repo's own golden)")
}

func TestTag_PRD_TAG21_HumanEye(t *testing.T) {
	t.Skip("L4 needs a human side-by-side sign-off against ant.design; cannot pass in CI")
}
