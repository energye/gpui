package render_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/render"
)

// TestDeviceScale_ConcurrentReadWrite locks the R1-3 fix: the raster thread
// rewrites the scale at the present boundary (SetDeviceScale) while UI-side
// readers (snapshot path, diagnostics) load it concurrently. Plain float64
// was a -race report. Run with -race.
func TestDeviceScale_ConcurrentReadWrite(t *testing.T) {
	dc := render.NewContext(64, 64)
	defer func() { _ = dc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = dc.DeviceScale()
				_ = dc.PixelWidth()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			dc.SetDeviceScale(1.0 + float64(j%3)*0.5)
		}
	}()
	wg.Wait()
	if got := dc.DeviceScale(); got != 1.0 && got != 1.5 && got != 2.0 {
		t.Fatalf("scale=%v want one of the stored values", got)
	}
}
