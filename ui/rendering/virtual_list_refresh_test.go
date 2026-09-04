package rendering_test

// M4.1 红灯测试:VirtualList 增量前缀刷新(ENGINE_TEXT_SCALE_PLAN M4 hitch 根因).
// 全量重建 O(n)+8MB 分配是 60s 窗 hitch 8–12 的唯一主因(诊断模式关刷新即归 0);
// RefreshExtents 只重读变化区间 + 后缀平移,零分配.被测 API 尚不存在,先红.

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

func refreshFixture(n int) ([]float64, rendering.ItemExtentFunc) {
	h := make([]float64, n)
	for i := range h {
		h[i] = 20
	}
	return h, func(i int) float64 {
		if i < 0 {
			i = 0
		}
		if i >= n {
			i = n - 1
		}
		return h[i]
	}
}

func refreshList(h []float64, fn rendering.ItemExtentFunc) *rendering.VirtualList {
	return rendering.NewVariableVirtualList(len(h), 20, fn, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(100, 20, 0.3, 0.3, 0.35, 1)
	})
}

// 刷新后几何与全量重建逐项一致(总量 + 抽样偏移 + 逆查).
func TestRefreshExtents_MatchesFullRebuild(t *testing.T) {
	const n = 10000
	h, fn := refreshFixture(n)
	list := refreshList(h, fn)
	if list.ContentHeight() != float64(n)*20 {
		t.Fatalf("base total=%v", list.ContentHeight())
	}
	// 模拟懒实测:区间内零星折行(区间外一律不动——范围语义,调用方负责覆盖).
	for _, i := range []int{95, 100, 101, 500, 501, 502} {
		h[i] = 34
	}
	if !list.RefreshExtents(90, 600) {
		t.Fatal("RefreshExtents reported no change, want changed")
	}
	refH, refFn := refreshFixture(n)
	copy(refH, h)
	ref := refreshList(refH, refFn)
	if list.ContentHeight() != ref.ContentHeight() {
		t.Fatalf("total=%v want %v", list.ContentHeight(), ref.ContentHeight())
	}
	for _, i := range []int{0, 89, 90, 95, 100, 502, 599, 600, 601, 9999} {
		if a, b := list.OffsetOfIndex(i), ref.OffsetOfIndex(i); a != b {
			t.Fatalf("offset[%d]=%v want %v", i, a, b)
		}
		if back := list.IndexAtOffset(list.OffsetOfIndex(i) + 0.5); back != i {
			t.Fatalf("row %d inverts to %d", i, back)
		}
	}
}

// 高度无变化时返回 false 且不动前缀(纯估算行实测命中,不值得重发).
func TestRefreshExtents_NoChangeNoWork(t *testing.T) {
	const n = 5000
	h, fn := refreshFixture(n)
	list := refreshList(h, fn)
	before := list.ContentHeight()
	if list.RefreshExtents(100, 200) {
		t.Fatal("RefreshExtents reported change on identical heights")
	}
	if list.ContentHeight() != before {
		t.Fatalf("total moved %v -> %v", before, list.ContentHeight())
	}
}

// 越界区间钳制不断言不崩,且区间外折行同样被后缀平移覆盖.
func TestRefreshExtents_Clamp(t *testing.T) {
	const n = 2000
	h, fn := refreshFixture(n)
	list := refreshList(h, fn)
	h[1999] = 48
	if !list.RefreshExtents(-100, n+5000) {
		t.Fatal("RefreshExtents reported no change, want changed")
	}
	if want := float64(n)*20 + 28; list.ContentHeight() != want {
		t.Fatalf("total=%v want %v", list.ContentHeight(), want)
	}
	if back := list.IndexAtOffset(list.ContentHeight() - 0.5); back != n-1 {
		t.Fatalf("last row inverts to %d", back)
	}
}

// 刷新代价与总行数无关:同 60 行区间,1e6 行表的刷新耗时/全量重建 ≤ 0.2.
func BenchmarkRefreshExtents(b *testing.B) {
	const n = 1000000
	h, fn := refreshFixture(n)
	list := refreshList(h, fn)
	if list.ContentHeight() != float64(n)*20 {
		b.Fatalf("base total=%v", list.ContentHeight())
	}
	h[500000] = 34
	b.Run("refresh", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			h[500010] = 20 + float64(i%2)*14
			list.RefreshExtents(500000, 500060)
		}
	})
	b.Run("full", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			h[500010] = 20 + float64(i%2)*14
			list.InvalidateExtents()
			_ = list.ContentHeight()
		}
	})
}
