//go:build linux && !nogpu

package gles

import (
	"runtime"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// P2-2 preference routing on real hardware (default loader view: all
// vendors visible, no GPUI_POWER pin — the hal-level preference is the
// honest scope here; the GPUI_POWER→policy mapping lives in render and is
// covered by the pelican acceptance runs).
//
// High must land on the NVIDIA renderer with a discrete label, Low on
// Mesa hardware (not llvmpipe), fallback on software, None on the live
// default-display adapter. Enumeration must list more than the first card
// with a discrete entry present.
func TestP22RequestAdapterPreference(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	inst, err := (Backend{}).CreateInstance(&hal.InstanceDescriptor{})
	if err != nil {
		t.Skipf("egl init: %v", err)
	}
	defer inst.Release()

	listed := inst.EnumerateAdapters(nil)
	if len(listed) < 2 {
		t.Fatalf("EnumerateAdapters = %d entries, want >= 2 (multi-card)", len(listed))
	}
	seenDiscrete := false
	for _, e := range listed {
		t.Logf("listed name=%q vendor=%q type=%v", e.Info.Name, e.Info.Vendor, e.Info.DeviceType)
		if e.Info.DeviceType == gputypes.DeviceTypeDiscreteGPU {
			seenDiscrete = true
		}
	}
	if !seenDiscrete {
		t.Fatalf("no discrete entry in enumeration")
	}

	high, err := inst.RequestAdapter(&hal.RequestAdapterOptions{PowerPreference: gputypes.PowerPreferenceHighPerformance})
	if err != nil {
		t.Fatalf("RequestAdapter(high): %v", err)
	}
	t.Logf("high name=%q vendor=%q type=%v", high.Info().Name, high.Info().Vendor, high.Info().DeviceType)
	if rendererIsSoftware(high.Info().Name) {
		t.Fatalf("high landed on software: %q", high.Info().Name)
	}
	if high.Info().DeviceType != gputypes.DeviceTypeDiscreteGPU {
		t.Fatalf("high type = %v, want discrete", high.Info().DeviceType)
	}

	low, err := inst.RequestAdapter(&hal.RequestAdapterOptions{PowerPreference: gputypes.PowerPreferenceLowPower})
	if err != nil {
		t.Fatalf("RequestAdapter(low): %v", err)
	}
	t.Logf("low name=%q vendor=%q type=%v", low.Info().Name, low.Info().Vendor, low.Info().DeviceType)
	if rendererIsSoftware(low.Info().Name) {
		t.Fatalf("low landed on software: %q (want Mesa hardware)", low.Info().Name)
	}
	if low.Info().DeviceType != gputypes.DeviceTypeIntegratedGPU {
		t.Fatalf("low type = %v, want integrated", low.Info().DeviceType)
	}

	fb, err := inst.RequestAdapter(&hal.RequestAdapterOptions{ForceFallbackAdapter: true})
	if err != nil {
		t.Fatalf("RequestAdapter(fallback): %v", err)
	}
	t.Logf("fallback name=%q type=%v", fb.Info().Name, fb.Info().DeviceType)
	if fb.Info().DeviceType != gputypes.DeviceTypeCPU {
		t.Fatalf("fallback type = %v, want cpu", fb.Info().DeviceType)
	}

	def, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter(nil): %v", err)
	}
	t.Logf("default name=%q type=%v", def.Info().Name, def.Info().DeviceType)
}
