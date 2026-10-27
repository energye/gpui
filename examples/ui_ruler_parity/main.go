// Command ui_ruler_parity is the gpui side of the P3 comparison rulers.
//
// It deliberately mirrors the reference scenes instead of reusing the
// production capability windows: same client size, same content geometry,
// same animation/scroll phases, so Flutter/Godot numbers have a same-shape
// arm on the gpui side.  See docs/RENDER_BASE_RESOURCE_MATRIX.md §5.5 and
// README.md for the remaining cross-engine differences (font rasterizer,
// Flutter's ListView vs gpui's VirtualList, Godot's PhysicsServer).
//
//	export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RULER_SCENE=list RUN_SECONDS=60 go run ./examples/ui_ruler_parity
//	RULER_SCENE=list RUN_SECONDS=5 go run ./examples/ui_ruler_parity -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RULER_SCENE=list RUN_SECONDS=5 go run ./examples/ui_ruler_parity -auto-only
//	go run ./examples/ui_ruler_parity                 # manual, stays until close
//
// Scenes: spin / list / text (Flutter ruler) and godot (Godot bullet_shower).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	sceneSpin  = "flutter_spin"
	sceneList  = "flutter_list"
	sceneText  = "flutter_text"
	sceneGodot = "godot_bullets"
)

// sceneSpec is everything main needs from a parity scene. The scene owns its
// render tree; main owns the window, process accounting, and evidence.
type sceneSpec struct {
	name  string
	ref   string
	winW  int
	winH  int
	title string
	root  *rendering.AbsoluteBox

	clearR, clearG, clearB float64
	fullPaint              bool

	// tick advances a live scene. nil means the scene is static after the
	// first frame (the Flutter text arm).
	tick func(dt float64) bool
	// onEvent is optional input handling (Godot player follows the pointer).
	onEvent func(platform.Event)

	goldenRects []wrkit.GoldenRect
	probes      []wrkit.PixelProbe
	// extraCheck runs after the snapshot decodes; used by spin's glyph-ink
	// probe because a point probe cannot describe a rotating outline.
	extraCheck func(snapshot string) error
	cleanup    func()
}

type tickFunc struct{ f func(dt float64) bool }

func (t *tickFunc) Tick(dt float64) bool { return t.f(dt) }

func normalizedScene(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "spin", "flutter_spin", "flutter-spin":
		return sceneSpin, nil
	case "list", "flutter_list", "flutter-list":
		return sceneList, nil
	case "text", "flutter_text", "flutter-text":
		return sceneText, nil
	case "godot", "godot_bullets", "godot-bullets":
		return sceneGodot, nil
	default:
		return "", fmt.Errorf("unknown RULER_SCENE %q (want spin|list|text|godot)", s)
	}
}

func main() {
	sceneFlag := flag.String("scene", envDefault("RULER_SCENE", "spin"), "spin|list|text|godot")
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze, snapshot, pixel/Golden evidence")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = wait for close)")
	flag.Parse()

	sceneName, err := normalizedScene(*sceneFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(2)
	}
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		// tools/ruler_flutter/scripts/measure.py sets RULER_SECONDS to
		// duration+warmup+headroom for reference rulers. Accept the same
		// contract so one sampler can launch all three arms.
		if v := os.Getenv("RULER_SECONDS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				secs, secsSet = n, true
			}
		}
	}
	gateMode := *autoOnly
	if gateMode {
		if !secsSet {
			secs = 5
		}
		if secs < 5 {
			fmt.Fprintln(os.Stderr, "FAIL: auto-only needs RUN_SECONDS >= 5")
			os.Exit(2)
		}
	} else if !secsSet {
		secs = 0
	}

	// Do not preload the CJK/Latin UI face for the spin/Godot scenes: they
	// contain no text, and forcing the font chain would add ~100MB RSS to a
	// comparison arm that has no font workload. list/text load it lazily via
	// wrkit.Label/FaceAt.
	spec, err := buildScene(sceneName, gateMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(2)
	}

	win, err := platform.Open(platform.Options{
		Width: spec.winW, Height: spec.winH, Title: spec.title,
		Decorations: true, Resizable: false,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	var proc scheduler.ProcessTracker
	proc.Start()

	runFor := time.Duration(0)
	if gateMode || secsSet {
		runFor = time.Duration(secs) * time.Second
	} else if *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	snapPath := os.Getenv("RULER_SNAP")
	if gateMode {
		snapPath = filepath.Join("examples", "ui_ruler_parity", "testdata", "showcase_"+sceneName+".png")
		if err := os.MkdirAll(filepath.Dir(snapPath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
			os.Exit(1)
		}
	}

	app := embedder.NewPipelineApp(host, spec.root, embedder.PipelineOptions{
		ClearR: spec.clearR, ClearG: spec.clearG, ClearB: spec.clearB, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventCloseRequested || ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_ruler_parity: close (%s)\n", win.Backend())
			}
			if spec.onEvent != nil {
				spec.onEvent(ev)
			}
		},
	})
	// measure.py requests a normal close with WM_DELETE_WINDOW; SIGTERM and
	// SIGINT remain the fallback for manual/legacy runs. Trap both signals and
	// leave through Run -> app.Close -> win.Close instead of letting the Go
	// runtime exit with the GPU surface still alive: on the 940MX/NVIDIA Xorg
	// path that abrupt teardown crashes the X server (nvidia_drv.so
	// nvidiaUnlock). Keep this handler installed until the process exits; a
	// second signal is intentionally not force-exited here.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)
	go func() {
		sig, ok := <-sigCh
		if !ok {
			return
		}
		fmt.Fprintf(os.Stderr, "ui_ruler_parity: %s received, closing gracefully\n", sig)
		app.Quit()
	}()

	if spec.fullPaint {
		app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	}

	meter := &frameMeter{scene: sceneName, started: time.Now()}
	if !gateMode && spec.tick != nil {
		app.Scheduler().Tickers().Add(&tickFunc{f: func(dt float64) bool {
			spec.tick(dt)
			app.ScheduleFrame()
			proc.Sample()
			meter.tick(app)
			return true
		}})
		app.Scheduler().SetMode(scheduler.ModePersistent)
	}

	// A static scene must not drive frames. Sample RSS from a plain goroutine
	// (same posture as ui_base_idle), never from a ticker.
	var idleStop, idleDone chan struct{}
	if !gateMode && spec.tick == nil {
		idleStop, idleDone = make(chan struct{}), make(chan struct{})
		go func() {
			defer close(idleDone)
			tk := time.NewTicker(500 * time.Millisecond)
			defer tk.Stop()
			for {
				select {
				case <-tk.C:
					proc.Sample()
				case <-idleStop:
					return
				}
			}
		}()
	}

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	if idleStop != nil {
		close(idleStop)
		<-idleDone
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())
	if spec.cleanup != nil {
		spec.cleanup()
	}

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	meter.final(app, elapsed, presents)

	if gateMode {
		ok := runGate(spec, snapPath, snap, presents, elapsed)
		if !ok {
			os.Exit(1)
		}
		return
	}

	extra := map[string]any{
		"ruler_scene": sceneName,
		"reference":   spec.ref,
		"client":      fmt.Sprintf("%dx%d", spec.winW, spec.winH),
		"mode":        "live",
		"impl_note":   "same scene geometry/animation as the reference ruler; see README",
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "ui_ruler_parity",
		Scenario:      sceneName,
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(spec.winW * spec.winH),
		Warmup:        true,
		Extra:         extra,
	})
	raw, err := wrgate.Marshal(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: marshal report:", err)
		os.Exit(1)
	}
	fmt.Println(string(raw))
	fmt.Fprintln(os.Stderr, "ui_ruler_parity: OK")
}

func envDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// frameMeter emits the same machine-readable lines as the Flutter ruler so
// tools/ruler_flutter/scripts/measure.py can parse both arms unchanged.
type frameMeter struct {
	scene      string
	started    time.Time
	last       time.Time
	lastFrames int64
	lastHitch  int64
}

func (m *frameMeter) tick(app *embedder.PipelineApp) {
	now := time.Now()
	if m.last.IsZero() {
		m.last = now
		m.lastFrames = app.PresentCount()
		m.lastHitch = app.Metrics().Snapshot().HitchCount
		return
	}
	if now.Sub(m.last) < 2*time.Second {
		return
	}
	snap := app.Metrics().Snapshot()
	frames := app.PresentCount()
	dt := now.Sub(m.last).Seconds()
	fps := 0.0
	if dt > 0 {
		fps = float64(frames-m.lastFrames) / dt
	}
	hitch := snap.HitchCount - m.lastHitch
	if hitch < 0 {
		hitch = 0
	}
	obj := map[string]any{
		"event":  "RULER_METRIC",
		"scene":  m.scene,
		"t_s":    fmt.Sprintf("%.1f", now.Sub(m.started).Seconds()),
		"frames": frames - m.lastFrames,
		"fps":    round1(fps),
		"p95_ms": round2(snap.P95FrameIntervalMs),
		"hitch":  hitch,
	}
	_ = json.NewEncoder(os.Stdout).Encode(obj)
	m.last = now
	m.lastFrames = frames
	m.lastHitch = snap.HitchCount
}

func (m *frameMeter) final(app *embedder.PipelineApp, elapsed float64, presents int64) {
	snap := app.Metrics().Snapshot()
	fps := 0.0
	if elapsed > 0 {
		fps = float64(presents) / elapsed
	}
	obj := map[string]any{
		"event":         "RULER_FINAL",
		"scene":         m.scene,
		"t_s":           fmt.Sprintf("%.1f", elapsed),
		"frames":        presents,
		"fps_avg":       round2(fps),
		"p95_ms":        round2(snap.P95FrameIntervalMs),
		"raster_p95_ms": round2(snap.LastRasterMs),
	}
	_ = json.NewEncoder(os.Stdout).Encode(obj)
}

func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }
func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
