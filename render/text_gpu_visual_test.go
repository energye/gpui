package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func TestTextCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_TEXT_VISUAL") != "1" {
		t.Skip("set GPUI_TEXT_VISUAL=1 to run CPU/GPU text diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-text-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "text_cpu.png")
	gpuPath := filepath.Join(outDir, "text_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/text", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/text", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectTextMetrics(cpuImg)
	gpuMetrics := collectTextMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range textMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_TEXT_VISUAL_STRICT") == "1" {
		RequireGPUPathStats(t, gpuLog, true)
		assertTextVisualStrict(t, diff, cpuMetrics, gpuMetrics)
	}
}

func collectTextMetrics(img image.Image) map[string]imagediff.RegionMetric {
	return map[string]imagediff.RegionMetric{
		"title":    imagediff.MeasureRegion(img, image.Rect(40, 30, 500, 100), isInkDarkOrColored),
		"subtitle": imagediff.MeasureRegion(img, image.Rect(40, 100, 650, 150), isInkDarkOrColored),
		"left":     imagediff.MeasureRegion(img, image.Rect(40, 155, 300, 200), isInkDarkOrColored),
		"center":   imagediff.MeasureRegion(img, image.Rect(280, 195, 520, 245), isInkBlueish),
		"right":    imagediff.MeasureRegion(img, image.Rect(560, 235, 780, 285), isInkReddish),
		"measured": imagediff.MeasureRegion(img, image.Rect(40, 270, 350, 340), isInkGreenish),
		"fontinfo": imagediff.MeasureRegion(img, image.Rect(40, 345, 500, 395), isInkDarkOrColored),
	}
}

func textMetricNames() []string {
	return []string{"title", "subtitle", "left", "center", "right", "measured", "fontinfo"}
}

func assertTextVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric) {
	t.Helper()
	if diff.RMSE > 24 || diff.MeanAbs > 4 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range textMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		if cpu.Count == 0 || gpu.Count == 0 {
			t.Errorf("region %s missing pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			continue
		}
		ratio := float64(gpu.Count) / float64(cpu.Count)
		if ratio < 0.55 || ratio > 1.80 {
			t.Errorf("region %s pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				name, cpu.Count, gpu.Count, ratio)
		}
		if imagediff.AbsInt(gpu.Bounds.Dx()-cpu.Bounds.Dx()) > 40 || imagediff.AbsInt(gpu.Bounds.Dy()-cpu.Bounds.Dy()) > 24 {
			t.Errorf("region %s bbox differs too much: cpu=%s gpu=%s",
				name, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
		}
	}
}

func isInkDarkOrColored(p imagediff.Pixel) bool {
	return !isNearWhite(p)
}

func isInkBlueish(p imagediff.Pixel) bool {
	return p.B > p.R+20 && p.B > p.G+10 && p.B > 80
}

func isInkReddish(p imagediff.Pixel) bool {
	return p.R > p.G+40 && p.R > p.B+40 && p.R > 100
}

func isInkGreenish(p imagediff.Pixel) bool {
	return p.G > p.R+20 && p.G > p.B+20 && p.G > 80
}
