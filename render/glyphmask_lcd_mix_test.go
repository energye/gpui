//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/text"
)

// TestGlyphMaskMixedGrayLCDSameFrame guards the layout-switch atlas wipe:
// a frame that draws grayscale text before LCD text used to clear the atlas on
// SetLCDLayout(RGB), then the LCD glyphs re-allocated from (0,0) and overwrote
// the grayscale masks. The grayscale row rendered compressed/garbled while the
// LCD row looked fine. Both draw orders are covered because the wipe hit the
// batches that were already laid out.
func TestGlyphMaskMixedGrayLCDSameFrame(t *testing.T) {
	requireNativeGPU(t)
	src, err := text.NewFontSourceFromFile("text/testdata/source-sans/OTF/SourceSans3-Regular.otf")
	if err != nil {
		t.Skipf("test font unavailable: %v", err)
	}
	defer src.Close()
	face := src.Face(24)

	const (
		canvasW, canvasH = 320, 180
		grayBaseline     = 50
		lcdBaseline      = 120
		grayBandEnd      = 85
	)
	draw := func(steps ...func(dc *render.Context)) image.Image {
		dc := render.NewContext(canvasW, canvasH)
		defer dc.Close()
		dc.ClearWithColor(render.White)
		dc.SetFont(face)
		dc.SetRGB(0, 0, 0)
		for _, step := range steps {
			step(dc)
		}
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		return dc.Image().(image.Image)
	}
	gray := func(dc *render.Context) {
		dc.SetTextMode(render.TextModeGlyphMask)
		dc.SetLCDLayout(render.LCDLayoutNone)
		dc.DrawString("AgHij", 12, grayBaseline)
	}
	lcd := func(dc *render.Context) {
		dc.SetTextMode(render.TextModeGlyphMask)
		dc.SetLCDLayout(render.LCDLayoutRGB)
		dc.DrawString("AgHij", 12, lcdBaseline)
	}

	grayRef := draw(gray)
	lcdRef := draw(lcd)

	for _, tc := range []struct {
		name  string
		steps []func(dc *render.Context)
	}{
		{"gray_then_lcd", []func(dc *render.Context){gray, lcd}},
		{"lcd_then_gray", []func(dc *render.Context){lcd, gray}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mixed := draw(tc.steps...)
			if diff, max := bandDiff(mixed, grayRef, 0, grayBandEnd); diff > 0 || max > 2 {
				t.Fatalf("grayscale band differs from single-mode render: %d px, max channel delta %d", diff, max)
			}
			if diff, max := bandDiff(mixed, lcdRef, grayBandEnd, canvasH); diff > 0 || max > 2 {
				t.Fatalf("LCD band differs from single-mode render: %d px, max channel delta %d", diff, max)
			}
			// The LCD row must still produce subpixel color; the grayscale row
			// must stay neutral. This catches a silent fall-back to one mode.
			if colored := bandColored(mixed, grayBandEnd, canvasH); colored < 10 {
				t.Fatalf("LCD band has only %d colored pixels, want subpixel fringes", colored)
			}
			if colored := bandColored(mixed, 0, grayBandEnd); colored != 0 {
				t.Fatalf("grayscale band has %d colored pixels, want neutral gray", colored)
			}
		})
	}
}

func bandDiff(a, b image.Image, y0, y1 int) (diff, maxDelta int) {
	bounds := a.Bounds()
	for y := y0; y < y1 && y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			d := mixMaxInt(mixChannelDelta8(ar, br), mixMaxInt(mixChannelDelta8(ag, bg), mixChannelDelta8(ab, bb)))
			if d > 0 {
				diff++
			}
			if d > maxDelta {
				maxDelta = d
			}
		}
	}
	return diff, maxDelta
}

func bandColored(img image.Image, y0, y1 int) int {
	bounds := img.Bounds()
	n := 0
	for y := y0; y < y1 && y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if mixChannelDelta8(r, g) > 2 || mixChannelDelta8(g, b) > 2 || mixChannelDelta8(r, b) > 2 {
				n++
			}
		}
	}
	return n
}

func mixChannelDelta8(a, b uint32) int {
	d := int(a>>8) - int(b>>8)
	if d < 0 {
		return -d
	}
	return d
}

func mixMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
