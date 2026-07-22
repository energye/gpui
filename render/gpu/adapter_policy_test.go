package gpu

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
