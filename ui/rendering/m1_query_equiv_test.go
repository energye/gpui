package rendering

import (
	"testing"
)

// M1-a行为锁:下面查询改写(O(n²)→O(log n))前后输出必须逐值一致.
// 先在老实现上跑绿,改完再跑绿.阈值与老测试对齐(caret往返≤0.5px量级).

func m1EquivDoc() (string, *TextLayout) {
	txt := "Hello world\nGo语言排版\nab\nLast line here"
	return txt, BuildTextLayout(txt, nil, 14, 0, 1.2)
}

func TestM1_CaretForOffset_Lines_M1(t *testing.T) {
	txt, lay := m1EquivDoc()
	if lay.LineCount() != 4 {
		t.Fatalf("want 4 lines got %d", lay.LineCount())
	}
	for i := 0; i < lay.LineCount(); i++ {
		s, _, _, _, _ := lay.Line(i)
		gotLine, x, ok := lay.CaretForOffset(s)
		if !ok || gotLine != i {
			t.Fatalf("行%d StartByte=%d 查回行=%d ok=%v", i, s, gotLine, ok)
		}
		if x != lay.LineCarets(i)[0].X {
			t.Fatalf("行%d X=%v want %v", i, x, lay.LineCarets(i)[0].X)
		}
	}
	if _, x, ok := lay.CaretForOffset(0); !ok || x != 0 {
		t.Fatalf("off=0 应为行0 X=0,得 x=%v ok=%v", x, ok)
	}
	if gotLine, _, ok := lay.CaretForOffset(len(txt)); !ok || gotLine != 3 {
		t.Fatalf("文末应落末行,得行=%d ok=%v", gotLine, ok)
	}
}

func TestM1_BoxesForRange_Span_M1(t *testing.T) {
	_, lay := m1EquivDoc()
	l0s, _, _, _, _ := lay.Line(0)
	_, l1e, _, _, _ := lay.Line(1)
	boxes := lay.BoxesForRange(l0s, l1e)
	if len(boxes) != 2 {
		t.Fatalf("跨两行应返回2个盒,得%d", len(boxes))
	}
	if boxes[0].Min.X != lay.LineCarets(0)[0].X || boxes[1].Max.X-boxes[1].Min.X <= 0 {
		t.Fatalf("盒坐标异常:%+v", boxes)
	}
	if out := lay.BoxesForRange(5, 5); out != nil {
		t.Fatalf("空区间应返nil,得%v", out)
	}
}

func TestM1_NewAPIs_M1(t *testing.T) {
	_, lay := m1EquivDoc()
	if got := lay.LineCount(); got != 4 {
		t.Fatalf("LineCount=%d want 4", got)
	}
	s, e, w, h, ok := lay.Line(1)
	if !ok || s < 0 || e <= s || w <= 0 || h <= 0 {
		t.Fatalf("Line(1)=(%d,%d,%v,%v,%v) 行几何异常", s, e, w, h, ok)
	}
	if _, _, _, _, ok := lay.Line(9); ok {
		t.Fatalf("越界行应返ok=false")
	}
	x, ok := func() (float64, bool) {
		s2, _, _, _, ok := lay.Line(2)
		if !ok {
			return 0, false
		}
		return lay.CaretAt(2, s2)
	}()
	if !ok {
		t.Fatalf("CaretAt行首应命中")
	}
	_ = x
	if _, ok := lay.CaretAt(9, 0); ok {
		t.Fatalf("越界行CaretAt应返false")
	}
	if _, ok := lay.CaretAt(0, -3); ok {
		t.Fatalf("行内miss应返false")
	}
}

func TestM1_LineCaretsGlyphs_M1(t *testing.T) {
	_, lay := m1EquivDoc()
	for i := 0; i < lay.LineCount(); i++ {
		cs := lay.LineCarets(i)
		if len(cs) == 0 {
			t.Fatalf("行%d caret表不应空", i)
		}
		s, e, _, _, _ := lay.Line(i)
		if cs[0].ByteOff != s || cs[len(cs)-1].ByteOff != e {
			t.Fatalf("行%d caret首尾应与行区间对齐", i)
		}
		_ = lay.LineGlyphs(i)
	}
	if lay.LineCarets(9) != nil || lay.LineGlyphs(-1) != nil {
		t.Fatalf("越界应返nil")
	}
}

func TestM1_GetPositionForOffset_Edges_M1(t *testing.T) {
	_, lay := m1EquivDoc()
	lnStart, _, _, _, _ := lay.Line(2) // "ab"
	top := lay.LineTop(2)
	off, _ := lay.GetPositionForOffset(-5, top+1)
	if off != lnStart {
		t.Fatalf("x<0应落行首,得%d want %d", off, lnStart)
	}
	last := lay.LineCarets(2)
	lastX, lastOff := last[len(last)-1].X, last[len(last)-1].ByteOff
	off, _ = lay.GetPositionForOffset(lastX+50, top+1)
	if off != lastOff {
		t.Fatalf("x超宽应落行尾,得%d want %d", off, lastOff)
	}
	y := lay.LineTop(1) + lay.LineHeight(1)/2
	off, _ = lay.GetPositionForOffset(10000, y)
	tail := lay.LineCarets(1)
	if off != tail[len(tail)-1].ByteOff {
		t.Fatalf("第1行超宽点击应落该行尾,得%d", off)
	}
}
