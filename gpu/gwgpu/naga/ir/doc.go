//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// The IR is designed to be:
//   - Shader-agnostic: Not tied to any specific shading language
//   - Complete: Can represent all features needed for modern shaders
//   - Efficient: Optimized for analysis and transformation
//
// # Structure
//
// The IR is organized around a Module type that contains:
//   - Types: All type definitions used in the shader
//   - Constants: Module-scope constant values
//   - GlobalVariables: Module-scope variables (uniforms, storage, etc.)
//   - Functions: All function definitions
//   - EntryPoints: Shader entry points with stage information
//
// # Translation Pipeline
//
// The typical translation pipeline is:
//
//	Source (WGSL/GLSL) → AST → IR → Target (SPIR-V/GLSL/MSL)
//
// This allows for source-independent analysis and optimization,
// as well as multi-target compilation from a single IR.
//
// # References
package ir
