//go:build !nogpu

package gpu

import (
	"math"
	"strings"
	"testing"
)

// P3.1b-1 能力钉：sRGB↔linear 是整条 opt-in 线性目标的唯一转换真源，
// GPU WGSL 与 CPU Go 必须逐值一致。先写会红（函数不存在），再实现。
func TestP31bColorSpaceConversions(t *testing.T) {
	cases := []struct {
		name   string
		srgb   float64
		linear float64
	}{
		{"zero", 0, 0},
		{"threshold", 0.04045, 0.0031308},
		{"mid", 0.5, 0.21404114048223255},
		{"quarter", 0.25, 0.05087608817155679},
		{"high", 0.9, 0.787412020653578},
		{"one", 1, 1},
	}
	for _, c := range cases {
		if got := srgbToLinear(float32(c.srgb)); math.Abs(float64(got)-c.linear) > 1e-6 {
			t.Fatalf("%s: srgbToLinear(%v) = %v, want %v", c.name, c.srgb, got, c.linear)
		}
		if got := linearToSRGB(float32(c.linear)); math.Abs(float64(got)-c.srgb) > 1e-6 {
			t.Fatalf("%s: linearToSRGB(%v) = %v, want %v", c.name, c.linear, got, c.srgb)
		}
	}
	for i := 0; i <= 1000; i++ {
		v := float32(i) / 1000
		if got := linearToSRGB(srgbToLinear(v)); math.Abs(float64(got-v)) > 1e-5 {
			t.Fatalf("round-trip %v -> %v", v, got)
		}
	}
	// Out-of-range inputs clamp instead of producing NaN/hidden bands.
	if got := srgbToLinear(-1); got != 0 {
		t.Fatalf("srgbToLinear(-1) = %v, want 0", got)
	}
	if got := srgbToLinear(2); got != 1 {
		t.Fatalf("srgbToLinear(2) = %v, want 1", got)
	}
	if got := linearToSRGB(-0.5); got != 0 {
		t.Fatalf("linearToSRGB(-0.5) = %v, want 0", got)
	}
	if got := linearToSRGB(2); got != 1 {
		t.Fatalf("linearToSRGB(2) = %v, want 1", got)
	}
}

func TestP31bColorSpaceWGSLSingleSource(t *testing.T) {
	if colorSpaceWGSL == "" {
		t.Fatal("colorSpaceWGSL is empty")
	}
	for _, want := range []string{"fn srgb_to_linear", "fn linear_to_srgb"} {
		if !strings.Contains(colorSpaceWGSL, want) {
			t.Fatalf("colorSpaceWGSL missing %q", want)
		}
	}
}
