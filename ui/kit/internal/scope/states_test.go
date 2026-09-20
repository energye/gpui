package scope_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// TestScope_StatesResolve checks per-state lookup: each combination
// reads its own row, disabled wins over hover/pressed, and render
// code never branches inline.
func TestScope_StatesResolve(t *testing.T) {
	red := theme.Hex("#ff4d4f")
	hover := theme.Hex("#ff7875")
	active := theme.Hex("#d9363e")
	faint := theme.RGBA(0, 0, 0, 0.25)

	table := scope.MapResolver[theme.Color]{
		Values: map[scope.WidgetState]theme.Color{
			0:                                     red,
			scope.StateHover:                      hover,
			scope.StatePressed:                    active,
			scope.StateDisabled:                   faint,
			scope.StateHover | scope.StateFocused: hover,
		},
		Default: red,
	}
	res := scope.DisabledFirstResolver[theme.Color]{Disabled: faint, Rest: table}

	if got := res.Resolve(0); got != red {
		t.Fatalf("idle = %+v want red", got)
	}
	if got := res.Resolve(scope.StateHover); got != hover {
		t.Fatalf("hover = %+v", got)
	}
	if got := res.Resolve(scope.StatePressed); got != active {
		t.Fatalf("pressed = %+v", got)
	}
	if got := res.Resolve(scope.StateHover | scope.StateDisabled); got != faint {
		t.Fatalf("hover+disabled must stay faint, got %+v", got)
	}
	if got := res.Resolve(scope.StateFocused); got != red {
		t.Fatalf("unlisted focused falls back to default, got %+v", got)
	}

	if (scope.StateHover | scope.StatePressed).Has(scope.StateHover) == false {
		t.Fatal("Has must see the hover bit")
	}
	if scope.StateHover.With(scope.StatePressed).Without(scope.StateHover).Has(scope.StateHover) {
		t.Fatal("Without must clear the hover bit")
	}
	if (scope.StaticResolver[string]{Value: "x"}).Resolve(scope.StateHover) != "x" {
		t.Fatal("static resolver must ignore state")
	}
	if (scope.StateResolverFunc[int](func(scope.WidgetState) int { return 7 })).Resolve(0) != 7 {
		t.Fatal("func adapter must delegate")
	}
}
