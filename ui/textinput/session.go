package textinput

import (
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
)

// SessionState is the explicit IME session state machine (design D4/§4.6).
// Illegal transitions are no-ops and counted in IllegalTransitions so
// probes can see them; legal transitions have per-side-effect unit tests.
type SessionState int

const (
	// StateIdle no editable field focused.
	StateIdle SessionState = iota
	// StateActive a field holds the session, no composition live.
	StateActive
	// StateComposing a composition overlay is live on the attached editor.
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

// PlatformAdapter is the OUTBOUND half (host → platform), design §4.3.
// Implemented by each L0b backend; injected into ImeSession.
type PlatformAdapter interface {
	Enable(f platform.FieldSnapshot)
	Disable()
	CaretMoved(rect platform.Rect)
	SetPurpose(ct platform.ContentType)
	PushSurrounding(text string, cursor int)
}

// FieldSnapshotProvider is implemented by the focused edit target so the
// session can take field snapshots at Enable time (design I4 lineage).
type FieldSnapshotProvider interface {
	IMERect() platform.Rect
	ContentType() platform.ContentType
}

// ImeSession is THE facade upper layers depend on (user goal: "使用层直接
// 通过接口实例使用"). It implements the inbound handler half and exposes
// outbound actions; it owns the state machine. NOT concurrency-safe — all
// calls must be on the event-loop thread (§4.0 C1); async producers must
// post events instead.
type ImeSession struct {
	ed      *Editor
	adapter PlatformAdapter
	field   FieldSnapshotProvider

	state    SessionState
	lastRect platform.Rect
	hasRect  bool

	illegal int // no-op transition counter (probe-visible)
}

// NewImeSession wires the facade to its platform adapter.
func NewImeSession(adapter PlatformAdapter) *ImeSession {
	return &ImeSession{adapter: adapter}
}

// AttachEditor binds the editable target (focus-in). Opens the platform
// session with a fresh field snapshot.
func (s *ImeSession) AttachEditor(ed *Editor, field FieldSnapshotProvider) {
	if s == nil || ed == nil || field == nil {
		s.noop()
		return
	}
	if s.state != StateIdle && s.adapter != nil {
		s.adapter.Disable() // clean close of any previous field's session
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

// DetachEditor ends the session (focus-out). A live composition is dropped
// WITHOUT touching the buffer — the overlay vanishes by design (D1).
func (s *ImeSession) DetachEditor() {
	if s == nil || s.state == StateIdle {
		s.noop()
		return
	}
	if s.comp() != nil {
		s.ed.comp = nil
	}
	s.ed, s.field = nil, nil
	s.state = StateIdle
	if s.adapter != nil {
		s.adapter.Disable()
	}
}

// State returns the current machine state.
func (s *ImeSession) State() SessionState {
	if s == nil {
		return StateIdle
	}
	return s.state
}

// IllegalTransitions counts rejected events (probes/tests).
func (s *ImeSession) IllegalTransitions() int { return s.illegal }

func (s *ImeSession) noop() { s.illegal++ }

func (s *ImeSession) comp() *Composition {
	if s == nil || s.ed == nil {
		return nil
	}
	return s.ed.comp
}

// --- outbound actions (upper layers call these directly) ---

// SetContentType updates the declared purpose mid-session.
func (s *ImeSession) SetContentType(ct platform.ContentType) {
	if s == nil || s.state == StateIdle || s.adapter == nil {
		s.noop()
		return
	}
	s.adapter.SetPurpose(ct)
}

// CaretMoved refreshes the candidate anchor after local caret motion.
// Identical rects are suppressed here (motion storms must not reach the
// wire — v2.0/v2.3 lessons now structural).
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

// PushSurroundingIfDirty reports buffer+caret when they changed since the
// last push (epoch-gated; design D2). Off by default at call sites that
// cannot guarantee local-change-only semantics.
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

// --- inbound events (platform → ImeEventHandler) ---

// PreeditChanged applies a composition update. Legal in Active (starts a
// composition) and Composing (updates/clears it). R2: empty text never
// starts a session.
func (s *ImeSession) PreeditChanged(e input.PreeditEvent) {
	if s == nil || s.state == StateIdle || s.ed == nil {
		s.noop()
		return
	}
	if e.Text == "" {
		if s.ed.comp != nil {
			s.ed.comp = nil
			s.ed.changed()
			s.toActive()
		}
		return
	}
	if s.ed.comp == nil {
		s.ed.comp = &Composition{}
	}
	s.ed.comp.Text = e.Text
	s.ed.comp.Cursor = e.Cursor
	s.ed.comp.Segs = e.Segments
	s.ed.changed()
	s.state = StateComposing
	// The composition changes the anchor position — re-report it (§4.2:
	// CaretMoved MUST fire after preedit changes; v1.x regression guard).
	if s.field != nil {
		s.CaretMoved(s.field.IMERect())
	}
}

// Committed applies final text atomically (replaces the composing span via
// Editor.Insert). Legal in Active (direct commit) and Composing.
func (s *ImeSession) Committed(text string) {
	if s == nil || s.state == StateIdle || s.ed == nil {
		s.noop()
		return
	}
	s.ed.Insert(text) // clears the overlay inside
	s.toActive()
}

// DeleteSurrounding forwards the explicit deletion request. Buffer-only
// operation; composition overlay is unaffected.
func (s *ImeSession) DeleteSurrounding(before, after int) {
	if s == nil || s.state == StateIdle || s.ed == nil {
		s.noop()
		return
	}
	s.ed.DeleteSurrounding(before, after)
}

// Session applies engine-side activation news (enter/leave). An unexpected
// Disabled while idle is ignored as noise.
func (s *ImeSession) Session(active bool) {
	if s == nil {
		return
	}
	switch {
	case active && s.state == StateIdle:
		s.noop() // engine active with no field: nothing to attach to
	case !active && s.state != StateIdle:
		if s.comp() != nil {
			s.ed.comp = nil
		}
		s.state = StateActive
	}
}

func (s *ImeSession) toActive() {
	if s.state == StateComposing {
		s.state = StateActive
	}
}
