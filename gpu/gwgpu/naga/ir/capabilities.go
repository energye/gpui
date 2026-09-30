//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package ir

// Capabilities represents hardware/API features available to the shader.
//
// When compiling WGSL, types like f64, i64, u64, and f16 are only valid
// if the caller enables the corresponding capability. Without it, the
// lowerer rejects the shader with a descriptive error.
type Capabilities uint32

const (
	// CapFloat64 enables f64 scalar type (Float width=8).
	CapFloat64 Capabilities = 1 << 1

	// CapShaderInt64 enables i64 and u64 scalar types (Sint/Uint width=8).
	CapShaderInt64 Capabilities = 1 << 16

	// CapShaderFloat16 enables f16 scalar type (Float width=2).
	CapShaderFloat16 Capabilities = 1 << 26

	// CapAll enables all capabilities. Used by test infrastructure
	// and tools that need to accept any valid WGSL without restriction.
	CapAll Capabilities = CapFloat64 | CapShaderInt64 | CapShaderFloat16
)

// Contains reports whether c includes all the flags in flag.
func (c Capabilities) Contains(flag Capabilities) bool {
	return c&flag == flag
}
