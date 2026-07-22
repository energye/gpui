//go:build linux && !nogpu

// ui_polish_gallery — polish gallery (W3 visual/focus + W4 IME notes / Modal).
//
// §12.2 手操清单:
//
//  1. 静态浏览 — 圆角、1px 边、间距是否像控件
//
//  2. 鼠标扫 Button/Checkbox — hover/press（Button 每帧 SyncState）
//
//  3. Tab 走焦 — focus 环（Button）/ 主色边框（Input）可见
//
//  4. 点 Checkbox/Radio — 选中圆滑、居中
//
//  5. Linux 中文输入 — Caps 降级：LinuxHost 无 CapIME（见 README / ui/platform/ime.go）
//     Latin 键经 XLookupString → EventText；composition 单测走 Headless InjectIME
//
//  6. 开一次 Modal — 点 “Open Modal”；遮罩/OK/Cancel
//
//     export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//     go run ./examples/ui_polish_gallery
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
	winW, winH := 1024, 768
	// Default unlimited; set GPUI_ANIM_SECONDS>0 for timed CI smoke.
	seconds := 0.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 0 {
			seconds = 0
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_polish_gallery · 1024×768",
		// Scale left 0 → LinuxHost reads GPUI_SCALE / GDK_SCALE (default 1).
	})
	if err != nil {
		log.Fatalf("host: %v", err)
	}
	defer host.Close()
	log.Printf("host caps=%s CapIME=%v (CJK composition needs CapIME; see README)",
		host.Caps(), host.Caps().Has(platform.CapIME))

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
	adapter, device, err := exboot.OpenDevice(inst, surf, "ui-polish-gallery")
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
	exboot.WireAutoRecover(sc, adapter, "ui-polish-gallery",
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
	status := "ready — Tab for focus · click controls"

	// --- Section headers ---

	title := kit.NewText("Polish gallery · Ant-style kit (tabs by control type)")
	title.SetFace(face)
	title.Root.FontSize = 16

	hint := kit.NewText("Tabs switch panels · hover/press · Switch slide · Style API · Modal")
	hint.SetFace(face)
	hint.SetSecondary(true)

	// --- Buttons panel ---
	mkBtn := func(label string, typ kit.ButtonType, disabled bool) *kit.Button {
		b := kit.NewButton(label)
		b.SetType(typ)
		b.SetFace(face)
		if disabled {
			b.SetDisabled(true)
		}
		b.SetOnClick(func() { status = "click " + label })
		return b
	}
	primary := mkBtn("Primary", kit.ButtonPrimary, false)
	def := mkBtn("Default", kit.ButtonDefault, false)
	dashed := mkBtn("Dashed", kit.ButtonDashed, false)
	textBtn := mkBtn("Text", kit.ButtonText, false)
	link := mkBtn("Link", kit.ButtonLink, false)
	dis := mkBtn("Disabled", kit.ButtonPrimary, true)
	buttons := []*kit.Button{primary, def, dashed, textBtn, link, dis}
	btnRow := primitive.Row()
	btnRow.Gap = 10
	btnRow.CrossAlign = core.CrossCenter
	for _, b := range buttons {
		btnRow.AddChild(b.Node())
	}

	styGreen := kit.NewButton("Green 36")
	styGreen.SetFace(face)
	styGreen.SetStyle(kit.Style{
		Background: render.Hex("#52C41A"),
		Text:       render.Hex("#FFFFFF"),
		Height:     36,
		Radius:     8,
		FontSize:   14,
	})
	styGreen.SetOnClick(func() { status = "style green" })
	styOrange := kit.NewButton("Orange 8pt")
	styOrange.SetFace(face)
	styOrange.SetType(kit.ButtonDefault)
	styOrange.SetStyle(kit.Style{
		Background: render.Hex("#FFF7E6"),
		Border:     render.Hex("#FA8C16"),
		Text:       render.Hex("#D46B08"),
		Height:     28,
		Radius:     4,
		FontSize:   8,
	})
	styOrange.SetOnClick(func() { status = "style orange" })
	styBig := kit.NewButton("Tall 44")
	styBig.SetFace(face)
	styBig.SetType(kit.ButtonPrimary)
	styBig.SetStyle(kit.Style{Height: 44, FontSize: 16, Radius: 10, Width: 140})
	styBig.SetOnClick(func() { status = "style tall" })
	styleRow := primitive.Row(styGreen.Node(), styOrange.Node(), styBig.Node())
	styleRow.Gap = 10
	styleRow.CrossAlign = core.CrossCenter
	buttons = append(buttons, styGreen, styOrange, styBig)

	labTypes := kit.NewText("Types · primary / default / dashed / text / link / disabled")
	labTypes.SetFace(face)
	labTypes.SetSecondary(true)
	labStyle := kit.NewText("Style API · custom bg / text / height / font size")
	labStyle.SetFace(face)
	labStyle.SetSecondary(true)
	panelButtons := primitive.Column(labTypes.Node(), btnRow, labStyle.Node(), styleRow)
	panelButtons.Gap = 12
	panelButtons.CrossAlign = core.CrossStart
	panelButtons.Padding = primitive.All(12)

	// --- Input panel ---
	labIn := kit.NewText("Input · idle / type / focus border")
	labIn.SetFace(face)
	labIn.SetSecondary(true)
	name := kit.NewInput("Placeholder — type here")
	name.SetFace(face)
	name.SetOnChange(func(v string) { status = "input=" + v })
	name2 := kit.NewInput("Custom style")
	name2.SetFace(face)
	name2.SetStyle(kit.Style{
		Background: render.Hex("#F6FFED"),
		Border:     render.Hex("#52C41A"),
		Text:       render.Hex("#135200"),
		Height:     36,
		FontSize:   14,
	})
	panelInput := primitive.Column(labIn.Node(), name.Node(), name2.Node())
	panelInput.Gap = 12
	panelInput.CrossAlign = core.CrossStart
	panelInput.Padding = primitive.All(12)

	// --- Indicators panel ---
	labInd := kit.NewText("Checkbox / Radio (hollow selected) / Switch (press stretch + slide)")
	labInd.SetFace(face)
	labInd.SetSecondary(true)
	cb := kit.NewCheckbox("I agree")
	cb.SetFace(face)
	cb.SetOnChange(func(v bool) { status = fmt.Sprintf("checkbox=%v", v) })
	ra := kit.NewRadio("a", "Option A")
	rb := kit.NewRadio("b", "Option B")
	ra.SetFace(face)
	rb.SetFace(face)
	rg := kit.NewRadioGroup(ra, rb)
	rg.OnChange = func(v string) { status = "radio=" + v }
	rg.Select("a")
	sw := kit.NewSwitch()
	sw.OnChange = func(v bool) { status = fmt.Sprintf("switch=%v", v) }
	swLab := kit.NewText("Notifications")
	swLab.SetFace(face)
	swRow := primitive.Row(sw.Node(), swLab.Node())
	swRow.Gap = 10
	swRow.CrossAlign = core.CrossCenter
	panelInd := primitive.Column(labInd.Node(), cb.Node(), rg.Node(), swRow)
	panelInd.Gap = 12
	panelInd.CrossAlign = core.CrossStart
	panelInd.Padding = primitive.All(12)

	// --- Feedback panel ---
	labFb := kit.NewText("Progress / Spin / Skeleton")
	labFb.SetFace(face)
	labFb.SetSecondary(true)
	prog := kit.NewProgress(65)
	prog.Width = 320
	spin := kit.NewSpin(nil)
	sk := kit.NewSkeleton(240, 16)
	sk.SetActive(false)
	panelFb := primitive.Column(labFb.Node(), prog.Node(), spin.Node(), sk.Node())
	panelFb.Gap = 12
	panelFb.CrossAlign = core.CrossStart
	panelFb.Padding = primitive.All(12)

	// --- Select / Menu panel ---
	labSel := kit.NewText("Select / Menu")
	labSel.SetFace(face)
	labSel.SetSecondary(true)
	sel := kit.NewSelect("Please select",
		kit.SelectOption{Value: "1", Label: "One"},
		kit.SelectOption{Value: "2", Label: "Two"},
	)
	if face != nil {
		sel.SetFace(face)
	}
	menu := kit.NewMenu(
		kit.MenuItem{Key: "a", Label: "Item A"},
		kit.MenuItem{Key: "b", Label: "Item B"},
		kit.MenuItem{Key: "c", Label: "Item C"},
	)
	if face != nil {
		menu.Face = face
	}
	menu.SetSelected("b")
	panelSel := primitive.Column(labSel.Node(), sel.Node(), menu.Node())
	panelSel.Gap = 12
	panelSel.CrossAlign = core.CrossStart
	panelSel.Padding = primitive.All(12)

	// --- Modal panel ---
	labModal := kit.NewText("Modal · Open then Esc / mask / buttons")
	labModal.SetFace(face)
	labModal.SetSecondary(true)
	modalBody := kit.NewText("Minimal modal body for polish walkthrough.")
	modalBody.SetFace(face)
	modal := kit.NewModal("Confirm")
	modal.Face = face
	modal.SetContent(modalBody.Node())
	modal.Viewport = core.Size{Width: float64(winW), Height: float64(winH)}
	modal.OnOk = func() {
		status = "modal ok"
		modal.SetOpen(false)
	}
	modal.OnCancel = func() {
		status = "modal cancel"
		modal.SetOpen(false)
	}
	openModal := kit.NewButton("Open Modal")
	openModal.SetType(kit.ButtonDefault)
	openModal.SetFace(face)
	openModal.SetOnClick(func() {
		modal.SetOpen(true)
		status = "modal open"
	})
	buttons = append(buttons, openModal)
	panelModal := primitive.Column(labModal.Node(), openModal.Node(), modal.Node())
	panelModal.Gap = 12
	panelModal.CrossAlign = core.CrossStart
	panelModal.Padding = primitive.All(12)

	// --- Tabs ---
	tabs := kit.NewTabs(
		kit.MenuItem{Key: "btn", Label: "Button"},
		kit.MenuItem{Key: "input", Label: "Input"},
		kit.MenuItem{Key: "ind", Label: "Checkbox/Radio/Switch"},
		kit.MenuItem{Key: "fb", Label: "Feedback"},
		kit.MenuItem{Key: "sel", Label: "Select/Menu"},
		kit.MenuItem{Key: "modal", Label: "Modal"},
	)
	tabs.Face = face
	tabs.SetPosition(kit.TabLeft)
	// Configurable left-tabs metrics (0 → kit defaults: width 160, item height 40).
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.TabInkWidth = 3
	tabs.TabPadInline = 16
	tabs.TabPadBlock = 10
	tabs.SetContent("btn", panelButtons)
	tabs.SetContent("input", panelInput)
	tabs.SetContent("ind", panelInd)
	tabs.SetContent("fb", panelFb)
	tabs.SetContent("sel", panelSel)
	tabs.SetContent("modal", panelModal)
	tabs.SetActive("btn")

	statusTx := kit.NewText("status: " + status)
	statusTx.SetFace(face)
	statusTx.SetSecondary(true)

	tabsHost := primitive.NewFlexible(1, tabs.Node())
	col := primitive.Column(title.Node(), hint.Node(), tabsHost, statusTx.Node())
	col.Gap = 12
	col.CrossAlign = core.CrossStretch
	col.Padding = primitive.All(16)

	root := primitive.NewBox(col)
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)

	tree := core.NewTree(root)
	sw.AttachTicker(tree)
	lastStatus := status

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
			modal.Viewport = core.Size{Width: float64(w), Height: float64(h)}
			root.MarkNeedsLayout()
		},
		OnUpdate: func(dt float64) {
			for _, b := range buttons {
				b.SyncState()
			}
			openModal.SyncState()
			cb.SyncState()
			ra.SyncState()
			rb.SyncState()
			sw.SyncState()
			modal.Sync()
			if status != lastStatus {
				statusTx.SetValue("status: " + status)
				lastStatus = status
			}
		},
	})
	log.Printf("gallery exit status=%q paints=%d hops=%d scale=%.2f", status, res.Paints, res.Hops, host.ScaleFactor())
}
