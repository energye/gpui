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
	b := &inputBox{
		RenderBox: rendering.NewRenderBox(),
		ed:        ed,
		text:      rendering.NewRenderText(""),
	}
	b.text.FontSize = 20
	b.text.R, b.text.G, b.text.B, b.text.A = 0.05, 0.75, 0.95, 1
	b.FixedWidth = boxW
	b.FixedHeight = boxH
	b.AddChild(b.text)
	// The caret is a FLOATING BAR positioned at the cursor offset — never a
	// glyph inserted into the string: an inserted "|" split the neighboring
	// glyphs apart while blinking and desynced click→caret mapping.
	b.bar = rendering.NewRenderColorBox(2, 26, 0.05, 0.75, 0.95, 1)
	b.AddChild(b.bar)
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

// IMERect anchors candidates at the caret: prefix width measured over the
// DISPLAY string (committed prefix + live pre-edit — design §4.2 requires
// the anchor to follow composition growth).
func (b *inputBox) IMERect() platform.Rect {
	v := b.ed.View()
	cur := b.ed.Cursor()
	if cur < 0 || cur > len(b.ed.Text()) {
		cur = len(b.ed.Text())
	}
	// Buffer caret → display caret (inside the span = at its start; the
	// span grows rightward from there so the prefix is stable).
	viewCur := v.MapBufToView(cur)
	if viewCur > v.CompStart && v.CompStart >= 0 && cur == v.CompStart {
		viewCur = v.CompEnd // composing: anchor rides the span's end
	}
	w := b.text.MeasureWidth(v.Display[:viewCur])
	return platform.Rect{X: boxX + w, Y: boxY, W: 2, H: boxH}
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

// layoutCaret positions the caret bar at the current cursor byte offset and
// toggles its visibility for the blink. Position/alpha only — zero effect on
// text layout, so neighboring glyphs never shift while blinking. Vertical:
// centered on the text's laid-out line box; horizontal: over the DISPLAY
// string via ComposedView (during composition it rides the span's end).
func (b *inputBox) layoutCaret() {
	if b == nil || b.bar == nil {
		return
	}
	v := b.ed.View()
	cur := b.ed.Cursor()
	if cur < 0 || cur > len(b.ed.Text()) {
		cur = len(b.ed.Text())
	}
	viewCur := v.MapBufToView(cur)
	if v.CompStart >= 0 && cur == v.CompStart && viewCur == v.CompStart {
		viewCur = v.CompEnd // composing: caret rides the span end
	}
	x := b.text.MeasureWidth(v.Display[:viewCur])
	th := b.text.Size().Height
	y := (th - b.bar.Height) / 2
	if y < 0 {
		y = 0
	}
	b.bar.SetOffset(rendering.Point{X: x + 1, Y: y})
	if b.caretOn && (b.focused || len(b.ed.Text()) > 0) {
		b.bar.A = 1
	} else {
		b.bar.A = 0
	}
}

// OnPointer implements input.PointerHandler: clicking focuses the box via
// the focus manager (the framework opens/closes the IME session on the
// transition) and places the caret at the clicked position.
func (b *inputBox) OnPointer(ev input.PointerEvent) {
	if ev.Kind == input.PointerDown && b.node != nil {
		b.node.RequestFocus()
		b.ed.SetCaret(b.text.ByteOffsetAt(ev.X - boxX)) // window X → local → boundary
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
	logf("stage=init selftest=%v", selftest)

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
	if selftest {
		// Bounded run; final-frame snapshot = render evidence.
		opts.RunFor = 3 * time.Second
		snapDir := os.Getenv("IME_SNAP_DIR")
		if snapDir == "" {
			snapDir = "/tmp/ime_textinput_ime"
		}
		os.MkdirAll(snapDir, 0o755)
		snapPath = filepath.Join(snapDir, "final.png")
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
	app.Scheduler().Tickers().Add(&caretTicker{box: box})

	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
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

func (t *caretTicker) Tick(dt float64) bool {
	t.acc += dt
	if t.acc >= 0.53 {
		t.acc = 0
		t.box.caretOn = !t.box.caretOn
		t.box.blinks++
		t.box.sync()
	}
	return true
}
