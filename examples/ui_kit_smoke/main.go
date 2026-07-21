//go:build linux && !nogpu

// ui_kit_smoke — M1 proof: kit.Button/Text/Icon + Theme/Token + present.
//
//	export DISPLAY=:1 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	GPUI_ANIM_SECONDS=12 GPUI_SMOKE_AUTOCLICK=40 go run ./examples/ui_kit_smoke
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
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

func main() {
	exboot.InitEnv()
	winW, winH := 560, 400
	seconds := 12.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 1 {
			seconds = 1
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_kit_smoke (M1)",
	})
	if err != nil {
		log.Fatalf("LinuxHost: %v", err)
	}
	defer host.Close()

	inst, err := exboot.NewInstanceX11(host.Display(), 0)
	if err != nil {
		log.Fatalf("instance: %v", err)
	}
	defer inst.Release()
	surf, err := inst.CreateSurface(host.Display(), host.Window())
	if err != nil {
		log.Fatalf("surface: %v", err)
	}
	defer surf.Release()
	adapter, device, err := exboot.OpenDevice(inst, surf, "ui-kit-smoke")
	if err != nil {
		log.Fatalf("device: %v", err)
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
		log.Fatalf("configure: %v", err)
	}
	defer sc.Release()
	if err := exboot.BindProvider(device, adapter, sc.Format); err != nil {
		log.Fatalf("bind: %v", err)
	}
	defer exboot.ResetAccelerator()

	dc := render.NewContext(winW, winH)
	defer dc.Close()
	exboot.WireAutoRecover(sc, adapter, "ui-kit-smoke",
		func(dev *webgpu.Device) { device = dev },
		func() { dc.DropGPURenderContext() },
		nil,
	)

	var face text.Face
	for _, path := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"render/text/testdata/goregular.ttf",
	} {
		if src, err := text.NewFontSourceFromFile(path); err == nil {
			face = src.Face(14)
			log.Printf("font %s", path)
			break
		}
	}

	theme := kit.DefaultTheme()
	status := "idle"
	clicks := 0

	title := kit.NewText("M1 · kit B0 (Button / Text / Icon)")
	title.FontSize = 18
	title.SetFace(face)
	title.Root.FontSize = 18

	sub := kit.NewText("Theme tokens · Decorated · Pressable · focus Tab")
	sub.SetSecondary(true)
	sub.SetFace(face)

	primary := kit.NewButton("Primary")
	primary.SetType(kit.ButtonPrimary)
	primary.SetFace(face)
	primary.SetIcon("check")
	primary.SetOnClick(func() {
		clicks++
		status = fmt.Sprintf("Primary ×%d", clicks)
		log.Printf("click primary → %s", status)
	})

	defBtn := kit.NewButton("Default")
	defBtn.SetFace(face)
	defBtn.SetOnClick(func() {
		clicks++
		status = fmt.Sprintf("Default ×%d", clicks)
		log.Printf("click default → %s", status)
	})

	danger := kit.NewButton("Danger")
	danger.SetType(kit.ButtonPrimary)
	danger.SetDanger(true)
	danger.SetFace(face)
	danger.SetIcon("close")
	danger.SetOnClick(func() {
		clicks++
		status = fmt.Sprintf("Danger ×%d", clicks)
		log.Printf("click danger → %s", status)
	})

	dashed := kit.NewButton("Dashed")
	dashed.SetType(kit.ButtonDashed)
	dashed.SetFace(face)

	textBtn := kit.NewButton("Text")
	textBtn.SetType(kit.ButtonText)
	textBtn.SetFace(face)

	disabled := kit.NewButton("Disabled")
	disabled.SetType(kit.ButtonPrimary)
	disabled.SetDisabled(true)
	disabled.SetFace(face)

	btns := []*kit.Button{primary, defBtn, danger, dashed, textBtn, disabled}
	btnRow := primitive.Row()
	btnRow.Gap = 12
	btnRow.CrossAlign = core.CrossCenter
	for _, b := range btns {
		btnRow.AddChild(b.Node())
	}

	statusText := kit.NewText("status: idle")
	statusText.SetFace(face)

	div := primitive.NewDivider()
	div.ColorToken = core.TokenColorBorder
	div.Margin = primitive.Symmetric(0, 8)

	iconRow := primitive.Row()
	iconRow.Gap = 16
	for _, name := range []string{"check", "close", "plus", "search", "info", "chevron-right"} {
		ic := kit.NewIcon(name)
		ic.SetSize(20)
		iconRow.AddChild(ic.Node())
	}

	iconsLabel := kit.NewText("Icons")
	iconsLabel.SetFace(face)

	col := primitive.Column(
		title.Node(),
		sub.Node(),
		div,
		btnRow,
		statusText.Node(),
		primitive.NewDivider(),
		iconsLabel.Node(),
		iconRow,
	)
	col.Gap = 14
	col.CrossAlign = core.CrossStart
	col.Padding = primitive.All(28)

	root := primitive.NewBox(col)
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)

	tree := core.NewTree(root)

	deadline := time.Now().Add(time.Duration(seconds * float64(time.Second)))
	frames := 0
	lastStatus := status

	for time.Now().Before(deadline) {
		for _, ev := range host.PumpEvents() {
			if ev.Type == platform.EventClose {
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
				root.Width, root.Height = float64(winW), float64(winH)
				root.MarkNeedsLayout()
				_ = sc.Resize(uint32(winW), uint32(winH))
				_ = dc.Resize(winW, winH)
			}
		}
		for _, b := range btns {
			b.SyncState()
		}
		if status != lastStatus {
			statusText.SetValue("status: " + status)
			lastStatus = status
		}

		dc.BeginFrame()
		dc.ClearWithColor(theme.Color(core.TokenColorBgLayout))
		pc := &core.PaintContext{DC: dc, Scale: host.ScaleFactor(), Theme: theme}
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
			time.Sleep(16 * time.Millisecond)
			continue
		}
		host.Flush()
		frames++

		if v := os.Getenv("GPUI_SMOKE_AUTOCLICK"); v != "" {
			var n int
			fmt.Sscanf(v, "%d", &n)
			if n > 0 && frames == n {
				// Prefer exact primary button center after layout.
				x := primary.Root.Offset().X + primary.Root.Size().Width/2
				y := primary.Root.Offset().Y + primary.Root.Size().Height/2
				// Offsets are relative; walk absolute.
				x, y = absoluteCenter(primary.Root)
				platform.Dispatch(tree, platform.Event{
					Type: platform.EventPointer, Pointer: platform.PointerDown,
					X: x, Y: y, Button: platform.BtnLeft,
				})
				platform.Dispatch(tree, platform.Event{
					Type: platform.EventPointer, Pointer: platform.PointerUp,
					X: x, Y: y, Button: platform.BtnLeft,
				})
				for _, b := range btns {
					b.SyncState()
				}
				log.Printf("auto-click at %.0f,%.0f", x, y)
			}
		}
	}
done:
	fmt.Printf("ui_kit_smoke done frames=%d clicks=%d status=%q %s\n",
		frames, clicks, status, dc.RenderPathStats().LogLine())
}

func absoluteCenter(n core.Node) (x, y float64) {
	if n == nil {
		return 0, 0
	}
	b := n.Base()
	x = b.Size().Width / 2
	y = b.Size().Height / 2
	for cur := n; cur != nil; cur = cur.Parent() {
		o := cur.Base().Offset()
		x += o.X
		y += o.Y
	}
	return x, y
}
