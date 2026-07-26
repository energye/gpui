package ui_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestUIDoesNotImportGPU enforces G1: ui → render → gpu (no ui → gpu).
func TestUIDoesNotImportGPU(t *testing.T) {
	root := "."
	// Test file lives in ui/; walk from module-relative via this package dir.
	// When running go test ./ui/..., cwd package is ui.
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == "testdata" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if path == "github.com/energye/gpui/gpu" ||
				strings.HasPrefix(path, "github.com/energye/gpui/gpu/") {
				t.Errorf("%s imports forbidden %s", path, path)
			}
		}
		return nil
	})
	if err != nil {
		// If walk fails because tests run with package path, try walking known subdirs.
		t.Logf("walk . : %v (retry subpackages via import graph is enough if empty)", err)
	}
}

// TestUISubpackagesDoNotImportGPU walks platform/scheduler/raster/embedder relative to module.
func TestUISubpackagesDoNotImportGPU(t *testing.T) {
	// Resolve ui root: this file is in package ui.
	dirs := []string{
		".",
		"platform",
		"scheduler",
		"raster",
		"embedder",
	}
	fset := token.NewFileSet()
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Errorf("parse %s: %v", path, err)
				return nil
			}
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				if p == "github.com/energye/gpui/gpu" || strings.HasPrefix(p, "github.com/energye/gpui/gpu/") {
					t.Errorf("%s imports %s (forbidden)", path, p)
				}
			}
			return nil
		})
	}
}
