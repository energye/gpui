//go:build linux && !nogpu

// ui_kit_m5_smoke — M5 proof: Skeleton / Spin / Progress / Tour / Motion / A11y.
//
//	export DISPLAY=:1 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	GPUI_ANIM_SECONDS=12 go run ./examples/ui_kit_m5_smoke
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
	winW, winH := 720, 520
	seconds := 12.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 1 {
			seconds = 1
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_kit_m5_smoke (M5)",
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
	adapter, device, err := exboot.OpenDevice(inst, surf, "ui-kit-m5")
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
	exboot.WireAutoRecover(sc, adapter, "ui-kit-m5",
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
	vp := core.Size{Width: float64(winW), Height: float64(winH)}
	status := "ready"

	title := kit.NewText("M5 · Motion / Presence / Skeleton / Spin / Progress / Tour / A11y")
	title.SetFace(face)
	title.Root.FontSize = 15

	// Motion fade-in text
	hello := kit.NewText("Fading content (Motion)")
	hello.SetFace(face)
	motion := primitive.NewMotion(hello.Node())
	motion.Anim.Duration = 0.8
	motion.Anim.Start()

	// Presence panel
	presBody := kit.NewText("Presence panel — toggle hide/show")
	presBody.SetFace(face)
	presence := primitive.NewPresence(presBody.Node())
	togglePres := kit.NewButton("Toggle Presence")
	togglePres.SetFace(face)
	togglePres.SetOnClick(func() {
		if presence.WantVisible {
			presence.Hide()
			status = "presence hide"
		} else {
			presence.Show()
			status = "presence show"
		}
	})

	// Skeleton row
	sk1 := kit.NewSkeleton(180, 14)
	sk2 := kit.NewSkeleton(120, 14)
	sk3 := kit.NewSkeleton(160, 14)
	skRow := primitive.Column(sk1.Node(), sk2.Node(), sk3.Node())
	skRow.Gap = 8

	// Spin
	spinLabel := kit.NewText("Loading…")
	spinLabel.SetFace(face)
	spin := kit.NewSpin(spinLabel.Node())

	// Progress
	prog := kit.NewProgress(35)
	prog.Width = 280

	// Ring canvas
	ring := primitive.ProgressRing(48, 4, 0.65,
		theme.Color(core.TokenColorFillSecondary),
		theme.Color(core.TokenColorPrimary),
	)

	// Tour
	tourBtn := kit.NewButton("Start Tour")
	tourBtn.SetType(kit.ButtonPrimary)
	tourBtn.SetFace(face)
	tour := kit.NewTour(
		kit.TourStep{Title: "Welcome", Body: "This is a spotlight tour (M5 lite)."},
		kit.TourStep{Title: "Progress", Body: "Track loading with Progress & Spin."},
	)
	tour.Face = face
	tour.Viewport = vp
	tourBtn.SetOnClick(func() {
		// update targets after layout
		tour.Steps[0].Target = core.AbsoluteBounds(tourBtn.Root)
		tour.Steps[1].Target = core.AbsoluteBounds(prog.Node())
		tour.Index = 0
		tour.Viewport = vp
		tour.SetOpen(true)
		status = "tour open"
	})

	// A11y dump button
	a11yBtn := kit.NewButton("Log A11y")
	a11yBtn.SetFace(face)

	statusTx := kit.NewText("status: ready")
	statusTx.SetFace(face)
	statusTx.SetSecondary(true)

	row1 := primitive.Row(togglePres.Node(), tourBtn.Node(), a11yBtn.Node(), ring)
	row1.Gap = 12
	row1.CrossAlign = core.CrossCenter

	col := primitive.Column(
		title.Node(),
		motion,
		presence,
		kit.NewText("Skeleton").Node(),
		skRow,
		kit.NewText("Spin").Node(),
		spin.Node(),
		kit.NewText("Progress").Node(),
		prog.Node(),
		row1,
		statusTx.Node(),
		tour.Node(),
	)
	col.Gap = 12
	col.CrossAlign = core.CrossStart
	col.Padding = primitive.All(24)

	root := primitive.NewBox(col)
	root.Color = theme.Color(core.TokenColorBgLayout)
	root.Width = float64(winW)
	root.Height = float64(winH)
	tree := core.NewTree(root)

	a11yBtn.SetOnClick(func() {
		nodes := kit.CollectA11y(tree.Root())
		status = fmt.Sprintf("a11y nodes=%d", len(nodes))
		log.Printf("a11y: %+v", nodes)
	})

	deadline := time.Now().Add(time.Duration(seconds * float64(time.Second)))
	frames := 0
	last := status
	lastT := time.Now()
	pct := 35.0

	for time.Now().Before(deadline) {
		now := time.Now()
		dt := now.Sub(lastT).Seconds()
		lastT = now
		tree.TickClock(dt)

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
				vp = core.Size{Width: float64(winW), Height: float64(winH)}
				root.Width, root.Height = float64(winW), float64(winH)
				tour.Viewport = vp
				root.MarkNeedsLayout()
				_ = sc.Resize(uint32(winW), uint32(winH))
				_ = dc.Resize(winW, winH)
			}
		}

		// animations
		motion.AdvanceClock(tree)
		presence.Advance(dt, tree.Clock().ReduceMotion)
		sk1.Tick(dt)
		sk2.Tick(dt)
		sk3.Tick(dt)
		spin.Tick(dt)
		// Spin tick updates angle; full stack rebuild optional
		pct += dt * 15
		if pct > 100 {
			pct = 0
		}
		prog.SetPercent(pct)
		tour.Sync()

		if status != last {
			statusTx.SetValue("status: " + status)
			last = status
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
			time.Sleep(16 * time.Millisecond)
			continue
		}
		if _, err := dc.PresentFrameAuto(frame.Handle, frame.Width, frame.Height, func() error {
			return sc.EndFrame(frame)
		}); err != nil {
			sc.DiscardFrame(frame)
			if errors.Is(err, webgpu.ErrDeviceLost) {
				time.Sleep(16 * time.Millisecond)
			}
			continue
		}
		host.Flush()
		frames++
	}
done:
	fmt.Printf("ui_kit_m5_smoke done frames=%d status=%q a11y=%d %s\n",
		frames, status, len(kit.CollectA11y(tree.Root())), dc.RenderPathStats().LogLine())
}
