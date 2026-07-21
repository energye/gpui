//go:build linux && !nogpu

// antd_desktop_app — real desktop UI shell modeled on Ant Design Pro patterns.
//
// Upper-layer mapping (antd / Electron / Tauri host):
//
//   - Layout: Header + Sider + Content + Footer (ProLayout)
//
//   - Table: dense row list with sticky header strip (Table)
//
//   - Modal: dimmed mask + centered panel open/close (Modal)
//
//   - Drawer: side panel slide (Drawer)
//
//   - Toast: transient notification stack (message/notification)
//
//   - Primary token colors, card surfaces, split content
//
//   - Host: minimize → Unconfigure; ForceRecoverHealthy mid-session
//
//     GPUI_ANIM_SECONDS=12 go run ./examples/antd_desktop_app
//     GPUI_FORCE_LOST_AFTER=50 /tmp/antd_desktop_app
//     GPUI_SELFTEST_LIFECYCLE=1 GPUI_SELFTEST_MIN_AT=40 ... /tmp/antd_desktop_app
package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/energye/gpui/examples/exboot"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
)

// Ant Design default token-ish palette (approx).
var (
	cBgLayout    = [3]float64{0.94, 0.95, 0.97} // #f0f2f5
	cBgContainer = [3]float64{1, 1, 1}
	cPrimary     = [3]float64{0.09, 0.38, 0.91} // #1677ff
	cPrimaryHov  = [3]float64{0.25, 0.51, 0.95}
	cText        = [3]float64{0.0, 0.0, 0.0}
	cTextSec     = [3]float64{0.0, 0.0, 0.0}
	cBorder      = [3]float64{0.85, 0.85, 0.85}
	cSuccess     = [3]float64{0.32, 0.70, 0.38}
	cWarning     = [3]float64{0.98, 0.69, 0.13}
	cError       = [3]float64{1.0, 0.30, 0.28}
	cHeader      = [3]float64{0.00, 0.13, 0.27} // dark header
	cSider       = [3]float64{0.00, 0.13, 0.27}
)

func main() {
	bootstrapEnv()
	animSec := envInt("GPUI_ANIM_SECONDS", 12)
	targetFPS := envInt("GPUI_TARGET_FPS", 60)
	frameBudget := time.Second / time.Duration(max(15, targetFPS))

	winW, winH := 1100, 720
	xw, err := openX11Window(winW, winH, "gpui · Ant Design desktop shell")
	must(err)
	defer xw.Close()

	exboot.InitEnv()
	inst, err := exboot.NewInstanceX11(xw.Display, 0)
	must(err)
	defer inst.Release()
	surf, err := inst.CreateSurface(xw.Display, xw.Window)
	must(err)
	defer surf.Release()
	adapter, device, err := exboot.OpenDevice(inst, surf, "antd-desktop-app")
	must(err)
	defer adapter.Release()
	defer func() {
		if device != nil {
			device.Release()
		}
	}()

	sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	must(sc.ConfigureFromCapabilities(adapter))
	defer sc.Release()
	must(exboot.BindProvider(device, adapter, sc.Format))
	defer exboot.ResetAccelerator()

	dc := render.NewContext(winW, winH)
	defer dc.Close()
	// Modal content is a real offscreen Context (saveLayer / portal pattern).
	modalRT := render.NewContext(420, 280)
	defer modalRT.Close()

	needForceFull := true // first frame full clear present
	dropGPU := func() {
		dc.DropGPURenderContext()
		modalRT.DropGPURenderContext()
		needForceFull = true
	}
	exboot.WireAutoRecover(sc, adapter, "antd-desktop-app",
		func(dev *webgpu.Device) { device = dev },
		dropGPU,
		nil,
	)
	surfHost := &exboot.SurfaceHost{SC: sc, Adapter: adapter, Device: &device, DropGPU: dropGPU, Format: sc.Format}

	// App model (antd Pro: route + table data + modal/drawer/toast state).
	type toast struct {
		msg  string
		kind int // 0 info 1 success 2 warn 3 error
		born float64
		life float64
	}
	selectedMenu := 0
	menus := []string{"Dashboard", "List", "Form", "Profile", "Settings"}
	tableScroll := 0.0
	modalOpen := false
	modalT := 0.0 // 0..1 open animation
	drawerOpen := false
	drawerT := 0.0
	toasts := []toast{}
	btnHover := -1
	clock := 0.0

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	start := time.Now()
	frame, tick, presents := 0, 0, 0
	exitReason := "duration"
	windowMinimized, windowFullyObscured, windowFocused := false, false, true
	wasHidden := false
	hiddenSince := time.Time{}
	hiddenAccum := time.Duration(0)

	// Selftest defaults (optional).
	minAt, mapAt, lostAt, doneAt := 80, 140, 190, 260
	if v := os.Getenv("GPUI_SELFTEST_MIN_AT"); v != "" {
		fmt.Sscanf(v, "%d", &minAt)
	}
	if v := os.Getenv("GPUI_SELFTEST_MAP_AT"); v != "" {
		fmt.Sscanf(v, "%d", &mapAt)
	}
	if v := os.Getenv("GPUI_SELFTEST_LOST_AT"); v != "" {
		fmt.Sscanf(v, "%d", &lostAt)
	}
	if v := os.Getenv("GPUI_SELFTEST_DONE_AT"); v != "" {
		fmt.Sscanf(v, "%d", &doneAt)
	}
	selftest := os.Getenv("GPUI_SELFTEST_LIFECYCLE") == "1"

	log.Printf("antd_desktop_app %dx%d fps=%d seconds=%d selftest=%v", winW, winH, targetFPS, animSec, selftest)

	running := true
	for running {
		tick++
		deadline := time.Now().Add(frameBudget)
		// Events
		for xw.Pending() {
			ev := xw.NextEvent()
			if xw.IsDelete(ev) {
				running = false
				exitReason = "window_close"
				break
			}
			if ev.Type == xVisibilityNotify {
				windowFullyObscured = ev.Visibility == xVisibilityFullyObscured
			}
			if xw.IsWMStateProperty(ev) {
				windowMinimized = xw.IsIconic()
			}
			if ev.Type == xFocusIn {
				windowFocused = true
			}
			if ev.Type == xFocusOut {
				windowFocused = false
			}
			if ev.Type == xConfigureNotify {
				nw, nh := max(64, ev.Width), max(64, ev.Height)
				if nw != winW || nh != winH {
					winW, winH = nw, nh
					_ = dc.Resize(winW, winH)
					_ = sc.Resize(uint32(winW), uint32(winH))
					needForceFull = true
				}
			}
		}
		if !running {
			break
		}
		select {
		case <-stop:
			running = false
			exitReason = "signal"
			continue
		default:
		}

		// Demo script: open modal, drawer, toasts on a timeline (real product flows).
		clock = float64(frame) / 60.0
		if frame == 45 {
			modalOpen = true
		}
		if frame == 120 {
			modalOpen = false
		}
		if frame == 90 {
			drawerOpen = true
		}
		if frame == 160 {
			drawerOpen = false
		}
		if frame > 0 && frame%90 == 30 {
			kinds := []int{0, 1, 2, 3}
			msgs := []string{"Saved draft", "Submit success", "Network slow", "Permission denied"}
			k := kinds[(frame/90)%4]
			toasts = append(toasts, toast{msg: msgs[k], kind: k, born: clock, life: 3.0})
			if len(toasts) > 4 {
				toasts = toasts[len(toasts)-4:]
			}
		}
		// Menu selection cycles (route change).
		selectedMenu = (frame / 180) % len(menus)
		// Table virtual scroll.
		tableScroll = math.Mod(float64(frame)*0.35, 400)

		// Modal / drawer animation (antd CSSMotion-like).
		target := 0.0
		if modalOpen {
			target = 1
		}
		modalT += (target - modalT) * 0.18
		target = 0.0
		if drawerOpen {
			target = 1
		}
		drawerT += (target - drawerT) * 0.16
		// Simulated hover on primary button.
		btnHover = (frame / 40) % 5

		// Lifecycle injectors
		if selftest {
			switch tick {
			case minAt:
				log.Printf("ANTD: Iconify tick=%d", tick)
				xw.Iconify()
			case mapAt:
				log.Printf("ANTD: MapRaise tick=%d", tick)
				xw.MapRaise()
				windowMinimized = false
				windowFullyObscured = false
				windowFocused = true
			case lostAt:
				log.Printf("ANTD: ForceRecoverHealthy tick=%d frame=%d", tick, frame)
				if err := sc.ForceRecoverHealthy(); err != nil {
					log.Printf("ForceRecoverHealthy: %v", err)
				} else if sc.Device != nil {
					device = sc.Device
					needForceFull = true
				}
			}
			if tick >= doneAt {
				exitReason = "selftest_ok"
				break
			}
		}

		hidden := windowMinimized || windowFullyObscured
		// Unfocused but visible: KEEP presenting (antd desktop).
		_ = windowFocused
		if hidden {
			if !wasHidden {
				wasHidden = true
				hiddenSince = time.Now()
				surfHost.OnUnpresentable()
				log.Printf("hidden — adaptive unpresentable")
			}
			if device != nil {
				device.FlushCallbacks()
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if wasHidden {
			if !hiddenSince.IsZero() {
				hiddenAccum += time.Since(hiddenSince)
				hiddenSince = time.Time{}
			}
			wasHidden = false
			needForceFull = true
			surfHost.OnPresentable()
			if sc.Device != nil {
				device = sc.Device
			}
			log.Printf("visible again recoveries=%d", sc.Recoveries())
		}

		if animSec > 0 && time.Since(start)-hiddenAccum >= time.Duration(animSec)*time.Second {
			exitReason = "duration"
			break
		}

		fw, fh := float64(winW), float64(winH)
		const (
			headerH = 56.0
			siderW  = 200.0
			footerH = 40.0
		)

		// --- Modal offscreen (antd Modal body) ---
		var modalImg *render.ImageBuf
		if modalT > 0.02 {
			modalRT.BeginFrame()
			fill(modalRT, cBgContainer, 0, 0, 420, 280)
			// title bar
			fill(modalRT, cPrimary, 0, 0, 420, 48)
			// body cards
			fillRGB(modalRT, 0.96, 0.97, 0.98, 24, 72, 372, 80)
			fillRGB(modalRT, 0.96, 0.97, 0.98, 24, 164, 180, 72)
			fillRGB(modalRT, 0.96, 0.97, 0.98, 216, 164, 180, 72)
			// primary OK button
			fill(modalRT, cPrimary, 280, 230, 100, 32)
			_ = modalRT.ExportImageBuf(&modalImg)
		}

		dc.BeginFrame()
		if needForceFull {
			dc.MarkFullRedraw()
			needForceFull = false
		}

		// Layout background
		fill(dc, cBgLayout, 0, 0, fw, fh)

		// Header (antd Layout.Header dark)
		fill(dc, cHeader, 0, 0, fw, headerH)
		// Brand block
		fill(dc, cPrimary, 16, 12, 32, 32)
		// Header actions
		for i := 0; i < 3; i++ {
			fillRGB(dc, 1, 1, 1, fw-140+float64(i)*40, 16, 28, 24)
		}

		// Sider
		fill(dc, cSider, 0, headerH, siderW, fh-headerH-footerH)
		for i, _ := range menus {
			y := headerH + 16 + float64(i)*48
			if i == selectedMenu {
				fill(dc, cPrimary, 8, y, siderW-16, 40)
			} else {
				fillRGB(dc, 0.05, 0.18, 0.35, 8, y, siderW-16, 40)
			}
			_ = i
		}

		// Content area
		cx, cy := siderW+16, headerH+16
		cw, ch := fw-siderW-32, fh-headerH-footerH-32
		fill(dc, cBgContainer, cx, cy, cw, ch)
		// Content border
		strokeRect(dc, cBorder, cx, cy, cw, ch, 1)

		// Page header + primary button (antd PageHeader / Button)
		fillRGB(dc, 0.98, 0.98, 0.99, cx+16, cy+12, cw-32, 48)
		btnC := cPrimary
		if btnHover == 0 {
			btnC = cPrimaryHov
		}
		fill(dc, btnC, cx+cw-140, cy+20, 100, 32)

		// Table (antd Table): sticky header + scrolling rows
		tx, ty := cx+16, cy+76
		tw, th := cw-32, ch-100
		fillRGB(dc, 0.98, 0.98, 0.98, tx, ty, tw, 36) // header
		rowH := 32.0
		visibleRows := int(th/rowH) - 1
		base := int(tableScroll/rowH) % 200
		for r := 0; r < visibleRows; r++ {
			ry := ty + 36 + float64(r)*rowH
			if r%2 == 0 {
				fillRGB(dc, 1, 1, 1, tx, ry, tw, rowH)
			} else {
				fillRGB(dc, 0.98, 0.99, 1.0, tx, ry, tw, rowH)
			}
			// status tag cells
			tag := (base + r) % 4
			var tc [3]float64
			switch tag {
			case 0:
				tc = cPrimary
			case 1:
				tc = cSuccess
			case 2:
				tc = cWarning
			default:
				tc = cError
			}
			fill(dc, tc, tx+tw-90, ry+6, 70, 20)
			_ = r
		}
		// Table grid lines (light)
		for r := 0; r <= visibleRows; r++ {
			y := ty + 36 + float64(r)*rowH
			fill(dc, cBorder, tx, y, tw, 1)
		}

		// Cards row (antd Card / Statistic)
		cardY := cy + ch - 88
		for i := 0; i < 3; i++ {
			x := cx + 16 + float64(i)*(cw/3)
			fillRGB(dc, 1, 1, 1, x, cardY, cw/3-24, 72)
			strokeRect(dc, cBorder, x, cardY, cw/3-24, 72, 1)
			fill(dc, cPrimary, x+12, cardY+16, 8, 40)
		}

		// Drawer (antd Drawer from right)
		if drawerT > 0.01 {
			dw := 320.0 * drawerT
			fillRGBA(dc, 0, 0, 0, 0.35*drawerT, 0, 0, fw, fh)
			fill(dc, cBgContainer, fw-dw, headerH, dw, fh-headerH-footerH)
			fill(dc, cPrimary, fw-dw, headerH, dw, 48)
			for i := 0; i < 6; i++ {
				fillRGB(dc, 0.95, 0.96, 0.97, fw-dw+16, headerH+64+float64(i)*56, dw-32, 44)
			}
		}

		// Modal mask + panel
		if modalT > 0.02 && modalImg != nil {
			fillRGBA(dc, 0, 0, 0, 0.45*modalT, 0, 0, fw, fh)
			// scale-in from center
			mw, mh := 420.0, 280.0
			s := 0.85 + 0.15*modalT
			mw, mh = mw*s, mh*s
			mx, my := fw*0.5-mw*0.5, fh*0.5-mh*0.5
			dc.DrawImage(modalImg, mx, my)
		}

		// Toast stack (antd notification topRight)
		alive := toasts[:0]
		for _, t := range toasts {
			age := clock - t.born
			if age > t.life {
				continue
			}
			alive = append(alive, t)
		}
		toasts = alive
		for i, t := range toasts {
			age := clock - t.born
			alpha := 1.0
			if age > t.life-0.4 {
				alpha = (t.life - age) / 0.4
			}
			if age < 0.2 {
				alpha = age / 0.2
			}
			var tc [3]float64
			switch t.kind {
			case 1:
				tc = cSuccess
			case 2:
				tc = cWarning
			case 3:
				tc = cError
			default:
				tc = cPrimary
			}
			y := 72 + float64(i)*64
			fillRGBA(dc, 1, 1, 1, 0.96*alpha, fw-340, y, 320, 52)
			fillRGBA(dc, tc[0], tc[1], tc[2], alpha, fw-340, y, 6, 52)
		}

		// Footer
		fillRGB(dc, 1, 1, 1, 0, fh-footerH, fw, footerH)
		fill(dc, cBorder, 0, fh-footerH, fw, 1)
		// Focus strip: unfocused still draws (green = keep present policy)
		if !windowFocused {
			fillRGBA(dc, 0.2, 0.8, 0.4, 0.55, 0, 0, fw, 3)
		}

		if device != nil {
			device.FlushCallbacks()
		}
		fb, err := sc.BeginFrame()
		if err != nil {
			if errors.Is(err, webgpu.ErrRecovered) {
				needForceFull = true
				continue
			}
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				sc.ClearRecoverCooldown()
				if sc.Device != nil {
					device = sc.Device
				}
				time.Sleep(16 * time.Millisecond)
				continue
			}
			time.Sleep(2 * time.Millisecond)
			continue
		}
		if err := dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, func() error {
			return sc.EndFrame(fb)
		}); err != nil {
			sc.DiscardFrame(fb)
			needForceFull = true
			if errors.Is(err, webgpu.ErrDeviceLost) {
				time.Sleep(16 * time.Millisecond)
			}
			continue
		}
		presents++
		frame++

		if v := os.Getenv("GPUI_FORCE_LOST_AFTER"); v != "" {
			var n int
			fmt.Sscanf(v, "%d", &n)
			if n > 0 && frame == n {
				log.Printf("GPUI_FORCE_LOST_AFTER=%d ForceRecoverHealthy", n)
				if err := sc.ForceRecoverHealthy(); err != nil {
					log.Printf("ForceRecoverHealthy: %v", err)
				} else if sc.Device != nil {
					device = sc.Device
					needForceFull = true
				}
			}
		}

		if d := time.Until(deadline); d > 200*time.Microsecond {
			time.Sleep(d)
		}
	}

	log.Printf("DONE exit=%s frames=%d presents=%d recoveries=%d hidden=%.1fs modalT=%.2f drawerT=%.2f",
		exitReason, frame, presents, sc.Recoveries(), hiddenAccum.Seconds(), modalT, drawerT)
	if exitReason == "selftest_ok" || exitReason == "duration" || exitReason == "window_close" || exitReason == "signal" {
		os.Exit(0)
	}
	os.Exit(1)
}

func fill(dc *render.Context, c [3]float64, x, y, w, h float64) {
	dc.SetRGB(c[0], c[1], c[2])
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}
func fillRGB(dc *render.Context, r, g, b, x, y, w, h float64) {
	dc.SetRGB(r, g, b)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}
func fillRGBA(dc *render.Context, r, g, b, a, x, y, w, h float64) {
	dc.SetRGBA(r, g, b, a)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}
func strokeRect(dc *render.Context, c [3]float64, x, y, w, h, sw float64) {
	dc.SetRGB(c[0], c[1], c[2])
	dc.SetLineWidth(sw)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Stroke()
}

func bootstrapEnv() {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		if _, err := os.Stat("lib/libwgpu_native.so"); err == nil {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
		}
	}
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	}
}
func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// silence unused import if filters only
var _ = strings.TrimSpace
