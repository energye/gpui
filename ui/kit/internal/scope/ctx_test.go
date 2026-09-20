package scope_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

func loadScopeCases(t *testing.T) map[string]any {
	t.Helper()
	p := filepath.Join("testdata", "scope_cases.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse scope cases: %v", err)
	}
	return m
}

func strMap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%s missing or not an object", key)
	}
	return v
}

func numField(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	v, ok := m[key].(float64)
	if !ok {
		t.Fatalf("%s missing or not a number", key)
	}
	return v
}

// TestScope_DefaultCtxMatchesCases checks the F0-1 default context
// against testdata/scope_cases.json (seed + size + radius + alias).
func TestScope_DefaultCtxMatchesCases(t *testing.T) {
	cases := loadScopeCases(t)
	ctx := scope.DefaultCtx()

	if ctx.Size != scope.SizeMedium {
		t.Fatalf("size = %q want medium", ctx.Size)
	}
	if ctx.Dir != scope.DirLTR {
		t.Fatalf("dir = %q want ltr", ctx.Dir)
	}
	if ctx.Disabled {
		t.Fatal("default disabled must be false")
	}
	if !ctx.Motion.MotionEnabled() {
		t.Fatal("default motion must be enabled")
	}
	if ctx.Locale != "zh-CN" {
		t.Fatalf("locale = %q want zh-CN", ctx.Locale)
	}
	if ctx.PopupOverflow != scope.OverflowViewport {
		t.Fatalf("overflow = %q want viewport", ctx.PopupOverflow)
	}
	if ctx.Variant != scope.VariantOutlined {
		t.Fatalf("variant = %q want outlined", ctx.Variant)
	}
	if ctx.RenderEmpty == nil || ctx.HolderRender == nil {
		t.Fatal("empty/holder renders must be set")
	}

	seed := strMap(t, cases, "seed")
	if got := theme.DefaultTokens().FontSize; got != numField(t, seed, "fontSize") {
		t.Fatalf("fontSize = %v want %v", got, numField(t, seed, "fontSize"))
	}
	if got := theme.DefaultTokens().ControlHeight; got != numField(t, seed, "controlHeight") {
		t.Fatalf("controlHeight = %v want %v", got, numField(t, seed, "controlHeight"))
	}

	size := strMap(t, cases, "size")
	tok := theme.DefaultTokens()
	if tok.SizeXS != numField(t, size, "sizeXS") || tok.SizeXXL != numField(t, size, "sizeXXL") {
		t.Fatalf("size scale = %v/%v want %v/%v", tok.SizeXS, tok.SizeXXL,
			numField(t, size, "sizeXS"), numField(t, size, "sizeXXL"))
	}

	radius := strMap(t, cases, "radius")
	if tok.Radius != numField(t, radius, "borderRadius") ||
		tok.RadiusLG != numField(t, radius, "borderRadiusLG") ||
		tok.RadiusSM != numField(t, radius, "borderRadiusSM") {
		t.Fatal("radius scale mismatch")
	}
}

// TestScope_ProviderUseAspect checks copy-on-write plus aspect
// subscription: theme-only edits wake theme readers, and size-only
// edits leave them alone.
func TestScope_ProviderUseAspect(t *testing.T) {
	base := scope.DefaultCtx()
	themed := base.WithTheme(func() theme.Tokens {
		tok := base.Theme
		tok.FontSize = 18
		return tok
	}())

	changed := scope.ChangedAspects(base, themed)
	if !scope.Subscribed(changed, scope.AspectTheme) {
		t.Fatalf("theme change must notify theme readers, got %v", changed)
	}
	if scope.Subscribed(changed, scope.AspectSize) {
		t.Fatalf("theme change must not notify size readers, got %v", changed)
	}

	sized := base.WithSize(scope.SizeLarge)
	changed = scope.ChangedAspects(base, sized)
	if !scope.Subscribed(changed, scope.AspectSize) {
		t.Fatalf("size change must notify size readers, got %v", changed)
	}
	if scope.Subscribed(changed, scope.AspectTheme) {
		t.Fatalf("size change must not notify theme readers, got %v", changed)
	}

	if base.Theme.FontSize != 14 {
		t.Fatal("With* must copy, not mutate the parent")
	}
	if base.Normalize().Locale != "zh-CN" || base.Normalize().Size != scope.SizeMedium {
		t.Fatal("Normalize must fill defaults")
	}
}
