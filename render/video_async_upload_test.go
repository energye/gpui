//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"errors"
	"testing"
	"time"

	"github.com/energye/gpui/gpu/hal"
)

// plainQueue hides the optional cross-thread capability of a backend queue,
// which is what GLES reports.
type plainQueue struct{ hal.Queue }

// failingWriteQueue rejects every texture write and hides the capability, so
// the frame must survive on its CPU shadow.
type failingWriteQueue struct{ hal.Queue }

func (q failingWriteQueue) WriteTexture(*hal.ImageCopyTexture, []byte, *hal.ImageDataLayout, *hal.Extent3D) error {
	return errors.New("injected texture write failure")
}

// failingCrossQueue advertises cross-thread writes like wgpu but rejects the
// write itself, so the worker's publish path must still keep the frame.
type failingCrossQueue struct{ hal.Queue }

func (failingCrossQueue) SupportsCrossThreadTextureWrites() bool { return true }

func (q failingCrossQueue) WriteTexture(*hal.ImageCopyTexture, []byte, *hal.ImageDataLayout, *hal.Extent3D) error {
	return errors.New("injected texture write failure")
}

// nv12Red builds a BT.601 limited-range pure red NV12 frame (Y=76, U=84,
// V=255), the same fixture the real-GPU colour regression uses.
func nv12Red(w, h int) (y, uv []byte) {
	return nv12Solid(w, h, 76, 84, 255)
}

// nv12Solid builds a flat NV12 frame from one Y and one U/V pair.
func nv12Solid(w, h int, yv, u, v byte) (y, uv []byte) {
	y = make([]byte, w*h)
	for i := range y {
		y[i] = yv
	}
	uv = make([]byte, w*h/2)
	for i := 0; i+1 < len(uv); i += 2 {
		uv[i], uv[i+1] = u, v
	}
	return y, uv
}

func waitPlaneIdle(t *testing.T, b *VideoBridge) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		st := b.pendState
		b.mu.Unlock()
		if st == planeStageIdle {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("plane staging did not settle: pending=%v", b.UploadPending())
}

func waitPlaneReady(t *testing.T, b *VideoBridge) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		st := b.pendState
		b.mu.Unlock()
		if st == planeStageReady {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("plane staging never reached ready")
}

func cpuVideoContext(t *testing.T) *Context {
	t.Helper()
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := NewContext(64, 64)
	t.Cleanup(func() { _ = dc.Close() })
	return dc
}

// TestVideoBridgeAsyncPublishesFromWorker locks the cross-thread path used on
// wgpu: the UI thread hands the planes off and the worker uploads and publishes
// them. A successful upload leaves the texture drawable, so the planes are not
// copied again into the CPU shadow (that copy only guards rejected uploads,
// V-2). The noop device has no GPU session, so the draw cannot sample the
// texture and must fail closed instead of replaying an older shadow.
func TestVideoBridgeAsyncPublishesFromWorker(t *testing.T) {
	dev := openNoopVideoDevice(t)
	br := NewVideoBridge(dev)
	defer br.Close()
	if !br.crossThreadWrites {
		t.Fatal("noop queue must report cross-thread writes")
	}
	dc := cpuVideoContext(t)

	const w, h = 64, 32
	y, uv := nv12Red(w, h)
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("UploadPlanesFrameAsync = false, want true")
	}
	// The caller's planes stay owned by the route until pending clears.
	if !br.UploadPending() {
		t.Log("worker finished before the first poll; still valid")
	}
	waitPlaneIdle(t, br)
	st := br.Stats()
	if st.Frames != 1 || st.Uploads != 1 || st.Fallbacks != 0 {
		t.Fatalf("stats=%+v want frames=1 uploads=1 fallbacks=0", st)
	}
	br.mu.Lock()
	slot, back := br.pslot, br.pback
	shadow := len(br.pshadowY) + len(br.pshadowU)
	br.mu.Unlock()
	if slot == nil {
		t.Fatal("worker published no slot")
	}
	if back != nil {
		t.Fatalf("first frame retired pback=%p, want nil", back)
	}
	if shadow != 0 {
		t.Fatalf("successful upload kept a CPU copy (%d bytes), want none", shadow)
	}
	if direct, ok := br.PaintTexture(dc, TexturePaintOptions{Width: 32, Height: 16}); ok || direct {
		t.Fatalf("paint direct=%v ok=%v want false/false without a GPU session", direct, ok)
	}
}

// TestVideoBridgeAsyncDoubleBuffers locks the slot rotation: the second frame
// uploads into a fresh back slot while the first stays drawable, then the swap
// makes the second current and retires the first for the next write.
func TestVideoBridgeAsyncDoubleBuffers(t *testing.T) {
	dev := openNoopVideoDevice(t)
	br := NewVideoBridge(dev)
	defer br.Close()
	if !br.crossThreadWrites {
		t.Fatal("noop queue must report cross-thread writes")
	}
	const w, h = 64, 32
	y, uv := nv12Red(w, h)
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("first UploadPlanesFrameAsync = false")
	}
	waitPlaneIdle(t, br)
	br.mu.Lock()
	first := br.pslot
	br.mu.Unlock()
	if first == nil {
		t.Fatal("first frame published no slot")
	}
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("second UploadPlanesFrameAsync = false")
	}
	waitPlaneIdle(t, br)
	br.mu.Lock()
	second, back := br.pslot, br.pback
	br.mu.Unlock()
	if second == nil || second == first {
		t.Fatalf("second frame slot=%p want a different slot than %p", second, first)
	}
	if back != first {
		t.Fatalf("pback=%p want the retired first slot %p", back, first)
	}
}

// TestVideoBridgeAsyncPublishesOnRasterPath locks the GLES-shaped path: the
// worker only copies; the frame stays staged until the draw publishes it on the
// rendering thread.
func TestVideoBridgeAsyncPublishesOnRasterPath(t *testing.T) {
	dev := openNoopVideoDevice(t)
	queued := stubDevice{Device: dev, queue: plainQueue{dev.Queue()}}
	br := NewVideoBridge(queued)
	defer br.Close()
	if br.crossThreadWrites {
		t.Fatal("wrapped queue must not advertise cross-thread writes")
	}
	dc := cpuVideoContext(t)

	const w, h = 64, 32
	y, uv := nv12Red(w, h)
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("UploadPlanesFrameAsync = false, want true")
	}
	waitPlaneReady(t, br)
	st := br.Stats()
	if st.Frames != 0 || st.Uploads != 0 {
		t.Fatalf("stats before paint=%+v want frames=0 uploads=0", st)
	}
	if !br.UploadPending() {
		t.Fatal("a ready frame must keep the caller's planes owned until drawn")
	}

	if _, ok := br.PaintTexture(dc, TexturePaintOptions{Width: 32, Height: 16}); !ok {
		t.Fatal("paint after staging ok=false")
	}
	if br.UploadPending() {
		t.Fatal("paint must consume the staged frame")
	}
	st = br.Stats()
	if st.Frames != 1 || st.Uploads != 1 {
		t.Fatalf("stats after paint=%+v want frames=1 uploads=1", st)
	}
}

// TestVideoBridgeRasterPublishSwapsShadow locks the ping-pong between the draw
// shadow and the worker's scratch. After two raster-published frames the next
// staged planes must copy into the retired shadow buffer, not into the shadow
// the CPU fallback is about to convert; aliasing the two would smear the new
// frame's chroma under the old frame's luma.
func TestVideoBridgeRasterPublishSwapsShadow(t *testing.T) {
	dev := openNoopVideoDevice(t)
	queued := stubDevice{Device: dev, queue: plainQueue{dev.Queue()}}
	br := NewVideoBridge(queued)
	defer br.Close()
	dc := cpuVideoContext(t)
	const w, h = 64, 32

	frames := [][3]byte{{76, 84, 255}, {145, 54, 34}}
	for i, f := range frames {
		y, uv := nv12Solid(w, h, f[0], f[1], f[2])
		if !br.UploadPlanesFrameAsync(w, h, y, uv) {
			t.Fatalf("frame %d: UploadPlanesFrameAsync = false", i)
		}
		waitPlaneReady(t, br)
		if _, ok := br.PaintTexture(dc, TexturePaintOptions{Width: 32, Height: 16}); !ok {
			t.Fatalf("frame %d: paint ok=false", i)
		}
	}
	if st := br.Stats(); st.Frames != 2 || st.Uploads != 2 {
		t.Fatalf("stats=%+v want frames=2 uploads=2", st)
	}

	// Stage a third frame but do not draw it: the shadow must stay on frame 2.
	y, uv := nv12Solid(w, h, 41, 255, 84)
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("third UploadPlanesFrameAsync = false")
	}
	waitPlaneReady(t, br)

	br.mu.Lock()
	shadowY := br.pshadowY[0]
	shadowU, shadowV := br.pshadowU[0], br.pshadowU[1]
	alias := len(br.pendY) > 0 && len(br.pshadowY) > 0 && &br.pendY[0] == &br.pshadowY[0] ||
		len(br.pendU) > 0 && len(br.pshadowU) > 0 && &br.pendU[0] == &br.pshadowU[0]
	br.mu.Unlock()
	if alias {
		t.Fatal("staged planes alias the draw shadow; the next copy would overwrite pixels the fallback converts")
	}
	if shadowY != 145 || shadowU != 54 || shadowV != 34 {
		t.Fatalf("shadow=(y=%d,u=%d,v=%d) want the drawn frame 2 (145,54,34)", shadowY, shadowU, shadowV)
	}
}

// TestVideoBridgeAsyncBusyRejects locks the ownership rule: one staged frame at
// a time, and the synchronous routes keep out while staging is in flight.
func TestVideoBridgeAsyncBusyRejects(t *testing.T) {
	dev := openNoopVideoDevice(t)
	br := NewVideoBridge(dev)
	defer br.Close()
	dc := cpuVideoContext(t)
	const w, h = 64, 32
	y, uv := nv12Red(w, h)

	br.mu.Lock()
	br.pendState = planeStageCopying
	br.mu.Unlock()

	if br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("async accepted a frame while another is staged")
	}
	if br.UploadPlanesFrame(w, h, y, uv) {
		t.Fatal("sync planes accepted a frame while another is staged")
	}
	if br.UploadFrame(w, h, make([]byte, w*h*4)) {
		t.Fatal("sync RGBA accepted a frame while planes are staged")
	}
	if direct, ok := br.Show(dc, w, h, make([]byte, w*h*4), VideoDrawOptions{}); direct || ok {
		t.Fatal("Show accepted a frame while planes are staged")
	}

	br.mu.Lock()
	br.pendState = planeStageIdle
	br.mu.Unlock()
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("async rejected after staging cleared")
	}
	waitPlaneIdle(t, br)
}

// TestVideoBridgeAsyncUploadFailureFallsBackToShadow locks V-2 on the async
// route: a rejected GPU write must still draw the frame from the staged shadow.
func TestVideoBridgeAsyncUploadFailureFallsBackToShadow(t *testing.T) {
	dev := openNoopVideoDevice(t)
	queued := stubDevice{Device: dev, queue: failingWriteQueue{dev.Queue()}}
	br := NewVideoBridge(queued)
	defer br.Close()
	dc := cpuVideoContext(t)

	const w, h = 64, 32
	y, uv := nv12Red(w, h)
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("UploadPlanesFrameAsync = false, want true")
	}
	waitPlaneReady(t, br)
	direct, ok := br.PaintTexture(dc, TexturePaintOptions{Width: 32, Height: 16})
	if !ok || direct {
		t.Fatalf("fallback paint direct=%v ok=%v want false/true", direct, ok)
	}
	st := br.Stats()
	if st.Frames != 1 || st.Uploads != 0 || st.Fallbacks == 0 {
		t.Fatalf("stats=%+v want frames=1 uploads=0 fallbacks>0", st)
	}
	br.mu.Lock()
	dst := br.fallback.Data()
	pix := append([]byte(nil), dst[:4]...)
	slot := br.pslot
	br.mu.Unlock()
	if pix[0] < 250 || pix[1] > 5 || pix[2] > 5 {
		t.Fatalf("fallback pixel=(%d,%d,%d) want ~(255,0,0)", pix[0], pix[1], pix[2])
	}
	if slot != nil {
		t.Fatal("a failed upload must not keep the slot current")
	}
}

// TestVideoBridgeAsyncCrossUploadFailureFallsBackToShadow locks V-2 on the
// worker-publish route: a backend that advertises cross-thread writes but
// rejects this one must still hand the frame to the CPU fallback.
func TestVideoBridgeAsyncCrossUploadFailureFallsBackToShadow(t *testing.T) {
	dev := openNoopVideoDevice(t)
	queued := stubDevice{Device: dev, queue: failingCrossQueue{dev.Queue()}}
	br := NewVideoBridge(queued)
	defer br.Close()
	if !br.crossThreadWrites {
		t.Fatal("failing cross queue must still advertise cross-thread writes")
	}
	dc := cpuVideoContext(t)

	const w, h = 64, 32
	y, uv := nv12Red(w, h)
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("UploadPlanesFrameAsync = false, want true")
	}
	waitPlaneIdle(t, br)
	if st := br.Stats(); st.Frames != 1 || st.Uploads != 0 {
		t.Fatalf("stats=%+v want frames=1 uploads=0", st)
	}
	direct, ok := br.PaintTexture(dc, TexturePaintOptions{Width: 32, Height: 16})
	if !ok || direct {
		t.Fatalf("fallback paint direct=%v ok=%v want false/true", direct, ok)
	}
	if st := br.Stats(); st.Fallbacks == 0 {
		t.Fatalf("stats=%+v want a counted fallback", st)
	}
	br.mu.Lock()
	dst := br.fallback.Data()
	pix := append([]byte(nil), dst[:4]...)
	slot, back := br.pslot, br.pback
	br.mu.Unlock()
	if pix[0] < 250 || pix[1] > 5 || pix[2] > 5 {
		t.Fatalf("fallback pixel=(%d,%d,%d) want ~(255,0,0)", pix[0], pix[1], pix[2])
	}
	if slot != nil || back != nil {
		t.Fatalf("a failed upload must not keep plane slots (front=%p back=%p)", slot, back)
	}
}

// TestVideoBridgeAsyncCloseWaitsForWorker locks teardown: Close must not
// release the pool under a copy worker or leave staging buffers behind.
func TestVideoBridgeAsyncCloseWaitsForWorker(t *testing.T) {
	dev := openNoopVideoDevice(t)
	br := NewVideoBridge(dev)
	const w, h = 1920, 1080
	y, uv := nv12Red(w, h)
	if !br.UploadPlanesFrameAsync(w, h, y, uv) {
		t.Fatal("UploadPlanesFrameAsync = false, want true")
	}
	br.Close() // must wait for the worker instead of racing it
	if br.UploadPending() {
		t.Fatal("Close left a staged frame pending")
	}
	if st := br.Stats(); st.Live != 0 {
		t.Fatalf("Close left %d live slots", st.Live)
	}
	br.mu.Lock()
	staged := len(br.pendY) + len(br.pendU)
	state := br.pendState
	br.mu.Unlock()
	if staged != 0 || state != planeStageIdle {
		t.Fatalf("Close left staging buffers=%d state=%d", staged, state)
	}
}
