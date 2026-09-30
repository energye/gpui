//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !linux && !windows && !darwin

package text

// platformSystemFontCandidates: no built-in paths on this GOOS.
// Use text.SetDefaultFontPath / SetSystemFontPaths / FontResolver.SetFontFile.
func platformSystemFontCandidates(b FontRole) []string {
	return nil
}
