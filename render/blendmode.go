//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import intImage "github.com/energye/gpui/render/internal/image"

// BlendMode is the canonical paint-level compositing mode for render.Context
// (PushLayer, DrawImage, GPU paths, etc.).
//
// # Representations in this repo
//
// Historically several packages defined their own BlendMode enums. Since B3
// (P2.4e) there is ONE shared numeric domain and ONE boundary:
//
//   - render.BlendMode (this type) — paint API; alias of internal/image.BlendMode
//   - internal/blend.BlendMode — CPU dispatch; alias of the same
//     internal/image.BlendMode, so it needs no conversion
//   - scene.BlendMode (uint32) — scene-graph / WGSL wire encoding (CSS-like
//     order); wire values are its own and stay stable (no re-recording)
//
// The single scene→paint boundary:
//
//	paint := sceneMode.ToPaintBlendMode() // scene → render
//
// Never cast with render.BlendMode(sceneMode).
type BlendMode = intImage.BlendMode
