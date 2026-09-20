package scope_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// TestScope_ResolvePriority checks the three-level token merge:
// explicit call-site value beats the Ctx theme, which beats seed.
func TestScope_ResolvePriority(t *testing.T) {
	if v, from := scope.Merged("a", "b", "c"); v != "a" || from != scope.PriorityProps {
		t.Fatalf("props must win, got %q/%v", v, from)
	}
	if v, from := scope.Merged("", "b", "c"); v != "b" || from != scope.PriorityTheme {
		t.Fatalf("theme must beat seed, got %q/%v", v, from)
	}
	if v, from := scope.Merged("", "", "c"); v != "c" || from != scope.PrioritySeed {
		t.Fatalf("seed is the fallback, got %q/%v", v, from)
	}

	red := theme.Hex("#ff4d4f")
	hover := theme.Hex("#ff7875")
	seed := theme.Hex("#1677ff")
	if got := scope.MergedColor(red, true, hover, true, seed); got != red {
		t.Fatalf("explicit color must win, got %+v", got)
	}
	if got := scope.MergedColor(red, false, hover, true, seed); got != hover {
		t.Fatalf("theme color must beat seed, got %+v", got)
	}

	if got := scope.ResolveColor(&red, &hover, seed); got != red {
		t.Fatal("ResolveColor must prefer props")
	}
	if got := scope.ResolveColor(nil, &hover, seed); got != hover {
		t.Fatal("ResolveColor must fall back to theme")
	}
	if got := scope.ResolveColor(nil, nil, seed); got != seed {
		t.Fatal("ResolveColor must fall back to seed")
	}

	big, small := 18.0, 12.0
	if got := scope.ResolveFloat(&big, &small, 14); got != 18 {
		t.Fatal("ResolveFloat must prefer props")
	}
	if got := scope.ResolveFloat(nil, nil, 14); got != 14 {
		t.Fatal("ResolveFloat must fall back to seed")
	}

	if !scope.DisabledOr(true, false) || !scope.DisabledOr(false, true) {
		t.Fatal("either disable must disable")
	}
	if scope.DisabledOr(true, false) == false {
		t.Fatal("subtree disable sticks")
	}
	if scope.DisabledOr(false, false) {
		t.Fatal("both clear must stay enabled")
	}
}

// TestScope_FocusRing checks the library-wide keyboard ring: width 3,
// primary border color, offset 1, keyboard focus only, never disabled.
func TestScope_FocusRing(t *testing.T) {
	ring := scope.ResolveFocusRing(theme.DefaultTokens())
	if ring.Width != 3 {
		t.Fatalf("ring width = %v want 3", ring.Width)
	}
	if ring.Offset != 1 {
		t.Fatalf("ring offset = %v want 1", ring.Offset)
	}
	if ring.Color != theme.Hex("#91caff") {
		t.Fatalf("ring color = %+v want #91caff", ring.Color)
	}
	if !scope.ShowFocusRing(scope.StateFocused) {
		t.Fatal("keyboard focus must show the ring")
	}
	if scope.ShowFocusRing(0) {
		t.Fatal("unfocused must not show the ring")
	}
	if scope.ShowFocusRing(scope.StateFocused | scope.StateDisabled) {
		t.Fatal("disabled must never show the ring")
	}
}
