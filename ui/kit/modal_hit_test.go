package kit_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestModalOKButtonHitMatchesPaint(t *testing.T) {
	const W, H = 1024.0, 768.0
	modal := kit.NewModal("Confirm")
	modal.Viewport = core.Size{Width: W, Height: H}
	body := kit.NewText("body text")
	modal.SetContent(body.Node())

	// Mount portal in tree like gallery
	root := primitive.NewBox(modal.Node())
	root.Width, root.Height = W, H
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: W, Height: H})

	modal.SetOpen(true)
	// Force full layout again after open (overlay registered)
	tree.Layout(core.Size{Width: W, Height: H})

	if tree.Overlays().Len() < 1 {
		t.Fatal("modal not in overlays")
	}

	// Find OK pressable
	var okBtn *primitive.Pressable
	var dump func(core.Node, int)
	dump = func(n core.Node, d int) {
		if n == nil || d > 12 {
			return
		}
		abs := core.AbsoluteBounds(n)
		fmt.Printf("%*s%s size=%.0fx%.0f off=%v abs=%v parent=%T\n",
			d*2, "", n.TypeID(), n.Base().Size().Width, n.Base().Size().Height,
			n.Base().Offset(), abs, n.Parent())
		if p, ok := n.(*primitive.Pressable); ok {
			// Prefer rightmost / OK (primary often last)
			if okBtn == nil || abs.Min.X > core.AbsoluteBounds(okBtn).Min.X {
				okBtn = p
			}
		}
		for _, c := range n.Children() {
			dump(c, d+1)
		}
	}
	// Overlay content root
	for _, e := range tree.Overlays().Entries() {
		fmt.Println("--- overlay", e.ID, "nodeOff", e.Node.Base().Offset())
		dump(e.Node, 0)
	}

	if okBtn == nil {
		t.Fatal("OK button not found")
	}
	abs := core.AbsoluteBounds(okBtn)
	t.Logf("OK abs=%v size=%v", abs, okBtn.Size())
	// Child offset of pressable
	if kids := okBtn.Children(); len(kids) > 0 {
		t.Logf("OK child off=%v", kids[0].Base().Offset())
		if kids[0].Base().Offset().Y > 8 {
			t.Errorf("OK decorated offset.Y too large: %v", kids[0].Base().Offset())
		}
	}

	cx := (abs.Min.X + abs.Max.X) / 2
	cy := (abs.Min.Y + abs.Max.Y) / 2
	hit := tree.HitTest(core.Point{X: cx, Y: cy})
	t.Logf("hit(%.0f,%.0f)=%T", cx, cy, hit)
	if hit != okBtn {
		// dump what we hit
		if hit != nil {
			t.Logf("hit abs=%v type=%s", core.AbsoluteBounds(hit), hit.TypeID())
		}
		t.Fatalf("expected OK pressable at center of AbsoluteBounds, got %T", hit)
	}

	// Click should fire
	clicks := 0
	// re-open and find cancel similarly is harder — check hit on mask outside panel
	// Center of screen should hit something in modal (panel or button)
	// Top-left of viewport should hit mask
	hitTL := tree.HitTest(core.Point{X: 10, Y: 10})
	t.Logf("hitTL=%T", hitTL)

	// If AbsoluteBounds is wrong vs paint, hit at abs center fails (above).
	// Also verify panel is roughly centered: abs of OK should be near bottom of center panel.
	// Panel must be content-sized (~title+body+footer), not nearly full viewport.
	// Old Flexible/Spacer bug: footer Row height ≈ vh*0.9, buttons CrossCenter mid-panel.
	var panelH float64
	for _, e := range tree.Overlays().Entries() {
		var walk func(core.Node)
		walk = func(n core.Node) {
			if n == nil {
				return
			}
			if n.TypeID() == "primitive.Decorated" && n.Base().Size().Width >= 400 {
				if n.Base().Size().Height > panelH {
					panelH = n.Base().Size().Height
				}
			}
			for _, c := range n.Children() {
				walk(c)
			}
		}
		walk(e.Node)
	}
	t.Logf("panelH=%.1f OK.Y=%.1f", panelH, abs.Min.Y)
	if panelH > 280 {
		t.Fatalf("modal panel height=%.1f too tall (Spacer/Flexible inflated footer)", panelH)
	}
	// OK button should sit in lower portion of centered panel, not mid-window from fat footer.
	// With fixed footer ~32px + padding, panel ~150-200; OK near bottom of panel.
	if abs.Min.Y < 200 || abs.Min.Y > 550 {
		t.Fatalf("OK button Y=%.1f out of expected centered-dialog range", abs.Min.Y)
	}
	_ = clicks
}
