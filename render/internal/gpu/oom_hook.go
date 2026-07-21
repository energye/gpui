//go:build !nogpu

package gpu

// textureOOMHook is set by render/gpu to escalate host lifecycle policy.
var textureOOMHook func()

// SetTextureOOMHook registers a process-wide callback for CreateTexture OOM.
func SetTextureOOMHook(fn func()) { textureOOMHook = fn }

func noteTextureOOM() {
	if textureOOMHook != nil {
		textureOOMHook()
	}
}
