package render

import (
	"errors"
	"strings"
	"testing"
)

// TestOOMExit_SingleAuthority locks the merged OOM policy: the runtime
// present loop (ui/embedder delegate) and the open-time readiness gate share
// this threshold, phrasing, and message. Three consecutive OOMs exit;
// anything else resets; nil receiver stays inert.
func TestOOMExit_SingleAuthority(t *testing.T) {
	if OOMExitThreshold != 3 {
		t.Fatalf("OOMExitThreshold = %d, want 3 (multiwindow 1.3)", OOMExitThreshold)
	}
	var o OOMExit
	if o.Note(nil) {
		t.Fatal("nil must reset, not exit")
	}
	if o.Note(errors.New("surface outdated")) {
		t.Fatal("non-OOM must reset, not exit")
	}
	oom := errors.New("CreateTexture failed: out of memory")
	if o.Note(oom) {
		t.Fatal("1st OOM must not exit")
	}
	if o.Note(oom) {
		t.Fatal("2nd OOM must not exit")
	}
	if !o.Note(oom) {
		t.Fatal("3rd consecutive OOM must exit")
	}
	if err := o.RunErr(); err == nil || !strings.Contains(err.Error(), "GPUI_POWER") {
		t.Fatalf("latched error must name the action, got %v", err)
	}
	var nilExit *OOMExit
	if nilExit.Note(oom) || nilExit.RunErr() != nil {
		t.Fatal("nil tracker must be inert")
	}
}
