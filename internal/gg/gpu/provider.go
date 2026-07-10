package gpu

import (
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// deviceProvider implements gpucontext.DeviceProvider using a wgpu device.
type deviceProvider struct {
	adapter *wgpu.Adapter
	device  *wgpu.Device
	queue   *wgpu.Queue
	info    gpucontext.AdapterInfo
}

func (p *deviceProvider) Device() gpucontext.Device {
	return wgpu.DeviceToHandle(p.device)
}

func (p *deviceProvider) Queue() gpucontext.Queue {
	return wgpu.QueueToHandle(p.queue)
}

func (p *deviceProvider) SurfaceFormat() gputypes.TextureFormat {
	return gputypes.TextureFormatUndefined
}

func (p *deviceProvider) Adapter() gpucontext.Adapter {
	if p.adapter == nil {
		return gpucontext.Adapter{}
	}
	return wgpu.AdapterToHandle(p.adapter)
}

func (p *deviceProvider) AdapterInfo() gpucontext.AdapterInfo {
	return p.info
}

// InitGPU creates a headless wgpu device and configures the GPU accelerator.
func InitGPU() error {
	instance, err := wgpu.CreateInstance(nil)
	if err != nil {
		return err
	}
	defer instance.Release()

	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		return err
	}

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		return err
	}

	queue := device.Queue()

	// Build adapter info
	info := gpucontext.AdapterInfo{
		Name: "wgpu headless",
		Type: gpucontext.AdapterTypeUnknown,
	}

	provider := &deviceProvider{
		adapter: adapter,
		device:  device,
		queue:   queue,
		info:    info,
	}
	return SetDeviceProvider(provider)
}