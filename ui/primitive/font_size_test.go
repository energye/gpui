package primitive_test

import (
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestTextFontSizeOverridesFaceSize(t *testing.T) {
	src, err := text.NewFontSourceFromFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	if err != nil {
		t.Skip(err)
	}
	face14 := src.Face(14)
	tx := primitive.NewText("Hi")
	tx.Face = face14
	tx.FontSize = 8
	sz8 := tx.Layout(core.Loose(200, 100))
	tx.FontSize = 20
	tx.MarkNeedsLayout()
	sz20 := tx.Layout(core.Loose(200, 100))
	if sz20.Height <= sz8.Height {
		t.Fatalf("FontSize 20 height=%v should be > FontSize 8 height=%v", sz20.Height, sz8.Height)
	}
	if sz20.Width <= sz8.Width {
		t.Fatalf("FontSize 20 width=%v should be > FontSize 8 width=%v", sz20.Width, sz8.Width)
	}
}
