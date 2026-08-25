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
	// expectText is the self-test expectation: pinyin commit "你好" followed
	// by one plain ASCII keystroke "a".
	expectText = "你好a"
)

// inputBox renders the editor text plus a caret/compose marker. It is an
// EventTarget AND a TextEditTarget: the framework drives its whole IME
// session from focus transitions and edits (plan I4/I5) — no manual
// EnableIME/DisableIME anywhere.
type inputBox struct {
	*rendering.RenderBox
	ed      *textinput.Editor
	text    *rendering.RenderText
	node    *focus.FocusNode // registered with the window's focus manager
	sched   func()           // set before Run: request a frame after edits
	focused bool
	caretOn bool // idle-caret blink state (UI-thread only)
	blinks  int  // completed blink toggles (evidence in exit JSON)
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

// ContentPurpose declares a plain-text field.
func (b *inputBox) ContentPurpose() platform.ContentPurpose { return platform.PurposeNormal }

// IMERect anchors candidates at the caret: prefix width measured with the
// same face/size the text paints with.
func (b *inputBox) IMERect() platform.Rect {
	cursor := b.ed.Cursor()
	if cursor < 0 || cursor > len(b.ed.Text()) {
		cursor = len(b.ed.Text())
	}
	w := b.text.MeasureWidth(b.ed.Text()[:cursor])
	return platform.Rect{X: boxX + w, Y: boxY, W: 2, H: boxH}
}

// sync mirrors the editor into the RenderText. Must go through SetText (not
// a raw field write): SetText also invalidates glyph layout, a paint-only
// dirty would keep stale runs and never show newly typed characters.
func (b *inputBox) sync() {
	if b == nil || b.text == nil || b.ed == nil {
		return
	}
	t := b.ed.Text()
	switch {
	case b.ed.ComposeActive():
		t += "▌" // pre-edit live region marker
	case b.focused || len(t) > 0:
		if b.caretOn {
			t += "|"
		}
	default:
		t = "…(click, type, IME)▏"
	}
	b.text.SetText(t)
	if b.sched != nil {
		b.sched()
	}
}

// OnPointer implements input.PointerHandler: clicking focuses the box via
// the focus manager — the framework opens/closes the IME session on the
// transition itself.
func (b *inputBox) OnPointer(ev input.PointerEvent) {
	if ev.Kind == input.PointerDown && b.node != nil {
		b.node.RequestFocus()
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
	}
}

// OnText / OnIME implement input.TextHandler / input.IMEHandler. The router
// already feeds TextEditor directly; these keep the demo an EventTarget.
func (b *inputBox) OnText(ev input.TextEvent) {}
func (b *inputBox) OnIME(ev input.IMEEvent)   {}

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
		// Pre-loop injection (race-free): each event takes the production
		// path platform.Event → FromPlatform → InputRouter → Editor.
		steps := []platform.Event{
			{Type: platform.EventIME, IMEKind: 0, IMEText: "ni", IMEStart: 2, IMEEnd: 2},      // pre-edit "ni", caret @2
			{Type: platform.EventIME, IMEKind: 0, IMEText: "nihao", IMEStart: -1, IMEEnd: -1}, // pre-edit grows, caret @end
			{Type: platform.EventIME, IMEKind: 1, IMEText: "你好"},                              // candidate selected → commit
			{Type: platform.EventIME, IMEKind: 0, IMEText: ""},                                // post-commit empty preedit = reset (used to latch compose ON)
			{Type: platform.EventKey, Pressed: true, KeyCode: 'a', Rune: 'a'},                 // plain key must insert after the reset
		}
		for i, ev := range steps {
			router.RoutePlatform(ev)
			logf("selftest step=%d text=%q compose=%v", i, ed.Text(), ed.ComposeActive())
		}
		pass = ed.Text() == expectText && !ed.ComposeActive()
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
