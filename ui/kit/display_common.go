package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Package-level helpers for Data Display controls.

func alertIcon(typ string) string {
	switch typ {
	case "success":
		return "✓"
	case "warning":
		return "!"
	case "error":
		return "×"
	default:
		return "i"
	}
}

func alertColor(th *core.Theme, typ string) render.RGBA {
	switch typ {
	case "success":
		if c := th.Color(core.TokenColorSuccess); c.A > 0 {
			return c
		}
		return render.Hex("#52C41A")
	case "warning":
		if c := th.Color(core.TokenColorWarning); c.A > 0 {
			return c
		}
		return render.Hex("#FAAD14")
	case "error":
		if c := th.Color(core.TokenColorError); c.A > 0 {
			return c
		}
		return render.Hex("#FF4D4F")
	default:
		if c := th.Color(core.TokenColorPrimary); c.A > 0 {
			return c
		}
		return render.Hex("#1677FF")
	}
}
