package render

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func TestImagesCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_IMAGES_VISUAL") != "1" {
		t.Skip("set GPUI_IMAGES_VISUAL=1 to run CPU/GPU images diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-images-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "images_cpu.png")
	gpuPath := filepath.Join(outDir, "images_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/images", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/images", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}

	cpuMetrics := collectImagesMetrics(cpuImg)
	gpuMetrics := collectImagesMetrics(gpuImg)
	samples := collectImagesSamples(cpuImg, gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	for _, name := range imagesMetricNames() {
		cpu := cpuMetrics[name]
		gpu := gpuMetrics[name]
		ratio := 0.0
		if cpu.Count > 0 {
			ratio = float64(gpu.Count) / float64(cpu.Count)
		}
		t.Logf("region=%s cpu_pixels=%d gpu_pixels=%d ratio=%.3f cpu_bbox=%s gpu_bbox=%s",
			name, cpu.Count, gpu.Count, ratio, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}
	for _, sample := range samples {
		t.Logf("sample=%s at=%d,%d cpu=%s gpu=%s delta=%d",
			sample.Name, sample.X, sample.Y, formatPixel(sample.CPU), formatPixel(sample.GPU), sample.MaxDelta)
	}

	if os.Getenv("GPUI_IMAGES_VISUAL_STRICT") == "1" {
		assertImagesVisualStrict(t, diff, cpuMetrics, gpuMetrics, samples)
	}
}

type imageSample struct {
	Name     string
	X, Y     int
	CPU      imagediff.Pixel
	GPU      imagediff.Pixel
	MaxDelta uint32
}

func collectImagesMetrics(img image.Image) map[string]imagediff.RegionMetric {
	return map[string]imagediff.RegionMetric{
		"basic":      imagediff.MeasureRegion(img, image.Rect(50, 100, 150, 200), isImageContent),
		"scaled":     imagediff.MeasureRegion(img, image.Rect(50, 260, 250, 460), isImageContent),
		"opacity":    imagediff.MeasureRegion(img, image.Rect(300, 100, 400, 200), isImageContent),
		"nearest":    imagediff.MeasureRegion(img, image.Rect(500, 100, 650, 250), isImageContent),
		"src_rect":   imagediff.MeasureRegion(img, image.Rect(300, 260, 400, 360), isImageContent),
		"multiply":   imagediff.MeasureRegion(img, image.Rect(500, 260, 600, 360), isImageContent),
		"transform":  imagediff.MeasureRegion(img, image.Rect(65, 465, 235, 600), isImageContent),
		"pattern":    imagediff.MeasureRegion(img, image.Rect(390, 480, 510, 600), isImageContent),
		"text_marks": imagediff.MeasureRegion(img, image.Rect(0, 0, 800, 600), isTextMarker),
	}
}

func imagesMetricNames() []string {
	return []string{
		"basic",
		"scaled",
		"opacity",
		"nearest",
		"src_rect",
		"multiply",
		"transform",
		"pattern",
		"text_marks",
	}
}

func collectImagesSamples(cpuImg, gpuImg image.Image) []imageSample {
	points := []struct {
		name string
		x, y int
	}{
		{"basic_center", 100, 150},
		{"scaled_center", 150, 360},
		{"opacity_center", 350, 150},
		{"nearest_center", 575, 175},
		{"src_rect_center", 350, 310},
		{"multiply_center", 550, 310},
		{"transform_center", 150, 540},
		{"pattern_center", 450, 540},
	}
	out := make([]imageSample, 0, len(points))
	for _, p := range points {
		cpu := imagediff.PixelAt(cpuImg, p.x, p.y)
		gpu := imagediff.PixelAt(gpuImg, p.x, p.y)
		out = append(out, imageSample{
			Name:     p.name,
			X:        p.x,
			Y:        p.y,
			CPU:      cpu,
			GPU:      gpu,
			MaxDelta: pixelMaxDelta(cpu, gpu),
		})
	}
	return out
}

func assertImagesVisualStrict(t *testing.T, diff imagediff.Stats, cpuMetrics, gpuMetrics map[string]imagediff.RegionMetric, samples []imageSample) {
	t.Helper()
	if diff.RMSE > 28 || diff.MeanAbs > 5 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for _, name := range imagesMetricNames() {
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
		if imagediff.AbsInt(gpu.Bounds.Dx()-cpu.Bounds.Dx()) > 18 || imagediff.AbsInt(gpu.Bounds.Dy()-cpu.Bounds.Dy()) > 18 {
			t.Errorf("region %s bbox differs too much: cpu=%s gpu=%s",
				name, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
		}
	}
	for _, sample := range samples {
		threshold := uint32(18)
		if strings.Contains(sample.Name, "scaled") || strings.Contains(sample.Name, "transform") || strings.Contains(sample.Name, "pattern") {
			threshold = 36
		}
		if sample.MaxDelta > threshold {
			t.Errorf("sample %s differs too much at %d,%d: cpu=%s gpu=%s delta=%d threshold=%d",
				sample.Name, sample.X, sample.Y, formatPixel(sample.CPU), formatPixel(sample.GPU), sample.MaxDelta, threshold)
		}
	}
}

func isImageContent(p imagediff.Pixel) bool {
	return !isImageBackground(p)
}

func isImageBackground(p imagediff.Pixel) bool {
	return p.R > 236 && p.G > 236 && p.B > 236
}

func isTextMarker(p imagediff.Pixel) bool {
	return p.R > 35 && p.R < 70 && p.G > 35 && p.G < 70 && p.B > 35 && p.B < 70
}

func pixelMaxDelta(a, b imagediff.Pixel) uint32 {
	max := absU8(a.R, b.R)
	if d := absU8(a.G, b.G); d > max {
		max = d
	}
	if d := absU8(a.B, b.B); d > max {
		max = d
	}
	return max
}

func absU8(a, b uint8) uint32 {
	if a > b {
		return uint32(a - b)
	}
	return uint32(b - a)
}

func formatPixel(p imagediff.Pixel) string {
	return fmt.Sprintf("rgba(%d,%d,%d,%d)", p.R, p.G, p.B, p.A)
}
