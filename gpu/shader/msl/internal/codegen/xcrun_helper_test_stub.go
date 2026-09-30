//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !darwin

package codegen

import "testing"

func verifyMSLWithXcrun(t *testing.T, _ string) {
	t.Helper()
	t.Skip("xcrun metal not available on this platform")
}
