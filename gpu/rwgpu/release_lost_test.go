package rwgpu

import (
	"sync/atomic"
	"testing"
)

// TestDeviceLostReleasePathsSkipNative verifies that when a device is sticky-lost,
// Release/Destroy/Unconfigure clear Go-side handles without invoking native calls.
func TestDeviceLostReleasePathsSkipNative(t *testing.T) {
	const h uintptr = 0xa11ce
	d := testDevice(h)
	t.Cleanup(func() { unregisterLiveDevice(d) })
	markDeviceLost(h)

	var nativeCalls atomic.Int32
	prev := nativeCallHook
	nativeCallHook = func(kind string) {
		nativeCalls.Add(1)
		t.Logf("unexpected native call: %s", kind)
	}
	t.Cleanup(func() { nativeCallHook = prev })

	// Device release when lost: clear handle, skip native.
	d.Release()
	if d.handle != 0 {
		t.Fatalf("Device.Release must clear handle, got %#x", d.handle)
	}
	// Second call is no-op.
	d.Release()
	// Sticky lost survives Device.Release unregister.
	if !IsDeviceHandleLost(h) {
		t.Fatal("lost mark must remain sticky after Device.Release")
	}

	// Queue with parent lost (handle still sticky after device unregister).
	q := &Queue{handle: 0xb001, device: h}
	q.Release()
	if q.handle != 0 {
		t.Fatal("Queue.Release must clear handle when lost")
	}
	q.Release()

	// Buffer Destroy + Release (IsLost via *Device still works on a live object).
	d2 := testDevice(0xa11cf)
	t.Cleanup(func() { unregisterLiveDevice(d2) })
	markDeviceLost(0xa11cf)
	buf := &Buffer{handle: 0xb002, device: d2}
	buf.Destroy()
	if buf.handle != 0 {
		t.Fatal("Buffer.Destroy must clear handle when lost")
	}
	buf.Destroy()
	buf.Release()

	// Texture Destroy + Release.
	tex := &Texture{handle: 0xb003, device: h}
	tex.Destroy()
	if tex.handle != 0 {
		t.Fatal("Texture.Destroy must clear handle when lost")
	}
	tex.Destroy()
	tex.Release()

	// TextureView Release.
	tv := &TextureView{handle: 0xb004, device: h}
	tv.Release()
	if tv.handle != 0 {
		t.Fatal("TextureView.Release must clear handle when lost")
	}
	tv.Release()

	// Surface Unconfigure + Release.
	s := &Surface{handle: 0xb005, device: h, deviceRef: d2}
	// d2 is a different handle; bind surface to sticky-lost h.
	s.device = h
	s.deviceRef = nil
	s.Unconfigure()
	if s.device != 0 || s.deviceRef != nil {
		t.Fatal("Surface.Unconfigure must clear device binding when lost")
	}
	// handle preserved after Unconfigure
	if s.handle == 0 {
		t.Fatal("Unconfigure must not clear surface handle")
	}
	s.device = h
	s.Release()
	if s.handle != 0 {
		t.Fatal("Surface.Release must clear handle when lost")
	}
	s.Release()

	// QuerySet, Encoder, CommandBuffer, Pipelines.
	qs := &QuerySet{handle: 0xb006, device: h}
	qs.Destroy()
	if qs.handle != 0 {
		t.Fatal("QuerySet.Destroy must clear handle when lost")
	}
	enc := &CommandEncoder{handle: 0xb007, device: h}
	enc.Release()
	if enc.handle != 0 {
		t.Fatal("CommandEncoder.Release must clear handle when lost")
	}
	cb := &CommandBuffer{handle: 0xb008, device: h}
	cb.Release()
	if cb.handle != 0 {
		t.Fatal("CommandBuffer.Release must clear handle when lost")
	}
	rp := &RenderPipeline{handle: 0xb009, device: h}
	rp.Release()
	if rp.handle != 0 {
		t.Fatal("RenderPipeline.Release must clear handle when lost")
	}
	cp := &ComputePipeline{handle: 0xb00a, device: h}
	cp.Release()
	if cp.handle != 0 {
		t.Fatal("ComputePipeline.Release must clear handle when lost")
	}
	sm := &ShaderModule{handle: 0xb00b, device: h}
	sm.Release()
	if sm.handle != 0 {
		t.Fatal("ShaderModule.Release must clear handle when lost")
	}
	samp := &Sampler{handle: 0xb00c, device: h}
	samp.Release()
	if samp.handle != 0 {
		t.Fatal("Sampler.Release must clear handle when lost")
	}

	if n := nativeCalls.Load(); n != 0 {
		t.Fatalf("native release/destroy/unconfigure invoked %d times when device lost; want 0", n)
	}
}

// TestReleaseNilAndZeroHandleSafe verifies nil receivers and zero handles are no-ops.
func TestReleaseNilAndZeroHandleSafe(t *testing.T) {
	var nativeCalls atomic.Int32
	prev := nativeCallHook
	nativeCallHook = func(string) { nativeCalls.Add(1) }
	t.Cleanup(func() { nativeCallHook = prev })

	var d *Device
	d.Release()
	var q *Queue
	q.Release()
	var b *Buffer
	b.Destroy()
	b.Release()
	var tex *Texture
	tex.Destroy()
	tex.Release()
	var tv *TextureView
	tv.Release()
	var s *Surface
	s.Unconfigure()
	s.Release()
	var qs *QuerySet
	qs.Destroy()
	qs.Release()
	var enc *CommandEncoder
	enc.Release()
	var cb *CommandBuffer
	cb.Release()
	var rp *RenderPipeline
	rp.Release()
	var cp *ComputePipeline
	cp.Release()
	var sm *ShaderModule
	sm.Release()
	var samp *Sampler
	samp.Release()
	var bgl *BindGroupLayout
	bgl.Release()
	var bg *BindGroup
	bg.Release()
	var pl *PipelineLayout
	pl.Release()
	var rpe *RenderPassEncoder
	rpe.Release()
	var cpe *ComputePassEncoder
	cpe.Release()
	var rbe *RenderBundleEncoder
	rbe.Release()
	var rb *RenderBundle
	rb.Release()
	var inst *Instance
	inst.Release()
	var ad *Adapter
	ad.Release()

	// Zero-handle objects.
	(&Device{}).Release()
	(&Queue{}).Release()
	(&Buffer{}).Destroy()
	(&Buffer{}).Release()
	(&Texture{}).Destroy()
	(&Texture{}).Release()
	(&Surface{}).Unconfigure()
	(&Surface{}).Release()

	if n := nativeCalls.Load(); n != 0 {
		t.Fatalf("nil/zero release must not call native, got %d calls", n)
	}
}

// TestSurfaceUnconfigure_NilBeforeInit ensures Unconfigure does not call mustInit/panic on nil.
func TestSurfaceUnconfigure_NilBeforeInit(t *testing.T) {
	var s *Surface
	s.Unconfigure() // must not panic even without native lib
	s = &Surface{handle: 0}
	s.Unconfigure()
}

// TestReleaseNativeHandle_HelperIdempotent exercises the shared helper directly.
func TestReleaseNativeHandle_HelperIdempotent(t *testing.T) {
	var calls atomic.Int32
	prev := nativeCallHook
	nativeCallHook = func(string) { calls.Add(1) }
	t.Cleanup(func() { nativeCallHook = prev })

	h := uintptr(0x42)
	// Healthy path: native is invoked once.
	releaseNativeHandle(&h, false, func(got uintptr) {
		if got != 0x42 {
			t.Fatalf("native got %#x", got)
		}
	})
	if h != 0 {
		t.Fatal("handle not cleared")
	}
	if calls.Load() != 1 {
		t.Fatalf("want 1 native call, got %d", calls.Load())
	}
	// Idempotent second call.
	releaseNativeHandle(&h, false, func(uintptr) { t.Fatal("should not call") })

	// Lost path: skip native.
	h = 0x43
	calls.Store(0)
	releaseNativeHandle(&h, true, func(uintptr) { t.Fatal("lost must skip native") })
	if h != 0 {
		t.Fatal("lost path must still clear handle")
	}
	if calls.Load() != 0 {
		t.Fatal("lost path must not invoke hook")
	}
}
