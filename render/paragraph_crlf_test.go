package render

// K4（P0-3 探针暴露，2026-11-03 登记）：builder 的 span 边界按**原文**累计
// （\r\n = 2 rune），而 layout 侧 splitParagraphs 先归一 \r\n→\n（= 1 rune）
// 再排——cluster 是归一化空间的全局 rune 偏移，spanEnds 是原空间的。
// 每个前置 CRLF 让 run→span 边界右移 1：多段 + CRLF 混用时样式跨 span 渗色
// （LF 单段/无换行不受影响，latent）。修法 = Build 归一文本后再算边界，
// 与 splitParagraphs 同一口径。

import (
	"testing"

	"github.com/energye/gpui/render/text"
)

// TestParagraph_CRLFSpanBoundary pins the cluster-space span boundary with a
// CRLF hard break: "AB\r\n"(red) + "CD"(green). Normalized text = "AB\nCD":
// cluster 2 = '\n' (span 0), cluster 3 = 'C' of line 2 → span 1.
func TestParagraph_CRLFSpanBoundary(t *testing.T) {
	face := paragraphTestFace(t)
	b := NewParagraphBuilder(face, TextStyle{})
	b.PushStyle(TextStyle{Color: RGBA{R: 1, A: 1}})
	b.AddText("AB\r\n")
	b.PopStyle()
	b.PushStyle(TextStyle{Color: RGBA{G: 1, A: 1}})
	b.AddText("CD")
	p := b.Build(text.LayoutOptions{MaxWidth: 400})

	if got, _ := p.StyleForRun(2); got.Color.R != 1 || got.Color.G != 0 {
		t.Fatalf("cluster 2 ('\\n') must read span 0 (red), got %+v", got.Color)
	}
	// K4: the builder counted "AB\r\n" as 4 runes, layout counts it as 3 —
	// cluster 3 ('C') used to fall inside span 0 and bleed red across the
	// hard break.
	if got, _ := p.StyleForRun(3); got.Color.G != 1 || got.Color.R != 0 {
		t.Fatalf("cluster 3 ('C', line 2) must read span 1 (green), got %+v — K4 CRLF boundary shift", got.Color)
	}
	if got, _ := p.StyleForRun(4); got.Color.G != 1 || got.Color.R != 0 {
		t.Fatalf("cluster 4 ('D') must read span 1 (green), got %+v", got.Color)
	}
	// Run annotation: the second paragraph's run must carry span 1.
	found := false
	for _, line := range p.Layout.Lines {
		for _, r := range line.Runs {
			if len(r.Glyphs) == 0 {
				continue
			}
			if r.Glyphs[0].Cluster == 3 {
				found = true
				if r.Style != 1 {
					t.Fatalf("run at cluster 3 carries Style=%d, want 1 (span annotation uses the same shifted boundary)", r.Style)
				}
			}
		}
	}
	if !found {
		t.Fatal("no run at cluster 3 (layout shape changed?)")
	}
}

// TestParagraph_LFSpanBoundaryControl pins the already-correct LF case: one
// rune per break on both sides, no shift.
func TestParagraph_LFSpanBoundaryControl(t *testing.T) {
	face := paragraphTestFace(t)
	b := NewParagraphBuilder(face, TextStyle{})
	b.PushStyle(TextStyle{Color: RGBA{R: 1, A: 1}})
	b.AddText("AB\n")
	b.PopStyle()
	b.PushStyle(TextStyle{Color: RGBA{G: 1, A: 1}})
	b.AddText("CD")
	p := b.Build(text.LayoutOptions{MaxWidth: 400})

	if got, _ := p.StyleForRun(3); got.Color.G != 1 || got.Color.R != 0 {
		t.Fatalf("LF control: cluster 3 ('C') must read span 1 (green), got %+v", got.Color)
	}
}

// TestParagraph_MultiCRLFSpanBoundary pins the accumulated shift: two CRLFs
// before the boundary shift it right by two ("A\r\nB\r\n"(red) + "C"(green),
// normalized "A\nB\nC", 'C' at cluster 4 → span 1).
func TestParagraph_MultiCRLFSpanBoundary(t *testing.T) {
	face := paragraphTestFace(t)
	b := NewParagraphBuilder(face, TextStyle{})
	b.PushStyle(TextStyle{Color: RGBA{R: 1, A: 1}})
	b.AddText("A\r\nB\r\n")
	b.PopStyle()
	b.PushStyle(TextStyle{Color: RGBA{G: 1, A: 1}})
	b.AddText("C")
	p := b.Build(text.LayoutOptions{MaxWidth: 400})

	if got, _ := p.StyleForRun(4); got.Color.G != 1 || got.Color.R != 0 {
		t.Fatalf("cluster 4 ('C', line 3) must read span 1 (green), got %+v — K4 two-CRLF shift", got.Color)
	}
}
