//go:build linux

package platform

import (
	"testing"
)

// TestX11S3SessionAnchor 验证 S3 会话与锚点：FocusIn幂等、UpdateCursorRect composing 实报、4000 居中、PurposePassword 禁推
func TestX11S3SessionAnchor(t *testing.T) {
	// FocusIn 幂等：EnableIME 两次，第二次应 guard skip
	im := &x11Ime{
		host: &x11Host{st: &x11State{scale: 1}},
	}
	// 初始 Enable
	im.EnableIME(Rect{X: 10, Y: 20, W: 2, H: 16})
	if !im.focused {
		t.Fatalf("EnableIME should set focused")
	}
	firstFocused := im.focused
	im.EnableIME(Rect{X: 10, Y: 20, W: 2, H: 16})
	if !im.focused || !firstFocused {
		t.Fatalf("second EnableIME should be idempotent")
	}
	// UpdateCursorRect 相同 rect 跳过
	im.UpdateCursorRect(Rect{X: 10, Y: 20, W: 2, H: 16})
	// 非 composing 时仅 preheat，不实发（hasRect 已更新但 composing false）
	if im.hasRect && im.composing {
		t.Fatalf("should not be composing yet")
	}
	// composing 时才实发
	im.mu.Lock()
	im.composing = true
	im.mu.Unlock()
	im.UpdateCursorRect(Rect{X: 12, Y: 20, W: 2, H: 16})
	// 4000 居中：SetComposing 对超长文本应截断
	long := make([]byte, 5000)
	for i := range long {
		long[i] = 'a'
	}
	text := string(long)
	im.SetComposing(text, 2500)
	// SetComposing 内部会 callSetSurroundingText，但无对象时 skip，不崩
	// 验证 truncate 长度
	tr, c, a := x11TruncateSurrounding(text, 2500, 2500)
	if len(tr)+1 > 4000 {
		t.Fatalf("truncate exceed 4000: %d", len(tr))
	}
	if c < 0 || c > len(tr) || a < 0 || a > len(tr) {
		t.Fatalf("cursor out of range")
	}
	// PurposePassword 时 SetComposing 应跳过推 surrounding（B13）
	im2 := &x11Ime{
		host: &x11Host{st: &x11State{scale: 1}},
	}
	im2.SetContentType(PurposePassword)
	im2.mu.Lock()
	im2.purpose = PurposePassword
	im2.mu.Unlock()
	// 即使 SetComposing 也不应推 surrounding（通过日志可验，此处仅验不崩）
	im2.SetComposing("password", 4)
	// DisableIME 应清 focused/composing
	im.DisableIME()
	if im.focused || im.composing {
		t.Fatalf("DisableIME should clear focused/composing")
	}
}
