package prim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIcon_AntdLibrary(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "icon_antd_cases.json"))
	if err != nil {
		// prim testdata lives one level up for shared cases.
		raw, err = os.ReadFile(filepath.Join("..", "..", "testdata", "icon_antd_cases.json"))
		if err != nil {
			t.Skipf("antd cases missing: %v", err)
		}
	}
	var wrap struct {
		Cases []struct {
			ID       string   `json:"id"`
			Total    int      `json:"total"`
			Outlined int      `json:"outlined"`
			Filled   int      `json:"filled"`
			Twotone  int      `json:"twotone"`
			Key      string   `json:"key"`
			Name     string   `json:"name"`
			Theme    string   `json:"theme"`
			Paths    int      `json:"paths"`
			Fills    []string `json:"fills"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		t.Fatalf("decode antd cases: %v", err)
	}
	for _, c := range wrap.Cases {
		switch c.ID {
		case "ANT-COUNT":
			if AntdIconCount() != c.Total {
				t.Errorf("total=%d want %d", AntdIconCount(), c.Total)
			}
			got := map[string]int{}
			for i := range IconAntdEntries {
				got[IconAntdEntries[i].Theme]++
			}
			if got["outlined"] != c.Outlined || got["filled"] != c.Filled || got["twotone"] != c.Twotone {
				t.Errorf("themes=%v want o%d f%d t%d", got, c.Outlined, c.Filled, c.Twotone)
			}
		case "ANT-CHECK", "ANT-HOME-F", "ANT-ALERT":
			e := LookupAntdIcon(c.Name, c.Theme)
			if e == nil {
				t.Errorf("%s: missing %s|%s", c.ID, c.Name, c.Theme)
				continue
			}
			if len(e.Paths) != c.Paths {
				t.Errorf("%s: paths=%d want %d", c.ID, len(e.Paths), c.Paths)
			}
		case "ANT-SMILE":
			e := LookupAntdIcon(c.Name, c.Theme)
			if e == nil {
				t.Errorf("%s: missing", c.ID)
				continue
			}
			if len(e.Paths) != c.Paths {
				t.Errorf("%s: paths=%d want %d", c.ID, len(e.Paths), c.Paths)
				continue
			}
			for i, want := range c.Fills {
				if e.Paths[i].Fill != want {
					t.Errorf("%s: path %d fill=%q want %q", c.ID, i, e.Paths[i].Fill, want)
				}
			}
		}
	}
	// Every entry must parse to at least one path.
	bad := 0
	for k, list := range parsedAntdPaths() {
		if len(list) == 0 {
			t.Errorf("parsed empty: %s", k)
			bad++
		}
	}
	if got := len(parsedAntdPaths()); got != AntdIconCount() {
		t.Errorf("parsed=%d want %d", got, AntdIconCount())
	}
}
