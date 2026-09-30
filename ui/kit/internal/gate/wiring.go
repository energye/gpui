//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gate

import (
	"strings"
)

// CheckWiring reports whether product imports stay within L1/L2 plus
// theme: direct render/gpu imports fail. Stdlib and other ui/* engine
// packages pass; only the two forbidden roots are judged here.
func CheckWiring(imports []string) (bool, []string) {
	var bad []string
	for _, imp := range imports {
		p := strings.TrimSpace(imp)
		if p == "" {
			continue
		}
		if isForbiddenImport(p) {
			bad = append(bad, p)
		}
	}
	return len(bad) == 0, bad
}

func isForbiddenImport(p string) bool {
	if p == "render" || p == "gpu" {
		return true
	}
	if strings.HasSuffix(p, "/render") || strings.HasSuffix(p, "/gpu") {
		return true
	}
	if strings.Contains(p, "/render/") || strings.Contains(p, "/gpu/") {
		return true
	}
	return false
}
