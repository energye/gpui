//go:build !(js && wasm)

package core

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

func TestNewInstance(t *testing.T) {
	tests := []struct {
		name string
		desc *types.InstanceDescriptor
		want types.Backends
	}{
		{
			name: "nil descriptor uses defaults",
			desc: nil,
			want: types.BackendsPrimary,
		},
		{
			name: "custom backends",
			desc: &types.InstanceDescriptor{
				Backends: types.BackendsVulkan | types.BackendsMetal,
				Flags:    types.InstanceFlagsDebug,
			},
			want: types.BackendsVulkan | types.BackendsMetal,
		},
		{
			name: "all backends",
			desc: &types.InstanceDescriptor{
				Backends: types.BackendsAll,
				Flags:    0,
			},
			want: types.BackendsAll,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear global state before each test
			GetGlobal().Clear()

			instance := NewInstance(tt.desc)
			if instance == nil {
				t.Fatal("NewInstance returned nil")
			}

			got := instance.Backends()
			if got != tt.want {
				t.Errorf("Backends() = %v, want %v", got, tt.want)
			}

			// Verify mock adapter was created
			adapters := instance.EnumerateAdapters()
			if len(adapters) == 0 {
				t.Error("Expected at least one mock adapter")
			}
		})
	}
}

func TestInstanceFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags types.InstanceFlags
	}{
		{
			name:  "no flags",
			flags: 0,
		},
		{
			name:  "debug flag",
			flags: types.InstanceFlagsDebug,
		},
		{
			name:  "validation flag",
			flags: types.InstanceFlagsValidation,
		},
		{
			name:  "multiple flags",
			flags: types.InstanceFlagsDebug | types.InstanceFlagsValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GetGlobal().Clear()

			desc := types.InstanceDescriptor{
				Backends: types.BackendsPrimary,
				Flags:    tt.flags,
			}
			instance := NewInstance(&desc)

			got := instance.Flags()
			if got != tt.flags {
				t.Errorf("Flags() = %v, want %v", got, tt.flags)
			}
		})
	}
}

func TestEnumerateAdapters(t *testing.T) {
	GetGlobal().Clear()

	instance := NewInstance(nil)

	adapters := instance.EnumerateAdapters()
	if len(adapters) == 0 {
		t.Fatal("EnumerateAdapters returned empty list")
	}

	// Verify we get a copy, not a reference to internal state
	adapters1 := instance.EnumerateAdapters()
	adapters2 := instance.EnumerateAdapters()

	if &adapters1[0] == &adapters2[0] {
		t.Error("EnumerateAdapters should return a copy, not a reference")
	}

	// Verify adapter IDs are valid
	for i, adapterID := range adapters {
		if adapterID.IsZero() {
			t.Errorf("adapter %d has zero ID", i)
		}
	}
}

func TestRequestAdapter(t *testing.T) {
	tests := []struct {
		name    string
		options *types.RequestAdapterOptions
		wantErr bool
	}{
		{
			name:    "nil options returns first adapter",
			options: nil,
			wantErr: false,
		},
		{
			name: "no power preference",
			options: &types.RequestAdapterOptions{
				PowerPreference: types.PowerPreferenceNone,
			},
			wantErr: false,
		},
		{
			name: "high performance preference",
			options: &types.RequestAdapterOptions{
				PowerPreference: types.PowerPreferenceHighPerformance,
			},
			wantErr: false, // Mock adapter is discrete GPU
		},
		{
			name: "low power preference",
			options: &types.RequestAdapterOptions{
				PowerPreference: types.PowerPreferenceLowPower,
			},
			wantErr: false, // WebGPU spec: powerPreference is a hint, falls back to discrete GPU
		},
		{
			name: "force fallback adapter",
			options: &types.RequestAdapterOptions{
				ForceFallbackAdapter: true,
			},
			wantErr: true, // Mock adapter is not CPU
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GetGlobal().Clear()

			instance := NewInstance(nil)
			adapterID, err := instance.RequestAdapter(tt.options)

			if tt.wantErr {
				if err == nil {
					t.Error("RequestAdapter() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("RequestAdapter() unexpected error: %v", err)
				return
			}

			if adapterID.IsZero() {
				t.Error("RequestAdapter() returned zero ID")
			}

			// Verify the adapter exists
			hub := GetGlobal().Hub()
			_, err = hub.GetAdapter(adapterID)
			if err != nil {
				t.Errorf("returned adapter ID is invalid: %v", err)
			}
		})
	}
}

func TestRequestAdapterNoAdapters(t *testing.T) {
	GetGlobal().Clear()

	// Create instance but remove mock adapter
	instance := &Instance{
		backends: types.BackendsPrimary,
		flags:    0,
		adapters: []AdapterID{}, // Empty adapter list
	}

	_, err := instance.RequestAdapter(nil)
	if err == nil {
		t.Error("RequestAdapter() should fail when no adapters available")
	}
}

func TestMatchesPowerPreference(t *testing.T) {
	tests := []struct {
		name       string
		deviceType types.DeviceType
		preference types.PowerPreference
		want       bool
	}{
		{
			name:       "integrated GPU with low power preference",
			deviceType: types.DeviceTypeIntegratedGPU,
			preference: types.PowerPreferenceLowPower,
			want:       true,
		},
		{
			name:       "discrete GPU with low power preference",
			deviceType: types.DeviceTypeDiscreteGPU,
			preference: types.PowerPreferenceLowPower,
			want:       false,
		},
		{
			name:       "discrete GPU with high performance preference",
			deviceType: types.DeviceTypeDiscreteGPU,
			preference: types.PowerPreferenceHighPerformance,
			want:       true,
		},
		{
			name:       "integrated GPU with high performance preference",
			deviceType: types.DeviceTypeIntegratedGPU,
			preference: types.PowerPreferenceHighPerformance,
			want:       false,
		},
		{
			name:       "any GPU with no preference",
			deviceType: types.DeviceTypeDiscreteGPU,
			preference: types.PowerPreferenceNone,
			want:       true,
		},
		{
			name:       "CPU with low power preference",
			deviceType: types.DeviceTypeCPU,
			preference: types.PowerPreferenceLowPower,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPowerPreference(tt.deviceType, tt.preference)
			if got != tt.want {
				t.Errorf("matchesPowerPreference(%v, %v) = %v, want %v",
					tt.deviceType, tt.preference, got, tt.want)
			}
		})
	}
}

func TestInstanceConcurrentAccess(t *testing.T) {
	GetGlobal().Clear()

	instance := NewInstance(nil)

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = instance.EnumerateAdapters()
			_ = instance.Backends()
			_ = instance.Flags()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRequestAdapterPrefersGPUOverCPU(t *testing.T) {
	GetGlobal().Clear()

	instance := &Instance{
		backends: types.BackendsAll,
	}

	hub := GetGlobal().Hub()

	// Register CPU adapter first (like Software backend)
	cpuAdapter := &Adapter{
		Info: types.AdapterInfo{
			Name:       "Software Renderer",
			DeviceType: types.DeviceTypeCPU,
			Backend:    types.BackendVulkan,
		},
		Limits: types.DefaultLimits(),
	}
	cpuID := hub.RegisterAdapter(cpuAdapter)
	instance.adapters = append(instance.adapters, cpuID)

	// Register GPU adapter second (like GLES deferred)
	gpuAdapter := &Adapter{
		Info: types.AdapterInfo{
			Name:       "GLES GPU",
			DeviceType: types.DeviceTypeIntegratedGPU,
			Backend:    types.BackendGL,
		},
		Limits: types.DefaultLimits(),
	}
	gpuID := hub.RegisterAdapter(gpuAdapter)
	instance.adapters = append(instance.adapters, gpuID)

	// nil options should prefer GPU over CPU
	adapterID, err := instance.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter(nil) error: %v", err)
	}
	adapter, _ := hub.GetAdapter(adapterID)
	if adapter.Info.DeviceType == types.DeviceTypeCPU {
		t.Errorf("RequestAdapter(nil) returned CPU adapter, want GPU")
	}
	if adapter.Info.Name != "GLES GPU" {
		t.Errorf("RequestAdapter(nil) returned %q, want %q", adapter.Info.Name, "GLES GPU")
	}

	// options with no preference should also prefer GPU
	adapterID2, err := instance.RequestAdapter(&types.RequestAdapterOptions{})
	if err != nil {
		t.Fatalf("RequestAdapter({}) error: %v", err)
	}
	adapter2, _ := hub.GetAdapter(adapterID2)
	if adapter2.Info.DeviceType == types.DeviceTypeCPU {
		t.Errorf("RequestAdapter({}) returned CPU adapter, want GPU")
	}
}

func TestRequestAdapterForceFallbackReturnsCPU(t *testing.T) {
	GetGlobal().Clear()

	instance := &Instance{
		backends: types.BackendsAll,
	}

	hub := GetGlobal().Hub()

	gpuAdapter := &Adapter{
		Info: types.AdapterInfo{
			Name:       "Vulkan GPU",
			DeviceType: types.DeviceTypeDiscreteGPU,
		},
		Limits: types.DefaultLimits(),
	}
	gpuID := hub.RegisterAdapter(gpuAdapter)
	instance.adapters = append(instance.adapters, gpuID)

	cpuAdapter := &Adapter{
		Info: types.AdapterInfo{
			Name:       "Software Renderer",
			DeviceType: types.DeviceTypeCPU,
		},
		Limits: types.DefaultLimits(),
	}
	cpuID := hub.RegisterAdapter(cpuAdapter)
	instance.adapters = append(instance.adapters, cpuID)

	adapterID, err := instance.RequestAdapter(&types.RequestAdapterOptions{
		ForceFallbackAdapter: true,
	})
	if err != nil {
		t.Fatalf("ForceFallbackAdapter error: %v", err)
	}
	adapter, _ := hub.GetAdapter(adapterID)
	if adapter.Info.DeviceType != types.DeviceTypeCPU {
		t.Errorf("ForceFallbackAdapter returned %v, want CPU", adapter.Info.DeviceType)
	}
}

func TestRequestAdapterOnlyCPUAvailable(t *testing.T) {
	GetGlobal().Clear()

	instance := &Instance{
		backends: types.BackendsAll,
	}

	hub := GetGlobal().Hub()

	cpuAdapter := &Adapter{
		Info: types.AdapterInfo{
			Name:       "Software Renderer",
			DeviceType: types.DeviceTypeCPU,
		},
		Limits: types.DefaultLimits(),
	}
	cpuID := hub.RegisterAdapter(cpuAdapter)
	instance.adapters = append(instance.adapters, cpuID)

	// Should fallback to CPU when no GPU available
	adapterID, err := instance.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter(nil) with only CPU: %v", err)
	}
	adapter, _ := hub.GetAdapter(adapterID)
	if adapter.Info.Name != "Software Renderer" {
		t.Errorf("got %q, want %q", adapter.Info.Name, "Software Renderer")
	}
}
