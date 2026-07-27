//go:build linux && !nouiplatform

package platform

import (
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Wayland seat input (wl_seat / wl_pointer / wl_keyboard) via purego.
//
// Design:
//   - Seat is optional: missing global or zero capabilities is fine.
//   - Capabilities can appear later (hotplug); we get/release pointer & keyboard
//     dynamically — "no peripheral events" is a normal idle state.
//   - Events are enqueued under host.mu and drained by poll().

// Seat capability bits (wayland.xml). Touch not wired yet.
const (
	wlSeatCapPointer  = 1 << 0
	wlSeatCapKeyboard = 1 << 1
)

// Seat / pointer / keyboard request opcodes.
const (
	wlSeatGetPointer  = 0
	wlSeatGetKeyboard = 1
	wlSeatRelease     = 3
	wlPointerRelease  = 1
	wlKeyboardRelease = 0
)

// Pointer / keyboard event state enums.
const (
	wlPointerBtnReleased    = 0
	wlPointerBtnPressed     = 1
	wlPointerAxisVertical   = 0
	wlPointerAxisHorizontal = 1
	wlKeyReleased           = 0
	wlKeyPressed            = 1
)

// Linux evdev button codes (linux/input-event-codes.h).
const (
	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
)

// Common Linux evdev KEY_* codes used by wl_keyboard.key.
const (
	keyEsc        = 1
	key1          = 2
	keyTab        = 15
	keyBackspace  = 14
	keyEnter      = 28
	keyLeftCtrl   = 29
	keyA          = 30
	keyLeftShift  = 42
	keyRightShift = 54
	keyLeftAlt    = 56
	keySpace      = 57
	keyRightCtrl  = 97
	keyRightAlt   = 100
	keyHome       = 102
	keyUp         = 103
	keyPageUp     = 104
	keyLeft       = 105
	keyRight      = 106
	keyEnd        = 107
	keyDown       = 108
	keyPageDown   = 109
	keyDelete     = 111
	keyLeftMeta   = 125
	keyRightMeta  = 126
)

// seat fields live on WaylandHost (see wayland_host_linux.go).

func (h *WaylandHost) setupSeat(reg uintptr) {
	if h == nil || h.seatName == 0 || h.lib == nil || h.lib.ifaceSeat == 0 {
		return
	}
	ver := minU32(h.seatVer, 5)
	if ver < 1 {
		ver = 1
	}
	h.seat = h.bind(reg, h.seatName, h.lib.ifaceSeat, ver)
	if h.seat == 0 {
		return
	}
	// Listener: capabilities(0), name(1). name is optional noise.
	h.seatListener[0] = purego.NewCallback(wlSeatCapabilities)
	h.seatListener[1] = purego.NewCallback(wlSeatName)
	h.lib.proxyAddListener(h.seat, uintptr(unsafe.Pointer(&h.seatListener[0])), h.selfPtr)
	// Roundtrip so initial capabilities arrive before first present (if any).
	// Safe when there are no peripherals: capabilities=0, no get_pointer.
	if h.lib.displayRoundtrip != nil && h.display != 0 {
		h.lib.displayRoundtrip(h.display)
	}
}

func (h *WaylandHost) destroySeat() {
	if h == nil || h.lib == nil {
		return
	}
	h.releasePointer()
	h.releaseKeyboard()
	if h.seat != 0 {
		// release() only exists since v5; destroy proxy always.
		if h.seatVer >= 5 {
			h.lib.proxyMarshalArrayFlags(h.seat, wlSeatRelease, 0, 0, 0, nil)
		}
		h.lib.proxyDestroy(h.seat)
		h.seat = 0
	}
}

func (h *WaylandHost) releasePointer() {
	if h.pointer != 0 && h.lib != nil {
		h.lib.proxyMarshalArrayFlags(h.pointer, wlPointerRelease, 0, 0, 0, nil)
		h.lib.proxyDestroy(h.pointer)
		h.pointer = 0
	}
}

func (h *WaylandHost) releaseKeyboard() {
	if h.keyboard != 0 && h.lib != nil {
		h.lib.proxyMarshalArrayFlags(h.keyboard, wlKeyboardRelease, 0, 0, 0, nil)
		h.lib.proxyDestroy(h.keyboard)
		h.keyboard = 0
	}
}

func (h *WaylandHost) applySeatCapabilities(caps uint32) {
	if h == nil || h.lib == nil || h.seat == 0 {
		return
	}
	// Pointer
	wantPtr := caps&wlSeatCapPointer != 0
	if wantPtr && h.pointer == 0 && h.lib.ifacePointer != 0 {
		h.pointer = h.ctor(h.seat, wlSeatGetPointer, h.lib.ifacePointer, minU32(h.seatVer, 5))
		if h.pointer != 0 {
			// Fill no-ops first so EventCount from lib iface (v7–v9) is safe.
			noop := purego.NewCallback(wlListenerNoop)
			for i := range h.ptrListener {
				h.ptrListener[i] = noop
			}
			h.ptrListener[0] = purego.NewCallback(wlPointerEnter)
			h.ptrListener[1] = purego.NewCallback(wlPointerLeave)
			h.ptrListener[2] = purego.NewCallback(wlPointerMotion)
			h.ptrListener[3] = purego.NewCallback(wlPointerButton)
			h.ptrListener[4] = purego.NewCallback(wlPointerAxis)
			h.ptrListener[5] = purego.NewCallback(wlPointerFrame)
			h.ptrListener[6] = purego.NewCallback(wlPointerAxisSource)
			h.ptrListener[7] = purego.NewCallback(wlPointerAxisStop)
			h.ptrListener[8] = purego.NewCallback(wlPointerAxisDiscrete)
			h.lib.proxyAddListener(h.pointer, uintptr(unsafe.Pointer(&h.ptrListener[0])), h.selfPtr)
		}
	} else if !wantPtr && h.pointer != 0 {
		h.releasePointer()
	}

	// Keyboard
	wantKey := caps&wlSeatCapKeyboard != 0
	if wantKey && h.keyboard == 0 && h.lib.ifaceKeyboard != 0 {
		h.keyboard = h.ctor(h.seat, wlSeatGetKeyboard, h.lib.ifaceKeyboard, minU32(h.seatVer, 5))
		if h.keyboard != 0 {
			noop := purego.NewCallback(wlListenerNoop)
			for i := range h.keyListener {
				h.keyListener[i] = noop
			}
			h.keyListener[0] = purego.NewCallback(wlKeyboardKeymap)
			h.keyListener[1] = purego.NewCallback(wlKeyboardEnter)
			h.keyListener[2] = purego.NewCallback(wlKeyboardLeave)
			h.keyListener[3] = purego.NewCallback(wlKeyboardKey)
			h.keyListener[4] = purego.NewCallback(wlKeyboardModifiers)
			h.keyListener[5] = purego.NewCallback(wlKeyboardRepeatInfo)
			h.lib.proxyAddListener(h.keyboard, uintptr(unsafe.Pointer(&h.keyListener[0])), h.selfPtr)
		}
	} else if !wantKey && h.keyboard != 0 {
		h.releaseKeyboard()
	}
}

func (h *WaylandHost) enqueue(ev Event) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.queue = append(h.queue, ev)
	h.mu.Unlock()
	h.WakeUp()
}

func (h *WaylandHost) mods() (shift, ctrl, alt, meta bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.modShift, h.modCtrl, h.modAlt, h.modMeta
}

func (h *WaylandHost) ptrXY() (x, y float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ptrX, h.ptrY
}

// --- seat callbacks ---

func wlSeatCapabilities(data, seat, capabilities uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	h.applySeatCapabilities(uint32(capabilities))
	_ = seat
}

func wlSeatName(data, seat, name uintptr) {
	// Optional; ignore seat name string.
	_ = data
	_ = seat
	_ = name
}

// --- pointer callbacks ---

func wlFixedToFloat(fixed uintptr) float64 {
	// wl_fixed_t is 24.8 signed.
	return float64(int32(fixed)) / 256.0
}

func wlPointerEnter(data, pointer, serial, surface, surfaceX, surfaceY uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	x, y := wlFixedToFloat(surfaceX), wlFixedToFloat(surfaceY)
	h.mu.Lock()
	h.ptrX, h.ptrY = x, y
	h.mu.Unlock()
	sh, ctrl, alt, meta := h.mods()
	h.enqueue(Event{Type: EventFocus, Focused: true})
	h.enqueue(Event{
		Type: EventPointer, Pointer: PointerMove, X: x, Y: y,
		Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
	})
	_ = pointer
	_ = serial
	_ = surface
}

func wlPointerLeave(data, pointer, serial, surface uintptr) {
	// Leave alone does not always mean window focus loss (keyboard may remain).
	_ = data
	_ = pointer
	_ = serial
	_ = surface
}

func wlPointerMotion(data, pointer, time, surfaceX, surfaceY uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	x, y := wlFixedToFloat(surfaceX), wlFixedToFloat(surfaceY)
	h.mu.Lock()
	h.ptrX, h.ptrY = x, y
	h.mu.Unlock()
	sh, ctrl, alt, meta := h.mods()
	h.enqueue(Event{
		Type: EventPointer, Pointer: PointerMove, X: x, Y: y,
		Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
	})
	_ = pointer
	_ = time
}

func wlPointerButton(data, pointer, serial, time, button, state uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	btn := BtnNone
	switch uint32(button) {
	case btnLeft:
		btn = BtnLeft
	case btnMiddle:
		btn = BtnMiddle
	case btnRight:
		btn = BtnRight
	default:
		// Unknown side/extra buttons: still deliver as left-ish for hit tests? skip.
		return
	}
	kind := PointerUp
	if uint32(state) == wlPointerBtnPressed {
		kind = PointerDown
	}
	x, y := h.ptrXY()
	sh, ctrl, alt, meta := h.mods()
	h.enqueue(Event{
		Type: EventPointer, Pointer: kind, X: x, Y: y, Button: btn,
		Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
	})
	_ = pointer
	_ = serial
	_ = time
}

func wlPointerAxis(data, pointer, time, axis, value uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	// value is wl_fixed; positive vertical = scroll down (match X11 Button5).
	delta := wlFixedToFloat(value)
	// Scale fixed units (~1.0 per tick-ish) to pixel-ish steps like X11 host.
	const step = 48.0
	dx, dy := 0.0, 0.0
	switch uint32(axis) {
	case wlPointerAxisVertical:
		dy = delta // already signed
		// compositor often sends ~10 fixed units per notch; normalize softly
		if dy > 0 && dy < 1 {
			dy = step
		} else if dy < 0 && dy > -1 {
			dy = -step
		} else {
			dy = dy * (step / 10.0)
		}
	case wlPointerAxisHorizontal:
		dx = delta
		if dx > 0 && dx < 1 {
			dx = step
		} else if dx < 0 && dx > -1 {
			dx = -step
		} else {
			dx = dx * (step / 10.0)
		}
	default:
		return
	}
	x, y := h.ptrXY()
	sh, ctrl, alt, meta := h.mods()
	h.enqueue(Event{
		Type: EventScroll, X: x, Y: y, ScrollDX: dx, ScrollDY: dy,
		Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
	})
	_ = pointer
	_ = time
}

func wlPointerFrame(data, pointer uintptr)              {}
func wlPointerAxisSource(data, pointer, source uintptr) {}
func wlPointerAxisStop(data, pointer, time, axis uintptr) {
}
func wlPointerAxisDiscrete(data, pointer, axis, discrete uintptr) {}

// wlListenerNoop absorbs unknown / unused seat family events (extra args ignored
// by the SysV calling convention when the Go func takes fewer parameters —
// purego still requires a concrete func; use a wide signature).
func wlListenerNoop(a, b, c, d, e, f, g, h uintptr) {
	_, _, _, _, _, _, _, _ = a, b, c, d, e, f, g, h
}

// --- keyboard callbacks ---

func wlKeyboardKeymap(data, keyboard, format, fd, size uintptr) {
	// Must close the keymap fd even if we do not parse xkb (no CapIME / no xkbcommon yet).
	if fd != 0 && int(fd) >= 0 {
		_ = syscall.Close(int(fd))
	}
	_ = data
	_ = keyboard
	_ = format
	_ = size
}

func wlKeyboardEnter(data, keyboard, serial, surface, keys uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	h.enqueue(Event{Type: EventFocus, Focused: true})
	_ = keyboard
	_ = serial
	_ = surface
	_ = keys // wl_array* of currently pressed keys — ignore for now
}

func wlKeyboardLeave(data, keyboard, serial, surface uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	h.mu.Lock()
	h.modShift, h.modCtrl, h.modAlt, h.modMeta = false, false, false, false
	h.mu.Unlock()
	h.enqueue(Event{Type: EventFocus, Focused: false})
	_ = keyboard
	_ = serial
	_ = surface
}

func wlKeyboardKey(data, keyboard, serial, time, key, state uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	code := uint32(key) // evdev KEY_* (not X keycode)
	down := uint32(state) == wlKeyPressed

	// Track modifiers from key itself (no xkb).
	h.mu.Lock()
	switch code {
	case keyLeftShift, keyRightShift:
		h.modShift = down
	case keyLeftCtrl, keyRightCtrl:
		h.modCtrl = down
	case keyLeftAlt, keyRightAlt:
		h.modAlt = down
	case keyLeftMeta, keyRightMeta:
		h.modMeta = down
	}
	sh, ctrl, alt, meta := h.modShift, h.modCtrl, h.modAlt, h.modMeta
	h.mu.Unlock()

	name, text := wlKeyName(code, sh)
	if name == "" && text == "" {
		return
	}
	h.enqueue(Event{
		Type: EventKey, Key: name, Text: text, Down: down,
		Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
	})
	if down && text != "" && !ctrl && !alt && !meta {
		h.enqueue(Event{Type: EventText, Text: text})
	}
	_ = keyboard
	_ = serial
	_ = time
}

func wlKeyboardModifiers(data, keyboard, serial, depressed, latched, locked, group uintptr) {
	// Without xkb we cannot interpret the mask reliably; key-based tracking above is primary.
	_ = data
	_ = keyboard
	_ = serial
	_ = depressed
	_ = latched
	_ = locked
	_ = group
}

func wlKeyboardRepeatInfo(data, keyboard, rate, delay uintptr) {}

// wlKeyName maps evdev KEY_* to core-friendly names (+ optional Latin text).
// US-QWERTY only; full layouts need libxkbcommon (future CapIME path).
func wlKeyName(code uint32, shift bool) (key, text string) {
	switch code {
	case keyEsc:
		return "Escape", ""
	case keyTab:
		return "Tab", ""
	case keyBackspace:
		return "Backspace", ""
	case keyEnter:
		return "Enter", ""
	case keySpace:
		return " ", " "
	case keyLeft:
		return "Left", ""
	case keyRight:
		return "Right", ""
	case keyUp:
		return "Up", ""
	case keyDown:
		return "Down", ""
	case keyHome:
		return "Home", ""
	case keyEnd:
		return "End", ""
	case keyDelete:
		return "Delete", ""
	case keyPageUp:
		return "PageUp", ""
	case keyPageDown:
		return "PageDown", ""
	case keyLeftShift, keyRightShift:
		return "Shift", ""
	case keyLeftCtrl, keyRightCtrl:
		return "Control", ""
	case keyLeftAlt, keyRightAlt:
		return "Alt", ""
	case keyLeftMeta, keyRightMeta:
		return "Meta", ""
	}

	// Digits row KEY_1..KEY_0 = 2..11
	if code >= key1 && code <= 11 {
		digits := "1234567890"
		shifted := "!@#$%^&*()"
		i := code - key1
		if shift {
			return string(shifted[i]), string(shifted[i])
		}
		return string(digits[i]), string(digits[i])
	}

	// Letters KEY_A..KEY_Z scattered — use a small table of (code → base rune).
	if r, ok := wlLetter[code]; ok {
		if shift {
			u := r - 'a' + 'A'
			return string(u), string(u)
		}
		return string(r), string(r)
	}

	// Punctuation (US)
	if r, ok := wlPunct[code]; ok {
		if shift {
			if s, ok2 := wlPunctShift[code]; ok2 {
				return s, s
			}
		}
		return r, r
	}
	return "", ""
}

var wlLetter = map[uint32]rune{
	16: 'q', 17: 'w', 18: 'e', 19: 'r', 20: 't', 21: 'y', 22: 'u', 23: 'i', 24: 'o', 25: 'p',
	30: 'a', 31: 's', 32: 'd', 33: 'f', 34: 'g', 35: 'h', 36: 'j', 37: 'k', 38: 'l',
	44: 'z', 45: 'x', 46: 'c', 47: 'v', 48: 'b', 49: 'n', 50: 'm',
}

var wlPunct = map[uint32]string{
	12: "-", 13: "=", 26: "[", 27: "]", 39: ";", 40: "'", 41: "`", 43: "\\", 51: ",", 52: ".", 53: "/",
}

var wlPunctShift = map[uint32]string{
	12: "_", 13: "+", 26: "{", 27: "}", 39: ":", 40: "\"", 41: "~", 43: "|", 51: "<", 52: ">", 53: "?",
}
