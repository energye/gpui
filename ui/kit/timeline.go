package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// TimelineItem is one node on a Timeline.
type TimelineItem struct {
	Label string
	Color render.RGBA
}

// Timeline is Ant Design Timeline.
// https://ant.design/components/timeline
type Timeline struct {
	Root  *primitive.Flex
	Items []TimelineItem
	Face  text.Face
	Theme *core.Theme
}

// NewTimeline creates a vertical timeline.
func NewTimeline(items ...TimelineItem) *Timeline {
	tl := &Timeline{Items: append([]TimelineItem(nil), items...)}
	tl.rebuild()
	return tl
}

// Node returns root.
func (tl *Timeline) Node() core.Node {
	if tl.Root == nil {
		tl.rebuild()
	}
	return tl.Root
}

// SetFace sets font.
func (tl *Timeline) SetFace(face text.Face) {
	tl.Face = face
	tl.rebuild()
}

func (tl *Timeline) rebuild() {
	th := DefaultTheme()
	if tl.Theme != nil {
		th = tl.Theme
	}
	tl.Root = primitive.Column()
	tl.Root.Gap = 0
	tl.Root.CrossAlign = core.CrossStart
	for i, it := range tl.Items {
		dot := primitive.NewBox()
		dot.Width, dot.Height = 10, 10
		dot.Color = it.Color
		if dot.Color.A < 0.3 {
			dot.Color = th.Color(core.TokenColorPrimary)
		}
		lab := primitive.NewText(it.Label)
		lab.FontSize = 14
		lab.Face = tl.Face
		lab.Color = th.Color(core.TokenColorText)
		row := primitive.Row(dot, lab)
		row.Gap = 12
		row.CrossAlign = core.CrossCenter
		tl.Root.AddChild(row)
		if i < len(tl.Items)-1 {
			line := primitive.NewBox()
			line.Width, line.Height = 2, 20
			line.Color = th.Color(core.TokenColorBorder)
			// indent under dot
			pad := primitive.NewDecorated(line)
			pad.Padding = primitive.EdgeInsets{Left: 4}
			pad.BorderWidth = 0
			tl.Root.AddChild(pad)
		}
	}
}
