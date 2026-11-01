package recording

import (
	"bytes"
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

func recordingRRectTestRect(x, y, w, h float64) render.Rect {
	return render.NewRect(render.Pt(x, y), render.Pt(x+w, y+h))
}

func TestRecorderDrawRRect(t *testing.T) {
	rec := NewRecorder(200, 200)
	rr := render.NewRRectRadii(recordingRRectTestRect(20, 20, 120, 120), [4]render.Point{
		{X: 30, Y: 30}, {X: 10, Y: 10}, {X: 20, Y: 20}, {X: 5, Y: 5},
	})
	rec.DrawRRect(rr)

	if got := rec.currentPath.NumVerbs(); got != 10 {
		t.Fatalf("DrawRRect path verbs = %d, want 10", got)
	}
	coords := rec.currentPath.Coords()
	if coords[0] != 50 || coords[1] != 20 {
		t.Fatalf("DrawRRect starts at (%v,%v), want (50,20)", coords[0], coords[1])
	}

	rec.SetRGB(1, 0, 0)
	if err := rec.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	recording := rec.FinishRecording()
	hasFill := false
	for _, cmd := range recording.Commands() {
		if cmd.Type() == CmdFillPath {
			hasFill = true
			break
		}
	}
	if !hasFill {
		t.Fatal("DrawRRect + Fill should record a FillPath command")
	}
}

func TestRecorderClipRRectDispatch(t *testing.T) {
	rect := recordingRRectTestRect(20, 20, 120, 120)
	uniform := render.NewRRectXY(rect, 12, 12)
	complexRRect := render.NewRRectRadii(rect, [4]render.Point{
		{X: 80, Y: 8}, {X: 20, Y: 8}, {X: 80, Y: 8}, {X: 20, Y: 8},
	})

	t.Run("uniform records ClipRoundRect", func(t *testing.T) {
		rec := NewRecorder(200, 200)
		rec.ClipRRect(uniform)
		if len(rec.commands) != 1 {
			t.Fatalf("commands = %d, want 1", len(rec.commands))
		}
		cmd, ok := rec.commands[0].(ClipRoundRectCommand)
		if !ok {
			t.Fatalf("command type = %T, want ClipRoundRectCommand", rec.commands[0])
		}
		if cmd.Radius != 12 || cmd.X != 20 || cmd.Y != 20 || cmd.W != 120 || cmd.H != 120 {
			t.Fatalf("ClipRoundRectCommand = %+v, want (20,20,120,120,r=12)", cmd)
		}
	})

	t.Run("complex records SetClip", func(t *testing.T) {
		rec := NewRecorder(200, 200)
		rec.ClipRRect(complexRRect)
		if len(rec.commands) != 1 {
			t.Fatalf("commands = %d, want 1", len(rec.commands))
		}
		cmd, ok := rec.commands[0].(SetClipCommand)
		if !ok {
			t.Fatalf("command type = %T, want SetClipCommand", rec.commands[0])
		}
		path := rec.resources.GetPath(cmd.Path)
		if path == nil || path.NumVerbs() != 10 {
			t.Fatalf("recorded path = %v, want 10 verbs", path)
		}
	})

	t.Run("rotated uniform records SetClip", func(t *testing.T) {
		rec := NewRecorder(200, 200)
		rec.Rotate(0.3)
		rec.ClipRRect(uniform)
		if _, ok := rec.commands[len(rec.commands)-1].(SetClipCommand); !ok {
			t.Fatalf("last command = %T, want SetClipCommand", rec.commands[len(rec.commands)-1])
		}
	})
}

func TestRecorderClipRRectKeepsCurrentPath(t *testing.T) {
	rec := NewRecorder(200, 200)
	rec.MoveTo(10, 10)
	rec.LineTo(50, 10)
	before := rec.currentPath.NumVerbs()

	rec.ClipRRect(render.NewRRectRadii(recordingRRectTestRect(20, 20, 120, 120), [4]render.Point{
		{X: 30, Y: 30}, {X: 10, Y: 10}, {X: 20, Y: 20}, {X: 5, Y: 5},
	}))

	if got := rec.currentPath.NumVerbs(); got != before {
		t.Fatalf("current path verbs = %d after ClipRRect, want %d", got, before)
	}
}

func TestRecorderClipRRectPlaybackMatchesLive(t *testing.T) {
	rect := recordingRRectTestRect(20, 20, 80, 80)
	cases := []struct {
		name string
		rr   render.RRect
	}{
		{name: "uniform", rr: render.NewRRectXY(rect, 12, 12)},
		{name: "complex", rr: render.NewRRectRadii(rect, [4]render.Point{
			{X: 30, Y: 30}, {X: 10, Y: 10}, {X: 20, Y: 20}, {X: 5, Y: 5},
		})},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			live := render.NewContext(120, 120)
			defer func() { _ = live.Close() }()
			live.ClearWithColor(render.White)
			live.ClipRRect(tt.rr)
			live.SetRGB(1, 0, 0)
			live.DrawRectangle(0, 0, 120, 120)
			if err := live.Fill(); err != nil {
				t.Fatalf("live Fill: %v", err)
			}
			if err := live.FlushGPU(); err != nil {
				t.Fatalf("live FlushGPU: %v", err)
			}

			rec := NewRecorder(120, 120)
			rec.ClipRRect(tt.rr)
			rec.SetRGB(1, 0, 0)
			rec.DrawRectangle(0, 0, 120, 120)
			if err := rec.Fill(); err != nil {
				t.Fatalf("record Fill: %v", err)
			}

			replayed := render.NewContext(120, 120)
			defer func() { _ = replayed.Close() }()
			replayed.ClearWithColor(render.White)
			if err := rec.FinishRecording().Playback(NewCanvasBackend(replayed)); err != nil {
				t.Fatalf("Playback: %v", err)
			}
			if err := replayed.FlushGPU(); err != nil {
				t.Fatalf("replayed FlushGPU: %v", err)
			}

			liveImg := live.Image().(*image.RGBA)
			replayImg := replayed.Image().(*image.RGBA)
			if !bytes.Equal(liveImg.Pix, replayImg.Pix) {
				t.Fatal("recording playback must match live ClipRRect pixels")
			}
		})
	}
}
