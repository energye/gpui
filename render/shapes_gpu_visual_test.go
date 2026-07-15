package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func TestShapesCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_SHAPES_VISUAL") != "1" {
		t.Skip("set GPUI_SHAPES_VISUAL=1 to run CPU/GPU shapes diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-shapes-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "shapes_cpu.png")
	gpuPath := filepath.Join(outDir, "shapes_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/shapes", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/shapes", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectShapesMetrics(cpuImg)
	gpuMetrics := collectShapesMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range shapesMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_SHAPES_VISUAL_STRICT") == "1" {
		RequireGPUPathStats(t, gpuLog, true)
		assertShapesVisualStrict(t, diff, cpuMetrics, gpuMetrics)
	}
}

func collectShapesMetrics(img image.Image) map[string]imagediff.RegionMetric {
	return map[string]imagediff.RegionMetric{
		"rect_red": imagediff.MeasureRegion(img, image.Rect(45, 45, 205, 155), func(p imagediff.Pixel) bool {
			return p.R > 150 && p.G > 25 && p.G < 90 && p.B > 25 && p.B < 90
		}),
		"rrect_green": imagediff.MeasureRegion(img, image.Rect(245, 45, 405, 155), func(p imagediff.Pixel) bool {
			return p.G > 150 && p.R > 25 && p.R < 90 && p.B > 25 && p.B < 90
		}),
		"circle_blue": imagediff.MeasureRegion(img, image.Rect(435, 35, 565, 165), func(p imagediff.Pixel) bool {
			return p.B > 150 && p.R > 25 && p.R < 90 && p.G > 25 && p.G < 90
		}),
		"ellipse_yellow": imagediff.MeasureRegion(img, image.Rect(565, 45, 735, 155), func(p imagediff.Pixel) bool {
			return p.R > 150 && p.G > 150 && p.B > 25 && p.B < 90 && imagediff.AbsInt(int(p.R)-int(p.G)) < 35
		}),
		"pentagon_orange": imagediff.MeasureRegion(img, image.Rect(45, 245, 150, 355), func(p imagediff.Pixel) bool {
			return p.R > 210 && p.G > 90 && p.G < 160 && p.B < 60
		}),
		"hexagon_purple": imagediff.MeasureRegion(img, image.Rect(195, 245, 305, 355), func(p imagediff.Pixel) bool {
			return p.R > 90 && p.R < 160 && p.G < 60 && p.B > 210
		}),
		"octagon_cyan": imagediff.MeasureRegion(img, image.Rect(340, 245, 460, 355), func(p imagediff.Pixel) bool {
			return p.R < 60 && p.G > 150 && p.B > 150
		}),
		"black_line": imagediff.MeasureRegion(img, image.Rect(45, 445, 755, 455), func(p imagediff.Pixel) bool {
			return p.R < 40 && p.G < 40 && p.B < 40
		}),
		"arc_magenta": imagediff.MeasureRegion(img, image.Rect(585, 235, 715, 365), func(p imagediff.Pixel) bool {
			return p.R > 150 && p.G < 60 && p.B > 150
		}),
		"rotated_teal": imagediff.MeasureRegion(img, image.Rect(335, 435, 465, 565), func(p imagediff.Pixel) bool {
			return p.R > 25 && p.R < 90 && p.G > 110 && p.G < 180 && p.B > 150
		}),
	}
}

func shapesMetricNames() []string {
	return []string{
		"rect_red",
		"rrect_green",
		"circle_blue",
		"ellipse_yellow",
		"pentagon_orange",
		"hexagon_purple",
		"octagon_cyan",
		"black_line",
		"arc_magenta",
		"rotated_teal",
	}
}

func assertShapesVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric) {
	t.Helper()
	if diff.RMSE > 20 || diff.MeanAbs > 3.5 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range shapesMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		if cpu.Count == 0 || gpu.Count == 0 {
			t.Errorf("region %s missing pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			continue
		}
		ratio := float64(gpu.Count) / float64(cpu.Count)
		if ratio < 0.80 || ratio > 1.20 {
			t.Errorf("region %s pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				name, cpu.Count, gpu.Count, ratio)
		}
		if imagediff.AbsInt(gpu.Bounds.Dx()-cpu.Bounds.Dx()) > 14 || imagediff.AbsInt(gpu.Bounds.Dy()-cpu.Bounds.Dy()) > 14 {
			t.Errorf("region %s bbox differs too much: cpu=%s gpu=%s",
				name, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
		}
	}
}
