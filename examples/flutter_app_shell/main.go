//go:build linux && !nogpu

// flutter_app_shell — real app host modeled on Flutter Material + Skia Canvas usage.
//
// Upper-layer mapping:
//
//   - Scaffold: AppBar + body + FAB + bottom NavigationBar (Flutter Material)
//
//   - Navigator stack: push/pop routes as layered offscreen RTs (Navigator 1.0)
//
//   - setState rebuild: widget tree re-laid every frame from model (Element rebuild)
//
//   - CustomPainter: continuous path/animation in a dedicated paint region
//
//   - ListView: scrolling cards (SliverList-like)
//
//   - AppLifecycle: minimize → pause present + Unconfigure; resume force-full
//
//   - ImageFilter-ish: offscreen route with blur-like frosted panel (layer RT)
//
//   - Device lost: ForceRecoverHealthy (Rasterizer rebind / GrContext abandon)
//
//     go run ./examples/flutter_app_shell
//     GPUI_FORCE_LOST_AFTER=55 /tmp/flutter_app_shell
//     GPUI_SELFTEST_LIFECYCLE=1 ... /tmp/flutter_app_shell
package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/energye/gpui/examples/exboot"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
)

// Material 3-ish tokens
var (
	mPrimary   = [3]float64{0.40, 0.31, 0.64} // purple
	mOnPrimary = [3]float64{1, 1, 1}
	mSurface   = [3]float64{0.98, 0.97, 0.99}
	mSurface2  = [3]float64{0.94, 0.93, 0.97}
	mOutline   = [3]float64{0.78, 0.76, 0.80}
	mAppBar    = [3]float64{0.40, 0.31, 0.64}
	mNav       = [3]float64{0.96, 0.95, 0.98}
	mFAB       = [3]float64{0.82, 0.29, 0.47}
	mCanvasBg  = [3]float64{0.12, 0.12, 0.16}
)

func main() {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		if _, err := os.Stat("lib/libwgpu_native.so"); err == nil {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
		}
	}
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	}

	animSec := envInt("GPUI_ANIM_SECONDS", 0)
	targetFPS := envInt("GPUI_TARGET_FPS", 60)
	frameBudget := time.Second / time.Duration(max(15, targetFPS))
	winW, winH := 420, 780 // phone-like portrait (Flutter mobile + desktop)

	xw, err := openX11Window(winW, winH, "gpui · Flutter Material shell")
	must(err)
	defer xw.Close()

	exboot.InitEnv()
	inst, err := exboot.NewInstanceX11(xw.Display, 0)
	must(err)
	defer inst.Release()
	surf, err := inst.CreateSurface(xw.Display, xw.Window)
	must(err)
	defer surf.Release()
	adapter, device, err := exboot.OpenDevice(inst, surf, "flutter-app-shell")
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

	// Root scaffold context
	root := render.NewContext(winW, winH)
	defer root.Close()
	// Route overlay (Navigator push) — separate Context like a secondary Skia surface.
	routeRT := render.NewContext(winW-48, winH/2)
	defer routeRT.Close()
	// CustomPainter canvas
	paintRT := render.NewContext(280, 160)
	defer paintRT.Close()

	needForceFull := true // first frame full clear present
	dropGPU := func() {
		root.DropGPURenderContext()
		routeRT.DropGPURenderContext()
		paintRT.DropGPURenderContext()
		needForceFull = true
	}
	exboot.WireAutoRecover(sc, adapter, "flutter-app-shell",
		func(dev *webgpu.Device) { device = dev },
		dropGPU,
		nil,
	)
	surfHost := &exboot.SurfaceHost{SC: sc, Adapter: adapter, Device: &device, DropGPU: dropGPU, Format: sc.Format}

	// Model (setState fields)
	navIndex := 0
	listScroll := 0.0
	routeStack := 0 // 0 home, 1 detail pushed
	routeT := 0.0
	fabPulse := 0.0
	painterT := 0.0

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	start := time.Now()
	frame, tick, presents := 0, 0, 0
	exitReason := "duration"
	windowMinimized, windowFullyObscured, windowFocused := false, false, true
	wasHidden := false
	hiddenSince := time.Time{}
	hiddenAccum := time.Duration(0)

	minAt, mapAt, lostAt, doneAt := 70, 120, 170, 230
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

	log.Printf("flutter_app_shell %dx%d fps=%d seconds=%d", winW, winH, targetFPS, animSec)

	running := true
	for running {
		tick++
		deadline := time.Now().Add(frameBudget)
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
					_ = root.Resize(winW, winH)
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

		// setState timeline
		navIndex = (frame / 120) % 4
		listScroll = math.Mod(float64(frame)*0.8, 600)
		fabPulse = 0.5 + 0.5*math.Sin(float64(frame)*0.12)
		painterT = float64(frame) * 0.05
		// Navigator push/pop
		if frame == 60 {
			routeStack = 1
		}
		if frame == 150 {
			routeStack = 0
		}
		target := 0.0
		if routeStack > 0 {
			target = 1
		}
		routeT += (target - routeT) * 0.15

		if selftest {
			switch tick {
			case minAt:
				log.Printf("FLUTTER: Iconify tick=%d (AppLifecycle.paused)", tick)
				xw.Iconify()
			case mapAt:
				log.Printf("FLUTTER: MapRaise tick=%d (AppLifecycle.resumed)", tick)
				xw.MapRaise()
				windowMinimized = false
				windowFullyObscured = false
				windowFocused = true
			case lostAt:
				log.Printf("FLUTTER: ForceRecoverHealthy tick=%d (Rasterizer rebind)", tick)
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
		_ = windowFocused // unfocused still paints
		if hidden {
			if !wasHidden {
				wasHidden = true
				hiddenSince = time.Now()
				surfHost.OnUnpresentable()
				log.Printf("paused — adaptive unpresentable")
			}
			// Model still ticks while paused (Flutter isolates / timers).
			painterT += 0.02
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
			log.Printf("resumed recoveries=%d", sc.Recoveries())
		}

		if animSec > 0 && time.Since(start)-hiddenAccum >= time.Duration(animSec)*time.Second {
			exitReason = "duration"
			break
		}

		fw, fh := float64(winW), float64(winH)
		const appBarH, navH, fabR = 56.0, 64.0, 28.0

		// CustomPainter offscreen
		paintRT.BeginFrame()
		fill(paintRT, mCanvasBg, 0, 0, 280, 160)
		// animated star-ish path
		paintRT.SetRGB(0.6, 0.85, 1.0)
		paintRT.SetLineWidth(2)
		cx, cy := 140.0, 80.0
		for i := 0; i < 8; i++ {
			a0 := painterT + float64(i)*math.Pi/4
			a1 := a0 + 0.4
			paintRT.MoveTo(cx+50*math.Cos(a0), cy+40*math.Sin(a0))
			paintRT.LineTo(cx+70*math.Cos(a1), cy+55*math.Sin(a1))
		}
		_ = paintRT.Stroke()
		paintRT.SetRGB(1, 0.55, 0.7)
		paintRT.DrawCircle(cx+30*math.Cos(painterT*1.3), cy+20*math.Sin(painterT*1.7), 10+4*fabPulse)
		_ = paintRT.Fill()
		var paintImg *render.ImageBuf
		_ = paintRT.ExportImageBuf(&paintImg)

		// Route page offscreen
		var routeImg *render.ImageBuf
		if routeT > 0.02 {
			routeRT.BeginFrame()
			rw, rh := float64(winW-48), float64(winH/2)
			fill(routeRT, mSurface, 0, 0, rw, rh)
			fill(routeRT, mPrimary, 0, 0, rw, 48)
			// detail body
			for i := 0; i < 5; i++ {
				fill(routeRT, mSurface2, 16, 64+float64(i)*52, rw-32, 44)
			}
			_ = routeRT.ExportImageBuf(&routeImg)
		}

		// --- Scaffold rebuild (setState) ---
		root.BeginFrame()
		if needForceFull {
			root.MarkFullRedraw()
			needForceFull = false
		}
		fill(root, mSurface, 0, 0, fw, fh)

		// AppBar
		fill(root, mAppBar, 0, 0, fw, appBarH)
		// leading / title / actions placeholders
		fill(root, mOnPrimary, 16, 16, 24, 24)
		fillRGB(root, 1, 1, 1, fw-88, 16, 24, 24)
		fillRGB(root, 1, 1, 1, fw-48, 16, 24, 24)

		// Body
		bodyTop := appBarH
		bodyH := fh - appBarH - navH

		// Hero CustomPainter card
		if paintImg != nil {
			root.DrawImage(paintImg, 20, bodyTop+16)
		}

		// ListView cards
		listY0 := bodyTop + 190
		cardH := 72.0
		nVis := int((bodyH-210)/cardH) + 1
		base := int(listScroll/cardH) % 100
		for i := 0; i < nVis; i++ {
			y := listY0 + float64(i)*cardH - math.Mod(listScroll, cardH)
			if y+cardH < bodyTop || y > bodyTop+bodyH {
				continue
			}
			fill(root, mSurface2, 16, y, fw-32, cardH-8)
			// avatar
			fill(root, mPrimary, 28, y+12, 40, 40)
			// text lines
			fillRGB(root, 0.3, 0.3, 0.35, 84, y+16, fw-140, 12)
			fillRGB(root, 0.55, 0.55, 0.6, 84, y+36, fw-180, 10)
			_ = base + i
		}

		// Navigator route slide from right
		if routeT > 0.02 && routeImg != nil {
			fillRGBA(root, 0, 0, 0, 0.35*routeT, 0, 0, fw, fh)
			ox := fw * (1 - routeT)
			root.DrawImage(routeImg, 24+ox*0.15, bodyTop+40)
		}

		// FAB
		fabX, fabY := fw-56, fh-navH-56
		r := fabR * (0.92 + 0.08*fabPulse)
		root.SetRGB(mFAB[0], mFAB[1], mFAB[2])
		root.DrawCircle(fabX, fabY, r)
		_ = root.Fill()
		// + icon bars
		fillRGB(root, 1, 1, 1, fabX-10, fabY-2, 20, 4)
		fillRGB(root, 1, 1, 1, fabX-2, fabY-10, 4, 20)

		// Bottom NavigationBar
		fill(root, mNav, 0, fh-navH, fw, navH)
		fill(root, mOutline, 0, fh-navH, fw, 1)
		for i := 0; i < 4; i++ {
			x := fw*float64(i)/4 + fw/8
			if i == navIndex {
				fill(root, mPrimary, x-16, fh-navH+12, 32, 32)
			} else {
				fillRGB(root, 0.7, 0.7, 0.75, x-12, fh-navH+16, 24, 24)
			}
		}

		if !windowFocused {
			fillRGBA(root, 0.2, 0.8, 0.4, 0.5, 0, 0, fw, 3)
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
		if err := root.PresentFrameFull(fb.Handle, fb.Width, fb.Height, func() error {
			return sc.EndFrame(fb)
		}); err != nil {
			sc.DiscardFrame(fb)
			needForceFull = true
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

	log.Printf("DONE exit=%s frames=%d presents=%d recoveries=%d hidden=%.1fs routeT=%.2f nav=%d",
		exitReason, frame, presents, sc.Recoveries(), hiddenAccum.Seconds(), routeT, navIndex)
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
