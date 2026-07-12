//go:build !(js && wasm)

package software

import "github.com/energye/gpui/gpu/webgpu/hal"

// init registers the software backend with the HAL registry.
func init() {
	hal.RegisterBackend(API{})
}
