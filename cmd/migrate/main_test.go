package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigUsesEmbeddedConfigWhenPathIsEmpty(t *testing.T) {
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatalf("loadConfig(\"\"): %v", err)
	}
	if cfg.TargetModule != "github.com/energye/gpui" {
		t.Fatalf("unexpected embedded target module: %s", cfg.TargetModule)
	}
	if len(cfg.Mappings) == 0 {
		t.Fatal("embedded config has no mappings")
	}
}

func TestLoadConfigAcceptsConfigDirectory(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, defaultConfigName), `{
  "target_module": "example.com/acme/gpui",
  "target_dir": "/tmp/gpui-target",
  "mappings": [
    {"source": "/tmp/source", "target": "render", "module": "example.com/up/render"}
  ]
}`)

	cfg, err := loadConfig(tmp)
	if err != nil {
		t.Fatalf("loadConfig(directory): %v", err)
	}
	if cfg.TargetModule != "example.com/acme/gpui" {
		t.Fatalf("unexpected target module: %s", cfg.TargetModule)
	}
	if got := cfg.Mappings[0].Target; got != "render" {
		t.Fatalf("unexpected mapping target: %s", got)
	}
}

func TestMigrationRewritesAliasesPrunesAndPreservesTests(t *testing.T) {
	tmp := t.TempDir()
	sourceRoot := filepath.Join(tmp, "source")
	targetDir := filepath.Join(tmp, "target")

	writeTestFile(t, filepath.Join(sourceRoot, "gg", "go.mod"), "module example.com/up/gg\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(sourceRoot, "gg", "good.go"), `package gg

// See example.com/up/wgpu and example.com/up/gpucontext for upstream notes.
import (
	"context"

	"example.com/up/gpucontext"
	"example.com/up/wgpu"
)

const upstreamStringMustRewrite = "example.com/up/wgpu"

type Surface struct {
	_ context.Context
	Buffer wgpu.Buffer
	Device gpucontext.Device
}
`)
	writeTestFile(t, filepath.Join(sourceRoot, "gg", "bad.go"), `package gg

import "example.com/not/migrated"

var _ = migrated.Value
`)
	writeTestFile(t, filepath.Join(sourceRoot, "gg", "uses_optional.go"), `package gg

import "example.com/up/gg/optional"

var _ = optional.Value
`)
	writeTestFile(t, filepath.Join(sourceRoot, "gg", "render_test.go"), `package gg

import "testing"

func TestKept(t *testing.T) {}
`)
	writeTestFile(t, filepath.Join(sourceRoot, "gg", "testdata", "blob.bin"), "\x00\x01fixture")
	writeTestFile(t, filepath.Join(sourceRoot, "gg", "optional", "bad.go"), `package optional

import "example.com/not/migrated"

var Value = migrated.Value
`)

	writeTestFile(t, filepath.Join(sourceRoot, "wgpu", "go.mod"), "module example.com/up/wgpu\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(sourceRoot, "wgpu", "buffer.go"), `package wgpu

type Buffer struct{}
`)

	writeTestFile(t, filepath.Join(sourceRoot, "gpucontext", "go.mod"), "module example.com/up/gpucontext\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(sourceRoot, "gpucontext", "device.go"), `package gpucontext

type Device struct{}
`)

	cfg := Config{
		TargetModule: "example.com/acme/gpui",
		TargetDir:    targetDir,
		Mappings: []Mapping{
			{Source: filepath.Join(sourceRoot, "gg"), Target: "render", Module: "example.com/up/gg", RenamePackage: "render"},
			{Source: filepath.Join(sourceRoot, "wgpu"), Target: "gpu/webgpu", Module: "example.com/up/wgpu", RenamePackage: "webgpu"},
			{Source: filepath.Join(sourceRoot, "gpucontext"), Target: "gpu/context", Module: "example.com/up/gpucontext", RenamePackage: "context", Alias: "gpucontext"},
		},
	}

	excludeSet := makeSet([]string{"go.mod", "go.sum"})
	if _, err := copySources(cfg.Mappings, targetDir, excludeSet); err != nil {
		t.Fatalf("copySources: %v", err)
	}
	for _, m := range cfg.Mappings {
		if m.RenamePackage == "" {
			continue
		}
		if _, err := renamePackageDeclarations(filepath.Join(targetDir, m.Target), packageNameFromModule(m.Module), m.RenamePackage); err != nil {
			t.Fatalf("renamePackageDeclarations(%s): %v", m.Target, err)
		}
	}

	moduleMap := make(map[string]*Mapping)
	for i := range cfg.Mappings {
		moduleMap[cfg.Mappings[i].Module] = &cfg.Mappings[i]
	}
	for _, f := range listAllGoFiles(targetDir) {
		if _, err := rewriteImports(f, moduleMap, cfg.TargetModule, cfg.ReplaceImports, cfg.ImportAliases); err != nil {
			t.Fatalf("rewriteImports(%s): %v", f, err)
		}
	}
	for _, m := range cfg.Mappings {
		if m.RenamePackage == "" {
			continue
		}
		newQualifier := m.RenamePackage
		if m.Alias != "" {
			newQualifier = m.Alias
		}
		for _, f := range listAllGoFiles(targetDir) {
			if _, err := renamePackageQualifier(f, packageNameFromModule(m.Module), newQualifier); err != nil {
				t.Fatalf("renamePackageQualifier(%s): %v", f, err)
			}
		}
	}

	removed, err := pruneUnresolvedProductionFiles(targetDir, cfg.Mappings, cfg.TargetModule, nil, cfg.ReplaceImports)
	if err != nil {
		t.Fatalf("pruneUnresolvedProductionFiles: %v", err)
	}
	cleanEmptyDirs(filepath.Join(targetDir, "render"))
	if err := validateMigrationOutput(targetDir, cfg.Mappings, cfg.TargetModule, nil, cfg.ReplaceImports); err != nil {
		t.Fatalf("validateMigrationOutput: %v", err)
	}

	wantRemoved := map[string]bool{
		"render/bad.go":           true,
		"render/optional/bad.go":  true,
		"render/uses_optional.go": true,
	}
	for _, rel := range removed {
		delete(wantRemoved, rel)
	}
	if len(wantRemoved) != 0 {
		t.Fatalf("missing removals: %v; got %v", wantRemoved, removed)
	}

	good := readTestFile(t, filepath.Join(targetDir, "render", "good.go"))
	requireContains(t, good, "package render")
	requireContains(t, good, `"example.com/acme/gpui/gpu/webgpu"`)
	requireContains(t, good, `gpucontext "example.com/acme/gpui/gpu/context"`)
	requireContains(t, good, "Buffer webgpu.Buffer")
	requireContains(t, good, "Device gpucontext.Device")
	requireContains(t, good, "// See example.com/acme/gpui/gpu/webgpu and example.com/acme/gpui/gpu/context for upstream notes.")
	requireContains(t, good, `const upstreamStringMustRewrite = "example.com/acme/gpui/gpu/webgpu"`)
	requireNotContains(t, good, "example.com/up/wgpu")
	requireNotContains(t, good, "wgpu.Buffer")

	if _, err := os.Stat(filepath.Join(targetDir, "render", "render_test.go")); err != nil {
		t.Fatalf("test file was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "render", "testdata", "blob.bin")); err != nil {
		t.Fatalf("testdata was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "render", "optional")); !os.IsNotExist(err) {
		t.Fatalf("empty pruned package directory still exists or stat failed: %v", err)
	}
}

func TestRewriteImportsSupportsConfiguredImportAliases(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "consumer.go")
	writeTestFile(t, file, `package consumer

import "example.com/up/toolkit"

var _ = toolkit.Value
`)

	mapping := Mapping{Source: filepath.Join(tmp, "toolkit"), Target: "tools", Module: "example.com/up/toolkit", RenamePackage: "tools"}
	moduleMap := map[string]*Mapping{mapping.Module: &mapping}
	changed, err := rewriteImports(file, moduleMap, "example.com/acme/gpui", nil, map[string]string{
		"example.com/acme/gpui/tools": "gputools",
	})
	if err != nil {
		t.Fatalf("rewriteImports: %v", err)
	}
	if !changed {
		t.Fatal("rewriteImports did not report a change")
	}

	got := readTestFile(t, file)
	requireContains(t, got, `gputools "example.com/acme/gpui/tools"`)
}

func TestRewriteImportsRewritesStringOnlyReferences(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "string_only.go")
	writeTestFile(t, file, `package stringonly

const docs = "example.com/up/toolkit/subpkg"
`)

	mapping := Mapping{Source: filepath.Join(tmp, "toolkit"), Target: "tools", Module: "example.com/up/toolkit", RenamePackage: "tools"}
	moduleMap := map[string]*Mapping{mapping.Module: &mapping}
	changed, err := rewriteImports(file, moduleMap, "example.com/acme/gpui", nil, nil)
	if err != nil {
		t.Fatalf("rewriteImports: %v", err)
	}
	if !changed {
		t.Fatal("rewriteImports did not report a string-only change")
	}

	got := readTestFile(t, file)
	requireContains(t, got, `const docs = "example.com/acme/gpui/tools/subpkg"`)
	requireNotContains(t, got, "example.com/up/toolkit")
}

func TestValidateMigrationOutputRejectsUnexpectedProductionImports(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "target")
	writeTestFile(t, filepath.Join(targetDir, "render", "good.go"), `package render

import "example.com/not/migrated"

var _ = migrated.Value
`)
	writeTestFile(t, filepath.Join(targetDir, "render", "good_test.go"), `package render

import "example.com/test/only"

func TestOnly() {}
`)

	err := validateMigrationOutput(targetDir, []Mapping{
		{Target: "render", Module: "example.com/up/gg", RenamePackage: "render"},
	}, "example.com/acme/gpui", nil, nil)
	if err == nil {
		t.Fatal("validateMigrationOutput accepted an unexpected production import")
	}
	requireContains(t, err.Error(), "render/good.go imports example.com/not/migrated")
	requireNotContains(t, err.Error(), "example.com/test/only")
}

func packageNameFromModule(module string) string {
	if i := strings.LastIndex(module, "/"); i >= 0 {
		return module[i+1:]
	}
	return module
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(data)
}

func requireContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected to find %q in:\n%s", substr, s)
	}
}

func requireNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("did not expect to find %q in:\n%s", substr, s)
	}
}
