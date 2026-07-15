// mem_window_stress: long-running offscreen complex present churn for manual soak.
// Uses same scene ideas as TestMem_T3. For window path use go test TestMem_T4_*.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		// Prefer in-repo native lib when unset.
		if _, err := os.Stat("lib/libwgpu_native.so"); err == nil {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
		}
	}
	n := 120
	if v := os.Getenv("GPUI_MEM_ITERS"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}
	sizes := [][2]int{{320, 200}, {480, 270}, {400, 300}, {256, 256}, {640, 360}}
	t0 := time.Now()
	for i := 0; i < n; i++ {
		w, h := sizes[i%len(sizes)][0], sizes[i%len(sizes)][1]
		dc := render.NewContext(w, h)
		dc.SetRGB(0.12+float64(i%5)*0.02, 0.14, 0.18)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
		for k := 0; k < 8; k++ {
			dc.SetRGB(0.2, 0.5, 0.9)
			dc.DrawRoundedRectangle(float64(10+k*40), 20, 36, 80, 6)
			_ = dc.Fill()
		}
		dc.PushLayer(render.BlendNormal, 0.9)
		dc.SetRGB(1, 0.4, 0.2)
		dc.DrawCircle(float64(w)/2, float64(h)/2, 40)
		_ = dc.Fill()
		dc.PopLayer()
		view, rel := dc.CreateOffscreenTexture(w, h)
		if rel == nil {
			fmt.Println("offscreen unavailable")
			dc.Close()
			os.Exit(1)
		}
		if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
			rel()
			dc.Close()
			fmt.Println("present:", err)
			os.Exit(1)
		}
		rel()
		dc.Close()
		if i%20 == 19 {
			fmt.Printf("iter %d/%d elapsed=%s\n", i+1, n, time.Since(t0).Round(time.Millisecond))
		}
	}
	fmt.Println("ok", n, "iters in", time.Since(t0))
}
