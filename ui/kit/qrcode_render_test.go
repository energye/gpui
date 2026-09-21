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
}
