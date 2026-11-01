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

// BlendSpace selects the color domain in which source and destination are
// blended. It is the paint-level opt-in for linear-light compositing (P3.1):
// the default stays the historical sRGB-encoded domain so existing frames are
// bit-identical; only paints marked BlendSpaceLinear decode to linear before
// the blend and re-encode afterwards.
//
// GPU coverage (P3.1, route B): advanced modes (Multiply/Screen/…/HSL family)
// blend in linear light on the dual-texture shader path; fixed-function modes
// (Porter-Duff family) have no linear hardware blend until the sRGB-target
// format switch (P3.1b) and explicitly fall back to sRGB-domain blending for
// now. CPU paths honor Linear for every mode.
type BlendSpace uint8

const (
	// BlendSpaceSRGB blends in the sRGB-encoded domain (the historical
	// behavior). Zero value, so a zeroed Paint keeps today's output.
	BlendSpaceSRGB BlendSpace = iota

	// BlendSpaceLinear decodes source and destination from sRGB to linear
	// light, blends, and re-encodes. Alpha never undergoes gamma conversion.
	// Physically correct compositing: removes the midtone darkening of
	// sRGB-domain blends (Skia sRGB-aware blending / Flutter linear working
	// space semantics).
	BlendSpaceLinear
)

// linear reports whether the space requires gamma decode/encode around the
// blend. Internal only — the CPU and GPU execution layers carry a bool.
func (s BlendSpace) linear() bool {
	return s == BlendSpaceLinear
}
