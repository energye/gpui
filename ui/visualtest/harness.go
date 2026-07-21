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

// CaptureTreeOptions configures CaptureTreeEx.
type CaptureTreeOptions struct {
	Theme *core.Theme
	// Focus, if non-nil, is focused after layout before paint.
	Focus core.Node
	// Clear is the background color; zero A → white.
	Clear render.RGBA
}

// CaptureTree layouts and paints a root node into a fixed-size CPU canvas.
func CaptureTree(width, height int, root core.Node, theme *core.Theme) image.Image {
	return CaptureTreeEx(width, height, root, CaptureTreeOptions{Theme: theme})
}

// CaptureTreeEx layouts and paints with optional focus + clear color.
func CaptureTreeEx(width, height int, root core.Node, opts CaptureTreeOptions) image.Image {
	if width <= 0 || height <= 0 || root == nil {
		return nil
	}
	dc := render.NewContext(width, height)
	defer dc.Close()
	dc.BeginFrame()
	clear := opts.Clear
	if clear.A <= 0 {
		clear = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	dc.ClearWithColor(clear)
	tree := core.NewTree(root)
	// Layout first so focus targets have size, then focus, then paint.
	tree.Layout(core.Size{Width: float64(width), Height: float64(height)})
	if opts.Focus != nil {
		tree.SetFocus(opts.Focus)
	}
	pc := &core.PaintContext{
		DC:     dc,
		Origin: core.Point{},
		Scale:  1,
		Theme:  opts.Theme,
	}
	tree.Paint(pc)
	return dc.Image()
}
