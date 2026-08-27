package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
)

// recAdapter records every outbound call (state-machine side-effect asserts).
type recAdapter struct {
	enables  []platform.FieldSnapshot
	disables int
	rects    []platform.Rect
	purposes []platform.ContentPurpose
	surrs    []string
}

func (a *recAdapter) Enable(f platform.FieldSnapshot)    { a.enables = append(a.enables, f) }
func (a *recAdapter) Disable()                           { a.disables++ }
func (a *recAdapter) CaretMoved(r platform.Rect)         { a.rects = append(a.rects, r) }
func (a *recAdapter) SetPurpose(ct platform.ContentType) { a.purposes = append(a.purposes, ct.Purpose) }
func (a *recAdapter) PushSurrounding(text string, cursor int) {
	a.surrs = append(a.surrs, text)
}

// fakeField is a minimal FieldSnapshotProvider.
type fakeField struct{}

func (fakeField) IMERect() platform.Rect { return platform.Rect{X: 5, Y: 6, W: 100, H: 20} }
func (fakeField) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: platform.PurposeEmail}
}

// anchorField's rect varies with the editor's composition width — mirroring
// a real target whose candidate anchor follows preedit growth.
type anchorField struct{ ed *Editor }

func (f anchorField) IMERect() platform.Rect {
	return platform.Rect{X: 5 + float64(len(f.ed.CompositionText())), Y: 6, W: 100, H: 20}
}
func (anchorField) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: platform.PurposeEmail}
}

func newTestSession() (*ImeSession, *Editor, *recAdapter) {
	ad := &recAdapter{}
	s := NewImeSession(ad)
	ed := New()
	return s, ed, ad
}

func TestSessionAttachOpenDisableClose(t *testing.T) {
	s, ed, ad := newTestSession()
	if s.State() != StateIdle {
		t.Fatalf("initial state = %v", s.State())
	}
	s.AttachEditor(ed, fakeField{})
	if s.State() != StateActive || len(ad.enables) != 1 || ad.disables != 0 {
		t.Fatalf("after attach: state=%v enables=%d disables=%d", s.State(), len(ad.enables), ad.disables)
	}
	if len(ad.purposes) != 1 || ad.purposes[0] != platform.PurposeEmail {
		t.Fatalf("purposes = %v", ad.purposes)
	}
	if len(ad.enables) != 1 || ad.enables[0].Rect != (platform.Rect{X: 5, Y: 6, W: 100, H: 20}) {
		t.Fatalf("enable snapshot = %v", ad.enables)
	}
	s.DetachEditor()
	if s.State() != StateIdle || ad.disables != 1 {
		t.Fatalf("after detach: state=%v disables=%d", s.State(), ad.disables)
	}
}

func TestSessionComposingLeg(t *testing.T) {
	s, ed, _ := newTestSession()
	s.AttachEditor(ed, fakeField{})

	// Preedit starts composing; text now includes preedit (pure four-tuple).
	s.PreeditChanged(input.PreeditEvent{Text: "ni", Cursor: 2})
	if s.State() != StateComposing || ed.Text() != "ni" || !ed.ComposeActive() {
		t.Fatalf("compose: state=%v buf=%q", s.State(), ed.Text())
	}

	// Commit lands atomically and returns to Active.
	s.Committed("你")
	if s.State() != StateActive || ed.Text() != "你" || ed.ComposeActive() {
		t.Fatalf("commit: state=%v buf=%q active=%v", s.State(), ed.Text(), ed.ComposeActive())
	}

	// Empty preedit clears an active composition (R2) — and must NOT start
	// one from Active.
	s.PreeditChanged(input.PreeditEvent{Text: "x"})
	s.PreeditChanged(input.PreeditEvent{Text: "", Cursor: -1})
	if s.State() != StateActive || ed.ComposeActive() {
		t.Fatalf("clear leg: state=%v active=%v", s.State(), ed.ComposeActive())
	}
}

func TestSessionIllegalTransitionsCounted(t *testing.T) {
	s, _, ad := newTestSession()
	before := s.IllegalTransitions()
	s.PreeditChanged(input.PreeditEvent{Text: "x"}) // idle: illegal
	s.Committed("y")                                // idle: illegal
	s.DeleteSurrounding(1, 0)                       // idle: illegal
	s.Session(false)                                // noise while idle
	if got := s.IllegalTransitions(); got != before+3 {
		t.Fatalf("illegal count = %d, want %d", got, before+3)
	}
	if ad.disables != 0 {
		t.Fatal("idle session must not emit protocol calls")
	}
}

func TestSessionCaretMovedDedup(t *testing.T) {
	s, ed, ad := newTestSession()
	s.AttachEditor(ed, fakeField{})
	n := len(ad.rects)
	r := platform.Rect{X: 1, Y: 2, W: 3, H: 4}
	s.CaretMoved(r)
	s.CaretMoved(r) // duplicate suppressed
	s.CaretMoved(r)
	if got := len(ad.rects) - n; got != 1 {
		t.Fatalf("dedup failed: %d anchor pushes for 3 identical moves", got)
	}
}

func TestSessionPreeditTriggersAnchorRefresh(t *testing.T) {
	s, ed, ad := newTestSession()
	s.AttachEditor(ed, anchorField{ed}) // rect varies with composition width
	base := len(ad.rects)
	s.PreeditChanged(input.PreeditEvent{Text: "pin yin"}) // §4.2: anchor must follow preedit width
	if len(ad.rects) <= base {
		t.Fatal("preedit change did not refresh the candidate anchor")
	}
}

func TestSessionDeleteSurroundingForwarded(t *testing.T) {
	s, ed, _ := newTestSession()
	s.AttachEditor(ed, fakeField{})
	ed.SetText("hello")
	ed.SetCaret(5)
	s.DeleteSurrounding(2, 0)
	if ed.Text() != "hel" {
		t.Fatalf("delete-surrounding result = %q", ed.Text())
	}
}

func TestEpochGatedPush(t *testing.T) {
	s, ed, _ := newTestSession()
	s.AttachEditor(ed, fakeField{})
	pushed := ed.Epoch()
	if s.PushSurroundingIfDirty(pushed) {
		t.Fatal("clean epoch should not push")
	}
	ed.Insert("z")
	if !s.PushSurroundingIfDirty(pushed) {
		t.Fatal("dirty epoch must push")
	}
	pushed = ed.Epoch()
	if s.PushSurroundingIfDirty(pushed) {
		t.Fatal("re-push without change must be suppressed")
	}
}
