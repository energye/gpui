//go:build linux && !nogpu

// ui_kit_m5_smoke — M5 proof: Skeleton / Spin / Progress / Tour / Motion / A11y
// plus extra demand-frame animations (typewriter input, blink caret, counters).
//
// Demand-frame aligned (Flutter dirty): Continuous=false; animators use Tree.AddTicker
// and MarkNeedsPaint only — no full-tree MarkDirty every tick.
//
//	export DISPLAY=:1 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_kit_m5_smoke
package main

import (
	"fmt"
	"log"
	"math"
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

// progressTicker drives Progress percent under demand-frame ANIMATING.
type progressTicker struct {
	prog *kit.Progress
	pct  float64
	rate float64 // percent per second
}

func (p *progressTicker) Tick(dt float64) bool {
	if p == nil || p.prog == nil {
		return false
	}
	rate := p.rate
	if rate <= 0 {
		rate = 15
	}
	p.pct += dt * rate
	if p.pct > 100 {
		p.pct = 0
	}
	p.prog.SetPercent(p.pct)
	return true
}

// typewriterTicker simulates typing into an Input (IME-like demo without CapIME).
// Cycles: type full script → hold → clear → repeat.
type typewriterTicker struct {
	input  *kit.Input
	script []rune
	idx    int
	phase  int // 0 typing, 1 hold, 2 clear
	acc    float64
	cps    float64 // characters per second while typing
	hold   float64 // seconds to hold full text
}

func (t *typewriterTicker) Tick(dt float64) bool {
	if t == nil || t.input == nil || len(t.script) == 0 {
		return false
	}
	if t.cps <= 0 {
		t.cps = 12
	}
	if t.hold <= 0 {
		t.hold = 1.2
	}
	t.acc += dt
	switch t.phase {
	case 0: // typing
		interval := 1.0 / t.cps
		for t.acc >= interval && t.idx < len(t.script) {
			t.acc -= interval
			t.idx++
			t.input.SetValue(string(t.script[:t.idx]))
		}
		if t.idx >= len(t.script) {
			t.phase = 1
			t.acc = 0
		}
	case 1: // hold full string
		if t.acc >= t.hold {
			t.phase = 2
			t.acc = 0
		}
	case 2: // clear then restart
		t.idx = 0
		t.input.SetValue("")
		t.phase = 0
		t.acc = 0
	}
	return true
}

// blinkTicker toggles a caret-style indicator (text or canvas).
type blinkTicker struct {
	label *kit.Text
	on    string
	off   string
	acc   float64
	hz    float64
	show  bool
}

func (b *blinkTicker) Tick(dt float64) bool {
	if b == nil || b.label == nil {
		return false
	}
	if b.hz <= 0 {
		b.hz = 1.5
	}
	b.acc += dt
	period := 1.0 / b.hz
	if b.acc >= period {
		b.acc = math.Mod(b.acc, period)
		b.show = !b.show
		if b.show {
			b.label.SetValue(b.on)
		} else {
			b.label.SetValue(b.off)
		}
	}
	return true
}

// counterTicker animates a numeric label (e.g. FPS-style counter / live stats).
type counterTicker struct {
	label  *kit.Text
	prefix string
	value  float64
	rate   float64 // units per second
	wrap   float64 // wrap at this value (0 = no wrap)
}

func (c *counterTicker) Tick(dt float64) bool {
	if c == nil || c.label == nil {
		return false
	}
	c.value += dt * c.rate
	if c.wrap > 0 && c.value >= c.wrap {
		c.value = math.Mod(c.value, c.wrap)
	}
	c.label.SetValue(fmt.Sprintf("%s%.0f", c.prefix, c.value))
	return true
}

func main() {
	exboot.InitEnv()
	winW, winH := 760, 720
	// Default unlimited; set GPUI_ANIM_SECONDS>0 for timed CI smoke.
	seconds := 0.0
	if v := os.Getenv("GPUI_ANIM_SECONDS"); v != "" {
		fmt.Sscanf(v, "%f", &seconds)
		if seconds < 0 {
			seconds = 0
		}
	}

	host, err := platform.NewLinuxHost(platform.LinuxOptions{
		Width: winW, Height: winH, Title: "gpui ui_kit_m5_smoke (M5 demand + anim demos)",
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

	title := kit.NewText("M5 · demand dirty · animations (typewriter / blink / counters)")
	title.SetFace(face)
	title.Root.FontSize = 15

	// Motion fade-in text — ticker only while animating
	hello := kit.NewText("Fading content (Motion)")
	hello.SetFace(face)
	motion := primitive.NewMotion(hello.Node())
	motion.Anim.Duration = 0.8
	motion.Anim.Start()
	motionBoundary := primitive.NewRepaintBoundary(motion)

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

	// Skeleton row — each bar isolated in a RepaintBoundary
	sk1 := kit.NewSkeleton(180, 14)
	sk2 := kit.NewSkeleton(120, 14)
	sk3 := kit.NewSkeleton(160, 14)
	skRow := primitive.Column(
		primitive.NewRepaintBoundary(sk1.Node()),
		primitive.NewRepaintBoundary(sk2.Node()),
		primitive.NewRepaintBoundary(sk3.Node()),
	)
	skRow.Gap = 8

	// Spin
	spinLabel := kit.NewText("Loading…")
	spinLabel.SetFace(face)
	spin := kit.NewSpin(spinLabel.Node())
	spinBoundary := primitive.NewRepaintBoundary(spin.Node())

	// Dual progress bars (different rates)
	prog := kit.NewProgress(35)
	prog.Width = 280
	progBoundary := primitive.NewRepaintBoundary(prog.Node())
	progTick := &progressTicker{prog: prog, pct: 35, rate: 18}

	prog2 := kit.NewProgress(10)
	prog2.Width = 280
	prog2Boundary := primitive.NewRepaintBoundary(prog2.Node())
	prog2Tick := &progressTicker{prog: prog2, pct: 10, rate: 28}

	// Ring canvas (static demo ring) + animated ring via phase
	ring := primitive.ProgressRing(48, 4, 0.65,
		theme.Color(core.TokenColorFillSecondary),
		theme.Color(core.TokenColorPrimary),
	)
	animPhase := 0.35
	animRing := primitive.NewCanvas(48, 48, func(pc *core.PaintContext, sz core.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		track := theme.Color(core.TokenColorFillSecondary)
		fill := theme.Color(core.TokenColorPrimary)
		stroke := 4.0
		cx := pc.Origin.X + sz.Width/2
		cy := pc.Origin.Y + sz.Height/2
		r := 24.0 - stroke
		if r < 1 {
			r = 1
		}
		d := pc.DC
		d.SetRGBA(track.R, track.G, track.B, track.A)
		d.SetLineWidth(stroke)
		d.DrawCircle(cx, cy, r)
		_ = d.Stroke()
		start := -math.Pi / 2
		end := start + 2*math.Pi*animPhase
		steps := 48
		d.SetRGBA(fill.R, fill.G, fill.B, fill.A)
		d.SetLineWidth(stroke)
		for i := 0; i <= steps; i++ {
			a := start + (end-start)*float64(i)/float64(steps)
			x := cx + r*math.Cos(a)
			y := cy + r*math.Sin(a)
			if i == 0 {
				d.MoveTo(x, y)
			} else {
				d.LineTo(x, y)
			}
		}
		_ = d.Stroke()
	})
	animRingBoundary := primitive.NewRepaintBoundary(animRing)
	animRingTickFn := &funcTicker{fn: func(dt float64) bool {
		animPhase += dt * 0.45
		if animPhase > 1 {
			animPhase -= 1
		}
		animRing.MarkNeedsPaint()
		return true
	}}

	// --- Typewriter simulated input ---
	typeLabel := kit.NewText("Simulated typing (Input + ticker)")
	typeLabel.SetFace(face)
	typeIn := kit.NewInput("typewriter…")
	typeIn.SetFace(face)
	typeIn.SetFixedSize(320, 36)
	typeBoundary := primitive.NewRepaintBoundary(typeIn.Node())
	script := []rune("Hello, gpui! 模拟输入 typing demo…")
	typeTick := &typewriterTicker{
		input:  typeIn,
		script: script,
		cps:    14,
		hold:   1.5,
	}

	// Caret blink demo (text line with | caret)
	blinkLabel := kit.NewText("Caret blink demo")
	blinkLabel.SetFace(face)
	blinkTx := kit.NewText("Edit me|")
	blinkTx.SetFace(face)
	blinkBoundary := primitive.NewRepaintBoundary(blinkTx.Node())
	blinkTick := &blinkTicker{
		label: blinkTx,
		on:    "Edit me|",
		off:   "Edit me ",
		hz:    1.6,
		show:  true,
	}

	// Live counter
	countLabel := kit.NewText("Live counter")
	countLabel.SetFace(face)
	countTx := kit.NewText("frames: 0")
	countTx.SetFace(face)
	countBoundary := primitive.NewRepaintBoundary(countTx.Node())
	countTick := &counterTicker{
		label:  countTx,
		prefix: "frames: ",
		rate:   60, // ~visual frame counter
		wrap:   1e9,
	}

	// Tour
	tourBtn := kit.NewButton("Start Tour")
	tourBtn.SetType(kit.ButtonPrimary)
	tourBtn.SetFace(face)
	tour := kit.NewTour(
		kit.TourStep{Title: "Welcome", Body: "Demand-frame animations: tickers only MarkNeedsPaint."},
		kit.TourStep{Title: "Typewriter", Body: "Input is driven by a ticker (no real IME required)."},
	)
	tour.Face = face
	tour.Viewport = vp
	tourBtn.SetOnClick(func() {
		tour.Steps[0].Target = core.AbsoluteBounds(tourBtn.Root)
		tour.Steps[1].Target = core.AbsoluteBounds(typeIn.Node())
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

	row1 := primitive.Row(togglePres.Node(), tourBtn.Node(), a11yBtn.Node(), ring, animRingBoundary)
	row1.Gap = 12
	row1.CrossAlign = core.CrossCenter

	// Section: typewriter + blink + counter
	animSection := primitive.Column(
		typeLabel.Node(),
		typeBoundary,
		blinkLabel.Node(),
		blinkBoundary,
		countLabel.Node(),
		countBoundary,
	)
	animSection.Gap = 8
	animSection.CrossAlign = core.CrossStart

	col := primitive.Column(
		title.Node(),
		motionBoundary,
		presence,
		kit.NewText("Skeleton").Node(),
		skRow,
		kit.NewText("Spin").Node(),
		spinBoundary,
		kit.NewText("Progress (two rates)").Node(),
		progBoundary,
		prog2Boundary,
		kit.NewText("Animations").Node(),
		animSection,
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

	// Demand-frame tickers (ANIMATING)
	sk1.AttachTicker(tree)
	sk2.AttachTicker(tree)
	sk3.AttachTicker(tree)
	spin.AttachTicker(tree)
	motion.AttachTicker(tree)
	tree.AddTicker(progTick)
	tree.AddTicker(prog2Tick)
	tree.AddTicker(typeTick)
	tree.AddTicker(blinkTick)
	tree.AddTicker(countTick)
	tree.AddTicker(animRingTickFn)

	a11yBtn.SetOnClick(func() {
		nodes := kit.CollectA11y(tree.Root())
		status = fmt.Sprintf("a11y nodes=%d", len(nodes))
		log.Printf("a11y: %+v", nodes)
	})

	last := status

	res := exboot.RunUIDemand(exboot.UIDemandConfig{
		Host: host, Tree: tree, SC: sc, DC: dc, Device: device, Theme: theme,
		Clear:   theme.Color(core.TokenColorBgLayout),
		Seconds: seconds,
		// Continuous false: Flutter demand — paint only when Dirty (tickers MarkNeedsPaint).
		Continuous: false,
		Flush:      host.Flush,
		OnResize: func(w, h int) {
			winW, winH = w, h
			vp = core.Size{Width: float64(w), Height: float64(h)}
			root.Width, root.Height = float64(w), float64(h)
			tour.Viewport = vp
			root.MarkNeedsLayout()
			tree.MarkFullPaintRequired()
		},
		OnUpdate: func(dt float64) {
			_ = dt
			tour.Sync()
			if status != last {
				statusTx.SetValue("status: " + status)
				last = status
			}
		},
	})
	fmt.Printf("ui_kit_m5_smoke done frames=%d paints=%d hops=%d status=%q a11y=%d %s\n",
		res.Loops, res.Paints, res.Hops, status, len(kit.CollectA11y(tree.Root())), dc.RenderPathStats().LogLine())
}

// funcTicker adapts a function to core.Ticker.
type funcTicker struct {
	fn func(dt float64) bool
}

func (f *funcTicker) Tick(dt float64) bool {
	if f == nil || f.fn == nil {
		return false
	}
	return f.fn(dt)
}
