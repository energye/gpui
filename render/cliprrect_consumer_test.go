//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/render/text"
)

// TestClipRRectGPUTextureAndText pins the GPU consumers of the C-group
// "rrect image/text clip" evidence: the textured_quad blit and the glyph path
// under ClipRoundRect.
func TestClipRRectGPUTextureAndText(t *testing.T) {
	requireNativeGPU(t)

	const sw, sh = 80, 40
	src := render.NewContext(sw, sh)
	src.ClearWithColor(render.RGBA{R: 0.2, G: 0.4, B: 0.6, A: 1})
	src.ApplyInvert()
	view, vw, vh, ok := src.GPUFilterTexture()
	if !ok || view.IsNil() || vw != sw || vh != sh {
		src.Close()
		t.Fatalf("GPUFilterTexture: ok=%v nil=%v size=%dx%d", ok, view.IsNil(), vw, vh)
	}
	defer src.Close()

	const w, h = 200, 120
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.Push()
	dc.ClipRoundRect(40, 20, 120, 80, 24)
	dc.DrawGPUTexture(view, 20, 0, 160, 120)
	dc.Pop()
	rgba := backdropPresentRGBA(t, dc, w, h)
	pix := func(x, y int) [4]int { return backdropPixel(rgba, w, x, y) }
	t.Logf("tex center=%v corner=%v outside=%v", pix(100, 60), pix(41, 21), pix(10, 10))
	if got := pix(41, 21); !backdropNear(got, [4]int{255, 255, 255, 255}, 4) {
		t.Errorf("texture corner not clipped: %v", got)
	}
	if got := pix(100, 60); !backdropNear(got, [4]int{204, 153, 102, 255}, 8) {
		t.Errorf("texture center: %v want ~[204 153 102 255]", got)
	}

	const tw, th = 300, 160
	tdc := render.NewContext(tw, th)
	defer tdc.Close()
	tdc.BeginFrame()
	tdc.SetRGBA(1, 1, 1, 1)
	tdc.DrawRectangle(0, 0, tw, th)
	_ = tdc.Fill()
	face, _, err := text.LoadDefaultFace(48)
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	tdc.SetFont(face)
	tdc.SetRGBA(0, 0, 0, 1)
	tdc.Push()
	tdc.ClipRoundRect(80, 30, 140, 100, 30)
	for i := 0; i < 4; i++ {
		tdc.DrawString("MMMM MMMM MMMM", 30, float64(70+i*46))
	}
	tdc.Pop()
	trgba := backdropPresentRGBA(t, tdc, tw, th)
	tpix := func(x, y int) [4]int { return backdropPixel(trgba, tw, x, y) }
	t.Logf("text center=%v corner=%v outside=%v", tpix(150, 80), tpix(81, 31), tpix(20, 80))
	if got := tpix(81, 31); !backdropNear(got, [4]int{255, 255, 255, 255}, 4) {
		t.Errorf("text corner not clipped: %v", got)
	}
	insideDark, outsideDark := 0, 0
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			p := tpix(x, y)
			dark := p[0] < 100 && p[1] < 100 && p[2] < 100
			inClip := x >= 80 && x < 220 && y >= 30 && y < 130
			if !inClip && dark {
				outsideDark++
			}
			if inClip && dark {
				insideDark++
			}
		}
	}
	t.Logf("text ink inside=%d outside=%d", insideDark, outsideDark)
	if insideDark == 0 {
		t.Error("text did not render inside clip")
	}
	if outsideDark != 0 {
		t.Errorf("text ink outside clip: %d px", outsideDark)
	}
}

// TestClipRoundRectCPUBitmapText pins the CPU bitmap text path. It used to
// call text.Draw directly on c.pixmap and ignored the active ClipStack, so a
// rounded clip leaked glyph ink into the corner cut-outs. The GPU glyph path
// was already clipped; this keeps CPU fallback and the offscreen CPU contract
// on the same Skia save/clip semantics.
func TestClipRoundRectCPUBitmapText(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	const w, h = 430, 232
	const clipX, clipY, clipW, clipH, clipR = 16, 26, 390, 150, 36

	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1})
	face, _, err := text.LoadDefaultFace(40)
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	dc.SetFont(face)
	dc.SetRGBA(0.98, 0.98, 0.98, 1)
	dc.Push()
	dc.ClipRoundRect(clipX, clipY, clipW, clipH, clipR)
	for i := 0; i < 3; i++ {
		dc.DrawString("RRECT CLIP TEXT MMMM", clipX-56, clipY+42+float64(i*40))
	}
	dc.Pop()

	inside, outside := scanBrightInk(dc.Image(), w, h, clipX, clipY, clipW, clipH, 2)
	if inside == 0 {
		t.Fatal("CPU text produced no ink inside the clip")
	}
	if outside != 0 {
		t.Fatalf("CPU text ink outside ClipRoundRect: %d px", outside)
	}
}

// TestClipRoundRectCPUAliasedText covers the binary-coverage CPU text entry.
// It shares the same ClipStack compositing contract as the bitmap path.
func TestClipRoundRectCPUAliasedText(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	const w, h = 430, 232
	const clipX, clipY, clipW, clipH, clipR = 16, 26, 390, 150, 36

	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1})
	face, _, err := text.LoadDefaultFace(40)
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	dc.SetFont(face)
	dc.SetTextMode(render.TextModeAliased)
	dc.SetRGBA(0.98, 0.98, 0.98, 1)
	dc.Push()
	dc.ClipRoundRect(clipX, clipY, clipW, clipH, clipR)
	for i := 0; i < 3; i++ {
		dc.DrawString("RRECT CLIP TEXT MMMM", clipX-56, clipY+42+float64(i*40))
	}
	dc.Pop()

	inside, outside := scanBrightInk(dc.Image(), w, h, clipX, clipY, clipW, clipH, 2)
	if inside == 0 {
		t.Fatal("CPU aliased text produced no ink inside the clip")
	}
	if outside != 0 {
		t.Fatalf("CPU aliased text ink outside ClipRoundRect: %d px", outside)
	}
}

// TestClipRoundRectCPUScaledText covers drawStringScaled, the uniform-scale
// CPU bitmap entry. The clip and text are transformed together, so the scan
// uses the device-space clip rectangle.
func TestClipRoundRectCPUScaledText(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	const scale = 2.0
	const w, h = int(430 * scale), int(232 * scale)
	const clipX, clipY, clipW, clipH, clipR = 16.0, 26.0, 390.0, 150.0, 36.0

	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1})
	face, _, err := text.LoadDefaultFace(40)
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	dc.SetFont(face)
	dc.SetTextMode(render.TextModeBitmap)
	dc.SetRGBA(0.98, 0.98, 0.98, 1)
	dc.Scale(scale, scale)
	dc.Push()
	dc.ClipRoundRect(clipX, clipY, clipW, clipH, clipR)
	for i := 0; i < 3; i++ {
		dc.DrawString("RRECT CLIP TEXT MMMM", clipX-56, clipY+42+float64(i*40))
	}
	dc.Pop()

	inside, outside := scanBrightInk(dc.Image(), w, h,
		int(clipX*scale), int(clipY*scale), int(clipW*scale), int(clipH*scale), 2)
	if inside == 0 {
		t.Fatal("CPU scaled text produced no ink inside the clip")
	}
	if outside != 0 {
		t.Fatalf("CPU scaled text ink outside ClipRoundRect: %d px", outside)
	}
}

func scanBrightInk(img image.Image, w, h, clipX, clipY, clipW, clipH, margin int) (inside, outside int) {
	if img == nil {
		return 0, 0
	}
	b := img.Bounds()
	for y := 0; y < h && y < b.Dy(); y++ {
		for x := 0; x < w && x < b.Dx(); x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if 0.2126*float64(r>>8)+0.7152*float64(g>>8)+0.0722*float64(bl>>8) <= 150 {
				continue
			}
			in := x >= clipX && x < clipX+clipW && y >= clipY && y < clipY+clipH
			if in {
				inside++
			} else if x < clipX-margin || x >= clipX+clipW+margin || y < clipY-margin || y >= clipY+clipH+margin {
				outside++
			}
		}
	}
	return inside, outside
}
