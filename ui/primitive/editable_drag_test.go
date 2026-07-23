package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// layoutEdit mounts an editor at a known origin for pointer tests.
func layoutEdit(t *testing.T, value string, multiline bool) (*primitive.EditableText, *core.Tree, core.Point) {
	t.Helper()
	ed := primitive.NewEditableText()
	ed.Multiline = multiline
	ed.FontSize = 14
	ed.Width, ed.Height = 240, 80
	ed.SetValue(value)
	root := primitive.NewBox(ed)
	root.Width, root.Height = 240, 80
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 240, Height: 80})
	return ed, tree, core.AbsoluteOffset(ed)
}

func pointerAt(tree *core.Tree, typ core.PointerType, abs core.Point, lx, ly float64, shift bool) {
	tree.DispatchPointer(&core.PointerEvent{
		Type:  typ,
		X:     abs.X + lx,
		Y:     abs.Y + ly,
		Shift: shift,
	})
}

func TestEditableDragSelect(t *testing.T) {
	ed, tree, abs := layoutEdit(t, "abcdef", false)
	_, _, lineH := ed.LineMetricsForTest()
	midY := lineH / 2

	pointerAt(tree, core.PointerDown, abs, 1, midY, false)
	if !ed.IsDraggingSelection() {
		t.Fatal("down should start drag")
	}
	if tree.Capture() != ed {
		t.Fatalf("tree should capture editor, got %T", tree.Capture())
	}
	// Drag toward the right (later runes). Default advance ~0.5*fs without Face.
	pointerAt(tree, core.PointerMove, abs, 100, midY, false)
	pointerAt(tree, core.PointerUp, abs, 100, midY, false)
	if ed.IsDraggingSelection() {
		t.Fatal("up should end drag")
	}
	if ed.SelectedText() == "" {
		t.Fatalf("expected selection after drag, cursor=%d anchor=%d", ed.Cursor, ed.SelAnchor)
	}
}

func TestEditableShiftClickExtends(t *testing.T) {
	ed, tree, abs := layoutEdit(t, "hello", false)
	_, _, lineH := ed.LineMetricsForTest()
	midY := lineH / 2

	// Place caret at start, end drag, focus.
	pointerAt(tree, core.PointerDown, abs, 0, midY, false)
	pointerAt(tree, core.PointerUp, abs, 0, midY, false)
	tree.SetFocus(ed)
	ed.Cursor, ed.SelAnchor = 0, 0

	// Shift+click toward end.
	pointerAt(tree, core.PointerDown, abs, 100, midY, true)
	if ed.IsDraggingSelection() {
		t.Fatal("Shift+click must not start drag")
	}
	if ed.SelAnchor != 0 {
		t.Fatalf("Shift+click must keep anchor=0, got %d", ed.SelAnchor)
	}
	if ed.Cursor <= 0 || ed.SelectedText() == "" {
		t.Fatalf("Shift+click should extend: c=%d a=%d sel=%q", ed.Cursor, ed.SelAnchor, ed.SelectedText())
	}
}

func TestEditableDragSelectMultiline(t *testing.T) {
	ed, tree, abs := layoutEdit(t, "ab\ncd\nef", true)
	_, _, lineH := ed.LineMetricsForTest()

	// Down on first line, drag to third line.
	pointerAt(tree, core.PointerDown, abs, 2, lineH*0.3, false)
	if !ed.IsDraggingSelection() {
		t.Fatal("want drag")
	}
	pointerAt(tree, core.PointerMove, abs, 2, lineH*2.3, false)
	pointerAt(tree, core.PointerUp, abs, 2, lineH*2.3, false)
	if ed.IsDraggingSelection() {
		t.Fatal("drag should end")
	}
	// Cursor should have moved to a later line (rune index past first newline at 2).
	if ed.Cursor <= 2 && ed.SelAnchor <= 2 {
		t.Fatalf("multiline drag stuck on first line: c=%d a=%d", ed.Cursor, ed.SelAnchor)
	}
	if ed.SelectedText() == "" && ed.Cursor == ed.SelAnchor {
		t.Fatal("expected selection spanning lines")
	}
}

func TestEditablePlainClickCollapses(t *testing.T) {
	ed, tree, abs := layoutEdit(t, "xyz", false)
	ed.SelAnchor, ed.Cursor = 0, 3
	tree.SetFocus(ed)
	_, _, lineH := ed.LineMetricsForTest()
	pointerAt(tree, core.PointerDown, abs, 0, lineH/2, false)
	pointerAt(tree, core.PointerUp, abs, 0, lineH/2, false)
	if ed.SelectedText() != "" {
		t.Fatalf("plain click should collapse selection, got %q", ed.SelectedText())
	}
}

func TestEditableDragCancel(t *testing.T) {
	ed, tree, abs := layoutEdit(t, "abcd", false)
	_, _, lineH := ed.LineMetricsForTest()
	midY := lineH / 2
	pointerAt(tree, core.PointerDown, abs, 0, midY, false)
	pointerAt(tree, core.PointerMove, abs, 50, midY, false)
	pointerAt(tree, core.PointerCancel, abs, 50, midY, false)
	if ed.IsDraggingSelection() {
		t.Fatal("cancel should end drag")
	}
}

func TestEditablePointerModifiersMapped(t *testing.T) {
	// Shift on PointerEvent must reach HandlePointer (platform Dispatch maps it).
	ed, tree, abs := layoutEdit(t, "test", false)
	tree.SetFocus(ed)
	ed.Cursor, ed.SelAnchor = 0, 0
	_, _, lineH := ed.LineMetricsForTest()
	tree.DispatchPointer(&core.PointerEvent{
		Type: core.PointerDown, Shift: true,
		X: abs.X + 80, Y: abs.Y + lineH/2,
	})
	if ed.SelAnchor != 0 {
		t.Fatalf("anchor should stay 0, got %d", ed.SelAnchor)
	}
}
