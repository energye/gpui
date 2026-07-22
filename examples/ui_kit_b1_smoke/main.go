//go:build linux && !nogpu

// ui_kit_b1_smoke — M2 proof: Input / Checkbox / Switch / Scroll / Overlay.
//
//	export DISPLAY=:1 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_kit_b1_smoke
package main

import (
	"fmt"
	"log"
	"os"

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
	winW, winH := 640, 480
	// Default unlimited; set GPUI_ANIM_SECONDS>0 for timed CI smoke.
	seconds := 0.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 0 {
			seconds = 0
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_kit_b1_smoke (M2)",
	})
	if err != nil {
		log.Fatalf("host: %v", err)
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
	adapter, device, err := exboot.OpenDevice(inst, surf, "ui-kit-b1")
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
	exboot.WireAutoRecover(sc, adapter, "ui-kit-b1",
		func(dev *webgpu.Device) { device = dev },
		func() { dc.DropGPURenderContext() },
		nil,
	)

	var face text.Face
	for _, path := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"render/text/testdata/goregular.ttf",
	} {
		if src, err := text.NewFontSourceFromFile(path); err == nil {
			face = src.Face(14)
			break
		}
	}

	theme := kit.DefaultTheme()
	status := "ready"

	title := kit.NewText("M2 · Input / Checkbox / Switch / Scroll / Popover")
	title.SetFace(face)
	title.Root.FontSize = 16

	name := kit.NewInput("Your name")
	name.SetFace(face)
	name.SetOnChange(func(v string) {
		status = "name=" + v
	})
	name.SetPrefix(primitive.NewIcon("search"))

	cb := kit.NewCheckbox("I agree")
	cb.SetFace(face)
	cb.SetOnChange(func(v bool) {
		status = fmt.Sprintf("agree=%v", v)
	})

	sw := kit.NewSwitch()
	sw.OnChange = func(v bool) {
		status = fmt.Sprintf("switch=%v", v)
	}
	swLabel := kit.NewText("Notifications")
	swLabel.SetFace(face)
	swRow := primitive.Row(sw.Node(), swLabel.Node())
	swRow.Gap = 10
	swRow.CrossAlign = core.CrossCenter

	ra := kit.NewRadio("light", "Light")
	rb := kit.NewRadio("dark", "Dark")
	rg := kit.NewRadioGroup(ra, rb)
	rg.OnChange = func(v string) { status = "theme=" + v }
	rg.Select("light")

	// Scroll demo content
	scrollCol := primitive.Column()
	scrollCol.Gap = 6
	for i := 0; i < 20; i++ {
		tx := kit.NewText(fmt.Sprintf("Scroll row %02d — lorem ipsum", i+1))
		tx.SetFace(face)
		scrollCol.AddChild(tx.Node())
	}
	sv := primitive.NewScrollViewport(scrollCol)
	sv.Width, sv.Height = 280, 120
	scrollFrame := primitive.NewDecorated(sv)
	scrollFrame.BorderWidth = 1
	scrollFrame.BorderColor = theme.Color(core.TokenColorBorder)
	scrollFrame.Radius = 6
	scrollFrame.Padding = primitive.All(4)

	// Popover
	popBody := kit.NewText("Popover body · M2 AnchoredPopup")
	popBody.SetFace(face)
	popBtn := kit.NewButton("Popover")
	popBtn.SetFace(face)
	pop := kit.NewPopover(popBtn.Node(), popBody.Node())
	pop.Viewport = core.Size{Width: float64(winW), Height: float64(winH)}

	statusTx := kit.NewText("status: ready")
	statusTx.SetFace(face)
	statusTx.SetSecondary(true)

	form := primitive.Column(
		title.Node(),
		name.Node(),
		cb.Node(),
		swRow,
		rg.Node(),
		kit.NewText("ScrollViewport").Node(),
		scrollFrame,
		pop.Node(),
		statusTx.Node(),
	)
	form.Gap = 12
	form.CrossAlign = core.CrossStart
	form.Padding = primitive.All(24)

	root := primitive.NewBox(form)
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)

	tree := core.NewTree(root)
	lastStatus := status
	autoSeed := false

	res := exboot.RunUIDemand(exboot.UIDemandConfig{
		Host: host, Tree: tree, SC: sc, DC: dc, Device: device, Theme: theme,
		Clear:   theme.Color(core.TokenColorBgLayout),
		Seconds: seconds,
		Flush:   host.Flush,
		BeforeDispatch: func(tr *core.Tree, ev platform.Event) bool {
			if ev.Type == platform.EventKey && ev.Down && len(ev.Key) == 1 {
				tr.DispatchTextInput(&core.TextInputEvent{Text: ev.Key})
				return true
			}
			return false
		},
		OnResize: func(w, h int) {
			winW, winH = w, h
			root.Width, root.Height = float64(w), float64(h)
			root.MarkNeedsLayout()
		},
		OnUpdate: func(dt float64) {
			if status != lastStatus {
				statusTx.SetValue("status: " + status)
				lastStatus = status
			}
			if !autoSeed {
				autoSeed = true
				// seed after first update if empty (was frames==20)
				if name.Value == "" {
					tree.SetFocus(name.Editor())
					tree.DispatchTextInput(&core.TextInputEvent{Text: "Ada"})
					log.Printf("auto-typed Ada")
				}
			}
		},
	})
	fmt.Printf("ui_kit_b1_smoke done frames=%d paints=%d hops=%d status=%q %s\n",
		res.Loops, res.Paints, res.Hops, status, dc.RenderPathStats().LogLine())
}
