//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/gpu/gwgpu/naga/glsl"
	"github.com/energye/gpui/gpu/gwgpu/naga/ir"
)

// WGSL 输入一律读 testdata，不在测试里硬编码标准数据。

func h4cReadWGSL(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return string(b)
}

func TestShaderCache_HitReturnsIdenticalGLSL(t *testing.T) {
	clearShaderCache()
	wgsl := h4cReadWGSL(t, "h4c_triangle_vertex.wgsl")

	first, info1, err := compileWGSLToGLSL(glsl.Version330, wgsl, "vs_main", nil)
	if err != nil {
		t.Fatalf("first compile: %v", err)
	}
	if first == "" || len(info1.EntryPointNames) == 0 {
		t.Fatalf("first compile returned empty GLSL/entry points")
	}

	h0, m0, _ := shaderCacheStats()
	second, info2, err := compileWGSLToGLSL(glsl.Version330, wgsl, "vs_main", nil)
	if err != nil {
		t.Fatalf("second compile: %v", err)
	}
	if second != first {
		t.Fatal("cache hit returned different GLSL than miss")
	}
	if len(info2.EntryPointNames) != len(info1.EntryPointNames) {
		t.Fatal("cache hit returned different TranslationInfo")
	}
	h1, m1, size := shaderCacheStats()
	if h1-h0 != 1 {
		t.Errorf("hits delta = %d, want 1", h1-h0)
	}
	if m1-m0 != 0 {
		t.Errorf("misses delta = %d, want 0", m1-m0)
	}
	if size != 1 {
		t.Errorf("cache size = %d, want 1", size)
	}
}

func TestShaderCache_KeysDifferByVersionEntryLayout(t *testing.T) {
	clearShaderCache()
	wgsl := h4cReadWGSL(t, "h4c_triangle_vertex.wgsl")

	if _, _, err := compileWGSLToGLSL(glsl.Version330, wgsl, "vs_main", nil); err != nil {
		t.Fatalf("baseline compile: %v", err)
	}
	_, m0, _ := shaderCacheStats()

	// 不同入口点必须 miss（空串走首个入口，与具名键不共享条目）。
	if _, _, err := compileWGSLToGLSL(glsl.Version330, wgsl, "", nil); err != nil {
		t.Fatalf("empty-entry compile: %v", err)
	}
	_, m1, _ := shaderCacheStats()
	if m1-m0 != 1 {
		t.Errorf("entry-point variant misses delta = %d, want 1", m1-m0)
	}

	// 不同版本必须 miss（layout(binding) 按版本分流，不许串味）。
	if _, _, err := compileWGSLToGLSL(glsl.Version430, wgsl, "vs_main", nil); err != nil {
		t.Fatalf("version variant compile: %v", err)
	}
	_, m2, _ := shaderCacheStats()
	if m2-m1 != 1 {
		t.Errorf("version variant misses delta = %d, want 1", m2-m1)
	}

	// 不同 binding 布局必须 miss。
	bm := map[glsl.BindingMapKey]uint8{{Group: 0, Binding: 0}: 3}
	if _, _, err := compileWGSLToGLSL(glsl.Version330, wgsl, "vs_main", bm); err != nil {
		t.Fatalf("binding-map variant compile: %v", err)
	}
	_, m3, _ := shaderCacheStats()
	if m3-m2 != 1 {
		t.Errorf("binding-map variant misses delta = %d, want 1", m3-m2)
	}

	// 同内容不同 map 顺序必须 hit（键已规范化）。
	bmSame := map[glsl.BindingMapKey]uint8{{Group: 0, Binding: 0}: 3}
	h0, _, _ := shaderCacheStats()
	if _, _, err := compileWGSLToGLSL(glsl.Version330, wgsl, "vs_main", bmSame); err != nil {
		t.Fatalf("same-layout recompile: %v", err)
	}
	h1, _, _ := shaderCacheStats()
	if h1-h0 != 1 {
		t.Errorf("same-layout hits delta = %d, want 1", h1-h0)
	}
}

func TestShaderCache_CloneIsolation(t *testing.T) {
	clearShaderCache()
	wgsl := h4cReadWGSL(t, "h4c_triangle_fragment.wgsl")

	_, info1, err := compileWGSLToGLSL(glsl.Version330, wgsl, "fs_main", nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// 污染返回的 TranslationInfo，不许影响缓存条目。
	for k := range info1.EntryPointNames {
		info1.EntryPointNames[k] = "polluted"
	}
	for k := range info1.TextureMappings {
		delete(info1.TextureMappings, k)
	}

	_, info2, err := compileWGSLToGLSL(glsl.Version330, wgsl, "fs_main", nil)
	if err != nil {
		t.Fatalf("re-fetch: %v", err)
	}
	for _, v := range info2.EntryPointNames {
		if v == "polluted" {
			t.Fatal("cache entry mutated through returned TranslationInfo")
		}
	}
}

func TestShaderCache_EmptySourceNotCached(t *testing.T) {
	clearShaderCache()
	if _, _, err := compileWGSLToGLSL(glsl.Version330, "", "vs_main", nil); err == nil {
		t.Fatal("empty WGSL = nil error, want error")
	}
	_, misses, size := shaderCacheStats()
	if misses != 0 || size != 0 {
		t.Errorf("error path cached: misses=%d size=%d, want 0/0", misses, size)
	}
}

func TestCloneTranslationInfo_DeepCopy(t *testing.T) {
	src := glsl.TranslationInfo{
		EntryPointNames:     map[string]string{"vs_main": "vs_main_glsl"},
		UsedExtensions:      []string{"GL_ARB_test"},
		TextureSamplerPairs: []string{"tex_samp"},
		TextureMappings: map[string]glsl.TextureMapping{
			"combo": {
				TextureBinding: ir.ResourceBinding{Group: 0, Binding: 1},
				SamplerBinding: &ir.ResourceBinding{Group: 0, Binding: 2},
			},
			"image": {
				TextureBinding: ir.ResourceBinding{Group: 0, Binding: 3},
				SamplerBinding: nil,
			},
		},
		Uniforms: []glsl.UniformInfo{
			{BlockName: "U_block", Binding: ir.ResourceBinding{Group: 0, Binding: 0}},
		},
	}

	dst := cloneTranslationInfo(src)
	if len(dst.EntryPointNames) != 1 || dst.EntryPointNames["vs_main"] != "vs_main_glsl" {
		t.Fatalf("EntryPointNames not copied: %v", dst.EntryPointNames)
	}
	if len(dst.UsedExtensions) != 1 || len(dst.TextureSamplerPairs) != 1 || len(dst.Uniforms) != 1 {
		t.Fatal("slices not copied")
	}
	if len(dst.TextureMappings) != 2 {
		t.Fatalf("TextureMappings not copied: %v", dst.TextureMappings)
	}
	// 指针必须换新地址，不许与源共享。
	if dst.TextureMappings["combo"].SamplerBinding == src.TextureMappings["combo"].SamplerBinding {
		t.Fatal("SamplerBinding pointer shared with source")
	}
	if *dst.TextureMappings["combo"].SamplerBinding != *src.TextureMappings["combo"].SamplerBinding {
		t.Fatal("SamplerBinding value differs from source")
	}
	if dst.TextureMappings["image"].SamplerBinding != nil {
		t.Fatal("nil SamplerBinding not preserved")
	}

	// 改 dst 不许影响 src。
	dst.EntryPointNames["vs_main"] = "polluted"
	dst.UsedExtensions[0] = "polluted"
	dst.TextureSamplerPairs[0] = "polluted"
	dst.Uniforms[0].BlockName = "polluted"
	*dst.TextureMappings["combo"].SamplerBinding = ir.ResourceBinding{Group: 9, Binding: 9}
	if src.EntryPointNames["vs_main"] != "vs_main_glsl" || src.UsedExtensions[0] != "GL_ARB_test" ||
		src.TextureSamplerPairs[0] != "tex_samp" || src.Uniforms[0].BlockName != "U_block" ||
		*src.TextureMappings["combo"].SamplerBinding != (ir.ResourceBinding{Group: 0, Binding: 2}) {
		t.Fatal("source mutated through clone")
	}

	// 零值输入不 panic。
	_ = cloneTranslationInfo(glsl.TranslationInfo{})
}
