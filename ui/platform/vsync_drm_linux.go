//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux

package platform

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// DRM relative vblank wait flags.
const (
	drmVBlankRelative  = 0x1
	drmVBlankSecondary = 0x2
	// drmVBlankEvent (DRM_VBLANK_EVENT) makes the kernel queue a
	// drm_event_vblank on the fd and return from the ioctl immediately; the
	// vblank itself is then waited for on the fd.
	drmVBlankEvent = 0x04000000
)

// drm_event_vblank header values (drm.h).
const (
	drmEventTypeVBlank = 0x01
	drmEventSize       = 32
)

// Event round-trip budgets. A display that stopped producing vblanks (CRTC
// off, DPMS, device lost) must surface as an error the scheduler backs off
// from instead of parking forever.
const (
	drmWaitTimeout = 100 * time.Millisecond
	drmInitTimeout = 250 * time.Millisecond
)

// drmVBlank matches drm_wait_vblank union layout on linux amd64 for the request half.
type drmVBlank struct {
	Type     int32
	Sequence uint32
	Signal   uint64 // request.signal (ulong)
	_pad     int64  // reply.tval_usec space; unused for request
}

// drmEventVBlank mirrors struct drm_event_vblank (drm.h).
type drmEventVBlank struct {
	Type     uint32
	Length   uint32
	UserData uint64
	TVSec    uint32
	TVUsec   uint32
	Sequence uint32
	CRTCID   uint32
}

var (
	drmOnce    sync.Once
	drmInitErr error
	drmFD      = -1
	drmWaitFn  func(fd int, vbl unsafe.Pointer) int
	// drmEvents is drmFD wrapped for the Go runtime poller. The fd is opened
	// O_NONBLOCK so os.NewFile registers it with the netpoller: waiting for a
	// vblank parks this goroutine and returns its P to the scheduler, instead
	// of leaving a thread in a 16.6ms blocking ioctl that sysmon re-takes every
	// 10ms (runtime/proc.go retake → preemptone). Never call drmEvents.Fd():
	// that puts the descriptor back into blocking mode and unregisters it.
	drmEvents *os.File
	// drmEventTS is the kernel timestamp of the last accepted event; events at
	// or below it are duplicates of a vblank already stamped into the clock.
	drmEventTS uint64
	// drmWaitMu serializes event request + read: every caller owns exactly one
	// queued event, so two waiters can never consume each other's notification.
	drmWaitMu sync.Mutex
)

// ErrNoDRMVBlank is returned when no usable DRM vblank path is available.
var ErrNoDRMVBlank = errors.New("platform: no DRM vblank (open/libdrm/wait failed)")

// requestVBlankEvent queues one vblank event on fd. False means the kernel
// refused the request (no CRTC, no vblank irq).
func requestVBlankEvent(fd int) bool {
	vbl := drmVBlank{Type: drmVBlankRelative | drmVBlankEvent, Sequence: 1, Signal: 1}
	if rc := drmWaitFn(fd, unsafe.Pointer(&vbl)); rc == 0 {
		return true
	}
	// Multi-head: try secondary pipe once.
	vbl = drmVBlank{Type: drmVBlankRelative | drmVBlankEvent | drmVBlankSecondary, Sequence: 1, Signal: 1}
	return drmWaitFn(fd, unsafe.Pointer(&vbl)) == 0
}

// waitVBlankEvent parks on the DRM fd until one fresh vblank event arrives.
// Queue records that are not a vblank event are consumed and skipped; an event
// whose kernel timestamp is not newer than the last accepted one is a
// duplicate and is skipped too, keeping the pacing clock monotonic.
func waitVBlankEvent(f *os.File, timeout time.Duration) error {
	var raw [drmEventSize / 8]uint64 // uint64-aligned on every linux arch
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&raw[0])), drmEventSize)
	if err := f.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		// Not pollable: a read would return EAGAIN at once and spin.
		return fmt.Errorf("%w: drm fd not pollable: %v", ErrNoDRMVBlank, err)
	}
	for {
		if _, err := io.ReadFull(f, buf); err != nil {
			return fmt.Errorf("drm vblank event: %v", err)
		}
		ev := (*drmEventVBlank)(unsafe.Pointer(&raw[0]))
		if ev.Type != drmEventTypeVBlank || ev.Length != drmEventSize {
			continue
		}
		ts := uint64(ev.TVSec)*1e6 + uint64(ev.TVUsec)
		if ts != 0 && drmEventTS != 0 && ts <= drmEventTS {
			continue
		}
		if ts != 0 {
			drmEventTS = ts
		}
		return nil
	}
}

// InitDRMVBlank tries to open a DRM primary node and bind drmWaitVBlank via
// purego, then proves the async event path on that node (queue one vblank
// event, wait for it). Safe to call multiple times; subsequent calls return
// the cached result.
func InitDRMVBlank() error {
	drmOnce.Do(func() {
		lib, err := purego.Dlopen("libdrm.so.2", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib, err = purego.Dlopen("libdrm.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		}
		if err != nil {
			drmInitErr = fmt.Errorf("%w: dlopen: %v", ErrNoDRMVBlank, err)
			return
		}
		purego.RegisterLibFunc(&drmWaitFn, lib, "drmWaitVBlank")
		if drmWaitFn == nil {
			drmInitErr = fmt.Errorf("%w: drmWaitVBlank missing", ErrNoDRMVBlank)
			return
		}
		candidates := []string{
			"/dev/dri/card0",
			"/dev/dri/card1",
			"/dev/dri/card2",
		}
		var last error
		for _, path := range candidates {
			// O_NONBLOCK is what makes the fd pollable for the runtime poller
			// (os.NewFile derives that from F_GETFL).
			fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			if err != nil {
				last = err
				continue
			}
			f := os.NewFile(uintptr(fd), path)
			if f == nil {
				_ = unix.Close(fd)
				last = fmt.Errorf("os.NewFile(%s) failed", path)
				continue
			}
			if !requestVBlankEvent(fd) {
				_ = f.Close()
				last = fmt.Errorf("drmWaitVBlank(event) rejected on %s", path)
				continue
			}
			if err := waitVBlankEvent(f, drmInitTimeout); err != nil {
				_ = f.Close()
				last = fmt.Errorf("%s: %v", path, err)
				continue
			}
			drmFD = fd
			drmEvents = f
			drmInitErr = nil
			return
		}
		if last == nil {
			last = errors.New("no /dev/dri/card*")
		}
		drmInitErr = fmt.Errorf("%w: %v", ErrNoDRMVBlank, last)
	})
	return drmInitErr
}

// WaitDRMVBlank blocks until the next vertical blank on the opened DRM device.
// The vblank is requested as a DRM event and waited for on the fd, so the wait
// parks the calling goroutine and holds no P, while the pacing source stays the
// kernel vblank. Returns ErrNoDRMVBlank (or wrap) when unavailable so callers
// fall back to the software tick.
func WaitDRMVBlank() error {
	if err := InitDRMVBlank(); err != nil {
		return err
	}
	if drmFD < 0 || drmWaitFn == nil || drmEvents == nil {
		return ErrNoDRMVBlank
	}
	drmWaitMu.Lock()
	defer drmWaitMu.Unlock()
	if !requestVBlankEvent(drmFD) {
		return fmt.Errorf("%w: drmWaitVBlank(event) rc != 0", ErrNoDRMVBlank)
	}
	return waitVBlankEvent(drmEvents, drmWaitTimeout)
}

// HasDRMVBlank reports whether Init succeeded (per-call Wait may still error).
func HasDRMVBlank() bool {
	return InitDRMVBlank() == nil && drmFD >= 0
}
