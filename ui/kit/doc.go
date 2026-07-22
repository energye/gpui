// Package kit is the product control surface (Ant Design tokens + composition).
//
// Controls compose ui/primitive only — no OS, no GPU device ownership.
// Prefer shared Style / theme tokens; keep chrome in Decorated + Pressable;
// avoid per-control layout hacks (hit must match paint Offset).
//
// # File layout
//
//   - One exported control per file (button.go, modal.go, …).
//   - Co-located helpers for that control stay in the same file
//     (e.g. FormItem in form.go, MenuItem in menu.go, sliderHost in slider.go).
//   - Category-shared helpers live in:
//     general_common.go, layout_common.go, navigation_common.go,
//     entry_common.go, display_common.go, feedback_common.go, other_common.go.
//   - Cross-cutting Ant chrome tokens: ant_chrome.go
//   - Theme entry: DefaultTheme via ui/skin/default.
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
