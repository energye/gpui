//go:build linux && !nogpu

// vram_stages — stepwise VRAM attribution for 940MX-class GPUs.
//
//	GPUI_VRAM_STAGE=device|swapchain|clear|full GPUI_VRAM_SECONDS=4 go run ./examples/vram_stages
//	GPUI_POWER=high|low — adapter policy override
//	GPUI_DEVICE_NIL_LIMITS=1 — RequestDevice with label only
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

func main() {
	runtime.LockOSThread()
	bootstrap()
	stage := envOr("GPUI_VRAM_STAGE", "clear")
	sec, _ := strconv.Atoi(envOr("GPUI_VRAM_SECONDS", "4"))
	if sec < 1 {
		sec = 4
	}
	w, h := 960, 640
	if v := os.Getenv("GPUI_VRAM_W"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 64 {
			w = n
		}
	}
	if v := os.Getenv("GPUI_VRAM_H"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 64 {
			h = n
		}
	}
	log.Printf("stage=%s seconds=%d size=%dx%d pid=%d sample=%s power=%s nil_limits=%s",
		stage, sec, w, h, os.Getpid(),
		os.Getenv("GPUI_SURFACE_SAMPLE_COUNT"),
		os.Getenv("GPUI_POWER"),
		os.Getenv("GPUI_DEVICE_NIL_LIMITS"))

	report("start")

	// Open X11 first so GL backends can associate the display (InstanceExtras).
	var xw *x11Win
	if stage != "device" {
		var err error
		xw, err = openX11Window(w, h, "gpui vram_stages")
		must(err)
		defer xw.Close()
	}

	instDesc := &webgpu.InstanceDescriptor{}
	if xw != nil {
		instDesc.XlibDisplay = xw.Display
		instDesc.XlibScreen = int32(xw.Screen)
	}
	inst, err := webgpu.CreateInstance(instDesc) // env: GPUI_BACKEND
	must(err)
	defer inst.Release()
	report("after_instance")

	if stage == "device" {
		adpt, _, err := rendgpu.RequestAdapterWithPolicy(inst, nil, rendgpu.ResolveAdapterPolicy())
		must(err)
		defer adpt.Release()
		info := adpt.Info()
		log.Printf("adapter name=%q backend=%v type=%v vendor=%q", info.Name, info.Backend, info.DeviceType, info.Vendor)
		dev, err := requestDevice(adpt, "vram-device")
		must(err)
		defer dev.Release()
		report("after_device")
		sleepReport(sec)
		return
	}

	surf, err := inst.CreateSurface(xw.Display, xw.Window)
	must(err)
	defer surf.Release()
	adpt, _, err := rendgpu.RequestAdapterWithPolicy(inst, surf, rendgpu.ResolveAdapterPolicy())
	must(err)
	defer adpt.Release()
	info := adpt.Info()
	log.Printf("adapter name=%q backend=%v deviceType=%v vendor=%q policy=%v",
		info.Name, info.Backend, info.DeviceType, info.Vendor, rendgpu.ResolveAdapterPolicy())
	dev, err := requestDevice(adpt, "vram-stages")
	must(err)
	defer dev.Release()
	report("after_device")

	sc := webgpu.NewSwapchain(surf, dev, uint32(w), uint32(h))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	must(sc.ConfigureFromCapabilities(adpt))
	report("after_swapchain")

	if stage == "swapchain" {
		// Touch one acquire so surface images exist.
		if fb, err := sc.BeginFrame(); err == nil {
			_ = sc.EndFrame(fb)
		}
		report("after_first_acquire")
		sleepReport(sec)
		sc.Release()
		return
	}

	must(rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: dev, Adpt: adpt, Format: sc.Format,
	}))
	defer func() { _ = rendgpu.ResetAccelerator() }()

	dc := render.NewContext(w, h)
	defer dc.Close()
	report("after_context")

	deadline := time.Now().Add(time.Duration(sec) * time.Second)
	n := 0
	for time.Now().Before(deadline) {
		if stage == "full" {
			drawBusy(dc, w, h, float64(n)*0.016)
		} else {
			dc.SetRGB(0.1, 0.12, 0.18)
			dc.DrawRectangle(0, 0, float64(w), float64(h))
			_ = dc.Fill()
		}
		fb, err := sc.BeginFrame()
		if err != nil {
			log.Printf("BeginFrame: %v", err)
			time.Sleep(16 * time.Millisecond)
			continue
		}
		presentFn := func() error { return sc.EndFrame(fb) }
		if err := dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, presentFn); err != nil {
			if _, err2 := dc.PresentFrameAuto(fb.Handle, fb.Width, fb.Height, presentFn); err2 != nil {
				log.Printf("present: %v / %v", err, err2)
				sc.DiscardFrame(fb)
			}
		}
		n++
		if n == 1 || n%60 == 0 {
			report(fmt.Sprintf("frame_%d", n))
		}
	}
	report("end")
	log.Printf("frames=%d", n)
	sc.Release()
}

func requestDevice(adpt *webgpu.Adapter, label string) (*webgpu.Device, error) {
	if os.Getenv("GPUI_DEVICE_NIL_LIMITS") == "1" {
		return adpt.RequestDevice(&webgpu.DeviceDescriptor{Label: label})
	}
	return adpt.RequestDevice(rendgpu.DeviceDescriptorForAdapter(label, adpt))
}

func drawBusy(dc *render.Context, w, h int, t float64) {
	fw, fh := float64(w), float64(h)
	dc.SetRGB(0.07, 0.08, 0.12)
	dc.DrawRectangle(0, 0, fw, fh)
	_ = dc.Fill()
	for i := 0; i < 12; i++ {
		dc.SetRGBA(0.3, 0.5, 0.9, 0.55)
		dc.DrawCircle(fw*0.2+float64(i)*40, fh*0.4, 18)
		_ = dc.Fill()
	}
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawString(fmt.Sprintf("vram full t=%.1f", t), 12, 24)
}

func sleepReport(sec int) {
	deadline := time.Now().Add(time.Duration(sec) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		report("hold")
	}
}

func report(tag string) {
	pid := os.Getpid()
	mb := queryProcVRAM(pid)
	log.Printf("VRAM_STAGE tag=%s pid=%d smi_mib=%s", tag, pid, mb)
}

func queryProcVRAM(pid int) string {
	out, err := exec.Command("nvidia-smi", "--query-compute-apps=pid,used_gpu_memory", "--format=csv,noheader,nounits").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) >= 2 && strings.TrimSpace(parts[0]) == strconv.Itoa(pid) {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	// Full table parse (C+G rows)
	out2, err := exec.Command("nvidia-smi").Output()
	if err != nil {
		return "?"
	}
	for _, line := range strings.Split(string(out2), "\n") {
		if !strings.Contains(line, strconv.Itoa(pid)) {
			continue
		}
		fields := strings.Fields(line)
		for i, f := range fields {
			if strings.HasSuffix(f, "MiB") && i > 0 {
				return strings.TrimSuffix(f, "MiB")
			}
		}
	}
	return "not_listed"
}

func bootstrap() {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		if _, err := os.Stat("lib/libwgpu_native.so"); err == nil {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
		}
	}
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	}
	if os.Getenv("DISPLAY") == "" {
		_ = os.Setenv("DISPLAY", ":1")
	}
	if p := os.Getenv("WGPU_NATIVE_PATH"); p != "" {
		dir := filepath.Dir(p)
		if cur := os.Getenv("LD_LIBRARY_PATH"); cur == "" {
			_ = os.Setenv("LD_LIBRARY_PATH", dir)
		}
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
