//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package conformance

import (
	"fmt"
	"path/filepath"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// Draw replays the case ops on dc. Paths in ops (images, fonts) resolve
// against baseDir.
func Draw(dc *render.Context, c *Case, baseDir string) error {
	for i := range c.Ops {
		op := &c.Ops[i]
		if err := drawOp(dc, op, baseDir); err != nil {
			return fmt.Errorf("%s: op %d (%s): %w", c.Name, i, op.Kind, err)
		}
	}
	return nil
}

func drawOp(dc *render.Context, op *Op, baseDir string) error {
	switch op.Kind {
	case OpClear:
		dc.ClearWithColor(rgba(*op.Color))

	case OpFillRect:
		dc.SetFillBrush(render.Solid(rgba(*op.Color)))
		dc.DrawRectangle(op.Rect[0], op.Rect[1], op.Rect[2], op.Rect[3])
		return dc.Fill()

	case OpLinearGradient:
		brush := render.NewLinearGradientBrush(op.From[0], op.From[1], op.To[0], op.To[1])
		for _, s := range op.Stops {
			brush.AddColorStop(s.Offset, rgba(s.Color))
		}
		dc.SetFillBrush(brush)
		dc.DrawRectangle(op.Rect[0], op.Rect[1], op.Rect[2], op.Rect[3])
		return dc.Fill()

	case OpRadialGradient:
		brush := render.NewRadialGradientBrush(op.Center[0], op.Center[1], 0, op.Radius)
		for _, s := range op.Stops {
			brush.AddColorStop(s.Offset, rgba(s.Color))
		}
		dc.SetFillBrush(brush)
		dc.DrawRectangle(op.Rect[0], op.Rect[1], op.Rect[2], op.Rect[3])
		return dc.Fill()

	case OpClipRect:
		dc.ClipRect(op.Rect[0], op.Rect[1], op.Rect[2], op.Rect[3])

	case OpClipRoundRect:
		dc.ClipRoundRect(op.Rect[0], op.Rect[1], op.Rect[2], op.Rect[3], op.Radius)

	case OpPushLayer:
		opacity := op.Opacity
		if opacity == 0 {
			opacity = 1
		}
		dc.PushLayer(render.BlendNormal, opacity)

	case OpPopLayer:
		dc.PopLayer()

	case OpDrawImage:
		img, err := render.LoadImage(filepath.Join(baseDir, op.Path))
		if err != nil {
			return err
		}
		if img == nil {
			return fmt.Errorf("LoadImage(%s) returned nil", op.Path)
		}
		dc.DrawImage(img, op.X, op.Y)

	case OpText:
		src, err := text.NewFontSourceFromFile(filepath.Join(baseDir, op.Font))
		if err != nil {
			return err
		}
		dc.SetFont(src.Face(op.Size))
		dc.DrawString(op.Text, op.X, op.Y)

	default:
		return fmt.Errorf("unknown kind %q", op.Kind)
	}
	return nil
}

func rgba(c Color) render.RGBA {
	return render.RGBA{R: c[0], G: c[1], B: c[2], A: c[3]}
}
