//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/energye/gpui/gpu/gwgpu/naga/glsl"
	"github.com/energye/gpui/gpu/gwgpu/naga/ir"
)

// fullTranslationInfo exercises every field of TranslationInfo, including the
// nil and non-nil SamplerBinding forms, so a round trip proves the whole shape
// survives serialization.
func fullTranslationInfo() glsl.TranslationInfo {
	return glsl.TranslationInfo{
		EntryPointNames:     map[string]string{"vs_main": "vs_main_glsl", "fs_main": "fs_main_glsl"},
		UsedExtensions:      []string{"GL_ARB_test"},
		RequiredVersion:     glsl.Version{Major: 4, Minor: 30, ES: false},
		TextureSamplerPairs: []string{"tex_samp"},
		TextureMappings: map[string]glsl.TextureMapping{
			"combo": {
				TextureBinding: ir.ResourceBinding{Group: 0, Binding: 1},
				SamplerBinding: &ir.ResourceBinding{Group: 0, Binding: 2},
			},
			"image": {
				TextureBinding: ir.ResourceBinding{Group: 1, Binding: 3},
				SamplerBinding: nil,
			},
		},
		Uniforms: []glsl.UniformInfo{
			{BlockName: "U_block", Binding: ir.ResourceBinding{Group: 0, Binding: 0}},
			{BlockName: "S_block", Binding: ir.ResourceBinding{Group: 1, Binding: 4}, IsStorage: true},
		},
	}
}

func TestShaderDiskCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := newShaderDiskCache(dir)
	key := shaderCacheKey(glsl.Version330, "wgsl-source", "vs_main", map[glsl.BindingMapKey]uint8{{Group: 0, Binding: 1}: 2})
	info := fullTranslationInfo()

	c.put(key, "glsl text", info)
	if c.writes.Load() != 1 || c.errors.Load() != 0 {
		t.Fatalf("put counters: writes=%d errors=%d, want 1/0", c.writes.Load(), c.errors.Load())
	}

	// A second instance is the fresh-process case: only the file connects them.
	c2 := newShaderDiskCache(dir)
	code, got, ok := c2.get(key)
	if !ok {
		t.Fatalf("get after put: miss (errors=%d)", c2.errors.Load())
	}
	if code != "glsl text" {
		t.Fatalf("glsl text = %q, want %q", code, "glsl text")
	}
	if !reflect.DeepEqual(got, info) {
		t.Fatalf("translation info mismatch:\n got %+v\nwant %+v", got, info)
	}
	if c2.hits.Load() != 1 || c2.errors.Load() != 0 {
		t.Fatalf("get counters: hits=%d errors=%d, want 1/0", c2.hits.Load(), c2.errors.Load())
	}
}

func TestShaderDiskCache_MissWhenAbsent(t *testing.T) {
	c := newShaderDiskCache(t.TempDir())
	code, _, ok := c.get("never-written-key")
	if ok || code != "" {
		t.Fatal("absent entry reported as hit")
	}
	if c.misses.Load() != 1 || c.errors.Load() != 0 {
		t.Fatalf("counters: misses=%d errors=%d, want 1/0 (missing file is a plain miss)", c.misses.Load(), c.errors.Load())
	}
}

func TestShaderDiskCache_RejectsBrokenEntries(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"garbage", "this is not json"},
		{"unknown field", `{"format":1,"glsl":"void main(){}","info":{},"extra":1}`},
		{"format mismatch", `{"format":999,"glsl":"void main(){}","info":{}}`},
		{"empty glsl", `{"format":1,"glsl":"","info":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			c := newShaderDiskCache(dir)
			key := "broken-key"
			if err := os.WriteFile(filepath.Join(dir, c.fileName(key)), []byte(tc.data), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			if _, _, ok := c.get(key); ok {
				t.Fatal("broken entry reported as hit")
			}
			if c.misses.Load() != 1 || c.errors.Load() != 1 {
				t.Fatalf("counters: misses=%d errors=%d, want 1/1", c.misses.Load(), c.errors.Load())
			}
			// put must be able to replace the broken entry.
			c.put(key, "void main(){}", glsl.TranslationInfo{})
			if c.writes.Load() != 1 {
				t.Fatalf("writes after repair = %d, want 1", c.writes.Load())
			}
			if _, _, ok := c.get(key); !ok {
				t.Fatal("repaired entry not readable")
			}
		})
	}
}

func TestShaderDiskCache_ResolveFromEnv(t *testing.T) {
	t.Setenv(shaderCacheEnvDir, "")
	for _, v := range []string{"0", "off", "false", "OFF"} {
		t.Setenv(shaderCacheEnvDisable, v)
		if c := resolveShaderDiskCache(); c != nil {
			t.Fatalf("disable value %q still resolved a cache", v)
		}
	}

	t.Setenv(shaderCacheEnvDisable, "")
	dir := t.TempDir()
	t.Setenv(shaderCacheEnvDir, dir)
	c := resolveShaderDiskCache()
	if c == nil {
		t.Fatal("dir override resolved nil")
	}
	if c.dir != dir {
		t.Fatalf("cache dir = %q, want %q", c.dir, dir)
	}
}

func TestShaderDiskCache_ServesAcrossMemoryCacheResets(t *testing.T) {
	wgsl := h4cReadWGSL(t, "h4c_triangle_vertex.wgsl")
	c := newShaderDiskCache(t.TempDir())
	prev := shaderDiskCacheFor
	shaderDiskCacheFor = func() *shaderDiskCache { return c }
	defer func() { shaderDiskCacheFor = prev }()

	clearShaderCache()
	first, info1, err := compileWGSLToGLSL(glsl.Version330, wgsl, "vs_main", nil)
	if err != nil {
		t.Fatalf("first compile: %v", err)
	}
	if c.writes.Load() != 1 {
		t.Fatalf("disk writes after first compile = %d, want 1", c.writes.Load())
	}

	// Fresh-process shape: memory memo gone, disk entry left behind.
	clearShaderCache()
	second, info2, err := compileWGSLToGLSL(glsl.Version330, wgsl, "vs_main", nil)
	if err != nil {
		t.Fatalf("second compile (disk): %v", err)
	}
	if second != first {
		t.Fatal("disk hit returned different GLSL than the original compile")
	}
	if !reflect.DeepEqual(info1, info2) {
		t.Fatalf("disk hit returned different TranslationInfo:\n got %+v\nwant %+v", info2, info1)
	}
	if c.hits.Load() != 1 {
		t.Fatalf("disk hits = %d, want 1", c.hits.Load())
	}
}
