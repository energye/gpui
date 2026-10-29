//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"fmt"
	"time"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// requestDeviceWithRetry retries adapter.RequestDevice a few times when the
// adapter is temporarily out of GPU memory (multi-window shared stolen-memory
// budget on iGPUs). Other windows/processes may release memory between
// retries; this mirrors the degrade-not-crash behavior on transient
// resource pressure instead of failing the app outright.
//
// The single device-request retry implementation (P2.4a): the present chain
// and the standalone entry share it. Error texts are OOM-classified through
// IsGPUOutOfMemory on the wrapped chain, so callers' downgrade ladders keep
// working across the %w wrapping.
func requestDeviceWithRetry(adapter hal.Adapter, desc *hal.DeviceDescriptor, label string) (hal.Device, error) {
	if adapter == nil {
		return nil, fmt.Errorf("adapter is nil")
	}
	var device hal.Device
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		device, err = adapter.RequestDevice(desc)
		if err == nil {
			return device, nil
		}
		if !IsGPUOutOfMemory(err) {
			return nil, err
		}
		// Device creation is not on a hot path; a short backoff gives
		// other processes time to release GPU memory.
		time.Sleep(time.Second)
	}
	return nil, err
}

// StandaloneDevice is one standalone GPU chain opened by
// OpenStandaloneDevice. The caller owns every handle: Release/Destroy them
// in reverse order (Device, Adapter, Instance) when done — the 资源谁建谁放
// ledger counts them like any other resource.
type StandaloneDevice struct {
	Instance hal.Instance
	Adapter  hal.Adapter
	Device   hal.Device

	// ForceFallback reports that only the software/CPU adapter answered.
	ForceFallback bool
}

// OpenStandaloneDevice opens the one standalone GPU device for
// probe/offscreen/compute use (VelloAccelerator standalone, GPUShared
// standalone, test probes). This is the single non-present creation funnel:
// instance creation goes through backend.go's createNativeInstance (the
// only webgpu.CreateInstance sentence), adapter selection follows the
// window policy (RequestAdapterWithPolicy + ResolveAdapterPolicy), and the
// device request reuses the present chain's OOM retry.
//
// Zero-behavior notes (P2.4a):
//   - Backends stay pinned to Vulkan exactly as the previous per-callsite
//     webgpu.CreateInstance(BackendsVulkan) did; GPUI_BACKEND deliberately
//     does not hijack standalone opens (pure-Go GL has no surfaceless
//     standalone — GL users must go through a window's shared device).
//   - The descriptor is the plain DeviceDescriptor; the present chain's
//     waterline-aware DeviceDescriptorForAdapter stays present-only until
//     P2.4b unifies budget semantics.
//   - No waitDeviceReady gate: that is a present-path heap-reclaim gate,
//     and standalone opens never had it.
//
// On error every partially opened handle is released and nil handles are
// returned; error texts match the previous per-callsite wrapping
// ("create instance" / "request adapter" / "request device").
func OpenStandaloneDevice(label string) (*StandaloneDevice, error) {
	instance, err := createNativeInstance(types.BackendsVulkan)
	if err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	adapter, forceFallback, err := RequestAdapterWithPolicy(instance, nil, ResolveAdapterPolicy())
	if err != nil {
		instance.Release()
		return nil, fmt.Errorf("request adapter: %w", err)
	}
	device, err := requestDeviceWithRetry(adapter, DeviceDescriptor(label), label)
	if err != nil {
		adapter.Release()
		instance.Release()
		return nil, fmt.Errorf("request device: %w", err)
	}
	return &StandaloneDevice{
		Instance:      instance,
		Adapter:       adapter,
		Device:        device,
		ForceFallback: forceFallback,
	}, nil
}
