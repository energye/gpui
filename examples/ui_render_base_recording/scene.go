package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	recW, recH   = 398.0, 194.0
	recImgX      = 16.0
	recImgY      = 32.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colTitle  = [3]float64{0.80, 0.86, 0.94}
	colBorder = [3]float64{0.32, 0.36, 0.44}
	recBG     = [3]float64{0.055, 0.065, 0.09}
)

type recordingCard struct {
	rec  *recording.Recording
	img  *render.ImageBuf
	rgba *image.RGBA
}

type recordingLayout struct {
	icon     image.Image
	iconPath string
	fontPath string
	font     text.Face
	cards    [4]recordingCard
	contract *recording.Recording
}

var rlay recordingLayout

func resolveRecordingAsset(name string) (string, error) {
	candidates := []string{
		filepath.Join("examples", "ui_render_base_recording", "testdata", name),
		filepath.Join("testdata", name),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found (looked in %v)", name, candidates)
}

func loadRecordingIcon() (image.Image, string, error) {
	path, err := resolveRecordingAsset("recording_icon.png")
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode %s: %w", path, err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		return nil, "", fmt.Errorf("%s: %dx%d, want 64x64", path, img.Bounds().Dx(), img.Bounds().Dy())
	}
	return img, path, nil
}

func buildRecordingLayout() error {
	icon, iconPath, err := loadRecordingIcon()
	if err != nil {
		return err
	}
	rlay.icon = icon
	rlay.iconPath = iconPath

	fontPath, err := resolveRecordingAsset("SourceSans3-Regular.otf")
	if err != nil {
		return err
	}
	src, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", fontPath, err)
	}
	rlay.fontPath = fontPath
	rlay.font = src.Face(30)
	if rlay.font == nil {
		return fmt.Errorf("%s: Face(30) returned nil", fontPath)
	}

	rlay.cards[0].rec = buildRecA()
	rlay.cards[1].rec = buildRecB()
	rlay.cards[2].rec = buildRecC()
	rlay.cards[3].rec = buildRecD()
	for i := range rlay.cards {
		card := &rlay.cards[i]
		img, rgba, err := playbackRecording(card.rec)
		if err != nil {
			return fmt.Errorf("card %c playback: %w", 'A'+i, err)
		}
		card.img = img
		card.rgba = rgba
	}

	rlay.contract = buildRecordingContract()
	return nil
}

func cardBG(rec *recording.Recorder) {
	rec.SetFillRGBA(recBG[0], recBG[1], recBG[2], 1)
	rec.FillRectangle(0, 0, recW, recH)
}

func buildRecA() *recording.Recording {
	rec := recording.NewRecorder(int(recW), int(recH))
	cardBG(rec)

	grad := recording.NewLinearGradientBrush(8, 8, 188, 8).
		AddColorStop(0, render.RGBA{R: 0.92, G: 0.12, B: 0.12, A: 1}).
		AddColorStop(1, render.RGBA{R: 0.12, G: 0.24, B: 0.92, A: 1})
	rec.SetFillStyle(grad)
	rec.FillRectangle(8, 8, 180, 64)

	rec.SetFillRGBA(0.18, 0.85, 0.36, 1)
	rec.DrawCircle(70, 122, 36)
	rec.Fill()

	rec.SetFillRGBA(1, 0.82, 0.18, 1)
	rec.FillRectangle(250, 20, 120, 50)

	rec.SetStrokeRGBA(1, 0.55, 0.12, 1)
	rec.SetLineWidth(5)
	rec.DrawRoundedRectangle(190, 100, 90, 64, 12)
	rec.Stroke()

	rec.SetStrokeRGBA(0.85, 0.92, 1, 1)
	rec.SetLineWidth(3)
	rec.SetDash(12, 7)
	rec.MoveTo(20, 176)
	rec.LineTo(180, 176)
	rec.Stroke()
	rec.ClearDash()

	rec.Push()
	rec.Translate(300, 140)
	rec.Rotate(0.35)
	rec.SetFillRGBA(0.72, 0.35, 1, 1)
	rec.DrawRectangle(-28, -28, 56, 56)
	rec.Fill()
	rec.Pop()

	return rec.FinishRecording()
}

func buildRecB() *recording.Recording {
	rec := recording.NewRecorder(int(recW), int(recH))
	cardBG(rec)

	rec.SetFillRGBA(0.14, 0.18, 0.28, 1)
	rec.FillRectangle(20, 20, 160, 120)

	rec.DrawRectangle(30, 30, 150, 110)
	rec.Clip()
	rec.SetFillRGBA(0.92, 0.22, 0.20, 1)
	rec.FillRectangle(0, 0, recW, recH)
	rec.SetFillRGBA(0.15, 0.50, 1, 1)
	rec.FillRectangle(0, 100, recW, 40)
	rec.ResetClip()

	rec.SetFillRGBA(0.18, 0.86, 0.42, 1)
	rec.DrawCircle(280, 105, 46)
	rec.Fill()

	rec.Push()
	rec.ClipRoundRect(210, 20, 100, 70, 18)
	rec.SetFillRGBA(1, 0.78, 0.20, 1)
	rec.FillRectangle(200, 10, 150, 100)
	rec.Pop()

	return rec.FinishRecording()
}

func buildRecC() *recording.Recording {
	rec := recording.NewRecorder(int(recW), int(recH))
	cardBG(rec)

	rec.SetFont(rlay.font)
	rec.SetFillRGBA(0.95, 0.97, 1, 1)
	rec.DrawString("record", 20, 48)
	rec.DrawString("playback", 20, 86)
	rec.DrawString("raster", 20, 124)
	rec.DrawImageScaled(rlay.icon, 220, 30, 150, 120)

	return rec.FinishRecording()
}

func buildRecD() *recording.Recording {
	rec := recording.NewRecorder(int(recW), int(recH))
	cardBG(rec)

	rec.SetLineCap(recording.LineCapRound)
	rec.SetLineJoin(recording.LineJoinBevel)
	rec.SetMiterLimit(3)
	rec.SetFillRule(recording.FillRuleEvenOdd)

	rec.SetFillRGBA(0.92, 0.22, 0.20, 1)
	rec.FillRectangle(20, 40, 60, 100)
	rec.SetFillRGBA(0.18, 0.86, 0.42, 1)
	rec.FillRectangle(100, 40, 60, 100)
	rec.SetFillRGBA(0.22, 0.44, 0.95, 1)
	rec.FillRectangle(180, 40, 60, 100)

	rec.Push()
	rec.Translate(300, 90)
	rec.Rotate(0.45)
	rec.SetFillRGBA(0.72, 0.35, 1, 1)
	rec.DrawCircle(0, 0, 30)
	rec.Fill()
	rec.Pop()

	rec.SetStrokeRGBA(1, 0.62, 0.16, 1)
	rec.SetLineWidth(6)
	rec.StrokeRectangle(20, 155, 340, 20)

	return rec.FinishRecording()
}

// contractRecording exercises command families that the four visual cards do
// not need to show, so the playback switch is covered end to end.
func buildRecordingContract() *recording.Recording {
	rec := recording.NewRecorder(64, 64)
	rec.SetAntiAlias(true)
	rec.SetFillStyle(recording.NewSolidBrush(render.RGBA{R: 0.8, A: 1}))
	rec.SetStrokeStyle(recording.NewSolidBrush(render.RGBA{G: 0.8, A: 1}))
	rec.SetLineWidth(2)
	rec.SetLineCap(recording.LineCapSquare)
	rec.SetLineJoin(recording.LineJoinRound)
	rec.SetMiterLimit(5)
	rec.SetDash(4, 2)
	rec.SetFillRule(recording.FillRuleNonZero)
	rec.Push()
	rec.Translate(3, 4)
	rec.Rotate(0.2)
	rec.DrawRectangle(2, 2, 24, 24)
	rec.FillPreserve()
	rec.StrokePreserve()
	rec.DrawRectangle(8, 8, 32, 32)
	rec.Clip()
	rec.ResetClip()
	rec.ClipRoundRect(4, 4, 24, 24, 5)
	rec.FillRectangle(0, 0, 20, 10)
	rec.StrokeRectangle(20, 20, 20, 20)
	rec.DrawImageScaled(rlay.icon, 0, 0, 8, 8)
	rec.SetFont(rlay.font)
	rec.DrawString("R", 2, 20)
	rec.StrokeString("S", 2, 40)
	rec.Pop()
	return rec.FinishRecording()
}

func playbackRecording(rec *recording.Recording) (*render.ImageBuf, *image.RGBA, error) {
	backend, err := recording.NewBackend("raster")
	if err != nil {
		return nil, nil, err
	}
	if err := rec.Playback(backend); err != nil {
		return nil, nil, err
	}
	provider, ok := backend.(interface{ Image() image.Image })
	if !ok {
		return nil, nil, fmt.Errorf("raster backend has no Image()")
	}
	img := provider.Image()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		return nil, nil, fmt.Errorf("raster image type %T, want *image.RGBA", img)
	}
	return render.ImageBufFromImage(img), rgba, nil
}

func recRGBA(img *image.RGBA, x, y int) render.RGBA {
	c := img.RGBAAt(x, y)
	return render.RGBA{R: float64(c.R) / 255, G: float64(c.G) / 255, B: float64(c.B) / 255, A: float64(c.A) / 255}
}

func recNear(img *image.RGBA, x, y int, want [3]float64, tol float64) bool {
	c := recRGBA(img, x, y)
	return absF(c.R-want[0]) <= tol && absF(c.G-want[1]) <= tol && absF(c.B-want[2]) <= tol
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func recInk(img *image.RGBA, x0, y0, x1, y1 int) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := img.RGBAAt(x, y)
			if c.R > 150 && c.G > 150 && c.B > 150 && c.A > 150 {
				n++
			}
		}
	}
	return n
}

func hashRGBA(img *image.RGBA) string {
	return sha16Bytes(img.Pix)
}

func sha16Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:8])
}

// ---------------------------------------------------------------------------
// Logic probes
// ---------------------------------------------------------------------------

var recordingProbeKeys = []string{
	"raster_registered",
	"unknown_backend_error",
	"recording_dimensions",
	"command_coverage",
	"font_ref_recorded",
	"stroke_text_face_recorded",
	"resource_pool_counts",
	"replay_immutable",
	"gradient_pixels",
	"fillrect_pixel",
	"circle_pixel",
	"stroked_rect_pixel",
	"clip_pixels",
	"reset_clip_pixel",
	"clip_roundrect_pixel",
	"save_restore_pixel",
	"image_pixels",
	"text_ink",
	"spy_backend_dispatch",
}

func runRecordingLogic() (ran, failed []string) {
	add := func(key string, ok bool) {
		ran = append(ran, key)
		if !ok {
			failed = append(failed, key)
		}
	}

	backends := recording.Backends()
	hasRaster := false
	for _, b := range backends {
		if b == "raster" {
			hasRaster = true
		}
	}
	add("raster_registered", recording.IsRegistered("raster") && hasRaster && recording.Count() >= 1)

	_, unknownErr := recording.NewBackend("recording_no_such_backend")
	add("unknown_backend_error", unknownErr != nil)

	okDims := true
	for _, card := range rlay.cards {
		if card.rec.Width() != int(recW) || card.rec.Height() != int(recH) {
			okDims = false
		}
	}
	add("recording_dimensions", okDims)

	seen := map[string]bool{}
	countCommands := func(r *recording.Recording) {
		for _, c := range r.Commands() {
			seen[c.Type().String()] = true
		}
	}
	for _, card := range rlay.cards {
		countCommands(card.rec)
	}
	countCommands(rlay.contract)
	wantCommands := []string{
		"Save", "Restore", "SetTransform", "SetClip", "ClearClip", "ClipRoundRect",
		"FillPath", "StrokePath", "FillRect", "StrokeRect", "DrawImage",
		"DrawText", "StrokeText", "SetFillStyle", "SetStrokeStyle", "SetLineWidth",
		"SetLineCap", "SetLineJoin", "SetMiterLimit", "SetDash", "SetFillRule", "SetAntiAlias",
	}
	missing := make([]string, 0)
	for _, k := range wantCommands {
		if !seen[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	add("command_coverage", len(missing) == 0)

	fontOK := false
	strokeFontOK := false
	for _, c := range rlay.contract.Commands() {
		switch cmd := c.(type) {
		case recording.DrawTextCommand:
			fontOK = cmd.Font.IsValid() && rlay.contract.Resources().GetFont(cmd.Font) != nil
		case recording.StrokeTextCommand:
			strokeFontOK = cmd.Font.IsValid() && rlay.contract.Resources().GetFont(cmd.Font) != nil
		}
	}
	add("font_ref_recorded", fontOK)
	add("stroke_text_face_recorded", strokeFontOK)

	pool := rlay.contract.Resources()
	add("resource_pool_counts", pool.PathCount() > 0 && pool.BrushCount() > 0 &&
		pool.ImageCount() > 0 && pool.FontCount() > 0)

	before := len(rlay.cards[0].rec.Commands())
	_, first, err1 := playbackRecording(rlay.cards[0].rec)
	_, second, err2 := playbackRecording(rlay.cards[0].rec)
	after := len(rlay.cards[0].rec.Commands())
	add("replay_immutable", err1 == nil && err2 == nil && before == after &&
		hashRGBA(first) == hashRGBA(second))

	a := rlay.cards[0].rgba
	add("gradient_pixels", recNear(a, 20, 30, [3]float64{0.80, 0.11, 0.20}, 0.20) &&
		recNear(a, 170, 30, [3]float64{0.20, 0.12, 0.80}, 0.20))
	add("fillrect_pixel", recNear(a, 310, 45, [3]float64{1, 0.82, 0.18}, 0.04))
	add("circle_pixel", recNear(a, 70, 122, [3]float64{0.18, 0.85, 0.36}, 0.04))

	d := rlay.cards[3].rgba
	add("stroked_rect_pixel", recNear(d, 20, 165, [3]float64{1, 0.62, 0.16}, 0.06))

	b := rlay.cards[1].rgba
	add("clip_pixels", recNear(b, 60, 60, [3]float64{0.92, 0.22, 0.20}, 0.04) &&
		recNear(b, 200, 60, recBG, 0.02))
	add("reset_clip_pixel", recNear(b, 280, 105, [3]float64{0.18, 0.86, 0.42}, 0.04))
	add("clip_roundrect_pixel", recNear(b, 260, 55, [3]float64{1, 0.78, 0.20}, 0.04) &&
		recNear(b, 330, 55, recBG, 0.02))
	add("save_restore_pixel", recNear(b, 330, 55, recBG, 0.02))

	c := rlay.cards[2].rgba
	add("image_pixels",
		recNear(c, 235, 45, [3]float64{0.89, 0.23, 0.23}, 0.06) &&
			recNear(c, 355, 45, [3]float64{0.15, 0.77, 0.44}, 0.06) &&
			recNear(c, 235, 135, [3]float64{0.23, 0.40, 0.89}, 0.06) &&
			recNear(c, 355, 135, [3]float64{0.95, 0.84, 0.32}, 0.06))
	add("text_ink", recInk(c, 20, 16, 210, 140) > 300)

	spy := &recordingSpyBackend{}
	err := rlay.contract.Playback(spy)
	add("spy_backend_dispatch", err == nil && spy.begin && spy.end &&
		spy.calls["Save"] > 0 && spy.calls["Restore"] > 0 &&
		spy.calls["SetTransform"] > 0 && spy.calls["SetClip"] > 0 &&
		spy.calls["ClearClip"] > 0 && spy.calls["FillPath"] > 0 &&
		spy.calls["StrokePath"] > 0 && spy.calls["FillRect"] > 0 &&
		spy.calls["DrawImage"] > 0 && spy.calls["DrawText"] > 0 &&
		spy.textFace != nil)

	return ran, failed
}

type recordingSpyBackend struct {
	begin    bool
	end      bool
	textFace text.Face
	calls    map[string]int
}

func (s *recordingSpyBackend) hit(name string) {
	if s.calls == nil {
		s.calls = make(map[string]int)
	}
	s.calls[name]++
}

func (s *recordingSpyBackend) Begin(_, _ int) error { s.begin = true; return nil }
func (s *recordingSpyBackend) End() error           { s.end = true; return nil }
func (s *recordingSpyBackend) Save()                { s.hit("Save") }
func (s *recordingSpyBackend) Restore()             { s.hit("Restore") }
func (s *recordingSpyBackend) SetTransform(_ recording.Matrix) {
	s.hit("SetTransform")
}
func (s *recordingSpyBackend) SetClip(_ *render.Path, _ recording.FillRule) {
	s.hit("SetClip")
}
func (s *recordingSpyBackend) ClearClip() { s.hit("ClearClip") }
func (s *recordingSpyBackend) FillPath(_ *render.Path, _ recording.Brush, _ recording.FillRule) {
	s.hit("FillPath")
}
func (s *recordingSpyBackend) StrokePath(_ *render.Path, _ recording.Brush, _ recording.Stroke) {
	s.hit("StrokePath")
}
func (s *recordingSpyBackend) FillRect(_ recording.Rect, _ recording.Brush) {
	s.hit("FillRect")
}
func (s *recordingSpyBackend) DrawImage(_ image.Image, _, _ recording.Rect, _ recording.ImageOptions) {
	s.hit("DrawImage")
}
func (s *recordingSpyBackend) DrawText(_ string, _, _ float64, face text.Face, _ recording.Brush) {
	s.hit("DrawText")
	s.textFace = face
}

// ---------------------------------------------------------------------------
// Snapshot probes and scene
// ---------------------------------------------------------------------------

type pointProbe struct {
	key, desc string
	card      int
	rect      rect
	want      [3]float64
	tol       float64
}

type inkProbe struct {
	key, desc string
	card      int
	rect      rect
	min       int
}

func recordingPointProbes() []pointProbe {
	at := func(card int, x, y float64) rect {
		return rect{x: recImgX + x, y: recImgY + y, w: 1, h: 1}
	}
	return []pointProbe{
		{"A_gradient_left", "gradient red end", 0, at(0, 20, 30), [3]float64{0.80, 0.11, 0.20}, 0.20},
		{"A_gradient_right", "gradient blue end", 0, at(0, 170, 30), [3]float64{0.20, 0.12, 0.80}, 0.20},
		{"A_circle", "filled circle", 0, at(0, 70, 122), [3]float64{0.18, 0.85, 0.36}, 0.05},
		{"A_fill_rect", "FillRectangle", 0, at(0, 310, 45), [3]float64{1, 0.82, 0.18}, 0.05},
		{"A_transform", "Push/Translate/Rotate", 0, at(0, 300, 140), [3]float64{0.72, 0.35, 1}, 0.05},
		{"B_clip_inside", "inside Clip()", 1, at(1, 60, 60), [3]float64{0.92, 0.22, 0.20}, 0.05},
		{"B_clip_outside", "outside Clip()", 1, at(1, 200, 60), recBG, 0.02},
		{"B_reset_clip", "after ResetClip", 1, at(1, 280, 105), [3]float64{0.18, 0.86, 0.42}, 0.05},
		{"B_round_clip", "ClipRoundRect", 1, at(1, 260, 55), [3]float64{1, 0.78, 0.20}, 0.05},
		{"C_image_tl", "image red quadrant", 2, at(2, 235, 45), [3]float64{0.89, 0.23, 0.23}, 0.06},
		{"C_image_tr", "image green quadrant", 2, at(2, 355, 45), [3]float64{0.15, 0.77, 0.44}, 0.06},
		{"C_image_bl", "image blue quadrant", 2, at(2, 235, 135), [3]float64{0.23, 0.40, 0.89}, 0.06},
		{"C_image_br", "image yellow quadrant", 2, at(2, 355, 135), [3]float64{0.95, 0.84, 0.32}, 0.06},
		{"D_bar_red", "FillRect red", 3, at(3, 40, 80), [3]float64{0.92, 0.22, 0.20}, 0.05},
		{"D_bar_green", "FillRect green", 3, at(3, 120, 80), [3]float64{0.18, 0.86, 0.42}, 0.05},
		{"D_bar_blue", "FillRect blue", 3, at(3, 200, 80), [3]float64{0.22, 0.44, 0.95}, 0.05},
		{"D_stroke_rect", "StrokeRectangle", 3, at(3, 20, 165), [3]float64{1, 0.62, 0.16}, 0.06},
	}
}

func recordingInkProbes() []inkProbe {
	return []inkProbe{
		{"C_text", "recorded DrawString ink", 2, rect{x: recImgX + 20, y: recImgY + 16, w: 190, h: 124}, 250},
	}
}

func buildRecordingScene(shell *wrkit.ShellChrome) {
	cardWithBoundary := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	cardWithBoundary(0, "A  fill · gradient · stroke · transform", paintRecA)
	cardWithBoundary(1, "B  Clip / ResetClip · Save / Restore", paintRecB)
	cardWithBoundary(2, "C  DrawString + DrawImage", paintRecC)
	cardWithBoundary(3, "D  replay · resources · StrokeRect", paintRecD)
}

func paintRecA(pc *rendering.PaintContext, _ rendering.Size) { paintRecCard(pc, 0) }
func paintRecB(pc *rendering.PaintContext, _ rendering.Size) { paintRecCard(pc, 1) }
func paintRecC(pc *rendering.PaintContext, _ rendering.Size) { paintRecCard(pc, 2) }
func paintRecD(pc *rendering.PaintContext, _ rendering.Size) { paintRecCard(pc, 3) }

func paintRecCard(pc *rendering.PaintContext, i int) {
	if pc == nil || pc.DC == nil || rlay.cards[i].img == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, cardW, cardH)
	_ = dc.Fill()

	dc.DrawImageEx(rlay.cards[i].img, render.DrawImageOptions{
		X: ox + recImgX, Y: oy + recImgY,
		DstWidth: recW, DstHeight: recH,
		Interpolation: render.InterpNearest,
		Opacity:       1,
	})

	dc.SetRGBA(colBorder[0], colBorder[1], colBorder[2], 1)
	dc.SetLineWidth(1)
	dc.DrawRectangle(ox+recImgX, oy+recImgY, recW, recH)
	_ = dc.Stroke()
}
