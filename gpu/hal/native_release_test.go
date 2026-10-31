//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package hal

import "testing"

// TestWithForceNativeReleaseOnLost pins the dispatch semantics: nil fn is a
// no-op, no hook runs fn directly, a registered hook receives fn.
func TestWithForceNativeReleaseOnLost(t *testing.T) {
	WithForceNativeReleaseOnLost(nil) // must not panic

	ran := false
	WithForceNativeReleaseOnLost(func() { ran = true })
	if !ran {
		t.Fatal("no hook: fn must run directly")
	}

	prev := forceNativeReleaseOnLostHook
	defer func() { forceNativeReleaseOnLostHook = prev }()

	forwarded := false
	SetForceNativeReleaseOnLostHook(func(fn func()) {
		forwarded = true
		fn()
	})
	ran = false
	WithForceNativeReleaseOnLost(func() { ran = true })
	if !forwarded || !ran {
		t.Fatalf("hook: forwarded=%v ran=%v, want true true", forwarded, ran)
	}
}
