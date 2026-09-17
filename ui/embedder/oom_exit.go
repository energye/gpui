package embedder

import (
	"github.com/energye/gpui/render"
)

// oomFailThreshold mirrors render.OOMExitThreshold (single authority in
// render/oom_exit.go). Kept as an alias so present-loop call sites read
// unchanged; the count, phrasing, and message all come from render.
const oomFailThreshold = render.OOMExitThreshold

// oomExit tracks consecutive OOM-class present failures and latches the
// human-readable exit reason once. Embedded by App and PipelineApp (raster
// thread notes, UI thread reads the latched error).
// Thin delegate over render.OOMExit — behavior identical, one implementation.
type oomExit struct {
	render.OOMExit
}

// note tracks one present result: nil and non-OOM errors reset the streak,
// OOM errors advance it. Reports true once the window must exit.
func (o *oomExit) note(err error) bool {
	if o == nil {
		return false
	}
	return o.OOMExit.Note(err)
}

// runErr returns the latched exit reason, if any.
func (o *oomExit) runErr() error {
	if o == nil {
		return nil
	}
	return o.OOMExit.RunErr()
}
