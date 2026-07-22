package kit_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	"github.com/energye/gpui/ui/visualtest"
)

// Diagnostic: where is content laid out and painted (CPU path)?
func TestYOrientationDiag(t *testing.T) {
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

	var firstBtn *primitive.Pressable
	var walk func(core.Node, int)
	walk = func(n core.Node, d int) {
		if n == nil || d > 8 {
			return
		}
		abs := core.AbsoluteBounds(n)
		if abs.Height() > 0 && abs.Width() > 0 && d < 6 {
			t.Logf("%*s%s size=%.0fx%.0f absY=%.1f..%.1f hit=%v", d*2, "", n.TypeID(),
				n.Base().Size().Width, n.Base().Size().Height, abs.Min.Y, abs.Max.Y, n.Base().Hit)
		}
		if p, ok := n.(*primitive.Pressable); ok && firstBtn == nil && abs.Min.X > 160 {
			firstBtn = p
		}
		for _, c := range n.Children() {
			walk(c, d+1)
		}
	}
	walk(root, 0)

	if firstBtn == nil {
		t.Fatal("no content button")
	}
	abs := core.AbsoluteBounds(firstBtn)
	t.Logf("button abs=%v", abs)
	if abs.Min.Y > 200 {
		t.Errorf("button too low: abs.Min.Y=%.1f (layout should top-align)", abs.Min.Y)
	}

	// Hit at button center
	cx, cy := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	hit := tree.HitTest(core.Point{X: cx, Y: cy})
	t.Logf("hit(%.0f,%.0f)=%T", cx, cy, hit)
	if hit != firstBtn {
		t.Errorf("hit mismatch")
	}

	// CPU paint: find ink
	img := visualtest.CaptureTree(1024, 768, root, kit.DefaultTheme())
	b := img.Bounds()
	topInk := -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a > 0 && (r>>8 < 250 || g>>8 < 250 || bl>>8 < 250) {
				topInk = y
				break
			}
		}
		if topInk >= 0 {
			break
		}
	}
	t.Logf("top ink row y=%d", topInk)
	if topInk > 80 {
		t.Errorf("paint ink too low: y=%d (expect near top)", topInk)
	}

	// Sample content x=300
	for _, y := range []int{30, 60, 100, 400, 700} {
		r, g, bl, _ := img.At(300, y).RGBA()
		fmt.Printf("pixel(300,%d)=%d,%d,%d\n", y, r>>8, g>>8, bl>>8)
	}
}
