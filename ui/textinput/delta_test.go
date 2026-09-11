package textinput

import "testing"

func TestDelta_NonTextUpdate(t *testing.T) {
	e := New()
	mustSetText(t, e, "hello")
	oldText := e.GetText()
	oldSel := e.selection
	oldComp := e.composingRange
	e.SetSelection(TextRange{Base: 2, Extent: 2})
	d := e.ToDelta(oldText, oldSel, oldComp)
	if !d.IsNonTextUpdate() {
		t.Fatalf("should be non-text update")
	}
	e2 := New()
	mustSetText(t, e2, "hello")
	e2.ApplyDelta(d)
	if e2.GetText() != "hello" || e2.selection.Extent != 2 {
		t.Fatalf("ApplyDelta non-text failed")
	}
}

func TestDelta_Insert(t *testing.T) {
	e := New()
	mustSetText(t, e, "ab")
	oldText := "ab"
	oldSel := TextRange{Base: 2, Extent: 2}
	oldComp := TextRange{}
	e.SetSelection(TextRange{Base: 2, Extent: 2})
	e.AddText("c")
	d := e.ToDelta(oldText, oldSel, oldComp)
	if d.DeltaText != "c" || d.DeltaStart != 2 {
		t.Fatalf("delta insert %v", d)
	}
	e2 := New()
	mustSetText(t, e2, "ab")
	e2.ApplyDelta(d)
	if e2.GetText() != "abc" {
		t.Fatalf("ApplyDelta insert %q", e2.GetText())
	}
}
