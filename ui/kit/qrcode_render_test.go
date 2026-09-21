package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestQRCode_GeometryMatchesCases(t *testing.T) {
	cases := loadQRCodeCases(t)
	geo := cases["geometry"].(map[string]any)

	if kit.QRCodeDefaultSize != geo["size"].(float64) {
		t.Fatalf("size = %v want 160", kit.QRCodeDefaultSize)
	}
	if kit.ResolveQRCodePadding(true) != geo["pad_bordered"].(float64) {
		t.Fatal("bordered pad must be 12")
	}
	if kit.ResolveQRCodePadding(false) != geo["pad_borderless"].(float64) {
		t.Fatal("borderless pad must be 0")
	}
	lay := kit.ComputeQRCodeLayout(160, true, 40, 40, true)
	if lay.Content.W != geo["content_160_bordered"].(float64) {
		t.Fatalf("content = %v want 136", lay.Content.W)
	}
	if lay.Radius != geo["radius_bordered"].(float64) {
		t.Fatalf("radius = %v want 8", lay.Radius)
	}
	bare := kit.ComputeQRCodeLayout(200, false, 0, 0, false)
	if bare.Outer.W != 200 || bare.Pad != 0 || bare.Radius != 0 || bare.HasIcon {
		t.Fatalf("borderless 200 = %+v", bare)
	}
	// Icon centers in content: (160-2*12-40)/2 + 12 = 60.
	if lay.Icon.X != 60 || lay.Icon.Y != 60 {
		t.Fatalf("icon at %v,%v want 60,60", lay.Icon.X, lay.Icon.Y)
	}
	// Module math: 33 modules fill 136 content exactly.
	mod := kit.QRCodeModuleSize(136, 33, 0)
	if mod <= 0 || mod*33 != 136 {
		t.Fatalf("module = %v", mod)
	}
	x, y := kit.QRCodeModuleXY(12, 12, mod, 0, 0, 0)
	if x != 12 || y != 12 {
		t.Fatalf("first module at %v,%v want 12,12", x, y)
	}
	if kit.QRCodePaddedSize(33, 0) != 33 {
		t.Fatal("margin 0 keeps 33")
	}
	if kit.QRCodeLineWidth != geo["line"].(float64) {
		t.Fatal("lineWidth must be 1")
	}
	// Excavate: center 5x5 of hello clears, finder survives, input kept.
	gen := kit.DefaultQRCodeGenerateConfig()
	hello, err := gen.Encode("hello", kit.QRErrorLevelM)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	mod = kit.QRCodeModuleSize(136, len(hello), 0)
	cx := (136 - 5*mod) / 2
	dug := kit.ExcavateQRCodeIcon(hello, cx, cx, 5*mod, 5*mod, mod)
	cleared := 0
	for r := 0; r < len(dug); r++ {
		for c := 0; c < len(dug); c++ {
			if hello[r][c] && !dug[r][c] {
				cleared++
			}
			if !hello[r][c] && dug[r][c] {
				t.Fatalf("excavate must never darken %d,%d", r, c)
			}
		}
	}
	if cleared == 0 {
		t.Fatal("excavate must clear covered dark modules")
	}
	if !dug[0][0] || dug[1][1] {
		t.Fatal("finder must survive excavate")
	}
	// Margin 2 pads the edge by 4.
	if kit.QRCodePaddedSize(len(hello), 2) != len(hello)+4 {
		t.Fatal("margin 2 must pad 4")
	}
	mx, my := kit.QRCodeModuleXY(12, 12, mod, 0, 0, 2)
	if mx != 12+2*mod || my != 12+2*mod {
		t.Fatalf("margined origin = %v,%v", mx, my)
	}
}
