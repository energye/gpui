package app_test

import (
	"testing"

	"github.com/energye/gpui/ui/app"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

// F2: app.Attach bridges CursorHost → Tree.SetOnCursor.
func TestAppAttachBridgesCursor(t *testing.T) {
	host := platform.NewHeadless(200, 100)
	if !host.Caps().Has(platform.CapCursor) {
		t.Fatal("Headless must advertise CapCursor")
	}
	ed := primitive.NewEditableText()
	ed.Width, ed.Height = 100, 32
	ed.SetValue("hi")
	// EditableText sets CursorText on itself.
	lab := primitive.NewText("go")
	btn := primitive.NewPressable(lab)
	btn.SetCursor(core.CursorPointer)
	btn.Padding = primitive.Symmetric(12, 8)

	col := primitive.Column(ed, btn)
	col.Gap = 8
	root := primitive.NewBox(col)
	root.Width, root.Height = 200, 100
	tree := core.NewTree(root)
	a := app.New(app.Options{DisableRenderThread: true})
	defer a.Close()
	a.Attach(host, tree, nil)
	tree.Layout(core.Size{Width: 200, Height: 100})

	// Hover editor → text cursor
	eAbs := core.AbsoluteBounds(ed)
	tree.DispatchPointer(&core.PointerEvent{
		Type: core.PointerMove,
		X:    (eAbs.Min.X + eAbs.Max.X) / 2,
		Y:    (eAbs.Min.Y + eAbs.Max.Y) / 2,
	})
	if k, ok := host.LastCursor(); !ok || k != platform.CursorText {
		t.Fatalf("hover edit: cursor=%v ok=%v want Text", k, ok)
	}

	// Hover button → pointer
	bAbs := core.AbsoluteBounds(btn)
	tree.DispatchPointer(&core.PointerEvent{
		Type: core.PointerMove,
		X:    (bAbs.Min.X + bAbs.Max.X) / 2,
		Y:    (bAbs.Min.Y + bAbs.Max.Y) / 2,
	})
	if k, ok := host.LastCursor(); !ok || k != platform.CursorPointer {
		t.Fatalf("hover btn: cursor=%v ok=%v want Pointer", k, ok)
	}
}
