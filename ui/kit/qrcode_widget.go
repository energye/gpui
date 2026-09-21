package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// BuildQRCode is the sole QR construction entry.
func BuildQRCode(ctx scope.Ctx, props QRCodeProps) *QRCodeInstance {
	in := newQRCodeInstance(ctx, props, DefaultQRCodeGenerateConfig())
	in.Mount(ctx)
	return in
}

// BuildQRCodeWithGenerate binds a custom encode implementation.
// Components keep working unchanged: only the generate config swaps.
func BuildQRCodeWithGenerate(ctx scope.Ctx, props QRCodeProps, gen QRCodeGenerateConfig) *QRCodeInstance {
	in := newQRCodeInstance(ctx, props, gen)
	in.Mount(ctx)
	return in
}

// QRCodeSubscribedAspects reports Ctx aspects the QR code reads.
func QRCodeSubscribedAspects() []scope.Aspect {
	return []scope.Aspect{
		scope.AspectTheme,
		scope.AspectLocale,
	}
}
