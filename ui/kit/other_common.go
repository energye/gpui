package kit

import (
	"github.com/energye/gpui/ui/core"
)

// Package-level helpers for Other / app-level utilities.

// Density constants for Theme.Density.
const (
	DensityDefault = "default"
	DensityCompact = "compact"
	DensityLarge   = "large"
)

// ApplyDensity mutates control heights on a theme token set.
func ApplyDensity(th *core.Theme, density string) {
	if th == nil || th.Tokens == nil {
		return
	}
	th.Density = density
	switch density {
	case DensityCompact:
		th.Tokens.Sizes[core.TokenControlHeight] = 24
		th.Tokens.Sizes[core.TokenControlHeightSM] = 20
		th.Tokens.Sizes[core.TokenControlHeightLG] = 32
		th.Tokens.Sizes[core.TokenFontSize] = 12
	case DensityLarge:
		th.Tokens.Sizes[core.TokenControlHeight] = 40
		th.Tokens.Sizes[core.TokenControlHeightSM] = 32
		th.Tokens.Sizes[core.TokenControlHeightLG] = 48
		th.Tokens.Sizes[core.TokenFontSize] = 16
	default:
		th.Tokens.Sizes[core.TokenControlHeight] = 32
		th.Tokens.Sizes[core.TokenControlHeightSM] = 24
		th.Tokens.Sizes[core.TokenControlHeightLG] = 40
		th.Tokens.Sizes[core.TokenFontSize] = 14
	}
}
