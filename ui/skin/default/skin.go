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
// Decorated chrome is delegated to primitive.PaintDecorated (single source of truth).
func NewSkin() *core.MapSkin {
	s := core.NewMapSkin()
	s.Set(primitive.TypeDecorated, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
		}
	})
	return s
}

// Theme returns core.DefaultTheme with default skin attached.
func Theme() *core.Theme {
	th := core.DefaultTheme()
	th.Skin = NewSkin()
	return th
}
