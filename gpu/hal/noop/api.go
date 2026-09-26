//go:build !(js && wasm)

package noop

import (
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// API implements hal.Backend for the noop backend.
type Backend struct{}

// NewBackend returns a noop backend instance.
func NewBackend() Backend { return Backend{} }

// Variant returns the backend type identifier.
func (Backend) Variant() gputypes.Backend {
	return gputypes.BackendEmpty
}

// CreateInstance creates a new noop instance.
// Always succeeds and returns a placeholder instance.
func (Backend) CreateInstance(_ *hal.InstanceDescriptor) (hal.Instance, error) {
	return &Instance{}, nil
}

// Instance implements hal.Instance for the noop backend.
type Instance struct{}

// CreateSurface creates a noop surface.
// Always succeeds regardless of the target.
func (i *Instance) CreateSurface(_ hal.SurfaceTarget) (hal.Surface, error) {
	return &Surface{}, nil
}

// EnumerateAdapters returns a single default noop adapter.
// The surfaceHint is ignored.
func (i *Instance) EnumerateAdapters(_ hal.Surface) []hal.ExposedAdapter {
	return []hal.ExposedAdapter{
		{
			Adapter: &Adapter{},
			Info: gputypes.AdapterInfo{
				Name:       "Noop Adapter",
				Vendor:     "GoGPU",
				VendorID:   0,
				DeviceID:   0,
				DeviceType: gputypes.DeviceTypeOther,
				Driver:     "noop-1.0",
				DriverInfo: "No-operation backend for testing",
				Backend:    gputypes.BackendEmpty,
			},
			Features: 0, // No features supported
			Capabilities: hal.Capabilities{
				Limits: gputypes.DefaultLimits(),
				AlignmentsMask: hal.Alignments{
					BufferCopyOffset: 4,
					BufferCopyPitch:  256,
				},
				DownlevelCapabilities: gputypes.DefaultDownlevelCapabilities(),
			},
		},
	}
}

// Destroy is a no-op for the noop instance.
func (i *Instance) Destroy() {}

// RequestAdapter returns the default noop adapter.
// Matches webgpu Instance.RequestAdapter shape; options are accepted
// for API compatibility (PowerPreference/CompatibleSurface are moot —
// noop has a single adapter).
func (i *Instance) RequestAdapter(opts *hal.RequestAdapterOptions) (hal.Adapter, error) {
	var surface hal.Surface
	if opts != nil {
		surface = opts.CompatibleSurface
	}
	adapters := i.EnumerateAdapters(surface)
	if len(adapters) == 0 {
		return nil, hal.ErrBackendNotFound
	}
	return adapters[0].Adapter, nil
}

// ProcessEvents is a no-op for the noop backend (synchronous, no async callbacks).
func (i *Instance) ProcessEvents() {}
