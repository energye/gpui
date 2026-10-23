package rendering

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
)

// recordCanvas returns a recorder carrying a real UI face. Text is only
// recordable when some canvas can supply a face (the live path draws nothing
// without one, see TestRecordRenderText_NoFaceRecordsNothing), so these
// structural tests install the face the way an app's OnPaint does: the node
// keeps its own unshaped fallback route while the canvas can replay it.
func recordCanvas(t *testing.T, w, h int) (*recording.Recorder, text.Face) {
	t.Helper()
	face, _, err := TryLoadDefaultFace(16)
	if err != nil {
		t.Skipf("no UI font on this box: %v", err)
	}
	rec := recording.NewRecorder(w, h)
	rec.SetFont(face)
	return rec, face
}

// textCmd is one recorded draw-string command (the old Picture OpDrawString).
type textCmd struct {
	Text string
	X, Y float64
	RGBA render.RGBA
}

// recordedTexts returns every DrawTextCommand of a finished recording in
// paint order; it fails the test if any other drawing command kind shows up
// (the old tests asserted Kind == OpDrawString for every op).
func recordedTexts(t *testing.T, rec *recording.Recording) []textCmd {
	t.Helper()
	var out []textCmd
	for _, cmd := range rec.Commands() {
		switch c := cmd.(type) {
		case recording.DrawTextCommand:
			var rgba render.RGBA
			switch br := rec.Resources().GetBrush(c.Brush).(type) {
			case recording.SolidBrush:
				rgba = br.Color
			case *recording.SolidBrush:
				rgba = br.Color
			}
			out = append(out, textCmd{Text: c.Text, X: c.X, Y: c.Y, RGBA: rgba})
		case recording.DrawGlyphsCommand, recording.StrokeTextCommand:
			t.Fatalf("unexpected drawing command %T, want draw-string ops", cmd)
		}
	}
	return out
}

// Retained recordings must show the same lines as direct Paint frames: one
// draw-string per display line, never a single command holding raw newlines.
func TestRecordRenderText_Multiline(t *testing.T) {
	txt := NewRenderText("alpha\nbeta gamma delta epsilon zeta eta theta\nomega")
	txt.MaxWidth = 60
	rec, _ := recordCanvas(t, 60, 200)
	recordRenderText(rec, txt, 0, 0)
	built := rec.FinishRecording()
	if built.PaintOpCount() == 0 {
		t.Fatal("no commands recorded")
	}
	want := 0
	for _, ln := range txt.DisplayLines() {
		if ln != "" {
			want++
		}
	}
	cmds := recordedTexts(t, built)
	if len(cmds) != want {
		t.Fatalf("ops=%d want %d (display lines)", len(cmds), want)
	}
	prevY := -1.0
	for _, c := range cmds {
		if strings.Contains(c.Text, "\n") {
			t.Fatalf("op holds raw newline: %q", c.Text)
		}
		if c.Y <= prevY {
			t.Fatalf("Y not increasing: %v after %v", c.Y, prevY)
		}
		prevY = c.Y
	}
}

func TestRecordRenderText_YBandCullsToVisible(t *testing.T) {
	s := "hello" + strings.Repeat("\n", 30) + "TAIL"
	txt := NewRenderText(s)
	txt.FontSize = 16
	txt.MaxWidth = 864
	lay := txt.TextLayout()
	totalH := lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
	visH := 74.0
	scrollBottom := totalH - visH
	txt.SetViewportRect(0, 864, scrollBottom, visH)
	rec, _ := recordCanvas(t, 864, int(totalH)+8)
	recordRenderText(rec, txt, 0, 0)
	built := rec.FinishRecording()
	var foundTail, foundTop bool
	for _, c := range recordedTexts(t, built) {
		if c.Text == "TAIL" {
			foundTail = true
		}
		if c.Text == "hello" {
			foundTop = true
		}
	}
	if !foundTail {
		t.Fatalf("tail missing in culled record ops=%d", built.PaintOpCount())
	}
	if foundTop {
		t.Fatalf("top should be culled at bottom scroll")
	}
	if txt.ViewportBandExpiredY(scrollBottom, visH) {
		t.Fatalf("same band should not expire")
	}
	txt.SetViewportRect(0, 864, 0, visH)
	if !txt.ViewportBandExpiredY(0, visH) {
		t.Fatalf("moved band should expire")
	}
}

func TestRecordRuns_YBandCullsToVisible(t *testing.T) {
	pb := NewParagraphBuilder()
	pb.AddRun(TextRun{Text: "TOP" + strings.Repeat("\n", 30) + "TAIL", FontSize: 14, R: 1, G: 1, B: 1, A: 1})
	txt := NewRenderText("")
	txt.MaxWidth = 864
	txt.SetRuns(pb.Runs())
	lines := txt.layoutRunLines()
	if len(lines) != 31 {
		t.Fatalf("want 31 run lines got %d", len(lines))
	}
	totalH := 0.0
	for _, ln := range lines {
		totalH += ln.Height
	}
	visH := 74.0
	txt.SetViewportRect(0, 864, totalH-visH, visH)
	rec, _ := recordCanvas(t, 864, int(totalH)+8)
	recordRenderText(rec, txt, 0, 0)
	built := rec.FinishRecording()
	var foundTail, foundTop bool
	for _, c := range recordedTexts(t, built) {
		if strings.Contains(c.Text, "TAIL") {
			foundTail = true
		}
		if strings.Contains(c.Text, "TOP") {
			foundTop = true
		}
	}
	if !foundTail {
		t.Fatalf("tail missing in culled run record ops=%d", built.PaintOpCount())
	}
	if foundTop {
		t.Fatalf("top should be culled at bottom scroll")
	}
}

// Multi-run content must keep per-span text and color in retained recordings.
func TestRecordRenderText_MultiRun(t *testing.T) {
	pb := NewParagraphBuilder()
	pb.AddRun(TextRun{Text: "red ", FontSize: 14, R: 1, G: 0, B: 0, A: 1})
	pb.AddRun(TextRun{Text: "green", FontSize: 14, R: 0, G: 1, B: 0, A: 1})
	txt := NewRenderText("")
	txt.MaxWidth = 1000
	txt.SetRuns(pb.Runs())
	rec, _ := recordCanvas(t, 1000, 40)
	recordRenderText(rec, txt, 5, 7)
	built := rec.FinishRecording()
	cmds := recordedTexts(t, built)
	if len(cmds) != 2 {
		t.Fatalf("ops=%d want 2 spans", len(cmds))
	}
	if cmds[0].Text != "red " || cmds[0].RGBA.R != 1 {
		t.Fatalf("span0=%q color R=%v", cmds[0].Text, cmds[0].RGBA.R)
	}
	if cmds[1].Text != "green" || cmds[1].RGBA.G != 1 {
		t.Fatalf("span1=%q color G=%v", cmds[1].Text, cmds[1].RGBA.G)
	}
	if cmds[0].X != 5 {
		t.Fatalf("span0 X=%v want origin 5", cmds[0].X)
	}
}

// Single-line text keeps the previous single-op shape.
func TestRecordRenderText_SingleLine(t *testing.T) {
	txt := NewRenderText("hello")
	rec, _ := recordCanvas(t, 100, 40)
	recordRenderText(rec, txt, 2, 3)
	built := rec.FinishRecording()
	cmds := recordedTexts(t, built)
	if len(cmds) != 1 {
		t.Fatalf("ops=%d want 1", len(cmds))
	}
	if cmds[0].Text != "hello" || cmds[0].X != 2 {
		t.Fatalf("op=%+v", cmds[0])
	}
}

// inkPixels counts non-white pixels in [0,w)×[0,h) of a painted context.
func inkPixels(t *testing.T, dc *render.Context, w, h int) int {
	t.Helper()
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := dc.Image().At(x, y).RGBA()
			if r != 65535 || g != 65535 || b != 65535 {
				n++
			}
		}
	}
	return n
}

// paintInk paints txt into a fresh white context and reports the ink count.
func paintInk(t *testing.T, txt *RenderText) int {
	t.Helper()
	dc := render.NewContext(200, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	txt.Paint(NewPaintContext(dc, 1))
	return inkPixels(t, dc, 200, 60)
}

// TestRecordRenderText_NoFaceRecordsNothing locks the record/replay parity for
// the face-less case: text with no font is not drawable, so the recording must
// hold nothing rather than a command no backend can replay. Both sides are
// measured, with a with-face positive control proving the probe can see text
// at all.
func TestRecordRenderText_NoFaceRecordsNothing(t *testing.T) {
	face, _, err := TryLoadDefaultFace(16)
	if err != nil {
		t.Skipf("no UI font on this box: %v", err)
	}
	noFace := NewRenderText("hello")
	noFace.MaxWidth = 400
	if got := paintInk(t, noFace); got != 0 {
		t.Fatalf("live path painted %d px with no face: the recorded path may not skip what Paint draws", got)
	}
	rec := recording.NewRecorder(200, 60)
	recordRenderText(rec, noFace, 0, 0)
	if got := rec.FinishRecording().PaintOpCount(); got != 0 {
		t.Fatalf("face-less recording ops=%d want 0", got)
	}

	withFace := NewRenderText("hello")
	withFace.Face = face
	withFace.MaxWidth = 400
	if got := paintInk(t, withFace); got == 0 {
		t.Fatalf("live path painted nothing even with a face: probe is blind, cannot conclude parity")
	}
	rec2 := recording.NewRecorder(200, 60)
	recordRenderText(rec2, withFace, 0, 0)
	built := rec2.FinishRecording()
	if built.PaintOpCount() == 0 || !built.HasVisiblePaint() {
		t.Fatalf("with-face recording must be replayable: ops=%d visible=%v", built.PaintOpCount(), built.HasVisiblePaint())
	}
	dc := render.NewContext(200, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	if err := built.Playback(recording.NewCanvasBackend(dc)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	if got := inkPixels(t, dc, 200, 60); got == 0 {
		t.Fatal("replay of a with-face recording drew nothing")
	}
}
