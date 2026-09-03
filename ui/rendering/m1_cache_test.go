package rendering

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render/text"
)

// M1-b:双模式布局缓存 + 按行失效标记(I9).

func m1CacheLinesEqual(a, b *TextLayout) bool {
	if a.LineCount() != b.LineCount() {
		return false
	}
	for i := 0; i < a.LineCount(); i++ {
		as, ae, aw, ah, _ := a.Line(i)
		bs, be, bw, bh, _ := b.Line(i)
		if as != bs || ae != be || aw != bw || ah != bh {
			return false
		}
		ac, bc := a.LineCarets(i), b.LineCarets(i)
		if len(ac) != len(bc) {
			return false
		}
		for j := range ac {
			if ac[j] != bc[j] {
				return false
			}
		}
	}
	return true
}

func TestLayoutCache_Equiv_M1(t *testing.T) {
	docs := []struct {
		name string
		text string
		w    float64
	}{
		{"empty", "", 0},
		{"single", "Hello world", 0},
		{"multi", "Hello\nGo语言\nab\nLast", 0},
		{"trailingNL", "a\n", 0},
		{"wrapEst", "The quick brown fox jumps over the lazy dog and then keeps running far", 300},
		{"wrapEstMulti", "The quick brown fox jumps\nover the lazy dog and then keeps running far away", 300},
		{"crlf", "a\r\nb\r\nc", 0},
	}
	for _, d := range docs {
		c := newLayoutCache()
		got := c.buildCached(d.text, nil, 14, d.w, 1.2)
		want := BuildTextLayout(d.text, nil, 14, d.w, 1.2)
		if !m1CacheLinesEqual(got, want) {
			t.Fatalf("%s:缓存构建与直接构建行不一致(got %d行 want %d行)", d.name, got.LineCount(), want.LineCount())
		}
	}
}

func TestLayoutCache_NoWrap_KOnly_M1(t *testing.T) {
	c := newLayoutCache()
	l1 := c.buildCached("aaa\nbbb\nccc\nddd", nil, 14, 0, 1.2)
	blds1, hits1 := c.blds, c.hits
	if blds1 != 4 || hits1 != 0 {
		t.Fatalf("首次构建应全miss: blds=%d hits=%d", blds1, hits1)
	}
	l2 := c.buildCached("aaa\nBBX\nccc\nddd", nil, 14, 0, 1.2)
	if got := c.blds - blds1; got != 1 {
		t.Fatalf("改1行应只重建1行,实际新建%d", got)
	}
	if c.hits-hits1 != 3 {
		t.Fatalf("其余3行应命中,实际命中%d", c.hits-hits1)
	}
	if l2.Generation != l1.Generation+1 {
		t.Fatalf("Generation应全局自增: %d→%d", l1.Generation, l2.Generation)
	}
	for i := 0; i < l2.LineCount(); i++ {
		if i == 1 {
			if l2.LineGen[i] != l2.Generation {
				t.Fatalf("被改行标记应为新Generation")
			}
			continue
		}
		if l2.LineGen[i] != l1.LineGen[i] {
			t.Fatalf("未改行%d标记应保留: %d vs %d", i, l2.LineGen[i], l1.LineGen[i])
		}
	}
}

func TestLayoutCache_Wrap_SegIsolation_M1(t *testing.T) {
	c := newLayoutCache()
	d1 := "The quick brown fox jumps over the lazy dog\nsecond paragraph stays exactly the same here\nthird one too"
	l1 := c.buildCached(d1, nil, 14, 300, 1.2)
	blds1 := c.blds
	d2 := "CHANGED first paragraph text here\nsecond paragraph stays exactly the same here\nthird one too"
	l2 := c.buildCached(d2, nil, 14, 300, 1.2)
	if got := c.blds - blds1; got != 1 {
		t.Fatalf("改首段应只重建1段,实际新建%d", got)
	}
	n1, n2 := l1.LineCount(), l2.LineCount()
	if n1 == 0 || n2 == 0 {
		t.Fatalf("回绕行不应为空")
	}
	// 尾段未改:其首行标记应保留.
	if l2.LineGen[n2-1] != l1.LineGen[n1-1] {
		t.Fatalf("未改尾段标记应保留: %d vs %d", l2.LineGen[n2-1], l1.LineGen[n1-1])
	}
}

func TestGeneration_PerLine_M1(t *testing.T) {
	c := newLayoutCache()
	l1 := c.buildCached("aaa\nbbb", nil, 14, 0, 1.2)
	l2 := c.buildCached("aaa\nbbb", nil, 14, 0, 1.2)
	if l2.Generation <= l1.Generation {
		t.Fatalf("Generation必须全局自增")
	}
	for i := range l2.LineGen {
		if l2.LineGen[i] != l1.LineGen[i] {
			t.Fatalf("内容未变时行标记应复用")
		}
	}
}

// TestLayoutCache_Bounded_RV锁收敛:跨文档缓存条目有上限,超限清表后
// 构建仍正确(只丢命中率,不丢正确性).
func TestLayoutCache_Bounded_RV(t *testing.T) {
	c := newLayoutCache()
	for i := 0; i < maxCacheEntries+100; i++ {
		doc := strings.Repeat("x", 8) + strings.Repeat("y", i%64) + "#"
		c.buildCached(doc, nil, 14, 0, 1.2)
	}
	if len(c.rows) > maxCacheEntries {
		t.Fatalf("行缓存%d条超上限%d", len(c.rows), maxCacheEntries)
	}
	got := c.buildCached("aaa\nbbb", nil, 14, 0, 1.2)
	want := BuildTextLayout("aaa\nbbb", nil, 14, 0, 1.2)
	if !m1CacheLinesEqual(got, want) {
		t.Fatalf("清表后构建不一致")
	}
}

// TestLayoutCache_EstWidthKey_RV锁B1:无脸估算回绕的 approxCharW 必须进
// 缓存键.同文本同字号同宽、字宽因子不同,断行必须不同,且第二次不得命中
// 第一次的行.
func TestLayoutCache_EstWidthKey_RV(t *testing.T) {
	const doc = "The quick brown fox jumps over the lazy dog and then keeps running far away indeed"
	c := newLayoutCache()
	_, _ = c.cachedLinesFull(doc, nil, 14, 300, 1.2, 0.55, 1)
	b, _ := c.cachedLinesFull(doc, nil, 14, 300, 1.2, 1.0, 2)
	direct := BuildTextLayoutEx(doc, nil, 14, 300, 1.2, 1.0, 0, TextOverflowClip)
	if len(b) != direct.LineCount() {
		t.Fatalf("1.0二次命中行数%d,与直接构建%d不一致(撞了0.55的旧行)", len(b), direct.LineCount())
	}
	for i := range b {
		_, _, ww, _, _ := direct.Line(i)
		if b[i].Width != ww {
			t.Fatalf("第%d行二次命中宽%.1f,直接构建宽%.1f(撞了旧字宽的行)", i, b[i].Width, ww)
		}
	}
}

// TestLayoutCache_CRWrapEquiv_RV锁B2:含回车的文本,回绕模式下缓存构建
// 必须与直接构建一致(直接构建走 WrapText 的回车归一,缓存不得跳过回绕).
func TestLayoutCache_CRWrapEquiv_RV(t *testing.T) {
	face, _, err := text.LoadMultiFace(14)
	if err != nil || face == nil {
		t.Skipf("no face for CR wrap test: %v", err)
	}
	docs := []string{
		"aaa\r\nbbb\r\nccc ddd eee fff ggg hhh iii jjj kkk",
		"aaa\rbbb ccc ddd eee fff ggg hhh iii jjj kkk lll",
	}
	for _, doc := range docs {
		c := newLayoutCache()
		gotL, _ := c.cachedLinesFull(doc, face, 14, 300, 1.2, 0.55, 1)
		want := BuildTextLayoutEx(doc, face, 14, 300, 1.2, 0.55, 0, TextOverflowClip)
		if len(gotL) != want.LineCount() {
			t.Fatalf("%q:缓存%d行,直接%d行(含回车不得跳过回绕)", doc, len(gotL), want.LineCount())
		}
		for i := range gotL {
			ws, we, _, _, _ := want.Line(i)
			if gotL[i].StartByte != ws || gotL[i].EndByte != we {
				t.Fatalf("%q:第%d行区间缓存[%d,%d)直接[%d,%d)", doc, i, gotL[i].StartByte, gotL[i].EndByte, ws, we)
			}
		}
	}
}
