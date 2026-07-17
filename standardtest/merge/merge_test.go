package merge

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestComposeAndDescribe(t *testing.T) {
	std := image.NewRGBA(image.Rect(0, 0, 64, 40))
	act := image.NewRGBA(image.Rect(0, 0, 64, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 64; x++ {
			std.Set(x, y, color.RGBA{200, 40, 40, 255})
			act.Set(x, y, color.RGBA{40, 80, 200, 255})
		}
	}
	out, err := ComposeSideBySide(std, act, CompositeOptions{
		Title: "Demo", Description: "背景：红/蓝色块。\n应看到：左右并排可辨认。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Bounds().Dx() < 100 {
		t.Fatal(out.Bounds())
	}
	d := DescribeSceneCN(&SceneLite{
		ID: "D", Size: [2]int{10, 10},
		Ops: []map[string]any{
			{"op": "clear", "rgba": []any{1.0, 1.0, 1.0, 1.0}},
			{"op": "fill_rect", "rect": []any{0.0, 0.0, 5.0, 5.0}, "rgba": []any{1.0, 0.0, 0.0, 1.0}},
		},
	})
	if !strings.Contains(d, "背景") || !strings.Contains(d, "矩形") {
		t.Fatal(d)
	}
}
