package kit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestCoverage_MirrorsBoard(t *testing.T) {
	if len(kit.Controls) != 72 {
		t.Fatalf("controls=%d want 72 (71 kit + util)", len(kit.Controls))
	}
	seen := map[string]bool{}
	waves := map[string]int{}
	for _, c := range kit.Controls {
		if c.Name == "" || c.Doc == "" || c.Wave == "" {
			t.Fatalf("empty row %+v", c)
		}
		if seen[c.Name] {
			t.Fatalf("dup %s", c.Name)
		}
		seen[c.Name] = true
		waves[c.Wave]++
		if c.Name != "util" && c.Status != kit.NotStarted {
			t.Fatalf("%s status=%s want 未开工 (W0 only closes foundation)", c.Name, c.Status)
		}
		if c.P1 == "" && c.Name != "util" {
			t.Fatalf("%s missing P1 pointer", c.Name)
		}
		// Spec file must exist.
		if _, err := os.Stat(filepath.Join("..", "..", c.Doc)); err != nil {
			t.Fatalf("doc missing %s: %v", c.Doc, err)
		}
		// Kit dir must exist except util (no UI, not in kit).
		if c.Name != "util" {
			if _, err := os.Stat(c.Name); err != nil {
				t.Fatalf("kit dir missing %s: %v", c.Name, err)
			}
		}
	}
	// Wave counts per README board (icon W0=1, W1=19, W2=20, W3=18, W4=11, W5=2, any=1).
	want := map[string]int{"W0": 1, "W1": 19, "W2": 20, "W3": 18, "W4": 11, "W5": 2, "any": 1}
	for w, n := range want {
		if waves[w] != n {
			t.Fatalf("wave %s = %d want %d", w, waves[w], n)
		}
	}
	if len(kit.FoundationW0) != 5 {
		t.Fatalf("foundation=%d want 5", len(kit.FoundationW0))
	}
	for _, f := range kit.FoundationW0 {
		if f.Status != kit.P0Done {
			t.Fatalf("foundation %s status=%s want 首批完", f.Item, f.Status)
		}
	}
}

func TestCoverage_ByName(t *testing.T) {
	if kit.ByName("button") == nil || kit.ByName("no-such") != nil {
		t.Fatal("ByName")
	}
}
