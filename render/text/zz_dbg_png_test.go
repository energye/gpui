package text

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestDbgRenderPNG 渲染「静每合」为 PNG（light vs none 对照），供目视。
func TestDbgRenderPNG(t *testing.T) {
	src, err := NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()

	sizes := []float64{14, 28}
	runes := "静每合"
	for _, size := range sizes {
		for _, h := range []struct {
			name string
			hint Hinting
		}{{"none", HintingNone}, {"light", HintingVertical}} {
			face := src.Face(size, WithHinting(h.hint))
			img := image.NewRGBA(image.Rect(0, 0, len(runes)*int(size)+40, int(size*2.2)))
			for x := 0; x < img.Bounds().Dx(); x++ {
				for y := 0; y < img.Bounds().Dy(); y++ {
					img.Set(x, y, color.Gray{Y: 220})
				}
			}
			Draw(img, runes, face, 6, size*1.6, color.Black)
			out := filepath.Join(os.TempDir(), fmt.Sprintf("dbg_r18_%02.0fpx_%s.png", size, h.name))
			f, err := os.Create(out)
			if err != nil {
				t.Fatal(err)
			}
			_ = png.Encode(f, img)
			f.Close()
			t.Logf("wrote %s (%vpx %s)", out, size, h.name)
		}
	}
}