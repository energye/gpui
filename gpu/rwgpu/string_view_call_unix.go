//go:build linux || darwin

package rwgpu

import (
	"github.com/ebitengine/purego"
)

func callHandleStringView(proc Proc, handle uintptr, label *StringView) {
	unix, ok := proc.(*unixProc)
	if !ok || unix.fnPtr == 0 {
		return
	}

	var call func(uintptr, StringView)
	purego.RegisterFunc(&call, unix.fnPtr)
	call(handle, *label)
}
