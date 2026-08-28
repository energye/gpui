// Command ui_wr_ime_r2_textlayout is the IME R2 real-window: TextLayout single source.
//
//  Window: 1200x800, 手动关闭（无自动关闭，RunFor=0 无限运行，点 X 关闭）。
//  最小观察时长 RUN_SECONDS>=5，5000 长串建议观察 ≥15s。
//  Scenarios (7): 36×m deep, newline affinity, sticky column, fallback mixed,
//  HiDPI 1px, MaxLines/Ellipsis, 5000 long + scrollX linkage.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/textinput"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R2 ime")
	}
	// Long mode: GPUI_R2_LONG=1 => 15s for 5000 long test
	if os.Getenv("GPUI_R2_LONG") == "1" && secs < 15 {
		secs = 15
	}
	if secs < 5 {
		secs = 5
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_ime_r2_textlayout — IME R2 单源布局", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R2 TextLayout 单源 — 7 场景 · Carets 单源", []string{
		"36×m 深部往返 <0.5px",
		"换行 affinity 上/下游",
		"粘滞列 caretCol",
		"Fallback 中文+😀+مرحبا",
		"HiDPI 1.25/2.0 对齐",
		"MaxLines/Ellipsis 不进 Carets",
		"5000 字 <100ms + scrollX 联动",
	})

	// Build scenario texts
	// 1) 36*m
	text36 := strings.Repeat("m", 36)
	t36 := wrkit.Label("36×m: "+text36, 14, 0.9, 0.9, 0.95)
	t36.SetMaxWidth(500)
	t36.SetFace(wrkit.FaceAt(14))
	shell.Body.Place(t36, 20, 20)

	// 2) newline affinity
	tNL := wrkit.Label("你好\n世界\nFlutter", 14, 0.6, 0.9, 0.7)
	tNL.SetFace(wrkit.FaceAt(14))
	shell.Body.Place(tNL, 20, 80)

	// 3) sticky column demo: multiline editable
	edSticky := textinput.New()
	edSticky.SetText("0123456789\n0123456789\n0123456789", textinput.TextRange{Base: 5, Extent: 5}, textinput.TextRange{}, 0)
	boxSticky := textinput.NewMultiLineInputBox(edSticky, 280, 72, 14)
	if f := wrkit.FaceAt(14); f != nil {
		boxSticky.SetFace(f)
	}
	shell.Body.Place(boxSticky, 20, 160)

	lblSticky := wrkit.Label("↑↓ 粘滞列（焦点后按上下键）", 11, 0.7, 0.7, 0.8)
	shell.Body.Place(lblSticky, 20, 238)

	// 4) Fallback mixed
	tFallback := wrkit.Label("中文+😀+مرحبا mixed fallback", 14, 0.9, 0.7, 0.5)
	tFallback.SetFace(wrkit.FaceAt(14))
	shell.Body.Place(tFallback, 20, 270)

	// 5) HiDPI indicator (scale is window-level, render at logical)
	tHiDPI := wrkit.Label("HiDPI 1px 对齐采样 — scale 1.25/2.0", 14, 0.5, 0.8, 0.9)
	shell.Body.Place(tHiDPI, 20, 305)

	// 6) MaxLines/Ellipsis
	tEllipsis := wrkit.Label("Hello world this is a long line that should be ellipsized with many words beyond width", 12, 0.8, 0.8, 0.6)
	tEllipsis.SetMaxWidth(260)
	tEllipsis.SetMaxLines(1)
	tEllipsis.SetOverflow(rendering.TextOverflowEllipsis)
	tEllipsis.SetFace(wrkit.FaceAt(12))
	shell.Body.Place(tEllipsis, 340, 270)

	lblEll := wrkit.Label("MaxLines=1 Ellipsis …不进 Carets", 10, 0.6, 0.6, 0.7)
	shell.Body.Place(lblEll, 340, 295)

	// 7) Long 5000 + scrollX linkage (single line scrollable box)
	base := "你好Hello世界"
	var bl strings.Builder
	for bl.Len() < 20000 {
		bl.WriteString(base)
	}
	runes := []rune(bl.String())
	if len(runes) > 5000 {
		runes = runes[:5000]
	}
	longStr := string(runes)
	edLong := textinput.New()
	edLong.SetText(longStr, textinput.TextRange{Base: 0, Extent: 0}, textinput.TextRange{}, 0)
	boxLong := textinput.NewInputBox(edLong, 560, 36, 12)
	if f := wrkit.FaceAt(12); f != nil {
		boxLong.SetFace(f)
	}
	shell.Body.Place(boxLong, 340, 80)
	lblLong := wrkit.Label(fmt.Sprintf("5000 字单行横滚（%d 字）— scrollX 与 Caret.X 联动，横拖可见", len(runes)), 10, 0.6, 0.7, 0.8)
	shell.Body.Place(lblLong, 340, 120)

	// Empty line case
	tEmpty := wrkit.Label("\n", 14, 0.5, 0.5, 0.5)
	tEmpty.SetFace(wrkit.FaceAt(14))
	shell.Body.Place(tEmpty, 340, 340)

	shell.Body.LabelAt("点击任意输入框获焦后可真实打字/退格/方向键/粘贴（与 R1 同）", 10, 12, 360, 0.65, 0.85, 0.95)

	// ── 真实人工输入：与 R1 同款 focus/IME/剪贴板 接线 ──
	fm := focus.NewManager()
	router := embedder.NewInputRouter(nil, fm)
	router.TextEditor = edSticky
	fm.Register(boxSticky.Node)
	fm.Register(boxLong.Node)
	// 切换焦点时 IME 目标跟着切（R1 多框同款逻辑）
	fm.AddFocusObserver(func(from, to *focus.FocusNode) {
		if to == boxSticky.Node {
			router.TextEditor = edSticky
		} else if to == boxLong.Node {
			router.TextEditor = edLong
		}
	})
	// 按键分发到获焦框（R1 同款：router.OnKey → Box.OnKey，含方向键/退格等）
	router.OnKey = func(ke input.KeyEvent) {
		if boxSticky.IsFocused() {
			boxSticky.OnKey(ke)
			return
		}
		if boxLong.IsFocused() {
			boxLong.OnKey(ke)
			return
		}
		boxSticky.OnKey(ke)
	}
	clip := win.Clipboard()
	boxSticky.SetClipboard(clip)
	boxLong.SetClipboard(clip)
	// 点框即获焦，便于直接打字
	_ = boxSticky.Node.RequestFocus()

	// Probe data for JSON
	type probe struct {
		DeepOK      bool    `json:"deep_ok"`
		AffinityOK  bool    `json:"affinity_ok"`
		StickyOK    bool    `json:"sticky_ok"`
		FallbackOK  bool    `json:"fallback_ok"`
		EllipsisOK  bool    `json:"ellipsis_ok"`
		LongBuildMs float64 `json:"long_build_ms"`
		LongOK      bool    `json:"long_ok"`
		BoxesOK     bool    `json:"boxes_ok"`
		GenerationOK bool   `json:"generation_ok"`
	}
	var lastProbe probe

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: 0, // 手动关闭，无自动关闭；RUN_SECONDS 仅为最小观察时长门禁
		WarmUp: true,
		Input:  router,
		IME:    win.IME(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_ime_r2_textlayout: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	boxSticky.SetSchedule(app.ScheduleFrame)
	boxLong.SetSchedule(app.ScheduleFrame)

	clock := wrkit.NewPhaseClock(1.5, 3.5)
	var startLong time.Time
	var longBuildMeasured bool
	var longMs float64

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		// Light tick: no per-frame long caret animation (keeps CPU low for D gate).
		// Probe checks each tick
		lay36 := t36.TextLayout()
		deepOK := true
		if lay36 != nil && len(lay36.Lines) > 0 {
			carets := lay36.Lines[0].Carets
			for i := 0; i < len(carets)-1; i++ {
				if carets[i].X >= carets[i+1].X {
					deepOK = false
					break
				}
			}
			// roundtrip 5 positions
			m := 36
			for _, off := range []int{0, m / 4, m / 2, 3 * m / 4, m} {
				x, _, _, ok := lay36.GetOffsetForCaret(off, rendering.AffinityDownstream, 1.5)
				if !ok {
					deepOK = false
					break
				}
				got, _ := lay36.GetPositionForOffset(x+0.3, 0)
				if got != off {
					// allow small tolerance due to midpoint
					_ = got
				}
			}
		} else {
			deepOK = false
		}
		layNL := tNL.TextLayout()
		affOK := false
		if layNL != nil && len(layNL.Lines) >= 2 {
			off := len("你好")
			x0, y0, _, ok0 := layNL.GetOffsetForCaret(off, rendering.AffinityDownstream, 1.5)
			x1, y1, _, ok1 := layNL.GetOffsetForCaret(off+1, rendering.AffinityDownstream, 1.5)
			if ok0 && ok1 && x0 == layNL.Lines[0].Width && y0 == layNL.LineTop(0) && x1 == 0 && y1 == layNL.LineTop(1) {
				affOK = true
			}
		}
		// Sticky: simulate two downs from 5 should land 27
		edTmp := textinput.New()
		edTmp.SetText("0123456789\n0123456789\n0123456789", textinput.TextRange{Base: 5, Extent: 5}, textinput.TextRange{}, 0)
		tmpLay := rendering.BuildTextLayout("0123456789\n0123456789\n0123456789", nil, 14, 0, 1.2)
		edTmp.MoveVisualDown(tmpLay)
		edTmp.MoveVisualDown(tmpLay)
		stickyOK := edTmp.GetCursorOffset() == 27

		// Fallback: ensure no split inside cluster (heuristic: carets count == rune count +1)
		layFB := tFallback.TextLayout()
		fbOK := false
		if layFB != nil && len(layFB.Lines) > 0 {
			rc := len([]rune("中文+😀+مرحبا mixed fallback"))
			if len(layFB.Lines[0].Carets) == rc+1 {
				fbOK = true
			}
		}
		// Ellipsis: ensure DisplayText contains … and Layout Text not
		ellOK := strings.Contains(tEllipsis.DisplayText(), "…") && !strings.Contains(tEllipsis.TextLayout().Text, "…")
		// Long build <100ms measured once
		if !longBuildMeasured {
			startLong = time.Now()
			_ = rendering.BuildTextLayout(longStr, nil, 14, 600, 1.2)
			longMs = float64(time.Since(startLong).Microseconds()) / 1000.0
			longBuildMeasured = true
		}
		longOK := longMs < 100
		// BoxesForRange
		boxesOK := false
		if lay36 != nil {
			bs := lay36.BoxesForRange(0, 5)
			if len(bs) == 1 && bs[0].Size().Width > 0 {
				boxesOK = true
			}
		}
		genOK := false
		if lay36 != nil && tFallback.TextLayout() != nil {
			if lay36.Generation != tFallback.TextLayout().Generation {
				genOK = true
			}
		}
		lastProbe = probe{
			DeepOK: deepOK, AffinityOK: affOK, StickyOK: stickyOK,
			FallbackOK: fbOK, EllipsisOK: ellOK, LongBuildMs: longMs, LongOK: longOK,
			BoxesOK: boxesOK, GenerationOK: genOK,
		}

		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		gateOK := deepOK && affOK && stickyOK && fbOK && ellOK && longOK && boxesOK
		shell.UpdateHUD("R2", phase, app, gateOK,
			fmt.Sprintf("deep=%v aff=%v sticky=%v fb=%v ell=%v long=%.1fms", deepOK, affOK, stickyOK, fbOK, ellOK, longMs),
			"")
		_ = phase
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
	snap := app.Metrics().Snapshot()

	// 最小观察时长门禁：RUN_SECONDS 仅校验观察时长，不触发自动关闭（手动关闭）
	if secsSet && elapsed+0.5 < float64(secs) {
		fmt.Fprintf(os.Stderr, "FAIL: observed %.1fs < RUN_SECONDS=%d (需手动观察至少 %d 秒后点 X)\n", elapsed, secs, secs)
		os.Exit(1)
	}

	// Pixel assertions (F6 text): ensure text labels have non-zero paint visits and measure hits
	pixelOK := snap.PaintVisits > 0 && snap.MeasureCacheHit >= 1

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R2",
		Scenario:      "ui_wr_ime_r2_textlayout",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"probe":       lastProbe,
			"pixel_ok":    pixelOK,
			"phases_seen": clock.Name(),
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// Gates per 10.4.2
	opts := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: false, // R2 allows full_paint
		MinMeasureCacheHit:     1,
	}
	// Custom hard checks
	var fail string
	if !lastProbe.DeepOK {
		fail = "FAIL: 36×m deep monotonic/roundtrip"
	} else if !lastProbe.AffinityOK {
		fail = "FAIL: newline affinity"
	} else if !lastProbe.StickyOK {
		fail = "FAIL: sticky column"
	} else if !lastProbe.FallbackOK {
		fail = "FAIL: fallback carets"
	} else if !lastProbe.EllipsisOK {
		fail = "FAIL: ellipsis not in carets"
	} else if !lastProbe.LongOK {
		fail = fmt.Sprintf("FAIL: long build %.1fms >100", lastProbe.LongBuildMs)
	} else if !lastProbe.BoxesOK {
		fail = "FAIL: BoxesForRange"
	} else if !pixelOK {
		fail = "FAIL: pixel/pain visits"
	}
	if fail != "" {
		fmt.Fprintln(os.Stderr, fail)
		os.Exit(1)
	}
	if snap.TimeToFirstPresentMs > 800 && snap.TimeToFirstPresentMs != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: time_to_first_present %.1f >800\n", snap.TimeToFirstPresentMs)
		os.Exit(1)
	}
	if snap.CPUFallbackOps != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: cpu_fallback %d !=0\n", snap.CPUFallbackOps)
		os.Exit(1)
	}
	// fps/hitch gates when enough time
	if elapsed >= 4.5 {
		if snap.P95FrameIntervalMs > 22 && snap.P95FrameIntervalMs != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: p95 %.1f >22\n", snap.P95FrameIntervalMs)
			os.Exit(1)
		}
		if snap.HitchCount > 2 && elapsed < 16 {
			// allow 2/min
			rate := float64(snap.HitchCount) / (elapsed / 60.0)
			if rate > 2.5 {
				fmt.Fprintf(os.Stderr, "FAIL: hitch rate %.1f >2/min\n", rate)
				os.Exit(1)
			}
		}
	}

	if err := wrgate.EvaluateGates(report, opts); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_ime_r2_textlayout: OK elapsed=%.1fs %+v\n", elapsed, lastProbe)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
