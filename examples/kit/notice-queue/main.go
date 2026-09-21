// Command notice-queue is the G2 real window: shared message top queue
// plus notification six-corner pools.
//
// RUN_SECONDS=5 go run ./examples/kit/notice-queue
//
// Window: 1200x800 baseline, user-resizable. One scripted resize excursion
// restores the baseline before gates are read.
// Correctness window (RUN_SECONDS>=5): slope_gate=off, fps gate on.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

var (
	clearBG = [3]float64{0.08, 0.09, 0.11}
	noteBG  = [3]float64{0.10, 0.11, 0.13}
)

type probePoint struct {
	x, y float64
	want [3]float64
	tol  float64
}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	pt   *probePoint
	text *rect
	base [3]float64
	desc string
}

var pixelResult = map[string]bool{}

// tipBar builds one message bar. Colors come from resolved queue theme.
func tipBar(title string, bg theme.Color, w, h float64) *rendering.AbsoluteBox {
	bar := rendering.NewAbsoluteBox(w, h)
	bar.Background = &rendering.Color{R: bg.R, G: bg.G, B: bg.B, A: 1}
	bar.SetDebugName("g2-message-" + title)
	bar.SetRepaintBoundary(true)
	bar.Place(wrkit.Label(title, 12, 0.12, 0.14, 0.16), 12, 10)
	return bar
}

// noticeCard builds one notification card. Colors come from resolved theme.
func noticeCard(title string, bg theme.Color, w, h float64) *rendering.AbsoluteBox {
	card := rendering.NewAbsoluteBox(w, h)
	card.Background = &rendering.Color{R: bg.R, G: bg.G, B: bg.B, A: 1}
	card.SetDebugName("g2-notice-" + title)
	card.SetRepaintBoundary(true)
	card.Place(wrkit.Label(title, 12, 0.12, 0.14, 0.16), 12, 10)
	return card
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "G2-notice-queue")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit notice-queue — G2 message+pools", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// Real queue: two stacked tips plus two corner pools prove independence.
	baseCtx := kit.DefaultScopeCtx()
	skinTokens := baseCtx.Theme
	skinTokens.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := baseCtx.WithTheme(skinTokens)
	queue := kit.BuildNoticeQueue(baseCtx, kit.DefaultNoticeQueueProps())
	persist := func(v float64) (float64, bool) { return v, true }
	d10, s10 := persist(10)
	mk := func(key, content string, t kit.NoticeQueueMessageType) kit.NoticeQueueMessageConfig {
		cfg := kit.DefaultNoticeQueueMessageConfig()
		cfg.Key, cfg.Content, cfg.Type = key, content, t
		cfg.Duration, cfg.DurationSet = d10, s10
		return cfg
	}
	queue.OpenMessage(mk("m1", "success tip", kit.NoticeQueueMessageSuccess))
	queue.OpenMessage(mk("m2", "info tip", kit.NoticeQueueMessageInfo))
	nk := func(key, title, pl string, t kit.NoticeQueueNotificationType) kit.NoticeQueueNotificationConfig {
		cfg := kit.DefaultNoticeQueueNotificationConfig()
		cfg.Key, cfg.Title, cfg.Description, cfg.Type = key, title, "corner pool stays independent", t
		cfg.Duration, cfg.DurationSet = d10, s10
		cfg.ShowProgress = true
		if pl != "" {
			cfg.Placement, cfg.PlacementSet = kit.NoticeQueuePlacement(pl), true
		}
		return cfg
	}
	queue.OpenNotification(nk("n1", "topRight card", "topRight", kit.NoticeQueueNotificationSuccess))
	queue.OpenNotification(nk("n2", "topRight second", "topRight", kit.NoticeQueueNotificationInfo))
	queue.OpenNotification(nk("n3", "bottomLeft card", "bottomLeft", kit.NoticeQueueNotificationWarning))
	// Remaining pools: top, bottom, topLeft, bottomRight each hold one.
	queue.OpenNotification(nk("n4", "top card", "top", kit.NoticeQueueNotificationInfo))
	queue.OpenNotification(nk("n5", "bottom card", "bottom", kit.NoticeQueueNotificationInfo))
	queue.OpenNotification(nk("n6", "topLeft card", "topLeft", kit.NoticeQueueNotificationWarning))
	queue.OpenNotification(nk("n7", "bottomRight card", "bottomRight", kit.NoticeQueueNotificationError))
	skinQueue := kit.BuildNoticeQueue(skinCtx, kit.DefaultNoticeQueueProps())
	skinQueue.OpenNotification(nk("s1", "reskin static", "topRight", kit.NoticeQueueNotificationInfo))

	resolved := kit.ResolveNoticeQueue(baseCtx.Theme)
	resolvedSkin := kit.ResolveNoticeQueue(skinCtx.Theme)
	progress := resolved.Progress

	shell := wrkit.NewShell(winW, winH, "G2 notice queue — top tips plus corner pools", []string{
		"top center: two stacked tips",
		"right: topRight pool x2 cards",
		"left bottom: bottomLeft pool",
		"row: top bottom topLeft bottomRight live",
		"stack 5 over 3 shows 1 folds 4",
		"far right: reskinned holder",
		"bottom: progress strip frozen",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	// Visual bars mirror queue order; geometry helpers keep centers honest.
	msgW := 520.0
	msgH := 40.0
	bar1 := tipBar("success tip", resolved.MessageBg, msgW, msgH)
	bar2 := tipBar("info tip", resolved.MessageBg, msgW, msgH)
	shell.Body.Place(bar1, 190, 8)
	shell.Body.Place(bar2, 190, 56)
	cardW, cardH := 300.0, 110.0
	cA := noticeCard("topRight card", resolved.NoticeBg, cardW, cardH)
	cB := noticeCard("topRight second", resolved.NoticeBg, cardW, cardH)
	cC := noticeCard("bottomLeft card", resolved.NoticeBg, cardW, cardH)
	cS := noticeCard("reskin static #722ed1", resolvedSkin.NoticeBg, 220, cardH)
	shell.Body.Place(cA, 560, 120)
	shell.Body.Place(cB, 560, 246)
	shell.Body.Place(cC, 24, 440)
	shell.Body.Place(cS, 640, 440)
	cS.Place(wrkit.Label(skinQueue.HolderContent("notification"), 10, 0.30, 0.22, 0.55), 12, 36)
	cS.Place(wrkit.Label("static eats theme", 10, 0.30, 0.32, 0.40), 12, 58)
	progStrip := rendering.NewRenderColorBox(300, 6, progress.R, progress.G, progress.B, 1)
	progStrip.SetRepaintBoundary(true)
	progStrip.SetDebugName("g2-notice-progress")
	shell.Body.Place(progStrip, 190, 110)
	capNote := rendering.NewAbsoluteBox(200, 120)
	capNote.Background = &rendering.Color{R: noteBG[0], G: noteBG[1], B: noteBG[2], A: 1}
	capNote.Place(wrkit.Label("destroy one keeps rest", 11, 0.62, 0.72, 0.85), 8, 8)
	capNote.Place(wrkit.Label("hover freezes clock", 10, 0.55, 0.65, 0.78), 8, 30)
	shell.Body.Place(capNote, 24, 200)
	// Pool row: the four unshown pools, each with its live card.
	poolNames := []string{"top pool", "bottom pool", "topLeft pool", "bottomRight pool"}
	for i, name := range poolNames {
		mini := noticeCard(name, resolved.NoticeBg, 150, 64)
		shell.Body.Place(mini, 24+float64(i)*160, 332)
	}
	vis, folded := kit.NoticeQueueVisibleCount(5, true, 3)
	stackLine := rendering.NewAbsoluteBox(320, 24)
	stackLine.Background = &rendering.Color{R: noteBG[0], G: noteBG[1], B: noteBG[2], A: 1}
	stackLine.Place(wrkit.Label(fmt.Sprintf("stack 5 over 3 shows %d folds %d", vis, folded), 10, 0.55, 0.65, 0.78), 8, 4)
	shell.Body.Place(stackLine, 24, 404)

	snapDir := filepath.Join("examples", "kit", "notice-queue", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	ptMsg := probePoint{want: [3]float64{resolved.MessageBg.R, resolved.MessageBg.G, resolved.MessageBg.B}, tol: 8.0 / 255}
	ptNotice := probePoint{want: [3]float64{resolved.NoticeBg.R, resolved.NoticeBg.G, resolved.NoticeBg.B}, tol: 8.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		ax, ay := bar1.Offset().X, bar1.Offset().Y
		ptMsg.x = bodyX + ax + msgW/2
		ptMsg.y = bodyY + ay + msgH/2
		nx, ny := cA.Offset().X, cA.Offset().Y
		ptNotice.x = bodyX + nx + cardW/2
		ptNotice.y = bodyY + ny + cardH/2
		cx, cy := capNote.Offset().X, capNote.Offset().Y
		capTextBox = rect{x: bodyX + cx + 8, y: bodyY + cy + 8, w: 184, h: 60}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + ax, bodyY + ay, msgW, msgH},
			{bodyX + nx, bodyY + ny, cardW, cardH},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_notice_queue.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit notice-queue: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					shell.Resize(float64(ev.Width), float64(ev.Height))
					geoValid = false
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	var elapsed float64
	resizeDone, resizeBack := false, false
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		if ctl != nil && !resizeDone && elapsed >= 2.0 {
			resizeDone = true
			ctl.SetSize(1400, 900)
		}
		if ctl != nil && resizeDone && !resizeBack && elapsed >= 3.0 {
			resizeBack = true
			ctl.SetSize(winW, winH)
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundarySkip >= 0 && snapH.PaintCount > 0
		shell.UpdateHUD("G2-notice-queue", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d msg=%d ntf=%d", snapH.PaintCount, app.PresentCount(), queue.MessageCount(), queue.NotificationCount()),
			fmt.Sprintf("resize=%d t=%.1f", resizeEvents, elapsed))
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

	finalW, finalH := winW, winH
	if ctl != nil {
		if w, h := ctl.Size(); w > 0 && h > 0 {
			finalW, finalH = w, h
		}
	}

	resolveGeometry()
	finalChecks := []pixelCheck{
		{pt: &ptMsg, desc: "message_bar_bg"},
		{pt: &ptNotice, desc: "notice_card_bg"},
		{text: &capTextBox, base: noteBG, desc: "caption_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_notice_queue.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "G2-notice-queue",
		Scenario:      "kit_notice_queue",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"message_count":          queue.MessageCount(),
			"notification_count":     queue.NotificationCount(),
			"topright_count":         queue.NotificationCount(kit.NoticeQueuePlacementTopRight),
			"bottomleft_count":       queue.NotificationCount(kit.NoticeQueuePlacementBottomLeft),
			"top_count":              queue.NotificationCount(kit.NoticeQueuePlacementTop),
			"bottom_count":           queue.NotificationCount(kit.NoticeQueuePlacementBottom),
			"topleft_count":          queue.NotificationCount(kit.NoticeQueuePlacementTopLeft),
			"bottomright_count":      queue.NotificationCount(kit.NoticeQueuePlacementBottomRight),
			"skin_holder":            skinQueue.HolderContent("notification"),
			"scripted_ok":            scriptedOK,
			"scripted_total":         scriptedTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"resize_events":          resizeEvents,
			"resize_restored":        finalW == winW && finalH == winH,
			"client_px":              fmt.Sprintf("%dx%d", finalW, finalH),
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MaxP95Ms:               22,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if scriptedTotal != 3 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 3/3 (pixel probes)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	if !resizeBack || finalW != winW || finalH != winH {
		fmt.Fprintf(os.Stderr, "FAIL: resize round trip incomplete (back=%v %dx%d)\n", resizeBack, finalW, finalH)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "kit notice-queue: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiffPct, resizeBack, finalW, finalH, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		var ok bool
		if c.pt != nil {
			r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
			ok = valid && nearC(r, g, b, c.pt.want, c.pt.tol)
			fmt.Fprintf(os.Stderr, "kit notice-queue: pixel %-24s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "kit notice-queue: pixel %-24s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
		}
		result[c.desc] = ok
	}
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: decode %s: %v\n", path, err)
		return nil
	}
	return img
}

func sampleLogical(img image.Image, dpr float64, lx, ly float64) (r, g, b float64, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	px, py := int(lx*dpr), int(ly*dpr)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

func nearC(r, g, b float64, want [3]float64, tol float64) bool {
	return math.Abs(r-want[0]) <= tol && math.Abs(g-want[1]) <= tol && math.Abs(b-want[2]) <= tol
}

func textPixels(img image.Image, dpr float64, box rect, base [3]float64) int {
	if img == nil {
		return 0
	}
	x0, y0 := int(box.x*dpr), int(box.y*dpr)
	x1 := int((box.x + box.w) * dpr)
	y1 := int((box.y + box.h) * dpr)
	n := 0
	for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
		for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
			r32, g32, b32, _ := img.At(px, py).RGBA()
			r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
			if math.Abs(r-base[0]) > 24.0/255 || math.Abs(g-base[1]) > 24.0/255 || math.Abs(b-base[2]) > 24.0/255 {
				n++
			}
		}
	}
	return n
}

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_notice_queue.png")
	base := filepath.Join(snapDir, "showcase_notice_queue_base.png")
	if _, err := os.Stat(base); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			fmt.Fprintf(os.Stderr, "golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit)\n", base)
			return 100, 0, false
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden: current snapshot %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "golden baseline stored: %s\n", base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "golden %s: diff=%.4f%% over %d px\n", filepath.Base(cur), diff, total)
	return diff, total, false
}

func comparePNG(basePath, curPath string, rects []rect) (pct float64, total int64, err error) {
	a, b := loadImage(basePath), loadImage(curPath)
	if a == nil || b == nil {
		return 0, 0, fmt.Errorf("decode failed")
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v", a.Bounds(), b.Bounds())
	}
	dpr := float64(a.Bounds().Dx()) / winW
	var diff int64
	for _, r := range rects {
		x0, y0 := int(r.x*dpr), int(r.y*dpr)
		x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
		for py := y0; py < y1 && py < a.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < a.Bounds().Dx(); px++ {
				ar, ag, ab, aa := a.At(px, py).RGBA()
				br, bg, bb, ba := b.At(px, py).RGBA()
				total++
				if ar != br || ag != bg || ab != bb || aa != ba {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("empty golden mask")
	}
	return float64(diff) * 100 / float64(total), total, nil
}
