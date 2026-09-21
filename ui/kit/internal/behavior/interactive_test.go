package behavior_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/internal/behavior"
	"github.com/energye/gpui/ui/kit/internal/scope"
)

type interactiveFile struct {
	Activate []struct {
		Name     string `json:"name"`
		Disabled bool   `json:"disabled"`
		Loading  bool   `json:"loading"`
		Want     bool   `json:"want"`
	} `json:"activateCases"`
	Transitions []struct {
		Name        string   `json:"name"`
		Steps       []string `json:"steps"`
		WantClicks  int      `json:"wantClicks"`
		WantHover   bool     `json:"wantHover"`
		WantPressed bool     `json:"wantPressed"`
	} `json:"transitions"`
	FocusVisible struct {
		Name                 string `json:"name"`
		WantFocusedAfterTap  bool   `json:"wantFocusedAfterTap"`
		WantFocusedAfterKeys bool   `json:"wantFocusedAfterKeys"`
	} `json:"focusVisible"`
	DisableClears struct {
		Name        string `json:"name"`
		WantHover   bool   `json:"wantHover"`
		WantPressed bool   `json:"wantPressed"`
		WantFocused bool   `json:"wantFocused"`
	} `json:"disableClears"`
}

func loadInteractive(t *testing.T) interactiveFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "interactive_cases.json"))
	if err != nil {
		t.Fatalf("read interactive_cases.json: %v", err)
	}
	var f interactiveFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode interactive_cases.json: %v", err)
	}
	if len(f.Activate) == 0 {
		t.Fatal("interactive_cases.json holds no activate cases")
	}
	return f
}

func TestInteractive_CanActivate(t *testing.T) {
	f := loadInteractive(t)
	for _, c := range f.Activate {
		in := behavior.NewInteractive(behavior.InteractiveConfig{
			Disabled:  c.Disabled,
			Loading:   c.Loading,
			Focusable: true,
			Label:     c.Name,
		})
		if got := in.CanActivate(); got != c.Want {
			t.Fatalf("%s: CanActivate=%v want %v", c.Name, got, c.Want)
		}
	}
}

func TestInteractive_Transitions(t *testing.T) {
	f := loadInteractive(t)
	for _, c := range f.Transitions {
		in := behavior.NewInteractive(behavior.InteractiveConfig{Focusable: true, Label: c.Name})
		for _, s := range c.Steps {
			switch s {
			case "hover_on":
				in.SetHover(true)
			case "hover_off":
				in.SetHover(false)
			case "press_down":
				in.SetPressed(true)
			case "press_up":
				in.SetPressed(false)
			case "tap":
				in.DirectTap()
			case "disable":
				in.SetDisabled(true)
			case "loading_on":
				in.SetLoading(true)
			default:
				t.Fatalf("%s: unknown step %q", c.Name, s)
			}
		}
		if got := in.Clicks(); got != c.WantClicks {
			t.Fatalf("%s: clicks=%d want %d", c.Name, got, c.WantClicks)
		}
		if got := in.States().Has(scope.StateHover); got != c.WantHover {
			t.Fatalf("%s: hover=%v want %v", c.Name, got, c.WantHover)
		}
		if got := in.States().Has(scope.StatePressed); got != c.WantPressed {
			t.Fatalf("%s: pressed=%v want %v", c.Name, got, c.WantPressed)
		}
	}
}

func TestInteractive_FocusVisible(t *testing.T) {
	f := loadInteractive(t)
	mgr := focus.NewManager()
	in := behavior.NewInteractive(behavior.InteractiveConfig{Focusable: true, Label: "focusable"})
	in.Mount(mgr)
	defer in.Unmount()

	in.DirectTap()
	if got := in.States().Has(scope.StateFocused); got != f.FocusVisible.WantFocusedAfterTap {
		t.Fatalf("tap focused=%v want %v", got, f.FocusVisible.WantFocusedAfterTap)
	}
	if scope.ShowFocusRing(in.States()) {
		t.Fatal("pointer tap must not light the focus ring")
	}
	if !in.FocusNode().RequestFocus() {
		t.Fatal("RequestFocus refused")
	}
	if got := in.States().Has(scope.StateFocused); got != f.FocusVisible.WantFocusedAfterKeys {
		t.Fatalf("keyboard focused=%v want %v", got, f.FocusVisible.WantFocusedAfterKeys)
	}
	if !scope.ShowFocusRing(in.States()) {
		t.Fatal("keyboard focus must light the focus ring")
	}
	before := in.Clicks()
	if !in.HandleKey(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true}) {
		t.Fatal("Enter was not consumed")
	}
	if in.Clicks() != before+1 {
		t.Fatal("Enter must count one click")
	}
}

func TestInteractive_DisableClears(t *testing.T) {
	f := loadInteractive(t)
	mgr := focus.NewManager()
	in := behavior.NewInteractive(behavior.InteractiveConfig{Focusable: true, Label: "clear"})
	in.Mount(mgr)
	defer in.Unmount()

	in.SetHover(true)
	in.SetPressed(true)
	if !in.FocusNode().RequestFocus() {
		t.Fatal("RequestFocus refused")
	}
	in.SetDisabled(true)
	if got := in.States().Has(scope.StateHover); got != f.DisableClears.WantHover {
		t.Fatalf("hover after disable=%v want %v", got, f.DisableClears.WantHover)
	}
	if got := in.States().Has(scope.StatePressed); got != f.DisableClears.WantPressed {
		t.Fatalf("pressed after disable=%v want %v", got, f.DisableClears.WantPressed)
	}
	if got := in.States().Has(scope.StateFocused); got != f.DisableClears.WantFocused {
		t.Fatalf("focused after disable=%v want %v", got, f.DisableClears.WantFocused)
	}
	if scope.ShowFocusRing(in.States()) {
		t.Fatal("disabled must never show the focus ring")
	}
	if in.DirectTap() {
		t.Fatal("disabled tap must not count")
	}
}

func TestInteractive_SemanticsAndHit(t *testing.T) {
	in := behavior.NewInteractive(behavior.InteractiveConfig{Focusable: true, Label: "Submit"})
	n := in.Semantics()
	if n == nil || n.Label != "Submit" || n.Role == "" {
		t.Fatalf("semantics missing name/role: %+v", n)
	}
	if !behavior.Contains(8, 28, 220, 44, 20, 40) {
		t.Fatal("inside point must hit")
	}
	if behavior.Contains(8, 28, 220, 44, 500, 500) {
		t.Fatal("outside point must miss")
	}
}
