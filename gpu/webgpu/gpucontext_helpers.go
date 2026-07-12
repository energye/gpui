package webgpu

import (
	"unsafe"

	"github.com/energye/gpui/gpu/context"
)

// Handle conversion helpers — isolate unsafe.Pointer from consumers.
// Consumers write wgpu.DeviceFromHandle(dev) instead of (*wgpu.Device)(dev.Pointer()).
// DIP: wgpu (implementation) depends on gpucontext (abstraction),
// like database/sql depends on database/sql/driver.

// DeviceFromHandle extracts *Device from a gpucontext.Device handle.
func DeviceFromHandle(h context.Device) *Device {
	if h.IsNil() {
		return nil
	}
	return (*Device)(h.Pointer())
}

// QueueFromHandle extracts *Queue from a gpucontext.Queue handle.
func QueueFromHandle(h context.Queue) *Queue {
	if h.IsNil() {
		return nil
	}
	return (*Queue)(h.Pointer())
}

// AdapterFromHandle extracts *Adapter from a gpucontext.Adapter handle.
func AdapterFromHandle(h context.Adapter) *Adapter {
	if h.IsNil() {
		return nil
	}
	return (*Adapter)(h.Pointer())
}

// SurfaceFromHandle extracts *Surface from a gpucontext.Surface handle.
func SurfaceFromHandle(h context.Surface) *Surface {
	if h.IsNil() {
		return nil
	}
	return (*Surface)(h.Pointer())
}

// InstanceFromHandle extracts *Instance from a gpucontext.Instance handle.
func InstanceFromHandle(h context.Instance) *Instance {
	if h.IsNil() {
		return nil
	}
	return (*Instance)(h.Pointer())
}

// DeviceToHandle wraps *Device into a gpucontext.Device handle.
func DeviceToHandle(d *Device) context.Device {
	return context.NewDevice(unsafe.Pointer(d)) //nolint:gosec // ADR-018 opaque handle
}

// QueueToHandle wraps *Queue into a gpucontext.Queue handle.
func QueueToHandle(q *Queue) context.Queue {
	return context.NewQueue(unsafe.Pointer(q)) //nolint:gosec // ADR-018 opaque handle
}

// AdapterToHandle wraps *Adapter into a gpucontext.Adapter handle.
func AdapterToHandle(a *Adapter) context.Adapter {
	return context.NewAdapter(unsafe.Pointer(a)) //nolint:gosec // ADR-018 opaque handle
}

// SurfaceToHandle wraps *Surface into a gpucontext.Surface handle.
func SurfaceToHandle(s *Surface) context.Surface {
	return context.NewSurface(unsafe.Pointer(s)) //nolint:gosec // ADR-018 opaque handle
}

// InstanceToHandle wraps *Instance into a gpucontext.Instance handle.
func InstanceToHandle(i *Instance) context.Instance {
	return context.NewInstance(unsafe.Pointer(i)) //nolint:gosec // ADR-018 opaque handle
}
