// Command qrcode is the G7 real window: QR matrix plus covers.
//
// RUN_SECONDS=5 go run ./examples/kit/qrcode
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
	"github.com/energye/gpui/render"
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

// rasterizeQRCode paints the raw matrix into one image buffer at
// pxPerModule pixels per module (margin included, quiet zone white).
func rasterizeQRCode(matrix [][]bool, margin int, fg, bg theme.Color, pxPerModule int) *render.ImageBuf {
	n := len(matrix)
	total := n + margin*2
	buf, err := render.NewImageBuf(total*pxPerModule, total*pxPerModule, render.FormatRGBA8)
	if err != nil || buf == nil {
		return nil
	}
	fr, fg8, fb8 := toU8(fg), toU8g(fg), toU8b(fg)
	br, bg8, bb8 := toU8(bg), toU8g(bg), toU8b(bg)
	for y := 0; y < total*pxPerModule; y++ {
		for x := 0; x < total*pxPerModule; x++ {
			mr, mc := y/pxPerModule-margin, x/pxPerModule-margin
			dark := mr >= 0 && mr < n && mc >= 0 && mc < n && matrix[mr][mc]
			if dark {
				_ = buf.SetRGBA(x, y, fr, fg8, fb8, 255)
			} else {
				_ = buf.SetRGBA(x, y, br, bg8, bb8, 255)
			}
		}
	}
	return buf
}

func toU8(c theme.Color) uint8  { return uint8(math.Round(c.R * 255)) }
func toU8g(c theme.Color) uint8 { return uint8(math.Round(c.G * 255)) }
func toU8b(c theme.Color) uint8 { return uint8(math.Round(c.B * 255)) }

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "G7-qrcode")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit qrcode — G7 matrix+covers", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// Real code: antd home URL, boosted H, bordered 160.
	baseCtx := kit.DefaultScopeCtx()
	skinTokens := baseCtx.Theme
	skinTokens.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := baseCtx.WithTheme(skinTokens)
	props := kit.DefaultQRCodeProps()
	props.Value = "https://ant.design"
	qr := kit.BuildQRCode(baseCtx, props)
	skinQR := kit.BuildQRCode(skinCtx, props)

	// Sibling hosts prove covers/icon/color without touching the probe.
	eprops := kit.DefaultQRCodeProps()
	eprops.Value = "https://ant.design"
	eprops.Status = kit.QRCodeStatusExpired
	expiredQR := kit.BuildQRCode(baseCtx, eprops)
	expiredQR.SetOnRefresh(func() {})
	lprops := kit.DefaultQRCodeProps()
	lprops.Value = "https://ant.design"
	lprops.Status = kit.QRCodeStatusLoading
	loadingQR := kit.BuildQRCode(baseCtx, lprops)
	sprops := kit.DefaultQRCodeProps()
	sprops.Value = "https://ant.design"
	sprops.Status = kit.QRCodeStatusScanned
	scannedQR := kit.BuildQRCode(baseCtx, sprops)
	iprops := kit.DefaultQRCodeProps()
	iprops.Value = "https://ant.design"
	iprops.Icon = "https://example.com/logo.png"
	iconQR := kit.BuildQRCode(baseCtx, iprops)
	cprops := kit.DefaultQRCodeProps()
	cprops.Value = "hi"
	cprops.Color, cprops.ColorSet = "#ff0000", true
	cprops.BgColor, cprops.BgColorSet = "#ffff00", true
	colorQR := kit.BuildQRCode(baseCtx, cprops)

	resolved := kit.ResolveQRCode(baseCtx.Theme, "", "")
	resolvedSkin := kit.ResolveQRCode(skinCtx.Theme, "", "")
	edge := kit.ResolveQRCodeSize(props)
	lay := kit.ComputeQRCodeLayout(edge, true, 40, 40, iconQR.HasIcon())
	matrix := qr.Matrix()
	white := theme.Hex("#ffffff")
	img := rasterizeQRCode(matrix, 0, resolved.Module, white, 4)

	shell := wrkit.NewShell(winW, winH, "G7 qrcode — matrix plus covers", []string{
		"matrix: 25 modules finder ok",
		"boost: M lifts to H live",
		"right: expired loading scanned",
		"far right: reskinned holder",
		"bottom: icon color size live",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	// Main card: bordered 160 panel, matrix image, title.
	panel := rendering.NewAbsoluteBox(184, 210)
	panel.Background = &rendering.Color{R: resolved.RootBg.R, G: resolved.RootBg.G, B: resolved.RootBg.B, A: 1}
	panel.SetDebugName("g7-qr-panel")
	panel.SetRepaintBoundary(true)
	panel.Place(wrkit.Label("qrcode 160 bordered M", 12, 0.12, 0.14, 0.16), 12, 8)
	shell.Body.Place(panel, 24, 24)
	if img != nil {
		node := rendering.NewRenderImage(lay.Content.W, lay.Content.H)
		node.SetImage(img)
		node.SetDebugName("g7-qr-matrix")
		panel.Place(node, lay.Content.X, lay.Content.Y+26)
	}
	// Solid module swatch: pixel probe target (scaled matrix blurs).
	swatch := rendering.NewAbsoluteBox(24, 24)
	swatch.Background = &rendering.Color{R: resolved.Module.R, G: resolved.Module.G, B: resolved.Module.B, A: 1}
	swatch.SetDebugName("g7-qr-swatch")
	panel.Place(swatch, 12, 178)
	panel.Place(wrkit.Label(fmt.Sprintf("modules=%d level=%s", qr.Modules(), qr.UsedLevel()), 10, 0.30, 0.32, 0.40), 44, 184)
	// Cover cards: expired/loading/scanned each on its own host.
	coverCard := rendering.NewAbsoluteBox(280, 150)
	coverCard.Background = &rendering.Color{R: dimBG[0], G: dimBG[1], B: dimBG[2], A: 1}
	coverCard.SetDebugName("g7-qr-covers")
	coverCard.SetRepaintBoundary(true)
	shell.Body.Place(coverCard, 224, 24)
	coverCard.Place(wrkit.Label("covers expired loading scanned", 11, 0.62, 0.72, 0.85), 10, 8)
	coverCard.Place(wrkit.Label("expired: "+expiredQR.CoverText(), 10, 0.55, 0.65, 0.78), 10, 32)
	coverCard.Place(wrkit.Label(fmt.Sprintf("loading spin=%.0f cover=%v", loadingQR.SpinAngle(), loadingQR.HasCover()), 10, 0.55, 0.65, 0.78), 10, 54)
	coverCard.Place(wrkit.Label("scanned: "+scannedQR.CoverText(), 10, 0.55, 0.65, 0.78), 10, 76)
	coverCard.Place(wrkit.Label(fmt.Sprintf("refresh once=%v", expiredQR.ClickRefresh()), 10, 0.55, 0.65, 0.78), 10, 98)
	_ = loadingQR
	// Icon + color + size card.
	featCard := rendering.NewAbsoluteBox(280, 150)
	featCard.Background = &rendering.Color{R: dimBG[0], G: dimBG[1], B: dimBG[2], A: 1}
	featCard.SetDebugName("g7-qr-feats")
	featCard.SetRepaintBoundary(true)
	shell.Body.Place(featCard, 224, 186)
	featCard.Place(wrkit.Label("icon color size live", 11, 0.62, 0.72, 0.85), 10, 8)
	featCard.Place(wrkit.Label(fmt.Sprintf("icon=%v 40x40", iconQR.HasIcon()), 10, 0.55, 0.65, 0.78), 10, 32)
	featCard.Place(wrkit.Label(fmt.Sprintf("color modules=%d", colorQR.Modules()), 10, 0.55, 0.65, 0.78), 10, 54)
	featCard.Place(wrkit.Label("size 160 pad 12 r8", 10, 0.55, 0.65, 0.78), 10, 76)
	featCard.Place(wrkit.Label(fmt.Sprintf("empty=0 values=%d", len(iconQR.Values())), 10, 0.55, 0.65, 0.78), 10, 98)
	cS := rendering.NewAbsoluteBox(260, 130)
	cS.Background = &rendering.Color{R: resolvedSkin.RootBg.R, G: resolvedSkin.RootBg.G, B: resolvedSkin.RootBg.B, A: 1}
	cS.SetDebugName("g7-qr-reskin")
	cS.SetRepaintBoundary(true)
	cS.Place(wrkit.Label("reskin static #722ed1", 12, 0.12, 0.14, 0.16), 12, 10)
	cS.Place(wrkit.Label(skinQR.HolderContent(), 10, 0.30, 0.22, 0.55), 12, 36)
	cS.Place(wrkit.Label("static eats theme", 10, 0.30, 0.32, 0.40), 12, 58)
	cS.Place(wrkit.Label(fmt.Sprintf("skin modules=%d", skinQR.Modules()), 10, 0.30, 0.32, 0.40), 12, 80)
	shell.Body.Place(cS, 524, 24)
	capNote := rendering.NewAbsoluteBox(280, 130)
	capNote.Background = &rendering.Color{R: noteBG[0], G: noteBG[1], B: noteBG[2], A: 1}
	capNote.SetDebugName("g7-qr-cap")
	capNote.Place(wrkit.Label("skip2 backend swappable", 11, 0.62, 0.72, 0.85), 8, 8)
	capNote.Place(wrkit.Label("finder dark white dark", 10, 0.55, 0.65, 0.78), 8, 30)
	capNote.Place(wrkit.Label("same value same code", 10, 0.55, 0.65, 0.78), 8, 52)
	capNote.Place(wrkit.Label(fmt.Sprintf("backend=%s", qr.Generate().Name()), 10, 0.55, 0.65, 0.78), 8, 74)
	shell.Body.Place(capNote, 524, 170)
	// Margin + excavate card: margin-2 raster and icon-dug raster prove
	// the draw-layer helpers on live matrices (outside golden mask).
	marginCard := rendering.NewAbsoluteBox(300, 190)
	marginCard.Background = &rendering.Color{R: dimBG[0], G: dimBG[1], B: dimBG[2], A: 1}
	marginCard.SetDebugName("g7-qr-margin")
	marginCard.SetRepaintBoundary(true)
	shell.Body.Place(marginCard, 24, 250)
	marginCard.Place(wrkit.Label("margin 2 excavate live", 11, 0.62, 0.72, 0.85), 10, 8)
	helloMatrix, _ := kit.DefaultQRCodeGenerateConfig().Encode("hello", kit.QRErrorLevelM)
	if len(helloMatrix) > 0 {
		mimg := rasterizeQRCode(helloMatrix, 2, resolved.Module, white, 3)
		if mimg != nil {
			mnode := rendering.NewRenderImage(75, 75)
			mnode.SetImage(mimg)
			mnode.SetDebugName("g7-qr-margin-img")
			marginCard.Place(mnode, 12, 32)
		}
		marginCard.Place(wrkit.Label(fmt.Sprintf("margin2 edge=%d", kit.QRCodePaddedSize(len(helloMatrix), 2)), 10, 0.55, 0.65, 0.78), 95, 40)
		marginCard.Place(wrkit.Label("origin shifts 2mod", 10, 0.55, 0.65, 0.78), 95, 62)
		dug := kit.ExcavateQRCodeIcon(helloMatrix, (63-15)/2, (63-15)/2, 15, 15, 3)
		dimg := rasterizeQRCode(dug, 0, resolved.Module, white, 3)
		if dimg != nil {
			dnode := rendering.NewRenderImage(63, 63)
			dnode.SetImage(dimg)
			dnode.SetDebugName("g7-qr-dug-img")
			marginCard.Place(dnode, 12, 115)
		}
		cleared := 0
		for r := range dug {
			for c := range dug[r] {
				if helloMatrix[r][c] && !dug[r][c] {
					cleared++
				}
			}
		}
		marginCard.Place(wrkit.Label(fmt.Sprintf("dug 5x5 clears %d", cleared), 10, 0.55, 0.65, 0.78), 85, 125)
		marginCard.Place(wrkit.Label("finder survives", 10, 0.55, 0.65, 0.78), 85, 147)
	}

	snapDir := filepath.Join("examples", "kit", "qrcode", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	ptPanel := probePoint{want: [3]float64{resolved.RootBg.R, resolved.RootBg.G, resolved.RootBg.B}, tol: 8.0 / 255}
	ptSwatch := probePoint{want: [3]float64{resolved.Module.R, resolved.Module.G, resolved.Module.B}, tol: 8.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		px, py := panel.Offset().X, panel.Offset().Y
		ptPanel.x = bodyX + px + 184 - 16
		ptPanel.y = bodyY + py + 10
		sx, sy := swatch.Offset().X, swatch.Offset().Y
		ptSwatch.x = bodyX + px + sx + 12
		ptSwatch.y = bodyY + py + sy + 12
		nx, ny := capNote.Offset().X, capNote.Offset().Y
		capTextBox = rect{x: bodyX + nx + 8, y: bodyY + ny + 8, w: 264, h: 100}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + px, bodyY + py, 184, 210},
			{bodyX + px + 12, bodyY + py + 38, 136, 136},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_qrcode.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit qrcode: close (%s)\n", win.Backend())
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
		shell.UpdateHUD("G7-qrcode", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d modules=%d", snapH.PaintCount, app.PresentCount(), qr.Modules()),
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
		{pt: &ptPanel, desc: "qr_panel_bg"},
		{pt: &ptSwatch, desc: "qr_module_swatch"},
		{text: &capTextBox, base: noteBG, desc: "caption_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_qrcode.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "G7-qrcode",
		Scenario:      "kit_qrcode",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"backend":                qr.Generate().Name(),
			"modules":                qr.Modules(),
			"used_level":             string(qr.UsedLevel()),
			"has_cover_expired":      expiredQR.HasCover(),
			"has_icon":               iconQR.HasIcon(),
			"skin_holder":            skinQR.HolderContent(),
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
	fmt.Fprintf(os.Stderr, "kit qrcode: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
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
			fmt.Fprintf(os.Stderr, "kit qrcode: pixel %-24s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "kit qrcode: pixel %-24s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "showcase_qrcode.png")
	base := filepath.Join(snapDir, "showcase_qrcode_base.png")
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
