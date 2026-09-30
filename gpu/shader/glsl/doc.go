//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// This package generates GLSL source code from the IR representation.
// It supports multiple GLSL versions for different target platforms:
//
//   - GLSL ES 3.00: WebGL 2.0, Mobile OpenGL ES 3.0
//   - GLSL 3.30 Core: Desktop OpenGL 3.3+
//   - GLSL ES 3.10: Android 5.0+ with compute shaders
//   - GLSL 4.30 Core: Desktop OpenGL 4.3+ with compute shaders
//
// # Basic Usage
//
//	source, info, err := glsl.Compile(module, glsl.Options{
//	    LangVersion: glsl.Version330,
//	})
//
// # Texture/Sampler Handling
//
// WGSL separates textures and samplers, but GLSL combines them.
// The backend automatically generates combined sampler uniforms
// for texture-sampler pairs used together.
//
// # Reserved Words
//
// GLSL has over 500 reserved words (including future reserved).
// The backend automatically escapes conflicting identifier names
// by prefixing them with an underscore.
package glsl
