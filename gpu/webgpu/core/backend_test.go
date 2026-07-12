//go:build !(js && wasm)

package core

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu/hal"
)

// testProvider implements BackendProvider for testing.
type testProvider struct {
	variant   types.Backend
	available bool
}

func (p *testProvider) Variant() types.Backend { return p.variant }
func (p *testProvider) CreateInstance(_ *hal.InstanceDescriptor) (hal.Instance, error) {
	return nil, nil //nolint:nilnil
}
func (p *testProvider) IsAvailable() bool { return p.available }

func TestRegisterBackendProvider(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	provider := &testProvider{variant: types.BackendVulkan, available: true}
	RegisterBackendProvider(provider)

	got, ok := GetBackendProvider(types.BackendVulkan)
	if !ok {
		t.Fatal("expected provider to be registered")
	}
	if got.Variant() != types.BackendVulkan {
		t.Errorf("variant = %v, want BackendVulkan", got.Variant())
	}
}

func TestGetBackendProvider_NotRegistered(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	_, ok := GetBackendProvider(types.BackendVulkan)
	if ok {
		t.Error("expected GetBackendProvider to return false for unregistered provider")
	}
}

func TestAvailableBackendProviders(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	RegisterBackendProvider(&testProvider{variant: types.BackendVulkan, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendMetal, available: true})

	available := AvailableBackendProviders()
	if len(available) != 2 {
		t.Errorf("expected 2 providers, got %d", len(available))
	}
}

func TestGetOrderedBackendProviders(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	// Register in non-priority order
	RegisterBackendProvider(&testProvider{variant: types.BackendEmpty, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendVulkan, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendGL, available: true})

	ordered := GetOrderedBackendProviders()
	if len(ordered) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(ordered))
	}

	// Vulkan should be first (highest priority)
	if ordered[0].Variant() != types.BackendVulkan {
		t.Errorf("first provider = %v, want BackendVulkan", ordered[0].Variant())
	}
	// GL should be second
	if ordered[1].Variant() != types.BackendGL {
		t.Errorf("second provider = %v, want BackendGL", ordered[1].Variant())
	}
	// Empty should be last
	if ordered[2].Variant() != types.BackendEmpty {
		t.Errorf("third provider = %v, want BackendEmpty", ordered[2].Variant())
	}
}

func TestGetOrderedBackendProviders_SkipsUnavailable(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	RegisterBackendProvider(&testProvider{variant: types.BackendVulkan, available: false})
	RegisterBackendProvider(&testProvider{variant: types.BackendGL, available: true})

	ordered := GetOrderedBackendProviders()
	if len(ordered) != 1 {
		t.Fatalf("expected 1 available provider, got %d", len(ordered))
	}
	if ordered[0].Variant() != types.BackendGL {
		t.Errorf("expected BackendGL, got %v", ordered[0].Variant())
	}
}

func TestGetOrderedBackendProviders_CustomBackends(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	// Register a custom backend not in the priority list
	customVariant := types.Backend(42)
	RegisterBackendProvider(&testProvider{variant: customVariant, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendVulkan, available: true})

	ordered := GetOrderedBackendProviders()
	if len(ordered) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(ordered))
	}

	// Vulkan should be first (in priority list)
	if ordered[0].Variant() != types.BackendVulkan {
		t.Errorf("first = %v, want BackendVulkan", ordered[0].Variant())
	}
	// Custom should be last (not in priority list)
	if ordered[1].Variant() != customVariant {
		t.Errorf("second = %v, want custom(%d)", ordered[1].Variant(), customVariant)
	}
}

func TestSelectBestBackendProvider(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	RegisterBackendProvider(&testProvider{variant: types.BackendGL, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendVulkan, available: true})

	best := SelectBestBackendProvider()
	if best == nil {
		t.Fatal("SelectBestBackendProvider returned nil")
	}
	if best.Variant() != types.BackendVulkan {
		t.Errorf("best = %v, want BackendVulkan (highest priority)", best.Variant())
	}
}

func TestSelectBestBackendProvider_NoneAvailable(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	best := SelectBestBackendProvider()
	if best != nil {
		t.Errorf("expected nil when no providers registered, got %v", best.Variant())
	}
}

func TestFilterBackendsByMask(t *testing.T) {
	// Save and restore state
	providersMu.Lock()
	savedProviders := make(map[types.Backend]BackendProvider)
	for k, v := range providers {
		savedProviders[k] = v
	}
	providers = make(map[types.Backend]BackendProvider)
	providersMu.Unlock()
	defer func() {
		providersMu.Lock()
		providers = savedProviders
		providersMu.Unlock()
	}()

	RegisterBackendProvider(&testProvider{variant: types.BackendVulkan, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendMetal, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendDX12, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendGL, available: true})
	RegisterBackendProvider(&testProvider{variant: types.BackendEmpty, available: true})

	tests := []struct {
		name    string
		mask    types.Backends
		wantLen int
		wantHas []types.Backend
		wantNot []types.Backend
	}{
		{
			name:    "Vulkan only",
			mask:    types.BackendsVulkan,
			wantLen: 2, // Vulkan + Empty (always included as fallback)
			wantHas: []types.Backend{types.BackendVulkan, types.BackendEmpty},
			wantNot: []types.Backend{types.BackendMetal, types.BackendDX12},
		},
		{
			name:    "Metal only",
			mask:    types.BackendsMetal,
			wantLen: 2, // Metal + Empty
			wantHas: []types.Backend{types.BackendMetal, types.BackendEmpty},
		},
		{
			name:    "DX12 only",
			mask:    types.BackendsDX12,
			wantLen: 2, // DX12 + Empty
			wantHas: []types.Backend{types.BackendDX12, types.BackendEmpty},
		},
		{
			name:    "GL only",
			mask:    types.BackendsGL,
			wantLen: 2, // GL + Empty
			wantHas: []types.Backend{types.BackendGL, types.BackendEmpty},
		},
		{
			name:    "Vulkan + Metal",
			mask:    types.BackendsVulkan | types.BackendsMetal,
			wantLen: 3, // Vulkan + Metal + Empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterBackendsByMask(tt.mask)

			if len(filtered) != tt.wantLen {
				variants := make([]types.Backend, len(filtered))
				for i, p := range filtered {
					variants[i] = p.Variant()
				}
				t.Errorf("len = %d, want %d (variants: %v)", len(filtered), tt.wantLen, variants)
			}

			for _, want := range tt.wantHas {
				found := false
				for _, p := range filtered {
					if p.Variant() == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find %v in filtered results", want)
				}
			}

			for _, notWant := range tt.wantNot {
				for _, p := range filtered {
					if p.Variant() == notWant {
						t.Errorf("did not expect %v in filtered results", notWant)
					}
				}
			}
		})
	}
}

func TestHALBackendProvider(t *testing.T) {
	// Register a HAL backend and then create a provider for it
	// Since we're in the core package, we test the halBackendProvider wrapper

	mockBackend := &testHALBackend{variant: types.BackendGL}
	provider := &halBackendProvider{backend: mockBackend}

	if provider.Variant() != types.BackendGL {
		t.Errorf("Variant() = %v, want BackendGL", provider.Variant())
	}

	if !provider.IsAvailable() {
		t.Error("IsAvailable() = false, want true (HAL backends always available)")
	}

	instance, err := provider.CreateInstance(&hal.InstanceDescriptor{})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	if instance == nil {
		t.Fatal("CreateInstance returned nil")
	}
}

// testHALBackend implements hal.Backend for testing.
type testHALBackend struct {
	variant types.Backend
}

func (b *testHALBackend) Variant() types.Backend { return b.variant }
func (b *testHALBackend) CreateInstance(_ *hal.InstanceDescriptor) (hal.Instance, error) {
	return &testHALInstance{}, nil
}

type testHALInstance struct{}

func (i *testHALInstance) CreateSurface(_, _ uintptr) (hal.Surface, error) { return nil, nil } //nolint:nilnil
func (i *testHALInstance) EnumerateAdapters(_ hal.Surface) []hal.ExposedAdapter {
	return nil
}
func (i *testHALInstance) Destroy() {}
