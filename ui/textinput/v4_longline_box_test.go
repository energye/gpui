package textinput

// V4 单行击键全路径门禁（2026-11-14 L2/L3 真窗 burst 发现）：
// 既有 CPU 门禁 TestV4_SingleLineKeystroke_* 走 SetTextSpan+TextLayout 直连，
// 没经过 Box.sync 的光标跟随/窗口滑动 —— 真窗 burst 实测 Box 全路径击键
// 随行长线性增长（1e6=3.5ms / 3e6=17.5ms / 5e6=28.8ms），主源是
// ui/rendering capBytes 旧守卫的整行 rune 计数（画像 50.25% cum）。
// 本测试把 §2.2 的 5e6 单行击键 ≤16ms 钉在 Box 生产路径上（窗口滑动 +
// 布局 + 适配一次测全），防止只测直连路径的假绿。

import (
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func v4LongLineSeedText(t *testing.T, n int) string {
	t.Helper()
	b, err := os.ReadFile("../rendering/testdata/v4_longline_seed.txt")
	if err != nil || len(b) == 0 {
		t.Skipf("v4 seed missing: %v", err)
	}
	seed := strings.TrimRight(string(b), "\r\n")
	if seed == "" {
		t.Skipf("v4 seed is empty")
	}
	var sb strings.Builder
	sb.Grow(n + len(seed))
	for sb.Len() < n {
		sb.WriteString(seed)
	}
	return sb.String()[:n]
}

func TestV4Gate_LongLineBoxKeystroke5M(t *testing.T) {
	doc := v4LongLineSeedText(t, 5000000)
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(doc)
	b := NewMultiLineInputBox(ed, 800, 600, 14)
	b.sync()
	if b.VirtualMode() {
		t.Fatalf("single long line must stay on the RenderText window path (not virtual)")
	}
	docLen := ed.BufferSnapshot().Len()
	const n = 120
	stride := docLen / (n + 2)
	if stride < 64 {
		stride = 64
	}
	off := stride
	var ds []time.Duration
	for i := 0; i < n; i++ {
		ed.SetSelectionBytes(off, off)
		t0 := time.Now()
		ed.Insert("x")
		b.sync()
		ds = append(ds, time.Since(t0))
		off += stride
		if off >= docLen-1 {
			off = stride
		}
	}
	if ed.BufferSnapshot().Len() != docLen+n {
		t.Fatalf("V4 RED: keystrokes did not really edit the document (no-op timing)")
	}
	c := append([]time.Duration(nil), ds...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	median := c[len(c)/2]
	t.Logf("V4 gate 5e6 single-line box keystroke median = %v (gate <=16ms, 120 ops unique positions)", median)
	if median > 16*time.Millisecond {
		t.Fatalf("V4 RED: box keystroke median %v exceeds the 16ms gate (O(L) in the window-follow path)", median)
	}
}
