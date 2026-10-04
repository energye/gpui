// 对照窗：单进程三窗小损伤动画（诊断保留）。
// 每窗 800x600：静态深底 + 6 静色块（首帧后只贴回）+ 1 个 120x120 动块
// （水平往返，只脏小区域）。与鹈鹕整窗动画对照，判定多窗慢的贵账在
// 引擎开销还是在整窗内容。
// 用法：CTRL_WINDOWS=3 RUN_SECONDS=180 go run ./examples/zz_soak_control
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/ui/application"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 800, 600
	boxSize      = 120.0
	staticBlocks = 6
)

type animState struct {
	root *rendering.AbsoluteBox
	box  *rendering.AbsoluteBox
	x    float64
	dir  float64
}

func buildScene(st *animState) *rendering.AbsoluteBox {
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	cols := [][3]float64{
		{0.85, 0.25, 0.25}, {0.25, 0.75, 0.30}, {0.25, 0.45, 0.85},
		{0.85, 0.70, 0.20}, {0.60, 0.30, 0.80}, {0.25, 0.70, 0.70},
	}
	for i := 0; i < staticBlocks; i++ {
		b := rendering.NewAbsoluteBox(150, 100)
		c := cols[i%len(cols)]
		b.Background = &rendering.Color{R: c[0], G: c[1], B: c[2], A: 1}
		b.SetRepaintBoundary(true)
		root.Place(b, float64(20+(i%3)*260), float64(20+(i/3)*140))
	}
	st.box = rendering.NewAbsoluteBox(boxSize, boxSize)
	st.box.Background = &rendering.Color{R: 0.95, G: 0.55, B: 0.15, A: 1}
	st.box.SetRepaintBoundary(true)
	st.x, st.dir = 100, 260
	root.Place(st.box, st.x, 420)
	st.root = root
	return root
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func openWin(app *application.App, title string) *embedder.PipelineApp {
	st := &animState{}
	root := buildScene(st)
	w, err := app.SpawnWindow(application.WindowOptions{
		Width: winW, Height: winH, Title: title, Resizable: true,
	}, root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open:", err)
		os.Exit(1)
	}
	pipe := w.Pipeline()
	pipe.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		st.x += st.dir * dt
		if st.x > winW-100-boxSize {
			st.x = winW - 100 - boxSize
			st.dir = -st.dir
		}
		if st.x < 100 {
			st.x = 100
			st.dir = -st.dir
		}
		st.root.Place(st.box, st.x, 420)
		// 内容没变只挪位置，不标脏：走位移贴回，老位置由损伤补底。
		pipe.ScheduleFrame()
	}})
	pipe.Scheduler().SetMode(scheduler.ModePersistent)
	pipe.SetPresentPolicy(scheduler.PresentPolicyRetained)
	return pipe
}

func main() {
	secs := 120
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	nwins := 3
	if v := os.Getenv("CTRL_WINDOWS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			nwins = n
		}
	}
	app := application.New(application.Config{
		Name:    "gpui zz_soak_control",
		Backend: platform.DisplayAuto,
		RunFor:  time.Duration(secs) * time.Second,
		WarmUp:  true,
	})
	var pipes []*embedder.PipelineApp
	for i := 0; i < nwins; i++ {
		pipes = append(pipes, openWin(app, fmt.Sprintf("gpui zz_soak_control #%d", i+1)))
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	for i, p := range pipes {
		m := p.Metrics().Snapshot()
		fps := float64(p.PresentCount()) / elapsed
		fmt.Fprintf(os.Stderr, "CTRL win%d presents=%d ~%.1f fps avg=%.2f p50=%.2f p95=%.2f hitches=%d raster=%.2f flush=%.2f acquire=%.2f presentWait=%.2f rerecord=%d skip=%d\n",
			i+1, p.PresentCount(), fps,
			m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P95FrameIntervalMs, m.HitchCount,
			m.LastRasterMs, m.LastFlushMs, m.LastAcquireWaitMs, m.LastPresentWaitMs,
			m.BoundaryRerecord, m.BoundarySkip)
	}
}
