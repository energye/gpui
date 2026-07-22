package kit

import (
	"fmt"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Calendar is a simple month grid (Ant Calendar simplified).
// https://ant.design/components/calendar
type Calendar struct {
	Root        *primitive.Flex
	Year        int
	Month       time.Month // 1-12
	SelectedDay int        // 0 = none
	Face        text.Face
	Theme       *core.Theme
	OnSelect    func(day int)
}

// NewCalendar creates a calendar for the given year/month (0 → now).
func NewCalendar(year int, month time.Month) *Calendar {
	if year == 0 {
		now := time.Now()
		year, month = now.Year(), now.Month()
	}
	c := &Calendar{Year: year, Month: month}
	c.rebuild()
	return c
}

// Node returns root.
func (c *Calendar) Node() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// SetFace sets font.
func (c *Calendar) SetFace(face text.Face) {
	c.Face = face
	c.rebuild()
}

// SelectDay selects a day in the current month (1..daysInMonth).
func (c *Calendar) SelectDay(day int) {
	if day < 1 {
		return
	}
	max := daysInMonth(c.Year, c.Month)
	if day > max {
		day = max
	}
	c.SelectedDay = day
	if c.OnSelect != nil {
		c.OnSelect(day)
	}
	c.rebuild()
}

// SetMonth sets year/month and rebuilds the grid.
func (c *Calendar) SetMonth(year int, month time.Month) {
	if year == 0 {
		year = c.Year
	}
	if month < 1 {
		month = 1
	}
	if month > 12 {
		month = 12
	}
	c.Year, c.Month = year, month
	c.rebuild()
}

func (c *Calendar) rebuild() {
	th := DefaultTheme()
	if c.Theme != nil {
		th = c.Theme
	}
	prev := NewButton("<")
	prev.SetType(ButtonText)
	prev.SetFace(c.Face)
	prev.SetOnClick(func() {
		m := c.Month - 1
		y := c.Year
		if m < 1 {
			m = 12
			y--
		}
		c.Year, c.Month = y, m
		c.rebuild()
		if c.Root != nil {
			c.Root.MarkNeedsLayout()
		}
	})
	next := NewButton(">")
	next.SetType(ButtonText)
	next.SetFace(c.Face)
	next.SetOnClick(func() {
		m := c.Month + 1
		y := c.Year
		if m > 12 {
			m = 1
			y++
		}
		c.Year, c.Month = y, m
		c.rebuild()
		if c.Root != nil {
			c.Root.MarkNeedsLayout()
		}
	})
	title := primitive.NewText(fmt.Sprintf("%d-%02d", c.Year, int(c.Month)))
	title.FontSize = 16
	title.Face = c.Face
	title.Color = th.Color(core.TokenColorText)
	titleRow := primitive.Row(prev.Node(), title, next.Node())
	titleRow.Gap = 8
	titleRow.CrossAlign = core.CrossCenter
	// Week header
	head := primitive.Row()
	head.Gap = 4
	for _, d := range []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"} {
		lab := primitive.NewText(d)
		lab.FontSize = 12
		lab.Face = c.Face
		lab.Color = th.Color(core.TokenColorTextSecondary)
		box := primitive.NewDecorated(lab)
		box.Width, box.Height = 32, 24
		box.SetCenterContent(true)
		box.StretchChild = true
		box.BorderWidth = 0
		head.AddChild(box)
	}
	// Days
	first := time.Date(c.Year, c.Month, 1, 0, 0, 0, 0, time.UTC)
	startPad := int(first.Weekday())
	daysIn := daysInMonth(c.Year, c.Month)
	grid := primitive.Column()
	grid.Gap = 2
	var row *primitive.Flex
	cell := 0
	for i := 0; i < startPad; i++ {
		if row == nil {
			row = primitive.Row()
			row.Gap = 4
		}
		ph := primitive.NewBox()
		ph.Width, ph.Height = 32, 28
		row.AddChild(ph)
		cell++
	}
	for d := 1; d <= daysIn; d++ {
		if row == nil {
			row = primitive.Row()
			row.Gap = 4
		}
		d := d
		lab := primitive.NewText(fmt.Sprintf("%d", d))
		lab.FontSize = 13
		lab.Face = c.Face
		lab.Color = th.Color(core.TokenColorText)
		dec := primitive.NewDecorated(lab)
		dec.Width, dec.Height = 32, 28
		dec.Radius = 4
		dec.SetCenterContent(true)
		dec.StretchChild = true
		dec.BorderWidth = 0
		if d == c.SelectedDay {
			dec.Background = th.Color(core.TokenColorPrimary)
			lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		p := primitive.NewPressable(dec)
		p.ShowFocusRing = false
		p.Click = func() {
			c.SelectedDay = d
			if c.OnSelect != nil {
				c.OnSelect(d)
			}
			c.rebuild()
			if c.Root != nil {
				c.Root.MarkNeedsLayout()
			}
		}
		row.AddChild(p)
		cell++
		if cell%7 == 0 {
			grid.AddChild(row)
			row = nil
		}
	}
	if row != nil {
		grid.AddChild(row)
	}
	c.Root = primitive.Column(titleRow, head, grid)
	c.Root.Gap = 8
	c.Root.CrossAlign = core.CrossStart
	c.Root.Padding = primitive.All(8)
}
