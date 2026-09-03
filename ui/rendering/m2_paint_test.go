package rendering

import (
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/render/text"
)

// M2 红灯测试:以下 API 在实现前不存在,本文件先编译失败(红灯).
// 实现后全部转绿,门禁见 docs/ENGINE_TEXT_SCALE_PLAN.md M2 第 ⑥ 项.

// TestCullRange_Horizontal 锁 M2 第 2 项:横向可见区间计算正确,
// 边界含/不含语义明确,全不可见返回空.覆盖单行超长横滚形态.
func TestCullRange_Horizontal(t *testing.T) {
	lay := BuildTextLayout(strings.Repeat("a", 500), nil, 14, 0, 1.2)
	if lay.LineCount() != 1 {
		t.Fatalf("want 1 line got %d", lay.LineCount())
	}
	glyphs := lay.LineGlyphs(0)
	if len(glyphs) == 0 {
		t.Fatalf("no glyphs")
	}
	lo, hi := cullRangeForWindow(glyphs, 100, 300)
	if lo < 0 || hi > len(glyphs) || lo > hi {
		t.Fatalf("range [%d,%d) out of [0,%d)", lo, hi, len(glyphs))
	}
	if hi-lo == 0 {
		t.Fatalf("visible window [100,300) culled everything")
	}
	// 全不可见:窗口远在文本右侧.
	lo2, hi2 := cullRangeForWindow(glyphs, 1e9, 1e9+300)
	if lo2 != hi2 {
		t.Fatalf("fully invisible window want empty range, got [%d,%d)", lo2, hi2)
	}
	// 全可见:窗口覆盖全文.
	lo3, hi3 := cullRangeForWindow(glyphs, -1000, 1e9)
	if lo3 != 0 || hi3 != len(glyphs) {
		t.Fatalf("full window want [0,%d), got [%d,%d)", len(glyphs), lo3, hi3)
	}
}

// TestCullRange_Vertical 锁 M2 第 2 项纵向:多行下只提交可见行区间,
// 不可见行不得提交.用行高前缀和二分定位,不得遍历全表.
func TestCullRange_Vertical(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("line content here\n")
	}
	lay := BuildTextLayout(b.String(), nil, 14, 0, 1.2)
	if lay.LineCount() < 100 {
		t.Fatalf("want >=100 lines got %d", lay.LineCount())
	}
	y0 := lay.LineTop(50)
	y1 := lay.LineTop(60)
	lo, hi := lay.VisibleLineRange(y0, y1)
	if lo > 50 || hi < 60 {
		t.Fatalf("visible rows [%d,%d) must cover [50,61), y0=%.1f y1=%.1f", lo, hi, y0, y1)
	}
	// 全不可见:视口远在文档下方.
	total := lay.LineTop(lay.LineCount() - 1)
	lo2, hi2 := lay.VisibleLineRange(total+10000, total+20000)
	if lo2 != hi2 {
		t.Fatalf("fully invisible viewport want empty range, got [%d,%d)", lo2, hi2)
	}
}

// TestDamageRows 锁 M2 第 4 项:改第 K 行只报 K;改换行报 K 及之后;
// 回绕模式只报被改段.此处先锁几何部分(纯函数,不依赖管线).
func TestDamageRows(t *testing.T) {
	doc := "aaa\nbbb\nccc\nddd\n"
	lay := BuildTextLayout(doc, nil, 14, 0, 1.2)
	// 改第 1 行(0-based)只报第 1 行.
	r := lay.DamageRectForRows(1, 2)
	y1 := lay.LineTop(1)
	h1 := lay.LineHeight(1)
	if r.Min.Y != y1 || r.Max.Y != y1+h1 {
		t.Fatalf("row 1 damage Y=[%.1f,%.1f) want [%.1f,%.1f)", r.Min.Y, r.Max.Y, y1, y1+h1)
	}
	// 改换行(插入 \n)报 K 及之后:覆盖到文档底.
	r2 := lay.DamageRectForRows(1, lay.LineCount())
	total := lay.LineTop(lay.LineCount() - 1)
	lastH := lay.LineHeight(lay.LineCount() - 1)
	if r2.Max.Y != total+lastH {
		t.Fatalf("newline damage must reach doc bottom %.1f, got %.1f", total+lastH, r2.Max.Y)
	}
}

// TestCompositeBatch_MultiFace 锁 M2 第 1 项:MultiFace 行的字形必须带
// 按 face 分区的批量元数据,每分区的 face 均可独立批量提交
// (Source()!=nil),分区覆盖全部字形且不重叠.否则 Paint 只能逐字
// DrawString,顶点数回到 O(n).
func TestCompositeBatch_MultiFace(t *testing.T) {
	face, _, err := text.LoadMultiFace(14)
	if err != nil || face == nil {
		t.Skipf("no multiface for composite batch test: %v", err)
	}
	lay := BuildTextLayout("Hello世界abc你好", face, 14, 0, 1.2)
	if lay.LineCount() != 1 {
		t.Fatalf("want 1 line got %d", lay.LineCount())
	}
	glyphs := lay.LineGlyphs(0)
	if len(glyphs) == 0 {
		t.Fatalf("no glyphs for multiface line")
	}
	runs := lay.LineGlyphRuns(0)
	if len(runs) == 0 {
		t.Fatalf("no glyph runs for multiface line")
	}
	covered := 0
	for _, r := range runs {
		if r.Start < 0 || r.End > len(glyphs) || r.Start >= r.End {
			t.Fatalf("run [%d,%d) out of [0,%d)", r.Start, r.End, len(glyphs))
		}
		if r.Face == nil || r.Face.Source() == nil {
			t.Fatalf("run [%d,%d) face not batchable (nil or Source nil)", r.Start, r.End)
		}
		covered += r.End - r.Start
	}
	if covered != len(glyphs) {
		t.Fatalf("runs cover %d glyphs, want %d (overlap or gap)", covered, len(glyphs))
	}
}

// TestSubmittedGlyphEstimate 锁 M2 第 ⑥ 项 vertex_count 观测:无 hint
// 时等于全量字形;横向窄窗口与纵向行带均显著小于全量且非零;空文本为零.
func TestSubmittedGlyphEstimate(t *testing.T) {
	empty := NewRenderText("")
	if got := empty.SubmittedGlyphEstimate(); got != 0 {
		t.Fatalf("empty want 0 got %d", got)
	}
	long := NewRenderText(strings.Repeat("a世", 2500))
	long.FontSize = 14
	full := long.SubmittedGlyphEstimate()
	if full < 4000 {
		t.Fatalf("unbounded 5000-rune line want >=4000 glyphs, got %d", full)
	}
	lay := long.TextLayout()
	var w float64
	if g := lay.LineGlyphs(0); len(g) > 0 {
		last := g[len(g)-1]
		w = last.X + last.XAdvance
	}
	long.SetViewportHint(w-300, 300)
	narrow := long.SubmittedGlyphEstimate()
	if narrow <= 0 || narrow >= full {
		t.Fatalf("narrow window want (0,%d), got %d", full, narrow)
	}
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("line content here\n")
	}
	multi := NewRenderText(b.String())
	multi.FontSize = 14
	mFull := multi.SubmittedGlyphEstimate()
	mLay := multi.TextLayout()
	y0 := mLay.LineTop(100)
	vh := mLay.LineHeight(100) * 10
	multi.SetViewportRect(0, 0, y0, vh)
	band := multi.SubmittedGlyphEstimate()
	if band <= 0 || band >= mFull {
		t.Fatalf("row band want (0,%d), got %d", mFull, band)
	}
}

// TestCompositeBatch_RebasedPositions 锁复合批量提交的位置不变性:
// 每个分区变基到自原点提交后,提交坐标+原点偏移必须逐字形等于布局坐标.
// 变基错了,GPU 上各 run 会叠在行首(拉丁压中文),CPU 探针看不出来,
// 故此处逐字形断言,跨脚本混排(拉丁+CJK+阿拉伯RTL+泰文)全覆盖.
func TestCompositeBatch_RebasedPositions(t *testing.T) {
	face, _, err := text.LoadMultiFace(16)
	if err != nil || face == nil {
		t.Skipf("no multiface for rebase test: %v", err)
	}
	line := "Hello世界مرحباโลกabc你好"
	lay := BuildTextLayout(line, face, 16, 0, 1.2)
	if lay.LineCount() != 1 {
		t.Fatalf("want 1 line got %d", lay.LineCount())
	}
	glyphs := lay.LineGlyphs(0)
	runs := lay.LineGlyphRuns(0)
	if len(runs) == 0 {
		t.Fatalf("no runs for mixed-script line")
	}
	for _, r := range runs {
		part := glyphs[r.Start:r.End]
		shifted, off := rebaseGlyphs(part)
		if len(shifted) != len(part) {
			t.Fatalf("run [%d,%d): shifted len %d != %d", r.Start, r.End, len(shifted), len(part))
		}
		if shifted[0].X != 0 {
			t.Fatalf("run [%d,%d): rebased first X=%v want 0 (GPU pen origin)", r.Start, r.End, shifted[0].X)
		}
		for i := range part {
			if shifted[i].X+off != part[i].X {
				t.Fatalf("run [%d,%d) glyph %d: shifted %v + off %v != layout %v",
					r.Start, r.End, i, shifted[i].X, off, part[i].X)
			}
			if shifted[i].GID != part[i].GID || shifted[i].XAdvance != part[i].XAdvance {
				t.Fatalf("run [%d,%d) glyph %d: rebase must not touch GID/advance", r.Start, r.End, i)
			}
		}
	}
}

// BenchmarkPaintLine 锁 M2 第 ⑥ 项:固定可见窗口下,提交代价与总字数
// 无关(O(V)而非 O(n)).N∈{1e3,1e5},窗口恒 400px(约 40 字形),量
// 区间计算+可见字形遍历两步,比值≤2.单步皆 ns 级,另设 10µs 噪声
// 护栏:绝对耗时低于护栏即通过,不判比值(防抖动误杀).
func BenchmarkPaintLine(b *testing.B) {
	timing := func(n int) time.Duration {
		b.Helper()
		line := strings.Repeat("a世", n/2)
		lay := BuildTextLayout(line, nil, 14, 0, 1.2)
		glyphs := lay.LineGlyphs(0)
		// 窗口放在行尾:线性扫描要走完 O(n) 才找得到,二分仍 O(log n),
		// 这样基准能区分新旧实现(窗口放行首则两者都快,锁不住退化).
		var w float64
		if len(glyphs) > 0 {
			last := glyphs[len(glyphs)-1]
			w = last.X + last.XAdvance
		}
		lo, hi := w-500, w-100
		start := time.Now()
		const reps = 20
		var sink float64
		for i := 0; i < reps; i++ {
			s, e := cullRangeForWindow(glyphs, lo, hi)
			if s < 0 || e > len(glyphs) {
				b.Fatalf("range out of bounds")
			}
			// 可见字形提交环(与 Paint 的批量提交同形):窗口固定故
			// 遍历量恒定,总量翻 100 倍也不应变慢(O(V)证据).
			for k := s; k < e; k++ {
				sink += glyphs[k].X
			}
		}
		if sink < 0 {
			b.Fatalf("sink negative")
		}
		return time.Since(start) / reps
	}
	var a, c []time.Duration
	for i := 0; i < 5; i++ {
		a = append(a, timing(1000))
		c = append(c, timing(100000))
	}
	d1e3, d1e5 := m1Median(a), m1Median(c)
	b.Logf("可见窗口提交 1e3=%v 1e5=%v 比值=%.2f", d1e3, d1e5, float64(d1e5)/float64(d1e3))
	if r := float64(d1e5) / float64(d1e3); r > 2 && d1e5 > 10*time.Microsecond {
		b.Fatalf("可见窗口提交疑似随总量增长:1e5/1e3=%.1f,门禁≤2", r)
	}
}
