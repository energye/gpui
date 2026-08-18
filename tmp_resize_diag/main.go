// Command tmp_resize_diag reproduces "rendered content lags window size
// during a real mouse resize drag" (400<->1600 repeated). The window draws a
// rich desktop-GUI-like scene — title bar, sidebar list, main image + card
// grid + paragraphs + progress bar, status bar, and three animated hot spots
// (spinners / progress / HOT block) — so that "content does not follow the
// window" is plainly visible while dragging. A separate python process drives
// the real pointer drag (libXtst) and captures frames; stderr logs every
// EventResize for correlation.
package main

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
	_ "net/http/pprof"
)

const (
	topH    = 44.0 // title bar
	sideW   = 190.0
	statusH = 30.0
	gap     = 10.0
)

// diagScene is the rich desktop-GUI-like scene. Panels re-size on window
// resize; Align-anchored nodes (buttons, spinners, HOT block) follow their
// anchor automatically; cards / image / paragraphs re-size with the main area.
type diagScene struct {
	root    *rendering.AbsoluteBox
	top     *wrkit.Panel
	sidebar *wrkit.Panel
	main    *wrkit.Panel
	status  *wrkit.Panel

	spinnerTop    *rendering.RenderSpinner
	spinnerStatus *rendering.RenderSpinner
	title         *rendering.RenderText
	img           *rendering.RenderImage
	cards         []*rendering.RenderColorBox
	para          *rendering.RenderText
	barTrack      *rendering.AbsoluteBox
	barFill       *rendering.RenderColorBox
	barAlign      *rendering.RenderAlignBox
	hot           *rendering.RenderColorBox
	pctLabel      *rendering.RenderText
	statusTxt     *rendering.RenderText

	progress float64
	winW     float64
	winH     float64
}

// makeGradientChart synthesizes a small "chart/photo" bitmap (horizontal
// gradient + signal bars + diagonal) via render.ImageBuf so a real image
// texture participates in every frame.
func makeGradientChart(w, h int) *render.ImageBuf {
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		return nil
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8(40 + 90*x/w)
			g := uint8(60 + 120*y/h)
			b := uint8(180 - 100*x/w)
			// signal bars: short vertical stripes on the left half.
			if x < w/2 && (x/12)%2 == 0 {
				barH := 10 + (x*13)%(h*3/4)
				if y > h-barH {
					r, g, b = 230, 230, 60
				}
			}
			// diagonal line accent.
			if math.Abs(float64(y)-(float64(h)-float64(x)*float64(h)/float64(w))) < 1.5 {
				r, g, b = 40, 220, 220
			}
			_ = buf.SetRGBA(x, y, r, g, b, 255)
		}
	}
	return buf
}

// buildRichScene assembles the scene for a window of initial size (w0, h0).
func buildRichScene(w0, h0 float64) *diagScene {
	s := &diagScene{winW: w0, winH: h0}

	s.root = rendering.NewAbsoluteBox(w0, h0)
	s.root.Background = &rendering.Color{R: 0.06, G: 0.07, B: 0.09, A: 1}

	bh := h0 - topH - statusH - 3*gap
	mw := w0 - sideW - 3*gap

	// --- title bar: spinner + title + three Align-anchored buttons ---
	s.top = wrkit.NewPanel(w0, topH, 0.13, 0.15, 0.19, 1)
	s.top.PlaceOn(s.root, 0, 0)
	s.spinnerTop = rendering.NewRenderSpinner(22)
	s.top.Align(s.spinnerTop, 0.02, 0.5)
	s.title = wrkit.Label("gpui resize diag — rich GUI", 14, 0.88, 0.92, 0.98)
	s.top.Place(s.title, 46, 13)
	for i, ax := range []float64{0.85, 0.925, 1.0} {
		btn := rendering.NewRenderColorBox(52, 22, 0.25+0.1*float64(i), 0.45, 0.62, 1)
		btn.SetRepaintBoundary(true)
		s.top.Align(btn, ax, 0.5)
	}

	// --- sidebar: 8 icon+text list rows (icon is a RepaintBoundary) ---
	s.sidebar = wrkit.NewPanel(sideW, bh, 0.10, 0.11, 0.14, 1)
	s.sidebar.PlaceOn(s.root, gap, topH+gap)
	s.sidebar.LabelAt("FILES", 12, 12, 10, 0.55, 0.75, 0.95)
	for i := 0; i < 8; i++ {
		icon := rendering.NewRenderColorBox(28, 28, 0.15+0.06*float64(i%4), 0.35+0.06*float64((i/4)%3), 0.55, 1)
		icon.SetRepaintBoundary(true)
		s.sidebar.Place(icon, 12, 34+float64(i)*34)
		s.sidebar.LabelAt(fmt.Sprintf("item %02d.go", i+1), 12, 50, 39+float64(i)*34, 0.7, 0.78, 0.88)
	}

	// --- main area: image + card grid (follows window) + paragraph + progress ---
	s.main = wrkit.NewPanel(mw, bh, 0.09, 0.10, 0.12, 1)
	s.main.PlaceOn(s.root, sideW+2*gap, topH+gap)

	s.img = rendering.NewRenderImage(mw-28, 84)
	if buf := makeGradientChart(256, 96); buf != nil {
		s.img.SetImageShared(buf)
	} else {
		s.img.SetError()
	}
	s.main.Place(s.img, 14, 12)

	cardW := (mw - 28 - 3*6) / 4
	bh2 := bh - 104 - 42*3 - 32
	_ = bh2 // vertical space below the grid is left as empty client area
	for r := 0; r < 3; r++ {
		for c := 0; c < 4; c++ {
			card := rendering.NewRenderColorBox(cardW, 42, 0.14+0.05*float64(r), 0.30+0.06*float64(c), 0.38+0.07*float64((r+c)%3), 1)
			card.SetRepaintBoundary(true)
			s.main.Place(card, 14+float64(c)*(cardW+6), 104+float64(r)*48)
			s.cards = append(s.cards, card)
		}
	}
	s.para = wrkit.Label(
		"Resize me: panels re-size, cards stretch, text re-wraps, and the HOT block "+
			"tracks the bottom-right corner — watch whether the frame keeps up during a drag.",
		12, 0.72, 0.78, 0.88)
	s.para.MaxWidth = mw - 28
	s.main.Place(s.para, 14, 252)

	// progress bar (animated): dark track + fill anchored by alignment.
	trackW := mw - 28
	s.barTrack = rendering.NewAbsoluteBox(trackW, 12)
	s.barTrack.Background = &rendering.Color{R: 0.03, G: 0.04, B: 0.06, A: 1}
	s.main.Place(s.barTrack, 14, 282)
	s.barFill = rendering.NewRenderColorBox(24, 12, 0.1, 0.75, 0.35, 1)
	s.barFill.SetRepaintBoundary(true)
	s.barAlign = rendering.NewRenderAlignBox(s.barFill, 0, 0.5)
	s.barTrack.AddChild(s.barAlign)

	// HOT block: bottom-right anchor — the most visible "follows the window" probe.
	s.hot = rendering.NewRenderColorBox(64, 40, 0.95, 0.2, 0.2, 1)
	s.hot.SetRepaintBoundary(true)
	s.main.Align(s.hot, 0.9, 0.95)

	// --- status bar: text + spinner + progress % (follows width) ---
	s.status = wrkit.NewPanel(w0, statusH, 0.12, 0.13, 0.16, 1)
	s.status.PlaceOn(s.root, 0, h0-statusH)
	s.statusTxt = wrkit.Label("STATUS: ready", 11, 0.62, 0.68, 0.78)
	s.status.Place(s.statusTxt, 12, 8)
	s.spinnerStatus = rendering.NewRenderSpinner(18)
	s.status.Align(s.spinnerStatus, 0.94, 0.5)
	s.pctLabel = wrkit.Label("0%", 11, 0.85, 0.85, 0.5)
	s.status.Place(s.pctLabel, w0-86, 8)

	return s
}

// Resize re-lays the scene for a new window size (all panels track the
// window; anchored nodes follow automatically in the next layout pass).
func (s *diagScene) Resize(w, h float64) {
	if s == nil || s.root == nil {
		return
	}
	if w < sideW+3*gap+40 {
		w = sideW + 3*gap + 40
	}
	if h < topH+statusH+3*gap+40 {
		h = topH + statusH + 3*gap + 40
	}
	s.winW, s.winH = w, h
	bh := h - topH - statusH - 3*gap
	mw := w - sideW - 3*gap

	s.root.FixedWidth, s.root.FixedHeight = w, h
	s.root.MarkNeedsLayout()

	s.top.W, s.top.H = w, topH
	s.top.Box.FixedWidth, s.top.Box.FixedHeight = w, topH
	s.top.Box.MarkNeedsLayout()

	s.sidebar.W, s.sidebar.H = sideW, bh
	s.sidebar.Box.FixedWidth, s.sidebar.Box.FixedHeight = sideW, bh
	s.sidebar.Box.MarkNeedsLayout()

	s.main.W, s.main.H = mw, bh
	s.main.Box.FixedWidth, s.main.Box.FixedHeight = mw, bh
	s.main.Box.MarkNeedsLayout()

	s.status.W, s.status.H = w, statusH
	s.status.Box.FixedWidth, s.status.Box.FixedHeight = w, statusH
	s.status.Box.MarkNeedsLayout()
	// The status bar hugs the window bottom — its offset depends on h and must
	// be re-applied on every resize (PlaceOn re-anchors it; without this it
	// stays at the initial 400×400 build position and visibly stops following
	// the window as it grows).
	s.status.PlaceOn(s.root, 0, h-statusH)

	// Cards stretch with the main area.
	cardW := (mw - 28 - 3*6) / 4
	for i, card := range s.cards {
		if card.Width == cardW {
			continue
		}
		card.Width = cardW
		card.MarkNeedsLayout()
		r, c := i/4, i%4
		s.main.Place(card, 14+float64(c)*(cardW+6), 104+float64(r)*48)
	}

	// Image and paragraph re-size / re-wrap with the main area.
	s.img.Width = mw - 28
	s.img.MarkNeedsLayout()
	s.para.MaxWidth = mw - 28
	s.para.MarkNeedsLayout()

	trackW := mw - 28
	s.barTrack.FixedWidth = trackW
	s.barTrack.MarkNeedsLayout()

	// Status bar text pinned near the right edge (window-relative).
	s.status.Place(s.pctLabel, w-86, 8)
}

// Tick advances the always-on animation hot spots (spinners, progress bar,
// HOT block color) and the status fps readout — like a real GUI never idle.
func (s *diagScene) Tick(dt float64, fps float64) {
	if s == nil {
		return
	}
	s.spinnerTop.SetPhase(s.spinnerTop.Phase + dt*1.7)
	s.spinnerStatus.SetPhase(s.spinnerStatus.Phase + dt*2.3)

	s.progress += dt * 0.12
	if s.progress >= 1 {
		s.progress = 0
	}
	s.barAlign.SetAlignment(s.progress, 0.5)
	s.pctLabel.SetText(fmt.Sprintf("%d%%", int(s.progress*100)))
	s.pctLabel.MarkNeedsPaint()

	// HOT block cycles red -> amber -> cyan (visible repaint every frame).
	ph := s.progress * 2 * math.Pi
	s.hot.R = 0.95*math.Abs(math.Sin(ph)) + 0.05
	s.hot.G = 0.2*math.Abs(math.Sin(ph+1.2)) + 0.1
	s.hot.B = 0.95*math.Abs(math.Sin(ph+2.4)) + 0.05
	s.hot.MarkNeedsPaint()

	s.statusTxt.SetText(fmt.Sprintf("STATUS: %dx%d fps=%.0f cards=%d pic=img+text+grid", int(s.winW), int(s.winH), fps, len(s.cards)))
	s.statusTxt.MarkNeedsPaint()
}

func main() {
	secs := 60
	if v := os.Getenv("DIAG_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	fmt.Fprintf(os.Stderr, "diag: resize-drag repro (rich GUI scene), window 400x400 -> drag 400<->1600, %ds\n", secs)
	t0 := time.Now()
	w0, h0 := 400, 400
	big := os.Getenv("DIAG_FULLSCREEN") == "1"
	if big {
		// Big window (~full screen) but NOT fullscreen state: fullscreen
		// locks the resize handles on GNOME, and the point of this tool is
		// live resize-drag. Decorations keep a real WM frame to drag.
		w0, h0 = 1600, 1000
		fmt.Fprintln(os.Stderr, "diag: DIAG_FULLSCREEN=1 -> huge start size (not fullscreen state)")
	}
	win, err := platform.Open(platform.Options{
		Width: w0, Height: h0, Title: "gpui resize diag — rich GUI", Decorations: true,
		Resizable: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "diag: window:", err)
		os.Exit(1)
	}
	defer win.Close()
	ns := win.Host().NativeSurface()
	fmt.Fprintf(os.Stderr, "diag: opened backend=%s ns.Display=%x ns.Window=%x\n", win.Backend(), ns.Display, ns.Window)

	wrkit.EnsureUIFace()

	if os.Getenv("DIAG_PPROF") == "1" {
		go func() {
			if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
				fmt.Fprintf(os.Stderr, "diag: pprof: %v\n", err)
			}
		}()
		fmt.Fprintln(os.Stderr, "diag: pprof on 127.0.0.1:6060")
	}

	scene := buildRichScene(float64(w0), float64(h0))
	if os.Getenv("DIAG_LAYERS") == "1" {
		for i := 0; i < 40; i++ {
			c := rendering.NewRenderColorBox(30, 20, 0.30, 0.34, 0.30+float64(i%5)*0.05, 1)
			scene.root.AddChild(rendering.NewRenderAlignBox(c, float64(i%8)/7, float64(i/8)/4))
		}
		fmt.Fprintln(os.Stderr, "diag: DIAG_LAYERS=1 (+40 overlay blocks)")
	}

	app := embedder.NewPipelineApp(win.Host(), scene.root, embedder.PipelineOptions{
		ClearR: 0.10, ClearG: 0.12, ClearB: 0.16, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					scene.Resize(float64(ev.Width), float64(ev.Height))
				}
				fmt.Fprintf(os.Stderr, "DBG t=%5.1f EventResize %dx%d\n", time.Since(t0).Seconds(), ev.Width, ev.Height)
			case platform.EventResizeSync:
				fmt.Fprintf(os.Stderr, "DBG t=%5.1f EventResizeSync\n", time.Since(t0).Seconds())
			case platform.EventPointer:
				if os.Getenv("DIAG_TRACE") == "1" {
					fmt.Fprintf(os.Stderr, "DBG t=%5.1f Pointer %v btn=%d xy=%.0f,%.0f\n",
						time.Since(t0).Seconds(), ev.Pointer, ev.Button, ev.X, ev.Y)
				}
			case platform.EventExpose:
				if os.Getenv("DIAG_TRACE") == "1" {
					fmt.Fprintf(os.Stderr, "DBG t=%5.1f Expose\n", time.Since(t0).Seconds())
				}
			case platform.EventClose:
				fmt.Fprintln(os.Stderr, "diag: window close")
			}
		},
	})
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "diag: open:", err)
		os.Exit(1)
	}
	app.Scheduler().SetMode(scheduler.ModePersistent)

	// Always-on animation ticker (never idle, like a real GUI busy state).
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		fps := 0.0
		if snap := app.Metrics().Snapshot(); snap.AvgFrameIntervalMs > 1e-6 {
			fps = 1000.0 / snap.AvgFrameIntervalMs
		}
		scene.Tick(dt, fps)
		app.ScheduleFrame()
	}})

	// Presents-per-second curve: shows where the main thread submits frames
	// (displayed rate is far lower on llvmpipe; sampler is diagnostic only).
	if os.Getenv("DIAG_PRESENTS") == "1" {
		go func() {
			pc0 := app.PresentCount()
			last := time.Now()
			for {
				time.Sleep(1 * time.Second)
				pc := app.PresentCount()
				el := time.Since(last).Seconds()
				fmt.Fprintf(os.Stderr, "DBG t=%5.1f presents/s=%.0f (total %d)\n", time.Since(t0).Seconds(), float64(pc-pc0)/el, pc)
				pc0, last = pc, time.Now()
			}
		}()
	}

	// Heartbeat: frame-loop liveness + pipeline depth every 200ms
	// (DIAG_TRACE=1). Shows whether the main loop keeps submitting frames
	// while the window is being dragged and whether the raster thread keeps
	// up (inflight growing = raster stuck).
	if os.Getenv("DIAG_TRACE") == "1" {
		go func() {
			pc0 := app.PresentCount()
			last := time.Now()
			for {
				time.Sleep(200 * time.Millisecond)
				pc := app.PresentCount()
				el := time.Since(last).Seconds()
				w, h := win.Host().Size()
				snap := app.Metrics().Snapshot()
				fmt.Fprintf(os.Stderr, "DBG t=%5.1f heart presents/s=%.0f tot=%d host=%dx%d pipe=%d/%d lastraster=%.1fms\n",
					time.Since(t0).Seconds(), float64(pc-pc0)/el, pc, w, h, snap.PipelineDepth, snap.PipelineMax, snap.LastRasterMs)
				pc0, last = pc, time.Now()
			}
		}()
	}

	// Optional programmatic resize machinery (DIAG_STORM=1): a single SetSize
	// probe + a 400<->1600 triangle-wave resize storm with fps readout. Off by
	// default so a human can manually drag the window without interference.
	if os.Getenv("DIAG_STORM") == "1" {
		// Single programmatic resize probe: does SetSize change the window?
		go func() {
			time.Sleep(4 * time.Second)
			win.Controls().SetSize(800, 800)
			time.Sleep(500 * time.Millisecond)
			w, h := win.Host().Size()
			fmt.Fprintf(os.Stderr, "DBG probe SetSize(800): host.Size()=%dx%d\n", w, h)
		}()

		// Resize storm: continuously change the window size 400<->1600
		// (triangle wave) to approximate the event pressure of a real drag.
		ctrl := win.Controls()
		go func() {
			time.Sleep(3 * time.Second)
			stormDur := 35 * time.Second
			if d := time.Duration(secs-10) * time.Second; d < stormDur {
				stormDur = d
			}
			stormEnd := time.Now().Add(stormDur)
			dir := 1
			sz := 400
			fpsT0 := time.Now()
			fpsPC := app.PresentCount()
			for time.Now().Before(stormEnd) {
				ctrl.SetSize(sz, sz)
				sz += 40 * dir
				if sz >= 1600 {
					sz, dir = 1600, -1
				} else if sz <= 400 {
					sz, dir = 400, 1
				}
				time.Sleep(12 * time.Millisecond)
				if time.Since(fpsT0) >= 2*time.Second {
					pc := app.PresentCount()
					fps := float64(pc-fpsPC) / time.Since(fpsT0).Seconds()
					fmt.Fprintf(os.Stderr, "DBG storm fps=%.1f (2s window)\n", fps)
					fpsT0, fpsPC = time.Now(), pc
				}
			}
			fmt.Fprintln(os.Stderr, "diag: storm done")
		}()
	}

	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "diag: run:", err)
		os.Exit(1)
	}
	app.Close()
	fmt.Fprintf(os.Stderr, "diag: done presents=%d\n", app.PresentCount())
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
