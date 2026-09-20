package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestColorModel_ConvertMatchesCases(t *testing.T) {
	cases := loadColorCases(t)
	cv := cases["conversion"].(map[string]any)

	c, ok := kit.ParseColorModelColor(cv["hex_1677ff"].(string))
	if !ok {
		t.Fatal("parse #1677ff must succeed")
	}
	if c.R != int(cv["hex_1677ff_r"].(float64)) || c.G != int(cv["hex_1677ff_g"].(float64)) || c.BL != int(cv["hex_1677ff_b"].(float64)) {
		t.Fatalf("rgb = %d,%d,%d want 22,119,255", c.R, c.G, c.BL)
	}
	h, s, b, _ := c.ToHSB()
	if math.Abs(h-float64(cv["hsb_1677ff_h"].(float64))) > 1 {
		t.Fatalf("h = %v want ~215", h)
	}
	if math.Abs(s-cv["hsb_1677ff_s"].(float64)) > 0.01 {
		t.Fatalf("s = %v want ~0.914", s)
	}
	if math.Abs(b-cv["hsb_1677ff_b"].(float64)) > 0.01 {
		t.Fatalf("b = %v want 1", b)
	}
	if got := c.ToHsbString(); got != cv["hsb_string"].(string) {
		t.Fatalf("hsb string = %q", got)
	}
	if got := c.ToRgbString(); got != cv["rgb_string"].(string) {
		t.Fatalf("rgb string = %q", got)
	}
	if got := c.ToHexString(); got != cv["roundtrip_hex"].(string) {
		t.Fatalf("roundtrip = %q want #1677ff", got)
	}
	// Roundtrip through HSB stays within ±1 per channel.
	r2, g2, b2 := kit.ColorModelHSBToRGB(h, s, b)
	if absInt(r2-c.R) > 1 || absInt(g2-c.G) > 1 || absInt(b2-c.BL) > 1 {
		t.Fatalf("hsb roundtrip = %d,%d,%d want 22,119,255 ±1", r2, g2, b2)
	}
	// Alpha hex parses and re-renders.
	ca, ok := kit.ParseColorModelColor(cv["alpha_hex"].(string))
	if !ok {
		t.Fatal("parse alpha hex must succeed")
	}
	if ca.R != int(cv["alpha_hex_r"].(float64)) || ca.ToHexString() != cv["alpha_hex"].(string) {
		t.Fatalf("alpha roundtrip = %q", ca.ToHexString())
	}
	// White maps to h=0 s=0.
	w := kit.NewColorModelColorRGB(255, 255, 255)
	wh, ws, wb, _ := w.ToHSB()
	if wh != cv["white_hsb_h"].(float64) || ws != cv["white_hsb_s"].(float64) || wb != cv["white_hsb_b"].(float64) {
		t.Fatalf("white hsb = %v,%v,%v", wh, ws, wb)
	}
	// Gradient css + stops normalize + equals.
	stops := kit.NormalizeColorModelStops([]kit.ColorModelStop{
		{Color: kit.NewColorModelColorRGB(255, 0, 0), Percent: 100},
		{Color: c, Percent: 0},
	})
	if stops[0].Percent != 0 || stops[1].Percent != 100 {
		t.Fatalf("stops must sort ascending, got %v", stops)
	}
	if got := kit.GradientColorModelCSS(stops); got != cv["gradient_css"].(string) {
		t.Fatalf("gradient css = %q", got)
	}
	v := kit.ColorModelValue{Gradient: true, Stops: stops}
	if !v.IsGradient() || v.IsEmpty() {
		t.Fatal("gradient value flags wrong")
	}
	if !v.Equals(kit.ColorModelValue{Gradient: true, Stops: stops}) {
		t.Fatal("equals must hold for identical stops")
	}
	// Unknown input fails.
	if _, ok := kit.ParseColorModelColor("not-a-color"); ok {
		t.Fatal("garbage must not parse")
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
