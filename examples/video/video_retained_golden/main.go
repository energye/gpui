// Command video_retained_golden is the C4 Golden real-window: one committed
// NV12 frame through RenderVideo -> scene.TextureLayer -> retained composite,
// with colour probes and a zero-tolerance Golden.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	bash video/testdata/gen_c4_golden.sh          # fixture is committed; script documents/rechecks it
//	go run ./examples/video/video_retained_golden -auto-only
//	GPUI_ACCEPT_GOLDEN=1 go run ./examples/video/video_retained_golden -auto-only   # seed, review, commit
//	go run ./examples/video/video_retained_golden -manual-seconds 15
//
// Window: 1200x800. GPU window required. The fixture is 904x656, exactly the
// shell body at this window size, so the video draws 1:1 and the Golden does
// not depend on a sampler's scale path. The frame is frozen after one upload:
// retained replays are therefore byte-stable, which is what a committed Golden
// needs. Decode is deliberately out of scope (VR3 owns ffmpeg-anchored decode
// baselines); this window judges the C4 render path only.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/wrevidence"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	// videoW/videoH equal the wrkit body at 1200x800; the fixture matches
	// them 1:1.
	videoW, videoH = 904, 656

	clearR, clearG, clearB = 0.08, 0.09, 0.11

	// probeTol is the pixel-assertion standard's ≤8/255 for a colour compared
	// against theory (BT.601 limited here, the same numbers as the shader).
	probeTol = 8.0 / 255.0

	fixtureName = "c4_golden_904x656_nv12.yuv"
)

// probeSpec picks a point inside one flat colour field. Fields are wide
// (226x328), so the sample never lands on a bar edge or on the white cross.
type probeSpec struct {
	desc string
	fx   float64
	fy   float64
}

var probes = []probeSpec{
	{"red", 0.125, 0.25},
	{"green", 0.375, 0.25},
	{"blue", 0.625, 0.25},
	{"magenta", 0.625, 0.75},
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze one frame, replay twice, colour + Golden gates (default when -manual-seconds is 0)")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: stay open N seconds (>=5), no gate PNG")
	flag.Parse()

	gateMode := !*autoOnly && *manualSeconds <= 0
	if *autoOnly {
		gateMode = true
	}
	if *manualSeconds > 0 {
		gateMode = false
		if *manualSeconds < 5 {
			fmt.Fprintln(os.Stderr, "FAIL: -manual-seconds must be >= 5")
			os.Exit(1)
		}
	}

	wrkit.EnsureUIFace()

	data, fixturePath := loadFixture()
	yPlane, uvPlane := data[:videoW*videoH], data[videoW*videoH:]

	win, err := platform.Open(platform.Options{
		Width: winW, Height: winH, Title: "gpui video_retained_golden — 固定帧纹理层 Golden", Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C4 视频纹理层 — 固定帧 Golden", []string{
		"一帧 NV12 固定素材，只上传一次",
		"走 RenderVideo -> 纹理层 -> 合成",
		"八色块 U/V 各不相同，色度错位必现",
		"门禁：颜色探针 + 逐位 Golden",
		"种子：GPUI_ACCEPT_GOLDEN=1 再跑",
	})
	if shell.Body.W != videoW || shell.Body.H != videoH {
		fmt.Fprintf(os.Stderr, "FAIL: body 尺寸 %.0fx%.0f 与素材 %dx%d 不一致，请重算素材\n",
			shell.Body.W, shell.Body.H, videoW, videoH)
		os.Exit(1)
	}

	bridge := render.NewVideoBridge(nil)
	defer bridge.Close()
	node := rendering.NewRenderVideo(videoW, videoH, bridge)
	shell.Body.Place(node, 0, 0)

	snapDir := filepath.Join("examples", "video", "video_retained_golden", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 建 Golden 目录:", err)
		os.Exit(1)
	}
	retAPath := filepath.Join(os.TempDir(), "video_retained_golden_a.png")
	retBPath := filepath.Join(os.TempDir(), "video_retained_golden_b.png")

	runFor := 6 * time.Second
	if gateMode {
		secs, secsSet := wrkit.RunSecondsOpt()
		if secsSet {
			wrkit.RequireMinRun(secs, "C4 Golden")
			runFor = time.Duration(secs) * time.Second
		}
	} else {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	ctl := win.Controls()
	manualEvents := 0
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearR, ClearG: clearG, ClearB: clearB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "video_retained_golden: 关闭 (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle("gpui video_retained_golden — closing")
				}
				return
			}
			if ev.Type == platform.EventWake {
				return
			}
			manualEvents++
			fmt.Fprintf(os.Stderr, "video_retained_golden: 事件 #%d type=%v\n", manualEvents, ev.Type)
			if ctl != nil {
				ctl.SetTitle(fmt.Sprintf("gpui video_retained_golden — 事件 #%d type=%v", manualEvents, ev.Type))
			}
		},
	})

	var uploaded bool
	var capA, capB wrevidence.RetainedCapture
	var texPaintedMax int64
	settle := 0
	ticks := 0
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		ticks++
		if !uploaded {
			// The bridge borrows the window's device; the first ticks run
			// before it exists, so keep asking until the one-shot upload lands.
			if dev, _, _, _, ok := render.BorrowVideoBackend(); ok {
				bridge.EnsureDevice(dev)
				if node.UploadPlanes(videoW, videoH, yPlane, uvPlane, nil) {
					uploaded = true
				}
			}
		}
		app.ScheduleFrame()
		painted, _ := app.TexturePaintStats()
		if painted > texPaintedMax {
			texPaintedMax = painted
		}
		nst := node.NodeStats()
		showing := uploaded && nst.DrewLast && painted >= 1 && app.PresentCount() >= 3
		if showing {
			settle++
		}
		if gateMode && showing {
			if !capA.OK() && settle >= 2 {
				capA.Start(app, retAPath)
			} else if capA.OK() && !capB.OK() && settle >= 6 {
				capB.Start(app, retBPath)
			}
		}
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("C4", "固定帧", app, showing && uploaded,
			fmt.Sprintf("上传=%v 出画=%v 纹理=%d", uploaded, nst.DrewLast, painted),
			"Golden 固定帧")
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
	elapsed := time.Since(t0).Seconds()

	if !gateMode {
		nst := node.NodeStats()
		fmt.Fprintf(os.Stderr, "video_retained_golden: 人工模式 上传=%v 出画=%v 纹理出画峰值=%d 上屏=%d 事件=%d\n",
			uploaded, nst.DrewLast, texPaintedMax, app.PresentCount(), manualEvents)
		raw, _ := json.Marshal(struct {
			AbilityID     string   `json:"ability_id"`
			Mode          string   `json:"mode"`
			PixelEvidence string   `json:"pixel_evidence"`
			GoldenDiffPct *float64 `json:"pixel_golden_diff_pct"`
			ElapsedSec    float64  `json:"elapsed_sec"`
			Presents      int64    `json:"present_count"`
			Uploaded      bool     `json:"uploaded"`
			DrewLast      bool     `json:"video_drew_last"`
			TexPainted    int64    `json:"retained_tex_painted_max"`
			ManualEvents  int      `json:"manual_events"`
		}{
			AbilityID: "video_retained_golden", Mode: "manual",
			PixelEvidence: "manual_not_asserted",
			ElapsedSec:    elapsed, Presents: app.PresentCount(),
			Uploaded: uploaded, DrewLast: nst.DrewLast,
			TexPainted: texPaintedMax, ManualEvents: manualEvents,
		})
		fmt.Println(string(raw))
		app.Close()
		win.Close()
		return
	}

	// A replay reads the window's render context, so wait while close can still
	// submit jobs (4s bounds the 3s internal submit window).
	capA.Wait(4 * time.Second)
	capB.Wait(4 * time.Second)
	app.Close()
	win.Close()

	rep := report{
		AbilityID:     "video_retained_golden",
		Scenario:      "one committed NV12 frame -> RenderVideo -> scene.TextureLayer -> retained composite",
		ElapsedSec:    elapsed,
		Fixture:       fixturePath,
		Uploaded:      uploaded,
		Presents:      app.PresentCount(),
		PaintCount:    app.Metrics().Snapshot().PaintCount,
		TexPainted:    texPaintedMax,
		RetainedAOK:   capA.OK(),
		RetainedBOK:   capB.OK(),
		ColorTotal:    len(probes),
		GoldenMaskPx:  0,
		GoldenDiffPct: 100,
	}
	if elapsed > 0.001 {
		rep.FPS = float64(rep.Presents) / elapsed
	}
	bst := bridge.Stats()
	rep.VideoFrames, rep.VideoUploads, rep.VideoFallbacks = bst.Frames, bst.Uploads, bst.Fallbacks
	nst := node.NodeStats()
	rep.DrewOK, rep.DrewLast = nst.DrewOK, nst.DrewLast
	rep.RecordSkips = nst.RecordSkips
	rep.UploadPending = node.UploadPending()

	var pairErr string
	pair, pairErr := wrevidence.OpenRetainedPair(retAPath, retBPath, capA.OK(), capB.OK(), winW)
	scale := 1.0
	if pair == nil {
		rep.StabilityErr = pairErr
	} else {
		img := wrevidence.Load(retAPath)
		if img == nil {
			rep.StabilityErr = "快照解码失败"
		} else {
			scale = float64(img.Bounds().Dx()) / winW
			rects := goldenMask(shell, scale)
			rep.StabilityDiffPct, rep.GoldenMaskPx, _ = wrevidence.Golden(retAPath, retBPath, rects, "video_retained_golden/stability")
			if rep.StabilityDiffPct != 0 {
				rep.StabilityErr = fmt.Sprintf("两次 retained 回放不一致 diff=%.4f%%", rep.StabilityDiffPct)
			}
			checks := colorChecks(yPlane, uvPlane, shell, scale)
			res := wrevidence.CheckPixels(img, checks, "video_retained_golden")
			for _, c := range checks {
				ok := res[c.Desc]
				if ok {
					rep.ColorOK++
				}
				rep.Probes = append(rep.Probes, probeResult{Desc: c.Desc, OK: ok})
			}
			baseName := fmt.Sprintf("showcase_retained_golden_dpr%s_base.png", dprTag(scale))
			basePath := filepath.Join(snapDir, baseName)
			rep.GoldenBase = basePath
			rep.GoldenDiffPct, rep.GoldenTotalPx, rep.GoldenFirstRun =
				wrevidence.Golden(retAPath, basePath, rects, "video_retained_golden")
			if !rep.GoldenFirstRun {
				rep.GoldenBaseOK = baselineMatches(basePath, img)
			}
		}
	}

	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	failed := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		os.Exit(1)
	}
	if !uploaded {
		failed("固定素材一帧都没上传成功(device/fixture)")
	}
	if rep.VideoFrames != 1 || rep.VideoUploads != 1 || rep.VideoFallbacks != 0 {
		failed("上传纪律 帧=%d 上传=%d 回落=%d (固定帧必须恰好 1/1/0)", rep.VideoFrames, rep.VideoUploads, rep.VideoFallbacks)
	}
	if !nst.DrewOK || !nst.DrewLast {
		failed("桥最近一帧没画 曾=%v 末=%v", nst.DrewOK, nst.DrewLast)
	}
	if rep.TexPainted < 1 {
		failed("合成一帧都没画纹理层(纹理出画峰值=%d)", rep.TexPainted)
	}
	if rep.Presents < 1 {
		failed("一次都没上屏(需要真窗口)")
	}
	if !capA.OK() || !capB.OK() {
		failed("retained 回放缺片 a=%v b=%v", capA.OK(), capB.OK())
	}
	if rep.StabilityErr != "" {
		failed("%s", rep.StabilityErr)
	}
	if rep.ColorOK != rep.ColorTotal {
		failed("颜色探针 %d/%d 达标", rep.ColorOK, rep.ColorTotal)
	}
	if rep.GoldenFirstRun {
		fmt.Fprintf(os.Stderr, "video_retained_golden: 首次运行只种基线 %s（不算 PASS）；复查后提交，再跑一次应 diff=0\n", rep.GoldenBase)
		return
	}
	if !rep.GoldenBaseOK {
		failed("基线尺寸与本次快照不一致: %s", rep.GoldenBase)
	}
	if rep.GoldenDiffPct != 0 {
		failed("Golden diff=%.4f%% over %d px (零容差)", rep.GoldenDiffPct, rep.GoldenTotalPx)
	}
	fmt.Fprintf(os.Stderr, "video_retained_golden: 通过 纹理出画=%d 颜色=%d/%d 稳定=0 Golden=0(%d px, dpr=%s)\n",
		rep.TexPainted, rep.ColorOK, rep.ColorTotal, rep.GoldenTotalPx, dprTag(scale))
}

type report struct {
	AbilityID  string  `json:"ability_id"`
	Scenario   string  `json:"scenario"`
	ElapsedSec float64 `json:"elapsed_sec"`
	Fixture    string  `json:"fixture"`

	FPS        float64 `json:"fps_wall"`
	Presents   int64   `json:"present_count"`
	PaintCount int64   `json:"paint_count"`

	VideoFrames    uint64 `json:"video_frames"`
	VideoUploads   uint64 `json:"video_uploads"`
	VideoFallbacks uint64 `json:"video_fallbacks"`
	Uploaded       bool   `json:"uploaded"`
	UploadPending  bool   `json:"upload_pending"`

	DrewOK      bool  `json:"video_drew_ok"`
	DrewLast    bool  `json:"video_drew_last"`
	TexPainted  int64 `json:"retained_tex_painted_max"`
	RecordSkips int   `json:"video_record_skips"`

	RetainedAOK  bool   `json:"retained_snapshot_a_ok"`
	RetainedBOK  bool   `json:"retained_snapshot_b_ok"`
	StabilityErr string `json:"retained_stability_error"`

	ColorOK    int           `json:"pixel_color_ok"`
	ColorTotal int           `json:"pixel_color_total"`
	Probes     []probeResult `json:"pixel_probes"`

	StabilityDiffPct float64 `json:"retained_stability_diff_pct"`
	GoldenDiffPct    float64 `json:"pixel_golden_diff_pct"`
	GoldenTotalPx    int64   `json:"pixel_golden_total_px"`
	GoldenMaskPx     int64   `json:"pixel_golden_mask_px"`
	GoldenFirstRun   bool    `json:"pixel_golden_first_run"`
	GoldenBaseOK     bool    `json:"pixel_golden_base_ok"`
	GoldenBase       string  `json:"pixel_golden_base"`
}

type probeResult struct {
	Desc string `json:"desc"`
	OK   bool   `json:"ok"`
}

// loadFixture reads the committed NV12 frame. Missing or wrong-sized data is a
// hard failure (this window is the gate; it must not turn into a silent skip).
func loadFixture() ([]byte, string) {
	path := resolveData(filepath.Join("video", "testdata", fixtureName))
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: 固定素材缺失 %s (%v)；先跑 bash video/testdata/gen_c4_golden.sh\n", path, err)
		os.Exit(1)
	}
	want := videoW * videoH * 3 / 2
	if len(data) != want {
		fmt.Fprintf(os.Stderr, "FAIL: 素材大小=%d 期望=%d (%dx%d NV12)\n", len(data), want, videoW, videoH)
		os.Exit(1)
	}
	return data, path
}

func resolveData(name string) string {
	for _, p := range []string{name, filepath.Join("..", "..", name)} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name
}

// colorChecks derives each probe's expected colour from the fixture bytes at
// the same source pixel, using the shader's BT.601 limited-range matrix. The
// expectation is therefore data, not a hand-copied constant.
func colorChecks(yPlane, uvPlane []byte, shell *wrkit.ShellChrome, scale float64) []wrevidence.PixelCheck {
	bodyX, bodyY := shell.Body.X, shell.Body.Y
	out := make([]wrevidence.PixelCheck, 0, len(probes))
	for _, p := range probes {
		x, y := int(p.fx*videoW), int(p.fy*videoH)
		r, g, b := nv12RGB(yPlane, uvPlane, x, y)
		out = append(out, wrevidence.PixelCheck{
			Desc: p.desc,
			X:    (bodyX + float64(x)) * scale,
			Y:    (bodyY + float64(y)) * scale,
			Want: [3]float64{r, g, b},
			Tol:  probeTol,
		})
	}
	return out
}

func nv12RGB(yPlane, uvPlane []byte, x, y int) (float64, float64, float64) {
	c := float64(yPlane[y*videoW+x]) - 16
	i := (y/2)*videoW + (x/2)*2
	u := float64(uvPlane[i]) - 128
	v := float64(uvPlane[i+1]) - 128
	r := clampByte(1.164*c + 1.596*v)
	g := clampByte(1.164*c - 0.391*u - 0.813*v)
	b := clampByte(1.164*c + 2.018*u)
	return r / 255, g / 255, b / 255
}

func clampByte(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// goldenMask is the body panel, inset one logical pixel to keep the rounded
// panel edge out. Top bar, legend and HUD stay outside: the Golden compares
// only the video content.
func goldenMask(shell *wrkit.ShellChrome, scale float64) []wrevidence.Rect {
	return []wrevidence.Rect{{
		X: (shell.Body.X + 1) * scale, Y: (shell.Body.Y + 1) * scale,
		W: (shell.Body.W - 2) * scale, H: (shell.Body.H - 2) * scale,
	}}
}

// baselineMatches rejects a baseline recorded at a different device scale or
// window size instead of reporting a 100% diff as if the picture changed.
func baselineMatches(basePath string, img image.Image) bool {
	if _, err := os.Stat(basePath); err != nil {
		return false
	}
	base := wrevidence.Load(basePath)
	if base == nil || img == nil {
		return false
	}
	return base.Bounds() == img.Bounds()
}

func dprTag(scale float64) string {
	return strconv.FormatFloat(scale, 'g', -1, 64)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
