package aac

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoForbiddenImports(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "github.com/energye/gpui/"
	forbidden := []string{
		prefix + "ui",
		prefix + "render",
		prefix + "gpu",
		"import " + "\"C\"",
	}
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) < 4 || e.Name()[len(e.Name())-3:] != ".go" {
			continue
		}
		buf, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		s := string(buf)
		for _, f := range forbidden {
			if strings.Contains(s, f) {
				t.Fatalf("%s contains forbidden import %q", e.Name(), f)
			}
		}
	}
}
