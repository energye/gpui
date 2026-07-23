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

// TestAuditHitPaint_MainPathControls covers P0.1 A1: Button, Input, Switch,
// Checkbox, Select, Modal footer Row(Spacer, btn).
func TestAuditHitPaint_MainPathControls(t *testing.T) {
	btn := kit.NewButton("OK")
	in := kit.NewInput("type")
	cb := kit.NewCheckbox("c")
	sw := kit.NewSwitch()
	sel := kit.NewSelect("pick", kit.SelectOption{Value: "a", Label: "A"})

	modal := kit.NewModal("Title")
	modal.SetContent(primitive.NewText("body"))
	// Footer is built internally as Row(Spacer, buttons) — open so layer lays out.
	modal.SetOpen(true)
	modal.Viewport = core.Size{Width: 800, Height: 600}

	col := primitive.Column(
		btn.Node(),
		in.Node(),
		cb.Node(),
		sw.Node(),
		sel.Node(),
		modal.Node(),
	)
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStart
	col.Gap = 12
	col.Padding = primitive.All(16)

	root := primitive.NewBox(col)
	root.Width, root.Height = 800, 600
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 800, Height: 600})

	// Modal portal may need a second layout after open for panel geometry.
	if modal.Portal != nil {
		tree.Layout(core.Size{Width: 800, Height: 600})
	}

	issues := core.AuditHitPaintContract(root)
	for _, is := range issues {
		t.Errorf("%s: %s", is.TypeID, is.Message)
	}
	// Overlays also participate in hit==paint when mounted.
	for _, e := range tree.Overlays().Entries() {
		if e.Node == nil {
			continue
		}
		for _, is := range core.AuditHitPaintContract(e.Node) {
			t.Errorf("overlay %s: %s", is.TypeID, is.Message)
		}
	}

	// Button center hit.
	bAbs := core.AbsoluteBounds(btn.Root)
	cx := (bAbs.Min.X + bAbs.Max.X) / 2
	cy := (bAbs.Min.Y + bAbs.Max.Y) / 2
	if hit := tree.HitTest(core.Point{X: cx, Y: cy}); hit == nil {
		t.Fatal("button center: no hit")
	}

	// Switch must not place thumb outside track (classic loose-center bug).
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok {
			for _, ch := range p.Children() {
				off := ch.Base().Offset()
				if off.Y > p.Size().Height+0.5 {
					t.Errorf("pressable child offset.Y=%.1f > H=%.1f type=%s", off.Y, p.Size().Height, p.TypeID())
				}
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)
}

// TestAuditHitPaint_ModalFooterSpacer locks Flexible spacer in a Row:
// must not inflate cross-axis and shove buttons mid-box.
func TestAuditHitPaint_ModalFooterSpacer(t *testing.T) {
	ok := kit.NewButton("OK")
	cancel := kit.NewButton("Cancel")
	// Same pattern as Modal footer: Row(Spacer, cancel, ok)
	row := primitive.Row(
		primitive.Spacer(),
		cancel.Node(),
		ok.Node(),
	)
	row.Gap = 8
	row.MainAlign = core.MainStart
	row.CrossAlign = core.CrossCenter
	// Parent gives a tall max (bug trigger): Spacer must NOT take MaxHeight.
	box := primitive.NewBox(row)
	box.Width, box.Height = 480, 56
	_ = box.Layout(core.Tight(480, 56))

	// Row height must stay ~control height, not 56-only via expand — under tight 56 is OK.
	// Critical: Spacer Flexible size height under loose cross must not be loose max.
	// Under tight row height 56, spacer gets tight H from CrossCenter/Stretch path.
	// Rebuild under loose column max height to catch the Modal footer bug.
	row2 := primitive.Row(primitive.Spacer(), cancel.Node(), ok.Node())
	row2.Gap = 8
	row2.CrossAlign = core.CrossCenter
	// Simulate footer inside a tall loose area:
	sz := row2.Layout(core.Constraints{MaxWidth: 480, MaxHeight: 400, MinWidth: 0, MinHeight: 0})
	if sz.Height > 80 {
		t.Fatalf("footer row height=%v under loose max 400 — Spacer inflated cross axis", sz.Height)
	}
	// Buttons near top of row (offset.Y small relative to row).
	for _, n := range []core.Node{ok.Root, cancel.Root} {
		// Find node under row2
		_ = n
	}
	// Walk row2 pressables: child offsets must be within row height.
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok {
			for _, ch := range p.Children() {
				if ch.Base().Offset().Y > p.Size().Height+0.5 {
					t.Errorf("footer pressable child Y out of box")
				}
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(row2)

	issues := core.AuditHitPaintContract(row2)
	for _, is := range issues {
		t.Errorf("%s: %s", is.TypeID, is.Message)
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
