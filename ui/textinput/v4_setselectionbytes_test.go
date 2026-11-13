package textinput

// §2.2 caret 命中入口防假绿（2026-11-13 四档真窗 burst 冒烟发现）：
// SetSelectionBytes 是字节域选择的公开入口，旧实现每次调用物化整串
// （len(e.Text()) 钳制 ×2）并做两次全前缀 range 扫描数 UTF-16 单元——
// tree 模式大文档上结构性 O(n)：20MB 文档实测 21.08ms/次、53MB 文档
// 60.3ms/次（ui_text_large_scale burst 冒烟 + v4t_diag_test.go 诊断），
// 超 §2.2 caret ≤4ms 门禁 5-15 倍。CPU 门禁 TestV4Gate_Caret4ms 走
// caretAnchor 只测了锚点计算，没测这个入口——正是 V0 §7.2 说的假快陷阱。
// 标准方案：V1b-3 已建 Buffer.UTF16OffsetAt（O(log n)，块边界不劈 rune、
// utf16Count 与旧扫描逐 rune 语义一致；box.go:2183 / editor.go:1533 同款
// 已用），本测试钉住「不物化 + 语义一致 + 计时门禁」。

import (
	"strings"
	"testing"
	"time"
)

// TestV4Gate_SetSelectionBytesNoMaterialize locks the structural contract:
// a byte-offset selection on a tree-mode document must go through the
// buffer, never materialize the full-string cache.
func TestV4Gate_SetSelectionBytesNoMaterialize(t *testing.T) {
	doc := v4GateUniqueLines(1000000)
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(doc)
	if ed.ensureBuf().Mode() != "tree" {
		t.Fatalf("expected tree mode for the 1e6-line document")
	}
	off := len(doc) / 2
	ed.SetSelectionBytes(off, off)
	if !ed.textDirty {
		t.Fatalf("V4 RED: SetSelectionBytes materialized the full-string cache (O(n) per call)")
	}
	want := ed.ensureBuf().UTF16OffsetAt(off)
	if got := ed.SelectionRange().Base; got != want {
		t.Fatalf("V4 RED: SetSelectionBytes(%d) landed at utf16 %d, want %d", off, got, want)
	}
}

// TestV4Gate_SetSelectionBytesMultibyte pins byte->utf16 semantics on CJK
// text (3-byte runes), including mid-rune offsets and both ends.
func TestV4Gate_SetSelectionBytesMultibyte(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 20000; i++ {
		sb.WriteString("世界abc\n")
	}
	doc := sb.String()
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(doc)
	b := ed.ensureBuf()
	// Mid-rune offsets (1,2) must match the buffer's per-rune counting —
	// the same walk the old string scan used (RuneError counts as one unit).
	for _, off := range []int{0, 1, 2, 3, 6, 7, 100, 12345, len(doc) - 1, len(doc)} {
		ed.SetSelectionBytes(off, off)
		want := b.UTF16OffsetAt(off)
		if got := ed.SelectionRange().Base; got != want {
			t.Fatalf("off=%d: selection utf16 base=%d, want %d", off, got, want)
		}
	}
}

// TestV4Gate_SetSelectionBytesCaretBound is the §2.2 timing backstop: 60 far
// byte-offset selections on a 20MB tree document, median <= 4ms. The old
// scan measured ~21ms median (5x over the gate); the buffer path is
// microseconds — the margin keeps this stable on loaded machines.
func TestV4Gate_SetSelectionBytesCaretBound(t *testing.T) {
	doc := v4GateUniqueLines(1000000)
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(doc)
	var ds []time.Duration
	step := len(doc) / 64
	off := step
	for i := 0; i < 60; i++ {
		t0 := time.Now()
		ed.SetSelectionBytes(off, off)
		ds = append(ds, time.Since(t0))
		off += step
		if off >= len(doc)-1 {
			off = step
		}
	}
	median := v4GateMedian(ds)
	t.Logf("V4 gate SetSelectionBytes median = %v (gate <=4ms)", median)
	if median > 4*time.Millisecond {
		t.Fatalf("V4 RED: SetSelectionBytes median %v exceeds the 4ms caret gate (O(n) scan)", median)
	}
}
