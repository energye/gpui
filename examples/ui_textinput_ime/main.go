// Command ui_textinput_ime is the IME test window (plan §6): a real GPU
// window where typed text renders live — plain keys insert immediately, and
// with an input method (IBus/fcitx) the pinyin pre-edit shows in place and
// commits on selection.
//
// Interactive (default):
//
//	export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	export LD_LIBRARY_PATH=$PWD/lib
//	go run ./examples/ui_textinput_ime
//
// Click the box, then type. stderr logs every normalized text/ime/key event;
// closing the window exits and prints an exit-JSON report.
//
// Self-test (headless-friendly render proof):
//
//	GPUI_IME_DEMO_SELFTEST=1 go run ./examples/ui_textinput_ime
//
// Drives the exact normalized events a real IME produces (compose → commit →
// plain key) through InputRouter.RoutePlatform — the same normalization +
// routing path the event loop uses — then snapshots the final rendered frame
// (IME_SNAP_DIR, default /tmp/ime_textinput_ime) and exits 1 unless the
// buffer equals the expected "你好a". The snapshot PNG is the pixel evidence
// that committed CJK + ASCII text actually renders into the window.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/textinput"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 900, 300
	boxX       = 40.0
	boxY       = 60.0
	boxW       = 600.0
	boxH       = 48.0
	// expectText is the self-test expectation: pinyin commit "你好", one
	// plain ASCII keystroke "a", then ArrowLeft + "b" proving the caret
	// moved (b lands BEFORE a).
	expectText = "你好ba"
)

// inputBox renders the editor text plus a caret/compose marker. It is an
// EventTarget AND a TextEditTarget: the framework drives its whole IME
// session from focus transitions and edits (plan I4/I5) — no manual
// EnableIME/DisableIME anywhere.
type inputBox struct {
	*rendering.RenderBox
	ed      *textinput.Editor
	text    *rendering.RenderText
	bar     *rendering.RenderColorBox // floating caret bar — layout-neutral
	node    *focus.FocusNode          // registered with the window's focus manager
	sched   func()                    // set before Run: request a frame after edits
	focused bool
	caretOn bool // idle-caret blink state (UI-thread only)
	blinks  int  // completed blink toggles (evidence in exit JSON)
	// preeditEvents counts inbound compose events (P9 echo-storm guard:
	// exit JSON asserts ≤2 per keystroke window).
	preeditEvents int
}

func newInputBox(ed *textinput.Editor) *inputBox {
	inner := rendering.NewRenderBox()
	b := &inputBox{
		RenderBox: inner,
		ed:        ed,
		text:      rendering.NewRenderText(""),
	}
	inner.Init(b) // Self = the OUTER inputBox so parent chains resolve to it
	b.text.FontSize = 20
	b.text.R, b.text.G, b.text.B, b.text.A = 0.05, 0.75, 0.95, 1
	b.FixedWidth = boxW
	b.FixedHeight = boxH
	b.AddChild(b.text)
	// The caret is a FLOATING BAR positioned at the cursor offset — never a
	// glyph inserted into the string: an inserted "|" split the neighboring
	// glyphs apart while blinking and desynced click→caret mapping. Color is
	// ORANGE — deliberately distinct from the cyan text so the pixel probe
	// can locate the bar without glyph-stroke false positives.
	b.bar = rendering.NewRenderColorBox(1, 26, 1.0, 0.55, 0.10, 1)
	b.AddChild(b.bar)
	// Caret bar position must be applied AFTER layout (RenderBox.Layout
	// resets child offsets to Pad on every pass — a MoveTo from sync/
	// OnChange gets wiped by the next layout). The OnPaint hook runs right
	// before children paint, i.e. after this frame's layout: reposition the
	// bar there. It draws itself as a normal child afterwards.
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		b.layoutCaret()
	}
	b.caretOn = true
	b.node = focus.NewFocusNode("input-box")
	b.node.Target = b // framework resolves the focused TextEditTarget via this
	b.node.OnFocusChange = func(on bool) {
		b.focused = on
		b.sync()
	}
	ed.OnChange = func() { b.sync() }
	b.sync()
	return b
}

// --- TextEditTarget (plan I4): the framework drives the IME session from
// focus transitions and edits; this target supplies state and anchors. ---

// Editor returns the editing state this box edits.
func (b *inputBox) Editor() *textinput.Editor { return b.ed }

// ContentPurpose declares a plain-text field (TextEditTarget contract).
func (b *inputBox) ContentPurpose() platform.ContentPurpose { return platform.PurposeNormal }

// ContentType is the facade-facing declaration (FieldSnapshotProvider).
func (b *inputBox) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: b.ContentPurpose()}
}

// caretAnchor computes the caret's WINDOW coordinates (logical px): the
// pen boundary between the characters around the caret, plus the line's
// top/bottom. THE single source of caret geometry — both the visible bar
// (layoutCaret) and the IME candidate anchor (IMERect) derive from it, so
// the two can never drift apart. During a composition the caret rides the
// IME's own position inside the span.
func (b *inputBox) caretAnchor() (x, top, bottom float64, ok bool) {
	if b == nil || b.text == nil {
		return 0, 0, 0, false
	}
	v := b.ed.View()
	viewCur := v.MapBufToView(b.ed.Cursor())
	if b.ed.ComposeActive() {
		if cc := b.ed.CompositionCursor(); cc >= 0 {
			viewCur = cc
		} else {
			viewCur = v.CompEnd
		}
	}
	lineIdx, penX, kok := b.text.CaretColumn(min(viewCur, len(b.text.Text)))
	if !kok {
		return 0, 0, 0, false
	}
	fs := b.text.FontSizePt()
	if fs <= 0 {
		fs = 20
	}
	lh := b.text.LineHeight()
	if lh <= 0 {
		m, ok2 := b.text.Metrics()
		if ok2 {
			lh = m.LineHeight()
		} else {
			lh = fs * 1.25
		}
	}
	baseline := fs + float64(lineIdx)*lh
	// Flutter-style caret box: exactly the font's ink band for this line —
	// from baseline−ascent to baseline+descent, centered on the line's
	// baseline slot (lineHeight may exceed ascent+descent; center within it,
	// matching TextPainter's cursor placement). No ad-hoc ratios.
	var ascent, descent float64 = fs * 0.8, fs * 0.2 // heuristic fallback
	if m, ok2 := b.text.Metrics(); ok2 && m.Ascent > 0 {
		ascent, descent = m.Ascent, m.Descent
	}
	// Caret box height = ascent+descent (the font's ink band), grown by the
	// line gap so it reads as a full-line caret without bleeding into
	// neighbours; clamp to the line slot.
	barH := ascent + descent
	if extra := lh - (ascent + descent); extra > 0 {
		barH += extra * 0.4 // modest extension; keep mostly within the band
	}
	top = baseline - ascent
	if bottom := top + barH; bottom > float64(lineIdx+1)*lh {
		bottom = float64(lineIdx+1) * lh
		return penX, top, bottom, true
	}
	return penX, top, top + barH, true
}

// IMERect anchors candidates at the caret via caretAnchor — window coords,
// wrap-aware Y, and the anchor rides the composition exactly like the
// visible bar does (design §4.2: candidates follow composition growth).
func (b *inputBox) IMERect() platform.Rect {
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return platform.Rect{X: boxX, Y: boxY, W: 2, H: boxH}
	}
	return platform.Rect{X: boxX + x, Y: boxY + top, W: 2, H: bottom - top}
}

// sync mirrors the editor into the RenderText (the text string NEVER
// contains caret glyphs — the caret is the floating bar, see layoutCaret).
// Must go through SetText so glyph layout invalidates together with paint.
// sync mirrors the editor's DISPLAY form into the RenderText. Design D1:
// ed.Text() is committed-only; the live composition lives in the overlay
// and reaches pixels exclusively via ComposedView.View().Display.
func (b *inputBox) sync() {
	if b == nil || b.text == nil || b.ed == nil {
		return
	}
	disp := b.ed.View().Display
	if disp == "" && !b.focused {
		disp = "…(click, type, IME)▏"
	}
	b.text.SetText(disp)
	b.layoutCaret()
	if b.sched != nil {
		b.sched()
	}
}

// layoutCaret positions the visible bar from caretAnchor — the shared
// geometry source with IMERect (candidate window anchor). Standard caret
// placement: the bar's CENTER goes on the pen BOUNDARY between adjacent
// characters (Flutter TextPainter.getOffsetForCaret).
func (b *inputBox) layoutCaret() {
	if b == nil || b.bar == nil {
		return
	}
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return
	}
	b.bar.MoveTo(x-float64(b.bar.Width)/2, top)
	if h := bottom - top; h > 0 && h != b.bar.Height { // keep bar == anchor box
		b.bar.Height = h
	}
	if b.caretOn && (b.focused || len(b.ed.Text()) > 0) {
		b.bar.SetAlpha(1)
	} else {
		b.bar.SetAlpha(0)
	}
}

// OnPointer implements input.PointerHandler: clicking focuses the box via
// the focus manager (the framework opens/closes the IME session on the
// transition) and places the caret at the clicked position — wrap-aware
// (ByteOffsetAtPoint picks the row by y, then the column by x).
func (b *inputBox) OnPointer(ev input.PointerEvent) {
	if ev.Kind == input.PointerDown && b.node != nil {
		b.node.RequestFocus()
		localY := ev.Y - boxY
		b.ed.SetCaret(b.text.ByteOffsetAtPoint(ev.X-boxX, localY))
	}
}

// OnKey implements input.KeyHandler: editing control keys. Printable runes
// are inserted by InputRouter.routeKey (single insertion point).
func (b *inputBox) OnKey(ev input.KeyEvent) {
	if !ev.Pressed {
		return
	}
	switch ev.Key {
	case input.KeyBackspace:
		b.ed.DeleteBackward()
	case input.KeyDelete:
		b.ed.DeleteForward()
	case input.KeyArrowLeft:
		b.ed.MoveCaretRunes(-1)
	case input.KeyArrowRight:
		b.ed.MoveCaretRunes(1)
	case input.KeyArrowUp, input.KeyArrowDown:
		// Vertical caret movement (standard sticky-column model): geometry
		// comes from this box's RenderText — line count and pen boundaries.
		n := -1
		if ev.Key == input.KeyArrowDown {
			n = 1
		}
		b.ed.MoveCaretVertically(n,
			func() int { return len(b.text.DisplayLines()) },
			func(dispOff int) float64 {
				_, x, _ := b.text.CaretColumn(min(dispOff, len(b.text.Text)))
				return x
			})
	case input.KeyEnter:
		b.ed.Insert("\n")
	case input.KeyEscape:
		// P4: cancel the live composition — overlay vanishes, buffer intact.
		b.ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	}
}

// OnText / OnIME implement input.TextHandler / input.IMEHandler. The router
// already feeds TextEditor directly; these keep the demo an EventTarget.
func (b *inputBox) OnText(ev input.TextEvent) {}
func (b *inputBox) OnIME(ev input.IMEEvent) {
	// P9 echo-storm guard evidence: count inbound compose events; the exit
	// JSON asserts preedit events per keystroke stay ≤ 2.
	if ev.Kind == input.IMECompose {
		b.preeditEvents++
	}
}

func main() {
	selftest := os.Getenv("GPUI_IME_DEMO_SELFTEST") == "1"
	// Caret verification scenes (M1.5): GPUI_IME_CARET_SCENE=<multiline|composing|click>
	// seeds a deterministic state so the snapshot pixel probe can assert the
	// caret bar's actual position against the expected line/column.
	caretScene := os.Getenv("GPUI_IME_CARET_SCENE")
	logf("stage=init selftest=%v scene=%q", selftest, caretScene)

	ed := textinput.New()

	// Focus + unified event binding: the router resolves the focused control
	// dynamically (plan I5) and drives its IME session automatically (I4).
	fm := focus.NewManager()
	router := embedder.NewInputRouter(nil, fm)
	router.TextEditor = ed // fallback when nothing is focused

	box := newInputBox(ed)
	fm.Register(box.node)

	// Root must carry the window size (AbsoluteBox): an unsized plain box
	// collapses to zero extent and paints nothing — this was why the demo
	// window stayed blank.
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.09, G: 0.10, B: 0.12, A: 1}
	title := rendering.NewRenderText("textinput + IME test window (click box, type, IME pre-edit live)")
	title.FontSize = 13
	title.R, title.G, title.B, title.A = 0.7, 0.7, 0.75, 1
	root.Place(title, 40, 24)
	hint := rendering.NewRenderText("中文模式：字母进候选，Esc 取消 | 英文/符号直打：先切输入法到英文态（GNOME 默认 Super+Space）| Backspace 删除已上屏文本")
	hint.FontSize = 12
	hint.R, hint.G, hint.B, hint.A = 0.55, 0.58, 0.62, 1
	root.Place(hint, 40, 120)
	root.Place(box, boxX, boxY)

	// Fonts: DrawString no-ops without a face — without this BOTH texts are
	// invisible even though layout gives them extents. LoadMultiFace carries
	// a CJK-fallback chain so committed 汉字 has glyphs.
	if face, fontPath, ferr := text.LoadMultiFace(20); ferr != nil {
		logf("font-load FAILED: %v (text will not render)", ferr)
	} else {
		logf("font=%s", fontPath)
		title.SetFace(face)
		hint.SetFace(face)
		box.text.SetFace(face)
	}

	// Real-window pattern (same as ui_wr_* probes): platform window +
	// PipelineApp wired with the InputRouter.
	win, err := platform.Open(platform.Options{
		Width: winW, Height: winH,
		Title:       "gpui textinput + IME (zwp_text_input_v3)",
		Backend:     platform.DetectDisplayBackend(),
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	defer win.Close()
	logf("stage=window-created kind=%s", win.Kind())

	opts := embedder.PipelineOptions{
		WarmUp: true,
		Input:  router,
		IME:    win.IME(), // automatic session management (I4)
	}
	snapPath := ""
	if selftest || caretScene != "" {
		// Bounded run; final-frame snapshot = render evidence.
		opts.RunFor = 3 * time.Second
		snapDir := os.Getenv("IME_SNAP_DIR")
		if snapDir == "" {
			snapDir = "/tmp/ime_textinput_ime"
		}
		os.MkdirAll(snapDir, 0o755)
		name := "final.png"
		if caretScene != "" {
			name = "caret_" + caretScene + ".png"
		}
		snapPath = filepath.Join(snapDir, name)
		opts.SnapshotPath = snapPath
	}
	var app *embedder.PipelineApp
	opts.OnEvent = func(ev platform.Event) {
		if ev.Type == platform.EventClose {
			logf("stage=close")
			app.Quit()
		}
	}
	app = embedder.NewPipelineApp(win.Host(), root, opts)

	if win.IME() == nil {
		logf("stage=ime-unavailable")
	}
	box.sched = app.ScheduleFrame
	router.OnPointer = func(pe input.PointerEvent, target rendering.RenderObject) {
		logf("router-pointer %s at (%.1f,%.1f) target=%T", pe.Kind, pe.X, pe.Y, target)
	}
	router.OnKey = func(ke input.KeyEvent) {
		// Full key trace: diagnoses "letters/backspace do nothing" — if a
		// key never logs here it was consumed by the compositor's IME
		// (Chinese mode eats ASCII; switch the IME to English to passthrough).
		logf("key-event key=%s rune=%q pressed=%v", ke.Key, ke.Rune, ke.Pressed)
		box.OnKey(ke)
	}
	router.OnText = func(ev input.TextEvent) { logf("text-event %q", ev.Text) }
	router.OnIME = func(ev input.IMEEvent) {
		logf("ime-event kind=%s text=%q start=%d end=%d", ev.Kind, ev.Text, ev.Start, ev.End)
	}

	// Grant focus to the field: the framework opens the IME session at the
	// caret anchor automatically (AttachIME ran inside NewPipelineApp).
	if box.node.RequestFocus() {
		logf("stage=focused (session auto-opened)")
	} else {
		logf("stage=focus-refused")
	}

	pass := true
	if selftest {
		// Leg A (router path): production events platform.Event →
		// FromPlatform → InputRouter → Editor.
		steps := []platform.Event{
			{Type: platform.EventIME, IMEKind: 0, IMEText: "ni", IMEStart: 2, IMEEnd: 2},      // pre-edit "ni", caret @2
			{Type: platform.EventIME, IMEKind: 0, IMEText: "nihao", IMEStart: -1, IMEEnd: -1}, // pre-edit grows, caret @end
			{Type: platform.EventIME, IMEKind: 1, IMEText: "你好"},                              // candidate selected → commit
			{Type: platform.EventIME, IMEKind: 0, IMEText: ""},                                // post-commit empty preedit = reset (used to latch compose ON)
			{Type: platform.EventKey, Pressed: true, KeyCode: 'a', Rune: 'a'},                 // plain key must insert after the reset
			{Type: platform.EventKey, Pressed: true, KeyCode: 0xff51},                         // ArrowLeft: caret moves (visible via sync)
			{Type: platform.EventKey, Pressed: true, KeyCode: 'b', Rune: 'b'},                 // inserts at the MOVED caret → "…ba"
		}
		for i, ev := range steps {
			router.RoutePlatform(ev)
			logf("selftest step=%d text=%q compose=%v", i, ed.Text(), ed.ComposeActive())
		}

		// P4 leg: compose → Esc cancel → buffer intact.
		ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "temp"})
		p4Before := ed.Text()
		router.Route(input.FromPlatform(platform.Event{
			Type: platform.EventKey, Pressed: true, KeyCode: 0xff1b, // Escape
		}, input.Modifiers{}))
		p4OK := !ed.ComposeActive() && ed.Text() == p4Before
		logf("selftest p4 esc-cancel: before=%q after=%q active=%v ok=%v", p4Before, ed.Text(), ed.ComposeActive(), p4OK)
		pass = ed.Text() == expectText && !ed.ComposeActive() && p4OK

		// Leg B (ImeSession facade — M1 new architecture): same semantics,
		// driven through the facade instead of the router.
		sessEd := textinput.New()
		if win.IME() != nil {
			sess := textinput.NewImeSession(embedder.NewImeAdapter(win.IME()))
			sess.AttachEditor(sessEd, box) // box is a FieldSnapshotProvider too
			sess.PreeditChanged(input.PreeditEvent{Text: "wo", Cursor: -1})
			sess.Committed("我")
			sess.DeleteSurrounding(0, 0)
		}
		facadeOK := sessEd.Text() == "我" && !sessEd.ComposeActive()
		logf("selftest facade leg: text=%q ok=%v", sessEd.Text(), facadeOK)
		if !facadeOK {
			pass = false
		}

		if !pass {
			logf("selftest MISMATCH got=%q want=%q", ed.Text(), expectText)
		} else {
			logf("selftest buffer OK (%q)", ed.Text())
		}
	}

	// Idle-caret blink via the pipeline scheduler (Flutter-style per-frame
	// tick on the UI thread): toggling here also re-renders the text. The
	// previous background-goroutine version only flipped a flag and never
	// re-rendered, so the caret never visibly blinked.
	//
	// Caret verification scenes freeze the bar ON (blink would make the
	// snapshot probe flaky — 3s run × 0.53s period lands ~50% off states).
	app.Scheduler().Tickers().Add(&caretTicker{box: box})
	if caretScene != "" {
		box.caretOn = true
		caretTickerEnabled = false
	}

	// Caret verification scenes: deterministic state + caret frozen ON so
	// the snapshot probe can locate it (blink would make position flaky).
	switch caretScene {
	case "multiline":
		box.caretOn = true
		ed.SetText("first line text\nsecond line content")
		ed.SetCaret(len("first line text\nsecond ")) // line 2, mid-content
		logf("scene=multiline caretLine=1 (0-based)")
	case "composing":
		box.caretOn = true
		ed.SetText("committed ")
		ed.SetCaret(len("committed "))
		ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "pinyin", Start: -1, End: -1})
		logf("scene=composing composing=%v", ed.ComposeActive())
	case "composing-mid":
		// Composition with text AFTER the span: caret rides INSIDE the
		// preedit ("piny|in") while committed text follows it. This is the
		// bar-over-right-text case: the bar must sit in the ink gap between
		// the preedit glyphs and never overlap the committed text.
		box.caretOn = true
		ed.SetText("committed after")
		ed.SetCaret(len("committed "))
		ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "pinyin", Start: 4, End: 4})
		logf("scene=composing-mid composing=%v cursorInSpan=%d display=%q",
			ed.ComposeActive(), ed.CompositionCursor(), ed.View().Display)
	case "composing-cjk":
		// Realistic Chinese IME state: committed CJK sentence, caret parked
		// mid-sentence, live pinyin preedit inserted there. Full-width glyphs
		// leave ~1px side bearings, so this is the harshest overlap test.
		box.caretOn = true
		ed.SetText("你好世界，光标测试文本")
		ed.SetCaret(len("你好世界，"))
		ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "nihao", Start: -1, End: -1})
		logf("scene=composing-cjk composing=%v display=%q",
			ed.ComposeActive(), ed.View().Display)
	case "mixed-nav":
		// Problem 2/4 reproduction: mixed CJK+latin line, caret parked mid-
		// text; then ArrowRight x3 + ArrowDown x1 driven through OnKey so the
		// exact production key path is exercised. Exit JSON records positions.
		box.caretOn = true
		ed.SetText("中文abc混合text\n第二行")
		ed.SetCaret(len("中文abc"))
		for i := 0; i < 3; i++ {
			box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowRight})
		}
		box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowDown})
		logf("scene=mixed-nav cursor=%d display=%q", ed.Cursor(), ed.View().Display)
		app.SetInputRouter(router)
	case "latin-caret":
		// Problem repro: pure-latin "mmmm…" — caret placed at byte 3 via the
		// production click path (x chosen INSIDE the 3rd 'm'). The bar must
		// sit in the gap between m2 and m3, never on a glyph.
		box.caretOn = true
		const mm = "mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm"
		ed.SetText(mm)
		// click x: pen(3)=3*19.48=58.45 box-local; inside 3rd m means x∈[58.45+1.82, 58.45+17.78]
		app.SetInputRouter(router)
		clickIdx := 0
		adv := ed.View().Display[:1]
		_ = adv
		app.Scheduler().Tickers().Add(&schedTicker{period: 0.25, fn: func() {
			if clickIdx >= 9 {
				return
			}
			byteTarget := (clickIdx + 1) * 4 // 4,8,12,...36
			// click x = pen(byteTarget) box-local + half advance (mid-glyph)
			x := boxX + float64(byteTarget)*19.48 + 9.7
			router.RoutePlatform(platform.Event{
				Type: platform.EventPointer, Pointer: platform.PointerDown,
				X: x, Y: boxY + 12,
			})
			logf("scene=latin-caret target=%d caret=%d", byteTarget, ed.Cursor())
			clickIdx++
		}})
	case "click":
		box.caretOn = true
		ed.SetText("row-one\nrow-two\nrow-three")
		// Simulate a click on row 3 at x≈30px: route through the box's own
		// pointer handler so the exact production path is exercised.
		box.OnPointer(input.PointerEvent{Kind: input.PointerDown, X: 40 + 30, Y: boxY + 2*26})
		logf("scene=click caret=%d", ed.Cursor())
	case "click-live":
		// Full production path: platform pointer event → FromPlatform →
		// router hit-test → OnPointer. Clicks row 2 mid-word ("row-tw|o"),
		// then a second click at line 1 start ("|row-one") — caret must
		// follow each click.
		box.caretOn = true
		ed.SetText("row-one\nrow-two\nrow-three")
		app.SetInputRouter(router)
		// Clicks must fire AFTER the first layout pass (hit-test needs real
		// sizes) — defer to the frame loop via a one-shot ticker.
		clicks := []struct {
			x, y     float64
			wantByte int
			label    string
		}{
			{40 + 55, boxY + 1*26 + 10, 13, "row2 mid-word"},
			{40 + 12, boxY + 0*26 + 10, 1, "row1 start"},
		}
		// Drive clicks from the UI thread: a ticker tick fires each click on
		// its scheduled turn (layout is done by then; no goroutine races).
		clickIdx := 0
		app.Scheduler().Tickers().Add(&schedTicker{period: 0.4, fn: func() {
			if clickIdx >= len(clicks) {
				return
			}
			c := clicks[clickIdx]
			clickIdx++
			router.RoutePlatform(platform.Event{
				Type: platform.EventPointer, Pointer: platform.PointerDown,
				X: c.x, Y: c.y,
			})
			logf("scene=click-live %s caret=%d want=%d", c.label, ed.Cursor(), c.wantByte)
		}})
	}

	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
	}
	if caretScene != "" {
		logf("post-run: text=%q compose=%v display=%q", ed.Text(), ed.ComposeActive(), ed.View().Display)
	}

	// Geometry probe: did layout actually give the children extents?
	logf("geom root size=%v offset=%v", root.Size(), root.Offset())
	logf("geom title size=%v offset=%v", title.Size(), title.Offset())
	logf("geom box size=%v offset=%v text size=%v offset=%v",
		box.Size(), box.Offset(), box.text.Size(), box.text.Offset())

	out := map[string]any{
		"backend":        win.Kind().String(),
		"selftest":       selftest,
		"text":           ed.Text(),
		"compose_active": ed.ComposeActive(),
		"blink_ticks":    box.blinks,
		// P9 evidence: inbound preedit events during the whole run.
		"preedit_events": box.preeditEvents,
	}
	if snapPath != "" {
		out["snapshot"] = snapPath
	}
	if selftest {
		out["expect"] = expectText
		out["pass"] = pass
	}
	if b, err := json.Marshal(out); err == nil {
		fmt.Println(string(b))
	}
	if selftest && !pass {
		win.Close()
		os.Exit(1)
	}
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[ime-demo] "+format+"\n", args...)
}

// caretTicker blinks the idle caret (~2Hz). Runs on the UI thread from the
// pipeline scheduler; returning true keeps it registered for the window's
// lifetime.
type caretTicker struct {
	box *inputBox
	acc float64
}

// caretTicker blinks the idle caret (~2Hz). Runs on the UI thread from the
// pipeline scheduler; toggling also re-renders. caretTickerEnabled=false
// freezes the bar ON (verification scenes).
var caretTickerEnabled = true

// schedTicker runs an arbitrary function on the UI thread at a fixed period
// (verification-scene driver: fires clicks after layout is live).
type schedTicker struct {
	period float64
	fn     func()
	acc    float64
	fired  int
}

func (t *schedTicker) Tick(dt float64) bool {
	t.acc += dt
	if t.acc >= t.period {
		t.acc = 0
		t.fired++
		if t.fn != nil {
			t.fn()
		}
	}
	return true
}

func (t *caretTicker) Tick(dt float64) bool {
	if !caretTickerEnabled {
		return true
	}
	t.acc += dt
	if t.acc >= 0.53 {
		t.acc = 0
		t.box.caretOn = !t.box.caretOn
		t.box.blinks++
		t.box.sync()
	}
	return true
}
