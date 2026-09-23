package color

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Compliance lock for V-U3: video core stays pure Go with zero reverse
// dependencies. Fails the package when any file imports CGO or the
// display layers (bridge code lives window-side only).
func TestNoForbiddenImports(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(".", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no go files")
	}
	fset := token.NewFileSet()
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		ast, err := parser.ParseFile(fset, f, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range ast.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if p == "C" {
				t.Fatalf("%s: forbidden import C", f)
			}
			for _, ban := range []string{"gpui/ui", "gpui/render", "gpui/gpu"} {
				if strings.Contains(p, ban) {
					t.Fatalf("%s: forbidden import %s", f, p)
				}
			}
		}
	}
}
