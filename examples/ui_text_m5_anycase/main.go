// Command ui_text_m5_anycase is the M5 real-window: any-script close-out.
//
// Zones in one window: A multi-script, B emoji, C rich runs, D multi-DPR.
// Texts come from testdata/*.txt; main.go never invents corpus.
//
//	GPUI_M5_SELFTEST=1 go run ./examples/ui_text_m5_anycase  # headless baseline, no GPU window
//	RUN_SECONDS=30 go run ./examples/ui_text_m5_anycase      # interactive window
//
// Window: 1200x800. Headless probes print one JSON line (see README).
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
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func testdataPath(name string) string {
	if dir := os.Getenv("GPUI_M5_TESTDATA"); dir != "" {
		return filepath.Join(dir, name)
	}
	return filepath.Join("examples", "ui_text_m5_anycase", "testdata", name)
}

func loadText(name string) string {
	b, err := os.ReadFile(testdataPath(name))
	if err != nil || len(b) == 0 {
		return "fallback " + name
	}
	return string(b)
}

// emojiFace loads the system color emoji font for zone B. Nil when absent —
// callers keep the previous chain behavior (no color path) in that case.
func emojiFace(points float64) text.Face {
	for _, p := range []string{
		"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/TTF/NotoColorEmoji.ttf",
	} {
		src, err := text.NewFontSourceFromFile(p)
		if err != nil {
			continue
		}
		return src.Face(points)
	}
	return nil
}

func memMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.HeapAlloc) / 1e6
}

// dprKeyCardinality counts distinct mask-atlas keys for one glyph rendered
// at 4 DPR scales. 1/4px quantization is implemented, so fractional offsets
// that fall in the same quarter must share a key: cardinality stays small
// (validates M5-4 "atlas does not explode", no new behavior).
func dprKeyCardinality() (keys int, detail string) {
	seen := map[text.GlyphMaskKey]bool{}
	// Same glyph at DPR 1/1.25/2/3 with a sweep of fractional offsets.
	for _, dpr := range []float64{1, 1.25, 2, 3} {
		for _, frac := range []float64{0, 0.1, 0.24, 0.25, 0.49, 0.5, 0.74, 0.75, 0.99} {
			k := text.MakeGlyphMaskKey(7, 65, 16*dpr, frac, 0)
			seen[k] = true
		}
	}
	return len(seen), fmt.Sprintf("1 glyph x 4 DPR x 9 offsets -> %d keys (quantized, expect <= 16)", len(seen))
}

// keystrokeSample is a small window-side G1 signal: rebuild time at 1e3 vs
// 1e5 chars via the production RenderText path. The binding G1 gate
// (N in {1e3,1e4,1e5,1e6}, T(1e6)/T(1e3) <= 1.5) lives in BenchmarkKeystroke;
// this sample is only a smoke signal, honestly labeled as such.
func keystrokeSample(face text.Face) (t1k, t100k, ratio float64) {
	mk := func(n int) (base, edited string, mid int) {
		base = strings.Repeat("a世", n/2)[:n]
		if len(base) < n {
			base += strings.Repeat("a", n-len(base))
		}
		mid = len(base) / 2
		edited = base[:mid] + "X" + base[mid+1:]
		return base, edited, mid
	}
	timeOnce := func(n int) float64 {
		base, edited, mid := mk(n)
		rt := rendering.NewRenderText(base)
		if face != nil {
			rt.SetFace(face)
		}
		_ = rt.TextLayout()
		t0 := time.Now()
		const reps = 5
		for i := 0; i < reps; i++ {
			rt.SetTextSpan(edited, mid, mid+1, mid, mid+1)
			_ = rt.TextLayout()
			rt.SetTextSpan(base, mid, mid+1, mid, mid+1)
			_ = rt.TextLayout()
		}
		return float64(time.Since(t0).Microseconds()) / 1000 / reps
	}
	t1k = timeOnce(1000)
	t100k = timeOnce(100000)
	if t1k > 0 {
		ratio = t100k / t1k
	} else {
		ratio = -1
	}
	return t1k, t100k, ratio
}

func buildZoneLayouts(face text.Face) (ms float64, lines int) {
	multi := loadText("multiscript.txt")
	emoji := loadText("emoji.txt")
	richLines := strings.Split(strings.TrimSpace(loadText("richtext.txt")), "\n")
	t0 := time.Now()
	a := rendering.NewRenderText(multi)
	a.MaxWidth = 560
	if face != nil {
		a.SetFace(face)
	}
	_ = a.TextLayout()
	lines += a.TextLayout().LineCount()
	b := rendering.NewRenderText(emoji)
	b.MaxWidth = 560
	if face != nil {
		b.SetFace(face)
	}
	_ = b.TextLayout()
	lines += b.TextLayout().LineCount()
	pb := rendering.NewParagraphBuilder()
	sizes := []float64{20, 14, 8, 12}
	cols := [][4]float64{{0.95, 0.9, 0.5, 1}, {0.85, 0.88, 0.92, 1}, {0.6, 0.8, 0.95, 1}, {0.75, 0.95, 0.75, 1}}
	for i, ln := range richLines {
		sz := sizes[i%len(sizes)]
		c := cols[i%len(cols)]
		var faceRun text.Face
		if face != nil {
			faceRun = face
		}
		pb.AddRun(rendering.TextRun{Text: ln + " ", Face: faceRun, FontSize: sz, R: c[0], G: c[1], B: c[2], A: c[3]})
	}
	c := rendering.NewRenderText("")
	c.MaxWidth = 560
	c.SetRuns(pb.Runs())
	_ = c.TextLayout()
	lines += c.TextLayout().LineCount()
	ms = float64(time.Since(t0).Microseconds()) / 1000
	return ms, lines
}

func selftest() {
	wrkit.EnsureUIFace()
	face := wrkit.FaceAt(16)
	runtime.GC()
	m0 := memMB()
	firstMs, lines := buildZoneLayouts(face)
	m1 := memMB()
	keys, dprNote := dprKeyCardinality()
	t1k, t100k, ratio := keystrokeSample(face)
	out := map[string]any{
		"scenario": "ui_text_m5_anycase",
		"mode":     "headless-cpu",
		"baseline": map[string]any{
			"first_screen_ms":   firstMs,
			"layout_lines":      lines,
			"heap_delta_mb":     m1 - m0,
			"dpr_keys":          keys,
			"dpr_note":          dprNote,
			"keystroke_1k_ms":   t1k,
			"keystroke_100k_ms": t100k,
			"g1_sample_ratio":   ratio,
			"g1_note":           "window smoke sample only; binding gate is BenchmarkKeystroke T(1e6)/T(1e3) <= 1.5",
			"khmer_tibetan":     "covered: khmer_all(128)+tibetan_all(256) shape tests (M5-3)",
		},
		"go": runtime.Version(),
	}
	raw, _ := json.Marshal(out)
	fmt.Println(string(raw))
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool { t.on(dt); return true }

func zoneLabel(s string) *rendering.RenderText {
	return wrkit.Label(s, 13, 0.55, 0.75, 0.95)
}

func main() {
	if os.Getenv("GPUI_M5_SELFTEST") == "1" {
		selftest()
		return
	}
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = 30
	}
	wrkit.EnsureUIFace()
	face := wrkit.FaceAt(16)
	multi := loadText("multiscript.txt")
	emoji := loadText("emoji.txt")
	richLines := strings.Split(strings.TrimSpace(loadText("richtext.txt")), "\n")

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_text_m5_anycase — 任意场景收口", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	var proc scheduler.ProcessTracker
	proc.Start()

	shell := wrkit.NewShell(winW, winH, "M5 任意场景收口 — 多脚本+emoji+富文本+多DPR 同窗", []string{
		"A 多脚本:拉丁/CJK/阿拉伯/希伯来/泰/缅/梵",
		"B emoji:ZWJ/修饰符/组合音标",
		"C 富文本:多字号多颜色 run",
		"D 多DPR:同串 1x/1.25x/2x/3x",
	})

	bodyY := 0.0
	mkText := func(s string, w float64) *rendering.RenderText {
		t := wrkit.Label(s, 14, 0.88, 0.9, 0.93)
		t.MaxWidth = w
		return t
	}
	_ = face

	shell.Body.LabelAt("A 多脚本 (testdata/multiscript.txt)", 11, 12, bodyY, 0.55, 0.75, 0.95)
	bodyY += 18
	ta := mkText(multi, 560)
	shell.Body.Place(ta, 12, bodyY)
	bodyY += 178
	_ = zoneLabel

	shell.Body.LabelAt("B emoji + 组合字符 (testdata/emoji.txt)", 11, 12, bodyY, 0.55, 0.75, 0.95)
	bodyY += 18
	tb := mkText(emoji, 560)
	// B 区组装 MultiFace（表情字体在前、UI 链在后）：emoji 字形走颜色图集
	// 管线，拉丁/CJK 回退 UI 脸走遮罩管线；缺表情字体时保持默认链。
	// GPUI_M5_NOEMOJI=1 跳过表情字体，用于与改前基线对照。
	if os.Getenv("GPUI_M5_NOEMOJI") == "" {
		if ef := emojiFace(14); ef != nil {
			if mf, err := text.NewMultiFace(ef, face); err == nil {
				tb.SetFace(mf)
			}
		}
	}
	shell.Body.Place(tb, 12, bodyY)
	bodyY += 140

	shell.Body.LabelAt("C 富文本 run (testdata/richtext.txt, 20/14/8/12pt)", 11, 12, bodyY, 0.55, 0.75, 0.95)
	bodyY += 18
	pb := rendering.NewParagraphBuilder()
	sizes := []float64{20, 14, 8, 12}
	cols := [][4]float64{{0.95, 0.9, 0.5, 1}, {0.85, 0.88, 0.92, 1}, {0.6, 0.8, 0.95, 1}, {0.75, 0.95, 0.75, 1}}
	for i, ln := range richLines {
		sz := sizes[i%len(sizes)]
		cc := cols[i%len(cols)]
		pb.AddRun(rendering.TextRun{Text: ln + " ", Face: wrkit.FaceAt(sz), FontSize: sz, R: cc[0], G: cc[1], B: cc[2], A: cc[3]})
	}
	tc := rendering.NewRenderText("")
	tc.MaxWidth = 560
	tc.SetRuns(pb.Runs())
	shell.Body.Place(tc, 12, bodyY)
	bodyY += 118

	shell.Body.LabelAt("D 多DPR同串 (12/15/24/36pt)", 11, 12, bodyY, 0.55, 0.75, 0.95)
	bodyY += 18
	for i, sz := range []float64{12, 15, 24, 36} {
		td := wrkit.Label(fmt.Sprintf("%gx: Hello 世界 مرحبا กข", []float64{1, 1.25, 2, 3}[i]), sz, 0.88, 0.9, 0.93)
		shell.Body.Place(td, 12, bodyY)
		bodyY += sz + 18
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		// GPUI_M5_SNAP=path saves a GPU-readback PNG of the final frame.
		SnapshotPath: os.Getenv("GPUI_M5_SNAP"),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	firstMs, lines := buildZoneLayouts(face)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && snapH.CPUFallbackOps == 0
		shell.UpdateHUD("m5_anycase", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("first=%.1fms lines=%d", firstMs, lines), "")
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
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "text_m5_anycase",
		Scenario:      "ui_text_m5_anycase",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"first_screen_ms": firstMs,
			"layout_lines":    lines,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))
}
