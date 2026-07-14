package render

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	cpuLog := runTextTransformVisualCommand(t, repoRoot, nil, cpuPath)
	gpuLog := runTextTransformVisualCommand(t, repoRoot, []string{"-tags", "gpui_visual_gpu"}, gpuPath)

	cpuImg := decodePNG(t, cpuPath)
	gpuImg := decodePNG(t, gpuPath)
	diff := diffImages(cpuImg, gpuImg)
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
			formatRect(cpu.Bounds), formatRect(gpu.Bounds))
	}

	if os.Getenv("GPUI_TEXT_TRANSFORM_VISUAL_STRICT") == "1" {
		assertTextTransformVisualStrict(t, diff, cpuCells, gpuCells)
	}
}

func visualRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if filepath.Base(wd) == "render" {
		return filepath.Dir(wd)
	}
	return wd
}

func runTextTransformVisualCommand(t *testing.T, repoRoot string, goArgs []string, out string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	args := append([]string{"run"}, goArgs...)
	args = append(args, "./render/internal/visualcmd/text_transform", "-out", out)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go %s failed: %v\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	if ctx.Err() != nil {
		t.Fatalf("go %s timed out", strings.Join(args, " "))
	}
	return strings.TrimSpace(stdout.String() + "\n" + stderr.String())
}

func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open png %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode png %s: %v", path, err)
	}
	return img
}

type visualDiffStats struct {
	TotalPixels   int
	ChangedPixels int
	MeanAbs       float64
	RMSE          float64
	MaxDelta      uint32
}

func diffImages(a, b image.Image) visualDiffStats {
	ab := a.Bounds()
	bb := b.Bounds()
	if !ab.Eq(bb) {
		panic(fmt.Sprintf("image bounds differ: %v vs %v", ab, bb))
	}

	var sumAbs, sumSq float64
	var changed int
	var maxDelta uint32
	total := ab.Dx() * ab.Dy()
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			ar, ag, abv, _ := a.At(x, y).RGBA()
			br, bg, bbv, _ := b.At(x, y).RGBA()
			deltas := [3]uint32{absU32(ar, br), absU32(ag, bg), absU32(abv, bbv)}
			pixelChanged := false
			for _, d16 := range deltas {
				d8 := d16 / 257
				if d8 > 2 {
					pixelChanged = true
				}
				if d8 > maxDelta {
					maxDelta = d8
				}
				sumAbs += float64(d8)
				sumSq += float64(d8 * d8)
			}
			if pixelChanged {
				changed++
			}
		}
	}

	samples := float64(total * 3)
	return visualDiffStats{
		TotalPixels:   total,
		ChangedPixels: changed,
		MeanAbs:       sumAbs / samples,
		RMSE:          math.Sqrt(sumSq / samples),
		MaxDelta:      maxDelta,
	}
}

func absU32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
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
					r16, g16, b16, _ := img.At(x, y).RGBA()
					r, g, b := uint8(r16/257), uint8(g16/257), uint8(b16/257)
					if isTextDark(r, g, b) {
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
					if isCrosshairRed(r, g, b) {
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

func isTextDark(r, g, b uint8) bool {
	return r < 90 && g < 90 && b < 90
}

func isCrosshairRed(r, g, b uint8) bool {
	return r > 150 && g < 100 && b < 100
}

func formatRect(r image.Rectangle) string {
	if r.Empty() {
		return "empty"
	}
	return fmt.Sprintf("%dx%d@%d,%d", r.Dx(), r.Dy(), r.Min.X, r.Min.Y)
}

func assertTextTransformVisualStrict(t *testing.T, diff visualDiffStats, cpuCells, gpuCells []textTransformCellMetric) {
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
		if absInt(gpu.Bounds.Dx()-cpu.Bounds.Dx()) > 20 || absInt(gpu.Bounds.Dy()-cpu.Bounds.Dy()) > 20 {
			t.Errorf("cell %s bbox differs too much: cpu=%s gpu=%s",
				cpu.Name, formatRect(cpu.Bounds), formatRect(gpu.Bounds))
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
