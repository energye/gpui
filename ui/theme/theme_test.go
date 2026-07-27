package theme_test

import (
	"testing"

	"github.com/energye/gpui/ui/theme"
)

func TestTokens_Default(t *testing.T) {
	tok := theme.DefaultTokens()
	if tok.FontSize != 14 || tok.Spacing != 8 {
		t.Fatalf("%+v", tok)
	}
	if tok.Space(2) != 16 {
		t.Fatalf("Space(2)=%v", tok.Space(2))
	}
	if tok.Primary.A != 1 {
		t.Fatal("primary alpha")
	}
}

func TestProvider_OverrideStack(t *testing.T) {
	p := theme.NewProvider(theme.DefaultTokens())
	base := p.Current()
	over := base
	over.Primary.R = 1
	over.FontSize = 18
	p.Push(over)
	if p.Depth() != 1 || p.Current().FontSize != 18 {
		t.Fatalf("push %+v depth=%d", p.Current(), p.Depth())
	}
	if p.Base().FontSize != 14 {
		t.Fatal("base should stay 14")
	}
	p.Pop()
	if p.Depth() != 0 || p.Current().FontSize != 14 {
		t.Fatal("pop")
	}
}

func TestProvider_With(t *testing.T) {
	p := theme.NewProvider(theme.DefaultTokens())
	var seen float64
	tok := p.Current()
	tok.FontSize = 20
	p.With(tok, func() {
		seen = p.Current().FontSize
	})
	if seen != 20 {
		t.Fatalf("seen=%v", seen)
	}
	if p.Current().FontSize != 14 || p.Depth() != 0 {
		t.Fatal("with restore")
	}
}

func TestProvider_DefaultGlobal(t *testing.T) {
	if theme.Default.Current().Spacing <= 0 {
		t.Fatal("Default provider")
	}
}
