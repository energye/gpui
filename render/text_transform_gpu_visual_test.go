package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

const (
	textTransformVisualRows = 3
	textTransformVisualCols = 3

	textTransformVisualCellW = 280.0
	textTransformVisualCellH = 210.0

	textTransformVisualGridLeft = 25.0
	textTransformVisualGridTop  = 70.0
)

func TestTextTransformCPUvsGPUVisualDiagnostic(t *testing.T) {
	if os.Getenv("GPUI_TEXT_TRANSFORM_VISUAL") != "1" {
		t.Skip("set GPUI_TEXT_TRANSFORM_VISUAL=1 to run CPU/GPU text_transform diagnostics")
	}

	repoRoot := visualRepoRoot(t)
	outDir := filepath.Join(os.TempDir(), "gpui-text-transform-visual")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	cpuPath := filepath.Join(outDir, "text_transform_cpu.png")
	gpuPath := filepath.Join(outDir, "text_transform_gpu.png")

	cpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/text_transform", nil, cpuPath)
	gpuLog := runVisualCommand(t, repoRoot, "./render/internal/visualcmd/text_transform", []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNGForVisualTest(t, cpuPath)
	gpuImg := decodePNGForVisualTest(t, gpuPath)
	diff, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff images: %v", err)
	}
	cpuCells := collectTextTransformCellMetrics(cpuImg)
	gpuCells := collectTextTransformCellMetrics(gpuImg)

	t.Logf("cpu_png=%s", cpuPath)
	t.Logf("gpu_png=%s", gpuPath)
	t.Logf("cpu_log=%s", strings.TrimSpace(cpuLog))
	t.Logf("gpu_log=%s", strings.TrimSpace(gpuLog))
	t.Logf("diff: changed=%d/%d mean_abs=%.3f rmse=%.3f max_delta=%d",
		diff.ChangedPixels, diff.TotalPixels, diff.MeanAbs, diff.RMSE, diff.MaxDelta)

	for i := range cpuCells {
		cpu := cpuCells[i]
		gpu := gpuCells[i]
		ratio := 0.0
		if cpu.DarkPixels > 0 {
			ratio = float64(gpu.DarkPixels) / float64(cpu.DarkPixels)
		}
		t.Logf("cell=%s cpu_dark=%d gpu_dark=%d ratio=%.3f cpu_red=%d gpu_red=%d cpu_bbox=%s gpu_bbox=%s",
			cpu.Name, cpu.DarkPixels, gpu.DarkPixels, ratio, cpu.RedPixels, gpu.RedPixels,
			imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_TEXT_TRANSFORM_VISUAL_STRICT") == "1" {
		RequireGPUPathStats(t, gpuLog, true)
		assertTextTransformVisualStrict(t, diff, cpuCells, gpuCells)
	}
}

func decodePNGForVisualTest(t *testing.T, path string) image.Image {
	t.Helper()
	img, err := imagediff.DecodePNG(path)
	if err != nil {
		t.Fatalf("decode png %s: %v", path, err)
	}
	return img
}

type textTransformCellMetric struct {
	Name       string
	DarkPixels int
	RedPixels  int
	Bounds     image.Rectangle
}

func collectTextTransformCellMetrics(img image.Image) []textTransformCellMetric {
	names := [textTransformVisualRows][textTransformVisualCols]string{
		{"identity", "translate", "scale2x"},
		{"scale_down", "scale3x1", "rotate30"},
		{"rotate45", "shear", "scale_rotate"},
	}
	metrics := make([]textTransformCellMetric, 0, textTransformVisualRows*textTransformVisualCols)
	for row := 0; row < textTransformVisualRows; row++ {
		for col := 0; col < textTransformVisualCols; col++ {
			cx := int(textTransformVisualGridLeft + float64(col)*textTransformVisualCellW + float64(col)*10)
			cy := int(textTransformVisualGridTop + float64(row)*textTransformVisualCellH + float64(row)*5)
			rect := image.Rect(cx, cy+30, cx+int(textTransformVisualCellW), cy+int(textTransformVisualCellH)).Intersect(img.Bounds())
			metric := textTransformCellMetric{
				Name:   names[row][col],
				Bounds: image.Rectangle{},
			}
			minX, minY := rect.Max.X, rect.Max.Y
			maxX, maxY := rect.Min.X, rect.Min.Y
			for y := rect.Min.Y; y < rect.Max.Y; y++ {
				for x := rect.Min.X; x < rect.Max.X; x++ {
					p := imagediff.PixelAt(img, x, y)
					if isTextDark(p) {
						metric.DarkPixels++
						if x < minX {
							minX = x
						}
						if y < minY {
							minY = y
						}
						if x+1 > maxX {
							maxX = x + 1
						}
						if y+1 > maxY {
							maxY = y + 1
						}
					}
					if isCrosshairRed(p) {
						metric.RedPixels++
					}
				}
			}
			if metric.DarkPixels > 0 {
				metric.Bounds = image.Rect(minX, minY, maxX, maxY)
			}
			metrics = append(metrics, metric)
		}
	}
	return metrics
}

func isTextDark(p imagediff.Pixel) bool {
	return p.R < 90 && p.G < 90 && p.B < 90
}

func isCrosshairRed(p imagediff.Pixel) bool {
	return p.R > 150 && p.G < 100 && p.B < 100
}

func assertTextTransformVisualStrict(t *testing.T, diff imagediff.Stats, cpuCells, gpuCells []textTransformCellMetric) {
	t.Helper()
	if diff.RMSE > 16 || diff.MeanAbs > 2.5 {
		t.Errorf("CPU/GPU image diff too high: mean_abs=%.3f rmse=%.3f max_delta=%d",
			diff.MeanAbs, diff.RMSE, diff.MaxDelta)
	}
	for i := range cpuCells {
		cpu := cpuCells[i]
		gpu := gpuCells[i]
		if cpu.DarkPixels == 0 || gpu.DarkPixels == 0 {
			t.Errorf("cell %s missing text pixels: cpu=%d gpu=%d", cpu.Name, cpu.DarkPixels, gpu.DarkPixels)
			continue
		}
		ratio := float64(gpu.DarkPixels) / float64(cpu.DarkPixels)
		if ratio < 0.65 || ratio > 1.55 {
			t.Errorf("cell %s dark-pixel ratio out of range: cpu=%d gpu=%d ratio=%.3f",
				cpu.Name, cpu.DarkPixels, gpu.DarkPixels, ratio)
		}
		if imagediff.AbsInt(gpu.Bounds.Dx()-cpu.Bounds.Dx()) > 20 || imagediff.AbsInt(gpu.Bounds.Dy()-cpu.Bounds.Dy()) > 20 {
			t.Errorf("cell %s bbox differs too much: cpu=%s gpu=%s",
				cpu.Name, imagediff.FormatRect(cpu.Bounds), imagediff.FormatRect(gpu.Bounds))
		}
	}
}
