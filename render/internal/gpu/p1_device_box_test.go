//go:build !nogpu

package gpu

import (
	"testing"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	gputypes "github.com/energye/gpui/gpu/types"
)

type p1BoxProvider struct {
	dev hal.Device
}

func (p *p1BoxProvider) Device() gpucontext.Device {
	if p == nil || p.dev == nil {
		return gpucontext.Device{}
	}
	return gpucontext.PackDevice(p.dev)
}

func (p *p1BoxProvider) Queue() gpucontext.Queue {
	if p == nil || p.dev == nil {
		return gpucontext.Queue{}
	}
	return gpucontext.PackQueue(p.dev.Queue())
}

func (p *p1BoxProvider) SurfaceFormat() gputypes.TextureFormat {
	return gputypes.TextureFormatBGRA8Unorm
}

func (p *p1BoxProvider) Adapter() gpucontext.Adapter {
	return gpucontext.Adapter{}
}

func (p *p1BoxProvider) AdapterInfo() gpucontext.AdapterInfo {
	return gpucontext.AdapterInfo{Type: gpucontext.AdapterTypeUnknown}
}

func TestP1BoxProviderSetDevice(t *testing.T) {
	be := noop.NewBackend()
	inst, err := be.CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	defer inst.Release()
	adapters := inst.EnumerateAdapters(nil)
	if len(adapters) == 0 {
		t.Fatal("noop: no adapters")
	}
	opened, err := adapters[0].Adapter.Open(0, gputypes.Limits{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer opened.Device.Release()

	s := NewGPUShared()
	if err := s.SetDeviceProvider(&p1BoxProvider{dev: opened.Device}); err != nil {
		t.Fatalf("SetDeviceProvider(boxed noop): %v", err)
	}
	if s.device == nil {
		t.Fatal("shared device is nil after boxed provider")
	}
	if s.queue == nil {
		t.Fatal("shared queue is nil after boxed provider")
	}
}
