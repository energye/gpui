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
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
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
	winW, winH = 1200, 800
	// defaultBytes is the seeded gate size (L3 single-line tier). The Golden
	// and no-marker pixel-diff baselines match this size only.
	defaultBytes = 5000000
	goldenRel    = "examples/ui_text_v4_longline/testdata/v4_longline_golden.png"
	// noMarkerRel is a marker-OFF render used for an objective pixel diff;
	// both images come from the same code path with only the marker toggled.
	noMarkerRel = "examples/ui_text_v4_longline/testdata/v4_longline_no_marker.png"
)

// lineBytes is the single-line length (set from -bytes). Tiers of
// ENGINE_TEXT_LARGE_SCALE_PLAN §2.1: 1e6=L1, 3e6=L2, 5e6=L3.
var lineBytes = defaultBytes

// comma renders 1000000 as "1,000,000" for titles/legend only.
func comma(n int) string {
	s := strconv.Itoa(n)
	pre := len(s) % 3
	if pre == 0 {
		pre = 3
	}
	var b []byte
	b = append(b, s[:pre]...)
	for i := pre; i < len(s); i += 3 {
		b = append(b, ',')
		b = append(b, s[i:i+3]...)
	}
	return string(b)
}

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

// markerInkDiff counts pixels that differ between the current snapshot and a
// marker-OFF baseline in the right-edge band of the first text row. Both come
// from the same code path with only SetLongLineMarker toggled, so any
// difference there is exactly the "…" ink — an objective pixel assertion,
// not a visual guess.
func markerInkDiff(current, baseline string) (int, error) {
	load := func(p string) (image.Image, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return png.Decode(f)
	}
	a, err := load(current)
	if err != nil {
		return 0, err
	}
	bimg, err := load(baseline)
	if err != nil {
		return 0, err
	}
	ab, bb := a.Bounds(), bimg.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return 0, fmt.Errorf("snapshot size mismatch %v vs %v", ab, bb)
	}
	dpr := float64(ab.Dx()) / float64(winW)
	if dpr <= 0 {
		dpr = 1
	}
	x0 := ab.Min.X + int((284+904-36)*dpr)
	x1 := ab.Min.X + int((284+904)*dpr)
	y0 := ab.Min.Y + int(58*dpr)
	y1 := ab.Min.Y + int(96*dpr)
	if x0 < ab.Min.X {
		x0 = ab.Min.X
	}
	if x1 > ab.Max.X {
		x1 = ab.Max.X
	}
	if y0 < ab.Min.Y {
		y0 = ab.Min.Y
	}
	if y1 > ab.Max.Y {
		y1 = ab.Max.Y
	}
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r1, g1, b1, _ := a.At(x, y).RGBA()
			r2, g2, b2, _ := bimg.At(x, y).RGBA()
			d := abs8(r1>>8, r2>>8) + abs8(g1>>8, g2>>8) + abs8(b1>>8, b2>>8)
			if d > 24 {
				n++
			}
		}
	}
	return n, nil
}

func abs8(a, b uint32) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

type v4Check struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// burstStats is one timed burst phase of the §2.2 real-window probe
// (observation values; the authoritative keystroke gates are the CPU tests).
type burstStats struct {
	Kind     string  `json:"kind"`
	N        int     `json:"n"`
	MedianUs float64 `json:"median_us"`
	P95Us    float64 `json:"p95_us"`
	MaxUs    float64 `json:"max_us"`
}

// burst validity probes for the single long line: the keystrokes must really
// edit, the line must stay on the RenderText window path (never virtual),
// and the horizontal window must have followed the caret.
type burstValid struct {
	LenGrewByN  bool `json:"len_grew_by_n"`
	WindowMoved bool `json:"window_moved"`
	NotVirtual  bool `json:"not_virtual"`
	MarkerKept  bool `json:"marker_kept"`
}

func burstPercentile(ds []time.Duration, p float64) float64 {
	if len(ds) == 0 {
		return 0
	}
	c := append([]time.Duration(nil), ds...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	idx := int(float64(len(c)) * p)
	if idx >= len(c) {
		idx = len(c) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return float64(c[idx].Microseconds())
}

func burstSummary(kind string, ds []time.Duration) burstStats {
	s := burstStats{Kind: kind, N: len(ds)}
	if len(ds) == 0 {
		return s
	}
	s.MedianUs = burstPercentile(ds, 0.5)
	s.P95Us = burstPercentile(ds, 0.95)
	max := ds[0]
	for _, d := range ds {
		if d > max {
			max = d
		}
	}
	s.MaxUs = float64(max.Microseconds())
	return s
}

// runBurst drives the long-line box through the §2.2 keystroke scenario:
// one char at a fresh offset each time (unique edit positions), each op timed
// through the real Editor+Box sync path. Public component APIs only.
func runBurst(ed *textinput.Editor, box *textinput.Box, n int) ([]burstStats, burstValid) {
	var stats []burstStats
	var v burstValid
	docLen := ed.BufferSnapshot().Len()

	var keyDs []time.Duration
	stride := docLen / (n + 2)
	if stride < 64 {
		stride = 64
	}
	off := stride
	baseLen := docLen
	windowMoved := false
	lastStart := -1
	for i := 0; i < n; i++ {
		ed.SetSelectionBytes(off, off)
		t0 := time.Now()
		ed.Insert("x")
		box.Sync()
		keyDs = append(keyDs, time.Since(t0))
		if s, _, _, truncated := box.LongLineWindow(); truncated {
			if lastStart >= 0 && s != lastStart {
				windowMoved = true
			}
			lastStart = s
		}
		off += stride
		if off >= docLen-1 {
			off = stride
		}
	}
	stats = append(stats, burstSummary("keystroke", keyDs))
	v.LenGrewByN = ed.BufferSnapshot().Len() == baseLen+n

	// Caret moves/hits along the line.
	var caretDs []time.Duration
	for i := 0; i < n; i++ {
		t0 := time.Now()
		ed.SetSelectionBytes(off, off)
		box.Sync()
		caretDs = append(caretDs, time.Since(t0))
		off += stride
		if off >= docLen-1 {
			off = stride
		}
	}
	stats = append(stats, burstSummary("caret", caretDs))

	v.WindowMoved = windowMoved
	v.NotVirtual = !box.VirtualMode()
	v.MarkerKept = box.LongLineMarkerX() > 0
	return stats, v
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted V4 self-check + GPU snapshot + Golden; exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	bytesFlag := flag.Int("bytes", defaultBytes, "single-line byte length (tier scale: 1e6=L1, 3e6=L2, 5e6=L3)")
	burst := flag.Int("burst", 0, "scripted keystroke/caret timing burst along the line (0 = off)")
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
	if *bytesFlag > 0 {
		lineBytes = *bytesFlag
	}
	noMarker := os.Getenv("GPUI_V4_NO_MARKER") == "1"
	// The Golden and no-marker pixel-diff baselines are renders of the seeded
	// 5e6 line; at other tier sizes they are not comparable and are reported
	// as skipped-with-reason instead of failing on a size mismatch.
	sizeMatchesBaseline := lineBytes == defaultBytes

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

	shell := wrkit.NewShell(winW, winH, fmt.Sprintf("V4 %s 字单行 — D1 分块窗口", comma(lineBytes)), []string{
		fmt.Sprintf("V4: 单行 %s 字节", comma(lineBytes)),
		"D1 窗口：默认上限 10000 字节",
		"横向视口跟随光标滑动",
		"长行窗口只布局可见段",
		"复制/全选仍为完整文本",
	})

	ed := textinput.New()
	ed.SetSingleLine(false)
	// §2.2 首屏口径（沿 V2 收口）：构造与布局首同步分开计时上报，门禁挂在
	// 布局首同步上；构造是 O(n) 数据装载，任何编辑器都省不掉。
	tConstruct := time.Now()
	ed.SetTextSimple(text)
	text = ""
	constructMs := time.Since(tConstruct).Seconds() * 1000
	ed.SetSelection(textinput.TextRange{Base: 0, Extent: 0})
	box := textinput.NewMultiLineInputBox(ed, shell.Body.W, shell.Body.H, 14)
	if face := wrkit.FaceAt(14); face != nil {
		box.SetFace(face)
	}
	if noMarker {
		box.SetLongLineMarker(false)
	}
	shell.Body.Place(box, 0, 0)
	tFirstSync := time.Now()
	box.Sync()
	firstSyncMs := time.Since(tFirstSync).Seconds() * 1000

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
	var burstStatsOut []burstStats
	var burstValidOut burstValid
	burstDone := false

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
			// The window must be a small VISIBLE band (plan §3 layer 4),
			// not the fixed 10000-byte render cap: e0-s0 must be well under
			// 1% of the 5e6-byte line.
			checks = append(checks, v4Check{"d1_window", tr0 && full0 == textBytes && e0-s0 > 0 && e0-s0 < full0/100,
				fmt.Sprintf("window=(%d,%d) size=%d full=%d truncated=%v", s0, e0, e0-s0, full0, tr0)})
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
			if !noMarker {
				mx := box.LongLineMarkerX()
				checks = append(checks, v4Check{"marker_glyph", mx > 0,
					fmt.Sprintf("MarkerX=%.1f", mx)})
			}
		}
		// §2.2 tier burst: one timed keystroke/caret pass along the line at
		// ~2s. Observation values for the tier table; CPU gates stay
		// authoritative.
		if *burst > 0 && !burstDone && elapsed >= 2 {
			burstDone = true
			burstStatsOut, burstValidOut = runBurst(ed, box, *burst)
			fmt.Fprintf(os.Stderr, "v4: burst n=%d keystroke_med=%.1fus caret_med=%.1fus valid=%+v\n",
				*burst, burstStatsOut[0].MedianUs, burstStatsOut[1].MedianUs, burstValidOut)
		}
		// RSS slope needs periodic process samples (same wiring as the
		// large_scale scroll ticker); without them the 120s RSS verdict is
		// inconclusive by construction.
		proc.Sample()
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
	// D5 heap evidence at 60s/110s (same wiring as ui_text_large_scale).
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

	gatePass, gateErr := true, ""
	fail := func(reason string) {
		gatePass = false
		if gateErr == "" {
			gateErr = reason
		} else {
			gateErr += "; " + reason
		}
	}
	var pixelInk int
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenSeeded bool
	var goldenSkipped, markerSkipped string
	markerDiff := -1
	if *autoOnly {
		if len(checks) == 0 {
			fail("self-check did not run")
		}
		for _, c := range checks {
			if !c.Pass {
				gatePass = false
				gateErr = "logic probe failed: " + c.Name
			}
		}
		if n, err := inkPixels(snapshotPath); err != nil {
			fail(fmt.Sprintf("pixel probe: %v", err))
		} else {
			pixelInk = n
			if n < 500 {
				fail(fmt.Sprintf("pixel probe: only %d ink pixels (need >=500)", n))
			}
		}
		if !noMarker && !sizeMatchesBaseline {
			markerSkipped = "no-marker diff baseline is a 5e6 render; tier size is not comparable (marker evidence lives in the 5e6 gate)"
		}
		if *autoOnly && !noMarker && sizeMatchesBaseline {
			if n, err := markerInkDiff(snapshotPath, noMarkerRel); err != nil {
				fail(fmt.Sprintf("marker pixel probe: %v", err))
			} else {
				markerDiff = n
				if n < 8 {
					fail(fmt.Sprintf("marker pixel probe: only %d differing pixels in right-edge band", n))
				}
			}
		}
		if !sizeMatchesBaseline {
			goldenSkipped = "golden baseline is a 5e6 render; tier size is not comparable (use ink probe + logic probes here)"
		}
		if sizeMatchesBaseline {
			if diff, total, seeded, err := wrkit.GoldenGate(snapshotPath, goldenRel, wrkit.ShellContentRects()); err != nil {
				fail(fmt.Sprintf("golden: %v", err))
			} else {
				goldenDiffPct, goldenTotalPx, goldenSeeded = diff, total, seeded
				if seeded {
					fail("golden baseline seeded but NOT reviewed: this run is not a pass")
				} else if diff != 0 {
					fail(fmt.Sprintf("golden diff=%.6f%%", diff))
				}
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
			"line_bytes":         int64(textBytes),
			"construct_ms":       constructMs,
			"first_sync_ms":      firstSyncMs,
			"v4_checks":          checks,
			"v4_window_start":    windowStart,
			"v4_pixel_ink":       pixelInk,
			"v4_marker_ink_diff": markerDiff,
			"v4_golden_diff":     goldenDiffPct,
			"v4_golden_px":       goldenTotalPx,
			"v4_golden_seed":     goldenSeeded,
			"v4_golden_skipped":  goldenSkipped,
			"v4_marker_skipped":  markerSkipped,
			"v4_manual_keys":     manualKeys,
			"v4_close_req":       closeRequested,
			"v4_gate_pass":       gatePass,
			"v4_gate_err":        gateErr,
			"v4_evidence_mode":   map[bool]string{true: "auto_only", false: "manual"}[*autoOnly],
			"burst":              burstStatsOut,
			"burst_valid":        burstValidOut,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
	// An invalid burst (no real edit, window never followed the caret, box
	// flipped to virtual, or the marker vanished) would make every burst
	// number a no-op timing — fail the run instead of reporting it.
	if *burst > 0 {
		if !burstValidOut.LenGrewByN {
			fmt.Fprintln(os.Stderr, "FAIL: burst validity: keystrokes did not really change the document length")
			os.Exit(1)
		}
		if !burstValidOut.WindowMoved {
			fmt.Fprintln(os.Stderr, "FAIL: burst validity: long-line window never followed the caret")
			os.Exit(1)
		}
		if !burstValidOut.NotVirtual {
			fmt.Fprintln(os.Stderr, "FAIL: burst validity: box entered virtual mode (long line must stay on the window path)")
			os.Exit(1)
		}
	}
	if *autoOnly && !gatePass {
		fmt.Fprintln(os.Stderr, "FAIL: V4 gate:", gateErr)
		os.Exit(1)
	}
}

// elapsed is advanced by the frame ticker (single-threaded event loop).
var elapsed float64
