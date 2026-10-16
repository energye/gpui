//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"fmt"
	"os"
	"testing"
)

// TestMain keeps the shader disk cache hermetic: the whole test process runs
// against one fresh temp directory instead of the user's real cache, and the
// directory is removed on exit.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gpui-shader-cache-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: temp cache dir:", err)
		os.Exit(1)
	}
	_ = os.Setenv(shaderCacheEnvDir, dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
