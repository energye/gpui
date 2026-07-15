package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func TestCJKTextCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_CJK_TEXT_VISUAL") != "1" {
		t.Skip("set GPUI_CJK_TEXT_VISUAL=1 to run CPU/GPU cjk_text diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-cjk-text-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "cjk_text_cpu.png")
	gpuPath := filepath.Join(outDir, "cjk_text_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/cjk_text", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/cjk_text", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectCJKTextMetrics(cpuImg)
	gpuMetrics := collectCJKTextMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range cjkTextMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_CJK_TEXT_VISUAL_STRICT") == "1" {
		RequireGPUPathStats(t, gpuLog, true)
		assertCJKTextVisualStrict(t, diff, cpuMetrics, gpuMetrics)
	}
}

func collectCJKTextMetrics(img image.Image) map[string]imagediff.RegionMetric {
	return map[string]imagediff.RegionMetric{
		"body":    imagediff.MeasureRegion(img, image.Rect(10, 45, 780, 230), isInkDarkOrColored),
		"display": imagediff.MeasureRegion(img, image.Rect(10, 240, 450, 430), isInkDarkOrColored),
		"mixed":   imagediff.MeasureRegion(img, image.Rect(480, 40, 780, 150), isInkDarkOrColored),
		"titles":  imagediff.MeasureRegion(img, image.Rect(10, 10, 300, 45), isInkBlueish),
	}
}

func cjkTextMetricNames() []string {
	return []string{"body", "display", "mixed", "titles"}
}

func assertCJKTextVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric) {
	t.Helper()
	if diff.RMSE > 30 || diff.MeanAbs > 5 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range cjkTextMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		if cpu.Count == 0 || gpu.Count == 0 {
			t.Errorf("region %s missing pixels: cpu=%d gpu=%d", name, cpu.Count, gpu.Count)
			continue
		}
		ratio := float64(gpu.Count) / float64(cpu.Count)
		if ratio < 0.50 || ratio > 1.90 {
			t.Errorf("region %s pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				name, cpu.Count, gpu.Count, ratio)
		}
	}
}
