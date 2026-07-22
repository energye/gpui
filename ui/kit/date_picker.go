package kit

import (
	"fmt"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// DatePicker is Input + Calendar panel (simplified).
// https://ant.design/components/date-picker
type DatePicker struct {
	Root        *primitive.Flex
	input       *Input
	cal         *Calendar
	Value       string
	SelectedDay int // 1..31 when chosen
	Range       bool
	StartDay    int // range start (1..31), same-month
	EndDay      int // range end (1..31), same-month
	ShowTime    bool
	Face        text.Face
	OnChange    func(v string)
}

// NewDatePicker creates a date picker.
func NewDatePicker() *DatePicker {
	d := &DatePicker{}
	d.input = NewInput("Select date")
	d.cal = NewCalendar(0, 0)
	d.cal.OnSelect = func(day int) {
		d.applyDay(day)
	}
	d.Root = primitive.Column(d.input.Node(), d.cal.Node())
	d.Root.Gap = 8
	return d
}

// SelectDay chooses a day in the current calendar month (public R2 API).
func (d *DatePicker) SelectDay(day int) {
	if d == nil || day < 1 {
		return
	}
	d.applyDay(day)
}

// SelectRange selects a same-month day range [start, end].
func (d *DatePicker) SelectRange(start, end int) {
	if d == nil {
		return
	}
	if start > end {
		start, end = end, start
	}
	if start < 1 {
		start = 1
	}
	d.Range = true
	d.StartDay = start
	d.EndDay = end
	d.SelectedDay = end
	if d.cal != nil {
		d.cal.SelectedDay = end
		d.cal.rebuild()
	}
	if d.cal != nil {
		v := fmt.Sprintf("%d-%02d-%02d ~ %d-%02d-%02d",
			d.cal.Year, int(d.cal.Month), start,
			d.cal.Year, int(d.cal.Month), end)
		if d.ShowTime {
			v += " 00:00"
		}
		d.Value = v
		if d.input != nil {
			d.input.SetValue(v)
		}
		if d.OnChange != nil {
			d.OnChange(v)
		}
	}
}

func (d *DatePicker) applyDay(day int) {
	if d == nil || d.cal == nil {
		return
	}
	d.SelectedDay = day
	d.cal.SelectedDay = day
	v := fmt.Sprintf("%d-%02d-%02d", d.cal.Year, int(d.cal.Month), day)
	if d.ShowTime {
		v += " 00:00"
	}
	d.Value = v
	if d.input != nil {
		d.input.SetValue(v)
	}
	if d.OnChange != nil {
		d.OnChange(v)
	}
	d.cal.rebuild()
}

// YearMonth returns the embedded calendar year/month.
func (d *DatePicker) YearMonth() (year int, month int) {
	if d == nil || d.cal == nil {
		return 0, 0
	}
	return d.cal.Year, int(d.cal.Month)
}

// Node returns root.
func (d *DatePicker) Node() core.Node {
	if d == nil {
		return nil
	}
	return d.Root
}

// SetFace sets font.
func (d *DatePicker) SetFace(face text.Face) {
	d.Face = face
	if d.input != nil {
		d.input.SetFace(face)
	}
	if d.cal != nil {
		d.cal.SetFace(face)
	}
}
