package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

func TestProgress_Lifecycle_SetterOrder(t *testing.T) {
	p := kit.NewProgress(30)
	p.SetType(kit.ProgressLine)
	p.SetShowInfo(true)
	p.SetWidth(200)
	p.SetPercent(75)
	p.SetStatus(kit.ProgressStatusActive)
	if p.Node() == nil {
		t.Fatal("nil root")
	}
	if p.FillRatio() < 0.7 {
		t.Fatalf("fill=%v", p.FillRatio())
	}
}

func TestProgress_SkinType_UsesKitProgressPainter(t *testing.T) {
	p := kit.NewProgress(50)
	th := skindefault.Theme()
	if th.Painter(kit.TypeProgress) == nil {
		t.Fatal("missing painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeProgress, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	p.SetTheme(th)
	tree := core.NewTree(p.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 240, Height: 40})
	dc := render.NewContext(240, 40)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	p.ChromeNode().Paint(pc)
	if !painted {
		t.Fatal("painter not invoked")
	}
}

func TestProgress_EnsureBuilt_Node(t *testing.T) {
	p := kit.NewProgress(10)
	if p.Node() == nil || p.ChromeNode() == nil {
		t.Fatal("not built")
	}
}
