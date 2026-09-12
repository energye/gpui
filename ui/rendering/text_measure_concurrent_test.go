package rendering

import (
	"sync"
	"testing"
)

// Concurrent measure/invalidate must not fatal the runtime (concurrent map
// read and map write): layout runs on the UI thread while paint/record
// measure on the raster thread, and resize storms make the two collide.
func TestMeasureCache_Concurrent(t *testing.T) {
	rt := NewRenderText("bHello你好a世界bHello你好a世界bHello你好a世界")
	lines := []string{
		"bHello你好a世界bHello",
		"你好a世界bHello你好",
		"a世界bHello你好a世界",
		"TAIL尾巴",
		"short",
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				_ = rt.measureLine(lines[(i+g)%len(lines)])
				if i%37 == 0 {
					rt.invalidateMeasureCache()
				}
				if i%53 == 0 {
					_, _ = rt.MeasureCacheStats()
				}
			}
		}(g)
	}
	wg.Wait()
	h, m := rt.MeasureCacheStats()
	if h+m == 0 {
		t.Fatal("expected some measure activity")
	}
}
