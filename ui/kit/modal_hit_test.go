package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestModalOKButtonHitMatchesPaint(t *testing.T) {
	const W, H = 1024.0, 768.0
	modal := kit.NewModal("Confirm")
	modal.SetCentered(true)
	modal.Viewport = core.Size{Width: W, Height: H}
	body := kit.NewText("body text")
	modal.SetContent(body.Node())

	root := primitive.NewBox(modal.Node())
	root.Width, root.Height = W, H
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: W, Height: H})

	modal.SetOpen(true)
	tree.Layout(core.Size{Width: W, Height: H})

	if tree.Overlays().Len() < 1 {
		t.Fatal("modal not in overlays")
	}
	okBtn := modal.OkButton()
	if okBtn == nil || okBtn.Root == nil {
		t.Fatal("OK button not found")
	}
	abs := core.AbsoluteBounds(okBtn.Root)
	t.Logf("OK abs=%v size=%v", abs, okBtn.Root.Size())
	if kids := okBtn.Root.Children(); len(kids) > 0 {
		t.Logf("OK child off=%v", kids[0].Base().Offset())
		if kids[0].Base().Offset().Y > 8 {
			t.Errorf("OK decorated offset.Y too large: %v", kids[0].Base().Offset())
		}
	}

	cx := (abs.Min.X + abs.Max.X) / 2
	cy := (abs.Min.Y + abs.Max.Y) / 2
	hit := tree.HitTest(core.Point{X: cx, Y: cy})
	t.Logf("hit(%.0f,%.0f)=%T", cx, cy, hit)
	if hit != okBtn.Root {
		if hit != nil {
			t.Logf("hit abs=%v type=%s", core.AbsoluteBounds(hit), hit.TypeID())
		}
		t.Fatalf("expected OK pressable at center of AbsoluteBounds, got %T", hit)
	}

	hitTL := tree.HitTest(core.Point{X: 10, Y: 10})
	t.Logf("hitTL=%T", hitTL)
	if _, ok := hitTL.(*primitive.Mask); !ok {
		t.Fatalf("top-left should hit mask, got %T", hitTL)
	}

	panel := modal.Panel()
	if panel == nil {
		t.Fatal("panel nil")
	}
	panelH := panel.Size().Height
	t.Logf("panelH=%.1f OK.Y=%.1f", panelH, abs.Min.Y)
	if panelH > 280 {
		t.Fatalf("modal panel height=%.1f too tall (Spacer/Flexible inflated footer)", panelH)
	}
	// Centered dialog: OK sits in lower half of viewport around mid.
	if abs.Min.Y < 200 || abs.Min.Y > 550 {
		t.Fatalf("OK button Y=%.1f out of expected centered-dialog range", abs.Min.Y)
	}
}
