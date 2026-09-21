// Command kit_f0_content is the F0-3 content real window: single/multi/
// ellipsis/rich text plus icon row plus the picture three-state machine.
//
// RUN_SECONDS=15 go run ./examples/kit_f0_content
//
// Window: 1200x800 baseline, user-resizable. One scripted resize
// excursion restores the baseline before gates are read.
// Correctness window: slope_gate=off, fps gate on when steady, pixel
// probes use region density (never single-point glyph sampling).
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
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var textResult = map[string]bool{}
var pictureResult = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "F0-content")
	} else if *autoOnly {
		secs, secsSet = 15, true
	}
	_, faceName, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	hasFace := faceName != ""

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit_f0_content — label rich picture icon", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	ctx := kit.DefaultScopeCtx()
	tok := ctx.Theme
	textColor := tok.ColorText
	secondary := tok.ColorTextSecondary

	shell := wrkit.NewShell(winW, winH, "F0-3 content — text is real, picture has 3 states", []string{
		"single: one-line label",
		"multi: wrapped two lines",
		"ellipsis: maxLines 2 + …",
		"rich: two spans, two colors",
		"icons: 12/16/20 follow Ctx",
		"picture: idle->ready->error",
		"resize once, restore baseline",
		"slope_gate=off (15s window)",
	})

	face := wrkit.FaceAt(14)
	mkLabel := func(s string, size float64, maxW float64, maxLines int, ellipsis bool) *rendering.RenderText {
		lb := kit.PrimNewLabel(kit.PrimLabelProps{
			Text: s, MaxWidth: maxW, MaxLines: maxLines, Ellipsis: ellipsis,
			Seed: tok, Face: face,
			Style: kit.PrimTextStyle{FontSize: size, Color: textColor, HasColor: true, LineMult: tok.LineHeight},
		})
		if face == nil {
			lb.SetFace(nil)
		} else {
			wrkit.ApplyFace(lb, size)
		}
		lb.SetRepaintBoundary(true)
		return lb
	}

	single := mkLabel("F0-3 single line label", 14, 0, 0, false)
	shell.Body.Place(single, 8, 28)
	shell.Body.Box.Place(wrkit.Label("single", 11, 0.62, 0.72, 0.85), 8, 8)

	multi := mkLabel("F0-3 wrapped text band shows two lines of real glyphs", 14, 220, 0, false)
	shell.Body.Place(multi, 8, 90)
	shell.Body.Box.Place(wrkit.Label("multi", 11, 0.62, 0.72, 0.85), 8, 70)

	ellipsis := mkLabel("F0-3 ellipsis band line one line two line three line four", 14, 220, 2, true)
	shell.Body.Place(ellipsis, 8, 190)
	shell.Body.Box.Place(wrkit.Label("ellipsis maxLines=2", 11, 0.62, 0.72, 0.85), 8, 170)

	rich := kit.PrimNewRichLabel(kit.PrimRichProps{
		Spans: []kit.PrimSpan{
			{Text: "F0-3 rich ", Style: kit.PrimTextStyle{FontSize: 14, Color: textColor, HasColor: true}},
			{Text: "two colors", Style: kit.PrimTextStyle{FontSize: 14, Color: tok.ColorPrimary, HasColor: true}},
		},
		Seed: tok,
	})
	if face != nil {
		wrkit.ApplyFace(rich, 14)
		for _, r := range rich.Runs {
			_ = r
		}
		if len(rich.Runs) == 2 {
			runs := rich.Runs
			runs[0].Face = wrkit.FaceAt(14)
			runs[1].Face = wrkit.FaceAt(14)
			rich.SetRuns(runs)
		}
	}
	rich.SetRepaintBoundary(true)
	rich.SetDebugName("f0-rich")
	shell.Body.Place(rich, 280, 28)
	shell.Body.Box.Place(wrkit.Label("rich two spans", 11, 0.62, 0.72, 0.85), 280, 8)
	_ = secondary

	// Icon row: three boxes sized by Ctx tier (12/16/20).
	iconSpecs := []kit.PrimIconSpec{
		kit.PrimResolveIcon(ctx.WithSize(kit.ScopeSizeSmall), tok),
		kit.PrimResolveIcon(ctx.WithSize(kit.ScopeSizeMedium), tok),
		kit.PrimResolveIcon(ctx.WithSize(kit.ScopeSizeLarge), tok),
	}
	iconX := 280.0
	for i, spec := range iconSpecs {
		bx := kit.PrimIconBoxSize(spec)
		chip := rendering.NewRenderColorBox(bx.Width, bx.Height, spec.Color.R, spec.Color.G, spec.Color.B, spec.Color.A)
		chip.SetRepaintBoundary(true)
		chip.SetDebugName(fmt.Sprintf("f0-icon-%d", i))
		shell.Body.Place(chip, iconX, 90)
		iconX += bx.Width + 12
	}
	shell.Body.Box.Place(wrkit.Label("icons 12/16/20", 11, 0.62, 0.72, 0.85), 280, 70)

	// Picture band: placeholder now; ticker drives ready then error.
	picSpec := kit.PrimResolvePicture(220, 140, tok)
	pic := kit.PrimNewPicture(picSpec)
	pic.SetDebugName("f0-picture")
	pic.SetRepaintBoundary(true)
	picX, picY := 560.0, 28.0
	shell.Body.Place(pic, picX, picY)
	shell.Body.Box.Place(wrkit.Label("picture idle->ready->error", 11, 0.62, 0.72, 0.85), picX, 8)

	// Text band boxes for region probes (filled after layout).
	noteBG := [3]float64{0.10, 0.11, 0.13}

	snapDir := filepath.Join("examples", "kit_f0_content", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	var bands []textBand
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		bands = []textBand{
			{node: single, rect: rect{x: bodyX + 8, y: bodyY + 28, w: 300, h: 30}, base: noteBG, desc: "single_density"},
			{node: multi, rect: rect{x: bodyX + 8, y: bodyY + 90, w: 240, h: 60}, base: noteBG, desc: "multi_density"},
			{node: rich, rect: rect{x: bodyX + 280, y: bodyY + 28, w: 260, h: 30}, base: noteBG, desc: "rich_density"},
		}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + 8, bodyY + 28, 300, 30},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_content.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit_f0_content: close (%s)\n", win.Backend())
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

	paintBefore := int64(0)
	paintAtReady := int64(0)
	paintAtError := int64(0)
	phaseReady, phaseError := false, false
	var elapsed float64
	resizeDone, resizeBack := false, false
	// A small solid buffer stands in for decoded pixels: Picture only
	// asserts the state machine and one-cell dirty marking, never real IO.
	readyBuf := render.ImageBufFromImage(solidImage(8, 8, 70, 150, 255))
	// Dirty-marking evidence: captured synchronously right after each
	// transition, before the next flush. RepaintBoundary must stop the
	// mark at pic, so siblings stay clean and no layout is dirtied.
	var readyPicDirty, readySiblingClean, readyNoLayout bool
	var errorPicDirty, errorSiblingClean, errorNoLayout bool
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
		if !phaseReady && elapsed >= 5.0 {
			phaseReady = true
			paintBefore = app.Metrics().Snapshot().PaintCount
			pic.SetImageShared(readyBuf)
			readyPicDirty = pic.NeedsPaint()
			readySiblingClean = !single.NeedsPaint() && !multi.NeedsPaint()
			readyNoLayout = !pic.NeedsLayout() && !single.NeedsLayout() && !shell.Root.NeedsLayout()
		}
		if phaseReady && !phaseError && elapsed >= 9.0 {
			phaseError = true
			paintAtReady = app.Metrics().Snapshot().PaintCount
			pic.SetError()
			errorPicDirty = pic.NeedsPaint()
			errorSiblingClean = !single.NeedsPaint() && !multi.NeedsPaint()
			errorNoLayout = !pic.NeedsLayout() && !single.NeedsLayout() && !shell.Root.NeedsLayout()
		}
		if phaseError && paintAtError == 0 && elapsed >= 10.0 {
			paintAtError = app.Metrics().Snapshot().PaintCount
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundarySkip >= 0 && snapH.PaintCount > 0
		shell.UpdateHUD("F0-content", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("resize=%d pic=%d t=%.1f", resizeEvents, int(kit.PrimPicturePhase(pic)), elapsed))
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
	img := loadImage(filepath.Join(snapDir, "showcase_content.png"))
	runTextChecks(img, bands, textResult, hasFace)
	readyDelta := paintAtReady - paintBefore
	errorDelta := paintAtError - paintAtReady
	pictureResult["picture_ready_phase"] = phaseReady && phaseError && kit.PrimPicturePhase(pic) == kit.PrimPictureError
	pictureResult["picture_ready_one_cell"] = phaseReady && readyPicDirty && readySiblingClean && readyNoLayout
	pictureResult["picture_error_one_cell"] = phaseError && errorPicDirty && errorSiblingClean && errorNoLayout
	for k, v := range pictureResult {
		fmt.Fprintf(os.Stderr, "kit_f0_content: picture %-22s ok=%v (readyDirty=%v sibClean=%v noLayout=%v errDirty=%v sibClean=%v noLayout=%v frames readyΔ=%d errorΔ=%d)\n",
			k, v, readyPicDirty, readySiblingClean, readyNoLayout, errorPicDirty, errorSiblingClean, errorNoLayout, readyDelta, errorDelta)
	}
	scriptedOK, scriptedTotal := 0, 0
	for _, ok := range textResult {
		scriptedTotal++
		if ok {
			scriptedOK++
		}
	}
	for _, ok := range pictureResult {
		scriptedTotal++
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "F0-content",
		Scenario:      "kit_f0_content",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"has_face":               hasFace,
			"face_name":              faceName,
			"text_ok":                scriptedOK,
			"text_total":             scriptedTotal,
			"picture_phase":          int(kit.PrimPicturePhase(pic)),
			"picture_ready_dirty":    readyPicDirty,
			"picture_ready_sibclean": readySiblingClean,
			"picture_ready_nolayout": readyNoLayout,
			"picture_error_dirty":    errorPicDirty,
			"picture_error_sibclean": errorSiblingClean,
			"picture_error_nolayout": errorNoLayout,
			"picture_ready_delta":    readyDelta,
			"picture_error_delta":    errorDelta,
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
	if !hasFace {
		fmt.Fprintln(os.Stderr, "FAIL: no UI face available (need real glyphs)")
		os.Exit(1)
	}
	if scriptedTotal != 6 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 6/6 (text density + picture)\n", scriptedOK, scriptedTotal)
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
	fmt.Fprintf(os.Stderr, "kit_f0_content: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiffPct, resizeBack, finalW, finalH, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func solidImage(w, h int, r, g, b uint8) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = r, g, b, 255
		}
	}
	return img
}

type textBand struct {
	node *rendering.RenderText
	rect rect
	base [3]float64
	desc string
}

func runTextChecks(img image.Image, bands []textBand, result map[string]bool, hasFace bool) {
	for _, b := range bands {
		n := textPixels(img, 1.0, b.rect, b.base)
		ok := img != nil && hasFace && n >= 60
		fmt.Fprintf(os.Stderr, "kit_f0_content: text %-16s ink_px=%d (want >=60) ok=%v\n", b.desc, n, ok)
		result[b.desc] = ok
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
	cur := filepath.Join(snapDir, "showcase_content.png")
	base := filepath.Join(snapDir, "showcase_content_base.png")
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
