package rendering

import (
	"strings"
	"testing"
	"time"
)

// TestPaintPerRuneX_NoQuadratic_M1锁M1第13项:逐字绘制分支的byteOff→X
// 必须一次建表+O(1)查,不得随行长二次增长.三档1e3/1e4/1e5 rune行,
// O(n)下T(1e4)/T(1e3)≈10、T(1e5)/T(1e3)≈100;O(n²)下则为100/10000.
// 门禁取≤20/≤300,线性通过、二次必挂.中位数抗抖.
func TestPaintPerRuneX_NoQuadratic_M1(t *testing.T) {
	timing := func(n int) time.Duration {
		t.Helper()
		line := strings.Repeat("a世", n/2)
		lay := BuildTextLayout(line, nil, 14, 0, 1.2)
		if lay.LineCount() != 1 {
			t.Fatalf("want 1 line got %d", lay.LineCount())
		}
		carets := lay.LineCarets(0)
		start := time.Now()
		const reps = 20
		for i := 0; i < reps; i++ {
			xByOff := caretXByOffset(carets)
			sum := 0.0
			for _, c := range carets {
				sum += xByOff[c.ByteOff]
			}
			if sum <= 0 {
				t.Fatalf("lookup sum should be positive")
			}
		}
		return time.Since(start) / reps
	}
	var a, b, c []time.Duration
	for i := 0; i < 5; i++ {
		a = append(a, timing(1000))
		b = append(b, timing(10000))
		c = append(c, timing(100000))
	}
	d1e3, d1e4, d1e5 := m1Median(a), m1Median(b), m1Median(c)
	t.Logf("逐字X查表 1e3=%v 1e4=%v 1e5=%v 比值1e4/1e3=%.2f 1e5/1e3=%.2f",
		d1e3, d1e4, d1e5, float64(d1e4)/float64(d1e3), float64(d1e5)/float64(d1e3))
	if r := float64(d1e4) / float64(d1e3); r > 20 {
		t.Fatalf("逐字X查表疑似二次增长:行×10耗时×%.1f,门禁≤20", r)
	}
	if r := float64(d1e5) / float64(d1e3); r > 300 {
		t.Fatalf("逐字X查表疑似二次增长:行×100耗时×%.1f,门禁≤300", r)
	}
}

// TestCaretMatchesPaint_M1锁I1(查询与绘制同源):CaretForOffset /
// CaretAt / 绘制分支的xByOff三者读同一张caret表,ASCII文档逐偏移
// 偏差必须为0(≤1px门禁).真字形X vs caret X的端到端证据由M0的
// TestPaintUsesShapedX_MultiFaceBulk覆盖,此处只锁M1索引链不断链.
func TestCaretMatchesPaint_M1(t *testing.T) {
	doc := "hello world\nfoo bar baz\nlast line here"
	lay := BuildTextLayout(doc, nil, 14, 0, 1.2)
	for off := 0; off <= len(doc); off++ {
		if off < len(doc) && doc[off] == '\n' {
			continue
		}
		j, x, ok := lay.CaretForOffset(off)
		if !ok {
			t.Fatalf("CaretForOffset(%d) failed", off)
		}
		x2, ok := lay.CaretAt(j, off)
		if !ok {
			t.Fatalf("CaretAt(%d,%d) failed", j, off)
		}
		if x != x2 {
			t.Fatalf("off=%d: CaretForOffset x=%v != CaretAt x=%v", off, x, x2)
		}
		// LineCarets返回绝对偏移拷贝,在其上建表即绘制分支的查表形状.
		xByOff := caretXByOffset(lay.LineCarets(j))
		x3, hit := xByOff[off]
		if !hit || x3 != x {
			t.Fatalf("off=%d: paint查表x=%v(hit=%v) != 查询x=%v", off, x3, hit, x)
		}
	}
}
