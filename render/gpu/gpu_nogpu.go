//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build nogpu

// Package gpu is the `-tags nogpu` form of the GPU registration package.
//
// Importing it is always safe: with the nogpu build tag there is no GPU
// accelerator, coverage filler, or surface lifecycle policy to register, so
// this file provides an empty package and consumers that blank-import
// render/gpu keep compiling and fall back to CPU rendering. Code that calls
// the exported GPU-only helpers must gate those calls with `!nogpu`.
package gpu
