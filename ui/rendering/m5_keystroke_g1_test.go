package rendering

import (
	"testing"
	"time"
)

// TestKeystrokeRatio_M5是G1总收口门禁:BenchmarkKeystroke口径下
// T(1e6)/T(1e3) ≤ 1.5(硬目标).段长分布沿用M1口径:短段/长段分别测,
// WrapLong基线取5e3单个完整段(与M1同口径,测段隔离).
func TestKeystrokeRatio_M5(t *testing.T) {
	modes := []struct {
		name    string
		w       float64
		perLine int
		baseN   int
	}{
		{"NoWrap", 0, 36, 1000},
		{"WrapShort", 300, 36, 1000},
		{"WrapLong", 300, 5000, 5000},
	}
	for _, m := range modes {
		var a, c []time.Duration
		for i := 0; i < 5; i++ {
			a = append(a, m1TimeKeystroke(t, m.baseN, m.perLine, m.w))
			c = append(c, m1TimeKeystroke(t, 1000000, m.perLine, m.w))
		}
		dBase, d1e6 := m1Median(a), m1Median(c)
		t.Logf("G1击键%s: base=%v 1e6=%v 比值1e6/base=%.2f", m.name, dBase, d1e6, float64(d1e6)/float64(dBase))
		if r := float64(d1e6) / float64(dBase); r > 1.5 {
			t.Fatalf("G1未达标%s:文本×%d耗时×%.1f,门禁≤1.5", m.name, 1000000/m.baseN, r)
		}
	}
}
