package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func layoutFAB(t *testing.T, fb *kit.FloatButton) *primitive.Decorated {
	t.Helper()
	// Loose host so FAB fixed size is not forced to viewport.
	host := primitive.Row(fb.Node())
	tree := core.NewTree(host)
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	dec, ok := fb.Button().ChromeNode().(*primitive.Decorated)
	if !ok || dec == nil {
		t.Fatal("chrome")
	}
	return dec
}

func TestFloatButton_Defaults(t *testing.T) {
	fb := kit.NewFloatButton("+")
	if fb.Shape != kit.FloatButtonCircle {
		t.Fatal(fb.Shape)
	}
	if fb.Type != kit.ButtonPrimary {
		t.Fatal(fb.Type)
	}
	dec := layoutFAB(t, fb)
	sz := dec.Size()
	if sz.Width < kit.DefaultFloatButtonSize-0.5 || sz.Width > kit.DefaultFloatButtonSize+0.5 {
		t.Fatalf("width %v want %v", sz.Width, kit.DefaultFloatButtonSize)
	}
	wantR := kit.DefaultFloatButtonSize / 2
	if dec.Radius < wantR-0.5 || dec.Radius > wantR+0.5 {
		t.Fatalf("circle radius %v want %v", dec.Radius, wantR)
	}
}

func TestFloatButton_SetShape_Square(t *testing.T) {
	fb := kit.NewFloatButton("+")
	fb.SetShape(kit.FloatButtonSquare)
	dec := layoutFAB(t, fb)
	if dec.Radius != kit.DefaultFloatButtonSquareRadius {
		t.Fatalf("square radius %v want %v", dec.Radius, kit.DefaultFloatButtonSquareRadius)
	}
}

func TestFloatButton_SetSize_Icon(t *testing.T) {
	fb := kit.NewFloatButton("")
	fb.SetSize(48)
	fb.SetIcon("plus")
	dec := layoutFAB(t, fb)
	if dec.Size().Width < 47.5 || dec.Size().Width > 48.5 {
		t.Fatalf("size %v", dec.Size())
	}
	if fb.Button().IconName != "plus" {
		t.Fatal(fb.Button().IconName)
	}
	if dec.Radius < 23.5 || dec.Radius > 24.5 {
		t.Fatalf("circle r %v", dec.Radius)
	}
}

func TestFloatButton_SetDescription(t *testing.T) {
	fb := kit.NewFloatButton("")
	fb.SetIcon("plus")
	fb.SetDescription("Help")
	dec := layoutFAB(t, fb)
	if fb.Description != "Help" {
		t.Fatalf("description %q", fb.Description)
	}
	// Description body is a centered column inside chrome (not Button.Label).
	if len(dec.Children()) == 0 {
		t.Fatal("no body")
	}
	body := dec.Children()[0]
	if body.Base().Size().Width < dec.Size().Width-0.5 {
		t.Fatalf("body not stretched for centering: body=%v dec=%v", body.Base().Size(), dec.Size())
	}
}
