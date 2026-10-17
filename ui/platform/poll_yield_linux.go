//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux

package platform

import "runtime"

// yieldBeforeBlockingPoll hands the current P back to the Go scheduler right
// before a long blocking fd wait (poll over the display fd + wake pipe).
//
// A goroutine parked in a blocking syscall holds its P in _Psyscall with a
// frozen schedtick; runtime sysmon then re-takes that P on every ~10ms
// observation (runtime/proc.go retake → preemptone), which pins sysmon at its
// 20µs scan interval instead of backing off — thousands of park/wake cycles
// per second (~1% of one core at 60fps) spent on a P that cannot be
// preempted while it sits in the kernel. Re-entering the scheduler first
// bumps the P's schedtick (runtime execute), so sysmon sees a fresh tick and
// leaves the P alone. Cost is one runqueue hop on the waiting thread; the
// wait itself stays a single blocking poll over all fds, so the event-loop
// model and event latency are unchanged.
func yieldBeforeBlockingPoll() {
	runtime.Gosched()
}
