// gen_grid_png generates the 20 deck tiles for the RULER_SCENE=grid arm
// (matrix row 5, mirroring examples/ui_wr_r10_async_image). The pixel formula
// is the same as R10's makeSourcePNGs (96x56 striped tiles, per-index base
// hue) so the two arms decode identical imagery. Committing the outputs under
// assets/grid/ keeps the ruler arm free of runtime generation; rerunning this
// tool must produce byte-identical files.
//
// Usage: go run ./tools/ruler_flutter/gen_grid_png
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const count = 20

func main() {
	dir := filepath.Join("tools", "ruler_flutter", "assets", "grid")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	for i := 0; i < count; i++ {
		m := image.NewRGBA(image.Rect(0, 0, 96, 56))
		base := color.RGBA{R: uint8(30 + i*9), G: uint8(60 + i*7), B: uint8(140 + i*4), A: 255}
		for y := 0; y < 56; y++ {
			for x := 0; x < 96; x++ {
				if ((x+y)/8+i)%2 == 0 {
					m.Set(x, y, base)
				} else {
					m.Set(x, y, color.RGBA{R: 24, G: 26, B: 34, A: 255})
				}
			}
		}
		p := filepath.Join(dir, fmt.Sprintf("async_%02d.png", i))
		f, err := os.Create(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create:", err)
			os.Exit(1)
		}
		if err := png.Encode(f, m); err != nil {
			f.Close()
			fmt.Fprintln(os.Stderr, "encode:", err)
			os.Exit(1)
		}
		f.Close()
	}
	fmt.Printf("wrote %d pngs to %s\n", count, dir)
}
