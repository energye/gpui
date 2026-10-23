//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package recording

import (
	"image"
	"math"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// The footprint a recording reports is what a retained compositor sizes a
// texture with and damages the screen by. Two failure modes matter and both are
// tested here: under-coverage (content the recorder claims fits but replay
// paints outside — clipped pixels on screen) and vacuous coverage (a bounds so
// generous that any surface fits, which is what a real compositor pays for).

func finish(t *testing.T, rec *Recorder) *Recording {
	t.Helper()
	r := rec.FinishRecording()
	if r == nil {
		t.Fatal("FinishRecording returned nil")
	}
	return r
}

func mustBounds(t *testing.T, rec *Recorder) image.Rectangle {
	t.Helper()
	b := finish(t, rec).Bounds()
	if b.Empty() {
		t.Fatalf("expected non-empty bounds, got %v", b)
	}
	return b
}

func assertRect(t *testing.T, got, want image.Rectangle) {
	t.Helper()
	if got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
}

// --------------------------------------------------------------------------
// Geometry
// --------------------------------------------------------------------------

func TestRecordingBounds_EmptyRecordingHasNoBounds(t *testing.T) {
	rec := NewRecorder(100, 100)
	r := finish(t, rec)
	if !r.Bounds().Empty() {
		t.Errorf("Bounds = %v, want empty", r.Bounds())
	}
	if r.HasVisiblePaint() {
		t.Error("empty recording reported visible paint")
	}
}

func TestRecordingBounds_RectMapsThroughCTM(t *testing.T) {
	rec := NewRecorder(200, 200)
	rec.Translate(5, 7)
	rec.Scale(2, 2)
	rec.FillRectangle(0, 0, 10, 10)
	// CTM is translate-then-scale: (0,0)→(5,7), (10,10)→(25,27).
	assertRect(t, mustBounds(t, rec), image.Rect(5, 7, 25, 27))
}

func TestRecordingBounds_RotationUsesTransformedCorners(t *testing.T) {
	rec := NewRecorder(200, 200)
	rec.Translate(100, 100)
	rec.Rotate(math.Pi / 4)
	rec.FillRectangle(-10, -10, 20, 20)
	b := mustBounds(t, rec)
	// Corners (±10, ±10) rotate to (0, ±10√2) and (±10√2, 0), so the box spans
	// 100 ± 14.142: floor/ceil give 85..115 — wider than the unrotated 20 and
	// no wider than the circumscribed circle, which is what a lazy box bound
	// (transform the size, not the corners) would report.
	assertRect(t, b, image.Rect(85, 85, 115, 115))
}

func TestRecordingBounds_SaveRestoreIsolatesTransform(t *testing.T) {
	rec := NewRecorder(400, 400)
	rec.Save()
	rec.Translate(100, 100)
	rec.FillRectangle(0, 0, 10, 10)
	rec.Restore()
	rec.FillRectangle(0, 0, 10, 10)
	assertRect(t, mustBounds(t, rec), image.Rect(0, 0, 110, 110))
}

func TestRecordingBounds_UnbalancedRestoreKeepsAccumulatedGeometry(t *testing.T) {
	rec := NewRecorder(100, 100)
	rec.Restore() // no state saved: replay ignores it, bounds must not panic
	rec.FillRectangle(1, 2, 3, 4)
	assertRect(t, mustBounds(t, rec), image.Rect(1, 2, 4, 6))
}

func TestRecordingBounds_StrokeBandOutsetsPath(t *testing.T) {
	rec := NewRecorder(200, 200)
	rec.SetLineWidth(10)
	rec.SetLineCap(LineCapButt)
	rec.SetLineJoin(LineJoinRound)
	rec.MoveTo(20, 20)
	rec.LineTo(60, 20)
	rec.Stroke()
	// Round joins/caps: half width (5) for the band plus half again for the
	// join reach → 10 beyond the line on every side.
	assertRect(t, mustBounds(t, rec), image.Rect(10, 10, 70, 30))
}

func TestRecordingBounds_StrokeWidthZeroStrokesAtOne(t *testing.T) {
	rec := NewRecorder(200, 200)
	rec.SetLineWidth(0)
	rec.SetLineCap(LineCapButt)
	rec.SetLineJoin(LineJoinBevel)
	rec.MoveTo(20, 20)
	rec.LineTo(60, 20)
	rec.Stroke()
	// Replay strokes a width ≤0 at 1 (opStrokeWidth semantics); bevel joins
	// add half again, so the band reaches 1 beyond the line on each side.
	assertRect(t, mustBounds(t, rec), image.Rect(19, 19, 61, 21))
}

func TestRecordingBounds_DegenerateGeometryContributesNothing(t *testing.T) {
	rec := NewRecorder(100, 100)
	rec.FillRectangle(10, 10, 0, 20) // zero width fills nothing
	rec.MoveTo(5, 5)                 // a lone move-to has no area
	rec.Fill()
	r := finish(t, rec)
	if !r.Bounds().Empty() {
		t.Errorf("Bounds = %v, want empty (no coverable geometry)", r.Bounds())
	}
	if r.HasVisiblePaint() {
		t.Error("degenerate geometry reported visible paint")
	}
}

func TestRecordingBounds_NonFiniteGeometryIsSkipped(t *testing.T) {
	rec := NewRecorder(100, 100)
	rec.FillRectangle(math.NaN(), 0, 10, 10)
	rec.FillRectangle(10, 10, 20, 20)
	assertRect(t, mustBounds(t, rec), image.Rect(10, 10, 30, 30))
}

func TestRecordingBounds_ClipIsNotIntersected(t *testing.T) {
	rec := NewRecorder(200, 200)
	rec.ClipRect(0, 0, 10, 10)
	rec.FillRectangle(20, 20, 50, 50)
	// Clips shrink what reaches the screen, never the recorded footprint: a
	// consumer that cropped bounds by clip would mis-size a texture whenever
	// the clip and the content are recorded in different passes.
	assertRect(t, mustBounds(t, rec), image.Rect(20, 20, 70, 70))
}

func TestRecordingBounds_PostEffectsAreSurfaceWide(t *testing.T) {
	for name, apply := range map[string]func(*Recorder){
		"blur": func(r *Recorder) { r.ApplyBlur(4) },
		"shadow": func(r *Recorder) {
			r.ApplyDropShadow(2, 3, 5, render.RGBA{R: 0, G: 0, B: 0, A: 1})
		},
	} {
		t.Run(name, func(t *testing.T) {
			rec := NewRecorder(80, 60)
			apply(rec)
			rec.FillRectangle(10, 10, 5, 5)
			assertRect(t, mustBounds(t, rec), image.Rect(0, 0, 80, 60))
		})
	}
}

func TestRecordingBounds_ZeroRadiusBlurIsNotSurfaceWide(t *testing.T) {
	rec := NewRecorder(80, 60)
	rec.ApplyBlur(0) // Context returns early: nothing was filtered
	rec.FillRectangle(10, 10, 5, 5)
	assertRect(t, mustBounds(t, rec), image.Rect(10, 10, 15, 15))
}

// --------------------------------------------------------------------------
// Images
// --------------------------------------------------------------------------

func newTestImageBuf(t *testing.T, w, h int) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf(%d,%d): %v", w, h, err)
	}
	data := img.Data()
	for i := 0; i < len(data); i += 4 {
		data[i], data[i+1], data[i+2], data[i+3] = 0x40, 0x80, 0xc0, 0xff
	}
	img.MarkPixelsDirty()
	t.Cleanup(img.Dispose)
	return img
}

func TestRecordingBounds_ImageDestinations(t *testing.T) {
	img := newTestImageBuf(t, 8, 6)
	src := image.Rect(1, 1, 3, 4) // 2×3 sub-rect
	tests := []struct {
		name   string
		record func(*Recorder)
		want   image.Rectangle
	}{
		{
			name:   "native size",
			record: func(r *Recorder) { r.DrawImage(img, 3, 4) },
			want:   image.Rect(3, 4, 11, 10),
		},
		{
			name: "scaled destination",
			record: func(r *Recorder) {
				r.DrawImageEx(img, render.DrawImageOptions{X: 2, Y: 2, DstWidth: 20, DstHeight: 10})
			},
			want: image.Rect(2, 2, 22, 12),
		},
		{
			name: "source rect sets the size",
			record: func(r *Recorder) {
				r.DrawImageEx(img, render.DrawImageOptions{X: 0, Y: 0, SrcRect: &src})
			},
			want: image.Rect(0, 0, 2, 3),
		},
		{
			name:   "nine patch destination",
			record: func(r *Recorder) { r.DrawImageNine(img, image.Rect(2, 2, 6, 4), 5, 5, 30, 20) },
			want:   image.Rect(5, 5, 35, 25),
		},
		{
			name: "quad corners",
			record: func(r *Recorder) {
				corners := [4]render.Point{{X: 1, Y: 2}, {X: 21, Y: 0}, {X: 25, Y: 18}, {X: 3, Y: 20}}
				_ = r.DrawImageQuad(img, corners, render.QuadDrawOptions{})
			},
			want: image.Rect(1, 0, 25, 20),
		},
		{
			name:   "rounded keeps native size",
			record: func(r *Recorder) { r.DrawImageRounded(img, 4, 4, 2) },
			want:   image.Rect(4, 4, 12, 10),
		},
		{
			name:   "circular box is the circle",
			record: func(r *Recorder) { r.DrawImageCircular(img, 20, 20, 7) },
			want:   image.Rect(13, 13, 27, 27),
		},
		{
			name: "atlas batch unions every sprite",
			record: func(r *Recorder) {
				r.DrawAtlas(img, []render.AtlasSprite{
					{SrcX: 0, SrcY: 0, SrcW: 4, SrcH: 4, DstX: 0, DstY: 0, DstW: 10, DstH: 10},
					{SrcX: 0, SrcY: 0, SrcW: 2, SrcH: 2, DstX: 30, DstY: 20, DstW: 5, DstH: 5},
				})
			},
			want: image.Rect(0, 0, 35, 25),
		},
		{
			name: "rotated sprite spans its turned corners",
			record: func(r *Recorder) {
				// The pivot defaults to the destination's top-left corner.
				r.DrawAtlas(img, []render.AtlasSprite{
					{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 6, DstX: 10, DstY: 10, DstW: 10, DstH: 10, Rot: math.Pi / 4},
				})
			},
			want: image.Rect(2, 10, 18, 25),
		},
		{
			name: "depth sprites fold into the same box",
			record: func(r *Recorder) {
				r.DrawDepthSprites(img, []render.DepthSprite{
					{Sprite: render.AtlasSprite{SrcX: 0, SrcY: 0, SrcW: 6, SrcH: 4, DstX: 2, DstY: 3, DstW: 6, DstH: 4}},
				}, render.DepthDrawOptions{})
			},
			want: image.Rect(2, 3, 8, 7),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := NewRecorder(200, 200)
			tt.record(rec)
			r := finish(t, rec)
			assertRect(t, mustBounds(t, rec), tt.want)
			if !r.HasVisiblePaint() {
				t.Error("image draw reported no visible paint")
			}
		})
	}
}

func TestRecordingBounds_DisposedImageContributesNothing(t *testing.T) {
	img := newTestImageBuf(t, 8, 6)
	rec := NewRecorder(100, 100)
	rec.DrawImage(img, 0, 0)
	img.Dispose()
	r := finish(t, rec)
	if r.HasVisiblePaint() {
		t.Error("a disposed buffer cannot paint, but the recording claims it can")
	}
}

func TestRecordingImageBufs_ReturnsPooledBuffersInOrder(t *testing.T) {
	a := newTestImageBuf(t, 4, 4)
	b := newTestImageBuf(t, 2, 2)
	rec := NewRecorder(50, 50)
	rec.DrawImage(a, 0, 0)
	rec.DrawImage(b, 5, 5)
	rec.DrawImage(a, 9, 9) // re-pooled: the pool keeps one entry per Add
	r := finish(t, rec)
	got := r.ImageBufs()
	if len(got) != 3 {
		t.Fatalf("ImageBufs len = %d, want 3 (pool order, no dedup)", len(got))
	}
	if got[0] != a || got[1] != b || got[2] != a {
		t.Errorf("ImageBufs = %p %p %p, want the recorded buffers in order", got[0], got[1], got[2])
	}
}

// --------------------------------------------------------------------------
// Visibility
// --------------------------------------------------------------------------

func TestRecordingVisible_ZeroAlphaIsNotPaint(t *testing.T) {
	rec := NewRecorder(100, 100)
	rec.SetRGBA(1, 0, 0, 0) // alpha 0: replay emits nothing
	rec.FillRectangle(10, 10, 20, 20)
	r := finish(t, rec)
	if r.HasVisiblePaint() {
		t.Error("zero-alpha fill reported visible paint")
	}
	// Geometry still counts: the footprint is a property of what was drawn,
	// not of whether it was see-through.
	assertRect(t, mustBounds(t, rec), image.Rect(10, 10, 30, 30))
}

func TestRecordingVisible_CommandCountIsNotPaint(t *testing.T) {
	rec := NewRecorder(100, 100)
	rec.SetLineWidth(2)
	rec.SetDash(4, 2)
	rec.ClipRect(0, 0, 10, 10)
	rec.PushLayer(render.BlendMode(0), 0.5)
	rec.PopLayer()
	r := finish(t, rec)
	if len(r.Commands()) == 0 {
		t.Fatal("expected state commands to be recorded")
	}
	if r.HasVisiblePaint() {
		t.Error("state-only recording reported visible paint")
	}
}

// TestRecordingPaintOpCount locks the frame-budget length to the work a replay
// issues: drawing commands count, state commands do not, and a command that
// paints nothing still counts (recording cannot know a path is empty until the
// walk, and replay still issues the call).
func TestRecordingPaintOpCountCountsDrawingCommands(t *testing.T) {
	rec := NewRecorder(100, 100)
	rec.SetRGBA(1, 0, 0, 1)
	rec.Save()
	rec.ClipRect(0, 0, 90, 90)
	rec.FillRectangle(10, 10, 20, 20)
	rec.StrokeRectangle(0, 0, 30, 30)
	rec.PushLayer(render.BlendMode(0), 0.5)
	rec.PopLayer()
	rec.SetFontSize(20)
	rec.DrawString("no face set", 10, 30) // invisible, still a command to replay
	rec.Restore()
	r := finish(t, rec)
	if got := r.PaintOpCount(); got != 3 {
		t.Errorf("PaintOpCount=%d want 3 (2 rects + 1 text; state and clips count 0)", got)
	}
	if !r.HasVisiblePaint() {
		t.Error("fills reported no visible paint")
	}

	empty := finish(t, NewRecorder(10, 10))
	if empty.PaintOpCount() != 0 {
		t.Errorf("empty recording PaintOpCount=%d want 0", empty.PaintOpCount())
	}
	var nilRec *Recording
	if nilRec.PaintOpCount() != 0 {
		t.Error("nil recording must report no work, not panic")
	}
}

// TestRecordingCPUScratchFollowsImageCommands locks the offscreen-pass rule:
// only image commands can put pixels through the CPU, so only they may force
// the scratch swap (a vector/text pass paying memset+memcmp for nothing is the
// regression this guards).
func TestRecordingCPUScratchFollowsImageCommands(t *testing.T) {
	img := newTestImageBuf(t, 8, 6)
	yes := []struct {
		name   string
		record func(*Recorder)
	}{
		{"draw", func(r *Recorder) { r.DrawImage(img, 1, 1) }},
		{"scaled", func(r *Recorder) {
			r.DrawImageEx(img, render.DrawImageOptions{X: 1, Y: 1, DstWidth: 20, DstHeight: 20})
		}},
		{"nine", func(r *Recorder) { r.DrawImageNine(img, image.Rect(2, 2, 6, 4), 0, 0, 20, 20) }},
		{"quad", func(r *Recorder) {
			_ = r.DrawImageQuad(img, [4]render.Point{{X: 0, Y: 0}, {X: 8, Y: 0}, {X: 8, Y: 6}, {X: 0, Y: 6}}, render.QuadDrawOptions{})
		}},
		{"rounded", func(r *Recorder) { r.DrawImageRounded(img, 1, 1, 2) }},
		{"circular", func(r *Recorder) { r.DrawImageCircular(img, 10, 10, 5) }},
		{"atlas", func(r *Recorder) {
			r.DrawAtlas(img, []render.AtlasSprite{{SrcX: 0, SrcY: 0, SrcW: 4, SrcH: 4, DstX: 0, DstY: 0, DstW: 4, DstH: 4}})
		}},
		{"depth sprites", func(r *Recorder) {
			r.DrawDepthSprites(img, []render.DepthSprite{
				{Sprite: render.AtlasSprite{SrcX: 0, SrcY: 0, SrcW: 4, SrcH: 4, DstX: 0, DstY: 0, DstW: 4, DstH: 4}},
			}, render.DepthDrawOptions{})
		}},
		{"pattern brush fill", func(r *Recorder) {
			r.SetFillStyle(NewPatternBrush(img))
			r.FillRectangle(0, 0, 10, 10)
		}},
		{"pattern brush stroke", func(r *Recorder) {
			r.SetStrokeStyle(NewPatternBrush(img))
			r.StrokeRectangle(0, 0, 10, 10)
		}},
	}
	for _, tt := range yes {
		t.Run(tt.name, func(t *testing.T) {
			rec := NewRecorder(100, 100)
			tt.record(rec)
			if !finish(t, rec).NeedsCPUPassScratch() {
				t.Error("image content reported no CPU scratch need")
			}
		})
	}

	no := []struct {
		name   string
		record func(*Recorder)
	}{
		{"vector", func(r *Recorder) {
			r.SetRGBA(0, 0, 1, 1)
			r.FillRectangle(0, 0, 10, 10)
			r.StrokeRectangle(0, 0, 10, 10)
		}},
		{"path", func(r *Recorder) {
			r.MoveTo(1, 1)
			r.LineTo(9, 1)
			r.LineTo(5, 9)
			r.ClosePath()
			if err := r.Fill(); err != nil {
				t.Errorf("Fill: %v", err)
			}
		}},
		{"text", func(r *Recorder) {
			r.SetFontSize(20)
			r.DrawString("vector glyphs", 10, 30)
		}},
		{"gradient", func(r *Recorder) {
			b := NewLinearGradientBrush(0, 0, 10, 0)
			b.AddColorStop(0, render.RGBA{R: 1, A: 1})
			b.AddColorStop(1, render.RGBA{G: 1, A: 1})
			r.SetFillStyle(b)
			r.FillRectangle(0, 0, 10, 10)
		}},
		{"state only", func(r *Recorder) {
			r.SetLineWidth(3)
			r.ClipRect(0, 0, 10, 10)
			r.PushLayerIsolated(0.5)
			r.PopLayer()
		}},
	}
	for _, tt := range no {
		t.Run(tt.name, func(t *testing.T) {
			rec := NewRecorder(100, 100)
			tt.record(rec)
			if finish(t, rec).NeedsCPUPassScratch() {
				t.Error("non-image content forced a CPU scratch pass")
			}
		})
	}

	var nilRec *Recording
	if nilRec.NeedsCPUPassScratch() {
		t.Error("nil recording must need no scratch, not panic")
	}
}

func TestRecordingVisible_GradientStopsDecide(t *testing.T) {
	transparent := NewLinearGradientBrush(0, 0, 10, 0)
	transparent.AddColorStop(0, render.RGBA{R: 1, A: 0})
	transparent.AddColorStop(1, render.RGBA{G: 1, A: 0})
	rec := NewRecorder(100, 100)
	rec.SetFillStyle(transparent)
	rec.FillRectangle(0, 0, 50, 50)
	if finish(t, rec).HasVisiblePaint() {
		t.Error("gradient with no opaque stop reported visible paint")
	}

	spotted := NewLinearGradientBrush(0, 0, 10, 0)
	spotted.AddColorStop(0, render.RGBA{R: 1, A: 0})
	spotted.AddColorStop(1, render.RGBA{G: 1, A: 0.2})
	rec = NewRecorder(100, 100)
	rec.SetFillStyle(spotted)
	rec.FillRectangle(0, 0, 50, 50)
	if !finish(t, rec).HasVisiblePaint() {
		t.Error("gradient with a non-zero stop was reported invisible")
	}
}

func TestRecordingVisible_TextNeedsAFace(t *testing.T) {
	rec := NewRecorder(100, 100)
	rec.SetFontSize(20)
	rec.DrawString("no face set", 10, 30) // playback draws nothing without a face
	r := finish(t, rec)
	if r.HasVisiblePaint() {
		t.Error("face-less text reported visible paint")
	}
	if !r.Bounds().Empty() {
		t.Errorf("face-less text bounds = %v, want empty", r.Bounds())
	}
}

// --------------------------------------------------------------------------
// Text (real face)
// --------------------------------------------------------------------------

func testFontPath() string {
	for _, path := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
		"/Library/Fonts/Arial.ttf",
		"C:\\Windows\\Fonts\\arial.ttf",
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func testFace(t *testing.T, size float64) text.Face {
	t.Helper()
	path := testFontPath()
	if path == "" {
		t.Skipf("no system TrueType font available — text footprint cannot be measured")
	}
	source, err := text.NewFontSourceFromFile(path)
	if err != nil {
		t.Skipf("font %s unusable: %v", path, err)
	}
	t.Cleanup(func() { _ = source.Close() })
	return source.Face(size)
}

func TestRecordingBounds_TextFootprintMatchesPaintedInk(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	face := testFace(t, 24)

	const (
		line = "Hjp gQ"
		x, y = 20.0, 60.0
	)
	rec := NewRecorder(260, 120)
	rec.SetFont(face)
	rec.SetRGB(0, 0, 0)
	rec.DrawString(line, x, y)
	r := finish(t, rec)
	if !r.HasVisiblePaint() {
		t.Fatal("text recording reported no visible paint")
	}
	b := r.Bounds()
	if b.Empty() {
		t.Fatal("face-backed text produced no bounds")
	}

	// The horizontal extent is the pen's own: the live rasterizer advances by
	// the face's metric width, so the footprint ends there — at most a glyph's
	// right-side bearing further, never anywhere near the canvas edge.
	right := x + face.Advance(line)
	if float64(b.Max.X) < math.Ceil(right) || b.Max.X > int(math.Ceil(right))+8 {
		t.Errorf("bounds right edge = %d, want the advance %.1f plus bearing slack", b.Max.X, right)
	}
	if b.Dx() >= 260 || b.Dy() >= 120 {
		t.Errorf("bounds %v cover the whole canvas for one line of text", b)
	}

	// Every painted pixel must lie inside the footprint: an under-covering bounds
	// is what clips descenders and accents at the edge of the texture it sizes.
	ink := paintedExtent(t, r, 260, 120)
	if !b.Eq(b.Union(ink)) {
		t.Fatalf("painted ink %v lies outside the reported bounds %v", ink, b)
	}
}

// paintedExtent replays rec onto a transparent CPU context and returns the box
// of pixels that actually received paint.
func paintedExtent(t *testing.T, r *Recording, w, h int) image.Rectangle {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.Clear()
	if err := r.Playback(NewCanvasBackend(dc)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	img := dc.Image()
	bounds := img.Bounds()
	out := image.Rectangle{}
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if painted(img, x, y) {
				if !found {
					out = image.Rect(x, y, x+1, y+1)
					found = true
				} else {
					out = out.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
	}
	if !found {
		t.Fatal("replay painted nothing at all")
	}
	return out
}

func painted(img image.Image, x, y int) bool {
	_, _, _, a := img.At(x, y).RGBA()
	return a != 0
}
