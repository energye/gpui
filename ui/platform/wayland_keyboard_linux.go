//go:build linux

package platform

import (
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// wl_keyboard binding via purego + libxkbcommon.
//
// The Wayland keyboard is the input channel that drives BOTH plain text
// (key presses → utf8 via the xkb keymap) and the IME: the compositor sends
// keymap / enter / leave / key / modifiers events; the client translates
// keycodes into utf8 with libxkbcommon. The zwp_text_input_v3 binding
// (wayland_textinput_linux.go) rides on the same keyboard focus: when a
// text field is focused, the IME pre-edit/commit events flow through
// zwp_text_input_v3 while the raw key events still arrive here.
//
// Protocol (wl_keyboard, stable):
//
//	requests: release(0)
//	events:   keymap(0)[format,fd,size] enter(1)[serial,surface,keys]
//	          leave(2)[serial,surface] key(3)[serial,time,key,state]
//	          modifiers(4)[serial,depressed,latched,locked,group]
//	          repeat_info(5)[rate,delay]
//
// The keymap event hands us a file descriptor to an xkb keymap (text
// format); libxkbcommon parses it and converts keycodes → keysyms → utf8.
// Wayland keycodes are evdev codes; xkb uses keycode+8.

// wl_seat request opcodes (wayland.xml authoritative order:
// get_pointer=0, get_keyboard=1, get_touch=2, release=3[v5+]).
// NOTE: these MUST match the compositor's interface table. Reversing
// them (e.g. get_keyboard=2) makes the compositor dispatch get_touch,
// and with no touch device seat->touch==NULL -> wl_list_insert crash
// in mutter (gnome-shell SEGV / session restart).
const (
	wlSeatGetPointer  = 0
	wlSeatGetKeyboard = 1
	wlSeatGetTouch    = 2
	wlSeatRelease     = 3
)

// wl_keyboard event opcodes.
const (
	wlKbKeymap    = 0
	wlKbEnter     = 1
	wlKbLeave     = 2
	wlKbKey       = 3
	wlKbModifiers = 4
	wlKbRepeat    = 5
)

// XKB_KEYMAP_FORMAT_TEXT_V1 and state actions (xkbcommon.h).
const (
	xkbKeymapFormatTextV1 = 1
	xkbKeyDown            = 1
	xkbKeyUp              = 0
)

// xkb common function table (lazy-loaded once).
type xkbFuncs struct {
	contextNew      func(flags uintptr) uintptr
	contextUnref    func(ctx uintptr)
	keymapNewFrom   func(ctx uintptr, s *byte, format, flags uintptr) uintptr
	keymapUnref     func(km uintptr)
	stateNew        func(km uintptr) uintptr
	stateUnref      func(st uintptr)
	stateKeySym     func(st uintptr, keycode uintptr) uintptr
	stateUpdateKey  func(st uintptr, keycode, action uintptr) int
	stateUpdateMask func(st uintptr, depressed, latched, locked, group, compat, append uintptr) uintptr
	keysymToUTF8    func(ks uintptr, buf *byte, size uintptr) int
}

var xkbCache *xkbFuncs

func loadXKB() *xkbFuncs {
	if xkbCache != nil {
		return xkbCache
	}
	lib, err := purego.Dlopen("libxkbcommon.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libxkbcommon.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil
	}
	f := &xkbFuncs{}
	purego.RegisterLibFunc(&f.contextNew, lib, "xkb_context_new")
	purego.RegisterLibFunc(&f.contextUnref, lib, "xkb_context_unref")
	purego.RegisterLibFunc(&f.keymapNewFrom, lib, "xkb_keymap_new_from_string")
	purego.RegisterLibFunc(&f.keymapUnref, lib, "xkb_keymap_unref")
	purego.RegisterLibFunc(&f.stateNew, lib, "xkb_state_new")
	purego.RegisterLibFunc(&f.stateUnref, lib, "xkb_state_unref")
	purego.RegisterLibFunc(&f.stateKeySym, lib, "xkb_state_key_get_one_sym")
	purego.RegisterLibFunc(&f.stateUpdateKey, lib, "xkb_state_update_key")
	purego.RegisterLibFunc(&f.stateUpdateMask, lib, "xkb_state_update_mask")
	purego.RegisterLibFunc(&f.keysymToUTF8, lib, "xkb_keysym_to_utf8")
	xkbCache = f
	return f
}

// wlKeyboardState holds the bound wl_keyboard proxy + xkb translation state.
type wlKeyboardState struct {
	lib    *wlLib
	win    *wlWin
	kbd    uintptr // wl_keyboard proxy
	xkb    *xkbFuncs
	ctx    uintptr
	keymap uintptr
	state  uintptr

	listener [6]uintptr
	selfPtr  uintptr

	// Client-side key repeat: Wayland has NO server-side autorepeat — the
	// compositor sends repeat_info(rate, delay) once and the client
	// synthesizes repeated presses while a key is held.
	repeatMu   sync.Mutex
	repRate    int         // keys per second (repeat_info; ≤0 = default)
	repDelayMs int         // ms before the first repeat (≤0 = default)
	heldKC     uintptr     // xkb keycode of the repeating key (0 = none)
	heldKS     uintptr     // its keysym
	repTimer   *time.Timer // fires the next repeat (delay first, then interval)
}

// bindKeyboard creates a wl_keyboard from the seat and adds the listener.
// Standard Wayland client behavior: enabled by default when the seat exists.
// Returns nil when the seat/interface is unavailable (silent degrade).
func (w *wlWin) bindKeyboard() *wlKeyboardState {
	if w == nil || w.lib == nil || w.seat == 0 || w.lib.ifaceKeyboard == 0 {
		return nil
	}
	st := &wlKeyboardState{lib: w.lib, win: w, xkb: loadXKB()}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	// seat.get_keyboard(new_id wl_keyboard)
	args := []wlArg{argNewID()}
	st.kbd = w.lib.proxyMarshalArrayCtor(w.seat, wlSeatGetKeyboard, &args[0], w.lib.ifaceKeyboard, 1)
	if st.kbd == 0 {
		return nil
	}
	st.listener[wlKbKeymap] = purego.NewCallback(wlKbKeymapCB)
	st.listener[wlKbEnter] = purego.NewCallback(wlKbEnterCB)
	st.listener[wlKbLeave] = purego.NewCallback(wlKbLeaveCB)
	st.listener[wlKbKey] = purego.NewCallback(wlKbKeyCB)
	st.listener[wlKbModifiers] = purego.NewCallback(wlKbModifiersCB)
	st.listener[wlKbRepeat] = purego.NewCallback(wlKbRepeatCB)
	if w.lib.proxyAddListener(st.kbd, uintptr(unsafe.Pointer(&st.listener[0])), st.selfPtr) != 0 {
		w.lib.proxyDestroy(st.kbd)
		return nil
	}
	return st
}

func (st *wlKeyboardState) destroy() {
	if st == nil {
		return
	}
	st.cancelRepeat()
	if st.state != 0 && st.xkb != nil && st.xkb.stateUnref != nil {
		st.xkb.stateUnref(st.state)
		st.state = 0
	}
	if st.keymap != 0 && st.xkb != nil && st.xkb.keymapUnref != nil {
		st.xkb.keymapUnref(st.keymap)
		st.keymap = 0
	}
	if st.ctx != 0 && st.xkb != nil && st.xkb.contextUnref != nil {
		st.xkb.contextUnref(st.ctx)
		st.ctx = 0
	}
	if st.kbd != 0 && st.lib != nil {
		st.lib.proxyDestroy(st.kbd)
		st.kbd = 0
	}
}

func kbFrom(data uintptr) *wlKeyboardState {
	if data == 0 {
		return nil
	}
	return (*wlKeyboardState)(unsafe.Pointer(data))
}

// wlKbKeymapCB handles keymap(format, fd, size): reads the xkb text keymap
// from the fd and initializes the xkb context/keymap/state.
func wlKbKeymapCB(data, kbd, format, fd, size uintptr) {
	st := kbFrom(data)
	if st == nil || st.xkb == nil {
		return
	}
	// Wayland protocol: the client owns the keymap fd and MUST close it
	// after reading (regardless of format). format 0 = XKB_KEYMAP_FORMAT_
	// NO_KEYMAP with fd==-1; only TEXT_V1 (1) carries a readable map.
	if format != xkbKeymapFormatTextV1 || fd == 0 || int32(fd) < 0 {
		if int32(fd) >= 0 {
			syscall.Close(int(fd))
		}
		return
	}
	// mmap the keymap fd (text format), parse into xkb keymap.
	buf, err := syscall.Mmap(int(fd), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil || len(buf) == 0 {
		syscall.Close(int(fd))
		return
	}
	defer syscall.Munmap(buf)
	defer syscall.Close(int(fd)) // protocol: client must close the fd
	if st.ctx == 0 && st.xkb.contextNew != nil {
		st.ctx = st.xkb.contextNew(0) // XKB_CONTEXT_NO_FLAGS
	}
	if st.ctx == 0 {
		return
	}
	// xkb_keymap_new_from_string needs a NUL-terminated string; the keymap
	// fd content is text but may not be NUL-padded. Copy into a buffer with
	// a trailing NUL to be safe.
	keymapStr := make([]byte, len(buf)+1)
	copy(keymapStr, buf)
	keymapStr[len(buf)] = 0
	km := st.xkb.keymapNewFrom(st.ctx, &keymapStr[0], xkbKeymapFormatTextV1, 0)
	if km == 0 {
		return
	}
	if st.keymap != 0 && st.xkb.keymapUnref != nil {
		st.xkb.keymapUnref(st.keymap)
	}
	st.keymap = km
	if st.state != 0 && st.xkb.stateUnref != nil {
		st.xkb.stateUnref(st.state)
	}
	if st.xkb.stateNew != nil {
		st.state = st.xkb.stateNew(km)
	}
}

// wlKbEnterCB: keyboard focus entered our surface. Per zwp_text_input_v3
// (mutter 42.9 meta-wayland-text-input.c commit_state), the compositor only
// activates the input method when a commit arrives while text_input->surface
// is set — i.e. AFTER the keyboard focus reached the surface. An enable+commit
// sent earlier (e.g. at window creation, before focus) is dropped because
// surface==NULL. So re-send the text-input state on every keyboard focus-in
// (GTK/Flutter do the same: IM context refresh on focus-in).
func wlKbEnterCB(data, kbd, serial, surface, keys uintptr) {
	st := kbFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.win.refreshTextInput()
}

func wlKbLeaveCB(data, kbd, serial, surface uintptr) {
	// Keyboard focus left while a key may still be held: it will never see
	// its release — drop any active client-side repeat or it would keep
	// synthesizing presses into the (now unfocused) queue.
	if st := kbFrom(data); st != nil {
		st.cancelRepeat()
	}
}

// wlKbRepeatCB handles repeat_info(rate, delay): rate in keys per second,
// delay in milliseconds before the first repeat. Delivered once after bind
// and again on change; defaults apply until then.
func wlKbRepeatCB(data, kbd, rate, delay uintptr) {
	st := kbFrom(data)
	if st == nil {
		return
	}
	st.repeatMu.Lock()
	st.repRate = int(int32(rate))
	st.repDelayMs = int(int32(delay))
	st.repeatMu.Unlock()
}

// wlKbKeyCB handles key(serial, time, keycode, state): translates the
// Wayland keycode into a keysym + utf8 via xkb and pushes a platform key
// event.
func wlKbKeyCB(data, kbd, serial, time, key, state uintptr) {
	st := kbFrom(data)
	if st == nil || st.xkb == nil || st.state == 0 {
		return
	}
	st.win.lastSerial.Store(uint32(serial))
	// Wayland sends evdev keycodes; xkbcommon expects X11 keycodes
	// (evdev + 8) — see XKBEvdevOffset in gogpu internal/platform/xkb.
	kc := (key & 0xffff) + 8
	// state: 0 = released, 1 = pressed.
	action := uintptr(xkbKeyUp)
	if state&0xff == 1 {
		action = xkbKeyDown
	}
	if st.xkb.stateUpdateKey != nil {
		st.xkb.stateUpdateKey(st.state, kc, action)
	}
	// keysym → utf8.
	ks := uintptr(0)
	if st.xkb.stateKeySym != nil {
		ks = st.xkb.stateKeySym(st.state, kc)
	}
	ev := st.keysymEvent(ks, state&0xff == 1)
	st.win.pushKey(ev)
	// Client-side repeat (Wayland has no server autorepeat): a non-modifier
	// press becomes the repeating key; its release cancels. A modifier
	// press/release leaves an ongoing repeat untouched.
	if state&0xff == 1 {
		st.armRepeat(kc, ks)
	} else {
		st.cancelRepeatIf(kc)
	}
}

func wlKbModifiersCB(data, kbd, serial, depressed, latched, locked, group uintptr) {
	st := kbFrom(data)
	if st == nil || st.xkb == nil || st.state == 0 || st.xkb.stateUpdateMask == nil {
		return
	}
	st.xkb.stateUpdateMask(st.state, depressed, latched, locked, group, 0, 0)
}

// decodeRune decodes the first UTF-8 rune (returns its size).
func decodeRune(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0, 0
	}
	// Minimal UTF-8 decode (1–4 bytes).
	c := b[0]
	switch {
	case c < 0x80:
		return rune(c), 1
	case c < 0xE0 && len(b) >= 2:
		return rune(c&0x1F)<<6 | rune(b[1]&0x3F), 2
	case c < 0xF0 && len(b) >= 3:
		return rune(c&0x0F)<<12 | rune(b[1]&0x3F)<<6 | rune(b[2]&0x3F), 3
	case len(b) >= 4:
		return rune(c&0x07)<<18 | rune(b[1]&0x3F)<<12 | rune(b[2]&0x3F)<<6 | rune(b[3]&0x3F), 4
	}
	return 0, 0
}

func runeAt(b []byte) rune {
	r, _ := decodeRune(b)
	return r
}

// --- client-side key repeat (Wayland: no server autorepeat) ---

// Fallback cadence until repeat_info arrives (matches common desktop
// defaults); also used when a compositor sends zeros.
const (
	wlRepeatDefaultRate  = 25  // keys per second
	wlRepeatDefaultDelay = 400 // ms before the first repeat
)

// wlModifierKeysyms never auto-repeat: holding Shift must not spam Shift.
var wlModifierKeysyms = map[uintptr]bool{
	0xffe1: true, 0xffe2: true, // Shift L/R
	0xffe3: true, 0xffe4: true, // Control L/R
	0xffe5: true,               // Caps Lock
	0xffe6: true,               // Shift Lock
	0xffe7: true, 0xffe8: true, // Meta L/R
	0xffe9: true, 0xffea: true, // Alt L/R
	0xffeb: true, 0xffec: true, // Super L/R
	0xff13: true, // Scroll Lock
	0xff7f: true, // Num Lock
}

func isModifierKeysym(ks uintptr) bool { return wlModifierKeysyms[ks] }

// keysymEvent builds a platform key event for a keysym; the press path and
// synthesized repeats share it so both carry the same Rune derivation.
func (st *wlKeyboardState) keysymEvent(ks uintptr, pressed bool) Event {
	ev := Event{Type: EventKey, Pressed: pressed, KeyCode: int(ks)}
	if st.xkb != nil && st.xkb.keysymToUTF8 != nil && ks != 0 {
		var buf [16]byte
		n := st.xkb.keysymToUTF8(ks, &buf[0], uintptr(len(buf)))
		if n > 0 && int(n) <= len(buf) {
			// Printable text: surface as a text rune (Rune carries the first
			// utf8 rune; multi-rune IME output goes through zwp_text_input_v3).
			ev.Rune = runeAt(buf[:n])
		}
	}
	return ev
}

// armRepeat makes kc the repeating key (last non-modifier press wins — an
// earlier held key is replaced). No-op for modifier/unrecognized keysyms.
func (st *wlKeyboardState) armRepeat(kc, ks uintptr) {
	if ks == 0 || isModifierKeysym(ks) {
		return
	}
	st.repeatMu.Lock()
	defer st.repeatMu.Unlock()
	if st.heldKC == kc && st.repTimer != nil {
		return // duplicate press of the already-repeating key
	}
	if st.repTimer != nil {
		st.repTimer.Stop()
	}
	st.heldKC, st.heldKS = kc, ks
	st.repTimer = time.AfterFunc(st.repeatDelayLocked(), st.fireRepeat)
}

func (st *wlKeyboardState) repeatDelayLocked() time.Duration {
	d := st.repDelayMs
	if d <= 0 {
		d = wlRepeatDefaultDelay
	}
	return time.Duration(d) * time.Millisecond
}

func (st *wlKeyboardState) repeatIntervalLocked() time.Duration {
	r := st.repRate
	if r <= 0 {
		r = wlRepeatDefaultRate
	}
	return time.Duration(1000/r) * time.Millisecond
}

// fireRepeat pushes one synthesized press and reschedules while the key is
// still held. Runs on the timer goroutine; pushKey is mutex-guarded.
func (st *wlKeyboardState) fireRepeat() {
	st.repeatMu.Lock()
	defer st.repeatMu.Unlock()
	if st.heldKC == 0 || st.win == nil {
		return // released / focused out since armed
	}
	st.win.pushKey(st.keysymEvent(st.heldKS, true))
	st.repTimer = time.AfterFunc(st.repeatIntervalLocked(), st.fireRepeat)
}

// cancelRepeatIf stops repetition only when kc IS the repeating key; a
// different key's release (e.g. a tapped modifier) must not stop it.
func (st *wlKeyboardState) cancelRepeatIf(kc uintptr) {
	st.repeatMu.Lock()
	defer st.repeatMu.Unlock()
	if st.heldKC != kc {
		return
	}
	st.cancelRepeatLocked()
}

// cancelRepeat unconditionally stops repetition (focus lost, destroy).
func (st *wlKeyboardState) cancelRepeat() {
	st.repeatMu.Lock()
	defer st.repeatMu.Unlock()
	st.cancelRepeatLocked()
}

func (st *wlKeyboardState) cancelRepeatLocked() {
	if st.repTimer != nil {
		st.repTimer.Stop()
		st.repTimer = nil
	}
	st.heldKC, st.heldKS = 0, 0
}
