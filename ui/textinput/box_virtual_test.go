package textinput

import (
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// TestBoxVirtualSync_1e6 locks the V2b-2b-1 path: large multi-line no-wrap
// boxes sync via the viewport model (no GetText/full layout); non-collapsed
// selection falls back to the full path so selection/IME remain correct.
func TestBoxVirtualSync_1e6(t *testing.T) {
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(strings.Repeat("line content here\n", 1000000))
	t0 := time.Now()
	b := NewMultiLineInputBox(ed, 800, 600, 14)
	firstSync := time.Since(t0)
	t.Logf("1e6-line box first sync=%v", firstSync)
	if !b.virtualOn || !b.txt.Virtual() {
		t.Fatalf("large multi-line box did not enter virtual mode")
	}
	if b.txt.Text != "" {
		t.Fatalf("virtual sync materialized full text (len=%d)", len(b.txt.Text))
	}
	if !b.txt.NeedsPaint() {
		t.Fatalf("virtual text must be marked dirty before the first retained record")
	}
	if firstSync > 200*time.Millisecond {
		t.Fatalf("first sync=%v, looks like a full layout", firstSync)
	}
	if b.vtext.CacheLen() > 200 {
		t.Fatalf("cache=%d, expected O(visible)", b.vtext.CacheLen())
	}
	if _, top, _, ok := b.caretAnchor(); !ok || top < 0 || top > 600 {
		t.Fatalf("caret anchor top=%.1f ok=%v, expected inside the scrolled viewport", top, ok)
	}
	if line := ed.ensureBuf().LineIndex(ed.GetCursorOffset()); line < 999000 {
		t.Fatalf("caret line=%d, expected near document end", line)
	}
	// 首屏绘制必须物化可见行(抓不到 GPU 截图时的无头等价断言).
	ed.SetSelection(TextRange{Base: 0, Extent: 0})
	b.sync()
	if y := b.ScrollY(); y > 1 {
		t.Fatalf("caret at 0 but scrollY=%.1f", y)
	}
	pc := rendering.NewPaintContext(render.NewContext(800, 600), 1)
	b.txt.Paint(pc)
	if got := b.vtext.CacheLen(); got < 10 {
		t.Fatalf("virtual paint materialized %d lines, expected the visible window", got)
	}
	if b.txt.NeedsPaint() {
		t.Fatalf("paint did not clear the dirty flag")
	}
	// 滚动换窗必须重新标脏,否则 retained 录制会跳过文本.
	buf := ed.ensureBuf()
	u := buf.UTF16OffsetAt(buf.LineStart(1000))
	ed.SetSelection(TextRange{Base: u, Extent: u})
	b.sync()
	if !b.txt.NeedsPaint() {
		t.Fatalf("viewport window move must mark the virtual text dirty")
	}
	// 鼠标命中:第 50 万行的 x=0 应落在该行行首.
	line := 500000
	want := ed.ensureBuf().LineStart(line)
	got := b.virtualByteAt(0, b.vtext.LineTop(line))
	if got != want {
		t.Fatalf("virtualByteAt line %d = %d want %d", line, got, want)
	}
	// 打字后仍走虚拟,且只失效缓存.
	ed.AddText("x")
	b.sync()
	if !b.virtualOn || b.txt.Text != "" {
		t.Fatalf("typing dropped virtual mode or materialized text")
	}
	// 非折叠选区仍走虚拟,选区高亮按可见行生成.
	n := ed.ensureBuf().UTF16Len()
	ed.SetSelection(TextRange{Base: 0, Extent: n / 2})
	b.sync()
	if !b.virtualOn || b.txt.Text != "" {
		t.Fatalf("non-collapsed selection must stay virtual without materializing text")
	}
	if len(b.highlights) == 0 {
		t.Fatalf("virtual selection produced no visible highlight boxes")
	}
	// IME 合成仍走虚拟,候选框按可见行生成.
	ed.SetSelection(TextRange{Base: n, Extent: n})
	b.sync()
	ed.BeginComposing()
	ed.UpdateComposingText("x", TextRange{Base: n, Extent: n})
	b.sync()
	if !b.virtualOn {
		t.Fatalf("composing must stay virtual")
	}
	if r := b.IMERect(); r.W <= 0 || r.H <= 0 {
		t.Fatalf("virtual IME rect=%+v", r)
	}
}
