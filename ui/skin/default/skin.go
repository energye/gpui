// Package skindefault provides the default Ant-leaning TokenSet and Skin (L3c).
// Import as skindefault to avoid clashing with package name "default".
//
//	import skindefault "github.com/energye/gpui/ui/skin/default"
package skindefault

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Tokens returns a clone of the Ant light token table.
func Tokens() *core.TokenSet {
	return core.AntLightTokens()
}

// NewSkin builds the default map skin with painters for common primitives.
func NewSkin() *core.MapSkin {
	s := core.NewMapSkin()
	// Decorated: default painter is already on the node; optional override hook.
	s.Set(primitive.TypeDecorated, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			// Delegate to node default by clearing skin temporarily is awkward;
			// call public paint path: re-use node fields.
			paintDecorated(pc, d)
		}
	})
	return s
}

func paintDecorated(pc *core.PaintContext, d *primitive.Decorated) {
	// Mirror Decorated.paintDefault via public fields.
	sz := d.Size()
	bg := d.Background
	bd := d.BorderColor
	if pc.Theme != nil {
		if d.BackgroundToken != "" {
			if c := pc.Theme.Color(d.BackgroundToken); c.A > 0 {
				bg = c
			}
		}
		if d.BorderToken != "" {
			if c := pc.Theme.Color(d.BorderToken); c.A > 0 {
				bd = c
			}
		}
	}
	if bg.A > 0 {
		pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, d.Radius, bg)
	}
	if d.BorderWidth > 0 && bd.A > 0 {
		pc.StrokeLocalRoundRect(0, 0, sz.Width, sz.Height, d.Radius, d.BorderWidth, bd)
	}
}

// Theme returns core.DefaultTheme with default skin attached.
func Theme() *core.Theme {
	th := core.DefaultTheme()
	th.Skin = NewSkin()
	return th
}
