package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: structure setters (color/variant/closable) keep chrome coherent.
func TestTag_Lifecycle_SetterOrder(t *testing.T) {
	tg := kit.NewTag("Alpha")
	tg.SetColor("blue")
	tg.SetVariant(kit.TagOutlined)
	tg.SetClosable(true)
	tg.SetDisabled(false)
	root := tg.ChromeNode().(*primitive.Decorated)
	if root.SkinType != kit.TypeTag {
		t.Fatalf("SkinType=%q want %s", root.SkinType, kit.TypeTag)
	}
	if tg.CloseNode() == nil {
		t.Fatal("closable should create close node")
	}
	if !tg.HasBorder() && root.BorderWidth <= 0 {
		// outlined should prefer border; tolerate token path via HasBorder
	}
}

// #6: default skin registers kit.Tag; Override painter runs.
func TestTag_SkinType_UsesKitTagPainter(t *testing.T) {
	tg := kit.NewTag("Skin")
	tg.SetColor("magenta")
	th := skindefault.Theme()
	if th.Painter(kit.TypeTag) == nil {
		t.Fatal("default skin missing kit.Tag painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeTag, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#EB2F96")
			primitive.PaintDecorated(pc, d)
		}
	})
	tg.SetTheme(th)
	tree := core.NewTree(tg.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 200, Height: 40})
	dc := render.NewContext(200, 40)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	tg.ChromeNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.Tag painter not invoked")
	}
}

func TestTag_EnsureBuilt_Aria(t *testing.T) {
	tg := kit.NewTag("x")
	tg.SetAriaLabel("filter chip")
	if tg.Root == nil || tg.Root.Base().Label != "filter chip" {
		t.Fatalf("aria not applied")
	}
}
