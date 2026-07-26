package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: structure setters then theme still rebuild a valid trail.
func TestBreadcrumb_Lifecycle_SetterOrder(t *testing.T) {
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home", Link: true},
		kit.BreadcrumbItem{Title: "List"},
	)
	b.SetSeparator(">")
	b.SetItems([]kit.BreadcrumbItem{
		{Title: "A", Link: true},
		{Title: "B", Link: true},
		{Title: "C"},
	})
	b.SetTheme(kit.DefaultTheme())
	if b.ItemCount() != 3 {
		t.Fatalf("items=%d want 3", b.ItemCount())
	}
	if b.ResolvedSeparator() != ">" {
		t.Fatalf("sep=%q", b.ResolvedSeparator())
	}
	root := b.ChromeNode()
	if root == nil {
		t.Fatal("root nil")
	}
	if flex, ok := root.(*primitive.Flex); !ok || flex.SkinType != kit.TypeBreadcrumb {
		t.Fatalf("SkinType=%v want %s", root, kit.TypeBreadcrumb)
	}
}

// #6: default skin registers kit.Breadcrumb; Override painter is invoked.
func TestBreadcrumb_SkinType_UsesKitBreadcrumbPainter(t *testing.T) {
	b := kit.NewBreadcrumb(kit.BreadcrumbItem{Title: "Home"})
	flex := b.ChromeNode().(*primitive.Flex)
	if flex.SkinType != kit.TypeBreadcrumb {
		t.Fatalf("SkinType=%q", flex.SkinType)
	}
	th := skindefault.Theme()
	if th.Painter(kit.TypeBreadcrumb) == nil {
		t.Fatal("default skin missing kit.Breadcrumb painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeBreadcrumb, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if f, ok := n.(*primitive.Flex); ok {
			// Demo chrome: soft background under the trail.
			sz := f.Size()
			if pc != nil && sz.Width > 0 && sz.Height > 0 {
				pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, 4, render.Hex("#FFF0F6"))
			}
			f.DefaultPaintChildren(pc)
		}
	})
	b.SetTheme(th)
	tree := core.NewTree(b.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 400, Height: 48})
	dc := render.NewContext(400, 48)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	b.ChromeNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.Breadcrumb painter not invoked")
	}
}

func TestBreadcrumb_EnsureBuilt_AriaLabel(t *testing.T) {
	b := kit.NewBreadcrumb(kit.BreadcrumbItem{Title: "X"})
	b.SetAriaLabel("Trail")
	if b.Root == nil || b.Root.Base().Label != "Trail" {
		t.Fatalf("aria label not applied")
	}
}
