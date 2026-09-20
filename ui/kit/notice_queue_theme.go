package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// NoticeQueueResolved carries queue chrome colors resolved from seed.
type NoticeQueueResolved struct {
	MessageBg   theme.Color
	NoticeBg    theme.Color
	Title       theme.Color
	Description theme.Color
	Focus       scope.FocusRing
	Progress    theme.Color
	CloseActive theme.Color
}

// ResolveNoticeQueue resolves message/notice chrome (props > ctx theme > seed).
func ResolveNoticeQueue(seed theme.Tokens) NoticeQueueResolved {
	bg := seed.ColorBgElevated
	if bg.A <= 0 {
		bg = seed.ColorBgContainer
	}
	return NoticeQueueResolved{
		MessageBg:   bg,
		NoticeBg:    bg,
		Title:       seed.ColorTextHeading,
		Description: seed.ColorText,
		Focus:       scope.ResolveFocusRing(seed),
		Progress:    seed.ColorPrimary,
		CloseActive: seed.ColorTextSecondary,
	}
}

// NoticeQueueMessageIconBg resolves the semantic icon color for a tip.
func NoticeQueueMessageIconBg(seed theme.Tokens, t NoticeQueueMessageType) theme.Color {
	switch t {
	case NoticeQueueMessageSuccess:
		return seed.ColorSuccess
	case NoticeQueueMessageError:
		return seed.ColorError
	case NoticeQueueMessageWarning:
		return seed.ColorWarning
	case NoticeQueueMessageLoading:
		return seed.ColorPrimary
	default:
		return seed.ColorPrimary
	}
}

// NoticeQueueNotificationIconBg resolves the semantic icon color for a card.
func NoticeQueueNotificationIconBg(seed theme.Tokens, t NoticeQueueNotificationType) theme.Color {
	switch t {
	case NoticeQueueNotificationSuccess:
		return seed.ColorSuccess
	case NoticeQueueNotificationError:
		return seed.ColorError
	case NoticeQueueNotificationWarning:
		return seed.ColorWarning
	case NoticeQueueNotificationInfo:
		return seed.ColorPrimary
	default:
		return seed.ColorPrimary
	}
}

// NoticeQueueMessageBg applies the shallow style-class root override.
func NoticeQueueMessageBg(seed theme.Tokens, cfg NoticeQueueMessageConfig) theme.Color {
	base := ResolveNoticeQueue(seed).MessageBg
	if cfg.StyleBgSet {
		return theme.RGBA(cfg.StyleBgR, cfg.StyleBgG, cfg.StyleBgB, 1)
	}
	return base
}

// NoticeQueueNotificationBg applies the shallow style-class root override.
func NoticeQueueNotificationBg(seed theme.Tokens, cfg NoticeQueueNotificationConfig) theme.Color {
	base := ResolveNoticeQueue(seed).NoticeBg
	if cfg.StyleBgSet {
		return theme.RGBA(cfg.StyleBgR, cfg.StyleBgG, cfg.StyleBgB, 1)
	}
	return base
}

// NoticeQueueMessageRadius resolves root radius with shallow override.
func NoticeQueueMessageRadius(seed theme.Tokens, cfg NoticeQueueMessageConfig) float64 {
	if cfg.StyleRadiusSet && cfg.StyleRadius > 0 {
		return cfg.StyleRadius
	}
	return seed.RadiusLG
}

// NoticeQueueNotificationRadius resolves card radius with shallow override.
func NoticeQueueNotificationRadius(seed theme.Tokens, cfg NoticeQueueNotificationConfig) float64 {
	if cfg.StyleRadiusSet && cfg.StyleRadius > 0 {
		return cfg.StyleRadius
	}
	return seed.RadiusLG
}
