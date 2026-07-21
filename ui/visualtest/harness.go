package visualtest

import (
	"image"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// PaintFunc draws into a PaintContext whose Origin is (0,0) and Scale is 1.
type PaintFunc func(pc *core.PaintContext)

// Capture renders paint into a fixed-size CPU canvas and returns the image.
// Background is white (scale=1). Caller owns no Context after return.
func Capture(width, height int, paint PaintFunc) image.Image {
	if width <= 0 || height <= 0 {
		return nil
	}
	dc := render.NewContext(width, height)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
	if paint != nil {
		pc := &core.PaintContext{
			DC:     dc,
			Origin: core.Point{},
			Scale:  1,
		}
		paint(pc)
	}
	return dc.Image()
}

// CaptureTree layouts and paints a root node into a fixed-size CPU canvas.
func CaptureTree(width, height int, root core.Node, theme *core.Theme) image.Image {
	if width <= 0 || height <= 0 || root == nil {
		return nil
	}
	dc := render.NewContext(width, height)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
	tree := core.NewTree(root)
	pc := &core.PaintContext{
		DC:     dc,
		Origin: core.Point{},
		Scale:  1,
		Theme:  theme,
	}
	tree.Frame(pc, core.Size{Width: float64(width), Height: float64(height)})
	return dc.Image()
}
