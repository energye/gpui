// Command ui_text_edit_accept is the M6 acceptance window (ENGINE_TEXT_SCALE_PLAN §5.6).
//
// Real single-line + multi-line input boxes for layout/paint performance
// acceptance. The boxes themselves are engine controls (ui/textinput); this
// window only places controls, preloads text, and runs probes.
//
// Modes:
//
//	go run ./examples/ui_text_edit_accept            # interactive window
//	GPUI_ACCEPT_SELFTEST=1 go run ./examples/ui_text_edit_accept  # headless probes, no window
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render/text"
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

func testdataPath(name string) string {
	if dir := os.Getenv("GPUI_ACCEPT_TESTDATA"); dir != "" {
		return filepath.Join(dir, name)
	}
	return filepath.Join("examples", "ui_text_edit_accept", "testdata", name)
}

func loadText(name string) string {
	b, err := os.ReadFile(testdataPath(name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "accept: read %s: %v\n", name, err)
		return ""
	}
	return string(b)
}

// liveProbes measures the actual boxes in the window: whether every B/C
// line carries shapable glyphs (bulk branch precondition), the
// layout-internal caret-vs-glyph deviation, and B/C rebuild p99.
// Ink-vs-layout pixel comparison needs frame readback (unavailable here);
// the pixel half stays on the unmeasured list.
func liveProbes(boxB, boxC *textinput.ViewportInputBox, face text.Face, fontSize float64) probes {
	p := probes{PaintMsP99: -1, ScrollFPS: -1}
	bulkTaken := true
	delta := 0.0
	for _, b := range []*textinput.ViewportInputBox{boxB, boxC} {
		lay := b.TextLayout()
		if lay == nil || lay.LineCount() == 0 {
			bulkTaken = false
			continue
		}
		// Routable bulk mirrors RenderText.Paint (M2): single-face bulk
		// (real glyphs + sourced face) or composite batch (per-face
		// partitions all batchable). See lineBulkRoutable below.
		for i := 0; i < lay.LineCount(); i++ {
			glyphs := lay.LineGlyphs(i)
			carets := lay.LineCarets(i)
			if len(glyphs) == 0 || len(carets) == 0 {
				bulkTaken = false
				continue
			}
			if !lineBulkRoutable(lay, i) {
				bulkTaken = false
			}
			if glyphs[0].GID == 0 {
				continue
			}
			last := glyphs[len(glyphs)-1]
			glyphEnd := last.X + last.XAdvance
			caretEnd := carets[len(carets)-1].X
			if dd := caretEnd - glyphEnd; dd < 0 {
				dd = -dd
				if dd > delta {
					delta = dd
				}
			} else if dd > delta {
				delta = dd
			}
		}
	}
	p.CaretVsPaintMaxDeltaPx = delta
	p.BulkTaken = bulkTaken
	// delta 是布局内部一致性(实测值);批量路由状态由 bulk_taken 独立上报.
	for _, tc := range []struct {
		text string
		dst  *float64
	}{
		{bTextOf(boxB), &p.LayoutMsP99},
		{bTextOf(boxC), &p.KeystrokeMsP99},
	} {
		var ds []float64
		for i := 0; i < 31; i++ {
			t0 := time.Now()
			_ = rendering.BuildTextLayout(tc.text+"x", face, fontSize, 0, 1.25)
			ds = append(ds, float64(time.Since(t0).Microseconds())/1000)
		}
		*tc.dst = p99(ds)
	}
	p.Unmeasured = []string{"paint_ms_p99", "scroll_fps", "interval_p95_ms", "fps_interval", "hitch_rate_per_min", "rss_slope_kb_per_min", "pixel_ink_vs_layout"}
	return p
}

func bTextOf(b *textinput.ViewportInputBox) string {
	if b == nil || b.Editor() == nil {
		return ""
	}
	return b.Editor().GetText()
}

// lineBulkRoutable mirrors RenderText.Paint routing (example-side
// observability only): single-face bulk, or M2 composite batch with every
// partition carrying a sourced face. Must stay in sync with
// RenderText.paintCompositeRuns acceptance.
func lineBulkRoutable(lay *rendering.TextLayout, row int) bool {
	glyphs := lay.LineGlyphs(row)
	if len(glyphs) > 0 && glyphs[0].GID != 0 && lay.Face != nil && lay.Face.Source() != nil {
		return true
	}
	runs := lay.LineGlyphRuns(row)
	if len(runs) == 0 {
		return false
	}
	for _, r := range runs {
		if r.Start < 0 || r.End > len(glyphs) || r.Start >= r.End {
			return false
		}
		part := glyphs[r.Start:r.End]
		if len(part) == 0 || part[0].GID == 0 || r.Face == nil || r.Face.Source() == nil {
			return false
		}
	}
	return true
}

type probes struct {
	CaretVsPaintMaxDeltaPx float64  `json:"caret_vs_paint_max_delta_px"`
	BulkTaken              bool     `json:"bulk_taken"`
	LayoutMsP99            float64  `json:"layout_ms_p99"`
	KeystrokeMsP99         float64  `json:"keystroke_p99_ms"`
	PaintMsP99             float64  `json:"paint_ms_p99"`
	ScrollFPS              float64  `json:"scroll_fps"`
	Unmeasured             []string `json:"unmeasured"`
}

func p99(ds []float64) float64 {
	if len(ds) == 0 {
		return -1
	}
	cp := append([]float64(nil), ds...)
	sort.Float64s(cp)
	i := int(math.Ceil(0.99*float64(len(cp)))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(cp) {
		i = len(cp) - 1
	}
	return cp[i]
}

// computeProbes runs the CPU-side baseline. layoutX comes from the layout
// source of truth (BuildTextLayout float accumulation); snapX replays the
// paint-side per-glyph rounding (drawGlyphs snapPen model). Their gap is the
// constraint-① divergence the M0 fix must close.
// NOTE(M1): snapX replay的是M0前的逐字取整旧模型,现绘制已改走glyph.X/
// caret表(M0对策C+M1-13),故headless下该值恒≈821px(M0-pre基线)不代表回退;
// 真实一致性证据见TestPaintUsesShapedX_MultiFaceBulk与TestCaretMatchesPaint_M1,
// 像素墨迹对比仍待GPU真窗(见README"像素墨迹比对待补").
func computeProbes(bText string, face text.Face, fontSize float64) probes {
	p := probes{PaintMsP99: -1, ScrollFPS: -1}
	if face == nil || bText == "" {
		p.CaretVsPaintMaxDeltaPx = -1
		p.LayoutMsP99 = -1
		p.KeystrokeMsP99 = -1
		p.Unmeasured = []string{"no-face-or-empty", "paint_ms_p99", "scroll_fps", "interval_p95_ms", "fps_interval", "hitch_rate_per_min", "rss_slope_kb_per_min"}
		return p
	}
	lay := rendering.BuildTextLayout(bText, face, fontSize, 0, 1.25)
	layoutX := 0.0
	if lay != nil && lay.LineCount() > 0 {
		cs := lay.LineCarets(lay.LineCount() - 1)
		if len(cs) > 0 {
			layoutX = cs[len(cs)-1].X
		}
	}
	snapX := 0.0
	for _, r := range bText {
		snapX += math.Round(text.RuneAdvance(face, r))
	}
	p.CaretVsPaintMaxDeltaPx = math.Abs(layoutX - snapX)

	var ds []float64
	for i := 0; i < 31; i++ {
		t0 := time.Now()
		_ = rendering.BuildTextLayout(bText, face, fontSize, 0, 1.25)
		ds = append(ds, float64(time.Since(t0).Microseconds())/1000)
	}
	p.LayoutMsP99 = p99(ds)

	ds = ds[:0]
	for i := 0; i < 31; i++ {
		t0 := time.Now()
		_ = rendering.BuildTextLayout(bText+"x", face, fontSize, 0, 1.25)
		ds = append(ds, float64(time.Since(t0).Microseconds())/1000)
	}
	p.KeystrokeMsP99 = p99(ds)
	p.Unmeasured = []string{"paint_ms_p99", "scroll_fps", "interval_p95_ms", "fps_interval", "hitch_rate_per_min", "rss_slope_kb_per_min"}
	return p
}

func selftest() {
	wrkit.EnsureUIFace()
	face := wrkit.FaceAt(16)
	bText := loadText("b_5000.txt")
	p := computeProbes(bText, face, 16)
	out := map[string]any{
		"scenario": "ui_text_edit_accept",
		"mode":     "headless-cpu",
		"probes":   p,
		"go":       runtime.Version(),
	}
	raw, _ := json.Marshal(out)
	fmt.Println(string(raw))
}

func main() {
	if os.Getenv("GPUI_ACCEPT_SELFTEST") == "1" {
		selftest()
		return
	}
	wrkit.EnsureUIFace()
	face := wrkit.FaceAt(16)

	bText := loadText("b_5000.txt")
	cText := loadText("c_50000.txt")
	eText := loadText("e_1e5.txt")
	fText := loadText("f_wrap.txt")
	base := computeProbes(bText, face, 16)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_text_edit_accept — 文本排版绘制验收", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	var app *embedder.PipelineApp
	var proc scheduler.ProcessTracker
	proc.Start()

	shell := wrkit.NewShell(winW, winH, "ui_text_edit_accept — 单行+多行真实输入框验收 (§5.6)", []string{
		"A 单行短:手工键入",
		"B 单行5000字:横滚",
		"C 单行50000字:10x量级",
		"D 多行短:换行/上下键",
		"E 多行1e5字:纵滚",
		"F 多行回绕:段作废",
	})
	const gap = 12.0
	const topH = 48.0
	const hudH = 72.0
	legW := 260.0
	bodyX := gap + legW + gap
	bodyW := winW - bodyX - gap
	legH := winH - topH - hudH - gap*2
	shell.Legend.Box.FixedWidth = legW
	shell.Legend.Box.FixedHeight = legH
	shell.Body.W = bodyW
	shell.Body.Box.FixedWidth = bodyW
	shell.Body.Box.FixedHeight = legH
	shell.Root.Place(shell.Legend.Box, gap, topH+gap)
	shell.Root.Place(shell.Body.Box, bodyX, topH+gap)
	shell.Body.LabelAt(fmt.Sprintf("基线 caret_vs_paint=%.1fpx layout_p99=%.2fms key_p99=%.2fms",
		base.CaretVsPaintMaxDeltaPx, base.LayoutMsP99, base.KeystrokeMsP99),
		11, 12, 12, 0.95, 0.85, 0.55)

	mk := func() *textinput.Editor { return textinput.New() }
	edA, edB, edC := mk(), mk(), mk()
	edD, edE, edF := mk(), mk(), mk()
	boxW := bodyW - 24
	boxA := textinput.NewInputBox(edA, boxW, 40, 16)
	boxB := textinput.NewViewportInputBox(edB, boxW, 40, 16)
	boxC := textinput.NewViewportInputBox(edC, boxW, 40, 16)
	boxD := textinput.NewMultiLineInputBox(edD, boxW, 90, 16)
	boxE := textinput.NewMultiLineInputBox(edE, boxW, 90, 16)
	boxF := textinput.NewMultiLineInputBox(edF, boxW, 90, 16)
	for _, b := range []*textinput.InputBox{boxA} {
		if face != nil {
			b.SetFace(face)
		}
	}
	for _, b := range []*textinput.ViewportInputBox{boxB, boxC} {
		if face != nil {
			b.SetFace(face)
		}
	}
	for _, b := range []*textinput.MultiLineInputBox{boxD, boxE, boxF} {
		if face != nil {
			b.SetFace(face)
		}
	}
	boxF.SetWrap(true)
	// Preload policy (M0 default): B/C/F verify M0 truth (shaped layout +
	// per-rune paint + wrap derivation). E (1e5 chars) wedges first-frame
	// GPU atlas upload until M2/M4 culling lands, so it is opt-in:
	// GPUI_ACCEPT_ONLY=E enables it alone, GPUI_ACCEPT_FULL=1 enables all.
	// GPUI_ACCEPT_NOPREFILL=1 skips all preload.
	only := os.Getenv("GPUI_ACCEPT_ONLY")
	full := os.Getenv("GPUI_ACCEPT_FULL") != ""
	want := func(name string) bool { return full || only == "" && name != "E" || only == name }
	if os.Getenv("GPUI_ACCEPT_NOPREFILL") == "" {
		if want("B") {
			edB.SetTextSimple(bText)
		}
		if want("C") {
			edC.SetTextSimple(cText)
		}
		if want("E") {
			edE.SetTextSimple(eText)
		}
		if want("F") {
			edF.SetTextSimple(fText)
		}
	}
	boxA.SetSchedule(func() { app.ScheduleFrame() })
	boxB.SetSchedule(func() { app.ScheduleFrame() })
	boxC.SetSchedule(func() { app.ScheduleFrame() })
	boxD.SetSchedule(func() { app.ScheduleFrame() })
	boxE.SetSchedule(func() { app.ScheduleFrame() })
	boxF.SetSchedule(func() { app.ScheduleFrame() })

	y := 36.0
	placeLabel := func(label string) {
		shell.Body.LabelAt(label, 11, 12, y, 0.70, 0.78, 0.88)
		y += 18
	}
	placeLabel("A 单行短（手工键入）")
	shell.Body.Place(boxA, 12, y)
	y += 48
	placeLabel("B 单行5000字（横滚·末尾击键）")
	shell.Body.Place(boxB, 12, y)
	y += 48
	placeLabel("C 单行50000字（10x量级对比）")
	shell.Body.Place(boxC, 12, y)
	y += 48
	placeLabel("D 多行短（Enter换行·上下键）")
	shell.Body.Place(boxD, 12, y)
	y += 98
	placeLabel("E 多行1e5字（纵滚·末行击键）")
	shell.Body.Place(boxE, 12, y)
	y += 98
	placeLabel("F 多行回绕（段首插入看重排）")
	shell.Body.Place(boxF, 12, y)

	fm := focus.NewManager()
	router := embedder.NewInputRouter(nil, fm)
	fm.Register(boxA.FocusNode())
	fm.Register(boxB.FocusNode())
	fm.Register(boxC.FocusNode())
	fm.Register(boxD.FocusNode())
	fm.Register(boxE.FocusNode())
	fm.Register(boxF.FocusNode())
	router.OnPointer = func(pe input.PointerEvent, target rendering.RenderObject) {}
	tick := 0
	router.OnKey = func(ke input.KeyEvent) {
		switch {
		case boxF.IsFocused():
			boxF.OnKey(ke)
		case boxE.IsFocused():
			boxE.OnKey(ke)
		case boxD.IsFocused():
			boxD.OnKey(ke)
		case boxB.IsFocused():
			boxB.OnKey(ke)
		case boxC.IsFocused():
			boxC.OnKey(ke)
		default:
			boxA.OnKey(ke)
		}
	}
	router.OnText = func(ev input.TextEvent) {}
	router.OnIME = func(ev input.IMEEvent) {}

	runFor := time.Duration(0)
	if v := os.Getenv("GPUI_ACCEPT_RUN_SECONDS"); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			runFor = time.Duration(n) * time.Second
		}
	}
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		WarmUp: true,
		Input:  router,
		IME:    win.IME(),
		RunFor: runFor,
		// GPUI_ACCEPT_SNAP=path saves a GPU-readback PNG of the final frame.
		SnapshotPath: os.Getenv("GPUI_ACCEPT_SNAP"),
	})
	app.Scheduler().Tickers().Add(&acceptTicker{on: func(dt float64) {
		tick++
		if tick%30 == 0 {
			boxA.SetCaretOn(!boxA.IsCaretOn())
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && snapH.CPUFallbackOps == 0
		shell.UpdateHUD("text_edit_accept", "MANUAL", app, gateOK,
			fmt.Sprintf("tick=%d caret_vs_paint=%.1fpx", tick, base.CaretVsPaintMaxDeltaPx), "")
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
	live := liveProbes(boxB, boxC, face, 16)
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "text_edit_accept",
		Scenario:      "ui_text_edit_accept",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"present_policy":              "full_paint",
			"manual":                      true,
			"caret_vs_paint_max_delta_px": live.CaretVsPaintMaxDeltaPx,
			"bulk_taken":                  live.BulkTaken,
			"layout_ms_p99":               live.LayoutMsP99,
			"keystroke_p99_ms":            live.KeystrokeMsP99,
			"unmeasured":                  live.Unmeasured,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
}

type acceptTicker struct{ on func(dt float64) }

func (t *acceptTicker) Tick(dt float64) bool { t.on(dt); return true }
