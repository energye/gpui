package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// Tabs body gives children a huge loose MaxHeight. Pressable must NOT center its
// chrome using that max — hit stays top, paint used to slide mid/bottom.
func TestPressableLooseMaxHeightTopLeft(t *testing.T) {
	btn := kit.NewButton("Primary")
	// Simulate panel Column inside tall tab body: loose max height ~700.
	c := core.Constraints{MaxWidth: 800, MaxHeight: 700}
	sz := btn.Node().Layout(c)
	if sz.Height > 80 {
		t.Fatalf("button height=%v inflated by loose MaxHeight", sz.Height)
	}
	// Decorated child of Pressable must sit at padding top (not mid of 700).
	p := btn.Root
	kids := p.Children()
	if len(kids) == 0 {
		t.Fatal("no child")
	}
	off := kids[0].Base().Offset()
	if off.Y > 8 {
		t.Fatalf("child offset Y=%.1f want ~0..pad (was centering with MaxHeight=700)", off.Y)
	}
	abs := core.AbsoluteBounds(p)
	// After layout as root, abs top should be 0
	if abs.Min.Y != 0 {
		t.Fatalf("pressable abs.Min.Y=%.1f", abs.Min.Y)
	}
}

// Full gallery-like tabs: button paint bounds must match AbsoluteBounds (top of body).
func TestTabsBodyButtonPaintTopMatchesHit(t *testing.T) {
	btn := kit.NewButton("Primary")
	panel := primitive.Column(btn.Node())
	panel.MainAlign = core.MainStart
	panel.CrossAlign = core.CrossStart
	panel.Padding = primitive.All(12)

	tabs := kit.NewTabs(kit.MenuItem{Key: "b", Label: "Button"})
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetContent("b", panel)
	tabs.SetActive("b")

	host := primitive.NewFlexible(1, tabs.Node())
	col := primitive.Column(primitive.NewText("title"), host)
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	root := primitive.NewBox(col)
	root.Width, root.Height = 1024, 768

	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 1024, Height: 768})

	abs := core.AbsoluteBounds(btn.Root)
	t.Logf("button abs=%v size=%v", abs, btn.Root.Size())
	if abs.Min.Y > 120 {
		t.Fatalf("button too low abs.Min.Y=%.1f (tabs body must top-align)", abs.Min.Y)
	}
	// Child of Pressable (Decorated) offset relative to Pressable must be small.
	kids := btn.Root.Children()
	if len(kids) > 0 {
		off := kids[0].Base().Offset()
		if off.Y > 8 {
			t.Fatalf("decorated offset inside Pressable Y=%.1f — still centering on body MaxHeight", off.Y)
		}
	}
	// Hit at absolute center of button bounds must hit the pressable.
	cx := (abs.Min.X + abs.Max.X) / 2
	cy := (abs.Min.Y + abs.Max.Y) / 2
	hit := tree.HitTest(core.Point{X: cx, Y: cy})
	if hit != btn.Root {
		t.Fatalf("hit=%T want button pressable", hit)
	}

	// Decorated chrome AbsoluteBounds must coincide with Pressable (paint == hit).
	if len(kids) > 0 {
		dAbs := core.AbsoluteBounds(kids[0])
		if dAbs.Min.Y-abs.Min.Y > 8 {
			t.Fatalf("decorated abs.Min.Y=%.1f pressable=%.1f — paint chrome below hit box", dAbs.Min.Y, abs.Min.Y)
		}
	}

	// Mid-window should NOT hit the button (was the false visual position).
	midY := 400.0
	if abs.Min.Y < 100 {
		hitMid := tree.HitTest(core.Point{X: cx, Y: midY})
		if hitMid == btn.Root {
			t.Fatalf("button still hittable at mid-window y=400 — layout wrong")
		}
	}
}
