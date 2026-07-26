package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: count/child order keeps badge chrome.
func TestBadge_Lifecycle_SetterOrder(t *testing.T) {
	b := kit.NewBadge()
	b.SetCount(5)
	if b.DisplayCount() != "5" {
		t.Fatalf("DisplayCount=%q want 5", b.DisplayCount())
	}
	b.SetShowZero(true)
	b.SetCount(0)
	if b.DisplayCount() != "0" {
		t.Fatalf("DisplayCount=%q want 0", b.DisplayCount())
	}
	// Dot mode: clear count path
	b.SetCount(0)
	b.SetShowZero(false)
	b.SetDot(true)
	// ensureBuilt paths
	if b.Node() == nil || b.ChromeNode() == nil {
		t.Fatal("host not built")
	}
	// Status + text
	b.SetDot(false)
	b.SetStatus(kit.BadgeStatusSuccess)
	b.SetText("Success")
	if b.Node() == nil {
		t.Fatal("status rebuild failed")
	}
}

// #6: default skin registers kit.Badge; Override painter runs.
func TestBadge_SkinType_UsesKitBadgePainter(t *testing.T) {
	b := kit.NewBadge()
	b.SetCount(9)
	th := skindefault.Theme()
	if th.Painter(kit.TypeBadge) == nil {
		t.Fatal("default skin missing kit.Badge painter")
	}
	var painted int
	th.Skin = core.Override(th.Skin, kit.TypeBadge, func(pc *core.PaintContext, n core.Node) {
		painted++
		if d, ok := n.(*primitive.Decorated); ok {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#FA541C")
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	b.SetTheme(th)
	tree := core.NewTree(b.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 80, Height: 80})
	dc := render.NewContext(80, 80)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	// Paint host (badgeHost TypeID kit.Badge)
	b.ChromeNode().Paint(pc)
	// Paint count capsule if Decorated
	if ind := b.IndicatorNode(); ind != nil {
		ind.Paint(pc)
	}
	if painted < 1 {
		t.Fatal("overridden kit.Badge painter not invoked")
	}
}

func TestBadge_EnsureBuilt_Count(t *testing.T) {
	b := kit.NewBadge()
	b.SetCount(5)
	if b.DisplayCount() != "5" {
		t.Fatalf("DisplayCount=%q", b.DisplayCount())
	}
	if b.Node() == nil {
		t.Fatal("host not built")
	}
}
