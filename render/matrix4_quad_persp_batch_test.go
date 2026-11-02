//go:build !nogpu

package render_test

import (
	"image"
	"image/color"
	"math"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// Multi-quad projective frames: ui_wr_p35_persp card A + card B replayed
// offscreen. Every quad must raster at its own analytic band point, GPU and CPU
// alike (the clip-group split that misplaced projective draws is pinned in
// render/internal/gpu/perspective_quad_test.go).

const (
	ladderTexW    = 64
	ladderTexH    = 40
	ladderBandN   = 5
	ladderBandPix = ladderTexH / ladderBandN
)

var ladderBands = [ladderBandN][3]uint8{
	{235, 55, 55},
	{55, 235, 90},
	{70, 120, 235},
	{235, 205, 70},
	{190, 95, 235},
}

func ladderTexture() *render.ImageBuf {
	img := image.NewNRGBA(image.Rect(0, 0, ladderTexW, ladderTexH))
	for y := 0; y < ladderTexH; y++ {
		c := ladderBands[y/ladderBandPix]
		if y%ladderBandPix == ladderBandPix-1 {
			c = [3]uint8{8, 8, 12}
		}
		for x := 0; x < ladderTexW; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: c[0], G: c[1], B: c[2], A: 255})
		}
	}
	return render.ImageBufFromImage(img)
}

// ladderPlate projects the window's plate geometry through the engine口.
func ladderPlate(focal, cx, cy, hw, hh, near, far float64) ([4]render.Point, [4]float64, bool) {
	m, ok := render.ProjectorMatrix4(focal, render.Pt(cx, cy), render.Point{})
	if !ok {
		return [4]render.Point{}, [4]float64{}, false
	}
	world := [4]render.Point{{X: -hw, Y: -hh}, {X: hw, Y: -hh}, {X: hw, Y: hh}, {X: -hw, Y: hh}}
	return m.ProjectQuad(world, [4]float64{far, far, near, near})
}

func ladderMidLine(corners [4]render.Point, t float64) render.Point {
	topX := (corners[0].X + corners[1].X) / 2
	topY := (corners[0].Y + corners[1].Y) / 2
	botX := (corners[3].X + corners[2].X) / 2
	botY := (corners[3].Y + corners[2].Y) / 2
	return render.Pt(topX+(botX-topX)*t, topY+(botY-topY)*t)
}

type quadSpec struct {
	name           string
	cx, cy, hw, hh float64
	near, far      float64
	perspective    bool
}

// drawLadderScene replays the window's image commands into one context.
func drawLadderScene(t *testing.T, dc *render.Context, img *render.ImageBuf, specs []quadSpec) {
	t.Helper()
	dc.ClearWithColor(render.RGBA{R: 0.07, G: 0.08, B: 0.10, A: 1})
	for _, sp := range specs {
		corners, w, ok := ladderPlate(100, sp.cx, sp.cy, sp.hw, sp.hh, sp.near, sp.far)
		if !ok {
			t.Fatalf("%s: projection rejected", sp.name)
		}
		if err := dc.DrawImageQuad(img, corners, render.QuadDrawOptions{
			Interpolation: render.InterpNearest, Opacity: 1, Perspective: sp.perspective, Weights: w,
		}); err != nil {
			t.Fatalf("%s: %v", sp.name, err)
		}
	}
}

func diff8(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// TestQuadPerspectiveLadderEveryPlateLands draws the window's exact command list
// offscreen and demands ink at every quad's midline, both GPU and CPU twin.
func TestQuadPerspectiveLadderEveryPlateLands(t *testing.T) {
	requireNativeGPU(t)
	os.Unsetenv("GOGPU_RENDER_MODE")
	img := ladderTexture()
	defer img.Dispose()

	specs := []quadSpec{
		{name: "A_affine", cx: 400, cy: 175, hw: 110, hh: 150, near: 20, far: 260, perspective: false},
		{name: "A_persp", cx: 612, cy: 175, hw: 110, hh: 150, near: 20, far: 260, perspective: true},
		{name: "B0_far40", cx: 799, cy: 175, hw: 34, hh: 90, near: 20, far: 40, perspective: true},
		{name: "B1_far110", cx: 885, cy: 175, hw: 34, hh: 90, near: 20, far: 110, perspective: true},
		{name: "B2_far200", cx: 971, cy: 175, hw: 34, hh: 90, near: 20, far: 200, perspective: true},
		{name: "B3_far330", cx: 1057, cy: 175, hw: 34, hh: 90, near: 20, far: 330, perspective: true},
	}

	renderMode := func(cpu bool) image.Image {
		if cpu {
			os.Setenv("GOGPU_RENDER_MODE", "cpu")
		} else {
			os.Unsetenv("GOGPU_RENDER_MODE")
		}
		dc := render.NewContext(1200, 400)
		defer dc.Close()
		drawLadderScene(t, dc, img, specs)
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU cpu=%v: %v", cpu, err)
		}
		if !cpu {
			if st := dc.RenderPathStats(); st.GPUOps == 0 {
				t.Fatalf("no GPU ops: %s", st.LogLine())
			}
		}
		// Copy the pixels out before the context goes away.
		src := dc.Image()
		out := image.NewNRGBA(src.Bounds())
		for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
			for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
				out.Set(x, y, src.At(x, y))
			}
		}
		os.Unsetenv("GOGPU_RENDER_MODE")
		return out
	}

	gpu := renderMode(false)
	cpu := renderMode(true)

	const bgR, bgG, bgB = uint8(18), uint8(20), uint8(25)
	for _, sp := range specs {
		corners, w, _ := ladderPlate(100, sp.cx, sp.cy, sp.hw, sp.hh, sp.near, sp.far)
		tt := 0.5
		v := tt
		if sp.perspective {
			v = tt * w[0] / ((1-tt)*w[3] + tt*w[0])
		}
		row := int(math.Floor(v * ladderTexH))
		if row%ladderBandPix == ladderBandPix-1 {
			continue // separator row is not a colour witness
		}
		want := ladderBands[row/ladderBandPix]
		p := ladderMidLine(corners, tt)
		px, py := int(math.Floor(p.X)), int(math.Floor(p.Y))
		for name, src := range map[string]image.Image{"gpu": gpu, "cpu": cpu} {
			r, g, b, _ := src.At(px, py).RGBA()
			got := [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
			if got == [3]uint8{bgR, bgG, bgB} {
				t.Errorf("%s midline (%d,%d): %s shows bare background — quad missing", sp.name, px, py, name)
				continue
			}
			if diff8(got[0], want[0]) > 6 || diff8(got[1], want[1]) > 6 || diff8(got[2], want[2]) > 6 {
				t.Errorf("%s midline (%d,%d): %s = %v, want band %v", sp.name, px, py, name, got, want)
			}
		}
	}
}
