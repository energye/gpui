package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// UploadResolved carries list chrome colors resolved from seed.
type UploadResolved struct {
	PanelBg  theme.Color
	Text     theme.Color
	SubText  theme.Color
	Selected theme.Color
	Error    theme.Color
	Disabled theme.Color
	Focus    scope.FocusRing
}

// ResolveUpload resolves list chrome (props > ctx theme > seed).
func ResolveUpload(seed theme.Tokens) UploadResolved {
	bg := seed.ColorBgElevated
	if bg.A <= 0 {
		bg = seed.ColorBgContainer
	}
	return UploadResolved{
		PanelBg:  bg,
		Text:     seed.ColorText,
		SubText:  seed.ColorTextSecondary,
		Selected: seed.ColorPrimary,
		Error:    seed.ColorError,
		Disabled: seed.ColorTextDisabled,
		Focus:    scope.ResolveFocusRing(seed),
	}
}

// UploadStatusColor resolves the status tint for one file.
func UploadStatusColor(seed theme.Tokens, s UploadFileStatus) theme.Color {
	r := ResolveUpload(seed)
	if s == UploadStatusError {
		return r.Error
	}
	if s == UploadStatusUploading {
		return r.Selected
	}
	return r.Text
}
