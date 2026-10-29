//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package render

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// requestDeviceOOMErr is the exact error phrasing wgpu-native produced for
// the A3 finding (evidence/P2.4a/a3-dissection.md): an OOM at RequestDevice
// time wrapped by wgpu as a Validation Error. IsGPUOutOfMemory must classify
// it as OOM so the retry and downgrade ladders stay effective.
const requestDeviceOOMErr = "wgpu: failed to request device: wgpu: RequestDevice: Validation Error\n\nCaused by:\n  Not enough memory left."

// fakeAdapter scripts RequestDevice results; the embedded nil hal.Adapter
// fills the unused interface methods (retry tests only call RequestDevice
// and check errors/attempt counts).
type fakeAdapter struct {
	hal.Adapter

	script []error
	calls  int
}

// fakeDevice is a non-nil hal.Device marker for success returns; the
// embedded interface is never called (retry tests don't touch the device).
type fakeDevice struct{ hal.Device }

func (f *fakeAdapter) RequestDevice(_ *hal.DeviceDescriptor) (hal.Device, error) {
	if f.calls < len(f.script) {
		err := f.script[f.calls]
		f.calls++
		if err != nil {
			return nil, err
		}
		return &fakeDevice{}, nil
	}
	f.calls++
	return &fakeDevice{}, nil
}

func TestRequestDeviceWithRetry_OOMRetriesThenSucceeds(t *testing.T) {
	ad := &fakeAdapter{script: []error{
		errors.New(requestDeviceOOMErr),
		errors.New(requestDeviceOOMErr),
	}}
	dev, err := requestDeviceWithRetry(ad, DeviceDescriptor("t"), "t")
	if err != nil {
		t.Fatalf("want success after 2 OOM retries, got %v", err)
	}
	if dev == nil {
		t.Fatal("want non-nil device")
	}
	if ad.calls != 3 {
		t.Fatalf("want 3 attempts, got %d", ad.calls)
	}
}

func TestRequestDeviceWithRetry_OOMExhausted(t *testing.T) {
	ad := &fakeAdapter{script: []error{
		errors.New(requestDeviceOOMErr),
		errors.New(requestDeviceOOMErr),
		errors.New(requestDeviceOOMErr),
	}}
	_, err := requestDeviceWithRetry(ad, DeviceDescriptor("t"), "t")
	if err == nil {
		t.Fatal("want error after 3 OOM attempts")
	}
	if ad.calls != 3 {
		t.Fatalf("want exactly 3 attempts, got %d", ad.calls)
	}
	if !IsGPUOutOfMemory(err) {
		t.Fatalf("want OOM-class error, got %v", err)
	}
}

func TestRequestDeviceWithRetry_NonOOMFailsImmediately(t *testing.T) {
	ad := &fakeAdapter{script: []error{errors.New("render: device lost")}}
	_, err := requestDeviceWithRetry(ad, DeviceDescriptor("t"), "t")
	if err == nil {
		t.Fatal("want error")
	}
	if ad.calls != 1 {
		t.Fatalf("non-OOM must not retry, got %d attempts", ad.calls)
	}
	if IsGPUOutOfMemory(err) {
		t.Fatalf("non-OOM error misclassified: %v", err)
	}
}

func TestRequestDeviceWithRetry_NilAdapter(t *testing.T) {
	if _, err := requestDeviceWithRetry(nil, DeviceDescriptor("t"), "t"); err == nil {
		t.Fatal("want error for nil adapter")
	}
}

// TestOpenStandaloneDevice_UsableDevice proves the single standalone entry
// returns a usable device chain (P2.4a capability gate). Honest skips: no
// wgpu library, or GPU memory pressure (A3: RequestDevice OOM is
// environment-dependent, evidence/P2.4a/a3-dissection.md).
func TestOpenStandaloneDevice_UsableDevice(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skipf("WGPU_NATIVE_PATH unset: native wgpu library path required")
	}
	sd, err := OpenStandaloneDevice("render-device-open-test")
	if err != nil {
		if IsGPUOutOfMemory(err) {
			t.Skipf("GPU memory pressure (A3 environment class): %v", err)
		}
		t.Fatalf("OpenStandaloneDevice: %v", err)
	}
	if sd == nil || sd.Instance == nil || sd.Adapter == nil || sd.Device == nil {
		t.Fatalf("want full handle set, got %+v", sd)
	}
	// Usability probe: a 1x1 texture must create and destroy cleanly.
	tex, err := sd.Device.CreateTexture(&hal.TextureDescriptor{
		Label:         "render-device-open-test_probe",
		Size:          hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageCopySrc,
	})
	if err != nil {
		if IsGPUOutOfMemory(err) {
			t.Skipf("GPU memory pressure at probe texture: %v", err)
		}
		t.Fatalf("probe CreateTexture: %v", err)
	}
	tex.Destroy()
	// Release in reverse order (资源谁建谁放).
	sd.Device.Release()
	sd.Adapter.Release()
	sd.Instance.Release()
}

// TestOpenStandaloneDevice_ErrorTextsStable pins the wrapped error texts:
// internal consumers (gpu_shared/vello pre-convergence) and any caller
// classifying through IsGPUOutOfMemory rely on the chain, not the prefix.
func TestOpenStandaloneDevice_ErrorTextsStable(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") != "" {
		t.Skipf("error-shape check is only meaningful without a working wgpu library")
	}
	_, err := OpenStandaloneDevice("render-device-open-neg")
	if err == nil {
		t.Fatal("want error without wgpu library")
	}
	wrapped := fmt.Errorf("outer: %w", err)
	if IsGPUOutOfMemory(wrapped) {
		t.Fatalf("library-load failure must not classify as OOM: %v", err)
	}
}
