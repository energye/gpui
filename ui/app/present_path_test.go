// UI present-path convention (F4): product window smokes must use
// exboot.RunUIDemand → app.OwnedPresenter. Offline capture tools (ui_ant_compare)
// and engine demos (particle, device_lost, …) are exempt.
//
// This test locks the inventory of UI window examples so a new smoke cannot
// reintroduce a second compositor loop without failing CI.
package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUIWindowSmokesUseRunUIDemand(t *testing.T) {
	// Resolve repo root from this package path (…/ui/app).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// When running as ./ui/app, wd is …/ui/app; examples is ../../examples
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	examples := filepath.Join(root, "examples")
	if st, err := os.Stat(examples); err != nil || !st.IsDir() {
		t.Skip("examples/ not found from", wd)
	}

	// Window product smokes that must call RunUIDemand (OwnedPresenter path).
	must := []string{
		"ui_core_smoke",
		"ui_kit_smoke",
		"ui_kit_b1_smoke",
		"ui_kit_b2_smoke",
		"ui_kit_b3_smoke",
		"ui_kit_m5_smoke",
		"ui_kit_shell",
		"ui_polish_gallery",
	}
	for _, name := range must {
		main := filepath.Join(examples, name, "main.go")
		b, err := os.ReadFile(main)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		src := string(b)
		if !strings.Contains(src, "RunUIDemand") {
			t.Errorf("%s: must use exboot.RunUIDemand (app.OwnedPresenter); no second present loop", name)
		}
		if strings.Contains(src, "layer.NewCompositor") {
			t.Errorf("%s: must not create layer.Compositor directly — use OwnedPresenter via RunUIDemand", name)
		}
	}
}
