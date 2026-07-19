//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"fmt"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// prepareDeviceCall enforces the facade defense order for Device APIs:
// nil → released → invalid handle → lost.
// Library APIs return error/no-op rather than panicking.
func prepareDeviceCall(d *Device) error {
	if d == nil {
		return fmt.Errorf("wgpu: device is nil")
	}
	if d.released {
		return ErrReleased
	}
	if d.r == nil {
		return ErrInvalidHandle
	}
	if d.IsLost() {
		return ErrDeviceLost
	}
	return nil
}

// prepareQueueCall enforces nil → released → invalid handle → lost for Queue APIs.
// Returns webgpu.ErrDeviceLost (not the raw rwgpu sentinel) for facade callers.
func prepareQueueCall(q *Queue) error {
	if q == nil {
		return fmt.Errorf("wgpu: queue is nil")
	}
	if q.released {
		return ErrReleased
	}
	if q.r == nil {
		return ErrInvalidHandle
	}
	if q.device != nil && q.device.IsLost() {
		return ErrDeviceLost
	}
	return nil
}

// mapRWGPUErr rewrites rwgpu device-lost / invalid-handle sentinels to webgpu
// public errors so facade callers can errors.Is against ErrDeviceLost.
func mapRWGPUErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrDeviceLost) {
		return ErrDeviceLost
	}
	if errors.Is(err, rwgpu.ErrDeviceLost) || errors.Is(err, rwgpu.ErrSurfaceDeviceLost) {
		return ErrDeviceLost
	}
	if errors.Is(err, rwgpu.ErrInvalidHandle) {
		return ErrInvalidHandle
	}
	return err
}

// prepareSurfaceCall enforces nil → released → invalid handle → lost for Surface APIs.
func prepareSurfaceCall(s *Surface) error {
	if s == nil {
		return fmt.Errorf("wgpu: surface is nil")
	}
	if s.released {
		return ErrReleased
	}
	if s.r == nil {
		return ErrInvalidHandle
	}
	if s.device != nil && s.device.IsLost() {
		return ErrDeviceLost
	}
	return nil
}

// prepareResourceCall enforces nil receiver and released flag for generic resources.
func prepareResourceCall(released bool, name string) error {
	if released {
		return ErrReleased
	}
	if name == "" {
		name = "resource"
	}
	return nil
}
