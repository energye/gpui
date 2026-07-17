package render

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// Quality probe: circle + diagonal + cubic with MSAA default; dumps PNG for review.
func TestAAProbe_CircleDiag(t *testing.T) {
	if os.Getenv("GPUI_SKIP_GPU") == "1" {
		t.Skip("GPU skipped")
	}
	dc := NewContext(200, 200)
	defer dc.Close()
	dc.SetAntiAlias(true)
	// Clear() is transparent; white bg required for RGB-edge review of AA.
	dc.ClearWithColor(White)
	dc.SetRGBA(0.1, 0.2, 0.9, 1)
	dc.DrawCircle(60, 60, 40)
	_ = dc.Fill()

	dc.SetRGBA(0.9, 0.1, 0.1, 1)
	dc.SetLineWidth(2)
	dc.MoveTo(20, 180)
	dc.LineTo(180, 20)
	_ = dc.Stroke()

	dc.SetLineWidth(0) // hairline
	dc.MoveTo(20, 20)
	dc.LineTo(180, 180)
	_ = dc.Stroke()

	dc.SetLineWidth(1.5)
	dc.SetRGBA(0.1, 0.7, 0.2, 1)
	dc.MoveTo(10, 120)
	dc.CubicTo(50, 80, 90, 160, 130, 120)
	dc.CubicTo(160, 90, 180, 140, 190, 110)
	_ = dc.Stroke()

	out := filepath.Join("..", "tmp", "comp", "AA_PROBE_circle_diag.png")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := dc.SavePNG(out); err != nil {
		t.Fatal(err)
	}
	img := dc.Image()
	soft, hard := 0, 0
	for y := 15; y < 105; y++ {
		for x := 15; x < 105; x++ {
			r16, g16, b16, _ := img.At(x, y).RGBA()
			// composite-ready RGB on white
			rf, gf, bf := float64(r16)/65535, float64(g16)/65535, float64(b16)/65535
			dx := float64(x) + 0.5 - 60
			dy := float64(y) + 0.5 - 60
			d := math.Hypot(dx, dy)
			if math.Abs(d-40) > 2.5 {
				continue
			}
			// blue channel intermediate on white ≈ AA fringe
			if bf > 0.15 && bf < 0.95 && rf < 0.9 {
				soft++
			} else if bf >= 0.95 && d > 39.2 {
				hard++
			}
			_ = gf
		}
	}
	st := dc.RenderPathStats()
	t.Logf("circle edge soft=%d hard=%d gpu_ops=%d cpu_fallback=%d reason=%q wrote=%s",
		soft, hard, st.GPUOps, st.CPUFallbackOps, dc.LastCPUFallbackReason(), out)
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu fallback not allowed: %d reason=%q", st.CPUFallbackOps, dc.LastCPUFallbackReason())
	}
	if soft < 15 {
		t.Fatalf("expected soft AA samples on circle edge, soft=%d hard=%d", soft, hard)
	}
}
