//go:build !nogpu

package gpu

import (
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// paintSupportsGPUFixedBlend reports whether paint.BlendMode can be expressed
// with standard WebGPU blend factors on premul solid geometry (B.02).
// Advanced modes (Multiply/Screen/…) still fall back to CPU until a shader
// blend path is wired for Context solid fills.
func paintSupportsGPUFixedBlend(paint *render.Paint) bool {
	if paint == nil {
		return true
	}
	_, ok := gpuBlendStateForPaint(paint.BlendMode)
	return ok
}

func paintUsesSourceOver(paint *render.Paint) bool {
	if paint == nil {
		return true
	}
	return paint.BlendMode == render.BlendNormal
}

// gpuBlendStateForPaint maps public paint blend modes to WebGPU premul blend
// state. Returns false for modes that need a shader blend path.
func gpuBlendStateForPaint(mode render.BlendMode) (types.BlendState, bool) {
	switch mode {
	case render.BlendNormal:
		return types.BlendStatePremultiplied(), true
	case render.BlendCopy:
		return types.BlendStateReplace(), true
	case render.BlendClear:
		return types.BlendStateClear(), true
	case render.BlendPlus:
		return types.BlendStatePlus(), true
	default:
		return types.BlendState{}, false
	}
}

func paintBlendMode(paint *render.Paint) render.BlendMode {
	if paint == nil {
		return render.BlendNormal
	}
	return paint.BlendMode
}
