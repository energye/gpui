//go:build linux && !nogpu

// ui_core_smoke — M0 proof: Pressable+Text click changes color; Linux present.
//
//	export DISPLAY=:1
//	export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_core_smoke
//
// Headless path is covered by ui/primitive tests (no GPU).
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/energye/gpui/examples/exboot"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

func main() {
	exboot.InitEnv()

	winW, winH := 480, 320
	seconds := 12.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 1 {
			seconds = 1
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_core_smoke (M0)",
	})
	if err != nil {
		log.Fatalf("LinuxHost: %v", err)
	}
	defer host.Close()
	log.Printf("host display=%#x window=%#x caps=%s", host.Display(), host.Window(), host.Caps())

	// GPU bootstrap (same device as swapchain — exboot policy).
	inst, err := exboot.NewInstanceX11(host.Display(), 0)
	if err != nil {
		log.Fatalf("CreateInstance: %v", err)
	}
	defer inst.Release()

	surf, err := inst.CreateSurface(host.Display(), host.Window())
	if err != nil {
		log.Fatalf("CreateSurface: %v", err)
	}
	defer surf.Release()

	adapter, device, err := exboot.OpenDevice(inst, surf, "ui-core-smoke")
	if err != nil {
		log.Fatalf("OpenDevice: %v", err)
	}
	defer adapter.Release()
	defer func() {
		if device != nil {
			device.Release()
		}
	}()

	sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		log.Fatalf("Configure: %v", err)
	}
	defer sc.Release()

	if err := exboot.BindProvider(device, adapter, sc.Format); err != nil {
		log.Fatalf("BindProvider: %v", err)
	}
	defer exboot.ResetAccelerator()

	dc := render.NewContext(winW, winH)
	defer dc.Close()
	exboot.WireAutoRecover(sc, adapter, "ui-core-smoke",
		func(dev *webgpu.Device) { device = dev },
		func() { dc.DropGPURenderContext() },
		nil,
	)

	// Optional font for Text measure/draw.
	var face text.Face
	for _, path := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"render/text/testdata/goregular.ttf",
	} {
		src, err := text.NewFontSourceFromFile(path)
		if err == nil {
			face = src.Face(16)
			log.Printf("font %s", path)
			break
		}
	}

	// UI tree: Box > Column > Text + Pressable(Text)
	theme := core.DefaultTheme()
	title := primitive.NewText("M0 ui/core smoke")
	title.FontSize = 18
	title.Color = theme.ColorText
	title.Face = face

	hint := primitive.NewText("Click the button — color toggles")
	hint.FontSize = 13
	hint.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.55}
	hint.Face = face

	label := primitive.NewText("Press me")
	label.FontSize = 15
	label.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	label.Face = face

	on := false
	btn := primitive.NewPressable(label)
	btn.Padding = primitive.Symmetric(20, 12)
	applyBtnColors := func() {
		if on {
			btn.SetColors(
				render.RGBA{R: 0.33, G: 0.69, B: 0.18, A: 1}, // green
				render.RGBA{R: 0.45, G: 0.78, B: 0.28, A: 1},
				render.RGBA{R: 0.22, G: 0.55, B: 0.12, A: 1},
			)
			label.SetValue("On (clicked)")
		} else {
			btn.SetColors(
				theme.ColorPrimary,
				render.RGBA{R: 0.25, G: 0.55, B: 1.0, A: 1},
				render.RGBA{R: 0.05, G: 0.30, B: 0.75, A: 1},
			)
			label.SetValue("Press me")
		}
		btn.MarkNeedsPaint()
	}
	applyBtnColors()
	btn.Click = func() {
		on = !on
		applyBtnColors()
		log.Printf("click → on=%v", on)
	}

	col := primitive.Column(title, hint, btn)
	col.Gap = 16
	col.CrossAlign = core.CrossStart
	col.Padding = primitive.All(28)

	root := primitive.NewBox(col)
	root.Color = render.RGBA{R: 0.96, G: 0.96, B: 0.98, A: 1}
	root.Width = float64(winW)
	root.Height = float64(winH)

	tree := core.NewTree(root)
	plugin := core.NewPluginHost()
	_ = plugin.RegisterControl(primitive.TypePressable, btn) // empty-run registry

	deadline := time.Now().Add(time.Duration(seconds * float64(time.Second)))
	frames := 0
	clicksSeen := 0
	prevOn := on

	for time.Now().Before(deadline) {
		// Events
		for _, ev := range host.PumpEvents() {
			if ev.Type == platform.EventClose {
				log.Printf("close requested")
				goto done
			}
			if resize, _ := platform.Dispatch(tree, ev); resize != nil {
				winW, winH = resize.Width, resize.Height
				if winW < 64 {
					winW = 64
				}
				if winH < 64 {
					winH = 64
				}
				root.Width = float64(winW)
				root.Height = float64(winH)
				root.MarkNeedsLayout()
				if err := sc.Resize(uint32(winW), uint32(winH)); err != nil {
					log.Printf("swapchain resize: %v", err)
				}
				if err := dc.Resize(winW, winH); err != nil {
					// Resize may not exist — recreate context if needed
					log.Printf("dc resize: %v", err)
				}
			}
		}
		if on != prevOn {
			clicksSeen++
			prevOn = on
		}

		// Frame: layout → paint → present
		dc.BeginFrame()
		dc.ClearWithColor(render.RGBA{R: 0.96, G: 0.96, B: 0.98, A: 1})
		pc := &core.PaintContext{DC: dc, Origin: core.Point{}, Scale: host.ScaleFactor(), Theme: theme}
		tree.Frame(pc, core.Size{Width: float64(winW), Height: float64(winH)})

		if device != nil {
			device.FlushCallbacks()
		}
		frame, err := sc.BeginFrame()
		if err != nil {
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				time.Sleep(16 * time.Millisecond)
				continue
			}
			log.Printf("BeginFrame: %v", err)
			continue
		}
		if _, err := dc.PresentFrameAuto(frame.Handle, frame.Width, frame.Height, func() error {
			return sc.EndFrame(frame)
		}); err != nil {
			sc.DiscardFrame(frame)
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				time.Sleep(16 * time.Millisecond)
				continue
			}
			log.Printf("PresentFrameAuto: %v", err)
			continue
		}
		host.Flush()
		frames++

		// Optional auto-click for CI (headless-ish proof without human).
		if v := os.Getenv("GPUI_SMOKE_AUTOCLICK"); v != "" {
			var n int
			fmt.Sscanf(v, "%d", &n)
			if n > 0 && frames == n {
				// Click approximate button center (padding 28 + title/hint + btn pad).
				x, y := 28+60.0, 28+18+16+13+16+20.0
				platform.Dispatch(tree, platform.Event{
					Type: platform.EventPointer, Pointer: platform.PointerDown,
					X: x, Y: y, Button: platform.BtnLeft,
				})
				platform.Dispatch(tree, platform.Event{
					Type: platform.EventPointer, Pointer: platform.PointerUp,
					X: x, Y: y, Button: platform.BtnLeft,
				})
				log.Printf("auto-click at %.0f,%.0f", x, y)
			}
		}
	}

done:
	stats := dc.RenderPathStats()
	fmt.Printf("ui_core_smoke done frames=%d clicks=%d on=%v %s\n", frames, clicksSeen, on, stats.LogLine())
}
