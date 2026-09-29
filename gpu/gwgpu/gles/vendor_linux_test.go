//go:build linux && !(js && wasm)

package gles

import (
	"os"
	"path/filepath"
	"testing"
)

func writeVendorJSON(t *testing.T, dir, name, lib string) {
	t.Helper()
	raw := `{"file_format_version" : "1.0.0", "ICD" : {"library_path" : "` + lib + `"}}`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVendorPinForPower(t *testing.T) {
	dir := t.TempDir()
	writeVendorJSON(t, dir, "10_nvidia.json", "libEGL_nvidia.so.0")
	writeVendorJSON(t, dir, "50_mesa.json", "libEGL_mesa.so.0.0.0")
	writeVendorJSON(t, dir, "README.txt", "not-json")

	cases := []struct {
		name   string
		power  string
		env    string
		want   string
		wantOK bool
	}{
		{"high pins nvidia", "high", "", filepath.Join(dir, "10_nvidia.json"), true},
		{"discrete alias", "discrete", "", filepath.Join(dir, "10_nvidia.json"), true},
		{"low pins mesa", "low", "", filepath.Join(dir, "50_mesa.json"), true},
		{"unset leaves loader default", "", "", "", false},
		{"explicit env wins", "high", "/custom/vendor.json", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := vendorPinForPower(tc.power, tc.env, dir)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("vendorPinForPower(%q) = (%q, %v) want (%q, %v)",
					tc.power, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestVendorPinMissingDirFailsOpen(t *testing.T) {
	if got, ok := vendorPinForPower("high", "", filepath.Join(t.TempDir(), "absent")); ok || got != "" {
		t.Fatalf("missing dir must fail open, got (%q, %v)", got, ok)
	}
}

func TestRendererIsSoftware(t *testing.T) {
	for _, sw := range []string{
		"llvmpipe (LLVM 15)",
		"Gallium softpipe on llvmpipe",
		"Software Rasterizer",
		"ANGLE SwiftShader",
	} {
		if !rendererIsSoftware(sw) {
			t.Fatalf("rendererIsSoftware(%q) = false, want true", sw)
		}
	}
	for _, hw := range []string{
		"NVIDIA GeForce 940MX/PCIe/SSE2",
		"Mesa Intel(R) HD Graphics 520 (SKL GT2)",
		"Apple M1",
	} {
		if rendererIsSoftware(hw) {
			t.Fatalf("rendererIsSoftware(%q) = true, want false", hw)
		}
	}
}
