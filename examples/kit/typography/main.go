// Command kit_typography is the F1 Typography real window (official
// §2.4 non-debug showcase): all ten examples laid out in one window,
// three-evidence gate (logic scripted probes + pixel probes + bitwise
// golden) plus a resize round trip.
//
// RUN_SECONDS=15 go run ./examples/kit/typography
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

type rect struct{ x, y, w, h float64 }

var scripted = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "G7-typography")
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
		Title: "gpui kit typography — official showcase", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	ctx := kit.DefaultScopeCtx()
	seed := ctx.Theme

	shell := wrkit.NewShell(winW, winH, "G7 typography — text title paragraph link", []string{
		"basic: text/paragraph/link rows",
		"title: h1..h5 ladder 38..16",
		"decor: strong code mark delete underline kbd",
		"ellipsis: rows=2 cut + suffix reserved",
		"middle: head+mark+tail fixed cut",
		"copy/edit: OnCopy fires, editing Enter",
		"resize once, restore baseline",
		"slope_gate=off (15s window)",
	})
	body := shell.Body

	// ---- geometry: two columns of example cards ----
	const cardW, cardH = 440.0, 96.0
	colX := [2]float64{16, 460}
	rowY := [4]float64{14, 118, 222, 326}
	placeCard := func(title string, r, c int) (float64, float64) {
		x, y := colX[c], rowY[r]+26
		body.Place(wrkit.Label(title, 12, 0.55, 0.72, 0.88), colX[c], rowY[r])
		_ = title
		return x, y
	}
	_ = cardW
	_ = cardH

	var snapDir = filepath.Join("examples", "kit", "typography", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	// Apply the window face to a built typography node.
	applyFace := func(inst *kit.TypographyInstance) {
		rt, ok := inst.Node().(*rendering.RenderText)
		if ok && rt != nil {
			if face := wrkit.FaceAt(rt.FontSize); face != nil {
				rt.SetFace(face)
			}
		}
	}

	geoValid := false
	resizeEvents := 0
	var goldenRects []rect
	// probe rects are computed from card origins at resolve time.
	type probe struct {
		name string
		rect
	}
	var probes []probe
	var textProbes []probe
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		// Whole-body content rect (top bar/HUD/legend masked by Shell rects).
		goldenRects = []rect{
			{x: bodyX, y: bodyY, w: body.W, h: body.H},
		}
		// Text ink probes: each example card's title row + one content row.
		for _, p := range []probe{
			{name: "basic_text", rect: rect{colX[0], rowY[0] + 30, 300, 20}},
			{name: "title_h1", rect: rect{colX[1], rowY[0] + 30, 300, 44}},
			{name: "decor_row", rect: rect{colX[0], rowY[1] + 30, 420, 24}},
			{name: "ellipsis_row", rect: rect{colX[1], rowY[1] + 30, 420, 24}},
			{name: "middle_row", rect: rect{colX[0], rowY[2] + 30, 420, 24}},
			{name: "copyedit_row", rect: rect{colX[1], rowY[2] + 30, 420, 24}},
		} {
			p.rect.x += bodyX
			p.rect.y += bodyY
			textProbes = append(textProbes, p)
		}
		_ = probes
		geoValid = true
	}

	// ================= card 1: 基本 (text/paragraph/link rows) =================
	{
		x, y := placeCard("basic: Text Paragraph Link", 0, 0)
		tp := kit.DefaultTypographyProps()
		tp.Value = "Ant Design — a design language."
		textInst, textNode := kit.BuildTypographyText(ctx, tp)
		textNode.Root.(*rendering.RenderText).SetMaxWidth(330)
		body.Place(textNode.Root, x, y)
		applyFace(textInst)

		pp := kit.DefaultTypographyProps()
		pp.Kind = kit.TypographyParagraph
		pp.Value = "Paragraph: internal desktop systems."
		paraInst, paraNode := kit.BuildTypography(ctx, pp)
		// One wrapped line each, clamped to the card column; the paragraph
		// sits below the two-line text row without reaching the next card.
		paraNode.Root.(*rendering.RenderText).SetMaxWidth(330)
		body.Place(paraNode.Root, x, y+42)
		applyFace(paraInst)

		lp := kit.DefaultTypographyProps()
		lp.Value = "Link"
		linkInst, linkNode := kit.BuildTypographyLink(ctx, lp)
		body.Place(linkNode.Root, x+180, y+6)
		applyFace(linkInst)
	}

	// ================= card 2: 标题组件 (h1..h5 ladder) =================
	{
		x, y := placeCard("title: h1..h5", 0, 1)
		// Step by the ladder itself (38/30/24/20/16 ≈ ×2.2 em width for
		// the two-char label + 16 gap ≈ 340px total) so larger titles get
		// proportionally more room without leaving the card — the first
		// seeding showed both overlap (fixed pitch) and overflow (4.4em).
		steps := []float64{38 * 2.2, 30 * 2.2, 24 * 2.2, 20 * 2.2, 16 * 2.2}
		xx := x
		for i, lv := range []int{1, 2, 3, 4, 5} {
			tp := kit.DefaultTypographyProps()
			tp.Kind = kit.TypographyTitle
			tp.Level = lv
			tp.Value = fmt.Sprintf("h%d", lv)
			inst, node := kit.BuildTypographyTitle(ctx, tp)
			body.Place(node.Root, xx, y)
			applyFace(inst)
			xx += steps[i] + 12
		}
	}

	// ================= card 3: 修饰 (strong/code/mark/delete/underline/kbd) =================
	{
		x, y := placeCard("decor: strong code mark delete underline kbd", 1, 0)
		deco := []struct {
			label string
			mut   func(*kit.TypographyProps)
		}{
			{"Strong", func(p *kit.TypographyProps) { p.Strong = true }},
			{"Code", func(p *kit.TypographyProps) { p.Code = true }},
			{"Mark", func(p *kit.TypographyProps) { p.Mark = true }},
			{"Delete", func(p *kit.TypographyProps) { p.Delete = true }},
			{"Underline", func(p *kit.TypographyProps) { p.Underline = true }},
			{"Keyboard", func(p *kit.TypographyProps) { p.Keyboard = true }},
		}
		xx := x
		for _, d := range deco {
			tp := kit.DefaultTypographyProps()
			tp.Value = d.label
			d.mut(&tp)
			inst, node := kit.BuildTypographyText(ctx, tp)
			body.Place(node.Root, xx, y)
			applyFace(inst)
			xx += float64(len(d.label))*9 + 14
		}
	}

	// ================= card 4: 省略号 (rows=2 binary cut) =================
	{
		x, y := placeCard("ellipsis: rows=2 binary cut", 1, 1)
		tp := kit.DefaultTypographyProps()
		tp.Value = strings.Repeat("Ant Design is a design language ", 8)
		tp.Ellipsis = kit.TypographyEllipsis{On: true, Rows: 2, Expandable: true}
		inst, node := kit.BuildTypographyParagraph(ctx, tp)
		node.Root.(*rendering.RenderText).SetMaxWidth(420)
		body.Place(node.Root, x, y)
		applyFace(inst)
	}

	// ================= card 5: 省略中间 (head+mark+tail) =================
	{
		x, y := placeCard("middle: head+mark+tail", 2, 0)
		tp := kit.DefaultTypographyProps()
		tp.Value = strings.Repeat("Like ants", 12) + " with discipline"
		tp.Ellipsis = kit.TypographyEllipsis{On: true, Middle: true}
		inst, node := kit.BuildTypographyText(ctx, tp)
		// Fixed head/tail double cut then clamp the draw width so the
		// result stays inside the card (observed crossing into the
		// copy/edit card in the seeded golden).
		node.Root.(*rendering.RenderText).SetMaxWidth(420)
		body.Place(node.Root, x, y)
		applyFace(inst)
	}

	// ================= card 6: 可复制/可编辑 (state machines live) =================
	{
		x, y := placeCard("copy/edit: OnCopy + editing", 2, 1)
		cp := kit.DefaultTypographyProps()
		cp.Value = "Copyable Text."
		copyCalls := 0
		cp.Copyable = kit.TypographyCopyable{On: true, OnCopy: func(string) { copyCalls++ }}
		cpInst, cpNode := kit.BuildTypographyText(ctx, cp)
		body.Place(cpNode.Root, x, y)
		applyFace(cpInst)

		ep := kit.DefaultTypographyProps()
		ep.Value = "Editable Text."
		var committed string
		ep.Editable = kit.TypographyEditable{On: true, OnChange: func(v string) { committed = v }}
		epInst, epNode := kit.BuildTypographyText(ctx, ep)
		body.Place(epNode.Root, x, y+24)
		applyFace(epInst)

		// Scripted state-machine probes (logic evidence): drive the
		// machines through their public verbs and record the outcomes.
		if payload := cpInst.Copy(); payload != "Copyable Text." || copyCalls != 1 {
			scripted["copy_ok"] = false
		} else {
			scripted["copy_ok"] = true
		}
		epInst.StartEditing()
		epInst.SetEditingValue("Edited!")
		epInst.CommitEditing()
		scripted["edit_ok"] = committed == "Edited!" && !epInst.Editing()
		epInst.StartEditing()
		epInst.CancelEditing()
		scripted["edit_cancel_ok"] = !epInst.Editing()
		_ = epNode
	}

	// ================= card 7: 后缀 (P1 suffix) =================
	{
		x, y := placeCard("suffix: ellipsis keeps suffix visible", 3, 0)
		tp := kit.DefaultTypographyProps()
		tp.Value = strings.Repeat("spacing ", 30)
		tp.Ellipsis = kit.TypographyEllipsis{On: true, Rows: 1, Suffix: "--with suffix"}
		inst, node := kit.BuildTypographyText(ctx, tp)
		node.Root.(*rendering.RenderText).SetMaxWidth(400)
		body.Place(node.Root, x, y)
		applyFace(inst)
	}

	// ================= card 8: 表格 (P1 tabular composition) =================
	{
		x, y := placeCard("table: tabular typography", 3, 1)
		headers := []string{"Name", "Age", "Address"}
		for i, h := range headers {
			tp := kit.DefaultTypographyProps()
			tp.Value = h
			tp.Strong = true
			inst, node := kit.BuildTypographyText(ctx, tp)
			body.Place(node.Root, x+float64(i)*140, y)
			applyFace(inst)
		}
		tp := kit.DefaultTypographyProps()
		tp.Value = "Joe Black 32 Sydney No.1 Park"
		inst, node := kit.BuildTypographyText(ctx, tp)
		body.Place(node.Root, x, y+22)
		applyFace(inst)
	}

	// ---- scripted logic evidence: title ladder via tokens ----
	{
		want := []float64{38, 30, 24, 20, 16}
		ok := true
		for lv := 1; lv <= 5; lv++ {
			res := kit.ResolveTypography(seed, kit.TypographyProps{Kind: kit.TypographyTitle, Level: lv}, false)
			if res.FontSize != want[lv-1] {
				ok = false
			}
		}
		scripted["title_ladder"] = ok
		scripted["body_14"] = kit.ResolveTypography(seed, kit.DefaultTypographyProps(), false).FontSize == 14
	}

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_typography.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit typography: close (%s)\n", win.Backend())
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
		gateOK := snapH.PaintCount > 0
		shell.UpdateHUD("G7-typography", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
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
	img := loadImage(filepath.Join(snapDir, "showcase_typography.png"))
	// Pixel probes: text ink density per example card (body bg ≈ 0.10-0.13).
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, p := range textProbes {
		n := textPixels(img, dpr, p.rect, [3]float64{0.10, 0.11, 0.13})
		ok := img != nil && n >= 60
		scripted["ink_"+p.name] = ok
		fmt.Fprintf(os.Stderr, "kit typography: pixel %-18s ink_px=%d (want >=60) ok=%v\n", p.name, n, ok)
	}

	scriptedOK, scriptedTotal := 0, len(scripted)
	for _, k := range []string{"copy_ok", "edit_ok", "edit_cancel_ok", "title_ladder", "body_14",
		"ink_basic_text", "ink_title_h1", "ink_decor_row", "ink_ellipsis_row", "ink_middle_row", "ink_copyedit_row"} {
		if scripted[k] {
			scriptedOK++
		}
	}
	scriptedTotal = 11

	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "G7-typography",
		Scenario:      "kit_typography",
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
	if scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d\n", scriptedOK, scriptedTotal)
		for k, v := range scripted {
			if !v {
				fmt.Fprintf(os.Stderr, "  red: %s\n", k)
			}
		}
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
	fmt.Fprintf(os.Stderr, "kit typography: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiffPct, resizeBack, finalW, finalH, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
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
	cur := filepath.Join(snapDir, "showcase_typography.png")
	base := filepath.Join(snapDir, "showcase_typography_base.png")
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
		return 100, 0, fmt.Errorf("load image")
	}
	dprA := float64(a.Bounds().Dx()) / winW
	dprB := float64(b.Bounds().Dx()) / winW
	if math.Abs(dprA-dprB) > 1e-9 {
		return 100, 0, fmt.Errorf("dpr mismatch %v vs %v", dprA, dprB)
	}
	var diffPx, allPx int64
	for _, rc := range rects {
		x0, y0 := int(rc.x*dprA), int(rc.y*dprA)
		x1, y1 := int((rc.x+rc.w)*dprA), int((rc.y+rc.h)*dprA)
		for py := y0; py < y1 && py < a.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < a.Bounds().Dx(); px++ {
				ar, ag, ab, _ := a.At(px, py).RGBA()
				br, bg, bb, _ := b.At(px, py).RGBA()
				allPx++
				if ar != br || ag != bg || ab != bb {
					diffPx++
				}
			}
		}
	}
	if allPx == 0 {
		return 100, 0, fmt.Errorf("no comparison pixels")
	}
	return float64(diffPx) / float64(allPx) * 100, allPx, nil
}
