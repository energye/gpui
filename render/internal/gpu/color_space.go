//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import (
	"math"
	"strings"
)

// P3.1b-1: single source for sRGB <-> linear-light conversion.
//
// The GPU shader snippet and the CPU Go functions must implement the same
// IEC 61966-2-1 curve; advanced blend (P3.1) and the future opt-in linear
// target (P3.1b) share this source instead of keeping per-shader copies.
const colorSpaceWGSL = `
fn srgb_to_linear(c: f32) -> f32 {
    if (c <= 0.04045) {
        return c / 12.92;
    }
    return pow((c + 0.055) / 1.055, 2.4);
}

fn linear_to_srgb(c: f32) -> f32 {
    if (c <= 0.0031308) {
        return c * 12.92;
    }
    return 1.055 * pow(c, 1.0 / 2.4) - 0.055;
}
`

const linearOutputWGSL = `
const LINEAR_OUT: bool = false;

// Premultiplied sRGB bytes <-> linear-light premultiplied values. The
// conversion must un-premultiply before applying the transfer curve and
// re-premultiply afterwards; converting the premultiplied channels directly
// darkens semi-transparent pixels (Skia SkColorSpaceXform / Impeller do the
// same premul-aware conversion).
fn premul_srgb_to_linear(c: vec4<f32>) -> vec4<f32> {
    if (c.a <= 0.0) {
        return c;
    }
    return vec4<f32>(
        srgb_to_linear(c.r / c.a) * c.a,
        srgb_to_linear(c.g / c.a) * c.a,
        srgb_to_linear(c.b / c.a) * c.a,
        c.a,
    );
}

fn premul_linear_to_srgb(c: vec4<f32>) -> vec4<f32> {
    if (c.a <= 0.0) {
        return c;
    }
    return vec4<f32>(
        linear_to_srgb(c.r / c.a) * c.a,
        linear_to_srgb(c.g / c.a) * c.a,
        linear_to_srgb(c.b / c.a) * c.a,
        c.a,
    );
}

fn encode_out(c: vec4<f32>) -> vec4<f32> {
    if (LINEAR_OUT) {
        return premul_srgb_to_linear(c);
    }
    return c;
}

// encode_src converts a sampled texel from the source color space to the
// destination's shader output space. src_linear=true means the source texture
// is an sRGB format, so textureSample already decoded it to linear light.
fn encode_src(c: vec4<f32>, src_linear: bool) -> vec4<f32> {
    if (LINEAR_OUT) {
        if (src_linear) {
            return c;
        }
        return premul_srgb_to_linear(c);
    }
    if (src_linear) {
        return premul_linear_to_srgb(c);
    }
    return c;
}
`

const linearTargetPreludeWGSL = colorSpaceWGSL + linearOutputWGSL

func flipLinearOut(prefix string, linear bool) string {
	if linear {
		return strings.Replace(prefix, "const LINEAR_OUT: bool = false;", "const LINEAR_OUT: bool = true;", 1)
	}
	return prefix
}

// shaderSourceForTarget prepends the shared color-space helpers and flips the
// LINEAR_OUT const when the target is an opt-in sRGB/linear target. The shader
// body must return encode_out(color) at every color-writing fragment entry.
func shaderSourceForTarget(src string, linear bool) string {
	return flipLinearOut(linearTargetPreludeWGSL, linear) + src
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// srgbToLinear converts one sRGB-encoded channel (0..1) to linear light.
func srgbToLinear(c float32) float32 {
	v := clamp01(float64(c))
	if v <= 0.04045 {
		return float32(v / 12.92)
	}
	return float32(math.Pow((v+0.055)/1.055, 2.4))
}

// linearToSRGB converts one linear-light channel (0..1) to sRGB encoding.
func linearToSRGB(c float32) float32 {
	v := clamp01(float64(c))
	if v <= 0.0031308 {
		return float32(v * 12.92)
	}
	return float32(1.055*math.Pow(v, 1.0/2.4) - 0.055)
}
