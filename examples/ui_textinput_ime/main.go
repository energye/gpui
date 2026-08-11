// Command ui_textinput_ime proves the cross-platform input pipeline end to
// end on a real Wayland window (plan §6): platform.Event → input.FromPlatform
// → InputRouter → focused textinput.Editor, with the IME pre-edit rendered
// live.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	GPUI_DISPLAY=wayland go run ./examples/ui_textinput_ime
//
// Controls:
//   - Click the input box to focus it (enables IME via zwp_text_input_v3).
//   - Type ASCII: committed via keyboard. With an input method (IBus/fcitx)
//     active, pre-edit (拼音候选) shows live and commits on Enter/selection.
//   - Backspace deletes; arrows move the caret.
//
// JSON at exit reports the editor text and IME activity.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/ui/application"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/textinput"

	_ "github.com/energye/gpui/render/gpu"
)

// inputBox renders the editor text and caret-ish underline; it is the
// EventTarget that receives pointer/key/text/ime from the InputRouter.
type inputBox struct {
	*rendering.RenderBox
	ed   *textinput.Editor
	text *rendering.RenderText
}

func newInputBox(ed *textinput.Editor) *inputBox {
	box := rendering.NewRenderBox()
	b := &inputBox{RenderBox: box, ed: ed, text: rendering.NewRenderText("")}
	b.text.FontSize = 20
	b.text.R, b.text.G, b.text.B, b.text.A = 0.05, 0.75, 0.95, 1
	b.FixedWidth = 600
	b.FixedHeight = 48
	b.AddChild(b.text)
	ed.OnChange = func() { b.sync() }
	b.sync()
	return b
}

func (b *inputBox) sync() {
	if b == nil || b.text == nil || b.ed == nil {
		return
	}
	t := b.ed.Text()
	if b.ed.ComposeActive() {
		t += "▌"
	} else if len(t) > 0 {
		t += "|"
	} else {
		t = "…(click, type, IME)▏"
	}
	b.text.Text = t
	b.text.MarkNeedsPaint()
}

// OnPointer implements input.PointerHandler: clicking focuses (the demo uses
// a fixed caret; the InputRouter's TextEditor is pre-bound so text lands).
func (b *inputBox) OnPointer(ev input.PointerEvent) {}

// OnKey implements input.KeyHandler: editing keys.
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

// OnText implements input.TextHandler.
func (b *inputBox) OnText(ev input.TextEvent) {}

// OnIME implements input.IMEHandler.
func (b *inputBox) OnIME(ev input.IMEEvent) {}

func main() {
	const winW, winH = 900, 300
	ed := textinput.New()

	// Route normalized text/IME into the focused editor automatically.
	router := embedder.NewInputRouter(nil, nil)
	router.TextEditor = ed

	app := application.New(application.Config{
		Name:    "ui_textinput_ime",
		Backend: platform.DisplayWayland,
	})
	win, err := app.NewWindow(application.WindowOptions{
		Width: winW, Height: winH, Title: "gpui textinput + IME (zwp_text_input_v3)",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	win.SetInput(router)

	box := newInputBox(ed)
	box.SetOffset(rendering.Point{X: 40, Y: 60})

	root := rendering.NewRenderBox()
	title := rendering.NewRenderText("textinput + IME demo (click box, type, IME pre-edit live)")
	title.FontSize = 13
	title.R, title.G, title.B, title.A = 0.7, 0.7, 0.75, 1
	title.SetOffset(rendering.Point{X: 40, Y: 24})
	root.AddChild(title)
	root.AddChild(box)

	if err := win.SetRoot(root); err != nil {
		fmt.Fprintln(os.Stderr, "root:", err)
		os.Exit(1)
	}

	// Auto-focus the editor so IME is enabled immediately (demonstration).
	router.TextEditor = ed

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
	}
	app.Close()
	_ = t0

	// Wire IME capability if the compositor exposes it (informational).
	var ime string
	if p := win.Platform(); p != nil && p.IME() != nil {
		ime = "available"
	} else {
		ime = "unavailable"
	}

	out := map[string]any{
		"ime_capability": ime,
		"text":           ed.Text(),
		"compose_active": ed.ComposeActive(),
		"backend":        win.Platform().Kind().String(),
	}
	if b, err := json.Marshal(out); err == nil {
		fmt.Println(string(b))
	}
	fmt.Fprintf(os.Stderr, "ui_textinput_ime: ime=%s text=%q compose=%v\n", ime, ed.Text(), ed.ComposeActive())
}
