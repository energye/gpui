//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package webgpu

// AfterSurfaceUnconfigure is invoked after a successful Surface.Unconfigure.
// render/gpu registers PurgeSurfaceResources here.
var AfterSurfaceUnconfigure func()

// BeforeDeviceRecover is invoked at the start of ForceRecoverHealthy /
// tryRecoverDeviceLocked abandon sequence so the engine can purge+abandon
// even if OnDeviceAbandon is unset.
var BeforeDeviceRecover func()
