package render

import (
	"os"
	"testing"

	"github.com/energye/gpui/render/text"
)

// TestColorEmojiDrawStringNoCrash_M5 locks the M5-window crash: drawing an
// emoji run through a DejaVu+NotoColorEmoji MultiFace must not nil-deref in
// DrawWithEmoji's face.Source() chain (CPU bitmap path).
func TestColorEmojiDrawStringNoCrash_M5(t *testing.T) {
	open := func(p string) text.Face {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		src, err := text.NewFontSource(b)
		if err != nil {
			return nil
		}
		t.Cleanup(func() { _ = src.Close() })
		return src.Face(16)
	}
	latin := open("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	emoji := open("/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf")
	if latin == nil || emoji == nil {
		t.Skip("need DejaVuSans + NotoColorEmoji system fonts")
	}
	mf, err := text.NewMultiFace(latin, emoji)
	if err != nil {
		t.Fatalf("NewMultiFace: %v", err)
	}
	// 嵌套链（窗内形态）：外层由内层链 + 表情脸组成，表情段经 DrawString
	// 走 CPU 位图路时曾以 face.Source()==nil 空指针崩溃，守卫必须接住。
	inner, err := text.NewMultiFace(latin)
	if err != nil {
		t.Fatalf("NewMultiFace inner: %v", err)
	}
	nested, err := text.NewMultiFace(inner, emoji)
	if err != nil {
		t.Fatalf("NewMultiFace nested: %v", err)
	}
	dc := NewContext(400, 200)
	defer dc.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DrawString panicked: %v", r)
		}
	}()
	dc.SetFont(mf)
	dc.DrawString("hi 🎉 bye", 10, 50)
	dc.DrawString("🎉", 10, 100)
	dc.SetFont(nested)
	dc.DrawString("hi 🎉 bye", 10, 150)
	dc.DrawString("🎉", 10, 180)
}
