package textinput

import (
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
)

type SessionState int

const (
	StateIdle SessionState = iota
	StateActive
	StateComposing
)

func (s SessionState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateActive:
		return "active"
	case StateComposing:
		return "composing"
	}
	return "?"
}

type PlatformAdapter interface {
	Enable(f platform.FieldSnapshot)
	Disable()
	CaretMoved(rect platform.Rect)
	SetPurpose(ct platform.ContentType)
	PushSurrounding(text string, cursor int)
}

type FieldSnapshotProvider interface {
	IMERect() platform.Rect
	ContentType() platform.ContentType
}

type ImeSession struct {
	ed      *Editor
	adapter PlatformAdapter
	field   FieldSnapshotProvider
	state    SessionState
	lastRect platform.Rect
	hasRect  bool
	illegal int
}

func NewImeSession(adapter PlatformAdapter) *ImeSession {
	return &ImeSession{adapter: adapter}
}

func (s *ImeSession) AttachEditor(ed *Editor, field FieldSnapshotProvider) {
	if s == nil || ed == nil || field == nil {
		s.noop()
		return
	}
	if s.state != StateIdle && s.adapter != nil {
		s.adapter.Disable()
	}
	s.ed, s.field = ed, field
	s.state = StateActive
	s.hasRect = false
	if s.adapter != nil {
		ct := field.ContentType()
		s.adapter.SetPurpose(ct)
		rect := field.IMERect()
		s.adapter.Enable(platform.FieldSnapshot{Rect: rect, Type: ct})
		s.lastRect, s.hasRect = rect, true
	}
}

func (s *ImeSession) DetachEditor() {
	if s == nil || s.state == StateIdle {
		s.noop()
		return
	}
	if s.ed != nil && s.ed.composing {
		s.ed.EndComposing()
	}
	s.ed, s.field = nil, nil
	s.state = StateIdle
	if s.adapter != nil {
		s.adapter.Disable()
	}
}

func (s *ImeSession) State() SessionState {
	if s == nil {
		return StateIdle
	}
	return s.state
}
func (s *ImeSession) IllegalTransitions() int { return s.illegal }
func (s *ImeSession) noop()                   { s.illegal++ }
func (s *ImeSession) compActive() bool {
	if s == nil || s.ed == nil {
		return false
	}
	return s.ed.composing
}

func (s *ImeSession) SetContentType(ct platform.ContentType) {
	if s == nil || s.state == StateIdle || s.adapter == nil {
		s.noop()
		return
	}
	s.adapter.SetPurpose(ct)
}

func (s *ImeSession) CaretMoved(rect platform.Rect) {
	if s == nil || s.state == StateIdle || s.adapter == nil {
		return
	}
	if s.hasRect && rect == s.lastRect {
		return
	}
	s.lastRect, s.hasRect = rect, true
	s.adapter.CaretMoved(rect)
}

func (s *ImeSession) PushSurroundingIfDirty(lastPushed uint64) bool {
	if s == nil || s.state == StateIdle || s.adapter == nil || s.ed == nil {
		return false
	}
	if s.ed.Epoch() == lastPushed {
		return false
	}
	text, cursor := s.ed.Snapshot()
	s.adapter.PushSurrounding(text, cursor)
	return true
}

func (s *ImeSession) PreeditChanged(e input.PreeditEvent) {
	if s == nil || s.state == StateIdle || s.ed == nil {
		s.noop()
		return
	}
	if e.Text == "" {
		if s.ed.composing {
			s.ed.EndComposing()
			s.toActive()
		}
		return
	}
	if !s.ed.composing {
		s.ed.BeginComposing()
	}
	cuOff := utf16Len(e.Text)
	if e.Cursor >= 0 && e.Cursor <= len(e.Text) {
		cuOff = utf16Len(e.Text[:e.Cursor])
		if e.Cursor < 0 {
			cuOff = utf16Len(e.Text)
		}
	}
	// UpdateComposingText replaces composingRange or selection and sets new range
	// sel is start+cuOff
	start := s.ed.composingRange.Start()
	if s.ed.composingRange.Collapsed() {
		start = s.ed.selection.Extent
		// need to re-derive after BeginComposing sets collapsed at caret
		start = s.ed.composingRange.Start()
	}
	sel := TextRange{Base: start + cuOff, Extent: start + cuOff}
	// Need to handle first preedit: UpdateComposingText expects composingRange possibly collapsed at caret
	// Use helper: if composingRange collapsed, UpdateComposingText will replace selection
	s.ed.UpdateComposingText(e.Text, sel)
	s.state = StateComposing
	if s.field != nil {
		s.CaretMoved(s.field.IMERect())
	}
}

func (s *ImeSession) Committed(text string) {
	if s == nil || s.state == StateIdle || s.ed == nil {
		s.noop()
		return
	}
	s.ed.AddText(text)
	s.toActive()
}

func (s *ImeSession) DeleteSurrounding(before, after int) {
	if s == nil || s.state == StateIdle || s.ed == nil {
		s.noop()
		return
	}
	s.ed.DeleteSurrounding(-before, before+after) // legacy calls with (before, after) counts
	// new DeleteSurrounding expects offset/count; adapt: before is positive count before caret, after is count after
	// Use direct: offset=-before, count=before+after? Actually spec: offset negative, count positive.
	// Keep simple forward to editor's byte version handled above.
	_ = after
}

func (s *ImeSession) Session(active bool) {
	if s == nil {
		return
	}
	switch {
	case active && s.state == StateIdle:
		s.noop()
	case !active && s.state != StateIdle:
		if s.ed != nil && s.ed.composing {
			s.ed.EndComposing()
		}
		s.state = StateActive
	}
}

func (s *ImeSession) toActive() {
	if s.state == StateComposing {
		s.state = StateActive
	}
}
