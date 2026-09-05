//go:build !nogpu

package gpu

import (
	"log"
	"sync"
	"time"
)

// textureOOMHook is set by render/gpu to escalate host lifecycle policy.
var textureOOMHook func()

// SetTextureOOMHook registers a process-wide callback for CreateTexture OOM.
func SetTextureOOMHook(fn func()) { textureOOMHook = fn }

func noteTextureOOM() {
	if textureOOMHook != nil {
		textureOOMHook()
	}
}

var oomLogMu sync.Mutex
var oomLogLast time.Time

// oomLogThrottled logs OOM diagnostics at most once per second. Persistent
// exhaustion fails every frame; unthrottled logging would flood stderr
// (multiwindow 1.3 caps OOM logs at 3 lines/sec).
func oomLogThrottled(format string, args ...any) {
	oomLogMu.Lock()
	defer oomLogMu.Unlock()
	if time.Since(oomLogLast) < time.Second {
		return
	}
	oomLogLast = time.Now()
	log.Printf(format, args...)
}
