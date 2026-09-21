// Command upload is the G6 real window: upload list plus trigger.
//
// RUN_SECONDS=5 go run ./examples/kit/upload
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
	dimBG   = [3]float64{0.13, 0.15, 0.19}
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

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "G6-upload")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit upload — G6 list+trigger", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// Real upload: done + uploading + error rows, accept/pastable/maxCount
	// siblings prove the semantic matrix.
	baseCtx := kit.DefaultScopeCtx()
	skinTokens := baseCtx.Theme
	skinTokens.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := baseCtx.WithTheme(skinTokens)
	props := kit.DefaultUploadProps()
	up := kit.BuildUpload(baseCtx, props)
	up.SelectFiles([]kit.UploadLocalFile{{Name: "a.png", Type: "image/png"}})
	up.Tick(10)
	up.SelectFiles([]kit.UploadLocalFile{{Name: "b.png", Type: "image/png"}})
	up.Tick(0.2)
	efile := up.SelectFiles([]kit.UploadLocalFile{{Name: "c.png"}})
	up.ReportError(efile[0].UID, "timeout")
	list := up.FileList()
	skinUp := kit.BuildUpload(skinCtx, props)

	resolved := kit.ResolveUpload(baseCtx.Theme)
	resolvedSkin := kit.ResolveUpload(skinCtx.Theme)
	progW := 0.0
	var firstStatus kit.UploadFileStatus
	if len(list) > 1 {
		progW = list[1].Percent / 100 * 240
		firstStatus = list[0].Status
	}

	shell := wrkit.NewShell(winW, winH, "G6 upload — list plus trigger", []string{
		"list: done plus uploading live",
		"rows: error red plus removed",
		"right: accept png plus paste",
		"far right: reskinned holder",
		"bottom: maxCount1 replace live",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	// List card: one row per live file; rows come from the instance list.
	listCard := rendering.NewAbsoluteBox(300, 150)
	listCard.Background = &rendering.Color{R: resolved.PanelBg.R, G: resolved.PanelBg.G, B: resolved.PanelBg.B, A: 1}
	listCard.SetDebugName("g6-upload-list")
	listCard.SetRepaintBoundary(true)
	listCard.Place(wrkit.Label("uploading done error", 12, 0.12, 0.14, 0.16), 12, 8)
	shell.Body.Place(listCard, 24, 24)
	for i, f := range list {
		y := 36 + float64(i)*32
		st := kit.UploadStatusColor(baseCtx.Theme, f.Status)
		listCard.Place(wrkit.Label(fmt.Sprintf("%s %s %.0f", f.Name, f.Status, f.Percent), 10, st.R, st.G, st.B), 12, y)
	}
	barBg := rendering.NewAbsoluteBox(240, 4)
	barBg.Background = &rendering.Color{R: 0.20, G: 0.22, B: 0.26, A: 1}
	listCard.Place(barBg, 12, 128)
	var progBar *rendering.AbsoluteBox
	if progW > 2 {
		progBar = rendering.NewAbsoluteBox(progW, 4)
		progBar.Background = &rendering.Color{R: resolvedSkin.Selected.R, G: resolvedSkin.Selected.G, B: resolvedSkin.Selected.B, A: 1}
		progBar.SetDebugName("g6-upload-progress")
		listCard.Place(progBar, 12, 128)
	}
	// Trigger card: accept/maxCount/paste siblings each on own host.
	trigCard := rendering.NewAbsoluteBox(280, 150)
	trigCard.Background = &rendering.Color{R: dimBG[0], G: dimBG[1], B: dimBG[2], A: 1}
	trigCard.SetDebugName("g6-upload-triggers")
	trigCard.SetRepaintBoundary(true)
	shell.Body.Place(trigCard, 340, 24)
	mprops := kit.DefaultUploadProps()
	mprops.MaxCount = 1
	mhost := kit.BuildUpload(baseCtx, mprops)
	mhost.SelectFiles([]kit.UploadLocalFile{{Name: "old.png"}})
	mhost.SelectFiles([]kit.UploadLocalFile{{Name: "new.png"}})
	mlist := mhost.FileList()
	mname := "empty"
	if len(mlist) == 1 {
		mname = mlist[0].Name
	}
	pprops := kit.DefaultUploadProps()
	pprops.Accept = ".png"
	phost := kit.BuildUpload(baseCtx, pprops)
	pngOK := len(phost.SelectFiles([]kit.UploadLocalFile{{Name: "a.png"}})) == 1
	pngNo := len(phost.SelectFiles([]kit.UploadLocalFile{{Name: "b.pdf", Type: "application/pdf"}})) == 0
	sprops := kit.DefaultUploadProps()
	sprops.Pastable = true
	shost := kit.BuildUpload(baseCtx, sprops)
	pasteOK := len(shost.PasteFiles([]kit.UploadLocalFile{{Name: "clip.png"}})) == 1
	trigCard.Place(wrkit.Label("accept png maxCount paste", 11, 0.62, 0.72, 0.85), 10, 8)
	trigCard.Place(wrkit.Label(fmt.Sprintf("png in=%v pdf out=%v", pngOK, pngNo), 10, 0.55, 0.65, 0.78), 10, 32)
	trigCard.Place(wrkit.Label(fmt.Sprintf("maxCount1 keeps %s", mname), 10, 0.55, 0.65, 0.78), 10, 54)
	trigCard.Place(wrkit.Label(fmt.Sprintf("paste clip=%v", pasteOK), 10, 0.55, 0.65, 0.78), 10, 76)
	trigCard.Place(wrkit.Label(fmt.Sprintf("first=%s uploading=%d", firstStatus, up.UploadingCount()), 10, 0.55, 0.65, 0.78), 10, 98)
	_ = listCard
	cS := rendering.NewAbsoluteBox(260, 130)
	cS.Background = &rendering.Color{R: resolvedSkin.PanelBg.R, G: resolvedSkin.PanelBg.G, B: resolvedSkin.PanelBg.B, A: 1}
	cS.SetDebugName("g6-upload-reskin")
	cS.SetRepaintBoundary(true)
	cS.Place(wrkit.Label("reskin static #722ed1", 12, 0.12, 0.14, 0.16), 12, 10)
	cS.Place(wrkit.Label(skinUp.HolderContent(), 10, 0.30, 0.22, 0.55), 12, 36)
	cS.Place(wrkit.Label("static eats theme", 10, 0.30, 0.32, 0.40), 12, 58)
	cS.Place(wrkit.Label(fmt.Sprintf("list=%d done=1", len(list)), 10, 0.30, 0.32, 0.40), 12, 80)
	shell.Body.Place(cS, 640, 24)
	capNote := rendering.NewAbsoluteBox(280, 130)
	capNote.Background = &rendering.Color{R: noteBG[0], G: noteBG[1], B: noteBG[2], A: 1}
	capNote.SetDebugName("g6-upload-cap")
	capNote.Place(wrkit.Label("beforeUpload skip ignore live", 11, 0.62, 0.72, 0.85), 8, 8)
	capNote.Place(wrkit.Label("customRequest callbacks live", 10, 0.55, 0.65, 0.78), 8, 30)
	capNote.Place(wrkit.Label("controlled ghost events drop", 10, 0.55, 0.65, 0.78), 8, 52)
	capNote.Place(wrkit.Label(fmt.Sprintf("files=%d error timeout", len(list)), 10, 0.55, 0.65, 0.78), 8, 74)
	shell.Body.Place(capNote, 640, 170)

	snapDir := filepath.Join("examples", "kit", "upload", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	ptPanel := probePoint{want: [3]float64{resolved.PanelBg.R, resolved.PanelBg.G, resolved.PanelBg.B}, tol: 8.0 / 255}
	ptBar := probePoint{want: [3]float64{resolvedSkin.Selected.R, resolvedSkin.Selected.G, resolvedSkin.Selected.B}, tol: 8.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		px, py := listCard.Offset().X, listCard.Offset().Y
		ptPanel.x = bodyX + px + 300 - 20
		ptPanel.y = bodyY + py + 10
		if progBar != nil {
			bx, by := progBar.Offset().X, progBar.Offset().Y
			ptBar.x = bodyX + px + bx + progW/2
			ptBar.y = bodyY + py + by + 2
		}
		nx, ny := capNote.Offset().X, capNote.Offset().Y
		capTextBox = rect{x: bodyX + nx + 8, y: bodyY + ny + 8, w: 264, h: 100}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + px, bodyY + py, 300, 150},
			{bodyX + px, bodyY + py + 36, 300, 100},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_upload.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit upload: close (%s)\n", win.Backend())
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
		up.Tick(dt)
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundarySkip >= 0 && snapH.PaintCount > 0
		shell.UpdateHUD("G6-upload", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d files=%d", snapH.PaintCount, app.PresentCount(), len(up.FileList())),
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
		{pt: &ptPanel, desc: "upload_panel_bg"},
		{pt: &ptBar, desc: "upload_progress_bar"},
		{text: &capTextBox, base: noteBG, desc: "caption_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_upload.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "G6-upload",
		Scenario:      "kit_upload",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"files":                  len(up.FileList()),
			"first_status":           string(firstStatus),
			"uploading":              up.UploadingCount(),
			"maxCount1":              mname,
			"png_filter":             fmt.Sprintf("%v/%v", pngOK, pngNo),
			"paste":                  pasteOK,
			"skin_holder":            skinUp.HolderContent(),
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
	fmt.Fprintf(os.Stderr, "kit upload: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
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
			fmt.Fprintf(os.Stderr, "kit upload: pixel %-24s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "kit upload: pixel %-24s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "showcase_upload.png")
	base := filepath.Join(snapDir, "showcase_upload_base.png")
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
