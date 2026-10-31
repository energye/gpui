//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

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

// GetBlendFunc returns the CPU blend function for this mode. Since B3
// (P2.4e) internal/blend shares the render value domain, so the single
// scene→render boundary table feeds it directly.
//
// Usage:
//
//	blendFn := scene.BlendMultiply.GetBlendFunc()
//	r, g, b, a := blendFn(sr, sg, sb, sa, dr, dg, db, da)
func (mode BlendMode) GetBlendFunc() blend.BlendFunc {
	return blend.GetBlendFunc(mode.ToPaintBlendMode())
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
