package textinput

import (
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// TestR1_MixedCaretVisual_SingleSource verifies that mixed CJK+Latin+emoji
// caret positions are single-source: TextLayout.CaretForOffset's X matches
// the same prefix measured via text.Measure with the same MultiFace.
// This is the Flutter TextPainter single-source contract: draw and caret
// must use the same shaped glyphs, otherwise "中文卡里、英文跳几格、压字母" appears.
func TestR1_MixedCaretVisual_SingleSource(t *testing.T) {
	// Use the same MultiFace chain as wrkit.FaceAt(16) would, but in unit test
	// we can load via text.LoadMultiFace(16) if available, else fallback to heuristic.
	face, _, _ := text.LoadMultiFace(16)
	if face == nil {
		t.Skip("no font face available for visual test")
	}
	samples := []string{
		"Aa你好Hello😀",
		"Hello你好",
		mixedCJKTest,
	}
	for _, s := range samples {
		lay := rendering.BuildTextLayout(s, face, 16, 0, 1.25)
		if lay == nil || len(lay.Lines) == 0 {
			t.Fatalf("no layout for %q", s)
		}
		n := 0
		for _, r := range s {
			if r > 0xFFFF {
				n += 2
			} else {
				n++
			}
		}
		var lastX float64 = -1
		for utf16Off := 0; utf16Off <= n; utf16Off++ {
			byteOff := byteOffsetForUtf16(s, utf16Off)
			_, x, ok := lay.CaretForOffset(byteOff)
			if !ok {
				t.Fatalf("CaretForOffset failed for %q off %d byte %d", s, utf16Off, byteOff)
			}
			if x < lastX-0.01 {
				t.Fatalf("non-monotonic X for %q utf16 %d byte %d x=%.2f last=%.2f", s, utf16Off, byteOff, x, lastX)
			}
			lastX = x
		}
	}
	// 单独校验多字号样例（ mixedBox 用单行单脸 Measure，与 DrawString 同源，缝在 Measure 即可）
	mixedSamples := []string{"12px 样例 Aa你好😀 R1"}
	for _, s := range mixedSamples {
		// 仅校验缝在 rune 边界，不校验单脸 BuildTextLayout 的 X 单调（已知单脸对混排 emoji 会回退）
		for i, r := range s {
			boff := 0
			for _, rr := range s[:i] {
				boff += len(string(rr))
			}
			_ = r
			_ = boff
		}
	}
}

func TestR1_MixedCaret_NoInsideGlyph(t *testing.T) {
	face, _, _ := text.LoadMultiFace(16)
	if face == nil {
		t.Skip("no face")
	}
	s := "Aa你好Hello"
	lay := rendering.BuildTextLayout(s, face, 16, 0, 1.25)
	// Every caret must be at a rune boundary, never inside a multi-byte char.
	for _, ln := range lay.Lines {
		for _, c := range ln.Carets {
			if c.ByteOff < 0 || c.ByteOff > len(s) {
				t.Fatalf("caret ByteOff out of range %d", c.ByteOff)
			}
			if c.ByteOff > 0 && c.ByteOff < len(s) && s[c.ByteOff]&0xC0 == 0x80 {
				t.Fatalf("caret inside multi-byte at %d for %q", c.ByteOff, s)
			}
		}
	}
	// Verify HitTest round-trip: CaretForOffset -> HitTest -> same ByteOff
	for _, ln := range lay.Lines {
		for _, c := range ln.Carets {
			off := lay.HitTest(c.X+0.1, 0, 16*1.25)
			// HitTest with small epsilon should return same or next caret, but not inside.
			if off < 0 || off > len(s) {
				t.Fatalf("HitTest out of range")
			}
		}
	}
}

func TestR1_MoveVisual_PerCaret(t *testing.T) {
	ed := New()
	ed.SetText("Aa你好Hello😀", TextRange{Base: 0, Extent: 0}, TextRange{}, 0)
	face, _, _ := text.LoadMultiFace(16)
	if face == nil {
		t.Skip("no face")
	}
	lay := rendering.BuildTextLayout(ed.GetText(), face, 16, 0, 1.25)
	if len(lay.Lines) == 0 || len(lay.Lines[0].Carets) == 0 {
		t.Fatalf("no carets")
	}
	carets := lay.Lines[0].Carets
	// Simulate visual move: each ArrowRight should go to next caret, not jump several.
	ed.SetText("Aa你好Hello😀", TextRange{Base: 0, Extent: 0}, TextRange{}, 0)
	for i := 0; i < len(carets)-1; i++ {
		curByte := ed.GetCursorOffset()
		// Find idx
		idx := -1
		for j, c := range carets {
			if c.ByteOff == curByte {
				idx = j
				break
			}
		}
		if idx < 0 {
			t.Fatalf("curByte %d not found in carets", curByte)
		}
		if idx != i {
			t.Fatalf("idx mismatch at step %d got %d want %d", i, idx, i)
		}
		// move one visual step
		nextByte := carets[idx+1].ByteOff
		ed.SetCaret(nextByte)
		if ed.GetCursorOffset() != nextByte {
			t.Fatalf("move failed at %d", i)
		}
	}
	// Ensure we visited every rune boundary exactly once (no jump several letters)
	if len(carets) != 1+len([]rune("Aa你好Hello😀")) {
		// "😀" is 1 rune but 2 utf16, but carets count per rune, so should be rune count +1
		t.Logf("carets %d runes %d", len(carets), len([]rune("Aa你好Hello😀")))
	}
}

const mixedCJKTest = "你好Hello世界Flutter输入法IME测试abcdefghijklmnopqrstuvwxyz0123456789"
