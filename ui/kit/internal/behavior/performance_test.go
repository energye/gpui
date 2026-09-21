package behavior_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/behavior"
)

type perfFile struct {
	Budgets struct {
		MinFPSWall float64 `json:"minFPSWall"`
		MaxP95Ms   float64 `json:"maxP95Ms"`
		FPSWall    float64 `json:"fpsWall"`
		P95Ms      float64 `json:"p95Ms"`
		WantOK     bool    `json:"wantOK"`
	} `json:"budgets"`
	Isolation struct {
		AnimatedDirty bool `json:"animatedDirty"`
		SiblingDirty  bool `json:"siblingDirty"`
		WantOK        bool `json:"wantOK"`
	} `json:"isolation"`
	Cache struct {
		Cap         int      `json:"cap"`
		Keys        []string `json:"keys"`
		WantEvicted string   `json:"wantEvicted"`
		WantLen     int      `json:"wantLen"`
	} `json:"cache"`
	Virtual struct {
		Total  int  `json:"total"`
		Bound  int  `json:"bound"`
		WantOK bool `json:"wantOK"`
	} `json:"virtual"`
}

func loadPerf(t *testing.T) perfFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "performance_cases.json"))
	if err != nil {
		t.Fatalf("read performance_cases.json: %v", err)
	}
	var f perfFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode performance_cases.json: %v", err)
	}
	return f
}

func TestPerformance_BudgetsAndIsolation(t *testing.T) {
	f := loadPerf(t)
	b := behavior.Budgets{MinFPSWall: f.Budgets.MinFPSWall, MaxP95Ms: f.Budgets.MaxP95Ms}
	if got := b.CheckFPS(f.Budgets.FPSWall, f.Budgets.P95Ms); got != f.Budgets.WantOK {
		t.Fatalf("budgets=%v want %v", got, f.Budgets.WantOK)
	}
	if got := behavior.CheckIsolation(f.Isolation.AnimatedDirty, f.Isolation.SiblingDirty); got != f.Isolation.WantOK {
		t.Fatalf("isolation=%v want %v", got, f.Isolation.WantOK)
	}
	if behavior.CheckIsolation(true, true) {
		t.Fatal("dirty sibling must break isolation")
	}
}

func TestPerformance_CacheAndVirtual(t *testing.T) {
	f := loadPerf(t)
	c := behavior.NewCacheBudget(f.Cache.Cap)
	var lastEvicted string
	for _, k := range f.Cache.Keys {
		if ev := c.Set(k); ev != "" {
			lastEvicted = ev
		}
	}
	if lastEvicted != f.Cache.WantEvicted {
		t.Fatalf("evicted=%q want %q", lastEvicted, f.Cache.WantEvicted)
	}
	if c.Len() != f.Cache.WantLen {
		t.Fatalf("len=%d want %d", c.Len(), f.Cache.WantLen)
	}
	if c.Has(f.Cache.WantEvicted) {
		t.Fatalf("%q must be evicted", f.Cache.WantEvicted)
	}
	if got := behavior.VirtualBoundOK(f.Virtual.Total, f.Virtual.Bound); got != f.Virtual.WantOK {
		t.Fatalf("virtual=%v want %v", got, f.Virtual.WantOK)
	}
}
