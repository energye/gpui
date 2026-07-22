package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestAuditHitPaint_TabsNestedControls(t *testing.T) {
	btn := kit.NewButton("Primary")
	cb := kit.NewCheckbox("agree")
	sw := kit.NewSwitch()
	sel := kit.NewSelect("pick", kit.SelectOption{Value: "1", Label: "One"})
	panel := primitive.Column(btn.Node(), cb.Node(), sw.Node(), sel.Node())
	panel.MainAlign = core.MainStart
	panel.CrossAlign = core.CrossStart
	panel.Gap = 12
	panel.Padding = primitive.All(12)

	tabs := kit.NewTabs(kit.MenuItem{Key: "x", Label: "X"})
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetContent("x", panel)
	tabs.SetActive("x")

	host := primitive.NewFlexible(1, tabs.Node())
	col := primitive.Column(primitive.NewText("title"), host)
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	root := primitive.NewBox(col)
	root.Width, root.Height = 1024, 768

	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 1024, Height: 768})

	issues := core.AuditHitPaintContract(root)
	for _, is := range issues {
		t.Errorf("%s: %s", is.TypeID, is.Message)
	}

	// Paint origin == AbsoluteOffset for every pressable under tabs body.
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok {
			abs := core.AbsoluteBounds(p)
			po := core.PaintOriginFor(p)
			if abs.Min.X != po.X || abs.Min.Y != po.Y {
				t.Errorf("pressable AbsoluteBounds.Min=%v PaintOrigin=%v", abs.Min, po)
			}
			if abs.Min.Y > 200 {
				// first body controls must stick near top of gallery
				t.Logf("pressable absY=%.1f size=%v (ok if later in column)", abs.Min.Y, p.Size())
			}
			// Child must sit inside pressable (hit box)
			for _, ch := range p.Children() {
				off := ch.Base().Offset()
				if off.Y > p.Size().Height+0.5 {
					t.Errorf("pressable child offset.Y=%.1f > size.H=%.1f", off.Y, p.Size().Height)
				}
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)

	// Hit at button center works; mid-window does not hit first button.
	bAbs := core.AbsoluteBounds(btn.Root)
	cx := (bAbs.Min.X + bAbs.Max.X) / 2
	cy := (bAbs.Min.Y + bAbs.Max.Y) / 2
	if hit := tree.HitTest(core.Point{X: cx, Y: cy}); hit != btn.Root {
		t.Fatalf("button center hit=%T", hit)
	}
	if bAbs.Min.Y < 100 {
		if hit := tree.HitTest(core.Point{X: cx, Y: 400}); hit == btn.Root {
			t.Fatal("button still hittable at y=400")
		}
	}
}

func TestPaintOriginEqualsAbsoluteOffset(t *testing.T) {
	a := primitive.NewBox()
	a.Width, a.Height = 10, 10
	b := primitive.NewBox(a)
	b.Width, b.Height = 100, 100
	// manual offset
	a.Base().SetOffset(core.Point{X: 3, Y: 7})
	if po := core.PaintOriginFor(a); po.X != 3 || po.Y != 7 {
		// parent offset 0
		t.Fatalf("origin=%v", po)
	}
	// with parent offset
	b.Base().SetOffset(core.Point{X: 1, Y: 2})
	// AbsoluteOffset walks parents including self
	po := core.PaintOriginFor(a)
	// a.offset(3,7) + b.offset(1,2) = (4,9)
	if po.X != 4 || po.Y != 9 {
		t.Fatalf("nested origin=%v want 4,9", po)
	}
}
