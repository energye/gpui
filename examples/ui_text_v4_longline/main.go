// Command ui_text_v4_longline is the V4 real-window: one 5,000,000-byte
// logical line on the D1 truncation window (ENGINE_TEXT_LARGE_SCALE_PLAN §V4).
//
//	# gate (isolated display only, standard close):
//	DISPLAY=:3 go run ./examples/ui_text_v4_longline -auto-only
//	# manual window: stays open until close, logs real keys
//	DISPLAY=:3 go run ./examples/ui_text_v4_longline -manual-seconds 60
//
// The body is one production textinput multi-line box; it only uses public
// component APIs. Verification only: no engine behaviour lives here.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
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
	winW, winH = 1200, 800
	lineBytes  = 5000000
	goldenRel  = "examples/ui_text_v4_longline/testdata/v4_longline_golden.png"
)

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool { t.on(dt); return true }

// buildLine builds one logical line from the V4 testdata seed (no hardcoded
// standard data in the example).
func buildLine(n int) string {
	b, err := os.ReadFile("ui/rendering/testdata/v4_longline_seed.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: v4 seed:", err)
		os.Exit(1)
	}
	seed := strings.TrimRight(string(b), "\r\n")
	if seed == "" {
		fmt.Fprintln(os.Stderr, "FAIL: v4 seed is empty")
		os.Exit(1)
	}
	var sb strings.Builder
	sb.Grow(n + len(seed))
	for sb.Len() < n {
		sb.WriteString(seed)
	}
	return sb.String()[:n]
}

// inkPixels counts green text pixels in the body content rect of a GPU
// readback snapshot (pixel probe; the Golden comparison is the exact gate).
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

type v4Check struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted V4 self-check + GPU snapshot + Golden; exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "text_v4_longline")
	} else if *autoOnly {
		secs = 15
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	} else {
		secs = 0 // manual: stay until window close
	}
	wrkit.EnsureUIFace()
	text := buildLine(lineBytes)
	textBytes := len(text)

	shell := wrkit.NewShell(winW, winH, "V4 5e6 字单行 — D1 分块窗口", []string{
		"V4: 单行 5,000,000 字节",
		"D1 窗口：默认上限 10000 字节",
		"横向视口跟随光标滑动",
		"长行窗口只布局可见段",
		"复制/全选仍为完整文本",
	})

	ed := textinput.New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(text)
	text = ""
	ed.SetSelection(textinput.TextRange{Base: 0, Extent: 0})
	box := textinput.NewMultiLineInputBox(ed, shell.Body.W, shell.Body.H, 14)
	if face := wrkit.FaceAt(14); face != nil {
		box.SetFace(face)
	}
	shell.Body.Place(box, 0, 0)
	box.Sync()

	winTitle := "gpui ui_text_v4_longline — D1 window"
	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open:", err)
		os.Exit(1)
	}
	host := win.Host()

	var proc scheduler.ProcessTracker
	proc.Start()

	snapshotPath := os.Getenv("GPUI_V4_SNAPSHOT")
	if *autoOnly {
		snapshotPath = filepath.Join(os.TempDir(), "v4_longline_snapshot.png")
	}

	var checks []v4Check
	selfCheckDone := false
	var windowStart int
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
			switch ev.Type {
			case platform.EventCloseRequested, platform.EventClose:
				closeRequested = true
				fmt.Fprintln(os.Stderr, "v4: close requested (standard close path)")
			case platform.EventKey:
				if ev.Pressed {
					manualKeys++
					fmt.Fprintf(os.Stderr, "v4: real key code=%#x pressed (total=%d)\n", ev.KeyCode, manualKeys)
					win.Controls().SetTitle(fmt.Sprintf("%s — keys=%d", winTitle, manualKeys))
				}
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		// V4 gate: deterministic self-check at ~2s, public APIs only.
		if *autoOnly && !selfCheckDone && elapsed >= 2 {
			selfCheckDone = true
			s0, e0, full0, tr0 := box.LongLineWindow()
			checks = append(checks, v4Check{"d1_window", tr0 && full0 == textBytes && e0-s0 > 0 && e0-s0 <= 10000,
				fmt.Sprintf("window=(%d,%d) full=%d truncated=%v", s0, e0, full0, tr0)})
			checks = append(checks, v4Check{"not_virtual", !box.VirtualMode(),
				fmt.Sprintf("VirtualMode()=%v", box.VirtualMode())})
			// Slide the window by moving the caret to the far end (the
			// horizontal viewport follows the caret).
			ed.SetSelectionBytes(full0/2, full0/2)
			box.Sync()
			s1, _, _, _ := box.LongLineWindow()
			windowStart = s1
			checks = append(checks, v4Check{"horizontal_viewport", s1 > s0,
				fmt.Sprintf("window start %d -> %d", s0, s1)})
		}
		shell.NoteHUDTick(dt)
		// V4 never enters V2 virtual mode by design; the HUD flag here means
		// "capability active", not a gate verdict (the gate is in the JSON).
		shell.UpdateHUD("V4", wrkit.PhaseSteady, app, true,
			fmt.Sprintf("win=%d", windowStart),
			fmt.Sprintf("virtual=%v", box.VirtualMode()))
		app.ScheduleFrame()
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
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	gatePass, gateErr := true, ""
	var pixelInk int
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenSeeded bool
	if *autoOnly {
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
		AbilityID:     "text_v4_longline",
		Scenario:      "ui_text_v4_longline",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"line_bytes":       int64(textBytes),
			"v4_checks":        checks,
			"v4_window_start":  windowStart,
			"v4_pixel_ink":     pixelInk,
			"v4_golden_diff":   goldenDiffPct,
			"v4_golden_px":     goldenTotalPx,
			"v4_golden_seed":   goldenSeeded,
			"v4_manual_keys":   manualKeys,
			"v4_close_req":     closeRequested,
			"v4_gate_pass":     gatePass,
			"v4_gate_err":      gateErr,
			"v4_evidence_mode": map[bool]string{true: "auto_only", false: "manual"}[*autoOnly],
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
	if *autoOnly && !gatePass {
		fmt.Fprintln(os.Stderr, "FAIL: V4 gate:", gateErr)
		os.Exit(1)
	}
}

// elapsed is advanced by the frame ticker (single-threaded event loop).
var elapsed float64
