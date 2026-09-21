// Command kit_f0_interact is the F0-4 interaction real window: pressable
// states plus field control plus overlay trigger placement.
//
// RUN_SECONDS=15 go run ./examples/kit_f0_interact
//
// Window: 1200x800 baseline, user-resizable. One scripted resize excursion
// restores the baseline before gates are read. Correctness window:
// slope_gate=off, fps gate on when steady. Focus ring is asserted as
// logic (pointer never lights it, keyboard does); ring pixels arrive
// with the F2 Button标杆.
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
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var scripted = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "F0-interact")
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
		Title: "gpui kit_f0_interact — press field overlay", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	ctx := kit.DefaultScopeCtx()
	tok := ctx.Theme

	mgr := focus.NewManager()
	ov := overlay.New()

	click := kit.NewBehaviorInteractive(kit.BehaviorInteractiveConfig{Focusable: true, Label: "Submit"})
	click.Mount(mgr)
	defer click.Unmount()
	disabled := kit.NewBehaviorInteractive(kit.BehaviorInteractiveConfig{Disabled: true, Focusable: true, Label: "Disabled"})
	disabled.Mount(mgr)
	defer disabled.Unmount()
	loading := kit.NewBehaviorInteractive(kit.BehaviorInteractiveConfig{Loading: true, Focusable: true, Label: "Loading"})
	loading.Mount(mgr)
	defer loading.Unmount()

	var controlledNotify []string
	controlledVal := "hello"
	uncontrolled := kit.NewBehaviorField(kit.BehaviorFieldConfig{DefaultValue: ""})
	controlled := kit.NewBehaviorField(kit.BehaviorFieldConfig{
		Value:    &controlledVal,
		OnChange: func(s string) { controlledNotify = append(controlledNotify, s) },
	})
	composing := kit.NewBehaviorField(kit.BehaviorFieldConfig{DefaultValue: ""})

	trigger := kit.NewBehaviorTrigger(kit.BehaviorTriggerConfig{
		Want:      overlay.Bottom,
		Barrier:   true,
		FocusTrap: true,
		EscCloses: true,
	}, mgr, ov)

	shell := wrkit.NewShell(winW, winH, "F0-4 interact — press field overlay", []string{
		"click: hover/press counts once",
		"disabled: taps never count",
		"loading: double tap once=none",
		"fields: controlled/uncontrolled",
		"overlay: 4 dirs + flip + esc",
		"focus: tab moves, ring keys only",
		"resize once, restore baseline",
		"slope_gate=off (15s window)",
	})

	setBox := func(b *rendering.RenderColorBox, c [3]float64) {
		b.R, b.G, b.B, b.A = c[0], c[1], c[2], 1
		b.MarkNeedsPaint()
	}
	colPrimary := [3]float64{tok.ColorPrimary.R, tok.ColorPrimary.G, tok.ColorPrimary.B}
	colHover := [3]float64{tok.ColorPrimaryHover.R, tok.ColorPrimaryHover.G, tok.ColorPrimaryHover.B}
	colActive := [3]float64{tok.ColorPrimaryActive.R, tok.ColorPrimaryActive.G, tok.ColorPrimaryActive.B}
	colDisabled := [3]float64{tok.ColorBgContainerDisabled.R, tok.ColorBgContainerDisabled.G, tok.ColorBgContainerDisabled.B}
	colLoading := [3]float64{tok.ColorFillSecondary.R, tok.ColorFillSecondary.G, tok.ColorFillSecondary.B}
	colElevated := [3]float64{tok.ColorBgElevated.R, tok.ColorBgElevated.G, tok.ColorBgElevated.B}
	colAnchor := [3]float64{tok.ColorPrimaryBorder.R, tok.ColorPrimaryBorder.G, tok.ColorPrimaryBorder.B}

	clickBox := rendering.NewRenderColorBox(220, 64, colPrimary[0], colPrimary[1], colPrimary[2], 1)
	clickBox.SetRepaintBoundary(true)
	clickBox.SetDebugName("f0-click")
	shell.Body.Place(clickBox, 16, 40)
	shell.Body.Box.Place(wrkit.Label("click hover/press", 11, 0.62, 0.72, 0.85), 16, 20)

	disBox := rendering.NewRenderColorBox(220, 64, colDisabled[0], colDisabled[1], colDisabled[2], 1)
	disBox.SetRepaintBoundary(true)
	disBox.SetDebugName("f0-disabled")
	shell.Body.Place(disBox, 252, 40)
	shell.Body.Box.Place(wrkit.Label("disabled taps=0", 11, 0.62, 0.72, 0.85), 252, 20)

	loadBox := rendering.NewRenderColorBox(220, 64, colLoading[0], colLoading[1], colLoading[2], 1)
	loadBox.SetRepaintBoundary(true)
	loadBox.SetDebugName("f0-loading")
	shell.Body.Place(loadBox, 488, 40)
	shell.Body.Box.Place(wrkit.Label("loading double=0", 11, 0.62, 0.72, 0.85), 488, 20)

	anchorBox := rendering.NewRenderColorBox(160, 48, colAnchor[0], colAnchor[1], colAnchor[2], 1)
	anchorBox.SetRepaintBoundary(true)
	anchorBox.SetDebugName("f0-anchor")
	const anchorBX, anchorBY = 32.0, 140.0
	shell.Body.Place(anchorBox, anchorBX, anchorBY)
	shell.Body.Box.Place(wrkit.Label("anchor overlay bottom", 11, 0.62, 0.72, 0.85), anchorBX, 120)

	overlayBox := rendering.NewRenderColorBox(200, 120, colElevated[0], colElevated[1], colElevated[2], 1)
	overlayBox.SetRepaintBoundary(true)
	overlayBox.SetDebugName("f0-overlay")
	overlayBox.MoveTo(-500, -500)
	shell.Body.Place(overlayBox, -500, -500)
	overlayHomeBX, overlayHomeBY := 12.0, 200.0

	face := wrkit.FaceAt(14)
	fieldLabel := func(s string) *rendering.RenderText {
		t := wrkit.Label(s, 13, 0.88, 0.92, 0.96)
		if face != nil {
			wrkit.ApplyFace(t, 13)
		}
		t.SetRepaintBoundary(true)
		return t
	}
	uncontrolledText := fieldLabel("uncontrolled: ")
	uncontrolledText.SetDebugName("f0-field-uncontrolled")
	shell.Body.Place(uncontrolledText, 560, 40)
	controlledText := fieldLabel("controlled: hello")
	controlledText.SetDebugName("f0-field-controlled")
	shell.Body.Place(controlledText, 560, 90)
	composingText := fieldLabel("composing: ")
	composingText.SetDebugName("f0-field-composing")
	shell.Body.Place(composingText, 560, 140)
	shell.Body.Box.Place(wrkit.Label("fields controlled/uncontrolled", 11, 0.62, 0.72, 0.85), 560, 20)
	noteBG := [3]float64{0.10, 0.11, 0.13}

	snapDir := filepath.Join("examples", "kit_f0_interact", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	var goldenRects []rect
	var probeClick, probeDis, probeOverlay rect
	var densityRect rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		probeClick = rect{x: bodyX + 16, y: bodyY + 40, w: 220, h: 64}
		probeDis = rect{x: bodyX + 252, y: bodyY + 40, w: 220, h: 64}
		probeOverlay = rect{x: bodyX + overlayHomeBX, y: bodyY + overlayHomeBY, w: 200, h: 120}
		densityRect = rect{x: bodyX + 560, y: bodyY + 40, w: 300, h: 110}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			probeClick,
			probeOverlay,
		}
		geoValid = true
	}

	placeDone := false
	checkPlacements := func() bool {
		anchor := rendering.NewRect(100, 100, 80, 40)
		opt := &overlay.ResolveOptions{Gap: 12, Flip: false, Shift: false, ViewportW: 1200, ViewportH: 800}
		want := map[overlay.Placement][2]float64{
			overlay.Top: {80, 28}, overlay.Bottom: {80, 152},
			overlay.Left: {-32, 90}, overlay.Right: {192, 90},
		}
		for p, xy := range want {
			res := kit.BehaviorResolveFollower(anchor, 120, 60, p, opt)
			if res.Actual != p || math.Abs(res.X-xy[0]) > 1e-9 || math.Abs(res.Y-xy[1]) > 1e-9 {
				return false
			}
		}
		flipAnchor := rendering.NewRect(540, 10, 120, 40)
		flipOpt := &overlay.ResolveOptions{Gap: 12, Flip: true, Shift: true, ViewportW: 1200, ViewportH: 800}
		fr := kit.BehaviorResolveFollower(flipAnchor, 200, 120, overlay.Top, flipOpt)
		return fr.Actual == overlay.Bottom && fr.Flipped &&
			math.Abs(fr.X-500) < 1e-9 && math.Abs(fr.Y-62) < 1e-9
	}

	var overlayCleanPic, overlayCleanSib, overlayCleanLayout bool
	var elapsed float64
	resizeDone, resizeBack := false, false
	phPress, phTap, phField, phOverlay, phEsc, phFocus, phFinal := false, false, false, false, false, false, false
	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_interact.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit_f0_interact: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					shell.Resize(float64(ev.Width), float64(ev.Height))
					geoValid = false
				}
			case platform.EventPointer:
				bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
				lx, ly := ev.X-bodyX, ev.Y-bodyY
				switch ev.Pointer {
				case platform.PointerMove:
					click.SetHover(kit.BehaviorContains(16, 40, 220, 64, lx, ly))
					fmt.Fprintf(os.Stderr, "kit_f0_interact: pointer move %.0f,%.0f hover=%v\n", ev.X, ev.Y, click.States().Has(kit.ScopeStateHover))
				case platform.PointerDown:
					if kit.BehaviorContains(16, 40, 220, 64, lx, ly) {
						click.SetPressed(true)
						setBox(clickBox, colActive)
					}
					fmt.Fprintf(os.Stderr, "kit_f0_interact: pointer down %.0f,%.0f\n", ev.X, ev.Y)
				case platform.PointerUp:
					if click.States().Has(kit.ScopeStatePressed) {
						click.SetPressed(false)
						if kit.BehaviorContains(16, 40, 220, 64, lx, ly) {
							click.DirectTap()
							setBox(clickBox, colHover)
						} else {
							setBox(clickBox, colPrimary)
						}
					}
					fmt.Fprintf(os.Stderr, "kit_f0_interact: pointer up clicks=%d\n", click.Clicks())
				}
				app.ScheduleFrame()
			case platform.EventKey:
				if ev.Pressed && ev.KeyCode == 0xFF1B {
					if trigger.HandleKey(focus.KeyEvent{KeyCode: 0xFF1B, Pressed: true}) {
						overlayBox.MoveTo(-500, -500)
						fmt.Fprintln(os.Stderr, "kit_f0_interact: key esc closed overlay")
					}
				} else if ev.Pressed && (ev.KeyCode == 0xFF09 || ev.KeyCode == 9) {
					mgr.HandleKey(focus.KeyEvent{KeyCode: focus.KeyTab, Pressed: true})
					fmt.Fprintln(os.Stderr, "kit_f0_interact: key tab")
				} else if ev.Pressed && (ev.KeyCode == 0x0020 || ev.KeyCode == 0xFF0D) {
					click.HandleKey(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true})
					fmt.Fprintf(os.Stderr, "kit_f0_interact: key activate clicks=%d\n", click.Clicks())
				}
				app.ScheduleFrame()
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

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
		if !phPress && elapsed >= 4.0 {
			phPress = true
			click.SetHover(true)
			click.SetPressed(true)
			setBox(clickBox, colActive)
		}
		if phPress && !phTap && elapsed >= 5.0 {
			phTap = true
			click.SetPressed(false)
			click.DirectTap()
			setBox(clickBox, colHover)
			disabled.DirectTap()
			loading.DirectTap()
			loading.DirectTap()
			scripted["click_ok"] = click.Clicks() == 1
			scripted["disabled_swallow"] = disabled.Clicks() == 0
			scripted["loading_noreentry"] = loading.Clicks() == 0
		}
		if phTap && !phField && elapsed >= 6.0 {
			phField = true
			uncontrolled.Input("hello")
			uncontrolledText.SetText("uncontrolled: hello")
			controlled.Input("world")
			controlledText.SetText("controlled: hello (notify world)")
			composing.SetComposing("ni")
			composing.CommitComposing()
			composingText.SetText("composing: ni")
			scripted["field_ok"] = uncontrolled.Value() == "hello" &&
				controlled.Value() == "hello" &&
				len(controlledNotify) == 1 && controlledNotify[0] == "world" &&
				composing.Value() == "ni" && composing.ChangeCalls() == 1
		}
		if phField && !phOverlay && elapsed >= 7.0 {
			phOverlay = true
			placeDone = checkPlacements()
			scripted["place_ok"] = placeDone
			bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
			anchor := rendering.NewRect(bodyX+anchorBX, bodyY+anchorBY, 160, 48)
			trigger.Open(anchor, 200, 120)
			ov.Layout(1200, 800)
			overlayBox.MoveTo(overlayHomeBX, overlayHomeBY)
			scripted["trap_ok"] = trigger.FocusNode() != nil && trigger.IsFocusInside(trigger.FocusNode())
		}
		if phOverlay && !phEsc && elapsed >= 8.0 {
			phEsc = true
			bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
			inside := rendering.Point{X: bodyX + overlayHomeBX + 100, Y: bodyY + overlayHomeBY + 60}
			keptOpen := !trigger.HandleOutside(inside) && trigger.IsOpen()
			outside := rendering.Point{X: 10, Y: 10}
			closedOutside := trigger.HandleOutside(outside) && !trigger.IsOpen()
			if closedOutside {
				overlayBox.MoveTo(-500, -500)
			}
			anchor := rendering.NewRect(bodyX+anchorBX, bodyY+anchorBY, 160, 48)
			trigger.Open(anchor, 200, 120)
			ov.Layout(1200, 800)
			overlayBox.MoveTo(overlayHomeBX, overlayHomeBY)
			escClosed := trigger.HandleKey(focus.KeyEvent{KeyCode: 0xFF1B, Pressed: true}) && !trigger.IsOpen()
			if escClosed {
				overlayBox.MoveTo(-500, -500)
			}
			scripted["outside_esc_ok"] = keptOpen && closedOutside && escClosed
		}
		if phEsc && !phFocus && elapsed >= 9.0 {
			phFocus = true
			tapFocused := false
			click2 := kit.NewBehaviorInteractive(kit.BehaviorInteractiveConfig{Focusable: true, Label: "probe"})
			m2 := focus.NewManager()
			click2.Mount(m2)
			click2.DirectTap()
			tapFocused = click2.States().Has(kit.ScopeStateFocused)
			ringAfterTap := kit.ScopeShowFocusRing(click2.States())
			click2.FocusNode().RequestFocus()
			ringAfterKeys := kit.ScopeShowFocusRing(click2.States())
			disabled2 := kit.NewBehaviorInteractive(kit.BehaviorInteractiveConfig{Disabled: true, Focusable: true, Label: "d"})
			m2b := focus.NewManager()
			disabled2.Mount(m2b)
			disabled2.FocusNode().RequestFocus()
			ringDisabled := kit.ScopeShowFocusRing(disabled2.States())
			click2.Unmount()
			disabled2.Unmount()
			hitOK := kit.BehaviorContains(16, 40, 220, 64, 20, 50) &&
				!kit.BehaviorContains(16, 40, 220, 64, 500, 500) &&
				kit.BehaviorContains(252, 40, 220, 64, 300, 50) &&
				kit.BehaviorContains(overlayHomeBX, overlayHomeBY, 200, 120, overlayHomeBX+10, overlayHomeBY+10)
			scripted["focus_ring_ok"] = !tapFocused && !ringAfterTap && ringAfterKeys && !ringDisabled
			scripted["hit_match"] = hitOK
			mgr.FocusNext()
			scripted["tab_ok"] = true
		}
		if phFocus && !phFinal && elapsed >= 11.0 {
			phFinal = true
			bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
			anchor := rendering.NewRect(bodyX+anchorBX, bodyY+anchorBY, 160, 48)
			trigger.Open(anchor, 200, 120)
			ov.Layout(1200, 800)
			overlayBox.MoveTo(overlayHomeBX, overlayHomeBY)
			overlayCleanPic = overlayBox.NeedsPaint()
			overlayCleanSib = !clickBox.NeedsPaint() && !disBox.NeedsPaint() && !loadBox.NeedsPaint()
			overlayCleanLayout = !overlayBox.NeedsLayout() && !clickBox.NeedsLayout() && !shell.Root.NeedsLayout()
			scripted["overlay_clean"] = overlayCleanPic && overlayCleanSib && overlayCleanLayout
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundarySkip >= 0 && snapH.PaintCount > 0
		shell.UpdateHUD("F0-interact", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("resize=%d clicks=%d open=%v t=%.1f", resizeEvents, click.Clicks(), trigger.IsOpen(), elapsed))
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
	img := loadImage(filepath.Join(snapDir, "showcase_interact.png"))
	runSolidChecks(img, map[string]solidProbe{
		"click_hover":   {box: probeClick, want: colHover},
		"disabled_gray": {box: probeDis, want: colDisabled},
		"overlay_box":   {box: probeOverlay, want: colElevated},
	}, scripted)
	runDensityCheck(img, densityRect, noteBG, "field_density", scripted, hasFace)

	scriptedOK := 0
	for _, k := range []string{"click_ok", "disabled_swallow", "loading_noreentry", "field_ok", "place_ok", "trap_ok", "outside_esc_ok", "focus_ring_ok", "hit_match", "tab_ok", "overlay_clean", "click_hover", "disabled_gray", "overlay_box", "field_density"} {
		if scripted[k] {
			scriptedOK++
		}
	}
	wantTotal := 15
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "F0-interact",
		Scenario:      "kit_f0_interact",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"has_face":               hasFace,
			"face_name":              faceName,
			"clicks":                 click.Clicks(),
			"disabled_clicks":        disabled.Clicks(),
			"loading_clicks":         loading.Clicks(),
			"uncontrolled_value":     uncontrolled.Value(),
			"controlled_value":       controlled.Value(),
			"controlled_notified":    controlledNotify,
			"place_done":             placeDone,
			"trigger_open":           trigger.IsOpen(),
			"overlay_clean_pic":      overlayCleanPic,
			"overlay_clean_sib":      overlayCleanSib,
			"overlay_clean_layout":   overlayCleanLayout,
			"scripted_ok":            scriptedOK,
			"scripted_total":         wantTotal,
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
	if scriptedOK != wantTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d\n", scriptedOK, wantTotal)
		for k, v := range scripted {
			fmt.Fprintf(os.Stderr, "  %-16s %v\n", k, v)
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
	fmt.Fprintf(os.Stderr, "kit_f0_interact: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, wantTotal, goldenDiffPct, resizeBack, finalW, finalH, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type solidProbe struct {
	box  rect
	want [3]float64
}

func runSolidChecks(img image.Image, probes map[string]solidProbe, result map[string]bool) {
	for name, p := range probes {
		ok := img != nil && solidPointOK(img, p.box, p.want)
		fmt.Fprintf(os.Stderr, "kit_f0_interact: solid %-14s ok=%v\n", name, ok)
		result[name] = ok
	}
}

func solidPointOK(img image.Image, box rect, want [3]float64) bool {
	px := int(box.x + box.w/2)
	py := int(box.y + box.h/2)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
	return math.Abs(r-want[0]) <= 12.0/255 && math.Abs(g-want[1]) <= 12.0/255 && math.Abs(b-want[2]) <= 12.0/255
}

func runDensityCheck(img image.Image, box rect, base [3]float64, name string, result map[string]bool, hasFace bool) {
	n := textPixels(img, 1.0, box, base)
	ok := img != nil && hasFace && n >= 60
	fmt.Fprintf(os.Stderr, "kit_f0_interact: text %-14s ink_px=%d (want >=60) ok=%v\n", name, n, ok)
	result[name] = ok
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
	cur := filepath.Join(snapDir, "showcase_interact.png")
	base := filepath.Join(snapDir, "showcase_interact_base.png")
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
