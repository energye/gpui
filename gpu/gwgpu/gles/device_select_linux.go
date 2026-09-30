//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

//
// The render layer (ResolveAdapterPolicy + RequestAdapterWithPolicy) already
// speaks PowerPreference; this file makes the gles backend answer honestly:
// HighPerformance → NVIDIA device, LowPower → Mesa hardware device,
// ForceFallbackAdapter → software device, None → the live default-display
// adapter (existing behavior). Cross-family fallback stays in render
// (presentLevels); each preference here is strict and errors when its
// family has no usable device, so the render chain can try the next level.
//
// Only the picked device pays for a context (B-lite): enumeration returns
// info-only entries (correct family types, no contexts), materialize builds
// a display + context for the winner. Instance owns materialized contexts
// (devCtxs, drained in Release); Adapter.Release stays a no-op like the
// shared contexts.

type deviceFamily int

const (
	familyOther deviceFamily = iota
	familyNVIDIA
	familyMesaHW
	familySoftware
)

func (f deviceFamily) String() string {
	switch f {
	case familyNVIDIA:
		return "nvidia"
	case familyMesaHW:
		return "mesa-hw"
	case familySoftware:
		return "software"
	default:
		return "other"
	}
}

func deviceFamilyOf(d egl.DeviceInfo) deviceFamily {
	switch {
	case d.IsSoftware:
		return familySoftware
	case d.IsNVIDIA:
		return familyNVIDIA
	case d.IsMesa:
		return familyMesaHW
	default:
		return familyOther
	}
}

// liveFamilyOf maps a live GL context's strings to its family.
func liveFamilyOf(vendor, renderer string) deviceFamily {
	if rendererIsSoftware(renderer) {
		return familySoftware
	}
	if strings.Contains(strings.ToLower(vendor), "nvidia") {
		return familyNVIDIA
	}
	if vendor == "" && renderer == "" {
		return familyOther
	}
	return familyMesaHW
}

// familyDeviceType is the honest DeviceType for an info-only entry: the
// entry has no GL context, so only the enumeration family is known.
func familyDeviceType(fam deviceFamily) gputypes.DeviceType {
	switch fam {
	case familyNVIDIA:
		return gputypes.DeviceTypeDiscreteGPU
	case familyMesaHW:
		return gputypes.DeviceTypeIntegratedGPU
	case familySoftware:
		return gputypes.DeviceTypeCPU
	default:
		return gputypes.DeviceTypeOther
	}
}

func familyVendorName(fam deviceFamily) string {
	switch fam {
	case familyNVIDIA:
		return "NVIDIA Corporation"
	case familyMesaHW, familySoftware:
		return "Mesa"
	default:
		return vendorUnknown
	}
}

// probeDeviceDisplay reports whether a device display initializes. Used at
// enumeration time to skip entries whose driver cannot serve a display
// (observed: the Mesa shadow of the NVIDIA card answers enumeration but
// fails init with a dri2-screen warning). No context is created here.
func probeDeviceDisplay(dev uintptr) bool {
	disp := egl.PlatformDisplayForDevice(dev)
	if disp == egl.NoDisplay {
		return false
	}
	var major, minor egl.EGLInt
	ok := egl.Initialize(disp, &major, &minor) != egl.False
	egl.Terminate(disp)
	return ok
}

// lazyDeviceAdapter builds the info-only enumeration entry for one device:
// correct family type and DRM identity, no GL context. Open on it fails
// descriptively; live adapters come from RequestAdapter (materialize).
func lazyDeviceAdapter(d egl.DeviceInfo) hal.ExposedAdapter {
	fam := deviceFamilyOf(d)
	name := d.Name
	if name == "" {
		name = "EGL software device"
	}
	return hal.ExposedAdapter{
		Adapter: &Adapter{eglDevice: d.Device, renderer: name},
		Info: gputypes.AdapterInfo{
			Name:       name,
			Vendor:     familyVendorName(fam),
			DeviceType: familyDeviceType(fam),
			Driver:     driverOpenGL,
			DriverInfo: name,
			Backend:    gputypes.BackendGL,
		},
		Capabilities: hal.Capabilities{
			Limits: gputypes.DefaultLimits(),
			AlignmentsMask: hal.Alignments{
				BufferCopyOffset: 4,
				BufferCopyPitch:  256,
			},
		},
	}
}

// enumerateDeviceAdapters lists info-only entries for usable EGL devices,
// skipping the live context's family (already represented live) and
// entries whose display fails init. Empty when enumeration is unsupported
// (fail-open: callers keep the live/placeholder path).
func (i *Instance) enumerateDeviceAdapters(liveFam deviceFamily) []hal.ExposedAdapter {
	var out []hal.ExposedAdapter
	for _, d := range egl.QueryDevices() {
		if d.Device == 0 || deviceFamilyOf(d) == liveFam {
			continue
		}
		if !probeDeviceDisplay(d.Device) {
			continue
		}
		out = append(out, lazyDeviceAdapter(d))
	}
	return out
}

// liveAdapterIf returns the live default-display adapter when it exists and
// matches want. Lets preference picks reuse the existing context instead of
func (i *Instance) liveAdapterIf(want deviceFamily) *hal.ExposedAdapter {
	if i == nil || i.ctx == nil || i.ctx.GL() == nil {
		return nil
	}
	out := makeAdapterFromContext(i.ctx)
	if liveFamilyOf(out.Info.Vendor, out.Info.Name) != want {
		return nil
	}
	return &out
}

// materializeDeviceAdapter opens a display + context on the first usable
// device of a family and returns the live adapter with true GL strings and
// probed caps. Ownership goes to Instance.devCtxs (drained in Release).
func (i *Instance) materializeDeviceAdapter(want deviceFamily) (hal.Adapter, error) {
	var lastErr error
	for _, d := range egl.QueryDevices() {
		if d.Device == 0 || deviceFamilyOf(d) != want {
			continue
		}
		a, err := i.openDeviceOnDisplay(d)
		if err != nil {
			lastErr = err
			continue
		}
		if want == familyNVIDIA && a.Info().DeviceType == gputypes.DeviceTypeCPU {
			continue
		}
		return a, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no enumerated device")
	}
	return nil, fmt.Errorf("gles: no usable %s adapter: %w", want, lastErr)
}

// openDeviceOnDisplay builds a live adapter on one enumerated device.
func (i *Instance) openDeviceOnDisplay(d egl.DeviceInfo) (hal.Adapter, error) {
	disp := egl.PlatformDisplayForDevice(d.Device)
	if disp == egl.NoDisplay {
		return nil, fmt.Errorf("no platform display")
	}
	config := egl.DefaultContextConfig()
	config.GLES = false
	if deviceFamilyOf(d) == familySoftware {
		config.AllowSoftwareConfigs = true
	}
	eglCtx, err := egl.NewContextOnDisplay(disp, config)
	if err != nil {
		return nil, err
	}
	if err := eglCtx.MakeCurrent(); err != nil {
		eglCtx.Destroy()
		return nil, err
	}
	glCtx := &gl.Context{}
	if err := glCtx.Load(egl.GetGLProcAddress, config.GLES); err != nil {
		eglCtx.Destroy()
		return nil, err
	}
	version := glCtx.GetString(gl.VERSION)
	renderer := glCtx.GetString(gl.RENDERER)
	hal.Logger().Info("gles: device adapter materialized",
		"version", version, "renderer", renderer, "device", d.Name)
	// Probe caps while current (unbound contexts answer every GL query
	// with silent empty values — LockMakeCurrentErr discipline).
	caps := queryAdapterCapabilities(glCtx)
	// Same unbind as the other creation paths: don't pin the new context
	// to the creation thread.
	_ = egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, true)
	i.devCtxs = append(i.devCtxs, actx)
	return &Adapter{
		ctx:       actx,
		eglDevice: d.Device,
		version:   version,
		renderer:  renderer,
		caps:      caps,
	}, nil
}
