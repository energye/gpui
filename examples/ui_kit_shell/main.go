//go:build linux && !nogpu

// ui_kit_shell — M6 kit-first desktop shell (antd_desktop_app 对照迁 kit 路径).
//
//	Header + Sider(Menu) + Content(Table) + Footer + Modal + Message
//
//	export DISPLAY=:1 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	GPUI_ANIM_SECONDS=15 go run ./examples/ui_kit_shell
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
	winW, winH := 960, 640
	seconds := 15.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 1 {
			seconds = 1
		}
	}

	host, err := platform.NewHost(platform.HostOptions{
		Width: winW, Height: winH, Title: "gpui ui_kit_shell (M6)",
	})
	if err != nil {
		log.Fatalf("host: %v", err)
	}
	defer host.Close()
	if !platform.GPUPresentReady(host) {
		log.Fatalf("GPU present requires Linux host (got %T)", host)
	}
	lh := host.(*platform.LinuxHost)

	inst, err := exboot.NewInstanceX11(lh.Display(), 0)
	if err != nil {
		log.Fatalf("instance: %v", err)
	}
	defer inst.Release()
	surf, err := inst.CreateSurface(lh.Display(), lh.Window())
	if err != nil {
		log.Fatalf("surface: %v", err)
	}
	defer surf.Release()
	adapter, device, err := exboot.OpenDevice(inst, surf, "ui-kit-shell")
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
	exboot.WireAutoRecover(sc, adapter, "ui-kit-shell",
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
			face = src.Face(13)
			break
		}
	}

	theme := kit.DefaultTheme()
	vp := core.Size{Width: float64(winW), Height: float64(winH)}
	status := "shell ready"

	// Header
	brand := kit.NewText("gpui · kit shell")
	brand.SetFace(face)
	brand.Root.FontSize = 16
	userBtn := kit.NewButton("New")
	userBtn.SetType(kit.ButtonPrimary)
	userBtn.SetFace(face)
	header := primitive.NewDecorated(primitive.Row(brand.Node(), primitive.Spacer(), userBtn.Node()))
	header.Padding = primitive.Symmetric(16, 10)
	header.Background = theme.Color(core.TokenColorBgContainer)
	header.BorderWidth = 0

	// Sider menu
	menu := kit.NewMenu(
		kit.MenuItem{Key: "dash", Label: "Dashboard"},
		kit.MenuItem{Key: "users", Label: "Users"},
		kit.MenuItem{Key: "settings", Label: "Settings"},
	)
	menu.Face = face
	menu.SetSelected("users")
	menu.OnSelect = func(k string) { status = "nav=" + k }
	sider := primitive.NewDecorated(menu.Node())
	sider.Padding = primitive.All(8)
	sider.Background = theme.Color(core.TokenColorBgContainer)
	sider.MinWidth = 180

	// Content table
	cols := []kit.TableColumn{
		{Key: "id", Title: "ID", Width: 48},
		{Key: "name", Title: "Name", Flex: 1},
		{Key: "role", Title: "Role", Width: 100},
	}
	var data []map[string]string
	for i := 1; i <= 40; i++ {
		data = append(data, map[string]string{
			"id": fmt.Sprintf("%d", i), "name": fmt.Sprintf("User %02d", i), "role": "member",
		})
	}
	table := kit.NewTable(cols, data)
	table.Face = face
	table.OnRowClick = func(i int, row map[string]string) {
		status = fmt.Sprintf("row %s", row["name"])
	}
	pager := kit.NewPagination(5)
	pager.Face = face
	pager.OnChange = func(p int) { status = fmt.Sprintf("page=%d", p) }

	contentCol := primitive.Column(
		kit.NewText("Users").Node(),
		table.Node(),
		pager.Node(),
	)
	contentCol.Gap = 10
	contentCol.Padding = primitive.All(12)
	contentCol.CrossAlign = core.CrossStretch

	// Modal + message
	modal := kit.NewModal("Create user")
	modal.Face = face
	modal.Viewport = vp
	form := kit.NewForm(nil)
	form.Face = face
	nameIn := kit.NewInput("Name")
	nameIn.SetFace(face)
	form.BindInput("name", nameIn, true, "Name")
	form.OnFinish = func(vals map[string]string) {
		status = "created " + vals["name"]
		modal.SetOpen(false)
	}
	modal.SetContent(form.Node())
	modal.OnOk = func() { form.Validate() }

	userBtn.SetOnClick(func() {
		modal.Viewport = vp
		modal.SetOpen(true)
	})

	msgs := kit.NewMessageHost()
	msgs.Face = face
	msgs.Viewport = vp
	toast := kit.NewButton("Toast")
	toast.SetFace(face)
	toast.SetOnClick(func() { msgs.Success("Saved") })

	// Footer
	statusTx := kit.NewText("status: shell ready")
	statusTx.SetFace(face)
	statusTx.SetSecondary(true)
	footer := primitive.NewDecorated(primitive.Row(statusTx.Node(), primitive.Spacer(), toast.Node()))
	footer.Padding = primitive.Symmetric(12, 8)
	footer.Background = theme.Color(core.TokenColorBgContainer)

	body := primitive.Row(sider, contentCol)
	body.Gap = 0
	body.CrossAlign = core.CrossStretch
	// stretch content
	body.AddChild(primitive.NewFlexible(1, contentCol)) // careful - already has contentCol
	// rebuild body cleanly
	body = primitive.Row(sider, primitive.NewFlexible(1, contentCol))
	body.CrossAlign = core.CrossStretch

	shell := primitive.Column(header, primitive.NewFlexible(1, body), footer)
	shell.CrossAlign = core.CrossStretch

	root := primitive.NewBox(shell, modal.Node(), msgs.Node())
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)
	// Box multi-child stacks — use column for shell only; portals as siblings via stack
	stack := primitive.NewStack(shell, modal.Node(), msgs.Node())
	root = primitive.NewBox(stack)
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)

	tree := core.NewTree(root)
	last := status

	res := exboot.RunUIDemand(exboot.UIDemandConfig{
		Host: lh, Tree: tree, SC: sc, DC: dc, Device: device, Theme: theme,
		Clear:   theme.Color(core.TokenColorBgLayout),
		Seconds: seconds,
		Flush:   lh.Flush,
		OnResize: func(w, h int) {
			winW, winH = w, h
			vp = core.Size{Width: float64(w), Height: float64(h)}
			root.Width, root.Height = float64(w), float64(h)
			modal.Viewport, msgs.Viewport = vp, vp
			root.MarkNeedsLayout()
		},
		OnUpdate: func(dt float64) {
			modal.Sync()
			msgs.Sync()
			if status != last {
				statusTx.SetValue("status: " + status)
				last = status
			}
		},
	})
	sum := kit.CoverageSummary(kit.AntCoverage())
	fmt.Printf("ui_kit_shell done frames=%d paints=%d hops=%d status=%q coverage=%v %s\n",
		res.Loops, res.Paints, res.Hops, status, sum, dc.RenderPathStats().LogLine())
}
