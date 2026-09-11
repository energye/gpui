package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
)

type recAdapter struct {
	enables  []platform.FieldSnapshot
	disables int
	rects    []platform.Rect
}

func (a *recAdapter) Enable(f platform.FieldSnapshot)    { a.enables = append(a.enables, f) }
func (a *recAdapter) Disable()                           { a.disables++ }
func (a *recAdapter) CaretMoved(r platform.Rect)         { a.rects = append(a.rects, r) }
func (a *recAdapter) SetPurpose(ct platform.ContentType) {}
func (a *recAdapter) PushSurrounding(text string, cursor int) {}

type fakeField struct{}

func (fakeField) IMERect() platform.Rect { return platform.Rect{X: 5, Y: 6, W: 100, H: 20} }
func (fakeField) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: platform.PurposeNormal}
}

func newTestSession() (*ImeSession, *Editor, *recAdapter) {
	ad := &recAdapter{}
	s := NewImeSession(ad)
	ed := New()
	return s, ed, ad
}

func setText(t *testing.T, e *Editor, s string) {
	t.Helper()
	n := utf16Len(s)
	e.SetText(s, TextRange{Base: n, Extent: n}, TextRange{}, 0)
}

func TestSessionAttachOpenDisableClose(t *testing.T) {
	s, ed, ad := newTestSession()
	s.AttachEditor(ed, fakeField{})
	if s.State() != StateActive || len(ad.enables) != 1 {
		t.Fatalf("attach failed state=%v enables=%d", s.State(), len(ad.enables))
	}
	s.DetachEditor()
	if s.State() != StateIdle || ad.disables != 1 {
		t.Fatalf("detach failed")
	}
}

func TestSessionComposingLeg(t *testing.T) {
	s, ed, _ := newTestSession()
	s.AttachEditor(ed, fakeField{})
	s.PreeditChanged(input.PreeditEvent{Text: "ni", Cursor: 2})
	if s.State() != StateComposing || ed.GetText() != "ni" {
		t.Fatalf("compose state=%v text=%q", s.State(), ed.GetText())
	}
	s.Committed("你")
	if s.State() != StateActive || ed.GetText() != "你" {
		t.Fatalf("commit failed text=%q", ed.GetText())
	}
	s.PreeditChanged(input.PreeditEvent{Text: "x"})
	s.PreeditChanged(input.PreeditEvent{Text: ""})
	if s.State() != StateActive || ed.ComposeActive() {
		t.Fatalf("clear failed")
	}
}

func TestSessionCaretMovedDedup(t *testing.T) {
	s, ed, ad := newTestSession()
	s.AttachEditor(ed, fakeField{})
	// R4: non-composing is preheat only, no push
	n := len(ad.rects)
	r := platform.Rect{X: 1, Y: 2, W: 3, H: 4}
	s.CaretMoved(r)
	if len(ad.rects)-n != 0 {
		t.Fatalf("non-composing should be preheat, not push")
	}
	// composing should push and dedup
	s.PreeditChanged(input.PreeditEvent{Text: "a", Cursor: 1})
	n2 := len(ad.rects)
	r2 := platform.Rect{X: 9, Y: 9, W: 3, H: 4}
	s.CaretMoved(r2)
	s.CaretMoved(r2)
	if len(ad.rects)-n2 != 1 {
		t.Fatalf("composing dedup failed")
	}
}
