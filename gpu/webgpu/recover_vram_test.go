//go:build linux && !nogpu && !js

package webgpu_test

import (
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// TestDeviceRecover_CreateTextureAfterRelease reproduces AutoRecover VRAM path.
// On this libwgpu_native, wgpuDeviceDestroy (Device.Destroy old path) left the
// adapter unable to CreateTexture ("Not enough memory left"). Recover must drop
// the old device via Release only after abandoning children.
func TestDeviceRecover_CreateTextureAfterRelease(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		if st, err := os.Stat("lib/libwgpu_native.so"); err == nil && !st.IsDir() {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
			_ = os.Setenv("LD_LIBRARY_PATH", "lib:"+os.Getenv("LD_LIBRARY_PATH"))
		}
	}
	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		t.Skip(err)
	}
	defer inst.Release()
	adpt, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{PowerPreference: webgpu.PowerPreferenceHighPerformance})
	if err != nil {
		t.Skip(err)
	}
	defer adpt.Release()

	mkDepth := func(dev *webgpu.Device, w, h uint32) (*webgpu.Texture, error) {
		return dev.CreateTexture(&webgpu.TextureDescriptor{
			Label: "session_depth_stencil",
			Size: webgpu.Extent3D{
				Width: w, Height: h, DepthOrArrayLayers: 1,
			},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatDepth24PlusStencil8,
			Usage:         types.TextureUsageRenderAttachment,
		})
	}

	dev, err := adpt.RequestDevice(&webgpu.DeviceDescriptor{Label: "recover-old"})
	if err != nil {
		t.Fatal(err)
	}
	// Allocate like a live session (multiple depth targets).
	var texs []*webgpu.Texture
	for i := 0; i < 3; i++ {
		tex, err := mkDepth(dev, 1280, 720)
		if err != nil {
			t.Fatalf("pre-alloc: %v", err)
		}
		texs = append(texs, tex)
	}
	for _, tex := range texs {
		tex.Release()
	}
	_ = dev.WaitIdle()
	// Critical: Release only (no native DeviceDestroy).
	dev.Release()
	time.Sleep(20 * time.Millisecond)
	inst.ProcessEvents()

	dev2, err := adpt.RequestDevice(&webgpu.DeviceDescriptor{Label: "recover-new"})
	if err != nil {
		t.Fatalf("RequestDevice after Release: %v", err)
	}
	defer dev2.Release()
	tex, err := mkDepth(dev2, 960, 640)
	if err != nil {
		t.Fatalf("CreateTexture after recover (would be session_depth_stencil OOM): %v", err)
	}
	tex.Release()
}
