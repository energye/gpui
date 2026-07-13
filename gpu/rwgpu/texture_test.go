package rwgpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

func TestCreateTexture(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	defer inst.Release()

	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice failed: %v", err)
	}
	defer device.Release()

	t.Log("Creating 2D texture...")
	texture, err := device.CreateTexture(&TextureDescriptor{
		Usage:     types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
		Dimension: types.TextureDimension2D,
		Size: types.Extent3D{
			Width:              256,
			Height:             256,
			DepthOrArrayLayers: 1,
		},
		Format:        types.TextureFormatRGBA8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer texture.Release()

	if texture.Handle() == 0 {
		t.Fatal("Texture handle is zero")
	}

	t.Logf("Texture created: handle=%#x", texture.Handle())
}

func TestCreateTextureView(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	defer inst.Release()

	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice failed: %v", err)
	}
	defer device.Release()

	texture, err := device.CreateTexture(&TextureDescriptor{
		Usage:     types.TextureUsageTextureBinding,
		Dimension: types.TextureDimension2D,
		Size: types.Extent3D{
			Width:              128,
			Height:             128,
			DepthOrArrayLayers: 1,
		},
		Format:        types.TextureFormatRGBA8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer texture.Release()

	t.Log("Creating texture view...")
	view, err := texture.CreateView(nil)
	if err != nil {
		t.Fatalf("CreateView: %v", err)
	}
	defer view.Release()

	if view.Handle() == 0 {
		t.Fatal("TextureView handle is zero")
	}

	t.Logf("TextureView created: handle=%#x", view.Handle())
}

func TestCreateDepthTexture(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	defer inst.Release()

	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice failed: %v", err)
	}
	defer device.Release()

	t.Log("Creating depth texture...")
	depthTexture := device.CreateDepthTexture(800, 600, types.TextureFormatDepth24Plus)
	if depthTexture == nil {
		t.Fatal("CreateDepthTexture returned nil")
	}
	defer depthTexture.Release()

	if depthTexture.Handle() == 0 {
		t.Fatal("Depth texture handle is zero")
	}

	t.Logf("Depth texture created: handle=%#x", depthTexture.Handle())
}

func TestCreateSampler(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	defer inst.Release()

	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice failed: %v", err)
	}
	defer device.Release()

	t.Log("Creating sampler...")
	sampler, err := device.CreateSampler(&SamplerDescriptor{
		AddressModeU: types.AddressModeRepeat,
		AddressModeV: types.AddressModeRepeat,
		AddressModeW: types.AddressModeRepeat,
		MagFilter:    types.FilterModeLinear,
		MinFilter:    types.FilterModeLinear,
		MipmapFilter: types.MipmapFilterModeLinear,
		Anisotropy:   1,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}
	defer sampler.Release()

	if sampler.Handle() == 0 {
		t.Fatal("Sampler handle is zero")
	}

	t.Logf("Sampler created: handle=%#x", sampler.Handle())
}

func TestCreateSamplerSimple(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	defer inst.Release()

	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice failed: %v", err)
	}
	defer device.Release()

	t.Log("Creating sampler with minimal settings...")
	sampler, err := device.CreateSampler(&SamplerDescriptor{
		Anisotropy: 1, // Required to be >= 1
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}
	defer sampler.Release()

	t.Logf("Simple sampler created: handle=%#x", sampler.Handle())
}

func TestTextureFormats(t *testing.T) {
	// Test common texture format constants
	formats := []struct {
		name   string
		format types.TextureFormat
	}{
		{"RGBA8Unorm", types.TextureFormatRGBA8Unorm},
		{"BGRA8Unorm", types.TextureFormatBGRA8Unorm},
		{"Depth24Plus", types.TextureFormatDepth24Plus},
		{"Depth32Float", types.TextureFormatDepth32Float},
		{"R8Unorm", types.TextureFormatR8Unorm},
		{"RG8Unorm", types.TextureFormatRG8Unorm},
	}

	for _, f := range formats {
		if f.format == 0 {
			t.Errorf("Format %s has zero value", f.name)
		}
		t.Logf("Format %s = %#x", f.name, f.format)
	}
}
