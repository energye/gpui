package behavior

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// Direction facade (F0-5 §6.5): Dir rides in Ctx; icons and arrows
// follow it. Empty copy resolves through Ctx.RenderEmpty with locale.

// IsRTL reports right-to-left layout.
func IsRTL(ctx scope.Ctx) bool { return ctx.Dir == scope.DirRTL }

// MirrorX mirrors a horizontal offset inside width for RTL.
// LTR returns x unchanged.
func MirrorX(x, width float64, dir scope.Direction) float64 {
	if dir == scope.DirRTL {
		return width - x
	}
	return x
}

// ArrowPrevNext returns (prev,next) arrow labels for dir.
// RTL swaps the pair so pagination arrows follow reading order.
func ArrowPrevNext(dir scope.Direction, prev, next string) (string, string) {
	if dir == scope.DirRTL {
		return next, prev
	}
	return prev, next
}

// EmptyText resolves empty copy for component via Ctx. Custom renders
// run first; otherwise it falls back to the built-in empty render so no
// user-facing copy is hardcoded in behavior.
func EmptyText(ctx scope.Ctx, component string) string {
	ctx = ctx.Normalize()
	if ctx.RenderEmpty != nil && component != "" {
		return ctx.RenderEmpty(component)
	}
	return scope.DefaultRenderEmpty(component)
}
