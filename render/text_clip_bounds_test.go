package render

import (
	"image"
	"image/color"
	"testing"

	"github.com/energye/gpui/render/text"
)

// TestTextRunBoundsContainRenderedInk guards the tightened CPU text-clip
// scratch allocation. The hint must contain every rendered glyph, including
// ascenders, descenders, and side bearings; the clip path falls back to the
// full clip if the border scan detects a miss.
func TestTextRunBoundsContainRenderedInk(t *testing.T) {
	face, _, err := text.LoadDefaultFace(20)
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	cases := []string{
		"The quick brown fox jumps over the lazy dog",
		"MMMM WWWW",
		"gjpqy AVATAR",
		"fi ffl 0123456789",
	}
	for _, s := range cases {
		img := image.NewRGBA(image.Rect(0, 0, 800, 160))
		text.Draw(img, s, face, 200, 100, color.White)
		ink := opaqueBounds(img)
		if ink.Empty() {
			t.Fatalf("no ink for %q", s)
		}
		hint := textRunBounds(s, face, 200, 100)
		if hint.Min.X > ink.Min.X || hint.Min.Y > ink.Min.Y ||
			hint.Max.X < ink.Max.X || hint.Max.Y < ink.Max.Y {
			t.Fatalf("hint %v does not contain ink %v for %q", hint, ink, s)
		}
	}
}

func opaqueBounds(img *image.RGBA) image.Rectangle {
	if img == nil {
		return image.Rectangle{}
	}
	b := img.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.Pix[y*img.Stride+x*4+3] == 0 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if minX >= maxX || minY >= maxY {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX, maxY)
}
