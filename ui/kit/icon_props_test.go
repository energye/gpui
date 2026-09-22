package kit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type iconCase struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Size     float64 `json:"size"`
	WantEdge float64 `json:"wantEdge"`
	Known    *bool   `json:"known"`
}

func loadIconCases(t *testing.T) []iconCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "icon_cases.json"))
	if err != nil {
		t.Skipf("icon cases missing: %v", err)
	}
	var wrap struct {
		Cases []iconCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		t.Fatalf("decode icon cases: %v", err)
	}
	return wrap.Cases
}

func TestIcon_PRD_Props(t *testing.T) {
	for _, c := range loadIconCases(t) {
		p := DefaultIconProps(c.Name)
		if c.Size != 0 {
			p.Size, p.SizeSet = c.Size, true
		}
		if c.WantEdge != 0 && ResolveIconSize(p) != c.WantEdge {
			t.Errorf("%s: edge=%v want %v", c.ID, ResolveIconSize(p), c.WantEdge)
		}
		if c.Known != nil {
			in := NewIcon(c.Name)
			if in.Known() != *c.Known {
				t.Errorf("%s: known=%v want %v", c.ID, in.Known(), *c.Known)
			}
		}
	}
	if ResolveIconSize(DefaultIconProps("check")) != DefaultIconSize {
		t.Errorf("default edge != 16")
	}
}
