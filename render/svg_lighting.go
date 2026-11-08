//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

// SVGLightingParams is the SVG feDiffuseLighting/feSpecularLighting attribute
// set (P3.8 4b-2): three light sources × two modes, resvg lighting.rs
// semantics. The height map is the content's alpha channel; the output is a
// freshly computed lit image (diffuse: opaque; specular: alpha=max(R,G,B)).
type SVGLightingParams struct {
	Specular                        bool
	SurfaceScale                    float32
	DiffuseConstant                 float32
	SpecularConstant                float32
	SpecularExponent                float32
	LightColor                      RGBA
	LightType                       int // 0=distant, 1=point, 2=spot
	Azimuth                         float32
	Elevation                       float32
	LightX, LightY, LightZ          float32
	PointsAtX, PointsAtY, PointsAtZ float32
	SpotSpecularExponent            float32
	LimitingConeAngle               float32
	HasLimitingCone                 bool
	LinearRGB                       bool // color-interpolation-filters="linearRGB" (the spec default)
}

// ApplySVGLighting runs the SVG lighting pair over the content (P3.8 4b-2).
// CPU-only for now — the pixel-exact reference is resvg lighting.rs and the
// conformance harness needs deterministic offline results; a GPU variant is
// a registered follow-up batch (same stance as turbulence/convolve).
func (c *Context) ApplySVGLighting(p SVGLightingParams) {
	if c == nil || c.pixmap == nil {
		return
	}
	_ = c.FlushGPU()
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if svgLightingApply != nil {
			svgLightingApply(src, dst, p)
		}
	})
	c.markLayerFullComposite()
}

// ApplyCompositeArithmetic runs SVG feComposite operator="arithmetic" over
// the content (P3.8 4b-2): result = k1·in1·in2 + k2·in1 + k3·in2 + k4 per
// channel in the premul domain, RGB clamped to the new alpha (resvg
// composite.rs). in2 is the second input; alpha≈0 pixels go fully
// transparent. CPU-only (same stance as ApplySVGLighting).
func (c *Context) ApplyCompositeArithmetic(k1, k2, k3, k4 float32, in2 *ImageBuf, linearRGB bool) {
	if c == nil || c.pixmap == nil {
		return
	}
	_ = c.FlushGPU()
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if compositeArithmeticApply != nil {
			compositeArithmeticApply(src, dst, k1, k2, k3, k4, in2, linearRGB)
		}
	})
	c.markLayerFullComposite()
}
