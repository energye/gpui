// Command ui_text_large_scale is the V2 real-window: 1e6-line multi-line
// input box on the viewport model (ENGINE_TEXT_LARGE_SCALE_PLAN §V2).
// With -wrap it is also the V3c gate window (soft wrap + anchor + visual-row
// arrows on the same production box).
//
//	# V2/V3c gate (isolated display only, standard close):
//	DISPLAY=:3 RUN_SECONDS=60 go run ./examples/ui_text_large_scale -wrap
//	DISPLAY=:3 go run ./examples/ui_text_large_scale -wrap -auto-only
//	# manual window (stays open until close; logs real keys, title shows state):
//	DISPLAY=:3 go run ./examples/ui_text_large_scale -wrap -manual-seconds 60
//
// The body is one production textinput multi-line box; it only uses public
// component APIs. -wrap enables V3c soft wrap through Box.SetWrap.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/textinput"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	lineCount    = 1000000
	closeSeconds = 30
	goldenRel    = "examples/ui_text_large_scale/testdata/v3c_wrap_golden.png"
)

func sampleLine() string {
	path := filepath.Join("examples", "ui_text_large_scale", "testdata", "line_sample.txt")
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return "2026-11-13T00:00:00Z INFO fallback line content\n"
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) == 0 {
		return "2026-11-13T00:00:00Z INFO fallback line content\n"
	}
	return lines[0] + "\n"
}

// buildNumberedText prefixes each sample line with a 7-digit line number so
// scrolling windows can be checked by eye against the expected range.
func buildNumberedText(line string, count int) string {
	var b strings.Builder
	b.Grow(count * (len(line) + 8))
	num := make([]byte, 0, 8)
	for i := 0; i < count; i++ {
		num = num[:0]
		num = strconv.AppendInt(num, int64(i+1), 10)
		for k := len(num); k < 7; k++ {
			b.WriteByte('0')
		}
		b.Write(num)
		b.WriteByte(' ')
		b.WriteString(line)
	}
	return b.String()
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool { t.on(dt); return true }

// v3cCheck is one public-API logic probe for the V3c gate.
type v3cCheck struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// inkPixels counts green text pixels in the body content rect of a GPU
// readback snapshot. A window that renders nothing cannot pass on logic
// probes alone; this is the pixel half of the three-evidence gate.
func inkPixels(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return 0, err
	}
	b := img.Bounds()
	dpr := float64(b.Dx()) / float64(winW)
	if dpr <= 0 {
		dpr = 1
	}
	// body rect from wrkit.ShellContentRects(): x=284,y=60,w=904,h=656.
	x0, y0 := b.Min.X+int(284*dpr), b.Min.Y+int(60*dpr)
	x1, y1 := b.Min.X+int((284+904)*dpr), b.Min.Y+int((60+656)*dpr)
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	count := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			rr, gg, bb := int(r>>8), int(g>>8), int(bl>>8)
			if gg > rr+25 && gg > bb+15 {
				count++
			}
		}
	}
	return count, nil
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted V3c self-check + GPU snapshot + Golden; exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	wrap := flag.Bool("wrap", false, "V3c: enable soft wrap on the 1e6-line production box")
	cpuprofile := flag.String("cpuprofile", "", "write a CPU profile of the run to this path (verification only)")
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: cpuprofile:", err)
			os.Exit(1)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: start cpuprofile:", err)
			os.Exit(1)
		}
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "text_large_scale")
	} else if *autoOnly {
		secs = 15
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	} else {
		secs = 0 // manual: stay until window close
	}
	wrkit.EnsureUIFace()
	line := sampleLine()
	if *wrap {
		// V3c: each logical line must be wider than the body, otherwise soft
		// wrap produces one visual row per line and the gate is a false green.
		// Repeat the seed line so a numbered line is ~2x the body width.
		one := strings.TrimRight(line, "\n")
		line = one + " " + one + "\n"
	}
	text := buildNumberedText(line, lineCount)
	textBytes := len(text)

	legend := []string{
		"1,000,000 行生产多行输入框",
		"行号 0000001–1000000",
		"首屏/滚动只物化可见行",
		"滚动脚本：慢滚 → 快滑 → 减速",
		"无 GetText、无全量 TextLayout",
	}
	if *wrap {
		legend = append(legend, "V3c: 软回绕开启（按宽度重建视图）")
	}
	shell := wrkit.NewShell(winW, winH, "V2 1e6 行多行框 — 视口行模型", legend)

	ed := textinput.New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(text)
	text = ""
	runtime.GC()
	ed.SetSelection(textinput.TextRange{Base: 0, Extent: 0})
	box := textinput.NewMultiLineInputBox(ed, shell.Body.W, shell.Body.H, 14)
	if *wrap {
		box.SetWrap(true)
	}
	if face := wrkit.FaceAt(14); face != nil {
		box.SetFace(face)
	}
	shell.Body.Place(box, 0, 0)
	box.Sync()

	winTitle := "gpui ui_text_large_scale — 1e6 line box"
	if *wrap {
		winTitle += " [V3c wrap]"
	}
	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open:", err)
		os.Exit(1)
	}
	host := win.Host()

	var proc scheduler.ProcessTracker
	proc.Start()

	snapshotPath := os.Getenv("GPUI_V2_SNAPSHOT")
	if *autoOnly {
		snapshotPath = filepath.Join(os.TempDir(), "v3c_wrap_snapshot.png")
	}

	var manualKeys int
	closeRequested := false
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapshotPath,
		OnRunEnd:     proc.Stop,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
			if *autoOnly {
				return
			}
			// Manual window: real events are logged and the title shows the
			// live state, so a human can confirm the wrap effect by eye.
			switch ev.Type {
			case platform.EventCloseRequested, platform.EventClose:
				closeRequested = true
				fmt.Fprintln(os.Stderr, "v3c: close requested (standard close path)")
			case platform.EventKey:
				if ev.Pressed {
					manualKeys++
					fmt.Fprintf(os.Stderr, "v3c: real key code=%#x pressed (total=%d)\n", ev.KeyCode, manualKeys)
					win.Controls().SetTitle(fmt.Sprintf("%s — keys=%d", winTitle, manualKeys))
				}
			}
		},
	})

	const cycleLen = 9.0
	var elapsed, dir, acc float64 = 0, 1, 0
	var linesMoved int64
	// The gate's scripted self-check needs a static viewport: the scrolling
	// ticker moves the caret and scrollY every frame and would scramble the
	// anchor probe. The 60s RUN_SECONDS window separately covers scroll perf.
	noScroll := os.Getenv("GPUI_V2_NOSCROLL") == "1" || *autoOnly
	retPath := os.Getenv("GPUI_V2_RETAINED_SNAPSHOT")
	retTaken := false
	var checks []v3cCheck
	selfCheckDone := false
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		// V3c gate: one deterministic scripted self-check at ~2s, using only
		// public component APIs. The exact anchor contract is locked by the
		// CPU unit test; here we assert the view did not teleport.
		if *autoOnly && *wrap && !selfCheckDone && elapsed >= 2 {
			selfCheckDone = true
			checks = append(checks, v3cCheck{"wrap_enabled", box.Wrap(), fmt.Sprintf("Wrap()=%v", box.Wrap())})
			checks = append(checks, v3cCheck{"virtual_mode", box.VirtualMode(), fmt.Sprintf("VirtualMode()=%v", box.VirtualMode())})
			// Anchor contract = editing INSIDE the visible area must not move
			// the viewport. Park the caret at the top of the current window;
			// jumping to the document middle would legitimately make
			// ensureVisible follow the caret, which is not a jump.
			ed.SetSelection(textinput.TextRange{Base: 0, Extent: 0})
			box.Sync()
			cursorBefore := ed.GetCursorOffset()
			topBefore := box.TopVisibleLine()
			box.OnKey(input.KeyEvent{Key: input.KeyArrowDown, Pressed: true})
			afterDown := ed.GetCursorOffset()
			// A wrapped line steps into the next visual row of the SAME
			// logical line (delta < len(line)); hopping a whole line would
			// mean wrap never produced a second visual row (false green).
			checks = append(checks, v3cCheck{"visual_row_down",
				afterDown > cursorBefore && afterDown-cursorBefore < len(line),
				fmt.Sprintf("caret %d -> %d (+%d, logical line len=%d)", cursorBefore, afterDown, afterDown-cursorBefore, len(line))})
			ed.Insert("\n")
			box.Sync()
			// Objective anchor probe: the logical line at the top of the
			// viewport must be unchanged. Raw scrollY is NOT a valid judge
			// (rebasing the height map legitimately changes its value).
			topAfter := box.TopVisibleLine()
			checks = append(checks, v3cCheck{"anchor_top_line_stable", topAfter == topBefore,
				fmt.Sprintf("top logical line %d -> %d", topBefore, topAfter)})
			checks = append(checks, v3cCheck{"virtual_after_edit", box.VirtualMode(), fmt.Sprintf("VirtualMode()=%v", box.VirtualMode())})
		}
		if !retTaken && retPath != "" && elapsed >= 2 {
			retTaken = true
			go app.RetainedSnapshot(retPath)
		}
		if noScroll {
			shell.NoteHUDTick(dt)
			shell.UpdateHUD("V2", wrkit.PhaseSteady, app, box.VirtualMode(),
				fmt.Sprintf("scroll=%.0f", box.ScrollY()),
				fmt.Sprintf("virtual=%v", box.VirtualMode()))
			app.ScheduleFrame()
			return
		}
		ct := elapsed
		for ct >= cycleLen {
			ct -= cycleLen
		}
		var speed float64
		phase := wrkit.PhaseSteady
		switch {
		case ct < 3:
			speed, phase = 200, wrkit.PhaseSteady
		case ct < 6:
			speed, phase = 4000, wrkit.PhaseSpike
		default:
			speed, phase = 4000*(1-(ct-6)/3), wrkit.PhaseRecover
			if speed < 50 {
				speed = 50
			}
		}
		acc += speed * dt
		n := int(acc)
		acc -= float64(n)
		moved := 0
		// One caret move fires OnChange -> Box.sync -> BuildTextLayout.
		// Without a batch, a 4000-line jump does 4000 full line layouts per
		// frame; BeginBatchEdit/EndBatchEdit collapse that into one.
		ed.BeginBatchEdit()
		for i := 0; i < n; i++ {
			var ok bool
			if dir > 0 {
				ok = ed.MoveCursorDown()
			} else {
				ok = ed.MoveCursorUp()
			}
			if !ok {
				dir = -dir
				break
			}
			moved++
		}
		ed.EndBatchEdit()
		if moved > 0 {
			linesMoved += int64(moved)
			box.Sync()
		}
		proc.Sample()
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("V2", phase, app, box.VirtualMode(),
			fmt.Sprintf("lines=%d scroll=%.0f", linesMoved, box.ScrollY()),
			fmt.Sprintf("virtual=%v", box.VirtualMode()))
		app.ScheduleFrame()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	stopProf := wrkit.StartHeapProfiles(os.Getenv("GPUI_HEAP_PROF_DIR"), 60, 110)
	defer stopProf()
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	// V3c three-evidence gate (auto-only): logic probes + pixel probe + Golden.
	// A seeded baseline is explicitly NOT a pass (wrkit.GoldenGate contract).
	gatePass, gateErr := true, ""
	var pixelInk int
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenSeeded bool
	if *autoOnly {
		if !*wrap {
			gatePass, gateErr = false, "-auto-only requires -wrap (V3c gate)"
		}
		if len(checks) == 0 {
			gatePass, gateErr = false, "self-check did not run"
		}
		for _, c := range checks {
			if !c.Pass {
				gatePass = false
				gateErr = "logic probe failed: " + c.Name
			}
		}
		if n, err := inkPixels(snapshotPath); err != nil {
			gatePass, gateErr = false, fmt.Sprintf("pixel probe: %v", err)
		} else {
			pixelInk = n
			if n < 500 {
				gatePass, gateErr = false, fmt.Sprintf("pixel probe: only %d ink pixels (need >=500)", n)
			}
		}
		if diff, total, seeded, err := wrkit.GoldenGate(snapshotPath, goldenRel, wrkit.ShellContentRects()); err != nil {
			gatePass, gateErr = false, fmt.Sprintf("golden: %v", err)
		} else {
			goldenDiffPct, goldenTotalPx, goldenSeeded = diff, total, seeded
			if seeded {
				gatePass, gateErr = false, "golden baseline seeded but NOT reviewed: this run is not a pass"
			} else if diff != 0 {
				gatePass, gateErr = false, fmt.Sprintf("golden diff=%.6f%%", diff)
			}
		}
	}

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "text_large_scale",
		Scenario:      "ui_text_large_scale",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"line_count":        int64(lineCount),
			"lines_moved":       linesMoved,
			"scroll_y":          box.ScrollY(),
			"virtual_mode":      box.VirtualMode(),
			"text_bytes":        int64(textBytes),
			"v3c_wrap":          *wrap,
			"v3c_checks":        checks,
			"v3c_pixel_ink":     pixelInk,
			"v3c_golden_diff":   goldenDiffPct,
			"v3c_golden_px":     goldenTotalPx,
			"v3c_golden_seed":   goldenSeeded,
			"v3c_manual_keys":   manualKeys,
			"v3c_close_req":     closeRequested,
			"v3c_gate_pass":     gatePass,
			"v3c_gate_err":      gateErr,
			"v3c_evidence_mode": map[bool]string{true: "auto_only", false: "manual"}[*autoOnly],
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
	if *autoOnly && !gatePass {
		fmt.Fprintln(os.Stderr, "FAIL: V3c gate:", gateErr)
		os.Exit(1)
	}
}
