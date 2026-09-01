//go:build linux

package platform

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	xSelectionClear   = 29
	xSelectionRequest = 30
	xSelectionNotify  = 31
	xPropertyNotify   = 28
	xPropModeReplace  = 0
	xCurrentTime      = 0
	clipGetTimeout    = 500 * time.Millisecond
	clipPollInterval  = 10 * time.Millisecond
	clipIncrTimeout   = 500 * time.Millisecond
)

type x11ClipboardLib struct {
	once sync.Once
	lib  uintptr
	internAtom       func(dpy uintptr, name *byte, onlyIf int) uintptr
	setSelectionOwner func(dpy uintptr, sel uintptr, win uintptr, t uintptr) int
	getSelectionOwner func(dpy uintptr, sel uintptr) uintptr
	convertSelection func(dpy uintptr, sel uintptr, target uintptr, prop uintptr, win uintptr, t uintptr) int
	getWindowProperty func(dpy uintptr, w, prop uintptr, longOff, longLen int64, del int, reqType uintptr, actualType *uintptr, actualFormat *int, nitems, bytesAfter *uint64, propRet **byte) int
	changeProperty   func(dpy uintptr, w, prop, typ uintptr, format, mode int, data unsafe.Pointer, nelems int) int
	deleteProperty   func(dpy uintptr, w uintptr, prop uintptr) int
	sendEvent        func(dpy uintptr, w uintptr, propagate int, mask int64, ev *byte) int
	flush            func(dpy uintptr) int
	freeData         func(ptr unsafe.Pointer) int
}

var clipLib x11ClipboardLib

func (l *x11ClipboardLib) open() *x11ClipboardLib {
	l.once.Do(func() {
		lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		}
		if err != nil {
			return
		}
		l.lib = lib
		purego.RegisterLibFunc(&l.internAtom, lib, "XInternAtom")
		purego.RegisterLibFunc(&l.setSelectionOwner, lib, "XSetSelectionOwner")
		purego.RegisterLibFunc(&l.getSelectionOwner, lib, "XGetSelectionOwner")
		purego.RegisterLibFunc(&l.convertSelection, lib, "XConvertSelection")
		purego.RegisterLibFunc(&l.getWindowProperty, lib, "XGetWindowProperty")
		purego.RegisterLibFunc(&l.changeProperty, lib, "XChangeProperty")
		purego.RegisterLibFunc(&l.deleteProperty, lib, "XDeleteProperty")
		purego.RegisterLibFunc(&l.sendEvent, lib, "XSendEvent")
		purego.RegisterLibFunc(&l.flush, lib, "XFlush")
		purego.RegisterLibFunc(&l.freeData, lib, "XFree")
	})
	return l
}

func (l *x11ClipboardLib) ok() bool { return l != nil && l.lib != 0 && l.internAtom != nil }

// globalAtoms caches interned atoms per process (server-global).
var globalAtoms struct {
	once  sync.Once
	mu    sync.Mutex
	cache map[string]uintptr
}

func internAtomCached(dpy uintptr, name string) uintptr {
	globalAtoms.mu.Lock()
	if globalAtoms.cache == nil {
		globalAtoms.cache = make(map[string]uintptr)
	}
	if v, ok := globalAtoms.cache[name]; ok {
		globalAtoms.mu.Unlock()
		return v
	}
	globalAtoms.mu.Unlock()
	lib := clipLib.open()
	if !lib.ok() {
		return 0
	}
	b := append([]byte(name), 0)
	v := lib.internAtom(dpy, &b[0], 0)
	if v != 0 {
		globalAtoms.mu.Lock()
		globalAtoms.cache[name] = v
		globalAtoms.mu.Unlock()
	}
	return v
}

type x11Clipboard struct {
	host *x11Host
	mu   sync.RWMutex
	atoms struct {
		clipboard  uintptr
		targets    uintptr
		utf8String uintptr
		utf8Mime   uintptr
		textPlain  uintptr
		stringAtom uintptr
		textAtom   uintptr
		incr       uintptr
		atomAtom   uintptr
		prop       uintptr
		once       sync.Once
	}
	ownData string
	ownKind string
	pendingMu   sync.Mutex
	pendingData string
	pendingErr  error
	pendingDone chan struct{}
}

func clipForX11(host *x11Host) Clipboard {
	if host == nil || host.st == nil || host.st.display == 0 || host.st.window == 0 {
		return FallbackClipboard()
	}
	return &x11Clipboard{host: host}
}

func (c *x11Clipboard) ensureAtoms() {
	c.atoms.once.Do(func() {
		if c.host == nil || c.host.st == nil || c.host.st.display == 0 {
			return
		}
		dpy := c.host.st.display
		c.atoms.clipboard = internAtomCached(dpy, "CLIPBOARD")
		c.atoms.targets = internAtomCached(dpy, "TARGETS")
		c.atoms.utf8String = internAtomCached(dpy, "UTF8_STRING")
		c.atoms.stringAtom = internAtomCached(dpy, "STRING")
		c.atoms.textAtom = internAtomCached(dpy, "TEXT")
		c.atoms.incr = internAtomCached(dpy, "INCR")
		c.atoms.atomAtom = internAtomCached(dpy, "ATOM")
		c.atoms.textPlain = internAtomCached(dpy, "text/plain")
		c.atoms.utf8Mime = internAtomCached(dpy, "text/plain;charset=utf-8")
		c.atoms.prop = internAtomCached(dpy, "GPUI_CLIPBOARD")
	})
}

func (c *x11Clipboard) isOwner() bool {
	c.ensureAtoms()
	lib := clipLib.open()
	if !lib.ok() || c.host == nil || c.host.st == nil || c.atoms.clipboard == 0 {
		return false
	}
	owner := lib.getSelectionOwner(c.host.st.display, c.atoms.clipboard)
	return owner != 0 && owner == c.host.st.window
}

func (c *x11Clipboard) Set(kind, data string) error {
	if kind == "" {
		kind = "text/plain"
	}
	c.ensureAtoms()
	lib := clipLib.open()
	if !lib.ok() || c.host == nil || c.host.st == nil || c.host.st.display == 0 || c.host.st.window == 0 || c.atoms.clipboard == 0 {
		return FallbackClipboard().Set(kind, data)
	}
	c.mu.Lock()
	c.ownData = data
	c.ownKind = kind
	c.mu.Unlock()
	lib.setSelectionOwner(c.host.st.display, c.atoms.clipboard, c.host.st.window, xCurrentTime)
	if lib.flush != nil {
		lib.flush(c.host.st.display)
	}
	_ = FallbackClipboard().Set(kind, data)
	return nil
}

func (c *x11Clipboard) Get(kind string) (string, error) {
	if kind == "" {
		kind = "text/plain"
	}
	c.ensureAtoms()
	lib := clipLib.open()
	if c.isOwner() {
		c.mu.RLock()
		d := c.ownData
		c.mu.RUnlock()
		if d != "" {
			return d, nil
		}
	}
	if !lib.ok() || c.host == nil || c.host.st == nil || c.host.st.display == 0 || c.atoms.clipboard == 0 || c.atoms.prop == 0 {
		return FallbackClipboard().Get(kind)
	}
	target := c.atoms.utf8String
	if target == 0 {
		target = c.atoms.stringAtom
	}
	if target == 0 {
		return FallbackClipboard().Get(kind)
	}
	c.pendingMu.Lock()
	c.pendingData = ""
	c.pendingErr = nil
	done := make(chan struct{})
	c.pendingDone = done
	c.pendingMu.Unlock()

	lib.convertSelection(c.host.st.display, c.atoms.clipboard, target, c.atoms.prop, c.host.st.window, xCurrentTime)
	if lib.flush != nil {
		lib.flush(c.host.st.display)
	}

	timer := time.NewTimer(clipGetTimeout)
	ticker := time.NewTicker(clipPollInterval)
	defer timer.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-done:
			c.pendingMu.Lock()
			d := c.pendingData
			err := c.pendingErr
			c.pendingDone = nil
			c.pendingMu.Unlock()
			if err != nil {
				if fb, fbErr := FallbackClipboard().Get(kind); fbErr == nil && fb != "" {
					return fb, nil
				}
				return "", err
			}
			if d == "" {
				if fb, fbErr := FallbackClipboard().Get(kind); fbErr == nil && fb != "" {
					return fb, nil
				}
				return "", fmt.Errorf("x11: clipboard is empty")
			}
			return d, nil
		case <-timer.C:
			c.pendingMu.Lock()
			c.pendingDone = nil
			c.pendingMu.Unlock()
			if fb, fbErr := FallbackClipboard().Get(kind); fbErr == nil && fb != "" {
				return fb, nil
			}
			return "", fmt.Errorf("x11: clipboard timeout")
		case <-ticker.C:
			if c.host != nil && c.host.st != nil && c.host.st.pending != nil && c.host.st.pending() > 0 {
				_ = c.host.drainX()
			}
		}
	}
}

func (c *x11Clipboard) handleSelectionRequest(buf []byte) {
	c.ensureAtoms()
	lib := clipLib.open()
	if !lib.ok() || c.host == nil || c.host.st == nil {
		return
	}
	requestor := uintptr(readU64(buf, 40))
	selection := uintptr(readU64(buf, 48))
	target := uintptr(readU64(buf, 56))
	property := uintptr(readU64(buf, 64))
	if selection != c.atoms.clipboard {
		return
	}
	if property == 0 {
		property = target
	}
	display := c.host.st.display
	c.mu.RLock()
	own := c.ownData
	c.mu.RUnlock()

	switch {
	case target == c.atoms.targets:
		replyType := c.atoms.atomAtom
		atoms := []uintptr{c.atoms.targets, c.atoms.utf8String, c.atoms.stringAtom, c.atoms.textAtom}
		var filtered []uintptr
		for _, a := range atoms {
			if a != 0 {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) > 0 {
			lib.changeProperty(display, requestor, property, replyType, 32, xPropModeReplace, unsafe.Pointer(&filtered[0]), len(filtered))
		} else {
			lib.changeProperty(display, requestor, property, replyType, 32, xPropModeReplace, nil, 0)
		}
		c.sendSelectionNotify(requestor, selection, target, property)
		return
	case target == c.atoms.utf8String || target == c.atoms.stringAtom || target == c.atoms.textAtom || target == c.atoms.textPlain || target == c.atoms.utf8Mime:
		data := []byte(own)
		if len(data) > 0 {
			lib.changeProperty(display, requestor, property, target, 8, xPropModeReplace, unsafe.Pointer(&data[0]), len(data))
		} else {
			lib.changeProperty(display, requestor, property, target, 8, xPropModeReplace, nil, 0)
		}
		if lib.flush != nil {
			lib.flush(display)
		}
		c.sendSelectionNotify(requestor, selection, target, property)
		return
	default:
		c.sendSelectionNotify(requestor, selection, target, 0)
	}
}

func (c *x11Clipboard) sendSelectionNotify(requestor, selection, target, property uintptr) {
	lib := clipLib.open()
	if !lib.ok() || c.host == nil || c.host.st == nil {
		return
	}
	var ev [192]byte
	*(*int32)(unsafe.Pointer(&ev[0])) = int32(xSelectionNotify)
	*(*uintptr)(unsafe.Pointer(&ev[24])) = c.host.st.display
	*(*uintptr)(unsafe.Pointer(&ev[32])) = requestor
	*(*uintptr)(unsafe.Pointer(&ev[40])) = selection
	*(*uintptr)(unsafe.Pointer(&ev[48])) = target
	*(*uintptr)(unsafe.Pointer(&ev[56])) = property
	*(*uintptr)(unsafe.Pointer(&ev[64])) = xCurrentTime
	lib.sendEvent(c.host.st.display, requestor, 0, 0, &ev[0])
	if lib.flush != nil {
		lib.flush(c.host.st.display)
	}
}

func (c *x11Clipboard) handleSelectionNotify(buf []byte) {
	c.ensureAtoms()
	lib := clipLib.open()
	if !lib.ok() || c.host == nil || c.host.st == nil {
		return
	}
	selection := uintptr(readU64(buf, 40))
	property := uintptr(readU64(buf, 56))
	if selection != c.atoms.clipboard {
		return
	}
	if property == 0 {
		c.pendingMu.Lock()
		if done := c.pendingDone; done != nil {
			c.pendingErr = fmt.Errorf("x11: clipboard conversion failed")
			closeDone(done)
		}
		c.pendingMu.Unlock()
		return
	}
	data, err := c.readProperty(property)
	if lib.deleteProperty != nil && c.atoms.prop != 0 {
		lib.deleteProperty(c.host.st.display, c.host.st.window, c.atoms.prop)
	}
	c.pendingMu.Lock()
	if done := c.pendingDone; done != nil {
		if err != nil {
			c.pendingErr = err
		} else {
			c.pendingData = string(data)
		}
		closeDone(done)
	}
	c.pendingMu.Unlock()
}

func closeDone(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func (c *x11Clipboard) handleSelectionClear(buf []byte) {
	c.ensureAtoms()
	selection := uintptr(readU64(buf, 40))
	if selection != c.atoms.clipboard {
		return
	}
	c.mu.Lock()
	c.ownData = ""
	c.ownKind = ""
	c.mu.Unlock()
}

func (c *x11Clipboard) readProperty(prop uintptr) ([]byte, error) {
	lib := clipLib.open()
	if !lib.ok() || c.host == nil || c.host.st == nil {
		return nil, fmt.Errorf("x11: clipboard lib not ready")
	}
	display := c.host.st.display
	win := c.host.st.window
	var actualType uintptr
	var actualFormat int
	var nitems, bytesAfter uint64
	var propRet *byte
	ret := lib.getWindowProperty(display, win, prop, 0, 1<<20, 0, 0, &actualType, &actualFormat, &nitems, &bytesAfter, &propRet)
	if ret != 0 {
		return nil, fmt.Errorf("x11: GetWindowProperty failed %d", ret)
	}
	if propRet == nil {
		return nil, fmt.Errorf("x11: clipboard is empty")
	}
	defer func() {
		if lib.freeData != nil && propRet != nil {
			lib.freeData(unsafe.Pointer(propRet))
		}
	}()
	c.ensureAtoms()
	if actualType == c.atoms.incr {
		if lib.deleteProperty != nil {
			lib.deleteProperty(display, win, prop)
		}
		if lib.flush != nil {
			lib.flush(display)
		}
		var out []byte
		deadline := time.Now().Add(clipIncrTimeout)
		for time.Now().Before(deadline) {
			time.Sleep(clipPollInterval)
			var t2 uintptr
			var f2 int
			var n2, b2 uint64
			var p2 *byte
			ret2 := lib.getWindowProperty(display, win, prop, 0, 1<<20, 0, 0, &t2, &f2, &n2, &b2, &p2)
			if ret2 != 0 || p2 == nil {
				if p2 != nil && lib.freeData != nil {
					lib.freeData(unsafe.Pointer(p2))
				}
				continue
			}
			if n2 == 0 {
				if lib.freeData != nil {
					lib.freeData(unsafe.Pointer(p2))
				}
				break
			}
			chunk := unsafe.Slice(p2, n2)
			out = append(out, chunk...)
			if lib.freeData != nil {
				lib.freeData(unsafe.Pointer(p2))
			}
			if lib.deleteProperty != nil {
				lib.deleteProperty(display, win, prop)
			}
			if lib.flush != nil {
				lib.flush(display)
			}
		}
		return out, nil
	}
	if nitems == 0 {
		return nil, fmt.Errorf("x11: clipboard is empty")
	}
	var b []byte
	if actualFormat == 8 {
		b = unsafe.Slice(propRet, nitems)
	} else if actualFormat == 32 {
		raw := unsafe.Slice((*uint32)(unsafe.Pointer(propRet)), nitems)
		b = make([]byte, nitems*4)
		for i, v := range raw {
			b[i*4] = byte(v)
			b[i*4+1] = byte(v >> 8)
			b[i*4+2] = byte(v >> 16)
			b[i*4+3] = byte(v >> 24)
		}
	} else {
		b = unsafe.Slice(propRet, nitems)
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}
