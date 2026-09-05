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
