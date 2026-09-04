package rendering

import (
	"testing"
)

// M1-d红灯:增量Update快照必须与直接构建逐字节一致;只重建影响区.

func m1UpdateEquiv(t *testing.T, c *layoutCache, text string, w float64) {
	t.Helper()
	got := c.update(text, nil, 14, w, 1.2, 0.55, 0, TextOverflowClip)
	want := BuildTextLayout(text, nil, 14, w, 1.2)
	if !m1CacheLinesEqual(got, want) {
		t.Fatalf("Update快照不一致(got %d行 want %d行, w=%v text=%q)", got.LineCount(), want.LineCount(), w, text)
	}
}

func TestLayoutUpdate_Equiv_M1(t *testing.T) {
	for _, w := range []float64{0, 300} {
		c := newLayoutCache()
		doc := "aaa\nbbb\nccc"
		m1UpdateEquiv(t, c, doc, w)
		m1UpdateEquiv(t, c, "aaa\nBBXb\nccc", w)   // 行内改字
		m1UpdateEquiv(t, c, "aaa\nBBXb\ncccddd", w) // 末尾追加
		m1UpdateEquiv(t, c, "Xaaa\nBBXb\ncccddd", w) // 首部插入
		m1UpdateEquiv(t, c, "Xaaa\nBB\nXb\ncccddd", w) // 插入换行拆行
		m1UpdateEquiv(t, c, "Xaaa\nBBXb\ncccddd", w)  // 删除换行并行
		m1UpdateEquiv(t, c, "", w)                  // 清空
		m1UpdateEquiv(t, c, "hello", w)             // 从空重建
		m1UpdateEquiv(t, c, "hello", w)             // 同文重建
	}
}

func TestLayoutUpdate_MaxLines_M1(t *testing.T) {
	c := newLayoutCache()
	textLayoutGen.Add(1)
	wantGen := textLayoutGen.Load() + 1
	got := c.update("a\nb\nc\nd", nil, 14, 0, 1.2, 0.55, 2, TextOverflowClip)
	want := BuildTextLayoutEx("a\nb\nc\nd", nil, 14, 0, 1.2, 0.55, 2, TextOverflowClip)
	_ = wantGen
	if !m1CacheLinesEqual(got, want) || got.LineCount() != 2 || !got.Truncated {
		t.Fatalf("截断快照不一致(got %d行 want %d行 trunc=%v)", got.LineCount(), want.LineCount(), got.Truncated)
	}
	if got.MaxLines != 2 || got.Overflow != TextOverflowClip {
		t.Fatalf("截断字段未透传")
	}
}

func TestLayoutUpdate_KOnly_M1(t *testing.T) {
	c := newLayoutCache()
	c.update("aaa\nbbb\nccc\nddd", nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip)
	blds := c.blds
	c.update("aaa\nBBX\nccc\nddd", nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip)
	if got := c.blds - blds; got != 1 {
		t.Fatalf("改1行应只重建1行,实际新建%d", got)
	}
}

// TestLayoutUpdate_TailTops_RV锁行顶增量:行数变化时尾部行顶必须按
// 新增高度顺延,与直接构建逐项一致.回车多一行少顺延即叠字,删行多留即裂缝.
func TestLayoutUpdate_TailTops_RV(t *testing.T) {
	checkTops := func(c *layoutCache, text string, w float64) {
		t.Helper()
		got := c.update(text, nil, 14, w, 1.2, 0.55, 0, TextOverflowClip)
		want := BuildTextLayout(text, nil, 14, w, 1.2)
		if got.LineCount() != want.LineCount() {
			t.Fatalf("行数不一致(got %d want %d text=%q)", got.LineCount(), want.LineCount(), text)
		}
		for i := 0; i < want.LineCount(); i++ {
			if got.LineTop(i) != want.LineTop(i) {
				t.Fatalf("第%d行顶%.2f,直接%.2f(text=%q)", i, got.LineTop(i), want.LineTop(i), text)
			}
		}
	}
	c := newLayoutCache()
	checkTops(c, "aaa\nbbb\nccc", 0)
	checkTops(c, "aaa\n\nbbb\nccc", 0)
	checkTops(c, "aaa\nbbb\nccc", 0)
	checkTops(c, "\naaa\nbbb\nccc", 0)
	checkTops(c, "hello world hello world hello world hello world\nTAIL", 200)
	checkTops(c, "hello\nworld hello world hello world hello world\nTAIL", 200)
}

// TestLineClusters_StaleBounds_NoPanic锁拖选越界:增量快照在定时器线程
// 被读到一半撕裂时,尾行EndByte可能比Text长1字节.查询必须钳制,不得panic.
func TestLineClusters_StaleBounds_NoPanic(t *testing.T) {
	l := &TextLayout{Text: "aaa\nbbb", FontSize: 14, LineSpacing: 1.2}
	lh := lineHeightFor(nil, 14, 1.2)
	l.lines = []TextLayoutLine{
		{StartByte: 0, EndByte: 3, Height: lh, Carets: []GlyphCaret{{ByteOff: 0, X: 0}, {ByteOff: 3, X: 30}}},
		{StartByte: 4, EndByte: 8, Height: lh, Carets: []GlyphCaret{{ByteOff: 0, X: 0}, {ByteOff: 3, X: 30}}},
	}
	off, _ := l.GetPositionForOffset(1000, 1000)
	if off < 0 || off > len(l.Text) {
		t.Fatalf("越界快照查询结果未钳制:off=%d len=%d", off, len(l.Text))
	}
	if got := l.lineClusters(1); len(got) == 0 {
		t.Fatalf("钳制后簇表为空")
	}
}

func TestLayoutUpdate_Generation_M1(t *testing.T) {
	c := newLayoutCache()
	l1 := c.update("aaa", nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip)
	l2 := c.update("aaa", nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip)
	if l2.Generation != l1.Generation+1 {
		t.Fatalf("同文重建Generation也必须自增")
	}
	if len(l1.LineGen) != 1 || len(l2.LineGen) != 1 || l2.LineGen[0] != l1.LineGen[0] {
		t.Fatalf("内容未变行标记应复用")
	}
}

// TestPatchIndex_InPlace_RV锁收敛:行数不变的增量更新,索引数组必须原地
// 改(零分配),且值与重建逐项一致.指针不变是优化存在的证据,值一致是
// 正确性的证据,缺一即回退重建.
func TestPatchIndex_InPlace_RV(t *testing.T) {
	c := newLayoutCache()
	doc := "aaa\nbbb\nccc\nddd\neee"
	l1 := c.update(doc, nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip)
	before := c.live.idx
	n0 := before.n
	doc2 := "aaa\nBBXb\nccc\nddd\neee"
	l2 := c.update(doc2, nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip)
	if c.live.idx != before {
		t.Fatalf("行数不变应原地改索引,对象被换了")
	}
	if c.live.idx.n != n0 {
		t.Fatalf("行数应不变:%d vs %d", c.live.idx.n, n0)
	}
	want := BuildTextLayout(doc2, nil, 14, 0, 1.2)
	if !m1CacheLinesEqual(l2, want) {
		t.Fatalf("原地索引下快照与直接构建不一致")
	}
	for i := 0; i < want.LineCount(); i++ {
		if l2.LineTop(i) != want.LineTop(i) {
			t.Fatalf("第%d行顶%.2f,直接%.2f", i, l2.LineTop(i), want.LineTop(i))
		}
	}
	// 行数变化仍走重建:对象换新,值正确.
	doc3 := "aaa\nBBXb\nccc\nNEW\nddd\neee"
	l3 := c.update(doc3, nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip)
	if c.live.idx == before {
		t.Fatalf("行数变化应重建索引,对象没换")
	}
	want3 := BuildTextLayout(doc3, nil, 14, 0, 1.2)
	if !m1CacheLinesEqual(l3, want3) {
		t.Fatalf("重建索引下快照与直接构建不一致")
	}
	_ = l1
}
