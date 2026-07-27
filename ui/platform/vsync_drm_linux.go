//go:build linux

package platform

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// DRM relative vblank wait flags (libdrm drm.h).
const (
	drmVBlankRelative  = 0x1
	drmVBlankSecondary = 0x2
)

// drmVBlank matches drm_wait_vblank union layout on linux amd64 for the request half.
type drmVBlank struct {
	Type     int32
	Sequence uint32
	Signal   uint64 // request.signal (ulong)
	_pad     int64  // reply.tval_usec space; unused for request
}

var (
	drmOnce    sync.Once
	drmInitErr error
	drmFD      = -1
	drmWaitFn  func(fd int, vbl unsafe.Pointer) int
)

// ErrNoDRMVBlank is returned when no usable DRM vblank path is available.
var ErrNoDRMVBlank = errors.New("platform: no DRM vblank (open/libdrm/wait failed)")

// InitDRMVBlank tries to open a DRM primary node and bind drmWaitVBlank via purego.
// Safe to call multiple times; subsequent calls return the cached result.
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
			f, err := os.OpenFile(path, os.O_RDWR, 0)
			if err != nil {
				last = err
				continue
			}
			raw := int(f.Fd())
			dup, err := unix.Dup(raw)
			_ = f.Close()
			if err != nil {
				last = err
				continue
			}
			var vbl drmVBlank
			vbl.Type = drmVBlankRelative
			vbl.Sequence = 1
			rc := drmWaitFn(dup, unsafe.Pointer(&vbl))
			if rc != 0 {
				// Multi-head: try secondary pipe once.
				vbl = drmVBlank{Type: drmVBlankRelative | drmVBlankSecondary, Sequence: 1}
				rc = drmWaitFn(dup, unsafe.Pointer(&vbl))
			}
			if rc != 0 {
				_ = unix.Close(dup)
				last = fmt.Errorf("drmWaitVBlank rc=%d on %s", rc, path)
				continue
			}
			drmFD = dup
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
// Returns ErrNoDRMVBlank (or wrap) when unavailable so callers fall back to software tick.
func WaitDRMVBlank() error {
	if err := InitDRMVBlank(); err != nil {
		return err
	}
	if drmFD < 0 || drmWaitFn == nil {
		return ErrNoDRMVBlank
	}
	var vbl drmVBlank
	vbl.Type = drmVBlankRelative
	vbl.Sequence = 1
	rc := drmWaitFn(drmFD, unsafe.Pointer(&vbl))
	if rc != 0 {
		vbl = drmVBlank{Type: drmVBlankRelative | drmVBlankSecondary, Sequence: 1}
		rc = drmWaitFn(drmFD, unsafe.Pointer(&vbl))
		if rc != 0 {
			return fmt.Errorf("%w: drmWaitVBlank rc=%d", ErrNoDRMVBlank, rc)
		}
	}
	return nil
}

// HasDRMVBlank reports whether Init succeeded (per-call Wait may still error).
func HasDRMVBlank() bool {
	return InitDRMVBlank() == nil && drmFD >= 0
}
