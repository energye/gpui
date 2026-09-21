package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// QRCodeResolved carries QR chrome colors resolved from seed.
type QRCodeResolved struct {
	Module   theme.Color
	Bg       theme.Color
	RootBg   theme.Color
	Border   theme.Color
	CoverBg  theme.Color
	CoverTxt theme.Color
	Focus    scope.FocusRing
}

// ResolveQRCode resolves module/border/cover colors.
// Explicit color/bgColor strings win; empty or invalid falls back to
// seed tokens (colorText / white root / transparent bg), never a
// hardcoded brand color.
func ResolveQRCode(seed theme.Tokens, colorStr, bgStr string) QRCodeResolved {
	module := seed.ColorText
	if colorStr != "" {
		if c := theme.Hex(colorStr); c.A > 0 {
			module = c
		} else {
			module = theme.Hex("#000000")
		}
	}
	bg := theme.RGBA(0, 0, 0, 0)
	rootBg := theme.Hex("#ffffff")
	if bgStr != "" {
		if c := theme.Hex(bgStr); c.A > 0 {
			bg = c
			rootBg = c
		}
	}
	cover := seed.ColorBgContainer
	cover.A = 0.96
	return QRCodeResolved{
		Module:   module,
		Bg:       bg,
		RootBg:   rootBg,
		Border:   seed.ColorBorderSecondary,
		CoverBg:  cover,
		CoverTxt: seed.ColorText,
		Focus:    scope.ResolveFocusRing(seed),
	}
}
