package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func TestClippingCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_CLIPPING_VISUAL") != "1" {
		t.Skip("set GPUI_CLIPPING_VISUAL=1 to run CPU/GPU clipping diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-clipping-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "clipping_cpu.png")
	gpuPath := filepath.Join(outDir, "clipping_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/clipping", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/clipping", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectClippingMetrics(cpuImg)
	gpuMetrics := collectClippingMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range clippingMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_CLIPPING_VISUAL_STRICT") == "1" {
		assertClippingVisualStrict(t, diff, cpuMetrics, gpuMetrics)
	}
}

func collectClippingMetrics(img image.Image) map[string]imagediff.RegionMetric {
	return map[string]imagediff.RegionMetric{
		"circular_clip": imagediff.MeasureRegion(img, image.Rect(50, 50, 250, 250), isColorful),
		"rect_clip":     imagediff.MeasureRegion(img, image.Rect(300, 50, 460, 210), isColorful),
		"rect_clip_leak": imagediff.MeasureRegionXY(img, image.Rect(250, 0, 500, 240), func(x, y int, p imagediff.Pixel) bool {
			inClip := x >= 300 && x < 460 && y >= 50 && y < 210
			return !inClip && isColorful(p)
		}),
		"clip_preserve_star": imagediff.MeasureRegion(img, image.Rect(75, 375, 225, 525), isColorfulOrBlack),
		"nested_clips":       imagediff.MeasureRegion(img, image.Rect(300, 350, 460, 510), isColorfulOrBlack),
		"complex_path":       imagediff.MeasureRegion(img, image.Rect(580, 45, 760, 195), isColorfulOrBlack),
		"round_rect":         imagediff.MeasureRegion(img, image.Rect(50, 600, 250, 740), isColorfulOrBlack),
		"reset_clip_fill": imagediff.MeasureRegion(img, image.Rect(300, 600, 460, 760), func(p imagediff.Pixel) bool {
			return p.R > 170 && p.G > 120 && p.G < 210 && p.B > 35 && p.B < 120
		}),
		"reset_clip_leak": imagediff.MeasureRegionXY(img, image.Rect(250, 550, 500, 800), func(x, y int, p imagediff.Pixel) bool {
			inClip := x >= 300 && x < 460 && y >= 600 && y < 760
			return !inClip && p.R > 170 && p.G > 120 && p.G < 210 && p.B > 35 && p.B < 120
		}),
	}
}

func clippingMetricNames() []string {
	return []string{
		"circular_clip",
		"rect_clip",
		"rect_clip_leak",
		"clip_preserve_star",
		"nested_clips",
		"complex_path",
		"round_rect",
		"reset_clip_fill",
		"reset_clip_leak",
	}
}

func assertClippingVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric) {
	t.Helper()
	if diff.RMSE > 24 || diff.MeanAbs > 4 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range clippingMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		if strings.HasSuffix(name, "_leak") {
			if cpu.Count > 8 || gpu.Count > 8 {
				t.Errorf("region %s leaked pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			}
			continue
		}
		if cpu.Count == 0 || gpu.Count == 0 {
			t.Errorf("region %s missing pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			continue
		}
		ratio := float64(gpu.Count) / float64(cpu.Count)
		if ratio < 0.80 || ratio > 1.20 {
			t.Errorf("region %s pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				name, cpu.Count, gpu.Count, ratio)
		}
		if imagediff.AbsInt(gpu.Bounds.Dx()-cpu.Bounds.Dx()) > 18 || imagediff.AbsInt(gpu.Bounds.Dy()-cpu.Bounds.Dy()) > 18 {
			t.Errorf("region %s bbox differs too much: cpu=%s gpu=%s",
				name, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
		}
	}
}

func isColorful(p imagediff.Pixel) bool {
	return !isNearWhite(p) && !isMostlyBlack(p)
}

func isColorfulOrBlack(p imagediff.Pixel) bool {
	return !isNearWhite(p)
}

func isNearWhite(p imagediff.Pixel) bool {
	return p.R > 245 && p.G > 245 && p.B > 245
}

func isMostlyBlack(p imagediff.Pixel) bool {
	return p.R < 45 && p.G < 45 && p.B < 45
}
