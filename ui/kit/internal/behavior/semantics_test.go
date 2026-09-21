package behavior_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/behavior"
	"github.com/energye/gpui/ui/semantics"
	"github.com/energye/gpui/ui/theme"
)

type semanticsFile struct {
	Cases []struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		Role        string `json:"role"`
		WantOK      bool   `json:"wantOK"`
		WantMissing string `json:"wantMissing"`
	} `json:"cases"`
	Tree struct {
		Total int `json:"total"`
		Named int `json:"named"`
	} `json:"tree"`
	MinTouch struct {
		W      float64 `json:"w"`
		H      float64 `json:"h"`
		WantOK bool    `json:"wantOK"`
	} `json:"minTouch"`
	Contrast struct {
		FG struct {
			R float64 `json:"r"`
			G float64 `json:"g"`
			B float64 `json:"b"`
		} `json:"fg"`
		BG struct {
			R float64 `json:"r"`
			G float64 `json:"g"`
			B float64 `json:"b"`
		} `json:"bg"`
		WantRatioMin float64 `json:"wantRatioMin"`
		WantPass     bool    `json:"wantPass"`
	} `json:"contrast"`
}

func loadSemantics(t *testing.T) semanticsFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "semantics_cases.json"))
	if err != nil {
		t.Fatalf("read semantics_cases.json: %v", err)
	}
	var f semanticsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode semantics_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("semantics_cases.json holds no cases")
	}
	return f
}

func TestSemantics_NamedRole(t *testing.T) {
	f := loadSemantics(t)
	for _, c := range f.Cases {
		n := semantics.New(semantics.Role(c.Role), c.Label)
		ok, missing := behavior.AuditOne(n)
		if ok != c.WantOK {
			t.Fatalf("%s: ok=%v want %v", c.Name, ok, c.WantOK)
		}
		if !c.WantOK && missing != c.WantMissing {
			t.Fatalf("%s: missing=%q want %q", c.Name, missing, c.WantMissing)
		}
	}
	// Tree: two named of three total.
	root := semantics.New(semantics.RoleGeneric, "root")
	root.Add(semantics.New(semantics.RoleButton, "Submit"))
	root.Add(semantics.New(semantics.RoleTextField, "Search"))
	root.Add(semantics.New(semantics.RoleButton, ""))
	total, named := behavior.AuditTree(root)
	if total != f.Tree.Total || named != f.Tree.Named {
		t.Fatalf("tree total=%d named=%d want %d/%d", total, named, f.Tree.Total, f.Tree.Named)
	}
}

func TestSemantics_Advisory(t *testing.T) {
	f := loadSemantics(t)
	if got := behavior.MinTouchOK(f.MinTouch.W, f.MinTouch.H); got != f.MinTouch.WantOK {
		t.Fatalf("minTouch=%v want %v", got, f.MinTouch.WantOK)
	}
	fg := theme.Color{R: f.Contrast.FG.R / 255, G: f.Contrast.FG.G / 255, B: f.Contrast.FG.B / 255, A: 1}
	bg := theme.Color{R: f.Contrast.BG.R / 255, G: f.Contrast.BG.G / 255, B: f.Contrast.BG.B / 255, A: 1}
	ratio := behavior.ContrastRatio(fg, bg)
	if ratio < f.Contrast.WantRatioMin {
		t.Fatalf("contrast=%v want >= %v", ratio, f.Contrast.WantRatioMin)
	}
	if got := behavior.PassContrast(ratio); got != f.Contrast.WantPass {
		t.Fatalf("pass=%v want %v", got, f.Contrast.WantPass)
	}
}
