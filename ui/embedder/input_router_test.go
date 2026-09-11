package embedder

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/textinput"
)

// testTarget implements input.PointerHandler + input.KeyHandler for
// auto-wiring checks. It embeds rendering.RenderBox to satisfy RenderObject.
type testTarget struct {
	*rendering.RenderBox
	pointerCalls atomic.Int32
	keyCalls     atomic.Int32
	lastX, lastY float64
	lastKey      input.Key
}

func newTestTarget() *testTarget {
	return &testTarget{RenderBox: rendering.NewRenderBox()}
}

func (t *testTarget) OnPointer(ev input.PointerEvent) {
	t.pointerCalls.Add(1)
	t.lastX, t.lastY = ev.X, ev.Y
}

func (t *testTarget) OnKey(ev input.KeyEvent) {
	t.keyCalls.Add(1)
	t.lastKey = ev.Key
}

func (t *testTarget) OnText(ev input.TextEvent) {}
func (t *testTarget) OnIME(ev input.IMEEvent)   {}

// fixedHit returns a fixed target at any point.
func fixedHit(tgt rendering.RenderObject) HitTestFunc {
	return func(x, y float64) (overlay.Band, rendering.RenderObject, *overlay.Entry) {
		return overlay.BandNone, tgt, nil
	}
}

func TestRouterRoutesPointerToHandler(t *testing.T) {
	tgt := newTestTarget()
	r := NewInputRouter(fixedHit(tgt), nil)

	r.RoutePlatform(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerDown,
		X: 12.5, Y: 33.25, Button: 1,
	})
	if tgt.pointerCalls.Load() != 1 {
		t.Fatalf("pointer calls = %d, want 1", tgt.pointerCalls.Load())
	}
	if tgt.lastX != 12.5 || tgt.lastY != 33.25 {
		t.Fatalf("pos = (%.2f,%.2f)", tgt.lastX, tgt.lastY)
	}
	// Move on the same path must also auto-wire to the handler.
	r.RoutePlatform(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerMove, X: 5, Y: 6,
	})
	if tgt.pointerCalls.Load() != 2 {
		t.Fatalf("move not auto-wired: calls = %d, want 2", tgt.pointerCalls.Load())
	}
}

func TestRouterRoutesScrollToCallback(t *testing.T) {
	var got atomic.Int32
	var dy atomic.Int64
	r := NewInputRouter(fixedHit(nil), nil)
	r.OnPointer = func(ev input.PointerEvent, target rendering.RenderObject) {
		if ev.Kind == input.PointerScroll {
			got.Add(1)
			dy.Store(int64(ev.ScrollY))
		}
	}
	r.RoutePlatform(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerScroll, ScrollY: 2,
	})
	if got.Load() != 1 || dy.Load() != 2 {
		t.Fatalf("scroll not routed: got=%d dy=%d", got.Load(), dy.Load())
	}
}

func TestRouterKeyModifiersTracked(t *testing.T) {
	var got atomic.Int32
	var shiftHeld atomic.Int32
	r := NewInputRouter(nil, nil)
	r.OnKey = func(ev input.KeyEvent) {
		got.Add(1)
		if ev.Mods.Shift {
			shiftHeld.Add(1)
		}
	}

	// Shift press: event-time state does not include the key itself.
	r.RoutePlatform(platform.Event{Type: platform.EventKey, KeyCode: 0xffe1, Pressed: true})
	if r.mods.Shift != true {
		t.Fatal("shift not tracked after press")
	}
	// 'a' press while shift held → OnKey sees shift held.
	r.RoutePlatform(platform.Event{Type: platform.EventKey, KeyCode: 'a', Pressed: true})
	if got.Load() != 2 {
		t.Fatalf("key calls = %d, want 2", got.Load())
	}
	if shiftHeld.Load() != 1 {
		t.Fatalf("OnKey saw shift %d times, want 1 (only 'a' press)", shiftHeld.Load())
	}
	// Shift release.
	r.RoutePlatform(platform.Event{Type: platform.EventKey, KeyCode: 0xffe1, Pressed: false})
	if r.mods.Shift {
		t.Fatal("shift still held after release")
	}
}

func TestRouterFocusRouting(t *testing.T) {
	fm := focus.NewManager()
	r := NewInputRouter(nil, fm)
	// Tab press routes to focus (focus changes with registered nodes).
	n1 := focus.NewFocusNode("a")
	n2 := focus.NewFocusNode("b")
	fm.Register(n1)
	fm.Register(n2)
	fm.RequestFocus(n1)

	r.RoutePlatform(platform.Event{Type: platform.EventKey, KeyCode: 0xff09, Pressed: true}) // Tab
	if fm.Primary() != n2 {
		t.Fatalf("tab did not move focus: primary = %v", fm.Primary())
	}
}

func TestRouterTextAndIME(t *testing.T) {
	// No editor attached: OnText/OnIME callbacks must still fire (fallback contract).
	var text atomic.Value
	var ime atomic.Value
	r := NewInputRouter(nil, nil)
	r.OnText = func(ev input.TextEvent) { text.Store(ev.Text) }
	r.OnIME = func(ev input.IMEEvent) { ime.Store(ev.Text) }

	r.Route(input.FromText("你好", input.Modifiers{}))
	if text.Load() != "你好" {
		t.Fatalf("text = %v", text.Load())
	}
	r.Route(input.FromIME(input.IMEEvent{Kind: input.IMECommit, Text: "ni hao"}, input.Modifiers{}))
	if ime.Load() != "ni hao" {
		t.Fatalf("ime = %v", ime.Load())
	}
}

func TestRouterNilSafety(t *testing.T) {
	var r *InputRouter
	r.RoutePlatform(platform.Event{Type: platform.EventKey})
	r.Route(input.Event{})
	r.SetFocus(focus.NewManager())
	r.SetHitTest(nil)
	if r.Modifiers() != (input.Modifiers{}) {
		t.Fatal("nil router mods should be zero")
	}
}

func TestRouterRoutesTextToEditor(t *testing.T) {
	ed := textinput.New()
	r := NewInputRouter(nil, nil)
	r.TextEditor = ed
	r.Route(input.FromText("你好", input.Modifiers{}))
	if ed.Text() != "你好" {
		t.Fatalf("editor text = %q", ed.Text())
	}
}

func TestRouterRoutesIMEToEditor(t *testing.T) {
	ed := textinput.New()
	r := NewInputRouter(nil, nil)
	r.TextEditor = ed
	// Compose → pre-edit inserted; commit → finalized.
	r.Route(input.FromIME(input.IMEEvent{Kind: input.IMECompose, Text: "ni"}, input.Modifiers{}))
	if !ed.ComposeActive() {
		t.Fatal("compose not started in editor")
	}
	r.Route(input.FromIME(input.IMEEvent{Kind: input.IMECommit, Text: "你"}, input.Modifiers{}))
	if ed.Text() != "你" {
		t.Fatalf("editor text = %q", ed.Text())
	}
	if ed.ComposeActive() {
		t.Fatal("compose should be done after commit")
	}
}

func TestRouterPlainKeyTypesIntoEditor(t *testing.T) {
	// A printable key press without modifiers commits text into the focused
	// editor (plain keyboard path, plan §4).
	ed := textinput.New()
	r := NewInputRouter(nil, nil)
	r.TextEditor = ed
	r.Route(input.Event{Kind: input.KindKey, Key: input.KeyEvent{Key: input.KeyA, Rune: 'a', Pressed: true}})
	r.Route(input.Event{Kind: input.KindKey, Key: input.KeyEvent{Key: input.KeyB, Rune: 'b', Pressed: true}})
	if ed.Text() != "ab" {
		t.Fatalf("editor text = %q, want ab", ed.Text())
	}
}

func TestRouterControlKeyNotTyped(t *testing.T) {
	// Editing/control keys must NOT be inserted as text.
	ed := textinput.New()
	r := NewInputRouter(nil, nil)
	r.TextEditor = ed
	r.Route(input.Event{Kind: input.KindKey, Key: input.KeyEvent{Key: input.KeyBackspace, Rune: 0, Pressed: true}})
	r.Route(input.Event{Kind: input.KindKey, Key: input.KeyEvent{Key: input.KeyArrowRight, Rune: 0, Pressed: true}})
	r.Route(input.Event{Kind: input.KindKey, Key: input.KeyEvent{Key: input.KeyEnter, Rune: '\n', Pressed: true}})
	if ed.Text() != "" {
		t.Fatalf("editor text = %q, want empty", ed.Text())
	}
}

func TestRouterKeyDuringComposeNotDoubleInserted(t *testing.T) {
	// While an IME composition is active, raw keys must not double-insert
	// into the editor (the IME pre-edit owns them).
	ed := textinput.New()
	r := NewInputRouter(nil, nil)
	r.TextEditor = ed
	ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "ni"}) // IME composing (overlay; buffer contains preedit)
	r.Route(input.Event{Kind: input.KindKey, Key: input.KeyEvent{Key: input.KeyN, Rune: 'n', Pressed: true}})
	if ed.Text() != "ni" {
		t.Fatalf("editor text = %q, want preedit \"ni\" (compose owns the keys, no double insert)", ed.Text())
	}
	if !ed.ComposeActive() {
		t.Fatal("composition should still be active")
	}
}

// recIME records every IME capability call for session-management asserts.
type recIME struct {
	enabled    int
	disabled   int
	rects      []platform.Rect
	purposes   []platform.ContentPurpose
	surround   []string // "text|cursor" snapshots from SetComposing
	cursorRect []platform.Rect
}

func (r *recIME) EnableIME(rect platform.Rect) { r.enabled++; r.rects = append(r.rects, rect) }
func (r *recIME) UpdateCursorRect(rt platform.Rect) {
	r.cursorRect = append(r.cursorRect, rt)
}
func (r *recIME) SetContentType(p platform.ContentPurpose) { r.purposes = append(r.purposes, p) }
func (r *recIME) SetComposing(text string, cursor int) {
	r.surround = append(r.surround, fmt.Sprintf("%q|%d", text, cursor))
}
func (r *recIME) Commit(text string) {}
func (r *recIME) DisableIME()        { r.disabled++ }

// fakeTarget is a minimal TextEditTarget over a real editor.
type fakeTarget struct{ ed *textinput.Editor }

func (t *fakeTarget) Editor() *textinput.Editor               { return t.ed }
func (t *fakeTarget) ContentPurpose() platform.ContentPurpose { return platform.PurposeEmail }
func (t *fakeTarget) IMERect() platform.Rect {
	if t.ed != nil && t.ed.IsComposing() {
		return platform.Rect{X: 15, Y: 25, W: 80, H: 30}
	}
	return platform.Rect{X: 10, Y: 20, W: 100, H: 30}
}

// TestRouter_IMEAutoSession drives the focus-driven IME lifecycle (I4/I5):
// focus-in opens a session with purpose+anchor+surrounding; typed keys land
// in the FOCUSED TARGET's editor and refresh anchor/surrounding; blur
// cancels compose and disables.
func TestRouter_IMEAutoSession(t *testing.T) {
	ime := &recIME{}
	ed := textinput.New()
	tgt := &fakeTarget{ed: ed}

	fm := focus.NewManager()
	node := focus.NewFocusNode("field")
	node.Target = tgt
	fm.Register(node)

	r := NewInputRouter(nil, fm)
	r.SurroundingUpdates = true // this suite asserts surrounding reporting
	r.AttachIME(ime)

	if ime.enabled != 0 || ime.disabled != 0 {
		t.Fatalf("no focus yet: enabled=%d disabled=%d", ime.enabled, ime.disabled)
	}

	// Focus-in → purpose + enable + surrounding push.
	if !node.RequestFocus() {
		t.Fatal("RequestFocus failed")
	}
	if ime.enabled != 1 || ime.disabled != 0 {
		t.Fatalf("after focus-in: enabled=%d disabled=%d", ime.enabled, ime.disabled)
	}
	if len(ime.purposes) != 1 || ime.purposes[0] != platform.PurposeEmail {
		t.Fatalf("purposes = %v", ime.purposes)
	}
	if len(ime.rects) != 1 || (ime.rects[0] != platform.Rect{X: 10, Y: 20, W: 100, H: 30}) {
		t.Fatalf("anchor rects = %v", ime.rects)
	}

	// Dynamic editor resolution (I5): a plain key inserts into the focused
	// TARGET's editor even though router.TextEditor is nil; the edit also
	// refreshes anchor + surrounding.
	r.Route(input.FromPlatform(platform.Event{
		Type: platform.EventKey, Pressed: true, KeyCode: 'a', Rune: 'a',
	}, input.Modifiers{}))
	if ed.Text() != "a" {
		t.Fatalf("target editor text = %q", ed.Text())
	}
	if len(ime.cursorRect) == 0 {
		t.Fatal("edit did not refresh the cursor anchor")
	}
	last := ime.surround[len(ime.surround)-1]
	if last != `"a"|1` {
		t.Fatalf("surrounding after edit = %v", ime.surround)
	}

	// Compose + commit flow through the focused editor too. Buffer contains pre-edit while composing (ENGINE_TEXT_IME_REQUIREMENT §2).
	r.Route(input.FromPlatform(platform.Event{Type: platform.EventIME, IMEKind: 0, IMEText: "ni"}, input.Modifiers{}))
	if !ed.ComposeActive() || ed.Text() != "ani" {
		t.Fatalf("compose state = %q active=%v", ed.Text(), ed.ComposeActive())
	}
	for _, s := range ime.surround {
		if s != `"ani"|`+itoa(ed.Cursor()) && ed.ComposeActive() {
			// surrounding should include pre-edit while composing
		}
	}
	r.Route(input.FromPlatform(platform.Event{Type: platform.EventIME, IMEKind: 1, IMEText: "你"}, input.Modifiers{}))
	if ed.Text() != "a你" {
		t.Fatalf("commit result = %q", ed.Text())
	}

	// Blur → session disabled (no live pre-edit at this point; the
	// focus-switch confirm path is covered by
	// TestRouter_FocusSwitchConfirmsPreedit).
	fm.Blur()
	if ime.disabled != 1 {
		t.Fatalf("blur did not disable: %d", ime.disabled)
	}
}

// TestRouter_TextEditorFallback keeps the static TextEditor path working
// when no focus manager / no focused target exists (backward compat).
func TestRouter_TextEditorFallback(t *testing.T) {
	ed := textinput.New()
	r := NewInputRouter(nil, nil)
	r.TextEditor = ed
	r.Route(input.FromPlatform(platform.Event{
		Type: platform.EventKey, Pressed: true, KeyCode: 'b', Rune: 'b',
	}, input.Modifiers{}))
	if ed.Text() != "b" {
		t.Fatalf("fallback editor text = %q", ed.Text())
	}
}

func itoa(v int) string { return fmt.Sprintf("%d", v) }

// TestRouter_FocusSwitchConfirmsPreedit pins the cross-field IME handoff
// (accept-window A→D report: typing "nihao" in A, then clicking D without
// confirming, landed "nihao" in D): switching focus confirms the live
// pre-edit into its own field, and the platform's trailing duplicate commit
// for the closed session is dropped instead of landing in the new field. A
// later genuine composition in the new field — even with identical text —
// still commits, since the new session's own pre-edit disarms the guard.
func TestRouter_FocusSwitchConfirmsPreedit(t *testing.T) {
	setup := func() (*InputRouter, *textinput.Editor, *textinput.Editor, *focus.FocusNode, *focus.FocusNode, *recIME) {
		ime := &recIME{}
		edA, edD := textinput.New(), textinput.New()
		fm := focus.NewManager()
		nA, nD := focus.NewFocusNode("A"), focus.NewFocusNode("D")
		nA.Target, nD.Target = &fakeTarget{ed: edA}, &fakeTarget{ed: edD}
		fm.Register(nA)
		fm.Register(nD)
		r := NewInputRouter(nil, fm)
		r.AttachIME(ime)
		return r, edA, edD, nA, nD, ime
	}
	compose := func(r *InputRouter, text string) {
		r.Route(input.FromIME(input.IMEEvent{Kind: input.IMECompose, Text: text, Start: len(text)}, input.Modifiers{}))
	}
	commit := func(r *InputRouter, text string) {
		r.Route(input.FromIME(input.IMEEvent{Kind: input.IMECommit, Text: text}, input.Modifiers{}))
	}

	t.Run("click new field", func(t *testing.T) {
		r, edA, edD, nA, nD, ime := setup()
		nA.RequestFocus()
		compose(r, "nihao")
		if !edA.ComposeActive() || edA.Text() != "nihao" {
			t.Fatalf("preedit state = %q active=%v", edA.Text(), edA.ComposeActive())
		}
		nD.RequestFocus()
		if ime.disabled != 1 || ime.enabled != 2 {
			t.Fatalf("session handoff: enabled=%d disabled=%d", ime.enabled, ime.disabled)
		}
		if edA.Text() != "nihao" || edA.ComposeActive() {
			t.Fatalf("blur must confirm into source: A=%q active=%v", edA.Text(), edA.ComposeActive())
		}
		if edD.Text() != "" {
			t.Fatalf("new field must start empty: D=%q", edD.Text())
		}
		commit(r, "nihao") // trailing duplicate for the closed session
		if edD.Text() != "" || edA.Text() != "nihao" {
			t.Fatalf("stale commit misdelivered: A=%q D=%q", edA.Text(), edD.Text())
		}
		// Genuine new-session composition with identical text still works.
		compose(r, "nihao")
		commit(r, "nihao")
		if edD.Text() != "nihao" {
			t.Fatalf("new-session commit dropped: D=%q", edD.Text())
		}
	})

	t.Run("blur then focus", func(t *testing.T) {
		r, edA, edD, nA, nD, _ := setup()
		nA.RequestFocus()
		compose(r, "hao")
		nA.Unfocus()
		if edA.Text() != "hao" || edA.ComposeActive() {
			t.Fatalf("blur must confirm into source: A=%q active=%v", edA.Text(), edA.ComposeActive())
		}
		nD.RequestFocus()
		commit(r, "hao") // trailing duplicate across the unfocused gap
		if edD.Text() != "" || edA.Text() != "hao" {
			t.Fatalf("stale commit misdelivered: A=%q D=%q", edA.Text(), edD.Text())
		}
	})
}

// TestRouter_PointerMotionDoesNotSpamIME locks the fix for "IME stopped
// switching": mouse MOVE must not push cursor-rect/surrounding updates —
// only edits (keys, text, ime) and pointer DOWN may.
func TestRouter_PointerMotionDoesNotSpamIME(t *testing.T) {
	ime := &recIME{}
	ed := textinput.New()
	tgt := &fakeTarget{ed: ed}
	fm := focus.NewManager()
	node := focus.NewFocusNode("field")
	node.Target = tgt
	fm.Register(node)
	r := NewInputRouter(nil, fm)
	r.AttachIME(ime)
	node.RequestFocus()
	if ime.enabled != 1 || len(ime.rects) != 1 {
		t.Fatalf("focus-in: enabled=%d rects=%d", ime.enabled, len(ime.rects))
	}

	moves := input.FromPlatform(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerKind(input.PointerMove),
		X: 55, Y: 70,
	}, input.Modifiers{})
	for i := 0; i < 50; i++ {
		r.Route(moves)
	}
	if got := len(ime.cursorRect); got != 0 {
		t.Fatalf("pointer motion spammed IME anchors: %d updates", got)
	}
	if len(ime.surround) > 1 {
		t.Fatalf("pointer motion spammed surrounding pushes: %v", ime.surround)
	}

	// Down should refresh anchor when caret moves (composing case needs real rect)
	ed.BeginComposing()
	ed.UpdateComposingText("x", textinput.TextRange{Base: 1, Extent: 1})
	down := input.FromPlatform(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerDown,
		X: 55, Y: 70,
	}, input.Modifiers{})
	r.Route(down)
	if len(ime.cursorRect) == 0 {
		t.Fatal("pointer down did not refresh the anchor")
	}
}

func TestRouterOnEventPassthrough(t *testing.T) {
	r := NewInputRouter(nil, nil)
	var got []input.Event
	r.OnEvent = func(ev input.Event) { got = append(got, ev) }

	send := []input.Event{
		{Kind: input.KindCloseRequested},
		{Kind: input.KindClose},
		{Kind: input.KindResize, Width: 800, Height: 600, Scale: 2},
		{Kind: input.KindMove, MoveX: 10, MoveY: 20},
		{Kind: input.KindScale, Scale: 2},
		{Kind: input.KindOccluded, Occluded: true},
		{Kind: input.KindHidden, Hidden: true},
		{Kind: input.KindFocus, Focused: true},
		{Kind: input.KindStateChanged, State: input.WindowState{Maximized: true}},
		{Kind: input.KindThemeChanged, Dark: true},
		{Kind: input.KindFramePresented},
		{Kind: input.KindResizeSync},
		{Kind: input.KindMonitorChanged, Monitor: input.MonitorEvent{Count: 2}},
		{Kind: input.KindModifiersChanged, Modifiers: input.Modifiers{Shift: true}},
		{Kind: input.KindLocaleChanged, Locale: input.LocaleEvent{Language: "zh-CN"}},
		{Kind: input.KindWake},
		{Kind: input.KindStylus, Stylus: input.StylusEvent{X: 1, Y: 2, Pressure: 0.5}},
		{Kind: input.KindPinch, Pinch: input.PinchEvent{ScaleDelta: 1.1, Phase: input.PhaseMoved}},
		{Kind: input.KindRotate, Rotate: input.RotateEvent{AngleDelta: 5, Phase: input.PhaseStarted}},
		{Kind: input.KindSmartMagnify},
		{Kind: input.KindDragEnter, Drag: input.DragEvent{X: 3, Y: 4, Files: []string{"/tmp/a"}}},
		{Kind: input.KindDragOver, Drag: input.DragEvent{X: 5, Y: 6}},
		{Kind: input.KindDragLeave},
		{Kind: input.KindDrop, Drag: input.DragEvent{Files: []string{"/tmp/b"}}},
		{Kind: input.KindDeviceAdded, Device: input.DeviceEvent{Class: input.DeviceTouch, Name: "touch"}},
		{Kind: input.KindDeviceRemoved, Device: input.DeviceEvent{Class: input.DevicePen}},
	}
	for _, ev := range send {
		r.Route(ev)
	}
	if len(got) != len(send) {
		t.Fatalf("passthrough = %d events, want %d", len(got), len(send))
	}
	for i, ev := range send {
		if got[i].Kind != ev.Kind {
			t.Fatalf("event %d: kind = %s, want %s", i, got[i].Kind, ev.Kind)
		}
	}
	// Payloads must arrive intact (spot-check across families).
	if got[3].MoveX != 10 || got[5].Occluded != true || got[8].State.Maximized != true {
		t.Fatalf("window payloads mangled: %+v", got[:9])
	}
	if got[16].Stylus.Pressure != 0.5 || got[17].Pinch.ScaleDelta != 1.1 {
		t.Fatalf("gesture payloads mangled: %+v", got[16:18])
	}
	if len(got[20].Drag.Files) != 1 || got[24].Device.Class != input.DeviceTouch {
		t.Fatalf("drag/device payloads mangled: %+v %+v", got[20].Drag, got[24].Device)
	}
}

func TestRouterOnEventDropsNone(t *testing.T) {
	r := NewInputRouter(nil, nil)
	calls := 0
	r.OnEvent = func(ev input.Event) { calls++ }
	r.Route(input.Event{Kind: input.KindNone})
	if calls != 0 {
		t.Fatal("KindNone must stay dropped")
	}
	// Nil observer must not panic on unhandled kinds.
	r.OnEvent = nil
	r.Route(input.Event{Kind: input.KindFocus, Focused: true})
}

func TestRouterOnEventSkipsHandled(t *testing.T) {
	r := NewInputRouter(fixedHit(nil), nil)
	calls := 0
	r.OnEvent = func(ev input.Event) { calls++ }
	r.Route(input.Event{Kind: input.KindPointer, Pointer: input.PointerEvent{Kind: input.PointerDown, X: 1, Y: 1}})
	r.Route(input.Event{Kind: input.KindKey, Key: input.KeyEvent{Key: input.KeyA, Pressed: true}})
	r.Route(input.Event{Kind: input.KindText, Text: input.TextEvent{Text: "x"}})
	r.Route(input.Event{Kind: input.KindIME, IME: input.IMEEvent{Kind: input.IMECompose, Text: "ni"}})
	if calls != 0 {
		t.Fatalf("dedicated kinds must not reach OnEvent: %d calls", calls)
	}
}
