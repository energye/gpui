package kit

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Steps is Ant Design Steps (horizontal or vertical).
// https://ant.design/components/steps
type Steps struct {
	Root    *primitive.Flex
	Items   []string
	Current int
	// Statuses per-step: wait | process | finish | error (empty → derived from Current).
	Statuses []string
	// Direction: "horizontal" (default) or "vertical".
	Direction string
	Face      text.Face
	Theme     *core.Theme
}

// NewSteps creates steps from titles.
func NewSteps(items ...string) *Steps {
	s := &Steps{Items: append([]string(nil), items...), Direction: "horizontal"}
	s.rebuild()
	return s
}

// Node returns root.
func (s *Steps) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// SetCurrent sets active step index.
func (s *Steps) SetCurrent(i int) {
	s.Current = i
	s.rebuild()
}

// SetStatus sets status for step i (wait|process|finish|error).
func (s *Steps) SetStatus(i int, status string) {
	if i < 0 {
		return
	}
	for len(s.Statuses) <= i {
		s.Statuses = append(s.Statuses, "")
	}
	s.Statuses[i] = status
	s.rebuild()
}

// SetFace sets font.
func (s *Steps) SetFace(face text.Face) {
	s.Face = face
	s.rebuild()
}

func (s *Steps) stepStatus(i int) string {
	if i >= 0 && i < len(s.Statuses) && s.Statuses[i] != "" {
		return s.Statuses[i]
	}
	if i < s.Current {
		return "finish"
	}
	if i == s.Current {
		return "process"
	}
	return "wait"
}

func (s *Steps) rebuild() {
	th := DefaultTheme()
	if s.Theme != nil {
		th = s.Theme
	}
	vertical := s.Direction == "vertical"
	if vertical {
		s.Root = primitive.Column()
	} else {
		s.Root = primitive.Row()
	}
	s.Root.Gap = 8
	s.Root.CrossAlign = core.CrossCenter
	primary := th.Color(core.TokenColorPrimary)
	muted := th.Color(core.TokenColorTextSecondary)
	errC := th.Color(core.TokenColorError)
	for i, title := range s.Items {
		i, title := i, title
		st := s.stepStatus(i)
		num := primitive.NewText(fmt.Sprintf("%d", i+1))
		num.FontSize = 12
		num.Face = s.Face
		num.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		dot := primitive.NewDecorated(num)
		dot.Width, dot.Height = 24, 24
		dot.Radius = 12
		dot.SetCenterContent(true)
		dot.StretchChild = true
		switch st {
		case "finish", "process":
			dot.Background = primary
		case "error":
			dot.Background = errC
		default:
			dot.Background = muted
			if dot.Background.A < 0.2 {
				dot.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
			}
		}
		lab := primitive.NewText(title)
		lab.FontSize = 14
		lab.Face = s.Face
		if st == "wait" {
			lab.Color = muted
		} else {
			lab.Color = th.Color(core.TokenColorText)
		}
		cell := primitive.Row(dot, lab)
		cell.Gap = 8
		cell.CrossAlign = core.CrossCenter
		s.Root.AddChild(cell)
		if i < len(s.Items)-1 {
			line := primitive.NewBox()
			if vertical {
				line.Width, line.Height = 1, 24
			} else {
				line.Width, line.Height = 40, 1
			}
			if st == "finish" {
				line.Color = primary
			} else {
				line.Color = th.Color(core.TokenColorBorder)
			}
			s.Root.AddChild(line)
		}
	}
}
