//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// HLSL is Microsoft's shader language for DirectX and is used extensively
// on Windows platforms. This package generates HLSL source code compatible
// with both legacy FXC (Shader Model 5.x) and modern DXC (Shader Model 6.x)
// compilers.
//
// # Shader Model Support
//
// The package supports Shader Models from 5.0 to 6.7:
//   - SM 5.0-5.1: Legacy FXC compiler, DXBC output
//   - SM 6.0+: Modern DXC compiler, DXIL output
//
// # Usage
//
//	module := parseWGSL(source) // or other frontend
//	options := hlsl.DefaultOptions()
//	options.ShaderModel = hlsl.ShaderModel6_0
//
//	hlslCode, info, err := hlsl.Compile(module, options)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Register Binding
//
// HLSL uses register-based resource binding with spaces:
//
//	cbuffer : register(b#, space#) // Constant buffers
//	Texture : register(t#, space#) // Textures/SRVs
//	Sampler : register(s#, space#) // Samplers
//	RWTexture: register(u#, space#) // UAVs
//
// The BindingMap in Options allows explicit control over register assignment.
package hlsl
