package scene

import (
	"fmt"
	"path/filepath"

	"github.com/energye/gpui/render"
)

// RunGPUI executes the scene into a new GPU-capable Context and returns it.
// Caller must Close the context. fontRoot is used to resolve relative font paths
// (typically module root or testdata/).
func RunGPUI(s *Scene, fontRoot string) (*render.Context, error) {
	if s == nil {
		return nil, fmt.Errorf("nil scene")
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	w, h := s.Size[0], s.Size[1]
	var dc *render.Context
	if s.Scale != 0 && s.Scale != 1 {
		dc = render.NewContext(w, h, render.WithDeviceScale(s.Scale))
	} else {
		dc = render.NewContext(w, h)
	}

	if s.Font != nil && s.Font.Path != "" {
		fp := s.Font.Path
		if !filepath.IsAbs(fp) {
			fp = filepath.Join(fontRoot, fp)
		}
		sz := s.Font.Size
		if sz <= 0 {
			sz = 16
		}
		if err := dc.LoadFontFace(fp, sz); err != nil {
			dc.Close()
			return nil, fmt.Errorf("font %s: %w", fp, err)
		}
	}

	// default clear white
	dc.ClearWithColor(render.White)

	for i, op := range s.Ops {
		if err := applyGPUI(dc, s, &op, fontRoot); err != nil {
			dc.Close()
			return nil, fmt.Errorf("op[%d] %s: %w", i, op.Op, err)
		}
	}
	if err := dc.FlushGPU(); err != nil {
		dc.Close()
		return nil, err
	}
	return dc, nil
}

func applyGPUI(dc *render.Context, s *Scene, op *Op, fontRoot string) error {
	switch op.Op {
	case "clear":
		r, g, b, a := rgba4(op.RGBA, 1, 1, 1, 1)
		dc.ClearWithColor(render.RGBA{R: r, G: g, B: b, A: a})
		dc.SetRGBA(r, g, b, a)
		dc.DrawRectangle(0, 0, float64(s.Size[0]), float64(s.Size[1]))
		return dc.Fill()

	case "fill_rect":
		x, y, w, h, err := rect4(op.Rect)
		if err != nil {
			return err
		}
		r, g, b, a := rgba4(op.RGBA, 0, 0, 0, 1)
		dc.SetRGBA(r, g, b, a)
		dc.DrawRectangle(x, y, w, h)
		return dc.Fill()

	case "fill_rrect":
		x, y, w, h, err := rect4(op.Rect)
		if err != nil {
			return err
		}
		r, g, b, a := rgba4(op.RGBA, 0, 0, 0, 1)
		dc.SetRGBA(r, g, b, a)
		dc.DrawRoundedRectangle(x, y, w, h, op.Radius)
		return dc.Fill()

	case "fill_circle":
		r, g, b, a := rgba4(op.RGBA, 0, 0, 0, 1)
		dc.SetRGBA(r, g, b, a)
		dc.DrawCircle(op.CX, op.CY, op.Radius)
		return dc.Fill()

	case "stroke_line":
		r, g, b, a := rgba4(op.RGBA, 0, 0, 0, 1)
		dc.SetRGBA(r, g, b, a)
		dc.SetLineWidth(op.Width)
		dc.DrawLine(op.X, op.Y, op.X2, op.Y2)
		return dc.Stroke()

	case "clip_rect":
		x, y, w, h, err := rect4(op.Rect)
		if err != nil {
			return err
		}
		dc.ClipRect(x, y, w, h)
		return nil

	case "clip_rrect":
		x, y, w, h, err := rect4(op.Rect)
		if err != nil {
			return err
		}
		dc.ClipRoundRect(x, y, w, h, op.Radius)
		return nil

	case "clip_path":
		if len(op.Points) < 2 {
			return fmt.Errorf("clip_path needs points")
		}
		dc.NewSubPath()
		dc.MoveTo(op.Points[0][0], op.Points[0][1])
		for _, p := range op.Points[1:] {
			if len(p) >= 2 {
				dc.LineTo(p[0], p[1])
			}
		}
		if op.Close {
			dc.ClosePath()
		}
		dc.Clip()
		return nil

	case "reset_clip":
		dc.ResetClip()
		return nil

	case "layer_begin":
		blend := parseBlend(op.Blend)
		opac := op.Opacity
		if opac <= 0 {
			opac = 1
		}
		dc.PushLayer(blend, opac)
		return nil

	case "layer_end":
		dc.PopLayer()
		return nil

	case "set_blend":
		dc.SetBlendMode(parseBlend(op.Blend))
		return nil

	case "fill_text":
		r, g, b, a := rgba4(op.RGBA, 0, 0, 0, 1)
		dc.SetRGBA(r, g, b, a)
		if op.FontSize > 0 && s.Font != nil && s.Font.Path != "" {
			fp := s.Font.Path
			if !filepath.IsAbs(fp) {
				fp = filepath.Join(fontRoot, fp)
			}
			_ = dc.LoadFontFace(fp, op.FontSize)
		}
		dc.DrawString(op.Text, op.X, op.Y)
		return nil

	case "push":
		dc.Push()
		return nil
	case "pop":
		dc.Pop()
		return nil
	case "translate":
		dc.Translate(op.X, op.Y)
		return nil
	case "scale":
		sx, sy := op.X, op.Y
		if sy == 0 {
			sy = sx
		}
		dc.Scale(sx, sy)
		return nil
	case "rotate":
		dc.Rotate(op.Angle)
		return nil

	case "draw_image_solid":
		// procedural solid RGBA image
		iw, ih := op.ImgW, op.ImgH
		if iw <= 0 {
			iw = int(op.W)
		}
		if ih <= 0 {
			ih = int(op.H)
		}
		if iw <= 0 || ih <= 0 {
			return fmt.Errorf("draw_image_solid needs img_w/img_h")
		}
		r, g, b, a := rgba4(op.RGBA, 0, 0, 0, 1)
		img, err := render.NewImageBuf(iw, ih, render.FormatRGBA8)
		if err != nil {
			return err
		}
		R := uint8(clamp01(r) * 255)
		G := uint8(clamp01(g) * 255)
		B := uint8(clamp01(b) * 255)
		A := uint8(clamp01(a) * 255)
		for y := 0; y < ih; y++ {
			for x := 0; x < iw; x++ {
				_ = img.SetRGBA(x, y, R, G, B, A)
			}
		}
		dc.DrawImage(img, op.X, op.Y)
		return nil

	default:
		return fmt.Errorf("unsupported op %q", op.Op)
	}
}

func parseBlend(s string) render.BlendMode {
	switch s {
	case "plus", "Plus", "add":
		return render.BlendPlus
	case "multiply", "Multiply":
		return render.BlendMultiply
	case "screen", "Screen":
		return render.BlendScreen
	default:
		return render.BlendNormal
	}
}

func rgba4(v []float64, dr, dg, db, da float64) (r, g, b, a float64) {
	r, g, b, a = dr, dg, db, da
	if len(v) >= 3 {
		r, g, b = v[0], v[1], v[2]
	}
	if len(v) >= 4 {
		a = v[3]
	}
	return
}

func rect4(v []float64) (x, y, w, h float64, err error) {
	if len(v) < 4 {
		return 0, 0, 0, 0, fmt.Errorf("rect needs [x,y,w,h]")
	}
	return v[0], v[1], v[2], v[3], nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
