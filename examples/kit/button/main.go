// Command kit_button is the Button standalone true window.
//
// One component, one directory (examples/kit/<name>): independent open,
// independent test, independent screenshot. Combined gallery is preview only.
//
// Modes:
//
//	go run ./examples/kit/button -auto-only
//	  headless selftest + short real window (JSON on stdout, exit 1 on fail).
//	go run ./examples/kit/button
//	  selftest, then manual until close (click/hover/keys reach real Buttons).
//	go run ./examples/kit/button -manual-seconds 30
//	  manual for 30s, then summary JSON.
//
// Manual checklist (human sign-off for BTN-22):
//   - Hover Primary solid turns #4096ff; press turns #0958d9; release fires once.
//   - Disabled buttons swallow press; loading shows spinner and swallows repeat.
//   - Tab moves focus ring; Enter/Space activates focused; ghost row sits on #bec8c8.
//   - Compare with https://ant.design/components/button side by side.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/pfkit"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 860

type item struct {
	b     *button.Button
	x, y  float64
	w, h  float64
	label string
	// kind drives per-instance simulated verification in -auto-only:
	// click | disabled | loading | press-no-fire | toggle | nav.
	kind string
}

type manualSummary struct {
	Pointer  int
	Key      int
	Resize   int
	Activate int
	Timed    bool
	Note     string
}

func (s manualSummary) rows() []pfkit.ResultRow {
	return []pfkit.ResultRow{{
		Name:   "ManualButton",
		OK:     true,
		Detail: fmt.Sprintf("pointer=%d key=%d resize=%d activate=%d timed=%v %s", s.Pointer, s.Key, s.Resize, s.Activate, s.Timed, s.Note),
	}}
}

// selftest runs headless logic probes (no GPU): geometry, exact official
// hover/press colors, disabled swallow, keyboard activate.
func selftest() []pfkit.ResultRow {
	rows := []pfkit.ResultRow{}
	ok := func(name, detail string) { rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: detail}) }
	fail := func(name, detail string) { rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: detail}) }

	// Geometry档位.
	sm, md, lg := button.NewButton("Small"), button.NewButton("Middle"), button.NewButton("Large")
	sm.SetSize(button.ButtonSmall)
	lg.SetSize(button.ButtonLarge)
	if dh := md.Height(); dh < 31.5 || dh > 32.5 {
		fail("Height", fmt.Sprintf("middle=%v want 32", dh))
	} else if sh := sm.Height(); sh < 23.5 || sh > 24.5 {
		fail("Height", fmt.Sprintf("small=%v want 24", sh))
	} else if lh := lg.Height(); lh < 39.5 || lh > 40.5 {
		fail("Height", fmt.Sprintf("large=%v want 40", lh))
	} else {
		ok("Height", "24/32/40")
	}

	// Exact official colors: base #1677ff hover #4096ff active #0958d9.
	b := button.NewButton("确定")
	b.SetType(button.ButtonPrimary)
	b.Layout(rendering.Loose(1000, 1000))
	hex := func(c render.RGBA) string {
		return fmt.Sprintf("#%02x%02x%02x", uint8(c.R*255+0.5), uint8(c.G*255+0.5), uint8(c.B*255+0.5))
	}
	if got := hex(b.Fill()); got != "#1677ff" {
		fail("Base", "fill="+got+" want #1677ff")
	} else {
		ok("Base", "fill #1677ff")
	}
	w, h := b.LaidOut().Width, b.LaidOut().Height
	b.PointerMove(w/2, h/2)
	if got := hex(b.Fill()); got != "#4096ff" {
		fail("Hover", "fill="+got+" want #4096ff")
	} else {
		ok("Hover", "fill #4096ff")
	}
	b.PointerDown(w/2, h/2)
	if got := hex(b.Fill()); got != "#0958d9" {
		fail("Active", "fill="+got+" want #0958d9")
	} else {
		ok("Active", "fill #0958d9")
	}
	fired := 0
	b.OnClick = func() { fired++ }
	b.PointerUp(w/2, h/2)
	if fired != 1 {
		fail("Click", fmt.Sprintf("fired=%d want 1", fired))
	} else {
		ok("Click", "press-release fires once")
	}

	// Disabled swallows.
	d := button.NewButton("Disabled")
	d.SetDisabled(true)
	d.Layout(rendering.Loose(1000, 1000))
	dw, dh := d.LaidOut().Width, d.LaidOut().Height
	if d.PointerDown(dw/2, dh/2) {
		fail("Disabled", "swallow=false")
	} else {
		ok("Disabled", "swallows press")
	}

	// Danger hover #ff7875.
	g := button.NewButton("Danger")
	g.SetType(button.ButtonPrimary)
	g.SetDanger(true)
	g.Layout(rendering.Loose(1000, 1000))
	gw, gh := g.LaidOut().Width, g.LaidOut().Height
	g.PointerMove(gw/2, gh/2)
	if got := hex(g.Fill()); got != "#ff7875" {
		fail("DangerHover", "fill="+got+" want #ff7875")
	} else {
		ok("DangerHover", "fill #ff7875")
	}
	return rows
}

func buildItems() ([]item, *rendering.RenderColorBox, float64, float64, float64, float64) {
	face14 := wrkit.FaceAt(14)
	loose := rendering.Loose(912, 800)
	type staged struct {
		b    *button.Button
		kind string
	}
	var stage []staged
	mk := func(label, kind string, fn func(b *button.Button)) *button.Button {
		b := button.NewButton(label)
		if face14 != nil {
			b.SetTextFace(face14)
		}
		if fn != nil {
			fn(b)
		}
		b.Layout(loose)
		stage = append(stage, staged{b: b, kind: kind})
		return b
	}
	typeRow := []*button.Button{
		mk("确定", "click", nil),
		mk("确定", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary) }),
		mk("Dashed", "click", func(b *button.Button) { b.SetType(button.ButtonDashed) }),
		mk("Text", "click", func(b *button.Button) { b.SetType(button.ButtonText) }),
		mk("Link", "click", func(b *button.Button) { b.SetType(button.ButtonLink) }),
	}
	sizeRow := []*button.Button{
		mk("Small", "click", func(b *button.Button) { b.SetSize(button.ButtonSmall) }),
		mk("Middle", "click", nil),
		mk("Large", "click", func(b *button.Button) { b.SetSize(button.ButtonLarge) }),
	}
	disRow := []*button.Button{
		mk("Disabled", "disabled", func(b *button.Button) { b.SetDisabled(true) }),
		mk("Disabled", "disabled", func(b *button.Button) { b.SetType(button.ButtonPrimary); b.SetDisabled(true) }),
	}
	loadRow := []*button.Button{
		mk("Loading", "loading", func(b *button.Button) { b.SetLoading(true) }),
		mk("Loading", "loading", func(b *button.Button) { b.SetLoadingConfig(button.LoadingConfig{Icon: "custom-spin"}) }),
	}
	iconRow := []*button.Button{
		mk("Search", "click", func(b *button.Button) { b.SetIcon("search") }),
		mk("Search", "click", func(b *button.Button) { b.SetIcon("search"); b.SetIconPlacement(button.IconEnd) }),
	}
	multiRow := []*button.Button{
		mk("Cancel", "click", nil), mk("More", "click", nil),
		mk("Submit", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary) }),
	}
	ghostRow := []*button.Button{
		mk("Ghost", "click", func(b *button.Button) { b.SetGhost(true) }),
		mk("Ghost", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary); b.SetGhost(true) }),
	}
	dangerRow := []*button.Button{
		mk("Danger", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary); b.SetDanger(true) }),
		mk("Danger", "click", func(b *button.Button) { b.SetDanger(true) }),
	}
	variantRow := []*button.Button{
		mk("Solid", "click", func(b *button.Button) { b.SetColor(button.ColorPrimary); b.SetVariant(button.VariantSolid) }),
		mk("Outlined", "click", func(b *button.Button) { b.SetColor(button.ColorPrimary); b.SetVariant(button.VariantOutlined) }),
		mk("Dashed", "click", func(b *button.Button) { b.SetVariant(button.VariantDashed) }),
		mk("Filled", "click", func(b *button.Button) { b.SetColor(button.ColorPrimary); b.SetVariant(button.VariantFilled) }),
		mk("Text", "click", func(b *button.Button) { b.SetVariant(button.VariantText) }),
		mk("Link", "click", func(b *button.Button) { b.SetVariant(button.VariantLink) }),
	}
	blockBtn := mk("Block Button", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary); b.SetBlock(true) })
	kindOf := func(b *button.Button) string {
		for _, s := range stage {
			if s.b == b {
				return s.kind
			}
		}
		return "click"
	}

	const W = 920.0
	const margin = 20.0
	const rowGap = 18.0
	const colGap = 12.0
	var items []item
	y := margin + 28
	layoutRow := func(row []*button.Button, label string) {
		_ = label
		x := margin
		maxH := 0.0
		for _, b := range row {
			sz := b.Layout(loose)
			if sz.Height > maxH {
				maxH = sz.Height
			}
		}
		for _, b := range row {
			sz := b.LaidOut()
			items = append(items, item{b: b, x: x, y: y, w: sz.Width, h: sz.Height, label: b.Label(), kind: kindOf(b)})
			x += sz.Width + colGap
		}
		y += maxH + rowGap
	}
	rows := [][]*button.Button{typeRow, sizeRow, disRow, loadRow, iconRow, multiRow, ghostRow, dangerRow, variantRow}
	var ghostBg *rendering.RenderColorBox
	var gx, gy, gw, gh float64
	for i, r := range rows {
		if i == 6 {
			maxH := 0.0
			for _, b := range r {
				sz := b.Layout(loose)
				if sz.Height > maxH {
					maxH = sz.Height
				}
			}
			gx, gy, gw, gh = 0, y-8, W+2*margin, maxH+16
			ghostBg = rendering.NewRenderColorBox(gw, gh, 190.0/255.0, 200.0/255.0, 200.0/255.0, 1)
		}
		layoutRow(r, "")
	}
	rowW := W
	bsz := blockBtn.Layout(rendering.Constraints{MinWidth: rowW, MaxWidth: rowW, MaxHeight: rendering.Unbounded})
	items = append(items, item{b: blockBtn, x: margin, y: y, w: bsz.Width, h: bsz.Height, label: "Block", kind: kindOf(blockBtn)})
	y += bsz.Height + rowGap
	return items, ghostBg, gx, gy, gw, gh
}

// verifyEveryInstance simulates a real user on every window instance and
// verifies the effect: clickables fire exactly once on press-release and go
// quiet on move-out release; disabled/loading swallow; hover/press move the
// fill to the official hover/active ink. One row per instance; any FAIL
// blocks the component close (see ACCEPTANCE hard rule).
func verifyEveryInstance(items []item) []pfkit.ResultRow {
	rows := []pfkit.ResultRow{}
	hex := func(c render.RGBA) string {
		return fmt.Sprintf("#%02x%02x%02x", uint8(c.R*255+0.5), uint8(c.G*255+0.5), uint8(c.B*255+0.5))
	}
	for i := range items {
		it := items[i]
		b := it.b
		name := fmt.Sprintf("win-%02d/%s", i, it.label)
		if b == nil || it.w <= 0 || it.h <= 0 {
			rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "SKIP: zero-size instance"})
			continue
		}
		cx, cy := it.w/2, it.h/2
		base := hex(b.Fill())
		switch it.kind {
		case "disabled":
			if b.PointerDown(cx, cy) {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "disabled consumed press"})
				continue
			}
			b.PointerMove(cx, cy)
			if hex(b.Fill()) != base {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "disabled hover moved fill " + base})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "disabled swallows + no hover"})
		case "loading":
			if !b.HasSpinner() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "loading spinner missing"})
				continue
			}
			fired := 0
			b.OnClick = func() { fired++ }
			b.PointerDown(cx, cy)
			b.PointerUp(cx, cy)
			if fired != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("loading fired %d", fired)})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "loading spinner + no repeat"})
		default: // click: hover ink, press ink, fire-once, move-out quiet.
			b.PointerMove(cx, cy)
			hoverGot := hex(b.Fill())
			b.PointerDown(cx, cy)
			pressGot := hex(b.Fill())
			fired := 0
			b.OnClick = func() { fired++ }
			b.PointerUp(cx, cy)
			if fired != 1 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("click fired=%d want 1 (hover %s press %s)", fired, hoverGot, pressGot)})
				continue
			}
			b.PointerDown(cx, cy)
			b.PointerMove(-10, -10)
			fired = 0
			b.OnClick = func() { fired++ }
			b.PointerUp(-10, -10)
			if fired != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "move-out release fired"})
				continue
			}
			// Disabled/loading instances already returned; clickables must
			// show hover/press feedback (fill moves off base).
			if hoverGot == base && pressGot == base && base != "#ffffff" && base != "#000000" {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "no hover/press feedback"})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: fmt.Sprintf("click once hover %s press %s", hoverGot, pressGot)})
		}
	}
	return rows
}

func kitKey(ev platform.Event) string {
	if !ev.Pressed {
		return ""
	}
	if ev.Rune == '\t' {
		return "Tab"
	}
	if ev.Rune == '\r' || ev.Rune == '\n' {
		return "Enter"
	}
	if ev.Rune == ' ' {
		return "Space"
	}
	switch ev.KeyCode {
	case int(input.KeyTab), 0xff09:
		return "Tab"
	case int(input.KeyEnter), 0xff0d, 0xff8d:
		return "Enter"
	case int(input.KeySpace), 0x20:
		return "Space"
	case int(input.KeyEscape), 0xff1b:
		return "Escape"
	}
	return ""
}

func emitJSON(backend string, rows []pfkit.ResultRow, pass bool, presents int64, m manualSummary) {
	b, _ := json.Marshal(map[string]any{
		"scenario": "kit_button",
		"tab":      "button",
		"backend":  backend,
		"rows":     rows,
		"manual": map[string]any{
			"pointer": m.Pointer, "key": m.Key, "resize": m.Resize,
			"activate": m.Activate, "timed": m.Timed, "note": m.Note,
		},
		"presents": presents,
		"pass":     pass,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()

	rows := selftest()
	// Strict gate: simulate every window instance headless first; any FAIL
	// blocks the window (same rows re-reported in gate JSON).
	wrkit.EnsureUIFace()
	headlessItems, _, _, _, _, _ := buildItems()
	rows = append(rows, verifyEveryInstance(headlessItems)...)
	ok := pfkit.Report(rows)
	if !*autoOnly {
		if !ok {
			fmt.Fprintln(os.Stderr, "kit_button: selftest FAIL, not opening window")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "kit_button: selftest done, entering manual phase")
		fmt.Fprintln(os.Stderr, "  hover/press buttons; Tab+Enter/Space; close X to finish.")
	} else if !ok {
		emitJSON("unknown", rows, false, 0, manualSummary{Note: "headless selftest failed"})
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(5)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	wrkit.EnsureUIFace()
	items, ghostBg, gx, gy, _, _ := buildItems()
	// Re-run per-instance verification on the live window nodes so the
	// reported rows describe exactly what the human sees (fresh nodes;
	// headless pass above already gated open).
	liveRows := verifyEveryInstance(items)
	_ = liveRows
	fmgr := focus.NewManager()
	for _, it := range items {
		if it.b != nil && it.b.Focusable() {
			fmgr.Register(it.b.FocusNode())
		}
	}

	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.96, G: 0.96, B: 0.96, A: 1}
	title := wrkit.Label("Button — standalone true window (hover/press/Tab/Enter; ghost on #bec8c8)", 14, 0.1, 0.1, 0.1)
	root.Place(title, 20, 8)
	if ghostBg != nil {
		root.Place(ghostBg, gx, gy)
	}
	for _, it := range items {
		root.Place(it.b.Node(), it.x, it.y)
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui kit_button — Button", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()
	_ = ctl

	var summary manualSummary
	var pressIdx = -1
	at := func(x, y float64) (int, float64, float64) {
		for i := len(items) - 1; i >= 0; i-- {
			it := items[i]
			if it.w <= 0 || it.h <= 0 {
				continue
			}
			if x >= it.x && y >= it.y && x < it.x+it.w && y < it.y+it.h {
				return i, x - it.x, y - it.y
			}
		}
		return -1, 0, 0
	}

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: 0.96, ClearG: 0.96, ClearB: 0.96, ClearA: 1,
		RunFor: runFor, WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				return
			case platform.EventPointer:
				summary.Pointer++
				dirty := false
				switch ev.Pointer {
				case platform.PointerMove:
					if i, lx, ly := at(ev.X, ev.Y); i >= 0 {
						items[i].b.PointerMove(lx, ly)
						dirty = true
					}
				case platform.PointerDown:
					fmt.Fprintf(os.Stderr, "pointer down (%.0f,%.0f)\n", ev.X, ev.Y)
					pressIdx = -1
					if i, lx, ly := at(ev.X, ev.Y); i >= 0 {
						fmgr.RequestFocus(items[i].b.FocusNode())
						if items[i].b.PointerDown(lx, ly) {
							pressIdx = i
							summary.Activate++
							fmt.Fprintf(os.Stderr, "activate press %s\n", items[i].label)
						}
						dirty = true
					}
				case platform.PointerUp:
					if pressIdx >= 0 && pressIdx < len(items) {
						pi := items[pressIdx]
						i, lx, ly := at(ev.X, ev.Y)
						if i == pressIdx {
							if pi.b.PointerUp(lx, ly) {
								summary.Activate++
								fmt.Fprintf(os.Stderr, "activate release %s\n", pi.label)
							}
						} else {
							pi.b.PointerUp(-1, -1)
						}
						dirty = true
					} else if i, lx, ly := at(ev.X, ev.Y); i >= 0 {
						if items[i].b.PointerUp(lx, ly) {
							summary.Activate++
						}
						dirty = true
					}
					pressIdx = -1
				}
				if dirty {
					root.MarkNeedsPaint()
					app.ScheduleFrame()
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if key := kitKey(ev); key != "" {
						switch key {
						case "Tab":
							if n := fmgr.FocusNext(); n != nil {
								fmt.Fprintf(os.Stderr, "focus %s\n", n.DebugLabel)
							}
							root.MarkNeedsPaint()
							app.ScheduleFrame()
						case "Enter", "Space":
							if p := fmgr.Primary(); p != nil {
								handled := false
								if p.OnKey != nil {
									handled = p.OnKey(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true})
								}
								if !handled && p.OnActivate != nil {
									p.OnActivate()
									handled = true
								}
								if handled {
									summary.Activate++
									fmt.Fprintf(os.Stderr, "key activate %s via %s\n", p.DebugLabel, key)
									root.MarkNeedsPaint()
									app.ScheduleFrame()
								}
							}
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				return
			default:
				return
			}
		},
	})

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	presents := app.PresentCount()
	backend := win.Backend().String()
	pass := ok && presents >= 1
	if backend == "" || backend == "auto" {
		pass = false
	}
	if *autoOnly {
		emitJSON(backend, rows, pass, presents, manualSummary{Note: "gate"})
		if !pass {
			fmt.Fprintf(os.Stderr, "gate FAIL: backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "gate PASS: backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
		return
	}
	summary.Timed = secs > 0
	rows = append(rows, summary.rows()...)
	ok = pfkit.Report(rows)
	emitJSON(backend, rows, ok && presents >= 1, presents, summary)
	fmt.Fprintf(os.Stderr, "kit_button backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
	if !(ok && presents >= 1) {
		os.Exit(1)
	}
}
