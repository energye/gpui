package rendering

import (
	"testing"
)

// M1-c红灯:点击定位不得落进簇内.布局"e+音标+x"(nil face每字10px):
// carets为0,1,3,4;在X(1)=10处点击,中点规则先给1,须再吸附到簇首0.
func TestClusterBoundary_ClickNoSplit_M1(t *testing.T) {
	txt := "e\u0301x"
	lay := BuildTextLayout(txt, nil, 14, 0, 1.2)
	var x1 float64
	for _, c := range lay.LineCarets(0) {
		if c.ByteOff == 1 {
			x1 = c.X
		}
	}
	off, _ := lay.GetPositionForOffset(x1, lay.LineTop(0)+1)
	if off == 1 {
		t.Fatalf("点击落进簇内(1),必须吸附到边界")
	}
	if off != 0 {
		t.Fatalf("簇内点击应吸附到簇首0,得%d", off)
	}
}

// M1-c:簇首/簇尾/串尾点击保持原语义(只验不劈簇,不改中点规则).
func TestClusterBoundary_ClickEdges_M1(t *testing.T) {
	txt := "e\u0301x"
	lay := BuildTextLayout(txt, nil, 14, 0, 1.2)
	top := lay.LineTop(0) + 1
	if off, _ := lay.GetPositionForOffset(-5, top); off != 0 {
		t.Fatalf("行首点击应为0,得%d", off)
	}
	tail := lay.LineCarets(0)
	last := tail[len(tail)-1]
	if off, _ := lay.GetPositionForOffset(last.X+50, top); off != last.ByteOff {
		t.Fatalf("行尾点击应为%d,得%d", last.ByteOff, off)
	}
}
