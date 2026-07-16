package render

import (
	"testing"
)

// TestPushPopLayer tests basic layer push/pop functionality.
func TestPushPopLayer(t *testing.T) {
	dc := NewContext(100, 100)
	originalPixmap := dc.pixmap

	// Normal is F1 opacity-group: no isolation pixmap switch.
	dc.PushLayer(BlendNormal, 0.5)
	if dc.layerStack == nil || len(dc.layerStack.layers) != 1 {
		t.Fatalf("Normal PushLayer should push opacity-group, layers=%v", dc.layerStack)
	}
	if !dc.layerStack.layers[0].opacityGroup {
		t.Error("Normal PushLayer should be opacity-group")
	}
	if dc.pixmap != originalPixmap {
		t.Error("opacity-group should keep parent pixmap")
	}
	dc.PopLayer()
	if len(dc.layerStack.layers) != 0 {
		t.Errorf("Expected 0 layers after pop, got %d", len(dc.layerStack.layers))
	}

	// Advanced blend allocates isolation surface.
	dc.PushLayer(BlendMultiply, 1.0)
	if dc.pixmap == originalPixmap {
		t.Error("isolated PushLayer should create a new pixmap")
	}
	if dc.basePixmap == nil {
		t.Error("isolated PushLayer should save base pixmap")
	}
	dc.PopLayer()
	if dc.pixmap != originalPixmap {
		t.Error("PopLayer should restore original pixmap")
	}
	if dc.basePixmap != nil {
		t.Error("PopLayer should clear base pixmap")
	}
}

// TestNestedLayers tests nested push/pop operations.
func TestNestedLayers(t *testing.T) {
	dc := NewContext(100, 100)
	base := dc.pixmap

	// Outer Normal = opacity-group (same pixmap); inner Multiply = isolation.
	dc.PushLayer(BlendNormal, 0.8)
	dc.PushLayer(BlendMultiply, 0.5)
	if dc.pixmap == base {
		t.Error("isolated nested layer should switch pixmap")
	}
	if len(dc.layerStack.layers) != 2 {
		t.Errorf("Expected 2 layers, got %d", len(dc.layerStack.layers))
	}
	dc.PopLayer()
	if dc.pixmap != base {
		t.Error("after popping isolation, opacity-group should keep base pixmap")
	}
	if len(dc.layerStack.layers) != 1 {
		t.Errorf("Expected 1 layer after first pop, got %d", len(dc.layerStack.layers))
	}
	dc.PopLayer()
	if len(dc.layerStack.layers) != 0 {
		t.Errorf("Expected 0 layers after second pop, got %d", len(dc.layerStack.layers))
	}
}

// TestLayerCompositing tests layer compositing with SetPixel.
func TestLayerCompositing(t *testing.T) {
	dc := NewContext(10, 10)
	dc.ClearWithColor(White)

	// Use isolation blend so SetPixel writes a true layer surface.
	dc.PushLayer(BlendMultiply, 1.0)
	dc.SetPixel(5, 5, RGBA{R: 1, G: 0, B: 0, A: 1})

	layerPixel := dc.pixmap.GetPixel(5, 5)
	if layerPixel.R != 1.0 {
		t.Fatalf("Layer pixel should be red, got R=%f", layerPixel.R)
	}

	dc.PopLayer()

	_ = dc.FlushGPU()

	pixel := dc.pixmap.GetPixel(5, 5)
	tolerance := 0.1
	if abs(pixel.R-1.0) > tolerance {
		t.Errorf("Expected R ~1.0, got %f", pixel.R)
	}
}

// TestPopWithoutPush tests that PopLayer doesn't crash when no layer is pushed.
func TestPopWithoutPush(t *testing.T) {
	dc := NewContext(100, 100)
	dc.PopLayer() // Should not crash
	if dc.pixmap == nil {
		t.Error("PopLayer without PushLayer should not modify pixmap")
	}
}

// TestLayerOpacityClamping tests that opacity is clamped to [0, 1].
func TestLayerOpacityClamping(t *testing.T) {
	dc := NewContext(100, 100)

	dc.PushLayer(BlendNormal, -0.5)
	if dc.layerStack.layers[0].opacity != 0 {
		t.Errorf("Expected opacity 0, got %f", dc.layerStack.layers[0].opacity)
	}
	dc.PopLayer()

	dc.PushLayer(BlendNormal, 1.5)
	if dc.layerStack.layers[0].opacity != 1 {
		t.Errorf("Expected opacity 1, got %f", dc.layerStack.layers[0].opacity)
	}
	dc.PopLayer()

	dc.PushLayer(BlendNormal, 0.7)
	if dc.layerStack.layers[0].opacity != 0.7 {
		t.Errorf("Expected opacity 0.7, got %f", dc.layerStack.layers[0].opacity)
	}
	dc.PopLayer()
	_ = dc.FlushGPU()
}

// TestLayerClearTransparent tests that isolated layers start transparent.
// Normal/Copy use F1 opacity-group (no RT); advanced blend allocates isolation.
func TestLayerClearTransparent(t *testing.T) {
	dc := NewContext(10, 10)
	dc.ClearWithColor(White)

	dc.PushLayer(BlendMultiply, 1.0)

	pixel := dc.pixmap.GetPixel(5, 5)
	if pixel.A != 0 {
		t.Errorf("New isolated layer should be transparent, got alpha %f", pixel.A)
	}

	dc.PopLayer()
}

// TestMultipleLayerCycles tests multiple push/pop cycles.
func TestMultipleLayerCycles(t *testing.T) {
	dc := NewContext(50, 50)

	for i := 0; i < 5; i++ {
		dc.PushLayer(BlendMultiply, 1.0)
		dc.SetPixel(25, 25, RGBA{R: float64(i) / 5.0, G: 0, B: 0, A: 1})
		dc.PopLayer()
	}

	if len(dc.layerStack.layers) != 0 {
		t.Errorf("Expected 0 layers after cycles, got %d", len(dc.layerStack.layers))
	}
	if dc.basePixmap != nil {
		t.Error("Expected basePixmap to be nil after all layers popped")
	}
}

// TestSetBlendMode tests the SetBlendMode method.
func TestSetBlendMode(t *testing.T) {
	dc := NewContext(100, 100)
	dc.SetBlendMode(BlendMultiply)
	dc.SetBlendMode(BlendScreen)
	dc.SetBlendMode(BlendOverlay)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
