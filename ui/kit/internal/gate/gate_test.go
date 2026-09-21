package gate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/gate"
	"github.com/energye/gpui/ui/kit/internal/scope"
)

type gateFile struct {
	Props struct {
		Total   int      `json:"total"`
		Covered int      `json:"covered"`
		Missing []string `json:"missing"`
		WantOK  bool     `json:"wantOK"`
	} `json:"props"`
	State struct {
		States []string `json:"states"`
		WantOK bool     `json:"wantOK"`
	} `json:"state"`
	Wiring struct {
		Imports []string `json:"imports"`
		WantBad []string `json:"wantBad"`
		WantOK  bool     `json:"wantOK"`
	} `json:"wiring"`
	Token struct {
		FromTheme bool `json:"fromTheme"`
		Hardcoded int  `json:"hardcoded"`
		WantOK    bool `json:"wantOK"`
	} `json:"token"`
}

func loadGate(t *testing.T) gateFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "gate_cases.json"))
	if err != nil {
		t.Fatalf("read gate_cases.json: %v", err)
	}
	var f gateFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode gate_cases.json: %v", err)
	}
	return f
}

func TestGate_FourChecks(t *testing.T) {
	f := loadGate(t)
	ok, missing := gate.CheckProps(gate.PropsCoverage{Total: f.Props.Total, Covered: f.Props.Covered, Missing: f.Props.Missing})
	if ok != f.Props.WantOK {
		t.Fatalf("props ok=%v want %v missing=%v", ok, f.Props.WantOK, missing)
	}
	if !ok {
		t.Skipf("props gate red (expected green in seed data)")
	}

	states := map[string]scope.WidgetState{
		"disabled": scope.StateDisabled,
		"hover":    scope.StateHover,
		"loading":  scope.StateLoading,
	}
	var sf gate.StateFlow
	for _, name := range f.State.States {
		s, known := states[name]
		if !known {
			t.Fatalf("unknown state %q", name)
		}
		sf.States = append(sf.States, s)
		sf.Resolved = append(sf.Resolved, true)
	}
	if got := gate.CheckState(sf); got != f.State.WantOK {
		t.Fatalf("state=%v want %v", got, f.State.WantOK)
	}
	// Unresolved entry must fail.
	sf.Resolved[0] = false
	if gate.CheckState(sf) {
		t.Fatal("unresolved state must fail")
	}

	ok, bad := gate.CheckWiring(f.Wiring.Imports)
	if ok != f.Wiring.WantOK {
		t.Fatalf("wiring ok=%v want %v bad=%v", ok, f.Wiring.WantOK, bad)
	}
	if len(bad) != len(f.Wiring.WantBad) {
		t.Fatalf("wiring bad=%v want %v", bad, f.Wiring.WantBad)
	}
	// Direct render import must fail.
	if ok, bad := gate.CheckWiring([]string{"github.com/energye/gpui/render", "github.com/energye/gpui/gpu"}); ok {
		t.Fatalf("render/gpu must fail, got %v", bad)
	}

	if got := gate.CheckToken(gate.TokenUse{FromTheme: f.Token.FromTheme, HardcodedCount: f.Token.Hardcoded}); got != f.Token.WantOK {
		t.Fatalf("token=%v want %v", got, f.Token.WantOK)
	}
	if gate.CheckToken(gate.TokenUse{FromTheme: true, HardcodedCount: 1}) {
		t.Fatal("hardcoded token must fail")
	}
}
