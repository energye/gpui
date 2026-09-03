package rendering

import (
	"testing"
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
