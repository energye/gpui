//go:build linux

package platform

import (
	"errors"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Headless tests for the Wayland outbound drag source (S6-P1 item 4):
// no compositor needed — error paths, the send/cancelled serving branches,
// and the drag-vs-clipboard source split are all exercised in-process.

func TestWlDragStartDragToUnsupported(t *testing.T) {
	c := &waylandController{h: &wlHost{win: &wlWin{}}}
	if err := c.StartDragTo(nil, DragOffer{Files: []string{"/tmp/a"}}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("StartDragTo = %v, want ErrUnsupported", err)
	}
}

func TestWlDragStartDragNoWindow(t *testing.T) {
	var nilCtl *waylandController
	if err := nilCtl.StartDrag(DragOffer{Files: []string{"/tmp/a"}}); err == nil {
		t.Fatal("nil controller StartDrag must fail")
	}
	c := &waylandController{h: &wlHost{win: &wlWin{}}}
	if err := c.StartDrag(DragOffer{}); err == nil {
		t.Fatal("empty offer must fail")
	}
	if err := c.StartDrag(DragOffer{Files: []string{"/tmp/a"}}); err == nil {
		t.Fatal("window without data-device must fail")
	}
}

// Without any input serial yet the compositor would drop the request —
// honest ErrUnsupported before touching the wire (fully headless: the
// serial gate runs before create_data_source).
func TestWlDragStartDragNeedsSerial(t *testing.T) {
	w := &wlWin{
		lib:     &wlLib{},
		display: 1,
		surface: 2,
		dds:     &wlDataDeviceState{mgr: 3, dd: 4},
	}
	c := &waylandController{h: &wlHost{win: w}}
	if err := c.StartDrag(DragOffer{Files: []string{"/tmp/a"}}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("serial-0 StartDrag = %v, want ErrUnsupported", err)
	}
}

// The send callback serves the drag payload to destinations by MIME when
// the source is our drag source, and the clipboard copy otherwise.
func TestWlDragSourceSendServesByMIME(t *testing.T) {
	st := &wlDataDeviceState{
		dragSource: 0x1234,
		dragData:   map[string][]byte{"text/plain": []byte("drag-bytes")},
		ownData:    "clip-bytes",
	}
	self := uintptr(unsafe.Pointer(st))
	defer runtime.KeepAlive(st)

	readAll := func(source uintptr, mime string) string {
		t.Helper()
		p := [2]int{-1, -1}
		if err := unix.Pipe(p[:]); err != nil {
			t.Fatalf("pipe: %v", err)
		}
		mb := append([]byte(mime), 0)
		wlSourceSendCB(self, source, uintptr(unsafe.Pointer(&mb[0])), uintptr(p[1]))
		runtime.KeepAlive(mb)
		var out []byte
		tmp := make([]byte, 64)
		for {
			n, _ := unix.Read(p[0], tmp)
			if n <= 0 {
				break
			}
			out = append(out, tmp[:n]...)
		}
		_ = unix.Close(p[0])
		return string(out)
	}

	if got := readAll(0x1234, "text/plain"); got != "drag-bytes" {
		t.Fatalf("drag send = %q, want drag-bytes", got)
	}
	if got := readAll(0x1234, "image/png"); got != "" {
		t.Fatalf("unannounced mime send = %q, want empty", got)
	}
	if got := readAll(0x5678, "text/plain"); got != "clip-bytes" {
		t.Fatalf("clipboard send = %q, want clip-bytes", got)
	}
	wlSourceSendCB(0, 0x1234, 0, 0) // nil state / closed fd: must not panic
}

// Cancelled clears only the matching source side (drag vs clipboard).
func TestWlDragSourceCancelledSplits(t *testing.T) {
	st := &wlDataDeviceState{
		ownSource:  0x5678,
		ownData:    "clip",
		dragSource: 0x1234,
		dragData:   map[string][]byte{"text/plain": {1}},
	}
	self := uintptr(unsafe.Pointer(st))
	defer runtime.KeepAlive(st)

	wlSourceCancelledCB(self, 0x1234)
	if st.dragSource != 0 || st.dragData != nil {
		t.Fatal("drag cancel must clear the drag source")
	}
	if st.ownSource == 0 || st.ownData == "" {
		t.Fatal("drag cancel must keep the clipboard source")
	}
	wlSourceCancelledCB(self, 0x5678)
	if st.ownSource != 0 || st.ownData != "" {
		t.Fatal("clipboard cancel must clear the clipboard source")
	}
}
