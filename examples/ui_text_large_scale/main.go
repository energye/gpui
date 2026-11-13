// Command ui_text_large_scale is the V2 real-window: 1e6-line multi-line
// input box on the viewport model (ENGINE_TEXT_LARGE_SCALE_PLAN §V2).
//
//	DISPLAY=:3 RUN_SECONDS=60 go run ./examples/ui_text_large_scale
//
// The body is one production textinput multi-line box; it only uses public
// component APIs. 60s RSS is screening only (O3 requires 120s for a verdict).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/textinput"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	lineCount    = 1000000
	closeSeconds = 30
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

func main() {
	secs, set := wrkit.RunSecondsOpt()
	if !set {
		secs = closeSeconds
	}
	wrkit.EnsureUIFace()
	line := sampleLine()
	text := buildNumberedText(line, lineCount)
	textBytes := len(text)

	shell := wrkit.NewShell(winW, winH, "V2 1e6 行多行框 — 视口行模型", []string{
		"1,000,000 行生产多行输入框",
		"行号 0000001–1000000",
		"首屏/滚动只物化可见行",
		"滚动脚本：慢滚 → 快滑 → 减速",
		"无 GetText、无全量 TextLayout",
	})

	ed := textinput.New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(text)
	text = ""
	runtime.GC()
	ed.SetSelection(textinput.TextRange{Base: 0, Extent: 0})
	box := textinput.NewMultiLineInputBox(ed, shell.Body.W, shell.Body.H, 14)
	if face := wrkit.FaceAt(14); face != nil {
		box.SetFace(face)
	}
	shell.Body.Place(box, 0, 0)
	box.Sync()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_text_large_scale — 1e6 line box", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open:", err)
		os.Exit(1)
	}
	host := win.Host()

	var proc scheduler.ProcessTracker
	proc.Start()

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: os.Getenv("GPUI_V2_SNAPSHOT"),
		OnRunEnd:     proc.Stop,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})

	const cycleLen = 9.0
	var elapsed, dir, acc float64 = 0, 1, 0
	var linesMoved int64
	noScroll := os.Getenv("GPUI_V2_NOSCROLL") == "1"
	retPath := os.Getenv("GPUI_V2_RETAINED_SNAPSHOT")
	retTaken := false
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
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
			"line_count":   int64(lineCount),
			"lines_moved":  linesMoved,
			"scroll_y":     box.ScrollY(),
			"virtual_mode": box.VirtualMode(),
			"text_bytes":   int64(textBytes),
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
}
