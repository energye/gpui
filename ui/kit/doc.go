// Package kit is the product control surface (Ant Design tokens + composition).
//
// Controls compose ui/primitive only — no OS, no GPU device ownership.
// Prefer shared Style / theme tokens; keep chrome in Decorated + Pressable;
// avoid per-control layout hacks (hit must match paint Offset).
//
// DefaultTheme returns Ant-leaning tokens via ui/skin/default.
package kit

import (
	"github.com/energye/gpui/ui/core"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// Type IDs for plugin/skin hooks.
const (
	TypeButton = "kit.Button"
	TypeText   = "kit.Text"
	TypeIcon   = "kit.Icon"
)

// DefaultTheme returns the default product theme (Ant-leaning tokens + skin).
func DefaultTheme() *core.Theme {
	return skindefault.Theme()
}
