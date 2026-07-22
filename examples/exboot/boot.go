//go:build linux && !nogpu

// Package exboot shares GPU bootstrap for examples (device policy, X11 instance,
// auto-recover wiring). Keeps mem_anim / PKS / capability_matrix aligned with
// device_lost_redraw so recover does not OOM on 1GB GPUs.
package exboot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

// InitEnv sets native lib / display defaults when unset.
//
// MSAA: default GPUI_SURFACE_SAMPLE_COUNT=4 for Ant-class soft edges in the
// window. Set GPUI_SURFACE_SAMPLE_COUNT=1 only when VRAM is tight.
func InitEnv() {
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		// UI chrome: 4× MSAA (was 1 — hard edges made window look non-Ant
		// while ui_ant_compare CPU path looked fine).
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "4")
	}
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		for _, p := range []string{
			"lib/libwgpu_native.so",
			"libwgpu_native.so",
			filepath.Join("..", "lib", "libwgpu_native.so"),
		} {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				_ = os.Setenv("WGPU_NATIVE_PATH", p)
				if cur := os.Getenv("LD_LIBRARY_PATH"); cur == "" {
					_ = os.Setenv("LD_LIBRARY_PATH", filepath.Dir(p))
				}
				break
			}
		}
	}
	if os.Getenv("DISPLAY") == "" {
		_ = os.Setenv("DISPLAY", ":1")
	}
}

// NewInstanceX11 creates a wgpu instance associated with the X11 display
// (required for GL backends; harmless for Vulkan).
func NewInstanceX11(display uintptr, screen int) (*webgpu.Instance, error) {
	InitEnv()
	return webgpu.CreateInstance(&webgpu.InstanceDescriptor{
		XlibDisplay: display,
		XlibScreen:  int32(screen), //nolint:gosec
	})
}

// OpenDevice requests adapter+device with the shared policy
// (default: hybrid prefer iGPU; GPUI_POWER=high|low override).
func OpenDevice(inst *webgpu.Instance, surf *webgpu.Surface, label string) (*webgpu.Adapter, *webgpu.Device, error) {
	if inst == nil {
		return nil, nil, fmt.Errorf("exboot: nil instance")
	}
	adpt, soft, err := rendgpu.RequestAdapterWithPolicy(inst, surf, rendgpu.ResolveAdapterPolicy())
	if err != nil {
		return nil, nil, err
	}
	info := adpt.Info()
	log.Printf("exboot: adapter name=%q backend=%v type=%v vendor=%q policy=%v soft=%v",
		info.Name, info.Backend, info.DeviceType, info.Vendor, rendgpu.ResolveAdapterPolicy(), soft)
	dev, err := adpt.RequestDevice(rendgpu.DeviceDescriptorForAdapter(label, adpt))
	if err != nil {
		return nil, nil, err
	}
	return adpt, dev, nil
}

// BindProvider installs the shared device into the render accelerator.
// Logs env + resolved MSAA sample count (Skia-style: GPU on, coverage+MSAA quality).
func BindProvider(dev *webgpu.Device, adpt *webgpu.Adapter, format webgpu.TextureFormat) error {
	err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: dev, Adpt: adpt, Format: format,
	})
	scEnv := os.Getenv("GPUI_SURFACE_SAMPLE_COUNT")
	if scEnv == "" {
		scEnv = "(default→4)"
	}
	if err != nil {
		log.Printf("exboot: BindProvider failed: %v (GPUI_SURFACE_SAMPLE_COUNT=%s)", err, scEnv)
		return err
	}
	// Actual pipeline samples after bind (not just env).
	samples := uint32(0)
	if a := render.Accelerator(); a != nil {
		if m, ok := a.(render.MSAAAware); ok {
			samples = m.MSAASampleCount()
		}
	}
	log.Printf("exboot: BindProvider ok format=%v env_SAMPLE_COUNT=%s resolved_msaa=%d ui_supersample=%q",
		format, scEnv, samples, os.Getenv("GPUI_UI_SUPERSAMPLE"))
	return nil
}

// WireAutoRecover arms swapchain recovery (Skia abandon+recreate).
//
// Engine already invalidates ALL render.Context GPU state on AbandonDevice
// (context GPU registry). dropGPUCtx is optional host extra (e.g. example-level
// image caches); nil is OK for simple hosts.
//
// setDevice may be nil. onRecover is optional extra host work.
func WireAutoRecover(
	sc *webgpu.Swapchain,
	adpt *webgpu.Adapter,
	label string,
	setDevice func(*webgpu.Device),
	dropGPUCtx func(),
	onRecover func(*webgpu.Device),
) {
	if sc == nil || adpt == nil {
		return
	}
	sc.OnDeviceAbandon = func(_ *webgpu.Device) {
		// Host GPU contexts/sessions first (while force-release is about to run),
		// then shared accelerator caches. Order matches Skia abandon.
		if dropGPUCtx != nil {
			dropGPUCtx()
		}
		rendgpu.AbandonDevice()
	}
	sc.EnableAutoRecover(adpt, label, func(dev *webgpu.Device) {
		if setDevice != nil {
			setDevice(dev)
		}
		if err := BindProvider(dev, adpt, sc.Format); err != nil {
			log.Printf("exboot: SetDeviceProvider after recover: %v", err)
		}
		// Drop again after rebind so any session rebuilt mid-callback is cleared.
		if dropGPUCtx != nil {
			dropGPUCtx()
		}
		if onRecover != nil {
			onRecover(dev)
		}
		log.Printf("exboot: GPU device recovered label=%s recoveries=%d", label, sc.Recoveries())
	})
}

// ResetAccelerator releases the shared render GPU binding (defer at process exit).
func ResetAccelerator() {
	_ = rendgpu.ResetAccelerator()
}
