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

func TestRouterEventTargetOnPath(t *testing.T) {
	// A control implementing PointerHandler receives OnPointer automatically
	// when hit — the framework's unified event binding for custom controls.
	tgt := newTestTarget()
	r := NewInputRouter(fixedHit(tgt), nil)
	r.RoutePlatform(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerMove, X: 5, Y: 6,
	})
	if tgt.pointerCalls.Load() != 1 {
		t.Fatalf("handler not auto-wired: %d", tgt.pointerCalls.Load())
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

func TestRouterIMEWithoutEditorStillCallsOnIME(t *testing.T) {
	var imeGot atomic.Value
	r := NewInputRouter(nil, nil)
	r.OnIME = func(ev input.IMEEvent) { imeGot.Store(ev.Text) }
	r.Route(input.FromIME(input.IMEEvent{Kind: input.IMECommit, Text: "x"}, input.Modifiers{}))
	if imeGot.Load() != "x" {
		t.Fatalf("OnIME not called: %v", imeGot.Load())
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
	ed.BeginCompose("ni") // IME composing
	r.Route(input.Event{Kind: input.KindKey, Key: input.KeyEvent{Key: input.KeyN, Rune: 'n', Pressed: true}})
	if ed.Text() != "ni" {
		t.Fatalf("editor text = %q, want ni (no double insert)", ed.Text())
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
func (t *fakeTarget) IMERect() platform.Rect                  { return platform.Rect{X: 10, Y: 20, W: 100, H: 30} }

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

	// Compose + commit flow through the focused editor too; surrounding text
	// must EXCLUDE the pre-edit region while composing.
	r.Route(input.FromPlatform(platform.Event{Type: platform.EventIME, IMEKind: 0, IMEText: "ni"}, input.Modifiers{}))
	if !ed.ComposeActive() || ed.Text() != "ani" {
		t.Fatalf("compose state = %q active=%v", ed.Text(), ed.ComposeActive())
	}
	for _, s := range ime.surround {
		if s == `"ani"|`+itoa(ed.Cursor()) && ed.ComposeActive() {
			t.Fatalf("surrounding includes pre-edit: %v", ime.surround)
		}
	}
	r.Route(input.FromPlatform(platform.Event{Type: platform.EventIME, IMEKind: 1, IMEText: "你"}, input.Modifiers{}))
	if ed.Text() != "a你" {
		t.Fatalf("commit result = %q", ed.Text())
	}

	// Blur → pre-edit canceled + session disabled.
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
