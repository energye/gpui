//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Command apidoc 校验公开 API 总账文档与代码公开面的一致性，覆盖两个域：
//
//   - render 域：docs/RENDER_API_CATALOG.md 对 render 主包（以及
//     scene/recording/surface/svg 子包）。任何顶层符号与导出方法必须出现在
//     目录文档文本中，否则报缺失并以非零码退出（可挂 CI）。
//   - engine 域：docs/ENGINE_API_CATALOG.md 对 engine 根包与全部 engine/*
//     子包。判定口径=顶层导出符号与**导出类型**的导出方法（挂未导出接收者
//     类型的导出方法名包外不可达，不参与检查）。
//
// 用途：AGENTS.md「API 文档同步纪律」的机器抓手。改 render 或 engine 代码后跑：
//
//	go run ./scripts/apidoc
//
// 跳过规则：render/text 采用族级归纳（附录以 go doc 为准），不参与逐符号检查；
// render/filters 是纯 init 副作用包（无顶层符号），跳过。engine 根包只有包注释
// （doc.go），无顶层符号，扫描结果自然为空。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const docPath = "docs/RENDER_API_CATALOG.md"
const engineDocPath = "docs/ENGINE_API_CATALOG.md"

// scanPkg 解析包顶层目录下的 .go 文件（非递归、排除 _test.go），返回：
// 顶层导出符号（func/type/const/var）与各导出类型的导出方法。
// 注意：render/ 下还有 render/internal、render/text 等子包，包名同为 render，
// 必须只读目录直属文件（子包自成 import 路径，不在目录内）。
func scanPkg(dir string) (top []string, methods map[string][]string) {
	methods = map[string][]string{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readdir %s: %v\n", dir, err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, perr)
			continue
		}
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				if !decl.Name.IsExported() {
					continue
				}
				if decl.Recv == nil {
					top = append(top, decl.Name.Name)
					continue
				}
				recv := ""
				if len(decl.Recv.List) > 0 {
					switch t := decl.Recv.List[0].Type.(type) {
					case *ast.StarExpr:
						if id, ok := t.X.(*ast.Ident); ok {
							recv = "*" + id.Name
						}
					case *ast.Ident:
						recv = t.Name
					}
				}
				methods[recv] = append(methods[recv], decl.Name.Name)
			case *ast.GenDecl:
				for _, s := range decl.Specs {
					switch spec := s.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							top = append(top, spec.Name.Name)
						}
					case *ast.ValueSpec:
						for _, n := range spec.Names {
							if n.IsExported() {
								top = append(top, n.Name)
							}
						}
					}
				}
			}
		}
	}
	sort.Strings(top)
	return top, methods
}

// checkDomain 对照一份目录文档核一个域的全部公开符号，返回缺失条目。
// 口径：符号名按 word boundary（\bName\b）在文档文本中出现即算覆盖；方法只查
// 导出接收者（值或 *T 且 T 大写开头），挂未导出类型的方法名包外不可达，跳过。
func checkDomain(docFile string, kind string) []string {
	doc, err := os.ReadFile(docFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", docFile, err)
		os.Exit(2)
	}
	text := string(doc)
	has := func(name string) bool {
		re, err := regexp.Compile(`\b` + regexp.QuoteMeta(name) + `\b`)
		if err != nil {
			return strings.Contains(text, name)
		}
		return re.MatchString(text)
	}
	var missing []string
	check := func(tag string, names []string) {
		for _, n := range names {
			if !has(n) {
				missing = append(missing, tag+" "+n)
			}
		}
	}
	if kind == "render" {
		// 主包
		top, methods := scanPkg("render")
		check("render mainpkg", top)
		for recv := range methods {
			sort.Strings(methods[recv])
			// 主包公开接收者：类型或 *类型（首字母大写）
			base := strings.TrimPrefix(recv, "*")
			if base == "" || !(base[0] >= 'A' && base[0] <= 'Z') {
				continue
			}
			check("render method", methods[recv])
		}
		// 子包顶层（scene/recording/svg/gpu/hdr；text 族级、filters 无符号）
		for _, sub := range []string{"scene", "recording", "svg", "gpu", "hdr"} {
			t, _ := scanPkg("render/" + sub)
			check("render subpkg", t)
		}
		return missing
	}
	// engine 域：根包 + 全部子包（目录枚举，新增子包自动纳入）
	t0, m0 := scanPkg("engine")
	check("engine mainpkg", t0)
	for recv := range m0 {
		sort.Strings(m0[recv])
		base := strings.TrimPrefix(recv, "*")
		if base == "" || !(base[0] >= 'A' && base[0] <= 'Z') {
			continue
		}
		check("engine method", m0[recv])
	}
	subdirs, err := os.ReadDir("engine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "readdir engine: %v\n", err)
		os.Exit(2)
	}
	var names []string
	for _, e := range subdirs {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		t, m := scanPkg("engine/" + n)
		check("engine subpkg "+n, t)
		for recv := range m {
			sort.Strings(m[recv])
			base := strings.TrimPrefix(recv, "*")
			if base == "" || !(base[0] >= 'A' && base[0] <= 'Z') {
				continue
			}
			check("engine method "+n, m[recv])
		}
	}
	return missing
}

func main() {
	missing := checkDomain(docPath, "render")
	missing = append(missing, checkDomain(engineDocPath, "engine")...)
	if len(missing) > 0 {
		fmt.Printf("API 目录缺失 %d 个符号：\n", len(missing))
		for _, m := range missing {
			fmt.Println("  " + m)
		}
		fmt.Println("→ render 缺失补 docs/RENDER_API_CATALOG.md，engine 缺失补 docs/ENGINE_API_CATALOG.md（增/改/删 API 时同步更新）。")
		os.Exit(1)
	}
	fmt.Println("OK: docs/RENDER_API_CATALOG.md 覆盖 render 主包 + scene/recording/svg/gpu/hdr 全部公开符号")
	fmt.Println("OK: docs/ENGINE_API_CATALOG.md 覆盖 engine 根包 + 全部子包的顶层导出符号与导出类型导出方法")
}

func mapToPrefixed(names []string, prefix string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = prefix + n
	}
	return out
}
