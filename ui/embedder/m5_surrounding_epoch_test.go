package embedder

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/textinput"
)

// fakeM5IME counts surrounding pushes. WantsSurrounding=true forces the push
// path even when SurroundingUpdates is off (X11 D-Bus behavior).
type fakeM5IME struct {
	pushes int
	last   string
	cursor int
}

func (f *fakeM5IME) EnableIME(rect platform.Rect)                   {}
func (f *fakeM5IME) UpdateCursorRect(rect platform.Rect)            {}
func (f *fakeM5IME) SetContentType(purpose platform.ContentPurpose) {}
func (f *fakeM5IME) Commit(text string)                             {}
func (f *fakeM5IME) DisableIME()                                    {}
func (f *fakeM5IME) WantsSurrounding() bool                         { return true }
func (f *fakeM5IME) SetComposing(text string, cursor int) {
	f.pushes++
	f.last = text
	f.cursor = cursor
}

// TestPushSurroundingEpochDedupe_M5 locks M5-6: surrounding dedupe compares
// the Editor epoch (O(1)) instead of building the 4000-byte key string.
func TestPushSurroundingEpochDedupe_M5(t *testing.T) {
	r := NewInputRouter(nil, nil)
	ed := textinput.New()
	ed.SetTextSimple("hello")
	ime := &fakeM5IME{}

	r.pushSurroundingForEditor(ime, ed)
	if ime.pushes != 1 {
		t.Fatalf("first push = %d, want 1", ime.pushes)
	}
	if got := r.lastSurrEpoch; got != ed.Epoch() {
		t.Fatalf("lastSurrEpoch = %d, want editor epoch %d", got, ed.Epoch())
	}

	// Same epoch, no edit: must not push again.
	r.pushSurroundingForEditor(ime, ed)
	if ime.pushes != 1 {
		t.Fatalf("repeat push with same epoch = %d, want 1 (dedupe)", ime.pushes)
	}

	// Real edit bumps epoch: must push again.
	ed.SetTextSimple("hello world")
	r.pushSurroundingForEditor(ime, ed)
	if ime.pushes != 2 {
		t.Fatalf("push after edit = %d, want 2", ime.pushes)
	}
	if got := r.lastSurrEpoch; got != ed.Epoch() {
		t.Fatalf("lastSurrEpoch after edit = %d, want %d", got, ed.Epoch())
	}
}
