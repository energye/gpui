// Package standardtest is the independent standard visual test toolkit.
// It generates CanvasKit standards, gpui under-test images, side-by-side
// merge comparisons, and pixel compare reports. It must not embed code
// into the render engine.
package standardtest

import (
	"os"
	"path/filepath"
)

// Layout:
//
//	standardtest/scenes/           drawing scripts
//	standardtest/fonts/            shared fonts
//	standardtest/canvaskit/        CanvasKit oracle (Node)
//	standardtest/diff/standard/    standard PNGs
//	standardtest/diff/test/        gpui under-test PNGs
//	standardtest/diff/merge/       side-by-side compare PNGs
//	standardtest/diff/pixel/       pixel-diff highlight PNGs (failures)
//	standardtest/diff/report.json  batch compare report
type Paths struct {
	Root      string
	Self      string
	Scenes    string
	Fonts     string
	FontRoot  string // module root (scene font paths are module-relative)
	Diff      string
	Standard  string
	Test      string
	Merge     string
	Pixel     string
	Report    string // report.json path
	CanvasKit string
	Catalog   string // optional policy catalog JSON
}

// ResolvePaths finds the module root and builds standardtest layout paths.
func ResolvePaths(start string) (Paths, error) {
	if start == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Paths{}, err
		}
		start = wd
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return Paths{}, err
	}
	root := dir
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	self := filepath.Join(root, "standardtest")
	diff := filepath.Join(self, "diff")
	return Paths{
		Root:      root,
		Self:      self,
		Scenes:    filepath.Join(self, "scenes"),
		Fonts:     filepath.Join(self, "fonts"),
		FontRoot:  root,
		Diff:      diff,
		Standard:  filepath.Join(diff, "standard"),
		Test:      filepath.Join(diff, "test"),
		Merge:     filepath.Join(diff, "merge"),
		Pixel:     filepath.Join(diff, "pixel"),
		Report:    filepath.Join(diff, "report.json"),
		CanvasKit: filepath.Join(self, "canvaskit"),
		Catalog:   filepath.Join(self, "catalog.json"),
	}, nil
}

// EnsureDiffDirs creates standard/test/merge/pixel output directories.
func (p Paths) EnsureDiffDirs() error {
	for _, d := range []string{p.Standard, p.Test, p.Merge, p.Pixel} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
