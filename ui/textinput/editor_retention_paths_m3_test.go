package textinput

import (
	"testing"
	"unsafe"
)

// TestUndoDelta_NoDocRetention_Paths_M3 covers the edit paths that
// TestUndoDelta_NoDocRetention_M3 does not exercise (it only uses
// SetTextSimple): DeleteSurrounding, AddText-over-selection and the IME
// composing entry (BeginComposing → DeleteSelected). The first edit after
// SetTextSimple slices the original document array, so at HEAD (no Clone)
// its delta aliases the full doc backing and this test fails; with the M3
// pushDelta Clone fix all deltas are independent copies (I4).
func TestUndoDelta_NoDocRetention_Paths_M3(t *testing.T) {
	doc := m3Doc(t, 100000)
	docBase := uintptr(unsafe.Pointer(unsafe.StringData(doc)))
	docEnd := docBase + uintptr(len(doc))
	inDoc := func(s string) bool {
		if len(s) == 0 {
			return false
		}
		p := uintptr(unsafe.Pointer(unsafe.StringData(s)))
		return p >= docBase && p < docEnd
	}
	check := func(e *Editor, what string) {
		t.Helper()
		for gi, g := range e.history {
			for _, op := range g.ops {
				if len(op.deleted) > 0 && len(op.deleted) <= 64 && inDoc(op.deleted) {
					t.Fatalf("%s: group %d deleted aliases full doc array", what, gi)
				}
				if len(op.inserted) > 0 && len(op.inserted) <= 64 && inDoc(op.inserted) {
					t.Fatalf("%s: group %d inserted aliases full doc array", what, gi)
				}
			}
		}
	}

	e := New()
	e.SetTextSimple(doc)
	endCaret(e)
	if !e.DeleteSurrounding(-3, 3) {
		t.Fatal("DeleteSurrounding failed")
	}
	check(e, "delete-surrounding")

	e2 := New()
	e2.SetTextSimple(doc)
	e2.SetSelection(TextRange{Base: 0, Extent: 10})
	if !e2.AddText("0123456789ABCDEF") {
		t.Fatal("AddText over selection failed")
	}
	check(e2, "add-over-selection")

	e3 := New()
	e3.SetTextSimple(doc)
	e3.SetSelection(TextRange{Base: 20, Extent: 30})
	e3.BeginComposing()
	e3.UpdateComposingText("ni", TextRange{Base: 20, Extent: 22})
	e3.CommitComposing()
	check(e3, "composing")
}
