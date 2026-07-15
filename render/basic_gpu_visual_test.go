package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func TestBasicCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_BASIC_VISUAL") != "1" {
		t.Skip("set GPUI_BASIC_VISUAL=1 to run CPU/GPU basic diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-basic-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "basic_cpu.png")
	gpuPath := filepath.Join(outDir, "basic_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/basic", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/basic", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectBasicMetrics(cpuImg)
	gpuMetrics := collectBasicMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range []string{"red_circle", "blue_rect", "green_stroke"} {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_BASIC_VISUAL_STRICT") == "1" {
		RequireGPUPathStats(t, gpuLog, true)
		assertBasicVisualStrict(t, diff, cpuMetrics, gpuMetrics)
	}
}

func collectBasicMetrics(img image.Image) map[string]imagediff.RegionMetric {
	all := img.Bounds()
	return map[string]imagediff.RegionMetric{
		"red_circle": imagediff.MeasureRegion(img, all, func(p imagediff.Pixel) bool {
			return p.R > 180 && p.G < 80 && p.B < 80
		}),
		"blue_rect": imagediff.MeasureRegion(img, all, func(p imagediff.Pixel) bool {
			return p.B > 180 && p.R < 80 && p.G < 80
		}),
		"green_stroke": imagediff.MeasureRegion(img, all, func(p imagediff.Pixel) bool {
			return p.G > 180 && p.R < 80 && p.B < 80
		}),
	}
}

func assertBasicVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric) {
	t.Helper()
	if diff.RMSE > 18 || diff.MeanAbs > 3 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range []string{"red_circle", "blue_rect", "green_stroke"} {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		if cpu.Count == 0 || gpu.Count == 0 {
			t.Errorf("region %s missing pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			continue
		}
		ratio := float64(gpu.Count) / float64(cpu.Count)
		if ratio < 0.85 || ratio > 1.15 {
			t.Errorf("region %s pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				name, cpu.Count, gpu.Count, ratio)
		}
		if imagediff.AbsInt(gpu.Bounds.Dx()-cpu.Bounds.Dx()) > 8 || imagediff.AbsInt(gpu.Bounds.Dy()-cpu.Bounds.Dy()) > 8 {
			t.Errorf("region %s bbox differs too much: cpu=%s gpu=%s",
				name, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
		}
	}
}
