//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

func createNativeTestDevice(t *testing.T) (hal.Device, hal.Queue, func()) {
	t.Helper()

	instance, err := webgpu.CreateInstance(&hal.InstanceDescriptor{
		Backends: types.BackendsPrimary,
	})
	if err != nil {
		// "unavailable" here is usually NOT a missing library: under VRAM
		// pressure wgpu wraps "Not enough memory left" into a Validation
		// Error at any of these three stages (A3 dissection: 46 P1.3 skips
		// were all OOM; the same failure surfaces at CreateBuffer on a hot
		// machine). Retry standalone or free VRAM before treating it as an
		// environment gap.
		t.Skipf("webgpu CreateInstance unavailable (often VRAM OOM wrapped as Validation Error, see A3): %v", err)
	}

	adapter, err := instance.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference: types.PowerPreferenceHighPerformance,
	})
	if err != nil {
		instance.Release()
		t.Skipf("webgpu RequestAdapter unavailable (often VRAM OOM wrapped as Validation Error, see A3): %v", err)
	}

	device, err := adapter.RequestDevice(render.DeviceDescriptor("render-gpu-test"))
	if err != nil {
		adapter.Release()
		instance.Release()
		t.Skipf("webgpu RequestDevice unavailable (often VRAM OOM wrapped as Validation Error, see A3): %v", err)
	}

	queue := device.Queue()
	cleanup := func() {
		device.Release()
		adapter.Release()
		instance.Release()
	}
	return device, queue, cleanup
}

func createNativeDevice(t *testing.T) (hal.Device, hal.Queue, func()) {
	t.Helper()
	return createNativeTestDevice(t)
}
