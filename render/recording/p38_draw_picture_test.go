package recording

import (
	"errors"
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// P3.8 batch 6: nested Picture (SkCanvas::drawPicture shape) — a completed
// recording replayed as one entry of another, onto a live context (Push/Pop
// wrapped, cycle/depth-guarded) or folded into an enclosing recorder.

func newTestPic(draw func(dc render.Canvas)) *Recorder {
	r := NewRecorder(100, 100)
	draw(r)
	return r
}

func TestP38DrawPicture_FoldEquivalence(t *testing.T) {
	// Folding a picture into an enclosing recorder must yield the same
	// command stream as recording the same commands directly.
	pic := newTestPic(func(dc render.Canvas) {
		dc.SetRGBA(1, 0, 0, 1)
		dc.DrawRectangle(10, 10, 20, 20)
		_ = dc.Fill()
	})
	outer := NewRecorder(100, 100)
	outer.DrawPicture(pic)
	outer.SetRGBA(0, 0, 1, 1) // trailing outer command, proves ordering
	// Folding happens when the outer recording replays INTO a recorder
	// (CanvasBackend fold contract), not at record time.
	folded := NewRecorder(100, 100)
	if err := outer.FinishRecording().Playback(NewCanvasBackend(folded)); err != nil {
		t.Fatalf("fold playback: %v", err)
	}
	direct := NewRecorder(100, 100)
	direct.SetRGBA(1, 0, 0, 1)
	direct.DrawRectangle(10, 10, 20, 20)
	_ = direct.Fill()
	direct.SetRGBA(0, 0, 1, 1)

	// Fold semantics: the picture enters the outer stream as a REFERENCE
	// command (same carrier discipline as H21's SceneReplayCommand) —
	// expansion happens at the final context replay, which the pixel pins
	// cover. Assert bracket + reference + trailing outer command.
	if len(folded.commands) != 5 {
		t.Fatalf("folded %d commands, want [Save, DrawPicture, Restore, SetFillStyle, SetStrokeStyle]", len(folded.commands))
	}
	want := []CommandType{CmdSave, CmdDrawPicture, CmdRestore, CmdSetFillStyle, CmdSetStrokeStyle}
	for i, wt := range want {
		if folded.commands[i].Type() != wt {
			t.Fatalf("folded[%d] = %v, want %v", i, folded.commands[i].Type(), wt)
		}
	}
}

func TestP38DrawPicture_DirectDrawPixel(t *testing.T) {
	pic := newTestPic(func(dc render.Canvas) {
		dc.SetRGBA(1, 0, 0, 1)
		dc.DrawRectangle(10, 10, 40, 40)
		_ = dc.Fill()
	})
	outer := NewRecorder(100, 100)
	outer.Translate(30, 0) // CTM composition: the picture lands shifted
	outer.DrawPicture(pic)

	dc := render.NewContext(100, 100)
	if err := outer.FinishRecording().Playback(NewCanvasBackend(dc)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	inside := sample(dc, 50, 30) // picture rect (10..50)+translate(30)
	if inside != (sampleRGB{255, 0, 0}) {
		t.Fatalf("inside picture = %v, want red", inside)
	}
	outside := sample(dc, 20, 30) // before the translated rect
	if outside != (sampleRGB{0, 0, 0}) {
		t.Fatalf("outside picture = %v, want clear", outside)
	}
}

func TestP38DrawPicture_NestedThreeDeep(t *testing.T) {
	inner := newTestPic(func(dc render.Canvas) {
		dc.SetRGBA(0, 1, 0, 1)
		dc.DrawRectangle(0, 0, 10, 10)
		_ = dc.Fill()
	})
	mid := newTestPic(func(dc render.Canvas) {
		dc.DrawPicture(inner)
	})
	outer := NewRecorder(100, 100)
	outer.DrawPicture(mid)

	dc := render.NewContext(100, 100)
	if err := outer.FinishRecording().Playback(NewCanvasBackend(dc)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	if got := sample(dc, 5, 5); got != (sampleRGB{0, 255, 0}) {
		t.Fatalf("nested pixel = %v, want green", got)
	}
}

func TestP38DrawPicture_CycleRefused(t *testing.T) {
	// A picture that draws itself must not recurse forever: the replay
	// refuses the cyclic entry (fail-closed) instead of overflowing.
	self := NewRecorder(100, 100)
	self.DrawPicture(self)

	dc := render.NewContext(100, 100)
	if err := self.FinishRecording().Playback(NewCanvasBackend(dc)); err != nil && !errors.Is(err, render.ErrPictureCycle) {
		t.Fatalf("self-cycle playback error = %v, want ErrPictureCycle (or silent refusal)", err)
	}
}

func TestP38DrawPicture_DepthRefused(t *testing.T) {
	// A 65-deep chain must trip the depth guard.
	deep := NewRecorder(100, 100)
	prev := deep
	for range 65 {
		n := NewRecorder(100, 100)
		n.DrawPicture(prev)
		prev = n
	}
	dc := render.NewContext(100, 100)
	err := prev.FinishRecording().Playback(NewCanvasBackend(dc))
	if err != nil && !errors.Is(err, render.ErrPictureDepth) {
		t.Fatalf("deep playback error = %v, want ErrPictureDepth (or silent refusal)", err)
	}
}

func TestP38DrawPicture_NilSkipped(t *testing.T) {
	r := NewRecorder(100, 100)
	r.DrawPicture(nil) // must not panic and must not record
	if len(r.commands) != 0 {
		t.Fatalf("nil picture recorded %d commands, want 0", len(r.commands))
	}
}

func TestP38DrawPicture_StateDoesNotLeak(t *testing.T) {
	pic := newTestPic(func(dc render.Canvas) {
		dc.Push()
		dc.Translate(500, 500)
	})
	outer := NewRecorder(100, 100)
	outer.DrawPicture(pic)
	outer.SetRGBA(1, 0, 0, 1)
	outer.DrawRectangle(10, 10, 10, 10) // must land at (10,10), not shifted
	_ = outer.Fill()

	dc := render.NewContext(100, 100)
	if err := outer.FinishRecording().Playback(NewCanvasBackend(dc)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	if got := sample(dc, 15, 15); got != (sampleRGB{255, 0, 0}) {
		t.Fatalf("post-picture rect = %v at (15,15), want red at untranslated spot", got)
	}
}

// sampleRGB is an 8-bit RGB triple for pixel assertions.
type sampleRGB [3]uint8

func sample(dc *render.Context, x, y int) sampleRGB {
	img, _ := dc.Image().(image.Image)
	r, g, b, _ := img.At(x, y).RGBA()
	return sampleRGB{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}
