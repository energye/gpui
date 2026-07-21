// Package kit is the product control surface (L4).
// Controls are composed from ui/primitive only — no OS, no GPU device ownership.
// Default visual target may align with Ant Design; package name is kit, not antd.
//
// M1 B0: Button, Text, Icon.
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
