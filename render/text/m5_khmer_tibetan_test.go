package text

import (
	"os"
	"testing"
)

// khmer/tibetan wordlists live in hint/testdata (khmer_all.txt: U+1780-U+17FF
// 128 codepoints, tibetan_all.txt: U+0F00-U+0FFF 256 codepoints, mirroring
// th_all.txt convention). Tests read the files; no range-generated tables.
func m5ShapeFile(t *testing.T, wordlist string, fontPaths []string) Face {
	t.Helper()
	raw, err := os.ReadFile(wordlist)
	if err != nil {
		t.Fatalf("read %s: %v", wordlist, err)
	}
	if len([]rune(string(raw))) < 100 {
		t.Fatalf("%s has %d runes, want >= 100", wordlist, len([]rune(string(raw))))
	}
	var data []byte
	for _, p := range fontPaths {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			data = b
			break
		}
	}
	if len(data) == 0 {
		t.Skipf("no Khmer/Tibetan font found (tried %v)", fontPaths)
	}
	src, err := NewFontSource(data)
	if err != nil {
		t.Fatalf("open font: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	return src.Face(16)
}

// TestShapeKhmer_M5 / TestShapeTibetan_M5 close the M5-3 gap: the only two
// scripts without wordlist coverage shape to non-empty glyph sequences.
func TestShapeKhmer_M5(t *testing.T) {
	face := m5ShapeFile(t, "hint/testdata/khmer_all.txt", []string{
		"/usr/share/fonts/truetype/ttf-khmeros-core/KhmerOS.ttf",
		"/usr/share/fonts/truetype/ttf-khmeros-core/KhmerOSsys.ttf",
	})
	raw, _ := os.ReadFile("hint/testdata/khmer_all.txt")
	glyphs := Shape(string(raw), face)
	if len(glyphs) == 0 {
		t.Fatal("Shape(Khmer 128) returned 0 glyphs")
	}
}

func TestShapeTibetan_M5(t *testing.T) {
	face := m5ShapeFile(t, "hint/testdata/tibetan_all.txt", []string{
		"/usr/share/fonts/truetype/tibetan-machine/TibetanMachineUni.ttf",
	})
	raw, _ := os.ReadFile("hint/testdata/tibetan_all.txt")
	glyphs := Shape(string(raw), face)
	if len(glyphs) == 0 {
		t.Fatal("Shape(Tibetan 256) returned 0 glyphs")
	}
}
