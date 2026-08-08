package hint

import (
	"strings"
	"testing"
)

// TestVerifierSet 字集完备性：非空、分组结构、字符统计。
func TestVerifierSet(t *testing.T) {
	gs := VerifierSet()
	if len(gs) == 0 {
		t.Fatal("verifier set empty")
	}
	total := 0
	seen := map[string]bool{}
	for _, g := range gs {
		if g.Name == "" || g.Chars == "" {
			t.Fatalf("group has empty name or chars: %+v", g)
		}
		if seen[g.Name] {
			t.Fatalf("duplicate group name %q", g.Name)
		}
		seen[g.Name] = true
		n := len([]rune(g.Chars))
		total += n
		t.Logf("%-22s runes=%3d  %s", g.Name, n, g.Desc)
	}
	t.Logf("TOTAL  runes=%d", total)
	// 覆盖语言族齐全
	for _, want := range []string{"cjk-", "latin-", "thai-", "arabic-", "cyrillic", "greek"} {
		has := false
		for _, g := range gs {
			if strings.HasPrefix(g.Name, want) {
				has = true
			}
		}
		if !has {
			t.Errorf("missing script group prefix %q", want)
		}
	}
}