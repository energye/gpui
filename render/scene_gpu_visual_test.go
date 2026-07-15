package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func TestSceneCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_SCENE_VISUAL") != "1" {
		t.Skip("set GPUI_SCENE_VISUAL=1 to run CPU/GPU scene diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-scene-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "scene_cpu.png")
	gpuPath := filepath.Join(outDir, "scene_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/scene", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/scene", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectSceneMetrics(cpuImg)
	gpuMetrics := collectSceneMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range sceneMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_SCENE_VISUAL_STRICT") == "1" {
		assertSceneVisualStrict(t, diff, cpuMetrics, gpuMetrics)
	}
}

func collectSceneMetrics(img image.Image) map[string]imagediff.RegionMetric {
	return map[string]imagediff.RegionMetric{
		"rect_red": imagediff.MeasureRegion(img, image.Rect(45, 45, 135, 135), func(p imagediff.Pixel) bool {
			return p.R > 160 && p.G < 120 && p.B < 120
		}),
		"rect_green": imagediff.MeasureRegion(img, image.Rect(155, 45, 245, 135), func(p imagediff.Pixel) bool {
			return p.G > 160 && p.R < 120 && p.B < 120
		}),
		"rect_blue": imagediff.MeasureRegion(img, image.Rect(265, 45, 355, 135), func(p imagediff.Pixel) bool {
			return p.B > 160 && p.R < 120 && p.G < 120
		}),
		"rect_yellow": imagediff.MeasureRegion(img, image.Rect(375, 45, 465, 135), func(p imagediff.Pixel) bool {
			return p.R > 160 && p.G > 160 && p.B < 120
		}),
		"center_circle": imagediff.MeasureRegion(img, image.Rect(150, 150, 360, 360), func(p imagediff.Pixel) bool {
			return p.R > 120 && p.B > 120 && p.G < 180
		}),
		"blend_zone": imagediff.MeasureRegion(img, image.Rect(195, 175, 365, 365), func(p imagediff.Pixel) bool {
			return !(p.R > 235 && p.G > 235 && p.B > 235)
		}),
	}
}

func sceneMetricNames() []string {
	return []string{"rect_red", "rect_green", "rect_blue", "rect_yellow", "center_circle", "blend_zone"}
}

func assertSceneVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric) {
	t.Helper()
	if diff.RMSE > 28 || diff.MeanAbs > 5 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range sceneMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		if cpu.Count == 0 || gpu.Count == 0 {
			t.Errorf("region %s missing pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			continue
		}
		ratio := float64(gpu.Count) / float64(cpu.Count)
		if ratio < 0.70 || ratio > 1.30 {
			t.Errorf("region %s pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				name, cpu.Count, gpu.Count, ratio)
		}
	}
}

func TestSceneGPUCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_SCENE_GPU_VISUAL") != "1" {
		t.Skip("set GPUI_SCENE_GPU_VISUAL=1 to run CPU/GPU scene_gpu diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-scene-gpu-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "scene_gpu_cpu.png")
	gpuPath := filepath.Join(outDir, "scene_gpu_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/scene_gpu", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/scene_gpu", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectSceneGPUMetrics(cpuImg)
	gpuMetrics := collectSceneGPUMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range sceneGPUMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_SCENE_GPU_VISUAL_STRICT") == "1" {
		assertSceneGPUVisualStrict(t, diff, cpuMetrics, gpuMetrics)
	}
}

func collectSceneGPUMetrics(img image.Image) map[string]imagediff.RegionMetric {
	return map[string]imagediff.RegionMetric{
		"corner_tl": imagediff.MeasureRegion(img, image.Rect(15, 15, 105, 105), func(p imagediff.Pixel) bool {
			return p.R > 140 && p.G < 120 && p.B < 120
		}),
		"corner_tr": imagediff.MeasureRegion(img, image.Rect(405, 15, 500, 105), func(p imagediff.Pixel) bool {
			return p.G > 140 && p.R < 120 && p.B < 120
		}),
		"corner_bl": imagediff.MeasureRegion(img, image.Rect(15, 405, 105, 500), func(p imagediff.Pixel) bool {
			return p.B > 140 && p.R < 120 && p.G < 120
		}),
		"corner_br": imagediff.MeasureRegion(img, image.Rect(405, 405, 500, 500), func(p imagediff.Pixel) bool {
			return p.R > 140 && p.G > 140 && p.B < 120
		}),
		"center": imagediff.MeasureRegion(img, image.Rect(190, 190, 320, 320), func(p imagediff.Pixel) bool {
			return p.B > 120 && p.R < 120
		}),
		"orbit": imagediff.MeasureRegion(img, image.Rect(80, 80, 430, 430), func(p imagediff.Pixel) bool {
			return !isSceneGPUBackground(p)
		}),
	}
}

func sceneGPUMetricNames() []string {
	return []string{"corner_tl", "corner_tr", "corner_bl", "corner_br", "center", "orbit"}
}

func assertSceneGPUVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric) {
	t.Helper()
	if diff.RMSE > 35 || diff.MeanAbs > 6 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range sceneGPUMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		if cpu.Count == 0 || gpu.Count == 0 {
			t.Errorf("region %s missing pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			continue
		}
		ratio := float64(gpu.Count) / float64(cpu.Count)
		if ratio < 0.60 || ratio > 1.40 {
			t.Errorf("region %s pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				name, cpu.Count, gpu.Count, ratio)
		}
	}
}

func isSceneGPUBackground(p imagediff.Pixel) bool {
	return p.R < 45 && p.G < 45 && p.B < 70
}
