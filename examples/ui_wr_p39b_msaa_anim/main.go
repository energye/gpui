// Command ui_wr_p39b_msaa_anim is the P3.9b measurement real window: a full-
// window, general-purpose animated UI scene used to compare 1x vs 4x MSAA on
// real GPU hardware. It adds NO engine capability and NO public API; it only
// drives existing render / ui/rendering public APIs.
//
// The whole window is content (no legend / no partitioned shell): a 3x3 mesh
// gradient background grid, linear/radial gradient fills, three cubic Bezier
// strokes, four rotating rounded cards with text, a rotating scaled image, a
// translucent SaveLayer disc, a 40-dot particle field, and two fixed-color
// anchor cards (pixel-probe invariants). A HUD line floats over the top-left
// of the content and is excluded from the golden comparison.
//
// Three modes:
//
//	-auto-only            frozen phase: logic probes + pixel probes + golden, gate PASS/FAIL
//	-measure              advancing phase: full repaint every frame, performance JSON
//	(no flag)             persistent manual window: real events logged + HUD title state
//
// Config (all env, read before platform.Open):
//
//	GPUI_P39B_W / GPUI_P39B_H   logical window size (default 1920x1080; 3840x2160 reserved)
//	GPUI_P39B_MSAA              1 or 4 (default 1); 4 calls SetMSAASampleCount before Open
//	GPUI_BACKEND                native | go (engine's own backend switch)
//	GPUI_P39B_BREAK             "freeze" pins the phase (negative control for phase_advanced)
//	GPUI_ACCEPT_GOLDEN          1 seeds the golden baseline (gate mode only)
//
// Usage:
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=18 go run ./examples/ui_wr_p39b_msaa_anim -measure
//	GPUI_P39B_MSAA=4 RUN_SECONDS=18 go run ./examples/ui_wr_p39b_msaa_anim -measure
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_p39b_msaa_anim -auto-only
//	go run ./examples/ui_wr_p39b_msaa_anim                 # manual, stays open
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

// ---------------------------------------------------------------- config ---

var (
	winW, winH = envInt("GPUI_P39B_W", 1920), envInt("GPUI_P39B_H", 1080)
	msaaReq    = envInt("GPUI_P39B_MSAA", 1)
	breakMode  = os.Getenv("GPUI_P39B_BREAK")
)

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func backendTag() string {
	if os.Getenv("GPUI_BACKEND") == "go" {
		return "go"
	}
	return "native"
}

const hudH = 44.0 // metric line floating over the content; excluded from golden

// ---------------------------------------------------------------- scene -----

// phase is derived from wall time (no shared mutable state between the UI and
// raster threads). Gate mode freezes it to a constant so the frame is stable.
var (
	startTime   time.Time
	phaseFrozen bool
	phaseConst  = 1.234
)

func currentPhase() float64 {
	if phaseFrozen || breakMode == "freeze" {
		return phaseConst
	}
	return time.Since(startTime).Seconds()
}

// sceneCounters are written by the paint walk and read once after Run; atomic
// keeps -race clean across the UI/raster split.
var (
	primitives   atomic.Int64
	meshTris     atomic.Int64
	phaseWall    atomic.Int64 // phase at last paint, in 1e3 units
	manualEvents atomic.Int64
	// contentW/H record the ACTUAL OnPaint content size. With window
	// decorations the compositor may hand back a surface smaller than the
	// requested width/height; probes and golden rects must use the real size,
	// otherwise they drift (the whole scene would be clipped and probe
	// coordinates mis-scaled).
	contentW atomic.Int64
	contentH atomic.Int64
)

func contentSize() (float64, float64) {
	return float64(contentW.Load()), float64(contentH.Load())
}

func hue(h float64) render.RGBA {
	h = math.Mod(h, 1)
	if h < 0 {
		h++
	}
	s, v := 0.80, 1.0
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h*6, 2)-1))
	m := v - c
	var r, g, b float64
	switch int(h*6) % 6 {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return render.RGBA{R: r + m, G: g + m, B: b + m, A: 1}
}

// demoImage is a small procedurally generated RGBA image (no external file).
func demoImage() *render.ImageBuf {
	const w, h = 96, 96
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			stripe := (x/8 + y/8) % 2
			var col color.RGBA
			if stripe == 0 {
				col = color.RGBA{235, 90, 70, 255}
			} else {
				col = color.RGBA{40, 60, 120, 255}
			}
			if (x-48)*(x-48)+(y-48)*(y-48) < 22*22 {
				col = color.RGBA{250, 220, 90, 255}
			}
			img.Set(x, y, col)
		}
	}
	return render.ImageBufFromImage(img)
}

var sceneImg = demoImage()

// meshGrid rebuilds a 3x3 MeshGradient each frame: vertex positions wobble and
// colors flow with the phase. This is the P3.9a high-level grid, drawn through
// the public DrawMeshGradient API.
func meshGrid(w, h, phase float64) *render.MeshGradient {
	const rows, cols = 3, 3
	vs := make([]render.MeshGradientVertex, (rows+1)*(cols+1))
	for r := 0; r <= rows; r++ {
		for c := 0; c <= cols; c++ {
			i := r*(cols+1) + c
			bx := float64(c) / cols * w
			by := float64(r) / rows * h
			wob := 26.0 * math.Sin(phase*0.9+float64(r)*1.3+float64(c)*0.7)
			vs[i] = render.MeshGradientVertex{
				Pos:   render.Pt(bx+wob, by+18*math.Cos(phase*0.7+float64(c)*1.1+float64(r)*0.5)),
				Color: hue(0.58 + float64(i)/float64(len(vs)) + phase*0.03),
			}
		}
	}
	g, err := render.NewMeshGradient(rows, cols, vs)
	if err != nil {
		return nil
	}
	g.ColorModel = render.MeshGradientCatmullRom
	return g
}

// drawGradient wraps DrawMeshGradientEx to capture triangle stats, mirroring
// the P3.9/P3.9a windows (live path only for per-frame stats).
func drawGradient(pc *rendering.PaintContext, g *render.MeshGradient) {
	if pc == nil || pc.DC == nil || g == nil {
		return
	}
	if dc, ok := pc.LiveContext(); ok {
		dc.Push()
		dc.Translate(pc.OriginX, pc.OriginY)
		res, err := dc.DrawMeshGradientEx(g, render.VertDrawOptions{})
		dc.Pop()
		meshTris.Add(int64(res.Triangles))
		primitives.Add(1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p39b_msaa_anim: DrawMeshGradientEx: %v\n", err)
		}
		return
	}
	pc.DC.Push()
	pc.DC.Translate(pc.OriginX, pc.OriginY)
	pc.DC.DrawMeshGradient(g)
	pc.DC.Pop()
	primitives.Add(1)
}

// paintScene draws the entire window content. All coordinates are normalized
// to w,h so 4K and 1080p share one scene definition.
func paintScene(pc *rendering.PaintContext, w, h, phase float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc := pc.DC

	// (0) dark base.
	rendering.FillRect(pc, 0, 0, w, h, 0.055, 0.065, 0.085, 1)
	primitives.Add(1)

	// (1) full-window mesh gradient grid.
	drawGradient(pc, meshGrid(w, h, phase))

	// (2) linear + radial gradient bands.
	rendering.FillLinearGradient(pc, 0, h*0.30, w, h*0.10,
		0, h*0.30, w, h*0.40, 0.10, 0.55, 0.90, 0.55, 0.95, 0.30, 0.45, 0.55)
	primitives.Add(1)
	rendering.FillRadialGradient(pc, w*0.62, h*0.50, w*0.30, h*0.34,
		w*0.77, h*0.67, 10, w*0.20, 1.0, 0.75, 0.25, 0.65, 0.20, 0.30, 0.85, 0.05)
	primitives.Add(1)

	// (3) three cubic Bezier strokes (analytic-AA path primitives).
	for k := 0; k < 3; k++ {
		p := render.NewPath()
		y := h*(0.18+float64(k)*0.09) + 22*math.Sin(phase*0.8+float64(k))
		p.MoveTo(w*0.04, y)
		p.CubicTo(w*0.30, y-160+80*math.Cos(phase+float64(k)),
			w*0.66, y+160+80*math.Sin(phase*1.1+float64(k)),
			w*0.96, y+30*math.Sin(phase+float64(k)*1.7))
		rendering.StrokePath(pc, p, 3.5, 0.95, 0.96, 1.0, 0.85)
		primitives.Add(1)
	}

	// (4) four rotating rounded cards with a circle + text.
	cardW, cardH := w*0.15, h*0.20
	for k := 0; k < 4; k++ {
		cx := w * (0.14 + 0.24*float64(k))
		cy := h * (0.62 + 0.045*math.Sin(phase*1.2+float64(k)))
		ang := 0.35 * math.Sin(phase*0.7+float64(k)*1.3)
		m := render.Translate(cx, cy).Multiply(render.Rotate(ang)).Multiply(render.Translate(-cx, -cy))
		pc.PushTransform(m)
		rendering.FillRoundRect(pc, cx-cardW/2, cy-cardH/2, cardW, cardH, 18,
			0.12, 0.16, 0.24, 0.78)
		rendering.StrokeRoundRect(pc, cx-cardW/2, cy-cardH/2, cardW, cardH, 18, 2,
			0.90, 0.93, 1.0, 0.85)
		rendering.FillCircle(pc, cx, cy-h*0.03, h*0.035, hue(phase*0.05+float64(k)/4).R,
			hue(phase*0.05+float64(k)/4).G, hue(phase*0.05+float64(k)/4).B, 0.95)
		if face := wrkit.FaceAt(h * 0.024); face != nil {
			dc.Push()
			dc.SetFont(face)
			dc.SetRGBA(0.95, 0.96, 1.0, 0.95)
			dc.DrawString(fmt.Sprintf("CARD %d", k+1), cx-cardW*0.36, cy+cardH*0.24)
			dc.Pop()
		}
		pc.PopTransform()
		primitives.Add(4)
	}

	// (5) rotating + scaled image.
	{
		cx, cy := w*0.50, h*0.42
		ang := phase * 0.45
		sz := w * 0.13 * (0.85 + 0.15*math.Sin(phase*1.4))
		dc.Push()
		dc.Translate(cx, cy)
		dc.Rotate(ang)
		dc.DrawImageEx(sceneImg, render.DrawImageOptions{
			X: -sz / 2, Y: -sz / 2, DstWidth: sz, DstHeight: sz,
			Interpolation: render.InterpBilinear, Opacity: 0.92,
		})
		dc.Pop()
		primitives.Add(1)
	}

	// (6) translucent SaveLayer disc with breathing opacity.
	{
		if pc.SaveLayer(w, h, 0.55+0.35*math.Abs(math.Sin(phase*0.9))) {
			rendering.FillCircle(pc, w*0.34, h*0.44, h*0.11, 0.95, 0.45, 0.75, 0.30)
			rendering.FillCircle(pc, w*0.70, h*0.30, h*0.07, 0.35, 0.85, 0.95, 0.30)
			primitives.Add(2)
			pc.RestoreLayer()
		}
	}

	// (7) particle field (40 small circles).
	for k := 0; k < 40; k++ {
		fk := float64(k)
		px := w * (0.5 + 0.46*math.Cos(phase*0.6+fk*0.83))
		py := h * (0.5 + 0.42*math.Sin(phase*0.5+fk*1.31))
		r := 3 + 3*math.Abs(math.Sin(phase*2+fk))
		c := hue(0.15 + fk/40 + phase*0.02)
		rendering.FillCircle(pc, px, py, r, c.R, c.G, c.B, 0.9)
	}
	primitives.Add(40)

	// (9) HUD line floating over the top-left of the content.
	rendering.FillRect(pc, 0, 0, w, hudH-4, 0.04, 0.05, 0.07, 0.72)
	if face := wrkit.FaceAt(h * 0.020); face != nil {
		dc.Push()
		dc.SetFont(face)
		dc.SetRGBA(0.90, 0.95, 1.0, 0.95)
		dc.DrawString(fmt.Sprintf("P3.9b  %dx%d  MSAA %dx  backend=%s  phase=%.2f  prim=%d  tris=%d",
			int(w), int(h), msaaReq, backendTag(), phase, primitives.Load(), meshTris.Load()),
			w*0.008, hudH*0.62)
		dc.Pop()
	}
	primitives.Add(1)

	// (8) fixed-color anchor cards, drawn LAST so nothing translucent covers
	// them (pixel-probe invariants must be exact, not alpha-blended).
	rendering.FillRect(pc, w*0.90, h*0.86, w*0.05, h*0.05, 1, 0, 1, 1) // magenta
	rendering.FillRect(pc, w*0.90, h*0.92, w*0.05, h*0.05, 0, 1, 1, 1) // cyan
	primitives.Add(2)

	phaseWall.Store(int64(phase * 1000))
}

// ---------------------------------------------------------------- probes ---

type logicResult struct {
	name string
	ok   bool
	det  string
}

// windowNotBlack samples the readback PNG on a coarse grid. A GPU path that
// submits commands but fails to resolve/present (e.g. GLES 4x MSAA) yields an
// all-black snapshot; catching it here prevents a silent "green" measurement.
func windowNotBlack(path string) (nonblack, total int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return 0, 0
	}
	b := img.Bounds()
	for y := b.Min.Y + 4; y < b.Max.Y; y += 9 {
		for x := b.Min.X + 4; x < b.Max.X; x += 9 {
			r, g, bl, _ := img.At(x, y).RGBA()
			total++
			if r>>8+g>>8+bl>>8 > 24 {
				nonblack++
			}
		}
	}
	return
}

func runLogicProbes(mode string, surfaceBytes int64, snapPath string) []logicResult {
	var out []logicResult
	add := func(name string, ok bool, det string) { out = append(out, logicResult{name, ok, det}) }
	prim, tris := primitives.Load(), meshTris.Load()
	add("scene_primitives", prim > 60, fmt.Sprintf("primitives=%d (want >60)", prim))
	add("mesh_triangles", tris > 0, fmt.Sprintf("mesh tris=%d", tris))
	add("msaa_setting", int(render.MSAASampleCount()) == msaaReq,
		fmt.Sprintf("requested=%d effective=%d", msaaReq, render.MSAASampleCount()))
	add("device_surface_bytes", surfaceBytes > 0, fmt.Sprintf("bytes=%d", surfaceBytes))
	advancing := !phaseFrozen && breakMode != "freeze" && mode == "measure"
	add("phase_advanced", (mode != "measure" || advancing) && (mode != "measure" || phaseWall.Load() > 0),
		fmt.Sprintf("mode=%s advancing=%v lastPhase(x1000)=%d freeze=%v break=%q",
			mode, advancing, phaseWall.Load(), phaseFrozen, breakMode))
	nb, tot := windowNotBlack(snapPath)
	add("window_not_black", tot > 0 && nb*10 > tot,
		fmt.Sprintf("nonblack=%d/%d (want >10%%)", nb, tot))
	return out
}

func deviceSurfaceBytes() int64 {
	var total int64
	for _, p := range render.GPUPoolStats() {
		if p.Name == "device-surface" || p.Name == "surface-pool" {
			total += int64(p.Bytes)
		}
	}
	return total
}

// ---------------------------------------------------------------- main ------

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze phase, snapshot + probes + golden")
	measure := flag.Bool("measure", false, "measurement mode: advancing phase, full repaint, performance JSON")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until closed)")
	flag.Parse()

	// MSAA must be set before the first window / GPU accelerator is created.
	if msaaReq == 4 {
		render.SetMSAASampleCount(render.MSAASampleCount4)
	} else {
		render.SetMSAASampleCount(render.MSAASampleCount1)
	}

	mode := "manual"
	if *autoOnly {
		mode = "gate"
	} else if *measure {
		mode = "measure"
	}

	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		switch {
		case *autoOnly:
			secs = 6
		case *measure:
			secs = 18
		case *manualSeconds > 0:
			secs = int(*manualSeconds)
		default:
			secs = 0 // persistent manual window: RunFor = 0
		}
	} else if secs < 5 && (*autoOnly || *measure) {
		wrkit.RequireMinRun(secs, "P39b-msaa-anim")
	}
	wrkit.EnsureUIFace()

	// Gate mode freezes the phase; measure/manual advance it.
	phaseFrozen = *autoOnly
	startTime = time.Now()

	title := fmt.Sprintf("gpui ui_wr_p39b_msaa_anim — %dx%d MSAA %dx [%s]", winW, winH, msaaReq, mode)
	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: title,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = float64(winW), float64(winH)
	root.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		w, h := sz.Width, sz.Height
		if w <= 0 || h <= 0 {
			w, h = float64(winW), float64(winH)
		}
		contentW.Store(int64(w + 0.5))
		contentH.Store(int64(h + 0.5))
		paintScene(pc, w, h, currentPhase())
	}

	snapDir := filepath.Join("examples", "ui_wr_p39b_msaa_anim", "testdata")
	_ = os.MkdirAll(filepath.Join(snapDir, "goldens"), 0o755)
	snapPath := filepath.Join(snapDir, "p39b_anim_final.png")
	basePath := filepath.Join(snapDir, "goldens",
		fmt.Sprintf("p39b_anim_%s_msaa%d.png", backendTag(), msaaReq))

	var proc scheduler.ProcessTracker
	proc.Start()

	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: 0.055, ClearG: 0.065, ClearB: 0.085, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			// Only real input/window events are logged. Per-frame events
			// (frame-presented / wake / expose) must NOT hit stdout or SetTitle:
			// doing so would dominate the measurement we are here to take.
			if !notableEvent(ev.Type) {
				return
			}
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "ui_wr_p39b_msaa_anim: close event (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle("gpui ui_wr_p39b_msaa_anim — closing")
				}
			default:
				n := manualEvents.Add(1)
				fmt.Fprintf(os.Stderr, "ui_wr_p39b_msaa_anim: event #%d type=%v\n", n, ev.Type)
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("gpui ui_wr_p39b_msaa_anim — event #%d %v", n, ev.Type))
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.Scheduler().Tickers().Add(&tickFunc{on: func(dt float64) {
		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())
	elapsed := time.Since(t0).Seconds()

	surfaceBytes := deviceSurfaceBytes()
	logic := runLogicProbes(mode, surfaceBytes, snapPath)
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p39b_msaa_anim: logic %-24s ok=%v %s\n", l.name, l.ok, l.det)
		if l.ok {
			logicOK++
		}
	}

	// Pixel probes + golden only in gate mode (frozen phase = stable frame).
	// Use the ACTUAL content size (decorations shrink the surface below the
	// requested size); CheckPixelProbes and GoldenGateW derive the device scale
	// from this logical width, so probes and rects must be in that same space.
	var pixelErr, goldenErr error
	goldenDiff, goldenPx, seeded := 0.0, int64(0), false
	cw, ch := contentSize()
	if *autoOnly {
		anchors := []wrkit.PixelProbe{
			{Name: "anchor_magenta", X: cw * 0.925, Y: ch * 0.885, R: 1, G: 0, B: 1, Tol: 6},
			{Name: "anchor_cyan", X: cw * 0.925, Y: ch * 0.945, R: 0, G: 1, B: 1, Tol: 6},
		}
		for _, pr := range anchors {
			if err := wrkit.CheckPixelProbes(snapPath, cw, []wrkit.PixelProbe{pr}); err != nil {
				fmt.Fprintf(os.Stderr, "ui_wr_p39b_msaa_anim: pixel %-18s FAIL %v\n", pr.Name, err)
				if pixelErr == nil {
					pixelErr = err
				}
			} else {
				fmt.Fprintf(os.Stderr, "ui_wr_p39b_msaa_anim: pixel %-18s ok\n", pr.Name)
			}
		}
		rects := []wrkit.GoldenRect{{X: 0, Y: hudH, W: cw, H: ch - hudH}}
		goldenDiff, goldenPx, seeded, goldenErr = wrkit.GoldenGateW(snapPath, basePath, cw, rects)
	}

	nb, tot := windowNotBlack(snapPath)
	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"mode": mode, "phase_frozen": phaseFrozen, "break_mode": breakMode,
		"window_nonblack": nb, "window_samples": tot,
		"resolution_w": winW, "resolution_h": winH,
		"content_w": int64(cw), "content_h": int64(ch),
		"msaa_requested": msaaReq, "msaa_setting": int(render.MSAASampleCount()),
		"device_surface_bytes": surfaceBytes,
		"backend_env":          backendTag(),
		"scene_primitives":     primitives.Load(), "mesh_triangles": meshTris.Load(),
		"phase_last_x1000": phaseWall.Load(), "manual_events": manualEvents.Load(),
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal, "logic_failed": logicTotal - logicOK,
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
		"apis":                   "MeshGradient/DrawMeshGradient, DrawImageEx, DrawString, rendering.Fill{Linear,Radial}Gradient/StrokePath/FillRoundRect/FillCircle, SaveLayer, PushTransform",
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P39b-msaa-anim", Scenario: "ui_wr_p39b_msaa_anim", Backend: win.Backend().String(),
		Snap: app.Metrics().Snapshot(), PresentCount: app.PresentCount(), ElapsedSec: elapsed,
		SurfaceAreaPx: int64(winW) * int64(winH), Warmup: true, Extra: extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if *autoOnly || *measure {
			os.Exit(1)
		}
	}
	if logicOK != logicTotal {
		fail("logic probes %d/%d ok", logicOK, logicTotal)
	}
	if *autoOnly {
		if pixelErr != nil {
			fail("pixel probes: %v", pixelErr)
		}
		if goldenErr != nil {
			fail("golden: %v", goldenErr)
		}
		if !seeded && goldenDiff != 0 {
			fail("golden diff %.4f%% want 0 over %d px", goldenDiff, goldenPx)
		}
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_p39b_msaa_anim: OK mode=%s msaa=%dx backend=%s res=%dx%d presents=%d logic=%d/%d golden=%.4f%% seeded=%v surface=%dB elapsed=%.1fs\n",
		mode, msaaReq, backendTag(), winW, winH, app.PresentCount(), logicOK, logicTotal,
		goldenDiff, seeded, surfaceBytes, elapsed)
}

// notableEvent filters out the per-frame scheduler/present chatter so the
// measurement is of rendering, not of logging and SetTitle syscalls.
func notableEvent(t platform.EventType) bool {
	switch t {
	case platform.EventCloseRequested, platform.EventClose, platform.EventResize,
		platform.EventResizeSync, platform.EventMove, platform.EventScale,
		platform.EventStateChanged, platform.EventFocus, platform.EventKey,
		platform.EventPointer, platform.EventTouch, platform.EventIME,
		platform.EventDrop, platform.EventHidden, platform.EventOccluded:
		return true
	}
	return false
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
