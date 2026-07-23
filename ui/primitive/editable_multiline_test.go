package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestMultilineCaretGeomLine2(t *testing.T) {
	ed := primitive.NewEditableText()
	ed.Multiline = true
	ed.FontSize = 14
	ed.SetValue("ab\ncd")
	// Cursor after 'c' on line 1 (index 3: a b \n c)
	ed.Cursor = 3
	ed.SelAnchor = 3
	x, y := ed.CaretLocalPos()
	if y <= 0 {
		t.Fatalf("caret Y on line2 must be >0, got y=%v x=%v", y, x)
	}
	// Line height ~ fs*1.3 ≈ 18; y should be around that.
	if y < 10 || y > 40 {
		t.Fatalf("caret Y=%v out of expected second-line band", y)
	}
}

func TestMultilineArrowDownPreservesColumn(t *testing.T) {
	ed := primitive.NewEditableText()
	ed.Multiline = true
	ed.FontSize = 14
	ed.SetValue("hello\nworld")
	// caret after "he" (index 2)
	ed.Cursor = 2
	ed.SelAnchor = 2
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowDown"})
	// Expect roughly same column on line 2 → after "wo" = index 6+2=8?
	// line2 starts at 6 (h e l l o \n = 6), +2 = 8
	if ed.Cursor < 6 {
		t.Fatalf("cursor=%d still on line1", ed.Cursor)
	}
	// column offset from line start
	col := ed.Cursor - 6
	if col < 1 || col > 3 {
		t.Fatalf("cursor=%d col=%d want ~2", ed.Cursor, col)
	}
}

func TestMultilineClickPlacesCaret(t *testing.T) {
	ed := primitive.NewEditableText()
	ed.Multiline = true
	ed.FontSize = 14
	ed.Width, ed.Height = 200, 80
	ed.SetValue("aa\nbb\ncc")
	root := primitive.NewBox(ed)
	root.Width, root.Height = 200, 80
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 200, Height: 80})
	// Click on second line (~ y = lineH)
	_, _, lineH := ed.LineMetricsForTest()
	abs := core.AbsoluteOffset(ed)
	tree.DispatchPointer(&core.PointerEvent{
		Type: core.PointerDown,
		X:    abs.X + 2,
		Y:    abs.Y + lineH + 2,
	})
	// Should be on line 2 (index >= 3)
	if ed.Cursor < 3 {
		t.Fatalf("click line2 cursor=%d", ed.Cursor)
	}
}

func TestMultilineHomeEndLineLocal(t *testing.T) {
	ed := primitive.NewEditableText()
	ed.Multiline = true
	ed.SetValue("one\ntwo\nthree")
	// cursor in middle of "two" (index 5: o n e \n t)
	ed.Cursor = 5
	ed.SelAnchor = 5
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Home"})
	if ed.Cursor != 4 { // start of "two"
		t.Fatalf("Home line start cursor=%d want 4", ed.Cursor)
	}
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "End"})
	if ed.Cursor != 7 { // end of "two" before \n
		t.Fatalf("End line end cursor=%d want 7", ed.Cursor)
	}
}
