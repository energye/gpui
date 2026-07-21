package core

import "github.com/energye/gpui/render"

// Token keys (semantic · Ant-leaning defaults in skin/default).
const (
	TokenColorPrimary         = "colorPrimary"
	TokenColorPrimaryHover    = "colorPrimaryHover"
	TokenColorPrimaryActive   = "colorPrimaryActive"
	TokenColorText            = "colorText"
	TokenColorTextSecondary   = "colorTextSecondary"
	TokenColorTextInverse     = "colorTextInverse"
	TokenColorBg              = "colorBg"
	TokenColorBgContainer     = "colorBgContainer"
	TokenColorBgLayout        = "colorBgLayout"
	TokenColorBorder          = "colorBorder"
	TokenColorBorderSecondary = "colorBorderSecondary"
	TokenColorError           = "colorError"
	TokenColorSuccess         = "colorSuccess"
	TokenColorWarning         = "colorWarning"
	TokenColorDisabledBg      = "colorDisabledBg"
	TokenColorDisabledText    = "colorDisabledText"
	TokenColorFillSecondary   = "colorFillSecondary"
	TokenColorBgMask          = "colorBgMask"

	TokenFontSize        = "fontSize"
	TokenFontSizeSM      = "fontSizeSM"
	TokenFontSizeLG      = "fontSizeLG"
	TokenControlHeight   = "controlHeight"
	TokenControlHeightSM = "controlHeightSM"
	TokenControlHeightLG = "controlHeightLG"
	TokenPaddingXS       = "paddingXS"
	TokenPaddingSM       = "paddingSM"
	TokenPadding         = "padding"
	TokenPaddingLG       = "paddingLG"
	TokenBorderRadius    = "borderRadius"
	TokenBorderRadiusLG  = "borderRadiusLG"
	TokenBorderRadiusSM  = "borderRadiusSM"
	TokenLineWidth       = "lineWidth"
	TokenMarginXS        = "marginXS"
	TokenMarginSM        = "marginSM"
)

// TokenSet holds semantic design tokens (colors + dimensions).
// Keys are stable strings; values are either render.RGBA or float64.
type TokenSet struct {
	Colors map[string]render.RGBA
	Sizes  map[string]float64
}

// NewTokenSet creates empty maps.
func NewTokenSet() *TokenSet {
	return &TokenSet{
		Colors: make(map[string]render.RGBA),
		Sizes:  make(map[string]float64),
	}
}

// Color returns a color token or zero.
func (t *TokenSet) Color(key string) render.RGBA {
	if t == nil || t.Colors == nil {
		return render.RGBA{}
	}
	return t.Colors[key]
}

// ColorOr returns a color token or fallback.
func (t *TokenSet) ColorOr(key string, fallback render.RGBA) render.RGBA {
	if t == nil || t.Colors == nil {
		return fallback
	}
	if c, ok := t.Colors[key]; ok {
		return c
	}
	return fallback
}

// Size returns a size token or 0.
func (t *TokenSet) Size(key string) float64 {
	if t == nil || t.Sizes == nil {
		return 0
	}
	return t.Sizes[key]
}

// SizeOr returns a size token or fallback.
func (t *TokenSet) SizeOr(key string, fallback float64) float64 {
	if t == nil || t.Sizes == nil {
		return fallback
	}
	if v, ok := t.Sizes[key]; ok {
		return v
	}
	return fallback
}

// Clone shallow-copies token maps.
func (t *TokenSet) Clone() *TokenSet {
	out := NewTokenSet()
	if t == nil {
		return out
	}
	for k, v := range t.Colors {
		out.Colors[k] = v
	}
	for k, v := range t.Sizes {
		out.Sizes[k] = v
	}
	return out
}

// Merge overlays other onto a clone of t (other wins).
func (t *TokenSet) Merge(other *TokenSet) *TokenSet {
	out := t.Clone()
	if other == nil {
		return out
	}
	for k, v := range other.Colors {
		out.Colors[k] = v
	}
	for k, v := range other.Sizes {
		out.Sizes[k] = v
	}
	return out
}

// Theme is TokenSet + optional Skin + density/scale (C-Theme).
// Convenience fields mirror common tokens for quick access.
type Theme struct {
	Tokens *TokenSet
	Skin   Skin
	// Dark hints skin/token choice; density reserved for M2+.
	Dark    bool
	Density string
	Scale   float64

	// Convenience mirrors (kept for M0 call sites).
	ColorPrimary render.RGBA
	ColorText    render.RGBA
	ColorBg      render.RGBA
	FontSize     float64
}

// DefaultTheme returns a light Ant-leaning theme with a full token table.
func DefaultTheme() *Theme {
	tok := AntLightTokens()
	th := &Theme{
		Tokens:       tok,
		Scale:        1,
		ColorPrimary: tok.Color(TokenColorPrimary),
		ColorText:    tok.Color(TokenColorText),
		ColorBg:      tok.Color(TokenColorBg),
		FontSize:     tok.SizeOr(TokenFontSize, 14),
	}
	return th
}

// Color resolves a token color (falls back to convenience fields for primary/text/bg).
func (th *Theme) Color(key string) render.RGBA {
	if th == nil {
		return render.RGBA{}
	}
	if th.Tokens != nil {
		if c, ok := th.Tokens.Colors[key]; ok {
			return c
		}
	}
	switch key {
	case TokenColorPrimary:
		return th.ColorPrimary
	case TokenColorText:
		return th.ColorText
	case TokenColorBg, TokenColorBgContainer:
		return th.ColorBg
	}
	return render.RGBA{}
}

// Size resolves a size token.
func (th *Theme) Size(key string) float64 {
	if th == nil {
		return 0
	}
	if key == TokenFontSize && th.FontSize > 0 && (th.Tokens == nil || th.Tokens.Sizes[key] == 0) {
		return th.FontSize
	}
	if th.Tokens != nil {
		return th.Tokens.Size(key)
	}
	return 0
}

// SizeOr resolves size with fallback.
func (th *Theme) SizeOr(key string, fallback float64) float64 {
	v := th.Size(key)
	if v == 0 {
		return fallback
	}
	return v
}

// Painter returns a skin painter for typeID, or nil.
func (th *Theme) Painter(typeID string) Painter {
	if th == nil || th.Skin == nil {
		return nil
	}
	return th.Skin.Painter(typeID)
}

// AntLightTokens is the default light Ant Design–leaning token table (M1).
func AntLightTokens() *TokenSet {
	t := NewTokenSet()
	// Colors (#1677ff primary family)
	t.Colors[TokenColorPrimary] = render.Hex("#1677FF")
	t.Colors[TokenColorPrimaryHover] = render.Hex("#4096FF")
	t.Colors[TokenColorPrimaryActive] = render.Hex("#0958D9")
	t.Colors[TokenColorText] = render.RGBA{R: 0, G: 0, B: 0, A: 0.88}
	t.Colors[TokenColorTextSecondary] = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	t.Colors[TokenColorTextInverse] = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	t.Colors[TokenColorBg] = render.Hex("#FFFFFF")
	t.Colors[TokenColorBgContainer] = render.Hex("#FFFFFF")
	t.Colors[TokenColorBgLayout] = render.Hex("#F5F5F5")
	t.Colors[TokenColorBorder] = render.Hex("#D9D9D9")
	t.Colors[TokenColorBorderSecondary] = render.Hex("#F0F0F0")
	t.Colors[TokenColorError] = render.Hex("#FF4D4F")
	t.Colors[TokenColorSuccess] = render.Hex("#52C41A")
	t.Colors[TokenColorWarning] = render.Hex("#FAAD14")
	t.Colors[TokenColorDisabledBg] = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
	t.Colors[TokenColorDisabledText] = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	t.Colors[TokenColorFillSecondary] = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	t.Colors[TokenColorBgMask] = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	// Sizes
	t.Sizes[TokenFontSize] = 14
	t.Sizes[TokenFontSizeSM] = 12
	t.Sizes[TokenFontSizeLG] = 16
	t.Sizes[TokenControlHeight] = 32
	t.Sizes[TokenControlHeightSM] = 24
	t.Sizes[TokenControlHeightLG] = 40
	t.Sizes[TokenPaddingXS] = 4
	t.Sizes[TokenPaddingSM] = 8
	t.Sizes[TokenPadding] = 16
	t.Sizes[TokenPaddingLG] = 24
	t.Sizes[TokenBorderRadius] = 6
	t.Sizes[TokenBorderRadiusLG] = 8
	t.Sizes[TokenBorderRadiusSM] = 4
	t.Sizes[TokenLineWidth] = 1
	t.Sizes[TokenMarginXS] = 4
	t.Sizes[TokenMarginSM] = 8
	return t
}
