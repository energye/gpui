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

func TestSurrounding_TruncateCentered(t *testing.T) {
	// 5000 bytes, cursor in middle
	s := ""
	for i := 0; i < 5000; i++ {
		s += "a"
	}
	cur := 2500
	tr, newCur := TruncateSurrounding(s, cur)
	if len(tr) != 3999 {
		t.Fatalf("truncate len %d", len(tr))
	}
	if newCur != 1999 && newCur != 2000 {
		t.Fatalf("newCur %d", newCur)
	}
	if tr[newCur] != 'a' {
		t.Fatalf("cursor not centered")
	}
	// Short text no truncate
	s2 := "hello"
	tr2, cur2 := TruncateSurrounding(s2, 2)
	if tr2 != s2 || cur2 != 2 {
		t.Fatalf("short truncate")
	}
}
