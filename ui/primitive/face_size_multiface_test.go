package primitive_test

import (
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestFaceAtSize_MultiFaceKeepsFallback(t *testing.T) {
	latin, err := text.NewFontSourceFromFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	if err != nil {
		t.Skip(err)
	}
	cjk, err := text.NewFontSourceFromFile("/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf")
	if err != nil {
		t.Skip(err)
	}
	mf, err := text.NewMultiFace(latin.Face(14), cjk.Face(14))
	if err != nil {
		t.Fatal(err)
	}
	if !mf.HasGlyph('中') {
		t.Fatal("MultiFace missing 中")
	}
	tx := primitive.NewText("基本布局")
	tx.Face = mf
	tx.FontSize = 20
	sz := tx.Layout(core.Loose(400, 100))
	if sz.Width < 40 {
		t.Fatalf("CJK title width=%v — face resize likely dropped CJK glyphs", sz.Width)
	}
	mf20 := mf.AtSize(20)
	if mf20.Size() < 19 || mf20.Size() > 21 {
		t.Fatalf("AtSize size=%v", mf20.Size())
	}
	if !mf20.HasGlyph('中') {
		t.Fatal("AtSize dropped CJK coverage")
	}
}
