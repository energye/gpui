package recording

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// canvasOnly hides the MaskCanvas capability while still being a full Canvas.
// It models an export/snapshot target that can replay user-space geometry but
// cannot reproduce a device-space mask plane.
type canvasOnly struct {
	render.Canvas
}

func isBlue(c render.RGBA) bool {
	return c.B > 0.8 && c.R < 0.3 && c.G < 0.3 && c.A > 0.8
}

// halfMask returns a mask that keeps the left half fully opaque and clears the
// right half.
func halfMask(w, h int) *render.Mask {
	m := render.NewMask(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w/2; x++ {
			m.Set(x, y, 255)
		}
	}
	return m
}

func TestRecorderMaskCommands(t *testing.T) {
	rec := NewRecorder(8, 8)
	m := render.NewMask(8, 8)
	m.Fill(255)

	rec.SetMask(m)
	rec.InvertMask()
	rec.ClearMask()
	rec.ApplyMask(m)
	rec.PushMaskLayer(m)
	rec.PopLayer()

	r := rec.FinishRecording()
	want := []CommandType{
		CmdSetMask,
		CmdInvertMask,
		CmdClearMask,
		CmdApplyMask,
		CmdPushMaskLayer,
		CmdPopLayer,
	}
	if len(r.Commands()) != len(want) {
		t.Fatalf("commands = %d, want %d", len(r.Commands()), len(want))
	}
	for i, ct := range want {
		if got := r.Commands()[i].Type(); got != ct {
			t.Errorf("commands[%d].Type() = %v, want %v", i, got, ct)
		}
	}
	for _, ct := range []CommandType{CmdSetMask, CmdApplyMask, CmdPushMaskLayer} {
		var mask *render.Mask
		switch c := r.Commands()[map[CommandType]int{CmdSetMask: 0, CmdApplyMask: 3, CmdPushMaskLayer: 4}[ct]].(type) {
		case SetMaskCommand:
			mask = c.Mask
		case ApplyMaskCommand:
			mask = c.Mask
		case PushMaskLayerCommand:
			mask = c.Mask
		}
		if mask == nil || mask.Width() != 8 || mask.Height() != 8 {
			t.Errorf("%v snapshot = %v, want an 8x8 mask", ct, mask)
		}
	}
}

// The recorder owns an immutable snapshot: the caller may keep mutating its
// Mask (the live scene does exactly that with InvertMask) without changing the
// display list, and playback must clone again so one run cannot poison the next.
func TestMaskSnapshotIsImmutable(t *testing.T) {
	const w, h = 8, 8
	m := render.NewMask(w, h)
	m.Fill(255)

	rec := NewRecorder(w, h)
	rec.SetMask(m)
	r := rec.FinishRecording()
	cmd, ok := r.Commands()[0].(SetMaskCommand)
	if !ok {
		t.Fatalf("commands[0] = %T, want SetMaskCommand", r.Commands()[0])
	}

	m.Fill(0)
	if got := cmd.Mask.At(0, 0); got != 255 {
		t.Fatalf("recorded mask changed to %d after the caller mutated its Mask", got)
	}

	dc := render.NewContext(w, h)
	if err := r.Playback(NewCanvasBackend(dc)); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if got := cmd.Mask.At(0, 0); got != 255 {
		t.Fatalf("recorded mask changed to %d during playback", got)
	}
}

// Replaying the same recording twice must produce the same pixels. InvertMask
// mutates the active mask in place, so a backend that handed the command's
// snapshot straight to the target would flip it permanently on the first run.
func TestMaskPlaybackIsRepeatable(t *testing.T) {
	const w, h = 16, 16
	m := halfMask(w, h)

	rec := NewRecorder(w, h)
	rec.SetMask(m)
	rec.SetRGBA(1, 0, 0, 1)
	rec.DrawRectangle(0, 0, w, h)
	if err := rec.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	rec.InvertMask()
	rec.SetRGBA(0, 0, 1, 1)
	rec.DrawRectangle(0, 0, w, h)
	if err := rec.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	rec.ClearMask()
	r := rec.FinishRecording()

	if got := m.At(0, 0); got != 255 {
		t.Fatalf("recording mutated the caller's mask: At(0,0) = %d", got)
	}

	for run := 0; run < 2; run++ {
		dc := render.NewContext(w, h)
		if err := r.Playback(NewCanvasBackend(dc)); err != nil {
			t.Fatalf("run %d Playback: %v", run, err)
		}
		img, ok := dc.Image().(*image.RGBA)
		if !ok {
			t.Fatalf("run %d image type = %T", run, dc.Image())
		}
		if c := rgbaAt(img, 2, 2); !isRed(c) {
			t.Errorf("run %d left = %v, want red (canonical mask)", run, c)
		}
		if c := rgbaAt(img, 13, 2); !isBlue(c) {
			t.Errorf("run %d right = %v, want blue (inverted mask)", run, c)
		}
	}
}

// The mask plane is device-space: a target with a different extent must skip
// and count instead of painting the plane at the wrong coordinates.
func TestMaskMismatchedExtentCountsSkip(t *testing.T) {
	ResetMaskSkips()
	defer ResetMaskSkips()

	const w, h = 16, 16
	m := halfMask(w, h)
	rec := NewRecorder(w, h)
	rec.SetMask(m)
	rec.SetRGBA(1, 0, 0, 1)
	rec.DrawRectangle(0, 0, w, h)
	if err := rec.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	rec.ClearMask()
	r := rec.FinishRecording()

	dc := render.NewContext(w*2, h*2)
	if err := r.Playback(NewCanvasBackend(dc)); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if got := MaskSkips(); got == 0 {
		t.Fatal("MaskSkips = 0, want the SetMask command counted as skipped")
	}
}

// A Canvas without the mask capability is also a counted skip, never an
// invisible hole. PushMaskLayer still opens a plain layer so PopLayer stays
// balanced.
func TestMaskUnsupportedTargetCountsSkipAndKeepsLayerBalance(t *testing.T) {
	ResetMaskSkips()
	defer ResetMaskSkips()

	const w, h = 16, 16
	m := render.NewMask(w, h)
	m.Fill(255)
	rec := NewRecorder(w, h)
	rec.PushMaskLayer(m)
	rec.SetRGBA(1, 0, 0, 1)
	rec.DrawRectangle(0, 0, w, h)
	if err := rec.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	rec.PopLayer()
	r := rec.FinishRecording()

	dc := render.NewContext(w, h)
	if err := r.Playback(NewCanvasBackend(canvasOnly{dc})); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if got := MaskSkips(); got == 0 {
		t.Fatal("MaskSkips = 0, want the masked layer counted as skipped")
	}
	img, ok := dc.Image().(*image.RGBA)
	if !ok {
		t.Fatalf("image type = %T", dc.Image())
	}
	if c := rgbaAt(img, 8, 8); !isRed(c) {
		t.Errorf("layer content = %v, want red (the fallback layer still draws)", c)
	}
}

// A mask layer that reaches a real MaskCanvas must modulate the layer content
// and PopLayer must composite it exactly once.
func TestMaskLayerPlaybackModulatesContent(t *testing.T) {
	const w, h = 16, 16
	m := render.NewMask(w, h)
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			m.Set(x, y, 255)
		}
	}
	rec := NewRecorder(w, h)
	rec.PushMaskLayer(m)
	rec.SetRGBA(1, 0, 0, 1)
	rec.DrawRectangle(0, 0, w, h)
	if err := rec.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	rec.PopLayer()
	r := rec.FinishRecording()

	dc := render.NewContext(w, h)
	if err := r.Playback(NewCanvasBackend(dc)); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	img, ok := dc.Image().(*image.RGBA)
	if !ok {
		t.Fatalf("image type = %T", dc.Image())
	}
	if c := rgbaAt(img, 8, 8); !isRed(c) {
		t.Errorf("inside mask = %v, want red", c)
	}
	if c := rgbaAt(img, 1, 1); c.A > 0.01 {
		t.Errorf("outside mask = %v, want transparent (masked out)", c)
	}
}
