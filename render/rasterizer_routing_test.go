package render

// 诊断测试（调查批 4f）：RasterizerAuto 是否真的把路径路由到注册的
// CoverageFiller。用 spy filler 观察调用；跑完可与批 4f 评估一起保留或删除。

import (
	"sync"
	"testing"
)

// spyFiller records FillCoverage invocations.
type spyFiller struct {
	mu    sync.Mutex
	calls []int // verb counts of routed paths
}

func (s *spyFiller) FillCoverage(path *Path, width, height int, fillRule FillRule,
	callback func(x, y int, coverage uint8)) {
	println("spyFiller CALLED verbs=", path.NumVerbs())
	s.mu.Lock()
	s.calls = append(s.calls, path.NumVerbs())
	s.mu.Unlock()
}

func TestRasterizerAutoRoutingSpy(t *testing.T) {
	spy := &spyFiller{}
	prev := GetCoverageFiller()
	RegisterCoverageFiller(spy)
	defer RegisterCoverageFiller(prev) // restore
	// 同进程先跑的 accelerator 测试会留 mock 全局状态：Fill 被其接单后
	// 永远到不了 CPU 路由层。清掉 accelerator（并还原）保证本测试钉的是
	// CPU 光栅化路由本身。
	prevAccel := Accelerator()
	CloseAccelerator()
	defer func() {
		if prevAccel != nil {
			_ = RegisterAccelerator(prevAccel)
		}
	}()

	dc := NewContext(64, 64)
	defer func() { _ = dc.Close() }()
	dc.SetRasterizerMode(RasterizerAuto)
	dc.SetRGBA(0, 0, 0, 1)
	// shouldUseTileRasterizer 的元素阈值：bbox 31×48 → adaptiveThreshold≈53，
	// 需要 >53 个元素才路由。锯齿多段线 60 段。
	dc.MoveTo(8, 8)
	for i := 1; i <= 60; i++ {
		dc.LineTo(8+48*float64(i%2), 8+float64(i))
	}
	dc.ClosePath()
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.calls) == 0 {
		// 路由诊断：同款 path 直接过 shouldUseTileRasterizer。
		p := NewPath()
		p.MoveTo(8, 8)
		for i := 1; i <= 60; i++ {
			p.LineTo(8+48*float64(i%2), 8+float64(i))
		}
		p.Close()
		x1, y1, x2, y2 := pathBounds(p)
		t.Logf("diag: verbs=%d bbox=(%.1f,%.1f)-(%.1f,%.1f) area=%.0f shouldTile=%v nested=%v",
			p.NumVerbs(), x1, y1, x2, y2, (x2-x1)*(y2-y1),
			shouldUseTileRasterizer(p), hasNestedContours(p))
		t.Fatalf("RasterizerAuto did NOT route the complex path to the registered CoverageFiller")
	}
	t.Logf("routed to CoverageFiller: calls=%d verbs=%v", len(spy.calls), spy.calls)
}
