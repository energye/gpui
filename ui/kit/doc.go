// Package kit is the product control surface (Ant Design tokens + composition).
//
// Controls compose ui/primitive only — no OS, no GPU device ownership.
// Prefer shared Style / theme tokens; keep chrome in Decorated + Pressable;
// avoid per-control layout hacks (hit must match paint Offset).
//
// # Developer guide (required reading)
//
//	docs/UI_KIT_DEV_GUIDE.md     — contracts for kit work on the stabilized foundation
//	docs/UI_KIT_ANT_V5_SPEC.md   — Ant Design v5 alignment goals, waves, L1–L4 acceptance
//	docs/UI_FOUNDATION_P0.md     — what the foundation delivered (Present, overlays, theme, …)
//	docs/LAYOUT_FOUNDATION.md    — hit == layout == paint
//
// Rules of thumb:
//   - Align Ant via behavior + tokens + in-repo golden (not browser pixel hash).
//   - DefaultXxx + SetXxx for chrome metrics (docs/UI_KIT_DEV_GUIDE.md §0.1) — Ant real defaults; override at use site.
//   - No ContinuousRender for product controls (use Tree.AddTicker).
//   - No per-frame Sync() requirement for popups (Tree.Layout refreshes anchors).
//   - Theme via themeOf / ConfigProvider / Tree.SetTheme + SetThemeHook(rebuild).
//   - True-window present: app.OwnedPresenter or exboot.RunUIDemand only.
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
//   - Theme resolve: theme_resolve.go (themeOf)
//   - Theme entry: DefaultTheme via ui/skin/default.
//   - Coverage table: coverage.go (AntCoverage) — source of truth for Ready/Notes.
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
