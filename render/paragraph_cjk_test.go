//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"os"
	"testing"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
)

// P3.3 交付后审查 P0-3 三钉：cluster 统一为 rune 下标后的非 ASCII 语义。
//
// 旧状（byte cluster）下这三条全部红：多 span 的 CJK run 颜色落错 span、
// 拉丁+CJK 混排的字重 face 丢失、非 ASCII Justify 空格定位失败不拉伸。
// 存量能力钉全 ASCII，是这条系统性错位的门禁盲区。

func cjkTestFamily(t *testing.T, size float64) (text.Face, *text.FontFamily) {
	t.Helper()
	const latinPath = "text/testdata/goregular.ttf"
	const cjkPath = "text/testdata/wqy-microhei.ttc"
	for _, p := range []string{latinPath, cjkPath} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("test font missing: %s (%v)", p, err)
		}
	}
	latinFace, _, _, err := text.LoadFontFamily(size, text.FontRef{Path: latinPath})
	if err != nil || latinFace == nil {
		t.Skipf("latin test font unavailable: err=%v", err)
	}
	cjkFace, cjkFam, _, err := text.LoadFontFamily(size, text.FontRef{Path: cjkPath})
	if err != nil || cjkFace == nil || cjkFam == nil {
		t.Skipf("CJK test font unavailable: err=%v", err)
	}
	mf, err := text.NewMultiFace(latinFace, cjkFace)
	if err != nil || mf == nil {
		t.Skipf("MultiFace unavailable: err=%v", err)
	}
	return mf, cjkFam
}

// 钉一：CJK 三 span 颜色。三段样式文字（红/黄/绿），每段 glyph 必须落在自己的
// span 上：counts 必须 = (2, 2, 4)。旧 byte cluster 下「黄色」的 cluster 被放大
// 到 6/7，落进最后一个 span，counts = (2, 0, 6)。
func TestParagraph_CJKThreeSpanColors(t *testing.T) {
	face, _ := cjkTestFamily(t, 20.0)
	b := NewParagraphBuilder(face, TextStyle{})
	b.PushStyle(TextStyle{Color: RGBA{R: 1, A: 1}})
	b.AddText("红色")
	b.PopStyle()
	b.PushStyle(TextStyle{Color: RGBA{G: 1, A: 1}})
	b.AddText("黄色")
	b.PopStyle()
	b.PushStyle(TextStyle{Color: RGBA{B: 1, A: 1}})
	b.AddText("绿ABC")
	b.PopStyle()
	p := b.Build(text.LayoutOptions{MaxWidth: 400})
	if p.StyleCount() != 3 {
		t.Fatalf("StyleCount=%d, want 3", p.StyleCount())
	}
	counts := []int{0, 0, 0}
	for _, line := range p.Layout.Lines {
		for _, r := range line.Runs {
			if r.Style < 0 || r.Style >= len(counts) {
				t.Fatalf("run.Style=%d out of span range", r.Style)
			}
			counts[r.Style] += len(r.Glyphs)
		}
	}
	if counts[0] != 2 || counts[1] != 2 || counts[2] != 4 {
		t.Fatalf("span glyph counts = %v, want [2 2 4]（「黄色」被错标进后一个 span 即 cluster 字节语义残留）", counts)
	}
}

// 钉二：拉丁+CJK 混排的字重 face。「世界」两个 rune 的 run 必须带 700 字重
// face。旧状 styledFace 收到的是段内切片却按全段边界裁剪，CJK 段被错按
// span0（无字重）解析，face 权重落在 base 上。
func TestParagraph_MixedLatinCJKWeightFace(t *testing.T) {
	face, fam := cjkTestFamily(t, 20.0)
	b := NewParagraphBuilder(face, TextStyle{})
	b.SetFamily(fam)
	b.AddText("Hello ")
	b.PushStyle(TextStyle{Weight: text.WeightBold})
	b.AddText("世界")
	b.PopStyle()
	p := b.Build(text.LayoutOptions{MaxWidth: 400})
	found := false
	for _, line := range p.Layout.Lines {
		for _, r := range line.Runs {
			if len(r.Glyphs) == 0 {
				continue
			}
			c := r.Glyphs[0].Cluster
			if c < 6 || c >= 8 {
				continue
			}
			found = true
			if r.Face == nil {
				t.Fatalf("CJK run carries no face")
			}
			if r.Face.Weight() != text.WeightBold {
				t.Fatalf("世界 run face weight=%d, want %d（700 字重 face 丢失）", r.Face.Weight(), text.WeightBold)
			}
		}
	}
	if !found {
		t.Fatal("no run carries the 世界 clusters (6..8)")
	}
}

// 钉三：非 ASCII Justify。多段 CJK+空格的行必须像 ASCII 一样把空格找对并拉伸
// 到 MaxWidth；旧 byte cluster 下空格 glyph 的 cluster 指向错误的 rune，gapCount=0
// 直接放弃拉伸。
func TestLayout_NonASCIIJustifyStretches(t *testing.T) {
	face, _ := cjkTestFamily(t, 20.0)
	const para = "红色 黄色 绿色 红色 黄色 绿色\n收尾"
	left := text.LayoutText(para, face, text.LayoutOptions{
		MaxWidth: 150, Alignment: text.AlignLeft,
	})
	just := text.LayoutText(para, face, text.LayoutOptions{
		MaxWidth: 150, Alignment: text.AlignJustify,
	})
	if len(just.Lines) < 3 {
		t.Fatalf("lines=%d, want >=3（首段必须折行出内行）", len(just.Lines))
	}
	if len(left.Lines) != len(just.Lines) {
		t.Fatalf("left=%d vs justify=%d lines, must match", len(left.Lines), len(just.Lines))
	}
	nat := left.Lines[0].Width
	stretched := just.Lines[0].Width
	if stretched < nat+10 {
		t.Fatalf("interior CJK line not stretched: natural=%.2f justified=%.2f MaxWidth=150（空格没被认出来）", nat, stretched)
	}
	if fdiff(stretched, 150) > 1 {
		t.Fatalf("justified interior line width=%.2f, want 150±1", stretched)
	}
	// 末行不拉伸（Skia/Flutter 语义）照旧成立。
	last := just.Lines[len(just.Lines)-1]
	if last.Width > 149 {
		t.Fatalf("last line must not stretch, got %.2f", last.Width)
	}
}

// P1-4 钉：字重 face 必须取基准字号。24px 的段落里 bold run 的 face 若被
// 硬编码成 20px，度量/字距/渲染全偏移。旧状 family.Face(20.0, ...) 写死。
func TestParagraph_WeightFaceFollowsBaseSize(t *testing.T) {
	const baseSize = 24.0
	face, fam := cjkTestFamily(t, baseSize)
	b := NewParagraphBuilder(face, TextStyle{})
	b.SetFamily(fam)
	b.AddText("你好 ")
	b.PushStyle(TextStyle{Weight: text.WeightBold})
	b.AddText("世界")
	b.PopStyle()
	p := b.Build(text.LayoutOptions{MaxWidth: 600})
	checked := 0
	for _, line := range p.Layout.Lines {
		for _, r := range line.Runs {
			if r.Face == nil || r.Face.Weight() != text.WeightBold {
				continue
			}
			checked++
			if fdiff(r.Face.Size(), baseSize) > 0.01 {
				t.Fatalf("bold run face size=%.2f, want base %.2f（字号硬编码残留）", r.Face.Size(), baseSize)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no bold run found in layout")
	}
}

// P1-5 段层钉：截断标记 run 的样式必须等于断点 rune 所属 span（Flutter
// 语义：省略号继承被截字符样式），且截断行不得只剩单 run。
func TestParagraph_TruncationEllipsisKeepsStyle(t *testing.T) {
	face, _ := cjkTestFamily(t, 20.0)
	b := NewParagraphBuilder(face, TextStyle{})
	b.PushStyle(TextStyle{Color: RGBA{R: 1, A: 1}})
	b.AddText("红色红色红色")
	b.PopStyle()
	b.PushStyle(TextStyle{Color: RGBA{B: 1, A: 1}})
	b.AddText("蓝色蓝色蓝色")
	b.PopStyle()
	p := b.Build(text.LayoutOptions{MaxWidth: 100, MaxLines: 1, Overflow: text.OverflowEllipsis})
	if len(p.Layout.Lines) != 1 {
		t.Fatalf("lines=%d, want 1", len(p.Layout.Lines))
	}
	line := &p.Layout.Lines[0]
	if len(line.Runs) < 2 {
		t.Fatalf("truncated paragraph line runs=%d, want >=2（前缀 + 标记）", len(line.Runs))
	}
	marker := &line.Runs[len(line.Runs)-1]
	st, ok := p.StyleForRun(marker.Glyphs[0].Cluster)
	if !ok {
		t.Fatal("marker cluster resolves to no style")
	}
	want := p.styles[marker.Style]
	if want.Color != st.Color {
		t.Fatalf("marker style color=%v, cluster-resolved=%v（标记样式与断点 span 不一致）", want.Color, st.Color)
	}
	// 标记 run 的 cluster 必须落在段内 rune 空间且不越全文。
	total := utf8.RuneCountInString("红色红色红色蓝色蓝色蓝色")
	if marker.Glyphs[0].Cluster < 0 || marker.Glyphs[0].Cluster >= total {
		t.Fatalf("marker cluster=%d out of [0,%d)", marker.Glyphs[0].Cluster, total)
	}
}

// P1-6 钉：DrawParagraphStyled 的逐 run SetRGBA 会把 paint.Pattern 置 nil，
// 快照/还原必须连 Pattern 一起回滚——图样画笔环境下调用后不许丢。
type patternStub struct{}

func (patternStub) ColorAt(_, _ float64) RGBA { return Green }

func TestDrawParagraphStyledRestoresPattern(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	face := paragraphTestFace(t)
	b := NewParagraphBuilder(face, TextStyle{})
	b.PushStyle(TextStyle{Color: RGBA{G: 1, A: 1}})
	b.AddText("cyan-ish")
	b.PopStyle()
	p := b.Build(text.LayoutOptions{MaxWidth: 200})

	dc := NewContext(64, 64)
	t.Cleanup(func() { _ = dc.Close() })
	dc.SetFillPattern(patternStub{})
	if dc.paint.Pattern == nil {
		t.Fatal("precondition: SetFillPattern must keep a non-solid pattern on paint")
	}
	dc.DrawParagraphStyled(p, 4, 40)
	if dc.paint.Pattern == nil {
		t.Fatal("DrawParagraphStyled leaked into paint.Pattern (SetRGBA cleared it, restore missed it)")
	}
}
