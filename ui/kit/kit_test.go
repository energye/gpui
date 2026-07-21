package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

func TestButtonComposition(t *testing.T) {
	btn := kit.NewButton("OK")
	btn.SetType(kit.ButtonPrimary)
	if btn.Node() == nil {
		t.Fatal("nil node")
	}
	if btn.Root.TypeID() != primitive.TypePressable {
		t.Fatalf("root type %s", btn.Root.TypeID())
	}
	// Layout under loose constraints.
	sz := btn.Node().Layout(core.Loose(400, 100))
	if sz.Width < 40 || sz.Height < 24 {
		t.Fatalf("button size=%+v too small", sz)
	}
}

func TestButtonClickHeadless(t *testing.T) {
	clicks := 0
	btn := kit.NewButton("Save")
	btn.SetType(kit.ButtonPrimary)
	btn.SetOnClick(func() { clicks++ })

	// Place at origin in a fixed box for predictable hit coords.
	root := primitive.NewBox(btn.Node())
	root.Width, root.Height = 400, 200
	root.Padding = primitive.All(20)

	tree := core.NewTree(root)
	host := platform.NewHeadless(400, 200)
	defer host.Close()
	tree.Layout(core.Size{Width: 400, Height: 200})

	// Button top-left after padding.
	bx := 20 + btn.Root.Size().Width/2
	by := 20 + btn.Root.Size().Height/2
	host.InjectClick(bx, by)
	for _, ev := range host.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	btn.SyncState()
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1 (hit %.0f,%.0f size=%+v)", clicks, bx, by, btn.Root.Size())
	}
}

func TestButtonDisabledNoClick(t *testing.T) {
	clicks := 0
	btn := kit.NewButton("No")
	btn.SetOnClick(func() { clicks++ })
	btn.SetDisabled(true)

	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 10, Y: 10, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 10, Y: 10, Button: core.ButtonLeft})
	if clicks != 0 {
		t.Fatalf("clicks=%d want 0", clicks)
	}
}

func TestButtonKeyboardActivate(t *testing.T) {
	clicks := 0
	btn := kit.NewButton("Go")
	btn.SetOnClick(func() { clicks++ })
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	// Focus via pointer down
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 5, Y: 5, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 5, Y: 5, Button: core.ButtonLeft})
	clicks = 0 // reset after click
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if clicks != 1 {
		t.Fatalf("enter clicks=%d want 1", clicks)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if clicks != 2 {
		t.Fatalf("space clicks=%d want 2", clicks)
	}
}

func TestTextAndIcon(t *testing.T) {
	tx := kit.NewText("Hello")
	tx.SetSecondary(true)
	sz := tx.Node().Layout(core.Loose(200, 50))
	if sz.Width <= 0 {
		t.Fatal("text width 0")
	}
	ic := kit.NewIcon("check")
	isz := ic.Node().Layout(core.Loose(50, 50))
	if isz.Width != 16 {
		t.Fatalf("icon size=%+v", isz)
	}
}

func TestDefaultThemeTokens(t *testing.T) {
	th := kit.DefaultTheme()
	if th.Color(core.TokenColorPrimary).A == 0 {
		t.Fatal("missing primary")
	}
	if th.Size(core.TokenControlHeight) != 32 {
		t.Fatalf("controlHeight=%v", th.Size(core.TokenControlHeight))
	}
	if th.Skin == nil {
		t.Fatal("skin nil")
	}
	// skindefault Tokens clone
	tok := skindefault.Tokens()
	if tok.Color(core.TokenColorError).A == 0 {
		t.Fatal("error token")
	}
}

func TestFocusTabOrder(t *testing.T) {
	a := kit.NewButton("A")
	b := kit.NewButton("B")
	col := primitive.Column(a.Node(), b.Node())
	col.Gap = 8
	col.CrossAlign = core.CrossStart
	tree := core.NewTree(col)
	tree.Layout(core.Size{Width: 300, Height: 200})
	// Focus first via click at center of A
	ax := a.Root.Offset().X + a.Root.Size().Width/2
	ay := a.Root.Offset().Y + a.Root.Size().Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: ax, Y: ay, Button: core.ButtonLeft})
	if tree.Focus() != a.Root {
		t.Fatalf("focus=%v want A (hit %.0f,%.0f size=%+v off=%+v)", tree.Focus(), ax, ay, a.Root.Size(), a.Root.Offset())
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Tab"})
	if tree.Focus() != b.Root {
		t.Fatalf("after tab focus=%v want B", tree.Focus())
	}
}

func TestPrimitiveP1Layout(t *testing.T) {
	dec := primitive.NewDecorated(primitive.NewText("x"))
	dec.Padding = primitive.All(8)
	dec.Radius = 4
	dec.BorderWidth = 1
	sz := dec.Layout(core.Loose(200, 100))
	if sz.Width < 16 {
		t.Fatalf("decorated=%+v", sz)
	}
	div := primitive.NewDivider()
	dsz := div.Layout(core.TightWidth(100, 20))
	if dsz.Width != 100 {
		t.Fatalf("divider w=%v", dsz.Width)
	}
	slot := primitive.NewSlot("prefix", primitive.NewIcon("search"))
	_ = slot.Layout(core.Loose(50, 50))
	ht := primitive.NewHitTarget(primitive.NewBox())
	ht.Expand = 10
	// painter
	pn := primitive.NewPainterNode(func(pc *core.PaintContext, size core.Size) {}, nil)
	pn.Width, pn.Height = 40, 40
	_ = pn.Layout(core.Loose(100, 100))
	_ = ht
}
