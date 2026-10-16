//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

// Readback helpers for the X11 window tests, which assert the image a real
// window shows.
//
// Oracle policy — the window image comes from the X server (XGetImage), and
// glReadPixels is used for engine-owned framebuffers only:
//
//   - glReadPixels on a window's default framebuffer (FBO 0) is NOT a reliable
//     oracle on a composited desktop (NVIDIA + mutter): under load it
//     intermittently returns the desktop content of the window's area or
//     zeros instead of the frame just drawn. Measured with the backend's own
//     frames: clear-only frames failed 6/8 and 1/8 in one process and 8/8 in
//     separate processes, while triangle frames passed 20/20 — the GL read of
//     FBO 0 depends on timing, not on the frame's content.
//   - The X server's copy of the same window (xwd -id, i.e. XGetImage) agreed
//     with a correct GL read byte-for-byte in every compared capture (8/8
//     window dumps, md5-identical), and holds the swapped image the client
//     presented, which is the thing under test.
//   - The offscreen route's swapchain FBO is an engine-owned texture, where
//     glReadPixels is exact and stays in use.
//
// A presented frame reaches the window content at the next vblank, so window
// captures go through pollWindowPixels instead of reading once.

import (
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// xImage mirrors the leading fields of Xlib's XImage (Xlib.h). Only the fields
// captureWindowPixels needs are declared; it validates them before trusting the
// pixel pointer, so a layout mismatch fails the test instead of decoding noise.
type xImage struct {
	width          int32
	height         int32
	xoffset        int32
	format         int32
	data           uintptr
	byteOrder      int32
	bitmapUnit     int32
	bitmapBitOrder int32
	bitmapPad      int32
	depth          int32
	bytesPerLine   int32
	bitsPerPixel   int32
	_              int32
	redMask        uint64
	greenMask      uint64
	blueMask       uint64
	obdata         uintptr
	createImage    uintptr
	destroyImage   uintptr
	getPixel       uintptr
	putPixel       uintptr
	subImage       uintptr
	addPixel       uintptr
}

// captureWindowPixels returns the content the X server holds for win, in
// engine row order (row 0 = the top row, the order the goldens were taken in)
// as RGBA8 with alpha 255 — an X11 window has no alpha channel, and with no
// compositor the server keeps the window's pixels from the swap.
//
// The window is raised first, and a not-yet-mapped window (the window manager
// intercepts the map request) is retried: XGetImage answers BadMatch then.
func captureWindowPixels(t *testing.T, lib, dpy, win uintptr, w, h int) []byte {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		img := tryCaptureWindowPixels(lib, dpy, win, w, h)
		if img != nil {
			return img
		}
		if time.Now().After(deadline) {
			t.Fatalf("XGetImage(%dx%d) on window 0x%x kept failing (window never viewable?)", w, h, win)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// tryCaptureWindowPixels is captureWindowPixels without the retry: it returns
// nil when the server refuses the capture (window not viewable yet).
func tryCaptureWindowPixels(lib, dpy, win uintptr, w, h int) []byte {
	var (
		xRaiseWindow func(dpy uintptr, win uintptr) int
		xGetImage    func(dpy uintptr, drawable uintptr, x, y int32, width, height uint32, planeMask uint64, format int32) unsafe.Pointer
		xGetPixel    func(img uintptr, x, y int32) uint64
	)
	purego.RegisterLibFunc(&xRaiseWindow, lib, "XRaiseWindow")
	purego.RegisterLibFunc(&xGetImage, lib, "XGetImage")
	purego.RegisterLibFunc(&xGetPixel, lib, "XGetPixel")

	const (
		zPixmap   = 2 // ZPixmap: each pixel is one unit (XImage format)
		allPlanes = ^uint64(0)
	)
	xRaiseWindow(dpy, win)
	img := xGetImage(dpy, win, 0, 0, uint32(w), uint32(h), allPlanes, zPixmap)
	if img == nil {
		return nil
	}
	xi := (*xImage)(img)
	defer purego.SyscallN(xi.destroyImage, uintptr(img))

	if int(xi.width) != w || int(xi.height) != h || xi.format != zPixmap || xi.bitsPerPixel != 32 {
		return nil
	}
	if xi.redMask == 0 || xi.greenMask == 0 || xi.blueMask == 0 {
		return nil
	}
	redShift, greenShift, blueShift := maskShift(xi.redMask), maskShift(xi.greenMask), maskShift(xi.blueMask)
	out := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px := uint32(xGetPixel(uintptr(img), int32(x), int32(y)))
			o := (y*w + x) * 4
			out[o+0] = byte(px >> redShift)
			out[o+1] = byte(px >> greenShift)
			out[o+2] = byte(px >> blueShift)
			out[o+3] = 255
		}
	}
	return out
}

// xErrorHandler is the Xlib error handler these tests install: Xlib's default
// handler prints the error and exits the process, which turns a window that is
// not mapped yet (X_GetImage answers BadMatch) into a dead test binary.
var xErrorHandler = purego.NewCallback(func(dpy unsafe.Pointer, ev uintptr) int32 { return 0 })

var xErrorHandlerOnce sync.Once

func installXErrorHandler(lib uintptr) {
	xErrorHandlerOnce.Do(func() {
		var xSetErrorHandler func(handler uintptr) uintptr
		purego.RegisterLibFunc(&xSetErrorHandler, lib, "XSetErrorHandler")
		xSetErrorHandler(xErrorHandler)
	})
}

// xWindowAttributes mirrors the leading fields of Xlib's XWindowAttributes
// (Xlib.h) up to map_state. waitUntilViewable only reads map_state, and the
// declaration order has to match C or the read lands on another field — which
// is how an earlier probe reported IsUnmapped for viewable windows.
type xWindowAttributes struct {
	x, y, width, height, borderWidth, depth int32
	visual                                  uintptr
	root                                    uint64
	class, bitGravity, winGravity           int32
	backingStore                            int32
	backingPlanes, backingPixel             uint64
	saveUnder                               int32
	_                                       int32
	colormap                                uint64
	mapInstalled                            int32
	mapState                                int32
	overrideRedirect                        int64
}

func xMapStateName(state int32) string {
	switch state {
	case 0:
		return "IsUnmapped"
	case 1:
		return "IsUnviewable"
	case 2:
		return "IsViewable"
	default:
		return "state?"
	}
}

// waitUntilViewable blocks until the X server reports the window as viewable,
// and fails the test when that never happens before the deadline.
//
// A present issued while the window is still unmapped never reaches the
// window: measured on this WM (GNOME/mutter, NVIDIA, 64x64 window), presenting
// straight after Configure lost that frame in 4 of 10 runs — the window then
// held its black background — while waiting for IsViewable first (4.8-10.7 ms
// here) made the very next present land in 10 of 10 runs, 3.1-7.4 ms after the
// swap. Tests call this from their window openers, so the first present they
// measure goes out onto a viewable window.
func waitUntilViewable(t *testing.T, lib, dpy, win uintptr, timeout time.Duration) {
	t.Helper()
	var xGetWindowAttributes func(dpy uintptr, win uintptr, attr *xWindowAttributes) int
	purego.RegisterLibFunc(&xGetWindowAttributes, lib, "XGetWindowAttributes")
	start := time.Now()
	deadline := start.Add(timeout)
	var last int32 = -1
	for {
		var a xWindowAttributes
		if xGetWindowAttributes(dpy, win, &a) != 0 {
			last = a.mapState
			if a.mapState == 2 {
				t.Logf("window 0x%x viewable after %v", win, time.Since(start))
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("window 0x%x never became viewable within %v (last state %s)", win, timeout, xMapStateName(last))
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// maskShift returns the bit position a channel mask starts at.
func maskShift(mask uint64) uint {
	var s uint
	for mask&1 == 0 && s < 64 {
		mask >>= 1
		s++
	}
	return s
}

// pollWindowPixels returns the window image a just-presented frame put on
// screen: the X server picks a swapped buffer up at the next vblank (measured
// 3–26 ms after eglSwapBuffers on NVIDIA + mutter), so the copy is re-read
// until it satisfies want and the last read is returned once the deadline
// passes — a frame that never lands fails the caller's assertions on the
// returned image instead of passing on the previous frame's content.
func pollWindowPixels(t *testing.T, lib, dpy, win uintptr, w, h int, want func([]byte) bool) []byte {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var img []byte
	for {
		if got := tryCaptureWindowPixels(lib, dpy, win, w, h); got != nil {
			img = got
			if want(img) {
				return img
			}
		}
		if time.Now().After(deadline) {
			if img == nil {
				t.Fatalf("no window capture for 0x%x: the server refused every XGetImage (%dx%d)", win, w, h)
			}
			return img
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// readSwapchainPixels reads the offscreen route's swapchain FBO back in engine
// row order (its storage row 0 is engine row 0, so no flip is involved). This
// is the engine's own texture — the reliable GL readback.
func readSwapchainPixels(t *testing.T, surf *Surface, w, h int) []byte {
	t.Helper()
	if surf.ctx == nil {
		t.Fatal("surface has no context")
	}
	glCtx := surf.ctx.Lock()
	defer surf.ctx.Unlock()
	glCtx.Finish()

	raw := make([]byte, w*h*4)
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	glCtx.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&raw[0]))
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	return raw
}

// pixelsAt returns the RGBA of pixel (x, y) of an engine-row-order readback.
func pixelsAt(pixels []byte, w, x, y int) []int {
	o := (y*w + x) * 4
	return []int{int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])}
}

// pixelsEqual reports whether two RGBA reads are the same four bytes.
func pixelsEqual(a, b []int) bool {
	return len(a) == 4 && len(b) == 4 &&
		a[0] == b[0] && a[1] == b[1] && a[2] == b[2] && a[3] == b[3]
}

// h4cTrianglePipeline builds the pipeline the window tests draw with: the H4C
// triangle shader set (red, spanning rows 16..47 of a 64x64 target) against an
// RGBA8 color target.
func h4cTrianglePipeline(t *testing.T, dev hal.Device) hal.RenderPipeline {
	t.Helper()
	vsMod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: h4cReadWGSL(t, "h4c_triangle_vertex.wgsl")})
	if err != nil {
		t.Fatalf("CreateShaderModule(vertex): %v", err)
	}
	fsMod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: h4cReadWGSL(t, "h4c_triangle_fragment.wgsl")})
	if err != nil {
		t.Fatalf("CreateShaderModule(fragment): %v", err)
	}
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Vertex: hal.VertexState{Module: vsMod, EntryPoint: "vs_main"},
		Fragment: &hal.FragmentState{
			Module:     fsMod,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    gputypes.TextureFormatRGBA8Unorm,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}
	return pipe
}
