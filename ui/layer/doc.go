// Package layer holds retained-layer bookkeeping for Flutter-style dirty paint.
//
// Phase-1 of the dirty-region work lives in ui/core (RepaintBoundary flag,
// paint dirty bubble stop, CompositeOnly frames) and ui/primitive.RepaintBoundary.
// This package is reserved for GPU Surface / Compositor helpers that cache
// offscreen textures for boundaries (CreateOffscreenTexture + DrawGPUTexture).
//
// See docs/UI_FRAMEWORK_MAP.md and ENGINE_GAPS G2 (vector MSAA always Clear;
// blit-only LoadOpLoad is the correct partial-preserve path).
package layer
