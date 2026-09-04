package render_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
)

type splitStubFont struct{ colorGID uint16 }

func (s *splitStubFont) HasColorTables() bool { return true }
func (s *splitStubFont) GlyphType(gid uint16) text.GlyphType {
	if gid == s.colorGID {
		return text.GlyphTypeBitmap
	}
	return text.GlyphTypeOutline
}
func (s *splitStubFont) BitmapGlyph(uint16, uint16) (*emoji.BitmapGlyph, error) {
	return nil, emoji.ErrNoCBDTTable
}
func (s *splitStubFont) COLRGlyph(uint16, int) (*emoji.COLRGlyph, error) {
	return nil, emoji.ErrNoCOLRTable
}

func TestSplitColorGlyphs_Mixed(t *testing.T) {
	cf := &splitStubFont{colorGID: 65}
	glyphs := []text.ShapedGlyph{{GID: 64}, {GID: 65}, {GID: 66}}
	color, outline := render.SplitColorGlyphs(cf, glyphs)
	if len(color) != 1 || color[0].GID != 65 {
		t.Fatalf("color = %+v, want [GID 65]", color)
	}
	if len(outline) != 2 || outline[0].GID != 64 || outline[1].GID != 66 {
		t.Fatalf("outline = %+v, want [64 66] in order", outline)
	}
}

func TestSplitColorGlyphs_AllOutline(t *testing.T) {
	cf := &splitStubFont{colorGID: 65}
	glyphs := []text.ShapedGlyph{{GID: 64}, {GID: 66}}
	color, outline := render.SplitColorGlyphs(cf, glyphs)
	if len(color) != 0 {
		t.Fatalf("color = %+v, want empty", color)
	}
	if len(outline) != 2 {
		t.Fatalf("outline = %+v, want both", outline)
	}
}

func colorEmojiFont(t *testing.T) string {
	t.Helper()
	for _, p := range []string{
		"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/TTF/NotoColorEmoji.ttf",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("NotoColorEmoji not available")
	return ""
}

func TestDrawShapedColorGlyphsCPU_Emoji(t *testing.T) {
	dc := render.NewContext(160, 160)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	if err := dc.LoadFontFace(colorEmojiFont(t), 16); err != nil {
		t.Fatalf("LoadFontFace: %v", err)
	}
	face := dc.Font()
	glyphs := text.LayoutGlyphs(face, "\U0001F600")
	if len(glyphs) == 0 {
		t.Fatal("no shaped glyphs for emoji")
	}
	dc.SetRGB(0, 0, 0)
	dc.DrawShapedColorGlyphs(glyphs, face, 20, 80)
	img := dc.Image()
	found := false
	for y := 0; y < 160 && !found; y++ {
		for x := 0; x < 160; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r < 0xFFFF || g < 0xFFFF || b < 0xFFFF {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no ink from DrawShapedColorGlyphs CPU fallback on NotoColorEmoji")
	}
}

func TestDrawShapedColorGlyphs_SkipsOutline(t *testing.T) {
	dc := render.NewContext(64, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	if err := dc.LoadFontFace(residualFont(t), 16); err != nil {
		t.Fatalf("LoadFontFace: %v", err)
	}
	face := dc.Font()
	glyphs := text.LayoutGlyphs(face, "A")
	dc.SetRGB(0, 0, 0)
	dc.DrawShapedColorGlyphs(glyphs, face, 8, 20)
	img := dc.Image()
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r < 0xFFFF || g < 0xFFFF || b < 0xFFFF {
				t.Fatalf("outline glyph drew ink at (%d,%d); color call must skip outlines", x, y)
			}
		}
	}
}
