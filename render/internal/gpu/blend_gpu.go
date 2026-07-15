//go:build !nogpu

package gpu

import (
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// paintSupportsGPUFixedBlend reports whether paint.BlendMode can be expressed
// with standard WebGPU blend factors on premul solid geometry (B.02).
func paintSupportsGPUFixedBlend(paint *render.Paint) bool {
	if paint == nil {
		return true
	}
	_, ok := gpuBlendStateForPaint(paint.BlendMode)
	return ok
}

// paintSupportsGPUAdvancedBlend reports separable advanced modes that need
// destination sampling. These use fillAdvancedBlendAsImage (CPU composite of
// the shape against current pixmap dest, then GPU textured blit).
func paintSupportsGPUAdvancedBlend(paint *render.Paint) bool {
	if paint == nil {
		return false
	}
	switch paint.BlendMode {
	case render.BlendMultiply, render.BlendScreen, render.BlendOverlay:
		return true
	default:
		return false
	}
}

func paintUsesSourceOver(paint *render.Paint) bool {
	if paint == nil {
		return true
	}
	return paint.BlendMode == render.BlendNormal
}

// gpuBlendStateForPaint maps public paint blend modes to WebGPU premul blend
// state. Returns false for modes that need a shader/advanced path.
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
	case render.BlendDestinationOut:
		// out = dst * (1 - srcA)
		return types.BlendState{
			Color: types.BlendComponent{
				SrcFactor: types.BlendFactorZero,
				DstFactor: types.BlendFactorOneMinusSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
			Alpha: types.BlendComponent{
				SrcFactor: types.BlendFactorZero,
				DstFactor: types.BlendFactorOneMinusSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
		}, true
	case render.BlendSourceAtop:
		// out = src * dstA + dst * (1 - srcA)
		return types.BlendState{
			Color: types.BlendComponent{
				SrcFactor: types.BlendFactorDstAlpha,
				DstFactor: types.BlendFactorOneMinusSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
			Alpha: types.BlendComponent{
				SrcFactor: types.BlendFactorDstAlpha,
				DstFactor: types.BlendFactorOneMinusSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
		}, true
	case render.BlendXor:
		// out = src * (1 - dstA) + dst * (1 - srcA)
		return types.BlendState{
			Color: types.BlendComponent{
				SrcFactor: types.BlendFactorOneMinusDstAlpha,
				DstFactor: types.BlendFactorOneMinusSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
			Alpha: types.BlendComponent{
				SrcFactor: types.BlendFactorOneMinusDstAlpha,
				DstFactor: types.BlendFactorOneMinusSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
		}, true
	case render.BlendDestinationOver:
		// out = src * (1 - dstA) + dst
		return types.BlendState{
			Color: types.BlendComponent{
				SrcFactor: types.BlendFactorOneMinusDstAlpha,
				DstFactor: types.BlendFactorOne,
				Operation: types.BlendOperationAdd,
			},
			Alpha: types.BlendComponent{
				SrcFactor: types.BlendFactorOneMinusDstAlpha,
				DstFactor: types.BlendFactorOne,
				Operation: types.BlendOperationAdd,
			},
		}, true
	case render.BlendSourceIn:
		// out = src * dstA
		return types.BlendState{
			Color: types.BlendComponent{
				SrcFactor: types.BlendFactorDstAlpha,
				DstFactor: types.BlendFactorZero,
				Operation: types.BlendOperationAdd,
			},
			Alpha: types.BlendComponent{
				SrcFactor: types.BlendFactorDstAlpha,
				DstFactor: types.BlendFactorZero,
				Operation: types.BlendOperationAdd,
			},
		}, true
	case render.BlendSourceOut:
		// out = src * (1 - dstA)
		return types.BlendState{
			Color: types.BlendComponent{
				SrcFactor: types.BlendFactorOneMinusDstAlpha,
				DstFactor: types.BlendFactorZero,
				Operation: types.BlendOperationAdd,
			},
			Alpha: types.BlendComponent{
				SrcFactor: types.BlendFactorOneMinusDstAlpha,
				DstFactor: types.BlendFactorZero,
				Operation: types.BlendOperationAdd,
			},
		}, true
	case render.BlendDestinationIn:
		// out = dst * srcA
		return types.BlendState{
			Color: types.BlendComponent{
				SrcFactor: types.BlendFactorZero,
				DstFactor: types.BlendFactorSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
			Alpha: types.BlendComponent{
				SrcFactor: types.BlendFactorZero,
				DstFactor: types.BlendFactorSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
		}, true
	case render.BlendDestinationAtop:
		// out = src * (1 - dstA) + dst * srcA
		return types.BlendState{
			Color: types.BlendComponent{
				SrcFactor: types.BlendFactorOneMinusDstAlpha,
				DstFactor: types.BlendFactorSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
			Alpha: types.BlendComponent{
				SrcFactor: types.BlendFactorOneMinusDstAlpha,
				DstFactor: types.BlendFactorSrcAlpha,
				Operation: types.BlendOperationAdd,
			},
		}, true
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
