//go:build linux && !nogpu

// ui_polish_gallery — §12.3 W3 polish gallery for visual + focus walkthrough.
//
// §12.2 手操清单（本示例可完成 #1–4；#5–6 见 W4）:
//
//  1. 静态浏览 — 圆角、1px 边、间距是否像控件
//
//  2. 鼠标扫 Button/Checkbox — hover/press（Button 每帧 SyncState）
//
//  3. Tab 走焦 — focus 环（Button）/ 主色边框（Input）可见
//
//  4. 点 Checkbox/Radio — 选中圆滑、居中
//
//  5. Linux 中文输入 — W4
//
//  6. 开一次 Modal — 点 “Open Modal”；Esc/遮罩关闭（有则验收）
//
//     export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//     GPUI_ANIM_SECONDS=60 go run ./examples/ui_polish_gallery
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
	winW, winH := 720, 560
	seconds := 60.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 1 {
			seconds = 1
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_polish_gallery (W3)",
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
	title := kit.NewText("Polish gallery · W3 (Button / Input / Indicators / Modal)")
	title.SetFace(face)
	title.Root.FontSize = 16

	hint := kit.NewText("Checklist §12.2 #1–4 on this screen · #5–6 IME/Modal notes → W4")
	hint.SetFace(face)
	hint.SetSecondary(true)

	// --- Buttons multi-type ---
	secBtn := kit.NewText("Button · primary / default / dashed / text / link / disabled")
	secBtn.SetFace(face)
	secBtn.SetSecondary(true)

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

	// --- Input ---
	secIn := kit.NewText("Input · idle / type / focus border (Tab or click)")
	secIn.SetFace(face)
	secIn.SetSecondary(true)
	name := kit.NewInput("Placeholder — type here")
	name.SetFace(face)
	name.SetOnChange(func(v string) { status = "input=" + v })

	// --- Checkbox / Radio ---
	secInd := kit.NewText("Checkbox / Radio / Switch")
	secInd.SetFace(face)
	secInd.SetSecondary(true)

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

	// --- Modal ---
	secModal := kit.NewText("Modal · Open then Esc / mask / buttons")
	secModal.SetFace(face)
	secModal.SetSecondary(true)

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

	statusTx := kit.NewText("status: " + status)
	statusTx.SetFace(face)
	statusTx.SetSecondary(true)

	col := primitive.Column(
		title.Node(),
		hint.Node(),
		secBtn.Node(),
		btnRow,
		secIn.Node(),
		name.Node(),
		secInd.Node(),
		cb.Node(),
		rg.Node(),
		swRow,
		secModal.Node(),
		openModal.Node(),
		statusTx.Node(),
		modal.Node(), // zero-size portal host
	)
	col.Gap = 12
	col.CrossAlign = core.CrossStart
	col.Padding = primitive.All(20)

	root := primitive.NewBox(col)
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)

	tree := core.NewTree(root)
	lastStatus := status
	deadline := time.Now().Add(time.Duration(seconds * float64(time.Second)))

	for time.Now().Before(deadline) {
		for _, ev := range host.PumpEvents() {
			if ev.Type == platform.EventClose {
				goto done
			}
			if ev.Type == platform.EventKey && ev.Down && len(ev.Key) == 1 {
				tree.DispatchTextInput(&core.TextInputEvent{Text: ev.Key})
				continue
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
				modal.Viewport = core.Size{Width: float64(winW), Height: float64(winH)}
				root.MarkNeedsLayout()
				_ = sc.Resize(uint32(winW), uint32(winH))
				_ = dc.Resize(winW, winH)
			}
		}
		// Hover/press chrome for buttons.
		for _, b := range buttons {
			b.SyncState()
		}
		openModal.SyncState()
		modal.Sync()
		if status != lastStatus {
			statusTx.SetValue("status: " + status)
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
	}
done:
	log.Printf("gallery exit status=%q", status)
}
