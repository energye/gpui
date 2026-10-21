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

import (
	"image"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// TestPresentDamageX11 is the real-window gate for damage-based incremental
// present:
//
//  1. Full presents must update the window on every frame (basic swap).
//  2. A full frame followed by small damage frames must eventually use the
//     partial swap path (presentPartialCount > 0).
//  3. Window pixels outside the damage rectangle must not change (no ghosting
//     from rotating back buffers), while pixels inside it must update.
func TestPresentDamageX11(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 128, 128
	xw := h4e2OpenX11Window(t, w, h)
	defer xw.close(t)

	var (
		xSync      func(dpy uintptr, discard int) int
		xGetImage  func(dpy, drawable uintptr, x, y int, width, height uint, planeMask uintptr, format int) uintptr
		xGetPixel  func(img uintptr, x, y int) uintptr
		xDestroyIm func(img uintptr) int
		xRaiseWin  func(dpy, w uintptr) int
	)
	purego.RegisterLibFunc(&xSync, xw.lib, "XSync")
	purego.RegisterLibFunc(&xGetImage, xw.lib, "XGetImage")
	purego.RegisterLibFunc(&xGetPixel, xw.lib, "XGetPixel")
	purego.RegisterLibFunc(&xDestroyIm, xw.lib, "XDestroyImage")
	purego.RegisterLibFunc(&xRaiseWin, xw.lib, "XRaiseWindow")
	xRaiseWin(xw.display, xw.window)
	xSync(xw.display, 0)

	backend := NewBackend()
	inst, err := backend.CreateInstance(nil)
	if err != nil {
		t.Skipf("CreateInstance unavailable (no EGL?): %v", err)
	}
	defer inst.Release()

	surfHal, err := inst.CreateSurface(hal.SurfaceTarget{
		Kind:          hal.SurfaceTargetXlibWindow,
		DisplayHandle: xw.display,
		WindowHandle:  xw.window,
	})
	if err != nil {
		t.Fatalf("CreateSurface(Xlib): %v", err)
	}
	surf, ok := surfHal.(*Surface)
	if !ok {
		t.Fatalf("surface type = %T, want *gles.Surface", surfHal)
	}
	defer surf.Destroy()

	adapter, err := inst.RequestAdapter(&hal.RequestAdapterOptions{CompatibleSurface: surfHal})
	if err != nil {
		t.Fatalf("RequestAdapter: %v", err)
	}
	opened, err := adapter.Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	queue := opened.Queue

	cfg := &hal.SurfaceConfiguration{
		Width:       w,
		Height:      h,
		Format:      gputypes.TextureFormatRGBA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: hal.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}
	if err := surf.Configure(nil, cfg); err != nil {
		t.Fatalf("Surface.Configure: %v", err)
	}

	if !egl.HasSwapBuffersWithDamage() || !egl.HasPartialUpdate() {
		t.Skipf("EGL incremental present unavailable on this display (swap=%v partial=%v)",
			egl.HasSwapBuffersWithDamage(), egl.HasPartialUpdate())
	}

	const (
		dmgX0, dmgY0 = 20, 20
		dmgX1, dmgY1 = 40, 40
	)
	small := image.Rect(dmgX0, dmgY0, dmgX1, dmgY1)

	clearFBO := func(r, g, b float32, rect *image.Rectangle) {
		t.Helper()
		glCtx, err := surf.ctx.TryLockForSurface(surf.eglSurface)
		if err != nil {
			t.Fatalf("clearFBO TryLockForSurface: %v", err)
		}
		glCtx.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
		if rect != nil {
			// FBO storage is top-left in UI terms (the present blit flips
			// it to the GL bottom-left window origin), so the source scissor
			// uses UI coordinates directly.
			glCtx.Enable(gl.SCISSOR_TEST)
			glCtx.Scissor(int32(rect.Min.X), int32(rect.Min.Y), int32(rect.Dx()), int32(rect.Dy()))
		} else {
			glCtx.Disable(gl.SCISSOR_TEST)
		}
		glCtx.ClearColor(r, g, b, 1)
		glCtx.Clear(gl.COLOR_BUFFER_BIT)
		glCtx.Disable(gl.SCISSOR_TEST)
		surf.ctx.Unlock()
	}

	present := func(rects []image.Rectangle) {
		t.Helper()
		acq, err := surf.AcquireTexture(nil)
		if err != nil {
			t.Fatalf("AcquireTexture: %v", err)
		}
		if acq == nil || acq.Texture == nil {
			t.Fatal("AcquireTexture returned nil texture")
		}
		if err := queue.Present(surfHal, acq.Texture, rects); err != nil {
			t.Fatalf("Queue.Present(%v): %v", rects, err)
		}
		if e := egl.GetError(); e != egl.Success {
			t.Fatalf("egl error after Present(%v) = 0x%x", rects, uint32(e))
		}
		xSync(xw.display, 0)
		time.Sleep(120 * time.Millisecond)
	}

	windowPixel := func(x, y int) uintptr {
		t.Helper()
		img := xGetImage(xw.display, xw.window, x, y, 1, 1, ^uintptr(0), 2 /* ZPixmap */)
		if img == 0 {
			t.Fatalf("XGetImage(window %d,%d) returned nil", x, y)
		}
		p := xGetPixel(img, 0, 0)
		xDestroyIm(img)
		return p
	}
	screenInside := func() uintptr { return windowPixel(dmgX0+5, dmgY0+5) }
	screenOutside := func() uintptr { return windowPixel(5, 5) }

	// Basic gate: full presents must hit the window every time. If this
	// fails, the present path (not the damage policy) is broken.
	fullColors := [][3]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	var prevInside uintptr
	for i, c := range fullColors {
		clearFBO(c[0], c[1], c[2], nil)
		present(nil)
		in, out := screenInside(), screenOutside()
		t.Logf("full frame %d: inside=%#x outside=%#x", i+1, in, out)
		if in != out {
			t.Fatalf("full frame %d: inside=%#x outside=%#x, want the whole window updated", i+1, in, out)
		}
		// The very first present after window creation can still be black
		// before the WM composites it; the steady-state gate starts at 2.
		if i > 0 && in == prevInside {
			t.Fatalf("full frame %d: window did not update (still %#x)", i+1, in)
		}
		prevInside = in
	}

	// Y-flip guard: the present blit flips the top-left FBO image onto the
	// bottom-left default framebuffer, so a UI top-left rect must appear at
	// the window top-left, never at the bottom-left. Take the red reference
	// from a frame that is definitely composited (not the first black one).
	clearFBO(1, 0, 0, nil)
	present(nil)
	redPx := screenOutside()
	clearFBO(0, 1, 0, nil)
	clearFBO(1, 0, 0, &small)
	present(nil)
	tl := windowPixel(dmgX0+5, dmgY0+5)
	bl := windowPixel(dmgX0+5, h-dmgY0-5)
	if tl != redPx || bl == redPx {
		t.Fatalf("Y-flip guard: top-left=%#x bottom-left=%#x, want top-left red %#x", tl, bl, redPx)
	}

	// Base full frame for the damage run. The extension string is the
	// reliable capability check; EGL_BUFFER_AGE_EXT itself is only meaningful
	// for the back buffer during a present.
	if !strings.Contains(egl.QueryString(surf.eglDisplay, egl.Extensions), "EGL_EXT_buffer_age") {
		t.Skip("EGL_EXT_buffer_age unavailable; partial present cannot be validated")
	}
	clearFBO(0, 1, 0, nil)
	present(nil)
	base := screenOutside()
	if screenInside() != base {
		t.Fatalf("damage base frame is not uniform: inside=%#x outside=%#x", screenInside(), base)
	}

	colors := [][3]float32{{0, 0, 1}, {1, 1, 0}, {1, 0, 1}, {0, 1, 1}, {1, 0, 0}, {0, 0, 1}}
	prevInside = screenInside()
	for i, c := range colors {
		clearFBO(c[0], c[1], c[2], &small)
		present([]image.Rectangle{small})
		in, out := screenInside(), screenOutside()
		t.Logf("damage frame %d: inside=%#x outside=%#x full=%d partial=%d",
			i+1, in, out, surf.presentFullCount, surf.presentPartialCount)
		if out != base {
			t.Fatalf("damage frame %d: outside=%#x, want unchanged %#x (ghosting)", i+1, out, base)
		}
		if in == prevInside {
			t.Fatalf("damage frame %d: inside did not update (%#x)", i+1, in)
		}
		prevInside = in
	}

	if surf.presentPartialCount == 0 {
		t.Fatalf("partial present never engaged: full=%d partial=%d", surf.presentFullCount, surf.presentPartialCount)
	}
	t.Logf("damage present X11: full=%d partial=%d (window 0x%x, %dx%d)",
		surf.presentFullCount, surf.presentPartialCount, xw.window, w, h)
}
