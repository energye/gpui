package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func TestGPUExampleCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_GPU_EXAMPLE_VISUAL") != "1" {
		t.Skip("set GPUI_GPU_EXAMPLE_VISUAL=1 to run CPU/GPU gpu example diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-gpu-example-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "gpu_example_cpu.png")
	gpuPath := filepath.Join(outDir, "gpu_example_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/gpu", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/gpu", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectGPUExampleMetrics(cpuImg)
	gpuMetrics := collectGPUExampleMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range gpuExampleMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_GPU_EXAMPLE_VISUAL_STRICT") == "1" {
		RequireGPUPathStats(t, gpuLog, true)
		assertGPUExampleVisualStrict(t, diff, cpuMetrics, gpuMetrics)
	}
}

func collectGPUExampleMetrics(img image.Image) map[string]imagediff.RegionMetric {
	return map[string]imagediff.RegionMetric{
		"circle_red": imagediff.MeasureRegion(img, image.Rect(45, 45, 155, 155), func(p imagediff.Pixel) bool {
			return p.R > 160 && p.G < 120 && p.B < 120
		}),
		"circle_green": imagediff.MeasureRegion(img, image.Rect(195, 45, 305, 155), func(p imagediff.Pixel) bool {
			return p.G > 160 && p.R < 120 && p.B < 120
		}),
		"rrect": imagediff.MeasureRegion(img, image.Rect(45, 195, 255, 325), func(p imagediff.Pixel) bool {
			return p.B > 140 && p.R < 120 && p.G < 160
		}),
		"stroke_circle": imagediff.MeasureRegion(img, image.Rect(335, 195, 465, 325), func(p imagediff.Pixel) bool {
			return p.G > 140 && p.B > 140 && p.R < 120
		}),
		"triangle": imagediff.MeasureRegion(img, image.Rect(495, 175, 705, 330), func(p imagediff.Pixel) bool {
			return p.R > 160 && p.G > 100 && p.B < 100
		}),
		"pentagon": imagediff.MeasureRegion(img, image.Rect(85, 365, 215, 495), func(p imagediff.Pixel) bool {
			return p.G > 140 && p.R < 120 && p.B < 150
		}),
		"hexagon": imagediff.MeasureRegion(img, image.Rect(290, 370, 410, 495), func(p imagediff.Pixel) bool {
			return p.R > 140 && p.B > 100 && p.G < 120
		}),
		"star": imagediff.MeasureRegion(img, image.Rect(480, 360, 620, 500), func(p imagediff.Pixel) bool {
			return p.R > 160 && p.G > 120 && p.B < 100
		}),
		"curve": imagediff.MeasureRegion(img, image.Rect(560, 360, 740, 510), func(p imagediff.Pixel) bool {
			return p.B > 120 && p.R > 60 && p.R < 160 && p.G < 120
		}),
	}
}

func gpuExampleMetricNames() []string {
	return []string{"circle_red", "circle_green", "rrect", "stroke_circle", "triangle", "pentagon", "hexagon", "star", "curve"}
}

func assertGPUExampleVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric) {
	t.Helper()
	if diff.RMSE > 24 || diff.MeanAbs > 4 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range gpuExampleMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		if cpu.Count == 0 || gpu.Count == 0 {
			t.Errorf("region %s missing pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			continue
		}
		ratio := float64(gpu.Count) / float64(cpu.Count)
		if ratio < 0.75 || ratio > 1.25 {
			t.Errorf("region %s pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				name, cpu.Count, gpu.Count, ratio)
		}
	}
}
