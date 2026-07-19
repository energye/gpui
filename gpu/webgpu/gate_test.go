//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"testing"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// lostTestDevice builds a facade Device with a sticky-lost rwgpu parent.
// No native library required — MarkLost is pure Go state.
func lostTestDevice(handle uintptr) *Device {
	rd := &rwgpu.Device{}
	// Set handle via MarkLost path: MarkLost only stores lost flag + handle map
	// when handle != 0. Use a thin registered path through MarkLost after
	// constructing with non-zero handle via CreateBuffer-style shell.
	// Device.handle is unexported; MarkLost still sets d.lost when handle is 0.
	// IsLost only needs d.lost — inject via MarkLost.
	rd.MarkLost()
	dev := &Device{r: rd}
	dev.queue = &Queue{r: &rwgpu.Queue{}, device: dev}
	return dev
}

func TestPrepareDeviceCall_NilReleasedLost(t *testing.T) {
	if err := prepareDeviceCall(nil); err == nil {
		t.Fatal("nil device must error")
	}

	d := &Device{released: true, r: &rwgpu.Device{}}
	if !errors.Is(prepareDeviceCall(d), ErrReleased) {
		t.Fatalf("released: %v", prepareDeviceCall(d))
	}

	d = &Device{r: nil}
	if !errors.Is(prepareDeviceCall(d), ErrInvalidHandle) {
		t.Fatalf("nil handle: %v", prepareDeviceCall(d))
	}

	// Sticky lost must return webgpu.ErrDeviceLost (facade contract).
	d = lostTestDevice(0xface)
	if !errors.Is(prepareDeviceCall(d), ErrDeviceLost) {
		t.Fatalf("lost: %v want ErrDeviceLost", prepareDeviceCall(d))
	}
	if !d.IsLost() {
		t.Fatal("Device.IsLost must be true after MarkLost")
	}
}

func TestPrepareQueueCall_LostReturnsErrDeviceLost(t *testing.T) {
	d := lostTestDevice(0xbeef)
	if err := prepareQueueCall(d.queue); !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("lost queue: %v want ErrDeviceLost", err)
	}
}

func TestDeviceCreateBuffer_NilDevice(t *testing.T) {
	var d *Device
	buf, err := d.CreateBuffer(&BufferDescriptor{Size: 16, Usage: 1})
	if buf != nil || err == nil {
		t.Fatalf("nil device CreateBuffer: buf=%v err=%v", buf, err)
	}
}

func TestDeviceCreateBuffer_Released(t *testing.T) {
	d := &Device{released: true, r: &rwgpu.Device{}}
	buf, err := d.CreateBuffer(&BufferDescriptor{Size: 16, Usage: 1})
	if buf != nil || !errors.Is(err, ErrReleased) {
		t.Fatalf("released CreateBuffer: buf=%v err=%v", buf, err)
	}
}

func TestDeviceCreate_LostReturnsErrDeviceLost(t *testing.T) {
	d := lostTestDevice(0x1001)

	buf, err := d.CreateBuffer(&BufferDescriptor{Size: 16, Usage: 1})
	if buf != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateBuffer after lost: buf=%v err=%v", buf, err)
	}

	tex, err := d.CreateTexture(&TextureDescriptor{
		Size: Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
	})
	if tex != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateTexture after lost: tex=%v err=%v", tex, err)
	}

	mod, err := d.CreateShaderModule(&ShaderModuleDescriptor{WGSL: "@vertex fn vs() {}"})
	if mod != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateShaderModule after lost: mod=%v err=%v", mod, err)
	}

	sm, err := d.CreateSampler(nil)
	if sm != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("CreateSampler after lost: sm=%v err=%v", sm, err)
	}
}

func TestQueueSubmit_LostReturnsErrDeviceLost(t *testing.T) {
	d := lostTestDevice(0x1002)
	idx, err := d.queue.Submit()
	if idx != 0 || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("Submit after lost: idx=%d err=%v", idx, err)
	}
	if err := d.queue.WriteBuffer(&Buffer{r: &rwgpu.Buffer{}}, 0, []byte{1}); !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("WriteBuffer after lost: %v", err)
	}
}

func TestSurfacePresent_LostReturnsErrDeviceLost(t *testing.T) {
	d := lostTestDevice(0x1003)
	s := &Surface{
		r:      &rwgpu.Surface{},
		device: d,
	}
	if err := s.Present(&SurfaceTexture{}); !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("Present after lost: %v", err)
	}
	st, _, err := s.GetCurrentTexture()
	if st != nil || !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("GetCurrentTexture after lost: st=%v err=%v", st, err)
	}
}

func TestDeviceCreateTexture_NilDevice(t *testing.T) {
	var d *Device
	tex, err := d.CreateTexture(&TextureDescriptor{
		Size: Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
	})
	if tex != nil || err == nil {
		t.Fatalf("nil device CreateTexture: tex=%v err=%v", tex, err)
	}
}

func TestQueueSubmit_NilAndReleased(t *testing.T) {
	var q *Queue
	if _, err := q.Submit(); err == nil {
		t.Fatal("nil queue Submit must error")
	}
	q = &Queue{released: true, r: &rwgpu.Queue{}}
	if _, err := q.Submit(); !errors.Is(err, ErrReleased) {
		t.Fatalf("released Submit: %v", err)
	}
}

func TestSurfaceGate_NilReleasedPresent(t *testing.T) {
	var s *Surface
	if err := s.Present(nil); err == nil {
		t.Fatal("nil surface Present must error")
	}
	s = &Surface{released: true, r: &rwgpu.Surface{}}
	if err := s.Present(&SurfaceTexture{}); !errors.Is(err, ErrReleased) {
		t.Fatalf("released Present: %v", err)
	}
	// Unconfigure / Release must not panic on nil/released.
	s.Unconfigure()
	s.Release()
	var nilSurf *Surface
	nilSurf.Unconfigure()
	nilSurf.Release()
}

func TestSurfaceGetCurrentTexture_Released(t *testing.T) {
	s := &Surface{released: true, r: &rwgpu.Surface{}}
	st, _, err := s.GetCurrentTexture()
	if st != nil || !errors.Is(err, ErrReleased) {
		t.Fatalf("released GetCurrentTexture: %v %v", st, err)
	}
}

func TestDeviceRelease_NilSafeIdempotent(t *testing.T) {
	var d *Device
	d.Release()
	d = &Device{released: false, r: nil}
	d.Release()
	d.Release() // second call no-op
	if !d.released {
		t.Fatal("Release must set released")
	}
}

func TestMapRWGPUErr_DeviceLost(t *testing.T) {
	if !errors.Is(mapRWGPUErr(rwgpu.ErrDeviceLost), ErrDeviceLost) {
		t.Fatal("mapRWGPUErr must rewrite rwgpu.ErrDeviceLost")
	}
	if !errors.Is(mapRWGPUErr(rwgpu.ErrSurfaceDeviceLost), ErrDeviceLost) {
		t.Fatal("mapRWGPUErr must rewrite ErrSurfaceDeviceLost")
	}
}
