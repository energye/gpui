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

const linearOutputWGSL = colorSpaceWGSL + `
const LINEAR_OUT: bool = false;
fn encode_out(c: vec4<f32>) -> vec4<f32> {
    if (LINEAR_OUT) {
        return vec4<f32>(srgb_to_linear(c.r), srgb_to_linear(c.g), srgb_to_linear(c.b), c.a);
    }
    return c;
}
`

// shaderSourceForTarget prepends the shared color-space helpers and flips the
// LINEAR_OUT const when the target is an opt-in sRGB/linear target. The shader
// body must return encode_out(color) at every color-writing fragment entry.
func shaderSourceForTarget(src string, linear bool) string {
	prefix := linearOutputWGSL
	if linear {
		prefix = strings.Replace(prefix, "const LINEAR_OUT: bool = false;", "const LINEAR_OUT: bool = true;", 1)
	}
	return prefix + src
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
