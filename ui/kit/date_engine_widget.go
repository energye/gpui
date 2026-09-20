package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// BuildDateEngine is the sole engine construction entry (std impl).
func BuildDateEngine(ctx scope.Ctx, props DateEngineProps) *DateEngineInstance {
	in := newDateEngineInstance(ctx, props, DefaultDateEngineGenerateConfig())
	in.Mount(ctx)
	return in
}

// BuildDateEngineWithGenerate binds a custom date implementation.
// Components keep working unchanged: only the generate config swaps.
func BuildDateEngineWithGenerate(ctx scope.Ctx, props DateEngineProps, gen DateEngineGenerateConfig) *DateEngineInstance {
	in := newDateEngineInstance(ctx, props, gen)
	in.Mount(ctx)
	return in
}

// DateEngineSubscribedAspects reports Ctx aspects the engine reads.
func DateEngineSubscribedAspects() []scope.Aspect {
	return []scope.Aspect{
		scope.AspectTheme,
		scope.AspectLocale,
	}
}
