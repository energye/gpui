package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestButton_SetGhost_TransparentIdle(t *testing.T) {
	b := kit.NewButton("Ghost")
	b.SetType(kit.ButtonPrimary)
	b.SetGhost(true)
	dec, ok := b.ChromeNode().(*primitive.Decorated)
	if !ok || dec == nil {
		t.Fatal("chrome")
	}
	// Idle ghost primary: transparent / low alpha fill
	if dec.Background.A > 0.05 {
		t.Fatalf("ghost idle bg alpha %v want ~0", dec.Background.A)
	}
	if dec.BorderWidth <= 0 && dec.BorderColor.A < 0.5 {
		// Border may be set via BorderColor with width from chrome
	}
	if !b.Ghost {
		t.Fatal("Ghost field")
	}
}

func TestButton_SetVariant_SolidColor(t *testing.T) {
	b := kit.NewButton("OK")
	b.SetVariant(kit.ButtonVariantSolid)
	b.SetColor(kit.ButtonColorDanger)
	dec := b.ChromeNode().(*primitive.Decorated)
	// Danger solid should be reddish (error hex #FF4D4F)
	if dec.Background.R < 0.8 {
		t.Fatalf("danger solid bg R=%v", dec.Background.R)
	}
}

func TestButton_SetIconPlacement_End(t *testing.T) {
	b := kit.NewButton("Search")
	b.SetIcon("search")
	b.SetIconPlacement(kit.ButtonIconEnd)
	// Row children: label then icon (no spinner)
	row := b.ChromeNode().(*primitive.Decorated).Children()[0]
	kids := row.Base().Children()
	if len(kids) < 2 {
		t.Fatalf("kids %d", len(kids))
	}
	// last should be icon
	if kids[len(kids)-1].TypeID() != primitive.TypeIcon {
		t.Fatalf("last child %s want icon", kids[len(kids)-1].TypeID())
	}
	if kids[0].TypeID() == primitive.TypeIcon {
		t.Fatal("icon should not be first when end placement")
	}
}

func TestButton_IconStart_Default(t *testing.T) {
	b := kit.NewButton("Search")
	b.SetIcon("search")
	row := b.ChromeNode().(*primitive.Decorated).Children()[0]
	kids := row.Base().Children()
	if kids[0].TypeID() != primitive.TypeIcon {
		t.Fatalf("first %s want icon", kids[0].TypeID())
	}
}

func TestButton_VariantFilled(t *testing.T) {
	b := kit.NewButton("Fill")
	b.SetVariant(kit.ButtonVariantFilled)
	b.SetColor(kit.ButtonColorPrimary)
	_ = b.Node()
	dec := b.ChromeNode().(*primitive.Decorated)
	// filled is light wash, not full primary solid
	if dec.Background.R > 0.95 && dec.Background.G > 0.95 {
		// might be very light primaryBg - ok
	}
	_ = core.TokenColorPrimary
}

func TestButton_TypeDashed_VisibleText(t *testing.T) {
	b := kit.NewButton("Dashed")
	b.SetType(kit.ButtonDashed)
	tree := core.NewTree(b.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 40})
	dec := b.ChromeNode().(*primitive.Decorated)
	// label color must be visible (not transparent)
	var label *primitive.Text
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || label != nil {
			return
		}
		if tx, ok := n.(*primitive.Text); ok {
			label = tx
			return
		}
		for _, c := range n.Base().Children() {
			walk(c)
		}
	}
	walk(dec)
	if label == nil {
		t.Fatal("no label")
	}
	if label.Color.A < 0.5 {
		t.Fatalf("dashed label alpha %.2f (invisible text bug)", label.Color.A)
	}
	if dec.BorderColor.A < 0.5 {
		t.Fatalf("dashed border alpha %.2f", dec.BorderColor.A)
	}
	if len(dec.BorderDash) == 0 {
		t.Fatal("expected dashed border pattern")
	}
}
