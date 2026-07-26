// Package skindefault provides the default Ant-leaning TokenSet and Skin (L3c).
// Import as skindefault to avoid clashing with package name "default".
//
//	import skindefault "github.com/energye/gpui/ui/skin/default"
package skindefault

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Product typeIDs as string literals to avoid importing ui/kit (cycle).
// Must stay equal to kit.TypeButton / kit.TypeBreadcrumb etc.
const (
	typeButton     = "kit.Button"
	typeBreadcrumb = "kit.Breadcrumb"
	typeInput      = "kit.Input"
)

// Tokens returns a clone of the Ant light token table.
func Tokens() *core.TokenSet {
	return core.AntLightTokens()
}

// NewSkin builds the default map skin with painters for common primitives
// and product chrome hooks.
//
//	TypeDecorated  — generic box chrome
//	kit.Button     — Button Decorated chrome (default = PaintDecorated)
//	kit.Breadcrumb — Breadcrumb Flex root (default = paint children)
//	kit.Input      — Input Decorated chrome
//
// Decorated chrome is delegated to primitive.PaintDecorated (single source of truth).
func NewSkin() *core.MapSkin {
	s := core.NewMapSkin()
	paintDeco := func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
		}
	}
	s.Set(primitive.TypeDecorated, paintDeco)
	// Button tags its Decorated with SkinType=kit.Button so product skins can
	// override only buttons without replacing all Decorated chrome.
	s.Set(typeButton, paintDeco)
	// Breadcrumb root is a Flex tagged SkinType=kit.Breadcrumb.
	s.Set(typeBreadcrumb, func(pc *core.PaintContext, n core.Node) {
		if f, ok := n.(*primitive.Flex); ok {
			f.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeInput, paintDeco)
	return s
}

// Theme returns core.DefaultTheme with default skin attached.
func Theme() *core.Theme {
	th := core.DefaultTheme()
	th.Skin = NewSkin()
	return th
}
