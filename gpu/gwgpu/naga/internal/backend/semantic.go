//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package backend holds cross-backend constants and helpers shared between
// the text backends (HLSL) and binary backends (DXIL) that both target
// the D3D ecosystem. Centralizing these values here prevents the HLSL
// writer and the DXIL emitter from drifting — a drift that directly
// causes D3D12 pipeline-state creation failures at the input-layout
// boundary.
//
// Anything placed here must be:
//   - a spec-level output convention, not IR semantics
//   - referenced by at least two sibling backends that otherwise would
//     duplicate a literal
//   - stable enough that external tooling can rely on it
package backend

// DXIL does not prescribe a name for
// arbitrary (non-SV_*) semantics; the backends choose one and every
// consumer must agree.
//
// A mismatch between the DXIL input signature and the D3D12 input layout
// causes CreateGraphicsPipelineState to return E_INVALIDARG. IDxcValidator
// cannot detect this because the DXIL container is internally consistent;
// the break only surfaces at the container-to-pipeline boundary.
const LocationSemantic = "LOC"
