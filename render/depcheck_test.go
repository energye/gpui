//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// backendWhitelist lists the render-tree production files allowed to import a
// concrete backend package. Everything else under render/ must go through
// gpu/hal interfaces (AGENTS 渲染层后端解耦). Entries are relative to render/.
//
//	backend.go / present_swapchain*.go / register_gles.go — 创建入口族
//	  (webgpu.CreateInstance 唯一句、GL 建链、后端注册行，AGENTS 明文豁免)
//	gpu/gpu.go            — GPU 注册包本体（导入即注册 accelerator/surface 钩子）
//	internal/gpu/gpu_shared.go     — DeviceFromHandle 设备桥解包 + webgpu 门面胶水
//	                                   （rwgpu 直引已收敛 hal，P2.5 D2）
//	internal/gpu/vello_accelerator.go — DeviceFromHandle 设备桥解包
var backendWhitelist = map[string]string{
	"backend.go":                        "创建入口：webgpu.CreateInstance 唯一句",
	"present_swapchain.go":              "创建入口：native 建链",
	"present_swapchain_gl.go":           "创建入口：GL 建链",
	"register_gles.go":                  "创建入口：gles 注册行",
	"gpu/gpu.go":                        "GPU 注册包本体",
	"internal/gpu/gpu_shared.go":        "设备桥解包 + 门面胶水",
	"internal/gpu/vello_accelerator.go": "设备桥解包",
}

func isBackendImport(impPath string) bool {
	for _, root := range []string{
		"github.com/energye/gpui/gpu/webgpu",
		"github.com/energye/gpui/gpu/gwgpu",
		"github.com/energye/gpui/gpu/rwgpu",
	} {
		if impPath == root || strings.HasPrefix(impPath, root+"/") {
			return true
		}
	}
	return false
}

// TestRenderDoesNotImportBackends pins AGENTS 渲染层后端解耦: render/**
// production code stores only hal interfaces + types + the gpucontext bridge;
// concrete backend imports (webgpu / gwgpu / rwgpu) require an explicit
// 创建入口 whitelist entry above. _test.go files are test seams.
func TestRenderDoesNotImportBackends(t *testing.T) {
	seen := map[string]bool{}
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
		rel, err := filepath.Rel(".", path)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if !isBackendImport(impPath) {
				continue
			}
			if _, ok := backendWhitelist[rel]; ok {
				seen[rel] = true
				continue
			}
			t.Errorf("%s imports forbidden backend %q (add to backendWhitelist only for 创建入口/桥接 glue)", rel, impPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk render tree: %v", err)
	}
	// Stale whitelist entries must be pruned so the list mirrors reality.
	for file, why := range backendWhitelist {
		if !seen[file] {
			t.Errorf("backendWhitelist entry %q (%s) no longer imports a backend — prune it", file, why)
		}
	}
}
