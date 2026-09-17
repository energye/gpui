package render

import (
	"testing"
)

func TestResolveAdapterPolicy(t *testing.T) {
	t.Setenv("GPUI_POWER", "")
	t.Setenv("GPUI_LOW_VRAM", "1") // ignored for adapter selection
	if p := ResolveAdapterPolicy(); p != PolicyDefault {
		t.Fatalf("default policy=%v want default (LOW_VRAM ignored)", p)
	}
	t.Setenv("GPUI_POWER", "high")
	if p := ResolveAdapterPolicy(); p != PolicyHigh {
		t.Fatalf("high policy=%v", p)
	}
	t.Setenv("GPUI_POWER", "low")
	if p := ResolveAdapterPolicy(); p != PolicyLow {
		t.Fatalf("low policy=%v", p)
	}
	t.Setenv("GPUI_POWER", "discrete")
	if p := ResolveAdapterPolicy(); p != PolicyHigh {
		t.Fatalf("discrete alias=%v", p)
	}
	t.Setenv("GPUI_POWER", "auto")
	if p := ResolveAdapterPolicy(); p != PolicyDefault {
		t.Fatalf("auto → default got=%v", p)
	}
}

func TestAdapterPolicyString(t *testing.T) {
	if PolicyDefault.String() != "default" {
		t.Fatal(PolicyDefault.String())
	}
	if PolicyHigh.String() != "high" {
		t.Fatal(PolicyHigh.String())
	}
	if PolicyLow.String() != "low" {
		t.Fatal(PolicyLow.String())
	}
	// deprecated aliases share values
	if PolicyNone != PolicyDefault || PolicyAuto != PolicyDefault {
		t.Fatal("None/Auto must alias Default")
	}
	if PolicyHighPerformance != PolicyHigh || PolicyLowPower != PolicyLow {
		t.Fatal("HighPerformance/LowPower must alias High/Low")
	}
}

func TestPresentLevels_Order(t *testing.T) {
	for _, tc := range []struct {
		start AdapterPolicy
		want  []AdapterPolicy // policies in order; last level is software
	}{
		{PolicyHigh, []AdapterPolicy{PolicyHigh, PolicyLow}},
		{PolicyDefault, []AdapterPolicy{PolicyDefault, PolicyLow}},
		{PolicyLow, []AdapterPolicy{PolicyLow}},
	} {
		levels := presentLevels(tc.start)
		if len(levels) != len(tc.want)+1 {
			t.Fatalf("start=%v levels=%d want %d+software", tc.start, len(levels), len(tc.want))
		}
		for i, want := range tc.want {
			if levels[i].software {
				t.Fatalf("start=%v level %d must not be software", tc.start, i)
			}
			if levels[i].policy != want {
				t.Fatalf("start=%v level %d policy=%v want %v", tc.start, i, levels[i].policy, want)
			}
		}
		last := levels[len(levels)-1]
		if !last.software {
			t.Fatalf("start=%v last level must be software fallback", tc.start)
		}
	}
}

// TestLowVRAMWaterline_AutoTrip locks the B4 hands-free switch: at/below
// the 80% ledger waterline a discrete adapter still gets full limits; past
// it the descriptor flips to LowVRAM without GPUI_LOW_VRAM. Pure ledger
// accounting via GPUI_VRAM_BUDGET_MB — no GPU needed.
func TestLowVRAMWaterline_AutoTrip(t *testing.T) {
	t.Setenv("GPUI_LOW_VRAM", "")
	t.Setenv("GPUI_LOW_VRAM_WATERLINE_PCT", "")
	t.Setenv("GPUI_VRAM_BUDGET_MB", "100")
	t.Setenv("GOGPU_RENDER_MODE", "")
	resetVramLedgerForTest(t, 0)
	if lowVRAMWaterlineTripped() {
		t.Fatal("empty ledger must not trip the waterline")
	}
	resetVramLedgerForTest(t, 79*1024*1024)
	if lowVRAMWaterlineTripped() {
		t.Fatal("79MB of 100MB must not trip the 80% waterline")
	}
	resetVramLedgerForTest(t, 80*1024*1024)
	if !lowVRAMWaterlineTripped() {
		t.Fatal("80MB of 100MB must trip the 80% waterline")
	}
	t.Setenv("GPUI_LOW_VRAM_WATERLINE_PCT", "50")
	resetVramLedgerForTest(t, 49*1024*1024)
	if lowVRAMWaterlineTripped() {
		t.Fatal("49MB of 100MB must not trip a 50% waterline")
	}
	resetVramLedgerForTest(t, 50*1024*1024)
	if !lowVRAMWaterlineTripped() {
		t.Fatal("50MB of 100MB must trip a 50% waterline")
	}
	t.Setenv("GPUI_VRAM_BUDGET_MB", "0")
	resetVramLedgerForTest(t, 1<<62)
	if lowVRAMWaterlineTripped() {
		t.Fatal("disabled budget must never trip")
	}
}
