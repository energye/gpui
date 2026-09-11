package input

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func TestFromPlatform_KeyASCII(t *testing.T) {
	// X11 keysym: lowercase 'a' = 0x61 with rune 'a'.
	ev := FromPlatform(platform.Event{
		Type:    platform.EventKey,
		KeyCode: 0x61, // 'a'
		Rune:    'a',
		Pressed: true,
	}, Modifiers{})
	if ev.Kind != KindKey {
		t.Fatalf("kind = %s, want key", ev.Kind)
	}
	if ev.Key.Key != KeyA {
		t.Fatalf("key = %s, want a", ev.Key.Key)
	}
	if ev.Key.Rune != 'a' {
		t.Fatalf("rune = %q, want a", ev.Key.Rune)
	}
	if !ev.Key.Pressed {
		t.Fatal("Pressed should be true")
	}
	if !ev.Modifiers.IsEmpty() {
		t.Fatal("mods should be empty")
	}
}

func TestFromPlatform_KeySpecial(t *testing.T) {
	cases := []struct {
		code int
		want Key
	}{
		{0xff09, KeyTab},
		{0xffe1, KeyShift},
		{0xffe2, KeyShift},
		{0xffe3, KeyControl},
		{0xffe9, KeyAlt},
		{0xffeb, KeyMeta},
		{0xff08, KeyBackspace},
		{0xff1b, KeyEscape},
		{0xff51, KeyArrowLeft},
		{0xff52, KeyArrowUp},
		{0xff53, KeyArrowRight},
		{0xff54, KeyArrowDown},
		{0xff50, KeyHome},
		{0xff57, KeyEnd},
		{0xff55, KeyPageUp},
		{0xff56, KeyPageDown},
		{0xffbe, KeyF1},
		{0xffc9, KeyF12},
		{0xffd5, KeyF24},
	}
	for _, c := range cases {
		ev := FromPlatform(platform.Event{Type: platform.EventKey, KeyCode: c.code, Pressed: true}, Modifiers{})
		if ev.Key.Key != c.want {
			t.Errorf("keysym 0x%x: got %s, want %s", c.code, ev.Key.Key, c.want)
		}
	}
}

func TestFromPlatform_EnterKeys(t *testing.T) {
	// keysym Return (0xff0d) → KeyEnter logical key.
	ev := FromPlatform(platform.Event{Type: platform.EventKey, KeyCode: 0xff0d, Pressed: true}, Modifiers{})
	if ev.Key.Key != KeyEnter {
		t.Fatalf("got %s, want enter", ev.Key.Key)
	}
	// ASCII '\r' also enters.
	ev2 := FromPlatform(platform.Event{Type: platform.EventKey, KeyCode: int('\r'), Pressed: true}, Modifiers{})
	if ev2.Key.Key != KeyEnter {
		t.Fatalf("CR: got %s, want enter", ev2.Key.Key)
	}
}

func TestFromPlatform_KeyModifiers(t *testing.T) {
	mods := Modifiers{Shift: true, Control: true}
	ev := FromPlatform(platform.Event{Type: platform.EventKey, KeyCode: 'z', Pressed: false}, mods)
	if ev.Key.Pressed {
		t.Fatal("pressed should be false")
	}
	if ev.Key.Key != KeyZ {
		t.Fatalf("got %s, want z", ev.Key.Key)
	}
	if !ev.Modifiers.Shift || !ev.Modifiers.Control || ev.Modifiers.Alt || ev.Modifiers.Meta {
		t.Fatalf("mods not preserved: %+v", ev.Modifiers)
	}
}

func TestFromPlatform_Pointer(t *testing.T) {
	ev := FromPlatform(platform.Event{
		Type:    platform.EventPointer,
		Pointer: platform.PointerDown,
		X:       12.5,
		Y:       33.25,
		Button:  1,
	}, Modifiers{})
	if ev.Kind != KindPointer {
		t.Fatalf("kind = %s, want pointer", ev.Kind)
	}
	if ev.Pointer.ID != PrimaryPointerID {
		t.Fatalf("id = %d, want primary", ev.Pointer.ID)
	}
	if ev.Pointer.X != 12.5 || ev.Pointer.Y != 33.25 {
		t.Fatalf("pos = (%.2f,%.2f)", ev.Pointer.X, ev.Pointer.Y)
	}
	if ev.Pointer.Kind != PointerDown || ev.Pointer.Button != 1 {
		t.Fatalf("kind/button = %s/%d", ev.Pointer.Kind, ev.Pointer.Button)
	}
}

func TestFromPlatform_PointerMoveIsMove(t *testing.T) {
	ev := FromPlatform(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerMove, X: 1, Y: 2,
	}, Modifiers{})
	if ev.Pointer.Kind != PointerMove {
		t.Fatalf("got %s, want move", ev.Pointer.Kind)
	}
}

func TestFromPlatform_Scroll(t *testing.T) {
	ev := FromPlatform(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerScroll,
		ScrollY: 1, ScrollX: -0.5,
	}, Modifiers{})
	if ev.Kind != KindScroll {
		t.Fatalf("kind = %s, want scroll", ev.Kind)
	}
	if ev.Pointer.Kind != PointerScroll || ev.Pointer.ScrollY != 1 || ev.Pointer.ScrollX != -0.5 {
		t.Fatalf("scroll = %+v", ev.Pointer)
	}
}

func TestFromPlatform_Lifecycle(t *testing.T) {
	cases := []struct {
		ev   platform.Event
		kind Kind
	}{
		{platform.Event{Type: platform.EventCloseRequested}, KindCloseRequested},
		{platform.Event{Type: platform.EventClose}, KindClose},
		{platform.Event{Type: platform.EventWake}, KindWake},
		{platform.Event{Type: platform.EventResize, Width: 800, Height: 600, Scale: 2}, KindResize},
		{platform.Event{Type: platform.EventExpose}, KindNone}, // exposure is a redraw signal, not input
		{platform.Event{Type: platform.EventNone}, KindNone},
	}
	for _, c := range cases {
		ev := FromPlatform(c.ev, Modifiers{})
		if ev.Kind != c.kind {
			t.Errorf("type %d: got %s, want %s", c.ev.Type, ev.Kind, c.kind)
		}
		if c.kind == KindResize {
			if ev.Width != 800 || ev.Height != 600 || ev.Scale != 2 {
				t.Fatalf("resize = %dx%d scale %.1f", ev.Width, ev.Height, ev.Scale)
			}
		}
	}
}

func TestFromPlatform_CloseSplit(t *testing.T) {
	// CloseRequested (interceptable ask) and Close (already destroyed) are
	// distinct Kinds; the embedder quits on both (see embedder.EventQuits).
	if ev := FromPlatform(platform.Event{Type: platform.EventCloseRequested}, Modifiers{}); ev.Kind != KindCloseRequested {
		t.Fatalf("close-requested = %s, want close-requested", ev.Kind)
	}
	if ev := FromPlatform(platform.Event{Type: platform.EventClose}, Modifiers{}); ev.Kind != KindClose {
		t.Fatalf("close = %s, want close", ev.Kind)
	}
}

func TestFromPlatform_KeyRepeat(t *testing.T) {
	ev := FromPlatform(platform.Event{Type: platform.EventKey, KeyCode: 'a', Rune: 'a', Pressed: true, Repeat: true}, Modifiers{})
	if ev.Kind != KindKey || !ev.Key.Repeat {
		t.Fatalf("repeat not carried: %+v", ev.Key)
	}
	plain := FromPlatform(platform.Event{Type: platform.EventKey, KeyCode: 'a', Rune: 'a', Pressed: true}, Modifiers{})
	if plain.Key.Repeat {
		t.Fatal("unset Repeat must stay false")
	}
}

func TestFromPlatform_PointerPhases(t *testing.T) {
	cases := []struct {
		in   platform.PointerKind
		want PointerKind
	}{
		{platform.PointerMove, PointerMove},
		{platform.PointerDown, PointerDown},
		{platform.PointerUp, PointerUp},
		{platform.PointerEnter, PointerEnter},
		{platform.PointerLeave, PointerLeave},
		{platform.PointerCancel, PointerCancel},
	}
	for _, c := range cases {
		ev := FromPlatform(platform.Event{Type: platform.EventPointer, Pointer: c.in, X: 5, Y: 6}, Modifiers{})
		if ev.Kind != KindPointer || ev.Pointer.Kind != c.want {
			t.Errorf("phase %v: got %s/%s", c.in, ev.Kind, ev.Pointer.Kind)
		}
	}
}

func TestFromPlatform_HorizontalScroll(t *testing.T) {
	// X11 buttons 6/7 arrive as ordinary button presses; only the press edge
	// surfaces as horizontal scroll (backend 4/5 convention: negative =
	// left/up), so one tilt ticks once. The release stays an ordinary Up.
	press6 := FromPlatform(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, Button: 6, X: 1, Y: 2}, Modifiers{})
	if press6.Kind != KindScroll || press6.Pointer.ScrollX != -1 || press6.Pointer.ScrollY != 0 {
		t.Fatalf("button 6 press: got %s scroll %+v", press6.Kind, press6.Pointer)
	}
	press7 := FromPlatform(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, Button: 7}, Modifiers{})
	if press7.Kind != KindScroll || press7.Pointer.ScrollX != 1 {
		t.Fatalf("button 7 press: got %s scroll %+v", press7.Kind, press7.Pointer)
	}
	release := FromPlatform(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, Button: 6}, Modifiers{})
	if release.Kind != KindPointer || release.Pointer.Kind != PointerUp || release.Pointer.Button != 6 {
		t.Fatalf("button 6 release must stay an ordinary up: %+v", release.Pointer)
	}
	// Ordinary buttons and moves are untouched.
	btn1 := FromPlatform(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, Button: 1}, Modifiers{})
	if btn1.Kind != KindPointer || btn1.Pointer.Kind != PointerDown || btn1.Pointer.Button != 1 {
		t.Fatalf("button 1 changed: %+v", btn1.Pointer)
	}
	move := FromPlatform(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerMove, Button: 6}, Modifiers{})
	if move.Kind != KindPointer || move.Pointer.Kind != PointerMove {
		t.Fatalf("move with button 6 must stay a move: %+v", move.Pointer)
	}
}

func TestFromPlatform_ResizeDefaultScale(t *testing.T) {
	ev := FromPlatform(platform.Event{Type: platform.EventResize, Width: 10, Height: 10, Scale: 0}, Modifiers{})
	if ev.Scale != 1 {
		t.Fatalf("scale = %.1f, want default 1", ev.Scale)
	}
}

func TestFromTouch(t *testing.T) {
	ev := FromTouch(TouchEvent{Kind: PointerDown, ID: 2, X: 30, Y: 40}, Modifiers{Alt: true})
	if ev.Kind != KindTouch || ev.Touch.ID != 2 || ev.Touch.X != 30 || ev.Touch.Y != 40 {
		t.Fatalf("touch = %+v", ev)
	}
	if !ev.Modifiers.Alt {
		t.Fatal("touch mods not preserved")
	}
}

func TestFromText(t *testing.T) {
	ev := FromText("你好", Modifiers{})
	if ev.Kind != KindText || ev.Text.Text != "你好" {
		t.Fatalf("text = %+v", ev)
	}
}

func TestFromIME(t *testing.T) {
	ev := FromIME(IMEEvent{Kind: IMECommit, Text: "ni hao"}, Modifiers{})
	if ev.Kind != KindIME || ev.IME.Kind != IMECommit || ev.IME.Text != "ni hao" {
		t.Fatalf("ime = %+v", ev)
	}
}

func TestFromPlatform_IME(t *testing.T) {
	// platform.Event{Type: platform.EventIME} → normalized input.Event{Kind: IME}.
	ev := FromPlatform(platform.Event{
		Type: platform.EventIME, IMEKind: 0, IMEText: "ni", IMEStart: 5, IMEEnd: 7,
	}, Modifiers{})
	if ev.Kind != KindIME {
		t.Fatalf("kind = %s, want ime", ev.Kind)
	}
	if ev.IME.Kind != IMECompose || ev.IME.Text != "ni" || ev.IME.Start != 5 || ev.IME.End != 7 {
		t.Fatalf("ime = %+v", ev.IME)
	}
	// Commit kind.
	ev2 := FromPlatform(platform.Event{Type: platform.EventIME, IMEKind: 1, IMEText: "你"}, Modifiers{})
	if ev2.IME.Kind != IMECommit || ev2.IME.Text != "你" {
		t.Fatalf("ime commit = %+v", ev2.IME)
	}
	// Out-of-range kind clamps to compose.
	ev3 := FromPlatform(platform.Event{Type: platform.EventIME, IMEKind: 99, IMEText: "x"}, Modifiers{})
	if ev3.IME.Kind != IMECompose {
		t.Fatalf("out-of-range kind = %+v", ev3.IME)
	}
	// Delete-surrounding kind (zwp delete_surrounding_text).
	ev4 := FromPlatform(platform.Event{
		Type: platform.EventIME, IMEKind: 3, IMEStart: -2, IMEEnd: 1,
	}, Modifiers{})
	if ev4.IME.Kind != IMEDeleteSurrounding || ev4.IME.Start != -2 || ev4.IME.End != 1 {
		t.Fatalf("ime delete-surrounding = %+v", ev4.IME)
	}
}

func TestKeySingleLetter(t *testing.T) {
	if KeyA == KeyB {
		t.Fatal("letters must be distinct")
	}
	if KeyZ != KeyA+25 {
		t.Fatal("letter range must be contiguous")
	}
}

func TestKeyFunct(t *testing.T) {
	if KeyF24 != KeyF1+23 {
		t.Fatal("F-key range must be contiguous")
	}
	if KeyF1.String() != "f1" || KeyF12.String() != "f12" {
		t.Fatalf("F string: %s %s", KeyF1, KeyF12)
	}
}

func TestEventKindString(t *testing.T) {
	if KindKey.String() != "key" || KindIME.String() != "ime" {
		t.Fatalf("kind string: %s %s", KindKey, KindIME)
	}
	if KindNone.String() != "none" {
		t.Fatalf("none string: %s", KindNone)
	}
}
