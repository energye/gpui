package scene

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/blend"
)

// =============================================================================
// BlendMode String Tests
// =============================================================================

func TestBlendModeString(t *testing.T) {
	tests := []struct {
		mode BlendMode
		want string
	}{
		// Standard blend modes
		{BlendNormal, "Normal"},
		{BlendMultiply, "Multiply"},
		{BlendScreen, "Screen"},
		{BlendOverlay, "Overlay"},
		{BlendDarken, "Darken"},
		{BlendLighten, "Lighten"},
		{BlendColorDodge, "ColorDodge"},
		{BlendColorBurn, "ColorBurn"},
		{BlendHardLight, "HardLight"},
		{BlendSoftLight, "SoftLight"},
		{BlendDifference, "Difference"},
		{BlendExclusion, "Exclusion"},
		// HSL blend modes
		{BlendHue, "Hue"},
		{BlendSaturation, "Saturation"},
		{BlendColor, "Color"},
		{BlendLuminosity, "Luminosity"},
		// Porter-Duff modes
		{BlendClear, "Clear"},
		{BlendCopy, "Copy"},
		{BlendDestination, "Destination"},
		{BlendSourceOver, "SourceOver"},
		{BlendDestinationOver, "DestinationOver"},
		{BlendSourceIn, "SourceIn"},
		{BlendDestinationIn, "DestinationIn"},
		{BlendSourceOut, "SourceOut"},
		{BlendDestinationOut, "DestinationOut"},
		{BlendSourceAtop, "SourceAtop"},
		{BlendDestinationAtop, "DestinationAtop"},
		{BlendXor, "Xor"},
		{BlendPlus, "Plus"},
		// Unknown
		{BlendMode(255), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("BlendMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestBlendModeIsPorterDuff(t *testing.T) {
	porterDuffModes := []BlendMode{
		BlendClear, BlendCopy, BlendDestination, BlendSourceOver,
		BlendDestinationOver, BlendSourceIn, BlendDestinationIn,
		BlendSourceOut, BlendDestinationOut, BlendSourceAtop,
		BlendDestinationAtop, BlendXor, BlendPlus,
	}

	for _, mode := range porterDuffModes {
		t.Run(mode.String(), func(t *testing.T) {
			if !mode.IsPorterDuff() {
				t.Errorf("%s.IsPorterDuff() = false, want true", mode.String())
			}
		})
	}

	// Non-Porter-Duff modes should return false
	nonPDModes := []BlendMode{
		BlendNormal, BlendMultiply, BlendScreen, BlendOverlay,
		BlendHue, BlendSaturation,
	}

	for _, mode := range nonPDModes {
		t.Run(mode.String()+"_NotPD", func(t *testing.T) {
			if mode.IsPorterDuff() {
				t.Errorf("%s.IsPorterDuff() = true, want false", mode.String())
			}
		})
	}
}

func TestBlendModeIsAdvanced(t *testing.T) {
	advancedModes := []BlendMode{
		BlendNormal, BlendMultiply, BlendScreen, BlendOverlay,
		BlendDarken, BlendLighten, BlendColorDodge, BlendColorBurn,
		BlendHardLight, BlendSoftLight, BlendDifference, BlendExclusion,
	}

	for _, mode := range advancedModes {
		t.Run(mode.String(), func(t *testing.T) {
			if !mode.IsAdvanced() {
				t.Errorf("%s.IsAdvanced() = false, want true", mode.String())
			}
		})
	}

	// HSL and Porter-Duff modes should return false
	nonAdvModes := []BlendMode{
		BlendHue, BlendSaturation, BlendColor, BlendLuminosity,
		BlendClear, BlendCopy, BlendSourceOver,
	}

	for _, mode := range nonAdvModes {
		t.Run(mode.String()+"_NotAdv", func(t *testing.T) {
			if mode.IsAdvanced() {
				t.Errorf("%s.IsAdvanced() = true, want false", mode.String())
			}
		})
	}
}

func TestBlendModeIsHSL(t *testing.T) {
	hslModes := []BlendMode{
		BlendHue, BlendSaturation, BlendColor, BlendLuminosity,
	}

	for _, mode := range hslModes {
		t.Run(mode.String(), func(t *testing.T) {
			if !mode.IsHSL() {
				t.Errorf("%s.IsHSL() = false, want true", mode.String())
			}
		})
	}

	// Non-HSL modes should return false
	nonHSLModes := []BlendMode{
		BlendNormal, BlendMultiply, BlendClear, BlendSourceOver,
	}

	for _, mode := range nonHSLModes {
		t.Run(mode.String()+"_NotHSL", func(t *testing.T) {
			if mode.IsHSL() {
				t.Errorf("%s.IsHSL() = true, want false", mode.String())
			}
		})
	}
}

// =============================================================================
// Blend Mode Conversion Tests
// =============================================================================

func TestToPaintBlendMode_PorterDuff(t *testing.T) {
	tests := []struct {
		scene BlendMode
		paint render.BlendMode
	}{
		{BlendClear, render.BlendClear},
		{BlendCopy, render.BlendCopy},
		{BlendDestination, render.BlendNormal}, // no paint-level Destination: degrade to Normal
		{BlendSourceOver, render.BlendNormal},
		{BlendNormal, render.BlendNormal},
		{BlendDestinationOver, render.BlendDestinationOver},
		{BlendSourceIn, render.BlendSourceIn},
		{BlendDestinationIn, render.BlendDestinationIn},
		{BlendSourceOut, render.BlendSourceOut},
		{BlendDestinationOut, render.BlendDestinationOut},
		{BlendSourceAtop, render.BlendSourceAtop},
		{BlendDestinationAtop, render.BlendDestinationAtop},
		{BlendXor, render.BlendXor},
		{BlendPlus, render.BlendPlus},
	}

	for _, tt := range tests {
		t.Run(tt.scene.String(), func(t *testing.T) {
			got := tt.scene.ToPaintBlendMode()
			if got != tt.paint {
				t.Errorf("ToPaintBlendMode(%s) = %d, want %d", tt.scene.String(), got, tt.paint)
			}
		})
	}
}

func TestToPaintBlendMode_Advanced(t *testing.T) {
	tests := []struct {
		scene BlendMode
		paint render.BlendMode
	}{
		{BlendMultiply, render.BlendMultiply},
		{BlendScreen, render.BlendScreen},
		{BlendOverlay, render.BlendOverlay},
		{BlendDarken, render.BlendDarken},
		{BlendLighten, render.BlendLighten},
		{BlendColorDodge, render.BlendColorDodge},
		{BlendColorBurn, render.BlendColorBurn},
		{BlendHardLight, render.BlendHardLight},
		{BlendSoftLight, render.BlendSoftLight},
		{BlendDifference, render.BlendDifference},
		{BlendExclusion, render.BlendExclusion},
	}

	for _, tt := range tests {
		t.Run(tt.scene.String(), func(t *testing.T) {
			got := tt.scene.ToPaintBlendMode()
			if got != tt.paint {
				t.Errorf("TestToPaintBlendMode_Advanced(%s) = %d, want %d", tt.scene.String(), got, tt.paint)
			}
		})
	}
}

func TestToPaintBlendMode_HSL(t *testing.T) {
	tests := []struct {
		scene BlendMode
		paint render.BlendMode
	}{
		{BlendHue, render.BlendHue},
		{BlendSaturation, render.BlendSaturation},
		{BlendColor, render.BlendColor},
		{BlendLuminosity, render.BlendLuminosity},
	}

	for _, tt := range tests {
		t.Run(tt.scene.String(), func(t *testing.T) {
			got := tt.scene.ToPaintBlendMode()
			if got != tt.paint {
				t.Errorf("TestToPaintBlendMode_HSL(%s) = %d, want %d", tt.scene.String(), got, tt.paint)
			}
		})
	}
}

func TestToPaintBlendMode_Unknown(t *testing.T) {
	unknown := BlendMode(255)
	got := unknown.ToPaintBlendMode()
	if got != render.BlendNormal {
		t.Errorf("unknown blend mode should default to Normal, got %d", got)
	}
}

func TestGetBlendFunc(t *testing.T) {
	// Test that GetBlendFunc returns a valid function for all modes
	modes := AllBlendModes()
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			fn := mode.GetBlendFunc()
			if fn == nil {
				t.Errorf("%s.GetBlendFunc() returned nil", mode.String())
			}

			// Test the function works (doesn't panic)
			r, g, b, a := fn(255, 128, 64, 200, 100, 150, 200, 128)
			// Verify the function executed and returned valid bytes
			// (byte type is always 0-255, so this is just a smoke test)
			_ = r
			_ = g
			_ = b
			_ = a
		})
	}
}

// TestBlendSharedValueDomain pins the B3 (P2.4e) value-domain unification:
// internal/blend.BlendMode aliases internal/image.BlendMode — the same domain
// as render.BlendMode — so the CPU path consumes ToPaintBlendMode output
// directly and needs no numeric translation.
func TestBlendSharedValueDomain(t *testing.T) {
	var _ blend.BlendMode = render.BlendNormal

	if blend.BlendSourceOver != render.BlendNormal {
		t.Error("blend.BlendSourceOver must equal render.BlendNormal (shared domain)")
	}
	if blend.BlendSource != render.BlendCopy {
		t.Error("blend.BlendSource must equal render.BlendCopy (shared domain)")
	}
	if blend.BlendMultiply != 1 || blend.BlendClear != 8 || blend.BlendModulate != 27 {
		t.Error("blend constants must sit on the image value domain (Normal=0..Modulate=27)")
	}
}

// =============================================================================
// Mode List Tests
// =============================================================================

func TestAllBlendModes(t *testing.T) {
	modes := AllBlendModes()
	// 16 standard/HSL modes + 13 Porter-Duff modes = 29 total
	if len(modes) != 29 {
		t.Errorf("AllBlendModes() returned %d modes, want 29", len(modes))
	}

	// Check for duplicates
	seen := make(map[BlendMode]bool)
	for _, mode := range modes {
		if seen[mode] {
			t.Errorf("AllBlendModes() contains duplicate: %s", mode.String())
		}
		seen[mode] = true
	}
}

func TestPorterDuffModes(t *testing.T) {
	modes := PorterDuffModes()
	if len(modes) != 13 {
		t.Errorf("PorterDuffModes() returned %d modes, want 13", len(modes))
	}

	for _, mode := range modes {
		if !mode.IsPorterDuff() {
			t.Errorf("%s from PorterDuffModes() is not Porter-Duff", mode.String())
		}
	}
}

func TestAdvancedModes(t *testing.T) {
	modes := AdvancedModes()
	if len(modes) != 12 {
		t.Errorf("AdvancedModes() returned %d modes, want 12", len(modes))
	}

	for _, mode := range modes {
		if !mode.IsAdvanced() {
			t.Errorf("%s from AdvancedModes() is not advanced", mode.String())
		}
	}
}

func TestHSLModes(t *testing.T) {
	modes := HSLModes()
	if len(modes) != 4 {
		t.Errorf("HSLModes() returned %d modes, want 4", len(modes))
	}

	for _, mode := range modes {
		if !mode.IsHSL() {
			t.Errorf("%s from HSLModes() is not HSL", mode.String())
		}
	}
}

// =============================================================================
// Layer Stack Tests
// =============================================================================

func TestLayerStackAcquireRelease(t *testing.T) {
	stack := NewLayerStack()

	// Acquire layer
	layer := stack.AcquireLayer()
	if layer == nil {
		t.Fatal("AcquireLayer() returned nil")
	}
	if layer.Encoding == nil {
		t.Error("acquired layer should have encoding")
	}

	// Configure and use
	layer.Kind = LayerRegular
	layer.BlendMode = BlendMultiply
	layer.Alpha = 0.5

	// Release back to pool
	stack.ReleaseLayer(layer)

	// Acquire again (should get pooled layer)
	layer2 := stack.AcquireLayer()
	if layer2 == nil {
		t.Fatal("second AcquireLayer() returned nil")
	}
	// Layer should be reset
	if layer2.BlendMode != BlendNormal || layer2.Alpha != 1.0 {
		t.Error("reused layer should be reset")
	}
}

func TestLayerStackAll(t *testing.T) {
	stack := NewLayerStack()

	// Initial state - just root
	all := stack.All()
	if len(all) != 1 {
		t.Errorf("All() = %d layers, want 1", len(all))
	}

	// Push layers
	layer1 := NewLayerState(LayerRegular, BlendMultiply, 0.5)
	layer2 := NewLayerState(LayerFiltered, BlendScreen, 0.8)
	stack.Push(layer1)
	stack.Push(layer2)

	all = stack.All()
	if len(all) != 3 {
		t.Errorf("All() = %d layers, want 3", len(all))
	}
	if all[1] != layer1 || all[2] != layer2 {
		t.Error("All() should return layers in push order")
	}
}

// =============================================================================
// Deep Layer Nesting Tests
// =============================================================================

func TestDeepLayerNesting(t *testing.T) {
	scene := NewScene()
	const depth = 100

	// Push 100 layers
	for i := 0; i < depth; i++ {
		mode := BlendMode(i % 28) // Cycle through all blend modes
		scene.PushLayer(mode, 0.9, nil)
	}

	if scene.LayerDepth() != depth+1 { // +1 for root
		t.Errorf("LayerDepth() = %d, want %d", scene.LayerDepth(), depth+1)
	}

	// Pop all layers
	for i := 0; i < depth; i++ {
		if !scene.PopLayer() {
			t.Errorf("PopLayer() failed at depth %d", depth-i)
		}
	}

	if scene.LayerDepth() != 1 {
		t.Errorf("LayerDepth() after pop all = %d, want 1", scene.LayerDepth())
	}
}

func TestDeepLayerNestingWithContent(t *testing.T) {
	scene := NewScene()
	const depth = 100

	// Push layers with content at each level
	for i := 0; i < depth; i++ {
		mode := AllBlendModes()[i%len(AllBlendModes())]
		alpha := float32(i%10+1) / 10.0 // 0.1 to 1.0
		scene.PushLayer(mode, alpha, nil)

		// Draw something in each layer
		rect := NewRectShape(float32(i*2), float32(i*2), 10, 10)
		scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Red), rect)
	}

	// Get flattened encoding
	enc := scene.Flatten()
	if enc == nil {
		t.Fatal("Flatten() returned nil")
	}
	if enc.IsEmpty() {
		t.Error("Flatten() should not return empty encoding")
	}
}

// =============================================================================
// Layer Opacity Tests
// =============================================================================

func TestLayerOpacityRange(t *testing.T) {
	tests := []struct {
		input    float32
		expected float32
	}{
		{0.0, 0.0},
		{0.25, 0.25},
		{0.5, 0.5},
		{0.75, 0.75},
		{1.0, 1.0},
		{-0.5, 0.0},  // Clamped to 0
		{-1.0, 0.0},  // Clamped to 0
		{1.5, 1.0},   // Clamped to 1
		{100.0, 1.0}, // Clamped to 1
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			layer := NewLayerState(LayerRegular, BlendNormal, tt.input)
			if layer.Alpha != tt.expected {
				t.Errorf("NewLayerState with alpha=%f got Alpha=%f, want %f",
					tt.input, layer.Alpha, tt.expected)
			}
		})
	}
}

func TestLayerOpacityInScene(t *testing.T) {
	scene := NewScene()

	// Test opacity clamping through scene API
	scene.PushLayer(BlendNormal, 1.5, nil) // Should clamp to 1.0

	enc := scene.Encoding()
	it := enc.NewIterator()

	// Find the PushLayer command and check alpha
	for {
		tag, ok := it.Next()
		if !ok {
			break
		}
		if tag == TagPushLayer {
			data := it.ReadDrawData(2)
			if data == nil {
				t.Fatal("could not read push layer data")
			}
			// data[1] is alpha as float32 bits
			alpha := float32frombits(data[1])
			if alpha > 1.0 {
				t.Errorf("layer alpha should be clamped to 1.0, got %f", alpha)
			}
			break
		}
	}
}

// float32frombits converts uint32 bits to float32
func float32frombits(bits uint32) float32 {
	return math.Float32frombits(bits)
}

// =============================================================================
// Layer Clip Region Tests
// =============================================================================

func TestLayerWithClipRegion(t *testing.T) {
	scene := NewScene()

	// Create a clip region
	clip := NewCircleShape(50, 50, 30)

	// Push layer with clip
	scene.PushLayer(BlendNormal, 1.0, clip)

	// Draw content
	rect := NewRectShape(0, 0, 100, 100)
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Blue), rect)

	scene.PopLayer()

	enc := scene.Encoding()

	// Verify clip commands are present
	hasBeginClip := false
	hasEndClip := false
	for _, tag := range enc.Tags() {
		if tag == TagBeginClip {
			hasBeginClip = true
		}
		if tag == TagEndClip {
			hasEndClip = true
		}
	}

	if !hasBeginClip {
		t.Error("layer with clip should have BeginClip command")
	}
	if !hasEndClip {
		t.Error("layer with clip should have EndClip command")
	}
}

// =============================================================================
func TestLayerPoolReuse(t *testing.T) {
	pool := newLayerPool()

	// Get a layer
	layer1 := pool.get()
	if layer1 == nil {
		t.Fatal("pool.get() returned nil")
	}

	// Modify it
	layer1.Kind = LayerFiltered
	layer1.BlendMode = BlendMultiply
	layer1.Alpha = 0.5
	if layer1.Encoding != nil {
		layer1.Encoding.EncodeFill(SolidBrush(render.Red), FillNonZero)
	}

	// Return to pool
	pool.put(layer1)

	// Get again - should be reset
	layer2 := pool.get()
	if layer2.Kind != LayerRegular {
		t.Error("pooled layer should have Kind reset to Regular")
	}
	if layer2.BlendMode != BlendNormal {
		t.Error("pooled layer should have BlendMode reset to Normal")
	}
	if layer2.Alpha != 1.0 {
		t.Error("pooled layer should have Alpha reset to 1.0")
	}
	if layer2.Encoding != nil && !layer2.Encoding.IsEmpty() {
		t.Error("pooled layer encoding should be reset")
	}
}

func TestLayerPoolPutNil(t *testing.T) {
	pool := newLayerPool()
	// Should not panic
	pool.put(nil)
}

func TestLayerPoolMultiple(t *testing.T) {
	pool := newLayerPool()

	// Get multiple layers
	layers := make([]*LayerState, 10)
	for i := range layers {
		layers[i] = pool.get()
		if layers[i] == nil {
			t.Fatalf("pool.get() returned nil at index %d", i)
		}
	}

	// Return all
	for _, layer := range layers {
		pool.put(layer)
	}

	// Get all again - should work
	for i := range layers {
		layer := pool.get()
		if layer == nil {
			t.Fatalf("second pool.get() returned nil at index %d", i)
		}
	}
}

// =============================================================================
// Scene Flatten Tests
// =============================================================================

func TestSceneFlatten(t *testing.T) {
	scene := NewScene()

	// Draw background
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.White),
		NewRectShape(0, 0, 100, 100))

	// Push layer and draw
	scene.PushLayer(BlendMultiply, 0.5, nil)
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Red),
		NewCircleShape(50, 50, 30))
	// Don't pop - Flatten should handle it

	flat := scene.Flatten()
	if flat == nil {
		t.Fatal("Flatten() returned nil")
	}
	if flat.IsEmpty() {
		t.Error("Flatten() should not return empty encoding")
	}

	// Scene should still be usable after Flatten
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Blue),
		NewRectShape(0, 0, 10, 10))
}

func TestSceneFlattenIsClone(t *testing.T) {
	scene := NewScene()
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Red),
		NewRectShape(0, 0, 50, 50))

	flat := scene.Flatten()
	originalHash := flat.Hash()

	// Modify scene
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Blue),
		NewRectShape(50, 50, 50, 50))

	// Original flat should be unchanged
	if flat.Hash() != originalHash {
		t.Error("Flatten() should return independent clone")
	}
}

func TestSceneFlattenMultipleLayers(t *testing.T) {
	scene := NewScene()

	// Create nested layers
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.White),
		NewRectShape(0, 0, 100, 100))

	scene.PushLayer(BlendMultiply, 1.0, nil)
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Red),
		NewRectShape(10, 10, 30, 30))

	scene.PushLayer(BlendScreen, 0.8, nil)
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Green),
		NewRectShape(20, 20, 30, 30))

	scene.PushLayer(BlendOverlay, 0.5, nil)
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Blue),
		NewRectShape(30, 30, 30, 30))

	// Don't pop layers - Flatten handles it
	flat := scene.Flatten()

	if flat.IsEmpty() {
		t.Error("Flatten() with nested layers should not be empty")
	}

	// Count expected content - at least the fills
	fillCount := 0
	for _, tag := range flat.Tags() {
		if tag == TagFill {
			fillCount++
		}
	}
	if fillCount < 4 {
		t.Errorf("expected at least 4 fills, got %d", fillCount)
	}
}

// =============================================================================
// LayerState Tests
// =============================================================================

func TestLayerStateIsEmpty(t *testing.T) {
	// Regular layer with no content
	layer := NewLayerState(LayerRegular, BlendNormal, 1.0)
	if !layer.IsEmpty() {
		t.Error("new layer should be empty")
	}

	// Add content
	layer.Encoding.EncodeFill(SolidBrush(render.Red), FillNonZero)
	if layer.IsEmpty() {
		t.Error("layer with content should not be empty")
	}

	// Clip layer with no shape
	clipLayer := &LayerState{Kind: LayerClip, Clip: nil}
	if !clipLayer.IsEmpty() {
		t.Error("clip layer with nil clip should be empty")
	}

	// Clip layer with shape
	clipLayer.Clip = NewRectShape(0, 0, 100, 100)
	if clipLayer.IsEmpty() {
		t.Error("clip layer with shape should not be empty")
	}
}

func TestLayerStateUpdateBounds(t *testing.T) {
	layer := NewLayerState(LayerRegular, BlendNormal, 1.0)

	// Initial bounds should be empty
	if !layer.Bounds.IsEmpty() {
		t.Error("new layer bounds should be empty")
	}

	// Update bounds
	layer.UpdateBounds(Rect{MinX: 10, MinY: 20, MaxX: 30, MaxY: 40})
	if layer.Bounds.MinX != 10 || layer.Bounds.MinY != 20 {
		t.Errorf("bounds = %+v, want (10,20)-(30,40)", layer.Bounds)
	}

	// Update with another rect - should union
	layer.UpdateBounds(Rect{MinX: 0, MinY: 0, MaxX: 50, MaxY: 50})
	if layer.Bounds.MinX != 0 || layer.Bounds.MinY != 0 ||
		layer.Bounds.MaxX != 50 || layer.Bounds.MaxY != 50 {
		t.Errorf("bounds after union = %+v, want (0,0)-(50,50)", layer.Bounds)
	}
}

func TestLayerStateReset(t *testing.T) {
	layer := NewLayerState(LayerFiltered, BlendMultiply, 0.5)
	layer.Clip = NewRectShape(0, 0, 100, 100)
	layer.Transform = TranslateAffine(10, 20)
	layer.ClipStackDepth = 5
	layer.UpdateBounds(Rect{MinX: 10, MinY: 10, MaxX: 90, MaxY: 90})

	layer.Reset()

	if layer.Kind != LayerRegular {
		t.Error("Kind should reset to Regular")
	}
	if layer.BlendMode != BlendNormal {
		t.Error("BlendMode should reset to Normal")
	}
	if layer.Alpha != 1.0 {
		t.Error("Alpha should reset to 1.0")
	}
	if layer.Clip != nil {
		t.Error("Clip should reset to nil")
	}
	if !layer.Transform.IsIdentity() {
		t.Error("Transform should reset to identity")
	}
	if layer.ClipStackDepth != 0 {
		t.Error("ClipStackDepth should reset to 0")
	}
	if !layer.Bounds.IsEmpty() {
		t.Error("Bounds should reset to empty")
	}
}

// =============================================================================
// Integration Tests with All Blend Modes
// =============================================================================

func TestAllBlendModesInLayer(t *testing.T) {
	modes := AllBlendModes()

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			scene := NewScene()

			// Draw background
			scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.White),
				NewRectShape(0, 0, 100, 100))

			// Push layer with this blend mode
			scene.PushLayer(mode, 0.75, nil)

			// Draw content
			scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Red),
				NewCircleShape(50, 50, 25))

			scene.PopLayer()

			enc := scene.Encoding()
			if enc.IsEmpty() {
				t.Error("encoding should not be empty")
			}

			// Verify PushLayer has correct blend mode
			hasPushLayer := false
			for _, tag := range enc.Tags() {
				if tag == TagPushLayer {
					hasPushLayer = true
					break
				}
			}
			if !hasPushLayer {
				t.Error("encoding should have PushLayer command")
			}
		})
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkBlendModeString(b *testing.B) {
	modes := AllBlendModes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, mode := range modes {
			_ = mode.String()
		}
	}
}

func BenchmarkToPaintBlendMode(b *testing.B) {
	modes := AllBlendModes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, mode := range modes {
			_ = mode.ToPaintBlendMode()
		}
	}
}

func BenchmarkGetBlendFunc(b *testing.B) {
	modes := AllBlendModes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, mode := range modes {
			_ = mode.GetBlendFunc()
		}
	}
}

func BenchmarkLayerPool(b *testing.B) {
	pool := newLayerPool()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		layer := pool.get()
		layer.BlendMode = BlendMultiply
		pool.put(layer)
	}
}

func BenchmarkDeepLayerNesting(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		scene := NewScene()
		for j := 0; j < 50; j++ {
			scene.PushLayer(BlendMode(j%28), 0.9, nil)
		}
		for j := 0; j < 50; j++ {
			scene.PopLayer()
		}
		_ = scene.Flatten()
	}
}

func BenchmarkSceneFlatten(b *testing.B) {
	scene := NewScene()
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.White),
		NewRectShape(0, 0, 100, 100))
	scene.PushLayer(BlendMultiply, 0.5, nil)
	scene.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.Red),
		NewCircleShape(50, 50, 25))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = scene.Flatten()
	}
}
