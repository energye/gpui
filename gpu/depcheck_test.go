//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package gpu_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestGPUDoesNotImportRenderOrUI pins the one-way call direction of AGENTS
// 分层单向调用: ui → render → gpu, so gpu/** production code must not import
// the render or ui trees. _test.go files are test seams (fixtures may reach
// upward); testdata/vendor are skipped.
func TestGPUDoesNotImportRenderOrUI(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "testdata", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if impPath == "github.com/energye/gpui/render" ||
				strings.HasPrefix(impPath, "github.com/energye/gpui/render/") ||
				impPath == "github.com/energye/gpui/ui" ||
				strings.HasPrefix(impPath, "github.com/energye/gpui/ui/") {
				t.Errorf("%s imports forbidden %q (gpu must not call upward)", path, impPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk gpu tree: %v", err)
	}
}
