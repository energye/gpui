package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: setter order must not drop chrome (Icon then Type, Background then Size).
func TestButton_Lifecycle_SetterOrderPreservesChrome(t *testing.T) {
	b := kit.NewButton("Go")
	b.SetIcon("search")
	b.SetType(kit.ButtonPrimary)
	dec, ok := b.ChromeNode().(*primitive.Decorated)
	if !ok || dec == nil {
		t.Fatal("decorated nil")
	}
	// Primary solid → non-zero fill from tokens
	if dec.Background.A < 0.5 {
		t.Fatalf("after SetIcon then SetType: bg alpha=%v want primary solid", dec.Background.A)
	}
	if b.IconName != "search" {
		t.Fatal("icon lost")
	}

	b2 := kit.NewButton("X")
	want := render.Hex("#FF00AA")
	b2.SetBackground(want)
	b2.SetSize(kit.ButtonLarge)
	dec2 := b2.ChromeNode().(*primitive.Decorated)
	if dec2.Background != want {
		t.Fatalf("SetBackground then SetSize lost bg: got %+v want %+v", dec2.Background, want)
	}
}

// #6: default skin registers kit.Button; Decorated.SkinType tags product chrome.
func TestButton_SkinType_UsesKitButtonPainter(t *testing.T) {
	b := kit.NewButton("Skin")
	b.SetType(kit.ButtonPrimary)
	dec := b.ChromeNode().(*primitive.Decorated)
	if dec.SkinType != kit.TypeButton {
		t.Fatalf("SkinType=%q want %q", dec.SkinType, kit.TypeButton)
	}

	th := skindefault.Theme()
	if th.Skin == nil || th.Painter(kit.TypeButton) == nil {
		t.Fatal("default skin missing kit.Button painter")
	}

	// Override only Button chrome — prove Skin is pluggable.
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeButton, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			// Magenta fill for test visibility.
			d.Background = render.Hex("#FF00FF")
			primitive.PaintDecorated(pc, d)
		}
	})
	b.Theme = th
	// Mount and paint once.
	tree := core.NewTree(b.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 200, Height: 80})
	pc := &core.PaintContext{Theme: th, Scale: 1}
	// DC may be nil — PaintDecorated no-ops fills without DC, but painter still runs if we call node paint path.
	// Force chrome paint path through Decorated.Paint with a software context.
	dc := render.NewContext(200, 80)
	pc.DC = dc
	b.ChromeNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.Button painter was not invoked")
	}
}

func TestButton_EnsureBuilt_ChromeChangeWithoutPriorNode(t *testing.T) {
	// Zero-value-ish: only set fields, never call NewButton rebuild path...
	// NewButton always builds; instead verify chromeChange after Node() is stable.
	b := kit.NewButton("A")
	b.SetType(kit.ButtonDefault)
	b.SetType(kit.ButtonPrimary)
	dec := b.ChromeNode().(*primitive.Decorated)
	if dec.Background.A < 0.5 {
		t.Fatalf("primary chrome not applied bg=%+v", dec.Background)
	}
	if dec.SkinType != kit.TypeButton {
		t.Fatalf("SkinType=%q", dec.SkinType)
	}
}
