//go:build linux && !nogpu

// ui_kit_b3_smoke — M4 proof: Table / List / Tree / Pagination / Grid / Split.
//
//	export DISPLAY=:1 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_kit_b3_smoke
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
	winW, winH := 900, 640
	// Default unlimited; set GPUI_ANIM_SECONDS>0 for timed CI smoke.
	seconds := 0.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 0 {
			seconds = 0
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_kit_b3_smoke (M4)",
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
	adapter, device, err := exboot.OpenDevice(inst, surf, "ui-kit-b3")
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
	exboot.WireAutoRecover(sc, adapter, "ui-kit-b3",
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
	status := "ready"
	vp := core.Size{Width: float64(winW), Height: float64(winH)}

	title := kit.NewText("M4 · Table / List / Tree / Pagination / Grid / Split / Dropdown")
	title.SetFace(face)
	title.Root.FontSize = 15

	// Table
	cols := []kit.TableColumn{
		{Key: "id", Title: "ID", Width: 50},
		{Key: "name", Title: "Name", Flex: 1},
		{Key: "role", Title: "Role", Width: 100},
	}
	var data []map[string]string
	for i := 1; i <= 50; i++ {
		data = append(data, map[string]string{
			"id": fmt.Sprintf("%d", i), "name": fmt.Sprintf("User %02d", i), "role": "dev",
		})
	}
	table := kit.NewTable(cols, data)
	table.Face = face
	table.OnRowClick = func(i int, row map[string]string) {
		status = fmt.Sprintf("row %d %s", i, row["name"])
		log.Printf("%s", status)
	}

	// List + Tree in split
	list := kit.NewList("Alpha", "Bravo", "Charlie", "Delta", "Echo")
	list.Face = face
	list.OnSelect = func(i int, s string) { status = "list=" + s }

	tree := kit.NewTree(
		&kit.TreeNode{Key: "src", Title: "src", Expanded: true, Children: []*kit.TreeNode{
			{Key: "ui", Title: "ui", Expanded: true, Children: []*kit.TreeNode{
				{Key: "kit", Title: "kit"},
				{Key: "core", Title: "core"},
			}},
			{Key: "render", Title: "render"},
		}},
	)
	tree.Face = face
	tree.OnSelect = func(k string) { status = "tree=" + k }

	split := primitive.NewSplitPane(list.Node(), tree.Node())
	split.Ratio = 0.4

	// Pagination
	pager := kit.NewPagination(8)
	pager.Face = face
	pager.OnChange = func(p int) { status = fmt.Sprintf("page=%d", p) }

	// Dropdown
	dd := kit.NewDropdown("Actions",
		kit.MenuItem{Key: "edit", Label: "Edit"},
		kit.MenuItem{Key: "del", Label: "Delete"},
	)
	dd.Face = face
	dd.Viewport = vp
	dd.OnSelect = func(k string) { status = "action=" + k }

	// Grid of cards
	mkCard := func(s string) core.Node {
		tx := kit.NewText(s)
		tx.SetFace(face)
		d := primitive.NewDecorated(tx.Node())
		d.Padding = primitive.All(12)
		d.Radius = 6
		d.Background = theme.Color(core.TokenColorBgContainer)
		d.BorderWidth = 1
		d.BorderColor = theme.Color(core.TokenColorBorder)
		return d
	}
	grid := primitive.NewGrid([]core.GridTrack{{Fr: 1}, {Fr: 1}, {Fr: 1}},
		mkCard("Grid A"), mkCard("Grid B"), mkCard("Grid C"),
		mkCard("Grid D"), mkCard("Grid E"), mkCard("Grid F"),
	)

	// Transfer
	xfer := kit.NewTransfer([]string{"Go", "Rust", "Zig", "C"})
	xfer.Face = face

	statusTx := kit.NewText("status: ready")
	statusTx.SetFace(face)
	statusTx.SetSecondary(true)

	top := primitive.Row(dd.Node(), pager.Node())
	top.Gap = 16
	top.CrossAlign = core.CrossCenter

	col := primitive.Column(
		title.Node(),
		top,
		table.Node(),
		kit.NewText("Split · List | Tree").Node(),
		split,
		kit.NewText("Grid").Node(),
		grid,
		kit.NewText("Transfer").Node(),
		xfer.Node(),
		statusTx.Node(),
	)
	col.Gap = 10
	col.CrossAlign = core.CrossStretch
	col.Padding = primitive.All(16)

	// scroll whole page
	page := primitive.NewScrollViewport(col)
	page.Width = float64(winW)
	page.Height = float64(winH)

	root := primitive.NewBox(page)
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)
	uiTree := core.NewTree(root)

	last := status

	res := exboot.RunUIDemand(exboot.UIDemandConfig{
		Host: host, Tree: uiTree, SC: sc, DC: dc, Device: device, Theme: theme,
		Clear:   theme.Color(core.TokenColorBgLayout),
		Seconds: seconds,
		Flush:   host.Flush,
		OnResize: func(w, h int) {
			winW, winH = w, h
			vp = core.Size{Width: float64(w), Height: float64(h)}
			root.Width, root.Height = float64(w), float64(h)
			page.Width, page.Height = float64(w), float64(h)
			dd.Viewport = vp
			root.MarkNeedsLayout()
		},
		OnUpdate: func(dt float64) {
			dd.Sync()
			if status != last {
				statusTx.SetValue("status: " + status)
				last = status
			}
		},
	})
	fmt.Printf("ui_kit_b3_smoke done frames=%d paints=%d hops=%d status=%q %s\n",
		res.Loops, res.Paints, res.Hops, status, dc.RenderPathStats().LogLine())
}
