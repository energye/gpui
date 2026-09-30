//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Self-maintained port: only the WGSL→IR path used by gpu/gwgpu/gles is
// kept (Parse/Lower/Validate). SPIR-V/MSL/HLSL backends are not ported.
package naga

import (
	"fmt"

	"github.com/energye/gpui/gpu/gwgpu/naga/ir"
	"github.com/energye/gpui/gpu/gwgpu/naga/wgsl"
)

// Capability constants re-exported from ir for convenience.
// See [ir.Capabilities] for full documentation.
const (
	// CapFloat64 enables f64 scalar type.
	CapFloat64 = ir.CapFloat64

	// CapShaderInt64 enables i64 and u64 scalar types.
	CapShaderInt64 = ir.CapShaderInt64

	// CapShaderFloat16 enables f16 scalar type.
	CapShaderFloat16 = ir.CapShaderFloat16

	// CapAll enables all capabilities.
	CapAll = ir.CapAll
)

// Parse parses WGSL source code to AST (Abstract Syntax Tree).
//
// This is the first stage of compilation. The AST represents the syntactic
// structure of the shader but does not include semantic information like types.
func Parse(source string) (*wgsl.Module, error) {
	// Tokenize
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("tokenization error: %w", err)
	}

	// Parse to AST
	parser := wgsl.NewParser(tokens)
	module, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return module, nil
}

// Lower converts WGSL AST to IR (Intermediate Representation).
// All capabilities are enabled (permissive mode for tools and tests).
// For strict capability validation, use [LowerWithCapabilities].
//
// The IR is a lower-level representation that includes type information,
// resolved identifiers, and a simpler structure suitable for code generation.
func Lower(ast *wgsl.Module) (*ir.Module, error) {
	return LowerWithSource(ast, "")
}

// LowerWithSource converts WGSL AST to IR, keeping source for error messages.
// All capabilities are enabled (permissive mode for tools and tests).
// For strict capability validation, use [LowerWithCapabilities].
//
// When source is provided, errors will include line:column information
// and can show source context using ErrorList.FormatAll().
func LowerWithSource(ast *wgsl.Module, source string) (*ir.Module, error) {
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		return nil, err
	}
	return module, nil
}

// LowerWithCapabilities converts WGSL AST to IR with explicit capability control.
// The lowerer is always permissive; capability validation is performed by the
// validator after lowering. Types that require specific capabilities (f64, i64,
// u64, f16) produce validation errors unless the corresponding flag is set.
func LowerWithCapabilities(ast *wgsl.Module, source string, caps ir.Capabilities) (*ir.Module, error) {
	// Lower permissively — all types are accepted during lowering.
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		return nil, err
	}
	// Validate with capability restrictions.
	validationErrors, verr := ir.ValidateWithCapabilities(module, caps)
	if verr != nil {
		return nil, verr
	}
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: %w", &validationErrors[0])
	}
	return module, nil
}

// Validate validates an IR module for correctness with all capabilities enabled.
//
// For capability-restricted validation, use [ir.ValidateWithCapabilities] directly.
// Returns a slice of validation errors. If the slice is empty, validation passed.
func Validate(module *ir.Module) ([]ir.ValidationError, error) {
	return ir.Validate(module)
}
