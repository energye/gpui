package imagediff

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
)

type Pixel struct {
	R, G, B, A uint8
}

type Stats struct {
	TotalPixels   int
	ChangedPixels int
	MeanAbs       float64
	RMSE          float64
	MaxDelta      uint32
}

type RegionMetric struct {
	Count  int
	Bounds image.Rectangle
}

func DecodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func Images(a, b image.Image) (Stats, error) {
	ab := a.Bounds()
	bb := b.Bounds()
	if !ab.Eq(bb) {
		return Stats{}, fmt.Errorf("image bounds differ: %v vs %v", ab, bb)
	}

	var sumAbs, sumSq float64
	var changed int
	var maxDelta uint32
	total := ab.Dx() * ab.Dy()
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			ap := PixelAt(a, x, y)
			bp := PixelAt(b, x, y)
			deltas := [3]uint32{
				absU8(ap.R, bp.R),
				absU8(ap.G, bp.G),
				absU8(ap.B, bp.B),
			}
			pixelChanged := false
			for _, d := range deltas {
				if d > 2 {
					pixelChanged = true
				}
				if d > maxDelta {
					maxDelta = d
				}
				sumAbs += float64(d)
				sumSq += float64(d * d)
			}
			if pixelChanged {
				changed++
			}
		}
	}

	samples := float64(total * 3)
	return Stats{
		TotalPixels:   total,
		ChangedPixels: changed,
		MeanAbs:       sumAbs / samples,
		RMSE:          math.Sqrt(sumSq / samples),
		MaxDelta:      maxDelta,
	}, nil
}

func PixelAt(img image.Image, x, y int) Pixel {
	r16, g16, b16, a16 := img.At(x, y).RGBA()
	return Pixel{
		R: uint8(r16 / 257),
		G: uint8(g16 / 257),
		B: uint8(b16 / 257),
		A: uint8(a16 / 257),
	}
}

func MeasureRegion(img image.Image, rect image.Rectangle, match func(Pixel) bool) RegionMetric {
	return MeasureRegionXY(img, rect, func(_ int, _ int, p Pixel) bool {
		return match(p)
	})
}

func MeasureRegionXY(img image.Image, rect image.Rectangle, match func(x, y int, p Pixel) bool) RegionMetric {
	rect = rect.Intersect(img.Bounds())
	metric := RegionMetric{}
	minX, minY := rect.Max.X, rect.Max.Y
	maxX, maxY := rect.Min.X, rect.Min.Y
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if !match(x, y, PixelAt(img, x, y)) {
				continue
			}
			metric.Count++
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
	}
	if metric.Count > 0 {
		metric.Bounds = image.Rect(minX, minY, maxX, maxY)
	}
	return metric
}

func FormatRect(r image.Rectangle) string {
	if r.Empty() {
		return "empty"
	}
	return fmt.Sprintf("%dx%d@%d,%d", r.Dx(), r.Dy(), r.Min.X, r.Min.Y)
}

func AbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func absU8(a, b uint8) uint32 {
	if a > b {
		return uint32(a - b)
	}
	return uint32(b - a)
}
