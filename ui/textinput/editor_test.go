package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/input"
)

func mustSetText(t *testing.T, e *Editor, s string) {
	t.Helper()
	n := utf16Len(s)
	if !e.SetText(s, TextRange{Base: n, Extent: n}, TextRange{}, 0) {
		t.Fatalf("SetText failed")
	}
}

func TestR1_AddTextReplacesComposing(t *testing.T) {
	e := New()
	mustSetText(t, e, "ab")
	e.SetSelection(TextRange{Base: 2, Extent: 2})
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "nihao"})
	e.ApplyIME(input.IMEEvent{Kind: input.IMECommit, Text: "你好"})
	if e.GetText() != "ab你好" || e.ComposeActive() {
		t.Fatalf("commit %q active=%v", e.GetText(), e.ComposeActive())
	}
}

func TestR1_DeleteSurrounding(t *testing.T) {
	e := New()
	mustSetText(t, e, "abcdef")
	e.SetSelection(TextRange{Base: 3, Extent: 3})
	e.DeleteSurrounding(-1, 1)
	if e.GetText() != "abdef" {
		t.Fatalf("DeleteSurrounding %q", e.GetText())
	}
}
