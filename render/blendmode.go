package render

import intImage "github.com/energye/gpui/render/internal/image"

// BlendMode is the canonical paint-level compositing mode for render.Context
// (PushLayer, DrawImage, GPU paths, etc.).
//
// # Representations in this repo
//
// Several packages historically defined their own BlendMode enums. They are
// NOT interchangeable by raw numeric cast:
//
//   - render.BlendMode (this type) — paint API; alias of internal/image.BlendMode
//   - scene.BlendMode (uint32) — scene-graph / WGSL wire encoding (CSS-like order)
//   - internal/blend.BlendMode — CPU Porter-Duff + advanced dispatch order
//   - surface.BlendMode — surface capability subset; values align with this type
//
// Always convert explicitly:
//
//	paint := sceneMode.ToPaintBlendMode()   // scene → render
//	fn := sceneMode.ToInternalBlendMode()   // scene → internal/blend
//
// Never write render.BlendMode(sceneMode) or scene.BlendMode(paintMode).
type BlendMode = intImage.BlendMode
