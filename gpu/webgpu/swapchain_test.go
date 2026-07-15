//go:build !(js && wasm) && !nogpu

package webgpu_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

func TestSwapchain_ConfigureNilGuards(t *testing.T) {
	sc := &webgpu.Swapchain{}
	if err := sc.Configure(); err == nil {
		t.Fatal("expected error for nil surface/device")
	}
	sc = webgpu.NewSwapchain(nil, nil, 0, 0)
	if err := sc.Configure(); err == nil {
		t.Fatal("expected error for zero extent")
	}
}

func TestSwapchain_BeginFrameRequiresSurface(t *testing.T) {
	sc := webgpu.NewSwapchain(nil, nil, 64, 64)
	if _, err := sc.BeginFrame(); err == nil {
		t.Fatal("expected BeginFrame error without surface")
	}
}

func TestSwapchain_CreateSurfaceInvalidHandles(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Log("WGPU_NATIVE_PATH unset; relying on default discovery")
	}
	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		t.Skipf("CreateInstance: %v", err)
	}
	defer inst.Release()

	// Invalid platform handles must fail cleanly (not abort the process).
	_, err = inst.CreateSurface(0, 0)
	if err == nil {
		t.Fatal("CreateSurface(0,0) should return an error, not succeed or abort")
	}
	t.Logf("CreateSurface(0,0) error (expected): %v", err)
}

func TestSwapchain_NewDefaults(t *testing.T) {
	sc := webgpu.NewSwapchain(nil, nil, 100, 50)
	if sc.Width != 100 || sc.Height != 50 {
		t.Fatalf("extent %dx%d", sc.Width, sc.Height)
	}
	if sc.Format != types.TextureFormatBGRA8Unorm {
		t.Fatalf("format %v", sc.Format)
	}
	if sc.PresentMode != webgpu.PresentModeFifo {
		t.Fatalf("present mode %v", sc.PresentMode)
	}
	if sc.Usage != types.TextureUsageRenderAttachment {
		t.Fatalf("usage %v", sc.Usage)
	}
}
