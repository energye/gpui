// Package scene provides blend mode integration with internal/blend package.
package scene

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/blend"
)

// ToPaintBlendMode converts scene.BlendMode (wire/shader encoding) to the
// canonical paint-level render.BlendMode used by render.Context.
//
// scene and paint enums share names but NOT numeric values past the first
// few modes — never cast with render.BlendMode(sceneMode).
func (mode BlendMode) ToPaintBlendMode() render.BlendMode {
	switch mode {
	case BlendNormal, BlendSourceOver:
		return render.BlendNormal
	case BlendMultiply:
		return render.BlendMultiply
	case BlendScreen:
		return render.BlendScreen
	case BlendOverlay:
		return render.BlendOverlay
	case BlendHue:
		return render.BlendHue
	case BlendSaturation:
		return render.BlendSaturation
	case BlendColor:
		return render.BlendColor
	case BlendLuminosity:
		return render.BlendLuminosity
	case BlendClear:
		return render.BlendClear
	case BlendCopy:
		return render.BlendCopy
	case BlendPlus:
		return render.BlendPlus
	case BlendDestinationOut:
		return render.BlendDestinationOut
	case BlendSourceAtop:
		return render.BlendSourceAtop
	case BlendXor:
		return render.BlendXor
	case BlendDestinationOver:
		return render.BlendDestinationOver
	case BlendSourceIn:
		return render.BlendSourceIn
	case BlendSourceOut:
		return render.BlendSourceOut
	case BlendDestinationIn:
		return render.BlendDestinationIn
	case BlendDestinationAtop:
		return render.BlendDestinationAtop
	case BlendDarken:
		return render.BlendDarken
	case BlendLighten:
		return render.BlendLighten
	case BlendColorDodge:
		return render.BlendColorDodge
	case BlendColorBurn:
		return render.BlendColorBurn
	case BlendHardLight:
		return render.BlendHardLight
	case BlendSoftLight:
		return render.BlendSoftLight
	case BlendDifference:
		return render.BlendDifference
	case BlendExclusion:
		return render.BlendExclusion
	case BlendDestination:
		// No dedicated paint-level Destination; SourceOver is the safe default
		// for layers that only need to composite the backdrop.
		return render.BlendNormal
	default:
		return render.BlendNormal
	}
}

// PaintBlendModeToScene converts a paint-level render.BlendMode to scene.BlendMode.
func PaintBlendModeToScene(mode render.BlendMode) BlendMode {
	switch mode {
	case render.BlendNormal:
		return BlendNormal
	case render.BlendMultiply:
		return BlendMultiply
	case render.BlendScreen:
		return BlendScreen
	case render.BlendOverlay:
		return BlendOverlay
	case render.BlendHue:
		return BlendHue
	case render.BlendSaturation:
		return BlendSaturation
	case render.BlendColor:
		return BlendColor
	case render.BlendLuminosity:
		return BlendLuminosity
	case render.BlendClear:
		return BlendClear
	case render.BlendCopy:
		return BlendCopy
	case render.BlendPlus:
		return BlendPlus
	case render.BlendDestinationOut:
		return BlendDestinationOut
	case render.BlendSourceAtop:
		return BlendSourceAtop
	case render.BlendXor:
		return BlendXor
	case render.BlendDestinationOver:
		return BlendDestinationOver
	case render.BlendSourceIn:
		return BlendSourceIn
	case render.BlendSourceOut:
		return BlendSourceOut
	case render.BlendDestinationIn:
		return BlendDestinationIn
	case render.BlendDestinationAtop:
		return BlendDestinationAtop
	case render.BlendDarken:
		return BlendDarken
	case render.BlendLighten:
		return BlendLighten
	case render.BlendColorDodge:
		return BlendColorDodge
	case render.BlendColorBurn:
		return BlendColorBurn
	case render.BlendHardLight:
		return BlendHardLight
	case render.BlendSoftLight:
		return BlendSoftLight
	case render.BlendDifference:
		return BlendDifference
	case render.BlendExclusion:
		return BlendExclusion
	case render.BlendModulate:
		return BlendMultiply
	default:
		return BlendNormal
	}
}

// ToInternalBlendMode converts scene.BlendMode to internal blend.BlendMode.
// This provides the mapping between the scene graph blend modes and
// the low-level pixel blending implementation.
//
// The internal blend.BlendMode uses a different enumeration order,
// so this function provides the translation.
func (mode BlendMode) ToInternalBlendMode() blend.BlendMode {
	switch mode {
	// Porter-Duff modes (internal order matches porter_duff.go)
	case BlendClear:
		return blend.BlendClear
	case BlendCopy:
		return blend.BlendSource
	case BlendDestination:
		return blend.BlendDestination
	case BlendSourceOver, BlendNormal: // BlendNormal is SourceOver
		return blend.BlendSourceOver
	case BlendDestinationOver:
		return blend.BlendDestinationOver
	case BlendSourceIn:
		return blend.BlendSourceIn
	case BlendDestinationIn:
		return blend.BlendDestinationIn
	case BlendSourceOut:
		return blend.BlendSourceOut
	case BlendDestinationOut:
		return blend.BlendDestinationOut
	case BlendSourceAtop:
		return blend.BlendSourceAtop
	case BlendDestinationAtop:
		return blend.BlendDestinationAtop
	case BlendXor:
		return blend.BlendXor
	case BlendPlus:
		return blend.BlendPlus

	// Advanced separable blend modes (from advanced.go)
	case BlendMultiply:
		return blend.BlendMultiply
	case BlendScreen:
		return blend.BlendScreen
	case BlendOverlay:
		return blend.BlendOverlay
	case BlendDarken:
		return blend.BlendDarken
	case BlendLighten:
		return blend.BlendLighten
	case BlendColorDodge:
		return blend.BlendColorDodge
	case BlendColorBurn:
		return blend.BlendColorBurn
	case BlendHardLight:
		return blend.BlendHardLight
	case BlendSoftLight:
		return blend.BlendSoftLight
	case BlendDifference:
		return blend.BlendDifference
	case BlendExclusion:
		return blend.BlendExclusion

	// HSL non-separable blend modes (from advanced.go / hsl.go)
	case BlendHue:
		return blend.BlendHue
	case BlendSaturation:
		return blend.BlendSaturation
	case BlendColor:
		return blend.BlendColor
	case BlendLuminosity:
		return blend.BlendLuminosity

	default:
		// Default to source-over for unknown modes
		return blend.BlendSourceOver
	}
}

// GetBlendFunc returns the internal blend function for this mode.
// This is a convenience method that combines ToInternalBlendMode
// with blend.GetBlendFunc.
//
// Usage:
//
//	blendFn := scene.BlendMultiply.GetBlendFunc()
//	r, g, b, a := blendFn(sr, sg, sb, sa, dr, dg, db, da)
func (mode BlendMode) GetBlendFunc() blend.BlendFunc {
	return blend.GetBlendFunc(mode.ToInternalBlendMode())
}

// BlendModeFromInternal converts an internal blend.BlendMode to scene.BlendMode.
// This is the reverse mapping for cases where you need to convert from
// internal representation back to the scene graph representation.
func BlendModeFromInternal(internal blend.BlendMode) BlendMode {
	switch internal {
	// Porter-Duff modes
	case blend.BlendClear:
		return BlendClear
	case blend.BlendSource:
		return BlendCopy
	case blend.BlendDestination:
		return BlendDestination
	case blend.BlendSourceOver:
		return BlendSourceOver
	case blend.BlendDestinationOver:
		return BlendDestinationOver
	case blend.BlendSourceIn:
		return BlendSourceIn
	case blend.BlendDestinationIn:
		return BlendDestinationIn
	case blend.BlendSourceOut:
		return BlendSourceOut
	case blend.BlendDestinationOut:
		return BlendDestinationOut
	case blend.BlendSourceAtop:
		return BlendSourceAtop
	case blend.BlendDestinationAtop:
		return BlendDestinationAtop
	case blend.BlendXor:
		return BlendXor
	case blend.BlendPlus:
		return BlendPlus
	case blend.BlendModulate:
		return BlendMultiply // Modulate is similar to Multiply

	// Advanced separable blend modes
	case blend.BlendMultiply:
		return BlendMultiply
	case blend.BlendScreen:
		return BlendScreen
	case blend.BlendOverlay:
		return BlendOverlay
	case blend.BlendDarken:
		return BlendDarken
	case blend.BlendLighten:
		return BlendLighten
	case blend.BlendColorDodge:
		return BlendColorDodge
	case blend.BlendColorBurn:
		return BlendColorBurn
	case blend.BlendHardLight:
		return BlendHardLight
	case blend.BlendSoftLight:
		return BlendSoftLight
	case blend.BlendDifference:
		return BlendDifference
	case blend.BlendExclusion:
		return BlendExclusion

	// HSL non-separable blend modes
	case blend.BlendHue:
		return BlendHue
	case blend.BlendSaturation:
		return BlendSaturation
	case blend.BlendColor:
		return BlendColor
	case blend.BlendLuminosity:
		return BlendLuminosity

	default:
		return BlendNormal
	}
}

// AllBlendModes returns a slice of all supported blend modes.
// This is useful for testing and iteration.
func AllBlendModes() []BlendMode {
	return []BlendMode{
		// Standard modes
		BlendNormal,
		BlendMultiply,
		BlendScreen,
		BlendOverlay,
		BlendDarken,
		BlendLighten,
		BlendColorDodge,
		BlendColorBurn,
		BlendHardLight,
		BlendSoftLight,
		BlendDifference,
		BlendExclusion,
		// HSL modes
		BlendHue,
		BlendSaturation,
		BlendColor,
		BlendLuminosity,
		// Porter-Duff modes
		BlendClear,
		BlendCopy,
		BlendDestination,
		BlendSourceOver,
		BlendDestinationOver,
		BlendSourceIn,
		BlendDestinationIn,
		BlendSourceOut,
		BlendDestinationOut,
		BlendSourceAtop,
		BlendDestinationAtop,
		BlendXor,
		BlendPlus,
	}
}

// PorterDuffModes returns a slice of Porter-Duff compositing modes.
func PorterDuffModes() []BlendMode {
	return []BlendMode{
		BlendClear,
		BlendCopy,
		BlendDestination,
		BlendSourceOver,
		BlendDestinationOver,
		BlendSourceIn,
		BlendDestinationIn,
		BlendSourceOut,
		BlendDestinationOut,
		BlendSourceAtop,
		BlendDestinationAtop,
		BlendXor,
		BlendPlus,
	}
}

// AdvancedModes returns a slice of advanced separable blend modes.
func AdvancedModes() []BlendMode {
	return []BlendMode{
		BlendNormal,
		BlendMultiply,
		BlendScreen,
		BlendOverlay,
		BlendDarken,
		BlendLighten,
		BlendColorDodge,
		BlendColorBurn,
		BlendHardLight,
		BlendSoftLight,
		BlendDifference,
		BlendExclusion,
	}
}

// HSLModes returns a slice of HSL-based non-separable blend modes.
func HSLModes() []BlendMode {
	return []BlendMode{
		BlendHue,
		BlendSaturation,
		BlendColor,
		BlendLuminosity,
	}
}
