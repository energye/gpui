// Command spin_min is the single-dynamic-icon truth window: one 64px
// spinning sync icon centered in a 480x320 window, nothing else moves.
// It exists for CPU testing and optimization (see README.md).
// Structure mirrors examples/ui_l1_spinner (verified visible); only the
// content is the kit icon (behavior lives in ui/kit, the example only
// places it and ticks it).
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	// 和 icon、ui_l1_spinner 同一写法：注册 GPU 加速。缺了它真窗上屏是黑的。
	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 480, 320
	iconEdge   = 64
)

type ticker struct {
	box *kit.IconBox
	app *embedder.PipelineApp
}

func (t *ticker) Tick(dt float64) bool {
	if t.box.Inst.Tick(dt) {
		t.box.SyncIconTransform()
	}
	t.app.ScheduleFrame()
	return true
}

func main() {
	secs, autoOnly, ok := 0, false, true
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			secs = n
		}
	}
	flag.BoolVar(&autoOnly, "auto-only", true, "fail without GPU window")
	flag.Parse()
	if !autoOnly {
		ok = false
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit spin_min — one spinning icon", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	ctx := kit.DefaultScopeCtx()
	props := kit.DefaultIconProps("sync")
	props.Variant = "outlined"
	props.Spin = true
	// 深色底配白色图标，不然默认的黑字看不见。走 IconProps 公开字段传。
	props.Color, props.ColorSet = "#ffffff", true
	spin := kit.BuildIconBox(ctx, props, iconEdge, "spin-min-sync")
	// AbsoluteBox 才保得住居中坐标，RenderBox 排版时会归零。用 Place 摆，和 icon 窗一样。
	root := rendering.NewAbsoluteBox(float64(winW), float64(winH))
	center := func(w, h int) {
		root.Place(spin.Outer,
			float64(w)/2-iconEdge/2,
			float64(h)/2-iconEdge/2)
	}
	center(winW, winH)

	var proc scheduler.ProcessTracker
	proc.Start()

	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: 0.10, ClearG: 0.12, ClearB: 0.16, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		// 按需快照：设 SPIN_SNAP=路径就存最后一帧，不设就是原来行为。
		SnapshotPath: os.Getenv("SPIN_SNAP"),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "spin_min: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					center(ev.Width, ev.Height)
					root.MarkNeedsLayout()
				}
			}
		},
	})
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: app open:", err)
		os.Exit(1)
	}

	t0 := time.Now()
	app.Scheduler().Tickers().Add(&ticker{box: spin, app: app})

	runErr := app.Run()
	elapsed := time.Since(t0)
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	if runErr != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", runErr)
		os.Exit(1)
	}
	presents := app.PresentCount()
	m := app.Metrics().Snapshot()
	cpu := proc.CPUPctAvg
	fmt.Fprintf(os.Stderr, "spin_min: %d presents in %v (%s)\n",
		presents, elapsed.Round(time.Millisecond), win.Backend())
	fmt.Fprintf(os.Stderr, "spin_min: metrics %+v\n", m)
	fmt.Fprintf(os.Stderr, "spin_min: cpu %.2f%% (proc)\n", cpu)
	if presents < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents (needs_gpu_window)")
		os.Exit(1)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "FAIL: --auto-only=false")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "spin_min: OK")
}
