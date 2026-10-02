// Command video_4k_play is the P3 4K real-window: 3840x2160 loop play +
// same-source big+small views (one upload, two draws) + overlay.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	bash video/testdata/gen_p3_2k4k.sh   # clips are generated, never committed
//	RUN_SECONDS=60 go run ./examples/video/video_4k_play
//
// Window: 1600x900. GPU window required. One 960x540 view plus one
// 320x180 PiP of the same 4K frame (second draw reuses the texture).
// Gate: fps/p95 + upload discipline (uploads+fallbacks==frames, PiP draws
// free) + first/last frame non-black + memory cap.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1600, 900

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "4K")
	} else {
		secs = 60
	}
	wrkit.EnsureUIFace()

	live, clipPath, err := openLive()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	defer live.Close()
	info := live.Info()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_4k_play — 4K直传", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "4K 直传 — 循环播/同源大小窗/浮层", []string{
		"4K真片循环播，走直传纹理",
		"大小窗同源同帧，不重传",
		"解码分辨率不动，显示缩放",
		"门禁：fps/p95+上传对帧数",
		"跑满60秒，首尾帧非黑",
	})
	shell.Body.LabelAt("4K 直传播放", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(fmt.Sprintf("%s %dx%d %.0ffps 封顶%dMB", clipName, info.Width, info.Height, info.FrameRate, live.Stats().MemCapKB>>10),
		13, 20, 44, 0.72, 0.8, 0.9)

	bridge := render.NewVideoBridge(nil)
	defer bridge.Close()
	node := rendering.NewRenderVideoPiP(960, 540, 320, 180, bridge)
	shell.Body.LabelAt("直播（大窗+右下小窗同帧）", 13, 20, 176, 0.6, 0.8, 0.95)
	shell.Body.Place(node, 20, 202)

	var proc scheduler.ProcessTracker
	proc.Start()

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "video_4k_play: 关闭 (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	phase := ""
	hotTick := 0
	liveShown := int64(0)
	clock := wrkit.NewPhaseClock(8, 12)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		if hotTick%30 == 0 {
			if dev, _, _, _, ok := render.BorrowVideoBackend(); ok {
				bridge.EnsureDevice(dev)
			}
		}
		if f, _ := live.Poll(); f != nil {
			liveShown++
			node.UploadPlanes(f.Width, f.Height, f.Y, f.UV, f.Pix)
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		bst := bridge.Stats()
		nst := node.NodeStats()
		gatePreview := snapH.PresentCount > 0 && nst.DrewOK && nst.SmallDrew
		shell.UpdateHUD("4K", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("显示=%d 上传=%d 回落=%d 重画=%d", liveShown, bst.Uploads, bst.Fallbacks, bst.Redraws),
			fmt.Sprintf("hw=%s 直接=%v", info.HWName, nst.DirectLast))
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 打开失败:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 运行失败:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	pst := live.Stats()
	bst := bridge.Stats()
	nst := node.NodeStats()
	rep := buildReport(snap, app.PresentCount(), elapsed, info, clipPath, pst, bst, nst, liveShown, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(4K需要真窗口)")
		os.Exit(1)
	}
	fpsWall := 0.0
	if elapsed > 0.001 {
		fpsWall = float64(app.PresentCount()) / elapsed
	}
	if fpsWall < 55 || snap.P95FrameIntervalMs > 22 {
		fmt.Fprintf(os.Stderr, "FAIL: 帧时不达标 fps=%.1f p95=%.2f毫秒(要≥55fps且p95≤22毫秒)\n", fpsWall, snap.P95FrameIntervalMs)
		os.Exit(1)
	}
	if liveShown < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 直播一帧没播出来")
		os.Exit(1)
	}
	if bst.Frames < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 桥一帧没收到")
		os.Exit(1)
	}
	if bst.Uploads+bst.Fallbacks != bst.Frames {
		fmt.Fprintf(os.Stderr, "FAIL: 上传纪律破 上传=%d 回落=%d 帧=%d(上传+回落必须等于帧数，无多余重传)\n", bst.Uploads, bst.Fallbacks, bst.Frames)
		os.Exit(1)
	}
	if bst.Evictions != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 视频槽淘汰=%d，健康态必须为零\n", bst.Evictions)
		os.Exit(1)
	}
	if !nst.DrewOK || !nst.SmallDrew {
		fmt.Fprintf(os.Stderr, "FAIL: 双窗没画全 大=%v 小=%v\n", nst.DrewOK, nst.SmallDrew)
		os.Exit(1)
	}
	if bst.Redraws < 1 && bst.Fallbacks == 0 {
		fmt.Fprintln(os.Stderr, "FAIL: 同源小窗一次没重画（直传下重画应免费）")
		os.Exit(1)
	}
	if nst.FirstMean <= 5 || nst.LastMean <= 5 {
		fmt.Fprintf(os.Stderr, "FAIL: 首尾帧黑 首=%.1f 尾=%.1f(必须>5)\n", nst.FirstMean, nst.LastMean)
		os.Exit(1)
	}
	if rep.RSSPeakKB > rep.MemCapKB && rep.MemCapKB > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 内存峰值=%dKB 超过上限=%dKB\n", rep.RSSPeakKB, rep.MemCapKB)
		os.Exit(1)
	}
	if rep.TimeToFirstFrameMs > 2000 && rep.TimeToFirstFrameMs > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 首帧=%.1f毫秒 超过2000毫秒\n", rep.TimeToFirstFrameMs)
		os.Exit(1)
	}
	if err := checkBaseline(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "video_4k_play: 通过 显示=%d 上传=%d 回落=%d 重画=%d 上屏=%d 用时=%.1f秒\n",
		liveShown, bst.Uploads, bst.Fallbacks, bst.Redraws, app.PresentCount(), elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func phaseCN(phase string) string {
	switch phase {
	case wrkit.PhaseSteady:
		return "稳态"
	case wrkit.PhaseSpike:
		return "冲击"
	case wrkit.PhaseRecover:
		return "恢复"
	default:
		return phase
	}
}
