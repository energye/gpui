// Command ui_text_m4_virtual is the M4 real-window: 1e6 行日志虚拟化
// (ENGINE_TEXT_SCALE_PLAN §M4).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/ui_text_m4_virtual
//
// Window: 1200x800. Headless probes: GPUI_M4_SELFTEST=1 (no GPU window).
//
// 行内容来自 testdata/log_sample.txt（40 条代表性日志），窗内按序循环复用
// 到 1e6 行，main.go 不发明行内容。几何真源是 VirtualTextLines（懒测量 +
// Fenwick 前缀和）；VirtualList 只负责挂载视口 cell，行高经 ExtentFunc 从
// VirtualTextLines 取（复用 virtual_list.go 范式，不加新概念）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	winW, winH   = 1200, 800
	closeSeconds = 30
	lineCount    = 1000000
	lineEst      = 20.0
	vpH          = 800.0
	listW        = 620.0
	rightW       = 250.0
)

func testdataPath(name string) string {
	if dir := os.Getenv("GPUI_M4_TESTDATA"); dir != "" {
		return filepath.Join(dir, name)
	}
	return filepath.Join("examples", "ui_text_m4_virtual", "testdata", name)
}

func loadSample() []string {
	b, err := os.ReadFile(testdataPath("log_sample.txt"))
	if err != nil || len(b) == 0 {
		return []string{"2026-09-04T00:00:00Z INFO fallback log line"}
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if ln != "" {
			out = append(out, ln)
		}
	}
	if len(out) == 0 {
		return []string{"2026-09-04T00:00:00Z INFO fallback log line"}
	}
	return out
}

// measureHeight 模拟变行高：每 97 行一条折行日志（34px），其余 20px。
func measureHeight(i int) float64 {
	if i%97 == 96 {
		return 34
	}
	return 20
}

func memMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.HeapAlloc) / 1e6
}

type baseline struct {
	FullNil2kMs   float64 `json:"full_nilface_2k_ms"`
	FullNil20kMs  float64 `json:"full_nilface_20k_ms"`
	FullReal2kMs  float64 `json:"full_realface_2k_ms"`
	Virtual1MUs   float64 `json:"virtual_1M_first_us"`
	VirtualWindow int     `json:"virtual_window_rows"`
	VirtualTotal  float64 `json:"virtual_total_px"`
	VirtualHeapMB float64 `json:"virtual_heap_delta_mb"`
	Note          string  `json:"note"`
}

func measureFull(sample []string, n int) float64 {
	txt := strings.Repeat(sample[0]+"\n", n)
	t0 := time.Now()
	lay := rendering.BuildTextLayout(txt, nil, 16, 0, 1.25)
	_ = lay
	return float64(time.Since(t0).Microseconds()) / 1000
}

func selftest() {
	sample := loadSample()
	// 改前全量基线：face==nil 估算路径（快，可全量跑）；真字体只跑 2k 档锚定。
	full2k := measureFull(sample, 2000)
	full20k := measureFull(sample, 20000)
	real2k := -1.0
	if _, _, err := wrkit.EnsureUIFace(); err == nil {
		if face := wrkit.FaceAt(16); face != nil {
			txt := strings.Repeat(sample[0]+"\n", 2000)
			t0 := time.Now()
			_ = rendering.BuildTextLayout(txt, face, 16, 0, 1.25)
			real2k = float64(time.Since(t0).Microseconds()) / 1000
		}
	}
	runtime.GC()
	m0 := memMB()
	t0 := time.Now()
	v := rendering.NewVariableVirtualTextLines(lineCount, lineEst, measureHeight)
	f, l := v.SetViewport(0, vpH)
	n := v.MeasureWindow()
	vus := float64(time.Since(t0).Microseconds())
	m1 := memMB()
	out := map[string]any{
		"scenario": "ui_text_m4_virtual",
		"mode":     "headless-cpu",
		"baseline": baseline{
			FullNil2kMs:   full2k,
			FullNil20kMs:  full20k,
			FullReal2kMs:  real2k,
			Virtual1MUs:   vus,
			VirtualWindow: l - f,
			VirtualTotal:  v.TotalHeight(),
			VirtualHeapMB: m1 - m0,
			Note:          fmt.Sprintf("full=改前全量排版 O(n); virtual首屏只物化窗口内 %d 行; 1M全量未直接跑(按2k/20k线性外推约数十秒+数GB)", n),
		},
		"go": runtime.Version(),
	}
	raw, _ := json.Marshal(out)
	fmt.Println(string(raw))
}

func buildCell(sample []string, w float64, i int) rendering.RenderObject {
	h := measureHeight(i)
	cell := rendering.NewAbsoluteBox(w, h)
	t := wrkit.Label(fmt.Sprintf("%07d %s", i, sample[i%len(sample)]), 13, 0.82, 0.86, 0.92)
	cell.Place(t, 10, 2)
	return cell
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool { t.on(dt); return true }

func main() {
	if os.Getenv("GPUI_M4_SELFTEST") == "1" {
		selftest()
		return
	}
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	}
	wrkit.EnsureUIFace()
	sample := loadSample()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_text_m4_virtual — 1e6 行日志虚拟化", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "M4 1e6 行日志虚拟化 — 只物化视口 + 懒测量", []string{
		"100 万行日志，首屏只排视口 ~50 行",
		"变行高：折行 34px，其余 20px（懒实测）",
		"滚动脚本：慢滚 → 快滑 → 减速，到底 ping-pong",
		"bind_count ≪ 1e6，窗口滑动不跳变",
	})

	// 几何真源：VirtualTextLines；视图：VirtualList 经 ExtentFunc 取行高。
	vtl := rendering.NewVariableVirtualTextLines(lineCount, lineEst, measureHeight)
	list := rendering.NewVariableVirtualList(lineCount, lineEst, vtl.ExtentFunc(), func(i int) rendering.RenderObject {
		return buildCell(sample, listW, i)
	})
	list.CacheExtent = lineEst * 2

	vp := rendering.NewRenderViewport(list)
	vp.FixedWidth = listW
	vp.FixedHeight = vpH
	shell.Body.Place(vp, 0, 0)

	track := wrkit.NewPanel(10, vpH, 0.05, 0.06, 0.08, 1)
	shell.Body.Place(track.Box, listW-14, 0)
	thumb := rendering.NewRenderColorBox(6, 56, 0.45, 0.75, 0.95, 1)
	thumbAlign := track.Align(thumb, 0.5, 0)

	right := wrkit.NewPanel(rightW, vpH, 0.11, 0.12, 0.15, 1)
	shell.Body.Place(right.Box, listW+16, 0)
	right.LabelAt("M4 PROBE（视口内行才排版）", 12, 12, 10, 0.55, 0.75, 0.95)
	rangeLabel := right.LabelAt("rows ---- -- ---- / 1000000", 13, 12, 34, 0.95, 0.9, 0.4)
	rangeLabel.SetRepaintBoundary(true)
	totalLabel := right.LabelAt("total ---- px", 13, 12, 58, 0.70, 0.78, 0.88)
	totalLabel.SetRepaintBoundary(true)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_text_m4_virtual: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})

	maxY := vtl.TotalHeight() - vpH
	if maxY < 0 {
		maxY = 0
	}
	const cycleLen = 9.0
	var elapsed, dir, y float64 = 0, 1, 0
	lastFirst, lastLast := -1, -1
	maxBind, anchorJumps, winLagMax := 0, 0, 0
	invCount, invSkipped := 0, 0
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		ct := elapsed
		for ct >= cycleLen {
			ct -= cycleLen
		}
		var phase string
		var speed float64
		switch {
		case ct < 3:
			phase, speed = wrkit.PhaseSteady, 160
		case ct < 6:
			phase, speed = wrkit.PhaseSpike, 2600
		default:
			phase = wrkit.PhaseRecover
			speed = 2600 * (1 - (ct-6)/3)
			if speed < 40 {
				speed = 40
			}
		}
		y += dir * speed * dt
		if y >= maxY {
			y, dir = maxY, -1
		}
		if y <= 0 {
			y, dir = 0, 1
		}
		vp.SetScrollOffset(0, y)
		// 懒测量推进：只测 vtl 窗口内行（O(窗口)，每帧可做）。
		// 视图发布走增量刷新（M4.1）：只重读实测区间 + 后缀平移，
		// 约 0.6ms/次（全量重建约 10ms）。总高没真变时直接跳过。
		// 节流期间 list 显示旧高度（滞后很小，窗口锚点不动，不跳变）。
		vf, vl := vtl.SetViewport(y, vpH)
		anchorBefore := vtl.OffsetOf(vf)
		newMeasured := vtl.MeasureWindow()
		if vtl.OffsetOf(vf) != anchorBefore {
			anchorJumps++
		}
		if newMeasured > 0 && list.RefreshExtents(vf, vl) {
			invCount++
		} else if newMeasured > 0 {
			invSkipped++
		}
		if lf, ll := list.BoundRange(); lf != vf || ll != vl {
			if d := lf - vf; d < 0 {
				d = -d
				if d > winLagMax {
					winLagMax = d
				}
			} else if d > winLagMax {
				winLagMax = d
			}
		}
		if maxY > 0 {
			thumbAlign.SetAlignment(0.5, y/maxY)
		}
		f, l := list.BoundRange()
		if f != lastFirst || l != lastLast {
			lastFirst, lastLast = f, l
			rangeLabel.SetText(fmt.Sprintf("rows %07d-%07d / %d", f, l, lineCount))
			rangeLabel.MarkNeedsPaint()
			totalLabel.SetText(fmt.Sprintf("total %.0f px measured %d", vtl.TotalHeight(), vtl.MeasuredCount()))
			totalLabel.MarkNeedsPaint()
		}
		if list.BindCount > maxBind {
			maxBind = list.BindCount
		}
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BindCount > 0 && snapH.BindCount <= 128 && int64(lineCount) >= 1000
		shell.UpdateHUD("M4", phase, app, gateOK,
			fmt.Sprintf("bind=%d rows=%d-%d jumps=%d", snapH.BindCount, f, l, anchorJumps),
			fmt.Sprintf("maxBind=%d y=%.0f", maxBind, y))
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

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	vf, vl := vtl.Window()
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "text_m4_virtual",
		Scenario:      "ui_text_m4_virtual",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"line_count":        int64(lineCount),
			"bind_count":        snap.BindCount,
			"max_bind_count":    int64(maxBind),
			"bound_first":       int64(lastFirst),
			"bound_last":        int64(lastLast),
			"vtl_window_first":  int64(vf),
			"vtl_window_last":   int64(vl),
			"anchor_jumps":      int64(anchorJumps),
			"window_lag_max":    int64(winLagMax),
			"prefix_refreshes":   int64(invCount),
			"prefix_throttled":   int64(invSkipped),
			"measured_rows":     int64(vtl.MeasuredCount()),
			"total_height_px":   vtl.TotalHeight(),
			"sample_lines_file": "testdata/log_sample.txt",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
}
