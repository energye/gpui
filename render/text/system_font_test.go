package text_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/energye/gpui/render/text"
)

func TestSystemFontCandidates_UINotEmpty(t *testing.T) {
	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		c := text.SystemFontCandidates(text.FontRoleUI)
		if len(c) == 0 {
			t.Fatalf("expected platform UI candidates on %s", runtime.GOOS)
		}
		t.Logf("UI candidates (%d): first=%s", len(c), c[0])
	default:
		t.Log("skip on", runtime.GOOS)
	}
}

func TestSetDefaultFontPath_Override(t *testing.T) {
	text.ClearSystemFontPaths()
	t.Cleanup(text.ClearSystemFontPaths)

	fake := filepath.Join(t.TempDir(), "app-font.ttf")
	text.SetDefaultFontPath(fake)
	c := text.SystemFontCandidates(text.FontRoleUI)
	if len(c) == 0 || c[0] != fake {
		t.Fatalf("SetDefaultFontPath should lead, got %v", c)
	}
}

func TestFontResolver_SetFontFile_Isolated(t *testing.T) {
	text.ClearSystemFontPaths()
	t.Cleanup(text.ClearSystemFontPaths)

	procFake := filepath.Join(t.TempDir(), "process.ttf")
	text.SetDefaultFontPath(procFake)

	r := text.NewFontResolver()
	appFake := filepath.Join(t.TempDir(), "app.ttf")
	r.SetFontFile(appFake)

	c := r.Candidates(text.FontRoleUI)
	if len(c) == 0 || c[0] != appFake {
		t.Fatalf("SetFontFile should lead resolver, got %v", c)
	}
	for _, p := range c {
		if p == procFake {
			t.Fatalf("isolated resolver must not see process SetDefaultFontPath")
		}
	}
}

func TestLoadDefaultFace_SingleSystemFont(t *testing.T) {
	text.ClearSystemFontPaths()
	face, desc, err := text.LoadDefaultFace(16)
	if err != nil {
		t.Skip(err)
	}
	t.Log("default face:", desc)
	if face == nil {
		t.Fatal("nil face")
	}
	if !face.HasGlyph('A') {
		t.Fatal("default UI font should have Latin A")
	}
	if _, ok := face.(*text.MultiFace); ok {
		t.Fatal("LoadDefaultFace must not build MultiFace")
	}
}

func TestLoadMultiFace_OptIn(t *testing.T) {
	text.ClearSystemFontPaths()
	face, desc, err := text.LoadMultiFace(16)
	if err != nil {
		t.Skip(err)
	}
	t.Log("multi:", desc)
	if face == nil {
		t.Fatal("nil")
	}
	_ = face.HasGlyph('A')
}
