package embedder

import (
	"errors"
	"strings"
	"testing"
)

func TestOOMExit_StreakAndLatch(t *testing.T) {
	var o oomExit
	if o.note(nil) {
		t.Fatal("nil must reset, not exit")
	}
	if o.note(errors.New("surface outdated")) {
		t.Fatal("non-OOM must reset, not exit")
	}
	oom := errors.New("CreateTexture failed: out of memory")
	if o.note(oom) {
		t.Fatal("1st OOM must not exit")
	}
	if o.note(oom) {
		t.Fatal("2nd OOM must not exit")
	}
	if !o.note(oom) {
		t.Fatal("3rd consecutive OOM must exit")
	}
	if err := o.runErr(); err == nil || !strings.Contains(err.Error(), "GPUI_POWER") {
		t.Fatalf("latched error must name the action, got %v", err)
	}
	if !o.note(oom) {
		t.Fatal("stays latched after exit")
	}
	// Success resets the streak only before the latch trips.
	var o2 oomExit
	o2.note(oom)
	o2.note(oom)
	o2.note(nil)
	if o2.note(oom) {
		t.Fatal("streak must restart after a success")
	}
	if o2.note(oom) {
		t.Fatal("restarted streak at 2 must not exit")
	}
	if !o2.note(oom) {
		t.Fatal("restarted streak at 3 must exit")
	}
	var nilExit *oomExit
	if nilExit.note(oom) || nilExit.runErr() != nil {
		t.Fatal("nil tracker must be inert")
	}
}
