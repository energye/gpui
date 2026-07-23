package kit

import "github.com/energye/gpui/ui/core"

// themeOf resolves the product theme for a control (F8):
//
//  1. explicit field Theme on the control (highest priority)
//  2. nearest ThemeProvider ancestor (ConfigProvider) via ResolveTheme
//  3. Tree.Theme if mounted
//  4. DefaultTheme()
func themeOf(field *core.Theme, n core.Node) *core.Theme {
	if field != nil {
		return field
	}
	if n != nil {
		if th := core.ResolveTheme(n); th != nil {
			return th
		}
	}
	return DefaultTheme()
}
