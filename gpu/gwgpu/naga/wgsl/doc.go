//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package wgsl provides WGSL parsing and lowering.
//
// WGSL is the shader language for WebGPU, designed to be portable
// and map well to modern GPU APIs like Vulkan, Metal, and DX12.
//
// # Components
//
// The wgsl package consists of several components:
//
//   - Lexer: Tokenizes WGSL source code into tokens
//   - Parser: Parses tokens into an AST (Abstract Syntax Tree)
//   - Lowerer: Converts AST to IR (Intermediate Representation)
//
// # Usage
//
// To parse and lower a WGSL shader:
//
//	lexer := wgsl.NewLexer(source)
//	tokens, err := lexer.Tokenize()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	parser := wgsl.NewParser(tokens)
//	module, err := parser.Parse()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	irModule, err := wgsl.LowerWithSource(module, source)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # WGSL Specification
//
// This implementation follows the WGSL specification:
// https://www.w3.org/TR/WGSL/
//
// # Supported Features
package wgsl
