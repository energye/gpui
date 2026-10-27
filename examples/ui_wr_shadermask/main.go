// Command ui_wr_shadermask is the real window for the user-space ShaderMask
// capability (render.DrawShaderMask / PaintContext.DrawShaderMask): content is
// drawn into an isolated layer, then modulated by the alpha of a mask brush —
// Flutter ShaderMaskLayer / Skia saveLayer(bounds) + drawRect(mask, dstIn).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_shadermask -auto-only
//	go run ./examples/ui_wr_shadermask                         # manual: until close
//	go run ./examples/ui_wr_shadermask -manual-seconds 15      # manual: auto-close
//
// Full-window content: two columns of four cells. The left column paints the
// mask in the retained UI walk (PaintContext.DrawShaderMask); the right column
// replays a Recorder picture built with render.DrawShaderMask. Both columns must
// land on the same pixels (pair probes), the analytic mask oracle (pixel
// probes), a bitwise golden, and a live screen grab must agree.
//
// Each cell: base fill, then a masked layer with kind-specific content:
//
//	fade     vertical gradient white 1→0; image + text fade out, a blue bar in
//	         the zero-alpha tail is erased
//	vignette radial white 1→0; the orange panel keeps its centre only
//	solid    constant white alpha=0.5; opaque stripes come out at half alpha
//	overlap  two translucent blocks composite INSIDE the layer first, then the
//	         whole composite is masked (proves compose-then-mask)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800

	cellW, cellH = 285.0, 361.0
	cellGap      = 12.0
	colPitch     = cellW + cellGap
	rowPitch     = cellH + cellGap
	firstColX    = 12.0
	firstRowY    = 60.0

	// hudH is the floating metrics strip at the top-left; the golden skips it
	// because the live counters change every frame.
	hudH = 40.0

	windowTitle = "gpui ui_wr_shadermask — user-space ShaderMask (retained ≡ direct)"

	// The grab and the snapshot are the same render path, so a correct window
	// differs by compositor rounding only. A black window or an unmasked cell
	// differs far above 1% of the compared pixels.
	screenTol        = 2
	screenMaxDiffPct = 1.0
)

var (
	rootBG = [3]float64{0.09, 0.10, 0.12}
	cellBG = [3]float64{0.16, 0.17, 0.20}

	imgGreen    = [3]float64{0.20, 0.749, 0.349} // 51,191,89 / 255
	orange      = [3]float64{0.90, 0.45, 0.10}
	stripeRed   = [3]float64{0.85, 0.30, 0.25}
	stripeGreen = [3]float64{0.25, 0.80, 0.40}
	stripeBlue  = [3]float64{0.30, 0.45, 0.95}
	blockA      = [3]float64{0.90, 0.20, 0.20}
	blockB      = [3]float64{0.20, 0.40, 0.90}
	blueBar     = [3]float64{0.20, 0.45, 0.95}
)

type cellKind int

const (
	kindFade cellKind = iota
	kindVignette
	kindSolid
	kindOverlap
)

func (k cellKind) name() string {
	switch k {
	case kindFade:
		return "FADE"
	case kindVignette:
		return "VIGNETTE"
	case kindSolid:
		return "SOLID 0.5"
	case kindOverlap:
		return "OVERLAP"
	}
	return "?"
}

type cellPos struct {
	kind     cellKind
	retained bool
	x, y     float64
}

// cells lays out the two columns: left = retained UI walk, right = direct
// picture replay, one row per mask kind in each column pair.
func cells() []cellPos {
	return []cellPos{
		{kindFade, true, firstColX, firstRowY},
		{kindFade, false, firstColX + colPitch, firstRowY},
		{kindVignette, true, firstColX + 2*colPitch, firstRowY},
		{kindVignette, false, firstColX + 3*colPitch, firstRowY},
		{kindSolid, true, firstColX, firstRowY + rowPitch},
		{kindSolid, false, firstColX + colPitch, firstRowY + rowPitch},
		{kindOverlap, true, firstColX + 2*colPitch, firstRowY + rowPitch},
		{kindOverlap, false, firstColX + 3*colPitch, firstRowY + rowPitch},
	}
}

// cellMask builds the mask brush in the coordinate space of the canvas that
// will draw the cell (window-absolute for the retained walk, cell-local for the
// recorder picture).
func cellMask(kind cellKind, x, y, w, h float64) render.Brush {
	white := render.RGBA{R: 1, G: 1, B: 1, A: 1}
	clear := render.RGBA{R: 1, G: 1, B: 1, A: 0}
	switch kind {
	case kindFade:
		return render.NewLinearGradientBrush(x, y+8, x, y+h-8).
			AddColorStop(0, white).AddColorStop(1, clear)
	case kindVignette:
		return render.NewRadialGradientBrush(x+w/2, y+h/2, 0, 0.52*math.Min(w, h)).
			AddColorStop(0, white).AddColorStop(1, clear)
	default:
		return render.SolidBrush{Color: render.RGBA{R: 1, G: 1, B: 1, A: 0.5}}
	}
}

func setRGBA(c render.Canvas, col [3]float64, a float64) {
	c.SetRGBA(col[0], col[1], col[2], a)
}

// maskDraw is the one line that differs between the two columns: the retained
// walk goes through PaintContext.DrawShaderMask (budget + stats), the direct
// column through render.DrawShaderMask on its recording canvas.
type maskDraw func(x, y, w, h float64, mask render.Brush, draw func())

// paintCell draws one cell at (x,y) in the canvas' user space. The retained
// walk passes pc.Abs(0,0) because its DC is not translated per box; the direct
// column records cell-local (0,0) and Playback runs under a Translate to the
// cell origin, so its gradient brushes follow the replay CTM.
func paintCell(c render.Canvas, kind cellKind, x, y, w, h float64, img *render.ImageBuf, face text.Face, mask maskDraw) {
	if c == nil {
		return
	}
	c.SetFillBrush(render.SolidBrush{Color: render.RGBA{R: cellBG[0], G: cellBG[1], B: cellBG[2], A: 1}})
	c.DrawRectangle(x, y, w, h)
	_ = c.Fill()

	// The kind label sits on the cell base, outside the masked layer, so it
	// stays readable no matter how the mask fades the content.
	if face != nil {
		c.SetFont(face)
	}
	c.SetRGBA(0.55, 0.62, 0.72, 1)
	c.DrawString(kind.name(), x+12, y+16)

	mask(x, y, w, h, cellMask(kind, x, y, w, h), func() {
		switch kind {
		case kindFade:
			c.DrawImage(img, x+20, y+24)
			if face != nil {
				c.SetFont(face)
				c.SetRGBA(0.95, 0.97, 1, 1)
				c.DrawString("SHADER MASK", x+20, y+152)
				c.SetRGBA(0.75, 0.85, 0.95, 1)
				c.DrawString("retained = direct", x+20, y+176)
			}
			// Blue bar inside the gradient's zero-alpha tail: it must be erased.
			setRGBA(c, blueBar, 1)
			c.DrawRectangle(x+20, y+h-30, w-40, 16)
			_ = c.Fill()
		case kindVignette:
			setRGBA(c, orange, 1)
			c.DrawRectangle(x+w*0.15, y+h*0.15, w*0.7, h*0.7)
			_ = c.Fill()
			if face != nil {
				c.SetFont(face)
				c.SetRGBA(1, 1, 1, 0.9)
				c.DrawString("VIGNETTE", x+20, y+h/2+60)
			}
		case kindSolid:
			stripes := []struct {
				col [3]float64
				off float64
			}{
				{stripeRed, 50},
				{stripeGreen, 120},
				{stripeBlue, 190},
			}
			for _, s := range stripes {
				setRGBA(c, s.col, 1)
				c.DrawRectangle(x+20, y+s.off, w-40, 46)
				_ = c.Fill()
			}
		case kindOverlap:
			setRGBA(c, blockA, 1)
			c.DrawRectangle(x+20, y+130, 150, 100)
			_ = c.Fill()
			setRGBA(c, blockB, 0.5)
			c.DrawRectangle(x+100, y+160, 150, 100)
			_ = c.Fill()
		}
	})
}

func mix(c, bg [3]float64, a float64) [3]float64 {
	return [3]float64{
		a*c[0] + (1-a)*bg[0],
		a*c[1] + (1-a)*bg[1],
		a*c[2] + (1-a)*bg[2],
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// fadeAlpha is the vertical mask ramp: 1 at y+8, 0 at y+h-8.
func fadeAlpha(ly float64) float64 {
	return clamp01(1 - (ly-8)/(cellH-16))
}

// vigAlpha is the radial mask ramp: 1 at the centre, 0 at 0.52*min(w,h).
func vigAlpha(r float64) float64 {
	return clamp01(1 - r/(0.52*math.Min(cellW, cellH)))
}

type probePoint struct {
	key  string
	kind cellKind
	lx   float64
	ly   float64
	want [3]float64
	tol  float64
}

// probePoints are the analytic oracles on the snapshot. Every point is inside
// a flat fill, away from anti-aliased edges and text.
func probePoints() []probePoint {
	// B over A inside the layer (SourceOver), then the whole layer masked 0.5.
	aOnly := mix(blockA, cellBG, 0.5)
	overlap := mix(mix(blockB, blockA, 0.5), cellBG, 0.5)
	return []probePoint{
		{key: "fade_img", kind: kindFade, lx: 40, ly: 35, want: mix(imgGreen, cellBG, fadeAlpha(35)), tol: 0.04},
		{key: "fade_tail", kind: kindFade, lx: cellW / 2, ly: cellH - 16, want: mix(blueBar, cellBG, fadeAlpha(cellH-16)), tol: 0.03},
		{key: "vig_center", kind: kindVignette, lx: cellW / 2, ly: cellH / 2, want: orange, tol: 0.03},
		{key: "vig_corner", kind: kindVignette, lx: 8, ly: 8, want: cellBG, tol: 0.03},
		{key: "vig_mid", kind: kindVignette, lx: cellW/2 + 74, ly: cellH / 2, want: mix(orange, cellBG, vigAlpha(74)), tol: 0.03},
		{key: "solid_green", kind: kindSolid, lx: cellW / 2, ly: 143, want: mix(stripeGreen, cellBG, 0.5), tol: 0.03},
		{key: "overlap", kind: kindOverlap, lx: 135, ly: 180, want: overlap, tol: 0.03},
		{key: "block_a", kind: kindOverlap, lx: 50, ly: 180, want: aOnly, tol: 0.03},
		{key: "exterior", kind: kindOverlap, lx: 266, ly: 180, want: cellBG, tol: 0.03},
	}
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	screenCheck := flag.Bool("screencheck", true, "manual mode: grab the live window and compare it with the snapshot (gate mode always runs it)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "A-shadermask")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	face, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	img, err := render.NewImageBuf(200, 90, render.FormatRGBA8)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: image buf:", err)
		os.Exit(1)
	}
	defer img.Dispose()
	for y := 0; y < 90; y++ {
		for x := 0; x < 200; x++ {
			_ = img.SetRGBA(x, y, 51, 191, 89, 255)
		}
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: windowTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: rootBG[0], G: rootBG[1], B: rootBG[2], A: 1}

	var app *embedder.PipelineApp
	var hudLine1, hudLine2 string

	header := rendering.NewRenderBox()
	header.FixedWidth, header.FixedHeight = winW, firstRowY
	header.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		c := pc.DC
		if face != nil {
			c.SetFont(face)
		}
		c.SetRGBA(0.85, 0.90, 0.97, 1)
		c.DrawString(hudLine1, 10, 16)
		c.SetRGBA(0.25, 0.85, 0.45, 1)
		c.DrawString(hudLine2, 10, 34)
		c.SetRGBA(0.65, 0.72, 0.85, 1)
		c.DrawString("RETAINED — PaintContext.DrawShaderMask（录制回放）", firstColX, 54)
		c.DrawString("DIRECT — Recorder + DrawShaderMask（Picture 回放）", firstColX+2*colPitch, 54)
	}
	root.Place(header, 0, 0)

	// Direct column: one cell-local picture per cell, recorded once and replayed
	// each frame under Translate(cell origin). Playback composes onto the
	// canvas' CTM, so mask gradients recorded in cell-local space land on the
	// cell — the same placement the retained walk gets from pc.Abs(0,0).
	pictures := make([]*recording.Recording, len(cells()))
	for i, cell := range cells() {
		if cell.retained {
			continue
		}
		rec := recording.NewRecorder(int(cellW), int(cellH))
		paintCell(rec, cell.kind, 0, 0, cellW, cellH, img, face,
			func(x, y, w, h float64, m render.Brush, draw func()) {
				render.DrawShaderMask(rec, x, y, w, h, m, draw)
			})
		pictures[i] = rec.FinishRecording()
	}

	for i := range cells() {
		cell := cells()[i]
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cellW, cellH
		if cell.retained {
			kind := cell.kind
			box.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
				if pc == nil || pc.DC == nil {
					return
				}
				ax, ay := pc.Abs(0, 0)
				paintCell(pc.DC, kind, ax, ay, cellW, cellH, img, face,
					func(x, y, w, h float64, m render.Brush, draw func()) {
						pc.DrawShaderMask(x, y, w, h, m, draw)
					})
			}
		} else {
			pic := pictures[i]
			box.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
				if pc == nil || pc.DC == nil || pic == nil {
					return
				}
				ax, ay := pc.Abs(0, 0)
				pc.DC.Push()
				pc.DC.Translate(ax, ay)
				_ = pic.Playback(recording.NewCanvasBackend(pc.DC))
				pc.DC.Pop()
			}
		}
		root.Place(box, cell.x, cell.y)
	}

	snapDir := filepath.Join("examples", "ui_wr_shadermask", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_shadermask.png")

	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}
	app = embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: rootBG[0], ClearG: rootBG[1], ClearB: rootBG[2], ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_shadermask: close (%s)\n", win.Backend())
			case platform.EventResize:
				fmt.Fprintf(os.Stderr, "ui_wr_shadermask: resize %dx%d (fixed layout)\n", ev.Width, ev.Height)
			default:
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("%s — event type=%v", windowTitle, ev.Type))
				}
				fmt.Fprintf(os.Stderr, "ui_wr_shadermask: event type=%v\n", ev.Type)
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		header.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		s := app.Metrics().Snapshot()
		fps := 0.0
		if s.AvgFrameIntervalMs > 1e-6 {
			fps = 1000.0 / s.AvgFrameIntervalMs
		}
		allow, reject := app.SaveLayerStats()
		hudLine1 = fmt.Sprintf("A-shadermask %s fps=%.0f p95=%.1f layers=%d/%d", phase, fps, s.P95FrameIntervalMs, allow, reject)
		pol := s.PresentPolicy
		if pol == "" {
			pol = scheduler.PresentPolicyRetained
		}
		hudLine2 = fmt.Sprintf("%s mode=%s paint=%d n=%d gpu=%d cpu_fb=%d", pol, s.PresentMode, s.PaintCount, app.PresentCount(), s.GPUOps, s.CPUFallbackOps)
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	rendering.ResetLiveOnlySkips()
	recording.ResetMaskSkips()
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}

	// The snapshot is a synthetic repaint written at Close; grab the live client
	// area now so a correct snapshot over a black window still fails the gate.
	var screenGrabPath string
	var screenGrabErr error
	if gateMode || *screenCheck {
		screenGrabPath = filepath.Join(os.TempDir(), "gpui_shadermask_screen.png")
		if _, _, err := grabClientArea(windowTitle, screenGrabPath); err != nil {
			screenGrabErr = err
			screenGrabPath = ""
		}
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	allowOps, rejectOps := app.SaveLayerStats()

	origins := make(map[cellKind][2][2]float64, 4)
	for _, cell := range cells() {
		idx := 0
		if !cell.retained {
			idx = 1
		}
		v := origins[cell.kind]
		v[idx] = [2]float64{cell.x, cell.y}
		origins[cell.kind] = v
	}

	imgSnap := loadImage(snapPath)
	dpr := 1.0
	if imgSnap != nil {
		dpr = float64(imgSnap.Bounds().Dx()) / winW
	}
	points := probePoints()
	pixelResult := make(map[string]bool, len(points)*2)
	pixelOK, pixelTotal := 0, 0
	for _, p := range points {
		for idx, side := range []string{"L", "R"} {
			ox, oy := origins[p.kind][idx][0], origins[p.kind][idx][1]
			px, py := int((ox+p.lx)*dpr), int((oy+p.ly)*dpr)
			r, g, b, valid := readPixel(imgSnap, px, py)
			ok := valid && nearC(r, g, b, p.want[0], p.want[1], p.want[2], p.tol)
			key := side + "_" + p.key
			fmt.Fprintf(os.Stderr, "ui_wr_shadermask: pixel %-16s @(%4d,%4d) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				key, px, py, r, g, b, p.want[0], p.want[1], p.want[2], ok)
			pixelResult[key] = ok
			pixelTotal++
			if ok {
				pixelOK++
			}
		}
	}

	pairOK, pairTotal := 0, 0
	for _, p := range points {
		lx := origins[p.kind][0][0] + p.lx
		ly := origins[p.kind][0][1] + p.ly
		rx := origins[p.kind][1][0] + p.lx
		ry := origins[p.kind][1][1] + p.ly
		ar, ag, ab, av := readPixel(imgSnap, int(lx*dpr), int(ly*dpr))
		br, bg, bb, bv := readPixel(imgSnap, int(rx*dpr), int(ry*dpr))
		ok := av && bv && nearC(ar, ag, ab, br, bg, bb, 0.02)
		fmt.Fprintf(os.Stderr, "ui_wr_shadermask: pair  %-16s retained=(%.3f,%.3f,%.3f) direct=(%.3f,%.3f,%.3f) ok=%v\n",
			p.key, ar, ag, ab, br, bg, bb, ok)
		pairTotal++
		if ok {
			pairOK++
		}
	}

	goldenRects := []rect{{x: 0, y: hudH, w: winW, h: winH - hudH}}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	screenRes := screenCheckResult{}
	if gateMode || *screenCheck {
		screenRes.Attempted = true
		screenRes.GrabPath = screenGrabPath
		if screenGrabErr != nil {
			screenRes.Detail = "grab failed: " + screenGrabErr.Error()
		} else if pct, n, cmpErr := compareScreenToSnapshot(screenGrabPath, snapPath, goldenRects, screenTol); cmpErr != nil {
			screenRes.Detail = cmpErr.Error()
		} else if pct > screenMaxDiffPct {
			screenRes.DiffPct = pct
			screenRes.Compared = n
			screenRes.Detail = fmt.Sprintf("diff=%.4f%% over %d px > %.2f%% (grab vs snapshot)", pct, n, screenMaxDiffPct)
		} else {
			screenRes.OK = true
			screenRes.DiffPct = pct
			screenRes.Compared = n
			screenRes.Detail = fmt.Sprintf("diff=%.4f%% over %d px tol=±%d", pct, n, screenTol)
		}
		fmt.Fprintf(os.Stderr, "ui_wr_shadermask: screen check %s (%s)\n", screenRes.Detail, screenRes.GrabPath)
	}

	type logicProbe struct {
		key  string
		pass bool
	}
	logic := []logicProbe{
		{"presents_positive", app.PresentCount() >= 1},
		{"paint_count_positive", snap.PaintCount > 0},
		{"gpu_ops_positive", snap.GPUOps > 0},
		{"cpu_fallback_zero", snap.CPUFallbackOps == 0},
		{"mask_skips_zero", recording.MaskSkips() == 0},
		{"live_only_skips_zero", rendering.LiveOnlySkips() == 0},
		{"savelayer_allow_positive", allowOps > 0},
		{"savelayer_reject_zero", rejectOps == 0},
		{"pixel_probes_all", pixelTotal > 0 && pixelOK == pixelTotal},
		{"pairs_all", pairTotal > 0 && pairOK == pairTotal},
		{"golden_zero", goldenFirstRun || goldenDiffPct == 0},
	}
	logicOK := 0
	bad := make([]string, 0, len(logic))
	for _, p := range logic {
		if p.pass {
			logicOK++
		} else {
			bad = append(bad, p.key)
		}
		fmt.Fprintf(os.Stderr, "ui_wr_shadermask: logic %-24s pass=%v\n", p.key, p.pass)
	}
	logicPass := len(bad) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "A-shadermask",
		Scenario:      "ui_wr_shadermask",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"fps_gate":               "off_correctness",
			"slope_gate":             "off",
			"apis":                   "render.DrawShaderMask,PaintContext.DrawShaderMask,Canvas.SetBlendMode",
			"mask_gpu_route":         fmt.Sprintf("gpu_ops=%d cpu_fallback_ops=%d last=%q", snap.GPUOps, snap.CPUFallbackOps, snap.LastCPUFallbackReason),
			"logic_probes_ok":        logicOK,
			"logic_probes_total":     len(logic),
			"logic_bad":              bad,
			"pixel_probes_ok":        pixelOK,
			"pixel_probes_total":     pixelTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"retained_direct_pairs":  fmt.Sprintf("%d/%d", pairOK, pairTotal),
			"screen_check_ok":        screenRes.OK,
			"screen_check_diff_pct":  screenRes.DiffPct,
			"screen_check_px":        screenRes.Compared,
			"screen_check_detail":    screenRes.Detail,
			"savelayer_allow":        allowOps,
			"savelayer_reject":       rejectOps,
			"mask_skips":             recording.MaskSkips(),
			"live_only_skips":        rendering.LiveOnlySkips(),
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if !logicPass {
		fail("logic probes %d/%d bad=%v", logicOK, len(logic), bad)
	}
	if pixelTotal == 0 || pixelOK != pixelTotal {
		fail("pixel probes %d/%d ok", pixelOK, pixelTotal)
	}
	if pairTotal == 0 || pairOK != pairTotal {
		fail("retained/direct pairs %d/%d ok", pairOK, pairTotal)
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
	}
	if screenRes.Attempted && !screenRes.OK {
		fail("screen check: %s", screenRes.Detail)
	}
	if skips := rendering.LiveOnlySkips(); skips != 0 {
		fail("live_only_skips=%d: a masked cell never reached a recordable target", skips)
	}
	if skips := recording.MaskSkips(); skips != 0 {
		fail("mask_skips=%d: playback dropped a mask command", skips)
	}
	if snap.GPUOps == 0 {
		fail("gpu_ops=0: mask draws did not ride the GPU")
	}
	if snap.CPUFallbackOps != 0 {
		fail("cpu_fallback_ops=%d last=%q", snap.CPUFallbackOps, snap.LastCPUFallbackReason)
	}
	if allowOps == 0 {
		fail("savelayer_allow=0: retired mask entry never pushed an isolated layer")
	}
	if rejectOps != 0 {
		fail("savelayer_reject=%d", rejectOps)
	}
	if gateMode {
		if errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
		}); errG != nil {
			fail("%v", errG)
		}
		if snap.VSyncSource == "" {
			fail("vsync_source missing")
		}
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_shadermask: OK presents=%d logic=%d/%d pixel=%d/%d pairs=%d/%d golden=%.4f%% layers=%d/%d gpu=%d cpu_fb=%d screen=%v mode=%s elapsed=%.1fs\n",
		app.PresentCount(), logicOK, len(logic), pixelOK, pixelTotal, pairOK, pairTotal,
		goldenDiffPct, allowOps, rejectOps, snap.GPUOps, snap.CPUFallbackOps, screenRes.OK,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
