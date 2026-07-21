package gpu

import (
	"testing"
)

func TestResolveAdapterPolicy(t *testing.T) {
	t.Setenv("GPUI_POWER", "")
	t.Setenv("GPUI_LOW_VRAM", "")
	t.Setenv("GPUI_AUTO_VRAM", "")
	if p := ResolveAdapterPolicy(); p != PolicyHighPerformance {
		t.Fatalf("default policy=%v want high (discrete-first)", p)
	}
	t.Setenv("GPUI_POWER", "high")
	if p := ResolveAdapterPolicy(); p != PolicyHighPerformance {
		t.Fatalf("high policy=%v", p)
	}
	t.Setenv("GPUI_POWER", "low")
	if p := ResolveAdapterPolicy(); p != PolicyLowPower {
		t.Fatalf("low policy=%v", p)
	}
	t.Setenv("GPUI_POWER", "")
	t.Setenv("GPUI_LOW_VRAM", "1")
	if p := ResolveAdapterPolicy(); p != PolicyLowPower {
		t.Fatalf("low_vram policy=%v", p)
	}
	t.Setenv("GPUI_LOW_VRAM", "")
	t.Setenv("GPUI_POWER", "auto")
	if p := ResolveAdapterPolicy(); p != PolicyAuto {
		t.Fatalf("auto policy=%v", p)
	}
}

func TestAdapterPolicyString(t *testing.T) {
	if PolicyAuto.String() != "auto" {
		t.Fatal(PolicyAuto.String())
	}
	if PolicyHighPerformance.String() != "high" {
		t.Fatal(PolicyHighPerformance.String())
	}
}
