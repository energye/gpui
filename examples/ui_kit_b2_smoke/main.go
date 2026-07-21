//go:build linux && !nogpu

// ui_kit_b2_smoke — M3 proof: Form / Tabs / Select / Modal / Message / VirtualList.
//
//	export DISPLAY=:1 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	GPUI_ANIM_SECONDS=12 go run ./examples/ui_kit_b2_smoke
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
	winW, winH := 720, 560
	seconds := 12.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 1 {
			seconds = 1
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_kit_b2_smoke (M3)",
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
	adapter, device, err := exboot.OpenDevice(inst, surf, "ui-kit-b2")
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
	exboot.WireAutoRecover(sc, adapter, "ui-kit-b2",
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
	vp := core.Size{Width: float64(winW), Height: float64(winH)}

	title := kit.NewText("M3 · Form / Tabs / Select / Modal / Message / VirtualList")
	title.SetFace(face)
	title.Root.FontSize = 15

	// Form
	form := kit.NewForm(nil)
	form.Face = face
	form.OnFinish = func(vals map[string]string) {
		status = fmt.Sprintf("form ok name=%s", vals["name"])
		log.Printf("form finish %v", vals)
	}
	nameIn := kit.NewInput("Name")
	nameIn.SetFace(face)
	form.BindInput("name", nameIn, true, "Name")
	emailIn := kit.NewInput("Email")
	emailIn.SetFace(face)
	form.BindInput("email", emailIn, false, "Email")

	// Select
	sel := kit.NewSelect("Role",
		kit.SelectOption{Value: "dev", Label: "Developer"},
		kit.SelectOption{Value: "ops", Label: "Ops"},
		kit.SelectOption{Value: "pm", Label: "PM"},
	)
	sel.Face = face
	sel.Viewport = vp
	sel.OnChange = func(v string) { status = "role=" + v }

	// Tabs + VirtualList
	vlist := primitive.NewVirtualList(28, func(i int) core.Node {
		tx := kit.NewText(fmt.Sprintf("Row %03d — virtual", i))
		tx.SetFace(face)
		return tx.Node()
	})
	vlist.ItemCount = 200
	vlist.Width, vlist.Height = 360, 140
	vframe := primitive.NewDecorated(vlist)
	vframe.BorderWidth = 1
	vframe.BorderColor = theme.Color(core.TokenColorBorder)
	vframe.Radius = 6
	vframe.Padding = primitive.All(4)

	tabs := kit.NewTabs(
		kit.MenuItem{Key: "form", Label: "Form"},
		kit.MenuItem{Key: "list", Label: "VirtualList"},
	)
	tabs.Face = face
	tabs.SetContent("form", form.Node())
	tabs.SetContent("list", vframe)

	// Modal
	modal := kit.NewModal("Confirm")
	modal.Face = face
	modal.Viewport = vp
	modal.SetContent(kit.NewText("Modal body — OK closes.").Node())
	modal.OnOk = func() { status = "modal ok" }

	openModal := kit.NewButton("Open Modal")
	openModal.SetFace(face)
	openModal.SetType(kit.ButtonPrimary)
	openModal.SetOnClick(func() {
		modal.Viewport = vp
		modal.SetOpen(true)
		status = "modal open"
	})

	// Drawer
	drawer := kit.NewDrawer("Side panel")
	drawer.Face = face
	drawer.Viewport = vp
	drawer.SetContent(kit.NewText("Drawer content").Node())
	openDrawer := kit.NewButton("Open Drawer")
	openDrawer.SetFace(face)
	openDrawer.SetOnClick(func() {
		drawer.Viewport = vp
		drawer.SetOpen(true)
	})

	// Messages
	msgs := kit.NewMessageHost()
	msgs.Face = face
	msgs.Viewport = vp
	toastBtn := kit.NewButton("Toast")
	toastBtn.SetFace(face)
	toastBtn.SetOnClick(func() {
		msgs.Success("Saved successfully")
		status = "toast"
	})

	btnRow := primitive.Row(openModal.Node(), openDrawer.Node(), toastBtn.Node(), sel.Node())
	btnRow.Gap = 10
	btnRow.CrossAlign = core.CrossCenter

	statusTx := kit.NewText("status: ready")
	statusTx.SetFace(face)
	statusTx.SetSecondary(true)

	col := primitive.Column(
		title.Node(),
		btnRow,
		tabs.Node(),
		statusTx.Node(),
		modal.Node(),
		drawer.Node(),
		msgs.Node(),
	)
	col.Gap = 12
	col.CrossAlign = core.CrossStart
	col.Padding = primitive.All(20)

	root := primitive.NewBox(col)
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)
	tree := core.NewTree(root)

	last := status

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
			vp = core.Size{Width: float64(w), Height: float64(h)}
			root.Width, root.Height = float64(w), float64(h)
			modal.Viewport, drawer.Viewport, msgs.Viewport = vp, vp, vp
			root.MarkNeedsLayout()
		},
		OnUpdate: func(dt float64) {
			if status != last {
				statusTx.SetValue("status: " + status)
				last = status
			}
		},
	})
	fmt.Printf("ui_kit_b2_smoke done frames=%d paints=%d hops=%d status=%q %s\n",
		res.Loops, res.Paints, res.Hops, status, dc.RenderPathStats().LogLine())
}
