//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

func createNativeTestDevice(t *testing.T) (*webgpu.Device, *webgpu.Queue, func()) {
	t.Helper()

	instance, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{
		Backends: types.BackendsPrimary,
	})
	if err != nil {
		t.Skipf("webgpu CreateInstance unavailable: %v", err)
	}

	adapter, err := instance.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference: types.PowerPreferenceHighPerformance,
	})
	if err != nil {
		instance.Release()
		t.Skipf("webgpu RequestAdapter unavailable: %v", err)
	}

	device, err := adapter.RequestDevice(renderDeviceDescriptor("render-gpu-test"))
	if err != nil {
		adapter.Release()
		instance.Release()
		t.Skipf("webgpu RequestDevice unavailable: %v", err)
	}

	queue := device.Queue()
	cleanup := func() {
		device.Release()
		adapter.Release()
		instance.Release()
	}
	return device, queue, cleanup
}

func createNativeDevice(t *testing.T) (*webgpu.Device, *webgpu.Queue, func()) {
	t.Helper()
	return createNativeTestDevice(t)
}
