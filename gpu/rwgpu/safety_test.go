package rwgpu

import (
	"errors"
	"sync"
	"testing"
)

func TestClassifyError_Sentinels(t *testing.T) {
	cases := []struct {
		err  error
		want ErrorClass
	}{
		{nil, ErrorClassUnknown},
		{ErrDeviceLost, ErrorClassDeviceLost},
		{ErrSurfaceDeviceLost, ErrorClassDeviceLost},
		{ErrOutOfMemory, ErrorClassOutOfMemory},
		{ErrSurfaceOutOfMemory, ErrorClassOutOfMemory},
		{ErrInvalidHandle, ErrorClassInvalidHandle},
		{ErrSurfaceLost, ErrorClassSurfaceInvalid},
		{ErrSurfaceNeedsReconfigure, ErrorClassSurfaceInvalid},
		{ErrSurfaceOccluded, ErrorClassSurfaceInvalid},
		{ErrSurfaceTimeout, ErrorClassSurfaceInvalid},
		{ErrSurfaceInvalid, ErrorClassSurfaceInvalid},
		{ErrValidation, ErrorClassValidation},
		{&WGPUError{Op: "CreateTexture", Type: ErrorTypeOutOfMemory, Message: "oom"}, ErrorClassOutOfMemory},
		{&WGPUError{Op: "x", Message: "Parent device is lost"}, ErrorClassDeviceLost},
		{&WGPUError{Op: "x", Message: "device is nil or released"}, ErrorClassInvalidHandle},
		{&WGPUError{Op: "x", Message: "surface needs reconfigure"}, ErrorClassSurfaceInvalid},
		{&WGPUError{Op: "x", Message: "Not enough memory left"}, ErrorClassOutOfMemory},
		{errors.New("unrelated boom"), ErrorClassUnknown},
	}
	for _, tc := range cases {
		got := ClassifyError(tc.err)
		if got != tc.want {
			t.Errorf("ClassifyError(%v)=%v want %v", tc.err, got, tc.want)
		}
	}
}

func TestDeviceLostSticky_BlocksCreateTexture(t *testing.T) {
	const h uintptr = 0xcafebabe
	d := testDevice(h)
	t.Cleanup(func() { unregisterLiveDevice(d) })

	// Before mark: refuseIfLost is clear (won't call native with fake handle).
	if err := refuseIfLost("CreateTexture", d.handle); err != nil {
		t.Fatalf("unexpected refuse before mark: %v", err)
	}

	markDeviceLost(h)
	err := refuseIfLost("CreateTexture", d.handle)
	if !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("after mark refuseIfLost=%v want ErrDeviceLost", err)
	}
	// CreateTexture must refuse without purego call on lost device.
	tex, err := d.CreateTexture(&TextureDescriptor{
		Usage:         0x10,
		Dimension:     2,
		Size:          Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		Format:        0x17,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if tex != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateTexture after lost: tex=%v err=%v", tex, err)
	}
	// Isolation: a different device is not blocked.
	d2 := testDevice(0x1111)
	t.Cleanup(func() { unregisterLiveDevice(d2) })
	if err := refuseIfLost("CreateBuffer", d2.handle); err != nil {
		t.Fatalf("other device must not be blocked: %v", err)
	}
}

func TestDestroyNullsHandle_Buffer(t *testing.T) {
	// Synthetic handle: Destroy must null without panicking even if native
	// is unavailable — but Destroy calls mustInit + purego. Use zero path.
	var b *Buffer
	b.Destroy() // nil-safe

	// Zero handle: no-op
	b = &Buffer{handle: 0}
	b.Destroy()
	if b.handle != 0 {
		t.Fatal("zero handle Destroy should stay zero")
	}
}

func TestDestroyNullsHandle_WithRealDevice(t *testing.T) {
	if err := Init(); err != nil {
		t.Skipf("wgpu-native unavailable: %v", err)
	}
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Skipf("CreateInstance: %v", err)
	}
	defer inst.Release()
	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Skipf("RequestAdapter: %v", err)
	}
	defer adapter.Release()
	dev, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("RequestDevice: %v", err)
	}
	defer dev.Release()

	buf, err := dev.CreateBuffer(&BufferDescriptor{
		Size:  256,
		Usage: BufferUsageCopySrc | BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	if buf.handle == 0 {
		t.Fatal("expected non-zero handle")
	}
	buf.Destroy()
	if buf.handle != 0 {
		t.Fatalf("Destroy must null handle, got %#x", buf.handle)
	}
	// Post-destroy use: Size/ops must not call native with wild handle.
	if sz := buf.Size(); sz != 0 {
		// Size() may still call native if handle non-zero; after null should be 0.
		t.Fatalf("Size after Destroy = %d, want 0", sz)
	}
	// Second Destroy / Release are no-ops.
	buf.Destroy()
	buf.Release()
}

func TestDestroyNullsHandle_Texture(t *testing.T) {
	if err := Init(); err != nil {
		t.Skipf("wgpu-native unavailable: %v", err)
	}
	// Clear sticky so this test is not poisoned by earlier lost marks.
	// Do not force-clear sticky if something else set it mid-run; only skip.
	// Device.lost is per-object; no process-wide skip needed.
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Skipf("CreateInstance: %v", err)
	}
	defer inst.Release()
	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Skipf("RequestAdapter: %v", err)
	}
	defer adapter.Release()
	dev, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("RequestDevice: %v", err)
	}
	defer dev.Release()

	tex, err := dev.CreateTexture(&TextureDescriptor{
		Usage:         TextureUsageRenderAttachment,
		Dimension:     TextureDimension2D,
		Size:          Extent3D{Width: 16, Height: 16, DepthOrArrayLayers: 1},
		Format:        TextureFormatRGBA8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	if tex.handle == 0 {
		t.Fatal("expected non-zero texture handle")
	}
	tex.Destroy()
	if tex.handle != 0 {
		t.Fatalf("Destroy must null texture handle, got %#x", tex.handle)
	}
	tex.Destroy()
	tex.Release()
}

func TestWithGPU_Serializes(t *testing.T) {
	var order []int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			WithGPU(func() {
				mu.Lock()
				order = append(order, n)
				mu.Unlock()
			})
		}(i)
	}
	wg.Wait()
	if len(order) != 8 {
		t.Fatalf("expected 8 serialized entries, got %d", len(order))
	}
}

func TestSurface_GetCurrentTexture_StickyLost(t *testing.T) {
	const h uintptr = 0xbeef
	d := testDevice(h)
	t.Cleanup(func() { unregisterLiveDevice(d) })

	s := &Surface{handle: 1, device: h, deviceRef: d}
	markDeviceLost(h)
	_, _, err := s.GetCurrentTexture()
	if err == nil {
		t.Fatal("expected error after device lost")
	}
	if !errors.Is(err, ErrDeviceLost) && !errors.Is(err, ErrSurfaceDeviceLost) {
		t.Fatalf("GetCurrentTexture after lost: %v (class=%v), want ErrDeviceLost/ErrSurfaceDeviceLost",
			err, ClassifyError(err))
	}
	if ClassifyError(err) != ErrorClassDeviceLost {
		t.Fatalf("ClassifyError=%v want ErrorClassDeviceLost", ClassifyError(err))
	}
}

func TestDeviceLostSticky_BlocksCreateCommandEncoderAndSubmit(t *testing.T) {
	const h uintptr = 0xdeadbeef
	d := testDevice(h)
	t.Cleanup(func() { unregisterLiveDevice(d) })
	q := &Queue{handle: 0xabc, device: h}

	// Before mark: gate is clear (no native call yet).
	if err := gateDevice("CreateCommandEncoder", d); err != nil {
		t.Fatalf("unexpected gate before mark: %v", err)
	}

	markDeviceLost(h)

	enc, err := d.CreateCommandEncoder(nil)
	if enc != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateCommandEncoder after lost: enc=%v err=%v", enc, err)
	}
	if ClassifyError(err) != ErrorClassDeviceLost {
		t.Fatalf("ClassifyError=%v", ClassifyError(err))
	}

	// Queue.Submit must refuse without purego Call.
	idx, err := q.Submit(&CommandBuffer{handle: 1})
	if idx != 0 || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("Submit after lost: idx=%d err=%v", idx, err)
	}

	// WriteBuffer / CreateQuerySet / CreateSampler / CreateShaderModuleWGSL
	if err := q.WriteBuffer(&Buffer{handle: 1}, 0, []byte{1}); !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("WriteBuffer after lost: %v", err)
	}
	qs, err := d.CreateQuerySet(&QuerySetDescriptor{Type: 1, Count: 1})
	if qs != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateQuerySet after lost: %v %v", qs, err)
	}
	sm, err := d.CreateSampler(&SamplerDescriptor{Anisotropy: 1})
	if sm != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateSampler after lost: %v %v", sm, err)
	}
	mod, err := d.CreateShaderModuleWGSL("@vertex fn vs() {}")
	if mod != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateShaderModuleWGSL after lost: %v %v", mod, err)
	}
	// Device.Queue / Poll must not purego-call after sticky.
	if got := d.Queue(); got != nil {
		t.Fatal("Queue() after lost must return nil")
	}
	if !d.Poll(false) {
		t.Fatal("Poll after lost should report empty (true) without native call")
	}
}

func TestWGPUErrorIs_MessageOnlySentinel(t *testing.T) {
	err := &WGPUError{Op: "CreateTexture", Type: ErrorTypeUnknown, Message: "device lost"}
	if !errors.Is(err, ErrDeviceLost) {
		t.Fatal("message-only ErrDeviceLost should match Op-tagged error")
	}
	err2 := &WGPUError{Op: "x", Message: "invalid handle"}
	if !errors.Is(err2, ErrInvalidHandle) {
		t.Fatal("message-only ErrInvalidHandle should match")
	}
}

func TestErrorClass_String(t *testing.T) {
	if ErrorClassDeviceLost.String() != "device_lost" {
		t.Fatal(ErrorClassDeviceLost.String())
	}
	if ErrorClassInvalidHandle.String() != "invalid_handle" {
		t.Fatal(ErrorClassInvalidHandle.String())
	}
}
