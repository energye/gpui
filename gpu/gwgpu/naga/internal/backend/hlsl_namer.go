//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package backend

import "strings"

// NeedsTrailingUnderscore reports whether a variable name requires a
// trailing underscore suffix in HLSL output. The HLSL namer (matching
// with an ASCII digit or collides with an HLSL reserved keyword. The
// DXIL backend uses this to produce metadata resource names that match
// what DXC would generate from the HLSL roundtrip.
func NeedsTrailingUnderscore(name string) bool {
	if name == "" {
		return false
	}
	if EndsWithDigit(name) {
		return true
	}
	if _, found := HLSLReservedKeywords[name]; found {
		return true
	}
	_, found := HLSLCaseInsensitiveKeywords[strings.ToLower(name)]
	return found
}

// EndsWithDigit checks if a string ends with an ASCII digit.
func EndsWithDigit(s string) bool {
	if s == "" {
		return false
	}
	c := s[len(s)-1]
	return c >= '0' && c <= '9'
}

// IsASCIIAlphanumeric checks if a rune is an ASCII letter or digit.
func IsASCIIAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
