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

	// UI face: Latin + CJK fallback. Flex/Divider pages use Chinese kit.NewText
	// titles/descriptions; DejaVu alone has no CJK glyphs → missing/bad strokes.
	// MultiFace keeps fallback when SetFontSize re-derives via AtSize.
	var face text.Face
	loadFace := func(path string, size float64) text.Face {
		src, err := text.NewFontSourceFromFile(path)
		if err != nil {
			return nil
		}
		return src.Face(size)
	}
	const uiPt = 14.0
	latinPaths := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"render/text/testdata/goregular.ttf",
	}
	cjkPaths := []string{
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
	}
	var latin, cjk text.Face
	for _, path := range latinPaths {
		if f := loadFace(path, uiPt); f != nil {
			latin = f
			log.Printf("font latin %s", path)
			break
		}
	}
	for _, path := range cjkPaths {
		if f := loadFace(path, uiPt); f != nil {
			cjk = f
			log.Printf("font cjk %s", path)
			break
		}
	}
	switch {
	case latin != nil && cjk != nil:
		if mf, err := text.NewMultiFace(latin, cjk); err == nil {
			face = mf
			log.Printf("font MultiFace latin+cjk @%.0fpt", uiPt)
		} else {
			face = latin
			log.Printf("font MultiFace failed: %v — latin only", err)
		}
	case latin != nil:
		face = latin
	case cjk != nil:
		face = cjk
	}

	theme := kit.DefaultTheme()
	// --- Ant Design catalog panels ---

	title := kit.NewText("Polish gallery · Ant Design catalog")
	title.SetFace(face)
	title.SetFontSize(16)

	var buttons []*kit.Button
	var tickers []interface{ AttachTicker(*core.Tree) }
	status := "ready — catalog"
	// App-level notify host (Ant App): must remain mounted for Message/Notification.
	msgHost := kit.NewMessageHost()
	msgHost.Face = face
	msgHost.Viewport = core.Size{Width: float64(winW), Height: float64(winH)}
	items, panels, modal := buildCatalogPanels(face, theme, &status, &buttons, &tickers, msgHost)

	tabs := kit.NewTabs(items...)
	tabs.Face = face
	tabs.SetPosition(kit.TabLeft)
	tabs.SetTabWidth(168)
	tabs.SetTabItemHeight(36)
	tabs.SetInkSize(3)
	tabs.SetInkAnimated(true)
	for k, n := range panels {
		tabs.SetContent(k, n)
	}
	if k := tabs.FirstSelectableKey(); k != "" {
		tabs.SetActive(k)
	}
	// Sliding ink indicator animation (demand loop TickActive)
	tickers = append(tickers, tabs)

	// Title bar (compact) + Tabs filling the rest of the window.
	titleBar := primitive.NewDecorated(title.Node())
	titleBar.Padding = primitive.Symmetric(16, 10)
	titleBar.Background = theme.Color(core.TokenColorBgLayout)
	titleBar.ExpandWidth = true

	tabsHost := primitive.NewFlexible(1, tabs.Node())
	tabsHost.FillChild = true
	// msgHost: zero-size OverlayPortal; keep mounted at root for Message/Notification.
	col := primitive.Column(titleBar, tabsHost, msgHost.Node())
	col.Gap = 0
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	// No outer padding: Tabs rail/body go edge-to-edge under the title bar.

	// StretchChild + fixed size: Column receives TIGHT max so Flexible truly
	// consumes remaining height (Box+Expand only passes loose max).
	root := primitive.NewDecorated(col)
	root.Background = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)
	root.StretchChild = true
	root.ExpandWidth = true

	tree := core.NewTree(root)
	for _, tk := range tickers {
		tk.AttachTicker(tree)
	}
	if modal != nil {
		modal.Viewport = core.Size{Width: float64(winW), Height: float64(winH)}
	}

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
			if modal != nil {
				modal.Viewport = core.Size{Width: float64(w), Height: float64(h)}
			}
			if msgHost != nil {
				msgHost.Viewport = core.Size{Width: float64(w), Height: float64(h)}
			}
			root.MarkNeedsLayout()
		},
		OnUpdate: func(dt float64) {
			for _, b := range buttons {
				b.SyncState()
			}
			if modal != nil {
				modal.Sync()
			}
			if msgHost != nil {
				msgHost.Sync()
			}
		},
	})
	log.Printf("gallery exit paints=%d hops=%d scale=%.2f", res.Paints, res.Hops, host.ScaleFactor())
}
