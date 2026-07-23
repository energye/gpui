package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestEditableSelectionShiftArrowAndDelete(t *testing.T) {
	ed := primitive.NewEditableText()
	ed.SetValue("hello")
	// caret at end
	ed.Cursor = 5
	ed.SelAnchor = 5
	// Shift+Left three times → select "llo"
	for i := 0; i < 3; i++ {
		ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowLeft", Shift: true})
	}
	if ed.SelectedText() != "llo" {
		t.Fatalf("selected=%q want llo (cursor=%d anchor=%d)", ed.SelectedText(), ed.Cursor, ed.SelAnchor)
	}
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Backspace"})
	if ed.Value != "he" {
		t.Fatalf("value=%q want he", ed.Value)
	}
	if ed.SelectedText() != "" {
		t.Fatal("selection should clear")
	}
}

func TestEditableClipboardRoundTrip(t *testing.T) {
	clip := core.NewMemoryClipboard()
	ed := primitive.NewEditableText()
	ed.SetValue("abcdef")
	root := primitive.NewBox(ed)
	tree := core.NewTree(root)
	tree.SetClipboard(clip)
	tree.Layout(core.Size{Width: 200, Height: 40})
	tree.SetFocus(ed)

	// Select "cde" : anchor 2, cursor 5
	ed.SelAnchor = 2
	ed.Cursor = 5
	if ed.SelectedText() != "cde" {
		t.Fatalf("sel=%q", ed.SelectedText())
	}
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "c", Ctrl: true})
	got, ok := clip.ReadText()
	if !ok || got != "cde" {
		t.Fatalf("clipboard=%q ok=%v", got, ok)
	}
	// Move caret to end and paste
	ed.Cursor = 6
	ed.SelAnchor = 6
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "v", Ctrl: true})
	if ed.Value != "abcdefcde" {
		t.Fatalf("after paste value=%q", ed.Value)
	}
	// Select all + cut
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "a", Ctrl: true})
	if ed.SelectedText() != "abcdefcde" {
		t.Fatalf("select all=%q", ed.SelectedText())
	}
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "x", Ctrl: true})
	if ed.Value != "" {
		t.Fatalf("after cut value=%q", ed.Value)
	}
	got, _ = clip.ReadText()
	if got != "abcdefcde" {
		t.Fatalf("clip after cut=%q", got)
	}
}

func TestEditableInsertReplacesSelection(t *testing.T) {
	ed := primitive.NewEditableText()
	ed.SetValue("xyz")
	ed.SelAnchor = 0
	ed.Cursor = 3
	ed.HandleTextInput(&core.TextInputEvent{Text: "ok"})
	if ed.Value != "ok" {
		t.Fatalf("value=%q want ok", ed.Value)
	}
}
