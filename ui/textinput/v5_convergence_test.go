package textinput

// V5 收敛期钉子（ENGINE_TEXT_LARGE_SCALE_PLAN §4 V5 / D2 阈值定稿）：
// 1) 虚拟模式各公开查询口不得触发全量行表构建——V2-V4 架构已达成
//    （虚拟模式下 RenderText.Text 为空，全表构建的输入不存在），
//    本测试锁住这个结构契约，防止未来调用点回归。
// 2) 64KB 小文档/大文档分界定稿：virtualTextMinBytes / bufferThreshold /
//    textbuffer.SmallMax 三处同源（编译期同一常量），本测试锁语义与行为
//    （阈值上下两侧的路径分派）。
// 3) 击键路径不物化整串（§3 硬约束）：虚拟模式编辑后 b.txt.Text 保持空。

import (
	"testing"
	"time"
)

func TestV5_VirtualSurfacesNoFullBuild(t *testing.T) {
	doc := v4GateUniqueLines(1000000) // ~23MB, tree mode, virtual mode
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(doc)
	b := NewMultiLineInputBox(ed, 800, 600, 14)
	b.sync()
	if !b.VirtualMode() {
		t.Fatalf("V5: 1e6-line box must enter virtual mode")
	}
	if got := b.txt.Text; got != "" {
		t.Fatalf("V5 RED: virtual-mode RenderText.Text is %d bytes, want empty (full-table build input must not exist)", len(got))
	}
	// 各公开口在虚拟文档上必须是轻量路径（全量构建会是秒级；50ms 上界
	// 给足机器噪声余量）。
	probes := []struct {
		name string
		fn   func()
	}{
		{"TextLayout", func() { _ = b.TextLayout() }},
		{"IMERect", func() { _ = b.IMERect() }},
		{"TopVisibleLine", func() { _ = b.TopVisibleLine() }},
		{"Sync", func() { b.Sync() }},
		{"LongLineWindow", func() { _, _, _, _ = b.LongLineWindow() }},
	}
	for _, p := range probes {
		t0 := time.Now()
		p.fn()
		if d := time.Since(t0); d > 50*time.Millisecond {
			t.Fatalf("V5 RED: %s took %v on a virtual-mode document — full-table build path leaked (flat rows regression)", p.name, d)
		}
	}
	// 编辑后仍虚拟、txt.Text 仍空（击键路径不物化整串）。
	ed.SetSelectionBytes(0, 0)
	ed.Insert("x")
	b.sync()
	if !b.VirtualMode() || b.txt.Text != "" {
		t.Fatalf("V5 RED: after edit virtual=%v txt.Text=%d bytes — keystroke path materialized the full string", b.VirtualMode(), len(b.txt.Text))
	}
}

func TestV5_ThresholdConstantsFinalized(t *testing.T) {
	// 定稿值 64KB：三处常量同源（编译期已相等），这里锁语义与行为。
	if virtualTextMinBytes != 64*1024 {
		t.Fatalf("V5: virtualTextMinBytes = %d, want 65536 (D2 finalized threshold)", virtualTextMinBytes)
	}
	if bufferThreshold != virtualTextMinBytes {
		t.Fatalf("V5: bufferThreshold = %d diverges from virtualTextMinBytes = %d — thresholds must stay single-sourced", bufferThreshold, virtualTextMinBytes)
	}
	// 行为钉：阈值上下两侧路径分派（小文档 string buffer、大文档 tree）。
	small := v4GateUniqueLines(100) // ~2.4KB
	edSmall := New()
	edSmall.SetTextSimple(small)
	if edSmall.ensureBuf().Mode() != "string" {
		t.Fatalf("V5: sub-threshold doc must stay a plain string buffer")
	}
	large := v4GateUniqueLines(1000000)
	edLarge := New()
	edLarge.SetTextSimple(large)
	if edLarge.ensureBuf().Mode() != "tree" {
		t.Fatalf("V5: over-threshold doc must be a tree buffer")
	}
	b := NewMultiLineInputBox(edLarge, 800, 600, 14)
	b.sync()
	if !b.VirtualMode() {
		t.Fatalf("V5: over-threshold multi-line box must enter virtual mode (canVirtual threshold)")
	}
}
