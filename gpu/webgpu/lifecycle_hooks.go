//go:build !(js && wasm)

package webgpu

// AfterSurfaceUnconfigure is invoked after a successful Surface.Unconfigure.
// render/gpu registers PurgeSurfaceResources here (Skia freeGpuResources).
var AfterSurfaceUnconfigure func()

// BeforeDeviceRecover is invoked at the start of ForceRecoverHealthy /
// tryRecoverDeviceLocked abandon sequence so the engine can purge+abandon
// even if OnDeviceAbandon is unset.
var BeforeDeviceRecover func()
